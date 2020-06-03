package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"flag"
	"log"
	"m3u8-Downloader-Go/decrypter"
	"m3u8-Downloader-Go/joiner"
	"m3u8-Downloader-Go/zhttp"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafov/m3u8"
	"github.com/greyh4t/hackpool"
)

var (
	ZHTTP        *zhttp.Zhttp
	JOINER       *joiner.Joiner
	conf         *Conf
	keyCache     map[string][]byte
	keyCacheLock sync.Mutex
	headers      map[string]string
)

type Conf struct {
	Url       string
	ThreadNum int
	OutFile   string
	Retry     int
	Timeout   time.Duration
	Proxy     string
}

func init() {
	conf = &Conf{}

	flag.StringVar(&conf.Url, "u", "", "URL of m3u8")
	flag.IntVar(&conf.ThreadNum, "n", 10, "Thread number")
	flag.StringVar(&conf.OutFile, "o", "", "Out file")
	flag.IntVar(&conf.Retry, "r", 3, "Number of retries")
	flag.DurationVar(&conf.Timeout, "t", time.Second*30, "Timeout")
	flag.StringVar(&conf.Proxy, "p", "", "Proxy\nExample: http://127.0.0.1:8080")
	header := flag.String("H", "", "HTTP Headers\nExample: Referer=http://www.example.com;UserAgent=Mozilla/5.0")

	flag.Parse()

	checkConf()

	keyCache = map[string][]byte{}
	headers = map[string]string{}

	if *header != "" {
		lines := strings.Split(*header, ";")
		for _, line := range lines {
			s := strings.SplitN(line, "=", 2)
			if len(s) == 2 {
				headers[s[0]] = s[1]
			} else {
				headers[s[0]] = ""
			}
		}
	}
}

func checkConf() {
	if conf.Url == "" {
		flag.Usage()
		os.Exit(0)
		return
	}

	if conf.ThreadNum <= 0 {
		conf.ThreadNum = 10
	}

	if conf.Retry <= 0 {
		conf.Retry = 1
	}

	if conf.Timeout <= 0 {
		conf.Timeout = time.Second * 10
	}
}

func start(mpl *m3u8.MediaPlaylist) {
	pool := hackpool.New(conf.ThreadNum, download)

	go func() {
		var count = int(mpl.Count())
		for i := 0; i < count; i++ {
			pool.Push([]interface{}{i, mpl.Segments[i], mpl.Key})
		}
		pool.CloseQueue()
	}()

	go pool.Run()
}

func parseM3u8(m3u8Url string) (*m3u8.MediaPlaylist, error) {
	statusCode, data, err := ZHTTP.Get(m3u8Url, headers, conf.Retry)
	if err != nil {
		return nil, err
	}

	if statusCode/100 != 2 || len(data) == 0 {
		return nil, errors.New("download m3u8 file failed, http code: " + strconv.Itoa(statusCode))
	}

	playlist, listType, err := m3u8.Decode(*bytes.NewBuffer(data), true)
	if err != nil {
		return nil, err
	}

	if listType == m3u8.MEDIA {
		obj, _ := url.Parse(m3u8Url)
		mpl := playlist.(*m3u8.MediaPlaylist)

		if mpl.Key != nil && mpl.Key.URI != "" {
			uri, err := formatURI(obj, mpl.Key.URI)
			if err != nil {
				return nil, err
			}
			mpl.Key.URI = uri
		}

		count := int(mpl.Count())
		for i := 0; i < count; i++ {
			segment := mpl.Segments[i]

			uri, err := formatURI(obj, segment.URI)
			if err != nil {
				return nil, err
			}
			segment.URI = uri

			if segment.Key != nil && segment.Key.URI != "" {
				uri, err := formatURI(obj, segment.Key.URI)
				if err != nil {
					return nil, err
				}
				segment.Key.URI = uri
			}

			mpl.Segments[i] = segment
		}

		return mpl, nil
	}

	return nil, errors.New("Unsupport m3u8 type")
}

func getKey(url string) ([]byte, error) {
	keyCacheLock.Lock()
	defer keyCacheLock.Unlock()

	key := keyCache[url]
	if key != nil {
		return key, nil
	}

	statusCode, key, err := ZHTTP.Get(url, headers, conf.Retry)
	if err != nil {
		return nil, err
	}

	if len(key) == 0 {
		return nil, errors.New("body is empty, http code: " + strconv.Itoa(statusCode))
	}

	keyCache[url] = key

	return key, nil
}

func download(in interface{}) {
	params := in.([]interface{})
	id := params[0].(int)
	segment := params[1].(*m3u8.MediaSegment)
	globalKey := params[2].(*m3u8.Key)

	statusCode, data, err := ZHTTP.Get(segment.URI, headers, conf.Retry)
	if err != nil {
		log.Fatalln("[-] Download failed:", err)
	}

	if len(data) == 0 {
		log.Fatalln("[-] Download failed: body is empty, http code:", statusCode)
	}

	var keyUrl, ivStr string
	if segment.Key != nil && segment.Key.URI != "" {
		keyUrl = segment.Key.URI
		ivStr = segment.Key.IV
	} else if globalKey != nil && globalKey.URI != "" {
		keyUrl = globalKey.URI
		ivStr = globalKey.IV
	}

	if keyUrl != "" {
		var key, iv []byte
		key, err = getKey(keyUrl)
		if err != nil {
			log.Fatalln("[-] Download key failed:", keyUrl, err)
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

	log.Println("[+] Download succed:", segment.URI)

	JOINER.Join(id, data)
}

func formatURI(base *url.URL, u string) (string, error) {
	if strings.HasPrefix(u, "http") {
		return u, nil
	}

	obj, err := base.Parse(u)
	if err != nil {
		return "", err
	}

	return obj.String(), nil
}

func filename(u string) string {
	obj, _ := url.Parse(u)
	_, filename := filepath.Split(obj.Path)
	return filename
}

func main() {
	var err error
	ZHTTP, err = zhttp.New(conf.Timeout, conf.Proxy)
	if err != nil {
		log.Fatalln("[-] Init failed:", err)
	}

	mpl, err := parseM3u8(conf.Url)
	if err != nil {
		log.Fatalln("[-] Parse m3u8 file failed:", err)
	} else {
		log.Println("[+] Parse m3u8 file succed")
	}

	outFile := conf.OutFile
	if outFile == "" {
		outFile = filename(mpl.Segments[0].URI)
	}

	JOINER, err = joiner.New(outFile)
	if err != nil {
		log.Fatalln("[-] Open file failed:", err)
	} else {
		log.Println("[+] Will save to", JOINER.Name())
	}

	if mpl.Count() > 0 {
		log.Println("[+] Total", mpl.Count(), "files to download")

		start(mpl)

		err = JOINER.Run(int(mpl.Count()))
		if err != nil {
			log.Fatalln("[-] Write to file failed:", err)
		}
		log.Println("[+] Download succed, saved to", JOINER.Name())
	}
}
