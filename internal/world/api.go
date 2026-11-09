package world

import (
	"github.com/peter/enchantedwaters/internal/ecs"
)

// This file exposes the read-only and the action API of the world to the
// user interface.
//
// Every method here is safe to call from the render thread; the world
// takes care of the locking internally.

// Weather returns the active weather component, or nil.
func (w *World) Weather() *ecs.WeatherComponent {
	return w.weatherComponent()
}

// Speed returns the current simulation speed multiplier.
func (w *World) Speed() int { return w.speed }

// SetSpeed sets the simulation speed multiplier.
//
// A speed of 0 pauses the simulation. The value is clamped to the range
// [0, maxSpeed] to keep the tick loop bounded.
func (w *World) SetSpeed(s int) {
	if s < 0 {
		s = 0
	}
	if s > maxSpeed {
		s = maxSpeed
	}
	w.speed = s
}

// maxSpeed is the highest simulation speed the player can select.
const maxSpeed = 8

// CargoCapacity returns the cargo capacity of the fleet of an entity.
func (w *World) CargoCapacity(fleetID ecs.ID) float64 {
	fleet := getFleetComponent(w.ecs, fleetID)
	if fleet == nil {
		return 0
	}
	ship := w.cfg.Ship(fleet.ShipID)
	if ship == nil {
		return 0
	}
	return ship.CargoCapacity
}

// SailorCapacity returns the maximum crew size of the fleet of an entity.
func (w *World) SailorCapacity(fleetID ecs.ID) int {
	fleet := getFleetComponent(w.ecs, fleetID)
	if fleet == nil {
		return 0
	}
	ship := w.cfg.Ship(fleet.ShipID)
	if ship == nil {
		return 0
	}
	return ship.MaxSailors
}

// FleetSnapshot is a read-only view of a fleet, used by the UI.
type FleetSnapshot struct {
	// Name is the display name of the fleet.
	Name string
	// ShipName is the name of the hull class.
	ShipName string
	// HullPoints is the current durability.
	HullPoints float64
	// MaxHullPoints is the durability of a sound hull.
	MaxHullPoints float64
	// Sailors is the current crew size.
	Sailors int
	// MaxSailors is the crew capacity.
	MaxSailors int
	// Loyalty is the average crew loyalty.
	Loyalty float64
	// Fatigue is the average crew fatigue.
	Fatigue float64
	// Morale is the average crew morale.
	Morale float64
	// Food is the food stock in tons.
	Food float64
	// FreshWater is the fresh water stock in tons.
	FreshWater float64
	// InPort is the port the fleet is docked at.
	InPort string
	// X / Y is the map position.
	X, Y float64
	// Heading is the compass bearing.
	Heading float64
	// Speed is the speed over ground.
	Speed float64
}

// SnapshotFleet returns a read-only view of a fleet.
func (w *World) SnapshotFleet(fleetID ecs.ID) FleetSnapshot {
	var s FleetSnapshot
	fleet := getFleetComponent(w.ecs, fleetID)
	if fleet == nil {
		return s
	}
	s.Name = fleet.Name
	s.HullPoints = fleet.HullPoints
	s.Sailors = fleet.Sailors
	s.Loyalty = fleet.Loyalty
	s.Fatigue = fleet.Fatigue
	s.Morale = fleet.Morale
	s.Food = fleet.Food
	s.FreshWater = fleet.FreshWater
	s.InPort = fleet.InPort
	if ship := w.cfg.Ship(fleet.ShipID); ship != nil {
		s.ShipName = ship.Name
		s.MaxHullPoints = ship.HullPoints
		s.MaxSailors = ship.MaxSailors
	}
	if pos := getPositionComponent(w.ecs, fleetID); pos != nil {
		s.X = pos.X
		s.Y = pos.Y
		s.Heading = pos.Heading
		s.Speed = pos.Speed
	}
	return s
}

// PortSnapshot is a read-only view of a port, used by the UI.
type PortSnapshot struct {
	// ID is the port ID.
	ID string
	// Name is the display name.
	Name string
	// Class is the port class.
	Class string
	// Region is the region of the port.
	Region string
	// X / Y is the map position.
	X, Y float64
	// TaxRate is the tax of the port.
	TaxRate float64
	// ShipyardLevel is the shipyard level.
	ShipyardLevel int
	// TavernLevel is the tavern level.
	TavernLevel int
	// GovernorLevel is the governor level.
	GovernorLevel int
	// ChurchLevel is the church level.
	ChurchLevel int
	// InnLevel is the inn level.
	InnLevel int
}

// SnapshotPort returns a read-only view of a port.
func (w *World) SnapshotPort(portID string) PortSnapshot {
	var s PortSnapshot
	p := w.cfg.Port(portID)
	if p == nil {
		return s
	}
	s.ID = p.ID
	s.Name = p.Name
	s.Class = string(p.Class)
	s.Region = p.Region
	s.X = p.X
	s.Y = p.Y
	s.TaxRate = p.BaseTaxRate
	s.ShipyardLevel = p.ShipyardLevel
	s.TavernLevel = p.TavernLevel
	s.GovernorLevel = p.GovernorLevel
	s.ChurchLevel = p.ChurchLevel
	s.InnLevel = p.InnLevel
	return s
}

// PortIDs returns the deterministic list of the port IDs.
func (w *World) PortIDs() []string { return w.portIDs }

// ReputationSnapshot returns the three reputation axes of a faction.
func (w *World) ReputationSnapshot(factionID string) (adventure, trade, naval float64) {
	return w.Reputation(factionID, AxisAdventure),
		w.Reputation(factionID, AxisTrade),
		w.Reputation(factionID, AxisNaval)
}

// AdvanceDay advances the simulation by one day.
//
// The UI calls it according to the current simulation speed.
func (w *World) AdvanceDay() {
	w.Tick()
	w.dayFraction = 0
}
