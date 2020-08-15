package got

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type (
	// ProgressFunc to show progress state, called by RunProgress based on interval.
	ProgressFunc func(d *Download)

	// Download holds downloadable file config and infos.
	Download struct {
		Client *http.Client

		Concurrency uint

		URL, Dest string

		Interval, ChunkSize, MinChunkSize, MaxChunkSize uint64

		StopProgress bool

		ctx context.Context

		size, totalSize, lastSize uint64

		chunks []Chunk

		rangeable bool

		startedAt time.Time
	}
)

// GetInfo returns URL file size and rangeable state, and error if any.
func (d Download) GetInfo() (size uint64, rangeable bool, err error) {

	req, err := NewRequest(d.ctx, "HEAD", d.URL)

	if err != nil {
		return 0, false, err
	}

	res, err := d.Client.Do(req)

	if err != nil {
		return 0, false, err
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {

		// On 4xx HEAD request (work around for #3).
		if res.StatusCode != 404 && res.StatusCode >= 400 && res.StatusCode < 500 {
			return 0, false, nil
		}

		return 0, false, fmt.Errorf("Response status code is not ok: %d", res.StatusCode)
	}

	return uint64(res.ContentLength), res.Header.Get("accept-ranges") == "bytes", nil
}

// Init set defaults and split file into chunks and gets Info,
// you should call Init before Start
func (d *Download) Init() error {

	var (
		err                                error
		i, startRange, endRange, chunksLen uint64
	)

	// Set start time.
	d.startedAt = time.Now()

	// Set default client.
	if d.Client == nil {
		d.Client = GetDefaultClient()
	}

	// Set default context.
	if d.ctx == nil {
		d.ctx = context.Background()
	}

	// Get and set URL size and partial content support state.
	if d.totalSize, d.rangeable, err = d.GetInfo(); err != nil {
		return err
	}

	// Partial content not supported 😢!
	if d.rangeable == false || d.totalSize == 0 {
		return nil
	}

	// Set concurrency default to Num CPU * 2.
	if d.Concurrency == 0 {
		d.Concurrency = uint(runtime.NumCPU() * 2)
	}

	// Set default chunk size
	if d.ChunkSize == 0 {

		d.ChunkSize = d.totalSize / uint64(d.Concurrency)

		// if chunk size >= 102400000 bytes set default to (ChunkSize / 2)
		if d.ChunkSize >= 102400000 {
			d.ChunkSize = d.ChunkSize / 2
		}

		// Set default min chunk size to 1m, or file size / 2
		if d.MinChunkSize == 0 {

			d.MinChunkSize = 1000000

			if d.MinChunkSize > d.totalSize {
				d.MinChunkSize = d.totalSize / 2
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

	} else if d.ChunkSize > d.totalSize {

		d.ChunkSize = d.totalSize / 2
	}

	chunksLen = d.totalSize / d.ChunkSize

	d.chunks = make([]Chunk, 0, chunksLen)

	// Set chunk ranges.
	for ; i < chunksLen; i++ {

		startRange = (d.ChunkSize * i) + i
		endRange = startRange + d.ChunkSize

		if i == 0 {
			startRange = 0
		}

		if endRange > d.totalSize || i == (chunksLen-1) {
			endRange = 0
		}

		chunk := ChunkPool.Get().(*Chunk)
		chunk.Reset()
		chunk.Start = startRange
		chunk.End = endRange

		d.chunks = append(d.chunks, *chunk)

		// Break on last chunk if i < chunksLen.
		if endRange == 0 {
			break
		}
	}

	return nil
}

// Start downloads the file chunks, and merges them.
func (d *Download) Start() error {

	var (
		err  error
		temp string
	)

	// Create a new temp dir for this download.
	if temp, err = ioutil.TempDir("", "GotChunks"); err != nil {
		return err
	}

	// Remove temp dir.
	defer os.RemoveAll(temp)

	done := make(chan struct{}, 1)
	errs := make(chan error, 1)
	defer close(done)
	defer close(errs)

	// Partial content not supported, just download the file in one chunk.
	if len(d.chunks) == 0 {

		file, err := os.Create(d.Dest)

		if err != nil {
			return err
		}

		defer file.Close()

		return d.DownloadChunk(d.ctx, Chunk{}, file)
	}

	go func() {
		select {
		case <-d.ctx.Done():
			// System or user interrupted the program
			errs <- ErrDownloadAborted
			return
		case <-done:
			// Everything went ok, no interruptions
			return
		}
	}()

	// Download chunks.
	go d.dl(d.ctx, temp, errs)

	// Merge chunks.
	go func() {
		errs <- d.merge(d.ctx)
	}()

	// Wait for chunks...
	if err := <-errs; err != nil {

		// Remove dest file on error.
		os.Remove(d.Dest)

		return err
	}

	done <- struct{}{}
	return nil
}

// RunProgress runs ProgressFunc based on Interval and updates lastSize
func (d *Download) RunProgress(fn ProgressFunc) {

	// Set default interval.
	if d.Interval == 0 {
		d.Interval = uint64(400 / runtime.NumCPU())
	}

	sleepd := time.Duration(d.Interval) * time.Millisecond

	for {

		if d.StopProgress {
			break
		}

		// Context check.
		select {
		case <-d.ctx.Done():
			return
		default:
		}

		// Run progress func.
		fn(d)

		// Update last size
		atomic.StoreUint64(&d.lastSize, atomic.LoadUint64(&d.size))

		// Interval.
		time.Sleep(sleepd)
	}
}

// Context returns download context.
func (d Download) Context() context.Context {
	return d.ctx
}

// TotalSize returns file total size (0 if unknown).
func (d Download) TotalSize() uint64 {
	return d.totalSize
}

// Size returns downloaded size.
func (d Download) Size() uint64 {
	return atomic.LoadUint64(&d.size)
}

// Speed returns download speed.
func (d Download) Speed() uint64 {
	return (atomic.LoadUint64(&d.size) - atomic.LoadUint64(&d.lastSize)) / d.Interval * 1000
}

// AvgSpeed returns average download speed.
func (d Download) AvgSpeed() uint64 {

	if totalMills := d.TotalCost().Milliseconds(); totalMills > 0 {
		return uint64(atomic.LoadUint64(&d.size) / uint64(totalMills) * 1000)
	}

	return 0
}

// TotalCost returns download duration.
func (d Download) TotalCost() time.Duration {
	return time.Now().Sub(d.startedAt)
}

// Write updates progress size.
func (d *Download) Write(b []byte) (int, error) {
	n := len(b)
	atomic.AddUint64(&d.size, uint64(n))
	return n, nil
}

// IsRangeable returns file server partial content support state.
func (d Download) IsRangeable() bool {
	return d.rangeable
}

// Download chunks
func (d *Download) dl(ctx context.Context, temp string, errc chan error) {

	var (
		// Wait group.
		wg sync.WaitGroup

		// Concurrency limit.
		max = make(chan int, d.Concurrency)
	)

	for i := 0; i < len(d.chunks); i++ {

		max <- 1
		wg.Add(1)

		go func(i int){

			defer wg.Done()

			// Create chunk in temp dir.
			chunk, err := os.Create(filepath.Join(temp, fmt.Sprintf("chunk-%d", i)))

			if err != nil {
				errc <- err
				return
			}

			// Close chunk fd.
			defer chunk.Close()

			// Download chunk.
			if err = d.DownloadChunk(ctx, d.chunks[i], chunk); err != nil {
				errc <- err
				return
			}

			// Set chunk path name.
			d.chunks[i].Path = chunk.Name()

			// Mark this chunk as downloaded.
			close(d.chunks[i].Done)

			<- max
		}(i)
	}

	wg.Wait()
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
			return ctx.Err()
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

		// Put chunk in chunk pool
		ChunkPool.Put(&d.chunks[i])
	}

	return nil
}

// DownloadChunk downloads a file chunk.
func (d *Download) DownloadChunk(ctx context.Context, c Chunk, dest *os.File) error {

	var (
		err error
		req *http.Request
		res *http.Response
	)

	if req, err = NewRequest(ctx, "GET", d.URL); err != nil {
		return err
	}

	contentRange := fmt.Sprintf("bytes=%d-%d", c.Start, c.End)

	if c.End == 0 {
		contentRange = fmt.Sprintf("bytes=%d-", c.Start)
	}

	req.Header.Set("Range", contentRange)

	if res, err = d.Client.Do(req); err != nil {
		return err
	}

	defer res.Body.Close()

	_, err = io.Copy(dest, io.TeeReader(res.Body, d))

	return err
}

// NewDownload returns new *Download with context.
func NewDownload(ctx context.Context, URL, dest string) *Download {
	return &Download{
		ctx:  ctx,
		URL:  URL,
		Dest: dest,
	}
}
