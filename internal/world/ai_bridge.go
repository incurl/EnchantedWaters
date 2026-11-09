package world

import (
	"github.com/peter/enchantedwaters/internal/ai"
	"github.com/peter/enchantedwaters/internal/ecs"
)

// This file connects the AI package to the world simulation.
//
// The world builds a read-only market snapshot, hands it to the AI
// controller together with a snapshot of every AI fleet, and applies the
// decisions afterwards. This keeps the AI free of any dependency on the
// world package and makes the whole evaluation lock-free.

// SetAIController installs the AI controller of the world.
//
// The controller is created lazily by the caller; the world only needs it
// to evaluate the AI fleets.
func (w *World) SetAIController(c *ai.Controller) { w.ai = c }

// buildMarketView fills a market snapshot from the current port states.
//
// The snapshot is a deep copy of the price/stock/demand vectors, so the
// parallel workers can read it without any locking.
func (w *World) buildMarketView() *ai.MarketView {
	view := ai.NewMarketView()
	for _, portID := range w.portIDs {
		pd := w.ports[portID]
		if pd == nil {
			continue
		}
		pd.mu.Lock()
		for _, cid := range w.cfg.CommodityOrder {
			key := portID + "\x00" + cid
			view.Prices[key] = pd.prices[cid]
			view.Stock[key] = pd.stock[cid]
			view.Demand[key] = w.cfg.PortDemand(portID, cid)
		}
		pd.mu.Unlock()
		if port := w.cfg.Port(portID); port != nil {
			view.Tax[portID] = port.BaseTaxRate
		}
	}
	return view
}

// updateAIFleets evaluates every AI fleet and applies the results.
//
// The evaluation is parallelised: the fleets are snapshotted, the workers
// evaluate them independently, and the results are applied back to the
// ECS.
func (w *World) updateAIFleets() {
	if w.ai == nil {
		return
	}
	fleetStore := ecs.ComponentArrayOf[ecs.FleetComponent](w.ecs)
	aiStore := ecs.ComponentArrayOf[ecs.AIComponent](w.ecs)
	posStore := ecs.ComponentArrayOf[ecs.PositionComponent](w.ecs)
	cargoStore := ecs.ComponentArrayOf[ecs.CargoComponent](w.ecs)
	if fleetStore == nil || aiStore == nil || posStore == nil || cargoStore == nil {
		return
	}

	// --- collect the AI fleets ---
	if w.aiRequests == nil {
		w.aiRequests = make([]ai.UpdateRequest, 0, 64)
	}
	w.aiRequests = w.aiRequests[:0]

	aiStore.Each(func(id ecs.ID, aiComp *ecs.AIComponent) {
		fleet := getFleetComponent(w.ecs, id)
		pos := getPositionComponent(w.ecs, id)
		if fleet == nil || pos == nil {
			return
		}
		ship := w.cfg.Ship(fleet.ShipID)
		hullRatio := 1.0
		if ship != nil && ship.HullPoints > 0 {
			hullRatio = fleet.HullPoints / ship.HullPoints
		}
		state := ai.FleetState{
			ID:        id,
			ShipID:    fleet.ShipID,
			Owner:     fleet.Owner,
			InPort:    fleet.InPort,
			Position:  *pos,
			HullRatio: hullRatio,
		}
		if cargo := getCargoComponent(w.ecs, id); cargo != nil {
			// The manifest is copied so that the workers never see the
			// live map.
			state.Cargo = make(map[string]float64, len(cargo.Manifest))
			for k, v := range cargo.Manifest {
				state.Cargo[k] = v
			}
			state.Gold = cargo.Gold
		}
		w.aiRequests = append(w.aiRequests, ai.UpdateRequest{Fleet: state, AI: *aiComp})
	})
	if len(w.aiRequests) == 0 {
		return
	}

	// --- evaluate in parallel ---
	view := w.buildMarketView()
	results := w.ai.UpdateBatch(view, w.aiRequests, w.workerCount)

	// --- apply the results ---
	for i := range results {
		r := &results[i]
		aiComp := ecs.GetComponent[ecs.AIComponent](w.ecs, r.ID)
		if aiComp == nil {
			continue
		}
		*aiComp = r.AI
	}
}

// tradeAIFleets performs the actual buying and selling of the AI fleets.
//
// It runs after the AI decisions, so that the AI acts on the market it
// just evaluated.
func (w *World) tradeAIFleets() {
	if w.ai == nil {
		return
	}
	fleetStore := ecs.ComponentArrayOf[ecs.FleetComponent](w.ecs)
	aiStore := ecs.ComponentArrayOf[ecs.AIComponent](w.ecs)
	cargoStore := ecs.ComponentArrayOf[ecs.CargoComponent](w.ecs)
	if fleetStore == nil || aiStore == nil || cargoStore == nil {
		return
	}

	aiStore.Each(func(id ecs.ID, aiComp *ecs.AIComponent) {
		fleet := getFleetComponent(w.ecs, id)
		cargo := getCargoComponent(w.ecs, id)
		if fleet == nil || cargo == nil {
			return
		}
		if fleet.InPort == "" {
			return // only docked fleets trade
		}
		// The AI keeps its money in the cargo component, which is where
		// the save system persists it.
		gold := cargo.Gold

		// --- sell the cargo at the current port ---
		for cid, qty := range cargo.Manifest {
			if qty <= 0 {
				continue
			}
			price := w.Price(cid, fleet.InPort)
			if price <= 0 {
				continue
			}
			port := w.cfg.Port(fleet.InPort)
			tax := 0.0
			if port != nil {
				tax = port.BaseTaxRate
			}
			gold += qty * price * (1 - tax)
			delete(cargo.Manifest, cid)
		}
		w.updateCargoUsage(id)

		// --- buy the goods for the next voyage ---
		if aiComp.TargetCommodity == "" {
			cargo.Gold = gold
			return
		}
		commodity := w.cfg.Commodity(aiComp.TargetCommodity)
		if commodity == nil {
			cargo.Gold = gold
			return
		}
		ship := w.cfg.Ship(fleet.ShipID)
		if ship == nil {
			cargo.Gold = gold
			return
		}
		// The AI fills a fraction of the hold, as configured by its
		// profile.
		profile := w.cfg.AIProfiles[aiComp.ProfileID]
		fill := w.cfg.Params.AICargoFill
		if fill <= 0 {
			fill = 1
		}
		if profile != nil {
			fill = profile.CargoAggressiveness
		}
		capacity := ship.CargoCapacity * fill
		price := w.SellPrice(aiComp.TargetCommodity, fleet.InPort)
		if price <= 0 {
			cargo.Gold = gold
			return
		}
		port := w.cfg.Port(fleet.InPort)
		tax := 0.0
		if port != nil {
			tax = port.BaseTaxRate
		}
		unitCost := price * (1 + tax)
		quantity := capacity / commodity.WeightPerUnit
		// The AI cannot spend more gold than it has.
		if maxByGold := gold / unitCost; quantity > maxByGold {
			quantity = maxByGold
		}
		// The AI cannot buy more than the port has in stock.
		if stock := w.Stock(aiComp.TargetCommodity, fleet.InPort); quantity > stock {
			quantity = stock
		}
		if quantity <= 0 {
			cargo.Gold = gold
			return
		}
		cargo.Manifest[aiComp.TargetCommodity] += quantity
		gold -= quantity * unitCost
		cargo.Gold = gold
		w.updateCargoUsage(id)
		w.adjustStock(aiComp.TargetCommodity, fleet.InPort, -quantity)
	})
}
