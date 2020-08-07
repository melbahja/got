package got

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
)

type (

	// Downloadable URL info.
	Info struct {

		// File content length.
		Length int64

		// Supports partial content?
		Rangeable bool

		// URL Redirected.
		Redirected bool
	}

	// Got Download.
	Download struct {

		// Download file info.
		*Info

		// Progress func called based on Interval.
		ProgressFunc

		// URL to download
		URL string

		// File destination
		Dest string

		// Progress interval in ms.
		Interval int

		// Split file chunk by size in bytes.
		ChunkSize int64

		// Set maximum chunk size.
		MaxChunkSize int64

		// Set min chunk size.
		MinChunkSize int64

		// Max chunks to download at same time.
		Concurrency int

		// Stop progress loop.
		StopProgress bool

		// Chunks temp dir.
		temp string

		// Donwload file chunks.
		chunks []*Chunk

		// Http client.
		client *http.Client

		// Is the URL redirected to different location.
		redirected bool

		// Progress...
		progress *Progress

		// Chunk merge index.
		index int

		// Sync mutex.
		mu sync.RWMutex
	}
)


// Check Download and split file to chunks and set defaults,
// you should call Init first then call Start
func (d *Download) Init() error {

	var (
		err                                error
		i, startRange, endRange, chunksLen int64
	)

	// Set http client
	d.client = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			d.redirected = true
			return nil
		},
	}

	// Init progress.
	d.progress = &Progress{
		mu: d.mu,
	}

	// Set default interval.
	if d.Interval == 0 {
		d.Interval = 20
	}

	// Get URL info.
	if d.Info, err = d.GetInfo(); err != nil {
		return err
	}

	// Partial content not supported 😢!
	if d.Info.Rangeable == false || d.Info.Length == 0 {
		return nil
	}

	// Set concurrency default to 10.
	if d.Concurrency == 0 {
		d.Concurrency = 10
	}

	// Set default chunk size
	if d.ChunkSize == 0 {

		d.ChunkSize = d.Info.Length / int64(d.Concurrency)

		// if chunk size >= 102400000 bytes set default to (ChunkSize / 2)
		if d.ChunkSize >= 102400000 {
			d.ChunkSize = d.ChunkSize / 2
		}

		// Set default min chunk size to 1m, or file size / 2
		if d.MinChunkSize == 0 {

			d.MinChunkSize = 1000000

			if d.MinChunkSize > d.Info.Length {
				d.MinChunkSize = d.Info.Length / 2
			}
		}

		// if Chunk size < Min size set chunk size to min.
		if d.ChunkSize < d.MinChunkSize {
			d.ChunkSize = d.MinChunkSize
		}

		// Change ChunkSize if MaxChunkSize are set and ChunkSize > Max size
		if d.MaxChunkSize > 0 && d.ChunkSize > d.MaxChunkSize {
			d.ChunkSize = d.MaxChunkSize
		}

	} else if d.ChunkSize > d.Info.Length {

		d.ChunkSize = d.Info.Length / 2
	}

	chunksLen = d.Info.Length / d.ChunkSize

	// Set chunk ranges.
	for ; i < chunksLen; i++ {

		startRange = (d.ChunkSize * i) + i
		endRange   = startRange + d.ChunkSize

		if i == 0 {

			startRange = 0

		} else if d.chunks[i - 1].End == 0 {

			break
		}

		if endRange > d.Info.Length || i == (chunksLen - 1) {
			endRange = 0
		}

		d.chunks = append(d.chunks, &Chunk{
			Start:    startRange,
			End:      endRange,
			Progress: d.progress,
		})
	}

	return nil
}

// Start downloading.
func (d *Download) Start() (err error) {

	var (
		okChan  chan bool  = make(chan bool, 1)
		errChan chan error = make(chan error)
	)

	// Create a new temp dir for this download.
	d.temp, err = ioutil.TempDir("", "GotChunks")

	// ...
	if err != nil {
		return err
	}

	// Clean temp.
	defer os.RemoveAll(d.temp)

	// Run progress func.
	go d.progress.Run(d)

	// Partial content not supported,
	// just download the file in one chunk.
	if len(d.chunks) == 0 {

		file, err := os.Create(d.Dest)

		if err != nil {
			return err
		}

		defer file.Close()

		chunk := &Chunk{
			Progress: d.progress,
		}

		return chunk.Download(d.URL, d.client, file)
	}

	// Download chunks.
	go d.dl(&errChan)

	// Merge chunks.
	go d.merge(&errChan, &okChan)

	// Wait for chunks...
	for {

		select {

		case err := <-errChan:

			if err != nil {
				return err
			}

			break

		case <-okChan:

			d.StopProgress = true

			if d.ProgressFunc != nil {
				d.ProgressFunc(d.Info.Length, d.Info.Length, d)
			}

			return nil
		}
	}

	return nil
}

// Get url info.
func (d *Download) GetInfo() (*Info, error) {

	res, err := d.client.Head(d.URL)

	if err != nil {

		return nil, err

	} else if res.StatusCode < 200 || res.StatusCode > 299 {

		// When HEAD reuqest not supported.
		if res.StatusCode == http.StatusMethodNotAllowed {

			return &Info{}, nil
		}

		return nil, fmt.Errorf("Response status code is not ok: %d", res.StatusCode)
	}

	return &Info{
		Length:     res.ContentLength,
		Rangeable:  res.Header.Get("accept-ranges") == "bytes",
		Redirected: d.redirected,
	}, nil
}


// Merge downloaded chunks.
func (d *Download) merge(echan *chan error, done *chan bool) {

	file, err := os.Create(d.Dest)

	if err != nil {
		*echan <-err
		return
	}

	defer file.Close()

	chunksLen := len(d.chunks)

	for {

		for i := range d.chunks {

			d.mu.RLock()
			if d.chunks[i].Downloaded && d.chunks[i].Merged == false && i == d.index {

				chunk, err := os.Open(d.chunks[i].Path)

				if err != nil {
					*echan <-err
					return
				}

				_, err = io.Copy(file, chunk)

				if err != nil {
					*echan <-err
					return
				}

				go chunk.Close()

				// Sync dest file.
				file.Sync()

				d.chunks[i].Merged = true
				d.index++
			}
			d.mu.RUnlock()

			// done, all chunks merged.
			if d.index == chunksLen {
				*done <- true
				return
			}
		}
	}
}


// Download chunks
func (d *Download) dl(echan *chan error) {

	var (

		// Waiting group.
		swg sync.WaitGroup

		// Concurrency limit.
		max chan int = make(chan int, d.Concurrency)
	)

	for i := 0; i < len(d.chunks); i++ {

		max <-1
		swg.Add(1)

		go func(i int) {

			defer swg.Done()

			chunk, err := os.Create(fmt.Sprintf("%s/chunk-%d", d.temp, i))

			if err != nil {
				*echan <-err
				return
			}

			// Close chunk fd.
			defer chunk.Close()

			// Donwload chunk.
			*echan <-d.chunks[i].Download(d.URL, d.client, chunk)

			d.mu.Lock()
			d.chunks[i].Path = chunk.Name()
			d.chunks[i].Downloaded = true
			d.mu.Unlock()

			<-max
		}(i)
	}

	swg.Wait()
}

// New creates a new Download and calls Init.
func New(url string, dest string) (*Download, error) {

	d := &Download{
		URL:  url,
		Dest: dest,
	}

	if err := d.Init(); err != nil {
		return nil, err
	}

	return d, nil
}
