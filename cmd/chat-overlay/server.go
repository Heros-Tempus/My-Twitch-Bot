package main

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

//go:embed chat.html
var chatHTML []byte

func (a *App) serveHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(chatHTML)
}

func (a *App) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("Failed to accept websocket: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer c.Close(websocket.StatusInternalError, "closing connection")

	log.Println("OBS connected to overlay!")

	a.ClientsMu.Lock()
	a.Clients[c] = cancel
	a.ClientsMu.Unlock()

	<-ctx.Done()

	a.ClientsMu.Lock()
	delete(a.Clients, c)
	a.ClientsMu.Unlock()
	log.Println("OBS disconnected")
}

func (a *App) broadcastMessage(msgBytes []byte) {
	a.ClientsMu.Lock()
	defer a.ClientsMu.Unlock()

	for client, cancel := range a.Clients {
		ctx, writeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := client.Write(ctx, websocket.MessageText, msgBytes)
		writeCancel()

		if err != nil {
			log.Printf("Write error to OBS client, removing: %v", err)
			cancel()
		}
	}
}
