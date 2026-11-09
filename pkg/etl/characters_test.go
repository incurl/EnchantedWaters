package etl

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestCharacterAbilityColumnsAreTheEightStats verifies the meaning of the
// eight ability bytes of the character table.
//
// The reference implementation of the game names the eight stats of a
// sailor as leadership, seamanship, knowledge, intuition, courage,
// swordplay, charm and luck, and gives João Franco, Old Sea Hand Rocco and
// Brother Enrico as 78/75/73/85/82/82/89/50, 75/82/84/90/93/92/70/70 and
// 66/48/93/55/62/48/82/100. Those twenty four numbers pin the storage order
// of the bytes, which the message table alone had only half right: it
// suggested charm, skill and swordplay for the last three, but the last
// three are swordplay, charm and luck.
//
// The same three records carry their navigation level, their battle level
// and their age in the bytes that follow, which the test checks too.
func TestCharacterAbilityColumnsAreTheEightStats(t *testing.T) {
	path := filepath.Join(configDir(t), "original_characters.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Skip("the imported reference data is not present")
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(rows) < 2 {
		t.Fatal("the character table is empty")
	}
	header := map[string]int{}
	for i, name := range rows[0] {
		header[name] = i
	}

	// The eight stats, in storage order.
	stats := []string{"leadership", "seamanship", "knowledge", "intuition",
		"courage", "swordplay", "charm", "luck"}
	for _, s := range stats {
		if _, ok := header[s]; !ok {
			t.Fatalf("the character table has no %q column: the ability bytes must be "+
				"exported under their names", s)
		}
	}

	type want struct {
		first, last string
		stats       [8]int
		nav, battle int
		age         int
	}
	cases := []want{
		{"Joao", "Franco", [8]int{78, 75, 73, 85, 82, 82, 89, 50}, 1, 1, 18},
		{"Rocco", "Alemkel", [8]int{75, 82, 84, 90, 93, 92, 70, 70}, 30, 32, 65},
		{"Enrico", "Malione", [8]int{66, 48, 93, 55, 62, 48, 82, 100}, 1, 1, 24},
	}

	byName := map[string][]string{}
	for _, r := range rows[1:] {
		if len(r) < len(rows[0]) {
			continue
		}
		byName[r[header["first_name"]]+" "+r[header["last_name"]]] = r
	}

	for _, c := range cases {
		key := c.first + " " + c.last
		r, ok := byName[key]
		if !ok {
			t.Errorf("%s is missing from the character table", key)
			continue
		}
		atoi := func(col string) int {
			v, _ := strconv.Atoi(r[header[col]])
			return v
		}
		for i, s := range stats {
			if got := atoi(s); got != c.stats[i] {
				t.Errorf("%s: %s = %d, want %d", key, s, got, c.stats[i])
			}
		}
		if got := atoi("navigation_level"); got != c.nav {
			t.Errorf("%s: navigation_level = %d, want %d", key, got, c.nav)
		}
		if got := atoi("battle_level"); got != c.battle {
			t.Errorf("%s: battle_level = %d, want %d", key, got, c.battle)
		}
		if got := atoi("age"); got != c.age {
			t.Errorf("%s: age = %d, want %d", key, got, c.age)
		}
	}
}

// TestCharacterTableRanges verifies that the decoded columns stay inside
// the ranges the original can produce, which is what distinguishes a stat
// from a byte that happens to sit in that position.
func TestCharacterTableRanges(t *testing.T) {
	path := filepath.Join(configDir(t), "original_characters.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Skip("the imported reference data is not present")
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(rows) != 121 {
		t.Fatalf("the character table holds %d records, want 120", len(rows)-1)
	}
	header := map[string]int{}
	for i, name := range rows[0] {
		header[name] = i
	}

	cols := []struct {
		name     string
		min, max int
	}{
		{"leadership", 0, 100}, {"seamanship", 0, 100}, {"knowledge", 0, 100},
		{"intuition", 0, 100}, {"courage", 0, 100}, {"swordplay", 0, 100},
		{"charm", 0, 100}, {"luck", 0, 100},
		{"navigation_level", 1, 100}, {"battle_level", 1, 100},
		{"age", 15, 70},
	}
	for _, c := range cols {
		for _, r := range rows[1:] {
			v, err := strconv.Atoi(r[header[c.name]])
			if err != nil {
				t.Fatalf("%s: not a number: %q", c.name, r[header[c.name]])
			}
			if v < c.min || v > c.max {
				t.Errorf("%s = %d, outside the plausible range %d..%d", c.name, v, c.min, c.max)
				break
			}
		}
	}
}
