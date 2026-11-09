package importer

import (
	"fmt"
	"strings"
)

// This file implements the menu tree layout.
//
// The original game stores its menus as a list of entries; each entry
// holds a two byte header followed by the labels of the items, separated
// by NUL bytes:
//
//	byte 0    the width of the item column in pixels
//	byte 1    the number of items of the entry
//	byte 2..  the labels, NUL separated
//
// The entries themselves are addressed through an offset table, exactly
// like the message tables.

// ConvertMenuTree converts a table of menus.
//
// The result holds one row per item, with the index of the menu and the
// index of the item inside that menu.
func ConvertMenuTree(r *Reader, spec *Spec, table TableSpec, log Logger) (*Table, error) {
	out := &Table{Name: table.Name, Description: table.Description}
	out.Columns = []string{"id", "menu", "item", "column_width", "label"}

	offsetSize := table.OffsetSize
	if offsetSize != 2 && offsetSize != 4 {
		offsetSize = 2
	}
	bigEndian := table.OffsetBigEndian

	// --- read the offsets ---
	const maxEntries = 1 << 16
	offsets := make([]int64, 0, 256)
	buf := make([]byte, offsetSize)
	var prev int64 = -1
	for i := 0; i < maxEntries; i++ {
		if err := r.ReadAt(buf, int64(i)*int64(offsetSize)); err != nil {
			break
		}
		var off int64
		if bigEndian {
			if offsetSize == 2 {
				off = int64(bigEndianU16(buf))
			} else {
				off = int64(bigEndianU32(buf))
			}
		} else {
			if offsetSize == 2 {
				off = int64(littleEndianU16(buf))
			} else {
				off = int64(littleEndianU32(buf))
			}
		}
		if off <= prev || off >= r.Size() {
			break
		}
		offsets = append(offsets, off)
		prev = off
	}
	if len(offsets) == 0 {
		return nil, fmt.Errorf("table %s: no offset could be read", table.Name)
	}

	// --- read the menus ---
	index := 0
	for menu := range offsets {
		start := offsets[menu]
		end := r.Size()
		if menu+1 < len(offsets) {
			end = offsets[menu+1]
		}
		length := end - start
		if length < 2 {
			continue
		}
		raw := make([]byte, length)
		if err := r.ReadAt(raw, start); err != nil {
			out.Problems = append(out.Problems, fmt.Sprintf(
				"table %s: read the menu %d: %v", table.Name, menu, err))
			continue
		}
		columnWidth := int(raw[0])
		itemCount := int(raw[1])
		labels := strings.Split(string(raw[2:]), "\x00")
		// The empty labels are the padding of the last item.
		cleaned := make([]string, 0, len(labels))
		for _, label := range labels {
			if label == "" {
				continue
			}
			cleaned = append(cleaned, label)
		}
		for i, label := range cleaned {
			cells := []string{
				fmt.Sprintf("%d", index),
				fmt.Sprintf("%d", menu),
				fmt.Sprintf("%d", i),
				fmt.Sprintf("%d", columnWidth),
				decodeText([]byte(label), table),
			}
			out.Rows = append(out.Rows, Row{Cells: cells})
			log.Record(table.Name, index, cells)
			index++
		}
		// The count is only used as a consistency check.
		if itemCount != len(cleaned) && itemCount != 0 {
			out.Problems = append(out.Problems, fmt.Sprintf(
				"table %s: the menu %d declares %d items but holds %d",
				table.Name, menu, itemCount, len(cleaned)))
		}
	}
	return out, nil
}

// bigEndianU16 reads a big-endian 16 bit value.
func bigEndianU16(b []byte) uint16 {
	return uint16(b[0])<<8 | uint16(b[1])
}

// bigEndianU32 reads a big-endian 32 bit value.
func bigEndianU32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// littleEndianU16 reads a little-endian 16 bit value.
func littleEndianU16(b []byte) uint16 {
	return uint16(b[1])<<8 | uint16(b[0])
}

// littleEndianU32 reads a little-endian 32 bit value.
func littleEndianU32(b []byte) uint32 {
	return uint32(b[3])<<24 | uint32(b[2])<<16 | uint32(b[1])<<8 | uint32(b[0])
}
