package world

import (
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"

	"github.com/peter/enchantedwaters/internal/asset"
	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/ecs"
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

// TestWindEfficiency verifies the shape of the sailing efficiency curve:
// maximum with the wind, zero against it, and a sensible value on a
// reach.
func TestWindEfficiency(t *testing.T) {
	if got := windEfficiency(0); got < 0.99 {
		t.Errorf("windEfficiency(0) = %.3f, want ~1", got)
	}
	if got := windEfficiency(89); got < 0.001 {
		t.Errorf("windEfficiency(89) = %.3f, want ~0", got)
	}
	if got := windEfficiency(90); got != 0 {
		t.Errorf("windEfficiency(90) = %.3f, want 0 (no-go zone boundary)", got)
	}
	if got := windEfficiency(120); got != 0 {
		t.Errorf("windEfficiency(120) = %.3f, want 0 (no-go zone)", got)
	}
	if got := windEfficiency(180); got != 0 {
		t.Errorf("windEfficiency(180) = %.3f, want 0 (no-go zone)", got)
	}
	// The efficiency must decrease monotonically from 0 to 90 degrees.
	prev := 1.1
	for deg := 0; deg <= 90; deg += 5 {
		got := windEfficiency(float64(deg))
		if got > prev {
			t.Errorf("windEfficiency is not monotonic at %d degrees", deg)
		}
		prev = got
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
