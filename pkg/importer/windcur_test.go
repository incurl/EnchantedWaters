package importer

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseWindTable verifies the layout of the byte of a sea area.
func TestParseWindTable(t *testing.T) {
	data := make([]byte, 3*WindSeaAreas)
	// The direction is in the lower three bits and the strength in the
	// upper five bits.
	data[0] = 0x03 | 0x05<<3
	data[1] = 0x07
	data[2] = 0x00
	table, err := ParseWindTable(data)
	if err != nil {
		t.Fatalf("parsing the wind: %v", err)
	}
	if table.Direction[0] != 3 {
		t.Errorf("the direction of the first sea area is %d, want 3", table.Direction[0])
	}
	if table.Strength[0] != 5 {
		t.Errorf("the strength of the first sea area is %d, want 5", table.Strength[0])
	}
	if table.Direction[1] != 7 {
		t.Errorf("the direction of the second sea area is %d, want 7", table.Direction[1])
	}
	if table.Strength[1] != 0 {
		t.Errorf("the strength of the second sea area is %d, want 0", table.Strength[1])
	}
}

// TestParseWindTableShort verifies that a short file is refused.
func TestParseWindTableShort(t *testing.T) {
	if _, err := ParseWindTable(make([]byte, 10)); err == nil {
		t.Error("a file of ten bytes was accepted")
	}
}

// TestSeaArea verifies the division of the world map into the sea areas.
func TestSeaArea(t *testing.T) {
	cases := []struct {
		x, y, want int
	}{
		{0, 0, 0},
		{71, 71, 0},
		{72, 0, 1},
		{0, 72, WindColumns},
		{2159, 1079, WindSeaAreas - 1},
	}
	for _, c := range cases {
		if got := SeaArea(c.x, c.y); got != c.want {
			t.Errorf("the sea area of (%d,%d) is %d, want %d", c.x, c.y, got, c.want)
		}
	}
}

// TestWindFile verifies the wind file of the release, when it is there.
func TestWindFile(t *testing.T) {
	dir := townRawDir(t)
	path := filepath.Join(dir, WindFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("%s is not present: %v", WindFile, err)
	}
	table, err := ParseWindTable(data)
	if err != nil {
		t.Fatalf("parsing %s: %v", WindFile, err)
	}
	// The file holds three tables of one byte per sea area.
	if len(data) != 3*WindSeaAreas {
		t.Errorf("%s holds %d bytes, want %d", WindFile, len(data), 3*WindSeaAreas)
	}
	for i := 0; i < WindSeaAreas; i++ {
		if table.Direction[i] > 7 {
			t.Fatalf("the direction of the sea area %d is %d, want at most 7",
				i, table.Direction[i])
		}
		if table.Strength[i] > 31 {
			t.Fatalf("the strength of the sea area %d is %d, want at most 31",
				i, table.Strength[i])
		}
	}
}
