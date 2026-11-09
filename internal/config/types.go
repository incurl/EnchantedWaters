// Package config defines the *static* configuration layer of the game.
//
// Everything in this package describes immutable, table-driven data: the
// canonical source of truth for ports, commodities, ships, characters,
// weather, events and all numerical tuning parameters.
//
// # Design rules (project-wide invariant)
//
//   - Types declared here are READ-ONLY at runtime. They are loaded exactly
//     once at start-up by package internal/asset and then never mutated.
//   - No numeric gameplay constant may be hard-coded in Go source. Every
//     value that influences simulation must live in a CSV/JSON table that is
//     either hand-written in assets/config or exported by the asset importer
//     (see pkg/importer).
//   - The dynamic state layer (internal/world, internal/ecs) must never embed
//     copies of these structures; it stores IDs and references instead.
//
// The `csv` struct tags describe how each field is read from the matching
// table file in assets/config. The `opt` flag marks a column as optional,
// meaning a missing column or an empty cell yields the zero value instead of
// a load error.
package config

// CommodityClass groups commodities so the economy can apply differentiated
// pricing rules (luxury goods swing much harder than bulk staples).
type CommodityClass string

const (
	// ClassLuxury is high value, low volume, strongly price-elastic goods
	// (spices, silk, gems...). Their price variance is scaled up.
	ClassLuxury CommodityClass = "luxury"
	// ClassGeneral is the standard bulk goods (grain, timber, iron...).
	ClassGeneral CommodityClass = "general"
	// ClassSpecialty is a commodity produced by exactly one port region and
	// consumed everywhere else. Its local price is depressed at the source
	// and inflated abroad.
	ClassSpecialty CommodityClass = "specialty"
)

// Commodity is a tradeable good.
//
// The price of a commodity at a port at a given time is modelled as:
//
//	price = BasePrice * CycleFactor * SeasonalFactor * PortFactor * ScarcityFactor * (1 + Noise)
//
// where every factor is derived from the tables below and from dynamic state
// (inventory). See internal/world/economy.go for the exact implementation.
type Commodity struct {
	// ID is the stable primary key, e.g. "spice".
	ID string `csv:"id"`
	// Name is the display name (English).
	Name string `csv:"name"`
	// NameJP is the optional Japanese display name.
	NameJP string `csv:"name_jp,opt"`
	// Class selects the differentiated pricing behaviour.
	Class CommodityClass `csv:"class"`
	// BasePrice is the reference price in a "neutral" port at cycle phase 0.
	BasePrice float64 `csv:"base_price"`
	// MinPrice / MaxPrice clamp the final computed price. Zero disables the
	// corresponding bound.
	MinPrice float64 `csv:"min_price,opt"`
	MaxPrice float64 `csv:"max_price,opt"`
	// CyclePeriodDays is the length of one full business cycle.
	CyclePeriodDays float64 `csv:"cycle_period_days"`
	// CycleAmplitude is the relative peak-to-mean swing of the business cycle
	// (0.25 => price oscillates between 0.75x and 1.25x of the mean).
	CycleAmplitude float64 `csv:"cycle_amplitude"`
	// SeasonalAmplitude is the relative peak-to-mean swing caused by the
	// yearly season (harvest, monsoon...).
	SeasonalAmplitude float64 `csv:"seasonal_amplitude"`
	// SeasonalPhase is the day-of-year offset (0..365) of the seasonal peak.
	SeasonalPhase float64 `csv:"seasonal_phase"`
	// WeightPerUnit is the cargo weight of a single unit, in tons.
	WeightPerUnit float64 `csv:"weight_per_unit"`
	// VolumePerUnit is the hold space consumed by a single unit.
	VolumePerUnit float64 `csv:"volume_per_unit"`
	// Perishable marks goods that spoil; their stock decays every day.
	Perishable bool `csv:"perishable"`
	// SpoilageRate is the fraction of perishable stock lost per day.
	SpoilageRate float64 `csv:"spoilage_rate,opt"`
	// LuxuryVarianceScale multiplies the stochastic noise for luxury goods.
	LuxuryVarianceScale float64 `csv:"luxury_variance_scale,opt"`
	// MinPortPriceFactor / MaxPortPriceFactor clamp the per-port multiplier.
	MinPortPriceFactor float64 `csv:"min_port_price_factor,opt"`
	MaxPortPriceFactor float64 `csv:"max_port_price_factor,opt"`
}

// PortClass describes the economic weight of a settlement.
type PortClass string

const (
	// PortHamlet is a tiny village: few services, low taxes.
	PortHamlet PortClass = "hamlet"
	// PortTown is an ordinary town.
	PortTown PortClass = "town"
	// PortCity is a large trade hub with the full set of services.
	PortCity PortClass = "city"
	// PortCapital is a national capital with the richest facilities.
	PortCapital PortClass = "capital"
)

// Port is a settlement on the world map.
//
// Ports are static entities: their position, facilities and local price
// modifiers never change. Mutable data (inventory, prices, business cycle
// state, reputation) lives in the dynamic layer as PortDynamicComponent.
type Port struct {
	// ID is the stable primary key, e.g. "ports.izumo".
	ID string `csv:"id"`
	// Name is the English display name.
	Name string `csv:"name"`
	// NameJP is the optional Japanese display name.
	NameJP string `csv:"name_jp,opt"`
	// Class selects tax/service multipliers.
	Class PortClass `csv:"class"`
	// Region groups ports for specialty and price-diffusion purposes.
	Region string `csv:"region"`
	// X / Y are world-map pixel coordinates of the port anchor.
	X float64 `csv:"x"`
	Y float64 `csv:"y"`
	// Lat / Lon are the geographic coordinates used for navigation and for
	// the distance formula between ports.
	Lat float64 `csv:"lat"`
	Lon float64 `csv:"lon"`
	// SpecialtyCommodity is the commodity this port produces, if any.
	SpecialtyCommodity string `csv:"specialty_commodity,opt"`
	// SpecialtyDiscount is the relative price reduction applied to the local
	// specialty at the producing port (0.2 => 20% cheaper).
	SpecialtyDiscount float64 `csv:"specialty_discount,opt"`
	// BaseTaxRate is the per-transaction tax rate (0.05 => 5%).
	BaseTaxRate float64 `csv:"base_tax_rate"`
	// ShipDiscount is the discount the port grants on the purchase of a
	// ship (0.95 => 5% off). It defaults to 1.0 when the column is
	// absent.
	ShipDiscount float64 `csv:"ship_discount,opt"`
	// ShipyardLevel / TavernLevel / GovernorLevel / ChurchLevel / InnLevel
	// are facility levels (0..5) that unlock port interactions.
	ShipyardLevel int `csv:"shipyard_level"`
	TavernLevel   int `csv:"tavern_level"`
	GovernorLevel int `csv:"governor_level"`
	ChurchLevel   int `csv:"church_level"`
	InnLevel      int `csv:"inn_level"`
	// PriceMemoryFactor controls how strongly local stock influences price
	// (0 = prices ignore inventory, 1 = full scarcity response).
	PriceMemoryFactor float64 `csv:"price_memory_factor"`
}

// PortPrice is the static per-port multiplier of a commodity.
//
// price(port, commodity) = commodity.BasePrice * PortPrice.Factor * ...
//
// Ports not listed for a commodity implicitly use a factor of 1.0, which is
// resolved through the reverse index built at load time.
type PortPrice struct {
	// PortID references Port.ID.
	PortID string `csv:"port_id"`
	// CommodityID references Commodity.ID.
	CommodityID string `csv:"commodity_id"`
	// Factor is the static price multiplier of that commodity at that port.
	Factor float64 `csv:"factor"`
	// Supply is the baseline amount produced/available per day.
	Supply float64 `csv:"supply"`
	// Demand is the baseline amount consumed per day.
	Demand float64 `csv:"demand"`
}

// ShipClass groups ships for balance purposes.
type ShipClass string

const (
	// ClassSmall is a light coastal vessel.
	ClassSmall ShipClass = "small"
	// ClassMedium is a standard merchant/cruiser hull.
	ClassMedium ShipClass = "medium"
	// ClassLarge is a heavy galleon.
	ClassLarge ShipClass = "large"
	// ClassSpecial is a unique/cursed hull with special abilities.
	ClassSpecial ShipClass = "special"
)

// Ship is a vessel class definition.
type Ship struct {
	// ID is the stable primary key, e.g. "ship.merchant".
	ID string `csv:"id"`
	// Name is the display name.
	Name string `csv:"name"`
	// NameJP is the optional Japanese display name.
	NameJP string `csv:"name_jp,opt"`
	// Class groups the hull for balance and AI heuristics.
	Class ShipClass `csv:"class"`
	// HullPoints is the maximum durability.
	HullPoints float64 `csv:"hull_points"`
	// MaxSailors is the crew capacity.
	MaxSailors int `csv:"max_sailors"`
	// CargoCapacity is the total hold tonnage.
	CargoCapacity float64 `csv:"cargo_capacity"`
	// BaseSpeed is the hull speed in knots at wind angle 0 with no current.
	BaseSpeed float64 `csv:"base_speed"`
	// RepairCostPerHP is the shipyard cost to restore one hull point.
	RepairCostPerHP float64 `csv:"repair_cost_per_hp"`
	// Price is the purchase price at a shipyard.
	Price float64 `csv:"price"`
	// UpkeepPerDay is the daily maintenance cost.
	UpkeepPerDay float64 `csv:"upkeep_per_day"`
	// FoodPerSailorPerDay is the food consumption rate per crew member.
	FoodPerSailorPerDay float64 `csv:"food_per_sailor_per_day"`
	// WaterPerSailorPerDay is the fresh water consumption rate per crew member.
	WaterPerSailorPerDay float64 `csv:"water_per_sailor_per_day"`
	// ConsumableCapacity is the maximum tonnage of food+water carried.
	ConsumableCapacity float64 `csv:"consumable_capacity"`
	// WeaponLevel is the combat strength class (0..5).
	WeaponLevel int `csv:"weapon_level"`
	// ArmorLevel is the damage-reduction class (0..5).
	ArmorLevel int `csv:"armor_level"`
	// TurnSpeedDegPerDay is how fast the vessel can change heading.
	TurnSpeedDegPerDay float64 `csv:"turn_speed_deg_per_day"`
	// RequiredSailorLevel is the minimum sailor skill tier to operate the ship.
	RequiredSailorLevel int `csv:"required_sailor_level"`
}

// CharacterClass groups the four playable character archetypes.
type CharacterClass string

const (
	// ClassAdventurer is strong in exploration and combat.
	ClassAdventurer CharacterClass = "adventurer"
	// ClassMerchant is strong in trade.
	ClassMerchant CharacterClass = "merchant"
	// ClassNavigator is strong in seamanship.
	ClassNavigator CharacterClass = "navigator"
	// ClassNavalOfficer is strong in fleet command.
	ClassNavalOfficer CharacterClass = "naval_officer"
)

// CharacterTemplate is a recruit archetype used by taverns and events.
//
// The three-axis reputation system of the original game is modelled with
// Adventure / Trade / Naval values that grow with experience.
type CharacterTemplate struct {
	// ID is the stable primary key.
	ID string `csv:"id"`
	// Name is the display name.
	Name string `csv:"name"`
	// Class selects the growth curve.
	Class CharacterClass `csv:"class"`
	// Adventure / Trade / Naval are the initial reputation axes (0..100).
	Adventure float64 `csv:"adventure"`
	Trade     float64 `csv:"trade"`
	Naval     float64 `csv:"naval"`
	// SalaryPerDay is the daily wage.
	SalaryPerDay float64 `csv:"salary_per_day"`
	// LoyaltyBase is the starting loyalty (0..100).
	LoyaltyBase float64 `csv:"loyalty_base"`
	// LoyaltyDecayPerDay is how fast loyalty decays without pay.
	LoyaltyDecayPerDay float64 `csv:"loyalty_decay_per_day"`
	// HiringCost is the one-off recruitment fee.
	HiringCost float64 `csv:"hiring_cost"`
	// MinLevel / MaxLevel bound the level range.
	MinLevel int `csv:"min_level"`
	MaxLevel int `csv:"max_level"`
}

// SeaZone is a rectangular region of ocean with its own climate baseline.
//
// Currents and winds are evaluated per zone; the value at a point is the
// inverse-distance blend of the zone centres, which keeps the field smooth
// while remaining a cheap closed-form expression.
type SeaZone struct {
	// ID is the stable primary key.
	ID string `csv:"id"`
	// Name is the display name.
	Name string `csv:"name"`
	// CenterX / CenterY are the map coordinates of the zone centre.
	CenterX float64 `csv:"center_x"`
	CenterY float64 `csv:"center_y"`
	// Radius is the influence radius in map pixels.
	Radius float64 `csv:"radius"`
	// WindBaseSpeed is the baseline wind speed in knots.
	WindBaseSpeed float64 `csv:"wind_base_speed"`
	// WindBaseDirDeg is the baseline wind direction in degrees (0 = north).
	WindBaseDirDeg float64 `csv:"wind_base_dir_deg"`
	// CurrentBaseSpeed is the baseline current speed in knots.
	CurrentBaseSpeed float64 `csv:"current_base_speed"`
	// CurrentBaseDirDeg is the baseline current direction in degrees.
	CurrentBaseDirDeg float64 `csv:"current_base_dir_deg"`
	// StormProbability is the daily probability of a storm in this zone.
	StormProbability float64 `csv:"storm_probability"`
	// PirateProbability is the daily probability of a pirate encounter.
	PirateProbability float64 `csv:"pirate_probability"`
	// Latitude is the representative latitude used for day length.
	Latitude float64 `csv:"latitude"`
}

// WeatherKind classifies a weather state.
type WeatherKind string

const (
	// WeatherClear is calm weather.
	WeatherClear WeatherKind = "clear"
	// WeatherCloudy is overcast but harmless.
	WeatherCloudy WeatherKind = "cloudy"
	// WeatherRain is rain: slows sailing, raises sickness risk.
	WeatherRain WeatherKind = "rain"
	// WeatherStorm is a storm: heavy damage risk, big speed penalty.
	WeatherStorm WeatherKind = "storm"
	// WeatherFog reduces visibility and range.
	WeatherFog WeatherKind = "fog"
)

// WeatherTemplate is a static weather archetype.
//
// The dynamic layer picks a template and then perturbs its wind vector with
// the randomness described by the template.
type WeatherTemplate struct {
	// ID is the stable primary key.
	ID string `csv:"id"`
	// Name is the display name.
	Name string `csv:"name"`
	// Kind classifies the weather.
	Kind WeatherKind `csv:"kind"`
	// Weight is the relative selection weight.
	Weight float64 `csv:"weight"`
	// WindSpeedMin / WindSpeedMax bound the resulting wind speed in knots.
	WindSpeedMin float64 `csv:"wind_speed_min"`
	WindSpeedMax float64 `csv:"wind_speed_max"`
	// WindDirDegMin / WindDirDegMax bound the wind direction.
	WindDirDegMin float64 `csv:"wind_dir_deg_min"`
	WindDirDegMax float64 `csv:"wind_dir_deg_max"`
	// SpeedPenalty is the relative speed multiplier (0.5 => half speed).
	SpeedPenalty float64 `csv:"speed_penalty"`
	// DamagePerDay is the expected hull damage per day at full exposure.
	DamagePerDay float64 `csv:"damage_per_day"`
	// SicknessChance is the daily chance of a disease outbreak.
	SicknessChance float64 `csv:"sickness_chance"`
	// VisibilityFactor scales the map visibility radius.
	VisibilityFactor float64 `csv:"visibility_factor"`
	// MinDay / MaxDay restrict when the weather may be selected.
	MinDay int `csv:"min_day,opt"`
	MaxDay int `csv:"max_day,opt"`
}

// EventEffect is a single mechanical effect triggered by an event.
type EventEffect struct {
	// Kind is the effect verb, e.g. "damage", "resource", "reputation",
	// "discover", "message".
	Kind string `csv:"kind"`
	// Target selects what the effect applies to ("fleet", "crew", "cargo").
	Target string `csv:"target,opt"`
	// Value is the numeric magnitude.
	Value float64 `csv:"value"`
	// Key is the optional payload key (item id, region id, event id...).
	Key string `csv:"key,opt"`
	// Chance is the probability (0..1) that this effect triggers.
	Chance float64 `csv:"chance,opt"`
	// Message is the optional human readable text.
	Message string `csv:"message,opt"`
}

// EventTemplate is a static random-event definition.
//
// Event parameters are entirely table-driven: probabilities live in the
// weight column and in the effect chances, never as magic numbers in code.
type EventTemplate struct {
	// ID is the stable primary key.
	ID string `csv:"id"`
	// Name is the display name.
	Name string `csv:"name"`
	// Category groups events ("sea", "pirate", "storm", "discovery"...).
	Category string `csv:"category"`
	// Weight is the relative selection weight.
	Weight float64 `csv:"weight"`
	// MinDay / MaxDay restrict the day range in which it can occur.
	MinDay int `csv:"min_day,opt"`
	MaxDay int `csv:"max_day,opt"`
	// MinRegion / MaxRegion restrict the region range ("" = unrestricted).
	MinRegion string `csv:"min_region,opt"`
	MaxRegion string `csv:"max_region,opt"`
	// Repeatable marks events that may fire more than once.
	Repeatable bool `csv:"repeatable,opt"`
	// Script is an optional Lua script name executed by internal/event.
	Script string `csv:"script,opt"`
	// Effects is the list of mechanical effects (multi-value CSV column).
	Effects []string `csv:"effects,multi,opt"`
	// Message is the default log message.
	Message string `csv:"message,opt"`
}

// RumorTemplate is a piece of information that a tavern can reveal.
type RumorTemplate struct {
	// ID is the stable primary key.
	ID string `csv:"id"`
	// Text is the rumor body.
	Text string `csv:"text"`
	// Category groups rumors ("port", "treasure", "danger", "person"...).
	Category string `csv:"category"`
	// Weight is the relative selection weight.
	Weight float64 `csv:"weight"`
	// MinDay / MaxDay restrict the day range.
	MinDay int `csv:"min_day,opt"`
	MaxDay int `csv:"max_day,opt"`
	// TargetPort / TargetRegion restrict where the rumor is useful.
	TargetPort   string `csv:"target_port,opt"`
	TargetRegion string `csv:"target_region,opt"`
}

// Title is a reputation title unlocked at a given fame threshold.
type Title struct {
	// ID is the stable primary key.
	ID string `csv:"id"`
	// Name is the display name.
	Name string `csv:"name"`
	// Axis selects which reputation axis unlocks it.
	Axis string `csv:"axis"`
	// Threshold is the fame required on that axis.
	Threshold float64 `csv:"threshold"`
	// BonusTrade / BonusAdventure / BonusNaval are the effects of the title.
	BonusTrade     float64 `csv:"bonus_trade,opt"`
	BonusAdventure float64 `csv:"bonus_adventure,opt"`
	BonusNaval     float64 `csv:"bonus_naval,opt"`
	// Description is shown in the reputation screen.
	Description string `csv:"description,opt"`
}

// AIProfile parameterises an AI faction's trading behaviour.
type AIProfile struct {
	// ID is the stable primary key.
	ID string `csv:"id"`
	// Name is the display name.
	Name string `csv:"name"`
	// FleetCount is how many fleets the faction commands.
	FleetCount int `csv:"fleet_count"`
	// RiskAversion scales how much the AI discounts volatile commodities.
	RiskAversion float64 `csv:"risk_aversion"`
	// CargoAggressiveness is the fraction of hold the AI fills.
	CargoAggressiveness float64 `csv:"cargo_aggressiveness"`
	// MaxVoyageDays is how long the AI is willing to sail before returning.
	MaxVoyageDays int `csv:"max_voyage_days"`
	// StormAvoidance is the probability of detouring around a storm.
	StormAvoidance float64 `csv:"storm_avoidance"`
	// TradeSkill scales the AI's profit evaluation accuracy.
	TradeSkill float64 `csv:"trade_skill"`
	// HomePort restricts the AI to a home region.
	HomePort string `csv:"home_port,opt"`
}

// GlobalParams holds simulation-wide tuning constants.
//
// These live in a table like every other number so that balance changes do
// not require a recompilation.
type GlobalParams struct {
	// DayLengthRealSeconds is how many real seconds one in-game day takes.
	DayLengthRealSeconds float64 `csv:"day_length_real_seconds"`
	// DaysPerGameYear is the length of the in-game year in days.
	DaysPerGameYear int `csv:"days_per_game_year"`
	// StartMoney is the initial player capital.
	StartMoney float64 `csv:"start_money"`
	// StartFood is the initial food stock (tons).
	StartFood float64 `csv:"start_food"`
	// StartWater is the initial fresh water stock (tons).
	StartWater float64 `csv:"start_water"`
	// MapWidth / MapHeight are the world map dimensions in pixels.
	MapWidth  int `csv:"map_width"`
	MapHeight int `csv:"map_height"`
	// PixelsPerDegree converts world pixels to degrees of longitude.
	PixelsPerDegree float64 `csv:"pixels_per_degree"`
	// PriceNoiseScale is the relative magnitude of stochastic price noise.
	PriceNoiseScale float64 `csv:"price_noise_scale"`
	// PriceNoiseDecay is the per-day decay of the price noise random walk.
	PriceNoiseDecay float64 `csv:"price_noise_decay"`
	// ScarcityResponse is how strongly stock shortage raises prices.
	ScarcityResponse float64 `csv:"scarcity_response"`
	// ScarcitySmoothing is the per-day smoothing of the scarcity factor.
	ScarcitySmoothing float64 `csv:"scarcity_smoothing"`
	// FatiguePerDaySail is the fatigue gained per day of sailing.
	FatiguePerDaySail float64 `csv:"fatigue_per_day_sail"`
	// FatiguePerDayRest is the fatigue recovered per day in port.
	FatiguePerDayRest float64 `csv:"fatigue_per_day_rest"`
	// MutinyThreshold is the fatigue level at which a mutiny may start.
	MutinyThreshold float64 `csv:"mutiny_threshold"`
	// MutinyBaseChance is the daily mutiny probability at full fatigue.
	MutinyBaseChance float64 `csv:"mutiny_base_chance"`
	// MutinyLoyaltyFactor scales the mutiny chance by low loyalty.
	MutinyLoyaltyFactor float64 `csv:"mutiny_loyalty_factor"`
	// SeaEventPerDayChance is the daily chance of any sea event.
	SeaEventPerDayChance float64 `csv:"sea_event_per_day_chance"`
	// DiscoveryPerDayChance is the daily chance of a discovery.
	DiscoveryPerDayChance float64 `csv:"discovery_per_day_chance"`
	// ShipyardRepairSpeed is the fraction of damage repaired per day.
	ShipyardRepairSpeed float64 `csv:"shipyard_repair_speed"`
	// PriceRefreshIntervalSec is how often the exchange refreshes prices.
	PriceRefreshIntervalSec float64 `csv:"price_refresh_interval_sec"`
	// FameDecayPerDay is how fast the reputation axes decay.
	FameDecayPerDay float64 `csv:"fame_decay_per_day"`
	// FamePropagationFactor scales the effect of an ally's fame.
	FamePropagationFactor float64 `csv:"fame_propagation_factor"`
	// LoyaltyBonusPerFamePoint is the loyalty granted per fame point.
	LoyaltyBonusPerFamePoint float64 `csv:"loyalty_bonus_per_fame_point"`
	// LoyaltyDecayPerDay is how fast the loyalty of unhappy crews decays.
	LoyaltyDecayPerDay float64 `csv:"loyalty_decay_per_day"`
	// TravelSpeedScale converts knots into map pixels per day.
	TravelSpeedScale float64 `csv:"travel_speed_scale"`
	// AICargoFill is the fraction of the hold the AI fills when the
	// profile does not override it.
	AICargoFill float64 `csv:"ai_cargo_fill"`
	// AITradeTaxFactor is the weight of the transaction costs in the
	// profit evaluation of the AI.
	AITradeTaxFactor float64 `csv:"ai_trade_tax_factor"`
	// WindEfficiencyFloor is the share of the hull speed that is kept on
	// a beam reach.
	WindEfficiencyFloor float64 `csv:"wind_efficiency_floor"`
	// WindBlendStart is the angle (in degrees) at which the linear ramp
	// of the wind efficiency curve starts.
	WindBlendStart float64 `csv:"wind_blend_start"`
}
