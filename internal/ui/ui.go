package ui

import (
	"fmt"
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/peter/enchantedwaters/internal/ai"
	"github.com/peter/enchantedwaters/internal/asset"
	"github.com/peter/enchantedwaters/internal/audio"
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
	// art holds the pictures of the original game.
	//
	// The art is optional: when the files of the release are not present
	// the field is nil and the interface falls back on its procedural
	// drawings.
	art *asset.Art
	// portMaps holds the maps of the towns of the original game.
	portMaps []*asset.PortMap
	// chipSets holds the number of the set of tiles of every port.
	chipSets []int
	// towns caches the picture of every town that was drawn.
	//
	// A town is a grid of nine thousand two hundred and sixteen tiles, so
	// assembling it on every frame would be wasteful.
	towns map[string]*render.Sprite
	// sound holds the music and the effects of the original game.
	sound *asset.Sound
	// audio plays the sound of the original game.
	audio *audio.Player
	// musicIndex is the piece of music that is playing, or minus one when
	// nothing is playing yet.
	musicIndex int
	// musicStarted records that the music of the session has begun.
	musicStarted bool
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
	// text resolves the sentences of the interface.
	text *Text
	// canvas is the drawing surface of the interface.
	canvas *render.Canvas
	// zoom is the number of map pixels that one screen pixel covers.
	zoom float64
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
		text:         NewText(w.Config()),
		mapRenderer:  render.NewWorldMap(mapW, mapH),
		popups:       NewPopupStack(),
		followCamera: true,
		messageTicks: 180,
		zoom:         1,
	}
	u.logLines = w.LogEntries()
	return u
}

// SetArt gives the interface the pictures of the original game.
//
// The pictures are read from the files of the release by the caller, so
// that the interface itself does not know where they come from. Everything
// is optional: a missing picture leaves the interface with its procedural
// drawings.
func (u *UI) SetArt(art *asset.Art, maps []*asset.PortMap, chipSets []int) {
	u.art = art
	u.portMaps = maps
	u.chipSets = chipSets
}

// SetSound gives the interface the sound of the original game and the
// player that plays it.
//
// Everything is optional: without a sound the game is silent, and without
// the pictures of the sound the game plays nothing.
func (u *UI) SetSound(sound *asset.Sound, player *audio.Player) {
	u.sound = sound
	u.audio = player
}

// Sound returns the sound of the original game.
func (u *UI) Sound() *asset.Sound { return u.sound }

// Audio returns the player of the sound.
func (u *UI) Audio() *audio.Player { return u.audio }

// Art returns the pictures of the original game.
func (u *UI) Art() *asset.Art { return u.art }

// Portrait returns the portrait of a character of the original game.
//
// The portraits are numbered in the order of the file. The character
// templates of the configuration are numbered in their own order, so the
// number is taken modulo the number of portraits: every character gets a
// face of its own out of the set of the original game.
//
// The function returns nil when the pictures were not read.
func (u *UI) Portrait(index int) *render.Sprite {
	if u.art == nil || len(u.art.Portraits) == 0 {
		return nil
	}
	if index < 0 {
		index = -index
	}
	return u.art.Portraits[index%len(u.art.Portraits)]
}

// DrawPortrait draws the portrait of a character into the canvas.
//
// The portrait is drawn at the given position, and it is framed, because a
// face of the original game is a small picture and it needs an edge to read
// against the panel behind it.
func (u *UI) DrawPortrait(canvas *render.Canvas, index, x, y int) bool {
	portrait := u.Portrait(index)
	if portrait == nil {
		return false
	}
	render.DrawFrame(canvas, x-2, y-2, portrait.Width+4, portrait.Height+4, render.ColorFrame)
	render.DrawSpriteOpaque(canvas, portrait, x, y)
	return true
}

// PortScene returns the map of a port and the set of tiles that it is
// built from.
//
// The town of a port is the map whose number is the number of the port,
// and the tiles are the ones of the cultural group of the town. The
// function returns nil when the files of the original game were not read.
func (u *UI) PortScene(portID string) (*asset.PortMap, *asset.SpriteSet) {
	town := u.portMap(portID)
	if town == nil {
		return nil, nil
	}
	return town, u.art.PortChipSet(u.ChipSet(portID))
}

// ChipSet returns the number of the set of tiles of a port.
//
// The number comes from the table of the towns of the release. The release
// maps a hundred towns and holds a hundred and one maps, so the last port
// has no entry of its own; it is drawn out of the first set.
func (u *UI) ChipSet(portID string) int {
	index := u.portIndex(portID)
	if index < 0 || index >= len(u.chipSets) {
		return 0
	}
	return u.chipSets[index]
}

// portMap returns the map of the town of a port.
//
// The function returns nil when the files of the original game were not
// read, or when the port has no town of its own.
func (u *UI) portMap(portID string) *asset.PortMap {
	if u.art == nil || len(u.portMaps) == 0 {
		return nil
	}
	index := u.portIndex(portID)
	if index < 0 || index >= len(u.portMaps) {
		return nil
	}
	return u.portMaps[index]
}

// portIndex returns the number of a port in the order of the original
// game.
//
// The order of the ports of the configuration is the order of the original
// game: the towns come first, and the supply ports after them.
func (u *UI) portIndex(portID string) int {
	for i, id := range u.cfg.PortSourceOrder {
		if id == portID {
			return i
		}
	}
	return -1
}

// TownFor returns the picture of the town of a port.
//
// The picture is assembled on the first call and kept, so that the town of
// a port is built once and drawn on every frame after that.
//
// The function returns nil when the files of the original game were not
// read, or when the port has no town of its own.
func (u *UI) TownFor(portID string) *render.Sprite {
	if u.towns == nil {
		u.towns = make(map[string]*render.Sprite)
	}
	if town, ok := u.towns[portID]; ok {
		return town
	}
	town := u.BuildTown(portID)
	u.towns[portID] = town
	return town
}

// BuildTown assembles the picture of the town of a port out of its tiles.
//
// The picture is built once: the tiles of the town are laid out in the
// order that the map of the port gives, and the result is a picture of the
// whole town, sixteen pixels to a tile.
//
// The function returns nil when the files of the original game were not
// read, or when the port has no town of its own.
func (u *UI) BuildTown(portID string) *render.Sprite {
	town, set := u.PortScene(portID)
	if town == nil || set == nil {
		return nil
	}
	width := town.Width * TownTile
	height := town.Height * TownTile
	picture := render.NewSprite(width, height)
	// The town takes the palette of its tiles: the tiles of the ports use
	// the sixteen colours of the four plane art, not the eight colours of
	// the portraits.
	if len(set.Tiles) > 0 && set.Tiles[0] != nil {
		picture.Palette = set.Tiles[0].Palette
	} else {
		picture.Palette = u.art.Palette
	}

	for ty := 0; ty < town.Height; ty++ {
		for tx := 0; tx < town.Width; tx++ {
			index := town.Cell(tx, ty)
			if int(index) >= set.TileCount() {
				continue
			}
			tile := set.Tiles[index]
			if tile == nil {
				continue
			}
			ox, oy := tx*TownTile, ty*TownTile
			for y := 0; y < tile.Height; y++ {
				copy(picture.Pixels[(oy+y)*width+ox:(oy+y)*width+ox+tile.Width],
					tile.Pixels[y*tile.Width:(y+1)*tile.Width])
			}
		}
	}
	return picture
}

// ShowWorld centres the map on the whole world.
//
// It is used at start-up by the development screenshots and by the tests.
func (u *UI) ShowWorld() {
	w, h := u.mapRenderer.Size()
	fit := math.Max(float64(w)/LogicalWidth, float64(h)/LogicalHeight)
	u.setZoom(fit)
	u.followCamera = false
	u.camX, u.camY = 0, 0
	u.clampCamera()
}

// Canvas returns the drawing surface of the interface.
func (u *UI) Canvas() *render.Canvas { return u.canvas }

// Popups exposes the popup stack to the debug panel.
func (u *UI) Popups() *PopupStack { return u.popups }

// OpenScreen opens one of the screens of the interface at the start of
// the session.
//
// The screen is named by its action: "port", "town", "exchange",
// "shipyard", "tavern", "governor", "church", "inn", "sail" or
// "system". It is used by the development screenshots and by the automated
// tests.
func (u *UI) OpenScreen(name string) bool {
	portID := u.world.ArrivedAtPort(u.world.PlayerEntity())
	if portID == "" {
		return false
	}
	switch name {
	case "port":
		u.popups.Push(NewPortPopup(u, portID))
	case "town":
		// The town view is the town on its own, without the menu, so
		// that the whole of it can be looked at.
		town := NewTownPopup(u, portID)
		if town == nil {
			return false
		}
		u.popups.Push(town)
	case "exchange":
		u.popups.Push(NewExchangePopup(u, portID))
	case "shipyard":
		u.popups.Push(NewShipyardPopup(u, portID))
	case "tavern":
		u.popups.Push(NewTavernPopup(u, portID))
	case "governor":
		u.popups.Push(NewGovernorPopup(u, portID))
	case "church":
		u.popups.Push(NewChurchPopup(u, portID))
	case "inn":
		u.popups.Push(NewInnPopup(u, portID))
	case "sail":
		u.popups.Push(NewSailPopup(u, portID))
	case "system":
		u.popups.Push(NewSystemPopup(u))
	default:
		return false
	}
	return true
}

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
	u.updateMusic()
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
	u.camX = pos.X - LogicalWidth/2*u.zoom
	u.camY = pos.Y - LogicalHeight/2*u.zoom
	u.clampCamera()
}

// clampCamera keeps the camera inside the map bounds.
func (u *UI) clampCamera() {
	w, h := u.mapRenderer.Size()
	maxX := float64(w) - LogicalWidth*u.zoom
	maxY := float64(h) - LogicalHeight*u.zoom
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
	if u.canvas == nil {
		u.canvas = render.NewCanvas(LogicalWidth, LogicalHeight)
	}
	canvas := u.canvas

	u.drawMap(canvas)
	u.drawHUD(canvas)
	u.drawMapHints(canvas)
	u.drawStatusLine(canvas)
	if u.showLog {
		u.drawLog(canvas)
	}
	// The popups are drawn last, on top of everything.
	u.popups.Draw(canvas)

	// The canvas is blitted onto the screen in a single call.
	canvas.Present(screen)
}

// drawMap renders the world map and the fleet markers.
func (u *UI) drawMap(canvas *render.Canvas) {
	// The sea is painted first, so that the part of the screen that lies
	// outside the map is sea and not a blank band when the map is zoomed
	// out.
	canvas.Clear(render.ColorSeaDeep)

	// A ship in port shows the town it is in, which is what the player of
	// the original game sees: the quay, the houses and the harbour. The
	// town is the backdrop of every facility, so the screen does not
	// change behind the dialogue.
	if portID := u.world.ArrivedAtPort(u.world.PlayerEntity()); portID != "" {
		if u.drawTownBackdrop(canvas, portID) {
			return
		}
	}

	data := u.buildMapData()
	weatherKey := u.weatherKey()
	u.mapRenderer.Render(data, u.world.Day(), weatherKey)
	u.mapRenderer.Draw(canvas, u.camX, u.camY, u.zoom, LogicalWidth, LogicalHeight)

	// The fleet markers are drawn on top of the map.
	for _, f := range data.Fleets {
		x := (f.X - u.camX) / u.zoom
		y := (f.Y - u.camY) / u.zoom
		if x < -8 || y < -8 || x > LogicalWidth+8 || y > LogicalHeight+8 {
			continue
		}
		render.DrawFleet(canvas, x, y, f.Heading, f.Player)
	}
	// The weather and night overlays.
	//
	// The overlays are drawn on a transparent layer and then blended
	// over the map, so that they tint the map without hiding it.
	if data.WeatherTint > 0 {
		render.TintCanvas(canvas, color.RGBA{0x10, 0x18, 0x30, uint8(data.WeatherTint * 255)})
	}
	if data.NightTint > 0 {
		render.TintCanvas(canvas, color.RGBA{0x00, 0x00, 0x20, uint8(data.NightTint * 255)})
	}
}

// drawTownBackdrop draws the town of a port over the whole screen.
//
// The town is the map of the original game, and the view opens on the part
// of it that holds the houses and the roads. The function reports whether
// it drew anything: without the pictures of the original game it leaves the
// screen alone, and the caller falls back on the map.
func (u *UI) drawTownBackdrop(canvas *render.Canvas, portID string) bool {
	town := u.TownFor(portID)
	if town == nil {
		return false
	}
	rows := LogicalHeight / asset.TownTile
	offsetY := asset.PortMapFocus(u.portMap(portID), rows) * asset.TownTile
	offsetX := (town.Width - LogicalWidth) / 2
	if offsetX < 0 {
		offsetX = 0
	}
	for y := 0; y < LogicalHeight; y++ {
		for x := 0; x < LogicalWidth; x++ {
			index := town.At(offsetX+x, offsetY+y)
			if int(index) >= len(town.Palette) {
				continue
			}
			c := town.Palette[index]
			canvas.Set(x, y, colourOf(c))
		}
	}
	return true
}

// buildMapData collects the render data of the world.
func (u *UI) buildMapData() *render.MapData {
	// The land and sea mask of the world, when it has been derived.
	data := &render.MapData{Terrain: u.cfg.Terrain}
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
func (u *UI) drawHUD(canvas *render.Canvas) {
	const hudHeight = 56
	y := LogicalHeight - hudHeight
	render.DrawPanel(canvas, 0, y, LogicalWidth, hudHeight, render.ColorPanel)

	fleet := u.playerFleet()
	if fleet == nil {
		render.DrawText(canvas, "No fleet", 8, y+8, render.ColorTextDim)
		return
	}
	ship := u.cfg.Ship(fleet.ShipID)

	// --- left column: the ship and its condition ---
	name := fleet.Name
	if ship != nil {
		name = ship.Name
	}
	render.DrawText(canvas, name, 8, y+6, render.ColorTextBright)
	if ship != nil {
		ratio := fleet.HullPoints / ship.HullPoints
		render.DrawHBar(canvas, 8, y+18, 70, 5, ratio, render.ColorGood, render.ColorPanelLight)
		render.DrawText(canvas, fmt.Sprintf("Hull %d/%d", int(fleet.HullPoints), int(ship.HullPoints)), 8, y+26, render.ColorTextDim)
	}
	// The crew condition.
	render.DrawHBar(canvas, 8, y+38, 70, 5, fleet.Loyalty/100, render.ColorGold, render.ColorPanelLight)
	render.DrawText(canvas, fmt.Sprintf("Loyalty %d", int(fleet.Loyalty)), 82, y+38, render.ColorTextDim)

	// --- middle column: the cargo hold ---
	x := 170
	render.DrawText(canvas, "Hold", x, y+6, render.ColorText)
	if cargo := u.playerCargo(); cargo != nil {
		used := 0.0
		if ship != nil {
			used = cargo.UsedWeight / ship.CargoCapacity
		}
		render.DrawHBar(canvas, x, y+18, 70, 5, used, render.ColorCoast, render.ColorPanelLight)
		capacity := u.world.CargoCapacity(u.world.PlayerEntity())
		render.DrawText(canvas, fmt.Sprintf("%.0f/%.0f tons", cargo.UsedWeight, capacity), x, y+26, render.ColorTextDim)
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
				render.DrawText(canvas, fmt.Sprintf("%s %.0f", c.Name, bestQty), x, y+38, render.ColorTextDim)
			}
		}
	}

	// --- right column: the money, the date and the weather ---
	render.DrawTextRight(canvas, fmt.Sprintf("%d gold", int(u.world.Treasury())), LogicalWidth-8, y+6, render.ColorGold)
	render.DrawTextRight(canvas, fmt.Sprintf("Year %d Day %d", u.world.Year(), u.world.Day()), LogicalWidth-8, y+20, render.ColorText)
	if wc := u.world.Weather(); wc != nil {
		render.DrawTextRight(canvas, fmt.Sprintf("%s  wind %.0f kn", wc.Kind, wc.WindSpeed), LogicalWidth-8, y+34, render.ColorTextDim)
	}
	render.DrawTextRight(canvas, fmt.Sprintf("x%d", u.world.Speed()), LogicalWidth-8, y+46, render.ColorTextBright)
}

// drawStatusLine draws the transient message line.
func (u *UI) drawStatusLine(canvas *render.Canvas) {
	if u.message == "" {
		return
	}
	w := render.TextWidth(u.message) + 12
	render.DrawShadow(canvas, 8, 8, w, 16)
	render.DrawPanel(canvas, 8, 8, w, 16, render.ColorPanelLight)
	render.DrawText(canvas, u.message, 14, 13, render.ColorTextBright)
}

// drawLog draws the message log overlay.
func (u *UI) drawLog(canvas *render.Canvas) {
	lines := u.world.LogEntries()
	if len(lines) == 0 {
		return
	}
	const maxLines = 12
	w := 400
	h := maxLines*render.LineHeight + 12
	x := LogicalWidth - w - 8
	y := 8
	render.DrawShadow(canvas, x, y, w, h)
	render.DrawPanel(canvas, x, y, w, h, render.ColorPanel)
	for i := 0; i < maxLines && i < len(lines); i++ {
		idx := len(lines) - maxLines + i
		if idx < 0 {
			continue
		}
		render.DrawText(canvas, lines[idx], x+6, y+6+i*render.LineHeight, render.ColorTextDim)
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
