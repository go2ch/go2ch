package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"

	"github.com/go2ch/go2ch"
)

var (
	baseURL = flag.String("base", "https://api.2ch.net", "2ch API base URL")
	appKey  = flag.String("appkey", "", "2ch API appkey")
	hmKey   = flag.String("hmkey", "", "2ch API hmkey")
	addr    = flag.String("addr", ":8080", "listening address")
	roninID = flag.String("id", "", "Ronin login ID")
	roninPW = flag.String("pw", "", "Ronin login password")

	datURL = regexp.MustCompile(`^http://(\w+)\.(?:2ch\.net|bbspink\.com)/(\w+)/dat/(\d+)\.dat`)
)

func main() {
	flag.Parse()

	if *appKey == "" || *hmKey == "" {
		fmt.Println("no api key")
		flag.Usage()
		os.Exit(1)
	}

	api := go2ch.NewClient(*appKey, *hmKey)
	api.BaseURL = *baseURL

	if *roninID != "" && *roninPW != "" {
		api.Auth(*roninID, *roninPW)
	}

	proxy := &httputil.ReverseProxy{Director: func(req *http.Request) {}}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		m := datURL.FindStringSubmatch(req.URL.String())
		if m == nil {
			proxy.ServeHTTP(w, req)
			return
		}

		headers := make(map[string]string)

		if v := req.Header.Get("If-Modified-Since"); v != "" {
			headers["If-Modified-Since"] = v
		}

		if v := req.Header.Get("Range"); v != "" {
			headers["Range"] = v
		} else if req.Header.Get("Accept-Encoding") != "" {
			headers["Accept-Encoding"] = "gzip"
		}

		resp, err := api.Get(m[1], m[2], m[3], headers)
		if err != nil {
			switch err.Error() {
			case "not found/invalid range request":
				if headers["Range"] != "" {
					w.WriteHeader(416)
				} else {
					w.WriteHeader(302)
				}
			case "thread dat-out":
				w.WriteHeader(302)
			default:
				w.WriteHeader(500)
			}

			return
		}

		for k := range resp.Header {
			w.Header().Set(k, resp.Header.Get(k))
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		resp.Body.Close()
	})

	fmt.Println("listening on", *addr)
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		fmt.Println(err)
	}
}
