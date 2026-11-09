package config

// This file declares the types of the tables that were imported from the
// original game assets.
//
// The data of the original game is *reference material*: it holds the
// original wording of the interface, the original dialogues, the names
// of the colonies and the descriptions of the items. It is imported by
// the asset importer and it is loaded by the runtime, exactly like the
// hand written tables.
//
// The tables are kept in their own namespace (the "original_" prefix) so
// that they never collide with the tables of this project, and so that it
// stays obvious which data comes from where.

// OriginalText is one line of an imported text table.
//
// The tables of the original game are plain lists of strings; the
// identifier is the index of the line in the original table.
type OriginalText struct {
	// ID is the index of the line in the original table.
	ID int `csv:"id"`
	// Text is the text of the line. It is optional, because the original
	// tables contain empty lines.
	Text string `csv:"text,opt"`
}

// OriginalColony is one colony or wonder of the original game.
type OriginalColony struct {
	// ID is the index of the colony in the original table.
	ID int `csv:"id"`
	// Name is the name of the colony. It is optional, because the
	// original table ends with empty padding records.
	Name string `csv:"name,opt"`
	// Description is the description of the colony.
	Description string `csv:"description,opt"`
}

// OriginalItemDescription is the description of one item of the original
// game.
//
// The identifier of an item is its index in the original table; the item
// itself is not part of the imported data, only its description.
type OriginalItemDescription struct {
	// ID is the index of the item in the original table.
	ID int `csv:"id"`
	// Description is the description of the item.
	Description string `csv:"description,opt"`
}
