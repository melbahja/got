package got

import (
	"testing"
)

var TestUrls = map[string]string{
	"http://example.com/some/path/video.mp4?hash=deadbeef&expires=123456789": "video.mp4",
	"http://example.com/some/path/video.mp4":                                 "video.mp4",
	"http://example.com/index.html":                                          "index.html",
	"http://example.com/":                                                    "got.output",
	"http://example.com/?page=about":                                         "got.output",
	"http://example.com/some/path/":                                          "got.output",
	"http://example.com/some/path/?page=about":                               "got.output",
	"http://example.com/about.php?session=asdf":                              "about.php",
}

var TestHeaderValues = map[string]string{
	`attachment`:                                 "",
	`attachment; filename="filename.jpg"`:        "filename.jpg",
	`attachment; filename="../filename.jpg"`:     "",
	`attachment; filename="../../../etc/passwd"`: "",
	`attachment; name="test"; filename="go.mp4"`: "go.mp4",
}

func TestGetFilename(t *testing.T) {
	for url, expected := range TestUrls {
		if result := GetFilename(url); result != expected {
			t.Errorf("Expected name '%s' from url '%s', but got '%s'", expected, url, result)
		}
	}
}

func TestGetDefaultFileNameFromHeader(t *testing.T) {
	for url, expected := range TestHeaderValues {
		if result := getNameFromHeader(url); result != expected {
			t.Errorf("Expected name '%s' from url '%s', but got '%s'", expected, url, result)
		}
	}
}
