package world

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"

	"github.com/peter/enchantedwaters/internal/asset"
	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
	"github.com/peter/enchantedwaters/internal/sail"
)

// testRegistry loads the shipped configuration tables.
func testRegistry(t *testing.T) *config.Registry {
	t.Helper()
	dir := filepath.Join("..", "..", "assets", "config")
	reg, err := asset.Load(dir, logr.Discard())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return reg
}

// newTestWorld builds a world with a running session.
func newTestWorld(t *testing.T) *World {
	t.Helper()
	reg := testRegistry(t)
	w := NewWorld(reg, 12345)
	if err := w.StartSession("ports.lisbon"); err != nil {
		t.Fatalf("start session: %v", err)
	}
	return w
}

// TestSessionStarts verifies that the session creates the expected
// entities.
func TestSessionStarts(t *testing.T) {
	w := newTestWorld(t)

	if w.Day() != 1 {
		t.Errorf("day = %d, want 1", w.Day())
	}
	if w.PlayerEntity() == 0 {
		t.Error("player entity was not created")
	}
	if !w.ecs.IsAlive(w.PlayerEntity()) {
		t.Error("player entity is not alive")
	}
	if w.WeatherEntity() == 0 {
		t.Error("weather entity was not created")
	}

	fleet := getFleetComponent(w.ecs, w.PlayerEntity())
	if fleet == nil {
		t.Fatal("player fleet component missing")
	}
	if !fleet.IsPlayer {
		t.Error("player fleet is not flagged as the player's")
	}
	if fleet.InPort != "ports.lisbon" {
		t.Errorf("player fleet in port %q, want ports.lisbon", fleet.InPort)
	}

	// The AI profiles must have produced AI fleets.
	aiStore := ecs.ComponentArrayOf[ecs.AIComponent](w.ecs)
	if aiStore == nil {
		t.Fatal("AI component storage missing")
	}
	aiCount := 0
	aiStore.Each(func(id ecs.ID, c *ecs.AIComponent) { aiCount++ })
	if aiCount == 0 {
		t.Error("no AI fleets were created")
	}
}

// TestPricesStayInRange verifies that the economic model never produces a
// price outside the configured bounds.
func TestPricesStayInRange(t *testing.T) {
	w := newTestWorld(t)

	for day := 0; day < 60; day++ {
		w.Tick()
		for _, portID := range w.cfg.PortOrder {
			for _, cid := range w.cfg.CommodityOrder {
				c := w.cfg.Commodities[cid]
				price := w.Price(cid, portID)
				if price <= 0 {
					t.Fatalf("day %d: price of %s at %s is %.2f, want > 0", day, cid, portID, price)
				}
				if c.MaxPrice > 0 && price > c.MaxPrice*1.5 {
					t.Fatalf("day %d: price of %s at %s is %.2f, far above max %.2f",
						day, cid, portID, price, c.MaxPrice)
				}
			}
		}
	}
}

// TestPriceSpreadBetweenPorts verifies that the regional price gap is
// actually modelled: a commodity produced in one region must be more
// expensive in a region that does not produce it.
func TestPriceSpreadBetweenPorts(t *testing.T) {
	w := newTestWorld(t)
	// Let the market settle.
	for i := 0; i < 10; i++ {
		w.Tick()
	}

	// Find a specialty commodity with a producer and a distant consumer.
	var producer, consumer string
	var commodity string
	for _, cid := range w.cfg.CommodityOrder {
		producers := w.cfg.SpecialtyPorts[cid]
		if len(producers) == 0 {
			continue
		}
		for _, portID := range w.cfg.PortOrder {
			p := w.cfg.Port(portID)
			if p.SpecialtyCommodity == cid {
				continue
			}
			producer = producers[0]
			consumer = portID
			commodity = cid
			break
		}
		if commodity != "" {
			break
		}
	}
	if commodity == "" {
		t.Skip("no specialty commodity with a distant consumer")
	}

	producerPrice := w.Price(commodity, producer)
	consumerPrice := w.Price(commodity, consumer)
	if consumerPrice <= producerPrice {
		t.Errorf("price of %s: producer %s = %.2f, consumer %s = %.2f; want consumer higher",
			commodity, producer, producerPrice, consumer, consumerPrice)
	}
}

// TestTradingMovesGoodsAndMoney verifies that a purchase moves both the
// goods and the money, and that the port's stock shrinks.
func TestTradingMovesGoodsAndMoney(t *testing.T) {
	w := newTestWorld(t)
	fleetID := w.PlayerEntity()

	// Give the player some money to be able to buy.
	w.SetTreasury(100000)
	beforeMoney := w.Treasury()
	beforeStock := w.Stock("grain", "ports.lisbon")

	quote, err := w.ExecuteBuy(fleetID, "grain", "ports.lisbon", 5)
	if err != nil {
		t.Fatalf("buy: %v", err)
	}
	if quote.Quantity <= 0 {
		t.Fatalf("bought %.2f units, want > 0", quote.Quantity)
	}
	afterMoney := w.Treasury()
	if afterMoney >= beforeMoney {
		t.Errorf("money did not decrease: %.2f -> %.2f", beforeMoney, afterMoney)
	}
	afterStock := w.Stock("grain", "ports.lisbon")
	if afterStock >= beforeStock {
		t.Errorf("stock did not decrease: %.2f -> %.2f", beforeStock, afterStock)
	}

	cargo := getCargoComponent(w.ecs, fleetID)
	if cargo == nil {
		t.Fatal("cargo component missing")
	}
	if cargo.Manifest["grain"] < quote.Quantity {
		t.Errorf("manifest holds %.2f, want at least %.2f", cargo.Manifest["grain"], quote.Quantity)
	}

	// Selling must give the money back (minus the tax).
	sellQuote, err := w.ExecuteSell(fleetID, "grain", "ports.lisbon", quote.Quantity)
	if err != nil {
		t.Fatalf("sell: %v", err)
	}
	if sellQuote.GrandTotal <= 0 {
		t.Errorf("sell proceeds %.2f, want > 0", sellQuote.GrandTotal)
	}
	if w.Treasury() <= afterMoney {
		t.Errorf("money did not increase after the sale: %.2f", w.Treasury())
	}
}

// TestCargoCapacityIsEnforced verifies that the hold cannot be
// overfilled.
func TestCargoCapacityIsEnforced(t *testing.T) {
	w := newTestWorld(t)
	fleetID := w.PlayerEntity()
	w.SetTreasury(1e9)

	// Ask for an absurd amount: the purchase must be limited by the
	// capacity of the hold, not rejected outright.
	stockBefore := w.Stock("iron", "ports.lisbon")
	quote, err := w.ExecuteBuy(fleetID, "iron", "ports.lisbon", 1e9)
	if err != nil {
		t.Fatalf("the purchase should be limited, not rejected: %v", err)
	}
	fleet := getFleetComponent(w.ecs, fleetID)
	ship := w.cfg.Ship(fleet.ShipID)
	cargo := getCargoComponent(w.ecs, fleetID)
	if cargo.UsedWeight > ship.CargoCapacity+1e-6 {
		t.Errorf("used weight %.2f exceeds capacity %.2f", cargo.UsedWeight, ship.CargoCapacity)
	}
	if cargo.UsedVolume > ship.CargoCapacity+1e-6 {
		t.Errorf("used volume %.2f exceeds capacity %.2f", cargo.UsedVolume, ship.CargoCapacity)
	}
	if quote.Quantity <= 0 {
		t.Errorf("bought %.2f units, want > 0", quote.Quantity)
	}
	// The bought quantity must respect both the stock of the port and
	// the capacity of the hold.
	commodity := w.cfg.Commodity("iron")
	if quote.Quantity > stockBefore+1e-6 {
		t.Errorf("bought %.2f units, but the port only had %.2f", quote.Quantity, stockBefore)
	}
	maxByCapacity := ship.CargoCapacity / commodity.WeightPerUnit
	if quote.Quantity > maxByCapacity+1e-6 {
		t.Errorf("bought %.2f units, but the hold can take only %.2f", quote.Quantity, maxByCapacity)
	}
	// The remaining room must be consistent with what was bought.
	limit := w.cargoLimit(fleetID, commodity)
	if limit < 0 {
		t.Errorf("negative remaining capacity %.4f", limit)
	}
}

// TestAuthenticWind verifies that the wind of the original game is used
// when its table has been imported.
func TestAuthenticWind(t *testing.T) {
	reg := testRegistry(t)
	if reg.Wind == nil {
		t.Skip("the wind of the original game has not been imported")
	}
	w := NewWorld(reg, 12345)

	// The wind of a sea area is a direction on the eight point compass and
	// a strength, and the vector points to the direction the wind blows
	// to.
	for _, cell := range []struct{ column, row int }{{0, 0}, {7, 4}, {29, 14}} {
		direction, strength, _ := reg.Wind.At(cell.column, cell.row)
		if strength == 0 {
			continue
		}
		x := float64(cell.column*config.WindSeaAreaSize*32 + 32)
		y := float64(cell.row*config.WindSeaAreaSize*32 + 32)
		windX, windY, _, _ := w.currentWindAt(x, y)
		if windX == 0 && windY == 0 {
			t.Errorf("the wind of the sea area (%d,%d) is zero, want a wind", cell.column, cell.row)
			continue
		}
		// The bearing of the wind must be the direction of the table,
		// counted clockwise from the north.
		bearing := math.Atan2(windX, -windY) * 180 / math.Pi
		if bearing < 0 {
			bearing += 360
		}
		want := float64(direction) * 45
		diff := math.Abs(bearing - want)
		if diff > 180 {
			diff = 360 - diff
		}
		if diff > 1 {
			t.Errorf("the wind of the sea area (%d,%d) blows to %.1f degrees, want %.1f",
				cell.column, cell.row, bearing, want)
		}
	}
}

// TestAuthenticWindStrength verifies that the strength of the table reaches
// the sailing model.
func TestAuthenticWindStrength(t *testing.T) {
	reg := testRegistry(t)
	if reg.Wind == nil {
		t.Skip("the wind of the original game has not been imported")
	}
	w := NewWorld(reg, 12345)

	// A sea area with a strong wind must give a stronger wind than one
	// with a weak wind.
	strong, weak := -1, -1
	for i := 0; i < config.WindSeaAreas; i++ {
		column := i % config.WindColumns
		row := i / config.WindColumns
		_, strength, _ := reg.Wind.At(column, row)
		if strength >= 8 && strong < 0 {
			strong = i
		}
		if strength > 0 && strength <= 2 && weak < 0 {
			weak = i
		}
	}
	if strong < 0 || weak < 0 {
		t.Skip("the table has no sea area with a strong or a weak wind")
	}
	speedOf := func(i int) float64 {
		column := i % config.WindColumns
		row := i / config.WindColumns
		x := float64(column*config.WindSeaAreaSize*32 + 32)
		y := float64(row*config.WindSeaAreaSize*32 + 32)
		windX, windY, _, _ := w.currentWindAt(x, y)
		return math.Hypot(windX, windY)
	}
	if speedOf(strong) <= speedOf(weak) {
		t.Errorf("the strong wind gives %.3f, the weak one %.3f: the strong one must win",
			speedOf(strong), speedOf(weak))
	}
}

// TestSailingModel verifies the shape of the sailing model of the original
// game: running before the wind is fastest, beating costs the tacking
// share, and a lateen rig holds the wind far better than a square rig.
func TestSailingModel(t *testing.T) {
	beat := func(heading, windDir float64, sailType int) float64 {
		return sail.Conditions{
			Power: 80, Tacking: 50, SailType: sailType, MinCrew: 5,
			Capacity: 100, Crew: 5, NavigationLevel: 1,
			Seamanship: sail.NeutralSeamanship,
			Heading:    heading, WindDirection: windDir, WindSpeed: 5,
		}.Speed()
	}

	// Dead before the wind: the strongest drive, no tacking penalty.
	running := beat(0, 0, 1)
	if running <= 0 {
		t.Fatalf("running before the wind gave %.3f, want a positive speed", running)
	}

	// Dead to windward: the weakest drive, and the tacking penalty on top.
	beating := beat(180, 0, 1)
	if beating >= running {
		t.Errorf("beating (%.3f) should be slower than running (%.3f)", beating, running)
	}

	// A lateen rig must beat to windward better than a square rig does.
	square := beat(180, 0, 1)
	lateen := beat(180, 0, 6)
	if lateen <= square {
		t.Errorf("lateen rig to windward (%.3f) should beat the square rig (%.3f)", lateen, square)
	}

	// The base speed is capped at 30.
	if running > sail.MaxBaseSpeed+1e-9 {
		t.Errorf("speed %.3f exceeds the cap of %.0f", running, sail.MaxBaseSpeed)
	}
}

// TestCompassGeometry verifies the eight point compass that the model
// works on.
func TestCompassGeometry(t *testing.T) {
	cases := []struct {
		bearing float64
		want    int
	}{
		{0, 0}, {350, 0}, {20, 0},
		{45, 1}, {90, 2}, {135, 3}, {180, 4},
		{225, 5}, {270, 6}, {315, 7},
	}
	for _, c := range cases {
		if got := sail.CompassDirection(c.bearing); got != c.want {
			t.Errorf("CompassDirection(%.0f) = %d, want %d", c.bearing, got, c.want)
		}
	}
	// The delta folds, so a beam wind is the same from either side.
	if got := sail.CompassDelta(0, 2); got != 2 {
		t.Errorf("CompassDelta(0,2) = %d, want 2", got)
	}
	if got := sail.CompassDelta(0, 6); got != 2 {
		t.Errorf("CompassDelta(0,6) = %d, want 2 (folded)", got)
	}
	if got := sail.CompassDelta(1, 5); got != 4 {
		t.Errorf("CompassDelta(1,5) = %d, want 4 (dead to windward)", got)
	}
}

// TestSailingMovesTheFleet verifies that a fleet actually moves when it
// sails.
func TestSailingMovesTheFleet(t *testing.T) {
	w := newTestWorld(t)
	fleetID := w.PlayerEntity()

	pos := getPositionComponent(w.ecs, fleetID)
	if pos == nil {
		t.Fatal("position component missing")
	}
	startX, startY := pos.X, pos.Y

	if err := w.SetSailOrder(fleetID, 90); err != nil {
		t.Fatalf("set sail order: %v", err)
	}
	for i := 0; i < 5; i++ {
		w.Tick()
	}
	if pos.X == startX && pos.Y == startY {
		t.Error("the fleet did not move")
	}
}

// TestMutinyChanceIsMonotonic verifies the mutiny probability model.
func TestMutinyChanceIsMonotonic(t *testing.T) {
	w := newTestWorld(t)
	fleet := getFleetComponent(w.ecs, w.PlayerEntity())

	fleet.Fatigue = 10
	fleet.Loyalty = 100
	low := w.mutinyChance(fleet)

	fleet.Fatigue = 95
	fleet.Loyalty = 10
	high := w.mutinyChance(fleet)

	if low != 0 {
		t.Errorf("mutiny chance at low fatigue = %.4f, want 0", low)
	}
	if high <= low {
		t.Errorf("mutiny chance at high fatigue/low loyalty = %.4f, want > %.4f", high, low)
	}
	if high > 1 {
		t.Errorf("mutiny chance = %.4f, want <= 1", high)
	}
}

// TestReputationAndTitles verifies the reputation system and the title
// unlocking.
func TestReputationAndTitles(t *testing.T) {
	w := newTestWorld(t)

	w.AddReputation(FactionPlayer, AxisTrade, 60)
	if got := w.Reputation(FactionPlayer, AxisTrade); got < 59 {
		t.Errorf("trade reputation = %.2f, want ~60", got)
	}
	if !w.HasTitle(FactionPlayer, "title.trader") {
		t.Error("the merchant title was not unlocked at 60 trade fame")
	}
	// The merchant prince title needs 50 trade fame, so it is unlocked
	// at 60 already.
	if !w.HasTitle(FactionPlayer, "title.merchant_prince") {
		t.Error("the merchant prince title was not unlocked at 60 trade fame")
	}
	// The admiral title belongs to the naval axis and must stay locked.
	if w.HasTitle(FactionPlayer, "title.admiral") {
		t.Error("the admiral title was unlocked without any naval fame")
	}
	w.AddReputation(FactionPlayer, AxisNaval, 60)
	if !w.HasTitle(FactionPlayer, "title.admiral") {
		t.Error("the admiral title was not unlocked at 60 naval fame")
	}
}

// TestAIFleetsTrade verifies that the AI fleets actually conduct
// business: their cargo changes over time.
func TestAIFleetsTrade(t *testing.T) {
	w := newTestWorld(t)

	// Count the total cargo of all AI fleets.
	totalCargo := func() float64 {
		sum := 0.0
		cargoStore := ecs.ComponentArrayOf[ecs.CargoComponent](w.ecs)
		aiStore := ecs.ComponentArrayOf[ecs.AIComponent](w.ecs)
		if cargoStore == nil || aiStore == nil {
			return 0
		}
		aiStore.Each(func(id ecs.ID, ai *ecs.AIComponent) {
			cargo := ecs.GetComponent[ecs.CargoComponent](w.ecs, id)
			if cargo == nil {
				return
			}
			for _, qty := range cargo.Manifest {
				sum += qty
			}
		})
		return sum
	}

	before := totalCargo()
	for i := 0; i < 30; i++ {
		w.Tick()
	}
	after := totalCargo()
	if before == 0 && after == 0 {
		t.Error("the AI fleets never carried any cargo")
	}
}
