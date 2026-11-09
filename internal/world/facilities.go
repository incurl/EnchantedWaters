package world

import (
	"fmt"
	"math"

	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
)

// This file implements the port facilities: the shipyard, the tavern, the
// governor's mansion, the church and the inn.

// RepairQuote describes the cost of repairing a hull.
type RepairQuote struct {
	// MissingPoints is the number of hull points to restore.
	MissingPoints float64
	// Cost is the total repair cost.
	Cost float64
	// Days is how long the repair takes.
	Days int
	// Affordable reports whether the player can pay.
	Affordable bool
}

// QuoteRepair computes the repair cost of a fleet at a shipyard.
//
// cost = missingHullPoints * RepairCostPerHP
//
// The repair speed (hull points restored per day) comes from the global
// parameters table.
func (w *World) QuoteRepair(fleetID ecs.ID, portID string) RepairQuote {
	var q RepairQuote
	fleet := getFleetComponent(w.ecs, fleetID)
	if fleet == nil {
		return q
	}
	ship := w.cfg.Ship(fleet.ShipID)
	port := w.cfg.Port(portID)
	if ship == nil || port == nil {
		return q
	}
	q.MissingPoints = ship.HullPoints - fleet.HullPoints
	if q.MissingPoints <= 0 {
		return q
	}
	q.Cost = q.MissingPoints * ship.RepairCostPerHP
	q.Days = int(math.Ceil(q.MissingPoints / math.Max(1, w.cfg.Params.ShipyardRepairSpeed)))
	q.Affordable = w.treasury >= q.Cost
	return q
}

// ExecuteRepair repairs a fleet's hull at a shipyard.
func (w *World) ExecuteRepair(fleetID ecs.ID, portID string) (RepairQuote, error) {
	q := w.QuoteRepair(fleetID, portID)
	if q.MissingPoints <= 0 {
		return q, fmt.Errorf("the hull is already sound")
	}
	if !q.Affordable {
		return q, fmt.Errorf("not enough gold: %.0f needed", q.Cost)
	}
	fleet := getFleetComponent(w.ecs, fleetID)
	ship := w.cfg.Ship(fleet.ShipID)
	if fleet == nil || ship == nil {
		return q, fmt.Errorf("unknown fleet")
	}
	w.treasury -= q.Cost
	fleet.HullPoints = ship.HullPoints
	w.Log("repaired %s for %.0f gold", fleet.Name, q.Cost)
	return q, nil
}

// PurchaseQuote describes the cost of buying a new ship.
type PurchaseQuote struct {
	// ShipID is the ship class being bought.
	ShipID string
	// Cost is the purchase price.
	Cost float64
	// TradeIn is the credit granted for the old ship.
	TradeIn float64
	// Affordable reports whether the player can pay.
	Affordable bool
}

// QuotePurchase computes the price of a ship at a shipyard.
//
// A larger port offers a discount, modelled by a per-class factor.
func (w *World) QuotePurchase(shipID, portID string) PurchaseQuote {
	q := PurchaseQuote{ShipID: shipID}
	ship := w.cfg.Ship(shipID)
	port := w.cfg.Port(portID)
	if ship == nil || port == nil {
		return q
	}
	discount := 1.0
	switch port.Class {
	case config.PortCity:
		discount = 0.95
	case config.PortCapital:
		discount = 0.9
	}
	q.Cost = ship.Price * discount
	q.Affordable = w.treasury >= q.Cost
	return q
}

// ExecutePurchase buys a new ship at a shipyard.
//
// The old ship is traded in for half of its price, mirroring the original
// game's "sell and buy" flow.
func (w *World) ExecutePurchase(fleetID ecs.ID, shipID, portID string) (PurchaseQuote, error) {
	q := w.QuotePurchase(shipID, portID)
	if !q.Affordable {
		return q, fmt.Errorf("not enough gold")
	}
	fleet := getFleetComponent(w.ecs, fleetID)
	newShip := w.cfg.Ship(shipID)
	if fleet == nil || newShip == nil {
		return q, fmt.Errorf("unknown ship")
	}
	oldShip := w.cfg.Ship(fleet.ShipID)

	// Trade in the old ship.
	if oldShip != nil {
		q.TradeIn = oldShip.Price * 0.5
	}
	w.treasury -= q.Cost
	w.treasury += q.TradeIn

	fleet.ShipID = shipID
	fleet.HullPoints = newShip.HullPoints
	if newShip.MaxSailors > fleet.Sailors {
		fleet.Sailors = newShip.MaxSailors
	}
	w.Log("bought a %s for %.0f gold (trade-in %.0f)", newShip.Name, q.Cost, q.TradeIn)
	return q, nil
}

// RecruitSailor hires a sailor at a tavern.
//
// The hiring cost is reduced by the tavern level of the port.
func (w *World) RecruitSailor(fleetID ecs.ID, portID string, templateID string) (int, error) {
	fleet := getFleetComponent(w.ecs, fleetID)
	port := w.cfg.Port(portID)
	tmpl := w.cfg.Characters[templateID]
	if fleet == nil || port == nil || tmpl == nil {
		return 0, fmt.Errorf("unknown recruit")
	}
	if port.TavernLevel <= 0 {
		return 0, fmt.Errorf("%s has no tavern to hire in", port.Name)
	}
	ship := w.cfg.Ship(fleet.ShipID)
	if ship == nil {
		return 0, fmt.Errorf("unknown ship")
	}
	if fleet.Sailors >= ship.MaxSailors {
		return 0, fmt.Errorf("the crew is already at full strength")
	}

	discount := 1 - 0.1*float64(port.TavernLevel)
	cost := tmpl.HiringCost * discount
	if w.treasury < cost {
		return 0, fmt.Errorf("not enough gold: %.0f needed", cost)
	}
	w.treasury -= cost
	fleet.Sailors++
	return fleet.Sailors, nil
}

// FireSailor dismisses a sailor.
func (w *World) FireSailor(fleetID ecs.ID, portID string) (int, error) {
	fleet := getFleetComponent(w.ecs, fleetID)
	if fleet == nil {
		return 0, fmt.Errorf("unknown fleet")
	}
	if fleet.Sailors <= 1 {
		return 0, fmt.Errorf("cannot dismiss the last sailor")
	}
	fleet.Sailors--
	return fleet.Sailors, nil
}

// BuyFood provisions the hold with food and water.
//
// The price per ton follows the local market price of the food and water
// commodities, plus the port's tax. The consumables share the hold
// capacity with the cargo.
func (w *World) BuyFood(fleetID ecs.ID, portID string, food, water float64) (float64, error) {
	fleet := getFleetComponent(w.ecs, fleetID)
	port := w.cfg.Port(portID)
	if fleet == nil || port == nil {
		return 0, fmt.Errorf("unknown port")
	}
	ship := w.cfg.Ship(fleet.ShipID)
	if ship == nil {
		return 0, fmt.Errorf("unknown ship")
	}

	used := fleet.Food + fleet.FreshWater
	room := ship.ConsumableCapacity - used
	if room < 0 {
		room = 0
	}
	total := food + water
	if total > room {
		scale := 0.0
		if total > 0 {
			scale = room / total
		}
		food *= scale
		water *= scale
	}

	foodPrice := w.localFoodPrice(portID)
	waterPrice := w.localWaterPrice(portID)
	cost := (food*foodPrice + water*waterPrice) * (1 + port.BaseTaxRate)
	if w.treasury < cost {
		return 0, fmt.Errorf("not enough gold: %.0f needed", cost)
	}
	w.treasury -= cost
	fleet.Food += food
	fleet.FreshWater += water
	return cost, nil
}

// localFoodPrice returns the price of one ton of food at a port.
//
// The food commodity is looked up by the well-known ID "food"; when it is
// not present in the table, a fallback price derived from the global
// parameters is used. This keeps the port interaction working with any
// data set.
func (w *World) localFoodPrice(portID string) float64 {
	if c := w.cfg.Commodity("food"); c != nil {
		return w.Price("food", portID)
	}
	return w.cfg.Params.StartMoney * 0.01
}

// localWaterPrice returns the price of one ton of fresh water at a port.
func (w *World) localWaterPrice(portID string) float64 {
	if c := w.cfg.Commodity("water"); c != nil {
		return w.Price("water", portID)
	}
	return w.cfg.Params.StartMoney * 0.005
}

// BuyMap buys a chart of a region from the tavern.
//
// The chart reveals the positions of the ports of the region. Its price
// scales with the number of ports it contains.
func (w *World) BuyMap(regionID string) (float64, error) {
	if regionID == "" {
		return 0, fmt.Errorf("unknown region")
	}
	cost := float64(len(w.cfg.PortByRegion[regionID])) * w.cfg.Params.StartMoney * 0.02
	if w.treasury < cost {
		return 0, fmt.Errorf("not enough gold: %.0f needed", cost)
	}
	w.treasury -= cost
	w.Log("bought a chart of %s for %.0f gold", regionID, cost)
	return cost, nil
}

// RestInInn rests the crew at an inn.
//
// Resting restores morale and removes fatigue; the cost depends on the
// inn level and the crew size.
func (w *World) RestInInn(fleetID ecs.ID, portID string) (float64, error) {
	fleet := getFleetComponent(w.ecs, fleetID)
	port := w.cfg.Port(portID)
	if fleet == nil || port == nil {
		return 0, fmt.Errorf("unknown port")
	}
	if port.InnLevel <= 0 {
		return 0, fmt.Errorf("%s has no inn", port.Name)
	}
	cost := float64(fleet.Sailors) * w.cfg.Params.StartMoney * 0.001 * float64(port.InnLevel)
	if w.treasury < cost {
		return 0, fmt.Errorf("not enough gold: %.0f needed", cost)
	}
	w.treasury -= cost

	fleet.Morale += 10 * float64(port.InnLevel)
	if fleet.Morale > 100 {
		fleet.Morale = 100
	}
	fleet.Fatigue -= 10 * float64(port.InnLevel)
	if fleet.Fatigue < 0 {
		fleet.Fatigue = 0
	}
	w.Log("rested the crew at %s for %.0f gold", port.Name, cost)
	return cost, nil
}

// DonateToChurch donates money to the church.
//
// The donation raises the fame of the adventure axis, which unlocks story
// events and titles.
func (w *World) DonateToChurch(fleetID ecs.ID, portID string, amount float64) (float64, error) {
	port := w.cfg.Port(portID)
	if port == nil {
		return 0, fmt.Errorf("unknown port")
	}
	if port.ChurchLevel <= 0 {
		return 0, fmt.Errorf("%s has no church", port.Name)
	}
	if amount <= 0 {
		return 0, fmt.Errorf("the donation must be positive")
	}
	if w.treasury < amount {
		return 0, fmt.Errorf("not enough gold")
	}
	w.treasury -= amount

	gain := amount / math.Max(1, w.cfg.Params.StartMoney) * 10 * float64(port.ChurchLevel)
	w.AddReputation(FactionPlayer, AxisAdventure, gain)
	w.Log("donated %.0f gold to the church of %s", amount, port.Name)
	return gain, nil
}

// VisitGovernor meets the governor of a port.
//
// The governor grants reputation on the trading axis and may hand out a
// quest or a title.
func (w *World) VisitGovernor(fleetID ecs.ID, portID string) (float64, error) {
	port := w.cfg.Port(portID)
	if port == nil {
		return 0, fmt.Errorf("unknown port")
	}
	if port.GovernorLevel <= 0 {
		return 0, fmt.Errorf("%s has no governor to visit", port.Name)
	}
	gain := float64(port.GovernorLevel) * w.cfg.Params.FamePropagationFactor
	w.AddReputation(FactionPlayer, AxisTrade, gain)
	w.Log("paid respect to the governor of %s", port.Name)
	return gain, nil
}
