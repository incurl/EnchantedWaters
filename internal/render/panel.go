package render

import (
	"image/color"
)

// fillRect fills a rectangle of the canvas with a colour.
func fillRect(c *Canvas, x, y, w, h int, col color.Color) {
	if w <= 0 || h <= 0 {
		return
	}
	cr, cg, cb, ca := col.RGBA()
	r8, g8, b8, a8 := uint8(cr>>8), uint8(cg>>8), uint8(cb>>8), uint8(ca>>8)
	// The rectangle is clipped to the canvas.
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if x+w > c.width {
		w = c.width - x
	}
	if y+h > c.height {
		h = c.height - y
	}
	if w <= 0 || h <= 0 {
		return
	}
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			i := c.buf.PixOffset(col, row)
			c.buf.Pix[i] = r8
			c.buf.Pix[i+1] = g8
			c.buf.Pix[i+2] = b8
			c.buf.Pix[i+3] = a8
		}
	}
	c.uploaded = true
}

// DrawFrame draws a one pixel frame around a rectangle.
//
// The frame sets a picture of the original game apart from the panel
// behind it.
func DrawFrame(c *Canvas, x, y, w, h int, clr color.Color) {
	if c == nil || w <= 0 || h <= 0 {
		return
	}
	for i := 0; i < w; i++ {
		c.Set(x+i, y, clr)
		c.Set(x+i, y+h-1, clr)
	}
	for i := 0; i < h; i++ {
		c.Set(x, y+i, clr)
		c.Set(x+w-1, y+i, clr)
	}
}

// DrawPanel draws a bordered panel with a filled background.
//
// The border is drawn as a double line, which is the classic look of the
// original game's dialog boxes.
func DrawPanel(c *Canvas, x, y, w, h int, fill color.Color) {
	// The background.
	fillRect(c, x, y, w, h, fill)
	// The outer border.
	drawRectOutline(c, x, y, w, h, ColorBorder)
	// The inner border, one pixel inside.
	drawRectOutline(c, x+2, y+2, w-4, h-4, ColorBorder)
}

// drawRectOutline draws the four edges of a rectangle.
func drawRectOutline(canvas *Canvas, x, y, w, h int, c color.Color) {
	if w <= 0 || h <= 0 {
		return
	}
	fillRect(canvas, x, y, w, 1, c)
	fillRect(canvas, x, y+h-1, w, 1, c)
	fillRect(canvas, x, y, 1, h, c)
	fillRect(canvas, x+w-1, y, 1, h, c)
}

// DrawShadow draws a simple drop shadow under a rectangle.
//
// The shadow is what makes the panels pop out of the map, as in the
// original game.
func DrawShadow(c *Canvas, x, y, w, h int) {
	const depth = 3
	sh := color.RGBA{0, 0, 0, 0x60}
	for i := 0; i < depth; i++ {
		// The shadow is cast to the right and to the bottom of the
		// panel.
		fillRect(c, x+i, y+h+i, w, 1, sh)
		fillRect(c, x+w+i, y+i, 1, h, sh)
	}
}

// DrawVScrollBar draws a vertical scroll bar.
//
// thumbPos and thumbSize are given in the 0..1 range.
func DrawVScrollBar(c *Canvas, x, y, h int, thumbPos, thumbSize float64) {
	const width = 4
	fillRect(c, x, y, width, h, ColorPanelLight)
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
	fillRect(c, x, thumbY, width, thumbH, ColorBorder)
}

// DrawHBar draws a horizontal gauge, used for the crew condition bars.
func DrawHBar(c *Canvas, x, y, w, h int, ratio float64, fill color.Color, back color.Color) {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	fillRect(c, x, y, w, h, back)
	filled := int(float64(w) * ratio)
	if filled > 0 {
		fillRect(c, x, y, filled, h, fill)
	}
	drawRectOutline(c, x, y, w, h, ColorBorder)
}

// DrawSelectionMarker draws the highlighted row marker of a menu.
func DrawSelectionMarker(c *Canvas, x, y int) {
	// A small right-pointing triangle.
	for i := 0; i < 4; i++ {
		fillRect(c, x+i, y+3-i, 1, 1+i*2, ColorTextBright)
	}
}

// DimCanvas covers the canvas with a translucent dark layer.
//
// It is used to push the inactive popups of the stack into the
// background.
func DimCanvas(c *Canvas) {
	fillRect(c, 0, 0, c.width, c.height, color.RGBA{0, 0, 0, 0x70})
}
