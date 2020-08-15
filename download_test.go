package got_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/melbahja/got"
)

var (
	httpt      = NewHttptestServer()
	okFileStat os.FileInfo
)

func init() {

	var err error

	okFileStat, err = os.Stat("go.mod")

	if err != nil {
		panic(err)
	}
}

func TestGetInfoAndInit(t *testing.T) {

	t.Run("getInfoTest", getInfoTest)
	t.Run("okInitTest", okInitTest)
	t.Run("errInitTest", errInitTest)
}

func TestDownloading(t *testing.T) {

	t.Run("downloadOkFileTest", downloadOkFileTest)
	t.Run("downloadNotFoundTest", downloadNotFoundTest)
	t.Run("downloadOkFileContentTest", downloadOkFileContentTest)
	t.Run("downloadHeadNotSupported", downloadHeadNotSupported)
	t.Run("downloadPartialContentNotSupportedTest", downloadPartialContentNotSupportedTest)
}

func getInfoTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	dl := got.NewDownload(context.Background(), httpt.URL+"/ok_file", tmpFile)

	dl.Client = got.GetDefaultClient()

	size, rangeable, err := dl.GetInfo()

	if err != nil {
		t.Error(err)
		return
	}

	if rangeable == false {
		t.Error("rangeable should be true")
	}

	if size != uint64(okFileStat.Size()) {
		t.Errorf("Invalid file size, wants %d but got %d", okFileStat.Size(), size)
	}
}

func okInitTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	dl := &got.Download{
		URL:  httpt.URL + "/ok_file",
		Dest: tmpFile,
	}

	if err := dl.Init(); err != nil {
		t.Error(err)
	}
}

func errInitTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	dl := &got.Download{
		URL:  httpt.URL + "/not_found",
		Dest: tmpFile,
	}

	if err := dl.Init(); err == nil {
		t.Error("Expecting error but got nil")
	}
}

func downloadOkFileTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	dl := &got.Download{
		URL:  httpt.URL + "/ok_file",
		Dest: tmpFile,
	}

	// Init
	if err := dl.Init(); err != nil {
		t.Error(err)
		return
	}

	// Check size
	if dl.TotalSize() != uint64(okFileStat.Size()) {
		t.Errorf("Invalid file size, wants %d but got %d", okFileStat.Size(), dl.TotalSize())
	}

	// Start download
	if err := dl.Start(); err != nil {
		t.Error(err)
	}

	stat, err := os.Stat(tmpFile)

	if err != nil {
		t.Error(err)
	}

	if okFileStat.Size() != stat.Size() {
		t.Errorf("Expecting size: %d, but got %d", okFileStat.Size(), stat.Size())
	}
}

func downloadNotFoundTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	dl := &got.Download{
		URL:  httpt.URL + "/not_found",
		Dest: tmpFile,
	}

	err := dl.Init()

	if err == nil {
		t.Error("It should have an error")
		return
	}
}

func downloadOkFileContentTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d := &got.Download{
		URL:       httpt.URL + "/ok_file_with_range_delay",
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

func downloadHeadNotSupported(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d := &got.Download{
		URL:  httpt.URL + "/found_and_head_not_allowed",
		Dest: tmpFile,
	}

	// init
	if err := d.Init(); err != nil {
		t.Error(err)
		return
	}

	if d.TotalSize() != 0 {

		t.Error("Size should be 0")
	}

	if d.IsRangeable() != false {

		t.Error("rangeable should be false")
	}
}

func downloadPartialContentNotSupportedTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d := &got.Download{
		URL:  httpt.URL + "/found_and_head_not_allowed",
		Dest: tmpFile,
	}

	if err := d.Init(); err != nil {
		t.Error(err)
		return
	}

	if d.TotalSize() != 0 {
		t.Errorf("Expect length to be 0, but got %d", d.TotalSize())
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

func ExampleDownload() {

	defer os.Remove("/tmp/got_dl_file_test")

	ctx := context.Background()

	dl := got.NewDownload(ctx, testUrl, "/tmp/got_dl_file_test")

	// Init
	if err := dl.Init(); err != nil {
		fmt.Println(err)
	}

	// Start download
	if err := dl.Start(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("Done")

	// Output: Done
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
