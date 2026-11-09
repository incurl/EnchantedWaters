package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/render"
)

// ShipyardPopup is the ship repair and purchase screen.
type ShipyardPopup struct {
	// ui is the parent interface.
	ui *UI
	// portID is the port the shipyard belongs to.
	portID string
	// selected is the index of the highlighted ship class.
	selected int
}

// NewShipyardPopup creates the shipyard screen.
func NewShipyardPopup(u *UI, portID string) *ShipyardPopup {
	return &ShipyardPopup{ui: u, portID: portID}
}

// Title implements Popup.
func (s *ShipyardPopup) Title() string { return "Shipyard" }

// Update implements Popup.
func (s *ShipyardPopup) Update() bool {
	ships := s.ui.cfg.ShipOrder
	if len(ships) == 0 {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		s.selected = (s.selected - 1 + len(ships)) % len(ships)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		s.selected = (s.selected + 1) % len(ships)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		s.repair()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		s.buy(ships[s.selected])
	}
	return true
}

// repair repairs the player's hull.
func (s *ShipyardPopup) repair() {
	fleetID := s.ui.world.PlayerEntity()
	quote, err := s.ui.world.ExecuteRepair(fleetID, s.portID)
	if err != nil {
		s.ui.Message("%v", err)
		return
	}
	s.ui.Message("repaired for %d gold (%d days)", int(quote.Cost), quote.Days)
}

// buy purchases the selected ship class.
func (s *ShipyardPopup) buy(shipID string) {
	fleetID := s.ui.world.PlayerEntity()
	quote, err := s.ui.world.ExecutePurchase(fleetID, shipID, s.portID)
	if err != nil {
		s.ui.Message("%v", err)
		return
	}
	s.ui.Message("bought %s for %d gold (trade-in %d)", s.ui.cfg.Ship(shipID).Name, int(quote.Cost), int(quote.TradeIn))
}

// Draw implements Popup.
func (s *ShipyardPopup) Draw(canvas *render.Canvas) {
	ships := s.ui.cfg.ShipOrder
	w, h := 420, 240
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(canvas, x, y, w, h)
	render.DrawPanel(canvas, x, y, w, h, render.ColorPanel)

	render.DrawText(canvas, "Shipyard", x+12, y+10, render.ColorTextBright)
	render.DrawText(canvas, "R: repair   enter: buy   esc: back", x+12, y+22, render.ColorTextDim)

	// The repair quote.
	fleetID := s.ui.world.PlayerEntity()
	quote := s.ui.world.QuoteRepair(fleetID, s.portID)
	repairText := "The hull is sound."
	if quote.MissingPoints > 0 {
		repairText = fmt.Sprintf("Repair: %d points for %d gold (%d days)",
			int(quote.MissingPoints), int(quote.Cost), quote.Days)
	}
	render.DrawText(canvas, repairText, x+12, y+36, render.ColorTextDim)

	for i, shipID := range ships {
		ship := s.ui.cfg.Ship(shipID)
		if ship == nil {
			continue
		}
		rowY := y + 52 + i*render.LineHeight
		colour := render.ColorText
		if i == s.selected {
			render.DrawSelectionMarker(canvas, x+6, rowY)
			colour = render.ColorTextBright
		}
		render.DrawText(canvas, ship.Name, x+16, rowY, colour)
		render.DrawText(canvas, fmt.Sprintf("hull %d  crew %d  hold %d", int(ship.HullPoints), ship.MaxSailors, int(ship.CargoCapacity)), x+120, rowY, render.ColorTextDim)
		pq := s.ui.world.QuotePurchase(shipID, s.portID)
		render.DrawTextRight(canvas, fmt.Sprintf("%d", int(pq.Cost)), x+w-12, rowY, render.ColorGold)
	}
}

// TavernPopup is the recruit and rumour screen.
type TavernPopup struct {
	// ui is the parent interface.
	ui *UI
	// portID is the port the tavern belongs to.
	portID string
	// selected is the index of the highlighted recruit.
	selected int
	// rumor is the last rumour that was heard.
	rumor string
	// rumors are the rumour pool of the port.
	rumors []string
}

// NewTavernPopup creates the tavern screen.
func NewTavernPopup(u *UI, portID string) *TavernPopup {
	return &TavernPopup{ui: u, portID: portID}
}

// Title implements Popup.
func (t *TavernPopup) Title() string { return "Tavern" }

// Update implements Popup.
func (t *TavernPopup) Update() bool {
	templates := t.ui.characterIDs()
	if len(templates) == 0 {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		t.selected = (t.selected - 1 + len(templates)) % len(templates)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		t.selected = (t.selected + 1) % len(templates)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyH) {
		t.hearRumor()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		t.recruit(templates[t.selected])
	}
	return true
}

// recruit hires the selected character template.
func (t *TavernPopup) recruit(templateID string) {
	fleetID := t.ui.world.PlayerEntity()
	if _, err := t.ui.world.RecruitSailor(fleetID, t.portID, templateID); err != nil {
		t.ui.Message("%v", err)
		return
	}
	t.ui.Message("hired %s", t.ui.cfg.Characters[templateID].Name)
}

// hearRumor draws a rumour from the rumour table.
func (t *TavernPopup) hearRumor() {
	rumors := t.ui.rumorPool(t.portID)
	if len(rumors) == 0 {
		t.ui.Message("the tavern is quiet tonight")
		return
	}
	// The rumours are drawn deterministically from the day, so that the
	// player always hears the same rumour on the same day.
	idx := t.ui.world.Day() % len(rumors)
	text := rumors[idx]
	// The placeholders are replaced with the actual port or region.
	text = t.ui.fillRumor(text, t.portID)
	t.rumor = text
	t.rumors = append(t.rumors, text)
	t.ui.Message("rumour: %s", text)
}

// Draw implements Popup.
func (t *TavernPopup) Draw(canvas *render.Canvas) {
	templates := t.ui.characterIDs()
	w, h := 470, 230
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(canvas, x, y, w, h)
	render.DrawPanel(canvas, x, y, w, h, render.ColorPanel)

	// --- the header of the screen ---
	//
	// Every line has its own row, so that the wording of the original
	// game and the labels of this project never collide.
	render.DrawText(canvas, "Tavern", x+12, y+10, render.ColorTextBright)
	render.DrawText(canvas, "H: hear a rumour   enter: hire   esc: back", x+12, y+22, render.ColorTextDim)

	// The invitation of the original game.
	render.DrawText(canvas, t.ui.text.Get(textRecruitQuestion), x+12, y+34, render.ColorTextDim)

	crew := t.ui.world.SailorCapacity(t.ui.world.PlayerEntity())
	fleet := t.ui.world.SnapshotFleet(t.ui.world.PlayerEntity())
	render.DrawTextRight(canvas,
		fmt.Sprintf("crew %d/%d   %d gold", fleet.Sailors, crew, int(t.ui.world.Treasury())),
		x+w-12, y+10, render.ColorGold)

	// --- the column titles ---
	//
	// The columns are laid out on a fixed grid, so that a long class name
	// never runs into the cost.
	const (
		listTop  = 56
		colClass = 150
		colStats = 290
	)
	render.DrawText(canvas, "recruit", x+16, y+listTop-10, render.ColorTextDim)
	render.DrawText(canvas, "class", x+colClass, y+listTop-10, render.ColorTextDim)
	render.DrawText(canvas, "adm/trd/nav", x+colStats, y+listTop-10, render.ColorTextDim)
	render.DrawTextRight(canvas, "cost", x+w-12, y+listTop-10, render.ColorTextDim)

	// --- the recruits ---
	for i, templateID := range templates {
		tmpl := t.ui.cfg.Characters[templateID]
		if tmpl == nil {
			continue
		}
		rowY := y + listTop + 4 + i*render.LineHeight
		if rowY+render.CharHeight > y+h-4 {
			break
		}
		colour := render.ColorText
		if i == t.selected {
			render.DrawSelectionMarker(canvas, x+6, rowY)
			colour = render.ColorTextBright
		}
		render.DrawText(canvas, truncate(tmpl.Name, 16), x+16, rowY, colour)
		render.DrawText(canvas, truncate(string(tmpl.Class), 15), x+colClass, rowY, render.ColorTextDim)
		render.DrawText(canvas, fmt.Sprintf("%2.0f/%2.0f/%2.0f", tmpl.Adventure, tmpl.Trade, tmpl.Naval), x+colStats, rowY, render.ColorTextDim)
		render.DrawTextRight(canvas, fmt.Sprintf("%d", int(tmpl.HiringCost)), x+w-12, rowY, render.ColorGold)
	}

	// --- the portrait of the highlighted recruit ---
	//
	// The face is the one of the original game: the recruits are numbered
	// in a fixed order, so the same recruit always shows the same face.
	if len(templates) > 0 {
		index := t.selected
		if index >= len(templates) {
			index = len(templates) - 1
		}
		t.ui.DrawPortrait(canvas, index*7+3, x+w-78, y+52)
	}

	// --- the rumour ---
	if t.rumor != "" {
		render.DrawText(canvas, "rumour:", x+12, y+h-34, render.ColorTextBright)
		render.DrawText(canvas, wrap(t.rumor, 50), x+12, y+h-22, render.ColorTextDim)
	}
}
