package ui

import (
	"math"

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
		u.playEffect(effectAccept)
		u.openPortScreen()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		u.playEffect(effectClick)
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
	// The zoom: minus and plus, and M for the whole world.
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		u.setZoom(u.zoom * 2)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		u.setZoom(u.zoom / 2)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		// The whole world on one screen.
		w, h := u.mapRenderer.Size()
		fit := math.Max(float64(w)/LogicalWidth, float64(h)/LogicalHeight)
		if u.zoom > fit*1.05 {
			u.setZoom(fit)
			u.followCamera = false
			u.camX, u.camY = 0, 0
		} else {
			u.setZoom(1)
			u.followCamera = true
		}
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
func (u *UI) drawMapHints(canvas *render.Canvas) {
	hint := "arrows: pan  +/-: zoom  M: world  P: port  L: log"
	render.DrawText(canvas, hint, 8, LogicalHeight-66, render.ColorTextDim)
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

// setZoom sets the zoom of the map and keeps it in range.
//
// A zoom of one shows the map at its own resolution; a larger value shows
// less of it, a smaller value shows more.
func (u *UI) setZoom(z float64) {
	if z < 0.05 {
		z = 0.05
	}
	if z > 4 {
		z = 4
	}
	// The camera stays on the same map point when the zoom changes.
	cx := u.camX + LogicalWidth/2*u.zoom
	cy := u.camY + LogicalHeight/2*u.zoom
	u.zoom = z
	u.camX = cx - LogicalWidth/2*z
	u.camY = cy - LogicalHeight/2*z
	u.clampCamera()
}
