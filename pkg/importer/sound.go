package importer

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This file converts the sound of the original game.
//
// The music of the release is a file of the language of the sound driver,
// and the effects live in seven banks. Both are read by the parsers of this
// package and written out as a plain text format that the game reads.
//
// The text format is one line per note, and the fields are separated by
// commas:
//
//	kind,index,track,pitch,duration,volume
//
// The kind is "m" for a note of the music and "e" for a note of an effect.
// The index is the number of the song for the music and the number of the
// effect for the effects, and the track is the number of the track inside
// it. The pitch is a number of semitones, the duration a number of the
// units of the tempo and the volume a number from zero to sixty three.

// The names of the derived files of the sound.
const (
	// SoundMusicFile holds the music of the release.
	SoundMusicFile = "music.csv"
	// SoundEffectFile holds the sound effects of the release.
	SoundEffectFile = "effects.csv"
)

// ExportSound converts the music and the effects of the release.
//
// The music lives in D2.MML, and the effects in the seven banks SNR0.DAT to
// SNR6.DAT. A file that is missing is reported in the returned problems
// instead of stopping the export.
func ExportSound(rawDir, outDir string, log Logger) (files, problems []string) {
	// --- the music ---
	musicPath := filepath.Join(rawDir, "D2.MML")
	data, err := os.ReadFile(musicPath)
	if err != nil {
		// The music is optional: a release without it still has the
		// effects, and the game plays them.
		problems = append(problems, fmt.Sprintf("the music: %v", err))
	} else {
		songs, err := ParseMML(data)
		if err != nil {
			problems = append(problems, fmt.Sprintf("the music: %v", err))
		} else {
			out := filepath.Join(outDir, SoundMusicFile)
			if err := writeSoundCSV(out, songs, nil); err != nil {
				problems = append(problems, fmt.Sprintf("the music: %v", err))
			} else {
				files = append(files, out)
				notes := 0
				for _, s := range songs {
					for _, t := range s.Tracks {
						notes += len(t.Notes)
					}
				}
				log.Info("wrote %s: %d songs, %d notes", out, len(songs), notes)
			}
		}
	}

	// --- the effects ---
	var effects []SoundEffect
	for i := 0; i < 7; i++ {
		path := filepath.Join(rawDir, fmt.Sprintf("SNR%d.DAT", i))
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("the bank of the effects %d: %v", i, err))
			continue
		}
		bank, err := ParseSNDT(data)
		if err != nil {
			problems = append(problems, fmt.Sprintf("the bank of the effects %d: %v", i, err))
			continue
		}
		for _, e := range bank {
			e.Name = fmt.Sprintf("%d", i)
			effects = append(effects, e)
		}
	}
	if len(effects) > 0 {
		out := filepath.Join(outDir, SoundEffectFile)
		if err := writeSoundCSV(out, nil, effects); err != nil {
			problems = append(problems, fmt.Sprintf("the effects: %v", err))
		} else {
			files = append(files, out)
			log.Info("wrote %s: %d effects", out, len(effects))
		}
	}
	return files, problems
}

// writeSoundCSV writes the notes of the music and of the effects.
//
// The two kinds of sound share one file, and the kind of every line says
// which one it is.
func writeSoundCSV(path string, songs []MMLSong, effects []SoundEffect) error {
	var b strings.Builder
	b.WriteString("kind,index,track,pitch,duration,volume\n")
	for i, song := range songs {
		for j, track := range song.Tracks {
			for _, n := range track.Notes {
				fmt.Fprintf(&b, "m,%d,%d,%d,%d,%d\n", i, j, n.Pitch, n.Duration, n.Volume)
			}
		}
	}
	for i, effect := range effects {
		for _, n := range effect.Notes {
			fmt.Fprintf(&b, "e,%d,0,%d,%d,%d\n", i, n.Pitch, n.Duration, n.Volume)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ReadSoundCSV reads the derived file of the sound.
//
// The function returns the songs of the music and the effects, in the order
// of the file.
func ReadSoundCSV(data []byte) ([]MMLSong, []SoundEffect, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return nil, nil, fmt.Errorf("the file of the sound is empty")
	}
	var songs []MMLSong
	var effects []SoundEffect
	songTracks := map[int]map[int][]MMLNote{}
	effectNotes := map[int][]MMLNote{}

	for i, line := range lines {
		if i == 0 {
			// The heading of the table.
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 6 {
			return nil, nil, fmt.Errorf("line %d of the file of the sound is not a record: %q", i+1, line)
		}
		var (
			kind   = fields[0]
			index  = atoiSafe(fields[1])
			track  = atoiSafe(fields[2])
			pitch  = atoiSafe(fields[3])
			dur    = atoiSafe(fields[4])
			volume = atoiSafe(fields[5])
		)
		switch kind {
		case "m":
			tracks, ok := songTracks[index]
			if !ok {
				tracks = map[int][]MMLNote{}
				songTracks[index] = tracks
			}
			tracks[track] = append(tracks[track], MMLNote{
				Pitch: pitch, Duration: dur, Volume: volume,
			})
		case "e":
			effectNotes[index] = append(effectNotes[index], MMLNote{
				Pitch: pitch, Duration: dur, Volume: volume,
			})
		}
	}

	// The songs, in the order of their numbers.
	indices := make([]int, 0, len(songTracks))
	for index := range songTracks {
		indices = append(indices, index)
	}
	sortInts(indices)
	for _, index := range indices {
		song := MMLSong{}
		tracks := songTracks[index]
		trackIndices := make([]int, 0, len(tracks))
		for t := range tracks {
			trackIndices = append(trackIndices, t)
		}
		sortInts(trackIndices)
		for _, t := range trackIndices {
			song.Tracks = append(song.Tracks, MMLTrack{Notes: tracks[t]})
		}
		songs = append(songs, song)
	}

	effectIndices := make([]int, 0, len(effectNotes))
	for index := range effectNotes {
		effectIndices = append(effectIndices, index)
	}
	sortInts(effectIndices)
	for _, index := range effectIndices {
		effects = append(effects, SoundEffect{Notes: effectNotes[index]})
	}
	return songs, effects, nil
}

// atoiSafe reads a number, and gives zero for a field that is not a number.
func atoiSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var n int
	neg := false
	for i, c := range s {
		if i == 0 && c == '-' {
			neg = true
			continue
		}
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		return -n
	}
	return n
}

// sortInts sorts a list of numbers in place.
//
// The package does not pull in the whole sort package for one function.
func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i - 1
		for j >= 0 && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}

// binaryBigEndian is used by the readers of the derived data.
var _ = binary.BigEndian
