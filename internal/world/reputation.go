package world

import (
	"github.com/peter/enchantedwaters/internal/ecs"
)

// This file implements the three-axis reputation system of the original
// game: Adventure (exploration and combat), Trade (commerce) and Naval
// (fleet command).
//
// The fame of a faction decays slowly over time and is boosted by the
// deeds it performs. Titles are unlocked automatically when the fame on an
// axis crosses their threshold.

// Reputation axes.
const (
	// AxisAdventure is the exploration and combat axis.
	AxisAdventure = "adventure"
	// AxisTrade is the commerce axis.
	AxisTrade = "trade"
	// AxisNaval is the fleet command axis.
	AxisNaval = "naval"
)

// AddReputation increases the fame of a faction on one axis.
//
// The fame is clamped to [0, 100] per axis. Whenever a threshold is
// crossed, the corresponding title is unlocked.
func (w *World) AddReputation(factionID, axis string, amount float64) {
	entID, ok := w.factionEntities[factionID]
	if !ok {
		return
	}
	rep := getReputationComponent(w.ecs, entID)
	if rep == nil {
		return
	}
	switch axis {
	case AxisAdventure:
		rep.Adventure = clampFame(rep.Adventure + amount)
	case AxisTrade:
		rep.Trade = clampFame(rep.Trade + amount)
	case AxisNaval:
		rep.Naval = clampFame(rep.Naval + amount)
	default:
		return
	}
	w.checkTitles(factionID, rep)
}

// Reputation returns the fame of a faction on one axis.
func (w *World) Reputation(factionID, axis string) float64 {
	entID, ok := w.factionEntities[factionID]
	if !ok {
		return 0
	}
	rep := getReputationComponent(w.ecs, entID)
	if rep == nil {
		return 0
	}
	switch axis {
	case AxisAdventure:
		return rep.Adventure
	case AxisTrade:
		return rep.Trade
	case AxisNaval:
		return rep.Naval
	}
	return 0
}

// Titles returns the unlocked title IDs of a faction.
func (w *World) Titles(factionID string) []string {
	entID, ok := w.factionEntities[factionID]
	if !ok {
		return nil
	}
	rep := getReputationComponent(w.ecs, entID)
	if rep == nil {
		return nil
	}
	out := make([]string, 0, len(rep.Titles))
	for id := range rep.Titles {
		out = append(out, id)
	}
	sortStrings(out)
	return out
}

// HasTitle reports whether a faction has unlocked a title.
func (w *World) HasTitle(factionID, titleID string) bool {
	entID, ok := w.factionEntities[factionID]
	if !ok {
		return false
	}
	rep := getReputationComponent(w.ecs, entID)
	if rep == nil {
		return false
	}
	return rep.Titles[titleID]
}

// checkTitles unlocks the titles whose threshold has been reached.
func (w *World) checkTitles(factionID string, rep *ecs.ReputationComponent) {
	for _, titleID := range w.cfg.TitleOrder {
		title := w.cfg.Titles[titleID]
		if title == nil || rep.Titles[titleID] {
			continue
		}
		var value float64
		switch title.Axis {
		case AxisAdventure:
			value = rep.Adventure
		case AxisTrade:
			value = rep.Trade
		case AxisNaval:
			value = rep.Naval
		default:
			continue
		}
		if value >= title.Threshold {
			rep.Titles[titleID] = true
			w.Log("%s earned the title '%s'", factionID, title.Name)
			w.emit("title", map[string]any{"faction": factionID, "title": title.Name})
		}
	}
}

// TitleBonus returns the total bonus of the unlocked titles of a faction
// on one axis.
//
// The bonuses of all unlocked titles are summed; they apply to the
// economics of the faction (for example a trade bonus increases the
// selling price).
func (w *World) TitleBonus(factionID, axis string) float64 {
	entID, ok := w.factionEntities[factionID]
	if !ok {
		return 0
	}
	rep := getReputationComponent(w.ecs, entID)
	if rep == nil {
		return 0
	}
	bonus := 0.0
	for titleID := range rep.Titles {
		title := w.cfg.Titles[titleID]
		if title == nil {
			continue
		}
		switch axis {
		case AxisTrade:
			bonus += title.BonusTrade
		case AxisAdventure:
			bonus += title.BonusAdventure
		case AxisNaval:
			bonus += title.BonusNaval
		}
	}
	return bonus
}

// decayReputation applies the daily fame decay.
//
// The decay rate comes from the global parameters table.
func (w *World) decayReputation() {
	decay := w.cfg.Params.FameDecayPerDay
	if decay <= 0 {
		return
	}
	for _, entID := range w.factionEntities {
		rep := getReputationComponent(w.ecs, entID)
		if rep == nil {
			continue
		}
		rep.Adventure = clampFame(rep.Adventure - decay)
		rep.Trade = clampFame(rep.Trade - decay)
		rep.Naval = clampFame(rep.Naval - decay)
	}
}

// propagateFame spreads the fame of the strongest factions to their allies.
//
// The propagation models the "renown spreading through the world" mechanic:
// a faction gains fame when it acts together with a famous one.
//
//	gain = FamePropagationFactor * max(allyFame) * (1 - ownFame/300)
func (w *World) propagateFame() {
	factor := w.cfg.Params.FamePropagationFactor
	if factor <= 0 {
		return
	}
	// Compute the strongest ally fame per faction.
	strongest := make(map[string]float64, len(w.factionEntities))
	for factionID, entID := range w.factionEntities {
		rep := getReputationComponent(w.ecs, entID)
		if rep == nil {
			continue
		}
		maxAlly := 0.0
		for allyID, fame := range rep.AllyFame {
			if allyID == factionID {
				continue
			}
			if fame > maxAlly {
				maxAlly = fame
			}
		}
		strongest[factionID] = maxAlly
	}
	// Apply the propagation.
	for factionID, entID := range w.factionEntities {
		rep := getReputationComponent(w.ecs, entID)
		if rep == nil {
			continue
		}
		maxAlly := strongest[factionID]
		if maxAlly <= 0 {
			continue
		}
		own := rep.Adventure + rep.Trade + rep.Naval
		gain := factor * maxAlly * (1 - own/300)
		if gain <= 0 {
			continue
		}
		// The fame is spread over the three axes proportionally to the
		// current distribution.
		total := rep.Adventure + rep.Trade + rep.Naval
		if total <= 0 {
			rep.Trade += gain
		} else {
			rep.Adventure += gain * rep.Adventure / total
			rep.Trade += gain * rep.Trade / total
			rep.Naval += gain * rep.Naval / total
		}
	}
}

// SetAllyFame records the fame of an ally faction, which drives the fame
// propagation.
func (w *World) SetAllyFame(factionID, allyID string, fame float64) {
	entID, ok := w.factionEntities[factionID]
	if !ok {
		return
	}
	rep := getReputationComponent(w.ecs, entID)
	if rep == nil {
		return
	}
	rep.AllyFame[allyID] = fame
}

// clampFame clamps a fame value to [0, 100].
func clampFame(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
