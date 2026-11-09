package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"

	"github.com/peter/enchantedwaters/internal/asset"
	"github.com/peter/enchantedwaters/internal/render"
	"github.com/peter/enchantedwaters/internal/world"
)

// testUI builds an interface over the shipped configuration.
//
// The pictures of the original game are read when the directory of the
// release is present; the tests that need them check for it themselves.
func testUI(t *testing.T) *UI {
	t.Helper()
	dir := filepath.Join("..", "..", "assets", "config")
	reg, err := asset.Load(dir, logr.Discard())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	w := world.NewWorld(reg, 0xC0FFEE)
	portID := "ports.lisbon"
	if len(reg.PortSourceOrder) > 0 {
		portID = reg.PortSourceOrder[0]
	}
	if err := w.StartSession(portID); err != nil {
		t.Fatalf("start session: %v", err)
	}
	return New(w)
}

// loadTestArt reads the pictures of the original game into an interface.
//
// The pictures are derived data and they are part of the project.
func loadTestArt(t *testing.T, u *UI) {
	t.Helper()
	dir := filepath.Join("..", "..", "assets", "sprites")
	if _, err := os.Stat(filepath.Join(dir, "townmaps.bin")); err != nil {
		t.Skip("the pictures of the original game are not present")
	}
	art, _ := asset.LoadArt(dir)
	maps, err := asset.LoadPortMaps(dir)
	if err != nil {
		t.Fatalf("load the maps of the ports: %v", err)
	}
	chipSets, err := asset.LoadChipNumbers(dir)
	if err != nil {
		t.Fatalf("load the table of the tiles: %v", err)
	}
	u.SetArt(art, maps, chipSets)
}

// TestBuildTown verifies that the town of a port is assembled out of the
// tiles of its own set.
//
// The town is a picture of the whole map of the port, sixteen pixels to a
// tile, and every pixel of it must name a colour of the palette of the
// game.
func TestBuildTown(t *testing.T) {
	u := testUI(t)
	loadTestArt(t, u)

	portID := u.cfg.PortSourceOrder[0]
	town := u.BuildTown(portID)
	if town == nil {
		t.Fatal("the town of the first port was not built")
	}

	// The map of a town is ninety six tiles square.
	if town.Width != 96*TownTile || town.Height != 96*TownTile {
		t.Fatalf("the town is %dx%d, want %dx%d",
			town.Width, town.Height, 96*TownTile, 96*TownTile)
	}
	if len(town.Pixels) != town.Width*town.Height {
		t.Fatalf("the town holds %d pixels, want %d",
			len(town.Pixels), town.Width*town.Height)
	}
	// Every pixel names a colour of the palette.
	for i, v := range town.Pixels {
		if int(v) >= len(town.Palette) {
			t.Fatalf("pixel %d names colour %d, want below %d",
				i, v, len(town.Palette))
		}
	}
}

// TestBuildTownWithoutArt verifies that the interface draws its own
// pictures when the release is not present.
func TestBuildTownWithoutArt(t *testing.T) {
	u := testUI(t)
	if town := u.BuildTown(u.cfg.PortSourceOrder[0]); town != nil {
		t.Error("a town was built without the pictures of the original game")
	}
	if u.Portrait(0) != nil {
		t.Error("a portrait was handed out without the pictures of the original game")
	}
	// The drawing helpers must not panic without the pictures.
	canvas := render.NewCanvas(LogicalWidth, LogicalHeight)
	if u.DrawPortrait(canvas, 0, 0, 0) {
		t.Error("DrawPortrait reported a picture that is not there")
	}
	p := NewPortPopup(u, u.cfg.PortSourceOrder[0])
	p.Draw(canvas)
}

// TestPortPopupsDrawTheTown verifies that the port screens draw the town of
// the original game without panicking.
func TestPortPopupsDrawTheTown(t *testing.T) {
	u := testUI(t)
	loadTestArt(t, u)

	canvas := render.NewCanvas(LogicalWidth, LogicalHeight)
	portID := u.cfg.PortSourceOrder[0]

	// The port menu draws the town behind its list.
	NewPortPopup(u, portID).Draw(canvas)

	// The town view opens on the town and scrolls without leaving it.
	town := NewTownPopup(u, portID)
	if town == nil {
		t.Fatal("the town view was not created")
	}
	if town.scrollY < 0 || town.scrollY > 96*TownTile-LogicalHeight {
		t.Errorf("the town view opens at %d, which is outside the town", town.scrollY)
	}
	town.Draw(canvas)
	// A step to the left at the edge keeps the window inside the map.
	town.scrollX = -1000
	town.clamp()
	if town.scrollX != 0 {
		t.Errorf("the window is at %d after clamping, want 0", town.scrollX)
	}
	town.scrollX, town.scrollY = 1e6, 1e6
	town.clamp()
	if town.scrollX > 96*TownTile-LogicalWidth {
		t.Errorf("the window is at %d, which is past the right edge", town.scrollX)
	}
	if town.scrollY > 96*TownTile-LogicalHeight {
		t.Errorf("the window is at %d, which is past the bottom edge", town.scrollY)
	}
}

// TestPortSceneUsesTheRightSet verifies that a town is drawn out of the set
// of tiles of its own cultural group.
//
// The table of the towns of the release gives the set of every town.
func TestPortSceneUsesTheRightSet(t *testing.T) {
	u := testUI(t)
	loadTestArt(t, u)

	for i, portID := range u.cfg.PortSourceOrder {
		if i >= len(u.portMaps) {
			break
		}
		town, set := u.PortScene(portID)
		if town == nil || set == nil {
			t.Fatalf("%s has no town", portID)
		}
		want := u.ChipSet(portID)
		got := -1
		for j, candidate := range u.art.PortChips {
			if candidate == set {
				got = j
				break
			}
		}
		if got != want {
			t.Fatalf("%s is drawn out of set %d, want set %d", portID, got, want)
		}
	}
}

// TestTownTileSize verifies that the tiles of a town are the size of the
// tiles of the original game.
//
// The maps of the towns are grids of tiles, and the size of a tile is a
// property of the art. A wrong size shifts the whole town.
func TestTownTileSize(t *testing.T) {
	u := testUI(t)
	loadTestArt(t, u)

	_, set := u.PortScene(u.cfg.PortSourceOrder[0])
	if set == nil {
		t.Fatal("the first port has no set of tiles")
	}
	if set.TileWidth != TownTile || set.TileHeight != TownTile {
		t.Errorf("the tiles are %dx%d, want %dx%d",
			set.TileWidth, set.TileHeight, TownTile, TownTile)
	}
}
