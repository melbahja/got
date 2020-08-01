# Got

Got: Simple and fast GO command line tool to download files.


### Comparison in my machine:

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

### Comparison in cloud server:

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
