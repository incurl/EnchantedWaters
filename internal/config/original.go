package config

// This file declares the types of the tables that were imported from the
// original game assets.
//
// The data of the original game is *reference material*: it holds the
// original wording of the interface, the original dialogues, the names
// of the colonies and the descriptions of the items. It is imported by
// the asset importer and it is loaded by the runtime, exactly like the
// hand written tables.
//
// The tables are kept in their own namespace (the "original_" prefix) so
// that they never collide with the tables of this project, and so that it
// stays obvious which data comes from where.

// OriginalText is one line of an imported text table.
//
// The tables of the original game are plain lists of strings; the
// identifier is the index of the line in the original table.
type OriginalText struct {
	// ID is the index of the line in the original table.
	ID int `csv:"id"`
	// Text is the text of the line. It is optional, because the original
	// tables contain empty lines.
	Text string `csv:"text,opt"`
}

// OriginalColony is one colony or wonder of the original game.
type OriginalColony struct {
	// ID is the index of the colony in the original table.
	ID int `csv:"id"`
	// Name is the name of the colony. It is optional, because the
	// original table ends with empty padding records.
	Name string `csv:"name,opt"`
	// Description is the description of the colony.
	Description string `csv:"description,opt"`
}

// OriginalItemDescription is the description of one item of the original
// game.
//
// The identifier of an item is its index in the original table; the item
// itself is not part of the imported data, only its description.
type OriginalItemDescription struct {
	// ID is the index of the item in the original table.
	ID int `csv:"id"`
	// Description is the description of the item.
	Description string `csv:"description,opt"`
}

// WorldTerrain is the land and sea mask of the world map.
//
// The mask is coarse: every row is a line of cells, and a cell is a block
// of TerrainCellSize pixels of the map. A '#' is land and a '.' is sea.
//
// The mask is derived from the positions of the ports, so the continents
// follow the port clusters of the original world map.
type WorldTerrain struct {
	// Rows holds one line of the mask per row.
	Rows []string
	// CellSize is the size of one cell in map pixels.
	CellSize int
}

// At returns the terrain character of a map position.
//
// The result is '#' for land, '.' for the sea, and it is empty outside of
// the map.
func (t *WorldTerrain) At(x, y int) byte {
	if t == nil || t.CellSize <= 0 || len(t.Rows) == 0 {
		return 0
	}
	cx := x / t.CellSize
	cy := y / t.CellSize
	if cy < 0 || cy >= len(t.Rows) {
		return 0
	}
	row := t.Rows[cy]
	if cx < 0 || cx >= len(row) {
		return 0
	}
	return row[cx]
}

// IsLand reports whether a map position is on land.
func (t *WorldTerrain) IsLand(x, y int) bool { return t.At(x, y) == '#' }

// WorldWind is the wind and the currents of the sea areas of the world map.
//
// The data comes from WINDCUR.DAT of the original game. The world map is
// divided into 30 columns and 15 rows of sea areas, and every sea area
// holds the direction the wind blows to, the strength of the wind and the
// direction of the current.
//
// The directions are numbers from 0 to 7 counted clockwise from the north,
// and the strengths are numbers from 0 to 31.
type WorldWind struct {
	// Directions holds the direction of the wind of every sea area, row
	// by row.
	Directions []uint8
	// Strengths holds the strength of the wind of every sea area.
	Strengths []uint8
	// Currents holds the direction of the current of every sea area.
	Currents []uint8
}

// The geometry of the grid of the sea areas.
const (
	// WindColumns and WindRows are the size of the grid of the sea areas.
	WindColumns, WindRows = 30, 15
	// WindSeaAreas is the number of the sea areas of the map.
	WindSeaAreas = WindColumns * WindRows
	// WindSeaAreaSize is the size of a sea area in tiles.
	WindSeaAreaSize = 72
)

// At returns the wind of the sea area at the given column and row.
//
// The function returns zeroes when the position is outside of the map, so
// a caller never has to check the bounds itself.
func (w *WorldWind) At(column, row int) (direction, strength, current int) {
	if w == nil || column < 0 || row < 0 || column >= WindColumns || row >= WindRows {
		return 0, 0, 0
	}
	i := row*WindColumns + column
	if i >= len(w.Directions) || i >= len(w.Strengths) {
		return 0, 0, 0
	}
	current = 0
	if i < len(w.Currents) {
		current = int(w.Currents[i])
	}
	return int(w.Directions[i]), int(w.Strengths[i]), current
}

// SeaArea returns the wind of the sea area that holds a position.
//
// The position is in the tiles of the world map.
func (w *WorldWind) SeaArea(x, y int) (direction, strength, current int) {
	return w.At(x/WindSeaAreaSize, y/WindSeaAreaSize)
}
