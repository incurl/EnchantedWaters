// This file derives the playable game tables from the imported reference
// data of the original game.
//
// # What is derived and what is invented
//
// The *layout* of the world -- the name and the position of every port --
// comes from the original game. It is reference data: the map should be
// the map of the original.
//
// The *rules* -- the tax of a port, the facilities it offers, the goods it
// produces and the price of those goods -- are the balance of this
// project. They are derived from the position of the port through the
// documented rules below, so that the result is reproducible and every
// number can be explained.
//
// Nothing here is copied from the original game's code: the derivation is
// written from scratch and the reasoning is written next to it.
package etl

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// MapWidth and MapHeight are the size of the world map of the original
// game, in its own coordinate space. The port coordinates are expressed
// in that space, so the derived tables use it directly.
const (
	MapWidth  = 2160
	MapHeight = 960
)

// Region is a sea region of the world map.
type Region struct {
	// ID is the identifier of the region.
	ID string
	// Name is the display name.
	Name string
	// MinX / MinY / MaxX / MaxY bound the region on the map.
	MinX, MinY, MaxX, MaxY float64
}

// Regions are the sea regions of the world, as bounding boxes on the map
// of the original game.
//
// The boxes were calibrated against the positions of the ports: every box
// contains the ports of one geographic region, and the boxes are listed
// from the most specific to the most general, because the first match
// wins.
var Regions = []Region{
	{"japan", "Japan", 950, 300, 1100, 430},
	{"china", "China", 780, 340, 980, 520},
	{"south_east_asia", "South East Asia", 700, 570, 980, 730},
	{"india", "India", 560, 440, 700, 620},
	{"arabia", "Arabia", 400, 400, 560, 560},
	{"east_africa", "East Africa", 270, 600, 500, 900},
	{"west_africa", "West Africa", 40, 400, 280, 720},
	{"northern_europe", "Northern Europe", 130, 110, 330, 270},
	{"atlantic", "Atlantic Europe", 40, 190, 179, 400},
	{"mediterranean", "Mediterranean", 175, 290, 440, 440},
	{"caribbean", "Caribbean", 1700, 480, 2050, 660},
	{"south_america", "South America", 1650, 660, 2160, 960},
	{"north_america", "North America", 1300, 100, 2000, 480},
	{"arctic", "Arctic", 300, 40, 1250, 140},
	{"oceania", "Australia and Oceania", 1000, 600, 1500, 960},
	{"pacific", "Pacific", 1000, 300, 1700, 600},
}

// The map position to the geographic coordinates.
//
// The map of the original game is a hand drawn world map, not a true
// projection. The conversion below was calibrated against the known
// positions of the ports:
//
//	longitude  London (0 east) sits at x=180 and Nagasaki (130 east) at
//	           x=956, which gives 0.1666 degrees per pixel, and it fits
//	           every other port within a few degrees;
//	latitude   the fit goes through Narvik (68 north) and Cape Town
//	           (34 south); the middle latitudes are compressed on the
//	           original map, so the latitude is accurate at the extremes
//	           and up to about eight degrees off around the tropics.
//
// The approximation is good enough for what the simulation uses it for:
// the day length and the band of the wind system a sea zone belongs to.
const (
	// lonOriginX / lonPerPixel map the horizontal position to a longitude.
	lonOriginX  = 180.0
	lonPerPixel = 360.0 / 2160.0
	// latOriginY / latPerPixel map the vertical position to a latitude.
	latOriginY  = 114.0
	latPerPixel = 0.1371
	// latAtOrigin is the latitude at latOriginY.
	latAtOrigin = 68.4
)

// mapToLatLon converts a map position into geographic coordinates.
func mapToLatLon(x, y float64) (lat, lon float64) {
	lon = (x - lonOriginX) * lonPerPixel
	// The longitude is wrapped into the -180..180 range.
	for lon > 180 {
		lon -= 360
	}
	for lon < -180 {
		lon += 360
	}
	lat = latAtOrigin - (y-latOriginY)*latPerPixel
	if lat > 90 {
		lat = 90
	}
	if lat < -90 {
		lat = -90
	}
	return lat, lon
}

// regionOf returns the region of a map position.
func regionOf(x, y float64) Region {
	for _, r := range Regions {
		if x >= r.MinX && x <= r.MaxX && y >= r.MinY && y <= r.MaxY {
			return r
		}
	}
	// The closest region is used when the point falls between the boxes.
	best := Regions[0]
	bestDist := math.Inf(1)
	for _, r := range Regions {
		cx := (r.MinX + r.MaxX) / 2
		cy := (r.MinY + r.MaxY) / 2
		if d := math.Hypot(x-cx, y-cy); d < bestDist {
			bestDist = d
			best = r
		}
	}
	return best
}

// capitalPorts are the ports that this project treats as a capital: the
// seat of a nation and the largest trade hub of its region.
//
// The list is a design choice of this project, not data from the
// original game.
var capitalPorts = map[string]bool{
	"Lisbon": true, "Seville": true, "London": true, "Amsterdam": true,
	"Venice": true, "Istanbul": true, "Nagasaki": true, "Sakai": true,
	"Macao": true, "Malacca": true, "Goa": true, "Havana": true,
}

// majorPorts are the ports of the second rank: the regional trade hubs
// that are not a capital.
//
// The list is a design choice of this project.
var majorPorts = map[string]bool{
	"Genoa": true, "Naples": true, "Barcelona": true, "Marseille": true,
	"Alexandria": true, "Tunis": true, "Ragusa": true, "Athens": true,
	"Antwerp": true, "Copenhagen": true, "Hamburg": true, "Danzig": true,
	"Stockholm": true, "Bergen": true, "Calicut": true, "Cochin": true,
	"Ceylon": true, "Aden": true, "Hormuz": true, "Basra": true,
	"Muscat": true, "Macao": true, "Hanoi": true, "Changan": true,
	"Ternate": true, "Cairo": true, "Mecca": true, "Luanda": true,
	"Mozambique": true, "Veracruz": true, "Cartegena": true, "Panama": true,
	"Rio de Janeiro": true, "Cape Town": true, "Tahiti": true,
}

// portClass returns the class of a port.
//
// The class drives the facilities, the tax and the discount of the port.
func portClass(name string) string {
	if capitalPorts[name] {
		return "capital"
	}
	if majorPorts[name] {
		return "city"
	}
	return "town"
}

// specialtyOf returns the specialty of a region.
//
// Every region produces one good in abundance; the good is the one that
// history and geography associate with it, which is also what makes the
// long distance trade of the game interesting.
func specialtyOf(regionID string) string {
	switch regionID {
	case "northern_europe":
		return "stockfish"
	case "atlantic":
		return "cloth"
	case "mediterranean":
		return "wine"
	case "black_sea":
		return "grain"
	case "west_africa":
		return "salt"
	case "east_africa":
		return "gems"
	case "arabia":
		return "silver"
	case "india":
		return "pepper"
	case "south_east_asia":
		return "spice"
	case "china":
		return "silk"
	case "japan":
		return "sake"
	case "north_pacific":
		return "timber"
	case "pacific":
		return "tatami"
	case "caribbean":
		return "noren"
	case "south_america":
		return "lacquer"
	case "north_america":
		return "tea_house"
	case "australia_oceania":
		return "tea"
	default:
		return "porcelain"
	}
}

// OriginalPort is one port of the imported reference data.
type OriginalPort struct {
	// MapX / MapY is the position on the map of the original game.
	MapX, MapY int
	// Name is the name of the port.
	Name string
	// Raw19 is the byte of the original record that follows the name. Its
	// meaning is not confirmed, so it is kept for reference only.
	Raw19 int
}

// ReadOriginalPorts reads the imported port table.
func ReadOriginalPorts(path string) ([]OriginalPort, error) {
	rows, err := readTable(path)
	if err != nil {
		return nil, err
	}
	out := make([]OriginalPort, 0, len(rows))
	for i, row := range rows {
		name := row["name"]
		if name == "" {
			continue
		}
		x, err1 := strconv.Atoi(row["map_x"])
		y, err2 := strconv.Atoi(row["map_y"])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("port %d: invalid coordinates", i)
		}
		raw, _ := strconv.Atoi(row["raw_19"])
		out = append(out, OriginalPort{MapX: x, MapY: y, Name: name, Raw19: raw})
	}
	return out, nil
}

// PortID builds the identifier of a derived port.
//
// The identifier is stable: it is the lower case name with the characters
// that are not alphanumeric replaced by an underscore.
func PortID(name string) string {
	var sb strings.Builder
	sb.WriteString("ports.")
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// The port attribute byte.
//
// The byte that follows the name of a port is the attribute byte of the
// original record. The meaning of its bits was recovered by the analysis
// of the original engine that the Water1-MapViewer project documents (the
// engine of the first Uncharted Waters is the predecessor of this one, and
// the port record has the same size):
//
//	0x10  the court ruler facility is available
//	0x08  the port is discovered at the start of the game
//	0x04  the guild is available
//	0x03  the low two bits: the initial political code
//
// The court bit is the one this project can use: a port without a court
// has no governor to visit, exactly as in the original game.
//
// The guild bit is understood but not used: the Guild is a facility of its
// own in the original, while the Pub -- the tavern of this project -- is a
// different building that exists in every port, so the tavern level keeps
// following the class of the port.
//
// The low two bits are kept in the reference table but not interpreted,
// because the political codes of the second game differ from the first.
const (
	portAttrCourt    = 0x10
	portAttrGuild    = 0x04
	portAttrDiscoved = 0x08
)

// supplyPortCount is the number of records at the end of the original port
// table that are supply ports rather than towns.
//
// The table holds 130 records: 100 towns with a full set of buildings,
// then 30 anchorages that offer nothing but a harbour. The split is
// positional -- the last thirty records of the table are the anchorages --
// and it was confirmed against the reference implementation, which lists
// exactly those thirty names as supply ports and the other hundred as
// regular ports.
const supplyPortCount = 30

// DerivePorts generates assets/config/ports.csv from the imported port
// table of the original game.
//
// The layout is the original one; the balance is derived here:
//
//	tax           0.05 for a town, 0.07 for a city, 0.09 for a capital
//	ship discount 1.00 / 0.95 / 0.90 by class
//	facilities    a step up with the class
//	supply ports  the last thirty records: a harbour and nothing else
//	specialty     one good per region, the good the region is known for
//	latitude      derived from the map position with an equirectangular
//	              approximation, which is what the navigation needs
func DerivePorts(cfgDir string) (int, error) {
	ports, err := ReadOriginalPorts(filepath.Join(cfgDir, "original_ports.csv"))
	if err != nil {
		return 0, err
	}
	if len(ports) == 0 {
		return 0, fmt.Errorf("the imported port table is empty")
	}

	// The rows are written in the order of the original table, so that the
	// result is stable and easy to compare with the reference data.
	rows := [][]string{{
		"id", "name", "name_jp", "class", "region", "x", "y", "lat", "lon",
		"specialty_commodity", "specialty_discount", "base_tax_rate", "ship_discount",
		"shipyard_level", "tavern_level", "governor_level", "church_level", "inn_level",
		"supply_port", "price_memory_factor",
	}}

	seen := make(map[string]bool, len(ports))
	for i, p := range ports {
		id := PortID(p.Name)
		if seen[id] {
			return 0, fmt.Errorf("the port id %q is used twice", id)
		}
		seen[id] = true

		region := regionOf(float64(p.MapX), float64(p.MapY))
		class := portClass(p.Name)

		// The last thirty records of the table are anchorages, not towns.
		isSupply := supplyPortCount > 0 && i >= len(ports)-supplyPortCount

		// The balance of the port by class.
		var tax, shipDiscount float64
		var shipyard, tavern, governor, church, inn, memory int
		switch class {
		case "capital":
			tax, shipDiscount = 0.09, 0.90
			shipyard, tavern, governor, church, inn, memory = 5, 5, 5, 5, 5, 10
		case "city":
			tax, shipDiscount = 0.07, 0.95
			shipyard, tavern, governor, church, inn, memory = 4, 4, 4, 3, 4, 9
		default:
			tax, shipDiscount = 0.05, 1.00
			shipyard, tavern, governor, church, inn, memory = 3, 3, 3, 2, 3, 7
		}

		// The original attribute byte decides whether the port has a
		// court and a guild. A port without a court has no governor to
		// visit, exactly as in the original game.
		if p.Raw19&portAttrCourt == 0 {
			governor = 0
		} else if governor < 2 {
			// A court in a small port still offers the governor screen.
			governor = 2
		}
		// An anchorage has a harbour and nothing else: no market, no
		// shipyard, no inn. It exists to take on water and food.
		if isSupply {
			shipyard, tavern, governor, church, inn = 0, 0, 0, 0, 0
		}
		// The latitude and the longitude are read off the map with the
		// calibrated conversion (see mapToLatLon).
		lat, lon := mapToLatLon(float64(p.MapX), float64(p.MapY))

		rows = append(rows, []string{
			id,
			p.Name,
			"",
			class,
			region.ID,
			strconv.Itoa(p.MapX),
			strconv.Itoa(p.MapY),
			formatFloat(lat, 1),
			formatFloat(lon, 1),
			specialtyOf(region.ID),
			"0.18",
			formatFloat(tax, 2),
			formatFloat(shipDiscount, 2),
			strconv.Itoa(shipyard),
			strconv.Itoa(tavern),
			strconv.Itoa(governor),
			strconv.Itoa(church),
			strconv.Itoa(inn),
			boolField(isSupply),
			formatFloat(float64(memory)/10, 1),
		})
	}

	if err := writeTable(filepath.Join(cfgDir, "ports.csv"), rows); err != nil {
		return 0, err
	}
	return len(rows) - 1, nil
}

// formatFloat formats a float with the given number of decimals, without
// the trailing zeros.
func formatFloat(v float64, decimals int) string {
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "-0" {
		return "0"
	}
	return s
}

// boolField formats a boolean as the 0 or 1 that the CSV loader expects.
func boolField(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// writeTable writes a table as CSV.
func writeTable(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	cw := csv.NewWriter(f)
	cw.UseCRLF = false
	if err := cw.WriteAll(rows); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return cw.Error()
}

// SortedPortNames returns the port names of the imported table, sorted.
//
// It is used by the tests to check that the derivation covers every port.
func SortedPortNames(ports []OriginalPort) []string {
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		out = append(out, p.Name)
	}
	sort.Strings(out)
	return out
}

// DerivePortPrices generates assets/config/port_prices.csv for every port
// of the derived table.
//
// # The rule
//
// Every port trades every commodity. What differs is the balance between
// the supply and the demand, and that balance is what creates the regional
// price gaps that make the trade worth doing:
//
//   - the good a region produces is abundant there, so it is cheap at the
//     source;
//   - a good with a strong regional identity is scarce away from its home
//     region, so it is expensive elsewhere;
//   - the size of the local market scales with the class of the port.
//
// The derivation is deterministic: no randomness is involved, so the
// generated table can be reproduced and reviewed.
func DerivePortPrices(cfgDir string) (int, error) {
	ports, err := ReadOriginalPorts(filepath.Join(cfgDir, "original_ports.csv"))
	if err != nil {
		return 0, err
	}
	commodities, err := readTable(filepath.Join(cfgDir, "commodities.csv"))
	if err != nil {
		return 0, err
	}
	if len(commodities) == 0 {
		return 0, fmt.Errorf("the commodity table is empty")
	}

	baseSupply := map[string]float64{"general": 30, "luxury": 8, "specialty": 10}
	baseDemand := map[string]float64{"general": 24, "luxury": 7, "specialty": 9}
	portScale := map[string]float64{"hamlet": 0.55, "town": 1.0, "city": 1.6, "capital": 2.2}

	rows := [][]string{{"port_id", "commodity_id", "factor", "supply", "demand"}}
	for _, p := range ports {
		region := regionOf(float64(p.MapX), float64(p.MapY))
		scale := portScale[portClass(p.Name)]
		if scale == 0 {
			scale = 1
		}
		produced := specialtyOf(region.ID)

		for _, c := range commodities {
			cid := c["id"]
			if cid == "" {
				continue
			}
			supply := baseSupply[c["class"]] * scale
			demand := baseDemand[c["class"]] * scale
			if supply == 0 {
				supply = 20 * scale
			}
			if demand == 0 {
				demand = 18 * scale
			}

			factor := 1.0
			if cid == produced {
				supply *= 4.0
				demand *= 0.35
				factor = 0.45
			} else if regionalGoods[cid] {
				supply *= 0.55
				demand *= 1.25
				factor = 1.35
			}

			rows = append(rows, []string{
				PortID(p.Name), cid,
				formatFloat(factor, 2),
				formatFloat(supply, 1),
				formatFloat(demand, 1),
			})
		}
	}
	if err := writeTable(filepath.Join(cfgDir, "port_prices.csv"), rows); err != nil {
		return 0, err
	}
	return len(rows) - 1, nil
}

// regionalGoods are the commodities that are strongly tied to one region
// of the world: cheap where they are made, expensive everywhere else.
var regionalGoods = map[string]bool{
	"silk": true, "spice": true, "gems": true, "tea": true, "porcelain": true,
	"silver": true, "pepper": true, "wine": true, "stockfish": true,
	"sake": true, "lacquer": true, "tatami": true, "noren": true, "tea_house": true,
}

// DeriveSeaZones generates assets/config/sea_zones.csv.
//
// The zones follow the great wind systems of the world:
//
//   - the trade winds blow from the east in the tropics;
//   - the prevailing westerlies blow from the west in the middle
//     latitudes;
//   - the ocean currents follow the same pattern.
//
// The zones are placed on the regions of the map, so that the sailing has
// the shape the geography suggests.
func DeriveSeaZones(cfgDir string) (int, error) {
	rows := [][]string{{
		"id", "name", "name_jp", "center_x", "center_y", "radius",
		"wind_base_speed", "wind_base_dir_deg", "current_base_speed",
		"current_base_dir_deg", "storm_probability", "pirate_probability", "latitude",
	}}
	for _, r := range Regions {
		cx := (r.MinX + r.MaxX) / 2
		cy := (r.MinY + r.MaxY) / 2
		radius := math.Hypot(r.MaxX-r.MinX, r.MaxY-r.MinY) / 2
		lat, _ := mapToLatLon(cx, cy)

		var windSpeed, windDir, curSpeed, curDir, storm, pirate float64
		switch {
		case math.Abs(lat) < 30:
			windSpeed, windDir = 12, 90
			curSpeed, curDir = 1.8, 270
			storm, pirate = 0.05, 0.10
		case lat >= 30 && lat < 60:
			windSpeed, windDir = 15, 270
			curSpeed, curDir = 2.0, 90
			storm, pirate = 0.09, 0.06
		default:
			windSpeed, windDir = 17, 240
			curSpeed, curDir = 1.4, 60
			storm, pirate = 0.11, 0.02
		}

		rows = append(rows, []string{
			"zone." + r.ID, r.Name, "",
			formatFloat(cx, 0), formatFloat(cy, 0), formatFloat(radius, 0),
			formatFloat(windSpeed, 0), formatFloat(windDir, 0),
			formatFloat(curSpeed, 1), formatFloat(curDir, 0),
			formatFloat(storm, 2), formatFloat(pirate, 2),
			formatFloat(lat, 0),
		})
	}
	if err := writeTable(filepath.Join(cfgDir, "sea_zones.csv"), rows); err != nil {
		return 0, err
	}
	return len(rows) - 1, nil
}

// DeriveAll runs every derivation in order.
//
// It returns a human readable summary of what was written.
func DeriveAll(cfgDir string) (string, error) {
	ports, err := DerivePorts(cfgDir)
	if err != nil {
		return "", err
	}
	prices, err := DerivePortPrices(cfgDir)
	if err != nil {
		return "", err
	}
	zones, err := DeriveSeaZones(cfgDir)
	if err != nil {
		return "", err
	}
	land, err := DeriveTerrain(cfgDir)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("derived %d ports, %d port prices, %d sea zones and %d land cells",
		ports, prices, zones, land), nil
}

// TerrainCellSize is the size of one cell of the coarse terrain grid, in
// pixels of the world map. The grid is coarser than the map on purpose:
// it is a land and sea mask, not a picture.
const TerrainCellSize = 20

// DeriveTerrain generates assets/config/world_terrain.csv.
//
// # The rule
//
// The sea is the default. A cell becomes land when it is close enough to
// a port, and the mask is then smoothed twice:
//
//   - a dilation joins the cells that the port discs almost reach, which
//     is what turns a tight cluster of ports into one landmass;
//   - an erosion then pulls the coast back, so that the outline follows the
//     ports instead of bulging around them.
//
// The result is a mask whose continents are drawn from the positions of
// the ports alone: no picture of the original game is involved.
//
// The radius is expressed in cells. It is chosen so that a region with a
// port every few cells becomes one landmass, while an isolated port stays
// an island.
func DeriveTerrain(cfgDir string) (int, error) {
	ports, err := ReadOriginalPorts(filepath.Join(cfgDir, "original_ports.csv"))
	if err != nil {
		return 0, err
	}
	if len(ports) == 0 {
		return 0, fmt.Errorf("the imported port table is empty")
	}

	w := (MapWidth + TerrainCellSize - 1) / TerrainCellSize
	h := (MapHeight + TerrainCellSize - 1) / TerrainCellSize

	// The seed: a filled disc of land around every port.
	//
	// The radius is expressed in cells. A larger radius joins the ports of
	// a region into one continent; a smaller one leaves them as separate
	// islands. 2.7 cells -- 54 pixels -- is the value at which the world
	// keeps its shape: Europe, Africa and Asia stay apart, while the
	// ports of each region share a coastline.
	const seedRadius = 2.7
	grid := make([]bool, w*h)
	for _, p := range ports {
		cx := float64(p.MapX) / TerrainCellSize
		cy := float64(p.MapY) / TerrainCellSize
		for y := int(cy - seedRadius - 1); y <= int(cy+seedRadius+1); y++ {
			for x := int(cx - seedRadius - 1); x <= int(cx+seedRadius+1); x++ {
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				if math.Hypot(float64(x)-cx, float64(y)-cy) <= seedRadius {
					grid[y*w+x] = true
				}
			}
		}
	}

	// The morphological close: a dilation followed by an erosion.
	grid = dilate(grid, w, h)
	grid = erode(grid, w, h)

	// The table is written as one row per grid line, which keeps the file
	// small and easy to inspect by eye.
	rows := [][]string{{"row", "terrain"}}
	landCells := 0
	for y := 0; y < h; y++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			if grid[y*w+x] {
				sb.WriteByte('#')
				landCells++
			} else {
				sb.WriteByte('.')
			}
		}
		rows = append(rows, []string{strconv.Itoa(y), sb.String()})
	}

	if err := writeTable(filepath.Join(cfgDir, "world_terrain.csv"), rows); err != nil {
		return 0, err
	}
	return landCells, nil
}

// dilate grows the land by one cell in every direction.
func dilate(grid []bool, w, h int) []bool {
	out := make([]bool, len(grid))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !grid[y*w+x] {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					out[ny*w+nx] = true
				}
			}
		}
	}
	return out
}

// erode shrinks the land by one cell, which pulls the coast back.
func erode(grid []bool, w, h int) []bool {
	out := make([]bool, len(grid))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !grid[y*w+x] {
				continue
			}
			keep := true
			for dy := -1; dy <= 1 && keep; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						keep = false
						break
					}
					if !grid[ny*w+nx] {
						keep = false
						break
					}
				}
			}
			out[y*w+x] = keep
		}
	}
	return out
}
