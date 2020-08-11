package got

import (
	"net/url"
	"path/filepath"
)

// GetFilename it returns default file name from a URL.
func GetFilename(URL string) string {

	if u, err := url.Parse(URL); err == nil && filepath.Ext(u.Path) != "" {

		return filepath.Base(u.Path)
	}

	return "got.output"
}
