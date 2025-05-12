package zhttp

import (
	"compress/gzip"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Zhttp struct {
	client *http.Client
}

func New(timeout time.Duration, proxy string, skipVerify bool) (*Zhttp, error) {
	z := &Zhttp{
		client: &http.Client{
			Timeout:   timeout,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
	}
	if skipVerify {
		z.client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if proxy != "" {
		p, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		z.client.Transport.(*http.Transport).Proxy = http.ProxyURL(p)
	}

	return z, nil
}

func (z *Zhttp) Get(url string, headers map[string]string, retry int) (code int, body []byte, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	for retry > 0 {
		retry--
		code, body, err = z.get(req)
		if err == nil {
			if code/100 == 2 {
				return code, body, err
			}
		} else if strings.Contains(err.Error(), "INTERNAL_ERROR") {
			z.resetConnection()
		}
		time.Sleep(time.Second * 2)
	}

	return
}

func (z *Zhttp) resetConnection() {
	t := z.client.Transport.(*http.Transport)
	t.CloseIdleConnections()
	z.client.Transport = t.Clone()
}

func (z *Zhttp) get(req *http.Request) (int, []byte, error) {
	resp, err := z.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	r := resp.Body
	if equalFold(resp.Header.Get("Content-Encoding"), "gzip") && !resp.Uncompressed {
		r, err = gzip.NewReader(resp.Body)
		if err != nil {
			return 0, nil, err
		}
		defer r.Close()
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}

// equalFold is strings.equalFold, ASCII only. It reports whether s and t
// are equal, ASCII-case-insensitively.
func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if lower(s[i]) != lower(t[i]) {
			return false
		}
	}
	return true
}

// lower returns the ASCII lowercase version of b.
func lower(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}
