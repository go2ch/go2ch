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
  "time"
)

// Client is unofficial 2ch API client
type Client struct {
  AppKey string
  HmKey string

  SessionMaxAge time.Duration

  Client *http.Client

  user string
  pass string
  session string
  sessionExpire time.Time
}

func (c *Client) makeRequest(path string, headers map[string]string, data string) (*http.Response, error) {
  url := "https://api.2ch.net" + path
  req, err := http.NewRequest("POST", url, strings.NewReader(data))
  if err != nil {
    return nil, err
  }

  req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
  req.Header.Add("Content-Length", strconv.Itoa(len(data)))

  for k, v := range headers {
    req.Header.Add(k, v)
  }

  resp, err := c.Client.Do(req)
  if err != nil {
    fmt.Println(err)
    return nil, err
  }

  return resp, nil
}

// Auth sends authentication request
func (c *Client) Auth(user, pass string) error {
  ct := strconv.FormatInt(time.Now().Unix(), 10)
  mac := hmac.New(sha256.New, []byte(c.HmKey))
	mac.Write([]byte(c.AppKey + ct))
  hb := hex.EncodeToString(mac.Sum(nil))
  data := "ID=" + user + "&PW=" + pass + "&KY=" + c.AppKey + "&CT=" + ct + "&HB=" + hb

  headers := map[string]string{
    "Accept": "text/html, */*",
    "User-Agent": "Mozilla/3.0 (compatible; JaneStyle/3.83)",
    "X-2ch-UA": "JaneStyle/3.83",
  }

  resp, err := c.makeRequest("/v1/auth/", headers, data)
  if err != nil {
    return err
  }

  if resp.StatusCode == 500 {
    resp.Body.Close()
    return c.Auth(user, pass)
  }

  defer resp.Body.Close()
  buf := make([]byte, 26)
  resp.Body.Read(buf)

  if string(buf) == "SESSION-ID=Monazilla/1.00:" {
    session, _ := ioutil.ReadAll(resp.Body)
    c.user = user
    c.pass = pass
    c.session = string(session)
    c.sessionExpire = time.Now().Add(c.SessionMaxAge)
    return nil
  }

  return fmt.Errorf("auth error")
}

// Get sends thread request
func (c *Client) Get(server, bbs, key string, reqHeaders map[string]string) (*http.Response, error) {
  if c.session == "" {
    err := c.Auth(c.user, c.pass)
    if err != nil {
      return nil, err
    }
  } else if time.Since(c.sessionExpire) >= 0 && c.Auth(c.user, c.pass) != nil {
    c.sessionExpire = time.Now().Add(c.SessionMaxAge)
  }

  path := strings.Join([]string{"/v1", server, bbs, key}, "/")
  mac := hmac.New(sha256.New, []byte(c.HmKey))
  mac.Write([]byte(path + c.session + c.AppKey))
  hobo := hex.EncodeToString(mac.Sum(nil))
  data := "sid=" + c.session + "&hobo=" + hobo + "&appkey=" + c.AppKey

  headers := make(map[string]string)
  for k,v := range reqHeaders {
    headers[k] = v
  }
  headers["Accept"] = "text/html, */*"
  headers["User-Agent"] = "Mozilla/3.0 (compatible; JaneStyle/3.83)"

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
      resp.Body, err = gzip.NewReader(resp.Body)
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
    fallthrough
  case 400:
    fallthrough
  case 502:
    return c.Get(server, bbs, key, reqHeaders)
  default:
    return nil, fmt.Errorf("unknown error")
  }
}

// NewClient returns new Client instance
func NewClient(appKey, hmKey string) *Client {
  tr := &http.Transport{
    DisableKeepAlives: true,
    DisableCompression: true,
    Proxy: http.ProxyFromEnvironment,
  }

  return &Client{
    AppKey: appKey,
    HmKey: hmKey,
    SessionMaxAge: 6 * time.Hour,
    Client: &http.Client{Transport: tr},
  }
}
