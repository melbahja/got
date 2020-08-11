<div align="center">
	<h1>Got.</h1>
    <h4 align="center">
	   Simple and fast concurrent downloader.
	</h4>
</div>

<p align="center">
    <a href="#installation">Installation</a> ❘
    <a href="#command-line-tool-usage">CLI Usage</a> ❘
    <a href="#module-usage">Module Usage</a> ❘
    <a href="#license">License</a>
</p>

## Comparison

Comparison in my machine:

```bash
// cURL
$ time curl http://speedtest.ftp.otenet.gr/files/test10Mb.db --output test

real	0m38.225s
user	0m0.044s
sys	0m0.199s


// Got
$ time got --out test http://speedtest.ftp.otenet.gr/files/test10Mb.db

real	0m7.136s
user	0m0.793s
sys	0m0.507s
```
---
Comparison in cloud server:

```bash
// Got
$ time got --out /tmp/test http://www.ovh.net/files/1Gio.dat
 Total Size: 1.1 GB | Chunk Size: 54 MB | Concurrency: 10 | Progress: 1.1 GB | Done!

real	0m10.273s
user	0m0.205s
sys	0m3.296s

// cURL
$ time curl http://www.ovh.net/files/1Gio.dat --output /tmp/test1
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100 1024M  100 1024M    0     0  30.8M      0  0:00:33  0:00:33 --:--:-- 36.4M

real	0m33.318s
user	0m0.420s
sys	0m2.056s
```

## Installation

#### Download and install the latest [release](https://github.com/melbahja/got/releases):
```bash
# go to tmp dir.
cd /tmp

# Download latest version.
curl -sfL https://git.io/getgot | sh

# Make the binary executable.
chmod +x /tmp/bin/got

# Move the binary to your PATH
sudo mv /tmp/bin/got /usr/bin/got
```

#### Or Go ahead compile it yourself:
```bash
go get github.com/melbahja/got/cmd/got
```


## Command Line Tool Usage

#### Simple usage:
```bash
got https://example.com/file.mp4
```

#### Or you can specify destination path:
```bash
got --out /path/to/save https://example.com/file.mp4
```

#### To see all available flags type:
```bash
got --help
```


## Module Usage

You can use Got to download large files in your go code, the usage is simple as the CLI tool:

```bash
package main

import "github.com/melbahja/got"

func main() {

    dl, err := got.New("https://example.com/file.mp4", "/path/to/save")

    if err != nil {
    	// handle the error!
    }

    // Start the download
    err = dl.Start()
}

```

For more see [GoDocs](https://pkg.go.dev/github.com/melbahja/got).


## License

Got is provided under the [MIT License](https://github.com/melbahja/got/blob/master/LICENSE) © Mohammed El Bahja.
