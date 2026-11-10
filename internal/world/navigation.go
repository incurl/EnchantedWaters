package world

import (
	"math"

	"github.com/peter/enchantedwaters/internal/config"
	"github.com/peter/enchantedwaters/internal/sail"
)

// Sailing physics.
//
// The speed of a fleet over the ground is the vector sum of two
// contributions:
//
//	v_ground = v_sail(θ, w) + v_current
//
// where v_sail is the sailing model of the original game (package sail) and
// v_current is the drift of the sea zone.
//
// The original does not model the wind as a continuous angle. The compass
// has eight directions and the wind blows at an integer strength, so the
// drive of a hull is a lookup in a table indexed by the sail type and by
// the number of octants between the course and the wind. There is no
// no-go zone: a square rigged ship still creeps to windward at four tenths
// of its drive, and a lateen rigged one keeps eight tenths.
//
// The current contribution is added as a vector: a following current
// speeds the fleet up, a head current slows it down, and a beam current
// mostly pushes it sideways.
//
// The parameters (hull tables, weather penalties, turn rates) all come from
// the configuration tables.

// windSpeedEpsilon is the wind speed below which the wind direction is
// considered undefined.
const windSpeedEpsilon = 0.001

// vectorFromPolar converts a speed and a compass bearing into a vector.
//
// The bearing is measured clockwise from north (+Y axis on the map is
// south, so the sign of Y is inverted).
func vectorFromPolar(speed, bearingDeg float64) (x, y float64) {
	rad := bearingDeg * math.Pi / 180
	return speed * math.Sin(rad), -speed * math.Cos(rad)
}

// currentWindAt returns the wind and current vectors at a map position.
//
// The wind of the original game is a table: the world map is divided into
// sea areas, and every sea area holds the direction of the wind and its
// strength. When that table has been imported the wind comes from it, and
// the weather of the moment is added on top of it.
//
// Without the table the wind is derived from the sea zones of the
// configuration: the value is the inverse-distance weighted blend of the
// surrounding sea zones' baselines, plus the active weather's perturbation
// of the wind. Blending keeps the field continuous while staying a
// closed-form expression that can be evaluated cheaply.
func (w *World) currentWindAt(x, y float64) (windX, windY, curX, curY float64) {
	// --- the wind of the original game ---
	if wind := w.cfg.Wind; wind != nil {
		return w.authenticWindAt(wind, x, y)
	}

	type zoneSample struct {
		weight float64
		wind   float64
		dir    float64
		cur    float64
		cdir   float64
	}
	// Reuse a small stack slice: at most a handful of zones matter.
	var samples [16]zoneSample
	n := 0
	totalWeight := 0.0

	for _, zid := range w.cfgZoneOrder() {
		z := w.cfg.SeaZones[zid]
		dx := x - z.CenterX
		dy := y - z.CenterY
		dist := math.Hypot(dx, dy)
		// Zones outside their radius contribute nothing.
		if z.Radius > 0 && dist > z.Radius {
			continue
		}
		weight := 1.0
		if dist > 0 {
			weight = 1.0 / (1.0 + dist/1000.0)
		}
		if n < len(samples) {
			samples[n] = zoneSample{weight: weight, wind: z.WindBaseSpeed, dir: z.WindBaseDirDeg, cur: z.CurrentBaseSpeed, cdir: z.CurrentBaseDirDeg}
			n++
		}
		totalWeight += weight
	}
	if n == 0 || totalWeight == 0 {
		return 0, 0, 0, 0
	}

	var windVecX, windVecY, curVecX, curVecY float64
	var wSpeed, cSpeed float64
	for i := 0; i < n; i++ {
		s := samples[i]
		wSpeed += s.wind * s.weight
		cSpeed += s.cur * s.weight
		// Directions are blended as unit vectors to avoid the 0/360 wrap
		// problem.
		wx, wy := vectorFromPolar(1, s.dir)
		cx, cy := vectorFromPolar(1, s.cdir)
		windVecX += wx * s.weight
		windVecY += wy * s.weight
		curVecX += cx * s.weight
		curVecY += cy * s.weight
	}

	windSpeed := wSpeed / totalWeight
	curSpeedAvg := cSpeed / totalWeight

	// Add the active weather's wind on top of the climatic baseline.
	if wc := w.weatherComponent(); wc != nil {
		windSpeed += wc.WindSpeed
		curSpeedAvg += wc.CurrentSpeed
		// The weather direction replaces the blended direction when the
		// weather contributes a significant share of the wind.
		if wc.WindSpeed > 0 {
			blend := wc.WindSpeed / (windSpeed + wc.WindSpeed)
			wx, wy := vectorFromPolar(1, wc.WindDirDeg)
			windVecX = windVecX*(1-blend) + wx*blend
			windVecY = windVecY*(1-blend) + wy*blend
		}
		if wc.CurrentSpeed > 0 {
			blend := wc.CurrentSpeed / (curSpeedAvg + wc.CurrentSpeed)
			cx, cy := vectorFromPolar(1, wc.CurrentDirDeg)
			curVecX = curVecX*(1-blend) + cx*blend
			curVecY = curVecY*(1-blend) + cy*blend
		}
	}

	windX = windVecX * windSpeed
	windY = windVecY * windSpeed
	curX = curVecX * curSpeedAvg
	curY = curVecY * curSpeedAvg
	return windX, windY, curX, curY
}

// authenticWindAt returns the wind and the current of the original game at
// a map position.
//
// The table of the original game gives the direction and the strength of
// the wind of a sea area, and the direction and the strength of its
// current. The direction is a number from 0 to 7 counted clockwise from the
// north, and the strength is a number from 0 to 15.
//
// The strength of the table is turned into the knots of the game with the
// same factor that the table of the hulls uses, so that the drive of a
// hull is the one of the original game. The weather of the moment is
// added on top of the wind of the table, as it is in the original game:
// a storm adds to the wind and does not replace it.
//
// The position is in the pixels of the world map of the project, and the
// sea areas of the table are 72 by 72 tiles of 32 pixels.
func (w *World) authenticWindAt(wind *config.WorldWind, x, y float64) (windX, windY, curX, curY float64) {
	// --- the sea area that holds the position ---
	tileX := int(x / worldTilePixels)
	tileY := int(y / worldTilePixels)
	summer := w.isSummer()
	direction, strength, current := wind.SeaArea(tileX, tileY, summer)

	// --- the wind of the table ---
	// A direction of the table is a quarter turn of 45 degrees, counted
	// clockwise from the north.
	bearing := float64(direction) * 45
	units := windKnots(strength)

	// --- the weather of the moment ---
	weatherSpeed := 0.0
	weatherBearing := 0.0
	if wc := w.weatherComponent(); wc != nil {
		weatherSpeed = wc.WindSpeed
		weatherBearing = wc.WindDirDeg
	}
	if weatherSpeed > 0 {
		// The weather is added to the wind of the table, and it wins the
		// direction when it is the stronger of the two.
		units += weatherSpeed
		if weatherSpeed*2 > units {
			bearing = weatherBearing
		}
	}

	wx, wy := vectorFromPolar(units, bearing)
	windX, windY = wx, wy

	// --- the current of the table ---
	curBearing := float64(current) * 45
	curUnits := windKnots(wind.CurrentStrengthAt(tileX/config.WindSeaAreaSize, tileY/config.WindSeaAreaSize))
	if wc := w.weatherComponent(); wc != nil && wc.CurrentSpeed > 0 {
		curUnits += wc.CurrentSpeed
	}
	cx, cy := vectorFromPolar(curUnits, curBearing)
	curX, curY = cx, cy
	return windX, windY, curX, curY
}

// windKnots turns the strength of the original game into the knots that the
// wind vector of this project carries.
//
// The strength of the original game runs from zero to fifteen, and the
// sailing model works on the scale of zero to seven. The vector of the
// wind carries knots, and the caller turns the knots back into the scale of
// the model, so the strength is scaled to that scale and then into knots:
// a strength of fifteen is the strongest wind there is, which is seven
// units of the model, and so 24.5 knots.
func windKnots(strength int) float64 {
	if strength <= 0 {
		return 0
	}
	units := float64(strength) * sail.MaxWindUnits / float64(config.WindMaxStrength)
	knots := units * sail.KnotsPerWindUnit
	// The knots of the strongest wind there is.
	if max := sail.MaxWindUnits * sail.KnotsPerWindUnit; knots > max {
		knots = max
	}
	return knots
}

// isSummer reports whether the current day of the year lies in the summer.
//
// The original game tells the two seasons apart by the month, and the
// months from March to August are the summer. The game counts the days of
// the year from the first of March, so the first third of the year is the
// summer.
func (w *World) isSummer() bool {
	daysPerYear := w.cfg.Params.DaysPerGameYear
	if daysPerYear <= 0 {
		return true
	}
	// The first day of March is the first day of the year of the game, so
	// the summer is the first half of it.
	return w.day <= daysPerYear/2
}

// worldTilePixels is the size of one tile of the world map in pixels.
//
// The map of the original game is 2160 columns by 1080 rows of tiles of 32
// pixels, and the sea areas of the wind are 72 by 72 tiles.
const worldTilePixels = 32

// cfgZoneOrder returns the sea zone IDs in deterministic order.
func (w *World) cfgZoneOrder() []string {
	if w.zoneOrder == nil {
		w.zoneOrder = make([]string, 0, len(w.cfg.SeaZones))
		for id := range w.cfg.SeaZones {
			w.zoneOrder = append(w.zoneOrder, id)
		}
		sortStrings(w.zoneOrder)
	}
	return w.zoneOrder
}

// weatherComponent returns the active weather component, or nil.
func (w *World) weatherComponent() *ecsWeatherComponent {
	if w.weatherEntity == 0 {
		return nil
	}
	return getWeatherComponent(w.ecs, w.weatherEntity)
}

// advanceFleet moves a fleet for one day.
//
// The computation is:
//
//  1. Read the hull class, weather penalty and sea zone.
//  2. Evaluate the wind and current vectors at the fleet position.
//  3. Compute the over-the-ground velocity.
//  4. Turn the heading towards the desired course at the hull turn rate.
//  5. Integrate the position.
//  6. Consume food and water, apply fatigue and loyalty changes.
//
// All parameters come from the configuration tables.
func (w *World) advanceFleet(id ecsID, desiredHeading float64) {
	fleet := getFleetComponent(w.ecs, id)
	pos := getPositionComponent(w.ecs, id)
	if fleet == nil || pos == nil {
		return
	}
	ship := w.cfg.Ship(fleet.ShipID)
	if ship == nil {
		return
	}

	// --- wind and current ---
	windX, windY, curX, curY := w.currentWindAt(pos.X, pos.Y)

	// --- weather penalty ---
	weatherPenalty := 1.0
	if wc := w.weatherComponent(); wc != nil {
		if tmpl := w.cfg.Weather[wc.TemplateID]; tmpl != nil {
			weatherPenalty = tmpl.SpeedPenalty
		}
	}

	// --- turn the hull towards the desired heading ---
	turn := ship.TurnSpeedDegPerDay
	if turn > 0 {
		delta := normalizeAngle(desiredHeading - pos.Heading)
		maxTurn := turn
		if delta > maxTurn {
			delta = maxTurn
		} else if delta < -maxTurn {
			delta = -maxTurn
		}
		pos.Heading = normalizeAngle(pos.Heading + delta)
	}

	// --- speed over ground ---
	// The sailing model of the original game: the wind is read as a
	// direction on the eight point compass and a strength on a 0..7
	// scale, and the hull answers through the sail type table.
	windKnots := math.Hypot(windX, windY)
	windBearing := 0.0
	if windKnots > windSpeedEpsilon {
		windBearing = normalizeAngle(math.Atan2(windX, -windY) * 180 / math.Pi)
	}
	hullSpeed := w.sailingSpeed(id, fleet, ship, pos, windBearing, sail.WindUnits(windKnots)) * weatherPenalty

	// The hull contributes along the heading, the current adds a vector.
	vx, vy := vectorFromPolar(hullSpeed, pos.Heading)
	vx += curX
	vy += curY

	// Convert knots into map pixels per day using the global scale.
	scale := w.cfg.Params.TravelSpeedScale
	pos.X += vx * scale
	pos.Y += vy * scale
	pos.Speed = math.Hypot(vx, vy)

	// --- consumption ---
	w.consumeSupplies(fleet, ship)

	// --- fatigue and loyalty ---
	w.updateCrewState(fleet, ship)
}

// consumeSupplies consumes food and water for one day.
//
//	food   = sailors * ship.FoodPerSailorPerDay
//	water  = sailors * ship.WaterPerSailorPerDay
//
// Missing supplies translate into fatigue and morale damage.
func (w *World) consumeSupplies(fleet *ecsFleetComponent, ship *configShip) {
	needFood := float64(fleet.Sailors) * ship.FoodPerSailorPerDay
	needWater := float64(fleet.Sailors) * ship.WaterPerSailorPerDay

	foodShort := 0.0
	if fleet.Food < needFood {
		foodShort = needFood - fleet.Food
		fleet.Food = 0
	} else {
		fleet.Food -= needFood
	}
	waterShort := 0.0
	if fleet.FreshWater < needWater {
		waterShort = needWater - fleet.FreshWater
		fleet.FreshWater = 0
	} else {
		fleet.FreshWater -= needWater
	}

	// A shortage adds fatigue and lowers morale.
	shortage := foodShort + waterShort
	if shortage > 0 {
		penalty := shortage / math.Max(1, float64(fleet.Sailors))
		fleet.Fatigue += penalty * 10
		fleet.Morale -= penalty * 5
	}
}

// updateCrewState applies the daily fatigue, morale and loyalty drift.
//
//	fatigue += FatiguePerDaySail        (at sea)
//	fatigue -= FatiguePerDayRest        (in port)
//	loyalty  = clamp(loyalty + payBonus - decay)
//	mutiny probability = f(fatigue, loyalty)
func (w *World) updateCrewState(fleet *ecsFleetComponent, ship *configShip) {
	params := w.cfg.Params
	if fleet.InPort != "" {
		fleet.Fatigue -= params.FatiguePerDayRest
	} else {
		fleet.Fatigue += params.FatiguePerDaySail
	}
	if fleet.Fatigue < 0 {
		fleet.Fatigue = 0
	}
	if fleet.Fatigue > 100 {
		fleet.Fatigue = 100
	}

	// Loyalty: paid sailors gain loyalty proportional to the fleet's fame;
	// unpaid sailors lose it.
	fame := w.fameOf(fleet.Owner)
	loyaltyDelta := params.LoyaltyBonusPerFamePoint * fame / 100
	if fleet.Loyalty < 50 {
		loyaltyDelta -= params.LoyaltyDecayPerDay
	}
	fleet.Loyalty += loyaltyDelta
	if fleet.Loyalty < 0 {
		fleet.Loyalty = 0
	}
	if fleet.Loyalty > 100 {
		fleet.Loyalty = 100
	}

	// Morale recovers slowly when rested and in good conditions.
	if fleet.InPort != "" {
		fleet.Morale += 1
	}
	if fleet.Morale > 100 {
		fleet.Morale = 100
	}
	if fleet.Morale < 0 {
		fleet.Morale = 0
	}
}

// mutinyChance returns the daily probability of a mutiny.
//
//	P(mutiny) = MutinyBaseChance
//	          * clamp01((fatigue - MutinyThreshold) / (100 - MutinyThreshold))
//	          * (1 + MutinyLoyaltyFactor * (1 - loyalty/100))
func (w *World) mutinyChance(fleet *ecsFleetComponent) float64 {
	params := w.cfg.Params
	if fleet.Fatigue < params.MutinyThreshold {
		return 0
	}
	span := 100 - params.MutinyThreshold
	if span <= 0 {
		return 0
	}
	fatigueTerm := (fleet.Fatigue - params.MutinyThreshold) / span
	loyaltyTerm := 1 + params.MutinyLoyaltyFactor*(1-fleet.Loyalty/100)
	chance := params.MutinyBaseChance * fatigueTerm * loyaltyTerm
	if chance < 0 {
		return 0
	}
	if chance > 1 {
		return 1
	}
	return chance
}

// fameOf returns the total fame of a faction.
//
// The reputation is stored on a per-faction entity; the world keeps the
// mapping from faction ID to entity ID.
func (w *World) fameOf(factionID string) float64 {
	entID, ok := w.factionEntities[factionID]
	if !ok {
		return 0
	}
	rep := getReputationComponent(w.ecs, entID)
	if rep == nil {
		return 0
	}
	return rep.Adventure + rep.Trade + rep.Naval
}

// normalizeAngle normalises an angle in degrees to [0, 360).
func normalizeAngle(a float64) float64 {
	a = math.Mod(a, 360)
	if a < 0 {
		a += 360
	}
	return a
}

// bearingBetween returns the compass bearing from (x1,y1) to (x2,y2).
func bearingBetween(x1, y1, x2, y2 float64) float64 {
	return math.Atan2(x2-x1, -(y2-y1)) * 180 / math.Pi
}

// distanceBetween returns the Euclidean distance between two points.
func distanceBetween(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x2-x1, y2-y1)
}
