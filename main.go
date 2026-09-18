// Command soloist-go is a minimal proof-of-concept web server that mirrors
// Spotify Soloist playback state (currently-playing track and upcoming
// queue) to browser clients over WebSocket.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"soloist-go/internal/dotenv"
	"soloist-go/internal/hub"
	"soloist-go/internal/metadata"
	"soloist-go/internal/soloist"
)

//go:embed web/*
var webFS embed.FS

// state is the JSON payload pushed to browser clients: the currently-playing
// track and the upcoming queue, each enriched with database metadata, plus
// the current playback position anchor.
type state struct {
	NowPlaying metadata.Track   `json:"now_playing"`
	Queue      []metadata.Track `json:"queue"`
	Position   soloist.Position `json:"position"`
}

func main() {
	soloistAddr := flag.String("soloist-ws", "", "host:port of the Soloist WebSocket API (default: 127.0.0.1:$SOLOIST_PORT)")
	listenAddr := flag.String("listen", "", "host:port for this web server to listen on (default: 0.0.0.0:$WEB_PORT)")
	envFile := flag.String("env-file", "", "path to .env file (default: .env alongside the executable)")
	dbPath := flag.String("db-path", "", "path to the SQLite3 track metadata database (default: $DB_PATH from .env)")
	flag.Parse()

	if err := dotenv.Load(envFilePath(*envFile)); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	if *dbPath == "" {
		*dbPath = os.Getenv("DB_PATH")
	}
	if *dbPath == "" {
		log.Fatal("DB_PATH is not set (define it in .env or pass --db-path)")
	}
	if *soloistAddr == "" {
		*soloistAddr = "127.0.0.1:" + requiredEnv("SOLOIST_PORT")
	}
	if *listenAddr == "" {
		*listenAddr = "0.0.0.0:" + requiredEnv("WEB_PORT")
	}

	store, err := metadata.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database %s: %v", *dbPath, err)
	}
	defer store.Close()

	h := hub.New()

	client := soloist.NewClient(*soloistAddr, func(s soloist.State) {
		queue := make([]metadata.Track, len(s.Queue))
		for i, t := range s.Queue {
			queue[i] = store.Lookup(t.URI, t.Name, t.Artist)
		}

		data, err := json.Marshal(state{
			NowPlaying: store.Lookup(s.NowPlaying.URI, s.NowPlaying.Name, s.NowPlaying.Artist),
			Queue:      queue,
			Position:   s.Position,
		})
		if err != nil {
			log.Printf("encode state: %v", err)
			return
		}
		h.Broadcast(data)
	})
	go client.Run(context.Background())

	webRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(webRoot)))
	mux.HandleFunc("/ws", h.ServeWS)

	log.Printf("listening on %s (soloist ws: %s)", *listenAddr, *soloistAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, mux))
}

// envFilePath returns explicit if set, otherwise ".env" next to the running
// executable, so the file is found regardless of the caller's directory.
func envFilePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	exe, err := os.Executable()
	if err != nil {
		return ".env"
	}
	return filepath.Join(filepath.Dir(exe), ".env")
}

func requiredEnv(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	log.Fatalf("%s is not set (define it in .env or pass the corresponding address flag)", name)
	return ""
}
