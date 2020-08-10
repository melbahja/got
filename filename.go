package got

import (
	"path/filepath"
	"strings"
)

func GetFilename(url string) string {
	if strings.HasSuffix(url, "/") {
		return "index"
	}
	basename := filepath.Base(url)

	// find start of GET-parameters
	getparam := strings.Index(basename, "?")
	if getparam > 0 {
		return basename[:getparam]
	}

	if getparam == 0 {
		return "index"
	}

	return basename
}
