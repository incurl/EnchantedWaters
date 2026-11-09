package world

import (
	"github.com/peter/enchantedwaters/internal/ai"
	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
	"github.com/peter/enchantedwaters/internal/event"
)

// This file builds the initial entities of a game session.
//
// The session setup is the bridge between the static configuration layer and
// the dynamic ECS state: every static table entry that needs a runtime
// counterpart is instantiated here, exactly once.

// StartSession initialises the world with the player's fleet, the weather
// state, the session clock and the faction reputations.
//
// It returns an error when the configuration is inconsistent (for example
// when the starting port does not exist).
func (w *World) StartSession(startPortID string) error {
	if w.cfg.Port(startPortID) == nil {
		return &SessionError{Message: "unknown starting port " + startPortID}
	}

	// --- register the component storages ---
	fleetStore := ecs.RegisterComponentArray[ecs.FleetComponent](w.ecs, "FleetComponent")
	posStore := ecs.RegisterComponentArray[ecs.PositionComponent](w.ecs, "PositionComponent")
	cargoStore := ecs.RegisterComponentArray[ecs.CargoComponent](w.ecs, "CargoComponent")
	aiStore := ecs.RegisterComponentArray[ecs.AIComponent](w.ecs, "AIComponent")
	weatherStore := ecs.RegisterComponentArray[ecs.WeatherComponent](w.ecs, "WeatherComponent")
	repStore := ecs.RegisterComponentArray[ecs.ReputationComponent](w.ecs, "ReputationComponent")
	sessStore := ecs.RegisterComponentArray[ecs.SessionComponent](w.ecs, "SessionComponent")
	charStore := ecs.RegisterComponentArray[ecs.CharacterComponent](w.ecs, "CharacterComponent")
	discStore := ecs.RegisterComponentArray[ecs.DiscoveryComponent](w.ecs, "DiscoveryComponent")
	_ = fleetStore
	_ = posStore
	_ = cargoStore
	_ = weatherStore
	_ = sessStore
	_ = aiStore
	_ = repStore
	_ = charStore
	_ = discStore

	// --- session clock ---
	sessionID := w.ecs.CreateEntity()
	sessStore.Set(w.ecs, sessionID, ecs.SessionComponent{
		Day:          1,
		Year:         1,
		Treasury:     w.cfg.Params.StartMoney,
		PlayerEntity: 0,
	})
	w.day = 1
	w.year = 1
	w.treasury = w.cfg.Params.StartMoney
	// The simulation starts at the normal speed; the player can pause it
	// with the speed 0.
	w.speed = 1

	// --- weather ---
	weatherID := w.ecs.CreateEntity()
	weatherStore.Set(w.ecs, weatherID, ecs.WeatherComponent{
		TemplateID:       "weather.clear",
		Kind:             string(config.WeatherClear),
		WindSpeed:        5,
		WindDirDeg:       90,
		VisibilityFactor: 1,
	})
	w.weatherEntity = weatherID

	// --- the player's fleet ---
	shipID := w.firstShipID()
	if shipID == "" {
		return &SessionError{Message: "no ship class available"}
	}
	ship := w.cfg.Ship(shipID)
	start := w.cfg.Port(startPortID)

	playerID := w.ecs.CreateEntity()
	fleetStore.Set(w.ecs, playerID, ecs.FleetComponent{
		ShipID:     shipID,
		Name:       "Player's " + ship.Name,
		Owner:      FactionPlayer,
		IsPlayer:   true,
		HullPoints: ship.HullPoints,
		Food:       w.cfg.Params.StartFood,
		FreshWater: w.cfg.Params.StartWater,
		Sailors:    ship.MaxSailors,
		Loyalty:    70,
		Fatigue:    0,
		Morale:     70,
		InPort:     startPortID,
		Discovered: make(map[string]bool, 16),
	})
	posStore.Set(w.ecs, playerID, ecs.PositionComponent{
		X:       start.X,
		Y:       start.Y,
		Heading: 90,
		Speed:   0,
		ZoneID:  w.zoneAt(start.X, start.Y),
	})
	cargoStore.Set(w.ecs, playerID, ecs.CargoComponent{
		Manifest: make(map[string]float64, 16),
		Gold:     w.cfg.Params.StartMoney,
	})
	w.playerEntity = playerID

	// --- the player's faction reputation ---
	w.RegisterFaction(FactionPlayer)

	// --- event system ---
	// The selector draws the events, the resolver applies their effects
	// and the scripting engine runs the Lua scripts.
	if w.events == nil {
		w.events = event.NewSelector(w.cfg, seedOf(startPortID)^0xA5A5A5A5)
	}
	if w.resolver == nil {
		w.resolver = event.NewResolver(seedOf(startPortID))
	}

	// --- AI controller ---
	// The controller is created here so that the AI is always active; a
	// caller may replace it with a custom one via SetAIController.
	if w.ai == nil {
		w.ai = ai.NewController(w.cfg, seedOf(startPortID))
	}

	// --- AI factions ---
	for _, profileID := range w.cfg.AIProfileOrder {
		profile := w.cfg.AIProfiles[profileID]
		if profile == nil {
			continue
		}
		w.RegisterFaction(profileID)
		for i := 0; i < profile.FleetCount; i++ {
			w.spawnAIFleet(profile, i, startPortID)
		}
	}

	w.Log("session started at %s on day %d", start.Name, w.day)
	return nil
}

// FactionPlayer is the reserved faction ID of the player.
const FactionPlayer = "player"

// RegisterFaction creates the reputation entity of a faction.
func (w *World) RegisterFaction(factionID string) ecs.ID {
	if id, ok := w.factionEntities[factionID]; ok {
		return id
	}
	repStore := ecs.ComponentArrayOf[ecs.ReputationComponent](w.ecs)
	if repStore == nil {
		return 0
	}
	entID := w.ecs.CreateEntity()
	repStore.Set(w.ecs, entID, ecs.ReputationComponent{
		FactionID: factionID,
		Titles:    make(map[string]bool, 8),
		AllyFame:  make(map[string]float64, 8),
	})
	w.factionEntities[factionID] = entID
	return entID
}

// spawnAIFleet creates one AI fleet of a faction.
func (w *World) spawnAIFleet(profile *config.AIProfile, index int, homePortID string) ecs.ID {
	fleetStore := ecs.ComponentArrayOf[ecs.FleetComponent](w.ecs)
	posStore := ecs.ComponentArrayOf[ecs.PositionComponent](w.ecs)
	cargoStore := ecs.ComponentArrayOf[ecs.CargoComponent](w.ecs)
	aiStore := ecs.ComponentArrayOf[ecs.AIComponent](w.ecs)
	if fleetStore == nil || posStore == nil || cargoStore == nil || aiStore == nil {
		return 0
	}

	// The AI starts at its home port when it has one, otherwise at the
	// first port of the deterministic ordering.
	home := homePortID
	if profile.HomePort != "" && w.cfg.Port(profile.HomePort) != nil {
		home = profile.HomePort
	}
	port := w.cfg.Port(home)
	if port == nil {
		return 0
	}

	shipID := w.aiShipID(profile)
	ship := w.cfg.Ship(shipID)
	if ship == nil {
		return 0
	}

	fleetID := w.ecs.CreateEntity()
	fleetStore.Set(w.ecs, fleetID, ecs.FleetComponent{
		ShipID:     shipID,
		Name:       profile.Name + " fleet " + itoa(index+1),
		Owner:      profile.ID,
		IsPlayer:   false,
		HullPoints: ship.HullPoints,
		Food:       float64(ship.MaxSailors) * ship.FoodPerSailorPerDay * 10,
		FreshWater: float64(ship.MaxSailors) * ship.WaterPerSailorPerDay * 10,
		Sailors:    ship.MaxSailors,
		Loyalty:    70,
		Morale:     70,
		InPort:     home,
		Discovered: make(map[string]bool, 8),
	})
	posStore.Set(w.ecs, fleetID, ecs.PositionComponent{
		X:      port.X,
		Y:      port.Y,
		ZoneID: w.zoneAt(port.X, port.Y),
	})
	// The AI fleet needs working capital to buy its first cargo. The
	// capital scales with the profile's fleet count, so that a faction
	// with many fleets is not richer in total than one with few.
	capital := w.cfg.Params.StartMoney * float64(profile.FleetCount) / 4
	cargoStore.Set(w.ecs, fleetID, ecs.CargoComponent{
		Manifest: make(map[string]float64, 16),
		Gold:     capital,
	})
	aiStore.Set(w.ecs, fleetID, ecs.AIComponent{
		ProfileID:    profile.ID,
		State:        "idle",
		RiskAversion: profile.RiskAversion,
	})
	return fleetID
}

// largeCargoAggressiveness is the cargo aggressiveness above which an AI
// fleet is given a larger hull.
const largeCargoAggressiveness = 0.8

// firstShipID returns the ID of the smallest ship class, which the player
// starts with.
func (w *World) firstShipID() string {
	best := ""
	bestSize := 0.0
	for _, id := range w.cfg.ShipOrder {
		s := w.cfg.Ships[id]
		if s == nil {
			continue
		}
		if best == "" || s.CargoCapacity < bestSize {
			best = id
			bestSize = s.CargoCapacity
		}
	}
	return best
}

// aiShipID picks a ship class for an AI faction.
//
// The choice is deterministic: it depends on the fleet index and the
// profile's cargo aggressiveness, so that different AI fleets use
// different hulls without any randomness.
func (w *World) aiShipID(profile *config.AIProfile) string {
	if len(w.cfg.ShipOrder) == 0 {
		return ""
	}
	idx := len(w.cfg.ShipOrder) / 2
	if profile.CargoAggressiveness > largeCargoAggressiveness && idx+1 < len(w.cfg.ShipOrder) {
		idx++
	}
	return w.cfg.ShipOrder[idx]
}

// zoneAt returns the ID of the sea zone containing a map position.
//
// The zone with the smallest centre distance wins; when no zone matches,
// the empty string is returned.
func (w *World) zoneAt(x, y float64) string {
	best := ""
	bestDist := 0.0
	for _, zid := range w.cfgZoneOrder() {
		z := w.cfg.SeaZones[zid]
		dx := x - z.CenterX
		dy := y - z.CenterY
		d := dx*dx + dy*dy
		if best == "" || d < bestDist {
			best = zid
			bestDist = d
		}
	}
	return best
}

// SessionError describes an error that prevents a session from starting.
type SessionError struct {
	// Message describes the problem.
	Message string
}

// Error implements the error interface.
func (e *SessionError) Error() string { return "session error: " + e.Message }
