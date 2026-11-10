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
// The world map of the original game is 2160 columns by 1080 rows of tiles.
// It is divided into sea areas of 72 by 72 tiles, and there are 30 columns
// and 15 rows of them, so 450 sea areas in all.
//
// The division is confirmed by the executable: the routine that reads the
// wind divides the position of a ship by 72 and uses the quotient as the
// index of the sea area.
//
// # The tables
//
// The file holds three tables of 450 bytes:
//
//	bytes   0..450   the wind of the summer
//	bytes 450..900   the wind of the winter
//	bytes 900..1350  the currents
//
// The loader of the release reads the three tables in that order: it reads
// 450 bytes into one buffer, then 450 bytes into a second one, and then
// the last 450 bytes. The routine is at the offset 0x6763 of the fourth
// segment of MAIN.EXE.
//
// The first two tables are the wind of the summer and of the winter, and
// the third one holds the currents. The game tells the two seasons apart
// by the month: the month is at the offset 0x735 of the data, and the
// months from three to eight are the summer.
//
// # The byte of a sea area
//
// Every byte of a table packs two numbers into one byte:
//
//	bits 0 to 2  the direction, a number from 0 to 7
//	bit  3       the flag of a strong wind
//	bits 4 to 7  the strength, a number from 0 to 15
//
// The layout is the one of the record of a sea area, which the game keeps
// in memory: the record is 96 bytes long and holds the wind at its offset
// 0x13. The routine at the offset 0x9703 of the fourth segment of
// MAIN.EXE takes the byte of the record, masks it with 0xf8 to keep the
// strength and the flag, and writes a new direction into the lower three
// bits. The routine at the offset 0x816f masks the lower three bits to
// read the direction, and the routine at the offset 0x971f sets the flag
// with an "or" of the bit 3.
//
// The eight directions run clockwise from the north. The executable adds
// the direction to the angles of the compass and multiplies the result by
// the unit vectors of the eight points, so a direction is a quarter turn
// of 45 degrees.
const (
	// WindColumns and WindRows are the size of the grid of the sea areas.
	WindColumns, WindRows = 30, 15
	// WindSeaAreas is the number of the sea areas of the map.
	WindSeaAreas = WindColumns * WindRows
	// WindSeaAreaSize is the size of a sea area in tiles.
	WindSeaAreaSize = 72
	// WindMaxStrength is the top of the strength scale of the original.
	WindMaxStrength = 15
)

// WindTable holds the three tables of the file.
type WindTable struct {
	// SummerDirection holds the direction of the wind of the summer.
	SummerDirection []byte
	// SummerStrength holds the strength of the wind of the summer.
	SummerStrength []byte
	// WinterDirection holds the direction of the wind of the winter.
	WinterDirection []byte
	// WinterStrength holds the strength of the wind of the winter.
	WinterStrength []byte
	// CurrentDirection holds the direction of the currents.
	CurrentDirection []byte
	// CurrentStrength holds the strength of the currents.
	CurrentStrength []byte
}

// WindFile is the name of the file of the release.
const WindFile = "WINDCUR.DAT"

// WindTableFile is the name of the derived file.
const WindTableFile = "wind_current.csv"

// ParseWindTable reads the tables of the wind out of the bytes of the file.
//
// The function returns the three tables of the file: the wind of the
// summer, the wind of the winter and the currents.
func ParseWindTable(data []byte) (*WindTable, error) {
	if len(data) < 3*WindSeaAreas {
		return nil, fmt.Errorf("the wind file holds %d bytes, but %d are needed",
			len(data), 3*WindSeaAreas)
	}
	t := &WindTable{
		SummerDirection:  make([]byte, WindSeaAreas),
		SummerStrength:   make([]byte, WindSeaAreas),
		WinterDirection:  make([]byte, WindSeaAreas),
		WinterStrength:   make([]byte, WindSeaAreas),
		CurrentDirection: make([]byte, WindSeaAreas),
		CurrentStrength:  make([]byte, WindSeaAreas),
	}
	for i := 0; i < WindSeaAreas; i++ {
		summer := data[i]
		winter := data[WindSeaAreas+i]
		current := data[2*WindSeaAreas+i]
		t.SummerDirection[i] = summer & 0x07
		t.SummerStrength[i] = (summer >> 4) & 0x0f
		t.WinterDirection[i] = winter & 0x07
		t.WinterStrength[i] = (winter >> 4) & 0x0f
		t.CurrentDirection[i] = current & 0x07
		t.CurrentStrength[i] = (current >> 4) & 0x0f
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
// of the summer and of the winter, and the direction and the strength of
// the currents.
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

	header := "column,row,summer_direction,summer_strength," +
		"winter_direction,winter_strength,current_direction,current_strength\n"
	if _, err := f.WriteString(header); err != nil {
		return nil, []string{fmt.Sprintf("the wind: %v", err)}
	}
	for row := 0; row < WindRows; row++ {
		for column := 0; column < WindColumns; column++ {
			i := column + row*WindColumns
			line := fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%d\n",
				column, row,
				table.SummerDirection[i], table.SummerStrength[i],
				table.WinterDirection[i], table.WinterStrength[i],
				table.CurrentDirection[i], table.CurrentStrength[i])
			if _, err := f.WriteString(line); err != nil {
				return nil, []string{fmt.Sprintf("the wind: %v", err)}
			}
		}
	}
	log.Info("wrote %s: %d sea areas of %dx%d tiles",
		out, WindSeaAreas, WindColumns, WindRows)
	return []string{out}, nil
}
