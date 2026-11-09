package importer

import (
	"encoding/binary"
	"fmt"
)

// This file reads the sound effects of the original game.
//
// The effects live in a bank whose magic is "SNDT". The bank begins with
// the magic and the number of the bank, and the offsets of the effects
// follow; a zero ends nothing and a run of ones ends the table.
//
// The effects themselves are written in the same command language as the
// music, and they end with the two bytes 0xF8 0xF2, which the driver reads
// as a pointer back into the stream.
//
// The two kinds of sound of the original are therefore read by one parser:
// a stream of commands, and the notes that they produce.

// SNDTMagic is the magic of a bank of sound effects.
const SNDTMagic = "SNDT"

// The bytes that end a sound effect.
const (
	sndtEnd0 = 0xF8
	sndtEnd1 = 0xF2
)

// SoundEffect is one sound effect of the original game.
type SoundEffect struct {
	// Name is the name of the effect, when the bank carries one.
	Name string
	// Notes are the notes of the effect, in order.
	Notes []MMLNote
	// Transpose is the transposition of the effect in semitones.
	Transpose int
}

// ParseSNDT reads the bank of sound effects.
//
// The function returns the effects of the bank, in the order of the table.
func ParseSNDT(data []byte) ([]SoundEffect, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("the bank of the sound effects is too short")
	}
	if string(data[:4]) != SNDTMagic {
		return nil, fmt.Errorf("the file does not begin with the magic of a sound bank")
	}
	offsets, err := sndtOffsets(data)
	if err != nil {
		return nil, err
	}
	out := make([]SoundEffect, 0, len(offsets))
	for i, off := range offsets {
		end := len(data)
		if i+1 < len(offsets) {
			end = offsets[i+1]
		}
		effect, err := parseSNDEffect(data, off, end)
		if err != nil {
			return nil, err
		}
		out = append(out, effect)
	}
	return out, nil
}

// sndtOffsets reads the table of the effects.
//
// The offsets are sixteen bit and big endian, and a run of ones ends the
// table.
func sndtOffsets(data []byte) ([]int, error) {
	var offsets []int
	pos := 16
	for pos+4 <= len(data) {
		off := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		pos += 4
		if off == 0xFFFFFFFF {
			break
		}
		if off >= len(data) {
			return nil, fmt.Errorf("the table of the effects names the offset %d, which is past the end of the file", off)
		}
		offsets = append(offsets, off)
	}
	if len(offsets) == 0 {
		return nil, fmt.Errorf("the bank holds no effect")
	}
	return offsets, nil
}

// parseSNDEffect reads one effect.
//
// The effect runs from its offset to the offset of the next effect, and the
// stream ends with the two bytes that point back into it.
func parseSNDEffect(data []byte, off, end int) (SoundEffect, error) {
	effect := SoundEffect{}
	pos := off
	volume, duration := 0, 0
	for pos < end {
		// The end of the stream.
		if pos+1 < end && data[pos] == sndtEnd0 && data[pos+1] == sndtEnd1 {
			break
		}
		cmd := data[pos]
		pos++
		switch {
		case cmd <= 0x5F:
			if pos+2 > end {
				return effect, nil
			}
			volume = int(data[pos])
			duration = int(data[pos+1])
			pos += 2
		case cmd <= 0xBF:
			effect.Notes = append(effect.Notes, MMLNote{
				Pitch:    int(cmd&0x3F) + effect.Transpose,
				Duration: duration,
				Volume:   volume,
			})
		default:
			if pos >= end {
				return effect, nil
			}
			param := int(data[pos])
			pos++
			if cmd == 0xC0 {
				effect.Transpose = param
			}
		}
	}
	return effect, nil
}
