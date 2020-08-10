package got

import (
	"testing"
)

func TestGetFilename(t *testing.T) {
	for url, expected := range TestUrls {
		if result := GetFilename(url); result != expected {
			t.Errorf("expected name '%s' from url '%s', but got '%s'", expected, url, result)
		}
	}
}

var TestUrls = map[string]string{
	"http://example.com/some/path/video.mp4?hash=deadbeef&expires=123456789": "video.mp4",
	"http://example.com/some/path/video.mp4":                                 "video.mp4",
	"http://example.com/":                                                    "index",
	"http://example.com/index.html":                                          "index.html",
	"http://example.com/?page=about":                                         "index",
	"http://example.com/about.php?session=asdf":                              "about.php",
}
