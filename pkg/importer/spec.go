// Package importer converts original game asset files into the project's
// standard data format.
//
// # Legal and ethical scope
//
// The importer is a *development-only* tool. It reads the asset files that
// the user supplies from their own disk and writes the converted tables
// into the assets directory of this repository. No original asset is
// bundled with the source code, with the release binary, or with the
// runtime of the game.
//
// # Design
//
// The importer is driven by *format specifications*: small JSON files that
// describe where the data lives inside a binary file. This keeps the
// parser generic and free of any hard-coded knowledge about the layout of
// a particular game build: adding support for another build means adding
// a specification file, not changing the Go code.
//
// A specification describes:
//
//   - how the file is opened (a file, or a file inside an archive),
//   - where the tables live (an offset, a length, a count),
//   - how each record is laid out (the fields, their types and their
//     offsets),
//   - how the records are turned into the rows of a CSV table.
//
// The specifications live in assets/specs and are part of the repository,
// so the conversion is fully reproducible and auditable.
package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Spec is the description of how to parse one asset file.
//
// The zero value is not usable; use LoadSpec.
type Spec struct {
	// Name identifies the specification.
	Name string `json:"name"`
	// Description explains what the asset contains.
	Description string `json:"description"`
	// Source describes where the raw data is read from.
	Source SourceSpec `json:"source"`
	// Tables lists the tables that are extracted from the file.
	Tables []TableSpec `json:"tables"`
	// Graphics lists the sprite sheets that are extracted from the file.
	Graphics []GraphicSpec `json:"graphics,omitempty"`
	// Towns marks a file whose pictures of the towns are exported.
	//
	// The tiles of the ports and the maps of the towns do not go through
	// the general reader: the tiles are stored in segments and the maps
	// are grids of the indices of those tiles. A specification that sets
	// this flag hands the file to the exporter of the town data instead.
	Towns bool `json:"towns,omitempty"`
}

// SourceSpec describes the raw data source.
type SourceSpec struct {
	// Path is the path of the file, relative to the raw directory.
	Path string `json:"path"`
	// Archive is the optional archive that contains the file. When it is
	// empty, the file is read directly from the disk.
	Archive string `json:"archive,omitempty"`
	// Offset is the offset of the data inside the file.
	Offset int64 `json:"offset"`
	// Length is the length of the data; 0 means "until the end".
	Length int64 `json:"length"`
}

// TableSpec describes one extracted table.
type TableSpec struct {
	// Name is the base name of the exported files: "<name>.csv" and
	// "<name>.json".
	Name string `json:"name"`
	// Description explains the content of the table.
	Description string `json:"description"`
	// Offset is the offset of the table inside the data.
	Offset int64 `json:"offset"`
	// RecordSize is the size of a single record in bytes.
	RecordSize int `json:"record_size"`
	// Count is the number of records; 0 means "until the end of the
	// data".
	Count int `json:"count"`
	// CountOffset is the offset of the record count inside the data. It
	// overrides Count when it is not nil.
	CountOffset *int64 `json:"count_offset,omitempty"`
	// Fields describes the fields of a record.
	Fields []FieldSpec `json:"fields"`
	// Constants are the values that are written in every row.
	Constants map[string]string `json:"constants,omitempty"`
	// IDField is the name of the field that holds the primary key.
	IDField string `json:"id_field,omitempty"`
	// Kind selects the layout of the table.
	//
	// The empty value means "fixed size records" (the default). The
	// other kinds are:
	//
	//	"offset_table"  a table of offsets, followed by the strings
	//	"nul_list"     a list of NUL separated strings
	//	"fixed_text"   fixed size records of text
	//
	// The offset tables come in two flavours, which are selected by
	// OffsetSize (2 or 4 bytes) and by the byte order of the offsets
	// (OffsetBigEndian).
	Kind string `json:"kind,omitempty"`
	// OffsetSize is the size of an offset in bytes (2 or 4).
	OffsetSize int `json:"offset_size,omitempty"`
	// OffsetBigEndian selects the byte order of the offsets.
	OffsetBigEndian bool `json:"offset_big_endian,omitempty"`
	// StringTrim removes the trailing NUL bytes of the strings.
	StringTrim bool `json:"string_trim,omitempty"`
	// StringDecode is the code page of the strings ("ascii", "cp932"...).
	StringDecode string `json:"string_decode,omitempty"`
}

// FieldSpec describes one field of a record.
type FieldSpec struct {
	// Name is the name of the column in the exported table.
	Name string `json:"name"`
	// Type is the data type: "u8", "u16", "u32", "i8", "i16", "i32",
	// "f32", "str", "bytes".
	Type string `json:"type"`
	// Offset is the offset of the field inside the record.
	Offset int `json:"offset"`
	// Length is the length of a fixed size field (strings and byte
	// arrays). When it is zero, the field extends to the end of the
	// record.
	Length int `json:"length,omitempty"`
	// Scale multiplies a numeric value; it is used to convert the raw
	// integers of the original game into the real values.
	Scale float64 `json:"scale,omitempty"`
	// OffsetValue is added to a numeric value after the scaling.
	OffsetValue float64 `json:"offset_value,omitempty"`
	// Trim removes the trailing NUL bytes and spaces of a string.
	Trim bool `json:"trim,omitempty"`
	// Decode describes how a byte array is turned into text: "cp932",
	// "shift_jis", "utf8" or "none".
	Decode string `json:"decode,omitempty"`
}

// LoadSpec reads a specification file.
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", path, err)
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse spec %s: %w", path, err)
	}
	if spec.Name == "" {
		return nil, fmt.Errorf("spec %s has no name", path)
	}
	if len(spec.Tables) == 0 && len(spec.Graphics) == 0 && !spec.Towns {
		return nil, fmt.Errorf("spec %s has neither tables, graphics nor towns", path)
	}
	return &spec, nil
}

// LoadSpecs reads every specification of a directory.
//
// The specifications are returned sorted by name, so that the conversion
// is deterministic.
func LoadSpecs(dir string) ([]*Spec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read spec dir %s: %w", dir, err)
	}
	var out []*Spec
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		spec, err := LoadSpec(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		out = append(out, spec)
	}
	// The order of the directory listing is not guaranteed, so the specs
	// are sorted by name.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Name < out[j-1].Name; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}
