// Package sail holds the sailing model of the original game.
//
// Uncharted Waters II does not model the wind as a continuous angle. The
// compass has eight directions, the wind blows from one of them at an
// integer strength, and a hull's response is a lookup in a small table.
// The table is what makes the choice of hull matter: a square rig drives
// hard before the wind and loses most of its drive close hauled, while a
// lateen rig gives up almost nothing.
//
// The model below reproduces that arithmetic exactly. Given a hull, a crew
// and the wind, the speed over the ground before the current is
//
//	δ      = octants between the heading and the wind, folded into 0..4
//	f      = windFactor[sailType][δ]
//	w      = wind speed, floored at 3 for a rowed hull
//	drive  = power · f · w / 150
//	crew   = min(1, crew / minCrew)
//	cargo  = min(1.5, 1.8 - (payload + crew) / capacity)
//	skill  = (10 + navigationLevel) / 10
//	base   = min(30, drive · crew · cargo · skill)
//	speed  = base · tacking' · seamanship / 75
//
// where tacking' is tacking/100 when δ ≥ 3 (the ship has to beat) and 1
// otherwise. The cap of 30 and the divisor of 75 are the original's.
//
// Two quirks of the original are reproduced rather than fixed, because the
// point of this package is fidelity: a stronger headwind makes a ship
// faster (the wind speed multiplies in regardless of δ), and the wind
// direction still matters for a rowed hull when the wind speed is zero.
package sail

import "math"

// The constants of the model.
const (
	// MaxBaseSpeed caps the base speed before the tacking and seamanship
	// factors are applied.
	MaxBaseSpeed = 30.0
	// PropulsionDivisor turns power × wind factor × wind speed into a
	// speed.
	PropulsionDivisor = 150.0
	// OarWindFloor is the wind speed a rowed hull may always count on.
	OarWindFloor = 3.0
	// NeutralSeamanship is the seamanship at which the seamanship factor
	// is exactly 1. It is the value the original gives a starting crew.
	NeutralSeamanship = 75.0
	// MaxCargoFactor bounds the cargo factor; a light ship reaches it.
	MaxCargoFactor = 1.5
	// TackingThreshold is the wind angle, in octants, from which the rig
	// has to beat and loses tacking/100 of its drive.
	TackingThreshold = 3
	// CompassOctants is the number of directions on the compass.
	CompassOctants = 8
	// KnotsPerWindUnit converts the wind scale of this project's sea zones
	// (knots) into the 0..7 scale of the original game.
	//
	// The calibration keeps ordinary weather below the saturation point of
	// the model. The derived zones run from 12 to 17 knots; at 3.5 knots
	// to the unit that is 3.4 to 4.9, so a ship in a fair wind has room to
	// gain from a better hull instead of sitting on the cap of 30. A
	// stronger mapping would put every hull at the cap and flatten the
	// table into a single speed.
	KnotsPerWindUnit = 3.5
	// MaxWindUnits is the top of the original's wind scale.
	MaxWindUnits = 7.0
)

// windFactors is the wind factor table, indexed by sail type (1..6) and by
// the number of octants between the heading and the wind (0 = running
// before it, 4 = dead to windward).
var windFactors = [7][5]float64{
	0: {}, // sail type 0 is unused
	1: {10, 10, 8, 6, 4},
	2: {8, 9, 9, 7, 6},
	3: {10, 10, 10, 8, 6},
	4: {9, 10, 10, 9, 7},
	5: {9, 10, 10, 9, 7},
	6: {8, 10, 10, 10, 8},
}

// WindFactor returns the wind factor of a sail type at a given number of
// octants from the wind. Unknown sail types and out of range angles fall
// back to the most conservative entry rather than to zero, so that a
// malformed table cannot stop a ship dead in the water.
func WindFactor(sailType, delta int) float64 {
	if delta < 0 {
		delta = -delta
	}
	if delta >= 5 {
		delta = 4
	}
	if sailType < 1 || sailType >= len(windFactors) {
		return windFactors[5][delta]
	}
	return windFactors[sailType][delta]
}

// CompassDirection converts a bearing in degrees (0 = north, clockwise)
// into one of the eight compass directions.
func CompassDirection(bearingDeg float64) int {
	octant := int(math.Floor(bearingDeg/45 + 0.5))
	octant %= CompassOctants
	if octant < 0 {
		octant += CompassOctants
	}
	return octant
}

// CompassDelta returns the number of octants between two compass
// directions, folded into 0..4 so that a wind from the beam is the same
// whether it comes from port or from starboard.
func CompassDelta(a, b int) int {
	d := a - b
	if d < 0 {
		d = -d
	}
	d %= CompassOctants
	if d > CompassOctants/2 {
		d = CompassOctants - d
	}
	return d
}

// WindUnits converts a wind speed in knots into the 0..7 scale of the
// original game.
func WindUnits(knots float64) float64 {
	units := knots / KnotsPerWindUnit
	if units < 0 {
		units = 0
	}
	if units > MaxWindUnits {
		units = MaxWindUnits
	}
	return units
}

// Conditions is one evaluation of the model: a hull, a crew and the wind.
type Conditions struct {
	// Power is the propulsion of the hull.
	Power int
	// Tacking is the percentage of drive kept when beating.
	Tacking int
	// SailType selects the row of the wind factor table.
	SailType int
	// MinCrew is the crew needed to work the hull at full efficiency.
	MinCrew int
	// HasOars floors the wind a rowed hull can use.
	HasOars bool
	// Capacity is the hold, in tons.
	Capacity float64
	// Crew is the crew aboard.
	Crew int
	// Payload is the cargo aboard, in tons.
	Payload float64
	// NavigationLevel is the navigator's skill; the original's scale runs
	// from 1 to about 30.
	NavigationLevel float64
	// Seamanship is the crew's seamanship, on the same scale as
	// NeutralSeamanship.
	Seamanship float64
	// Heading is the course, in degrees.
	Heading float64
	// WindDirection is where the wind blows from, in degrees.
	WindDirection float64
	// WindSpeed is the wind strength on the original's 0..7 scale.
	WindSpeed float64
}

// Speed returns the speed over the ground in the original's units. The
// current is not part of the model; it is added by the caller as a vector.
func (c Conditions) Speed() float64 {
	delta := CompassDelta(CompassDirection(c.Heading), CompassDirection(c.WindDirection))
	factor := WindFactor(c.SailType, delta)

	wind := c.WindSpeed
	if c.HasOars && wind < OarWindFloor {
		wind = OarWindFloor
	}

	drive := float64(c.Power) * factor * wind / PropulsionDivisor

	crewFactor := 1.0
	if c.MinCrew > 0 {
		crewFactor = math.Min(1, float64(c.Crew)/float64(c.MinCrew))
	}

	cargoFactor := MaxCargoFactor
	if c.Capacity > 0 {
		cargoFactor = math.Min(MaxCargoFactor, 1.8-(c.Payload+float64(c.Crew))/c.Capacity)
	}
	if cargoFactor < 0 {
		cargoFactor = 0
	}

	skillFactor := (10 + c.NavigationLevel) / 10

	base := math.Min(MaxBaseSpeed, drive*crewFactor*cargoFactor*skillFactor)

	tackingFactor := 1.0
	if delta >= TackingThreshold && c.Tacking > 0 {
		tackingFactor = float64(c.Tacking) / 100
	}

	seamanship := c.Seamanship
	if seamanship <= 0 {
		seamanship = NeutralSeamanship
	}

	return base * tackingFactor * seamanship / NeutralSeamanship
}

// TypicalSpeed is the speed of a hull in ordinary conditions: a wind two
// points off the bow at strength 5, a full crew, an empty hold and a
// starting navigator. It is the figure to use when a voyage has to be
// estimated rather than simulated.
func TypicalSpeed(power, sailType, minCrew, capacity int) float64 {
	return Conditions{
		Power:           power,
		Tacking:         100,
		SailType:        sailType,
		MinCrew:         minCrew,
		Capacity:        float64(capacity),
		Crew:            minCrew,
		NavigationLevel: 1,
		Seamanship:      NeutralSeamanship,
		Heading:         90,
		WindDirection:   0,
		WindSpeed:       5,
	}.Speed()
}
