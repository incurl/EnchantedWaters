package event

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// Scripting runs the Lua scripts that are attached to the events.
//
// # The sandbox
//
// The scripts run in a fresh Lua state that only exposes the small API
// documented below. They can describe what happens, but they cannot
// touch the file system, the network or the simulation: the state is
// thrown away after every script.
//
// The API:
//
//	log(text)          writes a line into the event log
//	damage(amount)     returns the damage to apply to the hull
//	resource(amount)   returns the change of the food and water stores
//	reputation(amount) returns the change of the reputation
//	cargo(fraction)    returns the fraction of the cargo to lose
//	random(max)        returns a random number in [0, max)
//
// A script returns a table with the keys "damage", "resource",
// "reputation" and "cargo"; every key is optional.
type Scripting struct {
	// dir is the directory that holds the scripts.
	dir string
	// cache holds the loaded scripts, keyed by their file name.
	cache map[string]*lua.FunctionProto
}

// NewScripting creates the scripting engine.
//
// When dir is empty, the engine is disabled and every script is skipped.
func NewScripting(dir string) *Scripting {
	return &Scripting{dir: dir, cache: make(map[string]*lua.FunctionProto, 16)}
}

// Enabled reports whether the scripting engine is active.
func (s *Scripting) Enabled() bool { return s.dir != "" }

// Run executes a script and returns the outcome it describes.
//
// A missing script is not an error: the event simply falls back to its
// table driven effects.
func (s *Scripting) Run(name string, context map[string]any) (*Outcome, error) {
	if !s.Enabled() || name == "" {
		return nil, nil
	}
	path := filepath.Join(s.dir, name)
	if _, err := os.Stat(path); err != nil {
		// The script is optional; the caller falls back to the table.
		return nil, nil
	}

	L := lua.NewState()
	defer L.Close()

	// --- the sandbox API ---
	var logLines []string
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		logLines = append(logLines, L.ToString(1))
		return 0
	}))
	L.SetGlobal("damage", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(L.ToNumber(1)))
		return 1
	}))
	L.SetGlobal("resource", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(L.ToNumber(1)))
		return 1
	}))
	L.SetGlobal("reputation", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(L.ToNumber(1)))
		return 1
	}))
	L.SetGlobal("cargo", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(L.ToNumber(1)))
		return 1
	}))
	// The random source of the script is the standard math.random, so
	// that a script can also use it directly.
	L.SetGlobal("random", L.NewFunction(func(L *lua.LState) int {
		max := L.ToNumber(1)
		if max <= 0 {
			L.Push(lua.LNumber(0))
			return 1
		}
		if err := L.CallByParam(lua.P{Fn: L.GetGlobal("math").(*lua.LTable).RawGetString("random"), NRet: 1, Protect: true},
			lua.LNumber(max)); err != nil {
			L.Push(lua.LNumber(0))
			return 1
		}
		return 1
	}))

	// The context of the event is exposed as a global table.
	ctx := L.NewTable()
	for k, v := range context {
		switch tv := v.(type) {
		case string:
			L.SetField(ctx, k, lua.LString(tv))
		case float64:
			L.SetField(ctx, k, lua.LNumber(tv))
		case int:
			L.SetField(ctx, k, lua.LNumber(tv))
		case bool:
			L.SetField(ctx, k, lua.LBool(tv))
		default:
			L.SetField(ctx, k, lua.LNil)
		}
	}
	L.SetGlobal("event", ctx)

	if err := L.DoFile(path); err != nil {
		return nil, fmt.Errorf("run script %s: %w", name, err)
	}

	// --- the result table ---
	outcome := &Outcome{ScriptRan: true}
	if len(logLines) > 0 {
		outcome.Message = strings.Join(logLines, "\n")
	}
	ret := L.Get(-1)
	if tbl, ok := ret.(*lua.LTable); ok {
		readNumber(tbl, "damage", func(v float64) { outcome.addEffect(Effect{Kind: "damage", Target: "fleet", Value: v}) })
		readNumber(tbl, "resource", func(v float64) { outcome.addEffect(Effect{Kind: "resource", Target: "fleet", Value: v}) })
		readNumber(tbl, "reputation", func(v float64) { outcome.addEffect(Effect{Kind: "reputation", Target: "crew", Value: v}) })
		readNumber(tbl, "cargo", func(v float64) { outcome.addEffect(Effect{Kind: "cargo", Target: "cargo", Value: v}) })
		if msg, ok := tbl.RawGetString("message").(lua.LString); ok && string(msg) != "" {
			outcome.Message = string(msg)
		}
	}
	return outcome, nil
}

// readNumber reads a numeric field of a Lua table.
func readNumber(tbl *lua.LTable, key string, fn func(float64)) {
	v := tbl.RawGetString(key)
	if v == lua.LNil {
		return
	}
	if n, ok := v.(lua.LNumber); ok {
		fn(float64(n))
	}
}

// addEffect appends an effect to the outcome.
func (o *Outcome) addEffect(e Effect) {
	o.Effects = append(o.Effects, e)
}
