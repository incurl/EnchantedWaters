package asset

import (
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
	if err := loadOriginalText(filepath.Join(dir, "original_menu.csv"), reg.OriginalMenu, log, "original menu"); err != nil {
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
		if os.IsNotExist(err) {
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
