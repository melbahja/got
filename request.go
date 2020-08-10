package got

import (
	"net/http"
)

// DefaulUserAgent is the default Got user agent to send http request.
const DefaulUserAgent = "Got/1.0"

// NewRequest returns a new http.Request and error if any.
func NewRequest(method, URL string) (*http.Request, error) {

	req, err := http.NewRequest(method, URL, nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", DefaulUserAgent)

	return req, nil
}
