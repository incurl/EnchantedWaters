// Package world implements the world simulation: economy, weather,
// navigation and the turn/calendar loop.
//
// # Layering
//
// The world package owns the *dynamic state* layer. It reads immutable
// configuration from internal/config and never writes to it. Every numeric
// constant used here comes from a configuration table.
//
// # The economic model
//
// The price of commodity c at port p on day d is modelled as the product of
// independent factors:
//
//	price(c,p,d) = Base(c)
//	             * Cycle(c,d)        // business cycle, sine shaped
//	             * Season(c,d)       // yearly season, sine shaped
//	             * StaticFactor(c,p) // regional price gap (static table)
//	             * Specialty(c,p)    // local production discount
//	             * Scarcity(c,p)     // supply/demand from local stock
//	             * (1 + Noise(c,p))  // bounded random walk
//
// Each factor is normalised around 1.0, so the product is directly
// interpretable as "how expensive is this good here compared to its base
// price".
package world

import (
	"math"
	"math/rand/v2"
	"sync"
)

// CommodityPrice computes the current price of a commodity at a port.
//
// The function is pure: it reads the port's dynamic state and the immutable
// registry, and returns the price. It never mutates either, which makes it
// safe to call from parallel AI workers.
func (w *World) CommodityPrice(commodityID, portID string) float64 {
	c := w.cfg.Commodity(commodityID)
	if c == nil {
		return 0
	}
	p := w.cfg.Port(portID)
	if p == nil {
		return 0
	}
	pd := w.portState(portID)
	if pd == nil {
		return 0
	}

	price := c.BasePrice
	price *= w.cycleFactor(c, pd)
	price *= w.seasonalFactor(c)
	price *= w.cfg.PortPriceFactor(portID, commodityID)
	price *= w.specialtyFactor(c, p)
	price *= w.scarcityFactor(c, pd)
	price *= 1.0 + w.noiseOf(pd, commodityID)

	// Apply the static clamps from the configuration table.
	if c.MinPrice > 0 && price < c.MinPrice {
		price = c.MinPrice
	}
	if c.MaxPrice > 0 && price > c.MaxPrice {
		price = c.MaxPrice
	}
	// Port level clamps are relative to the base price.
	if c.MinPortPriceFactor > 0 {
		if min := c.BasePrice * c.MinPortPriceFactor; price < min {
			price = min
		}
	}
	if c.MaxPortPriceFactor > 0 {
		if max := c.BasePrice * c.MaxPortPriceFactor; price > max {
			price = max
		}
	}
	return math.Round(price*100) / 100
}

// cycleFactor returns the business cycle multiplier of a commodity.
//
// The business cycle is a sine wave over the cycle period:
//
//	Cycle(c,d) = 1 + A * sin(2*pi * (d + phase) / P)
//
// The phase advances with the port's CyclePhase, which is advanced every
// day by the simulation. Ports in the same region share the phase, which
// models the fact that a business cycle affects a whole region at once.
func (w *World) cycleFactor(c *configCommodity, pd *portDynamic) float64 {
	period := c.CyclePeriodDays
	if period <= 0 {
		return 1
	}
	// Port specific phase offset so that ports do not move in lockstep.
	phase := pd.cyclePhase*period + portPhaseOffset(pd.portID)
	theta := 2 * math.Pi * phase / period
	return 1 + c.CycleAmplitude*math.Sin(theta)
}

// seasonalFactor returns the yearly seasonal multiplier.
//
//	Season(c,d) = 1 + A * sin(2*pi * (dayOfYear + phase) / daysPerYear)
func (w *World) seasonalFactor(c *configCommodity) float64 {
	if c.SeasonalAmplitude == 0 {
		return 1
	}
	daysPerYear := float64(w.cfg.Params.DaysPerGameYear)
	if daysPerYear <= 0 {
		return 1
	}
	dayOfYear := math.Mod(float64(w.day), daysPerYear)
	theta := 2 * math.Pi * (dayOfYear + c.SeasonalPhase) / daysPerYear
	return 1 + c.SeasonalAmplitude*math.Sin(theta)
}

// specialtyFactor applies the local production discount.
//
// At a port that produces the commodity, the price is reduced by the
// configured discount; everywhere else the factor is 1.
func (w *World) specialtyFactor(c *configCommodity, p *configPort) float64 {
	if p.SpecialtyCommodity != c.ID || p.SpecialtyDiscount <= 0 {
		return 1
	}
	return 1 - p.SpecialtyDiscount
}

// scarcityFactor converts the local stock into a price multiplier.
//
// The model is a proportional feedback loop around the equilibrium stock:
//
//	Equilibrium = Demand / (Supply + Demand)
//	Ratio      = Stock / (Stock + Equilibrium)
//	Scarcity   = 1 + K * (1 - 2*Ratio)
//
// When the stock equals the equilibrium, the factor is exactly 1. When the
// stock is empty, the factor is 1+K (expensive); when the market is
// saturated, the factor is 1-K (cheap). K is the global scarcity response
// from the configuration table.
//
// The PortDynamicComponent holds a smoothed version of this value so that
// prices do not jump on every single transaction.
func (w *World) scarcityFactor(c *configCommodity, pd *portDynamic) float64 {
	supply := w.cfg.PortSupply(pd.portID, c.ID)
	demand := w.cfg.PortDemand(pd.portID, c.ID)
	if supply <= 0 && demand <= 0 {
		return 1
	}
	stock := pd.stock[c.ID]
	equilibrium := demand / (supply + demand)
	if equilibrium <= 0 {
		return 1
	}
	ratio := stock / (stock + equilibrium)
	k := w.cfg.Params.ScarcityResponse
	factor := 1 + k*(1-2*ratio)

	// Blend with the smoothed scarcity stored in the port state, so that
	// individual trades have a damped effect.
	smoothed := pd.scarcity[c.ID]
	if smoothed == 0 {
		smoothed = factor
	}
	blend := w.cfg.Params.ScarcitySmoothing
	return smoothed*(1-blend) + factor*blend
}

// noiseOf returns the current stochastic price perturbation of a commodity.
//
// The noise is a bounded mean-reverting random walk that is advanced once
// per day by the economy update:
//
//	e_{t+1} = e_t * (1 - decay) + sigma * N(0,1)
//
// and clamped to +-PriceNoiseScale.
func (w *World) noiseOf(pd *portDynamic, commodityID string) float64 {
	return pd.noise[commodityID]
}

// portPhaseOffset derives a stable per-port phase offset from the port ID.
//
// The offset is a hash of the ID mapped into [0,1). It makes every port
// cycle slightly out of phase, which produces natural-looking price
// differences across the world without any extra data.
func portPhaseOffset(portID string) float64 {
	if portID == "" {
		return 0
	}
	var h uint32 = 2166136261
	for i := 0; i < len(portID); i++ {
		h ^= uint32(portID[i])
		h *= 16777619
	}
	return float64(h%1000) / 1000.0
}

// updateEconomy advances the economic simulation by one day.
//
// It performs, in order:
//  1. Business cycle phase advance for every port.
//  2. Production and consumption of every commodity at every port.
//  3. Price noise random walk for every commodity at every port.
//  4. Scarcity factor smoothing.
//
// The per-port work is independent, so the whole update is parallelised
// across goroutines with a worker pool.
func (w *World) updateEconomy() {
	ports := w.portIDs
	workers := w.workerCount
	if workers > len(ports) {
		workers = len(ports)
	}
	if workers <= 1 {
		for _, id := range ports {
			w.updatePortEconomy(id)
		}
		return
	}

	var wg sync.WaitGroup
	ch := make(chan string, len(ports))
	for _, id := range ports {
		ch <- id
	}
	close(ch)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range ch {
				w.updatePortEconomy(id)
			}
		}()
	}
	wg.Wait()
}

// updatePortEconomy advances the economy of a single port by one day.
//
// Each port owns its own shard lock, so ports can be updated in parallel
// without contention.
func (w *World) updatePortEconomy(portID string) {
	pd := w.portState(portID)
	if pd == nil {
		return
	}
	p := w.cfg.Port(portID)
	if p == nil {
		return
	}

	pd.mu.Lock()
	defer pd.mu.Unlock()

	// --- 1. business cycle phase ---
	// The phase advances by 1/period per day; the period is taken from the
	// first commodity of the port as the canonical market cycle length.
	period := w.marketCyclePeriod()
	pd.cyclePhase += 1 / period
	if pd.cyclePhase >= 1 {
		pd.cyclePhase -= math.Floor(pd.cyclePhase)
	}
	// The trend is the forward difference of the phase, used by the AI to
	// anticipate rising or falling prices.
	pd.cycleTrend = 1 / period

	// --- 2. production and consumption ---
	for _, cid := range w.cfg.CommodityOrder {
		c := w.cfg.Commodities[cid]
		supply := w.cfg.PortSupply(portID, cid)
		demand := w.cfg.PortDemand(portID, cid)
		if supply == 0 && demand == 0 {
			continue
		}
		stock := pd.stock[cid]
		stock += supply
		stock -= demand
		if stock < 0 {
			// A deficit is paid for by the next shipment; keep the stock
			// non-negative but remember the shortage via the price.
			stock = 0
		}
		// Perishable goods decay.
		if c.Perishable && c.SpoilageRate > 0 {
			stock *= 1 - c.SpoilageRate
		}
		pd.stock[cid] = stock
	}

	// --- 3. price noise random walk ---
	decay := w.cfg.Params.PriceNoiseDecay
	scale := w.cfg.Params.PriceNoiseScale
	for _, cid := range w.cfg.CommodityOrder {
		prev := pd.noise[cid]
		// Gaussian noise via the central limit theorem on the fast
		// uniform generator: this is cheap and avoids a per-call
		// allocation of a new distribution object.
		g := (rand.Float64() + rand.Float64() + rand.Float64() + rand.Float64() - 2) // ~N(0,1)
		next := prev*(1-decay) + scale*g
		if next > scale {
			next = scale
		}
		if next < -scale {
			next = -scale
		}
		pd.noise[cid] = next
	}

	// --- 4. scarcity smoothing + recomputed price cache ---
	blend := w.cfg.Params.ScarcitySmoothing
	for _, cid := range w.cfg.CommodityOrder {
		supply := w.cfg.PortSupply(portID, cid)
		demand := w.cfg.PortDemand(portID, cid)
		if supply <= 0 && demand <= 0 {
			pd.scarcity[cid] = 1
			continue
		}
		equilibrium := demand / (supply + demand)
		stock := pd.stock[cid]
		ratio := stock / (stock + equilibrium)
		raw := 1 + w.cfg.Params.ScarcityResponse*(1-2*ratio)
		prev := pd.scarcity[cid]
		if prev == 0 {
			prev = raw
		}
		pd.scarcity[cid] = prev*(1-blend) + raw*blend
	}

	// Refresh the cached price vector so the UI can read it directly.
	for _, cid := range w.cfg.CommodityOrder {
		pd.prices[cid] = w.commodityPriceLocked(w.cfg.Commodities[cid], pd, p)
	}
}

// marketCyclePeriod returns the canonical business cycle length in days.
//
// It is the cycle period of the first commodity in the deterministic
// ordering, so that all commodities share the market rhythm while keeping
// their individual amplitudes.
func (w *World) marketCyclePeriod() float64 {
	if len(w.cfg.CommodityOrder) == 0 {
		return 100
	}
	period := w.cfg.Commodities[w.cfg.CommodityOrder[0]].CyclePeriodDays
	if period <= 0 {
		return 100
	}
	return period
}

// commodityPriceLocked computes the price while the port lock is held.
//
// It mirrors CommodityPrice but reads the port state directly, avoiding a
// lock re-acquisition.
func (w *World) commodityPriceLocked(c *configCommodity, pd *portDynamic, p *configPort) float64 {
	price := c.BasePrice
	price *= w.cycleFactor(c, pd)
	price *= w.seasonalFactor(c)
	price *= w.cfg.PortPriceFactor(pd.portID, c.ID)
	price *= w.specialtyFactor(c, p)
	price *= w.scarcityFactor(c, pd)
	price *= 1 + pd.noise[c.ID]

	if c.MinPrice > 0 && price < c.MinPrice {
		price = c.MinPrice
	}
	if c.MaxPrice > 0 && price > c.MaxPrice {
		price = c.MaxPrice
	}
	if c.MinPortPriceFactor > 0 {
		if min := c.BasePrice * c.MinPortPriceFactor; price < min {
			price = min
		}
	}
	if c.MaxPortPriceFactor > 0 {
		if max := c.BasePrice * c.MaxPortPriceFactor; price > max {
			price = max
		}
	}
	return math.Round(price*100) / 100
}
