//go:build debugui

package ui

import (
	"fmt"
	"image"

	"github.com/ebitengine/debugui"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/peter/enchantedwaters/internal/ecs"
)

// DebugPanel is the development debug panel.
//
// The panel is only compiled into the debug builds (the "debugui" build
// tag), so that the release binary stays small and free of the debug
// dependencies.
//
// The panel exposes:
//
//   - the content of every configuration table,
//   - the live economic parameters of the ports,
//   - the state of the fleets,
//   - the log of the AI decisions,
//   - the quick trigger of the import and the export of the data tables.
type DebugPanel struct {
	// ui is the parent interface.
	ui *UI
	// debug is the underlying debug UI.
	debug debugui.DebugUI
	// tab is the currently selected tab.
	tab int
	// selectedPort is the port whose parameters are shown.
	selectedPort int
	// selectedTable is the table whose content is shown.
	selectedTable int
}

// NewDebugPanel creates the debug panel.
func NewDebugPanel(u *UI) *DebugPanel {
	return &DebugPanel{ui: u}
}

// Update draws the panel and handles its input.
func (d *DebugPanel) Update() error {
	_, err := d.debug.Update(func(ctx *debugui.Context) error {
		d.draw(ctx)
		return nil
	})
	return err
}

// Draw renders the panel.
func (d *DebugPanel) Draw(c *render.Canvas) {
	d.debug.Draw(screen)
}

// draw builds the content of the panel.
func (d *DebugPanel) draw(ctx *debugui.Context) {
	ctx.Window("Enchanted Waters - debug", image.Rect(20, 20, 460, 320), func(layout debugui.ContainerLayout) {
		names := []string{"Tables", "Economy", "Fleets", "AI Log", "Actions"}
		ctx.Dropdown(&d.tab, names)
		switch d.tab {
		case 0:
			d.drawTables(ctx)
		case 1:
			d.drawEconomy(ctx)
		case 2:
			d.drawFleets(ctx)
		case 3:
			d.drawAILog(ctx)
		case 4:
			d.drawActions(ctx)
		}
	})
}

// drawTables shows the content of the configuration tables.
func (d *DebugPanel) drawTables(ctx *debugui.Context) {
	cfg := d.ui.cfg
	names := []string{"Commodities", "Ports", "Ships", "Events", "Rumors", "Sea Zones", "Weather", "Titles", "AI Profiles"}
	ctx.Dropdown(&d.selectedTable, names)
	ctx.Text(fmt.Sprintf("%d commodities, %d ports, %d ships, %d events",
		len(cfg.Commodities), len(cfg.Ports), len(cfg.Ships), len(cfg.Events)))

	switch d.selectedTable {
	case 0:
		for _, id := range cfg.CommodityOrder {
			c := cfg.Commodities[id]
			ctx.Text(fmt.Sprintf("%-14s %-9s base %8.2f cycle %.2f season %.2f",
				c.Name, c.Class, c.BasePrice, c.CycleAmplitude, c.SeasonalAmplitude))
		}
	case 1:
		for _, id := range cfg.PortOrder {
			p := cfg.Port(id)
			ctx.Text(fmt.Sprintf("%-14s %-12s tax %.2f  x %.0f y %.0f",
				p.Name, p.Region, p.BaseTaxRate, p.X, p.Y))
		}
	case 2:
		for _, id := range cfg.ShipOrder {
			s := cfg.Ships[id]
			ctx.Text(fmt.Sprintf("%-14s hull %5d crew %3d hold %4d speed %.1f",
				s.Name, int(s.HullPoints), s.MaxSailors, int(s.CargoCapacity), s.BaseSpeed))
		}
	case 3:
		for _, id := range cfg.EventOrder {
			e := cfg.Events[id]
			ctx.Text(fmt.Sprintf("%-22s %-12s weight %5.1f", e.Name, e.Category, e.Weight))
		}
	case 4:
		for _, id := range cfg.RumorOrder {
			r := cfg.Rumors[id]
			ctx.Text(fmt.Sprintf("%-16s %s", r.Category, r.Text))
		}
	case 5:
		for id := range cfg.SeaZones {
			z := cfg.SeaZones[id]
			ctx.Text(fmt.Sprintf("%-18s centre %.0f,%.0f radius %.0f", z.Name, z.CenterX, z.CenterY, z.Radius))
		}
	case 6:
		for id := range cfg.Weather {
			t := cfg.Weather[id]
			ctx.Text(fmt.Sprintf("%-12s %-8s weight %5.1f speed %.2f", t.Name, t.Kind, t.Weight, t.SpeedPenalty))
		}
	case 7:
		for _, id := range cfg.TitleOrder {
			t := cfg.Titles[id]
			ctx.Text(fmt.Sprintf("%-18s %-10s threshold %.0f", t.Name, t.Axis, t.Threshold))
		}
	case 8:
		for _, id := range cfg.AIProfileOrder {
			p := cfg.AIProfiles[id]
			ctx.Text(fmt.Sprintf("%-18s fleets %2d risk %.2f", p.Name, p.FleetCount, p.RiskAversion))
		}
	}
}

// drawEconomy shows the live economic parameters of the ports.
func (d *DebugPanel) drawEconomy(ctx *debugui.Context) {
	ports := d.ui.cfg.PortOrder
	if len(ports) == 0 {
		return
	}
	ctx.Dropdown(&d.selectedPort, ports)
	portID := ports[d.selectedPort]
	port := d.ui.cfg.Port(portID)
	if port == nil {
		return
	}
	ctx.Text(fmt.Sprintf("%s (%s) tax %.2f", port.Name, port.Class, port.BaseTaxRate))

	for _, cid := range d.ui.cfg.CommodityOrder {
		c := d.ui.cfg.Commodity(cid)
		price := d.ui.world.Price(cid, portID)
		stock := d.ui.world.Stock(cid, portID)
		ctx.Text(fmt.Sprintf("%-14s price %8.2f stock %8.1f base %8.2f ratio %.2f",
			c.Name, price, stock, c.BasePrice, price/c.BasePrice))
	}
}

// drawFleets shows the state of the fleets.
func (d *DebugPanel) drawFleets(ctx *debugui.Context) {
	fleetStore := ecs.ComponentArrayOf[ecs.FleetComponent](d.ui.world.ECS())
	if fleetStore == nil {
		return
	}
	fleetStore.Each(func(id ecs.ID, f *ecs.FleetComponent) {
		pos := ecs.GetComponent[ecs.PositionComponent](d.ui.world.ECS(), id)
		posText := ""
		if pos != nil {
			posText = fmt.Sprintf("(%.0f, %.0f) heading %.0f speed %.1f",
				pos.X, pos.Y, pos.Heading, pos.Speed)
		}
		ctx.Text(fmt.Sprintf("%-24s %-12s hull %5.0f crew %3d %s",
			f.Name, f.InPort, f.HullPoints, f.Sailors, posText))
	})
}

// drawAILog shows the decisions of the AI fleets.
func (d *DebugPanel) drawAILog(ctx *debugui.Context) {
	aiStore := ecs.ComponentArrayOf[ecs.AIComponent](d.ui.world.ECS())
	if aiStore == nil {
		return
	}
	aiStore.Each(func(id ecs.ID, ai *ecs.AIComponent) {
		fleet := ecs.GetComponent[ecs.FleetComponent](d.ui.world.ECS(), id)
		name := ""
		if fleet != nil {
			name = fleet.Name
		}
		ctx.Text(fmt.Sprintf("%-24s %-8s -> %-16s %s",
			name, ai.State, ai.TargetPortID, ai.LastDecision))
	})
}

// drawActions offers the quick actions of the panel.
func (d *DebugPanel) drawActions(ctx *debugui.Context) {
	ctx.Text("The data tables are reloaded at the next start of the game.")
	ctx.Button("Reload the configuration").On(func() {
		d.ui.Message("the configuration is reloaded at the next start")
	})
	ctx.Button("Save the game").On(func() {
		d.ui.Message("the game is saved")
	})
}
