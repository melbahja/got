package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dustin/go-humanize"
	"github.com/melbahja/got"
	"github.com/urfave/cli/v2"
	"gitlab.com/poldi1405/go-ansi"
	"gitlab.com/poldi1405/go-indicators/progress"
	"golang.org/x/crypto/ssh/terminal"
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
				Usage:   "Download `path`, if dir passed the path witll be `dir + output`.",
				Aliases: []string{"o"},
			},
			&cli.StringFlag{
				Name:    "dir",
				Usage:   "Save downloaded file to a `directory`.",
				Aliases: []string{"d"},
			},
			&cli.StringFlag{
				Name:    "file",
				Usage:   "Batch download from list of urls in a `file`.",
				Aliases: []string{"bf", "f"},
			},
			&cli.Uint64Flag{
				Name:    "size",
				Usage:   "Chunk size in `bytes` to split the file.",
				Aliases: []string{"chunk"},
			},
			&cli.UintFlag{
				Name:    "concurrency",
				Usage:   "Chunks that will be downloaded concurrently.",
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

	var (
		g *got.Got           = got.NewWithContext(ctx)
		p *progress.Progress = new(progress.Progress)
	)

	// Set progress style.
	p.SetStyle(progressStyle)

	// Progress func.
	g.ProgressFunc = func(d *got.Download) {

		// 55 is just an estimation of the text showed with the progress.
		// it's working fine with $COLUMNS >= 47
		// TODO: hide progress bar on terminal size of $COLUMNS <= 46
		p.Width = getWidth() - 55

		perc, err := progress.GetPercentage(float64(d.Size()), float64(d.TotalSize()))
		if err != nil {
			perc = 100
		}

		fmt.Printf(
			" %6.2f%% %s%s%s %s/%s @ %s/s%s\r",
			perc,
			r,
			color(p.GetBar(perc, 100)),
			l,
			humanize.Bytes(d.Size()),
			humanize.Bytes(d.TotalSize()),
			humanize.Bytes(d.Speed()),
			ansi.ClearRight(),
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
	if info.Mode()&os.ModeNamedPipe > 0 || info.Size() > 0 {

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
		fmt.Println(fmt.Sprintf("✔ %s", url))
	}

	return nil
}

func getWidth() int {

	if width, _, err := terminal.GetSize(0); err == nil && width > 0 {
		return width
	}

	return 80
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
		fmt.Println(fmt.Sprintf("✔ %s", url))
	}

	return nil
}

func download(ctx context.Context, c *cli.Context, g *got.Got, url string) (err error) {

	if url, err = getURL(url); err != nil {
		return err
	}

	return g.Do(&got.Download{
		URL:         url,
		Dir:         c.String("dir"),
		Dest:        c.String("output"),
		Interval:    150,
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
