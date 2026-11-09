package importer

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// This file implements the text oriented table layouts.
//
// The original game stores its text in three different shapes:
//
//   - an offset table followed by NUL terminated strings
//     (MESSAGE.DAT, MESSAGE2.DAT, MENU.DAT, SNR*.MES),
//   - a list of NUL separated strings (NAME.TBL),
//   - fixed size records of NUL padded text (ITEM.MES, COLONY.DAT).
//
// All three are converted into the same kind of CSV table, so that the
// runtime can load them with the same loader.

// ConvertOffsetTable converts a table of offsets followed by strings.
//
// The offsets are read at the beginning of the data section; the number
// of entries is derived from the size of the table, which ends where the
// first string begins.
//
// The strings are NUL terminated; the end of a string is the beginning of
// the next one, and the last string ends at the end of the data.
func ConvertOffsetTable(r *Reader, spec *Spec, table TableSpec, log Logger) (*Table, error) {
	out := &Table{Name: table.Name, Description: table.Description}

	offsetSize := table.OffsetSize
	if offsetSize != 2 && offsetSize != 4 {
		offsetSize = 2
	}
	bigEndian := table.OffsetBigEndian

	// --- read the offsets ---
	//
	// The table is scanned until the offsets stop being monotonic, which
	// is where the string data begins.
	const maxEntries = 1 << 20
	offsets := make([]int64, 0, 1024)
	buf := make([]byte, offsetSize)
	var prev int64 = -1
	for i := 0; i < maxEntries; i++ {
		if err := r.ReadAt(buf, int64(i)*int64(offsetSize)); err != nil {
			break
		}
		var off int64
		if bigEndian {
			if offsetSize == 2 {
				off = int64(binary.BigEndian.Uint16(buf))
			} else {
				off = int64(binary.BigEndian.Uint32(buf))
			}
		} else {
			if offsetSize == 2 {
				off = int64(binary.LittleEndian.Uint16(buf))
			} else {
				off = int64(binary.LittleEndian.Uint32(buf))
			}
		}
		// The offsets must be strictly increasing and inside the data.
		if off <= prev || off >= r.Size() {
			break
		}
		offsets = append(offsets, off)
		prev = off
	}
	if len(offsets) == 0 {
		return nil, fmt.Errorf("table %s: no offset could be read", table.Name)
	}
	log.Info("table %s: %d offsets, the string data starts at %d", table.Name, len(offsets), offsets[0])

	// --- read the strings ---
	//
	// The strings are read one by one; the length of a string is the
	// distance to the next offset.
	columns := []string{"id", "text"}
	out.Columns = columns

	for i, start := range offsets {
		end := r.Size()
		if i+1 < len(offsets) {
			end = offsets[i+1]
		}
		length := end - start
		if length <= 0 {
			continue
		}
		raw := make([]byte, length)
		if err := r.ReadAt(raw, start); err != nil {
			out.Problems = append(out.Problems, fmt.Sprintf(
				"table %s: read the string %d: %v", table.Name, i, err))
			continue
		}
		text := decodeText(raw, table)
		out.Rows = append(out.Rows, Row{Cells: []string{fmt.Sprintf("%d", i), text}})
		log.Record(table.Name, i, []string{fmt.Sprintf("%d", i), text})
	}
	return out, nil
}

// ConvertNulList converts a list of NUL separated strings.
//
// The format is used by the name tables: the strings are simply
// concatenated and separated by a NUL byte.
func ConvertNulList(r *Reader, spec *Spec, table TableSpec, log Logger) (*Table, error) {
	out := &Table{Name: table.Name, Description: table.Description}
	out.Columns = []string{"id", "text"}

	// The file is read in one go: the name tables are small.
	data := make([]byte, r.Size())
	if err := r.ReadAt(data, 0); err != nil {
		return nil, fmt.Errorf("table %s: %w", table.Name, err)
	}
	index := 0
	for _, part := range strings.Split(string(data), "\x00") {
		if part == "" {
			continue
		}
		text := decodeText([]byte(part), table)
		out.Rows = append(out.Rows, Row{Cells: []string{fmt.Sprintf("%d", index), text}})
		log.Record(table.Name, index, []string{fmt.Sprintf("%d", index), text})
		index++
	}
	return out, nil
}

// ConvertFixedText converts a table of fixed size text records.
//
// Each record holds one or more NUL padded text fields; the field
// boundaries are given by the field specifications.
func ConvertFixedText(r *Reader, spec *Spec, table TableSpec, log Logger) (*Table, error) {
	out := &Table{Name: table.Name, Description: table.Description}
	if table.RecordSize <= 0 {
		return nil, fmt.Errorf("table %s: a fixed text table needs a record size", table.Name)
	}

	// The columns are the field names, plus the identifier.
	columns := make([]string, 0, len(table.Fields)+1)
	columns = append(columns, "id")
	for _, f := range table.Fields {
		columns = append(columns, f.Name)
	}
	out.Columns = columns

	count := int(r.Size() / int64(table.RecordSize))
	buf := make([]byte, table.RecordSize)
	for i := 0; i < count; i++ {
		if err := r.ReadAt(buf, int64(i)*int64(table.RecordSize)); err != nil {
			return nil, fmt.Errorf("table %s: read the record %d: %w", table.Name, i, err)
		}
		cells := []string{fmt.Sprintf("%d", i)}
		for _, f := range table.Fields {
			length := f.Length
			if length <= 0 {
				length = table.RecordSize - f.Offset
			}
			if f.Offset < 0 || f.Offset+length > len(buf) {
				cells = append(cells, "")
				continue
			}
			text := decodeText(buf[f.Offset:f.Offset+length], table)
			cells = append(cells, text)
		}
		out.Rows = append(out.Rows, Row{Cells: cells})
		log.Record(table.Name, i, cells)
	}
	return out, nil
}

// decodeText decodes a raw text field and trims it.
//
// The trimming removes the trailing NUL bytes and the spaces, which the
// original data uses as the padding of the fixed size fields.
func decodeText(raw []byte, table TableSpec) string {
	encoding := table.StringDecode
	if encoding == "" {
		encoding = "ascii"
	}
	text := decodeBytes(raw, encoding)
	if table.StringTrim {
		text = trimString(text)
	}
	// The line breaks of the dialogues are kept, but the trailing
	// whitespace is removed.
	text = strings.TrimRight(text, " \t\r\n")
	return text
}
