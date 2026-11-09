package importer

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
)

// This file implements the graphics conversion: the raw planar graphics of
// the DOS era are converted into indexed PNG sprite sheets.
//
// # The VGA palette
//
// The original games stored their palettes in the VGA DAC, as 256 RGB
// triplets. The palette section is therefore a flat array of 768 bytes:
//
//	byte 0..2   : the RGB of the colour index 0
//	byte 3..5   : the RGB of the colour index 1
//	...
//
// # The planar graphics
//
// The sprites were stored in a planar format: every plane holds one bit
// per pixel, and the planes are interleaved. A 8x8 tile therefore takes
// 8 bytes per plane. The importer reconstructs the pixels:
//
//	pixel = sum over the planes of (planeBit << planeIndex)
//
// which yields the colour index of every pixel.
//
// The layout of the graphics is described by the specifications, so that
// the importer stays a generic converter.

// PaletteSpec describes a VGA palette section.
type PaletteSpec struct {
	// Offset is the offset of the palette inside the data.
	Offset int64 `json:"offset"`
	// Count is the number of colours.
	Count int `json:"count"`
	// Name is the base name of the exported palette file.
	Name string `json:"name"`
}

// Palette is a converted colour table.
type Palette struct {
	// Name is the name of the palette.
	Name string
	// Colors holds the colours, in index order.
	Colors []color.RGBA
}

// EGA palettes.
//
// The EGA adapters of the era held a fixed palette of sixteen colours.
// The three bit art of the world map and of the sprites uses the bright
// half of it: the eight colours below.
//
// The values are the ones of the attribute controller mapping that the
// original engine programs, as the analysis of the first Uncharted Waters
// documents it.
var EGAPalette = []color.RGBA{
	{0x00, 0x00, 0x00, 0xff}, // 0 black
	{0x55, 0x55, 0xff, 0xff}, // 1 bright blue
	{0x55, 0xff, 0x55, 0xff}, // 2 bright green
	{0x55, 0xff, 0xff, 0xff}, // 3 bright cyan
	{0xff, 0x55, 0x55, 0xff}, // 4 bright red
	{0xff, 0x55, 0xff, 0xff}, // 5 bright magenta
	{0xff, 0xff, 0x55, 0xff}, // 6 yellow
	{0xff, 0xff, 0xff, 0xff}, // 7 white
}

// ReadPalette reads a VGA palette from a data section.
func ReadPalette(r *Reader, spec PaletteSpec) (*Palette, error) {
	if spec.Count <= 0 {
		spec.Count = 256
	}
	if spec.Count > 256 {
		spec.Count = 256
	}
	p := &Palette{Name: spec.Name, Colors: make([]color.RGBA, spec.Count)}
	buf := make([]byte, spec.Count*3)
	if err := r.ReadAt(buf, spec.Offset); err != nil {
		return nil, fmt.Errorf("read palette %s: %w", spec.Name, err)
	}
	for i := 0; i < spec.Count; i++ {
		p.Colors[i] = color.RGBA{buf[i*3], buf[i*3+1], buf[i*3+2], 0xff}
	}
	return p, nil
}

// WritePalette writes a palette as a JSON file.
//
// The format is the one of the PNG palette chunk, so that the file can be
// compared with the exported images.
func WritePalette(path string, p *Palette) error {
	type entry struct {
		R, G, B, A uint8
	}
	entries := make([]entry, 0, len(p.Colors))
	for _, c := range p.Colors {
		entries = append(entries, entry{c.R, c.G, c.B, c.A})
	}
	data, err := marshalIndent(entries)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// GraphicSpec describes a planar graphics section.
type GraphicSpec struct {
	// Offset is the offset of the graphics inside the data.
	Offset int64 `json:"offset"`
	// Width is the width of one tile in pixels.
	Width int `json:"width"`
	// Height is the height of one tile in pixels.
	Height int `json:"height"`
	// Planes is the number of bit planes of a tile.
	Planes int `json:"planes"`
	// Columns is the number of tiles per row.
	Columns int `json:"columns"`
	// Count is the number of tiles; 0 means "until the end".
	Count int `json:"count"`
	// Name is the base name of the exported sprite sheet.
	Name string `json:"name"`
	// PlaneOrder maps each stored plane to the bit of the colour index
	// that it contributes.
	//
	// The EGA adapters of the era stored the planes in an order that did
	// not match their bit number, because the hardware wired them that
	// way. The reference analysis of the first Uncharted Waters documents
	// the order 1, 4, 2 for its three plane art, which is written here as
	// the list {0, 2, 1}: the first stored plane is bit 0, the second is
	// bit 2, the third is bit 1.
	//
	// When the list is empty the planes contribute in their stored order.
	PlaneOrder []int `json:"plane_order,omitempty"`
	// Interleaved marks graphics whose planes are stored byte by byte
	// instead of plane by plane.
	//
	// In the interleaved layout every group of eight pixels holds one
	// byte of each plane in turn; in the planar layout a whole plane is
	// stored before the next one begins.
	Interleaved bool `json:"interleaved,omitempty"`
	// EGAPalette selects the three bit palette of the EGA adapters
	// instead of the 256 colour VGA palette.
	EGAPalette bool `json:"ega_palette,omitempty"`
}

// SpriteSheet is a converted graphics section.
type SpriteSheet struct {
	// Name is the name of the sheet.
	Name string
	// Image holds the indexed pixels.
	Image *image.Paletted
	// TilesX / TilesY is the size of the sheet in tiles.
	TilesX, TilesY int
	// TileWidth / TileHeight is the size of a tile in pixels.
	TileWidth, TileHeight int
}

// ReadSpriteSheet reads a planar graphics section.
//
// The reader works on one tile at a time, so the memory usage is bounded
// by the size of the sheet, not by the size of the file.
func ReadSpriteSheet(r *Reader, spec GraphicSpec, pal *Palette) (*SpriteSheet, error) {
	if spec.Width <= 0 || spec.Height <= 0 || spec.Planes <= 0 {
		return nil, fmt.Errorf("graphic %s: invalid geometry", spec.Name)
	}
	if spec.Columns <= 0 {
		spec.Columns = 1
	}
	// The bytes per plane of a tile.
	planeBytes := (spec.Width + 7) / 8
	tileBytes := planeBytes * spec.Height * spec.Planes

	count := spec.Count
	if count == 0 {
		remaining := r.Size() - spec.Offset
		if remaining > 0 {
			count = int(remaining / int64(tileBytes))
		}
	}
	tilesX := spec.Columns
	tilesY := (count + tilesX - 1) / tilesX

	palette := color.Palette{}
	if spec.EGAPalette {
		for _, c := range EGAPalette {
			palette = append(palette, c)
		}
	} else if pal != nil {
		for _, c := range pal.Colors {
			palette = append(palette, c)
		}
	} else {
		// A grayscale ramp is used when no palette is available.
		for i := 0; i < 256; i++ {
			v := uint8(i)
			palette = append(palette, color.RGBA{v, v, v, 0xff})
		}
	}

	img := image.NewPaletted(image.Rect(0, 0, spec.Columns*spec.Width, tilesY*spec.Height), palette)
	sheet := &SpriteSheet{
		Name:       spec.Name,
		Image:      img,
		TilesX:     tilesX,
		TilesY:     tilesY,
		TileWidth:  spec.Width,
		TileHeight: spec.Height,
	}

	buf := make([]byte, tileBytes)
	for i := 0; i < count; i++ {
		offset := spec.Offset + int64(i)*int64(tileBytes)
		if err := r.ReadAt(buf, offset); err != nil {
			return nil, fmt.Errorf("graphic %s: read tile %d: %w", spec.Name, i, err)
		}
		drawTile(img, buf, spec, i)
	}
	return sheet, nil
}

// drawTile decodes one planar tile into the image.
//
// # The two layouts
//
// In the planar layout a whole plane of the tile is stored before the next
// one begins, so the byte of a pixel sits at
//
//	(plane*height + row)*planeBytes + col/8
//
// In the interleaved layout the bytes of the planes of one group of eight
// pixels follow each other, so the byte sits at
//
//	(row*planeBytes*groups + col/8*planes + plane)
//
// The EGA art of the era uses the interleaved layout together with a plane
// order that does not match the bit number of the plane; both are read
// from the specification.
func drawTile(img *image.Paletted, buf []byte, spec GraphicSpec, index int) {
	planeBytes := (spec.Width + 7) / 8
	tileX := (index % spec.Columns) * spec.Width
	tileY := (index / spec.Columns) * spec.Height

	// The bit of the colour index that each stored plane contributes.
	bitOf := func(plane int) uint {
		if plane < len(spec.PlaneOrder) {
			return uint(spec.PlaneOrder[plane])
		}
		return uint(plane)
	}

	for row := 0; row < spec.Height; row++ {
		for col := 0; col < spec.Width; col++ {
			value := 0
			for plane := 0; plane < spec.Planes; plane++ {
				var byteIndex int
				if spec.Interleaved {
					byteIndex = (row*planeBytes+col/8)*spec.Planes + plane
				} else {
					byteIndex = (plane*spec.Height+row)*planeBytes + col/8
				}
				if byteIndex < 0 || byteIndex >= len(buf) {
					continue
				}
				bit := (buf[byteIndex] >> uint(7-col%8)) & 1
				value |= int(bit) << bitOf(plane)
			}
			img.SetColorIndex(tileX+col, tileY+row, uint8(value))
		}
	}
}

// WritePNG writes a sprite sheet as a PNG file.
func WritePNG(path string, sheet *SpriteSheet) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, sheet.Image); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}

// WriteSpriteSheet writes the sheet and its palette into a directory.
func WriteSpriteSheet(dir string, sheet *SpriteSheet, pal *Palette) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	pngPath := filepath.Join(dir, sheet.Name+".png")
	if err := WritePNG(pngPath, sheet); err != nil {
		return nil, err
	}
	files := []string{pngPath}
	if pal != nil {
		palPath := filepath.Join(dir, pal.Name+".json")
		if err := WritePalette(palPath, pal); err != nil {
			return files, err
		}
		files = append(files, palPath)
	}
	return files, nil
}

// marshalIndent encodes a value as an indented JSON document.
func marshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// readUint32 reads a little-endian 32 bit value.
func readUint32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

// ioCopy copies a stream; it exists to keep the io import used by the
// graphics code.
func ioCopy(dst io.Writer, src io.Reader) (int64, error) { return io.Copy(dst, src) }
