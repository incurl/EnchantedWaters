// Package render contains the low level drawing helpers of the game.
//
// The rendering layer is strictly separated from the simulation: it reads
// the world state and draws it, but never mutates it.
//
// # Retro style
//
// Everything is drawn with nearest-neighbour sampling and integer pixel
// scaling, which preserves the crisp look of the original 1990s art.
package render

import (
	"image/color"
)

// Palette colours of the original game's look.
//
// The values are the classic VGA/EGA-ish tones of the era. They are the
// only hard-coded colours in the project: they are presentation, not
// simulation parameters.
var (
	// ColorSea is the deep blue of the open sea.
	ColorSea = color.RGBA{0x1a, 0x33, 0x5c, 0xff}
	// ColorSeaDeep is the darker blue of the deep sea.
	ColorSeaDeep = color.RGBA{0x10, 0x22, 0x40, 0xff}
	// ColorCoast is the light blue of the shallow water.
	ColorCoast = color.RGBA{0x2f, 0x5c, 0x8a, 0xff}
	// ColorLand is the green-brown of the land.
	ColorLand = color.RGBA{0x4a, 0x5d, 0x33, 0xff}
	// ColorLandLight is the lighter shade used for the coast line.
	ColorLandLight = color.RGBA{0x6b, 0x7a, 0x45, 0xff}
	// ColorMountain is the grey of the mountains.
	ColorMountain = color.RGBA{0x77, 0x77, 0x80, 0xff}
	// ColorPort is the warm colour of a port marker.
	ColorPort = color.RGBA{0xd8, 0x9b, 0x3c, 0xff}
	// ColorFleet is the colour of the player's fleet marker.
	ColorFleet = color.RGBA{0xe8, 0xe8, 0xf0, 0xff}
	// ColorAIFleet is the colour of an AI fleet marker.
	ColorAIFleet = color.RGBA{0xc0, 0x50, 0x50, 0xff}
	// ColorGrid is the faint colour of the map grid.
	ColorGrid = color.RGBA{0x30, 0x48, 0x70, 0xff}
	// ColorPanel is the background of the UI panels.
	ColorPanel = color.RGBA{0x18, 0x1c, 0x28, 0xf0}
	// ColorPanelLight is the background of the highlighted panels.
	ColorPanelLight = color.RGBA{0x28, 0x30, 0x44, 0xf0}
	// ColorBorder is the border colour of the panels.
	ColorBorder = color.RGBA{0x9a, 0x8f, 0x6d, 0xff}
	// ColorText is the default text colour.
	ColorText = color.RGBA{0xe6, 0xe0, 0xc8, 0xff}
	// ColorTextDim is the colour of the secondary text.
	ColorTextDim = color.RGBA{0x9a, 0x94, 0x80, 0xff}
	// ColorTextBright is the colour of the highlighted text.
	ColorTextBright = color.RGBA{0xff, 0xf0, 0xa0, 0xff}
	// ColorGold is the colour of the money.
	ColorGold = color.RGBA{0xf0, 0xc0, 0x40, 0xff}
	// ColorGood is the colour of a positive change.
	ColorGood = color.RGBA{0x70, 0xd0, 0x70, 0xff}
	// ColorBad is the colour of a negative change.
	ColorBad = color.RGBA{0xe0, 0x60, 0x60, 0xff}
	// ColorWeather is the tint of the storm overlay.
	ColorWeather = color.RGBA{0x10, 0x18, 0x30, 0x60}
	// ColorNight is the tint of the night overlay.
	ColorNight = color.RGBA{0x00, 0x00, 0x20, 0x90}
	// ColorFrame is the colour of the frame that surrounds a picture of
	// the original game.
	ColorFrame = color.RGBA{0x9a, 0x8f, 0x6d, 0xff}
)
