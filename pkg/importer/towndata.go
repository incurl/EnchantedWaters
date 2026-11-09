package importer

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// This file converts the pictures of the towns of the original game.
//
// The pictures are of two kinds, and neither of them can go through the
// general planar reader of graphic.go:
//
//   - The tiles of the ports are stored in four planes and in segments of
//     thirty two bytes to a plane, and one segment is exactly one tile of
//     sixteen by sixteen pixels.
//   - The maps of the towns are grids of the indices of those tiles, not
//     pictures at all.
//
// The exporter therefore reads both of them directly and writes the result
// as derived data: a PNG for the tiles and a small binary file for the
// maps. The game reads the derived data, never the files of the release.

// The geometry of the pictures of the towns.
const (
	// PortChipWidth and PortChipHeight are the size of a tile of a port.
	PortChipWidth, PortChipHeight = 16, 16
	// PortChipPlanes is the number of bit planes of a tile.
	PortChipPlanes = 4
	// PortChipAlign is the number of bytes of a plane inside a segment.
	PortChipAlign = 32
	// PortChipSet is the number of tiles of one set.
	PortChipSet = 240
	// PortChipSets is the number of sets in the file.
	PortChipSets = 7
	// TownMapWidth and TownMapHeight are the size of a map of a town in
	// tiles.
	TownMapWidth, TownMapHeight = 96, 96
)

// HorizonPalette is the palette of the game.
//
// The values are the ones of HORIZON_KAO.PAL of the release.
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

// The names of the derived files of the towns.
const (
	// PortChipFile is the picture of the tiles of the ports.
	PortChipFile = "portchips.png"
	// TownMapFile is the grid of the maps of the towns.
	TownMapFile = "townmaps.bin"
	// ChipSetFile is the table that maps a town to its set of tiles.
	ChipSetFile = "chip_sets.csv"
)

// townMapMagic marks the binary file of the maps of the towns.
//
// The four bytes are "EWTM", and they let the reader refuse a file that is
// not the one it expects instead of decoding nonsense.
const townMapMagic = "EWTM"

// ExportTownData converts the pictures of the towns and writes them into
// outDir.
//
// The files of the release are read from rawDir. A file that is missing is
// reported in the returned problems instead of stopping the export: the
// towns are one part of the art, and the rest of the import must go on.
func ExportTownData(rawDir, outDir string, log Logger) (files, problems []string) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		problems = append(problems, fmt.Sprintf("create %s: %v", outDir, err))
		return nil, problems
	}

	// --- the portraits of the characters ---
	kaoPath := filepath.Join(rawDir, "KAO.LZW")
	if data, err := os.ReadFile(kaoPath); err != nil {
		problems = append(problems, fmt.Sprintf("the portraits: %v", err))
	} else {
		out := filepath.Join(outDir, "portraits.png")
		count, err := writePortraits(out, data)
		if err != nil {
			problems = append(problems, fmt.Sprintf("the portraits: %v", err))
		} else {
			files = append(files, out)
			log.Info("wrote %s: %d portraits", out, count)
		}
	}

	// --- the tiles of the ports ---
	chipPath := filepath.Join(rawDir, "PORTCHIP.LZW")
	if data, err := os.ReadFile(chipPath); err != nil {
		problems = append(problems, fmt.Sprintf("the tiles of the ports: %v", err))
	} else {
		out := filepath.Join(outDir, PortChipFile)
		sets, err := writePortChips(out, data)
		if err != nil {
			problems = append(problems, fmt.Sprintf("the tiles of the ports: %v", err))
		} else {
			files = append(files, out)
			log.Info("wrote %s: %d sets of %d tiles", out, sets, PortChipSet)
		}
	}

	// --- the maps of the towns ---
	mapPath := filepath.Join(rawDir, "PORTMAP.LZW")
	if data, err := os.ReadFile(mapPath); err != nil {
		problems = append(problems, fmt.Sprintf("the maps of the towns: %v", err))
	} else {
		out := filepath.Join(outDir, TownMapFile)
		count, err := writeTownMaps(out, data)
		if err != nil {
			problems = append(problems, fmt.Sprintf("the maps of the towns: %v", err))
		} else {
			files = append(files, out)
			log.Info("wrote %s: %d maps of %dx%d tiles", out, count, TownMapWidth, TownMapHeight)
		}
	}

	// --- the table of the sets ---
	chipPath = filepath.Join(rawDir, "CHIP_NO.DAT")
	if data, err := os.ReadFile(chipPath); err != nil {
		problems = append(problems, fmt.Sprintf("the table of the tiles of the towns: %v", err))
	} else {
		out := filepath.Join(outDir, ChipSetFile)
		if err := writeChipSets(out, data); err != nil {
			problems = append(problems, fmt.Sprintf("the table of the tiles of the towns: %v", err))
		} else {
			files = append(files, out)
			log.Info("wrote %s: %d towns", out, len(data))
		}
	}

	// The palette is not a file of the DOS release: the eight colours are
	// copied into the code, and the derived pictures carry them.
	return files, problems
}

// writePortChips writes the tiles of the ports as one PNG.
//
// The sets follow each other from the top of the picture: a set is a column
// of two hundred and forty tiles, so the picture is sixteen pixels wide and
// two hundred and forty tiles times seven sets tall.
func writePortChips(path string, data []byte) (int, error) {
	block, err := decodeLSFile(data)
	if err != nil {
		return 0, err
	}
	tileBytes := PortChipWidth * PortChipHeight * PortChipPlanes / 8

	// The blocks of the file alternate between a set of tiles and a small
	// block of four bytes; only the long ones hold tiles.
	var sets [][]byte
	for _, b := range block {
		if len(b) >= tileBytes*PortChipSet {
			sets = append(sets, b)
		}
	}
	if len(sets) == 0 {
		return 0, fmt.Errorf("the file holds no set of tiles")
	}

	palette := color.Palette{}
	for _, c := range HorizonPalette {
		palette = append(palette, color.RGBA{c[0], c[1], c[2], 0xff})
	}
	width := PortChipWidth
	height := len(sets) * PortChipSet * PortChipHeight
	img := image.NewPaletted(image.Rect(0, 0, width, height), palette)

	for s, set := range sets {
		for t := 0; t < PortChipSet; t++ {
			off := t * tileBytes
			if off+tileBytes > len(set) {
				break
			}
			tile := portChipTile(set[off : off+tileBytes])
			top := (s*PortChipSet + t) * PortChipHeight
			for y := 0; y < PortChipHeight; y++ {
				for x := 0; x < PortChipWidth; x++ {
					img.SetColorIndex(x, top+y, tile[y*PortChipWidth+x])
				}
			}
		}
	}
	return len(sets), writePNG(path, img)
}

// portChipTile assembles one tile out of the strip that holds it.
//
// A strip holds PortChipAlign bytes of every plane, and the planes follow
// each other. The pixels of a plane follow each other eight to a byte, and
// the bits of a byte come out from the top down.
func portChipTile(raw []byte) []uint8 {
	pixels := make([]uint8, PortChipWidth*PortChipHeight)
	for j := 0; j < PortChipWidth*PortChipHeight; j++ {
		value := 0
		for plane := 0; plane < PortChipPlanes; plane++ {
			off := plane*PortChipAlign + j/8
			if off >= len(raw) {
				continue
			}
			bit := (raw[off] >> uint(7-j%8)) & 1
			value |= int(bit) << uint(PortChipPlanes-1-plane)
		}
		pixels[j] = uint8(value)
	}
	return pixels
}

// writeTownMaps writes the maps of the towns as one binary file.
//
// The file holds a header — the magic, the size of a map, the number of
// maps — followed by the index of the tile of every cell, row by row, one
// byte to a cell.
func writeTownMaps(path string, data []byte) (int, error) {
	blocks, err := decodeLSFile(data)
	if err != nil {
		return 0, err
	}
	size := TownMapWidth * TownMapHeight
	var maps [][]byte
	for _, b := range blocks {
		if len(b) < size {
			continue
		}
		maps = append(maps, b[:size])
	}
	if len(maps) == 0 {
		return 0, fmt.Errorf("the file holds no map of a town")
	}

	buf := make([]byte, 0, 12+len(maps)*size)
	buf = append(buf, townMapMagic...)
	buf = binary.BigEndian.AppendUint32(buf, uint32(TownMapWidth))
	buf = binary.BigEndian.AppendUint32(buf, uint32(TownMapHeight))
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(maps)))
	for _, m := range maps {
		buf = append(buf, m...)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return 0, err
	}
	return len(maps), nil
}

// writeChipSets writes the table that maps a town to its set of tiles.
//
// The file is a two column table: the number of the town and the number of
// its set of tiles.
func writeChipSets(path string, data []byte) error {
	var b strings.Builder
	b.WriteString("town,set\n")
	for i, v := range data {
		fmt.Fprintf(&b, "%d,%d\n", i, v)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// decodeLSFile parses and decodes an LS10 or LS11 container.
func decodeLSFile(data []byte) ([][]byte, error) {
	f, err := ParseLS(data)
	if err != nil {
		return nil, err
	}
	return DecodeLS(f, data)
}

// The geometry of the portraits of the characters.
const (
	// PortraitWidth and PortraitHeight are the size of a portrait.
	PortraitWidth, PortraitHeight = 64, 80
	// PortraitPlanes is the number of bit planes of a portrait.
	PortraitPlanes = 3
	// PortraitColumns is the number of portraits per row of the sheet.
	PortraitColumns = 8
)

// writePortraits writes the portraits of the characters as one PNG.
//
// The file holds one portrait per block, each of them sixty four by eighty
// pixels in three planes, and the head of a portrait is drawn eight to a
// row.
func writePortraits(path string, data []byte) (int, error) {
	blocks, err := decodeLSFile(data)
	if err != nil {
		return 0, err
	}
	size := PortraitWidth * PortraitHeight * PortraitPlanes / 8
	var portraits [][]uint8
	for _, b := range blocks {
		if len(b) < size {
			continue
		}
		portraits = append(portraits, portraitPixels(b[:size]))
	}
	if len(portraits) == 0 {
		return 0, fmt.Errorf("the file holds no portrait")
	}

	palette := color.Palette{}
	for _, c := range HorizonPalette {
		palette = append(palette, color.RGBA{c[0], c[1], c[2], 0xff})
	}
	rows := (len(portraits) + PortraitColumns - 1) / PortraitColumns
	img := image.NewPaletted(
		image.Rect(0, 0, PortraitColumns*PortraitWidth, rows*PortraitHeight), palette)
	for i, pixels := range portraits {
		ox := (i % PortraitColumns) * PortraitWidth
		oy := (i / PortraitColumns) * PortraitHeight
		for y := 0; y < PortraitHeight; y++ {
			for x := 0; x < PortraitWidth; x++ {
				img.SetColorIndex(ox+x, oy+y, pixels[y*PortraitWidth+x])
			}
		}
	}
	return len(portraits), writePNG(path, img)
}

// portraitPixels assembles one portrait out of its planar bytes.
//
// The bytes are stored a group of eight pixels at a time: a group holds one
// byte of every plane in turn, and the first byte holds the highest bit of
// the colour index. The bits of a byte come out from the top down.
func portraitPixels(raw []byte) []uint8 {
	pixels := make([]uint8, PortraitWidth*PortraitHeight)
	for g := 0; g < PortraitWidth*PortraitHeight/8; g++ {
		row := g / (PortraitWidth / 8)
		col := g % (PortraitWidth / 8)
		for plane := 0; plane < PortraitPlanes; plane++ {
			off := g*PortraitPlanes + plane
			if off >= len(raw) {
				continue
			}
			for bit := 0; bit < 8; bit++ {
				v := (raw[off] >> uint(7-bit)) & 1
				pixels[row*PortraitWidth+col*8+bit] |= v << uint(PortraitPlanes-1-plane)
			}
		}
	}
	return pixels
}

// writePNG writes an indexed picture.
func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}
