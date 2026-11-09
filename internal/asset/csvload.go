// Package asset loads the read-only static configuration layer.
//
// The loader reads CSV (and JSON) table files from assets/config, turns them
// into the strongly typed structures declared in internal/config and exposes
// them through a single, immutable *config.Registry.
//
// # Contract
//
//   - The loader runs exactly once per process (see internal/asset/registry.go
//     for the sync.Once singleton).
//   - Every table is indexed by its primary key, so runtime lookups are O(1).
//   - Numbers that do not parse are hard errors: silently defaulting numeric
//     data is the fastest way to corrupt a balance model.
package asset

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// fieldInfo caches the reflection metadata of one struct field.
type fieldInfo struct {
	// index is the index of the field inside the struct.
	index int
	// name is the CSV column name.
	name string
	// optional marks columns that may be missing or empty.
	optional bool
	// multi marks slice fields, whose cell is split on the separator.
	multi bool
	// kind is the reflect.Kind of the field.
	kind reflect.Kind
}

// structFields returns the reflection metadata of all exported fields of T
// that carry a "csv" tag, in declaration order.
//
// The "csv" tag format is: `csv:"column_name,opt"`. The "opt" flag makes the
// column optional: a missing column or an empty cell yields the zero value
// instead of an error.
func structFields(t reflect.Type) []fieldInfo {
	out := make([]fieldInfo, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, ok := f.Tag.Lookup("csv")
		if !ok {
			continue
		}
		parts := strings.Split(tag, ",")
		name := strings.TrimSpace(parts[0])
		if name == "" || name == "-" {
			continue
		}
		fi := fieldInfo{index: i, name: name, kind: f.Type.Kind()}
		for _, opt := range parts[1:] {
			switch strings.TrimSpace(opt) {
			case "opt":
				fi.optional = true
			case "multi":
				fi.multi = true
			}
		}
		out = append(out, fi)
	}
	return out
}

// LoadCSVTable reads a CSV file and decodes it into a slice of T.
//
// The first row must contain the column headers. Rows whose "id" column is
// empty are skipped (they act as separators in hand-edited tables).
func LoadCSVTable[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	return DecodeCSVTable[T](f, path)
}

// DecodeCSVTable decodes CSV data into a slice of T. The path argument is
// only used to build readable error messages.
func DecodeCSVTable[T any](r io.Reader, path string) ([]T, error) {
	var zero T
	elemType := reflect.TypeOf(zero)
	if elemType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s: target type %s is not a struct", path, elemType)
	}
	fields := structFields(elemType)

	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // allow ragged rows; validated per field below
	cr.TrimLeadingSpace = true
	cr.LazyQuotes = false

	header, err := cr.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("%s: empty table", path)
		}
		return nil, fmt.Errorf("%s: read header: %w", path, err)
	}

	// Map column name -> field index. Later duplicates win, which mirrors
	// the behaviour of most spreadsheet tools and keeps the loader forgiving.
	colToField := make(map[string]fieldInfo, len(header))
	for i, h := range header {
		name := strings.TrimSpace(h)
		if name == "" {
			continue
		}
		for _, fi := range fields {
			if fi.name == name {
				colToField[name] = fi
				_ = i
				break
			}
		}
	}
	// Detect required columns that are entirely absent.
	for _, fi := range fields {
		if fi.optional {
			continue
		}
		if _, ok := colToField[fi.name]; !ok {
			return nil, fmt.Errorf("%s: required column %q is missing", path, fi.name)
		}
	}

	out := make([]T, 0, 64)
	rowNum := 1
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			return nil, fmt.Errorf("%s: line %d: %w", path, rowNum, err)
		}
		// Skip fully blank lines.
		blank := true
		for _, c := range record {
			if strings.TrimSpace(c) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}

		v := reflect.New(elemType).Elem()
		for col, cell := range record {
			if col >= len(header) {
				break
			}
			name := strings.TrimSpace(header[col])
			fi, ok := colToField[name]
			if !ok {
				continue // unknown column: ignore, keeps tables forward compatible
			}
			cell = strings.TrimSpace(cell)
			if cell == "" {
				if fi.optional {
					continue
				}
				return nil, fmt.Errorf("%s: line %d: column %q must not be empty", path, rowNum, name)
			}
			if err := setField(v.Field(fi.index), fi, cell); err != nil {
				return nil, fmt.Errorf("%s: line %d: column %q: %w", path, rowNum, name, err)
			}
		}
		out = append(out, v.Interface().(T))
	}
	return out, nil
}

// setField converts a raw CSV cell into the destination field.
func setField(dst reflect.Value, fi fieldInfo, cell string) error {
	if fi.multi {
		if dst.Kind() != reflect.Slice {
			return fmt.Errorf("multi column requires a slice destination")
		}
		parts := strings.Split(cell, ";")
		items := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				items = append(items, p)
			}
		}
		dst.Set(reflect.ValueOf(items))
		return nil
	}

	switch fi.kind {
	case reflect.String:
		dst.SetString(cell)
	case reflect.Bool:
		b, err := strconv.ParseBool(cell)
		if err != nil {
			// Also accept the 0/1 convention used by some exporters.
			switch strings.ToLower(cell) {
			case "0", "n", "no":
				b = false
			case "1", "y", "yes":
				b = true
			default:
				return fmt.Errorf("invalid boolean %q", cell)
			}
		}
		dst.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(cell, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer %q", cell)
		}
		dst.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(cell, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer %q", cell)
		}
		dst.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(cell, 64)
		if err != nil {
			return fmt.Errorf("invalid number %q", cell)
		}
		dst.SetFloat(f)
	default:
		return fmt.Errorf("unsupported column type %s", fi.kind)
	}
	return nil
}
