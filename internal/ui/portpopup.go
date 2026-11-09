package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/config"
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

// portFacilityMenu is the index of the menu of the original game that
// holds the facilities of a port.
//
// The menu tree of the original game holds 12 facilities: Market, Pub,
// Shipyard, Harbor, Lodge, Palace, Guild, NPC, Bank, Item Shop, Church
// and Fortune.
const portFacilityMenu = 58

// menu returns the entries of the port menu.
//
// The entries are rebuilt on every call so that they always reflect the
// current state of the port.
//
// The labels of the facilities come from the menu tree of the original
// game when it is available; the facilities that this project does not
// implement yet are shown as disabled.
func (p *PortPopup) menu() []PortMenuEntry {
	port := p.ui.cfg.Port(p.portID)
	if port == nil {
		return nil
	}

	// --- the facilities of the original game ---
	original := p.ui.cfg.OriginalMenu(portFacilityMenu)
	entries := make([]PortMenuEntry, 0, len(original)+2)
	for _, label := range original {
		entries = append(entries, PortMenuEntry{
			Label:   label,
			Enabled: p.facilityEnabled(label, port),
			Action:  p.facilityAction(label),
		})
	}
	if len(entries) == 0 {
		// The fallback: the facilities that this project implements.
		entries = append(entries,
			PortMenuEntry{Label: "Exchange", Enabled: true, Action: "exchange"},
			PortMenuEntry{Label: "Shipyard", Enabled: port.ShipyardLevel > 0, Action: "shipyard"},
			PortMenuEntry{Label: "Tavern", Enabled: port.TavernLevel > 0, Action: "tavern"},
			PortMenuEntry{Label: "Governor's Mansion", Enabled: port.GovernorLevel > 0, Action: "governor"},
			PortMenuEntry{Label: "Church", Enabled: port.ChurchLevel > 0, Action: "church"},
			PortMenuEntry{Label: "Inn", Enabled: port.InnLevel > 0, Action: "inn"},
		)
	}

	entries = append(entries,
		PortMenuEntry{Label: "Set Sail", Enabled: true, Action: "sail"},
		PortMenuEntry{Label: "Leave", Enabled: true, Action: "leave"},
	)
	return entries
}

// facilityEnabled reports whether a facility of the original game can be
// used in a port.
//
// The facilities that this project does not implement yet are shown, but
// they cannot be selected: the player sees the structure of the original
// game without being led into a dead end.
func (p *PortPopup) facilityEnabled(label string, port *config.Port) bool {
	switch label {
	case "Market":
		return true
	case "Shipyard":
		return port.ShipyardLevel > 0
	case "Pub":
		return port.TavernLevel > 0
	case "Palace":
		return port.GovernorLevel > 0
	case "Church":
		return port.ChurchLevel > 0
	case "Lodge":
		return port.InnLevel > 0
	default:
		// The other facilities of the original game (Harbor, Guild, NPC,
		// Bank, Item Shop, Fortune) are not implemented yet.
		return false
	}
}

// facilityAction maps the label of a facility to the action that opens
// its screen.
//
// The labels are the ones of the original game.
func (p *PortPopup) facilityAction(label string) string {
	switch label {
	case "Market":
		return "exchange"
	case "Shipyard":
		return "shipyard"
	case "Pub":
		return "tavern"
	case "Palace":
		return "governor"
	case "Church":
		return "church"
	case "Lodge":
		return "inn"
	default:
		return "none"
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
func (p *PortPopup) Draw(canvas *render.Canvas) {
	menu := p.menu()
	// The panel is wide enough for the longest label of the original
	// game, and it is never taller than the screen.
	w := 200
	for _, entry := range menu {
		if width := render.TextWidth(entry.Label) + 40; width > w {
			w = width
		}
	}
	h := len(menu)*render.LineHeight + 40
	if h > LogicalHeight-8 {
		h = LogicalHeight - 8
	}
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(canvas, x, y, w, h)
	render.DrawPanel(canvas, x, y, w, h, render.ColorPanel)

	port := p.ui.cfg.Port(p.portID)
	title := "Port"
	if port != nil {
		title = port.Name
	}
	render.DrawText(canvas, title, x+12, y+10, render.ColorTextBright)
	render.DrawText(canvas, "choose a facility", x+12, y+22, render.ColorTextDim)

	// The visible rows are the ones around the selection, so that the
	// list can be longer than the panel.
	visible := (h - 36) / render.LineHeight
	if visible < 1 {
		visible = 1
	}
	first := 0
	if p.selected >= visible {
		first = p.selected - visible + 1
	}
	for i := 0; i < visible; i++ {
		idx := first + i
		if idx >= len(menu) {
			break
		}
		entry := menu[idx]
		rowY := y + 36 + i*render.LineHeight
		colour := render.ColorText
		if !entry.Enabled {
			colour = render.ColorTextDim
		}
		if idx == p.selected {
			render.DrawSelectionMarker(canvas, x+6, rowY)
			colour = render.ColorTextBright
		}
		render.DrawText(canvas, entry.Label, x+16, rowY, colour)
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
	var quote world.Quote
	var err error
	if e.selling {
		quote, err = e.ui.world.ExecuteSell(fleetID, commodityID, e.portID, float64(e.quantity))
	} else {
		quote, err = e.ui.world.ExecuteBuy(fleetID, commodityID, e.portID, float64(e.quantity))
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

	// The answer of the merchant comes from the original wording.
	switch {
	case e.selling:
		e.ui.Message("%s", e.ui.text.Get(textGoodPrice))
	case quote.GrandTotal > 0:
		e.ui.Message("%s", e.ui.text.Get(textPriceAccepted))
	}
}

// Draw implements Popup.
func (e *ExchangePopup) Draw(canvas *render.Canvas) {
	commodities := e.ui.cfg.CommodityOrder
	w, h := 440, 260
	x := (LogicalWidth - w) / 2
	y := (LogicalHeight - h) / 2
	render.DrawShadow(canvas, x, y, w, h)
	render.DrawPanel(canvas, x, y, w, h, render.ColorPanel)

	// --- the header of the screen ---
	//
	// Every line has its own row, so that the wording of the original
	// game and the labels of this project never collide.
	mode := "BUY"
	if e.selling {
		mode = "SELL"
	}
	render.DrawText(canvas, "Exchange - "+mode, x+12, y+10, render.ColorTextBright)

	// The merchant greets the player with the original wording.
	if !e.selling {
		render.DrawText(canvas, e.ui.text.Get(textGreet), x+12, y+22, render.ColorTextDim)
	}
	render.DrawText(canvas, "tab: switch mode   enter: trade   esc: back", x+12, y+34, render.ColorTextDim)

	// The money and the quantity are shown on the right of the header.
	render.DrawTextRight(canvas,
		fmt.Sprintf("%d gold   qty %d", int(e.ui.world.Treasury()), e.quantity),
		x+w-12, y+10, render.ColorGold)

	// --- the column titles of the list ---
	const visible = 9
	listTop := y + 62
	render.DrawText(canvas, "goods", x+16, listTop-10, render.ColorTextDim)
	render.DrawTextRight(canvas, "price", x+260, listTop-10, render.ColorTextDim)
	render.DrawTextRight(canvas, "stock", x+330, listTop-10, render.ColorTextDim)
	render.DrawTextRight(canvas, "base", x+w-12, listTop-10, render.ColorTextDim)
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
		rowY := listTop + 4 + i*render.LineHeight
		price := e.ui.world.Price(cid, e.portID)
		stock := e.ui.world.Stock(cid, e.portID)

		colour := render.ColorText
		if idx == e.selected {
			render.DrawSelectionMarker(canvas, x+6, rowY)
			colour = render.ColorTextBright
		}
		render.DrawText(canvas, c.Name, x+16, rowY, colour)
		render.DrawTextRight(canvas, fmt.Sprintf("%.0f", price), x+260, rowY, render.ColorGold)
		render.DrawTextRight(canvas, fmt.Sprintf("%.0f", stock), x+330, rowY, render.ColorTextDim)
		render.DrawTextRight(canvas, fmt.Sprintf("%.0f", c.BasePrice), x+w-12, rowY, render.ColorTextDim)
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
		render.DrawText(canvas, fmt.Sprintf("total %.0f   tax %.0f", quote.Total, quote.Tax), x+12, qy, render.ColorText)
		amount := fmt.Sprintf("pay %.0f", quote.GrandTotal)
		if e.selling {
			amount = fmt.Sprintf("receive %.0f", quote.GrandTotal)
		}
		render.DrawText(canvas, amount, x+12, qy+render.LineHeight, render.ColorGold)
		if quote.Reason != "" {
			render.DrawText(canvas, quote.Reason, x+150, qy+render.LineHeight, render.ColorBad)
		}
	}
}
