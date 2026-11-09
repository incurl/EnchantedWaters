package etl

import (
	"math"
	"testing"
)

// TestMapToLatLonMatchesTheOriginal checks the conversion of a map position
// into geographic coordinates.
//
// The conversion is the one of the original game, recovered from a third
// party trainer that prints the position of the fleet as a bearing. The
// positions below are the ones that the original gives for the ports, and
// they are within a degree of the real places.
func TestMapToLatLonMatchesTheOriginal(t *testing.T) {
	cases := []struct {
		name     string
		x, y     float64
		lat, lon float64
	}{
		{"Lisbon", 120, 358, 39, -9},
		{"Seville", 142, 372, 37, -6},
		{"Istanbul", 352, 344, 41, 28},
		{"Barcelona", 194, 342, 41, 2},
		{"Algiers", 200, 376, 36, 3},
		{"Tunis", 242, 372, 37, 10},
		{"Marseille", 212, 326, 43, 5},
		{"Genoa", 230, 320, 44, 8},
		{"Venice", 258, 318, 44, 13},
		{"Macao", 862, 474, 23, 113},
		{"Nagasaki", 956, 402, 33, 129},
		{"Cape Town", 286, 860, -30, 17},
		{"Montevideo", 1956, 906, -37, -63},
	}
	for _, c := range cases {
		lat, lon := mapToLatLon(c.x, c.y)
		if math.Abs(lat-c.lat) > 0.01 || math.Abs(lon-c.lon) > 0.01 {
			t.Errorf("%s: got %v,%v, want %v,%v", c.name, lat, lon, c.lat, c.lon)
		}
	}
}

// TestMapToLatLonStaysInRange checks the edges of the map.
//
// The map is 2160 by 1076 pixels, and the game names its edges the way it
// does: the eastern edge is 29 degrees west again, because the map wraps
// around the globe, and the equator runs at the row 640.
func TestMapToLatLonStaysInRange(t *testing.T) {
	cases := []struct {
		x, y     float64
		lat, lon float64
	}{
		{0, 0, 89, -29},
		{2160, 0, 89, -29},
		{0, 1076, -61, -29},
		{2160, 1076, -61, -29},
	}
	for _, c := range cases {
		lat, lon := mapToLatLon(c.x, c.y)
		if math.Abs(lat-c.lat) > 0.01 || math.Abs(lon-c.lon) > 0.01 {
			t.Errorf("(%v,%v): got %v,%v, want %v,%v", c.x, c.y, lat, lon, c.lat, c.lon)
		}
	}
}
