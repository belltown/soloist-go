// Package hub broadcasts Soloist playback state to connected web clients
// over WebSocket.
package hub

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub tracks connected web clients and broadcasts playback state to them.
type Hub struct {
	upgrader websocket.Upgrader

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	latest  []byte
}

// New creates an empty Hub.
func New() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			// Proof-of-concept: the Soloist API itself has no origin
			// checks, so this local demo server does not add any either.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// ServeWS upgrades the request to a WebSocket, registers the client, and
// immediately sends the latest known state.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("hub: upgrade failed: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	latest := h.latest
	h.mu.Unlock()

	if latest != nil {
		if err := conn.WriteMessage(websocket.TextMessage, latest); err != nil {
			h.remove(conn)
			return
		}
	}

	// This client doesn't send anything meaningful; just watch for close.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.remove(conn)
			return
		}
	}
}

// Broadcast sends data to every connected web client and remembers it as
// the latest state for newly connecting clients.
func (h *Hub) Broadcast(data []byte) {
	h.mu.Lock()
	h.latest = data
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
	}
	h.mu.Unlock()

	for _, conn := range clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.remove(conn)
		}
	}
}

func (h *Hub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
}
