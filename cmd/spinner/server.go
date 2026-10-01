package main

import (
	_ "embed"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
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
	
	secretKey := os.Getenv("HOTKEY_API_SECRET")
	authMiddleware := RequireAuth(secretKey)
	mux.Handle("/api/spin", authMiddleware(http.HandlerFunc(a.handleSpinTrigger)))

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

func RequireAuth(validToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			expectedHeader := "Bearer " + validToken

			if authHeader != expectedHeader {
				log.Printf("Unauthorized access attempt from %s", r.RemoteAddr)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
