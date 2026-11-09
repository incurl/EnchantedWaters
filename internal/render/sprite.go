package render

import (
	"image/color"
)

// This file draws the pictures of the original game.
//
// The pictures are planar: the colour of a pixel is one bit in each of
// several planes, and the index that comes out of them selects a colour of
// the palette of the game. The drawing therefore walks the pixels and
// writes the colour that the index names.
//
// The pictures are drawn in their own resolution. The game scales the
// screen by an integer factor, so a picture of the original resolution
// keeps its sharp edges.

// Sprite is a picture of the original game.
//
// The picture holds one colour index per pixel; the palette gives the
// colour of every index.
type Sprite struct {
	// Width and Height are the size of the picture in pixels.
	Width, Height int
	// Pixels holds one colour index per pixel, row by row.
	Pixels []uint8
	// Palette holds the colour of every index.
	Palette [][3]uint8
}

// NewSprite creates a picture of the given size.
func NewSprite(w, h int) *Sprite {
	return &Sprite{Width: w, Height: h, Pixels: make([]uint8, w*h)}
}

// At returns the colour index of a pixel of the picture.
//
// The pixels outside the picture are transparent, that is the index zero.
func (s *Sprite) At(x, y int) uint8 {
	if s == nil || x < 0 || y < 0 || x >= s.Width || y >= s.Height {
		return 0
	}
	return s.Pixels[y*s.Width+x]
}

// Set writes a colour index into the picture.
func (s *Sprite) Set(x, y int, index uint8) {
	if s == nil || x < 0 || y < 0 || x >= s.Width || y >= s.Height {
		return
	}
	s.Pixels[y*s.Width+x] = index
}

// ColorAt returns the colour of a pixel of the picture.
//
// The colour is the one of the palette that the index names. A picture
// without a palette of its own uses the palette of the game.
func (s *Sprite) ColorAt(x, y int) color.Color {
	if s == nil {
		return color.RGBA{}
	}
	index := s.At(x, y)
	if int(index) >= len(s.Palette) {
		return color.RGBA{}
	}
	c := s.Palette[index]
	return color.RGBA{c[0], c[1], c[2], 0xff}
}

// DrawSprite draws a picture into the canvas at the given position.
//
// The picture is drawn in its own resolution, one canvas pixel per pixel of
// the picture. The pixels that fall outside the canvas are dropped.
//
// The index zero is the transparent colour: the pixels that carry it are
// left alone, so that a picture can be drawn over another one.
func DrawSprite(c *Canvas, s *Sprite, x, y int) {
	if c == nil || s == nil {
		return
	}
	for py := 0; py < s.Height; py++ {
		cy := y + py
		if cy < 0 || cy >= c.height {
			continue
		}
		for px := 0; px < s.Width; px++ {
			cx := x + px
			if cx < 0 || cx >= c.width {
				continue
			}
			index := s.At(px, py)
			if index == 0 {
				continue
			}
			if int(index) >= len(s.Palette) {
				continue
			}
			col := s.Palette[index]
			c.Set(cx, cy, color.RGBA{col[0], col[1], col[2], 0xff})
		}
	}
}

// DrawSpriteOpaque draws a picture into the canvas, the index zero included.
//
// The picture is used for the background of a port, where the index zero
// is a colour of its own and not a hole.
func DrawSpriteOpaque(c *Canvas, s *Sprite, x, y int) {
	if c == nil || s == nil {
		return
	}
	for py := 0; py < s.Height; py++ {
		cy := y + py
		if cy < 0 || cy >= c.height {
			continue
		}
		for px := 0; px < s.Width; px++ {
			cx := x + px
			if cx < 0 || cx >= c.width {
				continue
			}
			index := s.At(px, py)
			if int(index) >= len(s.Palette) {
				continue
			}
			col := s.Palette[index]
			c.Set(cx, cy, color.RGBA{col[0], col[1], col[2], 0xff})
		}
	}
}

// SpriteSheet is a group of pictures of the same size.
type SpriteSheet struct {
	// TilesX and TilesY are the size of the sheet in tiles.
	TilesX, TilesY int
	// TileWidth and TileHeight are the size of a tile in pixels.
	TileWidth, TileHeight int
	// Tiles holds the tiles, row by row.
	Tiles []*Sprite
}

// Tile returns the tile at the given position of the sheet.
func (s *SpriteSheet) Tile(x, y int) *Sprite {
	if s == nil || x < 0 || y < 0 || x >= s.TilesX || y >= s.TilesY {
		return nil
	}
	return s.Tiles[y*s.TilesX+x]
}

// Count returns the number of tiles of the sheet.
func (s *SpriteSheet) Count() int {
	if s == nil {
		return 0
	}
	return len(s.Tiles)
}

// DrawTile draws a tile of a sheet into the canvas.
func DrawTile(c *Canvas, sheet *SpriteSheet, tileX, tileY, x, y int) {
	DrawSprite(c, sheet.Tile(tileX, tileY), x, y)
}
