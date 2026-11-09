package world

import (
	"fmt"

	"github.com/peter/enchantedwaters/internal/ecs"
	"github.com/peter/enchantedwaters/internal/save"
)

// This file connects the world to the save system.
//
// The world knows how to serialise its dynamic state and how to restore
// it; the save package knows how to write and read the files. The two
// layers stay separate.

// Snapshot serialises the dynamic state of the world.
//
// The static configuration is not part of the snapshot: it is loaded
// from the tables when the session is restored.
func (w *World) Snapshot(seed uint64) *save.File {
	f := &save.File{
		Version:         save.Version,
		Seed:            seed,
		Day:             w.day,
		Year:            w.year,
		DayFraction:     w.dayFraction,
		Treasury:        w.treasury,
		DiscoveredCount: w.discoveredCount,
		PlayerEntity:    uint64(w.playerEntity),
		WeatherEntity:   uint64(w.weatherEntity),
		Speed:           w.speed,
		Discovered:      make(map[string]int, len(w.discovered)),
	}

	// --- the ports ---
	for _, id := range w.portIDs {
		pd := w.ports[id]
		if pd == nil {
			continue
		}
		pd.mu.Lock()
		f.Ports = append(f.Ports, save.PortState{
			PortID:     id,
			Prices:     copyFloatMap(pd.prices),
			Stock:      copyFloatMap(pd.stock),
			Scarcity:   copyFloatMap(pd.scarcity),
			Noise:      copyFloatMap(pd.noise),
			CyclePhase: pd.cyclePhase,
			Reputation: pd.reputation,
		})
		pd.mu.Unlock()
	}

	// --- the fleets ---
	fleetStore := ecs.ComponentArrayOf[ecs.FleetComponent](w.ecs)
	posStore := ecs.ComponentArrayOf[ecs.PositionComponent](w.ecs)
	cargoStore := ecs.ComponentArrayOf[ecs.CargoComponent](w.ecs)
	aiStore := ecs.ComponentArrayOf[ecs.AIComponent](w.ecs)
	if fleetStore != nil {
		fleetStore.Each(func(id ecs.ID, fleet *ecs.FleetComponent) {
			state := save.FleetState{
				Entity:     uint64(id),
				ShipID:     fleet.ShipID,
				Name:       fleet.Name,
				Owner:      fleet.Owner,
				IsPlayer:   fleet.IsPlayer,
				HullPoints: fleet.HullPoints,
				Food:       fleet.Food,
				FreshWater: fleet.FreshWater,
				Sailors:    fleet.Sailors,
				Loyalty:    fleet.Loyalty,
				Fatigue:    fleet.Fatigue,
				Morale:     fleet.Morale,
				InPort:     fleet.InPort,
			}
			if pos := getPositionComponent(w.ecs, id); pos != nil {
				state.X = pos.X
				state.Y = pos.Y
				state.Heading = pos.Heading
				state.ZoneID = pos.ZoneID
			}
			if cargo := getCargoComponent(w.ecs, id); cargo != nil {
				state.Manifest = copyFloatMap(cargo.Manifest)
				state.Gold = cargo.Gold
			}
			if ai := getAIComponent(w.ecs, id); ai != nil {
				state.AI = &save.AIState{
					ProfileID:       ai.ProfileID,
					TargetPortID:    ai.TargetPortID,
					TargetCommodity: ai.TargetCommodity,
					State:           ai.State,
					RiskAversion:    ai.RiskAversion,
					VoyageDays:      ai.VoyageDays,
					LastDecision:    ai.LastDecision,
				}
			}
			if len(fleet.Discovered) > 0 {
				state.Discovered = make(map[string]bool, len(fleet.Discovered))
				for k, v := range fleet.Discovered {
					state.Discovered[k] = v
				}
			}
			f.Fleets = append(f.Fleets, state)
		})
	}
	_ = posStore
	_ = cargoStore
	_ = aiStore

	// --- the reputations ---
	for factionID, entID := range w.factionEntities {
		rep := getReputationComponent(w.ecs, entID)
		if rep == nil {
			continue
		}
		state := save.ReputationState{
			FactionID: factionID,
			Adventure: rep.Adventure,
			Trade:     rep.Trade,
			Naval:     rep.Naval,
			AllyFame:  copyFloatMap(rep.AllyFame),
		}
		for title := range rep.Titles {
			state.Titles = append(state.Titles, title)
		}
		f.Reputations = append(f.Reputations, state)
	}

	// --- the discoveries and the log ---
	for k, v := range w.discovered {
		f.Discovered[k] = v
	}
	f.Log = w.LogEntries()

	return f
}

// Restore loads a snapshot into the world.
//
// The static configuration must already be loaded: the restore only
// replaces the dynamic state.
func (w *World) Restore(f *save.File) error {
	if f == nil {
		return fmt.Errorf("no save file")
	}
	w.day = f.Day
	w.year = f.Year
	w.dayFraction = f.DayFraction
	w.treasury = f.Treasury
	w.discoveredCount = f.DiscoveredCount
	w.speed = f.Speed
	w.playerEntity = ecs.ID(f.PlayerEntity)
	w.weatherEntity = ecs.ID(f.WeatherEntity)

	// --- the ports ---
	for _, ps := range f.Ports {
		pd := w.ports[ps.PortID]
		if pd == nil {
			// The port does not exist in the current data set; the state
			// is skipped instead of aborting the whole restore.
			w.Log("save file references the unknown port %s", ps.PortID)
			continue
		}
		pd.mu.Lock()
		for cid, v := range ps.Prices {
			pd.prices[cid] = v
		}
		for cid, v := range ps.Stock {
			pd.stock[cid] = v
		}
		for cid, v := range ps.Scarcity {
			pd.scarcity[cid] = v
		}
		for cid, v := range ps.Noise {
			pd.noise[cid] = v
		}
		pd.cyclePhase = ps.CyclePhase
		pd.reputation = ps.Reputation
		pd.mu.Unlock()
	}

	// --- the fleets ---
	for _, fs := range f.Fleets {
		id := ecs.ID(fs.Entity)
		fleet := getFleetComponent(w.ecs, id)
		if fleet == nil {
			w.Log("save file references the unknown fleet %d", fs.Entity)
			continue
		}
		fleet.ShipID = fs.ShipID
		fleet.Name = fs.Name
		fleet.Owner = fs.Owner
		fleet.IsPlayer = fs.IsPlayer
		fleet.HullPoints = fs.HullPoints
		fleet.Food = fs.Food
		fleet.FreshWater = fs.FreshWater
		fleet.Sailors = fs.Sailors
		fleet.Loyalty = fs.Loyalty
		fleet.Fatigue = fs.Fatigue
		fleet.Morale = fs.Morale
		fleet.InPort = fs.InPort
		if len(fs.Discovered) > 0 {
			fleet.Discovered = make(map[string]bool, len(fs.Discovered))
			for k, v := range fs.Discovered {
				fleet.Discovered[k] = v
			}
		}
		if pos := getPositionComponent(w.ecs, id); pos != nil {
			pos.X = fs.X
			pos.Y = fs.Y
			pos.Heading = fs.Heading
			pos.ZoneID = fs.ZoneID
		}
		if cargo := getCargoComponent(w.ecs, id); cargo != nil {
			cargo.Manifest = copyFloatMap(fs.Manifest)
			cargo.Gold = fs.Gold
			w.updateCargoUsage(id)
		}
		if fs.AI != nil {
			if ai := getAIComponent(w.ecs, id); ai != nil {
				ai.ProfileID = fs.AI.ProfileID
				ai.TargetPortID = fs.AI.TargetPortID
				ai.TargetCommodity = fs.AI.TargetCommodity
				ai.State = fs.AI.State
				ai.RiskAversion = fs.AI.RiskAversion
				ai.VoyageDays = fs.AI.VoyageDays
				ai.LastDecision = fs.AI.LastDecision
			}
		}
	}

	// --- the reputations ---
	for _, rs := range f.Reputations {
		entID, ok := w.factionEntities[rs.FactionID]
		if !ok {
			continue
		}
		rep := getReputationComponent(w.ecs, entID)
		if rep == nil {
			continue
		}
		rep.Adventure = rs.Adventure
		rep.Trade = rs.Trade
		rep.Naval = rs.Naval
		for _, title := range rs.Titles {
			rep.Titles[title] = true
		}
		for k, v := range rs.AllyFame {
			rep.AllyFame[k] = v
		}
	}

	// --- the discoveries ---
	for k, v := range f.Discovered {
		w.discovered[k] = v
	}

	w.Log("session restored (day %d, year %d)", w.day, w.year)
	return nil
}

// copyFloatMap copies a float map.
//
// The copy is needed because the save file must not share its maps with
// the live state.
func copyFloatMap(src map[string]float64) map[string]float64 {
	if src == nil {
		return nil
	}
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
