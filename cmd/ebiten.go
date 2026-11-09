package main

import (
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/peter/enchantedwaters/internal/ui"
)

// ebitenSetWindowProperties configures the game window.
//
// The window uses an integer scaling factor and nearest-neighbour
// filtering, so that the pixel art stays crisp at every window size.
func ebitenSetWindowProperties() {
	ebiten.SetWindowSize(ui.LogicalWidth*2, ui.LogicalHeight*2)
	ebiten.SetWindowTitle("Enchanted Waters")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(false)
	ebiten.SetVsyncEnabled(!headless())
	// The FPS is capped at 60: the simulation is turn based, so a higher
	// frame rate would only burn CPU time.
	ebiten.SetTPS(60)
}

// headless reports whether the game runs without a display.
//
// The vertical synchronisation of a virtual frame buffer never fires, so
// the game would wait for it forever; the environment variable turns it
// off for the development screenshots, which are taken on a machine
// without a screen.
func headless() bool {
	return os.Getenv("ENCHANTED_NO_VSYNC") != ""
}

// runEbiten runs the main loop of the game.
func runEbiten(game *ui.Game) error {
	return ebiten.RunGame(game)
}
