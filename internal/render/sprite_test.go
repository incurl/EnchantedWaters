package render

import (
	"image/color"
	"testing"
)

// testPalette is the palette of the pictures of the game.
var testPalette = [][3]uint8{
	{0x00, 0x00, 0x00},
	{0xFF, 0x00, 0x00},
	{0x00, 0xFF, 0x00},
	{0x00, 0x00, 0xFF},
}

// TestDrawSprite verifies that a picture is drawn with the colour of every
// index.
func TestDrawSprite(t *testing.T) {
	c := NewCanvas(4, 4)
	s := &Sprite{
		Width:   2,
		Height:  2,
		Pixels:  []uint8{1, 2, 3, 1},
		Palette: testPalette,
	}
	DrawSprite(c, s, 1, 1)

	if got, want := c.Image().RGBAAt(1, 1), (color.RGBA{0xFF, 0, 0, 0xFF}); got != want {
		t.Errorf("pixel (1,1) is %v, want %v", got, want)
	}
	if got, want := c.Image().RGBAAt(2, 1), (color.RGBA{0, 0xFF, 0, 0xFF}); got != want {
		t.Errorf("pixel (2,1) is %v, want %v", got, want)
	}
	if got, want := c.Image().RGBAAt(1, 2), (color.RGBA{0, 0, 0xFF, 0xFF}); got != want {
		t.Errorf("pixel (1,2) is %v, want %v", got, want)
	}
	if got, want := c.Image().RGBAAt(2, 2), (color.RGBA{0xFF, 0, 0, 0xFF}); got != want {
		t.Errorf("pixel (2,2) is %v, want %v", got, want)
	}
}

// TestDrawSpriteLeavesZeroAlone verifies that the index zero is a hole.
//
// A picture of the original game is drawn over another one, so the pixels
// of index zero must not paint over what is already there.
func TestDrawSpriteLeavesZeroAlone(t *testing.T) {
	c := NewCanvas(2, 1)
	c.Clear(color.RGBA{0x10, 0x20, 0x30, 0xFF})
	s := &Sprite{
		Width:   2,
		Height:  1,
		Pixels:  []uint8{0, 1},
		Palette: testPalette,
	}
	DrawSprite(c, s, 0, 0)
	if got, want := c.Image().RGBAAt(0, 0), (color.RGBA{0x10, 0x20, 0x30, 0xFF}); got != want {
		t.Errorf("pixel (0,0) is %v, want the background %v", got, want)
	}
	if got, want := c.Image().RGBAAt(1, 0), (color.RGBA{0xFF, 0, 0, 0xFF}); got != want {
		t.Errorf("pixel (1,0) is %v, want %v", got, want)
	}
}

// TestDrawSpriteOpaque paints the index zero as well.
func TestDrawSpriteOpaque(t *testing.T) {
	c := NewCanvas(1, 1)
	c.Clear(color.RGBA{0x10, 0x20, 0x30, 0xFF})
	s := &Sprite{
		Width:   1,
		Height:  1,
		Pixels:  []uint8{0},
		Palette: testPalette,
	}
	DrawSpriteOpaque(c, s, 0, 0)
	if got, want := c.Image().RGBAAt(0, 0), (color.RGBA{0, 0, 0, 0xFF}); got != want {
		t.Errorf("pixel (0,0) is %v, want %v", got, want)
	}
}

// TestDrawSpriteClips verifies that a picture that reaches past the canvas
// is clipped instead of panicking.
func TestDrawSpriteClips(t *testing.T) {
	c := NewCanvas(2, 2)
	s := &Sprite{
		Width:   4,
		Height:  4,
		Pixels:  make([]uint8, 16),
		Palette: testPalette,
	}
	for i := range s.Pixels {
		s.Pixels[i] = 1
	}
	DrawSprite(c, s, -1, -1)
	DrawSprite(c, s, 1, 1)
	if got, want := c.Image().RGBAAt(1, 1), (color.RGBA{0xFF, 0, 0, 0xFF}); got != want {
		t.Errorf("pixel (1,1) is %v, want %v", got, want)
	}
}

// TestSpriteAt verifies the pixel lookup of a picture.
func TestSpriteAt(t *testing.T) {
	s := &Sprite{Width: 2, Height: 2, Pixels: []uint8{1, 2, 3, 4}, Palette: testPalette}
	if got := s.At(0, 0); got != 1 {
		t.Errorf("At(0,0) is %d, want 1", got)
	}
	if got := s.At(1, 1); got != 4 {
		t.Errorf("At(1,1) is %d, want 4", got)
	}
	if got := s.At(2, 0); got != 0 {
		t.Errorf("At(2,0) is %d, want 0", got)
	}
	// A missing picture reads as transparent everywhere.
	var missing *Sprite
	if got := missing.At(0, 0); got != 0 {
		t.Errorf("At on a nil picture is %d, want 0", got)
	}
	// The colour of the index outside the palette is transparent.
	s.Pixels[0] = 200
	if got := s.ColorAt(0, 0); got != (color.RGBA{}) {
		t.Errorf("ColorAt of an index outside the palette is %v, want transparent", got)
	}
}

// TestSpriteSheetTile verifies the addressing of a sheet of tiles.
func TestSpriteSheetTile(t *testing.T) {
	sheet := &SpriteSheet{
		TilesX: 2, TilesY: 2,
		TileWidth: 1, TileHeight: 1,
		Tiles: []*Sprite{
			{Width: 1, Height: 1, Pixels: []uint8{1}, Palette: testPalette},
			{Width: 1, Height: 1, Pixels: []uint8{2}, Palette: testPalette},
			{Width: 1, Height: 1, Pixels: []uint8{3}, Palette: testPalette},
			{Width: 1, Height: 1, Pixels: []uint8{4}, Palette: testPalette},
		},
	}
	if got := sheet.Tile(0, 0).Pixels[0]; got != 1 {
		t.Errorf("Tile(0,0) is %d, want 1", got)
	}
	if got := sheet.Tile(1, 1).Pixels[0]; got != 4 {
		t.Errorf("Tile(1,1) is %d, want 4", got)
	}
	if got := sheet.Tile(2, 0); got != nil {
		t.Error("Tile outside the sheet is not nil")
	}
	if got := sheet.Count(); got != 4 {
		t.Errorf("Count is %d, want 4", got)
	}
}

// TestDrawFrame verifies the frame helper.
func TestDrawFrame(t *testing.T) {
	c := NewCanvas(4, 4)
	DrawFrame(c, 1, 1, 3, 3, ColorFrame)
	if got := c.Image().RGBAAt(1, 1); got != ColorFrame {
		t.Errorf("corner is %v, want the frame colour", got)
	}
	if got := c.Image().RGBAAt(3, 3); got != ColorFrame {
		t.Errorf("corner is %v, want the frame colour", got)
	}
	if got := c.Image().RGBAAt(2, 2); got == ColorFrame {
		t.Error("the middle of the frame was painted")
	}
	// A frame of no size paints nothing, and a frame with a nil canvas
	// does not panic.
	DrawFrame(c, 0, 0, 0, 0, ColorFrame)
	DrawFrame(nil, 0, 0, 2, 2, ColorFrame)
}
