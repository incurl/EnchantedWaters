package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"

	"github.com/peter/enchantedwaters/internal/asset"
)

// testText creates the wording service over the shipped data.
func testText(t *testing.T) *Text {
	t.Helper()
	dir := filepath.Join("..", "..", "assets", "config")
	reg, err := asset.Load(dir, logr.Discard())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return NewText(reg)
}

// TestOriginalWordingIsUsed verifies that the sentences of the interface
// come from the imported data of the original game.
func TestOriginalWordingIsUsed(t *testing.T) {
	text := testText(t)

	// The greeting of the merchant must be the original one.
	greet := text.Get(textGreet)
	if greet != "How may I help you?" {
		t.Errorf("the greeting is %q, want the original wording", greet)
	}
	if !text.Available(textGreet) {
		t.Error("the greeting is reported as missing")
	}

	// The sentence with a format verb must keep its verb.
	noStock := text.Get(textNoStock, "silk")
	if !strings.Contains(noStock, "silk") {
		t.Errorf("the sentence %q does not hold the argument", noStock)
	}
	if !strings.Contains(noStock, "stock") {
		t.Errorf("the sentence %q is not the original one", noStock)
	}
}

// TestOriginalColonies verifies the imported colony table.
func TestOriginalColonies(t *testing.T) {
	text := testText(t)

	name, description := text.Colony(0)
	if name != "Stonehenge" {
		t.Errorf("the first colony is %q, want Stonehenge", name)
	}
	if !strings.Contains(description, "Prehistoric monument") {
		t.Errorf("the description of the first colony is %q", description)
	}

	// The last records of the table are padding.
	if name, _ := text.Colony(99); name != "" {
		t.Errorf("the last colony is %q, want an empty name", name)
	}
}

// TestOriginalNames verifies the imported name pool.
func TestOriginalNames(t *testing.T) {
	text := testText(t)
	if got := text.Name(0); got != "Affonzo" {
		t.Errorf("the first name is %q, want Affonzo", got)
	}
	if got := text.Name(1000); got != "" {
		t.Errorf("an unknown name returned %q", got)
	}
}

// TestOriginalItemDescriptions verifies the imported item table.
func TestOriginalItemDescriptions(t *testing.T) {
	text := testText(t)
	desc := text.ItemDescription(0)
	if !strings.Contains(desc, "knife") {
		t.Errorf("the first item description is %q", desc)
	}
}

// TestOriginalDialogues verifies the imported dialogues.
func TestOriginalDialogues(t *testing.T) {
	text := testText(t)
	lines := text.Dialogues(1)
	if len(lines) == 0 {
		t.Fatal("the scenario 1 has no dialogue")
	}
	// The first line of the scenario 1 names a speaker.
	speaker, body := CleanDialogue(lines[0])
	if speaker == "" {
		t.Errorf("the first line %q has no speaker", lines[0])
	}
	if body == "" {
		t.Errorf("the first line %q has no text", lines[0])
	}
	// An unknown scenario returns nothing.
	if got := text.Dialogues(99); got != nil {
		t.Error("an unknown scenario returned dialogues")
	}
}

// TestCleanDialogue verifies the parsing of the speaker prefix.
func TestCleanDialogue(t *testing.T) {
	speaker, body := CleanDialogue("|Butler Marco|\nI'm sorry.")
	if speaker != "Butler Marco" {
		t.Errorf("the speaker is %q, want Butler Marco", speaker)
	}
	if body != "I'm sorry." {
		t.Errorf("the text is %q, want \"I'm sorry.\"", body)
	}

	// A line without a speaker is returned unchanged.
	speaker, body = CleanDialogue("Just a line.")
	if speaker != "" {
		t.Errorf("the speaker is %q, want an empty one", speaker)
	}
	if body != "Just a line." {
		t.Errorf("the text is %q", body)
	}
}
