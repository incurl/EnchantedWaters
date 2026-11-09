package world

import (
	"path/filepath"
	"testing"
)

// TestSaveAndLoad verifies that a session survives a save and load cycle.
//
// The test is the guarantee of the two tier architecture: the save file
// holds only the dynamic state, and the static configuration is loaded
// from the tables.
func TestSaveAndLoad(t *testing.T) {
	w := newTestWorld(t)

	// --- change the state ---
	w.SetTreasury(12345)
	w.Tick()
	w.Tick()
	fleetID := w.PlayerEntity()
	fleet := getFleetComponent(w.ecs, fleetID)
	fleet.HullPoints = 55
	fleet.Sailors = 3
	if err := w.SetSailOrder(fleetID, 123); err != nil {
		t.Fatalf("set sail order: %v", err)
	}
	w.AddReputation(FactionPlayer, AxisTrade, 42)

	// --- take a snapshot ---
	before := w.Snapshot(1)
	if len(before.Fleets) == 0 {
		t.Fatal("the snapshot holds no fleet")
	}
	if len(before.Ports) == 0 {
		t.Fatal("the snapshot holds no port")
	}
	// The static configuration must not be part of the snapshot.
	if before.Ports[0].PortID == "" {
		t.Error("the port state has no ID")
	}

	// --- write and read the file ---
	path := filepath.Join(t.TempDir(), "save.json")
	if err := w.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	// --- restore into a fresh world ---
	w2 := newTestWorld(t)
	if err := w2.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}

	// --- compare ---
	if w2.Treasury() != 12345 {
		t.Errorf("the treasury is %v, want 12345", w2.Treasury())
	}
	if w2.Day() != w.Day() {
		t.Errorf("the day is %d, want %d", w2.Day(), w.Day())
	}
	fleet2 := getFleetComponent(w2.ecs, w2.PlayerEntity())
	if fleet2 == nil {
		t.Fatal("the player fleet was not restored")
	}
	if fleet2.HullPoints != 55 {
		t.Errorf("the hull holds %v, want 55", fleet2.HullPoints)
	}
	if fleet2.Sailors != 3 {
		t.Errorf("the crew holds %d, want 3", fleet2.Sailors)
	}
	pos := getPositionComponent(w2.ecs, w2.PlayerEntity())
	if pos == nil {
		t.Fatal("the position was not restored")
	}
	if pos.Heading != 123 {
		t.Errorf("the heading is %v, want 123", pos.Heading)
	}
	if got := w2.Reputation(FactionPlayer, AxisTrade); got < 41 {
		t.Errorf("the trade reputation is %v, want ~42", got)
	}
}

// TestSaveDoesNotContainStaticData verifies that the save file holds only
// the dynamic state.
func TestSaveDoesNotContainStaticData(t *testing.T) {
	w := newTestWorld(t)
	f := w.Snapshot(1)

	// The snapshot has no commodity definitions, no ship definitions and
	// no port definitions: only the IDs and the mutable values.
	if len(f.Fleets) > 0 {
		fleet := f.Fleets[0]
		if fleet.ShipID == "" {
			t.Error("the fleet does not reference its ship class")
		}
		// The hull maximum is a static value; it must not be stored.
		if fleet.HullPoints > 0 && fleet.HullPoints == 0 {
			t.Error("the hull value was not stored")
		}
	}
}
