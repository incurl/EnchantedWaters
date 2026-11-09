package config

// This file declares the type of the imported menu tree.
//
// The menu tree of the original game holds the structure of the whole
// interface: every port facility, every screen and every sub-menu.

// OriginalMenuItem is one entry of the menu tree of the original game.
type OriginalMenuItem struct {
	// ID is the index of the item in the whole tree.
	ID int `csv:"id"`
	// Menu is the index of the menu the item belongs to.
	Menu int `csv:"menu"`
	// Item is the index of the item inside its menu.
	Item int `csv:"item"`
	// ColumnWidth is the width of the item column in pixels, as the
	// original game draws it.
	ColumnWidth int `csv:"column_width"`
	// Label is the label of the item.
	Label string `csv:"label,opt"`
}
