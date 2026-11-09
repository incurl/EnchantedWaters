package ai

import (
	"sync"

	"github.com/peter/enchantedwaters/internal/ecs"
)

// Bridge connects the AI controller to the world simulation.
//
// The bridge lives in the ai package but operates on snapshots, so it does
// not depend on the world package (which would create an import cycle).
// The world calls into it with plain data.

// UpdateRequest describes one AI fleet that needs a decision.
type UpdateRequest struct {
	// Fleet is the snapshot of the fleet.
	Fleet FleetState
	// AI is a copy of the fleet's AI component. It is modified in place
	// and written back by the caller.
	AI ecs.AIComponent
}

// UpdateResult is the outcome of one AI decision.
type UpdateResult struct {
	// ID is the entity ID of the fleet.
	ID ecs.ID
	// AI is the updated AI component.
	AI ecs.AIComponent
	// Decision is the decision that was taken (nil when none).
	Decision *Decision
}

// UpdateBatch evaluates a batch of AI fleets in parallel.
//
// The work is split into chunks that are processed by independent
// goroutines. Every chunk owns its requests exclusively, so the
// evaluation runs without any lock.
//
// The number of workers is capped so that the goroutine overhead stays
// below the cost of the work itself.
func (c *Controller) UpdateBatch(view *MarketView, requests []UpdateRequest, workers int) []UpdateResult {
	if len(requests) == 0 {
		return nil
	}
	if workers <= 0 {
		workers = 1
	}
	if workers > len(requests) {
		workers = len(requests)
	}

	results := make([]UpdateResult, len(requests))
	if workers == 1 {
		for i := range requests {
			results[i] = c.updateOne(view, &requests[i])
		}
		return results
	}

	var wg sync.WaitGroup
	chunk := (len(requests) + workers - 1) / workers
	for i := 0; i < workers; i++ {
		start := i * chunk
		if start >= len(requests) {
			break
		}
		end := start + chunk
		if end > len(requests) {
			end = len(requests)
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			for j := lo; j < hi; j++ {
				results[j] = c.updateOne(view, &requests[j])
			}
		}(start, end)
	}
	wg.Wait()
	return results
}

// updateOne evaluates a single fleet.
func (c *Controller) updateOne(view *MarketView, req *UpdateRequest) UpdateResult {
	aiCopy := req.AI
	state, decision := c.UpdateFleet(view, &req.Fleet, &aiCopy)
	aiCopy.State = state
	return UpdateResult{ID: req.Fleet.ID, AI: aiCopy, Decision: decision}
}
