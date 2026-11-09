package world

import (
	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
	"github.com/peter/enchantedwaters/internal/sail"
)

// This file adapts the sailing model of the original game (package sail) to
// the components of the world. The arithmetic lives there; what lives here
// is the reading of the fleet and the hull.
//
// The model needs a navigation level and a seamanship for the crew. This
// project has no skill system yet, so both are held at the values the
// original gives a starting crew, which makes their factors neutral: the
// voyage is neither helped nor hindered by a skill that does not exist.

// neutralNavigationLevel is the navigation level of a starting crew in the
// original game. The skill factor is (10 + level) / 10.
const neutralNavigationLevel = 1.0

// sailingSpeed returns the speed of a fleet over the ground, before the
// current is added, in the units of the original game.
//
// It reads the hull, the crew and the cargo from the world and hands them
// to the model together with the wind.
func (w *World) sailingSpeed(id ecsID, fleet *ecsFleetComponent, ship *config.Ship, pos *ecs.PositionComponent, windBearing, windUnits float64) float64 {
	payload := 0.0
	if cargo := getCargoComponent(w.ecs, id); cargo != nil {
		payload = cargo.UsedWeight
	}
	return sail.Conditions{
		Power:           ship.Power,
		Tacking:         ship.Tacking,
		SailType:        ship.SailType,
		MinCrew:         ship.MinCrew,
		HasOars:         ship.HasOars,
		Capacity:        ship.CargoCapacity,
		Crew:            fleet.Sailors,
		Payload:         payload,
		NavigationLevel: neutralNavigationLevel,
		Seamanship:      sail.NeutralSeamanship,
		Heading:         pos.Heading,
		WindDirection:   windBearing,
		WindSpeed:       windUnits,
	}.Speed()
}
