package importer

import (
	"encoding/binary"
	"fmt"
)

// This file implements the LS10 and LS11 codec of the DOS releases of
// Uncharted Waters.
//
// The files are called ".LZW" and the community calls the codec "the KOEI
// LZW variant", but it is neither LZW nor a variant of it. There is no
// dictionary of phrases and no sliding window over the input. The payload
// of a block is a stream of variable length codes, each of which either
// names a byte out of a 256 entry substitution table or points back into the
// output that has already been produced.
//
// The layout of a file is
//
//	0..4    the magic, "LS10" or "LS11"
//	4..16   twelve zero bytes
//	16..272 a 256 byte substitution table, a permutation of 0..255
//	272..   the block table
//
// The block table is a list of twelve byte records of three big endian
// 32 bit numbers — the compressed size, the uncompressed size and the
// offset of the block — and it ends with four zero bytes.
//
// A block whose compressed size equals its uncompressed size is not
// compressed at all; its bytes are the output.
//
// The codes of a compressed block are read from a bitstream that starts at
// the first byte of the block and is read from the *least* significant bit
// of each byte upwards. Each code is
//
//	a run of one bits, a zero, and then the run length in bits
//
// where the run length counts the one bits of the run. The value of the
// code is the run of one bits read as a number, plus the factor read as a
// number:
//
//	code = run + factor
//
// A code below 256 is an index into the substitution table. A code of 256 or
// more is a back reference: the offset into the output is code - 256 and
// the length is a second code plus three.
//
// The blocks are independent of each other: each has its own bitstream and
// its own output buffer, and the back references of a block point into
// that block's own output. This is worth knowing, because the blocks of a
// file tile the payload byte for byte and it is tempting to read them as
// one continuous stream.
//
// The magic and the codec differ in nothing but the two digits: the first
// generation of the game and the second both use this scheme.

// LZW magic numbers.
const (
	// LSMagic10 is the header of the first generation of the codec.
	LSMagic10 = "LS10"
	// LSMagic11 is the header of the second generation.
	LSMagic11 = "LS11"
)

const (
	// lsHeaderSize is the size of the header, including the magic.
	lsHeaderSize = 16
	// lsDictOffset is the offset of the substitution table.
	lsDictOffset = 16
	// lsDictSize is the size of the substitution table.
	lsDictSize = 256
	// lsTableOffset is the offset of the block table.
	lsTableOffset = 272
	// lsBlockRecordSize is the size of one block table record.
	lsBlockRecordSize = 12
	// lsMatchMin is the smallest length that a back reference can have.
	// The length of a reference is the second code plus this.
	lsMatchMin = 3
)

// LSBlock is one record of the block table.
type LSBlock struct {
	// CompressedSize is the size of the block in the file.
	CompressedSize int
	// UncompressedSize is the size of the block once decoded.
	UncompressedSize int
	// Offset is the offset of the block in the file.
	Offset int
}

// LSFile is a parsed LS10 or LS11 file.
type LSFile struct {
	// Magic is the four byte magic.
	Magic string
	// Dictionary is the substitution table: 256 bytes.
	Dictionary [lsDictSize]byte
	// Blocks holds the block table.
	Blocks []LSBlock
}

// ParseLS reads an LS10 or LS11 file and returns its structure.
//
// The function does not decode the blocks; call DecodeLS for that.
func ParseLS(data []byte) (*LSFile, error) {
	if len(data) < lsTableOffset {
		return nil, fmt.Errorf("the file is too short to hold a header")
	}
	magic := string(data[:4])
	if magic != LSMagic10 && magic != LSMagic11 {
		return nil, fmt.Errorf("the magic is %q, want %q or %q", magic, LSMagic10, LSMagic11)
	}
	f := &LSFile{Magic: magic}
	copy(f.Dictionary[:], data[lsDictOffset:lsDictOffset+lsDictSize])

	// The block table runs to the end of the header area and ends with
	// four zero bytes. The check is on the compressed size alone.
	pos := lsTableOffset
	for pos+lsBlockRecordSize <= len(data) {
		c := binary.BigEndian.Uint32(data[pos : pos+4])
		u := binary.BigEndian.Uint32(data[pos+4 : pos+8])
		o := binary.BigEndian.Uint32(data[pos+8 : pos+12])
		if c == 0 {
			break
		}
		f.Blocks = append(f.Blocks, LSBlock{
			CompressedSize:   int(c),
			UncompressedSize: int(u),
			Offset:           int(o),
		})
		pos += lsBlockRecordSize
	}
	if len(f.Blocks) == 0 {
		return nil, fmt.Errorf("the block table is empty")
	}
	return f, nil
}

// DecodeLS decodes the blocks of an LS file.
//
// The function returns one byte slice per block. The total length is the
// sum of the uncompressed sizes of the blocks.
func DecodeLS(f *LSFile, data []byte) ([][]byte, error) {
	out := make([][]byte, 0, len(f.Blocks))
	for i, b := range f.Blocks {
		if b.Offset+b.CompressedSize > len(data) {
			return nil, fmt.Errorf("block %d runs past the end of the file", i)
		}
		raw := data[b.Offset : b.Offset+b.CompressedSize]
		if b.CompressedSize == b.UncompressedSize {
			// The block is not compressed.
			buf := make([]byte, len(raw))
			copy(buf, raw)
			out = append(out, buf)
			continue
		}
		buf, err := lsDecodeBlock(raw, b.CompressedSize, b.UncompressedSize, &f.Dictionary)
		if err != nil {
			return nil, fmt.Errorf("block %d: %w", i, err)
		}
		out = append(out, buf)
	}
	return out, nil
}

// lsDecodeBlock decodes one compressed block.
//
// The bitstream of a block starts at its first byte and is read most
// significant bit first: the bits of a byte come out from the top down,
// and the bytes follow each other in order.
func lsDecodeBlock(comp []byte, compSize, uncompSize int, dict *[lsDictSize]byte) ([]byte, error) {
	if compSize > len(comp) {
		return nil, fmt.Errorf("the block wants %d bytes but only %d are there", compSize, len(comp))
	}
	out := make([]byte, uncompSize)
	bitPos := 0
	bitLen := compSize * 8
	outPos := 0

	// readCode reads one code of the bitstream.
	readCode := func() (int, bool) {
		// The run of one bits, and the zero that ends it. The bits of
		// the run are read as a number as they come, and the zero that
		// ends it is part of that number.
		run := 0
		count := 0
		for {
			if bitPos >= bitLen {
				return 0, false
			}
			bit := lsBitMSB(comp, bitPos)
			bitPos++
			run = run<<1 | int(bit)
			count++
			if bit == 0 {
				break
			}
		}
		// The factor is count bits.
		factor := 0
		for i := 0; i < count; i++ {
			if bitPos >= bitLen {
				return 0, false
			}
			factor = factor<<1 | int(lsBitMSB(comp, bitPos))
			bitPos++
		}
		return run + factor, true
	}

	for outPos < uncompSize {
		code, ok := readCode()
		if !ok {
			break
		}
		if code < lsDictSize {
			out[outPos] = dict[code]
			outPos++
			continue
		}
		// A back reference into the output of this block.
		offset := code - lsDictSize
		length, ok := readCode()
		if !ok {
			break
		}
		length += lsMatchMin
		if offset <= 0 || offset > outPos {
			return nil, fmt.Errorf("the back reference points %d bytes back, but only %d are there", offset, outPos)
		}
		if outPos+length > uncompSize {
			length = uncompSize - outPos
		}
		// The copy may overlap, so it has to run forwards.
		for i := 0; i < length; i++ {
			out[outPos] = out[outPos-offset]
			outPos++
		}
	}
	return out[:outPos], nil
}

// lsBitMSB returns the bit at the given index, counting from the most
// significant bit of the first byte.
func lsBitMSB(data []byte, index int) byte {
	return data[index>>3] >> uint(7-index&7) & 1
}
