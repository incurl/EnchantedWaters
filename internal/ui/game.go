package ui

import (
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/world"
)

// Game implements the Ebiten game interface.
//
// The game owns the simulation and the user interface. The simulation is
// advanced from the Update method, at a rate that follows the selected
// game speed:
//
//	ticksPerSecond = DayLengthRealSeconds / speed
//
// The rendering happens in Draw, which is completely decoupled from the
// simulation.
type Game struct {
	// world is the dynamic simulation state.
	world *world.World
	// ui is the user interface.
	ui *UI
	// acc accumulates the real time since the last simulation tick.
	acc float64
	// tickInterval is the real time between two simulation ticks.
	tickInterval float64
	// paused reports whether the simulation is paused.
	paused bool
	// debug is the development debug panel. It is a stub in the release
	// builds.
	debug *DebugPanel
	// shotPath is the file the canvas is written into; it is empty when
	// no screenshot was requested.
	shotPath string
}

// NewGame creates the game of a session.
func NewGame(w *world.World) *Game {
	g := &Game{world: w}
	g.ui = New(w)
	g.debug = NewDebugPanel(g.ui)
	g.updateTickInterval()
	return g
}

// UI returns the user interface of the game.
func (g *Game) UI() *UI { return g.ui }

// SetScreenshot asks the game to write the canvas into a file.
//
// The screenshot is taken at the end of the next frame.
func (g *Game) SetScreenshot(path string) { g.shotPath = path }

// updateTickInterval recomputes the interval between two ticks.
//
// The interval is derived from the day length and the speed:
//
//	tickInterval = DayLengthRealSeconds / speed
//
// When the game is paused, the interval becomes infinite.
func (g *Game) updateTickInterval() {
	speed := g.world.Speed()
	if speed <= 0 {
		g.tickInterval = 0
		g.paused = true
		return
	}
	g.paused = false
	dayLength := g.world.Config().Params.DayLengthRealSeconds
	if dayLength <= 0 {
		dayLength = 6
	}
	g.tickInterval = dayLength / float64(speed)
}

// Update implements ebiten.Game.
func (g *Game) Update() error {
	// The tick interval follows the speed, which the player can change at
	// any moment.
	if g.tickInterval != 0 && g.world.Speed() != 0 {
		g.updateTickInterval()
	}

	if err := g.ui.Update(); err != nil {
		return err
	}
	if err := g.debug.Update(); err != nil {
		return err
	}
	g.handleSaveKeys()

	if !g.paused && g.tickInterval > 0 {
		g.acc += 1.0 / 60.0
		for g.acc >= g.tickInterval {
			g.acc -= g.tickInterval
			g.world.Tick()
		}
	}
	return nil
}

// handleSaveKeys handles the quick save and quick load keys.
//
//	F5 saves the session, F9 loads the last save.
func (g *Game) handleSaveKeys() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		path := filepath.Join(g.world.SaveDir(), "quicksave.json")
		if err := g.world.Save(path); err != nil {
			g.ui.Message("save failed: %v", err)
		} else {
			g.ui.Message("game saved")
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		path := filepath.Join(g.world.SaveDir(), "quicksave.json")
		if _, err := os.Stat(path); err != nil {
			g.ui.Message("no save file yet")
			return
		}
		if err := g.world.Load(path); err != nil {
			g.ui.Message("load failed: %v", err)
		} else {
			g.ui.Message("game loaded")
		}
	}
}

// Draw implements ebiten.Game.
func (g *Game) Draw(screen *ebiten.Image) {
	g.ui.Draw(screen)
	g.debug.Draw(screen)
	g.saveShot()
}

// saveShot writes the canvas into a PNG file.
//
// The screenshot is taken from the canvas, not from the window: the
// capture of a window depends on the window manager, while the canvas
// holds exactly what was drawn.
func (g *Game) saveShot() {
	if g.shotPath == "" {
		return
	}
	if err := g.ui.Canvas().SavePNG(g.shotPath); err != nil {
		g.ui.Message("the screenshot failed: %v", err)
	}
	g.shotPath = ""
}

// Layout implements ebiten.Game.
//
// The logical screen size is fixed; the window is scaled by an integer
// factor with nearest-neighbour sampling, which preserves the retro look.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return LogicalWidth, LogicalHeight
}
