// Package soloist connects to the Spotify Soloist WebSocket API and tracks
// the currently-playing track and upcoming queue.
package soloist

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const queueLimit = 10

// Entity is the common playback item envelope used by the Soloist API.
type Entity struct {
	URI         string `json:"uri"`
	EntityType  string `json:"entity_type"`
	Decorations struct {
		Identity struct {
			Name string `json:"name"`
		} `json:"identity"`
		Creators []struct {
			Entity Entity `json:"entity"`
		} `json:"creators"`
	} `json:"decorations"`
}

// Name returns the display name, falling back to the URI when unnamed.
func (e Entity) Name() string {
	if e.Decorations.Identity.Name != "" {
		return e.Decorations.Identity.Name
	}
	return e.URI
}

// Artist returns the first creator's display name, or "" when unknown.
func (e Entity) Artist() string {
	if len(e.Decorations.Creators) == 0 {
		return ""
	}
	return e.Decorations.Creators[0].Entity.Decorations.Identity.Name
}

type queueEntry struct {
	Item Entity `json:"item"`
}

type envelope struct {
	Type string `json:"type"`
}

type authStateEvent struct {
	LoggedIn bool `json:"logged_in"`
}

type playbackStateEvent struct {
	Item     Entity   `json:"item"`
	Position Position `json:"position"`
}

type trackChangedEvent struct {
	Item Entity `json:"item"`
}

type queueChangedEvent struct {
	Upcoming []queueEntry `json:"upcoming"`
}

type positionSyncEvent struct {
	Position Position `json:"position"`
}

type wsCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Limit   int    `json:"limit,omitempty"`
}

// Track identifies a playback item as reported by Soloist, before any
// database enrichment.
type Track struct {
	URI    string
	Name   string
	Artist string
}

// Position is a playback position anchor: PositionMs at TimestampMs,
// advancing at Speed (0 when paused or stopped).
type Position struct {
	PositionMs  int64   `json:"position_ms"`
	TimestampMs int64   `json:"timestamp_ms"`
	Speed       float64 `json:"speed"`
}

// State is the playback snapshot reported to onUpdate.
type State struct {
	NowPlaying Track
	Queue      []Track
	Position   Position
}

// Client maintains a connection to the Soloist WebSocket API and reports
// state changes through onUpdate.
type Client struct {
	addr     string
	onUpdate func(State)

	mu    sync.Mutex
	state State
}

// NewClient creates a client that dials the Soloist WebSocket API at addr
// (host:port, no scheme) and invokes onUpdate whenever the tracked state
// changes.
func NewClient(addr string, onUpdate func(State)) *Client {
	return &Client{addr: addr, onUpdate: onUpdate}
}

// Run connects to Soloist and reconnects with a fixed backoff until ctx is
// cancelled.
func (c *Client) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := c.connectOnce(ctx); err != nil {
			log.Printf("soloist: connection error: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (c *Client) connectOnce(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.Dial("ws://"+c.addr, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	log.Printf("soloist: connected at %s", c.addr)

	for ctx.Err() == nil {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		c.handleMessage(conn, data)
	}
	return nil
}

func (c *Client) handleMessage(conn *websocket.Conn, data []byte) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		log.Printf("soloist: decode event: %v", err)
		return
	}

	switch env.Type {
	case "auth_state":
		var ev authStateEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			log.Printf("soloist: decode auth_state: %v", err)
			return
		}
		// The queue is not sent automatically, so request it once logged in.
		if ev.LoggedIn {
			c.requestQueue(conn)
		}

	case "playback_state":
		var ev playbackStateEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			log.Printf("soloist: decode playback_state: %v", err)
			return
		}
		c.setNowPlaying(ev.Item)
		c.setPosition(ev.Position)
		c.requestQueue(conn)

	case "track_changed":
		var ev trackChangedEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			log.Printf("soloist: decode track_changed: %v", err)
			return
		}
		c.setNowPlaying(ev.Item)

	case "position_sync":
		var ev positionSyncEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			log.Printf("soloist: decode position_sync: %v", err)
			return
		}
		c.setPosition(ev.Position)

	case "queue_changed":
		var ev queueChangedEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			log.Printf("soloist: decode queue_changed: %v", err)
			return
		}
		tracks := make([]Track, 0, queueLimit)
		for _, entry := range ev.Upcoming {
			if len(tracks) >= queueLimit {
				break
			}
			tracks = append(tracks, Track{URI: entry.Item.URI, Name: entry.Item.Name(), Artist: entry.Item.Artist()})
		}
		c.setQueue(tracks)
	}
}

func (c *Client) requestQueue(conn *websocket.Conn) {
	cmd := wsCommand{Type: "command", Command: "get_queue", Limit: queueLimit}
	if err := conn.WriteJSON(cmd); err != nil {
		log.Printf("soloist: request queue: %v", err)
	}
}

func (c *Client) setNowPlaying(item Entity) {
	c.mu.Lock()
	c.state.NowPlaying = Track{URI: item.URI, Name: item.Name(), Artist: item.Artist()}
	state := c.state
	c.mu.Unlock()
	c.onUpdate(state)
}

func (c *Client) setQueue(tracks []Track) {
	c.mu.Lock()
	c.state.Queue = tracks
	state := c.state
	c.mu.Unlock()
	c.onUpdate(state)
}

func (c *Client) setPosition(position Position) {
	c.mu.Lock()
	c.state.Position = position
	state := c.state
	c.mu.Unlock()
	c.onUpdate(state)
}
