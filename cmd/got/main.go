package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/apoorvam/goterminal"
	"github.com/dustin/go-humanize"
	"github.com/melbahja/got"
)

var (
	url         string
	version     string
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
		URL:         url,
		Dest:        *dest,
		ChunkSize:   *chunkSize,
		Interval:    100,
		Concurrency: *concurrency,
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
			"Downloading: (%s/%s) | Time: %s | Avg: %s/s | Speed: %s/s | Chunk: %s | Concurrency: %d\n",
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
	fmt.Println("Done!")
}
