package importer

import (
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Row is one record of an imported table.
type Row struct {
	// Cells holds the value of every column, in the column order.
	Cells []string
}

// Table is the result of the conversion of one binary table.
type Table struct {
	// Name is the name of the table.
	Name string
	// Description explains the content of the table.
	Description string
	// Columns holds the column names.
	Columns []string
	// Rows holds the converted records.
	Rows []Row
	// Problems holds the human readable notes about the conversion.
	Problems []string
}

// ConvertTable reads the records of a binary table and converts them.
//
// The layout of the table is selected by its Kind field; the default is
// the fixed size record layout.
//
// The reader is used in a streaming fashion: only one record is held in
// memory at a time.
func ConvertTable(r *Reader, spec *Spec, table TableSpec, log Logger) (*Table, error) {
	// --- the text oriented layouts ---
	switch table.Kind {
	case "offset_table":
		return ConvertOffsetTable(r, spec, table, log)
	case "nul_list":
		return ConvertNulList(r, spec, table, log)
	case "fixed_text":
		return ConvertFixedText(r, spec, table, log)
	case "menu_tree":
		return ConvertMenuTree(r, spec, table, log)
	}

	out := &Table{Name: table.Name, Description: table.Description}

	// --- build the column list ---
	columns := make([]string, 0, len(table.Fields)+len(table.Constants))
	for _, f := range table.Fields {
		columns = append(columns, f.Name)
	}
	// The constant columns come first, so that they read naturally.
	constNames := make([]string, 0, len(table.Constants))
	for name := range table.Constants {
		constNames = append(constNames, name)
	}
	sortStrings(constNames)
	for _, name := range constNames {
		columns = append(columns, name)
	}
	out.Columns = columns

	// --- determine the record count ---
	count := table.Count
	if table.CountOffset != nil {
		var buf [4]byte
		if err := r.ReadAt(buf[:], *table.CountOffset); err != nil {
			return nil, fmt.Errorf("table %s: read record count: %w", table.Name, err)
		}
		count = int(binary.LittleEndian.Uint32(buf[:]))
	}
	if count == 0 {
		// The table extends until the end of the data section.
		remaining := r.Size() - table.Offset
		if remaining > 0 && table.RecordSize > 0 {
			count = int(remaining / int64(table.RecordSize))
		}
	}
	if table.RecordSize <= 0 {
		return nil, fmt.Errorf("table %s: invalid record size %d", table.Name, table.RecordSize)
	}
	// The count is clamped to the data that is actually available.
	available := (r.Size() - table.Offset) / int64(table.RecordSize)
	if int64(count) > available {
		out.Problems = append(out.Problems, fmt.Sprintf(
			"table %s: %d records declared but only %d fit in the data", table.Name, count, available))
		count = int(available)
	}

	// --- convert the records ---
	buf := make([]byte, table.RecordSize)
	for i := 0; i < count; i++ {
		offset := table.Offset + int64(i)*int64(table.RecordSize)
		if err := r.ReadAt(buf, offset); err != nil {
			return nil, fmt.Errorf("table %s: read record %d: %w", table.Name, i, err)
		}
		cells := make([]string, 0, len(columns))
		for _, f := range table.Fields {
			value, err := convertField(buf, f, table)
			if err != nil {
				out.Problems = append(out.Problems, fmt.Sprintf(
					"table %s: record %d field %q: %v", table.Name, i, f.Name, err))
				value = ""
			}
			cells = append(cells, value)
		}
		for _, name := range constNames {
			cells = append(cells, table.Constants[name])
		}
		out.Rows = append(out.Rows, Row{Cells: cells})
		// Every record is logged, so that a human can audit the
		// conversion before committing the result.
		log.Record(table.Name, i, cells)
	}
	return out, nil
}

// convertField converts one field of a record into its textual form.
//
// The table is passed so that the field inherits its trimming and decoding
// settings: a specification then describes the code page and the padding
// once, instead of repeating them on every field.
func convertField(data []byte, f FieldSpec, table TableSpec) (string, error) {
	switch f.Type {
	case "u8", "i8", "u16", "i16", "u32", "i32", "f32":
		v, err := readValue(data, f)
		if err != nil {
			return "", err
		}
		// The scale and the offset convert the raw integer of the
		// original game into the real value. A scale of zero means "no
		// scaling", so that a specification can omit the field.
		if f.Scale != 0 {
			v = v * f.Scale
		}
		v += f.OffsetValue
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case "str", "bytes":
		length := f.Length
		if length <= 0 {
			length = len(data) - f.Offset
		}
		if f.Offset < 0 || f.Offset+length > len(data) {
			return "", fmt.Errorf("string of %d bytes at %d is outside of the record", length, f.Offset)
		}
		raw := data[f.Offset : f.Offset+length]
		encoding := f.Decode
		if encoding == "" {
			encoding = table.StringDecode
		}
		s := decodeBytes(raw, encoding)
		if f.Trim || table.StringTrim {
			s = trimString(s)
		}
		return s, nil
	default:
		return "", fmt.Errorf("unknown type %q", f.Type)
	}
}

// decodeBytes decodes a byte array into a string.
//
// The original game used the DOS era Japanese code pages; the decoder
// falls back to a lossy conversion when the bytes are not valid in the
// requested code page, so that the conversion never fails on a single
// bad byte.
func decodeBytes(raw []byte, encoding string) string {
	switch strings.ToLower(encoding) {
	case "cp932", "shift_jis", "sjis":
		return decodeCP932(raw)
	case "utf8", "utf-8":
		return string(raw)
	case "none", "":
		return string(raw)
	default:
		return string(raw)
	}
}

// WriteCSV writes a table into a CSV file.
//
// The column order is the order of the Columns slice, which matches the
// order of the cells of every row.
func WriteCSV(w io.Writer, t *Table) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(t.Columns); err != nil {
		return err
	}
	for _, row := range t.Rows {
		if err := cw.Write(row.Cells); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
