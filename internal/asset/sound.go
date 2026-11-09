package asset

import (
	"os"
	"path/filepath"

	"github.com/peter/enchantedwaters/pkg/importer"
)

// This file loads the sound of the original game.
//
// The sound reaches the game as derived data: the import step reads the
// files of the release and writes out the notes of the music and of the
// effects, and the game reads that.
//
// Everything is optional. A missing file leaves the game silent.

// The names of the derived files of the sound.
const (
	// SoundMusicFile holds the music of the release.
	SoundMusicFile = "music.csv"
	// SoundEffectFile holds the sound effects of the release.
	SoundEffectFile = "effects.csv"
)

// Sound holds the sound of the original game.
type Sound struct {
	// Music holds the songs of the release, in the order of the file.
	Music []importer.MMLSong
	// Effects holds the sound effects of the release, in the order of the
	// file.
	Effects []importer.SoundEffect
}

// Empty reports whether the sound holds nothing at all.
func (s *Sound) Empty() bool {
	if s == nil {
		return true
	}
	return len(s.Music) == 0 && len(s.Effects) == 0
}

// Song returns the song of a number.
//
// The function returns nil when the number is outside the range of the
// music, so that the caller never has to check.
func (s *Sound) Song(index int) *importer.MMLSong {
	if s == nil || index < 0 || index >= len(s.Music) {
		return nil
	}
	return &s.Music[index]
}

// Effect returns the sound effect of a number.
func (s *Sound) Effect(index int) *importer.SoundEffect {
	if s == nil || index < 0 || index >= len(s.Effects) {
		return nil
	}
	return &s.Effects[index]
}

// LoadSound reads the sound of the original game out of a directory.
//
// The directory is the derived one, assets/sprites. The function returns
// the sound and the list of the problems it met.
func LoadSound(dir string) (*Sound, []string) {
	sound := &Sound{}
	var problems []string

	musicPath := filepath.Join(dir, SoundMusicFile)
	effectPath := filepath.Join(dir, SoundEffectFile)

	// The music and the effects live in two files, and both are
	// optional.
	if data, err := os.ReadFile(musicPath); err != nil {
		problems = append(problems, "the music: "+err.Error())
	} else {
		music, _, err := importer.ReadSoundCSV(data)
		if err != nil {
			problems = append(problems, "the music: "+err.Error())
		} else {
			sound.Music = music
		}
	}
	if data, err := os.ReadFile(effectPath); err != nil {
		problems = append(problems, "the effects: "+err.Error())
	} else {
		_, effects, err := importer.ReadSoundCSV(data)
		if err != nil {
			problems = append(problems, "the effects: "+err.Error())
		} else {
			sound.Effects = effects
		}
	}
	return sound, problems
}
