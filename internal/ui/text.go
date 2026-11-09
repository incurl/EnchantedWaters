package ui

import (
	"fmt"
	"strings"

	"github.com/peter/enchantedwaters/internal/config"
)

// Text is the wording service of the user interface.
//
// It resolves the sentences of the interface through the strings that
// were imported from the original game, and falls back to the wording of
// this project when a line is absent.
//
// The lookup is done through a table of keys: every key names the line of
// the original message table that holds the sentence, and the fallback is
// the sentence of this project.
//
// The resolution is done once, when the interface is created, so that the
// draw loop never touches the maps.
type Text struct {
	// cfg is the configuration registry, which holds the imported data.
	cfg *config.Registry
	// lines holds the resolved sentences, indexed by the key.
	lines map[string]string
}

// textKey identifies one sentence of the interface.
type textKey int

// The sentences of the interface.
//
// The numbers are the indices of the lines in the original message table;
// the fallback is the wording of this project.
const (
	textGreet textKey = iota
	textWhatLookingFor
	textNoStock
	textLocalSpecialty
	textHowMuchPay
	textNoGoods
	textCannotAfford
	textGoodPrice
	textTooExpensive
	textPriceAccepted
	textNoRoom
	textTooManyGoods
	textNoGold
	textCrewFull
	textRecruitQuestion
	textRecruitCall
	textThanks
	textComeAgain
	textSeeYou
)

// textEntry describes one sentence of the interface.
type textEntry struct {
	// original is the index of the line in the original message table.
	original int
	// fallback is the wording of this project.
	fallback string
}

// textEntries holds every sentence of the interface.
var textEntries = map[textKey]textEntry{
	textGreet:           {0, "How may I help you?"},
	textWhatLookingFor:  {3, "What are you looking for today?"},
	textNoStock:         {4, "Sorry, we don't have any %s in stock."},
	textLocalSpecialty:  {5, "%s %s the local specialty."},
	textHowMuchPay:      {6, "Well then, how much would you pay?(0-%d)"},
	textNoGoods:         {21, "We don't have any goods to sell."},
	textCannotAfford:    {22, "We don't have enough gold."},
	textGoodPrice:       {23, "What a bargain!"},
	textTooExpensive:    {24, "It seems a little expensive."},
	textPriceAccepted:   {25, "That price is acceptable."},
	textNoRoom:          {26, "Commodore, we don't have any more space in the hold."},
	textTooManyGoods:    {27, "Commodore, we've got too many kinds of goods. Try selling something first."},
	textNoGold:          {28, "Commodore, we have no gold!"},
	textCrewFull:        {29, "We have enough men in our crew."},
	textRecruitQuestion: {32, "Shall we recruit some men for our crew?"},
	textRecruitCall:     {33, "Hey! Do any of you tough sailors want to join our crew?"},
	textThanks:          {36, "Thanks for treating me."},
	textComeAgain:       {13, "Come back again."},
	textSeeYou:          {14, "What? Is this all?! Thanks for nothing!"},
}

// NewText creates the wording service.
//
// The sentences of the original game are resolved once, at the creation
// of the interface.
func NewText(cfg *config.Registry) *Text {
	t := &Text{cfg: cfg, lines: make(map[string]string, len(textEntries))}
	for key, entry := range textEntries {
		line := cfg.OriginalString(cfg.OriginalMessages, entry.original)
		if line == "" {
			line = entry.fallback
		}
		t.lines[fmt.Sprintf("%d", int(key))] = line
	}
	return t
}

// Get returns the sentence of a key.
//
// The format arguments are substituted with the fmt verbs of the
// original sentence.
func (t *Text) Get(key textKey, args ...any) string {
	line, ok := t.lines[fmt.Sprintf("%d", int(key))]
	if !ok {
		return ""
	}
	if len(args) == 0 {
		return line
	}
	return fmt.Sprintf(line, args...)
}

// Available reports whether the sentence comes from the original game.
//
// It is used by the debug panel to show which wording is in use.
func (t *Text) Available(key textKey) bool {
	entry, ok := textEntries[key]
	if !ok {
		return false
	}
	return t.cfg.OriginalString(t.cfg.OriginalMessages, entry.original) != ""
}

// Colony returns the name and the description of a colony of the
// original game.
//
// The identifier is the index in the original colony table.
func (t *Text) Colony(id int) (string, string) {
	c, ok := t.cfg.OriginalColonies[id]
	if !ok {
		return "", ""
	}
	return c.Name, c.Description
}

// Name returns one of the names of the original name pool.
//
// The identifier is the index in the original name table.
func (t *Text) Name(id int) string {
	return t.cfg.OriginalString(t.cfg.OriginalNames, id)
}

// ItemDescription returns the description of an item of the original
// game.
//
// The identifier is the index in the original item table.
func (t *Text) ItemDescription(id int) string {
	item, ok := t.cfg.OriginalItems[id]
	if !ok {
		return ""
	}
	return item.Description
}

// Dialogues returns the dialogue lines of a scenario of the original
// game.
//
// The scenario number is the index of the scenario (0 to 6).
func (t *Text) Dialogues(scenario int) []string {
	if scenario < 0 || scenario >= len(t.cfg.OriginalDialogue) {
		return nil
	}
	table := t.cfg.OriginalDialogue[scenario]
	out := make([]string, 0, len(table))
	for id := 0; id < len(table); id++ {
		if line, ok := table[id]; ok {
			out = append(out, line.Text)
		}
	}
	return out
}

// CleanDialogue removes the speaker prefix of a dialogue line.
//
// The dialogues of the original game start with "|Speaker|\n", which
// separates the name of the speaker from the sentence.
func CleanDialogue(line string) (speaker, text string) {
	if !strings.HasPrefix(line, "|") {
		return "", line
	}
	// The closing bar of the speaker is searched from the second
	// character: a line that only holds a bar has no speaker.
	end := strings.Index(line[1:], "|")
	if end < 0 {
		return "", line
	}
	speaker = line[1 : 1+end]
	text = strings.TrimPrefix(line[1+end+1:], "\n")
	return speaker, text
}
