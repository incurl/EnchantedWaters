package world

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"sync"

	"github.com/peter/enchantedwaters/internal/ecs"
	"github.com/peter/enchantedwaters/internal/event"
)

// Tick advances the simulation by one in-game day.
//
// The order of the phases matters and mirrors the original game's turn
// structure:
//
//  1. Weather advance   -- the new day's weather is drawn.
//  2. Fleet advance     -- every fleet sails (parallel, batched).
//  3. Crew / events     -- sea events, discoveries, mutiny checks.
//  4. Economy update    -- prices, stock and business cycle (parallel).
//  5. Calendar advance  -- day counter and season bookkeeping.
//
// Steps 2 and 4 are parallelised with a worker pool; the parallel work is
// done on disjoint data (different fleets, different ports), so no locking
// beyond the per-port shards is required.
func (w *World) Tick() {
	w.advanceWeather()
	w.updateAIFleets()
	w.tradeAIFleets()
	w.advanceFleets()
	w.checkDocking()
	w.advanceEvents()
	w.updateEconomy()
	w.decayReputation()
	w.propagateFame()
	w.advanceCalendar()
}

// advanceWeather draws the weather of the next day.
//
// The weather is selected with a weighted random draw over the weather
// templates, restricted to the templates that are valid for the current day.
// Its duration is drawn from the template's speed bounds.
func (w *World) advanceWeather() {
	wc := w.weatherComponent()
	if wc == nil {
		return
	}
	if wc.RemainingDays > 0 {
		wc.RemainingDays--
	}
	if wc.RemainingDays > 0 {
		return
	}

	// Weighted selection over the valid templates.
	total := 0.0
	for _, id := range w.weatherOrder() {
		t := w.cfg.Weather[id]
		if t == nil || t.Weight <= 0 {
			continue
		}
		if t.MinDay > 0 && w.day < t.MinDay {
			continue
		}
		if t.MaxDay > 0 && w.day > t.MaxDay {
			continue
		}
		total += t.Weight
	}
	if total <= 0 {
		return
	}
	pick := w.rng.Float64() * total
	chosen := ""
	acc := 0.0
	for _, id := range w.weatherOrder() {
		t := w.cfg.Weather[id]
		if t == nil || t.Weight <= 0 {
			continue
		}
		if t.MinDay > 0 && w.day < t.MinDay {
			continue
		}
		if t.MaxDay > 0 && w.day > t.MaxDay {
			continue
		}
		acc += t.Weight
		if pick <= acc {
			chosen = id
			break
		}
	}
	if chosen == "" {
		return
	}
	t := w.cfg.Weather[chosen]

	wc.TemplateID = chosen
	wc.Kind = string(t.Kind)
	wc.WindSpeed = t.WindSpeedMin + w.rng.Float64()*(t.WindSpeedMax-t.WindSpeedMin)
	wc.WindDirDeg = t.WindDirDegMin + w.rng.Float64()*(t.WindDirDegMax-t.WindDirDegMin)
	wc.CurrentSpeed = 0
	wc.CurrentDirDeg = 0
	wc.VisibilityFactor = t.VisibilityFactor
	// The weather lasts between 1 and 3 days; a storm naturally lasts
	// longer than a drizzle because its wind spread is larger.
	span := t.WindSpeedMax - t.WindSpeedMin
	days := 1
	if span > 0 {
		days = 1 + int(w.rng.Float64()*3)
	}
	wc.RemainingDays = days

	// A storm damages exposed hulls.
	if t.DamagePerDay > 0 {
		w.applyWeatherDamage(t.DamagePerDay)
	}
	w.Log("weather: %s (wind %.1f kn)", t.Name, wc.WindSpeed)
}

// applyWeatherDamage applies the storm damage to every fleet at sea.
func (w *World) applyWeatherDamage(damagePerDay float64) {
	fleetStorage := w.ecs.Storage("FleetComponent")
	if fleetStorage == nil {
		return
	}
	ca, ok := fleetStorage.(*ecs.ComponentArray[ecs.FleetComponent])
	if !ok {
		return
	}
	ca.Each(func(id ecs.ID, f *ecs.FleetComponent) {
		if f.InPort != "" {
			return // docked ships are protected
		}
		// The damage scales with the wind speed relative to the hull's
		// maximum: a small ship in a hurricane takes proportionally more
		// damage than a galleon.
		wind := 1.0
		if wc := w.weatherComponent(); wc != nil {
			wind = wc.WindSpeed
		}
		dmg := damagePerDay * (wind / 10.0)
		if dmg <= 0 {
			return
		}
		f.HullPoints -= dmg
		if f.HullPoints < 0 {
			f.HullPoints = 0
		}
	})
}

// advanceFleets moves every fleet of the world.
//
// The fleets are partitioned into batches that are processed by parallel
// workers. Each worker owns its fleets exclusively, so no lock is needed
// inside the movement code.
func (w *World) advanceFleets() {
	fleetStorage := w.ecs.Storage("FleetComponent")
	if fleetStorage == nil {
		return
	}
	ca, ok := fleetStorage.(*ecs.ComponentArray[ecs.FleetComponent])
	if !ok {
		return
	}

	// Collect the fleet IDs into a reusable slice.
	if w.fleetIDs == nil {
		w.fleetIDs = make([]ecs.ID, 0, 128)
	}
	w.fleetIDs = w.fleetIDs[:0]
	ca.Each(func(id ecs.ID, _ *ecs.FleetComponent) {
		w.fleetIDs = append(w.fleetIDs, id)
	})
	if len(w.fleetIDs) == 0 {
		return
	}

	// Determine the desired heading of each fleet: AI fleets ask their AI
	// controller, the player fleet keeps its manual heading.
	headings := w.desiredHeadings(w.fleetIDs)

	workers := w.workerCount
	if workers > len(w.fleetIDs) {
		workers = len(w.fleetIDs)
	}
	if workers <= 1 {
		for _, id := range w.fleetIDs {
			w.advanceFleet(id, headings[id])
		}
		return
	}

	var wg sync.WaitGroup
	chunk := (len(w.fleetIDs) + workers - 1) / workers
	for i := 0; i < workers; i++ {
		start := i * chunk
		if start >= len(w.fleetIDs) {
			break
		}
		end := start + chunk
		if end > len(w.fleetIDs) {
			end = len(w.fleetIDs)
		}
		wg.Add(1)
		go func(ids []ecs.ID) {
			defer wg.Done()
			for _, id := range ids {
				w.advanceFleet(id, headings[id])
			}
		}(w.fleetIDs[start:end])
	}
	wg.Wait()
}

// desiredHeadings computes the target heading of every fleet.
//
// It returns a map from entity ID to heading in degrees. The map is
// allocated once and reused between ticks.
func (w *World) desiredHeadings(fleetIDs []ecs.ID) map[ecs.ID]float64 {
	if w.headings == nil {
		w.headings = make(map[ecs.ID]float64, 256)
	}
	for k := range w.headings {
		delete(w.headings, k)
	}
	for _, id := range fleetIDs {
		fleet := getFleetComponent(w.ecs, id)
		if fleet == nil {
			continue
		}
		pos := getPositionComponent(w.ecs, id)
		if pos == nil {
			continue
		}
		heading := pos.Heading
		if ai := getAIComponent(w.ecs, id); ai != nil && ai.TargetPortID != "" {
			target := w.cfg.Port(ai.TargetPortID)
			if target != nil {
				heading = bearingBetween(pos.X, pos.Y, target.X, target.Y)
			}
		}
		w.headings[id] = heading
	}
	return w.headings
}

// advanceEvents runs the daily random event checks for every fleet.
func (w *World) advanceEvents() {
	params := w.cfg.Params
	fleetStorage := w.ecs.Storage("FleetComponent")
	if fleetStorage == nil {
		return
	}
	ca, ok := fleetStorage.(*ecs.ComponentArray[ecs.FleetComponent])
	if !ok {
		return
	}
	ca.Each(func(id ecs.ID, f *ecs.FleetComponent) {
		if f.InPort != "" {
			return // no sea events while docked
		}

		// --- mutiny check ---
		if w.rng.Float64() < w.mutinyChance(f) {
			w.triggerMutiny(id, f)
			return
		}

		// --- random sea event ---
		if w.rng.Float64() < params.SeaEventPerDayChance {
			w.triggerSeaEvent(id, f)
			return
		}

		// --- discovery ---
		if w.rng.Float64() < params.DiscoveryPerDayChance {
			w.triggerDiscovery(id, f)
		}
	})
}

// triggerMutiny resolves a mutiny on a fleet.
//
// A mutiny scatters a part of the cargo and lowers morale and loyalty.
func (w *World) triggerMutiny(id ecs.ID, f *ecs.FleetComponent) {
	cargo := getCargoComponent(w.ecs, id)
	lost := 0.0
	if cargo != nil {
		for cid, qty := range cargo.Manifest {
			loss := qty * 0.3
			cargo.Manifest[cid] = qty - loss
			lost += loss
			if cargo.Manifest[cid] < 0 {
				cargo.Manifest[cid] = 0
			}
		}
	}
	f.Morale -= 25
	f.Loyalty -= 15
	if f.Morale < 0 {
		f.Morale = 0
	}
	if f.Loyalty < 0 {
		f.Loyalty = 0
	}
	w.Log("mutiny on %s! %.1f tons of cargo lost", f.Name, lost)
	w.emit("mutiny", map[string]any{"fleet": f.Name, "lost": lost})
}

// triggerSeaEvent rolls a sea event for a fleet.
//
// The event is selected from the event table by weight; the selection and
// the effect resolution live in internal/event, which owns the event
// templates and the Lua scripting.
//
// The whole probability model is table driven: the weight of the events,
// their day range and the chances of their effects all live in the
// configuration tables.
func (w *World) triggerSeaEvent(id ecs.ID, f *ecs.FleetComponent) {
	if w.events == nil {
		w.Log("an unusual encounter on the horizon of %s", f.Name)
		return
	}

	// --- select the event ---
	region := w.regionAt(posX(w, id), posY(w, id))
	template := w.events.Select(w.day, region, f.Name)
	if template == nil {
		w.Log("an unusual encounter on the horizon of %s", f.Name)
		return
	}

	// --- collect the effects ---
	effects, err := event.ParseEffects(template.Effects)
	if err != nil {
		w.Log("the event %q has a malformed effect: %v", template.Name, err)
		return
	}

	// --- run the Lua script, when the event has one ---
	if template.Script != "" && w.scripting != nil && w.scripting.Enabled() {
		outcome, err := w.scripting.Run(template.Script, map[string]any{
			"name":   template.Name,
			"day":    w.day,
			"region": region,
		})
		if err != nil {
			w.Log("the script of the event %q failed: %v", template.Name, err)
		} else if outcome != nil {
			effects = outcome.Effects
			if outcome.Message != "" {
				template.Message = outcome.Message
			}
		}
	}

	// --- apply the effects ---
	target := w.eventTarget(id, f)
	messages := w.resolver.Apply(effects, target)
	w.applyEventTarget(id, f, target)

	// --- report ---
	message := template.Message
	if len(messages) > 0 {
		if message != "" {
			message += "\n"
		}
		message += event.Message(messages)
	}
	if message == "" {
		message = fmt.Sprintf("%s: something happens to %s.", template.Name, f.Name)
	}
	w.Log("%s", message)
	w.emit("sea_event", map[string]any{
		"fleet":   id,
		"name":    f.Name,
		"event":   template.ID,
		"title":   template.Name,
		"x":       posX(w, id),
		"y":       posY(w, id),
		"day":     w.day,
		"message": message,
	})
}

// eventTarget builds the mutable target of an event from the state of a
// fleet.
func (w *World) eventTarget(id ecs.ID, f *ecs.FleetComponent) *event.TargetState {
	target := &event.TargetState{
		HullPoints:  f.HullPoints,
		Food:        f.Food,
		FreshWater:  f.FreshWater,
		Reputation:  f.Morale,
		Cargo:       make(map[string]float64),
		Discoveries: make(map[string]bool),
	}
	if ship := w.cfg.Ship(f.ShipID); ship != nil {
		target.MaxHullPoints = ship.HullPoints
	}
	if cargo := getCargoComponent(w.ecs, id); cargo != nil {
		for k, v := range cargo.Manifest {
			target.Cargo[k] = v
		}
	}
	if f.Discovered != nil {
		for k, v := range f.Discovered {
			target.Discoveries[k] = v
		}
	}
	return target
}

// applyEventTarget writes the result of an event back into the state of
// the fleet.
func (w *World) applyEventTarget(id ecs.ID, f *ecs.FleetComponent, target *event.TargetState) {
	f.HullPoints = target.HullPoints
	f.Food = target.Food
	f.FreshWater = target.FreshWater
	f.Morale = target.Reputation
	if cargo := getCargoComponent(w.ecs, id); cargo != nil {
		for k, v := range target.Cargo {
			if v <= 0 {
				delete(cargo.Manifest, k)
				continue
			}
			cargo.Manifest[k] = v
		}
		w.updateCargoUsage(id)
	}
	if len(target.Discoveries) > 0 {
		if f.Discovered == nil {
			f.Discovered = make(map[string]bool, 8)
		}
		for k, v := range target.Discoveries {
			f.Discovered[k] = v
		}
	}
}

// regionAt returns the region of the map position.
//
// The region is the region of the closest port, which is a simple and
// stable approximation of the geographic regions.
func (w *World) regionAt(x, y float64) string {
	best := ""
	bestDist := 0.0
	for _, id := range w.portIDs {
		p := w.cfg.Port(id)
		if p == nil {
			continue
		}
		d := (p.X-x)*(p.X-x) + (p.Y-y)*(p.Y-y)
		if best == "" || d < bestDist {
			best = p.Region
			bestDist = d
		}
	}
	return best
}

// triggerDiscovery creates a discovery for a fleet.
func (w *World) triggerDiscovery(id ecs.ID, f *ecs.FleetComponent) {
	if f.Discovered == nil {
		f.Discovered = make(map[string]bool, 16)
	}
	// A discovery is identified by its position, so the same location can
	// only be discovered once per fleet.
	key := discoveryKey(posX(w, id), posY(w, id))
	if f.Discovered[key] {
		return
	}
	f.Discovered[key] = true

	reward := w.cfg.Params.StartMoney * 0.05
	if f.IsPlayer {
		w.treasury += reward
		w.discovered[key] = w.day
		w.discoveredCount++
		w.Log("discovery! %s found something remarkable (reward %.0f)", f.Name, reward)
		w.emit("discovery", map[string]any{"name": f.Name, "reward": reward, "key": key})
	} else {
		w.Log("%s made a discovery", f.Name)
	}
}

// discoveryKey builds the deterministic key of a map position.
func discoveryKey(x, y float64) string {
	return formatCoord(x) + "," + formatCoord(y)
}

// formatCoord formats a coordinate with a 10-pixel granularity, so that
// two fleets within a few pixels share the same discovery.
func formatCoord(v float64) string {
	q := math.Round(v/10) * 10
	return strconv.Itoa(int(q))
}

// advanceCalendar advances the day counter.
func (w *World) advanceCalendar() {
	w.day++
	daysPerYear := w.cfg.Params.DaysPerGameYear
	if daysPerYear > 0 {
		if w.day > daysPerYear {
			w.day = 1
			w.year++
		}
	}
}

// weatherOrder returns the weather template IDs in deterministic order.
func (w *World) weatherOrder() []string {
	if w.weatherIDs == nil {
		w.weatherIDs = make([]string, 0, len(w.cfg.Weather))
		for id := range w.cfg.Weather {
			w.weatherIDs = append(w.weatherIDs, id)
		}
		sortStrings(w.weatherIDs)
	}
	return w.weatherIDs
}

// posX returns the map X of an entity (0 when unknown).
func posX(w *World, id ecs.ID) float64 {
	if p := getPositionComponent(w.ecs, id); p != nil {
		return p.X
	}
	return 0
}

// posY returns the map Y of an entity (0 when unknown).
func posY(w *World, id ecs.ID) float64 {
	if p := getPositionComponent(w.ecs, id); p != nil {
		return p.Y
	}
	return 0
}

// randFloat returns a uniform random number in [0,1) from the world RNG.
//
// It exists so that callers outside the package do not need to import
// math/rand/v2.
func (w *World) randFloat() float64 { return w.rng.Float64() }

// randRange returns a uniform random number in [lo,hi).
func (w *World) randRange(lo, hi float64) float64 {
	return lo + w.rng.Float64()*(hi-lo)
}

// randNorm returns a normally distributed number with the given mean and
// standard deviation, using the central limit theorem on the fast uniform
// generator.
func (w *World) randNorm(mean, stddev float64) float64 {
	g := rand.Float64() + rand.Float64() + rand.Float64() + rand.Float64() - 2
	return mean + g*stddev
}
