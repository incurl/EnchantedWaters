// Package ai implements the NPC fleet behaviour.
//
// # Decision model
//
// An AI fleet runs a small state machine:
//
//	idle    -> evaluate the best trade opportunity
//	trade   -> sail to the port where the goods can be sold
//	return -> sail back home when the hold is empty or the voyage is long
//	flee    -> run for the nearest port when the hull is damaged
//
// The evaluation of a trade opportunity is a pure function of the market
// state, so it can be computed in parallel for many fleets:
//
//	expectedProfit(c, from, to) = (price(to, c) - price(from, c) - tax)
//	                           * quantity
//
// The AI picks the (commodity, destination) pair with the highest risk
// adjusted profit:
//
//	score = expectedProfit * (1 - RiskAversion * volatility(c))
//
// where volatility(c) is the relative price amplitude of the commodity
// (its cycle amplitude), and where a specialty commodity produced at the
// destination gets an extra bonus.
//
// # Concurrency
//
// The controller never touches the ECS directly. The world builds a
// MarketView snapshot and a list of FleetState snapshots, the parallel
// workers evaluate them independently, and the results are applied back
// by the world. This keeps the AI free of locks and of shared mutable
// state.
package ai

import (
	"math"
	"math/rand/v2"
	"sort"

	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
	"github.com/peter/enchantedwaters/internal/sail"
)

// fleeHullRatio is the relative condition of the hull below which the AI
// stops trading and runs for the nearest port.
const fleeHullRatio = 0.2

// Controller drives the AI fleets of one world.
//
// The controller reads the immutable configuration and the current market
// state; it never mutates the static configuration.
type Controller struct {
	// cfg is the immutable configuration registry.
	cfg *config.Registry
	// rng is the random source of the controller.
	rng *rand.Rand
	// portIDs is the deterministic list of port IDs.
	portIDs []string
	// commodityIDs is the deterministic list of commodity IDs.
	commodityIDs []string
}

// Decision is the outcome of an AI evaluation.
type Decision struct {
	// Commodity is the commodity the AI wants to buy.
	Commodity string
	// SourcePort is where the AI buys the commodity.
	SourcePort string
	// TargetPort is where the AI sells it.
	TargetPort string
	// Profit is the expected profit of the voyage.
	Profit float64
	// Score is the risk adjusted score of the opportunity.
	Score float64
	// Reason explains the decision in human readable form.
	Reason string
}

// FleetState is the snapshot of a fleet that the AI works on.
//
// It is a value type: the parallel workers each receive their own copy,
// so no locking is required while evaluating.
type FleetState struct {
	// ID is the entity ID of the fleet.
	ID ecs.ID
	// ShipID is the ship class of the fleet.
	ShipID string
	// Owner is the faction that controls the fleet.
	Owner string
	// InPort is the port the fleet is docked at ("" = at sea).
	InPort string
	// Position is the map position of the fleet.
	Position ecs.PositionComponent
	// Cargo maps a commodity ID to the carried amount.
	Cargo map[string]float64
	// Gold is the money carried aboard.
	Gold float64
	// HullRatio is the relative condition of the hull, in [0,1].
	HullRatio float64
}

// NewController creates an AI controller.
func NewController(cfg *config.Registry, seed uint64) *Controller {
	c := &Controller{
		cfg: cfg,
		rng: rand.New(rand.NewPCG(seed, seed^0xD1B54A32D192ED03)),
	}
	c.portIDs = make([]string, 0, len(cfg.Ports))
	for id := range cfg.Ports {
		c.portIDs = append(c.portIDs, id)
	}
	sort.Strings(c.portIDs)

	c.commodityIDs = make([]string, 0, len(cfg.Commodities))
	for id := range cfg.Commodities {
		c.commodityIDs = append(c.commodityIDs, id)
	}
	sort.Strings(c.commodityIDs)
	return c
}

// MarketView gives the AI a read-only snapshot of the market.
//
// It is filled once per tick by the world and then read by the parallel
// workers, which is what makes the parallel AI evaluation safe.
type MarketView struct {
	// Prices maps "portID\x00commodityID" to the current price.
	Prices map[string]float64
	// Stock maps "portID\x00commodityID" to the available stock.
	Stock map[string]float64
	// Demand maps "portID\x00commodityID" to the daily demand.
	Demand map[string]float64
	// Tax maps a port ID to its tax rate.
	Tax map[string]float64
}

// NewMarketView creates an empty market view.
func NewMarketView() *MarketView {
	return &MarketView{
		Prices: make(map[string]float64, 1024),
		Stock:  make(map[string]float64, 1024),
		Demand: make(map[string]float64, 1024),
		Tax:    make(map[string]float64, 64),
	}
}

// priceKey builds the composite key of the market view.
func priceKey(portID, commodityID string) string {
	return portID + "\x00" + commodityID
}

// price returns the price of a commodity at a port from the snapshot.
func (m *MarketView) price(portID, commodityID string) float64 {
	return m.Prices[priceKey(portID, commodityID)]
}

// Evaluate analyses the market and returns the best trade opportunity for
// a fleet that is currently at sourcePort.
//
// The search space is (commodity x destination port). For large data sets
// this is the hot loop of the AI, so it is written to avoid allocations.
func (c *Controller) Evaluate(view *MarketView, sourcePort string, riskAversion float64, capacity float64) *Decision {
	best := &Decision{}
	bestScore := math.Inf(-1)

	src := c.cfg.Port(sourcePort)
	if src == nil {
		return nil
	}

	for _, commodityID := range c.commodityIDs {
		commodity := c.cfg.Commodities[commodityID]
		if commodity == nil {
			continue
		}
		buyPrice := view.price(sourcePort, commodityID)
		if buyPrice <= 0 {
			continue
		}
		// The quantity that can be bought at the source.
		supply := view.Stock[priceKey(sourcePort, commodityID)]
		if supply <= 0 {
			continue
		}
		quantity := math.Min(supply, capacity/commodity.WeightPerUnit)
		if quantity <= 0 {
			continue
		}

		for _, targetPort := range c.portIDs {
			if targetPort == sourcePort {
				continue
			}
			sellPrice := view.price(targetPort, commodityID)
			if sellPrice <= 0 {
				continue
			}
			// The target's absorption capacity limits the quantity.
			demand := view.Demand[priceKey(targetPort, commodityID)]
			if demand <= 0 {
				continue
			}
			qty := math.Min(quantity, demand*10)
			if qty <= 0 {
				continue
			}

			// --- expected profit ---
			// The round trip pays the tax twice (once on the purchase,
			// once on the sale). The weight of the transaction costs is
			// a global parameter, so that the trade skill of the AI can
			// be tuned globally.
			taxFactor := c.cfg.Params.AITradeTaxFactor
			if taxFactor <= 0 {
				taxFactor = 1
			}
			taxRate := (view.Tax[sourcePort] + view.Tax[targetPort]) * taxFactor
			margin := sellPrice - buyPrice
			profit := (margin - (buyPrice+sellPrice)*taxRate) * qty
			if profit <= 0 {
				continue
			}

			// --- risk adjustment ---
			// The volatility of a commodity is its cycle amplitude; a
			// risk averse AI discounts volatile goods.
			//
			// The discount is a reciprocal, which keeps the multiplier
			// positive and bounded for every parameter value:
			//
			//	multiplier = 1 / (1 + riskAversion * volatility)
			//
			// It is 1 for a risk neutral AI and a perfectly stable
			// commodity, and it decreases monotonically with both
			// parameters.
			volatility := commodity.CycleAmplitude
			score := profit / (1 + riskAversion*volatility)

			// --- specialty bonus ---
			// A commodity produced at the destination is cheaper
			// everywhere else, so selling it there is a bonus.
			if target := c.cfg.Port(targetPort); target != nil && target.SpecialtyCommodity == commodityID {
				score *= 1 + target.SpecialtyDiscount
			}

			// --- distance penalty ---
			// A long voyage consumes supplies and exposes the fleet to
			// storms, so the profit per day must stay attractive.
			dst := c.cfg.Port(targetPort)
			if dst != nil {
				dist := math.Hypot(dst.X-src.X, dst.Y-src.Y)
				days := dist / math.Max(1, c.cfg.Params.TravelSpeedScale*avgShipSpeed(c.cfg))
				if days > 0 {
					score /= days
				}
			}

			if score > bestScore {
				bestScore = score
				best.Commodity = commodityID
				best.SourcePort = sourcePort
				best.TargetPort = targetPort
				best.Profit = profit
				best.Score = score
				best.Reason = "buy " + commodity.Name + " at " + sourcePort + ", sell at " + targetPort
			}
		}
	}
	if bestScore == math.Inf(-1) {
		return nil
	}
	return best
}

// avgShipSpeed returns the average typical speed of all ship classes.
//
// It is used to estimate the duration of a voyage. The figure comes from
// the sailing model evaluated at ordinary conditions, so it follows the
// hull table rather than a hand written number.
func avgShipSpeed(cfg *config.Registry) float64 {
	sum := 0.0
	n := 0
	for _, id := range cfg.ShipOrder {
		s := cfg.Ships[id]
		if s == nil {
			continue
		}
		sum += sail.TypicalSpeed(s.Power, s.SailType, s.MinCrew, int(s.CargoCapacity))
		n++
	}
	if n == 0 {
		return 1
	}
	return sum / float64(n)
}

// UpdateFleet advances the AI state machine of a single fleet.
//
// It returns the fleet's new AI state and the decision that produced it.
// The function reads only the snapshots, so many fleets can be updated in
// parallel.
func (c *Controller) UpdateFleet(view *MarketView, fleet *FleetState, ai *ecs.AIComponent) (string, *Decision) {
	profile := c.cfg.AIProfiles[ai.ProfileID]
	if profile == nil {
		return "idle", nil
	}

	// --- flee when the hull is critically damaged ---
	//
	// A ratio of zero means "unknown", which happens when the snapshot
	// could not be built; in that case the AI does not flee.
	if fleet.HullRatio > 0 && fleet.HullRatio < fleeHullRatio {
		ai.State = "flee"
		ai.TargetPortID = c.nearestPort(fleet.Position.X, fleet.Position.Y)
		ai.LastDecision = "hull too damaged: flee to " + ai.TargetPortID
		return ai.State, nil
	}

	// --- arrive at a port ---
	if fleet.InPort != "" {
		ai.VoyageDays = 0
		decision := c.Evaluate(view, fleet.InPort, ai.RiskAversion, cargoCapacity(c.cfg, fleet.ShipID))
		if decision == nil {
			ai.State = "idle"
			ai.LastDecision = "no profitable trade found"
			return ai.State, nil
		}
		ai.TargetCommodity = decision.Commodity
		ai.TargetPortID = decision.TargetPort
		ai.LastDecision = decision.Reason
		ai.State = "trade"
		return ai.State, decision
	}

	// --- at sea ---
	ai.VoyageDays++
	if ai.TargetPortID == "" {
		ai.State = "idle"
		return ai.State, nil
	}
	// Return home when the voyage is too long.
	if ai.VoyageDays >= profile.MaxVoyageDays {
		ai.State = "return"
		ai.TargetPortID = profile.HomePort
		if ai.TargetPortID == "" {
			ai.TargetPortID = c.nearestPort(fleet.Position.X, fleet.Position.Y)
		}
		ai.LastDecision = "voyage too long, returning home"
		return ai.State, nil
	}
	ai.State = "trade"
	return ai.State, nil
}

// nearestPort returns the port closest to a position.
func (c *Controller) nearestPort(x, y float64) string {
	best := ""
	bestDist := math.Inf(1)
	for _, id := range c.portIDs {
		p := c.cfg.Port(id)
		if p == nil {
			continue
		}
		d := math.Hypot(p.X-x, p.Y-y)
		if d < bestDist {
			bestDist = d
			best = id
		}
	}
	return best
}

// cargoCapacity returns the usable cargo capacity of a ship class.
//
// A fraction of the nominal capacity is kept free for consumables.
func cargoCapacity(cfg *config.Registry, shipID string) float64 {
	ship := cfg.Ship(shipID)
	if ship == nil {
		return 0
	}
	return ship.CargoCapacity * 0.8
}
