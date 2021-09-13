# m3u8-Downloader-Go
Download m3u8 media with multithreading, support decrypt

# How to use
`./m3u8-Downloader-Go -u "http://wwww.example.com/example.m3u8" -o video.ts`

`./m3u8-Downloader-Go -f example.m3u8 -H Referer:http://www.example.com -H 'User-Agent:Chrome/83.0.4103.61 Safari/537.36'`

### Note 
When using the -f parameter, if the m3u8 file does not contain a specific link to the media, but only the media name, you must specify the -u parameter

Some websites will add an image header at the beginning of the video file. If the video you downloaded cannot be played normally, please try the `--fix` parameter

```
./m3u8-Downloader-Go -h

Usage:
    ./m3u8-Downloader-Go [Flags] [Options] 

Flags:
    -F,--fix              try to repair the ts file by removing the image header

Options:
    -H,--header           http header. Example: Referer:http://www.example.com
    -f,--m3u8-file        local m3u8 file
    -n,--thread-number    thread number [default: 10]
    -o,--out-file         out file
    -p,--proxy            proxy. Example: http://127.0.0.1:8080
    -r,--retry            number of retries [default: 3]
    -t,--timeout          timeout [default: 60s]
    -u,--url              url of m3u8 file
```
