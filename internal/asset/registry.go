package asset

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-logr/logr"

	"github.com/peter/enchantedwaters/internal/config"
)

// Table file names inside the configuration directory.
const (
	fileCommodities = "commodities.csv"
	filePorts       = "ports.csv"
	filePortPrices  = "port_prices.csv"
	fileShips       = "ships.csv"
	fileCharacters  = "characters.csv"
	fileSeaZones    = "sea_zones.csv"
	fileWeather     = "weather.csv"
	fileEvents      = "events.csv"
	fileRumors      = "rumors.csv"
	fileTitles      = "titles.csv"
	fileAIProfiles  = "ai_profiles.csv"
	fileGlobals     = "globals.json"
)

// globalState holds the process-wide singleton.
//
// The static configuration is immutable after Load, therefore it can be
// shared by every goroutine without locking.
var globalState struct {
	once     sync.Once
	registry *config.Registry
	err      error
}

// Load reads every configuration table from dir and returns the indexed
// registry.
//
// The call is guarded by a sync.Once: subsequent calls return the very same
// registry pointer. This guarantees the "loaded once at start-up" invariant
// of the static configuration layer.
func Load(dir string, log logr.Logger) (*config.Registry, error) {
	globalState.once.Do(func() {
		globalState.registry, globalState.err = loadRegistry(dir, log)
	})
	return globalState.registry, globalState.err
}

// Registry returns the already loaded registry without triggering a load.
// It returns nil when Load has not been called yet.
func Registry() *config.Registry {
	return globalState.registry
}

// Reload discards the cached registry and loads the tables again.
//
// This exists purely for the development debug panel; the game itself loads
// the configuration exactly once.
func Reload(dir string, log logr.Logger) (*config.Registry, error) {
	globalState.once = sync.Once{}
	globalState.registry = nil
	globalState.err = nil
	return Load(dir, log)
}

// loadRegistry performs the actual load of all tables.
func loadRegistry(dir string, log logr.Logger) (*config.Registry, error) {
	reg := config.NewRegistry()

	// --- commodities -------------------------------------------------
	commodities, err := LoadCSVTable[config.Commodity](filepath.Join(dir, fileCommodities))
	if err != nil {
		return nil, err
	}
	for i := range commodities {
		reg.SetCommodity(&commodities[i])
	}
	log.Info("loaded commodities", "count", len(commodities))

	// --- ports -------------------------------------------------------
	ports, err := LoadCSVTable[config.Port](filepath.Join(dir, filePorts))
	if err != nil {
		return nil, err
	}
	for i := range ports {
		reg.SetPort(&ports[i])
	}
	log.Info("loaded ports", "count", len(ports))

	// --- per port commodity prices ----------------------------------
	prices, err := LoadCSVTable[config.PortPrice](filepath.Join(dir, filePortPrices))
	if err != nil {
		return nil, err
	}
	for i := range prices {
		reg.SetPortPrice(&prices[i])
	}
	log.Info("loaded port prices", "count", len(prices))

	// --- ships -------------------------------------------------------
	ships, err := LoadCSVTable[config.Ship](filepath.Join(dir, fileShips))
	if err != nil {
		return nil, err
	}
	for i := range ships {
		reg.SetShip(&ships[i])
	}
	log.Info("loaded ships", "count", len(ships))

	// --- characters --------------------------------------------------
	characters, err := LoadCSVTable[config.CharacterTemplate](filepath.Join(dir, fileCharacters))
	if err != nil {
		return nil, err
	}
	for i := range characters {
		reg.SetCharacter(&characters[i])
	}
	log.Info("loaded characters", "count", len(characters))

	// --- sea zones ---------------------------------------------------
	zones, err := LoadCSVTable[config.SeaZone](filepath.Join(dir, fileSeaZones))
	if err != nil {
		return nil, err
	}
	for i := range zones {
		reg.SetSeaZone(&zones[i])
	}
	log.Info("loaded sea zones", "count", len(zones))

	// --- weather -----------------------------------------------------
	weather, err := LoadCSVTable[config.WeatherTemplate](filepath.Join(dir, fileWeather))
	if err != nil {
		return nil, err
	}
	for i := range weather {
		reg.SetWeather(&weather[i])
	}
	log.Info("loaded weather templates", "count", len(weather))

	// --- events ------------------------------------------------------
	events, err := LoadCSVTable[config.EventTemplate](filepath.Join(dir, fileEvents))
	if err != nil {
		return nil, err
	}
	for i := range events {
		reg.SetEvent(&events[i])
	}
	log.Info("loaded events", "count", len(events))

	// --- rumors ------------------------------------------------------
	rumors, err := LoadCSVTable[config.RumorTemplate](filepath.Join(dir, fileRumors))
	if err != nil {
		return nil, err
	}
	for i := range rumors {
		reg.SetRumor(&rumors[i])
	}
	log.Info("loaded rumors", "count", len(rumors))

	// --- titles ------------------------------------------------------
	titles, err := LoadCSVTable[config.Title](filepath.Join(dir, fileTitles))
	if err != nil {
		return nil, err
	}
	for i := range titles {
		reg.SetTitle(&titles[i])
	}
	log.Info("loaded titles", "count", len(titles))

	// --- AI profiles -------------------------------------------------
	profiles, err := LoadCSVTable[config.AIProfile](filepath.Join(dir, fileAIProfiles))
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		reg.SetAIProfile(&profiles[i])
	}
	log.Info("loaded ai profiles", "count", len(profiles))

	// --- global parameters -------------------------------------------
	params, err := loadGlobals(filepath.Join(dir, fileGlobals))
	if err != nil {
		return nil, err
	}
	reg.Params = params

	// Build all reverse indexes and the deterministic orderings.
	reg.Finalize()

	// Structural validation: duplicate IDs, dangling references, bad ranges.
	if problems := reg.Validate(); len(problems) > 0 {
		for _, p := range problems {
			log.Error(errors.New("config validation problem"), "detail", p)
		}
		return nil, fmt.Errorf("config validation failed with %d problem(s)", len(problems))
	}

	log.Info("configuration loaded", "dir", dir,
		"commodities", len(reg.Commodities),
		"ports", len(reg.Ports),
		"ships", len(reg.Ships),
		"events", len(reg.Events))
	return reg, nil
}

// loadGlobals reads the global tuning parameters from a JSON file.
//
// The JSON file uses the same field names as the CSV tables, so that a
// single table can be exported in either format by the importer.
func loadGlobals(path string) (config.GlobalParams, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return config.GlobalParams{}, fmt.Errorf("read %s: %w", path, err)
	}
	return DecodeJSONObject[config.GlobalParams](data, path)
}
