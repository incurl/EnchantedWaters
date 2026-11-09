package render

import (
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
)

// Canvas is the drawing surface of the interface.
//
// # Why a CPU side buffer?
//
// The interface draws with direct pixel writes (Set), because that is the
// only way to guarantee the exact order of the drawing. Ebitengine,
// however, batches its own draw calls (DrawImage, the vector helpers) and
// flushes them at the end of the frame.
//
// Mixing the two breaks the order: the batched calls would then be painted
// over the pixels that were written in between. The symptom is a popup
// that disappears behind the map.
//
// The canvas therefore solves the problem once: every drawing goes into a
// plain Go image, and the canvas is blitted onto the screen in a single
// call at the very end of the frame. The order of the drawing is then
// always the order of the code.
type Canvas struct {
	// buf holds the pixels of the canvas.
	buf *image.RGBA
	// width / height are the size of the canvas in pixels.
	width, height int
	// screen is the target of the last blit.
	screen *ebiten.Image
	// uploaded marks the buffer as needing an upload to the GPU.
	uploaded bool
}

// NewCanvas creates a drawing surface of the given size.
func NewCanvas(width, height int) *Canvas {
	if width <= 0 {
		width = 640
	}
	if height <= 0 {
		height = 400
	}
	return &Canvas{
		buf:    image.NewRGBA(image.Rect(0, 0, width, height)),
		width:  width,
		height: height,
	}
}

// Size returns the size of the canvas.
func (c *Canvas) Size() (int, int) { return c.width, c.height }

// Image returns the buffer of the canvas.
//
// The buffer is an ordinary Go image: the drawing helpers write into it
// with direct pixel accesses.
func (c *Canvas) Image() *image.RGBA { return c.buf }

// Clear fills the canvas with a colour.
func (c *Canvas) Clear(col color.Color) {
	drawFillRect(c.buf, 0, 0, c.width, c.height, col)
	c.uploaded = true
}

// Set writes a single pixel.
//
// The write is clipped to the canvas.
func (c *Canvas) Set(x, y int, clr color.Color) {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return
	}
	r, g, b, a := clr.RGBA()
	i := c.buf.PixOffset(x, y)
	c.buf.Pix[i] = uint8(r >> 8)
	c.buf.Pix[i+1] = uint8(g >> 8)
	c.buf.Pix[i+2] = uint8(b >> 8)
	c.buf.Pix[i+3] = uint8(a >> 8)
	c.uploaded = true
}

// SavePNG writes the canvas into a PNG file.
//
// It is the reliable way to document the state of the interface: the
// capture of the window depends on the window manager, while the canvas
// holds exactly what was drawn.
func (c *Canvas) SavePNG(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, c.buf)
}

// MarkDirty marks the canvas as needing an upload to the GPU.
func (c *Canvas) MarkDirty() { c.uploaded = true }

// Present uploads the canvas to the screen and draws it.
//
// The upload happens only when the buffer has changed, which keeps the
// cost of an idle frame very low.
func (c *Canvas) Present(screen *ebiten.Image) {
	if c.screen == nil || c.screen.Bounds().Dx() != c.width || c.screen.Bounds().Dy() != c.height {
		c.screen = ebiten.NewImageWithOptions(
			image.Rect(0, 0, c.width, c.height),
			&ebiten.NewImageOptions{Unmanaged: true})
		c.uploaded = true
	}
	if c.uploaded {
		c.screen.WritePixels(c.buf.Pix)
		c.uploaded = false
	}
	// The single draw call of the frame.
	screen.DrawImage(c.screen, nil)
}

// DrawText draws a string into the canvas.
func (c *Canvas) DrawText(s string, x, y int, clr color.Color) int {
	return drawText(c, s, x, y, clr)
}

// TextWidth returns the pixel width of a string in the bitmap font.
func (c *Canvas) TextWidth(s string) int { return TextWidth(s) }

// drawText writes the glyphs of a string into the canvas.
func drawText(c *Canvas, s string, x, y int, clr color.Color) int {
	r, g, b, a := clr.RGBA()
	alpha := uint8(a >> 8)
	if alpha == 0 {
		return 0
	}
	cursor := x
	for _, ch := range s {
		if ch == '\n' {
			cursor = x
			y += LineHeight
			continue
		}
		cursor += drawGlyph(c, ch, cursor, y, uint8(r>>8), uint8(g>>8), uint8(b>>8), alpha)
	}
	return cursor - x
}

// drawGlyph draws a single rune into the canvas.
func drawGlyph(c *Canvas, ch rune, x, y int, cr, cg, cb, alpha uint8) int {
	if ch < firstRune || ch > lastRune {
		return CharAdvance
	}
	glyph := fontGlyphs[ch-firstRune]
	for row := 0; row < CharHeight; row++ {
		bits := glyph[row]
		if bits == 0 {
			continue
		}
		py := y + row
		for col := 0; col < CharWidth; col++ {
			if bits&(0x80>>uint(col)) == 0 {
				continue
			}
			px := x + col
			if px < 0 || py < 0 || px >= c.width || py >= c.height {
				continue
			}
			i := c.buf.PixOffset(px, py)
			c.buf.Pix[i] = cr
			c.buf.Pix[i+1] = cg
			c.buf.Pix[i+2] = cb
			c.buf.Pix[i+3] = alpha
		}
	}
	return CharAdvance
}

// DrawText draws a string on the canvas.
//
// It returns the width of the drawn text.
func DrawText(c *Canvas, s string, x, y int, clr color.Color) int {
	return drawText(c, s, x, y, clr)
}

// DrawTextRight draws a string right-aligned at x.
func DrawTextRight(c *Canvas, s string, x, y int, clr color.Color) {
	w := TextWidth(s)
	drawText(c, s, x-w, y, clr)
}

// DrawTextCentered draws a string centred on x.
func DrawTextCentered(c *Canvas, s string, x, y int, clr color.Color) {
	w := TextWidth(s)
	drawText(c, s, x-w/2, y, clr)
}

// TintCanvas covers the canvas with a translucent colour.
//
// It is used for the weather and the night overlays.
func TintCanvas(c *Canvas, col color.Color) {
	cr, cg, cb, ca := col.RGBA()
	alpha := float64(ca>>8) / 255.0
	if alpha <= 0 {
		return
	}
	r8, g8, b8 := uint8(cr>>8), uint8(cg>>8), uint8(cb>>8)
	for i := 0; i+3 < len(c.buf.Pix); i += 4 {
		c.buf.Pix[i] = blendChannel(c.buf.Pix[i], r8, alpha)
		c.buf.Pix[i+1] = blendChannel(c.buf.Pix[i+1], g8, alpha)
		c.buf.Pix[i+2] = blendChannel(c.buf.Pix[i+2], b8, alpha)
	}
	c.uploaded = true
}

// blendChannel blends a channel with a translucent colour.
func blendChannel(dst, src uint8, alpha float64) uint8 {
	return uint8(float64(dst)*(1-alpha) + float64(src)*alpha)
}
