package asset

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadWind verifies the reading of the table of the wind.
func TestLoadWind(t *testing.T) {
	dir := t.TempDir()
	content := "column,row,direction,strength,current\n" +
		"0,0,3,5,2\n" +
		"1,0,7,0,4\n" +
		"0,1,1,31,0\n"
	path := filepath.Join(dir, ArtFileWindCurrent)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing the table: %v", err)
	}
	table := LoadWind(dir)
	if table == nil {
		t.Fatal("the table was not read")
	}
	cell := table.Cell(0, 0)
	if cell.Direction != 3 || cell.Strength != 5 || cell.Current != 2 {
		t.Errorf("the cell (0,0) is %+v, want the direction 3, the strength 5 and the current 2", cell)
	}
	if got := table.Cell(1, 0).Direction; got != 7 {
		t.Errorf("the direction of the cell (1,0) is %d, want 7", got)
	}
	if got := table.Cell(0, 1).Strength; got != 31 {
		t.Errorf("the strength of the cell (0,1) is %d, want 31", got)
	}
}

// TestLoadWindMissing verifies that a missing file is not an error.
func TestLoadWindMissing(t *testing.T) {
	if table := LoadWind(t.TempDir()); table != nil {
		t.Errorf("a missing file gave %v, want nothing", table)
	}
}

// TestWindCellOutOfBounds verifies that a position outside of the map is
// handled without a panic.
func TestWindCellOutOfBounds(t *testing.T) {
	table := &WindTable{Cells: make([]WindCell, WindSeaAreas)}
	for _, c := range [][2]int{{-1, 0}, {0, -1}, {WindColumns, 0}, {0, WindRows}} {
		if got := table.Cell(c[0], c[1]); got != (WindCell{}) {
			t.Errorf("the cell (%d,%d) is %+v, want the zero cell", c[0], c[1], got)
		}
	}
}

// TestWindSeaArea verifies the division of the map into the sea areas.
func TestWindSeaArea(t *testing.T) {
	table := &WindTable{Cells: make([]WindCell, WindSeaAreas)}
	table.Cells[0] = WindCell{Direction: 1}
	table.Cells[1] = WindCell{Direction: 2}
	table.Cells[WindColumns] = WindCell{Direction: 3}
	if got := table.SeaArea(0, 0).Direction; got != 1 {
		t.Errorf("the wind at the origin is %d, want 1", got)
	}
	if got := table.SeaArea(72, 0).Direction; got != 2 {
		t.Errorf("the wind at (72,0) is %d, want 2", got)
	}
	if got := table.SeaArea(0, 72).Direction; got != 3 {
		t.Errorf("the wind at (0,72) is %d, want 3", got)
	}
}
