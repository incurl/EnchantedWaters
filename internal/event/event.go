// Package event implements the event system of the game.
//
// # Model
//
// The events are entirely table driven: the event templates live in the
// static configuration (assets/config/events.csv) and the *probabilities*
// are part of that table. There is not a single hard-coded chance in the
// Go code.
//
// The system has three parts:
//
//   - the Selector, which draws an event from the table according to the
//     weights, the day range and the region,
//   - the Resolver, which applies the effects of the drawn event,
//   - the Scripting engine, which lets a Lua script take over an event.
//
// # The scripting engine
//
// The Lua engine is optional: when no script is attached to an event, the
// effects of the table are applied directly. The bindings exposed to the
// scripts are deliberately small and side-effect free, so that a script
// can only describe what happens, never change the simulation itself.
package event

import (
	"fmt"
	"math/rand/v2"
	"sort"

	"github.com/peter/enchantedwaters/internal/config"
)

// Effect is one resolved mechanical effect of an event.
type Effect struct {
	// Kind is the effect verb.
	Kind string
	// Target selects what the effect applies to.
	Target string
	// Value is the numeric magnitude.
	Value float64
	// Key is the optional payload key.
	Key string
	// Message is the optional human readable text.
	Message string
}

// Outcome is the result of an event.
type Outcome struct {
	// EventID is the ID of the template that was drawn.
	EventID string
	// Name is the display name of the event.
	Name string
	// Message is the message that is shown to the player.
	Message string
	// Effects holds the effects that were applied.
	Effects []Effect
	// ScriptRan reports whether a Lua script took over the event.
	ScriptRan bool
}

// Selector draws events from the event table.
type Selector struct {
	// cfg is the immutable configuration registry.
	cfg *config.Registry
	// rng is the random source.
	rng *rand.Rand
	// eventIDs is the deterministic list of the event IDs.
	eventIDs []string
	// fired maps an event ID to the number of times it was fired, used
	// for the repeatable events.
	fired map[string]int
}

// NewSelector creates an event selector.
func NewSelector(cfg *config.Registry, seed uint64) *Selector {
	s := &Selector{
		cfg:   cfg,
		rng:   rand.New(rand.NewPCG(seed, seed^0x2545F4914F6CDD1D)),
		fired: make(map[string]int, 64),
	}
	s.eventIDs = make([]string, 0, len(cfg.Events))
	for id := range cfg.Events {
		s.eventIDs = append(s.eventIDs, id)
	}
	sort.Strings(s.eventIDs)
	return s
}

// Select draws an event that is eligible for the given context.
//
// The context holds the day, the region and the fleet that is at sea.
// Only the events whose day range and region range match are eligible.
// The draw is proportional to the weights of the eligible events.
func (s *Selector) Select(day int, region string, fleetID string) *config.EventTemplate {
	type candidate struct {
		id     string
		weight float64
	}
	candidates := make([]candidate, 0, len(s.eventIDs))
	total := 0.0
	for _, id := range s.eventIDs {
		e := s.cfg.Events[id]
		if e == nil || e.Weight <= 0 {
			continue
		}
		// The day range.
		if e.MinDay > 0 && day < e.MinDay {
			continue
		}
		if e.MaxDay > 0 && day > e.MaxDay {
			continue
		}
		// The region range.
		if e.MinRegion != "" && region < e.MinRegion {
			continue
		}
		if e.MaxRegion != "" && region > e.MaxRegion {
			continue
		}
		// A non repeatable event fires only once.
		if !e.Repeatable && s.fired[id] > 0 {
			continue
		}
		candidates = append(candidates, candidate{id: id, weight: e.Weight})
		total += e.Weight
	}
	if total <= 0 {
		return nil
	}

	pick := s.rng.Float64() * total
	acc := 0.0
	for _, c := range candidates {
		acc += c.weight
		if pick <= acc {
			s.fired[c.id]++
			return s.cfg.Events[c.id]
		}
	}
	// The floating point accumulation may leave a small remainder; the
	// last candidate is used in that case.
	last := s.cfg.Events[candidates[len(candidates)-1].id]
	s.fired[last.ID]++
	return last
}

// ParseEffects parses the effect column of an event template.
//
// The format of an effect is:
//
//	kind:target:value[:key]
//
// The parts may be omitted from the right: an effect without a target
// applies to the fleet, and an effect without a value is zero.
func ParseEffects(raw []string) ([]Effect, error) {
	out := make([]Effect, 0, len(raw))
	for _, entry := range raw {
		if entry == "" {
			continue
		}
		parts := splitN(entry, ':', 4)
		if len(parts) < 2 {
			return nil, fmt.Errorf("malformed effect %q", entry)
		}
		e := Effect{Kind: parts[0], Target: "fleet"}
		if len(parts) >= 2 && parts[1] != "" {
			e.Target = parts[1]
		}
		if len(parts) >= 3 && parts[2] != "" {
			v, err := parseFloat(parts[2])
			if err != nil {
				return nil, fmt.Errorf("malformed effect %q: %w", entry, err)
			}
			e.Value = v
		}
		if len(parts) >= 4 {
			e.Key = parts[3]
		}
		out = append(out, e)
	}
	return out, nil
}

// splitN splits a string into at most n parts.
func splitN(s string, sep byte, n int) []string {
	out := make([]string, 0, n)
	start := 0
	for i := 0; i < len(s) && len(out) < n-1; i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// parseFloat parses a decimal number.
func parseFloat(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%g", &v)
	return v, err
}
