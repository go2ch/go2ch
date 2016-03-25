package go2ch

import (
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client is unofficial 2ch API client
type Client struct {
	appKey string
	hmKey  string

	Transport     *http.Transport
	BaseURL       string
	MaxRetry      int
	SessionMaxAge time.Duration
	Timeout       time.Duration

	user          string
	pass          string
	session       string
	sessionExpire time.Time
	mutex         sync.Mutex
}

func (c *Client) makeRequest(path string, headers map[string]string, data string) (*http.Response, error) {
	url := c.BaseURL + path

	for i, t := 0, c.Timeout; i <= c.MaxRetry; i, t = i+1, t*2 {
		req, err := http.NewRequest("POST", url, strings.NewReader(data))
		if err != nil {
			return nil, err
		}

		req.Header = map[string][]string{
			"Accept":         {"text/html, */*"},
			"Content-Type":   {"application/x-www-form-urlencoded"},
			"Content-Length": {strconv.Itoa(len(data))},
			"User-Agent":     {"Mozilla/3.0 (compatible; JaneStyle/3.83)"},
		}

		for k, v := range headers {
			req.Header.Add(k, v)
		}

		client := &http.Client{Transport: c.Transport, Timeout: t}
		resp, err := client.Do(req)
		if err != nil {
			if strings.Contains(err.Error(), "Client.Timeout exceeded") {
				continue
			}

			return nil, err
		}

		switch resp.StatusCode {
		case 403:
			resp.Body.Close()
			return nil, fmt.Errorf("forbidden")
		case 400, 500, 502:
			if resp.Header.Get("Server") == "cloudflare-nginx" {
				resp.Body.Close()
				continue
			}
		}

		return resp, nil
	}

	return nil, fmt.Errorf("response error")
}

// Auth sends authentication request
func (c *Client) Auth(user, pass string) error {
	ct := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(c.hmKey))
	mac.Write([]byte(c.appKey + ct))
	hb := hex.EncodeToString(mac.Sum(nil))
	data := "ID=" + user + "&PW=" + pass + "&KY=" + c.appKey + "&CT=" + ct + "&HB=" + hb

	headers := map[string]string{
		"X-2ch-UA": "JaneStyle/3.83",
	}

	resp, err := c.makeRequest("/v1/auth/", headers, data)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if string(buf[:26]) == "SESSION-ID=Monazilla/1.00:" {
		c.user = user
		c.pass = pass
		c.session = string(buf[26:])
		c.sessionExpire = time.Now().Add(c.SessionMaxAge)
		return nil
	}

	switch string(buf) {
	case "ng (appkey incorrect length)":
		return fmt.Errorf("appkey incorrect length")
	}

	return fmt.Errorf("auth error")
}

// Get sends thread request
func (c *Client) Get(server, bbs, key string, reqHeaders map[string]string) (*http.Response, error) {
	c.mutex.Lock()

	if c.session == "" {
		err := c.Auth(c.user, c.pass)
		if err != nil {
			c.mutex.Unlock()
			return nil, err
		}
	} else if time.Since(c.sessionExpire) >= 0 && c.Auth(c.user, c.pass) != nil {
		c.sessionExpire = time.Now().Add(c.SessionMaxAge)
	}

	c.mutex.Unlock()

	path := strings.Join([]string{"/v1", server, bbs, key}, "/")
	mac := hmac.New(sha256.New, []byte(c.hmKey))
	mac.Write([]byte(path + c.session + c.appKey))
	hobo := hex.EncodeToString(mac.Sum(nil))
	data := "sid=" + c.session + "&hobo=" + hobo + "&appkey=" + c.appKey

	headers := make(map[string]string)
	for k, v := range reqHeaders {
		headers[k] = v
	}

	var addedGzip bool
	if headers["Accept-Encoding"] == "" && headers["Range"] == "" {
		headers["Accept-Encoding"] = "gzip"
		addedGzip = true
	}

	resp, err := c.makeRequest(path, headers, data)
	if err != nil {
		return nil, err
	}

	switch resp.Header.Get("Thread-Status") {
	case "0": // StatusCode: 404
		resp.Body.Close()
		return nil, fmt.Errorf("not found/invalid range request")
	case "1":
		if addedGzip && resp.Header.Get("Content-Encoding") == "gzip" {
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
			resp.ContentLength = -1
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				return nil, err
			}
			resp.Body = ioutil.NopCloser(reader)
		}
		return resp, nil
	case "8": // StatusCode: 200/501
		resp.Body.Close()
		return nil, fmt.Errorf("thread dat-out")
	}

	resp.Body.Close()

	switch resp.StatusCode {
	case 401:
		c.session = ""
		return c.Get(server, bbs, key, reqHeaders)
	default:
		return nil, fmt.Errorf("unknown error")
	}
}

// NewClient returns new Client instance
func NewClient(appKey, hmKey string) *Client {
	tr := &http.Transport{
		DisableKeepAlives:  true,
		DisableCompression: true,
		Proxy:              http.ProxyFromEnvironment,
	}

	return &Client{
		appKey:        appKey,
		hmKey:         hmKey,
		Transport:     tr,
		BaseURL:       "https://api.2ch.net",
		MaxRetry:      5,
		SessionMaxAge: 6 * time.Hour,
	}
}
