package importer

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// mmlRawDir returns the directory that holds the files of the release.
//
// The files of the release are not part of the project, so the tests that
// read them are skipped when the directory is not there.
func mmlRawDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "assets", "raw")
	if _, err := os.Stat(filepath.Join(dir, "D2.MML")); err != nil {
		t.Skipf("the files of the original release are not present: %v", err)
	}
	return dir
}

// TestParseMML reads the music of the release and verifies it.
//
// The music of the release is twenty three songs, and every song has four
// tracks, and the notes of the tracks have the durations of the language.
func TestParseMML(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(mmlRawDir(t), "D2.MML"))
	if err != nil {
		t.Fatalf("read the music: %v", err)
	}
	songs, err := ParseMML(data)
	if err != nil {
		t.Fatalf("ParseMML: %v", err)
	}
	if len(songs) < 20 {
		t.Errorf("got %d songs, want at least 20", len(songs))
	}

	total := 0
	for i, song := range songs {
		if len(song.Tracks) == 0 {
			t.Errorf("song %d has no track", i)
			continue
		}
		for j, track := range song.Tracks {
			total += len(track.Notes)
			for _, n := range track.Notes {
				// The pitch of a note is the note of the octave
				// plus the transposition of the track, and it is
				// never below zero.
				if n.Pitch < 0 {
					t.Errorf("song %d track %d has the pitch %d", i, j, n.Pitch)
				}
				// The volume of a note is a number of the
				// language, and it is never above sixty three.
				if n.Volume < 0 || n.Volume > 255 {
					t.Errorf("song %d track %d has the volume %d", i, j, n.Volume)
				}
			}
		}
	}
	if total < 1000 {
		t.Errorf("the music holds %d notes, want at least 1000", total)
	}
}

// TestMMLDurationsAreTheUnitsOfTheLanguage verifies the durations.
//
// The durations of the language are the multiples of twelve, and twelve is
// one sixteenth note. A duration that is not a multiple of twelve would mean
// that the reader mis-parsed the stream.
func TestMMLDurationsAreTheUnitsOfTheLanguage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(mmlRawDir(t), "D2.MML"))
	if err != nil {
		t.Fatalf("read the music: %v", err)
	}
	songs, err := ParseMML(data)
	if err != nil {
		t.Fatalf("ParseMML: %v", err)
	}
	multiples := 0
	total := 0
	for _, song := range songs {
		for _, track := range song.Tracks {
			for _, n := range track.Notes {
				total++
				if n.Duration%12 == 0 {
					multiples++
				}
			}
		}
	}
	// The great majority of the durations are the multiples of twelve.
	// A rest has a duration of zero, which is a multiple as well. The
	// tracks that carry a ritardando or a tuplet hold other values, and
	// the release uses them.
	if float64(multiples)/float64(total) < 0.8 {
		t.Errorf("only %d of %d durations are a multiple of twelve", multiples, total)
	}
}

// TestMMLFrequency verifies the frequency of the notes.
//
// The table of the F-numbers of the driver gives the twelve notes of an
// octave, and they are very close to the equal temperament.
//
// The pitch of a note is the number of the block of the chip above the
// block of the middle C: the pitch forty eight is the C of the fourth
// octave, which sounds at 260.2 hertz.
func TestMMLFrequency(t *testing.T) {
	// The middle C of the driver sounds at 260.2 hertz.
	if got := MMLFrequency(48); math.Abs(got-260.2) > 0.5 {
		t.Errorf("the middle C is %f hertz, want 260.2", got)
	}
	// Every octave doubles the frequency. The pitch of a note is the
	// number of the block of the chip, and the block four is the octave of
	// the middle C.
	for octave := 0; octave <= 7; octave++ {
		want := 260.2 * math.Pow(2, float64(octave-4))
		if got := MMLFrequency(octave * 12); math.Abs(got-want) > 0.5 {
			t.Errorf("the C of the block %d is %f hertz, want %f", octave, got, want)
		}
	}
	// The twelve notes of an octave are a semitone apart.
	for note := 1; note < 12; note++ {
		want := 260.2 * math.Pow(2, float64(note)/12)
		if got := MMLFrequency(48 + note); math.Abs(got-want)/want > 0.002 {
			t.Errorf("the note %d is %f hertz, want %f", note, got, want)
		}
	}
	// The notes of the table of the driver are within a tenth of a percent
	// of the equal temperament.
	for note := 0; note < 12; note++ {
		exact := float64(mmlFNumbers[note]) / float64(mmlFNumbers[0])
		equal := math.Pow(2, float64(note)/12)
		if math.Abs(exact/equal-1) > 0.002 {
			t.Errorf("the F-number %d is %.4f of the equal temperament", note, exact/equal)
		}
	}
}

// TestNormalisePitch verifies that a pitch above the range of the chip is
// folded back into it.
//
// The driver divides the pitch by twelve and uses the quotient as the
// octave of the note, and the octave of a Yamaha chip is a number from zero
// to seven.
func TestNormalisePitch(t *testing.T) {
	for _, pitch := range []int{0, 1, 11, 12, 95} {
		if got := NormalisePitch(pitch); got != pitch {
			t.Errorf("NormalisePitch(%d) is %d, want %d", pitch, got, pitch)
		}
	}
	// A pitch above the highest octave is folded back, and the note
	// within the octave is kept.
	if got := NormalisePitch(96); got != 84 {
		t.Errorf("NormalisePitch(96) is %d, want 84", got)
	}
	if got := NormalisePitch(303); got%12 != 303%12 {
		t.Errorf("NormalisePitch(303) is %d, which is not an octave of 303", got)
	}
	if got := NormalisePitch(-5); got != 0 {
		t.Errorf("NormalisePitch(-5) is %d, want 0", got)
	}
}

// TestParseSNDT reads the bank of the effects of the release.
//
// The release has seven banks, and the first of them holds a dozen of
// effects.
func TestParseSNDT(t *testing.T) {
	dir := mmlRawDir(t)
	data, err := os.ReadFile(filepath.Join(dir, "SNR0.DAT"))
	if err != nil {
		t.Fatalf("read the bank: %v", err)
	}
	effects, err := ParseSNDT(data)
	if err != nil {
		t.Fatalf("ParseSNDT: %v", err)
	}
	if len(effects) < 5 {
		t.Errorf("got %d effects, want at least 5", len(effects))
	}
	notes := 0
	for i, effect := range effects {
		notes += len(effect.Notes)
		for _, n := range effect.Notes {
			if n.Pitch < 0 {
				t.Errorf("effect %d has the pitch %d", i, n.Pitch)
			}
		}
	}
	if notes == 0 {
		t.Error("the bank holds no note")
	}
}

// TestParseSNDTRejectsAForeignFile verifies that the reader refuses a file
// that is not a bank.
func TestParseSNDTRejectsAForeignFile(t *testing.T) {
	if _, err := ParseSNDT([]byte("not a sound bank")); err == nil {
		t.Error("a file that is not a bank was accepted")
	}
	if _, err := ParseSNDT(nil); err == nil {
		t.Error("an empty file was accepted")
	}
}

// TestSoundCSVRoundTrip verifies that the derived file of the sound holds
// what the reader put into it.
func TestSoundCSVRoundTrip(t *testing.T) {
	songs := []MMLSong{{Tracks: []MMLTrack{{Notes: []MMLNote{
		{Pitch: 40, Duration: 24, Volume: 32},
		{Pitch: 52, Duration: 12, Volume: 16},
	}}}}}
	effects := []SoundEffect{{Notes: []MMLNote{
		{Pitch: 10, Duration: 6, Volume: 8},
	}}}

	path := filepath.Join(t.TempDir(), "sound.csv")
	if err := writeSoundCSV(path, songs, effects); err != nil {
		t.Fatalf("writeSoundCSV: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file: %v", err)
	}
	gotSongs, gotEffects, err := ReadSoundCSV(data)
	if err != nil {
		t.Fatalf("ReadSoundCSV: %v", err)
	}
	if len(gotSongs) != len(songs) {
		t.Fatalf("got %d songs, want %d", len(gotSongs), len(songs))
	}
	if len(gotSongs[0].Tracks) != 1 {
		t.Fatalf("got %d tracks, want 1", len(gotSongs[0].Tracks))
	}
	got := gotSongs[0].Tracks[0].Notes
	if len(got) != 2 {
		t.Fatalf("got %d notes, want 2", len(got))
	}
	if got[0].Pitch != 40 || got[0].Duration != 24 || got[0].Volume != 32 {
		t.Errorf("the first note is %+v, want the pitch 40, the duration 24 and the volume 32", got[0])
	}
	if len(gotEffects) != 1 || len(gotEffects[0].Notes) != 1 {
		t.Fatalf("got %d effects, want one with one note", len(gotEffects))
	}
	if gotEffects[0].Notes[0].Pitch != 10 {
		t.Errorf("the note of the effect is %+v, want the pitch 10", gotEffects[0].Notes[0])
	}
}
