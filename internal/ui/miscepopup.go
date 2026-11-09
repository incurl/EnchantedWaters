package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/render"
	"github.com/peter/enchantedwaters/internal/world"
)

// GovernorPopup is the screen of the governor's mansion.
type GovernorPopup struct {
	// ui is the parent interface.
	ui *UI
	// portID is the port the governor belongs to.
	portID string
	// gain is the fame gained by the last visit.
	gain float64
}

// NewGovernorPopup creates the governor screen.
func NewGovernorPopup(u *UI, portID string) *GovernorPopup {
	return &GovernorPopup{ui: u, portID: portID}
}

// Title implements Popup.
func (g *GovernorPopup) Title() string { return "Governor's Mansion" }

// Update implements Popup.
func (g *GovernorPopup) Update() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		fleetID := g.ui.world.PlayerEntity()
		gain, err := g.ui.world.VisitGovernor(fleetID, g.portID)
		if err != nil {
			g.ui.Message("%v", err)
		} else {
			g.gain = gain
			g.ui.Message("the governor wishes you well (+%.1f trade fame)", gain)
		}
	}
	return true
}

// Draw implements Popup.
func (g *GovernorPopup) Draw(canvas *render.Canvas) {
	w, h := 320, 150
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(canvas, x, y, w, h)
	render.DrawPanel(canvas, x, y, w, h, render.ColorPanel)

	port := g.ui.cfg.Port(g.portID)
	name := "the governor"
	if port != nil {
		name = "Governor of " + port.Name
	}
	render.DrawText(canvas, name, x+12, y+10, render.ColorTextBright)
	render.DrawText(canvas, "enter: pay respects   esc: back", x+12, y+22, render.ColorTextDim)

	// --- the portrait of the governor ---
	//
	// The face comes out of the portraits of the original game, and it is
	// fixed per port, so the governor of a port always looks the same.
	textX := x + 12
	if g.ui.DrawPortrait(canvas, g.ui.portIndex(g.portID)*11+5, x+12, y+44) {
		textX = x + 12 + 76
	}

	render.DrawText(canvas, "Your trade renown opens many doors.", textX, y+44, render.ColorText)
	if g.gain > 0 {
		render.DrawText(canvas, fmt.Sprintf("Trade fame +%.1f", g.gain), textX, y+60, render.ColorGood)
	}
	adv, trade, naval := g.ui.world.ReputationSnapshot(world.FactionPlayer)
	render.DrawText(canvas, fmt.Sprintf("Adventure %.0f", adv), textX, y+80, render.ColorTextDim)
	render.DrawText(canvas, fmt.Sprintf("Trade %.0f", trade), textX, y+92, render.ColorTextDim)
	render.DrawText(canvas, fmt.Sprintf("Naval %.0f", naval), textX, y+104, render.ColorTextDim)
	titles := g.ui.world.Titles(world.FactionPlayer)
	if len(titles) > 0 {
		render.DrawText(canvas, truncate("titles: "+strings.Join(titles, ", "), 34), textX, y+120, render.ColorGold)
	}
}

// ChurchPopup is the screen of the church.
type ChurchPopup struct {
	// ui is the parent interface.
	ui *UI
	// portID is the port the church belongs to.
	portID string
	// donation is the last donation amount.
	donation float64
	// gain is the fame gained by the last donation.
	gain float64
}

// NewChurchPopup creates the church screen.
func NewChurchPopup(u *UI, portID string) *ChurchPopup {
	return &ChurchPopup{ui: u, portID: portID}
}

// Title implements Popup.
func (c *ChurchPopup) Title() string { return "Church" }

// Update implements Popup.
func (c *ChurchPopup) Update() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		// The donation is a tenth of the treasury, rounded down.
		amount := float64(int(c.ui.world.Treasury() / 10))
		fleetID := c.ui.world.PlayerEntity()
		gain, err := c.ui.world.DonateToChurch(fleetID, c.portID, amount)
		if err != nil {
			c.ui.Message("%v", err)
		} else {
			c.donation = amount
			c.gain = gain
			c.ui.Message("donated %d gold (+%.1f adventure fame)", int(amount), gain)
		}
	}
	return true
}

// Draw implements Popup.
func (c *ChurchPopup) Draw(canvas *render.Canvas) {
	w, h := 320, 140
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(canvas, x, y, w, h)
	render.DrawPanel(canvas, x, y, w, h, render.ColorPanel)

	render.DrawText(canvas, "Church", x+12, y+10, render.ColorTextBright)
	render.DrawText(canvas, "enter: donate a tenth of your gold   esc: back", x+12, y+22, render.ColorTextDim)
	render.DrawText(canvas, "Your coin buys the prayers of the faithful.", x+12, y+44, render.ColorText)
	if c.donation > 0 {
		render.DrawText(canvas, fmt.Sprintf("Donated %d gold", int(c.donation)), x+12, y+62, render.ColorGold)
		render.DrawText(canvas, fmt.Sprintf("Adventure fame +%.1f", c.gain), x+12, y+76, render.ColorGood)
	}
	render.DrawText(canvas, fmt.Sprintf("Treasury: %d gold", int(c.ui.world.Treasury())), x+12, y+100, render.ColorTextDim)
}

// InnPopup is the screen of the inn.
type InnPopup struct {
	// ui is the parent interface.
	ui *UI
	// portID is the port the inn belongs to.
	portID string
	// cost is the price of the last rest.
	cost float64
}

// NewInnPopup creates the inn screen.
func NewInnPopup(u *UI, portID string) *InnPopup {
	return &InnPopup{ui: u, portID: portID}
}

// Title implements Popup.
func (i *InnPopup) Title() string { return "Inn" }

// Update implements Popup.
func (i *InnPopup) Update() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		fleetID := i.ui.world.PlayerEntity()
		cost, err := i.ui.world.RestInInn(fleetID, i.portID)
		if err != nil {
			i.ui.Message("%v", err)
		} else {
			i.cost = cost
			i.ui.Message("rested for %d gold", int(cost))
		}
	}
	return true
}

// Draw implements Popup.
func (i *InnPopup) Draw(canvas *render.Canvas) {
	w, h := 300, 130
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(canvas, x, y, w, h)
	render.DrawPanel(canvas, x, y, w, h, render.ColorPanel)

	render.DrawText(canvas, "Inn", x+12, y+10, render.ColorTextBright)
	render.DrawText(canvas, "enter: rest the crew   esc: back", x+12, y+22, render.ColorTextDim)

	fleet := i.ui.world.SnapshotFleet(i.ui.world.PlayerEntity())
	render.DrawText(canvas, fmt.Sprintf("Fatigue %.0f   Morale %.0f", fleet.Fatigue, fleet.Morale), x+12, y+44, render.ColorText)
	render.DrawHBar(canvas, x+12, y+58, 120, 6, fleet.Morale/100, render.ColorGood, render.ColorPanelLight)
	render.DrawHBar(canvas, x+12, y+70, 120, 6, fleet.Fatigue/100, render.ColorBad, render.ColorPanelLight)
	if i.cost > 0 {
		render.DrawText(canvas, fmt.Sprintf("Paid %d gold", int(i.cost)), x+12, y+88, render.ColorGold)
	}
}

// SailPopup lets the player choose the next destination.
type SailPopup struct {
	// ui is the parent interface.
	ui *UI
	// portID is the port the player is leaving.
	portID string
	// selected is the index of the highlighted destination.
	selected int
	// destinations are the candidate ports, sorted by distance.
	destinations []string
}

// NewSailPopup creates the destination chooser.
func NewSailPopup(u *UI, portID string) *SailPopup {
	p := &SailPopup{ui: u, portID: portID}
	// The destinations are the ports of the world, sorted by their
	// distance from the current port.
	origin := u.cfg.Port(portID)
	if origin != nil {
		for _, id := range u.cfg.PortOrder {
			if id == portID {
				continue
			}
			p.destinations = append(p.destinations, id)
		}
		sort.Slice(p.destinations, func(a, b int) bool {
			pa := u.cfg.Port(p.destinations[a])
			pb := u.cfg.Port(p.destinations[b])
			if pa == nil || pb == nil {
				return false
			}
			da := (pa.X-origin.X)*(pa.X-origin.X) + (pa.Y-origin.Y)*(pa.Y-origin.Y)
			db := (pb.X-origin.X)*(pb.X-origin.X) + (pb.Y-origin.Y)*(pb.Y-origin.Y)
			return da < db
		})
	}
	return p
}

// Title implements Popup.
func (s *SailPopup) Title() string { return "Set Sail" }

// Update implements Popup.
func (s *SailPopup) Update() bool {
	if len(s.destinations) == 0 {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		s.selected = (s.selected - 1 + len(s.destinations)) % len(s.destinations)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		s.selected = (s.selected + 1) % len(s.destinations)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		target := s.destinations[s.selected]
		fleetID := s.ui.world.PlayerEntity()
		bearing, err := s.ui.world.SailToPort(fleetID, target)
		if err != nil {
			s.ui.Message("%v", err)
			s.ui.playEffect(effectAlert)
		} else {
			s.ui.Message("course set to %s (bearing %.0f)", s.ui.cfg.Port(target).Name, bearing)
			// The sound of a ship that leaves a port.
			s.ui.playEffect(effectSail)
			s.ui.followCamera = true
			s.ui.popups.PopAll()
		}
	}
	return true
}

// Draw implements Popup.
func (s *SailPopup) Draw(canvas *render.Canvas) {
	const visible = 12
	w, h := 300, visible*render.LineHeight+46
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(canvas, x, y, w, h)
	render.DrawPanel(canvas, x, y, w, h, render.ColorPanel)

	render.DrawText(canvas, "Set Sail", x+12, y+10, render.ColorTextBright)
	render.DrawText(canvas, "up/down: choose   enter: depart   esc: back", x+12, y+22, render.ColorTextDim)

	start := 0
	if s.selected >= visible {
		start = s.selected - visible + 1
	}
	for i := 0; i < visible; i++ {
		idx := start + i
		if idx >= len(s.destinations) {
			break
		}
		port := s.ui.cfg.Port(s.destinations[idx])
		if port == nil {
			continue
		}
		rowY := y + 38 + i*render.LineHeight
		colour := render.ColorText
		if idx == s.selected {
			render.DrawSelectionMarker(canvas, x+6, rowY)
			colour = render.ColorTextBright
		}
		render.DrawText(canvas, port.Name, x+16, rowY, colour)
		render.DrawTextRight(canvas, port.Region, x+w-12, rowY, render.ColorTextDim)
	}
}

// characterIDs returns the deterministic list of the character template
// IDs.
func (u *UI) characterIDs() []string {
	if u.charIDs == nil {
		for id := range u.cfg.Characters {
			u.charIDs = append(u.charIDs, id)
		}
		sort.Strings(u.charIDs)
	}
	return u.charIDs
}

// rumorPool returns the rumours that can be heard at a port.
//
// A rumour is eligible when its day range matches the current day and
// its target port or region matches the port.
func (u *UI) rumorPool(portID string) []string {
	port := u.cfg.Port(portID)
	if port == nil {
		return nil
	}
	day := u.world.Day()
	out := make([]string, 0, 8)
	for _, id := range u.cfg.RumorOrder {
		rumor := u.cfg.Rumors[id]
		if rumor == nil {
			continue
		}
		if rumor.MinDay > 0 && day < rumor.MinDay {
			continue
		}
		if rumor.MaxDay > 0 && day > rumor.MaxDay {
			continue
		}
		if rumor.TargetPort != "" && rumor.TargetPort != portID {
			continue
		}
		if rumor.TargetRegion != "" && rumor.TargetRegion != port.Region {
			continue
		}
		out = append(out, rumor.Text)
	}
	return out
}

// fillRumor replaces the placeholders of a rumour with real names.
//
// The port placeholder is filled with the port the rumour was heard in;
// the region placeholder with the region of that port.
func (u *UI) fillRumor(text, portID string) string {
	port := u.cfg.Port(portID)
	region := ""
	portName := portID
	if port != nil {
		region = port.Region
		portName = port.Name
	}
	text = strings.ReplaceAll(text, "{port}", portName)
	text = strings.ReplaceAll(text, "{region}", region)
	return text
}

// wrap breaks a text into lines of at most width characters.
func wrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	current := words[0]
	for _, w := range words[1:] {
		if len(current)+1+len(w) > width {
			lines = append(lines, current)
			current = w
			continue
		}
		current += " " + w
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

// unusedConfig keeps the config import referenced; the type is used by
// the callers of this file through the UI struct.
var _ = config.PortCity
