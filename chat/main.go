// A chat application designed around persistent connection on the main page
// body where updates are rendered as they come in and the connection count is
// updated using CSS.

// Original project: https://github.com/wybiral/noscript-chat

package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Number of updates to keep in history
const historyLimit = 20

// Interval to send single-space ping to keep conntection alive
const pingRate = 1 * time.Second

// Maximum message length
const maxMsgLen = 1024

// Number of buffered messages per connection
const bufferSize = 5

// Leading portion of main page
const pageHead = `<!doctype html>
<html>
<head>
<title>noscript timeline</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" type="text/css" href="/static/style.css">
</head>
<body>
<header>
	<div id="count">Being seen by <span id="nc"></span> connection(s)</div>
	<form method="post">
		<textarea name="msg" placeholder="Start typing..." autofocus></textarea>
		<div><button>Post</button></div>
	</form>
</header>
<main>
`

func main() {
	app := NewApp()
	http.HandleFunc("/", app.handler)
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	log.Println("Serving on :8080")
	http.ListenAndServe(":8080", nil)
}

// Update represents a chat update
type Update struct {
	timestamp string
	message   string
}

// App represents the main application
type App struct {
	chansMutex   sync.RWMutex
	chans        map[chan []byte]struct{}
	historyMutex sync.RWMutex
	history      []*Update
}

// NewApp returns a new *App
func NewApp() *App {
	return &App{
		chans:   make(map[chan []byte]struct{}),
		history: make([]*Update, 0),
	}
}

// append an *Update to the chat log
func (a *App) append(update *Update) {
	a.historyMutex.Lock()
	defer a.historyMutex.Unlock()
	a.history = append(a.history, update)
	if len(a.history) > historyLimit {
		a.history = a.history[len(a.history)-historyLimit:]
	}
}

// sendCount sends the current connection count to all clients
func (a *App) sendCount() {
	fmtstr := "<style>#nc::before{content:\"%d\"}</style>"
	data := []byte(fmt.Sprintf(fmtstr, len(a.chans)))
	a.chansMutex.RLock()
	defer a.chansMutex.RUnlock()
	for ch := range a.chans {
		select {
		case ch <- data:
		default:
			continue
		}
	}
}

// send an *Update by appending it to the chat log and sending to clients
func (a *App) send(update *Update) {
	a.append(update)
	fmtstr := "<div class=\"new\"><p>%s</p><time>%s</time></div>"
	msg := fmt.Sprintf(fmtstr, update.message, update.timestamp)
	data := []byte(msg)
	a.chansMutex.RLock()
	defer a.chansMutex.RUnlock()
	for ch := range a.chans {
		select {
		case ch <- data:
		default:
			continue
		}
	}
}

// sendHistory sends chat log to a client
func (a *App) sendHistory(w http.ResponseWriter) error {
	fmtstr := "<div><p>%s</p><time>%s</time></div>"
	a.historyMutex.RLock()
	defer a.historyMutex.RUnlock()
	for _, update := range a.history {
		msg := fmt.Sprintf(fmtstr, update.message, update.timestamp)
		_, err := w.Write([]byte(msg))
		if err != nil {
			return err
		}
	}
	return nil
}

// addChan adds a client chan to listeners
func (a *App) addChan(ch chan []byte) {
	a.chansMutex.Lock()
	defer a.chansMutex.Unlock()
	a.chans[ch] = struct{}{}
}

// removeChan removes a client chan from listeners
func (a *App) removeChan(ch chan []byte) {
	a.chansMutex.Lock()
	defer a.chansMutex.Unlock()
	delete(a.chans, ch)
}

// handler is main HTTP entry point for requests
func (a *App) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		a.getHandler(w, r)
		return
	} else if r.Method == "POST" {
		a.postHandler(w, r)
		return
	}
}

// getHandler handles main page
func (a *App) getHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	// Create and register connection channel
	ch := make(chan []byte, bufferSize)
	a.addChan(ch)
	defer func() {
		a.removeChan(ch)
		a.sendCount()
	}()
	// Write page head and history
	w.Write([]byte(pageHead))
	err := a.sendHistory(w)
	if err != nil {
		return
	}
	flusher.Flush()
	a.sendCount()
	for {
		select {
		case msg := <-ch:
			_, err = w.Write(msg)
			if err != nil {
				return
			}
		case <-time.After(pingRate):
			_, err := w.Write([]byte{' '})
			if err != nil {
				return
			}
		}
		flusher.Flush()
	}
}

// postHandler handles new chat posts
func (a *App) postHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	msg := r.PostForm.Get("msg")
	if len(msg) > maxMsgLen {
		http.Redirect(w, r, "/", 302)
		return
	}
	msg = template.HTMLEscapeString(msg)
	msg = strings.TrimSpace(msg)
	if len(msg) > 0 {
		timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
		a.send(&Update{timestamp: timestamp, message: msg})
	}
	http.Redirect(w, r, "/", 302)
}
