package asset

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/peter/enchantedwaters/internal/render"
)

// This file loads the pictures of the original game at run time.
//
// The pictures are not the files of the release: the import step converts
// them into the derived data of assets/sprites, and the game reads that.
// The derived data is a set of indexed PNG files and a small binary table,
// which is what the pictures really are — a palette and one index per
// pixel.
//
// Everything is optional. The loader is forgiving: a missing file leaves
// the game with its own procedural drawings instead of stopping it.

// The palette of the game.
//
// The release ships it as HORIZON_KAO.PAL: eight RGB triples. The values
// are the ones of that file, which the import step copies into the derived
// data.
var HorizonPalette = [8][3]uint8{
	{0x00, 0x00, 0x00},
	{0x00, 0xA0, 0x60},
	{0xD0, 0x40, 0x00},
	{0xF0, 0xA0, 0x60},
	{0x00, 0x40, 0xD0},
	{0x00, 0xA0, 0xF0},
	{0xD0, 0x60, 0xA0},
	{0xF0, 0xE0, 0xD0},
}

// ChipPalette is the palette of the tiles of the ports.
//
// The tiles are stored in four planes, so a pixel names sixteen colours,
// while the palette of the game holds eight. The fourth plane is the
// intensity bit of the EGA adapters, which selects the bright half of the
// attribute table: the tiles of the ports use the indices nine, eleven,
// thirteen and fifteen about as often as the indices one, three, five and
// seven, which is exactly how a wall is drawn once in the shade and once in
// the light.
//
// The palette file of the release, HORIZON_KAO.PAL, carries only the eight
// base colours; the bright half is not in it. The table below therefore
// folds the two halves together: the intensity bit names the same colour as
// its base, so the towns are drawn in the eight colours that the release
// actually ships instead of inventing the other eight.
var ChipPalette = foldPalette(HorizonPalette)

// foldPalette builds the table of sixteen colours of the tiles of the ports
// out of the eight colours of the game.
func foldPalette(base [8][3]uint8) [16][3]uint8 {
	var out [16][3]uint8
	for i := 0; i < 16; i++ {
		out[i] = base[i%len(base)]
	}
	return out
}

// The names of the derived files of the pictures.
const (
	// ArtFilePortraits is the sheet of the portraits of the characters.
	ArtFilePortraits = "portraits.png"
	// ArtFilePortChips is the picture of the tiles of the ports.
	ArtFilePortChips = "portchips.png"
	// ArtFileTownMaps is the grid of the maps of the towns.
	ArtFileTownMaps = "townmaps.bin"
	// ArtFileChipSets is the table that maps a town to its set of tiles.
	ArtFileChipSets = "chip_sets.csv"
)

// The geometry of the pictures of the towns.
const (
	// chipWidth and chipHeight are the size of a tile of a port.
	chipWidth, chipHeight = 16, 16
	// chipsPerSet is the number of tiles of one set.
	chipsPerSet = 240
	// portraitWidth and portraitHeight are the size of a portrait.
	portraitWidth, portraitHeight = 64, 80
	// portraitColumns is the number of portraits per row of the sheet.
	portraitColumns = 8
)

// SpriteSet is a group of pictures of the ports.
//
// The tiles are the material out of which the picture of a town is built.
type SpriteSet struct {
	// Name is the name of the set.
	Name string
	// TilesX and TilesY are the size of the set in tiles.
	TilesX, TilesY int
	// TileWidth and TileHeight are the size of a tile in pixels.
	TileWidth, TileHeight int
	// Tiles holds the tiles, row by row.
	Tiles []*render.Sprite
}

// Tile returns the tile at the given position of the set.
func (s *SpriteSet) Tile(x, y int) *render.Sprite {
	if s == nil || x < 0 || y < 0 || x >= s.TilesX || y >= s.TilesY {
		return nil
	}
	return s.Tiles[y*s.TilesX+x]
}

// TileCount returns the number of tiles of the set.
func (s *SpriteSet) TileCount() int {
	if s == nil {
		return 0
	}
	return len(s.Tiles)
}

// Art holds the pictures of the original game that the game draws.
//
// Every field may be nil: the game falls back on its procedural drawings
// for whatever is missing.
type Art struct {
	// PortChips holds the tiles of the ports, one set per cultural
	// group.
	PortChips []*SpriteSet
	// Portraits holds the portraits of the characters, in the order of
	// the sheet.
	Portraits []*render.Sprite
	// Palette is the colour table of the game.
	Palette [][3]uint8
	// Source records where the art came from, for the log.
	Source string
}

// Empty reports whether the art holds no picture at all.
func (a *Art) Empty() bool {
	if a == nil {
		return true
	}
	return len(a.Portraits) == 0 && len(a.PortChips) == 0
}

// PortChipSet returns the tiles of the cultural group of a port.
//
// A number outside the range wraps around, so that a wrong entry in the
// table of the towns shows the wrong town instead of stopping the game.
func (a *Art) PortChipSet(portIndex int) *SpriteSet {
	if a == nil || len(a.PortChips) == 0 {
		return nil
	}
	n := len(a.PortChips)
	i := portIndex % n
	if i < 0 {
		i += n
	}
	return a.PortChips[i]
}

// Portrait returns the portrait of a character.
func (a *Art) Portrait(index int) *render.Sprite {
	if a == nil || index < 0 || index >= len(a.Portraits) {
		return nil
	}
	return a.Portraits[index]
}

// LoadArt reads the pictures of the original game out of a directory.
//
// The directory is the derived one, assets/sprites: it holds the sheets the
// import step wrote. Everything is optional, and the function returns the
// list of the problems it met.
func LoadArt(dir string) (*Art, []string) {
	art := &Art{Palette: HorizonPalette[:], Source: dir}
	var problems []string

	if sets, err := loadPortChips(filepath.Join(dir, ArtFilePortChips)); err != nil {
		problems = append(problems, fmt.Sprintf("the tiles of the ports: %v", err))
	} else {
		art.PortChips = sets
	}

	if portraits, err := loadPortraits(filepath.Join(dir, ArtFilePortraits)); err != nil {
		problems = append(problems, fmt.Sprintf("the portraits: %v", err))
	} else {
		art.Portraits = portraits
	}
	return art, problems
}

// loadPortChips reads the tiles of the ports.
//
// The sheet is a column of tiles: every set of two hundred and forty tiles
// follows the one before it, so the sheet is sixteen pixels wide and a
// multiple of two hundred and forty tiles tall.
func loadPortChips(path string) ([]*SpriteSet, error) {
	img, err := readPaletted(path)
	if err != nil {
		return nil, err
	}
	rows := img.Bounds().Dy()
	setRows := chipsPerSet * chipHeight
	if rows < setRows {
		return nil, fmt.Errorf("%s holds %d rows, too few for one set", path, rows)
	}
	var sets []*SpriteSet
	for top := 0; top+setRows <= rows; top += setRows {
		set := &SpriteSet{
			Name:       "port chips",
			TilesX:     chipsPerSet,
			TilesY:     1,
			TileWidth:  chipWidth,
			TileHeight: chipHeight,
			Tiles:      make([]*render.Sprite, chipsPerSet),
		}
		for t := 0; t < chipsPerSet; t++ {
			tile := &render.Sprite{
				Width:   chipWidth,
				Height:  chipHeight,
				Pixels:  make([]uint8, chipWidth*chipHeight),
				Palette: ChipPalette[:],
			}
			oy := top + t*chipHeight
			for y := 0; y < chipHeight; y++ {
				for x := 0; x < chipWidth; x++ {
					tile.Pixels[y*chipWidth+x] = colourIndexOf(img, x, oy+y)
				}
			}
			set.Tiles[t] = tile
		}
		sets = append(sets, set)
	}
	return sets, nil
}

// loadPortraits reads the sheet of the portraits of the characters.
//
// The sheet is a grid of eight portraits per row, and every portrait is
// sixty four by eighty pixels.
func loadPortraits(path string) ([]*render.Sprite, error) {
	img, err := readPaletted(path)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	columns := bounds.Dx() / portraitWidth
	rows := bounds.Dy() / portraitHeight
	if columns <= 0 || rows <= 0 {
		return nil, fmt.Errorf("%s holds no portrait of %dx%d", path, portraitWidth, portraitHeight)
	}
	out := make([]*render.Sprite, 0, columns*rows)
	for row := 0; row < rows; row++ {
		for col := 0; col < columns; col++ {
			portrait := &render.Sprite{
				Width:   portraitWidth,
				Height:  portraitHeight,
				Pixels:  make([]uint8, portraitWidth*portraitHeight),
				Palette: HorizonPalette[:],
			}
			ox, oy := col*portraitWidth, row*portraitHeight
			for y := 0; y < portraitHeight; y++ {
				for x := 0; x < portraitWidth; x++ {
					portrait.Pixels[y*portraitWidth+x] = colourIndexOf(img, ox+x, oy+y)
				}
			}
			out = append(out, portrait)
		}
	}
	return out, nil
}

// readPaletted reads an indexed PNG into the paletted form that the pictures
// use.
//
// A picture that was stored with a different depth is converted: the game
// only ever draws indexed pictures, and a sheet of the wrong form would
// have to be reduced anyway.
func readPaletted(path string) (*image.Paletted, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	if p, ok := decoded.(*image.Paletted); ok {
		return p, nil
	}
	// The sheet was stored in a true colour form: reduce it to the palette
	// of the game, which is the only palette the pictures use.
	bounds := decoded.Bounds()
	palette := make(color.Palette, 0, len(HorizonPalette))
	for _, c := range HorizonPalette {
		palette = append(palette, color.RGBA{c[0], c[1], c[2], 0xff})
	}
	out := image.NewPaletted(bounds, palette)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.Set(x, y, decoded.At(x, y))
		}
	}
	return out, nil
}

// colourIndexOf returns the colour index of a pixel of an indexed picture.
func colourIndexOf(img *image.Paletted, x, y int) uint8 {
	if !(image.Point{x, y}).In(img.Bounds()) {
		return 0
	}
	return img.Pix[img.PixOffset(x, y)]
}
