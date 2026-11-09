package importer

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

// TestPlanarDecodeMatchesTheReference verifies the planar decoder against
// the geometry that the analysis of the first Uncharted Waters documents:
// a sprite of 24x12 pixels stored as three interleaved planes in the order
// 1, 4, 2.
//
// The test builds such a sprite by hand: every pixel of the row is given a
// known colour index, the bytes are written in the documented layout, and
// the decoder must recover the very same pixels.
func TestPlanarDecodeMatchesTheReference(t *testing.T) {
	const (
		width  = 24
		height = 12
		planes = 3
	)
	planeBytes := width / 8 // three bytes per row

	// The colour index of every pixel: a diagonal ramp, so that every bit
	// of the three bit index is exercised.
	want := make([]uint8, width*height)
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			want[row*width+col] = uint8((row + col) % 8)
		}
	}

	// The bit that each stored plane contributes: the documented order
	// 1, 4, 2 means stored plane 0 is bit 0, plane 1 is bit 2, plane 2 is
	// bit 1.
	planeOrder := []int{0, 2, 1}

	// Write the bytes in the interleaved layout.
	buf := make([]byte, planeBytes*height*planes)
	for row := 0; row < height; row++ {
		for group := 0; group < planeBytes; group++ {
			for plane := 0; plane < planes; plane++ {
				var b byte
				for bit := 0; bit < 8; bit++ {
					col := group*8 + bit
					value := want[row*width+col]
					if value>>uint(planeOrder[plane])&1 == 1 {
						b |= 0x80 >> uint(bit)
					}
				}
				buf[(row*planeBytes+group)*planes+plane] = b
			}
		}
	}

	// Write the bytes into a data file and decode them through the reader,
	// so that the whole path is covered.
	dir := t.TempDir()
	path := filepath.Join(dir, "SPRITE.CIM")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	r := NewReader(f, 0, int64(len(buf)))
	spec := GraphicSpec{
		Width: width, Height: height, Planes: planes, Columns: 1, Count: 1,
		Name: "port_marker", PlaneOrder: planeOrder, Interleaved: true, EGAPalette: true,
	}
	sheet, err := ReadSpriteSheet(r, spec, nil)
	if err != nil {
		t.Fatalf("read the sprite sheet: %v", err)
	}

	// Every pixel must survive the round trip.
	wrong := 0
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			got := sheet.Image.ColorIndexAt(col, row)
			if got != want[row*width+col] {
				if wrong < 5 {
					t.Errorf("pixel (%d,%d) is %d, want %d", col, row, got, want[row*width+col])
				}
				wrong++
			}
		}
	}
	if wrong > 0 {
		t.Errorf("%d of %d pixels are wrong", wrong, width*height)
	}

	// The palette must be the EGA one.
	pal := sheet.Image.Palette
	if len(pal) != len(EGAPalette) {
		t.Fatalf("the palette holds %d colours, want %d", len(pal), len(EGAPalette))
	}
	r0, g0, b0, _ := pal[1].RGBA()
	if uint8(r0>>8) != 0x55 || uint8(g0>>8) != 0x55 || uint8(b0>>8) != 0xff {
		t.Errorf("the colour 1 is (%d,%d,%d), want (85,85,255)", r0>>8, g0>>8, b0>>8)
	}
}

// TestPlanarLayout verifies the plane by plane layout, which is the other
// layout the importer supports.
func TestPlanarLayout(t *testing.T) {
	const (
		width  = 8
		height = 2
		planes = 3
	)
	// Two rows: the first is colour 5, the second is colour 2.
	want := []uint8{5, 5, 5, 5, 5, 5, 5, 5, 2, 2, 2, 2, 2, 2, 2, 2}
	planeBytes := 1
	buf := make([]byte, planeBytes*height*planes)
	for row := 0; row < height; row++ {
		for plane := 0; plane < planes; plane++ {
			var b byte
			if want[row*width]>>uint(plane)&1 == 1 {
				b = 0xff
			}
			buf[(plane*height+row)*planeBytes] = b
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "T.BIN")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	r := NewReader(f, 0, int64(len(buf)))
	sheet, err := ReadSpriteSheet(r, GraphicSpec{
		Width: width, Height: height, Planes: planes, Columns: 1, Count: 1,
		Name: "t", EGAPalette: true,
	}, nil)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if got, w := sheet.Image.ColorIndexAt(col, row), want[row*width+col]; got != w {
				t.Errorf("(%d,%d) is %d, want %d", col, row, got, w)
			}
		}
	}
	_ = image.Paletted{}
}
