package got

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

// Download chunk.
type Chunk struct {

	// Progress to report written bytes.
	*Progress

	// Chunk start pos.
	Start int64

	// Chunk end pos.
	End int64

	// Path name where this chunk downloaded.
	Path string

	// Is this chunk merged to dest file?
	// merged bool
}

// Download a chunk, and report to Progress, it returns error if any!
func (c *Chunk) Download(URL string, client *http.Client, file *os.File) (err error) {

	if file == nil {

		file, err = ioutil.TempFile("", "GotTemp")

		if err != nil {
			return err
		}
	}

	req, err := http.NewRequest("GET", URL, nil)

	if err != nil {
		return err
	}

	contentRange := fmt.Sprintf("bytes=%d-%d", c.Start, c.End)

	if c.End == 0 {
		contentRange = fmt.Sprintf("bytes=%d-", c.Start)
	}

	req.Header.Set("Range", contentRange)
	req.Header.Set("User-Agent", "Got/1.0")

	res, err := client.Do(req)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	_, err = io.Copy(file, io.TeeReader(res.Body, c.Progress))

	if err == nil {
		c.Path = file.Name()
	}

	return err

}
