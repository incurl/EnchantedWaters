package importer

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

// Reader reads records out of a binary data section.
//
// The reader is a streaming reader: it never loads the whole file into
// memory, it reads exactly the bytes it needs. This matters for the
// large graphics files, which can be tens of megabytes.
type Reader struct {
	// r is the underlying stream.
	r io.ReadSeeker
	// base is the offset of the data section inside the stream.
	base int64
	// size is the length of the data section.
	size int64
}

// NewReader creates a reader over a data section of a file.
func NewReader(f *os.File, base, size int64) *Reader {
	return &Reader{r: f, base: base, size: size}
}

// NewSectionReader creates a reader over a data section of any stream.
func NewSectionReader(r io.ReadSeeker, base, size int64) *Reader {
	return &Reader{r: r, base: base, size: size}
}

// Size returns the length of the data section.
func (r *Reader) Size() int64 { return r.size }

// SeekTo moves the read cursor inside the data section.
//
// The offset is relative to the beginning of the section. The name avoids
// the clash with the Seek method of io.Seeker, whose signature is
// different.
func (r *Reader) SeekTo(offset int64) error {
	if offset < 0 || offset > r.size {
		return fmt.Errorf("seek to %d outside of the section (size %d)", offset, r.size)
	}
	_, err := r.r.Seek(r.base+offset, io.SeekStart)
	return err
}

// ReadAt reads len(buf) bytes at a relative offset of the section.
//
// The read is a single sequential read at the current position, which
// keeps the number of system calls proportional to the number of
// records.
func (r *Reader) ReadAt(buf []byte, offset int64) error {
	if offset < 0 || offset+int64(len(buf)) > r.size {
		return fmt.Errorf("read of %d bytes at %d outside of the section (size %d)", len(buf), offset, r.size)
	}
	if _, err := r.r.Seek(r.base+offset, io.SeekStart); err != nil {
		return err
	}
	_, err := io.ReadFull(r.r, buf)
	return err
}

// readValue reads a single numeric value of the given type at a relative
// offset.
//
// The value is converted with the little-endian byte order, which is the
// convention of the DOS era formats the importer targets.
func readValue(data []byte, f FieldSpec) (float64, error) {
	switch f.Type {
	case "u8":
		if len(data) < f.Offset+1 {
			return 0, fmt.Errorf("field %q: truncated", f.Name)
		}
		return float64(data[f.Offset]), nil
	case "i8":
		if len(data) < f.Offset+1 {
			return 0, fmt.Errorf("field %q: truncated", f.Name)
		}
		return float64(int8(data[f.Offset])), nil
	case "u16":
		if len(data) < f.Offset+2 {
			return 0, fmt.Errorf("field %q: truncated", f.Name)
		}
		return float64(binary.LittleEndian.Uint16(data[f.Offset:])), nil
	case "i16":
		if len(data) < f.Offset+2 {
			return 0, fmt.Errorf("field %q: truncated", f.Name)
		}
		return float64(int16(binary.LittleEndian.Uint16(data[f.Offset:]))), nil
	case "u32":
		if len(data) < f.Offset+4 {
			return 0, fmt.Errorf("field %q: truncated", f.Name)
		}
		return float64(binary.LittleEndian.Uint32(data[f.Offset:])), nil
	case "i32":
		if len(data) < f.Offset+4 {
			return 0, fmt.Errorf("field %q: truncated", f.Name)
		}
		return float64(int32(binary.LittleEndian.Uint32(data[f.Offset:]))), nil
	case "f32":
		if len(data) < f.Offset+4 {
			return 0, fmt.Errorf("field %q: truncated", f.Name)
		}
		return float64(math32frombits(binary.LittleEndian.Uint32(data[f.Offset:]))), nil
	case "str", "bytes":
		return 0, nil
	default:
		return 0, fmt.Errorf("field %q: unknown type %q", f.Name, f.Type)
	}
}

// math32frombits converts a float32 bit pattern into a float64.
//
// It is a small helper so that the reader keeps its imports minimal.
func math32frombits(b uint32) float64 {
	return float64(math.Float32frombits(b))
}

// trimString removes the trailing NUL bytes and the spaces of a string.
func trimString(s string) string {
	return strings.TrimRight(s, "\x00 ")
}
