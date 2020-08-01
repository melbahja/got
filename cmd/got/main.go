package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/dustin/go-humanize"
	"github.com/melbahja/got"
)

var (
	url         string
	version     string
	dest        = flag.String("out", "", "Downloaded file path.")
	redirects   = flag.Bool("redirects", false, "Follow redirects.")
	chunkSize   = flag.Int("size", 0, "Max chunk size.")
	concurrency = flag.Int("concurrency", 0, "Max download connections to open at the same time.")
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

	d := got.Download{
		URL:  url,
		Dest: *dest,
		ProgressFunc: func(i int64, t int64, d *got.Download) {

			fmt.Printf(
				"\r\r\b Total Size: %s | Chunk Size: %s | Concurrency: %d | Progress: %s ",
				humanize.Bytes(uint64(t)),
				humanize.Bytes(uint64(d.ChunkSize)),
				d.Concurrency,
				humanize.Bytes(uint64(i)),
			)
		},
		Redirects:   *redirects,
		ChunkSize:   int64(*chunkSize),
		Interval:    100,
		Concurrency: *concurrency,
	}

	if err := d.Init(); err != nil {
		log.Fatal(err)
	}

	if err := d.Start(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("| Done!")
}
