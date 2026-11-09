//go:build !debugui

package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// The release build has no debug panel.
//
// The two files implement the same interface, so that the game loop does
// not need any build tag of its own.

// DebugPanel is the stub of the debug panel for the release builds.
type DebugPanel struct {
	// ui is the parent interface.
	ui *UI
}

// NewDebugPanel creates the stub of the debug panel.
func NewDebugPanel(u *UI) *DebugPanel {
	return &DebugPanel{ui: u}
}

// Update does nothing in the release builds.
func (d *DebugPanel) Update() error { return nil }

// Draw does nothing in the release builds.
func (d *DebugPanel) Draw(screen *ebiten.Image) {}
