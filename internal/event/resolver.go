package event

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// Effect names understood by the resolver.
const (
	// EffectDamage reduces the hull points of the fleet.
	EffectDamage = "damage"
	// EffectResource changes the food and water stores.
	EffectResource = "resource"
	// EffectReputation changes the reputation of the fleet.
	EffectReputation = "reputation"
	// EffectCargo removes a fraction of the cargo.
	EffectCargo = "cargo"
	// EffectDiscover registers a discovery.
	EffectDiscover = "discover"
	// EffectMessage only produces a message.
	EffectMessage = "message"
)

// Resolver applies the effects of an event to a fleet.
//
// The resolver works on a plain data structure, so that it stays
// independent of the simulation package.
type Resolver struct {
	// rng is the random source used for the effect chances.
	rng *rand.Rand
}

// NewResolver creates an effect resolver.
func NewResolver(seed uint64) *Resolver {
	return &Resolver{rng: rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))}
}

// TargetState is the mutable state of the target of an event.
type TargetState struct {
	// HullPoints is the current durability.
	HullPoints float64
	// MaxHullPoints is the durability of a sound hull.
	MaxHullPoints float64
	// Food is the food stock.
	Food float64
	// FreshWater is the fresh water stock.
	FreshWater float64
	// Reputation is the reputation of the crew.
	Reputation float64
	// Cargo maps a commodity ID to the carried amount.
	Cargo map[string]float64
	// Discoveries are the discoveries that were made.
	Discoveries map[string]bool
}

// Apply applies a list of effects to a target.
//
// It returns the human readable messages of the effects that were
// applied.
func (r *Resolver) Apply(effects []Effect, target *TargetState) []string {
	messages := make([]string, 0, len(effects))
	for _, e := range effects {
		msg := r.applyOne(e, target)
		if msg != "" {
			messages = append(messages, msg)
		}
	}
	return messages
}

// applyOne applies a single effect.
func (r *Resolver) applyOne(e Effect, target *TargetState) string {
	switch e.Kind {
	case EffectDamage:
		amount := e.Value
		if amount < 0 {
			amount = -amount
		}
		// A negative value means a repair.
		if e.Value < 0 {
			target.HullPoints -= amount
			if target.HullPoints > target.MaxHullPoints {
				target.HullPoints = target.MaxHullPoints
			}
			return fmt.Sprintf("the hull is repaired by %.0f points", amount)
		}
		target.HullPoints -= amount
		if target.HullPoints < 0 {
			target.HullPoints = 0
		}
		return fmt.Sprintf("the hull takes %.0f points of damage", amount)

	case EffectResource:
		// The resource effect changes the food and the water in
		// proportion to their shortage.
		if e.Value >= 0 {
			target.Food += e.Value
			target.FreshWater += e.Value
			return fmt.Sprintf("%.0f tons of provisions are gained", e.Value)
		}
		loss := -e.Value
		// The loss is taken from the food first, then from the water.
		fromFood := minFloat(loss, target.Food)
		target.Food -= fromFood
		loss -= fromFood
		if loss > 0 {
			fromWater := minFloat(loss, target.FreshWater)
			target.FreshWater -= fromWater
			loss -= fromWater
		}
		return fmt.Sprintf("%.0f tons of provisions are lost", -e.Value)

	case EffectReputation:
		target.Reputation += e.Value
		return fmt.Sprintf("the reputation changes by %+.0f", e.Value)

	case EffectCargo:
		// The value is the fraction of the cargo that is lost.
		fraction := e.Value
		if fraction < 0 {
			fraction = -fraction
		}
		if fraction > 1 {
			fraction = 1
		}
		lost := 0.0
		for id, qty := range target.Cargo {
			loss := qty * fraction
			target.Cargo[id] = qty - loss
			lost += loss
		}
		return fmt.Sprintf("%.0f tons of cargo are lost", lost)

	case EffectDiscover:
		if target.Discoveries == nil {
			target.Discoveries = make(map[string]bool, 4)
		}
		if e.Key != "" {
			target.Discoveries[e.Key] = true
		}
		return fmt.Sprintf("a discovery is made: %s", e.Key)

	case EffectMessage:
		if e.Message != "" {
			return e.Message
		}
		if e.Key != "" {
			return e.Key
		}
		return ""
	}
	return ""
}

// Message joins the messages of the effects.
func Message(messages []string) string {
	return strings.Join(messages, "\n")
}

// minFloat returns the smaller of the two values.
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
