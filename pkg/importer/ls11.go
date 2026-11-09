package importer

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// This file implements the LS10/LS11 codec of the DOS releases of
// Uncharted Waters.
//
// The format is called "LZW" by the files that carry it, but that is a
// misnomer. There is no dictionary of phrases and no sliding window. The
// payload is a single stream of variable length codes, each of which
// selects one byte out of a 256 entry substitution table, and the length of
// a code is its own value in a self describing way.
//
// A code is read as follows. The reader counts the zeros that follow until
// the first one; call that count k. Then it reads k+1 bits, most
// significant first, as a number v. The code is
//
//	symbol = 2^(k+1) + v - 2
//
// which is the Elias gamma code of the symbol with the two lowest values
// removed. A symbol of zero is therefore the two bits "1 0" and a symbol
// of one is "1 0 1", and the table is consulted with the symbol as an
// index.
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
// The payload of the file is one continuous bitstream that starts at the
// offset of the first block. The blocks divide it into pieces of the
// uncompressed size, and the pieces are *not* aligned to byte boundaries,
// which is why a decoder that restarts at each block fails on everything
// but the smallest ones.
//
// The file does not say which symbol stands for which byte of the output;
// the substitution table does. The table is a permutation of the 256 byte
// values and it is ordered by how often the bytes occur in the file, so
// that the common bytes get the short codes.
//
// The LS10 and LS11 headers differ in nothing but the two digits. Both
// magics appear in the same release and both use this codec.

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
)

// LSBlock is one record of the block table.
type LSBlock struct {
	// CompressedSize is the size of the block in the payload.
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
	// Payload is the compressed stream.
	Payload []byte
}

// ParseLS reads an LS10 or LS11 file and returns its structure.
//
// The function does not decode the payload; call DecodeLS for that.
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
	// four zero bytes. The check is on the compressed size alone: a
	// block whose compressed size is zero is the terminator, and the
	// rest of the record is payload again.
	pos := lsTableOffset
	for pos+lsBlockRecordSize <= len(data) {
		c := binary.BigEndian.Uint32(data[pos : pos+4])
		u := binary.BigEndian.Uint32(data[pos+4 : pos+8])
		o := binary.BigEndian.Uint32(data[pos+8 : pos+12])
		if c == 0 {
			// The terminator is four zero bytes and the payload starts
			// right after it.
			pos += 4
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
	// The offset of the first block is where the payload starts, and the
	// offset of the last block plus its compressed size is where it ends.
	// The two are not always the ends of the file: KAO.LZW carries more
	// data behind its blocks.
	first := f.Blocks[0].Offset
	last := f.Blocks[len(f.Blocks)-1]
	end := last.Offset + last.CompressedSize
	if first > len(data) || end > len(data) {
		return nil, fmt.Errorf("the blocks run from %d to %d, past the end of the file", first, end)
	}
	f.Payload = data[first:end]
	return f, nil
}

// DecodeLS decodes the payload of an LS file into its blocks.
//
// The payload is one continuous bitstream. The blocks tile it byte-wise —
// the offset of a block is the offset of its predecessor plus the
// compressed size of that predecessor — but the *codes* cross the byte
// boundaries, so a decoder that restarts the bit reader at every block
// fails on everything but the smallest ones.
//
// The decoded bytes of a block are the values that the substitution table
// gives for the symbols of that piece.
//
// The function returns one byte slice per block. The total length is the
// sum of the uncompressed sizes of the blocks.
func DecodeLS(f *LSFile) ([][]byte, error) {
	symbols, err := lsDecodeSymbols(f.Payload)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, 0, len(f.Blocks))
	pos := 0
	for i, b := range f.Blocks {
		if pos+b.UncompressedSize > len(symbols) {
			return nil, fmt.Errorf("block %d needs %d bytes but only %d symbols were decoded",
				i, b.UncompressedSize, len(symbols)-pos)
		}
		chunk := symbols[pos : pos+b.UncompressedSize]
		pos += b.UncompressedSize
		bytes := make([]byte, len(chunk))
		for j, s := range chunk {
			if s < 0 || s >= lsDictSize {
				return nil, fmt.Errorf("block %d: the symbol %d at offset %d is outside the table", i, s, j)
			}
			bytes[j] = f.Dictionary[s]
		}
		out = append(out, bytes)
	}
	return out, nil
}

// lsFraming says which bit ends the unary prefix of a code.
type lsFraming int

const (
	// lsFramingZero ends the unary prefix with a zero, so the prefix
	// itself is a run of ones.
	//
	// This is the framing of the DOS release of the second game and of
	// the reference implementation of the Mega Drive release.
	lsFramingZero lsFraming = iota
	// lsFramingOne ends the unary prefix with a one, so the prefix
	// itself is a run of zeros.
	//
	// This is the framing of the scenario data and of the first game.
	lsFramingOne
)

// lsDecodeSymbols reads the variable length codes of a payload.
//
// A code is a unary prefix, the bit that ends it, and then k+1 bits most
// significant first; its value is 2^(k+1) + v - 2. The reading stops at
// the end of the payload or at a malformed code.
func lsDecodeSymbols(payload []byte) ([]int, error) {
	return lsDecodeSymbolsFramed(payload, lsFramingZero)
}

// lsDecodeSymbolsFramed reads the variable length codes of a payload with
// the given framing.
//
// The framing names the bit that ends the unary prefix; the prefix itself
// is a run of the other bit.
func lsDecodeSymbolsFramed(payload []byte, framing lsFraming) ([]int, error) {
	terminator := byte(0)
	if framing == lsFramingOne {
		terminator = 1
	}
	prefixBit := 1 - terminator
	symbols := make([]int, 0, len(payload)*8/5)
	bitPos := 0
	bitLen := len(payload) * 8
	for bitPos < bitLen {
		// Count the bits of the unary prefix.
		k := 0
		for bitPos < bitLen && lsBit(payload, bitPos) == prefixBit {
			k++
			bitPos++
		}
		if bitPos >= bitLen {
			break
		}
		// The next bit must be the one that ends the prefix.
		if lsBit(payload, bitPos) != terminator {
			return nil, fmt.Errorf("malformed code at bit %d: the unary prefix is not terminated", bitPos)
		}
		bitPos++
		// The factor is k+1 bits, most significant first.
		width := k + 1
		if bitPos+width > bitLen {
			return nil, fmt.Errorf("malformed code at bit %d: the factor runs past the end of the payload", bitPos)
		}
		value := 0
		for i := 0; i < width; i++ {
			value = value<<1 | int(lsBit(payload, bitPos+i))
		}
		bitPos += width
		symbols = append(symbols, (1<<width)+value-2)
	}
	return symbols, nil
}

// lsBit returns the bit at the given index, counting from the most
// significant bit of the first byte.
func lsBit(data []byte, index int) byte {
	return data[index>>3] >> (7 - uint(index&7)) & 1
}

// lsEncodeSymbols writes the variable length codes of a list of symbols.
//
// It is the inverse of lsDecodeSymbols and exists so that a decoder can be
// proved against the data: encoding the symbols of a payload and comparing
// the result with the payload must reproduce it byte for byte.
func lsEncodeSymbols(symbols []int) []byte {
	return lsEncodeSymbolsFramed(symbols, lsFramingZero)
}

// lsEncodeSymbolsFramed writes the variable length codes of a list of
// symbols with the given framing.
func lsEncodeSymbolsFramed(symbols []int, framing lsFraming) []byte {
	// The framing names the bit that *ends* the unary prefix: the
	// prefix is a run of the other bit.
	terminator := byte(0)
	if framing == lsFramingOne {
		terminator = 1
	}
	prefixBit := 1 - terminator
	stream := make([]byte, 0, len(symbols)*5)
	for _, s := range symbols {
		// The code is the Elias gamma code of s+2. The number is
		// written with its leading one implied, so the factor that
		// follows the prefix is one bit shorter than the number.
		n := s + 2
		factor := bits.Len(uint(n)) - 1
		// The prefix is factor-1 bits of the prefix value, the bit that
		// ends it, and then the factor, which is the number without
		// its leading one.
		for i := 0; i < factor-1; i++ {
			stream = append(stream, prefixBit)
		}
		stream = append(stream, terminator)
		for i := factor - 1; i >= 0; i-- {
			stream = append(stream, byte(n>>uint(i)&1))
		}
	}
	out := make([]byte, (len(stream)+7)/8)
	for i, b := range stream {
		if b != 0 {
			out[i>>3] |= 1 << (7 - uint(i&7))
		}
	}
	return out
}
