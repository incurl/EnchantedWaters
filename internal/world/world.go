package world

import (
	"fmt"
	"math/rand/v2"
	"sync"

	"github.com/peter/enchantedwaters/internal/ai"
	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
	"github.com/peter/enchantedwaters/internal/event"
)

// Local aliases keep the simulation code readable while making it explicit
// that these types are the immutable static configuration layer.
type (
	// configCommodity is a static commodity definition.
	configCommodity = config.Commodity
	// configPort is a static port definition.
	configPort = config.Port
	// configShip is a static ship class definition.
	configShip = config.Ship
	// configSeaZone is a static sea zone definition.
	configSeaZone = config.SeaZone
)

// portDynamic is the dynamic state of a single port.
//
// Each port owns its own mutex. This is the "shard / partial lock"
// strategy required by the project: the economy update touches every port
// every day, and a single global lock would serialise the whole update.
//
// The maps inside are pre-allocated when the port is created, so the daily
// update performs no allocation at all.
type portDynamic struct {
	// mu guards the mutable fields of this port.
	mu sync.Mutex
	// portID references config.Port.ID.
	portID string
	// prices holds the current local price of every commodity.
	prices map[string]float64
	// stock holds the current inventory of every commodity.
	stock map[string]float64
	// scarcity holds the smoothed scarcity factor of every commodity.
	scarcity map[string]float64
	// noise holds the stochastic price perturbation of every commodity.
	noise map[string]float64
	// cyclePhase is the position in the business cycle, in [0,1).
	cyclePhase float64
	// cycleTrend is the daily change of the cycle phase.
	cycleTrend float64
	// reputation is the player faction's reputation at this port.
	reputation float64
	// businessLevel describes the local business climate.
	businessLevel string
}

// World is the dynamic state layer of a game session.
//
// A World is created per game session and owns every mutable value of the
// simulation. It never mutates the static configuration it was given.
type World struct {
	// cfg is the immutable static configuration registry.
	cfg *config.Registry
	// ecs is the entity world holding fleets, ports and characters.
	ecs *ecs.World
	// rng is the simulation random source. It is used only from the
	// simulation goroutine; parallel workers receive their own stream.
	rng *rand.Rand

	// ports maps a port ID to its dynamic state.
	ports map[string]*portDynamic
	// portIDs is the deterministic list of port IDs.
	portIDs []string
	// portShards groups the port locks; ports are distributed over the
	// shards by hash so that unrelated ports rarely contend.
	portShards []*sync.Mutex

	// day is the current in-game day (1-based).
	day int
	// year is the current in-game year.
	year int
	// dayFraction is the elapsed fraction of the current day.
	dayFraction float64
	// weatherEntity is the entity holding the active weather state.
	weatherEntity ecs.ID
	// playerEntity is the entity of the player's fleet.
	playerEntity ecs.ID
	// treasury is the player's money.
	treasury float64
	// discovered maps discovery IDs to the day they were found.
	discovered map[string]int
	// discoveredCount counts the discoveries made so far.
	discoveredCount int

	// log collects human readable simulation messages.
	log []string
	// logMu guards the log slice: AI workers may append to it.
	logMu sync.Mutex

	// workerCount is the number of parallel workers used for the
	// per-tick batch updates.
	workerCount int
	// speed is the simulation speed multiplier set by the player.
	speed int

	// fleetIDs is the reusable scratch slice of fleet entity IDs.
	fleetIDs []ecs.ID
	// headings caches the desired heading of each fleet for the current
	// tick. It is reused between ticks to avoid per-tick allocations.
	headings map[ecs.ID]float64
	// weatherIDs caches the deterministic weather template ordering.
	weatherIDs []string
	// zoneOrder caches the deterministic sea zone ordering.
	zoneOrder []string
	// factionEntities maps a faction ID to the entity holding its
	// reputation component.
	factionEntities map[string]ecs.ID
	// ai is the AI controller that evaluates the NPC fleets.
	ai *ai.Controller
	// aiRequests is the reusable scratch slice of AI update requests.
	aiRequests []ai.UpdateRequest
	// events selects the random events from the event table.
	events *event.Selector
	// resolver applies the effects of the events.
	resolver *event.Resolver
	// scripting runs the Lua scripts that are attached to the events.
	scripting *event.Scripting

	// eventBus receives simulation events (see internal/event).
	eventBus EventSink
}

// EventSink receives simulation events.
//
// It is implemented by the event package; the world only knows this small
// interface so that the layers stay decoupled.
type EventSink interface {
	// Emit publishes a simulation event with the given kind and payload.
	Emit(kind string, payload map[string]any)
}

// NewWorld creates a dynamic world from an immutable configuration.
//
// The ports are instantiated eagerly: every port gets its dynamic state
// (prices, stock, noise) so that the very first tick is already consistent.
func NewWorld(cfg *config.Registry, seed uint64) *World {
	w := &World{
		cfg:             cfg,
		ecs:             ecs.NewWorld(),
		rng:             rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15)),
		ports:           make(map[string]*portDynamic, len(cfg.Ports)),
		discovered:      make(map[string]int, 64),
		factionEntities: make(map[string]ecs.ID, 8),
		workerCount:     defaultWorkerCount(),
	}

	// Allocate the port locks. Ports are distributed round-robin over the
	// shards, which keeps the per-shard contention low even though the
	// current update pattern touches all ports once per day.
	shardCount := defaultWorkerCount() * 2
	w.portShards = make([]*sync.Mutex, shardCount)
	for i := range w.portShards {
		w.portShards[i] = &sync.Mutex{}
	}

	// Instantiate the port states in deterministic order.
	for _, id := range cfg.PortOrder {
		pd := &portDynamic{
			portID:        id,
			prices:        make(map[string]float64, len(cfg.CommodityOrder)),
			stock:         make(map[string]float64, len(cfg.CommodityOrder)),
			scarcity:      make(map[string]float64, len(cfg.CommodityOrder)),
			noise:         make(map[string]float64, len(cfg.CommodityOrder)),
			cyclePhase:    0.5,
			businessLevel: "normal",
		}
		for _, cid := range cfg.CommodityOrder {
			pd.prices[cid] = cfg.Commodities[cid].BasePrice
			pd.noise[cid] = 0
			pd.scarcity[cid] = 1
			// Seed the stock with the equilibrium amount so that the first
			// day does not produce an artificial shock.
			supply := cfg.PortSupply(id, cid)
			demand := cfg.PortDemand(id, cid)
			if supply+demand > 0 {
				pd.stock[cid] = demand / (supply + demand) * 10
			} else {
				pd.stock[cid] = 0
			}
		}
		w.ports[id] = pd
		w.portIDs = append(w.portIDs, id)
	}
	return w
}

// defaultWorkerCount returns the number of parallel workers to use.
//
// It is derived from GOMAXPROCS but capped, because the batches are coarse
// and an excessive number of goroutines would only add scheduling noise.
func defaultWorkerCount() int {
	n := 8
	return n
}

// Config returns the immutable configuration registry of this world.
func (w *World) Config() *config.Registry { return w.cfg }

// ECS returns the entity world.
func (w *World) ECS() *ecs.World { return w.ecs }

// Day returns the current in-game day.
func (w *World) Day() int { return w.day }

// Year returns the current in-game year.
func (w *World) Year() int { return w.year }

// DayFraction returns the elapsed fraction of the current day.
func (w *World) DayFraction() float64 { return w.dayFraction }

// Treasury returns the player's money.
func (w *World) Treasury() float64 { return w.treasury }

// PlayerEntity returns the entity ID of the player's fleet.
func (w *World) PlayerEntity() ecs.ID { return w.playerEntity }

// WeatherEntity returns the entity ID holding the active weather state.
func (w *World) WeatherEntity() ecs.ID { return w.weatherEntity }

// DiscoveredCount returns the number of discoveries made so far.
func (w *World) DiscoveredCount() int { return w.discoveredCount }

// portState returns the dynamic state of a port, or nil.
func (w *World) portState(portID string) *portDynamic {
	return w.ports[portID]
}

// Price returns the current price of a commodity at a port.
//
// This is the public read API used by the UI and by the AI.
func (w *World) Price(commodityID, portID string) float64 {
	pd := w.ports[portID]
	if pd == nil {
		return 0
	}
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if p, ok := pd.prices[commodityID]; ok {
		return p
	}
	return w.cfg.Commodities[commodityID].BasePrice
}

// Stock returns the current stock of a commodity at a port.
func (w *World) Stock(commodityID, portID string) float64 {
	pd := w.ports[portID]
	if pd == nil {
		return 0
	}
	pd.mu.Lock()
	defer pd.mu.Unlock()
	return pd.stock[commodityID]
}

// SetTreasury sets the player's money.
func (w *World) SetTreasury(v float64) { w.treasury = v }

// AddTreasury adds (or subtracts) money from the player's treasury.
func (w *World) AddTreasury(v float64) { w.treasury += v }

// Log appends a human readable message to the simulation log.
//
// The log is protected by its own mutex because parallel AI workers may
// append concurrently.
func (w *World) Log(format string, args ...any) {
	w.logMu.Lock()
	defer w.logMu.Unlock()
	w.log = append(w.log, fmt.Sprintf(format, args...))
	// Keep the log bounded: the oldest entries are dropped first.
	if maxLog := 512; len(w.log) > maxLog {
		w.log = w.log[len(w.log)-maxLog:]
	}
}

// LogEntries returns a copy of the simulation log.
func (w *World) LogEntries() []string {
	w.logMu.Lock()
	defer w.logMu.Unlock()
	out := make([]string, len(w.log))
	copy(out, w.log)
	return out
}

// SetScripting installs the Lua scripting engine.
//
// The engine is optional: when it is not installed, the events are
// resolved from their table driven effects only.
func (w *World) SetScripting(s *event.Scripting) { w.scripting = s }

// SetEventSink installs the event sink. It must be called before the first
// tick so that no event is lost.
func (w *World) SetEventSink(sink EventSink) { w.eventBus = sink }

// emit publishes an event, if an event sink is installed.
func (w *World) emit(kind string, payload map[string]any) {
	if w.eventBus == nil {
		return
	}
	w.eventBus.Emit(kind, payload)
}

// seedOf derives a deterministic seed from a string.
//
// It uses the FNV-1a hash so that the AI controller always starts from
// the same state for the same starting port: a game session is therefore
// reproducible.
func seedOf(s string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}
