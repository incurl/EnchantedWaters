package importer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// lsEGAPalette is the palette of the game: eight colours, in the order of
// the indices that the planar graphics use.
var lsEGAPalette = color.Palette{
	color.RGBA{0x00, 0x00, 0x00, 0xff},
	color.RGBA{0x00, 0xA0, 0x60, 0xff},
	color.RGBA{0xD0, 0x40, 0x00, 0xff},
	color.RGBA{0xF0, 0xA0, 0x60, 0xff},
	color.RGBA{0x00, 0x40, 0xD0, 0xff},
	color.RGBA{0x00, 0xA0, 0xF0, 0xff},
	color.RGBA{0xD0, 0x60, 0xA0, 0xff},
	color.RGBA{0xF0, 0xE0, 0xD0, 0xff},
}

// lsDecodePlanar turns the planar bytes of an image into an indexed image.
//
// The layout is group interleaved: the pixels come in groups of eight, and
// each group is stored as three bytes, one per plane, most significant bit
// of a byte being the leftmost pixel of the group.
//
// The order of the planes is the one of the game: the first byte of a group
// holds the highest bit of the colour index.
func lsDecodePlanar(raw []byte, width, height, planes int) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, width, height), lsEGAPalette)
	stride := width / 8
	groups := width * height / 8
	// The colour of a pixel is assembled from one bit of each plane, so
	// the bits have to be collected before the pixel is written.
	idx := make([]uint8, width*height)
	for g := 0; g < groups; g++ {
		gx := g % stride
		gy := g / stride
		for plane := 0; plane < planes; plane++ {
			b := raw[g*planes+plane]
			for bit := 0; bit < 8; bit++ {
				x := gx*8 + bit
				idx[gy*width+x] |= ((b >> uint(7-bit)) & 1) << uint(planes-1-plane)
			}
		}
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetColorIndex(x, y, idx[y*width+x])
		}
	}
	return img
}

// lsReferenceDir returns the directory that holds the reference images of
// the portraits, if they are present.
//
// They come from a third party implementation of the game and are used only
// as a test oracle; they are not part of this repository.
func lsReferenceDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("ENCHANTED_REFERENCE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "..", "Downloads", "Uncharted2", "reference")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skip("the reference images are not present")
	}
	return dir
}

// TestLSPortraitPixelsMatchTheReference checks the decoded portraits against
// the reference images of a third party implementation.
//
// The check is at the level of the pixels, so it covers the codec, the
// substitution table, the layout of the planar data and the palette at
// once. The portraits of KAO.LZW are 64 by 80 pixels in three planes.
func TestLSPortraitPixelsMatchTheReference(t *testing.T) {
	refDir := lsReferenceDir(t)
	rawDir := lsRawDir(t)
	data, err := os.ReadFile(filepath.Join(rawDir, "KAO.LZW"))
	if err != nil {
		t.Skipf("KAO.LZW is not present: %v", err)
	}
	f, err := ParseLS(data)
	if err != nil {
		t.Fatalf("parsing KAO.LZW: %v", err)
	}
	parts, err := DecodeLS(f, data)
	if err != nil {
		t.Fatalf("decoding KAO.LZW: %v", err)
	}

	checked := 0
	for i, p := range parts {
		if f.Blocks[i].UncompressedSize != 1920 {
			// Not a portrait.
			continue
		}
		name := filepath.Join(refDir, "portraits", fmt.Sprintf("%04d.png", i+1))
		refData, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		ref, err := png.Decode(bytes.NewReader(refData))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if b := ref.Bounds(); b.Dx() != 64 || b.Dy() != 80 {
			continue
		}
		// The comparison is on the palette *index* of every pixel, so
		// that it does not depend on the exact shades of the reference
		// image, which come from a different palette file.
		got := lsDecodePlanar(p, 64, 80, 3)
		diff := 0
		for y := 0; y < 80; y++ {
			for x := 0; x < 64; x++ {
				if lsNearestIndex(ref.At(x, y)) != got.ColorIndexAt(x, y) {
					diff++
				}
			}
		}
		if diff != 0 {
			t.Errorf("portrait %d: %d of %d pixels differ", i+1, diff, 64*80)
		}
		checked++
	}
	if checked == 0 {
		t.Skip("no reference portraits were found")
	}
	t.Logf("compared %d portraits", checked)
}

// lsNearestIndex returns the index of the palette colour that is closest to
// a colour of the reference image.
//
// The reference images were made with a slightly different palette file, so
// the colours have to be matched rather than compared.
func lsNearestIndex(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	best := 0
	bestDist := int64(1) << 30
	for i, p := range lsEGAPalette {
		pr, pg, pb, _ := p.RGBA()
		dr := int64(r>>8) - int64(pr>>8)
		dg := int64(g>>8) - int64(pg>>8)
		db := int64(b>>8) - int64(pb>>8)
		d := dr*dr + dg*dg + db*db
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return uint8(best)
}
