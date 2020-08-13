package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/apoorvam/goterminal"
	"github.com/dustin/go-humanize"
	"github.com/melbahja/got"
)

var (
	url         string
	version     string
	batchFile   = flag.String("bf", "", "Batch download file from one list in file.")
	saveDir     = flag.String("save", "", "Downloaded file save destination.")
	dest        = flag.String("out", "", "Downloaded file destination.")
	chunkSize   = flag.Uint64("size", 0, "Maximum chunk size in bytes.")
	concurrency = flag.Uint("concurrency", 10, "Maximum chunks to download at the same time.")
)

func init() {

	flag.Usage = func() {
		fmt.Println("Got - the fast http downloader.")
		fmt.Println("\nUsage:\n  got --out path/file.zip http://example.com/file.zip")
		fmt.Printf("\nVersion:\n  %s\n", version)
		fmt.Println("\nAuthor:\n  Mohamed Elbahja <bm9qdW5r@gmail.com>")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
	}
}

func main() {

	flag.Parse()

	if url = flag.Arg(0); url == "" && *batchFile == "" {
		log.Fatal("Empty download url.")
	}

	urls := []string{}
	if url != "" && !(url[:7] == "http://" || url[:8] == "https://") {
		url = strings.TrimSpace(url)
		url = "https://" + url
	}

	if url != "" {
		url = strings.TrimSpace(url)
		urls = append(urls, url)
	}

	if *batchFile != "" {
		fileData, err := ioutil.ReadFile(*batchFile)
		if err == nil {
			for _, line := range strings.Split(string(fileData), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !(line[:7] == "http://" || line[:8] == "https://") {
					line = "https://" + line
				}
				if line != "" {
					urls = append(urls, line)
				}
			}
		}
	}

	if len(urls) < 1 {
		log.Fatal("Empty download url.")
	}

	t := len(urls)
	if t < 1 {
		t = 1
	}

	var wg sync.WaitGroup
	wg.Add(t)

	for _, curl := range urls {
		go func(curl string) {
			defer wg.Done()
			if *dest == "" {
				*dest = got.GetFilename(curl)
			}

			if *saveDir != "" {
				*dest = filepath.Join(*saveDir, *dest)
				pDir := filepath.Dir(*dest)
				if pDir != "." && pDir != "./" {
					if _, err := os.Stat(pDir); os.IsNotExist(err) {
						os.MkdirAll(pDir, os.ModePerm)
					}
				}
			}
			initDownloader(curl, *dest, *chunkSize, *concurrency)
		}(curl)
	}
	wg.Wait()
	os.Exit(0)
}

func initDownloader(url, dest string, chunkSize uint64, concurrency uint) {
	fmt.Println("download from ", url, " to ", dest, " starting...")

	d := got.Download{
		URL:         url,
		Dest:        dest,
		ChunkSize:   chunkSize,
		Interval:    100,
		Concurrency: concurrency,
	}

	if err := d.Init(); err != nil {
		log.Fatal(err)
	}

	// Goterm writer.
	writer := goterminal.New(os.Stdout)

	// Set progress func to update cli output.
	d.Progress.ProgressFunc = func(p *got.Progress, d *got.Download) {

		writer.Clear()

		fmt.Fprintf(
			writer,
			"Downloading %s: (%s/%s) | Time: %s | Avg: %s/s | Speed: %s/s | Chunk: %s | Concurrency: %d\n",
			d.URL,
			humanize.Bytes(p.Size),
			humanize.Bytes(p.TotalSize),
			p.TotalCost().Round(time.Second),
			humanize.Bytes(p.AvgSpeed()),
			humanize.Bytes(p.Speed()),
			humanize.Bytes(d.ChunkSize),
			d.Concurrency,
		)

		writer.Print()
	}

	if err := d.Start(); err != nil {
		log.Fatal(err)
	}

	writer.Reset()
	fmt.Println("download from ", url, " to ", dest, " Done!")
}
