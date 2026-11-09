package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/asset"
	"github.com/peter/enchantedwaters/internal/render"
)

// TownPopup shows the town of a port as the original game draws it.
//
// The town is the map of the port: a grid of tiles that the artists of the
// original game laid out, one map for every port. The view is a window on
// that map, and the player scrolls it with the arrow keys.
//
// The popup is only opened when the pictures of the original game were
// read; without them the port menu is shown on its own.
type TownPopup struct {
	// ui is the parent interface.
	ui *UI
	// portID is the port whose town is shown.
	portID string
	// town is the town, already assembled out of its tiles.
	town *render.Sprite
	// scrollX and scrollY are the position of the window on the town, in
	// pixels.
	scrollX, scrollY int
}

// TownTile is the size of a tile of a town in pixels.
const TownTile = 16

// NewTownPopup creates the view of the town of a port.
//
// The function returns nil when the town of the port could not be built
// out of the pictures of the original game.
func NewTownPopup(u *UI, portID string) *TownPopup {
	town := u.BuildTown(portID)
	if town == nil {
		return nil
	}
	p := &TownPopup{ui: u, portID: portID, town: town}
	// The view opens on the town, which is the part of the map with the
	// most houses and roads.
	rows := LogicalHeight / TownTile
	p.scrollY = asset.PortMapFocus(u.portMap(portID), rows) * TownTile
	p.clamp()
	return p
}

// Title implements Popup.
func (p *TownPopup) Title() string {
	if port := p.ui.cfg.Port(p.portID); port != nil {
		return port.Name
	}
	return "Town"
}

// clamp keeps the window inside the town.
func (p *TownPopup) clamp() {
	maxX := p.town.Width - LogicalWidth
	if maxX < 0 {
		maxX = 0
	}
	maxY := p.town.Height - LogicalHeight
	if maxY < 0 {
		maxY = 0
	}
	p.scrollX = clampInt(p.scrollX, 0, maxX)
	p.scrollY = clampInt(p.scrollY, 0, maxY)
}

// Update implements Popup.
func (p *TownPopup) Update() bool {
	const step = 32
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		p.scrollX -= step
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		p.scrollX += step
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		p.scrollY -= step
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		p.scrollY += step
	}
	p.clamp()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return false
	}
	return true
}

// Draw implements Popup.
func (p *TownPopup) Draw(canvas *render.Canvas) {
	// The town fills the screen, clipped to the window of the view.
	for y := 0; y < LogicalHeight; y++ {
		for x := 0; x < LogicalWidth; x++ {
			index := p.town.At(p.scrollX+x, p.scrollY+y)
			if int(index) >= len(p.town.Palette) {
				continue
			}
			c := p.town.Palette[index]
			canvas.Set(x, y, colourOf(c))
		}
	}

	// The name of the town, on a band at the top of the screen.
	render.DrawPanel(canvas, 0, 0, LogicalWidth, 18, render.ColorPanel)
	render.DrawText(canvas, p.Title(), 8, 5, render.ColorTextBright)

	// The keys of the view, on a band at the bottom.
	render.DrawPanel(canvas, 0, LogicalHeight-16, LogicalWidth, 16, render.ColorPanel)
	render.DrawText(canvas, "arrow keys: look around    enter: leave the town",
		8, LogicalHeight-13, render.ColorTextDim)
}

// colourOf converts a colour of a palette of the original game into a
// colour of the canvas.
func colourOf(c [3]uint8) color.RGBA {
	return color.RGBA{c[0], c[1], c[2], 0xff}
}

// clampInt limits a value to a range.
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
