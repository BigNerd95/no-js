// Track page visit duration using persistent background connection on the
// favicon of the page.

// Original project: https://github.com/wybiral/tracker

package main

import (
	"log"
	"net/http"
	"time"
)

// Time between updates to keep alive
// Shorter: increases resolution and network noise
// Longer: does the opposite of both
const timeout = time.Second

func main() {
	http.HandleFunc("/favicon.ico", favicon)
	http.HandleFunc("/page1", page("Page 1"))
	http.HandleFunc("/page2", page("Page 2"))
	http.HandleFunc("/", page("Index"))
	log.Println("Serving on :8080")
	http.ListenAndServe(":8080", nil)
}

// Handler /favicon.ico requests
func favicon(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	t0 := time.Now()
	page := r.Header.Get("referer")
	log.Println("IN", page)
	// When connection closes this deferred will call
	defer func() {
		t1 := time.Now()
		log.Println("OUT", page, t1.Sub(t0))
	}()
	// Disable caching
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	// Setup chunked transfer encoding
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	for {
		time.Sleep(timeout)
		_, err := w.Write([]byte{0})
		if err != nil {
			return
		}
		flusher.Flush()
	}
}

// Create page handler with title
func page(title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html>
	<head>
		<title>` + title + `</title>
	</head>
	<body>
		<a href="/">Index</a> -
		<a href="/page1">Page 1</a> -
		<a href="/page2">Page 2</a>
	</body>
</html>`))
	}
}
