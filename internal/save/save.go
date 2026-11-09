// Package save implements the persistence of a game session.
//
// # What is saved
//
// Only the *dynamic state* of a session is written to disk. The static
// configuration tables are never part of a save file: they are the
// canonical data of the game and are shared by every session.
//
// The save format is JSON, which keeps the files readable and diffable,
// and which makes it easy to write a migration when the data model
// changes.
package save

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Version is the current version of the save format.
//
// It is stored in every save file so that an old file can be migrated.
const Version = 1

// File is the complete state of a game session.
type File struct {
	// Version is the version of the save format.
	Version int `json:"version"`
	// SavedAt is the wall clock time of the save.
	SavedAt time.Time `json:"saved_at"`
	// Seed is the random seed of the session.
	Seed uint64 `json:"seed"`
	// Day is the current in-game day.
	Day int `json:"day"`
	// Year is the current in-game year.
	Year int `json:"year"`
	// DayFraction is the elapsed fraction of the current day.
	DayFraction float64 `json:"day_fraction"`
	// Treasury is the player's money.
	Treasury float64 `json:"treasury"`
	// DiscoveredCount is the number of discoveries made so far.
	DiscoveredCount int `json:"discovered_count"`
	// PlayerEntity is the entity ID of the player's fleet.
	PlayerEntity uint64 `json:"player_entity"`
	// WeatherEntity is the entity ID of the weather state.
	WeatherEntity uint64 `json:"weather_entity"`
	// Speed is the simulation speed.
	Speed int `json:"speed"`
	// Ports holds the dynamic state of every port.
	Ports []PortState `json:"ports"`
	// Fleets holds the dynamic state of every fleet.
	Fleets []FleetState `json:"fleets"`
	// Reputations holds the reputation of every faction.
	Reputations []ReputationState `json:"reputations"`
	// Discovered maps a discovery key to the day it was found.
	Discovered map[string]int `json:"discovered"`
	// Log holds the tail of the simulation log.
	Log []string `json:"log"`
}

// PortState is the dynamic state of a port.
type PortState struct {
	// PortID references config.Port.ID.
	PortID string `json:"port_id"`
	// Prices holds the current local price of every commodity.
	Prices map[string]float64 `json:"prices"`
	// Stock holds the current inventory of every commodity.
	Stock map[string]float64 `json:"stock"`
	// Scarcity holds the smoothed scarcity factor of every commodity.
	Scarcity map[string]float64 `json:"scarcity"`
	// Noise holds the stochastic price perturbation of every commodity.
	Noise map[string]float64 `json:"noise"`
	// CyclePhase is the position in the business cycle.
	CyclePhase float64 `json:"cycle_phase"`
	// Reputation is the player faction's reputation at this port.
	Reputation float64 `json:"reputation"`
}

// FleetState is the dynamic state of a fleet.
type FleetState struct {
	// Entity is the entity ID of the fleet.
	Entity uint64 `json:"entity"`
	// ShipID references config.Ship.ID.
	ShipID string `json:"ship_id"`
	// Name is the display name.
	Name string `json:"name"`
	// Owner is the faction that controls the fleet.
	Owner string `json:"owner"`
	// IsPlayer marks the player's fleet.
	IsPlayer bool `json:"is_player"`
	// HullPoints is the current durability.
	HullPoints float64 `json:"hull_points"`
	// Food is the food stock.
	Food float64 `json:"food"`
	// FreshWater is the fresh water stock.
	FreshWater float64 `json:"fresh_water"`
	// Sailors is the crew size.
	Sailors int `json:"sailors"`
	// Loyalty is the average crew loyalty.
	Loyalty float64 `json:"loyalty"`
	// Fatigue is the average crew fatigue.
	Fatigue float64 `json:"fatigue"`
	// Morale is the average crew morale.
	Morale float64 `json:"morale"`
	// InPort is the port the fleet is docked at.
	InPort string `json:"in_port"`
	// X / Y is the map position.
	X float64 `json:"x"`
	// Y is the map position.
	Y float64 `json:"y"`
	// Heading is the compass bearing.
	Heading float64 `json:"heading"`
	// ZoneID is the sea zone of the fleet.
	ZoneID string `json:"zone_id"`
	// Manifest is the cargo manifest.
	Manifest map[string]float64 `json:"manifest"`
	// Gold is the money carried aboard.
	Gold float64 `json:"gold"`
	// AI holds the AI state of the fleet.
	AI *AIState `json:"ai,omitempty"`
	// Discovered maps a discovery key to a flag.
	Discovered map[string]bool `json:"discovered,omitempty"`
}

// AIState is the AI state of a fleet.
type AIState struct {
	// ProfileID references config.AIProfile.ID.
	ProfileID string `json:"profile_id"`
	// TargetPortID is the port the fleet is heading to.
	TargetPortID string `json:"target_port_id"`
	// TargetCommodity is the commodity the fleet wants to buy.
	TargetCommodity string `json:"target_commodity"`
	// State is the high level AI state.
	State string `json:"state"`
	// RiskAversion is the risk parameter of the fleet.
	RiskAversion float64 `json:"risk_aversion"`
	// VoyageDays counts the days spent on the current voyage.
	VoyageDays int `json:"voyage_days"`
	// LastDecision is the trace of the last decision.
	LastDecision string `json:"last_decision"`
}

// ReputationState is the reputation of a faction.
type ReputationState struct {
	// FactionID identifies the faction.
	FactionID string `json:"faction_id"`
	// Adventure is the exploration and combat fame.
	Adventure float64 `json:"adventure"`
	// Trade is the commerce fame.
	Trade float64 `json:"trade"`
	// Naval is the fleet command fame.
	Naval float64 `json:"naval"`
	// Titles lists the unlocked title IDs.
	Titles []string `json:"titles"`
	// AllyFame records the fame of the allies.
	AllyFame map[string]float64 `json:"ally_fame"`
}

// Write saves a session into a file.
//
// The file is written atomically: it is first written into a temporary
// file in the same directory and then renamed, so that an interrupted
// save never corrupts an existing one.
func Write(path string, f *File) error {
	f.Version = Version
	f.SavedAt = time.Now()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create save directory: %w", err)
	}

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode save file: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".save-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary save file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write save file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename save file: %w", err)
	}
	return nil
}

// Read loads a session from a file.
func Read(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read save file: %w", err)
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("decode save file: %w", err)
	}
	if f.Version > Version {
		return nil, fmt.Errorf("the save file has the version %d, but this build only supports up to %d",
			f.Version, Version)
	}
	return &f, nil
}

// List returns the save files of a directory, sorted by their name.
func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 5 || name[len(name)-5:] != ".json" {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	return out, nil
}
