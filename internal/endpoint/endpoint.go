// Package endpoint records the last local HTTP port for native clients.
package endpoint

import (
	"encoding/json"
	"errors"
	"github.com/zelport/zelport/internal/privatefile"
	"github.com/zelport/zelport/internal/userpaths"
	"os"
	"path/filepath"
)

// Path is independent of any explicit gateway configuration path.
func Path() (string, error) { return userpaths.Endpoint() }

// Publish atomically replaces a port hint. It is not a process liveness record.
func Publish(path string, port int) error {
	if port < 1 || port > 65535 {
		return errors.New("invalid endpoint port")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := privatefile.CreateTemp(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := json.NewEncoder(f).Encode(struct {
		Port int `json:"port"`
	}{port}); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return privatefile.Replace(f.Name(), path)
}
