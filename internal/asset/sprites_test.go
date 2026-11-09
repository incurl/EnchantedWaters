package asset

import (
	"path/filepath"
	"testing"

	"github.com/peter/enchantedwaters/internal/render"
)

// artDir returns the path of the directory that holds the pictures of the
// original game.
//
// The pictures are derived data and they are part of the project, so the
// tests that read them always run.
func artDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "assets", "sprites")
}

// TestHorizonPalette verifies that the palette of the game is the one that
// the release ships.
//
// The table is the one of HORIZON_KAO.PAL: eight colours, three bytes to a
// colour. A wrong entry shows up as a wrong colour everywhere in the game,
// so the table is worth a test of its own.
func TestHorizonPalette(t *testing.T) {
	want := [8][3]uint8{
		{0x00, 0x00, 0x00},
		{0x00, 0xA0, 0x60},
		{0xD0, 0x40, 0x00},
		{0xF0, 0xA0, 0x60},
		{0x00, 0x40, 0xD0},
		{0x00, 0xA0, 0xF0},
		{0xD0, 0x60, 0xA0},
		{0xF0, 0xE0, 0xD0},
	}
	for i := range want {
		if HorizonPalette[i] != want[i] {
			t.Errorf("colour %d is %v, want %v", i, HorizonPalette[i], want[i])
		}
	}
}

// TestLoadArt reads the pictures of the original game and verifies their
// geometry.
func TestLoadArt(t *testing.T) {
	art, problems := LoadArt(artDir(t))
	if art == nil {
		t.Fatal("LoadArt returned no art")
	}
	if len(problems) > 0 {
		t.Fatalf("LoadArt reported %d problems: %v", len(problems), problems)
	}

	// --- the portraits ---
	// The release holds one hundred and twenty eight portraits of sixty
	// four by eighty pixels in three planes.
	if len(art.Portraits) != 128 {
		t.Errorf("got %d portraits, want 128", len(art.Portraits))
	}
	for i, p := range art.Portraits {
		if p.Width != 64 || p.Height != 80 {
			t.Fatalf("portrait %d is %dx%d, want 64x80", i, p.Width, p.Height)
		}
		if len(p.Pixels) != 64*80 {
			t.Fatalf("portrait %d holds %d pixels, want %d", i, len(p.Pixels), 64*80)
		}
		// Four planes of three bits can only give eight colours, so
		// no pixel of a portrait may name a colour outside the
		// palette of the game.
		for _, v := range p.Pixels {
			if int(v) >= len(HorizonPalette) {
				t.Fatalf("portrait %d has colour index %d, want below %d",
					i, v, len(HorizonPalette))
			}
		}
	}

	// --- the tiles of the ports ---
	if len(art.PortChips) != 7 {
		t.Errorf("got %d sets of tiles, want 7", len(art.PortChips))
	}
	for i, set := range art.PortChips {
		if set.TileCount() != 240 {
			t.Errorf("set %d holds %d tiles, want 240", i, set.TileCount())
		}
		if set.TileWidth != 16 || set.TileHeight != 16 {
			t.Errorf("set %d has tiles of %dx%d, want 16x16",
				i, set.TileWidth, set.TileHeight)
		}
		for j, tile := range set.Tiles {
			if tile == nil {
				t.Fatalf("set %d tile %d is missing", i, j)
			}
			if len(tile.Pixels) != 16*16 {
				t.Fatalf("set %d tile %d holds %d pixels, want 256",
					i, j, len(tile.Pixels))
			}
		}
	}
}

// TestPortChipSet verifies that the sets of tiles are handed out in a ring.
//
// The caller passes the number of the set, and a number outside the range
// must wrap instead of panicking: the table of the towns of the original
// game is a copy of the release, and a wrong entry must not stop the game.
func TestPortChipSet(t *testing.T) {
	art := &Art{PortChips: make([]*SpriteSet, 7)}
	for i := range art.PortChips {
		art.PortChips[i] = &SpriteSet{Name: "set"}
	}
	for _, index := range []int{0, 6, 7, 13, -1, -8} {
		if got := art.PortChipSet(index); got == nil {
			t.Errorf("PortChipSet(%d) returned nil", index)
		}
	}
	if art.Empty() {
		t.Error("art with seven sets reported itself as empty")
	}
	if (&Art{}).Empty() != true {
		t.Error("art without pictures did not report itself as empty")
	}
}

// TestSpriteColorAt verifies the colour lookup of a picture.
func TestSpriteColorAt(t *testing.T) {
	s := &render.Sprite{
		Width:   2,
		Height:  1,
		Pixels:  []uint8{1, 7},
		Palette: HorizonPalette[:],
	}
	if got := s.At(0, 0); got != 1 {
		t.Errorf("At(0,0) is %d, want 1", got)
	}
	if got := s.At(1, 0); got != 7 {
		t.Errorf("At(1,0) is %d, want 7", got)
	}
	// The pixels outside the picture are transparent.
	if got := s.At(2, 0); got != 0 {
		t.Errorf("At(2,0) is %d, want 0", got)
	}
	if got := s.At(-1, 0); got != 0 {
		t.Errorf("At(-1,0) is %d, want 0", got)
	}

	c := s.ColorAt(1, 0)
	r, g, b, _ := c.RGBA()
	want := HorizonPalette[7]
	if uint8(r>>8) != want[0] || uint8(g>>8) != want[1] || uint8(b>>8) != want[2] {
		t.Errorf("ColorAt(1,0) is %d,%d,%d, want %d,%d,%d",
			r>>8, g>>8, b>>8, want[0], want[1], want[2])
	}
}

// TestChipPaletteFoldsTheIntensityBit verifies that the sixteen colours of
// the tiles of the ports are the eight colours of the game folded twice.
//
// The tiles name sixteen colours because they are stored in four planes,
// but the palette file of the release holds only eight. The bright half of
// the table must therefore name the same colours as the dark half, so that
// the towns are drawn in the colours that the release actually ships.
func TestChipPaletteFoldsTheIntensityBit(t *testing.T) {
	if len(ChipPalette) != 16 {
		t.Fatalf("the palette holds %d colours, want 16", len(ChipPalette))
	}
	for i := 0; i < 8; i++ {
		if ChipPalette[i] != HorizonPalette[i] {
			t.Errorf("colour %d is %v, want %v", i, ChipPalette[i], HorizonPalette[i])
		}
		if ChipPalette[i+8] != HorizonPalette[i] {
			t.Errorf("colour %d is %v, want the colour of %d, %v",
				i+8, ChipPalette[i+8], i, HorizonPalette[i])
		}
	}
}
