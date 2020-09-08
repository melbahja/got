package got_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

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
	t.Run("downloadTimeoutContextTest", downloadTimeoutContextTest)
	t.Run("downloadHeadNotSupported", downloadHeadNotSupported)
	t.Run("downloadPartialContentNotSupportedTest", downloadPartialContentNotSupportedTest)
	t.Run("getFilenameTest", getFilenameTest)
	t.Run("coverTests", coverTests)
}

func getInfoTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	dl := got.NewDownload(context.Background(), httpt.URL+"/ok_file", tmpFile)

	info, err := dl.GetInfo()

	if err != nil {
		t.Error(err)
		return
	}

	if info.Rangeable == false {
		t.Error("rangeable should be true")
	}

	if info.Size != uint64(okFileStat.Size()) {
		t.Errorf("Invalid file size, wants %d but got %d", okFileStat.Size(), info.Size)
	}
}

func getFilenameTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	dl := got.NewDownload(context.Background(), httpt.URL+"/file_name", tmpFile)

	info, err := dl.GetInfo()

	if err != nil {

		t.Errorf("Unexpected error: " + err.Error())
	}

	if info.Name != "go.mod" {

		t.Errorf("Expecting file name to be: go.mod but got: " + info.Name)
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

func downloadTimeoutContextTest(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	defer cancel()

	d := got.NewDownload(ctx, httpt.URL+"/ok_file_with_range_delay", tmpFile)
	d.ChunkSize = 2

	if err := d.Init(); err != nil {
		t.Error(err)
	}

	if err := d.Start(); err == nil {
		t.Error("Expecting context deadline")
	}

	d = got.NewDownload(ctx, httpt.URL+"/ok_file_with_range_delay", tmpFile)

	if _, err := d.GetInfo(); err == nil {
		t.Error("Expecting context deadline")
	}

	// just to cover request error.
	g := got.NewWithContext(ctx)
	err := g.Download("invalid://ok_file_with_range_delay", tmpFile)

	if err == nil {
		t.Errorf("Expecting invalid scheme error")
	}
}

func downloadHeadNotSupported(t *testing.T) {

	tmpFile := createTemp()
	defer clean(tmpFile)

	d := &got.Download{
		URL:  httpt.URL + "/found_and_head_not_allowed",
		Dest: "/invalid/path/for_testing_got_start_method",
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

	if err := d.Start(); err == nil {
		t.Error("Expecting invalid path error")
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

func coverTests(t *testing.T) {

	// Just for testing
	destPath := createTemp()
	defer clean(destPath)

	// cover default dest path.
	// cover progress func and methods
	d := &got.Download{
		URL: httpt.URL + "/ok_file_with_range_delay",
	}

	// init
	if err := d.Init(); err != nil {
		t.Error(err)
	}

	if d.Name() != got.DefaultFileName {
		t.Errorf("Expecting name to be: %s but got: %s", got.DefaultFileName, d.Name())
	}

	go d.RunProgress(func(d *got.Download) {
		d.Size()
		d.Speed()
		d.AvgSpeed()
		d.TotalCost()
	})
}

func ExampleDownload() {

	// Just for testing
	destPath := createTemp()
	defer clean(destPath)

	ctx := context.Background()

	dl := got.NewDownload(ctx, testUrl, destPath)

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
