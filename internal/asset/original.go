package asset

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"

	"github.com/peter/enchantedwaters/internal/config"
)

// loadOriginalTables loads the tables that were imported from the
// original game assets.
//
// The tables are optional: a data set without them is perfectly valid,
// the runtime simply falls back to its own wording. This keeps the
// importer output a *reference* rather than a requirement.
func loadOriginalTables(dir string, reg *config.Registry, log logr.Logger) error {
	// --- the interface strings ---
	if err := loadOriginalText(filepath.Join(dir, "original_messages.csv"), reg.OriginalMessages, log, "original messages"); err != nil {
		return err
	}
	if err := loadOriginalText(filepath.Join(dir, "original_messages2.csv"), reg.OriginalMessages2, log, "original messages 2"); err != nil {
		return err
	}
	if err := loadOriginalText(filepath.Join(dir, "original_menu.csv"), reg.OriginalMenuText, log, "original menu"); err != nil {
		return err
	}
	if err := loadOriginalText(filepath.Join(dir, "original_names.csv"), reg.OriginalNames, log, "original names"); err != nil {
		return err
	}

	// --- the dialogues of the seven scenarios ---
	for i := 0; i < 7; i++ {
		// The map of the scenario is created here, because the loader
		// runs before the finalisation of the registry.
		if reg.OriginalDialogue[i] == nil {
			reg.OriginalDialogue[i] = make(map[int]*config.OriginalText)
		}
		name := fmt.Sprintf("original_dialogue_%d.csv", i)
		if err := loadOriginalText(filepath.Join(dir, name), reg.OriginalDialogue[i], log, "original dialogue "+fmt.Sprint(i)); err != nil {
			return err
		}
	}

	// --- the menu tree ---
	menuItems, err := LoadCSVTable[config.OriginalMenuItem](filepath.Join(dir, "original_menu.csv"))
	if err != nil {
		return err
	}
	for i := range menuItems {
		reg.OriginalMenuItems[menuItems[i].ID] = &menuItems[i]
	}
	log.Info("loaded the original menu tree", "count", len(menuItems))

	// --- the terrain mask ---
	if err := loadTerrain(filepath.Join(dir, "world_terrain.csv"), reg, log); err != nil {
		return err
	}

	// --- the wind and the currents ---
	if err := loadWind(filepath.Join(dir, "wind_current.csv"), reg, log); err != nil {
		return err
	}

	// --- the colonies ---
	colonies, err := LoadCSVTable[config.OriginalColony](filepath.Join(dir, "original_colonies.csv"))
	if err != nil {
		return err
	}
	for i := range colonies {
		reg.OriginalColonies[colonies[i].ID] = &colonies[i]
	}
	log.Info("loaded original colonies", "count", len(colonies))

	// --- the item descriptions ---
	items, err := LoadCSVTable[config.OriginalItemDescription](filepath.Join(dir, "original_item_descriptions.csv"))
	if err != nil {
		return err
	}
	for i := range items {
		reg.OriginalItems[items[i].ID] = &items[i]
	}
	log.Info("loaded original item descriptions", "count", len(items))

	return nil
}

// loadOriginalText loads one imported text table.
func loadOriginalText(path string, dst map[int]*config.OriginalText, log logr.Logger, what string) error {
	rows, err := LoadCSVTable[config.OriginalText](path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// The table is optional.
			log.V(1).Info("the original data is not present", "table", what)
			return nil
		}
		return err
	}
	for i := range rows {
		dst[rows[i].ID] = &rows[i]
	}
	log.Info("loaded the original data", "table", what, "count", len(rows))
	return nil
}

// loadWind loads the wind and the currents of the sea areas of the world map.
//
// The table is optional: when it is absent the game falls back on the
// weather that it derives from the sea zones.
func loadWind(path string, reg *config.Registry, log logr.Logger) error {
	rows, err := LoadCSVTable[struct {
		Column           int `csv:"column"`
		Row              int `csv:"row"`
		SummerDirection  int `csv:"summer_direction"`
		SummerStrength   int `csv:"summer_strength"`
		WinterDirection  int `csv:"winter_direction"`
		WinterStrength   int `csv:"winter_strength"`
		CurrentDirection int `csv:"current_direction"`
		CurrentStrength  int `csv:"current_strength"`
	}](path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.V(1).Info("the wind of the original game is not present")
			return nil
		}
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	wind := &config.WorldWind{
		SummerDirection:  make([]uint8, config.WindSeaAreas),
		SummerStrength:   make([]uint8, config.WindSeaAreas),
		WinterDirection:  make([]uint8, config.WindSeaAreas),
		WinterStrength:   make([]uint8, config.WindSeaAreas),
		CurrentDirection: make([]uint8, config.WindSeaAreas),
		CurrentStrength:  make([]uint8, config.WindSeaAreas),
	}
	for _, r := range rows {
		if r.Column < 0 || r.Row < 0 || r.Column >= config.WindColumns || r.Row >= config.WindRows {
			continue
		}
		i := r.Row*config.WindColumns + r.Column
		wind.SummerDirection[i] = uint8(r.SummerDirection)
		wind.SummerStrength[i] = uint8(r.SummerStrength)
		wind.WinterDirection[i] = uint8(r.WinterDirection)
		wind.WinterStrength[i] = uint8(r.WinterStrength)
		wind.CurrentDirection[i] = uint8(r.CurrentDirection)
		wind.CurrentStrength[i] = uint8(r.CurrentStrength)
	}
	reg.Wind = wind
	log.Info("loaded the wind of the original game", "sea areas", config.WindSeaAreas)
	return nil
}

// terrainCellSize is the size of one cell of the terrain mask in map
// pixels. It matches the derivation of the mask.
const terrainCellSize = 20

// loadTerrain loads the land and sea mask of the world map.
//
// The mask is optional: when it is absent the renderer draws the land as
// a disc around every port, which is the fallback that has always worked.
func loadTerrain(path string, reg *config.Registry, log logr.Logger) error {
	rows, err := LoadCSVTable[struct {
		Row     int    `csv:"row"`
		Terrain string `csv:"terrain,opt"`
	}](path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.V(1).Info("the terrain mask is not present")
			return nil
		}
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	terrain := &config.WorldTerrain{CellSize: terrainCellSize, Rows: make([]string, len(rows))}
	for _, r := range rows {
		if r.Row < 0 || r.Row >= len(rows) {
			continue
		}
		terrain.Rows[r.Row] = r.Terrain
	}
	reg.Terrain = terrain
	log.Info("loaded the terrain mask", "rows", len(terrain.Rows), "cell pixels", terrain.CellSize)
	return nil
}
