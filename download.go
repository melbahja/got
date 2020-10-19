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
		Name      string
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

		ctx context.Context

		size, lastSize uint64

		name string

		info *Info

		chunks []Chunk

		startedAt time.Time
	}

	GotHeader struct {
		Key   string
		Value string
	}
)

// GetInfo returns URL info, and error if any.
func (d Download) GetInfo() (*Info, error) {

	req, err := NewRequest(d.ctx, "HEAD", d.URL, d.Header)

	if err != nil {
		return nil, err
	}

	if res, err := d.Client.Do(req); err == nil && res.StatusCode == http.StatusOK {

		return &Info{
			Size:      uint64(res.ContentLength),
			Name:      getNameFromHeader(res.Header.Get("content-disposition")),
			Rangeable: res.Header.Get("accept-ranges") == "bytes",
		}, nil
	}

	return &Info{}, nil
}

// getInfoFromGetRequest download the first byte of the file, to get content length in
// case of HEAD request not supported, and if partial content not supported so this will download the
// file in one chunk. it returns *Info, and error if any.
func (d *Download) getInfoFromGetRequest() (*Info, error) {

	var (
		err error
		req *http.Request
		res *http.Response
	)

	if req, err = NewRequest(d.ctx, "GET", d.URL, d.Header); err != nil {
		return nil, err
	}

	req.Header.Set("Range", "bytes=0-1")

	if res, err = d.Client.Do(req); err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("Response status code is not ok: %d", res.StatusCode)
	}

	dest, err := os.Create(d.Name())

	if err != nil {
		return nil, err
	}

	defer dest.Close()

	if _, err = io.Copy(dest, io.TeeReader(res.Body, d)); err != nil {
		return nil, err
	}

	// Get content length from content-range response header,
	// if content-range exists, that's mean partial content is supported.
	if cr := res.Header.Get("content-range"); cr != "" && res.ContentLength == 2 {
		l := strings.Split(cr, "/")
		if len(l) == 2 {
			if length, err := strconv.ParseUint(l[1], 10, 64); err == nil {
				return &Info{
					Size:      length,
					Name:      getNameFromHeader(res.Header.Get("content-disposition")),
					Rangeable: true,
				}, nil
			}
		}
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

	// Get and set URL size and partial content support state.
	if d.info, err = d.GetInfo(); err != nil {
		return err
	}

	// Maybe partial content not supported 😢!
	if d.info.Rangeable == false || d.info.Size == 0 {

		if d.info, err = d.getInfoFromGetRequest(); err != nil {
			return err
		}

		// Partial content not supported, and the file downladed.
		if d.info.Rangeable == false {
			return nil
		}
	}

	// Set concurrency default.
	if d.Concurrency == 0 {
		d.Concurrency = getDefaultConcurrency()
	}

	// Set default chunk size
	if d.ChunkSize == 0 {
		d.ChunkSize = getDefaultChunkSize(d.info.Size, d.MinChunkSize, d.MaxChunkSize, uint64(d.Concurrency))
	}

	var i, startRange, endRange, chunksLen uint64

	chunksLen = d.info.Size / d.ChunkSize

	d.chunks = make([]Chunk, 0, chunksLen)

	// Set chunk ranges.
	for ; i < chunksLen; i++ {

		startRange = (d.ChunkSize * i) + i
		endRange = startRange + d.ChunkSize

		if i == 0 {
			startRange = 0
		}

		if endRange > d.info.Size || i == (chunksLen-1) {
			endRange = 0
		}

		chunk := ChunkPool.Get().(*Chunk)
		chunk.Path = ""
		chunk.Start = startRange
		chunk.End = endRange
		chunk.Done = make(chan struct{})

		d.chunks = append(d.chunks, *chunk)

		// Break on last chunk if i < chunksLen.
		if endRange == 0 {
			break
		}
	}

	return nil
}

// Start downloads the file chunks, and merges them.
func (d *Download) Start() (err error) {

	// Partial content not supported,
	// just download the file in one chunk.
	if len(d.chunks) == 0 {

		// The file already downloaded at getInfoFromGetRequest
		if d.size > 0 {
			return nil
		}

		file, err := os.Create(d.Name())

		if err != nil {
			return err
		}

		defer file.Close()

		return d.DownloadChunk(Chunk{}, file)
	}

	var (
		temp string
		done = make(chan struct{}, 1)
		errs = make(chan error, 1)
	)

	// Create a new temp dir for this download.
	if temp, err = ioutil.TempDir("", "GotChunks"); err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	// Download chunks.
	go d.dl(temp, errs)

	// Merge chunks.
	go func() {
		errs <- d.merge()
	}()

	select {
	case <-done:
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
func (d Download) Context() context.Context {
	return d.ctx
}

// TotalSize returns file total size (0 if unknown).
func (d Download) TotalSize() uint64 {
	return d.info.Size
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
	return d.info.Rangeable
}

// Download chunks
func (d *Download) dl(temp string, errc chan error) {

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

			// Create chunk in temp dir.
			chunk, err := os.Create(filepath.Join(temp, fmt.Sprintf("chunk-%d", i)))

			if err != nil {
				errc <- err
				return
			}

			// Close chunk fd.
			defer chunk.Close()

			// Download chunk.
			if err = d.DownloadChunk(d.chunks[i], chunk); err != nil {
				errc <- err
				return
			}

			// Set chunk path name.
			d.chunks[i].Path = chunk.Name()

			// Mark this chunk as downloaded.
			close(d.chunks[i].Done)

			<-max
		}(i)
	}

	wg.Wait()
}

// Merge downloaded chunks.
func (d *Download) merge() error {

	file, err := os.Create(d.Name())
	if err != nil {
		return err
	}

	defer file.Close()

	for i := range d.chunks {

		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case <-d.chunks[i].Done:
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

		// Put chunk in chunk pool
		ChunkPool.Put(&d.chunks[i])

		// Sync dest file.
		file.Sync()
	}

	return nil
}

// Name returns the downloaded file path.
func (d *Download) Name() string {

	// Avoid returning different path at runtime.
	if d.name == "" {

		fileName := d.Dest

		// Set default file name.
		if fileName == "" {

			// Content disposition name.
			fileName = d.info.Name

			// if name invalid get name from url.
			if fileName == "" {
				fileName = GetFilename(d.URL)
			}
		}

		d.name = filepath.Join(d.Dir, fileName)
	}

	return d.name
}

// DownloadChunk downloads a file chunk.
func (d *Download) DownloadChunk(c Chunk, dest *os.File) error {

	var (
		err error
		req *http.Request
		res *http.Response
	)

	if req, err = NewRequest(d.ctx, "GET", d.URL, d.Header); err != nil {
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
