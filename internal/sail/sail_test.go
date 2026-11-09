package sail

import (
	"math"
	"testing"
)

// TestSpeedMatchesTheHandComputation checks the arithmetic of the model
// against a case worked out by hand.
//
// The Buss: power 60, sail type 1, minimum crew 50, hold 500, tacking 50.
// With 50 hands aboard and 250 tons in the hold:
//
//	cargo  = min(1.5, 1.8 - (250 + 50)/500) = 1.2
//	skill  = (10 + 0)/10                    = 1
//
// running before the wind at strength 5, the sail type 1 factor is 10:
//
//	drive  = 60 · 10 · 5 / 150 = 20
//	base   = min(30, 20 · 1 · 1.2 · 1) = 24
//	speed  = 24 · 1 · 75/75 = 24
//
// dead to windward the factor is 4 and the tacking factor applies:
//
//	drive  = 60 · 4 · 5 / 150 = 8
//	base   = min(30, 8 · 1 · 1.2) = 9.6
//	speed  = 9.6 · 50/100 · 1 = 4.8
func TestSpeedMatchesTheHandComputation(t *testing.T) {
	buss := Conditions{
		Power: 60, Tacking: 50, SailType: 1, MinCrew: 50,
		Capacity: 500, Crew: 50, Payload: 250,
		NavigationLevel: 0, Seamanship: NeutralSeamanship,
		WindDirection: 0, WindSpeed: 5,
	}

	running := buss
	running.Heading = 0
	if got := running.Speed(); math.Abs(got-24) > 1e-9 {
		t.Errorf("running before the wind = %.6f, want 24", got)
	}

	beating := buss
	beating.Heading = 180
	if got := beating.Speed(); math.Abs(got-4.8) > 1e-9 {
		t.Errorf("dead to windward = %.6f, want 4.8", got)
	}
}

// TestCargoFactor verifies the load penalty: a light ship is faster, and
// the factor is bounded above by 1.5.
func TestCargoFactor(t *testing.T) {
	light := Conditions{Power: 60, SailType: 1, MinCrew: 50, Capacity: 500, Crew: 50, Seamanship: NeutralSeamanship, WindSpeed: 5}
	heavy := light
	heavy.Payload = 400
	if light.Speed() <= heavy.Speed() {
		t.Errorf("an empty hold (%.3f) should be faster than a full one (%.3f)", light.Speed(), heavy.Speed())
	}

	// A crew smaller than the minimum costs drive in proportion.
	short := heavy
	short.Crew = 25
	if short.Speed() >= heavy.Speed() {
		t.Errorf("half a crew (%.3f) should be slower than a full one (%.3f)", short.Speed(), heavy.Speed())
	}

	// The cap holds however light the ship is.
	empty := Conditions{Power: 1000, SailType: 6, MinCrew: 1, Capacity: 1000, Crew: 1, Seamanship: NeutralSeamanship, WindSpeed: 7}
	if got := empty.Speed(); got > MaxBaseSpeed+1e-9 {
		t.Errorf("speed %.3f exceeds the cap of %.0f", got, MaxBaseSpeed)
	}
}

// TestOarsFloorTheWind verifies that a rowed hull keeps moving in a calm,
// and that a hull without oars does not.
func TestOarsFloorTheWind(t *testing.T) {
	calm := Conditions{Power: 85, SailType: 6, MinCrew: 5, Capacity: 120, Crew: 5, Seamanship: NeutralSeamanship, WindSpeed: 0}
	if got := calm.Speed(); got != 0 {
		t.Errorf("a sailing hull in a dead calm should not move, got %.3f", got)
	}

	rowed := calm
	rowed.HasOars = true
	if got := rowed.Speed(); got <= 0 {
		t.Errorf("a rowed hull in a dead calm should still move, got %.3f", got)
	}
}

// TestWindUnits verifies the conversion of the project's knot scale into
// the 0..7 scale of the original.
func TestWindUnits(t *testing.T) {
	cases := []struct{ knots, want float64 }{
		{0, 0}, {12, 12.0 / 3.5}, {15, 15.0 / 3.5}, {17, 17.0 / 3.5}, {100, MaxWindUnits}, {-5, 0},
	}
	for _, c := range cases {
		if got := WindUnits(c.knots); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("WindUnits(%.0f) = %.4f, want %.4f", c.knots, got, c.want)
		}
	}
}

// TestWindFactorTable verifies the corners of the sail table and the
// fallback for an unknown sail type.
func TestWindFactorTable(t *testing.T) {
	if got := WindFactor(1, 0); got != 10 {
		t.Errorf("a square rig before the wind = %.0f, want 10", got)
	}
	if got := WindFactor(1, 4); got != 4 {
		t.Errorf("a square rig to windward = %.0f, want 4", got)
	}
	if got := WindFactor(6, 4); got != 8 {
		t.Errorf("a lateen rig to windward = %.0f, want 8", got)
	}
	// An unknown sail type falls back to the most conservative row.
	if got := WindFactor(99, 0); got != WindFactor(5, 0) {
		t.Errorf("unknown sail type = %.0f, want the type 5 fallback", got)
	}
	// Out of range angles clamp rather than panic.
	if got := WindFactor(1, 9); got != 4 {
		t.Errorf("delta 9 = %.0f, want the delta 4 entry", got)
	}
}
