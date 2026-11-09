package asset

import (
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"

	"github.com/peter/enchantedwaters/internal/config"
)

// configDir returns the path of the checked-in configuration directory.
func configDir(t *testing.T) string {
	t.Helper()
	// The test lives in internal/asset, so the tables are two levels up.
	return filepath.Join("..", "..", "assets", "config")
}

// TestLoadTables loads every table of the shipped data set and verifies
// that the registry is complete and consistent.
func TestLoadTables(t *testing.T) {
	reg, err := Load(configDir(t), logr.Discard())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if reg == nil {
		t.Fatal("registry is nil")
	}

	// --- the tables must not be empty ---
	if len(reg.Commodities) == 0 {
		t.Error("no commodities loaded")
	}
	if len(reg.Ports) == 0 {
		t.Error("no ports loaded")
	}
	if len(reg.Ships) == 0 {
		t.Error("no ships loaded")
	}
	if len(reg.Events) == 0 {
		t.Error("no events loaded")
	}
	if len(reg.Rumors) == 0 {
		t.Error("no rumors loaded")
	}
	if len(reg.AIProfiles) == 0 {
		t.Error("no AI profiles loaded")
	}
	if len(reg.SeaZones) == 0 {
		t.Error("no sea zones loaded")
	}
	if len(reg.Weather) == 0 {
		t.Error("no weather templates loaded")
	}

	// --- the global parameters must be sane ---
	p := reg.Params
	if p.StartMoney <= 0 {
		t.Errorf("start money = %v, want > 0", p.StartMoney)
	}
	if p.MapWidth <= 0 || p.MapHeight <= 0 {
		t.Errorf("map size = %dx%d, want > 0", p.MapWidth, p.MapHeight)
	}
	if p.DaysPerGameYear <= 0 {
		t.Errorf("days per game year = %d, want > 0", p.DaysPerGameYear)
	}

	// --- the reverse indexes must be built ---
	if len(reg.PortByRegion) == 0 {
		t.Error("region index is empty")
	}
	if len(reg.PortPriceAt) != len(reg.PortOrder)*len(reg.CommodityOrder) {
		t.Errorf("port price index has %d entries, want %d",
			len(reg.PortPriceAt), len(reg.PortOrder)*len(reg.CommodityOrder))
	}
	if len(reg.SpecialtyPorts) == 0 {
		t.Error("specialty index is empty")
	}
	if len(reg.EventByCategory) == 0 {
		t.Error("event category index is empty")
	}

	// --- the deterministic orderings must be sorted ---
	if !isSorted(reg.CommodityOrder) {
		t.Error("commodity order is not sorted")
	}
	if !isSorted(reg.PortOrder) {
		t.Error("port order is not sorted")
	}
}

// TestValidateIsClean verifies that the shipped data set passes the
// structural validation.
func TestValidateIsClean(t *testing.T) {
	reg, err := Load(configDir(t), logr.Discard())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if problems := reg.Validate(); len(problems) > 0 {
		for _, p := range problems {
			t.Errorf("validation problem: %s", p)
		}
	}
}

// TestSpecialtyDiscount verifies that a port producing a commodity sells
// it cheaper than the base price.
func TestSpecialtyDiscount(t *testing.T) {
	reg, err := Load(configDir(t), logr.Discard())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	// Find a port with a specialty and a port without.
	var producer *config.Port
	var consumer *config.Port
	for _, id := range reg.PortOrder {
		p := reg.Ports[id]
		if p.SpecialtyCommodity == "" {
			continue
		}
		if producer == nil {
			producer = p
			continue
		}
		if p.SpecialtyCommodity != producer.SpecialtyCommodity {
			consumer = p
			break
		}
	}
	if producer == nil {
		t.Skip("no specialty ports in the data set")
	}
	if consumer == nil {
		t.Skip("not enough ports to compare")
	}
	commodity := reg.Commodity(producer.SpecialtyCommodity)
	if commodity == nil {
		t.Fatalf("specialty commodity %q is missing", producer.SpecialtyCommodity)
	}
	producerFactor := reg.PortPriceFactor(producer.ID, commodity.ID)
	consumerFactor := reg.PortPriceFactor(consumer.ID, commodity.ID)
	if producerFactor >= consumerFactor {
		t.Errorf("producer factor %.4f should be below consumer factor %.4f",
			producerFactor, consumerFactor)
	}
}

// isSorted reports whether a string slice is sorted.
func isSorted(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}
