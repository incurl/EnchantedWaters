package world

import (
	"fmt"
	"math"

	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
)

// This file implements the port interaction system: the exchange, the
// shipyard, the tavern, the governor's mansion, the church and the inn.
//
// All the numbers used here come from the configuration tables; the
// functions only implement the *rules* of the original game.

// Quote describes the result of a trade operation.
type Quote struct {
	// CommodityID is the traded commodity.
	CommodityID string
	// Quantity is the number of units.
	Quantity float64
	// UnitPrice is the price per unit at the time of the quote.
	UnitPrice float64
	// Total is the total price before tax.
	Total float64
	// Tax is the tax charged by the port.
	Tax float64
	// GrandTotal is the amount actually transferred (Total + Tax for a
	// purchase, Total - Tax for a sale).
	GrandTotal float64
	// IsBuy reports whether the quote is a purchase (true) or a sale.
	IsBuy bool
	// Affordable reports whether the player can afford the operation.
	Affordable bool
	// Reason explains why the operation is not possible.
	Reason string
}

// BuyPrice returns the price at which the port buys a commodity from the
// player.
func (w *World) BuyPrice(commodityID, portID string) float64 {
	return w.Price(commodityID, portID)
}

// SellPrice returns the price at which the port sells a commodity to the
// player.
func (w *World) SellPrice(commodityID, portID string) float64 {
	return w.Price(commodityID, portID)
}

// QuoteBuy builds a quote for buying a commodity from the port.
//
// The maximum quantity is limited by the port's stock and by the player's
// remaining cargo capacity (weight and volume).
func (w *World) QuoteBuy(fleetID ecs.ID, commodityID, portID string, quantity float64) Quote {
	q := Quote{CommodityID: commodityID, Quantity: quantity, IsBuy: true}
	c := w.cfg.Commodity(commodityID)
	if c == nil {
		q.Reason = "unknown commodity"
		return q
	}
	port := w.cfg.Port(portID)
	if port == nil {
		q.Reason = "unknown port"
		return q
	}

	// --- stock limit ---
	stock := w.Stock(commodityID, portID)
	if quantity > stock {
		q.Quantity = stock
		q.Reason = fmt.Sprintf("only %.0f %s available", stock, c.Name)
	}
	if q.Quantity <= 0 {
		if q.Reason == "" {
			q.Reason = "no stock available"
		}
		return q
	}

	// --- cargo capacity limit ---
	if limit := w.cargoLimit(fleetID, c); q.Quantity > limit {
		q.Quantity = limit
		q.Reason = fmt.Sprintf("cargo hold can take only %.0f %s", limit, c.Name)
	}
	if q.Quantity <= 0 {
		if q.Reason == "" {
			q.Reason = "cargo hold is full"
		}
		return q
	}

	q.UnitPrice = w.SellPrice(commodityID, portID)
	q.Total = q.Quantity * q.UnitPrice
	q.Tax = q.Total * port.BaseTaxRate
	q.GrandTotal = q.Total + q.Tax
	q.Affordable = w.treasury >= q.GrandTotal
	if !q.Affordable {
		q.Reason = fmt.Sprintf("need %.0f gold, have %.0f", q.GrandTotal, w.treasury)
	}
	return q
}

// QuoteSell builds a quote for selling a commodity to the port.
//
// The quantity is limited by what the fleet actually carries and by the
// port's demand: a port cannot absorb unlimited amounts.
func (w *World) QuoteSell(fleetID ecs.ID, commodityID, portID string, quantity float64) Quote {
	q := Quote{CommodityID: commodityID, Quantity: quantity, IsBuy: false}
	c := w.cfg.Commodity(commodityID)
	if c == nil {
		q.Reason = "unknown commodity"
		return q
	}
	port := w.cfg.Port(portID)
	if port == nil {
		q.Reason = "unknown port"
		return q
	}

	cargo := getCargoComponent(w.ecs, fleetID)
	held := 0.0
	if cargo != nil {
		held = cargo.Manifest[commodityID]
	}
	if quantity > held {
		q.Quantity = held
		q.Reason = fmt.Sprintf("only %.0f %s in the hold", held, c.Name)
	}
	if q.Quantity <= 0 {
		if q.Reason == "" {
			q.Reason = "nothing to sell"
		}
		return q
	}

	// A port's absorption capacity grows with its demand.
	maxAbsorb := w.cfg.PortDemand(portID, commodityID) * 10
	if maxAbsorb > 0 && q.Quantity > maxAbsorb {
		q.Quantity = maxAbsorb
		q.Reason = fmt.Sprintf("the market absorbs only %.0f %s today", maxAbsorb, c.Name)
	}

	q.UnitPrice = w.BuyPrice(commodityID, portID)
	q.Total = q.Quantity * q.UnitPrice
	q.Tax = q.Total * port.BaseTaxRate
	// The port's share of the sale is the tax.
	q.GrandTotal = q.Total - q.Tax
	q.Affordable = true
	return q
}

// ExecuteBuy performs a purchase at a port.
//
// It moves the money, the goods and the port's stock, and returns the
// executed quote.
func (w *World) ExecuteBuy(fleetID ecs.ID, commodityID, portID string, quantity float64) (Quote, error) {
	q := w.QuoteBuy(fleetID, commodityID, portID, quantity)
	if q.Quantity <= 0 || !q.Affordable {
		return q, fmt.Errorf("%s", q.Reason)
	}
	cargo := getCargoComponent(w.ecs, fleetID)
	if cargo == nil {
		return q, fmt.Errorf("fleet has no cargo hold")
	}

	w.treasury -= q.GrandTotal
	cargo.Manifest[commodityID] += q.Quantity
	cargo.Gold = w.treasury
	w.updateCargoUsage(fleetID)

	// The port's stock decreases, which pushes the local price up.
	w.adjustStock(commodityID, portID, -q.Quantity)
	w.Log("bought %.0f %s at %s for %.0f gold", q.Quantity, w.cfg.Commodities[commodityID].Name, w.cfg.Ports[portID].Name, q.GrandTotal)
	return q, nil
}

// ExecuteSell performs a sale at a port.
func (w *World) ExecuteSell(fleetID ecs.ID, commodityID, portID string, quantity float64) (Quote, error) {
	q := w.QuoteSell(fleetID, commodityID, portID, quantity)
	if q.Quantity <= 0 {
		return q, fmt.Errorf("%s", q.Reason)
	}
	cargo := getCargoComponent(w.ecs, fleetID)
	if cargo == nil {
		return q, fmt.Errorf("fleet has no cargo hold")
	}

	w.treasury += q.GrandTotal
	cargo.Manifest[commodityID] -= q.Quantity
	if cargo.Manifest[commodityID] <= 0 {
		delete(cargo.Manifest, commodityID)
	}
	cargo.Gold = w.treasury
	w.updateCargoUsage(fleetID)

	// The port's stock increases, which pushes the local price down.
	w.adjustStock(commodityID, portID, q.Quantity)
	w.Log("sold %.0f %s at %s for %.0f gold", q.Quantity, w.cfg.Commodities[commodityID].Name, w.cfg.Ports[portID].Name, q.GrandTotal)
	return q, nil
}

// adjustStock adds (or removes) stock at a port and updates the cached
// price immediately, so that the player sees the effect of a trade right
// away.
func (w *World) adjustStock(commodityID, portID string, delta float64) {
	pd := w.portState(portID)
	if pd == nil {
		return
	}
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.stock[commodityID] += delta
	if pd.stock[commodityID] < 0 {
		pd.stock[commodityID] = 0
	}
	c := w.cfg.Commodities[commodityID]
	p := w.cfg.Port(portID)
	if c != nil && p != nil {
		pd.prices[commodityID] = w.commodityPriceLocked(c, pd, p)
	}
}

// cargoLimit returns how many units of a commodity the hold can still take.
//
// The limit is the minimum of the weight limit and the volume limit:
//
//	weightLimit  = (CargoCapacity - UsedWeight)  / WeightPerUnit
//	volumeLimit  = (CargoCapacity - UsedVolume)  / VolumePerUnit
func (w *World) cargoLimit(fleetID ecs.ID, c *config.Commodity) float64 {
	fleet := getFleetComponent(w.ecs, fleetID)
	cargo := getCargoComponent(w.ecs, fleetID)
	if fleet == nil || cargo == nil {
		return 0
	}
	ship := w.cfg.Ship(fleet.ShipID)
	if ship == nil {
		return 0
	}
	weightRoom := ship.CargoCapacity - cargo.UsedWeight
	volumeRoom := ship.CargoCapacity - cargo.UsedVolume
	limit := math.Inf(1)
	if c.WeightPerUnit > 0 {
		limit = math.Min(limit, weightRoom/c.WeightPerUnit)
	}
	if c.VolumePerUnit > 0 {
		limit = math.Min(limit, volumeRoom/c.VolumePerUnit)
	}
	if limit < 0 {
		return 0
	}
	return limit
}

// updateCargoUsage recomputes the cached weight and volume of a manifest.
func (w *World) updateCargoUsage(fleetID ecs.ID) {
	cargo := getCargoComponent(w.ecs, fleetID)
	if cargo == nil {
		return
	}
	weight := 0.0
	volume := 0.0
	for cid, qty := range cargo.Manifest {
		c := w.cfg.Commodities[cid]
		if c == nil {
			continue
		}
		weight += qty * c.WeightPerUnit
		volume += qty * c.VolumePerUnit
	}
	cargo.UsedWeight = weight
	cargo.UsedVolume = volume
}
