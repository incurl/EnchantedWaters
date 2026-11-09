package ui

import (
	"fmt"
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/ai"
	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
	"github.com/peter/enchantedwaters/internal/render"
	"github.com/peter/enchantedwaters/internal/world"
)

// overlayDim is the colour used to dim the inactive popups.
var overlayDim = color.RGBA{0, 0, 0, 0x70}

// LogicalWidth and LogicalHeight are the dimensions of the virtual screen.
//
// The window is scaled by an integer factor, which preserves the pixel
// perfect look of the original game.
const (
	LogicalWidth  = 640
	LogicalHeight = 400
)

// UI is the root of the user interface.
type UI struct {
	// world is the simulation the UI reads from and acts on.
	world *world.World
	// cfg is the immutable configuration registry.
	cfg *config.Registry
	// mapRenderer draws the world map.
	mapRenderer *render.WorldMap
	// popups holds the open modal dialogs.
	popups *PopupStack
	// camX / camY are the camera position in map pixels.
	camX, camY float64
	// followCamera keeps the camera centred on the player's fleet.
	followCamera bool
	// message is the transient status line.
	message string
	// messageTicks is the remaining lifetime of the status line.
	messageTicks int
	// logLines are the recent simulation messages.
	logLines []string
	// selectedPort is the port highlighted in the port list.
	selectedPort int
	// showLog toggles the message log overlay.
	showLog bool
	// selectedCommodity is the index of the highlighted commodity.
	selectedCommodity int
	// tradeMode is true when the exchange is in "sell" mode.
	tradeMode bool
	// tradeQuantity is the quantity to trade.
	tradeQuantity int
	// aiController exposes the AI for the debug panel.
	aiController *ai.Controller
	// charIDs caches the deterministic list of the character template IDs.
	charIDs []string
}

// New creates the user interface of a game session.
func New(w *world.World) *UI {
	params := w.Config().Params
	mapW, mapH := params.MapWidth, params.MapHeight
	if mapW <= 0 {
		mapW = 1024
	}
	if mapH <= 0 {
		mapH = 640
	}
	u := &UI{
		world:        w,
		cfg:          w.Config(),
		mapRenderer:  render.NewWorldMap(mapW, mapH),
		popups:       NewPopupStack(),
		followCamera: true,
		messageTicks: 180,
	}
	u.logLines = w.LogEntries()
	return u
}

// Popups exposes the popup stack to the debug panel.
func (u *UI) Popups() *PopupStack { return u.popups }

// Message sets the transient status line.
func (u *UI) Message(format string, args ...any) {
	u.message = fmt.Sprintf(format, args...)
	u.messageTicks = 180
}

// Update handles the input and advances the UI state.
func (u *UI) Update() error {
	// A modal popup captures the input.
	if u.popups.Len() > 0 {
		u.popups.Update()
		return nil
	}

	u.handleGlobalKeys()

	// The player fleet must exist for the map interaction to work.
	fleet := u.playerFleet()
	if fleet == nil {
		return nil
	}

	u.handleMapKeys()
	u.handleActionKeys()
	u.handleSpeedKeys()
	u.updateCamera()
	u.messageTicks--
	if u.messageTicks < 0 {
		u.message = ""
	}
	return nil
}

// handleGlobalKeys handles the keys that work everywhere.
func (u *UI) handleGlobalKeys() {
	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		u.showLog = !u.showLog
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if u.popups.Len() > 0 {
			u.popups.PopAll()
		} else {
			u.showLog = !u.showLog
		}
	}
}

// handleMapKeys handles the panning of the map.
func (u *UI) handleMapKeys() {
	const panSpeed = 6.0
	dx, dy := 0.0, 0.0
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		dx = -panSpeed
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		dx = panSpeed
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		dy = -panSpeed
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		dy = panSpeed
	}
	if dx != 0 || dy != 0 {
		u.followCamera = false
		u.camX += dx
		u.camY += dy
		u.clampCamera()
	}
}

// handleSpeedKeys handles the simulation speed control.
func (u *UI) handleSpeedKeys() {
	if inpututil.IsKeyJustPressed(ebiten.KeyDigit1) {
		u.world.SetSpeed(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDigit2) {
		u.world.SetSpeed(2)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDigit3) {
		u.world.SetSpeed(4)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDigit4) {
		u.world.SetSpeed(8)
	}
}

// updateCamera keeps the camera centred on the player's fleet when the
// follow mode is active.
func (u *UI) updateCamera() {
	if !u.followCamera {
		return
	}
	pos := u.playerPosition()
	if pos == nil {
		return
	}
	u.camX = pos.X - LogicalWidth/2
	u.camY = pos.Y - LogicalHeight/2
	u.clampCamera()
}

// clampCamera keeps the camera inside the map bounds.
func (u *UI) clampCamera() {
	w, h := u.mapRenderer.Size()
	maxX := float64(w - LogicalWidth)
	maxY := float64(h - LogicalHeight)
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	if u.camX < 0 {
		u.camX = 0
	}
	if u.camY < 0 {
		u.camY = 0
	}
	if u.camX > maxX {
		u.camX = maxX
	}
	if u.camY > maxY {
		u.camY = maxY
	}
}

// Draw renders the whole user interface.
func (u *UI) Draw(screen *ebiten.Image) {
	u.drawMap(screen)
	u.drawHUD(screen)
	u.drawMapHints(screen)
	u.drawStatusLine(screen)
	if u.showLog {
		u.drawLog(screen)
	}
	// The popups are drawn last, on top of everything.
	u.popups.Draw(screen)
}

// drawMap renders the world map and the fleet markers.
func (u *UI) drawMap(screen *ebiten.Image) {
	data := u.buildMapData()
	weatherKey := u.weatherKey()
	u.mapRenderer.Render(data, u.world.Day(), weatherKey)
	u.mapRenderer.Draw(screen, u.camX, u.camY, LogicalWidth, LogicalHeight)

	// The fleet markers are drawn on top of the map.
	for _, f := range data.Fleets {
		x := f.X - u.camX
		y := f.Y - u.camY
		if x < -8 || y < -8 || x > LogicalWidth+8 || y > LogicalHeight+8 {
			continue
		}
		render.DrawFleet(screen, x, y, f.Heading, f.Player)
	}
	// The weather and night overlays.
	//
	// The overlays are drawn on a transparent layer and then blended
	// over the map, so that they tint the map without hiding it.
	if data.WeatherTint > 0 {
		overlay := ebiten.NewImage(LogicalWidth, LogicalHeight)
		overlay.Fill(color.RGBA{0x10, 0x18, 0x30, uint8(data.WeatherTint * 255)})
		screen.DrawImage(overlay, nil)
	}
	if data.NightTint > 0 {
		overlay := ebiten.NewImage(LogicalWidth, LogicalHeight)
		overlay.Fill(color.RGBA{0x00, 0x00, 0x20, uint8(data.NightTint * 255)})
		screen.DrawImage(overlay, nil)
	}
}

// buildMapData collects the render data of the world.
func (u *UI) buildMapData() *render.MapData {
	data := &render.MapData{}
	for _, id := range u.cfg.PortOrder {
		p := u.cfg.Port(id)
		if p == nil {
			continue
		}
		marker := render.PortMarker{X: p.X, Y: p.Y}
		switch p.Class {
		case config.PortCapital:
			marker.Capital = true
		case config.PortCity:
			marker.City = true
		}
		data.Ports = append(data.Ports, marker)
	}
	for id := range u.cfg.SeaZones {
		z := u.cfg.SeaZones[id]
		if z == nil {
			continue
		}
		data.SeaZones = append(data.SeaZones, render.ZoneMarker{X: z.CenterX, Y: z.CenterY, Radius: z.Radius})
	}
	// The fleets.
	fleetStore := ecs.ComponentArrayOf[ecs.FleetComponent](u.world.ECS())
	if fleetStore != nil {
		fleetStore.Each(func(id ecs.ID, f *ecs.FleetComponent) {
			pos := ecs.GetComponent[ecs.PositionComponent](u.world.ECS(), id)
			if pos == nil {
				return
			}
			data.Fleets = append(data.Fleets, render.FleetMarker{
				X: pos.X, Y: pos.Y, Heading: pos.Heading, Player: f.IsPlayer,
			})
		})
	}
	// The weather tint.
	if wc := u.world.Weather(); wc != nil {
		switch wc.Kind {
		case string(config.WeatherStorm):
			data.WeatherTint = 0.35
		case string(config.WeatherRain):
			data.WeatherTint = 0.18
		case string(config.WeatherFog):
			data.WeatherTint = 0.12
		}
	}
	// The night overlay follows the day fraction of the session.
	frac := u.world.DayFraction()
	if frac > 0.75 {
		data.NightTint = (frac - 0.75) * 1.2
	} else if frac < 0.25 {
		data.NightTint = (0.25 - frac) * 1.2
	}
	return data
}

// weatherKey identifies the current weather for the map cache.
func (u *UI) weatherKey() string {
	wc := u.world.Weather()
	if wc == nil {
		return ""
	}
	return wc.TemplateID
}

// drawHUD draws the status bar at the bottom of the screen.
func (u *UI) drawHUD(screen *ebiten.Image) {
	const hudHeight = 56
	y := LogicalHeight - hudHeight
	render.DrawPanel(screen, 0, y, LogicalWidth, hudHeight, render.ColorPanel)

	fleet := u.playerFleet()
	if fleet == nil {
		render.DrawText(screen, "No fleet", 8, y+8, render.ColorTextDim)
		return
	}
	ship := u.cfg.Ship(fleet.ShipID)

	// --- left column: the ship and its condition ---
	name := fleet.Name
	if ship != nil {
		name = ship.Name
	}
	render.DrawText(screen, name, 8, y+6, render.ColorTextBright)
	if ship != nil {
		ratio := fleet.HullPoints / ship.HullPoints
		render.DrawHBar(screen, 8, y+18, 70, 5, ratio, render.ColorGood, render.ColorPanelLight)
		render.DrawText(screen, fmt.Sprintf("Hull %d/%d", int(fleet.HullPoints), int(ship.HullPoints)), 8, y+26, render.ColorTextDim)
	}
	// The crew condition.
	render.DrawHBar(screen, 8, y+38, 70, 5, fleet.Loyalty/100, render.ColorGold, render.ColorPanelLight)
	render.DrawText(screen, fmt.Sprintf("Loyalty %d", int(fleet.Loyalty)), 82, y+38, render.ColorTextDim)

	// --- middle column: the cargo hold ---
	x := 170
	render.DrawText(screen, "Hold", x, y+6, render.ColorText)
	if cargo := u.playerCargo(); cargo != nil {
		used := 0.0
		if ship != nil {
			used = cargo.UsedWeight / ship.CargoCapacity
		}
		render.DrawHBar(screen, x, y+18, 70, 5, used, render.ColorCoast, render.ColorPanelLight)
		capacity := u.world.CargoCapacity(u.world.PlayerEntity())
		render.DrawText(screen, fmt.Sprintf("%.0f/%.0f tons", cargo.UsedWeight, capacity), x, y+26, render.ColorTextDim)
		// The most valuable cargo.
		best := ""
		bestQty := 0.0
		for cid, qty := range cargo.Manifest {
			if qty > bestQty {
				best, bestQty = cid, qty
			}
		}
		if best != "" {
			if c := u.cfg.Commodity(best); c != nil {
				render.DrawText(screen, fmt.Sprintf("%s %.0f", c.Name, bestQty), x, y+38, render.ColorTextDim)
			}
		}
	}

	// --- right column: the money, the date and the weather ---
	render.DrawTextRight(screen, fmt.Sprintf("%d gold", int(u.world.Treasury())), LogicalWidth-8, y+6, render.ColorGold)
	render.DrawTextRight(screen, fmt.Sprintf("Year %d Day %d", u.world.Year(), u.world.Day()), LogicalWidth-8, y+20, render.ColorText)
	if wc := u.world.Weather(); wc != nil {
		render.DrawTextRight(screen, fmt.Sprintf("%s  wind %.0f kn", wc.Kind, wc.WindSpeed), LogicalWidth-8, y+34, render.ColorTextDim)
	}
	render.DrawTextRight(screen, fmt.Sprintf("x%d", u.world.Speed()), LogicalWidth-8, y+46, render.ColorTextBright)
}

// drawStatusLine draws the transient message line.
func (u *UI) drawStatusLine(screen *ebiten.Image) {
	if u.message == "" {
		return
	}
	w := render.TextWidth(u.message) + 12
	render.DrawShadow(screen, 8, 8, w, 16)
	render.DrawPanel(screen, 8, 8, w, 16, render.ColorPanelLight)
	render.DrawText(screen, u.message, 14, 13, render.ColorTextBright)
}

// drawLog draws the message log overlay.
func (u *UI) drawLog(screen *ebiten.Image) {
	lines := u.world.LogEntries()
	if len(lines) == 0 {
		return
	}
	const maxLines = 12
	w := 400
	h := maxLines*render.LineHeight + 12
	x := LogicalWidth - w - 8
	y := 8
	render.DrawShadow(screen, x, y, w, h)
	render.DrawPanel(screen, x, y, w, h, render.ColorPanel)
	for i := 0; i < maxLines && i < len(lines); i++ {
		idx := len(lines) - maxLines + i
		if idx < 0 {
			continue
		}
		render.DrawText(screen, lines[idx], x+6, y+6+i*render.LineHeight, render.ColorTextDim)
	}
}

// playerFleet returns the fleet component of the player, or nil.
func (u *UI) playerFleet() *ecs.FleetComponent {
	id := u.world.PlayerEntity()
	if id == 0 {
		return nil
	}
	return ecs.GetComponent[ecs.FleetComponent](u.world.ECS(), id)
}

// playerCargo returns the cargo component of the player, or nil.
func (u *UI) playerCargo() *ecs.CargoComponent {
	id := u.world.PlayerEntity()
	if id == 0 {
		return nil
	}
	return ecs.GetComponent[ecs.CargoComponent](u.world.ECS(), id)
}

// playerPosition returns the position component of the player, or nil.
func (u *UI) playerPosition() *ecs.PositionComponent {
	id := u.world.PlayerEntity()
	if id == 0 {
		return nil
	}
	return ecs.GetComponent[ecs.PositionComponent](u.world.ECS(), id)
}

// formatGold renders an amount of money.
func formatGold(v float64) string {
	return strconv.FormatInt(int64(v), 10)
}

// formatCoord renders a map coordinate.
func formatCoord(v float64) string {
	return strconv.FormatInt(int64(math.Round(v)), 10)
}
