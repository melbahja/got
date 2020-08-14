package got

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"
)

// ErrInterupted when user or system interrupts the program
var ErrInterupted = errors.New("Interupted")

// GetChunkSize gets size of individual chanck
func (d *Download) GetChunkSize() uint64 {
	return d.chunkSize
}

// Start downloading.
func (d *Download) Start() (err error) {
	stop := make(chan struct{})
	errs := make(chan error)
	defer close(stop)
	defer close(errs)
	// Create a new temp dir for this download.
	temp, err := ioutil.TempDir("", "GotChunks")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	// Run progress func.
	go d.Progress.Run(d.ctx, d)

	// Partial content not supported,
	// just download the file in one chunk.
	if len(d.chunks) == 0 {

		file, err := os.Create(d.Dest)

		if err != nil {
			return err
		}

		defer file.Close()

		chunk := &Chunk{
			Progress: d.Progress,
		}

		return chunk.Download(d.URL, d.client, file)
	}

	go func() {
		select {
		case <-d.ctx.Done():
			errs <- ErrInterupted
			// System or user interrupted the program
			return
		case <-stop:
			// Everything went ok, no interruptions
			return
		}
	}()

	// Download chunks.
	go func() {
		if err := d.dl(d.ctx, temp); err != nil {
			errs <- err
		}
	}()

	// Merge chunks.
	go func() { errs <- d.merge(d.ctx) }()

	// Wait for chunks...
	if err := <-errs; err != nil {
		// In case of an error, destination file should be removed
		_ = os.Remove(d.Dest)
		return err
	}

	// Update progress output after chunks finished.
	if d.Progress.ProgressFunc != nil {
		d.Progress.ProgressFunc(d.Progress, d)
	}

	stop <- struct{}{}
	return nil

}

// GetInfo gets Info, it returns error if status code > 500 or 404.
func (d *Download) GetInfo() (Info, error) {

	req, err := NewRequest(http.MethodHead, d.URL)

	if err != nil {
		return Info{}, err
	}

	res, err := d.client.Do(req)

	if err != nil {
		return Info{}, err
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {

		// On 4xx HEAD request (work around for #3).
		if res.StatusCode != 404 && res.StatusCode >= 400 && res.StatusCode < 500 {
			return Info{}, nil
		}

		return Info{}, fmt.Errorf("Response status code is not ok: %d", res.StatusCode)
	}

	return Info{
		Length:     uint64(res.ContentLength),
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
			_ = file.Sync()
		case <-ctx.Done():
			return ErrInterupted
		}
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
		current := i

		eg.Go(func() error {

			defer func() {
				<-max
			}()

			// Create chunk in temp dir.
			chunk, err := os.Create(filepath.Join(temp, fmt.Sprintf("chunk-%d", current)))

			if err != nil {
				return err
			}

			// Close chunk fd.
			defer chunk.Close()

			// Download chunk.
			err = d.chunks[current].Download(d.URL, d.client, chunk)
			if err != nil {
				return err
			}

			d.chunks[current].Path = chunk.Name()
			close(d.chunks[current].Done)
			return nil
		})
	}

	return eg.Wait()
}

func (d *Download) setChunkSize() {
	d.chunkSize = d.Length / uint64(d.Concurrency)

	// if chunk size >= 102400000 bytes set default to (ChunkSize / 2)
	if d.chunkSize >= 102400000 {
		d.chunkSize = d.chunkSize / 2
	}

	// Set default min chunk size to 1m, or file size / 2
	if d.MinChunkSize == 0 {

		d.MinChunkSize = 1000000

		if d.MinChunkSize > d.Length {
			d.MinChunkSize = d.Length / 2
		}
	}

	// if Chunk size < Min size set chunk size to min.
	if d.chunkSize < d.MinChunkSize {
		d.chunkSize = d.MinChunkSize
	}

	// Change ChunkSize if MaxChunkSize are set and ChunkSize > Max size
	if d.MaxChunkSize > 0 && d.chunkSize > d.MaxChunkSize {
		d.chunkSize = d.MaxChunkSize
	}

}

// Init set defaults and split file into chunks and gets Info,
// you should call Init before Start
func (d *Download) init() error {

	var (
		err                  error
		startRange, endRange uint64
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

	// Set default interval.
	if d.Interval == 0 {
		d.Interval = 20
	}

	// Get URL info.
	if d.Info, err = d.GetInfo(); err != nil {
		return err
	}

	// Set default progress.
	if d.Progress == nil {

		d.Progress = &Progress{
			startedAt: time.Now(),
			Interval:  d.Interval,
			TotalSize: d.Length,
		}
	}

	// Partial content not supported 😢!
	if d.Rangeable == false || d.Length == 0 {
		return nil
	}

	// Set concurrency default to 10.
	if d.Concurrency == 0 {
		d.Concurrency = 10
	}

	d.setChunkSize()

	chunksLen := d.Length / d.chunkSize

	d.chunks = make([]Chunk, 0, chunksLen)

	// Set chunk ranges.
	for i := uint64(0); i < chunksLen; i++ {

		startRange = (d.chunkSize * i) + i
		endRange = startRange + d.chunkSize

		if i == 0 {

			startRange = 0

		} else if d.chunks[i-1].End == 0 {

			break
		}

		if endRange > d.Length || i == (chunksLen-1) {
			endRange = 0
		}

		d.chunks = append(d.chunks, Chunk{
			Start:    startRange,
			End:      endRange,
			Progress: d.Progress,
			Done:     make(chan struct{}),
		})
	}

	return nil
}

// New creates a new Download and calls Init.
func New(ctx context.Context, cfg Config) (*Download, error) {

	if ctx == nil {
		return nil, errors.New("Context is not provided")
	}

	d := &Download{
		Config: cfg,
		ctx:    ctx,
	}

	if err := d.init(); err != nil {
		return nil, err
	}

	return d, nil
}
