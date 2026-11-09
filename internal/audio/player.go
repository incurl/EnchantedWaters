package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/ebitengine/oto/v3"

	"github.com/peter/enchantedwaters/pkg/importer"
)

// This file plays the sound of the original game.
//
// The player keeps one context of the sound device, and every piece of
// sound is a player of its own. The context is created once, at the start
// of the game, and it is closed when the game ends.
//
// Everything is optional. A machine without a sound device, or a build
// without the sound library, leaves the game silent but working.

// Player plays the music and the effects of the game.
type Player struct {
	// ctx is the context of the sound device.
	ctx *oto.Context
	// ready is closed when the context is ready to play.
	ready chan struct{}
	// mu guards the list of the players that are playing.
	mu sync.Mutex
	// playing holds the players that are playing.
	playing []*oto.Player
	// volume is the volume of the game, from zero to one.
	volume float64
	// musicVolume is the volume of the music, from zero to one.
	musicVolume float64
	// effectVolume is the volume of the effects, from zero to one.
	effectVolume float64
	// enabled reports whether the sound is on.
	enabled bool
	// failed records that the sound device could not be opened, so that
	// the game does not try again on every sound.
	failed bool
}

// NewPlayer opens the sound device.
//
// The function returns a player that is silent when the device could not be
// opened: the game works without a sound.
func NewPlayer() (*Player, error) {
	p := &Player{
		ready:        make(chan struct{}),
		volume:       0.7,
		musicVolume:  0.6,
		effectVolume: 0.9,
		enabled:      true,
	}
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   SampleRate,
		ChannelCount: 2,
		Format:       oto.FormatFloat32LE,
	})
	if err != nil {
		p.failed = true
		return p, fmt.Errorf("open the sound device: %w", err)
	}
	p.ctx = ctx
	go func() {
		<-ready
		close(p.ready)
	}()
	return p, nil
}

// Close releases the sound device.
func (p *Player) Close() {
	if p == nil || p.ctx == nil {
		return
	}
	p.mu.Lock()
	players := p.playing
	p.playing = nil
	p.mu.Unlock()
	for _, pl := range players {
		pl.Pause()
		pl.Close()
	}
}

// SetVolume sets the volume of the whole game.
func (p *Player) SetVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = clamp01(v)
}

// SetMusicVolume sets the volume of the music.
func (p *Player) SetMusicVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.musicVolume = clamp01(v)
}

// SetEffectVolume sets the volume of the effects.
func (p *Player) SetEffectVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.effectVolume = clamp01(v)
}

// Volume returns the volume of the game.
func (p *Player) Volume() float64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// Enabled reports whether the sound is on.
func (p *Player) Enabled() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.enabled && !p.failed
}

// SetEnabled turns the sound on or off.
func (p *Player) SetEnabled(on bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = on
}

// clamp01 limits a value to the range of zero to one.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// PlayMusic plays a song of the original game.
//
// The function returns when the sound has been handed over to the device,
// not when it is over.
func (p *Player) PlayMusic(song importer.MMLSong) {
	if p == nil || !p.Enabled() {
		return
	}
	synth := NewSynth(len(song.Tracks), DefaultTempo)
	samples := synth.RenderSong(song)
	if len(samples) == 0 {
		return
	}
	p.mu.Lock()
	gain := p.volume * p.musicVolume
	p.mu.Unlock()
	p.play(samples, gain)
}

// PlayEffect plays a sound effect of the original game.
func (p *Player) PlayEffect(effect importer.SoundEffect) {
	if p == nil || !p.Enabled() {
		return
	}
	synth := NewSynth(1, DefaultTempo)
	samples := synth.renderEffect(effect)
	if len(samples) == 0 {
		return
	}
	p.mu.Lock()
	gain := p.volume * p.effectVolume
	p.mu.Unlock()
	p.play(samples, gain)
}

// play hands the samples over to the device.
//
// The samples are interleaved into the two channels of the device, and the
// volume of the game is applied to them.
func (p *Player) play(samples []float64, gain float64) {
	if p.ctx == nil {
		return
	}
	// The player waits for the device, so that the sound starts when the
	// device is ready.
	<-p.ready
	if err := p.ctx.Resume(); err != nil {
		return
	}
	player := p.ctx.NewPlayer(bytes.NewReader(p.encode(samples)))
	player.SetVolume(gain)
	p.mu.Lock()
	p.playing = append(p.playing, player)
	p.mu.Unlock()
	player.Play()
}

// encode turns the samples into the bytes of the device.
//
// The device takes thirty two bit floats, interleaved over the two
// channels, and it wants the number of the frames in front of them.
func (p *Player) encode(samples []float64) []byte {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, int32(len(samples)))
	for i := 0; i < len(samples); i++ {
		v := float32(samples[i])
		binary.Write(buf, binary.LittleEndian, v)
		binary.Write(buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

// StopMusic stops everything that is playing.
func (p *Player) StopMusic() {
	if p == nil {
		return
	}
	p.mu.Lock()
	players := p.playing
	p.playing = nil
	p.mu.Unlock()
	for _, pl := range players {
		pl.Pause()
		pl.Close()
	}
}

// WriteSilence is the reader of a sound that plays nothing.
//
// It is the sound of a game that has no sound device: the device is never
// asked for anything, and the game runs on.
type WriteSilence struct{}

// Read fills the buffer with nothing.
func (WriteSilence) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

var _ io.Reader = WriteSilence{}
