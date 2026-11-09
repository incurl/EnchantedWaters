package ui

import (
	"github.com/peter/enchantedwaters/internal/asset"
	"github.com/peter/enchantedwaters/pkg/importer"
)

// This file connects the sound of the original game to the game.
//
// The original game played a piece of music in every place of the world and
// a sound effect for every thing the player does. The pieces of music of the
// release are twenty three, and the effects are forty eight, and the game
// chooses them by what happens: the music of the sea is the music of the
// town is the music of the battle.

// The music of the places of the world.
//
// The numbers are the numbers of the songs of the release.
const (
	// musicSea is the music of a ship that is under way.
	musicSea = 0
	// musicPort is the music of a ship that is in a port.
	musicPort = 1
	// musicTown is the music of a town.
	musicTown = 2
)

// The effects of the game.
//
// The numbers are the numbers of the effects of the release.
const (
	// effectClick is the sound of a key.
	effectClick = 0
	// effectAccept is the sound of a choice that is taken.
	effectAccept = 1
	// effectCancel is the sound of a choice that is given up.
	effectCancel = 2
	// effectCoin is the sound of money.
	effectCoin = 3
	// effectSail is the sound of a ship that leaves a port.
	effectSail = 4
	// effectAlert is the sound of bad news.
	effectAlert = 5
)

// playMusic plays a piece of the music of the original game.
//
// The function does nothing when the sound was not read, so that the game
// stays silent instead of failing.
func (u *UI) playMusic(index int) {
	if u.audio == nil || u.sound == nil {
		return
	}
	song := u.sound.Song(index)
	if song == nil {
		return
	}
	u.audio.StopMusic()
	u.audio.PlayMusic(*song)
}

// playEffect plays a sound effect of the original game.
func (u *UI) playEffect(index int) {
	if u.audio == nil || u.sound == nil {
		return
	}
	effect := u.sound.Effect(index)
	if effect == nil {
		return
	}
	u.audio.PlayEffect(*effect)
}

// updateMusic chooses the music that fits what the player is doing.
//
// The music changes when the ship arrives somewhere, and not on every
// frame: the game remembers the piece it is playing.
func (u *UI) updateMusic() {
	if u.audio == nil || u.sound == nil {
		return
	}
	want := musicSea
	if portID := u.world.ArrivedAtPort(u.world.PlayerEntity()); portID != "" {
		want = musicPort
	}
	if u.musicIndex == want {
		return
	}
	u.musicIndex = want
	u.playMusic(want)
}

// musicIndex is the piece of music that is playing.
//
// The value of minus one means that nothing is playing yet.
type musicState int

// startMusic plays the music of the place the player is in.
//
// The game calls it at the start of the session, and the music changes
// later on its own when the ship arrives somewhere.
func (u *UI) startMusic() {
	if u.musicStarted {
		return
	}
	u.musicStarted = true
	u.musicIndex = -1
	u.updateMusic()
}

// stopMusic stops the music.
func (u *UI) stopMusic() {
	if u.audio != nil {
		u.audio.StopMusic()
	}
	u.musicIndex = -1
}

// effectFor returns the effect of an action of the interface.
//
// The actions are the ones the player does most often, and the effects are
// the ones of the release that fit them best.
func effectFor(action string) int {
	switch action {
	case "click":
		return effectClick
	case "accept":
		return effectAccept
	case "cancel":
		return effectCancel
	case "coin":
		return effectCoin
	case "sail":
		return effectSail
	case "alert":
		return effectAlert
	}
	return -1
}

// soundOf returns the sound of the original game.
//
// The function is a helper for the tests and for the parts of the code
// that want the sound without the interface.
func soundOf(dir string) *asset.Sound {
	sound, _ := asset.LoadSound(dir)
	return sound
}

// songOf returns a song of the original game.
func songOf(dir string, index int) *importer.MMLSong {
	return soundOf(dir).Song(index)
}

// volumePercent returns the volume of the sound as a percentage.
func (u *UI) volumePercent() int {
	if u.audio == nil {
		return 0
	}
	return int(u.audio.Volume()*100 + 0.5)
}

// toggleSound turns the sound on and off.
func (u *UI) toggleSound() {
	if u.audio == nil {
		return
	}
	if u.audio.Enabled() {
		u.audio.SetEnabled(false)
		u.audio.StopMusic()
		return
	}
	u.audio.SetEnabled(true)
	u.updateMusic()
}

// raiseVolume turns the volume up, and turns the sound off at the top.
func (u *UI) raiseVolume() {
	if u.audio == nil {
		return
	}
	v := u.audio.Volume() + 0.1
	if v > 1.001 {
		v = 0
	}
	u.audio.SetVolume(v)
}
