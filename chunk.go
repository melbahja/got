package got

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// Chunk is a part of file.
type Chunk struct {

	// Progress to report written bytes.
	*Progress

	// Chunk start pos.
	Start uint64

	// Chunk end.
	End uint64

	// Path name where this chunk downloaded.
	Path string

	Done chan struct{}
}

// Download a chunk, and report to Progress, it returns error if any!
func (c *Chunk) Download(URL string, client *http.Client, dest *os.File) (err error) {

	req, err := NewRequest("GET", URL)

	if err != nil {
		return err
	}

	contentRange := fmt.Sprintf("bytes=%d-%d", c.Start, c.End)

	if c.End == 0 {
		contentRange = fmt.Sprintf("bytes=%d-", c.Start)
	}

	req.Header.Set("Range", contentRange)

	res, err := client.Do(req)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	_, err = io.Copy(dest, io.TeeReader(res.Body, c.Progress))

	return err
}
