package got

import (
	"strings"
	"net/url"
	"path/filepath"
)

// DefaultFileName is the fallback name for GetFilename.
var DefaultFileName = "got.output"

// GetFilename it returns default file name from a URL.
func GetFilename(URL string) string {

	if u, err := url.Parse(URL); err == nil && filepath.Ext(u.Path) != "" {

		return filepath.Base(u.Path)
	}

	return DefaultFileName
}


func getNameFromHeader(val string) string {

	if val == "" || strings.Contains(val, "..") || strings.Contains(val, "/") || strings.Contains(val, "\\") {
		return ""
	}

	parts := strings.SplitAfter(val, "filename=")

	if len(parts) >= 2 {
		return strings.Trim(parts[1], `"`)
	}

	return ""
}
