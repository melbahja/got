package got

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type (

	// Info holds downloadable file info.
	Info struct {
		Size      uint64
		Rangeable bool
	}

	// ProgressFunc to show progress state, called by RunProgress based on interval.
	ProgressFunc func(d *Download)

	// Download holds downloadable file config and infos.
	Download struct {
		Client *http.Client

		Concurrency uint

		URL, Dir, Dest string

		Interval, ChunkSize, MinChunkSize, MaxChunkSize uint64

		Header []GotHeader

		StopProgress bool

		path string

		unsafeName string

		ctx context.Context

		size, lastSize uint64

		info *Info

		chunks []*Chunk

		startedAt time.Time
	}

	GotHeader struct {
		Key   string
		Value string
	}
)

// Try downloading the first byte of the file using a range request.
// If the server supports range requests, then we'll extract the length info from content-range,
// Otherwise this just downloads the whole file in one go
func (d *Download) GetInfoOrDownload() (*Info, error) {

	var (
		err  error
		dest *os.File
		req  *http.Request
		res  *http.Response
	)

	if req, err = NewRequest(d.ctx, "GET", d.URL, append(d.Header, GotHeader{"Range", "bytes=0-0"})); err != nil {
		return &Info{}, err
	}

	if res, err = d.Client.Do(req); err != nil {
		return &Info{}, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return &Info{}, fmt.Errorf("Response status code is not ok: %d", res.StatusCode)
	}

	// Set content disposition non trusted name
	d.unsafeName = res.Header.Get("content-disposition")

	if dest, err = os.Create(d.Path()); err != nil {
		return &Info{}, err
	}
	defer dest.Close()

	if _, err = io.Copy(dest, io.TeeReader(res.Body, d)); err != nil {
		return &Info{}, err
	}

	// Get content length from content-range response header,
	// if content-range exists, that means partial content is supported.
	if cr := res.Header.Get("content-range"); cr != "" && res.ContentLength == 1 {
		l := strings.Split(cr, "/")
		if len(l) == 2 {
			if length, err := strconv.ParseUint(l[1], 10, 64); err == nil {

				return &Info{
					Size:      length,
					Rangeable: true,
				}, nil
			}
		}
		// Make sure the caller knows about the problem and we don't just silently fail
		return &Info{}, fmt.Errorf("Response includes content-range header which is invalid: %s", cr)
	}

	return &Info{}, nil
}

// Init set defaults and split file into chunks and gets Info,
// you should call Init before Start
func (d *Download) Init() (err error) {

	// Set start time.
	d.startedAt = time.Now()

	// Set default client.
	if d.Client == nil {
		d.Client = DefaultClient
	}

	// Set default context.
	if d.ctx == nil {
		d.ctx = context.Background()
	}

	// Get URL info and partial content support state
	if d.info, err = d.GetInfoOrDownload(); err != nil {
		return err
	}

	// Partial content not supported, and the file downladed.
	if d.info.Rangeable == false {
		return nil
	}

	// Set concurrency default.
	if d.Concurrency == 0 {
		d.Concurrency = getDefaultConcurrency()
	}

	// Set default chunk size
	if d.ChunkSize == 0 {
		d.ChunkSize = getDefaultChunkSize(d.info.Size, d.MinChunkSize, d.MaxChunkSize, uint64(d.Concurrency))
	}

	chunksLen := d.info.Size / d.ChunkSize
	d.chunks = make([]*Chunk, 0, chunksLen)

	// Set chunk ranges.
	for i := uint64(0); i < chunksLen; i++ {

		chunk := new(Chunk)
		d.chunks = append(d.chunks, chunk)

		chunk.Start = (d.ChunkSize * i) + i
		chunk.End = chunk.Start + d.ChunkSize
		if chunk.End >= d.info.Size || i == chunksLen-1 {
			chunk.End = d.info.Size - 1
			// Break on last chunk if i < chunksLen
			break
		}
	}

	return nil
}

// Start downloads the file chunks, and merges them.
// Must be called only after init
func (d *Download) Start() (err error) {
	// If the file was already downloaded during GetInfoOrDownload, then there will be no chunks
	if d.info.Rangeable == false {
		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		default:
			return nil
		}
	}

	// Otherwise there are always at least 2 chunks

	file, err := os.Create(d.Path())
	if err != nil {
		return err
	}
	defer file.Close()

	// Allocate the file completely so that we can write concurrently
	file.Truncate(int64(d.TotalSize()))

	// Download chunks.
	errs := make(chan error, 1)
	go d.dl(file, errs)

	select {
	case err = <-errs:
	case <-d.ctx.Done():
		err = d.ctx.Err()
	}

	return
}

// RunProgress runs ProgressFunc based on Interval and updates lastSize.
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
func (d *Download) Context() context.Context {
	return d.ctx
}

// TotalSize returns file total size (0 if unknown).
func (d *Download) TotalSize() uint64 {
	return d.info.Size
}

// Size returns downloaded size.
func (d *Download) Size() uint64 {
	return atomic.LoadUint64(&d.size)
}

// Speed returns download speed.
func (d *Download) Speed() uint64 {
	return (atomic.LoadUint64(&d.size) - atomic.LoadUint64(&d.lastSize)) / d.Interval * 1000
}

// AvgSpeed returns average download speed.
func (d *Download) AvgSpeed() uint64 {

	if totalMills := d.TotalCost().Milliseconds(); totalMills > 0 {
		return uint64(atomic.LoadUint64(&d.size) / uint64(totalMills) * 1000)
	}

	return 0
}

// TotalCost returns download duration.
func (d *Download) TotalCost() time.Duration {
	return time.Now().Sub(d.startedAt)
}

// Write updates progress size.
func (d *Download) Write(b []byte) (int, error) {
	n := len(b)
	atomic.AddUint64(&d.size, uint64(n))
	return n, nil
}

// IsRangeable returns file server partial content support state.
func (d *Download) IsRangeable() bool {
	return d.info.Rangeable
}

// Download chunks
func (d *Download) dl(dest io.WriterAt, errC chan error) {

	var (
		// Wait group.
		wg sync.WaitGroup

		// Concurrency limit.
		max = make(chan int, d.Concurrency)
	)

	for i := 0; i < len(d.chunks); i++ {

		max <- 1
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			// Concurrently download and write chunk
			if err := d.DownloadChunk(d.chunks[i], &OffsetWriter{dest, int64(d.chunks[i].Start)}); err != nil {
				errC <- err
				return
			}

			<-max
		}(i)
	}

	wg.Wait()
	errC <- nil
}

// Return constant path which will not change once the download starts
func (d *Download) Path() string {

	// Set the default path
	if d.path == "" {

		d.path = GetFilename(d.URL) // default case
		if d.Dest != "" {
			d.path = d.Dest
		} else if d.unsafeName != "" {
			if path := getNameFromHeader(d.unsafeName); path != "" {
				d.path = path
			}
		}
		d.path = filepath.Join(d.Dir, d.path)
	}

	return d.path
}

// DownloadChunk downloads a file chunk.
func (d *Download) DownloadChunk(c *Chunk, dest io.Writer) error {

	var (
		err error
		req *http.Request
		res *http.Response
	)

	if req, err = NewRequest(d.ctx, "GET", d.URL, d.Header); err != nil {
		return err
	}

	contentRange := fmt.Sprintf("bytes=%d-%d", c.Start, c.End)
	req.Header.Set("Range", contentRange)

	if res, err = d.Client.Do(req); err != nil {
		return err
	}

	// Verify the length
	if res.ContentLength != int64(c.End-c.Start+1) {
		return fmt.Errorf(
			"Range request returned invalid Content-Length: %d however the range was: %s",
			res.ContentLength, contentRange,
		)
	}

	defer res.Body.Close()

	_, err = io.CopyN(dest, io.TeeReader(res.Body, d), res.ContentLength)

	return err
}

// NewDownload returns new *Download with context.
func NewDownload(ctx context.Context, URL, dest string) *Download {
	return &Download{
		ctx:    ctx,
		URL:    URL,
		Dest:   dest,
		Client: DefaultClient,
	}
}

func getDefaultConcurrency() uint {

	c := uint(runtime.NumCPU() * 3)

	// Set default max concurrency to 20.
	if c > 20 {
		c = 20
	}

	// Set default min concurrency to 4.
	if c <= 2 {
		c = 4
	}

	return c
}

func getDefaultChunkSize(totalSize, min, max, concurrency uint64) uint64 {

	cs := totalSize / concurrency

	// if chunk size >= 102400000 bytes set default to (ChunkSize / 2)
	if cs >= 102400000 {
		cs = cs / 2
	}

	// Set default min chunk size to 2m, or file size / 2
	if min == 0 {

		min = 2097152

		if min >= totalSize {
			min = totalSize / 2
		}
	}

	// if Chunk size < Min size set chunk size to min.
	if cs < min {
		cs = min
	}

	// Change ChunkSize if MaxChunkSize are set and ChunkSize > Max size
	if max > 0 && cs > max {
		cs = max
	}

	// When chunk size > total file size, divide chunk / 2
	if cs >= totalSize {
		cs = totalSize / 2
	}

	return cs
}
