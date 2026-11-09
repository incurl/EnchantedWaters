package world

import (
	"math"

	"github.com/peter/enchantedwaters/internal/ecs"
)

// This file implements the docking logic: a fleet that reaches the
// proximity of a port is docked automatically, and a docked fleet that is
// told to leave sets out to sea again.

// dockingRadius is the distance (in map pixels) at which a fleet is
// considered to have reached a port.
//
// The radius is expressed as a fraction of the map width so that it scales
// with the map size configured in the globals table.
func (w *World) dockingRadius() float64 {
	width := float64(w.cfg.Params.MapWidth)
	if width <= 0 {
		width = 1024
	}
	return width * 0.02
}

// checkDocking docks every fleet that has reached a port.
//
// A fleet docks when it is within the docking radius of a port. The port
// is the nearest one; the search is a simple linear scan over the ports,
// which is fast enough for the number of ports in a game world and avoids
// a spatial index that would have to be rebuilt on every map change.
func (w *World) checkDocking() {
	fleetStore := ecs.ComponentArrayOf[ecs.FleetComponent](w.ecs)
	if fleetStore == nil {
		return
	}
	radius := w.dockingRadius()

	fleetStore.Each(func(id ecs.ID, fleet *ecs.FleetComponent) {
		if fleet.InPort != "" {
			// A docked fleet stays docked until it is told to leave.
			return
		}
		pos := getPositionComponent(w.ecs, id)
		if pos == nil {
			return
		}
		portID, dist := w.nearestPort(pos.X, pos.Y)
		if portID == "" || dist > radius {
			return
		}
		fleet.InPort = portID
		pos.Speed = 0
		pos.X = w.cfg.Port(portID).X
		pos.Y = w.cfg.Port(portID).Y
		w.Log("%s docked at %s", fleet.Name, w.cfg.Port(portID).Name)
		w.emit("dock", map[string]any{"fleet": fleet.Name, "port": portID})
	})
}

// nearestPort returns the port closest to a position and its distance.
func (w *World) nearestPort(x, y float64) (string, float64) {
	best := ""
	bestDist := math.Inf(1)
	for _, id := range w.portIDs {
		p := w.cfg.Port(id)
		if p == nil {
			continue
		}
		d := math.Hypot(p.X-x, p.Y-y)
		if d < bestDist {
			bestDist = d
			best = id
		}
	}
	return best, bestDist
}

// SetSailOrder sets the course of the player's fleet.
//
// The fleet starts to move towards the given bearing and leaves the port
// it was docked at.
func (w *World) SetSailOrder(fleetID ecs.ID, heading float64) error {
	fleet := getFleetComponent(w.ecs, fleetID)
	pos := getPositionComponent(w.ecs, fleetID)
	if fleet == nil || pos == nil {
		return &SessionError{Message: "unknown fleet"}
	}
	pos.Heading = normalizeAngle(heading)
	fleet.InPort = ""
	w.Log("%s set sail on heading %.0f", fleet.Name, pos.Heading)
	return nil
}

// SailToPort sets the course of a fleet towards a port.
//
// It returns the bearing that was set, so that the UI can display it.
func (w *World) SailToPort(fleetID ecs.ID, portID string) (float64, error) {
	fleet := getFleetComponent(w.ecs, fleetID)
	pos := getPositionComponent(w.ecs, fleetID)
	port := w.cfg.Port(portID)
	if fleet == nil || pos == nil || port == nil {
		return 0, &SessionError{Message: "unknown port"}
	}
	bearing := bearingBetween(pos.X, pos.Y, port.X, port.Y)
	pos.Heading = bearing
	fleet.InPort = ""
	w.Log("%s makes for %s (bearing %.0f)", fleet.Name, port.Name, bearing)
	return bearing, nil
}

// ArrivedAtPort returns the port the player's fleet is docked at, or the
// empty string when it is at sea.
func (w *World) ArrivedAtPort(fleetID ecs.ID) string {
	fleet := getFleetComponent(w.ecs, fleetID)
	if fleet == nil {
		return ""
	}
	return fleet.InPort
}
