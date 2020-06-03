# m3u8-Downloader-Go
Download m3u8 media with multithreading, support decrypt

# How to use
```
m3u8-Downloader-Go -u "http://wwww.example.com/example.m3u8" -o /video.ts

Usage of m3u8-Downloader-Go:
  -H string
        HTTP Headers
        Example: Referer=http://www.example.com;UserAgent=Mozilla/5.0
  -n int
        Thread number (default 10)
  -o string
        Out file
  -p string
        Proxy
        Example: http://127.0.0.1:8080
  -r int
        Number of retries (default 3)
  -t duration
        Timeout (default 30s)
  -u string
        URL of m3u8
```