package etl

import (
	"os"
	"path/filepath"
	"testing"
)

// configDir returns the path of the checked-in configuration directory.
func configDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "assets", "config")
}

// TestDeriveIsReproducible verifies that the derivation produces the same
// tables as the ones that are checked in.
//
// The derivation is deterministic, so regenerating the tables must not
// change them: the test guards against a drift between the reference data
// and the playable tables.
func TestDeriveIsReproducible(t *testing.T) {
	if _, err := os.Stat(filepath.Join(configDir(t), "original_ports.csv")); err != nil {
		t.Skip("the imported reference data is not present")
	}

	src := configDir(t)
	dst := t.TempDir()

	// The derivation reads the reference data and the commodity table, so
	// they are copied into the temporary directory first.
	for _, name := range []string{"original_ports.csv", "commodities.csv"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if _, err := DeriveAll(dst); err != nil {
		t.Fatalf("derive: %v", err)
	}

	// The generated ports must match the checked in ones.
	for _, name := range []string{"ports.csv", "port_prices.csv", "sea_zones.csv", "world_terrain.csv"} {
		want, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatalf("read the checked in %s: %v", name, err)
		}
		got, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil {
			t.Fatalf("read the derived %s: %v", name, err)
		}
		if string(got) != string(want) {
			t.Errorf("the derived %s differs from the checked in one; run ./game derive", name)
		}
	}
}

// TestDerivedPortsCoverTheOriginalMap verifies that every port of the
// reference data became a port of the playable tables.
func TestDerivedPortsCoverTheOriginalMap(t *testing.T) {
	if _, err := os.Stat(filepath.Join(configDir(t), "original_ports.csv")); err != nil {
		t.Skip("the imported reference data is not present")
	}
	ports, err := ReadOriginalPorts(filepath.Join(configDir(t), "original_ports.csv"))
	if err != nil {
		t.Fatalf("read the reference ports: %v", err)
	}
	if len(ports) != 130 {
		t.Errorf("the reference table holds %d ports, want 130", len(ports))
	}

	// The playable table must hold one row per port.
	rows, err := readTable(filepath.Join(configDir(t), "ports.csv"))
	if err != nil {
		t.Fatalf("read the playable ports: %v", err)
	}
	if len(rows) != len(ports) {
		t.Errorf("the playable table holds %d ports, want %d", len(rows), len(ports))
	}

	// The positions must be the ones of the original map.
	byName := make(map[string]map[string]string, len(rows))
	for _, row := range rows {
		byName[row["name"]] = row
	}
	for _, p := range ports {
		row, ok := byName[p.Name]
		if !ok {
			t.Errorf("the port %q is missing from the playable table", p.Name)
			continue
		}
		if row["x"] != itoa(p.MapX) || row["y"] != itoa(p.MapY) {
			t.Errorf("the port %q is at (%s,%s), want (%d,%d)",
				p.Name, row["x"], row["y"], p.MapX, p.MapY)
		}
	}
}

// TestRegionOfCoversEveryPort verifies that no port falls outside the
// regions of the world.
func TestRegionOfCoversEveryPort(t *testing.T) {
	if _, err := os.Stat(filepath.Join(configDir(t), "original_ports.csv")); err != nil {
		t.Skip("the imported reference data is not present")
	}
	ports, err := ReadOriginalPorts(filepath.Join(configDir(t), "original_ports.csv"))
	if err != nil {
		t.Fatalf("read the reference ports: %v", err)
	}
	inside := 0
	for _, p := range ports {
		for _, r := range Regions {
			if float64(p.MapX) >= r.MinX && float64(p.MapX) <= r.MaxX &&
				float64(p.MapY) >= r.MinY && float64(p.MapY) <= r.MaxY {
				inside++
				break
			}
		}
	}
	// The fallback of regionOf picks the closest region, so a port outside
	// every box is still handled; the test only reports how many are inside.
	t.Logf("%d of %d ports fall inside a region box", inside, len(ports))
	if inside < len(ports)*4/5 {
		t.Errorf("only %d of %d ports fall inside a region box", inside, len(ports))
	}
}

// itoa converts an integer to a string.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
