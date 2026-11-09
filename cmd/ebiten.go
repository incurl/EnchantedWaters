package main

import (
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
	ebiten.SetVsyncEnabled(true)
	// The FPS is capped at 60: the simulation is turn based, so a higher
	// frame rate would only burn CPU time.
	ebiten.SetTPS(60)
}

// runEbiten runs the main loop of the game.
func runEbiten(game *ui.Game) error {
	return ebiten.RunGame(game)
}
