package got_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"io/ioutil"

	"github.com/melbahja/got"
)

func TestGot(t *testing.T) {

	// using go.mod file for testing... why not?
	file, err := os.Open("go.mod")

	if err != nil {
		t.Error(err)
	}

	stat, err := file.Stat()

	if err != nil {
		t.Error(err)
	}

	httpt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		switch r.URL.String() {

		case "/file1":
			// http.ServeContent(w, r, "go.mod", stat.ModTime(), file)
			http.ServeFile(w, r, "go.mod")
			return

		case "/file2":

			if r.Method == "HEAD" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			fmt.Fprint(w, "helloworld")
			return

		case "/file4":

			w.WriteHeader(http.StatusMethodNotAllowed)
			return

		case "/file5":

			if strings.Contains(r.Header.Get("range"), "3-") {

				time.Sleep(2 * time.Second)
			}

			http.ServeFile(w, r, "go.mod")
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	defer httpt.Close()

	// Init test.
	t.Run("init", func(t *testing.T) {

		initTest(t, httpt.URL+"/file1")
	})

	// Get info teshttpt.
	t.Run("info", func(t *testing.T) {

		expect := got.Info{
			Length:     stat.Size(),
			Rangeable:  true,
			Redirected: false,
		}

		getInfoTest(t, httpt.URL+"/file1", expect)
	})

	// download tests.
	t.Run("download", func(t *testing.T) {

		t.Run("downloadChunksTest", func(t *testing.T) {

			// test info size and chunks.
			downloadChunksTest(t, httpt.URL+"/file1", stat.Size())
		})

		t.Run("downloadTest", func(t *testing.T) {

			// test init and start.
			downloadTest(t, httpt.URL+"/file1", stat.Size())
		})

		t.Run("downloadNotFoundTest", func(t *testing.T) {

			// test not found error.
			downloadNotFoundTest(t, httpt.URL+"/file3")
		})

		t.Run("downloadHeadNotSupported", func(t *testing.T) {

			// test not found error.
			downloadHeadNotSupported(t, httpt.URL+"/file4")
		})

		t.Run("downloadPartialContentNotSupportedTest", func(t *testing.T) {

			// test when partial content not supprted.
			downloadPartialContentNotSupportedTest(t, httpt.URL+"/file2")
		})

		t.Run("fileContentTest", func(t *testing.T) {

			// test when partial content not supprted.
			fileContentTest(t, httpt.URL+"/file5")
		})
	})
}

func getInfoTest(t *testing.T, url string, expect got.Info) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d, err := got.New(url, tmpFile)

	if err != nil {
		t.Error(err)
		return
	}

	info, err := d.GetInfo()

	if err != nil {
		t.Error(err)
		return
	}

	if expect != *info {

		t.Error("invalid info")
	}
}

func initTest(t *testing.T, url string) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d := got.Download{
		URL:  url,
		Dest: tmpFile,
	}

	if err := d.Init(); err != nil {
		t.Error(err)
	}
}

func downloadChunksTest(t *testing.T, url string, size int64) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d, err := got.New(url, tmpFile)

	if err != nil {

		t.Error(err)
		return
	}

	info, err := d.GetInfo()

	if err != nil {
		t.Error(err)
		return
	}

	if size != info.Length {
		t.Error("length and file size doesn't match")
	}
}

func downloadTest(t *testing.T, url string, size int64) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d := &got.Download{
		URL:         url,
		Dest:        tmpFile,
		Concurrency: 2,
	}

	if err := d.Init(); err != nil {

		t.Error(err)
		return
	}

	if err := d.Start(); err != nil {
		t.Error(err)
	}

	stat, err := os.Stat(tmpFile)

	if err != nil {
		t.Error(err)
	}

	if size != stat.Size() {
		t.Errorf("Expecting size: %d, but got %d", size, stat.Size())
	}
}

func downloadNotFoundTest(t *testing.T, url string) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	_, err := got.New(url, tmpFile)

	if err == nil {
		t.Error("It should have an error")
		return
	}
}

func downloadHeadNotSupported(t *testing.T, url string) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d := &got.Download{
		URL:  url,
		Dest: tmpFile,
	}

	// init
	if err := d.Init(); err != nil {
		t.Error(err)
		return
	}

	info, err := d.GetInfo()

	if err != nil {

		t.Error(err)
		return
	}

	if *info != (got.Info{}) {

		t.Error("It should have a empty Info{}")
	}
}

func downloadPartialContentNotSupportedTest(t *testing.T, url string) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d := &got.Download{
		URL:  url,
		Dest: tmpFile,
	}

	if err := d.Init(); err != nil {
		t.Error(err)
		return
	}

	if d.Info.Length != 0 {
		t.Errorf("Expect length to be 0, but got %d", d.Info.Length)
	}

	if err := d.Start(); err != nil {
		t.Error(err)
	}

	stat, err := os.Stat(tmpFile)

	if err != nil {
		t.Error(err)
	}

	if stat.Size() != 10 {
		t.Errorf("Invalid size: %d", stat.Size())
	}
}

func fileContentTest(t *testing.T, url string) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d := &got.Download{
		URL:       url,
		Dest:      tmpFile,
		ChunkSize: 10,
	}

	if err := d.Init(); err != nil {
		t.Error(err)
		return
	}

	if err := d.Start(); err != nil {
		t.Error(err)
		return
	}

	mod, err := ioutil.ReadFile("go.mod")

	if err != nil {
		t.Error(err)
		return
	}

	dlFile, err := ioutil.ReadFile(tmpFile)

	if err != nil {
		t.Error(err)
		return
	}

	if string(mod) != string(dlFile) {

		fmt.Println("a", string(mod))
		fmt.Println("b", string(dlFile))
		t.Error("Corrupted file")
	}
}

func createTemp() string {

	tmp, err := ioutil.TempFile("", "")

	if err != nil {
		panic(err)
	}

	defer tmp.Close()

	return tmp.Name()
}

func clean(tmpFile string) {

	os.Remove(tmpFile)
}
