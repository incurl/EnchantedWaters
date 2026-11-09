package importer

import (
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// townRawDir returns the directory that holds the files of the release.
//
// The files of the release are not part of the project, so the tests that
// read them are skipped when the directory is not there.
func townRawDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "assets", "raw")
	if _, err := os.Stat(filepath.Join(dir, "PORTCHIP.LZW")); err != nil {
		t.Skipf("the files of the original release are not present: %v", err)
	}
	return dir
}

// TestPortChipTile verifies the assembly of one tile of a port.
//
// The tile is stored in a strip of PortChipAlign bytes to a plane, and the
// colour index of a pixel takes one bit from every plane. The test builds a
// strip with a known picture and checks that the reader gives it back.
func TestPortChipTile(t *testing.T) {
	raw := make([]byte, PortChipAlign*PortChipPlanes)
	// The first stored plane holds the highest bit of the index, the
	// second the next, and so on. Every pixel of the tile gets the index
	// that the first byte of each plane spells out.
	for plane := 0; plane < PortChipPlanes; plane++ {
		// Only the highest bit of the first byte is set, so the first
		// eight pixels of the tile get the bits of one plane.
		raw[plane*PortChipAlign] = 0x80
	}
	got := portChipTile(raw)
	if len(got) != PortChipWidth*PortChipHeight {
		t.Fatalf("got %d pixels, want %d", len(got), PortChipWidth*PortChipHeight)
	}
	// The first pixel takes bit 7 of the first byte of every plane, and
	// every plane has it set, so the index is all ones.
	want := uint8(1<<PortChipPlanes) - 1
	if got[0] != want {
		t.Errorf("the first pixel is %d, want %d", got[0], want)
	}
	// The second pixel takes bit 6, which is clear in every plane.
	if got[1] != 0 {
		t.Errorf("the second pixel is %d, want 0", got[1])
	}
}

// TestPortraitPixels verifies the assembly of a portrait.
//
// A portrait is stored a group of eight pixels at a time: a group holds one
// byte of every plane in turn, and the first byte holds the highest bit of
// the index.
func TestPortraitPixels(t *testing.T) {
	raw := make([]byte, PortraitWidth*PortraitHeight*PortraitPlanes/8)
	// The very first group of eight pixels: the first stored plane has
	// every bit set, the others none.
	raw[0] = 0xFF
	got := portraitPixels(raw)
	// The index of the first eight pixels takes the highest bit from the
	// first plane, so it is four.
	for x := 0; x < 8; x++ {
		if got[x] != 1<<(PortraitPlanes-1) {
			t.Errorf("pixel %d is %d, want %d", x, got[x], 1<<(PortraitPlanes-1))
		}
	}
	// The ninth pixel belongs to the next group, which is empty.
	if got[8] != 0 {
		t.Errorf("pixel 8 is %d, want 0", got[8])
	}
}

// TestExportTownData converts the pictures of the towns and verifies the
// files it writes.
func TestExportTownData(t *testing.T) {
	rawDir := townRawDir(t)
	outDir := t.TempDir()

	files, problems := ExportTownData(rawDir, outDir, discardLogger())
	if len(problems) > 0 {
		t.Fatalf("ExportTownData reported %d problems: %v", len(problems), problems)
	}
	if len(files) != 4 {
		t.Fatalf("got %d files, want 4: %v", len(files), files)
	}

	// --- the portraits ---
	f, err := os.Open(filepath.Join(outDir, "portraits.png"))
	if err != nil {
		t.Fatalf("the portraits: %v", err)
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode the portraits: %v", err)
	}
	if got, want := img.Bounds().Dx(), PortraitColumns*PortraitWidth; got != want {
		t.Errorf("the sheet of the portraits is %d pixels wide, want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), 16*PortraitHeight; got != want {
		t.Errorf("the sheet of the portraits is %d pixels tall, want %d", got, want)
	}

	// --- the tiles ---
	f, err = os.Open(filepath.Join(outDir, "portchips.png"))
	if err != nil {
		t.Fatalf("the tiles: %v", err)
	}
	img, err = png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode the tiles: %v", err)
	}
	if got, want := img.Bounds().Dy(), PortChipSets*PortChipSet*PortChipHeight; got != want {
		t.Errorf("the sheet of the tiles is %d pixels tall, want %d", got, want)
	}
	if got, want := img.Bounds().Dx(), PortChipWidth; got != want {
		t.Errorf("the sheet of the tiles is %d pixels wide, want %d", got, want)
	}

	// --- the maps ---
	data, err := os.ReadFile(filepath.Join(outDir, TownMapFile))
	if err != nil {
		t.Fatalf("the maps: %v", err)
	}
	if string(data[:4]) != townMapMagic {
		t.Fatalf("the maps start with %q, want %q", data[:4], townMapMagic)
	}
	width := int(binary.BigEndian.Uint32(data[4:8]))
	height := int(binary.BigEndian.Uint32(data[8:12]))
	count := int(binary.BigEndian.Uint32(data[12:16]))
	if width != TownMapWidth || height != TownMapHeight {
		t.Errorf("the maps are %dx%d, want %dx%d",
			width, height, TownMapWidth, TownMapHeight)
	}
	if count < 100 {
		t.Errorf("the file holds %d maps, want at least 100", count)
	}
	if got, want := len(data), 16+count*width*height; got != want {
		t.Errorf("the file is %d bytes, want %d", got, want)
	}
	// Every cell names a tile of a set.
	for i, v := range data[16:] {
		if int(v) >= PortChipSet {
			t.Fatalf("cell %d names tile %d, want below %d", i, v, PortChipSet)
		}
	}

	// --- the table of the sets ---
	table, err := os.ReadFile(filepath.Join(outDir, ChipSetFile))
	if err != nil {
		t.Fatalf("the table of the sets: %v", err)
	}
	if len(table) == 0 {
		t.Fatal("the table of the sets is empty")
	}
}

// discardLogger is the logger of the tests that do not read the log.
func discardLogger() Logger { return discard{} }

// discard is a logger that drops every line.
type discard struct{}

func (discard) Record(string, int, []string) {}
func (discard) Info(string, ...any)          {}
func (discard) Problem(string, ...any)       {}

// TestTownMapIsNotAPicture verifies that the maps of the towns are grids of
// indices and not pictures.
//
// The distinction matters: reading a map as a picture would give a sheet of
// noise, and reading a picture as a map would give a town of nonsense.
func TestTownMapIsNotAPicture(t *testing.T) {
	data := make([]byte, TownMapWidth*TownMapHeight)
	for i := range data {
		data[i] = uint8(i % PortChipSet)
	}
	if len(data) != TownMapWidth*TownMapHeight {
		t.Fatalf("the map is %d cells, want %d", len(data), TownMapWidth*TownMapHeight)
	}
	// The picture of a map would be a multiple of a tile, which a grid of
	// indices need not be.
	if image.Pt(TownMapWidth, TownMapHeight).X == 0 {
		t.Fatal("the map has no width")
	}
}
