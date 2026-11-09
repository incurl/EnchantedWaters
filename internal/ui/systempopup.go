package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/render"
	"github.com/peter/enchantedwaters/internal/world"
)

// SystemPopup is the in-game menu.
//
// It gathers the system level actions: the save and load of the session,
// the speed of the simulation and the information about the game.
type SystemPopup struct {
	// ui is the parent interface.
	ui *UI
	// selected is the index of the highlighted entry.
	selected int
}

// systemEntries are the entries of the system menu.
var systemEntries = []struct {
	// label is the displayed text.
	label string
	// action is the identifier of the action.
	action string
}{
	{"Save the game", "save"},
	{"Load the game", "load"},
	{"Speed: 1", "speed1"},
	{"Speed: 2", "speed2"},
	{"Speed: 4", "speed4"},
	{"Speed: 8", "speed8"},
	{"Close", "close"},
}

// NewSystemPopup creates the system menu.
func NewSystemPopup(u *UI) *SystemPopup {
	return &SystemPopup{ui: u}
}

// Title implements Popup.
func (s *SystemPopup) Title() string { return "System" }

// Update implements Popup.
func (s *SystemPopup) Update() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		s.selected = (s.selected - 1 + len(systemEntries)) % len(systemEntries)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		s.selected = (s.selected + 1) % len(systemEntries)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		s.activate(systemEntries[s.selected].action)
	}
	return true
}

// activate performs the action of an entry.
func (s *SystemPopup) activate(action string) {
	switch action {
	case "save":
		path := s.ui.world.SaveDir() + "/quicksave.json"
		if err := s.ui.world.Save(path); err != nil {
			s.ui.Message("save failed: %v", err)
		} else {
			s.ui.Message("the game is saved")
		}
	case "load":
		path := s.ui.world.SaveDir() + "/quicksave.json"
		if err := s.ui.world.Load(path); err != nil {
			s.ui.Message("load failed: %v", err)
		} else {
			s.ui.Message("the game is loaded")
		}
	case "speed1":
		s.ui.world.SetSpeed(1)
	case "speed2":
		s.ui.world.SetSpeed(2)
	case "speed4":
		s.ui.world.SetSpeed(4)
	case "speed8":
		s.ui.world.SetSpeed(8)
	case "close":
		s.ui.popups.Pop()
	}
}

// Draw implements Popup.
func (s *SystemPopup) Draw(screen *ebiten.Image) {
	w, h := 240, len(systemEntries)*render.LineHeight+44
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(screen, x, y, w, h)
	render.DrawPanel(screen, x, y, w, h, render.ColorPanel)

	render.DrawText(screen, "System", x+12, y+10, render.ColorTextBright)
	render.DrawText(screen, "enter: choose   esc: back", x+12, y+22, render.ColorTextDim)

	// The speed entries reflect the current speed.
	speed := s.ui.world.Speed()
	for i, entry := range systemEntries {
		label := entry.label
		switch entry.action {
		case "speed1":
			if speed == 1 {
				label = "> Speed: 1"
			}
		case "speed2":
			if speed == 2 {
				label = "> Speed: 2"
			}
		case "speed4":
			if speed == 4 {
				label = "> Speed: 4"
			}
		case "speed8":
			if speed == 8 {
				label = "> Speed: 8"
			}
		}
		rowY := y + 40 + i*render.LineHeight
		colour := render.ColorText
		if i == s.selected {
			render.DrawSelectionMarker(screen, x+6, rowY)
			colour = render.ColorTextBright
		}
		render.DrawText(screen, label, x+16, rowY, colour)
	}

	// The reputation of the player.
	adv, trade, naval := s.ui.world.ReputationSnapshot(world.FactionPlayer)
	render.DrawText(screen, fmt.Sprintf("Adventure %.0f  Trade %.0f  Naval %.0f", adv, trade, naval),
		x+12, y+h-18, render.ColorTextDim)
}
