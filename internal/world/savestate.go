package world

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peter/enchantedwaters/internal/save"
)

// SaveDir returns the directory the save files are written to.
//
// The directory is next to the configuration tables, which keeps the
// saves with the game data.
func (w *World) SaveDir() string {
	return filepath.Join("saves")
}

// Save writes the current session into a file.
func (w *World) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create save directory: %w", err)
	}
	f := w.Snapshot(0xC0FFEE)
	if err := save.Write(path, f); err != nil {
		return err
	}
	w.Log("game saved to %s", path)
	return nil
}

// Load restores a session from a file.
//
// The configuration must already be loaded; the load only replaces the
// dynamic state.
func (w *World) Load(path string) error {
	f, err := save.Read(path)
	if err != nil {
		return err
	}
	return w.Restore(f)
}

// ListSaves returns the save files of the save directory.
func (w *World) ListSaves() ([]string, error) {
	return save.List(w.SaveDir())
}
