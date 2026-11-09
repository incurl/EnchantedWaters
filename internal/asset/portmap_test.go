package asset

import (
	"testing"
)

// TestLoadPortMaps reads the maps of the towns and verifies their geometry.
func TestLoadPortMaps(t *testing.T) {
	maps, err := LoadPortMaps(artDir(t))
	if err != nil {
		t.Fatalf("LoadPortMaps: %v", err)
	}
	// The release holds one map for every town of the original game.
	if len(maps) < 100 {
		t.Errorf("got %d maps, want at least 100", len(maps))
	}
	for i, m := range maps {
		if m.Width != 96 || m.Height != 96 {
			t.Fatalf("map %d is %dx%d, want 96x96", i, m.Width, m.Height)
		}
		if len(m.Index) != m.Width*m.Height {
			t.Fatalf("map %d holds %d cells, want %d",
				i, len(m.Index), m.Width*m.Height)
		}
		// Every cell names a tile of the set of the town, and a set
		// holds two hundred and forty tiles.
		for _, v := range m.Index {
			if int(v) >= 240 {
				t.Fatalf("map %d names tile %d, want below 240", i, v)
			}
		}
	}
}

// TestLoadChipNumbers reads the table that maps a town to its set of tiles.
func TestLoadChipNumbers(t *testing.T) {
	numbers, err := LoadChipNumbers(artDir(t))
	if err != nil {
		t.Fatalf("LoadChipNumbers: %v", err)
	}
	// The release holds one byte per town.
	if len(numbers) < 100 {
		t.Errorf("got %d entries, want at least 100", len(numbers))
	}
	// The game holds seven sets of tiles, so a number outside the range
	// means that the table and the pictures disagree.
	for i, n := range numbers {
		if n < 0 || n > 6 {
			t.Fatalf("town %d uses set %d, want 0 to 6", i, n)
		}
	}
}

// TestPortMapCell verifies that a cell outside the map is the first tile.
//
// The interface asks for the cells of a town while it draws the window on
// it, and the window can reach past the edge of the map. Those cells must
// give a harmless answer instead of a panic.
func TestPortMapCell(t *testing.T) {
	m := &PortMap{Index: make([]uint8, 4*2), Width: 4, Height: 2}
	m.Index[0] = 7
	if got := m.Cell(0, 0); got != 7 {
		t.Errorf("Cell(0,0) is %d, want 7", got)
	}
	for _, c := range [][2]int{{-1, 0}, {0, -1}, {4, 0}, {0, 2}, {99, 99}} {
		if got := m.Cell(c[0], c[1]); got != 0 {
			t.Errorf("Cell(%d,%d) is %d, want 0", c[0], c[1], got)
		}
	}
	if (*PortMap)(nil).Cell(0, 0) != 0 {
		t.Error("Cell on a nil map is not zero")
	}
}

// TestPortMapFocus verifies that the view opens on the part of the map that
// holds the town.
//
// The town is the part of the map with the most different tiles: the open
// sea uses one tile and the town uses dozens. The test builds a map with a
// band of distinctive tiles in the middle and checks that the window lands
// on the band.
func TestPortMapFocus(t *testing.T) {
	const (
		w    = 8
		h    = 40
		rows = 6
	)
	m := &PortMap{Index: make([]uint8, w*h), Width: w, Height: h}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.Index[y*w+x] = 0
		}
	}
	// The band of the town: the rows from twenty to twenty five, every
	// cell a different tile.
	band := 20
	for y := band; y < band+rows; y++ {
		for x := 0; x < w; x++ {
			m.Index[y*w+x] = uint8(x + 1)
		}
	}

	got := PortMapFocus(m, rows)
	if got != band {
		t.Errorf("PortMapFocus is %d, want %d", got, band)
	}

	// A window that is taller than the map opens at the top.
	if got := PortMapFocus(m, h+10); got != 0 {
		t.Errorf("PortMapFocus of a tall window is %d, want 0", got)
	}
	// A map without a window has no focus.
	if got := PortMapFocus(m, 0); got != 0 {
		t.Errorf("PortMapFocus of a window of no rows is %d, want 0", got)
	}
	if got := PortMapFocus(nil, rows); got != 0 {
		t.Errorf("PortMapFocus of a nil map is %d, want 0", got)
	}
}
