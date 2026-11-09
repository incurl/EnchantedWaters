package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/render"
	"github.com/peter/enchantedwaters/internal/world"
)

// PortPopup is the port menu: it is the entry point to every port
// facility (exchange, shipyard, tavern, governor, church, inn).
type PortPopup struct {
	// ui is the parent interface.
	ui *UI
	// portID is the port the menu belongs to.
	portID string
	// selected is the index of the highlighted menu entry.
	selected int
}

// PortMenuEntry is one entry of the port menu.
type PortMenuEntry struct {
	// Label is the displayed text.
	Label string
	// Enabled reports whether the entry can be selected.
	Enabled bool
	// Action is the identifier of the action.
	Action string
}

// NewPortPopup creates the port menu of a port.
func NewPortPopup(u *UI, portID string) *PortPopup {
	return &PortPopup{ui: u, portID: portID}
}

// Title implements Popup.
func (p *PortPopup) Title() string {
	if port := p.ui.cfg.Port(p.portID); port != nil {
		return "Port: " + port.Name
	}
	return "Port"
}

// menu returns the entries of the port menu.
//
// The entries are rebuilt on every call so that they always reflect the
// current state of the port.
func (p *PortPopup) menu() []PortMenuEntry {
	port := p.ui.cfg.Port(p.portID)
	if port == nil {
		return nil
	}
	return []PortMenuEntry{
		{Label: "Exchange", Enabled: true, Action: "exchange"},
		{Label: "Shipyard", Enabled: port.ShipyardLevel > 0, Action: "shipyard"},
		{Label: "Tavern", Enabled: port.TavernLevel > 0, Action: "tavern"},
		{Label: "Governor's Mansion", Enabled: port.GovernorLevel > 0, Action: "governor"},
		{Label: "Church", Enabled: port.ChurchLevel > 0, Action: "church"},
		{Label: "Inn", Enabled: port.InnLevel > 0, Action: "inn"},
		{Label: "Set Sail", Enabled: true, Action: "sail"},
		{Label: "Leave", Enabled: true, Action: "leave"},
	}
}

// Update implements Popup.
func (p *PortPopup) Update() bool {
	menu := p.menu()
	if len(menu) == 0 {
		return false
	}
	// The selection wraps around.
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		p.selected = (p.selected - 1 + len(menu)) % len(menu)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		p.selected = (p.selected + 1) % len(menu)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		entry := menu[p.selected]
		if !entry.Enabled {
			p.ui.Message("%s is not available here.", entry.Label)
			return true
		}
		p.activate(entry.Action)
	}
	return true
}

// activate opens the screen of a facility.
func (p *PortPopup) activate(action string) {
	switch action {
	case "exchange":
		p.ui.popups.Push(NewExchangePopup(p.ui, p.portID))
	case "shipyard":
		p.ui.popups.Push(NewShipyardPopup(p.ui, p.portID))
	case "tavern":
		p.ui.popups.Push(NewTavernPopup(p.ui, p.portID))
	case "governor":
		p.ui.popups.Push(NewGovernorPopup(p.ui, p.portID))
	case "church":
		p.ui.popups.Push(NewChurchPopup(p.ui, p.portID))
	case "inn":
		p.ui.popups.Push(NewInnPopup(p.ui, p.portID))
	case "sail":
		p.ui.popups.Push(NewSailPopup(p.ui, p.portID))
	case "leave":
		p.ui.popups.Pop()
	}
}

// Draw implements Popup.
func (p *PortPopup) Draw(screen *ebiten.Image) {
	menu := p.menu()
	w, h := 220, len(menu)*render.LineHeight+40
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(screen, x, y, w, h)
	render.DrawPanel(screen, x, y, w, h, render.ColorPanel)

	port := p.ui.cfg.Port(p.portID)
	title := "Port"
	if port != nil {
		title = port.Name
	}
	render.DrawText(screen, title, x+12, y+10, render.ColorTextBright)
	render.DrawText(screen, "choose a facility", x+12, y+22, render.ColorTextDim)

	for i, entry := range menu {
		rowY := y + 36 + i*render.LineHeight
		colour := render.ColorText
		if !entry.Enabled {
			colour = render.ColorTextDim
		}
		if i == p.selected {
			render.DrawSelectionMarker(screen, x+6, rowY)
			colour = render.ColorTextBright
		}
		render.DrawText(screen, entry.Label, x+16, rowY, colour)
	}
}

// ExchangePopup is the trading screen of a port.
type ExchangePopup struct {
	// ui is the parent interface.
	ui *UI
	// portID is the port the exchange belongs to.
	portID string
	// selected is the index of the highlighted commodity.
	selected int
	// selling switches the popup into the sell mode.
	selling bool
	// quantity is the number of units to trade.
	quantity int
}

// NewExchangePopup creates the exchange screen.
func NewExchangePopup(u *UI, portID string) *ExchangePopup {
	return &ExchangePopup{ui: u, portID: portID, quantity: 1}
}

// Title implements Popup.
func (e *ExchangePopup) Title() string {
	if e.selling {
		return "Exchange: sell"
	}
	return "Exchange: buy"
}

// Update implements Popup.
func (e *ExchangePopup) Update() bool {
	commodities := e.ui.cfg.CommodityOrder
	if len(commodities) == 0 {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		e.selected = (e.selected - 1 + len(commodities)) % len(commodities)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		e.selected = (e.selected + 1) % len(commodities)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		if e.quantity > 1 {
			e.quantity--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		e.quantity++
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		e.selling = !e.selling
		e.quantity = 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		e.trade(commodities[e.selected])
	}
	return true
}

// trade executes the trade of the selected commodity.
func (e *ExchangePopup) trade(commodityID string) {
	fleetID := e.ui.world.PlayerEntity()
	var err error
	if e.selling {
		_, err = e.ui.world.ExecuteSell(fleetID, commodityID, e.portID, float64(e.quantity))
	} else {
		_, err = e.ui.world.ExecuteBuy(fleetID, commodityID, e.portID, float64(e.quantity))
	}
	if err != nil {
		e.ui.Message("%v", err)
		return
	}
	action := "bought"
	if e.selling {
		action = "sold"
	}
	e.ui.Message("%s %d %s", action, e.quantity, e.ui.cfg.Commodity(commodityID).Name)
}

// Draw implements Popup.
func (e *ExchangePopup) Draw(screen *ebiten.Image) {
	commodities := e.ui.cfg.CommodityOrder
	w, h := 440, 260
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(screen, x, y, w, h)
	render.DrawPanel(screen, x, y, w, h, render.ColorPanel)

	// --- header ---
	mode := "BUY"
	if e.selling {
		mode = "SELL"
	}
	render.DrawText(screen, "Exchange - "+mode, x+12, y+10, render.ColorTextBright)
	render.DrawText(screen, fmt.Sprintf("gold: %d   qty: %d (left/right)", int(e.ui.world.Treasury()), e.quantity), x+12, y+22, render.ColorTextDim)
	render.DrawText(screen, "tab: switch mode   enter: trade   esc: back", x+12, y+34, render.ColorTextDim)

	// --- the commodity list ---
	const visible = 9
	start := 0
	if e.selected >= visible {
		start = e.selected - visible + 1
	}
	for i := 0; i < visible; i++ {
		idx := start + i
		if idx >= len(commodities) {
			break
		}
		cid := commodities[idx]
		c := e.ui.cfg.Commodity(cid)
		if c == nil {
			continue
		}
		rowY := y + 50 + i*render.LineHeight
		price := e.ui.world.Price(cid, e.portID)
		stock := e.ui.world.Stock(cid, e.portID)

		colour := render.ColorText
		if idx == e.selected {
			render.DrawSelectionMarker(screen, x+6, rowY)
			colour = render.ColorTextBright
		}
		render.DrawText(screen, c.Name, x+16, rowY, colour)
		render.DrawTextRight(screen, fmt.Sprintf("%.0f", price), x+260, rowY, render.ColorGold)
		render.DrawTextRight(screen, fmt.Sprintf("%.0f", stock), x+330, rowY, render.ColorTextDim)
		render.DrawTextRight(screen, fmt.Sprintf("%.0f", c.BasePrice), x+w-12, rowY, render.ColorTextDim)
	}

	// --- the quote of the selected commodity ---
	if e.selected < len(commodities) {
		cid := commodities[e.selected]
		fleetID := e.ui.world.PlayerEntity()
		var quote world.Quote
		if e.selling {
			quote = e.ui.world.QuoteSell(fleetID, cid, e.portID, float64(e.quantity))
		} else {
			quote = e.ui.world.QuoteBuy(fleetID, cid, e.portID, float64(e.quantity))
		}
		qy := y + h - 56
		render.DrawText(screen, fmt.Sprintf("total %.0f   tax %.0f", quote.Total, quote.Tax), x+12, qy, render.ColorText)
		amount := fmt.Sprintf("pay %.0f", quote.GrandTotal)
		if e.selling {
			amount = fmt.Sprintf("receive %.0f", quote.GrandTotal)
		}
		render.DrawText(screen, amount, x+12, qy+render.LineHeight, render.ColorGold)
		if quote.Reason != "" {
			render.DrawText(screen, quote.Reason, x+150, qy+render.LineHeight, render.ColorBad)
		}
	}
}
