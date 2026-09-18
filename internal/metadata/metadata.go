// Package metadata looks up Spotify track metadata from the local SQLite3
// database populated from spotify_track_metadata.
package metadata

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

const trackURIPrefix = "spotify:track:"

// Track is the metadata displayed for a currently-playing or queued track.
type Track struct {
	SpotifyID     string `json:"spotify_id,omitempty"`
	TrackName     string `json:"track_name"`
	ArtistName    string `json:"artist_name,omitempty"`
	Duration      int64  `json:"duration,omitempty"`
	Dance         string `json:"dance,omitempty"`
	Tempo         int64  `json:"tempo,omitempty"`
	OverrideTempo int64  `json:"override_tempo,omitempty"`
}

// Store queries the spotify_track_metadata table.
type Store struct {
	db *sql.DB
}

// Open opens the SQLite3 database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// SpotifyIDFromURI extracts the Spotify track ID from a Soloist "spotify:track:..."
// URI. It returns false for any other entity type.
func SpotifyIDFromURI(uri string) (string, bool) {
	return strings.CutPrefix(uri, trackURIPrefix)
}

// Lookup returns metadata for the track identified by uri, using the
// idx_spotify_id_unique index on spotify_track_metadata.spotify_id. When the
// track isn't a Spotify track or isn't found in the database, it returns a
// Track populated with fallbackName and fallbackArtist instead.
func (s *Store) Lookup(uri, fallbackName, fallbackArtist string) Track {
	track := Track{TrackName: fallbackName, ArtistName: fallbackArtist}

	id, ok := SpotifyIDFromURI(uri)
	if !ok || id == "" {
		return track
	}
	track.SpotifyID = id

	row := s.db.QueryRow(
		`SELECT track_name, artist_name, duration, dance, tempo, override_tempo
		 FROM spotify_track_metadata
		 WHERE spotify_id = ?`,
		id,
	)

	var (
		trackName     string
		artistName    string
		duration      int64
		dance         sql.NullString
		tempo         int64
		overrideTempo int64
	)
	if err := row.Scan(&trackName, &artistName, &duration, &dance, &tempo, &overrideTempo); err != nil {
		// Not found (or a query error): keep the fallback name/artist and Spotify ID.
		return track
	}

	track.TrackName = trackName
	track.ArtistName = artistName
	track.Duration = duration
	track.Dance = dance.String
	track.Tempo = tempo
	track.OverrideTempo = overrideTempo
	return track
}
