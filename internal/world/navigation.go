package world

import (
	"math"
)

// Sailing physics.
//
// The speed of a fleet over the ground is the vector sum of three
// contributions:
//
//	v_ground = v_hull * f_wind + v_current
//
// where v_hull is the hull speed multiplied by the wind efficiency factor
// and the weather penalty.
//
// The wind efficiency factor is the classic "sail angle" model: sailing
// with the wind (0 degrees relative) is fastest, sailing against it is
// impossible (the factor becomes 0), and reaching (90 degrees) is about
// 60-70% efficient:
//
//	f(θ) = cos(θ)                       for |θ| <= 90 degrees
//	f(θ) = 0                           for |θ| > 90 degrees (no-go zone)
//
// with θ being the angle between the heading and the wind direction. The
// function is smoothed near the boundaries by a cubic blend so that the
// speed does not change discontinuously when the ship tacks.
//
// The current contribution is added as a vector: a following current speeds
// the fleet up, a head current slows it down, and a beam current mostly
// pushes it sideways (which is handled by the heading update).
//
// The parameters (base speeds, weather penalties, turn rates) all come
// from the configuration tables.

// windEfficiency returns the sailing efficiency for a heading/wind angle.
//
// theta is the absolute angle between the ship heading and the wind
// direction, in degrees (0..180).
//
// The curve has three parts:
//
//	f(θ) = cos(θ)                    for θ ≤ blendStart
//	f(θ) = blend(cos, linear ramp)    for blendStart < θ ≤ 90
//	f(θ) = 0                          for θ > 90   (the no-go zone)
//
// The blend start and the floor of the linear ramp come from the
// configuration table, so that the shape of the curve can be tuned
// without touching the code.
func windEfficiency(theta float64) float64 {
	return windEfficiencyWith(theta, windBlendStart, windRampFloor)
}

// windSpeedEpsilon is the wind speed below which the wind direction is
// considered undefined.
const windSpeedEpsilon = 0.001

// windBlendStart is the angle (in degrees) at which the linear ramp of
// the wind efficiency starts.
//
// It is the default; the world overrides it with the value of the
// configuration table.
const windBlendStart = 60.0

// windRampFloor is the efficiency at the beam reach, before the linear
// ramp brings it down to zero.
const windRampFloor = 0.5

// windEfficiencyWith evaluates the curve with the given parameters.
func windEfficiencyWith(theta, blendStart, rampFloor float64) float64 {
	if theta >= 90 {
		// Inside the no-go zone the ship cannot make headway.
		return 0
	}
	cosPart := math.Cos(theta * math.Pi / 180)
	if blendStart <= 0 {
		blendStart = 90
	}
	if theta <= blendStart {
		return cosPart
	}
	// The linear ramp goes from the floor at the blend start down to zero
	// at the no-go zone.
	t := (theta - blendStart) / (90 - blendStart)
	linear := rampFloor * (1 - t)
	return cosPart*(1-t) + linear*t
}

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
// The value is the inverse-distance weighted blend of the surrounding sea
// zones' baselines, plus the active weather's perturbation of the wind.
// Blending keeps the field continuous while staying a closed-form
// expression that can be evaluated cheaply.
func (w *World) currentWindAt(x, y float64) (windX, windY, curX, curY float64) {
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

// windEfficiency evaluates the sailing efficiency curve with the
// parameters of the configuration table.
func (w *World) windEfficiency(theta float64) float64 {
	blend := w.cfg.Params.WindBlendStart
	if blend <= 0 {
		blend = windBlendStart
	}
	floor := w.cfg.Params.WindEfficiencyFloor
	if floor <= 0 {
		floor = windRampFloor
	}
	return windEfficiencyWith(theta, blend, floor)
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
	// The wind vector is converted into the wind's "angle to the bow".
	windSpeed := math.Hypot(windX, windY)
	theta := 0.0
	if windSpeed > windSpeedEpsilon {
		windBearing := math.Atan2(windX, -windY) * 180 / math.Pi
		theta = math.Abs(normalizeAngle(windBearing - pos.Heading))
		if theta > 180 {
			theta = 360 - theta
		}
	}
	eff := w.windEfficiency(theta)
	hullSpeed := ship.BaseSpeed * eff * weatherPenalty

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
