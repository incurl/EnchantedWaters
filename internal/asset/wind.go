package asset

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// This file loads the wind and the currents of the original game.
//
// The data is not a file of the release: the import step converts WINDCUR.DAT
// of the release into a table of the sea areas of the world map, and the game
// reads that table.
//
// The loader is forgiving in the same way as the loader of the pictures: a
// missing file leaves the game with the weather it derives on its own.

// The names of the derived files of the wind.
const (
	// ArtFileWindCurrent is the table of the wind of the sea areas.
	ArtFileWindCurrent = "wind_current.csv"
)

// The geometry of the world map.
const (
	// WindColumns and WindRows are the size of the grid of the sea areas.
	//
	// The world map of the original game is 2160 columns by 1080 rows of
	// tiles, and the sea areas are 72 by 72 tiles.
	WindColumns, WindRows = 30, 15
	// WindSeaAreas is the number of the sea areas of the map.
	WindSeaAreas = WindColumns * WindRows
	// WindSeaAreaSize is the size of a sea area in tiles.
	WindSeaAreaSize = 72
)

// WindCell is the wind of one sea area of the world map.
type WindCell struct {
	// Direction is the direction the wind blows to, as a number from 0 to
	// 7 counted clockwise from the north.
	Direction int
	// Strength is the strength of the wind, as a number from 0 to 31.
	Strength int
	// Current is the direction of the current of the sea area, as a
	// number from 0 to 7 counted clockwise from the north.
	Current int
}

// WindTable holds the wind of every sea area of the world map.
type WindTable struct {
	// Cells holds the sea areas, row by row.
	Cells []WindCell
}

// Cell returns the wind of the sea area at the given column and row.
//
// The function returns the zero cell when the position is outside of the
// map, so a caller never has to check the bounds itself.
func (t *WindTable) Cell(column, row int) WindCell {
	if t == nil || column < 0 || row < 0 || column >= WindColumns || row >= WindRows {
		return WindCell{}
	}
	if row*WindColumns+column >= len(t.Cells) {
		return WindCell{}
	}
	return t.Cells[row*WindColumns+column]
}

// SeaArea returns the wind of the sea area that holds a position.
//
// The position is in the tiles of the world map.
func (t *WindTable) SeaArea(x, y int) WindCell {
	return t.Cell(x/WindSeaAreaSize, y/WindSeaAreaSize)
}

// LoadWind reads the table of the wind out of a directory of derived data.
//
// The function returns nil when the file is not there: the wind is an
// optional extra, and the game falls back on its own weather.
func LoadWind(dir string) *WindTable {
	path := filepath.Join(dir, ArtFileWindCurrent)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil
	}
	if len(rows) < 2 {
		return nil
	}
	// The first row names the columns.
	table := &WindTable{Cells: make([]WindCell, WindSeaAreas)}
	for _, row := range rows[1:] {
		if len(row) < 5 {
			continue
		}
		column, err := strconv.Atoi(row[0])
		if err != nil {
			continue
		}
		rowIndex, err := strconv.Atoi(row[1])
		if err != nil {
			continue
		}
		if column < 0 || rowIndex < 0 || column >= WindColumns || rowIndex >= WindRows {
			continue
		}
		direction, _ := strconv.Atoi(row[2])
		strength, _ := strconv.Atoi(row[3])
		current, _ := strconv.Atoi(row[4])
		table.Cells[rowIndex*WindColumns+column] = WindCell{
			Direction: direction,
			Strength:  strength,
			Current:   current,
		}
	}
	return table
}

// String returns a short description of the table, for the logs.
func (t *WindTable) String() string {
	if t == nil {
		return "no wind"
	}
	return fmt.Sprintf("the wind of %d sea areas", len(t.Cells))
}
