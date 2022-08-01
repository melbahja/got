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

![Tests](https://github.com/melbahja/got/workflows/Test/badge.svg)

## Comparison

Comparison in cloud server:

```bash

[root@centos-nyc-12 ~]# time got -o /tmp/test -c 20 https://proof.ovh.net/files/1Gb.dat
URL: https://proof.ovh.net/files/1Gb.dat done!

real    0m8.832s
user    0m0.203s
sys 0m3.176s


[root@centos-nyc-12 ~]# time curl https://proof.ovh.net/files/1Gb.dat --output /tmp/test1
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
								 Dload  Upload   Total   Spent    Left  Speed
100 1024M  100 1024M    0     0  35.6M      0  0:00:28  0:00:28 --:--:-- 34.4M

real    0m28.781s
user    0m0.379s
sys 0m1.970s

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
go install github.com/melbahja/got/cmd/got@latest
```

#### Or from the AUR
Install [`got`](https://aur.archlinux.org/packages/got/) for the latest release version or `got-git` for the latest development version. 

> **Note:** these packages are not maintained by melbahja

## Command Line Tool Usage

#### Simple usage:
```bash
got https://example.com/file.mp4
```

#### You can specify destination path:
```bash
got -o /path/to/save https://example.com/file.mp4
```

#### You can download multiple URLs and save them to directory:
```bash
got --dir /path/to/dir https://example.com/file.mp4 https://example.com/file2.mp4
```

#### You can download multiple URLs from a file:
```bash
got --dir /path/to/dir -f urls.txt
```

### You can pipe multiple URLs:
```bash
cat urls.txt | got --dir /path/to/dir
```

#### Docs for available flags:
```bash
got help
```


## Module Usage

You can use Got to download large files in your go code, the usage is simple as the CLI tool:

```bash
package main

import "github.com/melbahja/got"

func main() {

	g := got.New()

	err := g.Download("http://localhost/file.ext", "/path/to/save")

	if err != nil {
		// ..
	}
}

```

For more see [PkgDocs](https://pkg.go.dev/github.com/melbahja/got).

## How It Works?

Got takes advantage of the HTTP range requests support in servers [RFC 7233](https://tools.ietf.org/html/rfc7233), if the server supports partial content Got split the file into chunks, then starts downloading and merging the chunks into the destinaton file concurrently.


## License

Got is provided under the [MIT License](https://github.com/melbahja/got/blob/master/LICENSE) © Mohammed El Bahja.
