package importer

import (
	"encoding/binary"
	"fmt"
	"math"
)

// This file reads the music of the original game.
//
// The music is written in a small command language that the sound driver of
// the release interprets. The driver is the ground truth of the format, and
// the reading below follows it instruction by instruction.
//
// # The table of the songs
//
// The file begins with a table of the offsets of the tracks. The offsets are
// sixteen bit and little endian, and a zero ends the tracks of a song: the
// table is a list of songs, and a song is a list of tracks.
//
// # The commands
//
// Every command is one byte, and the byte says how many bytes follow it:
//
//	byte 0x00 - 0x5F  a command with two parameters
//	byte 0x60 - 0xBF  a note
//	byte 0xC0 - 0xFF  a command with one parameter
//
// The notes are the interesting part. A note byte carries the pitch in its
// lower six bits: the driver takes the byte, subtracts 0x60 from it and adds
// the transposition of the channel and the transposition of the driver,
// which gives a number of semitones. That number divided by twelve is the
// octave and the rest is the note within the octave.
//
// A note is held for as long as the duration that the last command with two
// parameters set. The first of the two parameters is the volume of the
// channel and the second is the length of the notes; the second holds the
// values of the standard durations of the language, which are the multiples
// of twelve: 0x0C for one, 0x18 for two, 0x24 for three and so on.
//
// The commands with one parameter set the pitch bend, the timbre and the
// like; the driver reads them through a table of handlers.

// The F-numbers of the twelve notes of an octave.
//
// The table is the one of the sound driver of the release, and it holds the
// F-number register of the YM3812 for every semitone of the octave. The
// value 343 is the F-number of the C below the middle C, which the driver
// plays at 260.2 hertz.
var mmlFNumbers = [12]int{343, 363, 385, 408, 432, 458, 485, 514, 544, 577, 611, 647}

// MMLNote is one note of a track of the original game.
//
// The note is not a frequency: it is a pitch, and the driver adds the
// transposition of the channel and of itself to it.
type MMLNote struct {
	// Pitch is the note in semitones above the C of the lowest octave.
	Pitch int
	// Duration is the length of the note in the units of the tempo of the
	// track. The driver counts the units down, and a unit is a fraction
	// of the tempo of the track.
	Duration int
	// Volume is the volume of the note, from zero to sixty three.
	Volume int
}

// MMLTrack is one track of one song.
type MMLTrack struct {
	// Notes are the notes of the track, in order.
	Notes []MMLNote
	// Transpose is the transposition of the track in semitones.
	Transpose int
}

// MMLSong is one song of the original game.
type MMLSong struct {
	// Tracks are the tracks of the song, in order.
	Tracks []MMLTrack
}

// ParseMML reads the music of the original game.
//
// The function returns the songs of the file, in the order of the table.
func ParseMML(data []byte) ([]MMLSong, error) {
	groups, flat, err := mmlTrackOffsets(data)
	if err != nil {
		return nil, err
	}
	// The end of a track is the offset of the next track of the table, and
	// the tracks of the songs follow each other in the file.
	ends := make([]int, len(flat)+1)
	for i := range flat {
		if i+1 < len(flat) {
			ends[i] = flat[i+1]
		} else {
			ends[i] = len(data)
		}
	}
	endOf := map[int]int{}
	for i, off := range flat {
		endOf[off] = ends[i]
	}

	songs := make([]MMLSong, 0, len(groups))
	for _, group := range groups {
		song := MMLSong{}
		for _, off := range group {
			track, err := parseMMLTrack(data, off, endOf[off])
			if err != nil {
				return nil, err
			}
			song.Tracks = append(song.Tracks, track)
		}
		songs = append(songs, song)
	}
	return songs, nil
}

// mmlTrackOffsets reads the table of the tracks.
//
// The table is a list of songs, and every song is a list of tracks. A zero
// ends a song, and the end of the table ends the list of the songs.
//
// The table itself ends with a marker: the value that follows the last song
// is not an offset but the end of the table. The driver reads the table
// until the offset it holds is not inside the file, and the release writes
// the count of the songs there.
func mmlTrackOffsets(data []byte) ([][]int, []int, error) {
	var groups [][]int
	var group []int
	var flat []int
	pos := 0
	for pos+2 <= len(data) {
		off := int(binary.LittleEndian.Uint16(data[pos : pos+2]))
		pos += 2
		if off == 0 {
			if len(group) > 0 {
				groups = append(groups, group)
				group = nil
			}
			continue
		}
		if off >= len(data) {
			// The end of the table. The release writes the count of
			// the songs there, and the value is not an offset.
			break
		}
		group = append(group, off)
		flat = append(flat, off)
	}
	if len(group) > 0 {
		groups = append(groups, group)
	}
	if len(groups) == 0 {
		return nil, nil, fmt.Errorf("the file holds no song")
	}
	return groups, flat, nil
}

// parseMMLTrack reads one track.
//
// The track runs from its offset to the next offset of the table, or to the
// end of the file for the last one.
func parseMMLTrack(data []byte, off, end int) (MMLTrack, error) {
	track := MMLTrack{}
	pos := off
	volume, duration := 0, 0
	for pos < end {
		cmd := data[pos]
		pos++
		switch {
		case cmd <= 0x5F:
			// A command with two parameters: the length of the notes
			// that follow and the volume of the channel. The driver
			// counts the length down once for every tick of the
			// timer, and the length holds the multiples of twelve
			// of the language: twelve for one sixteenth note.
			if pos+2 > end {
				return track, nil
			}
			duration = int(data[pos])
			volume = int(data[pos+1])
			pos += 2
		case cmd <= 0xBF:
			// A note. The pitch is in the lower six bits, and the
			// transposition of the track is added to it.
			track.Notes = append(track.Notes, MMLNote{
				Pitch:    int(cmd&0x3F) + track.Transpose,
				Duration: duration,
				Volume:   volume,
			})
		default:
			// A command with one parameter.
			if pos >= end {
				return track, nil
			}
			param := int(data[pos])
			pos++
			mmlApplyCommand(&track, int(cmd), param)
		}
	}
	return track, nil
}

// mmlApplyCommand applies the commands that carry one parameter.
//
// The driver reads these through a table of handlers, and the ones that are
// understood here are the ones that change the music itself.
func mmlApplyCommand(track *MMLTrack, cmd, param int) {
	switch cmd {
	case 0xC0: // The transposition of the track.
		track.Transpose = param
	}
}

// MMLFrequency returns the frequency of a note of the original game.
//
// The pitch is a number of semitones above the C of the lowest octave, and
// the frequency follows the table of the F-numbers of the sound driver:
// every octave doubles the frequency, and the twelve notes of an octave have
// the F-numbers of the driver.
//
// The table is very slightly flat, at most a tenth of a percent away from
// the equal temperament, which is what a YM3812 with a clock of 3.579545
// megahertz does.
func MMLFrequency(pitch int) float64 {
	pitch = NormalisePitch(pitch)
	octave := pitch / 12
	note := pitch % 12
	// The frequency of the C of the octave, from the F-number 343.
	//
	// The octave of the driver is the block of the chip, and the block
	// four is the octave of the middle C: the F-number 343 at the block
	// four sounds at 260.2 hertz. The pitch of a note is therefore the
	// number of the block above that one.
	const base = 260.2
	const baseOctave = 4
	// The ratio of the F-number of the note to the F-number of the C.
	ratio := float64(mmlFNumbers[note]) / float64(mmlFNumbers[0])
	return base * math.Pow(2, float64(octave-baseOctave)) * ratio
}

// NormalisePitch brings a pitch into the range that the sound driver plays.
//
// The driver divides the pitch of a note by twelve and uses the quotient as
// the octave of the note, and the octave of a Yamaha chip is a number from
// zero to seven. A pitch whose octave is higher than seven is folded into
// the highest octave, and the note within the octave is kept.
//
// The music of the release uses the whole range, and the transposition of
// the tracks pushes some of the notes above the octave seven; the driver
// folds them back, and so does this function.
func NormalisePitch(pitch int) int {
	if pitch < 0 {
		pitch = 0
	}
	// The highest octave the chip plays.
	const maxOctave = 7
	octave, note := pitch/12, pitch%12
	if octave > maxOctave {
		octave = maxOctave
	}
	return octave*12 + note
}
