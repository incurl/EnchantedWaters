package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// fillRect fills a rectangle with a colour.
//
// Ebiten v2.10 draws shapes through the ebitenutil helpers, which build
// a small triangle batch for every call. The UI draws a limited number of
// shapes per frame, so the cost is acceptable; the hot paths (the map)
// use direct pixel writes instead.
func fillRect(screen *ebiten.Image, x, y, w, h int, c color.Color) {
	if w <= 0 || h <= 0 {
		return
	}
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), c)
}

// DrawPanel draws a bordered panel with a filled background.
//
// The border is drawn as a double line, which is the classic look of the
// original game's dialog boxes.
func DrawPanel(screen *ebiten.Image, x, y, w, h int, fill color.Color) {
	// The background.
	fillRect(screen, x, y, w, h, fill)
	// The outer border.
	drawRectOutline(screen, x, y, w, h, ColorBorder)
	// The inner border, one pixel inside.
	drawRectOutline(screen, x+2, y+2, w-4, h-4, ColorBorder)
}

// drawRectOutline draws the four edges of a rectangle.
func drawRectOutline(screen *ebiten.Image, x, y, w, h int, c color.Color) {
	if w <= 0 || h <= 0 {
		return
	}
	fillRect(screen, x, y, w, 1, c)
	fillRect(screen, x, y+h-1, w, 1, c)
	fillRect(screen, x, y, 1, h, c)
	fillRect(screen, x+w-1, y, 1, h, c)
}

// DrawShadow draws a simple drop shadow under a rectangle.
//
// The shadow is what makes the panels pop out of the map, as in the
// original game.
func DrawShadow(screen *ebiten.Image, x, y, w, h int) {
	const depth = 3
	sh := color.RGBA{0, 0, 0, 0x60}
	for i := 0; i < depth; i++ {
		fillRect(screen, x+i, y+h+i, w, 1, sh)
		fillRect(screen, x+w+i, y+i, 1, h+i, sh)
	}
}

// DrawVScrollBar draws a vertical scroll bar.
//
// thumbPos and thumbSize are given in the 0..1 range.
func DrawVScrollBar(screen *ebiten.Image, x, y, h int, thumbPos, thumbSize float64) {
	const width = 4
	fillRect(screen, x, y, width, h, ColorPanelLight)
	if thumbSize <= 0 {
		thumbSize = 0.1
	}
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos > 1 {
		thumbPos = 1
	}
	thumbH := int(float64(h) * thumbSize)
	if thumbH < 6 {
		thumbH = 6
	}
	if thumbH > h {
		thumbH = h
	}
	thumbY := y + int(float64(h-thumbH)*thumbPos)
	fillRect(screen, x, thumbY, width, thumbH, ColorBorder)
}

// DrawHBar draws a horizontal gauge, used for the crew condition bars.
func DrawHBar(screen *ebiten.Image, x, y, w, h int, ratio float64, fill color.Color, back color.Color) {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	fillRect(screen, x, y, w, h, back)
	filled := int(float64(w) * ratio)
	if filled > 0 {
		fillRect(screen, x, y, filled, h, fill)
	}
	drawRectOutline(screen, x, y, w, h, ColorBorder)
}

// DrawSelectionMarker draws the highlighted row marker of a menu.
func DrawSelectionMarker(screen *ebiten.Image, x, y int) {
	// A small right-pointing triangle.
	for i := 0; i < 4; i++ {
		fillRect(screen, x+i, y+3-i, 1, 1+i*2, ColorTextBright)
	}
}
