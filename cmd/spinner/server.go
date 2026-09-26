package main

import (
	_ "embed"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

//go:embed spinner.html
var spinnerHTML []byte

func (a *App) startFileServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/spinner.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(spinnerHTML)
	})

	mux.HandleFunc("/api/spin", a.handleSpinTrigger)

	mux.HandleFunc("/api/spinner/complete", a.handleSpinnerComplete)

	go func() {
		log.Println("serving spinner.html on :8082")
		log.Fatal(http.ListenAndServe("0.0.0.0:8082", mux))
	}()
}

func buildSpinnerURL(host string, items []string, winnerIndex int) string {
	u := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, "8082"),
		Path:   "/spinner.html",
	}
	q := u.Query()
	q.Set("items", strings.Join(items, ","))
	q.Set("index", strconv.Itoa(winnerIndex))
	u.RawQuery = q.Encode()

	log.Printf("Built Spinner URL: %s", u.String())
	log.Printf("Items: %v", items)
	log.Printf("Winner Index: %d", winnerIndex)
	return u.String()
}
