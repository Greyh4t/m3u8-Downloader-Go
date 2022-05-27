package zhttp

import (
	"io"
	"net/http"
	"net/url"
	"time"
)

type Zhttp struct {
	client *http.Client
}

func New(timeout time.Duration, proxy string) (*Zhttp, error) {
	zhttp := &Zhttp{
		client: &http.Client{
			Timeout: timeout,
		},
	}

	if proxy != "" {
		p, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}

		t := http.DefaultTransport.(*http.Transport).Clone()
		t.Proxy = func(*http.Request) (*url.URL, error) {
			return p, nil
		}
		zhttp.client.Transport = t
	}

	return zhttp, nil
}

func (zhttp *Zhttp) Get(url string, headers map[string]string, retry int) (code int, body []byte, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	for retry > 0 {
		retry--
		code, body, err = zhttp.get(req)
		if err == nil {
			return code, body, err
		}
	}

	return
}

func (zhttp *Zhttp) get(req *http.Request) (int, []byte, error) {
	resp, err := zhttp.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}
