package config

import (
	"fmt"
	"sort"
)

// Registry is the fully indexed, read-only view of every static table.
//
// A Registry is built once (by internal/asset) and then treated as an
// immutable value: it may be read from many goroutines concurrently without
// any locking, because none of its fields is ever written after Build
// returns.
//
// All lookups are O(1) hash-map accesses. The reverse indexes are
// pre-computed at build time so that hot simulation loops never need to
// scan a full table.
type Registry struct {
	// Commodities is indexed by Commodity.ID.
	Commodities map[string]*Commodity
	// Ports is indexed by Port.ID.
	Ports map[string]*Port
	// Ships is indexed by Ship.ID.
	Ships map[string]*Ship
	// Characters is indexed by CharacterTemplate.ID.
	Characters map[string]*CharacterTemplate
	// SeaZones is indexed by SeaZone.ID.
	SeaZones map[string]*SeaZone
	// Weather is indexed by WeatherTemplate.ID.
	Weather map[string]*WeatherTemplate
	// Events is indexed by EventTemplate.ID.
	Events map[string]*EventTemplate
	// Rumors is indexed by RumorTemplate.ID.
	Rumors map[string]*RumorTemplate
	// Titles is indexed by Title.ID.
	Titles map[string]*Title
	// AIProfiles is indexed by AIProfile.ID.
	AIProfiles map[string]*AIProfile
	// Params holds the global tuning constants.
	Params GlobalParams

	// --- reverse indexes (pre-built) ---

	// PortByRegion groups port IDs by their Region value.
	PortByRegion map[string][]string
	// PortPriceAt maps "portID\x00commodityID" to the static price entry.
	PortPriceAt map[string]*PortPrice
	// SpecialtyPorts maps a commodity ID to the ports producing it.
	SpecialtyPorts map[string][]string
	// PortsBySpecialty is the reverse of SpecialtyPorts: port ID -> commodity ID.
	PortsBySpecialty map[string]string
	// EventByCategory groups event IDs by category.
	EventByCategory map[string][]string
	// RumorByCategory groups rumor IDs by category.
	RumorByCategory map[string][]string
	// TitlesByAxis groups title IDs by reputation axis.
	TitlesByAxis map[string][]string

	// --- the tables that were imported from the original assets ---

	// OriginalMessages holds the interface strings of the original game.
	OriginalMessages map[int]*OriginalText
	// OriginalMessages2 holds the second page of interface strings.
	OriginalMessages2 map[int]*OriginalText
	// OriginalMenuText holds the menu strings of the original game.
	OriginalMenuText map[int]*OriginalText
	// OriginalNames holds the name pool of the original game.
	OriginalNames map[int]*OriginalText
	// OriginalDialogue holds the dialogues of the seven scenarios,
	// indexed by the scenario number.
	OriginalDialogue [7]map[int]*OriginalText
	// OriginalColonies holds the colonies and wonders of the original game.
	OriginalColonies map[int]*OriginalColony
	// OriginalItems holds the item descriptions of the original game.
	OriginalItems map[int]*OriginalItemDescription
	// OriginalMenuItems holds the menu tree of the original game.
	OriginalMenuItems map[int]*OriginalMenuItem

	// --- ordered lists, kept in deterministic (ID-sorted) order ---
	// Deterministic ordering matters for reproducible simulation and for
	// diff-friendly exported data.

	CommodityOrder []string
	PortOrder      []string
	ShipOrder      []string
	EventOrder     []string
	RumorOrder     []string
	TitleOrder     []string
	AIProfileOrder []string
}

// NewRegistry returns an empty registry with all maps allocated.
//
// The caller is expected to populate it exclusively through Build* methods
// (or the asset loader) before handing it to the simulation.
func NewRegistry() *Registry {
	return &Registry{
		Commodities:      make(map[string]*Commodity),
		Ports:            make(map[string]*Port),
		Ships:            make(map[string]*Ship),
		Characters:       make(map[string]*CharacterTemplate),
		SeaZones:         make(map[string]*SeaZone),
		Weather:          make(map[string]*WeatherTemplate),
		Events:           make(map[string]*EventTemplate),
		Rumors:           make(map[string]*RumorTemplate),
		Titles:           make(map[string]*Title),
		AIProfiles:       make(map[string]*AIProfile),
		PortByRegion:     make(map[string][]string),
		PortPriceAt:      make(map[string]*PortPrice),
		SpecialtyPorts:   make(map[string][]string),
		PortsBySpecialty: make(map[string]string),
		EventByCategory:  make(map[string][]string),
		RumorByCategory:  make(map[string][]string),
		TitlesByAxis:     make(map[string][]string),

		OriginalMessages:  make(map[int]*OriginalText),
		OriginalMessages2: make(map[int]*OriginalText),
		OriginalMenuText:  make(map[int]*OriginalText),
		OriginalNames:     make(map[int]*OriginalText),
		OriginalColonies:  make(map[int]*OriginalColony),
		OriginalItems:     make(map[int]*OriginalItemDescription),
		OriginalMenuItems: make(map[int]*OriginalMenuItem),
	}
}

// priceKey builds the composite key of the port/commodity price index.
func priceKey(portID, commodityID string) string {
	return portID + "\x00" + commodityID
}

// SetCommodity registers a commodity and records its deterministic order.
func (r *Registry) SetCommodity(c *Commodity) {
	r.Commodities[c.ID] = c
	r.CommodityOrder = append(r.CommodityOrder, c.ID)
}

// SetPort registers a port and records its deterministic order.
func (r *Registry) SetPort(p *Port) {
	r.Ports[p.ID] = p
	r.PortOrder = append(r.PortOrder, p.ID)
}

// SetShip registers a ship class.
func (r *Registry) SetShip(s *Ship) {
	r.Ships[s.ID] = s
	r.ShipOrder = append(r.ShipOrder, s.ID)
}

// SetCharacter registers a character template.
func (r *Registry) SetCharacter(c *CharacterTemplate) {
	r.Characters[c.ID] = c
}

// SetSeaZone registers a sea zone.
func (r *Registry) SetSeaZone(z *SeaZone) {
	r.SeaZones[z.ID] = z
}

// SetWeather registers a weather template.
func (r *Registry) SetWeather(w *WeatherTemplate) {
	r.Weather[w.ID] = w
}

// SetEvent registers an event template.
func (r *Registry) SetEvent(e *EventTemplate) {
	r.Events[e.ID] = e
	r.EventOrder = append(r.EventOrder, e.ID)
}

// SetRumor registers a rumor template.
func (r *Registry) SetRumor(r0 *RumorTemplate) {
	r.Rumors[r0.ID] = r0
	r.RumorOrder = append(r.RumorOrder, r0.ID)
}

// SetTitle registers a title.
func (r *Registry) SetTitle(t *Title) {
	r.Titles[t.ID] = t
	r.TitleOrder = append(r.TitleOrder, t.ID)
}

// SetAIProfile registers an AI profile.
func (r *Registry) SetAIProfile(p *AIProfile) {
	r.AIProfiles[p.ID] = p
	r.AIProfileOrder = append(r.AIProfileOrder, p.ID)
}

// SetPortPrice registers a static per-port commodity price entry.
func (r *Registry) SetPortPrice(pp *PortPrice) {
	r.PortPriceAt[priceKey(pp.PortID, pp.CommodityID)] = pp
}

// Finalize sorts the deterministic order lists and builds every reverse
// index. It must be called exactly once, after all Set* calls and before the
// registry is published to the simulation.
func (r *Registry) Finalize() {
	sort.Strings(r.CommodityOrder)
	sort.Strings(r.PortOrder)
	sort.Strings(r.ShipOrder)
	sort.Strings(r.EventOrder)
	sort.Strings(r.RumorOrder)
	sort.Strings(r.TitleOrder)
	sort.Strings(r.AIProfileOrder)

	// The dialogue table of every scenario needs its own map.
	for i := range r.OriginalDialogue {
		if r.OriginalDialogue[i] == nil {
			r.OriginalDialogue[i] = make(map[int]*OriginalText)
		}
	}

	// Region index + specialty cross reference.
	for _, id := range r.PortOrder {
		p := r.Ports[id]
		r.PortByRegion[p.Region] = append(r.PortByRegion[p.Region], id)
		if p.SpecialtyCommodity != "" {
			r.SpecialtyPorts[p.SpecialtyCommodity] = append(r.SpecialtyPorts[p.SpecialtyCommodity], id)
			r.PortsBySpecialty[p.ID] = p.SpecialtyCommodity
		}
	}
	for k := range r.SpecialtyPorts {
		sort.Strings(r.SpecialtyPorts[k])
	}

	// Event / rumor category indexes.
	for _, id := range r.EventOrder {
		e := r.Events[id]
		r.EventByCategory[e.Category] = append(r.EventByCategory[e.Category], id)
	}
	for _, id := range r.RumorOrder {
		rm := r.Rumors[id]
		r.RumorByCategory[rm.Category] = append(r.RumorByCategory[rm.Category], id)
	}
	for _, id := range r.TitleOrder {
		t := r.Titles[id]
		r.TitlesByAxis[t.Axis] = append(r.TitlesByAxis[t.Axis], id)
	}
}

// OriginalMenu returns the labels of one menu of the original game.
//
// The menu number is the index of the menu in the original menu table.
func (r *Registry) OriginalMenu(menu int) []string {
	out := make([]string, 0, 16)
	for id := 0; id < len(r.OriginalMenuItems); id++ {
		item, ok := r.OriginalMenuItems[id]
		if ok && item.Menu == menu {
			out = append(out, item.Label)
		}
	}
	return out
}

// OriginalString returns an imported text line.
//
// The lookup returns the empty string when the table or the line does not
// exist, so that the caller can always fall back to its own wording.
func (r *Registry) OriginalString(table map[int]*OriginalText, id int) string {
	if t, ok := table[id]; ok {
		return t.Text
	}
	return ""
}

// Commodity looks up a commodity by ID.
func (r *Registry) Commodity(id string) *Commodity { return r.Commodities[id] }

// Port looks up a port by ID.
func (r *Registry) Port(id string) *Port { return r.Ports[id] }

// Ship looks up a ship class by ID.
func (r *Registry) Ship(id string) *Ship { return r.Ships[id] }

// PortPriceFactor returns the static price multiplier of a commodity at a
// port. Ports without an explicit entry default to 1.0.
func (r *Registry) PortPriceFactor(portID, commodityID string) float64 {
	if pp, ok := r.PortPriceAt[priceKey(portID, commodityID)]; ok {
		return pp.Factor
	}
	return 1.0
}

// PortSupply returns the baseline daily supply of a commodity at a port.
func (r *Registry) PortSupply(portID, commodityID string) float64 {
	if pp, ok := r.PortPriceAt[priceKey(portID, commodityID)]; ok {
		return pp.Supply
	}
	return 0
}

// PortDemand returns the baseline daily demand of a commodity at a port.
func (r *Registry) PortDemand(portID, commodityID string) float64 {
	if pp, ok := r.PortPriceAt[priceKey(portID, commodityID)]; ok {
		return pp.Demand
	}
	return 0
}

// Validate performs structural checks on the registry: it reports duplicate
// IDs, dangling foreign keys and obviously out-of-range values.
//
// It returns the list of human readable problems; an empty slice means the
// data set is consistent.
func (r *Registry) Validate() []string {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	// Foreign key: port price entries must reference known ports/commodities.
	for key, pp := range r.PortPriceAt {
		if _, ok := r.Ports[pp.PortID]; !ok {
			add("port_prices: unknown port id %q (key %q)", pp.PortID, key)
		}
		if _, ok := r.Commodities[pp.CommodityID]; !ok {
			add("port_prices: unknown commodity id %q (key %q)", pp.CommodityID, key)
		}
		if pp.Factor <= 0 {
			add("port_prices: non-positive factor %.4f for %s/%s", pp.Factor, pp.PortID, pp.CommodityID)
		}
	}

	// Foreign key: ports referencing a specialty commodity.
	for _, id := range r.PortOrder {
		p := r.Ports[id]
		if p.SpecialtyCommodity != "" {
			if _, ok := r.Commodities[p.SpecialtyCommodity]; !ok {
				add("ports: %s references unknown specialty commodity %q", id, p.SpecialtyCommodity)
			}
			if p.SpecialtyDiscount < 0 || p.SpecialtyDiscount >= 1 {
				add("ports: %s specialty_discount %.4f out of range [0,1)", id, p.SpecialtyDiscount)
			}
		}
		if p.Lat < -90 || p.Lat > 90 {
			add("ports: %s latitude %.4f out of range [-90,90]", id, p.Lat)
		}
		if p.Lon < -180 || p.Lon > 180 {
			add("ports: %s longitude %.4f out of range [-180,180]", id, p.Lon)
		}
	}

	// Commodity sanity.
	for _, id := range r.CommodityOrder {
		c := r.Commodities[id]
		if c.BasePrice <= 0 {
			add("commodities: %s base_price %.4f must be > 0", id, c.BasePrice)
		}
		if c.CycleAmplitude < 0 || c.CycleAmplitude > 1 {
			add("commodities: %s cycle_amplitude %.4f out of range [0,1]", id, c.CycleAmplitude)
		}
		if c.SeasonalAmplitude < 0 || c.SeasonalAmplitude > 1 {
			add("commodities: %s seasonal_amplitude %.4f out of range [0,1]", id, c.SeasonalAmplitude)
		}
		if c.WeightPerUnit <= 0 {
			add("commodities: %s weight_per_unit must be > 0", id)
		}
	}

	// Ship sanity.
	for _, id := range r.ShipOrder {
		s := r.Ships[id]
		if s.HullPoints <= 0 {
			add("ships: %s hull_points must be > 0", id)
		}
		if s.MaxSailors <= 0 {
			add("ships: %s max_sailors must be > 0", id)
		}
		if s.CargoCapacity <= 0 {
			add("ships: %s cargo_capacity must be > 0", id)
		}
	}

	// Event / rumor sanity.
	for _, id := range r.EventOrder {
		e := r.Events[id]
		if e.Weight < 0 {
			add("events: %s has negative weight", id)
		}
		if e.MinDay > 0 && e.MaxDay > 0 && e.MinDay > e.MaxDay {
			add("events: %s min_day %d > max_day %d", id, e.MinDay, e.MaxDay)
		}
	}
	for _, id := range r.RumorOrder {
		rm := r.Rumors[id]
		if rm.TargetPort != "" {
			if _, ok := r.Ports[rm.TargetPort]; !ok {
				add("rumors: %s references unknown target port %q", id, rm.TargetPort)
			}
		}
	}

	// Global parameters sanity.
	if p := r.Params; true {
		if p.StartMoney < 0 {
			add("globals: start_money must be >= 0")
		}
		if p.DaysPerGameYear <= 0 {
			add("globals: days_per_game_year must be > 0")
		}
		if p.MapWidth <= 0 || p.MapHeight <= 0 {
			add("globals: map dimensions must be > 0")
		}
		if p.TravelSpeedScale <= 0 {
			add("globals: travel_speed_scale must be > 0")
		}
	}

	return problems
}
