package got

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"
)

type (

	// Info of the download url.
	Info struct {

		// File content length.
		Length int64

		// Supports partial content?
		Rangeable bool

		// URL Redirected.
		Redirected bool
	}

	// Download represents the download URL.
	Download struct {

		// Download file info.
		*Info

		// Progress func.
		ProgressFunc

		// URL to download.
		URL string

		// File destination.
		Dest string

		// Progress interval in ms.
		Interval int

		// Split file into chunks by ChunkSize in bytes.
		ChunkSize int64

		// Set maximum chunk size.
		MaxChunkSize int64

		// Set min chunk size.
		MinChunkSize int64

		// Max chunks to download at same time.
		Concurrency int

		// Download file chunks.
		chunks []*Chunk

		// Http client.
		client *http.Client

		// Is the URL redirected to a different location.
		redirected bool

		// Progress...
		progress *Progress
	}
)

// Init set defaults and split file into chunks and gets Info,
// you should call Init before Start
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
			Proxy:               http.ProxyFromEnvironment,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			d.redirected = true
			return nil
		},
	}

	// Init progress.
	d.progress = &Progress{}

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
		endRange = startRange + d.ChunkSize

		if i == 0 {

			startRange = 0

		} else if d.chunks[i-1].End == 0 {

			break
		}

		if endRange > d.Info.Length || i == (chunksLen-1) {
			endRange = 0
		}

		d.chunks = append(d.chunks, &Chunk{
			Start:    startRange,
			End:      endRange,
			Progress: d.progress,
			Done:     make(chan struct{}),
		})
	}

	return nil
}

// Start downloading.
func (d *Download) Start() (err error) {
	// Create a new temp dir for this download.
	temp, err := ioutil.TempDir("", "GotChunks")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	// Run progress func.
	go d.progress.Run(ctx, d)

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

	eg, ctx := errgroup.WithContext(ctx)

	// Download chunks.
	eg.Go(func() error {
		return d.dl(ctx, temp)
	})

	// Merge chunks.
	eg.Go(func() error {
		return d.merge(ctx)
	})

	// Wait for chunks...
	if err := eg.Wait(); err != nil {
		return err
	}

	if d.ProgressFunc != nil {
		d.ProgressFunc(d.progress.Size, d.Info.Length, d)
	}
	return nil
}

// GetInfo gets Info, it returns error if status code > 500 or 404.
func (d *Download) GetInfo() (*Info, error) {

	req, err := NewRequest("HEAD", d.URL)

	if err != nil {
		return nil, err
	}

	res, err := d.client.Do(req)

	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 400 {

		// On 4xx HEAD request (work around for #3).
		if res.StatusCode != 404 && res.StatusCode >= 400 && res.StatusCode < 500 {
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
func (d *Download) merge(ctx context.Context) error {
	file, err := os.Create(d.Dest)
	if err != nil {
		return err
	}

	defer file.Close()

	for i := range d.chunks {
		select {
		case <-d.chunks[i].Done:
		case <-ctx.Done():
			return nil
		}

		chunk, err := os.Open(d.chunks[i].Path)
		if err != nil {
			return err
		}

		if _, err = io.Copy(file, chunk); err != nil {
			return err
		}

		// Non-blocking chunk close.
		go chunk.Close()

		// Sync dest file.
		file.Sync()
	}
	return nil
}

// Download chunks
func (d *Download) dl(ctx context.Context, temp string) error {
	eg, ctx := errgroup.WithContext(ctx)
	// Concurrency limit.
	max := make(chan int, d.Concurrency)

	for i := 0; i < len(d.chunks); i++ {
		max <- 1
		i := i
		eg.Go(func() error {
			defer func() {
				<-max
			}()
			chunk, err := os.Create(filepath.Join(temp, fmt.Sprintf("chunk-%d", i)))

			if err != nil {
				return err
			}

			// Close chunk fd.
			defer chunk.Close()

			// Download chunk.
			err = d.chunks[i].Download(d.URL, d.client, chunk)
			if err != nil {
				return err
			}

			d.chunks[i].Path = chunk.Name()
			close(d.chunks[i].Done)
			return nil
		})
	}

	return eg.Wait()
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
