// Package config keeps the list of servers in a file and their session
// tokens in the Keychain.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type ServerID string

func NewServerID() ServerID {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return ServerID(hex.EncodeToString(b))
}

// Server is a server the user added. Its token is not here.
type Server struct {
	ID   ServerID `json:"id"`
	Name string   `json:"name"`
	URL  string   `json:"url"`
}

type File struct {
	Servers []Server `json:"servers"`
	Current ServerID `json:"current,omitempty"`
	// Last remembers the last team and channel per server.
	Last map[ServerID]Place `json:"last,omitempty"`
	// Theme names the look chosen in Settings; empty follows the system.
	Theme string `json:"theme,omitempty"`
}

type Place struct {
	Team    string `json:"team,omitempty"`
	Channel string `json:"channel,omitempty"`
}

// Load reads the file, or returns an empty one if there is none.
func Load(path string) (File, error) {
	var f File
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	err = json.Unmarshal(b, &f)
	return f, err
}

// Save writes the file atomically.
func Save(path string, f File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Secrets stores session tokens: the Keychain in the app, a map in tests.
type Secrets interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}

func TokenKey(id ServerID) string { return "session-" + string(id) }
