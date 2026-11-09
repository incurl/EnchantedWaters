package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/render"
	"github.com/peter/enchantedwaters/internal/world"
)

// This file implements the actions that the player can trigger directly
// on the map, without opening a port screen.

// handleActionKeys handles the global action keys of the map view.
//
//	P     open the port screen when docked
//	C     follow the camera
//	Space centre the camera on the fleet
//	F1    open the system menu
func (u *UI) handleActionKeys() {
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		u.openPortScreen()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		u.followCamera = !u.followCamera
		if u.followCamera {
			u.Message("camera follows the ship")
		} else {
			u.Message("camera released")
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		u.followCamera = true
		u.updateCamera()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		u.popups.Push(NewSystemPopup(u))
	}
}

// openPortScreen opens the menu of the port the fleet is docked at.
//
// It reports whether a screen was opened.
func (u *UI) openPortScreen() bool {
	fleetID := u.world.PlayerEntity()
	portID := u.world.ArrivedAtPort(fleetID)
	if portID == "" {
		u.Message("the ship is at sea")
		return false
	}
	u.popups.Push(NewPortPopup(u, portID))
	return true
}

// drawMapHints draws the key hints of the map view.
//
// The hints are shown in the corner of the screen, so that the player
// always knows which keys are available.
func (u *UI) drawMapHints(screen *ebiten.Image) {
	hint := "arrows: pan  P: port  L: log  1-4: speed"
	render.DrawText(screen, hint, 8, LogicalHeight-66, render.ColorTextDim)
}

// portUnderCursor returns the port that is close to the mouse position.
//
// The search radius is expressed in screen pixels, which keeps the
// interaction comfortable at any zoom level.
func (u *UI) portUnderCursor(mx, my int) string {
	const radius = 10.0
	best := ""
	bestDist := radius
	for _, id := range u.cfg.PortOrder {
		p := u.cfg.Port(id)
		if p == nil {
			continue
		}
		dx := p.X - u.camX - float64(mx)
		dy := p.Y - u.camY - float64(my)
		d := dx*dx + dy*dy
		if d < bestDist*bestDist {
			bestDist = sqrt(d)
			best = id
		}
	}
	return best
}

// sqrt returns the square root of a float.
func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	// Newton's method converges quickly for the small numbers used here.
	x := v
	for i := 0; i < 8; i++ {
		x = 0.5 * (x + v/x)
	}
	return x
}

// factionPlayer is the faction ID of the player.
const factionPlayer = world.FactionPlayer
