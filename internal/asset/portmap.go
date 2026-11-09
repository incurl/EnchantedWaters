package asset

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// This file loads the maps of the towns.
//
// A map of a town is a grid of the indices of the tiles of the ports: the
// import step writes the grids into one binary file, and the game lays the
// tiles out in the order that a grid gives.

// The geometry of the maps of the towns.
const (
	// TownTile is the size of a tile of a town in pixels.
	TownTile = 16
	// townMapMagic marks the binary file of the maps.
	townMapMagic = "EWTM"
)

// PortMap is the map of one town.
//
// The map is a grid of the indices of the tiles. The picture of the town is
// drawn by putting the tiles in their place.
type PortMap struct {
	// Index holds the index of the tile of every cell, row by row.
	Index []uint8
	// Width and Height are the size of the map in tiles.
	Width, Height int
}

// Cell returns the index of the tile of a cell of the map.
func (m *PortMap) Cell(x, y int) uint8 {
	if m == nil || x < 0 || y < 0 || x >= m.Width || y >= m.Height {
		return 0
	}
	return m.Index[y*m.Width+x]
}

// LoadPortMaps reads the maps of the towns out of a directory.
//
// The maps are returned in the order of the file, which is the order of the
// towns of the original game.
func LoadPortMaps(dir string) ([]*PortMap, error) {
	path := filepath.Join(dir, ArtFileTownMaps)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 || string(data[:4]) != townMapMagic {
		return nil, fmt.Errorf("%s is not a town map file", path)
	}
	width := int(binary.BigEndian.Uint32(data[4:8]))
	height := int(binary.BigEndian.Uint32(data[8:12]))
	count := int(binary.BigEndian.Uint32(data[12:16]))
	if width <= 0 || height <= 0 || count < 0 {
		return nil, fmt.Errorf("%s declares a map of %dx%d, which is not a size", path, width, height)
	}
	size := width * height
	if 16+count*size > len(data) {
		return nil, fmt.Errorf("%s declares %d maps of %dx%d, which is more than it holds",
			path, count, width, height)
	}
	out := make([]*PortMap, 0, count)
	for i := 0; i < count; i++ {
		index := make([]uint8, size)
		copy(index, data[16+i*size:16+(i+1)*size])
		out = append(out, &PortMap{Index: index, Width: width, Height: height})
	}
	return out, nil
}

// LoadChipNumbers reads the table that gives the set of tiles of every town.
//
// The file holds one line per town, and the second column is the number of
// the set of tiles that the town is drawn out of: the ports of northern
// Europe, of the Mediterranean, of the Caribbean, of Africa, of India, of
// China and of Japan each have their own set.
func LoadChipNumbers(dir string) ([]int, error) {
	path := filepath.Join(dir, ArtFileChipSets)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	out := make([]int, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || i == 0 {
			// The first line is the heading of the table.
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s line %d is not a pair: %q", path, i+1, line)
		}
		v, err := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %v", path, i+1, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// PortMapFocus returns the row at which the view of a town should start.
//
// A map of a town is taller than the screen of the game, and only a window
// of it is shown. The town — the houses, the roads, the church and the
// shipyard — is the part of the map that holds the most different tiles, so
// the window is centred on the rows where the variety is highest.
//
// The argument rows is the number of rows that the window shows, in tiles.
func PortMapFocus(m *PortMap, rows int) int {
	if m == nil || rows <= 0 {
		return 0
	}
	if rows >= m.Height {
		return 0
	}
	// The variety of a row: the houses of a town use many different tiles,
	// while the open sea and the open country use a handful.
	variety := make([]int, m.Height)
	for y := 0; y < m.Height; y++ {
		seen := make(map[uint8]struct{}, m.Width)
		for x := 0; x < m.Width; x++ {
			seen[m.Cell(x, y)] = struct{}{}
		}
		variety[y] = len(seen)
	}
	// The window with the highest total variety.
	best, bestSum := 0, -1
	sum := 0
	for y := 0; y < m.Height; y++ {
		sum += variety[y]
		if y >= rows {
			sum -= variety[y-rows]
		}
		if y >= rows-1 {
			start := y - rows + 1
			if sum > bestSum {
				bestSum, best = sum, start
			}
		}
	}
	return best
}
