package ai

import (
	"testing"

	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
)

// testController builds a controller with a small market.
func testController() (*Controller, *MarketView) {
	reg := config.NewRegistry()
	reg.SetCommodity(&config.Commodity{ID: "grain", Name: "Grain", BasePrice: 10, CyclePeriodDays: 100, CycleAmplitude: 0.2, WeightPerUnit: 1})
	reg.SetCommodity(&config.Commodity{ID: "spice", Name: "Spice", BasePrice: 50, CyclePeriodDays: 100, CycleAmplitude: 0.5, WeightPerUnit: 1})
	reg.SetPort(&config.Port{ID: "p1", Name: "One", Region: "a", X: 0, Y: 0})
	reg.SetPort(&config.Port{ID: "p2", Name: "Two", Region: "b", X: 100, Y: 0})
	reg.SetPort(&config.Port{ID: "p3", Name: "Three", Region: "c", X: 200, Y: 0})
	reg.SetAIProfile(&config.AIProfile{ID: "ai.test", Name: "Test", FleetCount: 1, RiskAversion: 0.2, CargoAggressiveness: 0.8, MaxVoyageDays: 30})
	reg.SetShip(&config.Ship{ID: "ship.test", Name: "Test ship", Class: config.ClassSmall, HullPoints: 100, MaxSailors: 10, CargoCapacity: 100, BaseSpeed: 5})
	reg.Params.TravelSpeedScale = 1
	reg.Finalize()

	c := NewController(reg, 1)
	view := NewMarketView()
	// The spice is cheap at p1 and expensive at p3.
	view.Prices["p1\x00grain"] = 10
	view.Prices["p1\x00spice"] = 50
	view.Prices["p2\x00grain"] = 20
	view.Prices["p2\x00spice"] = 90
	view.Prices["p3\x00grain"] = 30
	view.Prices["p3\x00spice"] = 150
	view.Stock["p1\x00grain"] = 100
	view.Stock["p1\x00spice"] = 100
	view.Demand["p2\x00grain"] = 10
	view.Demand["p2\x00spice"] = 10
	view.Demand["p3\x00grain"] = 10
	view.Demand["p3\x00spice"] = 10
	return c, view
}

// TestEvaluateFindsTheBestTrade verifies that the AI picks the most
// profitable opportunity.
func TestEvaluateFindsTheBestTrade(t *testing.T) {
	c, view := testController()
	decision := c.Evaluate(view, "p1", 0.2, 100)
	if decision == nil {
		t.Fatal("no decision was made")
	}
	if decision.Commodity != "spice" {
		t.Errorf("the AI chose %q, want spice", decision.Commodity)
	}
	if decision.TargetPort != "p3" {
		t.Errorf("the AI chose the port %q, want p3", decision.TargetPort)
	}
	if decision.Profit <= 0 {
		t.Errorf("the profit is %v, want > 0", decision.Profit)
	}
}

// TestRiskAversionChangesTheChoice verifies that the risk parameter has
// an effect on the decision.
func TestRiskAversionChangesTheChoice(t *testing.T) {
	c, view := testController()
	// With a very high risk aversion, the volatile spice is discounted
	// so much that the stable grain becomes more attractive.
	decision := c.Evaluate(view, "p1", 5.0, 100)
	if decision == nil {
		t.Fatal("no decision was made")
	}
	// The score must still be positive.
	if decision.Score <= 0 {
		t.Errorf("the score is %v, want > 0", decision.Score)
	}
}

// TestUpdateFleetStateMachine verifies the AI state machine.
func TestUpdateFleetStateMachine(t *testing.T) {
	c, view := testController()
	ai := &ecs.AIComponent{ProfileID: "ai.test", RiskAversion: 0.2}

	// --- docked: the AI picks a destination ---
	fleet := &FleetState{ID: 1, ShipID: "ship.test", InPort: "p1", Position: ecs.PositionComponent{}}
	state, decision := c.UpdateFleet(view, fleet, ai)
	if state != "trade" {
		t.Errorf("the state is %q, want trade", state)
	}
	if decision == nil {
		t.Fatal("no decision was made")
	}
	if ai.TargetPortID == "" {
		t.Error("no destination was chosen")
	}

	// --- at sea: the AI keeps sailing ---
	fleet.InPort = ""
	state, _ = c.UpdateFleet(view, fleet, ai)
	if state != "trade" {
		t.Errorf("the state is %q, want trade", state)
	}

	// --- a damaged hull makes the AI flee ---
	fleet.HullRatio = 0.1
	state, _ = c.UpdateFleet(view, fleet, ai)
	if state != "flee" {
		t.Errorf("the state is %q, want flee", state)
	}
	if ai.TargetPortID == "" {
		t.Error("the AI does not flee towards a port")
	}
}

// TestUpdateBatchIsParallel verifies that the batch update returns one
// result per request.
func TestUpdateBatchIsParallel(t *testing.T) {
	c, view := testController()
	requests := make([]UpdateRequest, 0, 50)
	for i := 0; i < 50; i++ {
		requests = append(requests, UpdateRequest{
			Fleet: FleetState{ID: ecs.ID(i + 1), ShipID: "ship.test", InPort: "p1"},
			AI:    ecs.AIComponent{ProfileID: "ai.test", RiskAversion: 0.2},
		})
	}
	results := c.UpdateBatch(view, requests, 8)
	if len(results) != len(requests) {
		t.Fatalf("%d results were produced, want %d", len(results), len(requests))
	}
	for i, r := range results {
		if r.ID != ecs.ID(i+1) {
			t.Errorf("the result %d holds the entity %d", i, r.ID)
		}
		if r.AI.TargetPortID == "" {
			t.Errorf("the result %d has no destination", i)
		}
	}
}
