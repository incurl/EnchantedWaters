package audio

import (
	"math"

	"github.com/peter/enchantedwaters/pkg/importer"
)

// This file turns the notes of the original game into sound.
//
// The original played the music of the DOS releases on a Yamaha OPL2 or
// OPL3 sound chip, and the driver of the release wrote the notes of the
// music straight into the registers of that chip. There is no recording of
// the game to copy, so the sound is synthesised again from the same notes
// with the same scale.
//
// The result is not the sound of the adapter — a chip of 1994 has a timbre
// that no sum of sine waves reproduces — but it is the same music: the same
// pitches, the same lengths and the same tempo.

// The rate of the sound.
const (
	// SampleRate is the number of samples of a second.
	SampleRate = 44100
	// The tempo of the music of the original game, in the beats of a
	// minute.
	DefaultTempo = 120
)

// The synthesiser holds the state of the voices of the music.
//
// The state is the phase of the oscillators, so that the wave of a voice
// does not jump when a note begins.
type Synth struct {
	// rate is the number of samples of a second.
	rate int
	// tempo is the speed of the music in the beats of a minute.
	tempo int
	// phases holds the phase of every voice.
	phases []float64
}

// NewSynth creates a synthesiser.
//
// The voices are the number of the tracks of the music, and every one of
// them is a single voice that plays one note at a time.
func NewSynth(voices, tempo int) *Synth {
	if voices <= 0 {
		voices = 4
	}
	if tempo <= 0 {
		tempo = DefaultTempo
	}
	return &Synth{
		rate:   SampleRate,
		tempo:  tempo,
		phases: make([]float64, voices),
	}
}

// SetTempo sets the speed of the music.
func (s *Synth) SetTempo(tempo int) {
	if tempo > 0 {
		s.tempo = tempo
	}
}

// Tempo returns the speed of the music.
func (s *Synth) Tempo() int { return s.tempo }

// RenderSong renders a song of the original game.
//
// The function returns the samples of the song, in the range of minus one
// to one. The length of the song is the length of its longest track.
func (s *Synth) RenderSong(song importer.MMLSong) []float64 {
	if len(song.Tracks) == 0 {
		return nil
	}
	// The length of a note, in samples, for every duration of the
	// language. The durations are the multiples of twelve, and twelve is
	// one sixteenth note.
	beat := 60.0 / float64(s.tempo)
	sixteenth := beat / 4.0

	// Every track is rendered on its own and they are mixed afterwards,
	// so that a track that is longer than the others does not cut the
	// others short.
	tracks := make([][]float64, len(song.Tracks))
	longest := 0
	for i, track := range song.Tracks {
		tracks[i] = s.renderTrack(track, sixteenth)
		if len(tracks[i]) > longest {
			longest = len(tracks[i])
		}
	}
	if longest == 0 {
		return nil
	}

	mixed := make([]float64, longest)
	for _, track := range tracks {
		for i, v := range track {
			mixed[i] += v
		}
	}
	// The gain of the mix: the voices of the original were five, and
	// they did not clip.
	gain := 0.5 / math.Sqrt(float64(len(tracks)))
	for i := range mixed {
		mixed[i] *= gain
	}
	return mixed
}

// renderTrack renders one track of a song.
func (s *Synth) renderTrack(track importer.MMLTrack, sixteenth float64) []float64 {
	if len(track.Notes) == 0 {
		return nil
	}
	// The length of the track.
	total := 0.0
	for _, n := range track.Notes {
		total += noteSeconds(n.Duration, sixteenth)
	}
	if total <= 0 {
		return nil
	}
	samples := make([]float64, int(total*float64(s.rate)))
	pos := 0
	for _, n := range track.Notes {
		length := int(noteSeconds(n.Duration, sixteenth) * float64(s.rate))
		if length <= 0 {
			continue
		}
		freq := importer.MMLFrequency(n.Pitch)
		amp := noteVolume(n.Volume)
		for i := 0; i < length && pos+i < len(samples); i++ {
			// The wave of a note is a saw with a little of a square
			// in it, which is close enough to the timbre of the
			// chip to sound like the game.
			t := float64(i) / float64(s.rate)
			phase := 2 * math.Pi * freq * t
			wave := 0.6*math.Sin(phase) + 0.25*math.Sin(2*phase) + 0.15*math.Sin(3*phase)
			// The note fades away at its end, so that the notes
			// do not click.
			env := 1.0
			if tail := length / 8; i > length-tail {
				env = float64(length-i) / float64(tail)
			}
			samples[pos+i] += amp * env * wave
		}
		pos += length
	}
	return samples
}

// noteSeconds returns the length of a note in seconds.
//
// The durations of the language are the multiples of twelve, and twelve is
// one sixteenth note.
func noteSeconds(duration int, sixteenth float64) float64 {
	return float64(duration) / 12.0 * sixteenth
}

// noteVolume returns the volume of a note.
//
// The volume of the language is a number from zero to sixty three, and the
// driver divides it by sixty three.
func noteVolume(volume int) float64 {
	if volume <= 0 {
		return 0
	}
	if volume > 63 {
		volume = 63
	}
	return float64(volume) / 63.0 * 0.6
}

// renderEffect renders one sound effect of the original game.
//
// The effects are short, and they have no tempo: their length comes from
// the durations of the notes, and a duration of twelve is a fiftieth of a
// second, which is the tick of the timer of the driver.
func (s *Synth) renderEffect(effect importer.SoundEffect) []float64 {
	if len(effect.Notes) == 0 {
		return nil
	}
	// The tick of the timer of the driver: the driver counts the
	// durations of the notes down once for every tick, and the tick of
	// a machine of the era was about a fiftieth of a second.
	const tick = 1.0 / 50.0
	return s.renderTrack(importer.MMLTrack{Notes: effect.Notes}, tick)
}
