package importer

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// writeSyntheticFile builds a small binary file with a numeric table and
// verifies that the importer converts it correctly.
//
// The synthetic data stands in for an original asset file: the importer
// never has to know what the real layout is, because the layout comes
// from the specification.
func writeSyntheticFile(t *testing.T, path string) {
	t.Helper()
	const recordSize = 16
	const count = 4
	buf := make([]byte, recordSize*count)
	for i := 0; i < count; i++ {
		off := i * recordSize
		// The identifier.
		binary.LittleEndian.PutUint16(buf[off:], uint16(i+1))
		// The base value.
		binary.LittleEndian.PutUint16(buf[off+2:], uint16(10*(i+1)))
		// The signed value.
		binary.LittleEndian.PutUint16(buf[off+4:], uint16(int16(-5*(i+1))))
		// The scaled value: 250 * 0.01 = 2.5.
		binary.LittleEndian.PutUint16(buf[off+6:], 250)
		// The text.
		copy(buf[off+8:], []byte("Item"))
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write synthetic file: %v", err)
	}
}

// TestConvertTable verifies the conversion of a numeric table.
func TestConvertTable(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "DATA.BIN")
	writeSyntheticFile(t, dataPath)

	spec := &Spec{
		Name: "test",
		Source: SourceSpec{
			Path: "DATA.BIN",
		},
		Tables: []TableSpec{
			{
				Name:       "test_items",
				RecordSize: 16,
				Fields: []FieldSpec{
					{Name: "id", Type: "u16", Offset: 0},
					{Name: "value", Type: "u16", Offset: 2},
					{Name: "delta", Type: "i16", Offset: 4},
					{Name: "ratio", Type: "u16", Offset: 6, Scale: 0.01},
					{Name: "name", Type: "str", Offset: 8, Length: 8, Decode: "cp932", Trim: true},
				},
			},
		},
	}

	f, err := os.Open(dataPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	info, _ := f.Stat()
	r := NewReader(f, 0, info.Size())

	log := NewConsoleLogger(os.Stderr, false)
	table, err := ConvertTable(r, spec, spec.Tables[0], log)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(table.Rows) != 4 {
		t.Fatalf("converted %d rows, want 4", len(table.Rows))
	}
	if len(table.Columns) != 5 {
		t.Errorf("table has %d columns, want 5", len(table.Columns))
	}

	// The first row: id=1, value=10, delta=-5, ratio=2.5, name="Item".
	row := table.Rows[0]
	if row.Cells[0] != "1" {
		t.Errorf("id = %q, want \"1\"", row.Cells[0])
	}
	if row.Cells[1] != "10" {
		t.Errorf("value = %q, want \"10\"", row.Cells[1])
	}
	if row.Cells[2] != "-5" {
		t.Errorf("delta = %q, want \"-5\"", row.Cells[2])
	}
	if row.Cells[3] != "2.5" {
		t.Errorf("ratio = %q, want \"2.5\"", row.Cells[3])
	}
	if row.Cells[4] != "Item" {
		t.Errorf("name = %q, want \"Item\"", row.Cells[4])
	}

	// The fourth row has the negative delta -20.
	if table.Rows[3].Cells[2] != "-20" {
		t.Errorf("delta of the last row = %q, want \"-20\"", table.Rows[3].Cells[2])
	}
}

// TestRunEndToEnd runs the whole import pipeline and checks that the CSV
// and the JSON files are written.
func TestRunEndToEnd(t *testing.T) {
	rawDir := t.TempDir()
	specDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "config")

	writeSyntheticFile(t, filepath.Join(rawDir, "DATA.BIN"))

	specJSON := `{
  "name": "test.e2e",
  "description": "end to end test",
  "source": { "path": "DATA.BIN", "offset": 0, "length": 0 },
  "tables": [
    {
      "name": "e2e_items",
      "record_size": 16,
      "id_field": "id",
      "fields": [
        { "name": "id", "type": "u16", "offset": 0 },
        { "name": "value", "type": "u16", "offset": 2 },
        { "name": "name", "type": "str", "offset": 8, "length": 8, "decode": "cp932", "trim": true }
      ]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(specDir, "e2e.json"), []byte(specJSON), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	log := NewConsoleLogger(os.Stderr, false)
	result, err := Run(Options{
		RawDir:  rawDir,
		SpecDir: specDir,
		OutDir:  outDir,
		Verbose: false,
	}, log)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Tables["e2e_items"] != 4 {
		t.Errorf("converted %d rows, want 4", result.Tables["e2e_items"])
	}

	// --- the CSV file must exist and hold the header plus the rows ---
	csvPath := filepath.Join(outDir, "e2e_items.csv")
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(data) == 0 {
		t.Error("the CSV file is empty")
	}
	// --- the JSON file must exist ---
	jsonPath := filepath.Join(outDir, "e2e_items.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("the JSON file was not written: %v", err)
	}
}

// TestDecodeCP932 verifies the decoding of the DOS era code page.
func TestDecodeCP932(t *testing.T) {
	// 0x82 0xA0 is the CP932 encoding of the hiragana "a".
	got := decodeCP932([]byte{0x82, 0xA0})
	if got != "あ" {
		t.Errorf("decodeCP932 = %q, want %q", got, "あ")
	}
	// The half width katakana range.
	got = decodeCP932([]byte{0xB1})
	if got != "ｱ" {
		t.Errorf("decodeCP932 = %q, want %q", got, "ｱ")
	}
	// An invalid sequence must not panic.
	got = decodeCP932([]byte{0x82})
	if got == "" {
		t.Error("an invalid sequence produced an empty string")
	}
}
