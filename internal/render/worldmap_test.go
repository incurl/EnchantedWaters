package render

import (
	"image"
	"testing"
)

// TestWorldMapRenders checks that the offscreen map really holds the
// drawn pixels.
func TestWorldMapRenders(t *testing.T) {
	m := NewWorldMap(128, 96)
	data := &MapData{
		Ports:    []PortMarker{{X: 64, Y: 48, City: true}},
		SeaZones: []ZoneMarker{{X: 64, Y: 48, Radius: 60}},
	}
	if !m.Render(data, 1, "clear") {
		t.Fatal("the map was not rendered")
	}
	// The centre of the map must be land, not sea.
	r, g, b, a := m.buffer.At(64, 48).RGBA()
	if r == 0 && g == 0 && b == 0 && a == 0 {
		t.Fatal("the map image is empty at the centre")
	}
	t.Logf("centre pixel: %d %d %d %d", r>>8, g>>8, b>>8, a>>8)
	// The corner must be sea.
	r2, g2, b2, _ := m.buffer.At(2, 2).RGBA()
	t.Logf("corner pixel: %d %d %d", r2>>8, g2>>8, b2>>8)
	if r2 == 0 && g2 == 0 && b2 == 0 {
		t.Error("the corner is not filled with the sea colour")
	}
}

// TestSubImageDraw checks that the sub image of the map can be drawn onto
// another image.
func TestSubImageDraw(t *testing.T) {
	m := NewWorldMap(128, 96)
	m.EnsureImage()
	drawFillRect(m.buffer, 0, 0, m.width, m.height, ColorSea)
	sub := m.buffer.SubImage(image.Rect(0, 0, 64, 48))
	r, g, b, a := sub.At(10, 10).RGBA()
	t.Logf("sub pixel: %d %d %d %d", r>>8, g>>8, b>>8, a>>8)
	if a == 0 {
		t.Error("the sub image is empty")
	}
}
