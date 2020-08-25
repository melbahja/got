package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dustin/go-humanize"
	"github.com/melbahja/got"
	"github.com/urfave/cli/v2"
	"gitlab.com/poldi1405/go-ansi"
	"gitlab.com/poldi1405/go-indicators/progress"
)

var version string

func main() {

	// New context.
	ctx, cancel := context.WithCancel(context.Background())

	interruptChan := make(chan os.Signal, 1)

	signal.Notify(interruptChan, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)

	go func() {
		<-interruptChan
		cancel()
		signal.Stop(interruptChan)
		log.Fatal(got.ErrDownloadAborted)
	}()

	// CLI app.
	app := &cli.App{
		Name:  "Got",
		Usage: "The fastest http downloader.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Usage:   "Download save `path`.",
				Aliases: []string{"out", "o"},
			},
			&cli.StringFlag{
				Name:    "dir",
				Usage:   "Save downloaded file to `directory`.",
				Aliases: []string{"d"},
			},
			&cli.StringFlag{
				Name:    "file",
				Usage:   "Batch download file from list in `file`.",
				Aliases: []string{"bf", "f"},
			},
			&cli.Uint64Flag{
				Name:    "size",
				Usage:   "File chunks size in `bytes`.",
				Aliases: []string{"chunk"},
			},
			&cli.UintFlag{
				Name:    "concurrency",
				Usage:   "Number of chunks to download at the same time.",
				Aliases: []string{"c"},
			},
		},
		Version: version,
		Authors: []*cli.Author{
			{
				Name:  "Mohamed Elbahja and Contributors",
				Email: "bm9qdW5r@gmail.com",
			},
		},
		Action: func(c *cli.Context) error {
			return run(ctx, c)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, c *cli.Context) error {

	// New *Got.
	g := got.NewWithContext(ctx)
	var p progress.Progress
	p.SetStyle(simpleProgressStyle)
	p.Width = 30

	// Progress.
	g.ProgressFunc = func(d *got.Download) {

		perc, err := progress.GetPercentage(float64(d.Size()), float64(d.TotalSize()))
		if err != nil {
			perc = 100
		}
		fmt.Printf(
			" %6.2f%% %s%s%s %s/%s @ %s/s"+ansi.ClearRight()+"\r",
			perc,
			r,
			color(p.GetBar(perc, 100)),
			l,
			humanize.Bytes(d.Size()),
			humanize.Bytes(d.TotalSize()),
			humanize.Bytes(d.Speed()),
		)
	}

	info, err := os.Stdin.Stat()

	if err != nil {
		return err
	}

	// Create dir if not exists.
	if c.String("dir") != "" {

		if _, err := os.Stat(c.String("dir")); os.IsNotExist(err) {
			os.MkdirAll(c.String("dir"), os.ModePerm)
		}
	}

	// Piped stdin
	if info.Mode()&os.ModeNamedPipe > 0 {

		if err := multiDownload(ctx, c, g, bufio.NewScanner(os.Stdin)); err != nil {
			return err
		}
	}

	// Batch file.
	if c.String("file") != "" {

		file, err := os.Open(c.String("file"))

		if err != nil {
			return err
		}

		if err := multiDownload(ctx, c, g, bufio.NewScanner(file)); err != nil {
			return err
		}
	}

	// Download from args.
	for _, url := range c.Args().Slice() {

		if err = download(ctx, c, g, url); err != nil {
			return err
		}

		fmt.Print(ansi.ClearLine())
		fmt.Println(fmt.Sprintf("URL: %s done!", url))
	}

	return nil
}

func multiDownload(ctx context.Context, c *cli.Context, g *got.Got, scanner *bufio.Scanner) error {

	for scanner.Scan() {

		url := strings.TrimSpace(scanner.Text())

		if url == "" {
			continue
		}

		if err := download(ctx, c, g, url); err != nil {
			return err
		}

		fmt.Print(ansi.ClearLine())
		fmt.Println(fmt.Sprintf("URL: %s done!", url))
	}

	return nil
}

func download(ctx context.Context, c *cli.Context, g *got.Got, url string) (err error) {

	if url, err = getURL(url); err != nil {
		return err
	}

	fname := c.String("output")

	if fname == "" {
		fname = got.GetFilename(url)
	}

	return g.Do(&got.Download{
		URL:         url,
		Dest:        filepath.Join(c.String("dir"), fname),
		Interval:    100,
		ChunkSize:   c.Uint64("size"),
		Concurrency: c.Uint("concurrency"),
	})
}

func getURL(URL string) (string, error) {

	u, err := url.Parse(URL)

	if err != nil {
		return "", err
	}

	// Fallback to https by default.
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	return u.String(), nil
}
