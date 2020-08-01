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

	// URL info.
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

		// Interval in ms
		Interval int

		// Split file chunk by size in bytes.
		ChunkSize int64

		// Set maximum chunk size.
		MaxChunkSize int64

		// Set min chunk size.
		MinChunkSize int64

		// Max connections to open at same time.
		Concurrency int

		// Stop Progress loop.
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
	}
)

// Start downloading.
func (d *Download) Start() (err error) {

	var (
		okChan  chan bool  = make(chan bool, 1)
		errChan chan error = make(chan error)
	)

	d.temp, err = ioutil.TempDir("", "GotChunks")

	if err != nil {
		return err
	}

	// Clean temp.
	defer os.RemoveAll(d.temp)

	// Call progress func.
	go d.progress.Run(d)

	// Download chunks.
	go d.work(&errChan, &okChan)

	// Wait...
	for {

		select {

		case err := <-errChan:
			return err

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

// Check Download and set ranges of chunks, set defaults and start the work,
// you should call Init first then call Start
func (d *Download) Init() (err error) {

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
	d.progress = new(Progress)

	// Set default interval.
	if d.Interval == 0 {
		d.Interval = 0
	}

	// Get URL info.
	d.Info, err = d.GetInfo()

	if err != nil {
		return
	}

	// Partial content not supported 😢!
	if d.Info.Rangeable == false || d.Info.Length == 0 {
		return
	}

	// Set concurrency default to 10.
	if d.Concurrency == 0 {
		d.Concurrency = 10
	}

	// Set default chunk size
	if d.ChunkSize == 0 {

		d.ChunkSize = d.Info.Length / int64(d.Concurrency)

		// if chunk size >= 102400000bytes set default to (ChunkSize / 2)
		if d.ChunkSize >= 102400000 {
			d.ChunkSize = d.ChunkSize / 2
		}

		// Change ChunkSize if MaxChunkSize are set and ChunkSize > Max size
		if d.MaxChunkSize > 0 && d.ChunkSize > d.MaxChunkSize {
			d.ChunkSize = d.MaxChunkSize
		}

		// Set default min chunk size to 1m, or file size / 2
		if d.MinChunkSize == 0 {

			d.MinChunkSize = 1000000

			if d.MinChunkSize > d.Info.Length {
				d.MinChunkSize = d.Info.Length / 2
			}
		}

		// if Chunk size < Min size set chunk size to length / 2
		if d.ChunkSize < d.MinChunkSize {
			d.ChunkSize = d.MinChunkSize
		}
	}

	var i, startRange, endRange, chunksLen int64

	// avoid divide by zero
	if d.ChunkSize > 0 {
		chunksLen = d.Info.Length / d.ChunkSize
	}

	// Set chunks.
	for ; i < chunksLen; i++ {

		startRange = (d.ChunkSize * i) + 1

		if i == 0 {
			startRange = 0
		}

		endRange = startRange + d.ChunkSize

		if i == (chunksLen - 1) {
			endRange = 0
		}

		d.chunks = append(d.chunks, &Chunk{
			Start:    startRange,
			End:      endRange,
			Progress: d.progress,
		})
	}

	return
}

// Download chunks and wait for them to finish,
// in same time merge them into dest path.
func (d *Download) work(echan *chan error, done *chan bool) {

	var (
		// Next chunk index.
		next int = 0

		// Waiting group.
		swg sync.WaitGroup

		// Concurrency limit.
		max chan int = make(chan int, d.Concurrency)

		// Chunk file.
		chunk *os.File
	)

	go func() {

		chunksLen := len(d.chunks)

		file, err := os.Create(d.Dest)

		if err != nil {
			*echan <- err
			return
		}

		defer file.Close()

		// Partial content not supported or file length is unknown,
		// so just download it directly in one chunk!
		if chunksLen == 0 {

			chunk := &Chunk{
				Progress: d.progress,
			}

			if err := chunk.Download(d.URL, d.client, file); err != nil {
				*echan <- err
				return
			}

			*done <- true
			return
		}

		for {

			for i := 0; i < len(d.chunks); i++ {

				if next == i && d.chunks[i].Path != "" {

					chunk, err = os.Open(d.chunks[i].Path)

					if err != nil {

						*echan <- err
						return
					}

					// Copy chunk content to dest file.
					_, err = io.Copy(file, chunk)

					// Close chunk fd.
					chunk.Close()

					if err != nil {

						*echan <- err
						return
					}

					next++
				}
			}

			if next == len(d.chunks) {
				*done <- true
				return
			}

			time.Sleep(6 * time.Millisecond)
		}
	}()

	for i := 0; i < len(d.chunks); i++ {

		max <- 1
		swg.Add(1)

		go func(i int) {

			defer swg.Done()

			dest, err := ioutil.TempFile(d.temp, fmt.Sprintf("chunk-%d", i))

			if err != nil {
				*echan <- err
				return
			}

			err = d.chunks[i].Download(d.URL, d.client, dest)

			if err != nil {
				*echan <- err
			}

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
