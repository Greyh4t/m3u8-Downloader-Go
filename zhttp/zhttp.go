package zhttp

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Zhttp struct {
	client *http.Client
}

func New(timeout time.Duration, proxy string) (*Zhttp, error) {
	z := &Zhttp{
		client: &http.Client{
			Timeout:   timeout,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
	}

	if proxy != "" {
		p, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		z.client.Transport.(*http.Transport).Proxy = func(*http.Request) (*url.URL, error) {
			return p, nil
		}
	}

	return z, nil
}

func (z *Zhttp) Get(url string, headers map[string]string, retry int) (code int, body []byte, err error) {
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
		code, body, err = z.get(req)
		if err == nil {
			return code, body, err
		}
		if strings.Contains(err.Error(), "INTERNAL_ERROR") {
			z.resetConnection()
		}
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
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}
