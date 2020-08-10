package got

import (
	"net/url"
	"path/filepath"
)

// GetFilename it returns default file name from a URL.
func GetFilename(URL string) string {

	u, err := url.Parse(URL)

	if err != nil {
		return "got_output"
	}

	basename := filepath.Base(u.Path)

	if basename == "/" {
		return "index"
	}

	return basename
}
