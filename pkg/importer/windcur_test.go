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
	// upper four bits.
	data[0] = 0x03 | 0x05<<4
	data[1] = 0x07
	data[2] = 0x00
	// The second table is the wind of the winter.
	data[WindSeaAreas] = 0x02 | 0x0a<<4
	// The third table holds the currents.
	data[2*WindSeaAreas] = 0x04 | 0x01<<4
	table, err := ParseWindTable(data)
	if err != nil {
		t.Fatalf("parsing the wind: %v", err)
	}
	if table.SummerDirection[0] != 3 {
		t.Errorf("the summer direction of the first sea area is %d, want 3", table.SummerDirection[0])
	}
	if table.SummerStrength[0] != 5 {
		t.Errorf("the summer strength of the first sea area is %d, want 5", table.SummerStrength[0])
	}
	if table.SummerDirection[1] != 7 {
		t.Errorf("the summer direction of the second sea area is %d, want 7", table.SummerDirection[1])
	}
	if table.SummerStrength[1] != 0 {
		t.Errorf("the summer strength of the second sea area is %d, want 0", table.SummerStrength[1])
	}
	if table.WinterDirection[0] != 2 {
		t.Errorf("the winter direction of the first sea area is %d, want 2", table.WinterDirection[0])
	}
	if table.WinterStrength[0] != 10 {
		t.Errorf("the winter strength of the first sea area is %d, want 10", table.WinterStrength[0])
	}
	if table.CurrentDirection[0] != 4 {
		t.Errorf("the current direction of the first sea area is %d, want 4", table.CurrentDirection[0])
	}
	if table.CurrentStrength[0] != 1 {
		t.Errorf("the current strength of the first sea area is %d, want 1", table.CurrentStrength[0])
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
		if table.SummerDirection[i] > 7 {
			t.Fatalf("the summer direction of the sea area %d is %d, want at most 7",
				i, table.SummerDirection[i])
		}
		if table.SummerStrength[i] > WindMaxStrength {
			t.Fatalf("the summer strength of the sea area %d is %d, want at most %d",
				i, table.SummerStrength[i], WindMaxStrength)
		}
		if table.WinterDirection[i] > 7 {
			t.Fatalf("the winter direction of the sea area %d is %d, want at most 7",
				i, table.WinterDirection[i])
		}
		if table.WinterStrength[i] > WindMaxStrength {
			t.Fatalf("the winter strength of the sea area %d is %d, want at most %d",
				i, table.WinterStrength[i], WindMaxStrength)
		}
		if table.CurrentDirection[i] > 7 {
			t.Fatalf("the current direction of the sea area %d is %d, want at most 7",
				i, table.CurrentDirection[i])
		}
		if table.CurrentStrength[i] > WindMaxStrength {
			t.Fatalf("the current strength of the sea area %d is %d, want at most %d",
				i, table.CurrentStrength[i], WindMaxStrength)
		}
	}
}
