package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peter/enchantedwaters/internal/render"
)

// TestScreenshotStopsTheGame verifies that the game leaves once it has
// written the screenshot.
//
// The screenshot is a request of a tool, and the game is not a tool: a
// game that waits for a key that no one will press runs forever. On a
// machine without a vertical synchronisation the loop of the library spins
// as fast as it can, and it burns every core of the machine.
//
// The test drives the two steps of the screenshot: the file is written, and
// the game is told to stop.
func TestScreenshotStopsTheGame(t *testing.T) {
	u := testUI(t)
	g := NewGame(u.world)
	if g == nil {
		t.Fatal("NewGame returned nothing")
	}

	// The game runs as long as no screenshot was asked for.
	if g.quit {
		t.Error("the game wants to stop before it was asked to")
	}

	// The canvas is created by the first draw, and the screenshot is
	// taken from it.
	g.ui.canvas = render.NewCanvas(LogicalWidth, LogicalHeight)
	g.ui.drawMap(g.ui.canvas)

	// The screenshot is written into a file of the test.
	path := filepath.Join(t.TempDir(), "shot.png")
	g.SetScreenshot(path)
	g.saveShot()

	// The file is there.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the screenshot was not written: %v", err)
	}
	// The game wants to stop.
	if !g.quit {
		t.Error("the game does not want to stop after the screenshot")
	}
	// The path is cleared, so that the screenshot is not written again.
	if g.shotPath != "" {
		t.Error("the path of the screenshot was not cleared")
	}
}

// TestSaveShotWithoutAPathDoesNothing verifies that the game does not stop
// when no screenshot was asked for.
func TestSaveShotWithoutAPathDoesNothing(t *testing.T) {
	u := testUI(t)
	g := NewGame(u.world)
	g.saveShot()
	if g.quit {
		t.Error("the game wants to stop without a screenshot")
	}
}
