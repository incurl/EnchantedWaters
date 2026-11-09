package event

import (
	"os"
	"testing"

	"github.com/peter/enchantedwaters/internal/config"
)

// testRegistry builds a small configuration for the tests.
func testRegistry() *config.Registry {
	reg := config.NewRegistry()
	reg.SetCommodity(&config.Commodity{ID: "grain", Name: "Grain", BasePrice: 10, CyclePeriodDays: 100, CycleAmplitude: 0.2})
	reg.SetPort(&config.Port{ID: "p1", Name: "One", Region: "a"})
	reg.SetPort(&config.Port{ID: "p2", Name: "Two", Region: "b"})
	reg.SetEvent(&config.EventTemplate{ID: "e.common", Name: "Common", Category: "sea", Weight: 10, Repeatable: true})
	reg.SetEvent(&config.EventTemplate{ID: "e.late", Name: "Late", Category: "sea", Weight: 10, MinDay: 50})
	reg.SetEvent(&config.EventTemplate{ID: "e.once", Name: "Once", Category: "story", Weight: 10, Repeatable: false})
	reg.Finalize()
	return reg
}

// TestParseEffects verifies the parsing of the effect column.
func TestParseEffects(t *testing.T) {
	effects, err := ParseEffects([]string{"damage:fleet:8", "resource:fleet:-2", "cargo:cargo:0.2", "discover:fleet:1:island"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(effects) != 4 {
		t.Fatalf("parsed %d effects, want 4", len(effects))
	}
	if effects[0].Kind != "damage" || effects[0].Target != "fleet" || effects[0].Value != 8 {
		t.Errorf("the first effect is %+v", effects[0])
	}
	if effects[1].Value != -2 {
		t.Errorf("the second effect holds %v, want -2", effects[1].Value)
	}
	if effects[3].Key != "island" {
		t.Errorf("the fourth effect holds the key %q, want \"island\"", effects[3].Key)
	}
}

// TestParseEffectsRejectsGarbage verifies that a malformed effect is
// reported instead of being silently ignored.
func TestParseEffectsRejectsGarbage(t *testing.T) {
	if _, err := ParseEffects([]string{"garbage"}); err == nil {
		t.Error("a malformed effect was accepted")
	}
	if _, err := ParseEffects([]string{"damage:fleet:notanumber"}); err == nil {
		t.Error("a malformed number was accepted")
	}
}

// TestSelectorRespectsTheDayRange verifies that the events are only drawn
// when they are eligible.
func TestSelectorRespectsTheDayRange(t *testing.T) {
	reg := testRegistry()
	sel := NewSelector(reg, 42)

	// On the first day, the late event is not eligible yet.
	seenFirst := make(map[string]bool)
	for i := 0; i < 50; i++ {
		e := sel.Select(1, "a", "fleet")
		if e == nil {
			t.Fatal("no event was drawn")
		}
		seenFirst[e.ID] = true
	}
	if seenFirst["e.late"] {
		t.Error("the late event was drawn on day 1")
	}
	// On the last day, the late event becomes eligible.
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		e := sel.Select(60, "a", "fleet")
		if e != nil {
			seen[e.ID] = true
		}
	}
	if !seen["e.late"] {
		t.Error("the late event was never drawn on day 60")
	}
}

// TestSelectorRespectsTheRepeatableFlag verifies that a non repeatable
// event fires only once.
func TestSelectorRespectsTheRepeatableFlag(t *testing.T) {
	reg := config.NewRegistry()
	reg.SetEvent(&config.EventTemplate{ID: "e.once", Name: "Once", Category: "story", Weight: 1})
	reg.Finalize()
	sel := NewSelector(reg, 7)

	first := sel.Select(1, "a", "fleet")
	if first == nil || first.ID != "e.once" {
		t.Fatalf("the first draw returned %v", first)
	}
	second := sel.Select(1, "a", "fleet")
	if second != nil {
		t.Errorf("the event %q was drawn twice", second.ID)
	}
}

// TestResolverAppliesTheEffects verifies the mechanical effects.
func TestResolverAppliesTheEffects(t *testing.T) {
	r := NewResolver(1)
	target := &TargetState{
		HullPoints:    100,
		MaxHullPoints: 100,
		Food:          10,
		FreshWater:    10,
		Cargo:         map[string]float64{"grain": 50},
		Discoveries:   map[string]bool{},
	}
	messages := r.Apply([]Effect{
		{Kind: EffectDamage, Target: "fleet", Value: 30},
		{Kind: EffectResource, Target: "fleet", Value: -4},
		{Kind: EffectCargo, Target: "cargo", Value: 0.5},
		{Kind: EffectDiscover, Target: "fleet", Value: 1, Key: "island"},
	}, target)

	if target.HullPoints != 70 {
		t.Errorf("the hull holds %v, want 70", target.HullPoints)
	}
	if target.Food != 6 {
		t.Errorf("the food holds %v, want 6", target.Food)
	}
	if target.Cargo["grain"] != 25 {
		t.Errorf("the cargo holds %v, want 25", target.Cargo["grain"])
	}
	if !target.Discoveries["island"] {
		t.Error("the discovery was not registered")
	}
	if len(messages) != 4 {
		t.Errorf("%d messages were produced, want 4", len(messages))
	}
}

// TestScriptingRunsAScript verifies the Lua scripting.
func TestScriptingRunsAScript(t *testing.T) {
	dir := t.TempDir()
	script := "-- a test script\n" +
		"log('something happens')\n" +
		"return { damage = 5, message = 'the script wins' }\n"
	writeFile(t, dir+"/test.lua", script)

	s := NewScripting(dir)
	if !s.Enabled() {
		t.Fatal("the scripting engine is not enabled")
	}
	outcome, err := s.Run("test.lua", map[string]any{"name": "Test"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if outcome == nil || !outcome.ScriptRan {
		t.Fatal("the script did not run")
	}
	if len(outcome.Effects) != 1 {
		t.Fatalf("the script produced %d effects, want 1", len(outcome.Effects))
	}
	if outcome.Effects[0].Value != 5 {
		t.Errorf("the effect holds %v, want 5", outcome.Effects[0].Value)
	}
	if outcome.Message != "the script wins" {
		t.Errorf("the message is %q, want \"the script wins\"", outcome.Message)
	}
}

// TestScriptingIgnoresAMissingScript verifies that a missing script is
// not an error.
func TestScriptingIgnoresAMissingScript(t *testing.T) {
	s := NewScripting(t.TempDir())
	outcome, err := s.Run("missing.lua", nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if outcome != nil {
		t.Error("a missing script produced an outcome")
	}
}

// writeFile writes a file and fails the test when it cannot.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
