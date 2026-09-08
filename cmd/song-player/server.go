package main

import (
	_ "embed"
	"log"
	"net"
	"net/http"
	"net/url"
)

//go:embed yt.html
var ytHTML []byte

func startFileServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/yt.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(ytHTML)
	})
	go func() {
		log.Println("serving yt.html on :8000")
		log.Fatal(http.ListenAndServe("0.0.0.0:8000", mux))
	}()
}

func buildBrowserSourceURL(host, videoID string) string {
	u := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, "8000"),
		Path:   "/yt.html",
	}
	q := u.Query()
	q.Set("v", videoID)
	u.RawQuery = q.Encode()
	return u.String()
}
