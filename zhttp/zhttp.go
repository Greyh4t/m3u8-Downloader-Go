package zhttp

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type Zhttp struct {
	client *http.Client
}

func New(timeout time.Duration, proxy string) (*Zhttp, error) {
	zhttp := &Zhttp{
		client: http.DefaultClient,
	}

	if timeout > 0 {
		zhttp.client.Timeout = timeout
	}

	if proxy != "" {
		p, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}

		t := &http.Transport{}
		t.Proxy = func(*http.Request) (*url.URL, error) {
			return p, nil
		}
		zhttp.client.Transport = t
	}

	return zhttp, nil
}

func (zhttp *Zhttp) Get(url string, headers map[string]string, retry int) (int, []byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	var body []byte
	var code int

	var count int
	for count < retry {
		count++

		var resp *http.Response
		resp, err = zhttp.client.Do(req)
		if err != nil {
			continue
		}

		body, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			continue
		}

		code = resp.StatusCode

		break
	}

	if err != nil {
		return 0, nil, err
	}

	return code, body, nil
}
