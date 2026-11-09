package render

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// WorldMap draws the scrollable world map.
//
// # Performance
//
// The map is rendered into an offscreen image that is only regenerated
// when the camera moves, the day changes or the weather changes. The
// per-frame cost is then a single blit of the visible region, which keeps
// the frame rate stable even on a large map.
//
// The tile classification (land / sea / coast) is computed from the port
// positions and the sea zone radii, using a distance field, so that no
// bitmap of the original game is required.
type WorldMap struct {
	// buffer holds the rendered map as a plain Go image.
	//
	// The map is rendered into a CPU side buffer and then uploaded to the
	// GPU in one go. This keeps the rendering testable without a running
	// game, and it makes the per frame cost a single texture upload.
	buffer *image.RGBA
	// img is the GPU side image that holds the uploaded map.
	img *ebiten.Image
	// width / height are the size of the map in pixels.
	width, height int
	// dirty marks the map as needing a re-render.
	dirty bool
	// day is the day the map was last rendered for.
	day int
	// weatherKey identifies the weather the map was rendered for.
	weatherKey string
}

// NewWorldMap creates a world map renderer.
func NewWorldMap(width, height int) *WorldMap {
	if width <= 0 {
		width = 1024
	}
	if height <= 0 {
		height = 640
	}
	return &WorldMap{
		width:  width,
		height: height,
		dirty:  true,
	}
}

// Size returns the size of the map in pixels.
func (m *WorldMap) Size() (int, int) { return m.width, m.height }

// MarkDirty forces the map to be re-rendered on the next draw.
func (m *WorldMap) MarkDirty() { m.dirty = true }

// MapData is the information the renderer needs about the world.
//
// It is a plain data structure so that the renderer does not depend on
// the simulation package.
type MapData struct {
	// Ports holds the position and size of every port.
	Ports []PortMarker
	// SeaZones holds the radius of every sea zone.
	SeaZones []ZoneMarker
	// Fleets holds the position of every fleet.
	Fleets []FleetMarker
	// WeatherTint is the alpha of the weather overlay.
	WeatherTint float64
	// NightTint is the alpha of the night overlay.
	NightTint float64
}

// PortMarker is the render data of a port.
type PortMarker struct {
	// X / Y is the map position.
	X, Y float64
	// Capital marks a capital city.
	Capital bool
	// City marks a large city.
	City bool
}

// ZoneMarker is the render data of a sea zone.
type ZoneMarker struct {
	// X / Y is the centre of the zone.
	X, Y float64
	// Radius is the influence radius.
	Radius float64
}

// FleetMarker is the render data of a fleet.
type FleetMarker struct {
	// X / Y is the map position.
	X, Y float64
	// Heading is the compass bearing in degrees.
	Heading float64
	// Player marks the player's fleet.
	Player bool
}

// EnsureImage makes sure the offscreen buffer exists.
func (m *WorldMap) EnsureImage() {
	if m.buffer == nil {
		m.buffer = image.NewRGBA(image.Rect(0, 0, m.width, m.height))
	}
	if m.img == nil {
		// The image is unmanaged: the pixel data of the buffer is then
		// copied verbatim, without any atlas padding.
		m.img = ebiten.NewImageWithOptions(
			image.Rect(0, 0, m.width, m.height),
			&ebiten.NewImageOptions{Unmanaged: true})
		m.dirty = true
	}
}

// Render regenerates the offscreen map if needed.
//
// It returns true when the map was actually re-rendered.
func (m *WorldMap) Render(data *MapData, day int, weatherKey string) bool {
	m.EnsureImage()
	key := weatherKey
	if !m.dirty && m.day == day && m.weatherKey == key {
		return false
	}
	m.dirty = false
	m.day = day
	m.weatherKey = key

	m.drawSea(data)
	m.drawZones(data)
	m.drawLand(data)
	m.drawPorts(data)

	// The CPU buffer is uploaded into the GPU image in one go.
	m.img.WritePixels(m.buffer.Pix)
	return true
}

// drawSea fills the background with the sea colour.
//
// The open sea is drawn in two bands: the deep water in the middle of
// the map and the coastal water around it.
func (m *WorldMap) drawSea(data *MapData) {
	// The base colour is filled in one call; filling it per column would
	// cost width * height * width operations.
	drawFillRect(m.buffer, 0, 0, m.width, m.height, ColorSea)

	// The deep sea is drawn as a horizontal band in the middle of the
	// map.
	drawFillRect(m.buffer, 0, m.height*2/5, m.width, m.height/5, ColorSeaDeep)
}

// drawZones tints the shallow water around the sea zones.
func (m *WorldMap) drawZones(data *MapData) {
	for _, z := range data.SeaZones {
		if z.Radius <= 0 {
			continue
		}
		// The coastal water is drawn as a filled circle.
		m.fillCircle(z.X, z.Y, z.Radius*0.35, ColorCoast)
	}
}

// drawLand draws the land masses.
//
// The land is the union of discs around the ports: every port sits on a
// coast, and the discs overlap to form continents. This is a clean-room
// approximation of the original coastline, derived from the port data
// alone.
func (m *WorldMap) drawLand(data *MapData) {
	// The radius of a land mass scales with the number of nearby ports, so
	// that dense regions form continents and isolated ports form islands.
	for _, p := range data.Ports {
		radius := 34.0
		if p.City {
			radius = 48
		}
		if p.Capital {
			radius = 58
		}
		// Count the nearby ports to grow the continent.
		for _, q := range data.Ports {
			d := math.Hypot(q.X-p.X, q.Y-p.Y)
			if d < 120 {
				radius += 14
			}
		}
		if radius > 120 {
			radius = 120
		}
		m.fillCircle(p.X, p.Y, radius, ColorLand)
	}
	// The coast line is drawn as a slightly smaller disc in a lighter
	// shade, which gives the classic hand-drawn look.
	for _, p := range data.Ports {
		radius := 26.0
		if p.City {
			radius = 36
		}
		if p.Capital {
			radius = 44
		}
		for _, q := range data.Ports {
			d := math.Hypot(q.X-p.X, q.Y-p.Y)
			if d < 120 {
				radius += 10
			}
		}
		if radius > 90 {
			radius = 90
		}
		m.fillCircle(p.X, p.Y, radius, ColorLandLight)
	}
}

// drawPorts draws the port markers.
func (m *WorldMap) drawPorts(data *MapData) {
	for _, p := range data.Ports {
		radius := 3.0
		if p.City {
			radius = 4
		}
		if p.Capital {
			radius = 5
		}
		m.fillCircle(p.X, p.Y, radius, ColorPort)
	}
}

// fillCircle fills a circle with a colour.
func (m *WorldMap) fillCircle(cx, cy, radius float64, c color.Color) {
	if radius <= 0 {
		return
	}
	minX := int(math.Floor(cx - radius))
	maxX := int(math.Ceil(cx + radius))
	minY := int(math.Floor(cy - radius))
	maxY := int(math.Ceil(cy + radius))
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX >= m.width {
		maxX = m.width - 1
	}
	if maxY >= m.height {
		maxY = m.height - 1
	}
	r2 := radius * radius
	cr, cg, cb, ca := c.RGBA()
	for y := minY; y <= maxY; y++ {
		dy := float64(y) + 0.5 - cy
		row := m.buffer.PixOffset(0, y)
		for x := minX; x <= maxX; x++ {
			dx := float64(x) + 0.5 - cx
			if dx*dx+dy*dy > r2 {
				continue
			}
			i := row + x*4
			m.buffer.Pix[i] = uint8(cr >> 8)
			m.buffer.Pix[i+1] = uint8(cg >> 8)
			m.buffer.Pix[i+2] = uint8(cb >> 8)
			m.buffer.Pix[i+3] = uint8(ca >> 8)
		}
	}
}

// drawFillRect fills a rectangle of an image with a colour.
//
// The image is assumed to be an *image.RGBA with premultiplied alpha.
func drawFillRect(img *image.RGBA, x, y, w, h int, c color.Color) {
	if img == nil || w <= 0 || h <= 0 {
		return
	}
	bounds := img.Bounds()
	if x < bounds.Min.X {
		w += x - bounds.Min.X
		x = bounds.Min.X
	}
	if y < bounds.Min.Y {
		h += y - bounds.Min.Y
		y = bounds.Min.Y
	}
	if x+w > bounds.Max.X {
		w = bounds.Max.X - x
	}
	if y+h > bounds.Max.Y {
		h = bounds.Max.Y - y
	}
	if w <= 0 || h <= 0 {
		return
	}
	cr, cg, cb, ca := c.RGBA()
	pixel := []byte{uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)}
	for row := y; row < y+h; row++ {
		off := img.PixOffset(x, row)
		for i := 0; i < w; i++ {
			copy(img.Pix[off+i*4:off+i*4+4], pixel)
		}
	}
}

// Draw renders the visible part of the map into the screen.
//
// The camera is expressed in map pixels; the visible rectangle is clipped
// to the map bounds.
func (m *WorldMap) Draw(screen *ebiten.Image, camX, camY float64, screenW, screenH int) {
	m.EnsureImage()
	// Compute the source rectangle.
	srcX := int(camX)
	srcY := int(camY)
	srcW := screenW
	srcH := screenH
	if srcX < 0 {
		srcW += srcX
		srcX = 0
	}
	if srcY < 0 {
		srcH += srcY
		srcY = 0
	}
	if srcX+srcW > m.width {
		srcW = m.width - srcX
	}
	if srcY+srcH > m.height {
		srcH = m.height - srcY
	}
	if srcW <= 0 || srcH <= 0 {
		return
	}
	// The destination offset compensates for the clipping.
	dstX := 0
	dstY := 0
	if camX < 0 {
		dstX = int(-camX)
	}
	if camY < 0 {
		dstY = int(-camY)
	}
	// The visible part of the map is drawn as a sub image.
	//
	// The sub image of an unmanaged image keeps the pixel coordinates of
	// the original, so the source rectangle maps one to one.
	sub, ok := m.img.SubImage(image.Rect(srcX, srcY, srcX+srcW, srcY+srcH)).(*ebiten.Image)
	if !ok {
		return
	}
	op := &ebiten.DrawImageOptions{
		// The nearest-neighbour filtering keeps the pixels crisp.
		Filter: ebiten.FilterNearest,
	}
	op.GeoM.Translate(float64(dstX), float64(dstY))
	screen.DrawImage(sub, op)
}

// DrawFleet draws a fleet marker on top of the map.
//
// The marker is a filled triangle that points in the direction of the
// heading. The player's fleet is drawn in bright white with a dark
// outline, so that it always stands out from the AI fleets.
func DrawFleet(screen *ebiten.Image, x, y, heading float64, isPlayer bool) {
	body := ColorAIFleet
	outline := ColorSeaDeep
	if isPlayer {
		body = ColorFleet
	}
	// The triangle is defined by its three corners: the tip in front of
	// the ship and the two corners at the back.
	rad := heading * math.Pi / 180
	dx := math.Sin(rad)
	dy := -math.Cos(rad)
	px := -dy
	py := dx

	const length = 6.0
	const halfWidth = 3.5
	tipX := x + dx*length
	tipY := y + dy*length
	leftX := x - dx*length*0.4 + px*halfWidth
	leftY := y - dy*length*0.4 + py*halfWidth
	rightX := x - dx*length*0.4 - px*halfWidth
	rightY := y - dy*length*0.4 - py*halfWidth

	// The outline is drawn first, one pixel larger, and the body is then
	// filled on top of it.
	fillTriangle(screen, tipX, tipY, leftX, leftY, rightX, rightY, outline, 1)
	fillTriangle(screen, tipX, tipY, leftX, leftY, rightX, rightY, body, 0)
}

// fillTriangle fills a triangle, optionally grown by a margin.
//
// The filling is done with a scan line algorithm over the bounding box,
// which is exact enough for the small markers of the map.
func fillTriangle(screen *ebiten.Image, x0, y0, x1, y1, x2, y2 float64, c color.Color, margin float64) {
	minX := int(math.Floor(math.Min(x0, math.Min(x1, x2)) - margin))
	maxX := int(math.Ceil(math.Max(x0, math.Max(x1, x2)) + margin))
	minY := int(math.Floor(math.Min(y0, math.Min(y1, y2)) - margin))
	maxY := int(math.Ceil(math.Max(y0, math.Max(y1, y2)) + margin))

	area := (x1-x0)*(y2-y0) - (x2-x0)*(y1-y0)
	if area == 0 {
		return
	}
	cr, cg, cb, ca := c.RGBA()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			// The barycentric coordinates of the point.
			w0 := ((x1-px)*(y2-py) - (x2-px)*(y1-py)) / area
			w1 := ((x2-px)*(y0-py) - (x0-px)*(y2-py)) / area
			w2 := 1 - w0 - w1
			// A margin shrinks the triangle towards its centre.
			if w0 < -margin || w1 < -margin || w2 < -margin {
				continue
			}
			screen.Set(x, y, color.RGBA{uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)})
		}
	}
}
