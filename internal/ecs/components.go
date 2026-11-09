package ecs

// This file declares every component type of the game.
//
// Components are deliberately plain structs: they hold *dynamic* state only.
// Static data (ship stats, port facilities, commodity definitions) is never
// duplicated here; components reference static data by ID and the runtime
// resolves it through the immutable configuration registry.

// PositionComponent places an entity on the world map.
//
// X / Y are pixel coordinates in the world map space. Heading is the compass
// bearing in degrees (0 = north, clockwise). Speed is the current speed over
// ground in knots.
type PositionComponent struct {
	// X is the horizontal map coordinate in pixels.
	X float64
	// Y is the vertical map coordinate in pixels.
	Y float64
	// Heading is the compass bearing in degrees (0..360).
	Heading float64
	// Speed is the speed over ground in knots.
	Speed float64
	// ZoneID is the sea zone the entity currently occupies.
	ZoneID string
}

// FleetComponent holds the identity and condition of a fleet.
//
// A "fleet" is a group of ships sailing together. The player owns exactly
// one fleet; AI factions own several.
type FleetComponent struct {
	// ShipID references config.Ship.ID of the hull class.
	ShipID string
	// Name is the display name of the fleet.
	Name string
	// Owner is the faction ID that controls the fleet ("player" or an AI
	// profile ID).
	Owner string
	// IsPlayer marks the player's fleet.
	IsPlayer bool
	// HullPoints is the current durability; it never exceeds the hull
	// maximum of the referenced ship class.
	HullPoints float64
	// Food is the food stock in tons.
	Food float64
	// FreshWater is the fresh water stock in tons.
	FreshWater float64
	// Sailors is the current crew size.
	Sailors int
	// Loyalty is the average crew loyalty (0..100).
	Loyalty float64
	// Fatigue is the average crew fatigue (0..100).
	Fatigue float64
	// Morale is the average crew morale (0..100).
	Morale float64
	// InPort is the ID of the port the fleet is docked at ("" = at sea).
	InPort string
	// Discovered lists the IDs of discoveries already made by this fleet.
	Discovered map[string]bool
}

// CargoComponent holds the cargo manifest of a fleet.
//
// Manifest maps a commodity ID to the carried amount (in units). The map is
// allocated once and reused to avoid per-tick allocations in hot loops.
type CargoComponent struct {
	// Manifest maps commodity ID -> carried units.
	Manifest map[string]float64
	// Gold is the money carried aboard (usually 0; the treasury is
	// tracked per faction).
	Gold float64
	// UsedWeight is the cached total weight of the manifest.
	UsedWeight float64
	// UsedVolume is the cached total volume of the manifest.
	UsedVolume float64
}

// PortDynamicComponent is the mutable state of a port.
//
// It is created once per port when a game session starts and is then
// updated by the economy simulation.
type PortDynamicComponent struct {
	// PortID references config.Port.ID.
	PortID string
	// Prices holds the current local price of every commodity.
	Prices map[string]float64
	// Stock holds the current inventory of every commodity.
	Stock map[string]float64
	// CyclePhase is the current position in the business cycle, in
	// [0,1) of the cycle period.
	CyclePhase float64
	// CycleTrend is the derivative of the cycle, used to project prices.
	CycleTrend float64
	// Noise is the current stochastic price perturbation (multiplicative).
	Noise map[string]float64
	// Reputation holds the player faction's reputation at this port.
	Reputation float64
	// BusinessLevel describes the local business climate ("boom",
	// "normal", "slump").
	BusinessLevel string
}

// ReputationComponent holds the three-axis reputation of a faction.
//
// Adventure / Trade / Naval are the axes of the original game; the values
// are "fame" points that unlock titles and story events.
type ReputationComponent struct {
	// FactionID identifies the owning faction.
	FactionID string
	// Adventure is the exploration/combat fame.
	Adventure float64
	// Trade is the merchant fame.
	Trade float64
	// Naval is the fleet-command fame.
	Naval float64
	// Titles lists the unlocked title IDs.
	Titles map[string]bool
	// AllyFame records the fame of allies, used for fame propagation.
	AllyFame map[string]float64
}

// AIComponent parameterises a fleet or faction under AI control.
type AIComponent struct {
	// ProfileID references config.AIProfile.ID.
	ProfileID string
	// TargetPortID is the port the AI is currently heading to.
	TargetPortID string
	// TargetCommodity is the commodity the AI currently wants to buy.
	TargetCommodity string
	// SellCommodity is the commodity the AI currently wants to sell.
	SellCommodity string
	// State is the high level AI state ("idle", "trade", "return", "flee").
	State string
	// RiskAversion mirrors the profile but may change at runtime.
	RiskAversion float64
	// VoyageDays counts the days spent on the current voyage.
	VoyageDays int
	// Cooldown prevents the AI from re-deciding every tick.
	Cooldown int
	// LastDecision is a human readable trace of the last decision.
	LastDecision string
}

// CharacterComponent describes a recruited crew member.
type CharacterComponent struct {
	// TemplateID references config.CharacterTemplate.ID.
	TemplateID string
	// Name is the display name.
	Name string
	// Class is the character class.
	Class string
	// Level is the current level.
	Level int
	// Adventure / Trade / Naval are the personal reputation axes.
	Adventure float64
	Trade     float64
	Naval     float64
	// Loyalty is the personal loyalty (0..100).
	Loyalty float64
	// SalaryPerDay is the individual wage.
	SalaryPerDay float64
	// XP is the accumulated experience points.
	XP float64
}

// WeatherComponent holds the active weather state of a world session.
//
// Exactly one weather state exists per session; it is stored as a component
// on a dedicated entity so that the render layer can read it uniformly.
type WeatherComponent struct {
	// TemplateID references config.WeatherTemplate.ID.
	TemplateID string
	// Kind is the weather classification.
	Kind string
	// WindSpeed is the current wind speed in knots.
	WindSpeed float64
	// WindDirDeg is the current wind direction in degrees.
	WindDirDeg float64
	// CurrentSpeed is the current speed in knots.
	CurrentSpeed float64
	// CurrentDirDeg is the current direction in degrees.
	CurrentDirDeg float64
	// RemainingDays is how long the weather lasts.
	RemainingDays int
	// VisibilityFactor scales the map visibility radius.
	VisibilityFactor float64
}

// DiscoveryComponent marks a discovery that the player has made.
type DiscoveryComponent struct {
	// DiscoveryID is the unique discovery identifier.
	DiscoveryID string
	// Day is the in-game day on which it was found.
	Day int
	// Name is the display name.
	Name string
	// Reward is the money granted for the discovery.
	Reward float64
}

// SessionComponent carries the global clock of a game session.
//
// It lives on a single "world" entity.
type SessionComponent struct {
	// Day is the current in-game day (1-based).
	Day int
	// Year is the current in-game year.
	Year int
	// DayFraction is the fraction of the current day that has elapsed.
	DayFraction float64
	// WeatherEntity references the weather entity.
	WeatherEntity ID
	// PlayerEntity references the player's fleet entity.
	PlayerEntity ID
	// Treasury is the player's money.
	Treasury float64
	// DiscoveredCount counts the discoveries made so far.
	DiscoveredCount int
}
