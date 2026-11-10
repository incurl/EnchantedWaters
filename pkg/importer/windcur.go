package importer

import (
	"fmt"
	"os"
	"path/filepath"
)

// This file reads the wind and the currents of the original game.
//
// # The file
//
// The file is called WINDCUR.DAT and it is a plain array of bytes: there is
// no header, no magic and no compression. Its size is 1350 bytes, which is
// three tables of 450 entries.
//
// # The grid of the sea areas
//
// The world map of the original game is 2160 by 1080 pixels. It is divided
// into sea areas of 72 by 72 pixels, and there are 30 columns and 15 rows of
// them, so 450 sea areas in all.
//
// The division is confirmed by the executable: the routine that reads the
// wind divides the position of a ship by 72 and uses the quotient as the
// index of the sea area.
//
// # The tables
//
// The file holds three tables of 450 bytes, one for the wind, one for the
// strength of the wind and one for the currents. The loader of the release
// reads the file twice, once for the summer and once for the winter, and it
// reads 450 bytes for the wind, then 900 bytes for the two other tables.
//
// # The byte of a sea area
//
// Every byte of a table packs two numbers into one byte:
//
//	bits 0 to 2  the direction, a number from 0 to 7
//	bits 3 to 7  the strength, a number from 0 to 31
//
// The direction is taken with a mask of seven and the strength is taken
// with a mask of 0xf8, which is what the executable does when it reads a
// wind value: the routine at the offset 0x816f of the fourth segment of
// MAIN.EXE takes the byte of the sea area, masks the lower three bits to
// get the direction, and then masks the byte with 0xf8 to keep the
// strength when it stores a new direction into the record.
//
// The eight directions run clockwise from the north. The executable adds
// the direction to the angles of the compass and multiplies the result by
// the unit vectors of the eight points, so a direction is a quarter turn
// of 45 degrees.
//
// # The directions
//
// The eight directions run clockwise from the north. The executable adds
// the direction to a base of the angles of the compass, so the direction is
// a quarter turn of 45 degrees.
const (
	// WindColumns and WindRows are the size of the grid of the sea areas.
	WindColumns, WindRows = 30, 15
	// WindSeaAreas is the number of the sea areas of the map.
	WindSeaAreas = WindColumns * WindRows
	// WindSeaAreaSize is the size of a sea area in pixels.
	WindSeaAreaSize = 72
)

// WindTable holds the three tables of the file.
type WindTable struct {
	// Direction holds the direction of the wind of every sea area.
	Direction []byte
	// Strength holds the strength of the wind of every sea area.
	Strength []byte
	// Current holds the direction and the strength of the currents.
	Current []byte
}

// WindFile is the name of the file of the release.
const WindFile = "WINDCUR.DAT"

// WindTableFile is the name of the derived file.
const WindTableFile = "wind_current.csv"

// ParseWindTable reads the tables of the wind out of the bytes of the file.
func ParseWindTable(data []byte) (*WindTable, error) {
	if len(data) < 3*WindSeaAreas {
		return nil, fmt.Errorf("the wind file holds %d bytes, but %d are needed",
			len(data), 3*WindSeaAreas)
	}
	t := &WindTable{
		Direction: make([]byte, WindSeaAreas),
		Strength:  make([]byte, WindSeaAreas),
		Current:   make([]byte, WindSeaAreas),
	}
	for i := 0; i < WindSeaAreas; i++ {
		t.Direction[i] = data[i] & 0x07
		t.Strength[i] = (data[i] >> 3) & 0x1f
		t.Current[i] = data[WindSeaAreas+i] & 0x07
	}
	return t, nil
}

// SeaArea returns the index of the sea area that holds a position.
//
// The position is in the pixels of the world map.
func SeaArea(x, y int) int {
	column := x / WindSeaAreaSize
	row := y / WindSeaAreaSize
	if column < 0 {
		column = 0
	}
	if row < 0 {
		row = 0
	}
	if column >= WindColumns {
		column = WindColumns - 1
	}
	if row >= WindRows {
		row = WindRows - 1
	}
	return column + row*WindColumns
}

// ExportWindTable converts the wind file of the release and writes the
// result into outDir.
//
// The derived file is a CSV with one row per sea area, holding the column
// and the row of the sea area, the direction and the strength of the wind
// and the direction of the currents.
func ExportWindTable(rawDir, outDir string, log Logger) (files, problems []string) {
	path := filepath.Join(rawDir, WindFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("the wind: %v", err)}
	}
	table, err := ParseWindTable(data)
	if err != nil {
		return nil, []string{fmt.Sprintf("the wind: %v", err)}
	}

	out := filepath.Join(outDir, WindTableFile)
	f, err := os.Create(out)
	if err != nil {
		return nil, []string{fmt.Sprintf("the wind: %v", err)}
	}
	defer f.Close()

	if _, err := f.WriteString("column,row,direction,strength,current\n"); err != nil {
		return nil, []string{fmt.Sprintf("the wind: %v", err)}
	}
	for row := 0; row < WindRows; row++ {
		for column := 0; column < WindColumns; column++ {
			i := column + row*WindColumns
			line := fmt.Sprintf("%d,%d,%d,%d,%d\n",
				column, row, table.Direction[i], table.Strength[i], table.Current[i])
			if _, err := f.WriteString(line); err != nil {
				return nil, []string{fmt.Sprintf("the wind: %v", err)}
			}
		}
	}
	log.Info("wrote %s: %d sea areas of %dx%d pixels",
		out, WindSeaAreas, WindColumns, WindRows)
	return []string{out}, nil
}
