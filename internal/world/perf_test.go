package world

import (
	"testing"
	"time"

	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
)

// TestManyAIFleets verifies that the simulation stays responsive with a
// large number of AI fleets.
//
// The requirement of the project is a stable frame rate with hundreds of
// parallel AI fleets; the test measures the cost of a hundred ticks with
// a fleet count that is far above the normal game.
func TestManyAIFleets(t *testing.T) {
	reg := testRegistry(t)

	// --- a hundred AI fleets ---
	profile := &config.AIProfile{
		ID:                  "ai.test",
		Name:                "Test",
		FleetCount:          100,
		RiskAversion:        0.3,
		CargoAggressiveness: 0.8,
		MaxVoyageDays:       30,
	}
	reg.SetAIProfile(profile)
	reg.Finalize()

	w := NewWorld(reg, 99)
	if err := w.StartSession("ports.lisbon"); err != nil {
		t.Fatalf("start session: %v", err)
	}

	// The fleet count of the test profile is applied by the session.
	count := 0
	aiStore := ecs.ComponentArrayOf[ecs.AIComponent](w.ecs)
	aiStore.Each(func(id ecs.ID, ai *ecs.AIComponent) { count++ })
	if count < 100 {
		t.Fatalf("%d AI fleets were created, want at least 100", count)
	}

	start := time.Now()
	const ticks = 100
	for i := 0; i < ticks; i++ {
		w.Tick()
	}
	elapsed := time.Since(start)
	perTick := elapsed / ticks

	t.Logf("%d AI fleets, %d ticks in %v (%.2f ms per tick)",
		count, ticks, elapsed, float64(perTick.Microseconds())/1000)

	// A tick must stay well below the frame budget: the simulation runs
	// at most a few times per second, and the rendering has to stay at
	// 60 FPS in between.
	if perTick > 250*time.Millisecond {
		t.Errorf("a tick takes %v, want less than 250ms", perTick)
	}
}
