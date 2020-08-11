package main

import (
	"flag"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/melbahja/got"
	"log"
	"time"
)

var (
	url         string
	version     string
	dest        = flag.String("out", "", "Downloaded file destination.")
	chunkSize   = flag.Int("size", 0, "Maximum chunk size in bytes.")
	concurrency = flag.Int("concurrency", 10, "Maximum chunks to download at the same time.")
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

	if url = flag.Arg(0); url == "" {
		log.Fatal("Empty download url.")
	}

	if !(url[:7] == "http://" || url[:8] == "https://") {
		url = "https://" + url
	}

	if *dest == "" {
		*dest = got.GetFilename(url)
	}

	d := got.Download{
		URL:  url,
		Dest: *dest,
		ChunkSize:   int64(*chunkSize),
		Interval:    100,
		Concurrency: *concurrency,
	}

	if err := d.Init(); err != nil {
		log.Fatal(err)
	}

	// Set progress func to update cli output.
	d.Progress.ProgressFunc = func(p *got.Progress, d *got.Download) {

		fmt.Printf(
			"\r\r\bTotal: %s | Chunk: %s | Concurrency: %d | Received: %s | Time: %s | Avg: %s/s | Speed: %s/s",
			humanize.Bytes(uint64(p.TotalSize)),
			humanize.Bytes(uint64(d.ChunkSize)),
			d.Concurrency,
			humanize.Bytes(uint64(p.Size)),
			p.TotalCost().Round(time.Second),
			humanize.Bytes(p.AvgSpeed()),
			humanize.Bytes(p.Speed()),
		)
	}

	if err := d.Start(); err != nil {
		log.Fatal(err)
	}

	fmt.Println(" | Done!")
}
