# Got

Got: GO CLI tool to download files faster than cURL and Wget!


### Comparison in my machine:
```bash
$ time curl http://speedtest.ftp.otenet.gr/files/test10Mb.db --output test

real	0m38.225s
user	0m0.044s
sys	0m0.199s
````

```bash
$ time got --out test http://speedtest.ftp.otenet.gr/files/test10Mb.db

real	0m7.136s
user	0m0.793s
sys	0m0.507s
```

### Comparison in cloud server: