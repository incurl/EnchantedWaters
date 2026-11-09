// Package etl implements the standalone data validation tool.
//
// The tool checks the configuration tables of the project for the
// problems that would break the simulation:
//
//   - duplicate primary keys,
//   - dangling foreign keys,
//   - values outside of their valid range,
//   - missing mandatory columns.
//
// It is a separate command so that the data can be validated in a CI
// pipeline without starting the game.
package etl

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Problem describes one validation problem.
type Problem struct {
	// Table is the name of the table the problem was found in.
	Table string
	// Kind classifies the problem: "duplicate", "missing", "range",
	// "reference" or "format".
	Kind string
	// Detail describes the problem.
	Detail string
	// Row is the row of the table, when the problem is row specific.
	Row int
}

// String returns the human readable description of the problem.
func (p Problem) String() string {
	if p.Row > 0 {
		return fmt.Sprintf("%s: [%s] row %d: %s", p.Table, p.Kind, p.Row, p.Detail)
	}
	return fmt.Sprintf("%s: [%s] %s", p.Table, p.Kind, p.Detail)
}

// Report is the result of a validation run.
type Report struct {
	// Problems holds the problems that were found.
	Problems []Problem
	// Tables holds the number of rows of every checked table.
	Tables map[string]int
}

// OK reports whether the validation passed.
func (r *Report) OK() bool { return len(r.Problems) == 0 }

// String returns the human readable summary of the report.
func (r *Report) String() string {
	var sb strings.Builder
	names := make([]string, 0, len(r.Tables))
	for name := range r.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&sb, "  %-24s %5d rows\n", name, r.Tables[name])
	}
	if len(r.Problems) == 0 {
		sb.WriteString("  no problem found\n")
		return sb.String()
	}
	fmt.Fprintf(&sb, "  %d problem(s):\n", len(r.Problems))
	for _, p := range r.Problems {
		sb.WriteString("    " + p.String() + "\n")
	}
	return sb.String()
}

// Validate checks every table of a configuration directory.
//
// The checks are table specific: each table declares its primary key, its
// foreign keys and the ranges of its numeric columns.
func Validate(dir string) (*Report, error) {
	report := &Report{Tables: make(map[string]int)}

	for _, spec := range tableSpecs() {
		path := filepath.Join(dir, spec.Name+".csv")
		rows, err := readTable(path)
		if err != nil {
			if os.IsNotExist(err) {
				report.Problems = append(report.Problems, Problem{
					Table:  spec.Name,
					Kind:   "missing",
					Detail: "the table file does not exist",
				})
				continue
			}
			return nil, err
		}
		report.Tables[spec.Name] = len(rows)
		checkTable(spec, rows, report)
	}
	// The foreign keys are checked once every table has been read.
	CheckReferences(dir, report)
	return report, nil
}

// parseFloat parses a decimal number.
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// tableSpec describes the validation rules of a table.
type tableSpec struct {
	// Name is the base name of the table.
	Name string
	// Key is the column that holds the primary key.
	Key string
	// Required lists the columns that must be present.
	Required []string
	// References maps a column to the table it references.
	References map[string]string
	// Ranges maps a column to the valid range of its values.
	Ranges map[string][2]float64
}

// tableSpecs returns the validation rules of every table.
//
// The rules live here, next to the checker, so that the tool stays
// independent of the game runtime.
func tableSpecs() []tableSpec {
	return []tableSpec{
		{
			Name:     "commodities",
			Key:      "id",
			Required: []string{"id", "name", "class", "base_price", "cycle_period_days", "cycle_amplitude", "seasonal_amplitude", "weight_per_unit"},
			Ranges: map[string][2]float64{
				"base_price":         {0.0001, 1e9},
				"cycle_period_days":  {1, 100000},
				"cycle_amplitude":    {0, 1},
				"seasonal_amplitude": {0, 1},
				"weight_per_unit":    {0.0001, 1e6},
				"min_price":          {0, 1e9},
				"max_price":          {0, 1e9},
			},
		},
		{
			Name:     "ports",
			Key:      "id",
			Required: []string{"id", "name", "class", "region", "x", "y", "lat", "lon", "base_tax_rate"},
			References: map[string]string{
				"specialty_commodity": "commodities",
			},
			Ranges: map[string][2]float64{
				"lat":                 {-90, 90},
				"lon":                 {-180, 180},
				"base_tax_rate":       {0, 1},
				"specialty_discount":  {0, 1},
				"price_memory_factor": {0, 1},
			},
		},
		{
			Name:     "port_prices",
			Required: []string{"port_id", "commodity_id", "factor", "supply", "demand"},
			References: map[string]string{
				"port_id":      "ports",
				"commodity_id": "commodities",
			},
			Ranges: map[string][2]float64{
				"factor": {0.01, 100},
				"supply": {0, 1e9},
				"demand": {0, 1e9},
			},
		},
		{
			Name:     "ships",
			Key:      "id",
			Required: []string{"id", "name", "class", "hull_points", "max_sailors", "cargo_capacity", "base_speed"},
			Ranges: map[string][2]float64{
				"hull_points":              {1, 1e6},
				"max_sailors":              {1, 10000},
				"cargo_capacity":           {1, 1e7},
				"base_speed":               {0.01, 1000},
				"repair_cost_per_hp":       {0, 1e6},
				"food_per_sailor_per_day":  {0, 100},
				"water_per_sailor_per_day": {0, 100},
			},
		},
		{
			Name:     "characters",
			Key:      "id",
			Required: []string{"id", "name", "class", "salary_per_day", "loyalty_base"},
			Ranges: map[string][2]float64{
				"adventure":      {0, 100},
				"trade":          {0, 100},
				"naval":          {0, 100},
				"loyalty_base":   {0, 100},
				"salary_per_day": {0, 1e6},
			},
		},
		{
			Name:     "sea_zones",
			Key:      "id",
			Required: []string{"id", "name", "center_x", "center_y", "radius"},
			Ranges: map[string][2]float64{
				"radius":             {1, 1e6},
				"wind_base_speed":    {0, 1000},
				"current_base_speed": {0, 1000},
			},
		},
		{
			Name:     "weather",
			Key:      "id",
			Required: []string{"id", "name", "kind", "weight", "wind_speed_min", "wind_speed_max", "speed_penalty"},
			Ranges: map[string][2]float64{
				"weight":            {0, 1e6},
				"wind_speed_min":    {0, 1000},
				"wind_speed_max":    {0, 1000},
				"speed_penalty":     {0, 1},
				"damage_per_day":    {0, 1e6},
				"visibility_factor": {0, 10},
			},
		},
		{
			Name:     "events",
			Key:      "id",
			Required: []string{"id", "name", "category", "weight"},
			Ranges: map[string][2]float64{
				"weight": {0, 1e6},
			},
		},
		{
			Name:     "rumors",
			Key:      "id",
			Required: []string{"id", "text", "category", "weight"},
			References: map[string]string{
				"target_port": "ports",
			},
			Ranges: map[string][2]float64{
				"weight": {0, 1e6},
			},
		},
		{
			Name:     "titles",
			Key:      "id",
			Required: []string{"id", "name", "axis", "threshold"},
			Ranges: map[string][2]float64{
				"threshold": {0, 100},
			},
		},
		{
			Name:     "ai_profiles",
			Key:      "id",
			Required: []string{"id", "name", "fleet_count", "risk_aversion", "cargo_aggressiveness"},
			References: map[string]string{
				"home_port": "ports",
			},
			Ranges: map[string][2]float64{
				"fleet_count":          {0, 10000},
				"risk_aversion":        {0, 10},
				"cargo_aggressiveness": {0, 1},
				"max_voyage_days":      {1, 100000},
				"storm_avoidance":      {0, 1},
				"trade_skill":          {0, 1},
			},
		},
	}
}

// readTable reads a CSV table into a slice of maps.
func readTable(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cr := csv.NewReader(f)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	header := records[0]
	out := make([]map[string]string, 0, len(records)-1)
	for _, rec := range records[1:] {
		row := make(map[string]string, len(header))
		for i, col := range header {
			if i < len(rec) {
				row[strings.TrimSpace(col)] = strings.TrimSpace(rec[i])
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// checkTable applies the validation rules of a table.
func checkTable(spec tableSpec, rows []map[string]string, report *Report) {
	// --- the mandatory columns must be present ---
	if len(rows) > 0 {
		for _, col := range spec.Required {
			if _, ok := rows[0][col]; !ok {
				report.Problems = append(report.Problems, Problem{
					Table:  spec.Name,
					Kind:   "missing",
					Detail: "the mandatory column \"" + col + "\" is absent",
				})
			}
		}
	}

	// --- the primary key must be unique ---
	if spec.Key != "" {
		seen := make(map[string]int, len(rows))
		for i, row := range rows {
			key := row[spec.Key]
			if key == "" {
				report.Problems = append(report.Problems, Problem{
					Table:  spec.Name,
					Kind:   "missing",
					Detail: "the primary key \"" + spec.Key + "\" is empty",
					Row:    i + 2,
				})
				continue
			}
			if prev, ok := seen[key]; ok {
				report.Problems = append(report.Problems, Problem{
					Table:  spec.Name,
					Kind:   "duplicate",
					Detail: fmt.Sprintf("the key %q is already used by the row %d", key, prev),
					Row:    i + 2,
				})
				continue
			}
			seen[key] = i + 2
		}
	}

	// --- the numeric ranges ---
	for i, row := range rows {
		for col, rng := range spec.Ranges {
			raw, ok := row[col]
			if !ok || raw == "" {
				continue
			}
			v, err := parseFloat(raw)
			if err != nil {
				report.Problems = append(report.Problems, Problem{
					Table:  spec.Name,
					Kind:   "format",
					Detail: fmt.Sprintf("the column %q holds %q, which is not a number", col, raw),
					Row:    i + 2,
				})
				continue
			}
			if v < rng[0] || v > rng[1] {
				report.Problems = append(report.Problems, Problem{
					Table:  spec.Name,
					Kind:   "range",
					Detail: fmt.Sprintf("the column %q holds %g, which is outside of [%g, %g]", col, v, rng[0], rng[1]),
					Row:    i + 2,
				})
			}
		}
	}
}

// CheckReferences verifies the foreign keys of a table against the
// referenced tables.
//
// It is called after every table has been read, so that the referenced
// tables are complete.
func CheckReferences(dir string, report *Report) {
	// Load the primary keys of every table.
	keys := make(map[string]map[string]bool)
	for _, spec := range tableSpecs() {
		rows, err := readTable(filepath.Join(dir, spec.Name+".csv"))
		if err != nil {
			continue
		}
		set := make(map[string]bool, len(rows))
		for _, row := range rows {
			if key := row[spec.Key]; key != "" {
				set[key] = true
			}
		}
		keys[spec.Name] = set
	}

	for _, spec := range tableSpecs() {
		if len(spec.References) == 0 {
			continue
		}
		rows, err := readTable(filepath.Join(dir, spec.Name+".csv"))
		if err != nil {
			continue
		}
		for i, row := range rows {
			for col, target := range spec.References {
				value := row[col]
				if value == "" {
					continue
				}
				set, ok := keys[target]
				if !ok {
					continue
				}
				if !set[value] {
					report.Problems = append(report.Problems, Problem{
						Table:  spec.Name,
						Kind:   "reference",
						Detail: fmt.Sprintf("the column %q holds %q, which does not exist in the table %q", col, value, target),
						Row:    i + 2,
					})
				}
			}
		}
	}
}
