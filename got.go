package got

import (
	"context"
	"net/http"
	"time"
)

// DefaulUserAgent is the default Got user agent to send http requests.
const DefaulUserAgent = "Got/1.0"

// ErrDownloadAborted - When download is aborted by the OS before it is completed, ErrDownloadAborted will be triggered
var ErrDownloadAborted = errors.New("Operation aborted")

// NewRequest returns a new http.Request and error if any.
func NewRequest(ctx context.Context, method, URL string) (req *http.Request, err error) {

	if req, err = http.NewRequestWithContext(ctx, method, URL, nil); err != nil {
		return
	}

	req.Header.Set("User-Agent", DefaulUserAgent)
	return
}

// GetDefaultClient returns Got default http.Client
func GetDefaultClient() *http.Client {

	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
			Proxy:               http.ProxyFromEnvironment,
		},
	}
}

// Got holds got download config.
type Got struct {
	ProgressFunc

	Client *http.Client

	ctx context.Context
}

// Download creates *Download item and runs it.
func (g Got) Download(URL, dest string) error {

	return g.Do(&Download{
		ctx:    g.ctx,
		URL:    URL,
		Dest:   dest,
		Client: g.Client,
	})
}

// Do inits and runs ProgressFunc if set and starts the Download.
func (g Got) Do(dl *Download) error {

	if err := dl.Init(); err != nil {
		return err
	}

	if g.ProgressFunc != nil {

		defer func() {
			dl.StopProgress = true
		}()

		go dl.RunProgress(g.ProgressFunc)
	}

	return dl.Start()
}

// New returns new *Got with default context and client.
func New() *Got {
	return NewWithContext(context.Background())
}

// NewWithContext wants Context and returns *Got with default http client.
func NewWithContext(ctx context.Context) *Got {
	return &Got{
		ctx:    ctx,
		Client: GetDefaultClient(),
	}
}
