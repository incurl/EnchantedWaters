package audio

import (
	"math"
	"testing"

	"github.com/peter/enchantedwaters/pkg/importer"
)

// TestRenderSong verifies that a song is rendered into sound.
//
// The function returns samples, and every one of them is in the range of the
// sound, and the sound is not silent.
func TestRenderSong(t *testing.T) {
	song := importer.MMLSong{Tracks: []importer.MMLTrack{
		{Notes: []importer.MMLNote{
			{Pitch: 48, Duration: 24, Volume: 32},
			{Pitch: 55, Duration: 24, Volume: 32},
		}},
		{Notes: []importer.MMLNote{
			{Pitch: 36, Duration: 48, Volume: 16},
		}},
	}}
	synth := NewSynth(2, 120)
	samples := synth.RenderSong(song)
	if len(samples) == 0 {
		t.Fatal("the song rendered nothing")
	}

	peak := 0.0
	for _, v := range samples {
		if math.Abs(v) > peak {
			peak = math.Abs(v)
		}
	}
	if peak == 0 {
		t.Fatal("the song is silent")
	}
	if peak > 1 {
		t.Errorf("the peak of the song is %f, which is over the range of the sound", peak)
	}

	// The length of the song: both tracks hold forty eight units, which
	// are four sixteenth notes. A sixteenth note lasts a quarter of a
	// beat, and a beat lasts half a second at a hundred and twenty beats
	// to the minute, so the song lasts half a second.
	want := 4.0 * (60.0 / 120.0) / 4.0 * SampleRate
	if got := float64(len(samples)); math.Abs(got-want) > SampleRate*0.05 {
		t.Errorf("the song is %d samples long, want about %d", int(got), int(want))
	}
}

// TestRenderSongIsSilentWithoutNotes verifies that a song without notes
// renders nothing.
func TestRenderSongIsSilentWithoutNotes(t *testing.T) {
	synth := NewSynth(1, 120)
	if got := synth.RenderSong(importer.MMLSong{}); got != nil {
		t.Errorf("an empty song rendered %d samples", len(got))
	}
	if got := synth.RenderSong(importer.MMLSong{Tracks: []importer.MMLTrack{{}}}); got != nil {
		t.Errorf("a song with an empty track rendered %d samples", len(got))
	}
}

// TestNoteVolume verifies the volume of a note.
//
// The volume of the language is a number from zero to sixty three, and the
// driver divides it by sixty three.
func TestNoteVolume(t *testing.T) {
	if got := noteVolume(0); got != 0 {
		t.Errorf("the volume of a silent note is %f, want 0", got)
	}
	if got := noteVolume(63); got <= 0 {
		t.Errorf("the volume of the loudest note is %f, want a positive number", got)
	}
	if got := noteVolume(1000); got != noteVolume(63) {
		t.Errorf("a volume above the range is %f, want %f", got, noteVolume(63))
	}
	if got := noteVolume(-5); got != 0 {
		t.Errorf("a negative volume is %f, want 0", got)
	}
}

// TestNoteSeconds verifies the length of a note.
//
// The durations of the language are the multiples of twelve, and twelve is
// one sixteenth note.
func TestNoteSeconds(t *testing.T) {
	sixteenth := 0.125
	if got := noteSeconds(12, sixteenth); math.Abs(got-sixteenth) > 1e-9 {
		t.Errorf("a duration of twelve is %f seconds, want %f", got, sixteenth)
	}
	if got := noteSeconds(24, sixteenth); math.Abs(got-2*sixteenth) > 1e-9 {
		t.Errorf("a duration of twenty four is %f seconds, want %f", got, 2*sixteenth)
	}
	if got := noteSeconds(0, sixteenth); got != 0 {
		t.Errorf("a duration of zero is %f seconds, want 0", got)
	}
}

// TestSynthTempo verifies the tempo of the synthesiser.
func TestSynthTempo(t *testing.T) {
	synth := NewSynth(4, 0)
	if synth.Tempo() != DefaultTempo {
		t.Errorf("the tempo is %d, want %d", synth.Tempo(), DefaultTempo)
	}
	synth.SetTempo(90)
	if synth.Tempo() != 90 {
		t.Errorf("the tempo is %d, want 90", synth.Tempo())
	}
	// A tempo of zero is refused.
	synth.SetTempo(0)
	if synth.Tempo() != 90 {
		t.Errorf("the tempo is %d, want 90", synth.Tempo())
	}
}

// TestSynthPhasesAreSeparate verifies that the voices of the synthesiser do
// not share their phase.
//
// Two voices that played the same note would sound like one loud voice if
// they shared their phase.
func TestSynthPhasesAreSeparate(t *testing.T) {
	synth := NewSynth(3, 120)
	if len(synth.phases) != 3 {
		t.Fatalf("the synthesiser has %d voices, want 3", len(synth.phases))
	}
}
