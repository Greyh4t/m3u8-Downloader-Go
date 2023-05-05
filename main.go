package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafov/m3u8"
	"github.com/greyh4t/hackpool"
	"github.com/greyh4t/m3u8-Downloader-Go/decrypter"
	"github.com/greyh4t/m3u8-Downloader-Go/joiner"
	"github.com/greyh4t/m3u8-Downloader-Go/processbar"
	"github.com/greyh4t/m3u8-Downloader-Go/ts"
	"github.com/greyh4t/m3u8-Downloader-Go/zhttp"
	"github.com/guonaihong/clop"
)

var (
	ZHTTP        *zhttp.Zhttp
	JOINER       joiner.Joiner
	BAR          *processbar.Bar
	conf         *Conf
	keyCache     = map[string][]byte{}
	keyCacheLock sync.Mutex
)

type Conf struct {
	URL               string        `clop:"-u; --url" usage:"url of m3u8 file"`
	File              string        `clop:"-f; --m3u8-file" usage:"use local m3u8 file instead of downloading from url"`
	Connections       int           `clop:"-c; --connections" usage:"number of connections" default:"16"`
	OutFile           string        `clop:"-o; --out-file" usage:"out file"`
	Retry             int           `clop:"-r; --retry" usage:"number of retries" default:"3"`
	Timeout           time.Duration `clop:"-t; --timeout" usage:"timeout" default:"60s"`
	Proxy             string        `clop:"-p; --proxy" usage:"proxy. Example: http://127.0.0.1:8080"`
	Headers           []string      `clop:"-H; --header; greedy" usage:"http header. Example: Referer:http://www.example.com"`
	NoFix             bool          `clop:"-n; --nofix" usage:"don't try to remove the image header of the ts file"`
	SkipVerify        bool          `clop:"-s; --skipverify" usage:"skip verify server certificate"`
	MergeWithFFmpeg   bool          `clop:"-m; --merge-with-ffmpeg" usage:"merge with ffmpeg"`
	FFmpeg            string        `clop:"-F; --ffmpeg" usage:"path of ffmpeg" default:"ffmpeg"`
	DesiredResolution string        `clop:"-d; --desired-resolution" usage:"desired resolution. Example: 1920x1080"`
	ListResolution    bool          `clop:"-l; --list-resolution" usage:"list resolution"`
	headers           map[string]string
}

func init() {
	conf = &Conf{}
	clop.CommandLine.SetExit(true)
	clop.SetVersion("1.5.0")
	clop.Bind(&conf)

	checkConf()

	if len(conf.Headers) > 0 {
		parseHeaders()
	}
}

func checkConf() {
	if conf.URL == "" && conf.File == "" {
		fmt.Println("You must set the -u or -f parameter")
		clop.Usage()
	}

	if conf.Connections <= 0 {
		conf.Connections = 10
	}

	if conf.Retry <= 0 {
		conf.Retry = 1
	}

	if conf.Timeout <= 0 {
		conf.Timeout = time.Second * 60
	}
}

func parseHeaders() {
	conf.headers = map[string]string{}
	for _, header := range conf.Headers {
		s := strings.SplitN(header, ":", 2)
		key := strings.TrimRight(s[0], " ")
		if len(s) == 2 {
			conf.headers[key] = strings.TrimLeft(s[1], " ")
		} else {
			conf.headers[key] = ""
		}
	}
}

func startDownload(mpl *m3u8.MediaPlaylist) {
	pool := hackpool.New(conf.Connections, download)

	go func() {
		var count = int(mpl.Count())
		for i := 0; i < count; i++ {
			pool.Push(i, mpl.Segments[i], mpl.Key)
		}
		pool.CloseQueue()
	}()

	pool.Run()
}

func downloadM3u8(m3u8URL string) ([]byte, error) {
	return get(m3u8URL, conf.headers, conf.Retry)
}

func getKey(url string) ([]byte, error) {
	keyCacheLock.Lock()
	defer keyCacheLock.Unlock()

	key := keyCache[url]
	if key != nil {
		return key, nil
	}

	key, err := get(url, conf.headers, conf.Retry)
	if err != nil {
		return nil, err
	}

	keyCache[url] = key

	return key, nil
}

func download(args ...interface{}) {
	id := args[0].(int)
	segment := args[1].(*m3u8.MediaSegment)
	globalKey := args[2].(*m3u8.Key)

	data, err := get(segment.URI, conf.headers, conf.Retry)
	if err != nil {
		log.Fatalln("[-] Download failed:", id, err)
	}

	var keyURL, ivStr string
	if segment.Key != nil && segment.Key.URI != "" {
		keyURL = segment.Key.URI
		ivStr = segment.Key.IV
	} else if globalKey != nil && globalKey.URI != "" {
		keyURL = globalKey.URI
		ivStr = globalKey.IV
	}

	if keyURL != "" {
		var key, iv []byte
		key, err = getKey(keyURL)
		if err != nil {
			log.Fatalln("[-] Download key failed:", keyURL, err)
		}

		if ivStr != "" {
			iv, err = hex.DecodeString(strings.TrimPrefix(ivStr, "0x"))
			if err != nil {
				log.Fatalln("[-] Decode iv failed:", err)
			}
		} else {
			iv = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, byte(id)}
		}

		data, err = decrypter.Decrypt(data, key, iv)
		if err != nil {
			log.Fatalln("[-] Decrypt failed:", err)
		}
	}

	if !conf.NoFix {
		data = ts.TryFix(data)
	}

	err = JOINER.Add(id, data)
	if err != nil {
		log.Fatalln("[-] Write file failed:", err)
	}

	BAR.Incr()
	BAR.Flush()
}

func formatURI(base string, uri string) (string, error) {
	if strings.HasPrefix(uri, "http") {
		return uri, nil
	}

	if base == "" {
		return "", fmt.Errorf("base url must be set")
	}

	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	u, err = u.Parse(uri)
	if err != nil {
		return "", err
	}

	return u.String(), nil
}

func filename(u string) string {
	obj, _ := url.Parse(u)
	_, filename := filepath.Split(obj.Path)
	ext := filepath.Ext(filename)
	lowerExt := strings.ToLower(ext)
	if lowerExt == ".ts" || lowerExt == ".mp4" {
		return filename
	}
	filename = strings.TrimSuffix(filename, ext) + ".ts"
	return filename
}

func get(url string, headers map[string]string, retry int) ([]byte, error) {
	statusCode, data, err := ZHTTP.Get(url, headers, retry)
	if err != nil {
		return nil, err
	}

	if statusCode/100 != 2 || len(data) == 0 {
		return nil, fmt.Errorf("http status code: %d", statusCode)
	}

	return data, nil
}

func parseM3u8(m3u8URL string, desiredResolution string, data []byte) (*m3u8.MediaPlaylist, error) {
	if data != nil {
		playlist, listType, err := m3u8.Decode(*bytes.NewBuffer(data), true)
		if err != nil {
			return nil, err
		}

		if listType == m3u8.MEDIA {
			mpl := playlist.(*m3u8.MediaPlaylist)

			if mpl.Key != nil && mpl.Key.URI != "" {
				uri, err := formatURI(m3u8URL, mpl.Key.URI)
				if err != nil {
					return nil, fmt.Errorf("format uri failed: %w", err)
				}
				mpl.Key.URI = uri
			}

			count := int(mpl.Count())
			for i := 0; i < count; i++ {
				segment := mpl.Segments[i]

				uri, err := formatURI(m3u8URL, segment.URI)
				if err != nil {
					return nil, fmt.Errorf("format uri failed: %w", err)
				}
				segment.URI = uri

				if segment.Key != nil && segment.Key.URI != "" {
					uri, err := formatURI(m3u8URL, segment.Key.URI)
					if err != nil {
						return nil, fmt.Errorf("format uri failed: %w", err)
					}
					segment.Key.URI = uri
				}

				mpl.Segments[i] = segment
			}

			return mpl, nil
			// Master Playlist
		} else {
			mpl := playlist.(*m3u8.MasterPlaylist)
			variant, err := findVariant(mpl.Variants, desiredResolution)
			if err != nil {
				return nil, err
			}

			u, err := formatURI(m3u8URL, variant.URI)
			if err != nil {
				return nil, fmt.Errorf("format uri failed: %w", err)
			}
			return parseM3u8(u, desiredResolution, nil)
		}
	}

	data, err := downloadM3u8(m3u8URL)
	if err != nil {
		return nil, err
	}
	return parseM3u8(m3u8URL, desiredResolution, data)
}

func listResolution(m3u8URL string, data []byte) error {
	if data != nil {
		playlist, listType, err := m3u8.Decode(*bytes.NewBuffer(data), true)
		if err != nil {
			return err
		}

		if listType == m3u8.MEDIA {
			return fmt.Errorf("resource is not a playlist")
		} else {
			mpl := playlist.(*m3u8.MasterPlaylist)
			var list []string
			for _, v := range mpl.Variants {
				if v.Iframe {
					continue
				}
				list = append(list, fmt.Sprintf("Resolution: %-9s Bandwidth: %-8d FrameRate: %.2f Codecs: %s", v.Resolution, v.Bandwidth, v.FrameRate, v.Codecs))
			}
			fmt.Println(strings.Join(list, "\n"))
			return nil
		}
	}

	data, err := downloadM3u8(m3u8URL)
	if err != nil {
		return err
	}
	return listResolution(m3u8URL, data)
}

func findVariant(variants []*m3u8.Variant, resolution string) (*m3u8.Variant, error) {
	if len(variants) == 0 {
		return nil, fmt.Errorf("variants not found")
	}

	sort.Slice(variants, func(i, j int) bool {
		if variants[i].Resolution != "" && variants[j].Resolution != "" {
			widthi, heighti := parseResolution(variants[i].Resolution)
			widthj, heightj := parseResolution(variants[j].Resolution)
			if widthi*heighti < widthj*heightj {
				return false
			} else if widthi*heighti > widthj*heightj {
				return true
			}
		}

		return variants[i].Bandwidth > variants[j].Bandwidth
	})

	if resolution != "" {
		for _, v := range variants {
			if v.Iframe {
				continue
			}
			if v.Resolution == resolution {
				return v, nil
			}
		}

		return nil, fmt.Errorf("resolution %s not found", resolution)
	}

	return variants[0], nil
}

func parseResolution(resolution string) (uint64, uint64) {
	arr := strings.Split(resolution, "x")
	if len(arr) != 2 {
		return 0, 0
	}
	width, err := strconv.ParseUint(arr[0], 10, 64)
	if err != nil {
		return 0, 0
	}
	height, err := strconv.ParseUint(arr[1], 10, 64)
	if err != nil {
		return 0, 0
	}
	return width, height
}

func main() {
	var err error
	ZHTTP, err = zhttp.New(conf.Timeout, conf.Proxy, conf.SkipVerify)
	if err != nil {
		log.Fatalln("[-] Initialization failed:", err)
	}

	var data []byte
	if conf.File != "" {
		data, err = os.ReadFile(conf.File)
		if err != nil {
			log.Fatalln("[-] Load m3u8 file failed:", err)
		}
	}

	if conf.ListResolution {
		err := listResolution(conf.URL, data)
		if err != nil {
			log.Fatalln("[-] Parse m3u8 file failed:", err)
		}
		return
	}

	mpl, err := parseM3u8(conf.URL, conf.DesiredResolution, data)
	if err != nil {
		log.Fatalln("[-] Parse m3u8 file failed:", err)
	}

	outFile := conf.OutFile
	if outFile == "" {
		outFile = filename(mpl.Segments[0].URI)
	}

	if conf.MergeWithFFmpeg {
		JOINER, err = joiner.NewFFmepg(conf.FFmpeg, outFile)
		if err != nil {
			log.Fatalln("[-]", err)
		}
	} else {
		JOINER, err = joiner.NewMem(outFile)
		if err != nil {
			log.Fatalln("[-]", err)
		}
	}

	if mpl.Count() > 0 {
		BAR = processbar.New(int(mpl.Count()))
		BAR.Flush()

		startDownload(mpl)
		BAR.Finish()

		err = JOINER.Merge()
		if err != nil {
			log.Fatalln("[-] Saved to", outFile, "failed:", err)
		} else {
			log.Println("[+] Saved to", outFile)
		}
	}
}
