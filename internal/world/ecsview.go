package world

import (
	"sort"
	"strconv"

	"github.com/peter/enchantedwaters/internal/ecs"
)

// This file is the thin adapter between the world simulation and the ECS.
//
// The simulation never touches the raw component storages directly; it goes
// through these accessors, which keeps the coupling in one place and makes
// the component layout easy to change.

// Local aliases for the component types, so that the simulation code reads
// naturally.
type (
	// ecsFleetComponent is the dynamic state of a fleet.
	ecsFleetComponent = ecs.FleetComponent
	// ecsPositionComponent is the map position of an entity.
	ecsPositionComponent = ecs.PositionComponent
	// ecsCargoComponent is the cargo manifest of a fleet.
	ecsCargoComponent = ecs.CargoComponent
	// ecsAIComponent is the AI state of a fleet.
	ecsAIComponent = ecs.AIComponent
	// ecsWeatherComponent is the active weather state.
	ecsWeatherComponent = ecs.WeatherComponent
	// ecsReputationComponent is the reputation of a faction.
	ecsReputationComponent = ecs.ReputationComponent
	// ecsSessionComponent is the global clock state.
	ecsSessionComponent = ecs.SessionComponent
	// ecsCharacterComponent describes a crew member.
	ecsCharacterComponent = ecs.CharacterComponent
	// ecsID is the entity identifier type.
	ecsID = ecs.ID
)

// getFleetComponent returns the fleet component of an entity, or nil.
func getFleetComponent(w *ecs.World, id ecs.ID) *ecs.FleetComponent {
	return ecs.GetComponent[ecs.FleetComponent](w, id)
}

// getPositionComponent returns the position component of an entity, or nil.
func getPositionComponent(w *ecs.World, id ecs.ID) *ecs.PositionComponent {
	return ecs.GetComponent[ecs.PositionComponent](w, id)
}

// getCargoComponent returns the cargo component of an entity, or nil.
func getCargoComponent(w *ecs.World, id ecs.ID) *ecs.CargoComponent {
	return ecs.GetComponent[ecs.CargoComponent](w, id)
}

// getAIComponent returns the AI component of an entity, or nil.
func getAIComponent(w *ecs.World, id ecs.ID) *ecs.AIComponent {
	return ecs.GetComponent[ecs.AIComponent](w, id)
}

// getWeatherComponent returns the weather component of an entity, or nil.
func getWeatherComponent(w *ecs.World, id ecs.ID) *ecs.WeatherComponent {
	return ecs.GetComponent[ecs.WeatherComponent](w, id)
}

// getReputationComponent returns the reputation component of a faction, or nil.
//
// Factions are represented by entities whose ID equals the faction ID
// encoded in the entity ID space; the world keeps a dedicated entity per
// faction and looks it up through this helper.
func getReputationComponent(w *ecs.World, id ecs.ID) *ecs.ReputationComponent {
	return ecs.GetComponent[ecs.ReputationComponent](w, id)
}

// getSessionComponent returns the session component, or nil.
func getSessionComponent(w *ecs.World, id ecs.ID) *ecs.SessionComponent {
	return ecs.GetComponent[ecs.SessionComponent](w, id)
}

// getCharacterComponent returns the character component of an entity, or nil.
func getCharacterComponent(w *ecs.World, id ecs.ID) *ecs.CharacterComponent {
	return ecs.GetComponent[ecs.CharacterComponent](w, id)
}

// sortStrings sorts a string slice in place. It exists to avoid pulling the
// sort package into every file of the package.
func sortStrings(s []string) { sort.Strings(s) }

// itoa converts an integer to its decimal representation.
//
// It is a tiny local helper so that the hot paths do not need to pull in
// the strconv package.
func itoa(v int) string { return strconv.Itoa(v) }
