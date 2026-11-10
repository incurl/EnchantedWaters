# Enchanted Waters

A clean-room reimplementation of **Uncharted Waters II** (Daikoukai Jidai 2), written
in Go with [Ebitengine](https://ebitengine.org), targeting modern Linux (X11 / Wayland).

> **Clean room.** No original executable code was decompiled, reverse engineered or
> copied. Every rule, formula and balance value in this project was re-derived from
> publicly documented game mechanics and from the data tables that ship with the
> repository.

---

## Features

| System | State |
|---|---|
| Scrollable, zoomable world map with a derived land/sea mask | done |
| The 130 ports of the original world map, at their original positions | done |
| Business cycle, seasonality, scarcity and stochastic price noise | done |
| Regional price gaps and local specialties | done |
| Sailing physics (wind angle efficiency, currents, weather penalty) | done |
| Food / water consumption, fatigue, loyalty, mutiny probability | done |
| Port exchange, shipyard, tavern, governor, church, inn | done |
| NPC AI fleets with parallel batch evaluation | done |
| Three-axis reputation (Adventure / Trade / Naval) with titles | done |
| Table-driven event system with optional Lua scripting | done |
| Save / load of the dynamic state only | done |
| The town of every port, drawn from the tiles of the original game | done |
| The 128 portraits and the twelve facilities of the original game | done |
| The music and the sound effects of the original game, synthesised again | done |
| Development-only asset importer (binary → CSV / JSON / PNG) | done |
| Debug panel with ImGui (development builds only) | done |

---

## Build

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o game ./cmd/
```

The result is a single binary with no CGO and no dynamic Go dependencies.

> **Note on the libc linkage.** Ebitengine reaches the X11 / Wayland / OpenGL drivers
> through a pure-Go dynamic loader (`purego`), which needs `libc.so.6` at runtime. The
> Go code itself is fully static (`CGO_ENABLED=0`); the libc dependency comes from the
> graphics backend of Ebitengine, not from this project.

For a debug build with the ImGui panel:

```sh
CGO_ENABLED=0 go build -tags debugui -o game-debug ./cmd/
```

---

## Usage

```sh
./game run                                # start the game
./game run --config=./assets/config        # with an explicit table directory
./game run --port=ports.edo                # start at another port
./game derive                             # regenerate the playable tables
./game validate                           # check the configuration tables
./game import --raw=./assets/raw --out=./assets/config
```

The game needs a display. On a machine without one, a virtual frame buffer
serves it, and the vertical synchronisation has to be turned off because it
never fires there:

```sh
ENCHANTED_NO_VSYNC=1 xvfb-run -a ./game run -- --shot=/tmp/shot.png
```

The `--` matters: `xvfb-run` eats any option it does not know, and the
screenshot flag would go with it. The flag writes the canvas into a PNG at
the end of the first frame, and `--world`, `--open=port`, `--open=town`,
`--open=exchange`, `--open=shipyard`, `--open=tavern`, `--open=governor` and
`--open=system` choose the screen.

The game leaves as soon as it has written the screenshot. A screenshot is a
request of a tool and not a mode of the game: a game that waits for a key
that no one will press runs forever, and on a machine without a vertical
synchronisation the loop of the library spins as fast as it can and burns
every core of the machine. The run therefore takes a second and leaves
nothing behind.

### Controls

| Key | Action |
|---|---|
| Arrow keys | pan the map |
| `+` / `-` | zoom out / in |
| `M` | the whole world on one screen |
| `P` | open the port screen (when docked) |
| `Space` | centre the camera on the fleet |
| `C` | toggle the camera follow mode |
| `L` | toggle the message log |
| `F1` | open the system menu (save / load / speed / sound) |
| `F5` / `F9` | quick save / quick load |
| `1` … `4` | simulation speed |
| `Esc` | close the current screen |

---

## Architecture

The project follows a strict **two-tier data architecture**.

```
            ┌──────────────────────────────┐
            │  Static configuration layer  │  read-only, loaded once
            │  internal/config  (types)     │  map[ID]Struct + reverse indexes
            │  internal/asset  (loader)     │  sync.Once singleton
            │  assets/config/*.csv|json     │
            └──────────────┬───────────────┘
                           │  immutable, lock-free reads
            ┌──────────────▼───────────────┐
            │  Dynamic state layer          │  one instance per session
            │  internal/world  (simulation) │  sharded locks, parallel batches
            │  internal/ecs    (entities)   │
            └──────────────┬───────────────┘
                           │
            ┌──────────────▼───────────────┐
            │  Presentation layer           │  never mutates the simulation
            │  internal/ui    (screens)     │
            │  internal/render (drawing)    │
            └──────────────────────────────┘
```

**The rules that make this work**

1. The static tables never contain dynamic data; the dynamic state never embeds
   static definitions — only IDs that are resolved through the registry.
2. Save files serialise **only** the dynamic state. The tables are reloaded from
   `assets/config` when a save is restored.
3. Every numerical change is traceable to a formula and to a table entry.
4. The importer is a development CLI. The game runtime never calls it.

### Directory layout

```
/cmd/main.go            entry point: run / import / validate
/internal/asset         table loading and validation of the static layer
/internal/config        static configuration types and the indexed registry
/internal/ecs           the lightweight custom ECS
/internal/world         simulation: economy, navigation, weather, reputation
/internal/ai            NPC fleet behaviour
/internal/ui            screens, popups, input
/internal/render        drawing primitives, bitmap font, world map
/internal/event         event selection, effect resolution, Lua scripting
/internal/save          save file format
/pkg/etl                standalone data validation tool
/pkg/importer           development-only asset importer
/assets/config/         the CSV / JSON configuration tables
/assets/specs/          the format specifications of the importer
/assets/scripts/        the Lua event scripts
/assets/sprites/        the PNG sprite sheets exported by the importer
/assets/raw/            gitignored: the original files you supply yourself
```

---

## The economic model

The price of a commodity `c` at a port `p` on day `d` is the product of six
independent factors, each of which is normalised around `1.0`:

```
price(c,p,d) = Base(c)
             * Cycle(c,d)        business cycle:  1 + A_c * sin(2π * phase(c,p) / P_c)
             * Season(c,d)       yearly season:   1 + A_c * sin(2π * (doy + φ_c) / D)
             * StaticFactor(c,p) regional price gap from port_prices.csv
             * Specialty(c,p)    local production discount
             * Scarcity(c,p)     stock-driven feedback: 1 + K * (1 - 2 * S/(S+E))
             * (1 + Noise(c,p))  bounded mean-reverting random walk
```

* `E = demand / (supply + demand)` is the equilibrium stock of the port.
* `K` (`scarcity_response`) is a global parameter.
* The noise is `e(t+1) = e(t)·(1-δ) + σ·N(0,1)`, clamped to `±σ`.

Luxury goods get a larger volatility (`luxury_variance_scale`) and a wider
permitted price band, while specialties are produced in exactly one region and are
therefore cheap at the source and expensive everywhere else.

### Sailing physics

```text
v_ground = v_sail(δ, w) · weatherPenalty + v_current
```

`v_sail` is the sailing model of the original game. The compass has eight
directions and the wind blows at an integer strength, so the drive of a hull is a
lookup rather than a curve:

```text
δ      = octants between the heading and the wind, folded into 0..4
f      = windFactor[sailType][δ]
w      = wind speed, floored at 3 for a rowed hull
drive  = power · f · w / 150
crew   = min(1, crew / minCrew)
cargo  = min(1.5, 1.8 − (payload + crew) / capacity)
skill  = (10 + navigationLevel) / 10
base   = min(30, drive · crew · cargo · skill)
speed  = base · tacking' · seamanship / 75
```

`tacking'` is `tacking/100` when `δ ≥ 3` — the rig has to beat — and `1`
otherwise. The wind factor table is the one thing that makes the choice of hull
matter:

| sail type | running | | | | dead to windward |
|---|---|---|---|---|---|
| 1 (square) | 10 | 10 | 8 | 6 | 4 |
| 2 | 8 | 9 | 9 | 7 | 6 |
| 3 | 10 | 10 | 10 | 8 | 6 |
| 4 | 9 | 10 | 10 | 9 | 7 |
| 5 | 9 | 10 | 10 | 9 | 7 |
| 6 (lateen) | 8 | 10 | 10 | 10 | 8 |

A square rig keeps four tenths of its drive dead to windward; a lateen rig keeps
eight. There is no no-go zone: a ship can always make headway, it simply makes
less of it. Both quirks of the original are reproduced rather than corrected — a
stronger headwind makes a ship *faster*, because the wind speed multiplies in
regardless of `δ`, and the wind direction still matters for a rowed hull when the
wind is calm.

The model lives in `internal/sail` and is shared by the world and the AI, so a
voyage estimate and a simulated voyage agree. Two factors are held neutral because
this project has no skill system yet: the navigation level sits at the starting
value of 1 and the seamanship at 75, which is the value at which the seamanship
factor is exactly 1.

`v_current` is added as a vector: a following current speeds the fleet up, a head
current slows it down, and a beam current mostly pushes it sideways.

---

## The asset importer

The importer converts the original binary asset files into the standard CSV / JSON /
PNG format of this project. It is a **development tool**: it reads the files you
supply from your own disk and never bundles them.

```sh
./game import --raw=/path/to/your/files --out=./assets/config
```

The importer is driven by **format specifications** (JSON files in `assets/specs`).
A specification describes where the data lives inside a binary file and how the
records are laid out, so the parser itself contains no knowledge about any
particular game build:

```json
{
  "name": "example.numeric",
  "source": { "path": "EXAMPLE.NUM", "offset": 0, "length": 0 },
  "tables": [{
    "name": "example_numeric",
    "record_size": 32,
    "fields": [
      { "name": "id",   "type": "u16", "offset": 0 },
      { "name": "name", "type": "str", "offset": 8, "length": 24, "decode": "cp932", "trim": true }
    ]
  }]
}
```

Supported record types: `u8`, `i8`, `u16`, `i16`, `u32`, `i32`, `f32`, `str`, `bytes`,
with an optional `scale` / `offset_value` for the raw integer encodings of the
original game. Text is decoded from CP932 (Shift-JIS) with `golang.org/x/text`.

**The importer always writes a detailed audit log** (one line per parsed record) so
that a human can check the converted values before committing them.

### What the importer reads

The four layouts the original game uses for its text and its tables are all
supported, and each one is described by a specification rather than by code:

| Kind | Layout | Used by |
|---|---|---|
| `offset_table` | a table of big-endian offsets, then NUL terminated strings | `MESSAGE.DAT`, `MESSAGE2.DAT`, `MENU.DAT`, `SNR*.MES` |
| `nul_list` | a list of NUL separated strings | `NAME.TBL` |
| `fixed_text` | fixed size records of NUL padded text | `ITEM.MES`, `COLONY.DAT` |
| `menu_tree` | a two byte header (column width, item count) then the labels | `MENU.DAT` |
| *(default)* | fixed size records of mixed text and numeric fields | `KOUKAI2.DAT` |

### What has been imported

Running `./game import --raw=assets/raw --out=assets/config` against the extracted
English release (*New Horizons*) converts:

| Table | Records | Content |
|---|---|---|
| `original_messages` | 1000 | the interface wording of every shop and screen |
| `original_messages2` | 423 | the second page of the interface wording |
| `original_menu` | 270 | the whole menu tree, including the twelve facilities of a port |
| `original_dialogue_0..6` | 4511 | the dialogue of the seven scenarios |
| `original_names` | 192 | the name pool the game draws character names from |
| `original_item_descriptions` | 172 | one description per item |
| `original_colonies` | 100 | the colonies and wonders of the world map |
| `original_ports` | 130 | every port with its position on the original map |
| `original_characters` | 120 | the six protagonists and their companions: eight named stats, a navigation and a battle rank, and an age |
| `original_story_npcs` | 21 | the cartographers, professors and nobles |

The graphics are converted as well, into `assets/sprites`:

| File | Content |
|---|---|
| `portraits.png` | the 128 faces of `KAO.LZW`, 64 by 80 in three planes, eight to a row |
| `portchips.png` | the seven sets of 240 tiles of 16 × 16 of `PORTCHIP.LZW` |
| `townmaps.bin` | the 101 maps of the towns of `PORTMAP.LZW`, 96 × 96 tile indices each |
| `chip_sets.csv` | the set of tiles of every town, from `CHIP_NO.DAT` |
| `music.csv` | the twenty three songs of `D2.MML` and their notes |
| `effects.csv` | the forty eight effects of the seven banks `SNR*.DAT` |

The runtime loads these as **optional reference data**. The wording service of the
interface resolves its sentences through the original text and falls back to the
wording of this project when a line is absent, the port menu is built from the twelve
facilities of the original game, and a ship in port shows the town of the original
while its tavern draws the faces of the original. A data set without the imported
tables stays perfectly valid.

### Deriving the playable world

`./game derive` turns the imported reference data into the playable tables:

```sh
./game derive --config=assets/config
# derived 130 ports, 2730 port prices, 16 sea zones and 1578 land cells
```

**What is taken from the original:** the name and the position of every port, the
attribute byte that says whether a port has a court and a guild, and the split of
the table into 100 towns and 30 anchorages. An anchorage derives with every
facility at zero: it has a harbour and nothing else, exactly as in the original.

**What this project decides:** the class of a port (and with it its tax and its
facilities), the goods a region produces, the supply and demand of every good at
every port, and the wind and the currents of each sea zone. Every rule is written
down next to the code that applies it, and the derivation is deterministic, so the
result can be regenerated and reviewed at any time.

The land and sea mask (`world_terrain.csv`) is derived from the ports alone: a cell
becomes land when it lies close to a port, and the mask is then dilated and eroded
so that the outline follows the clusters of ports. It is a schematic world, not a
cartographic one.

### The rules of the original

The data files record what the game *contains*; they do not record what it *does*.
The reference implementation supplies the second half, and the paragraphs below
collect what it establishes, whether or not this project has acted on it yet.

**The character record.** The forty two byte character record holds the eight stats
of a sailor in a fixed order — leadership, seamanship, knowledge, intuition,
courage, swordplay, charm and luck — followed by a navigation rank, a battle rank
and an age. The order was recovered by matching three named sailors against the
reference: João Franco is 78/75/73/85/82/82/89/50 at navigation rank 1 and age 18,
Old Sea Hand Rocco is 75/82/84/90/93/92/70/70 at rank 30 and age 65, and Brother
Enrico is 66/48/93/55/62/48/82/100 at rank 1 and age 24. Those twenty four numbers
pin the order, and they correct the guess the message table had suggested for the
last three bytes, which are swordplay, charm and luck rather than charm, skill and
swordplay. `pkg/etl/characters_test.go` holds the three records as a regression
test.

João's seamanship of 75 and navigation rank of 1 are worth noting: they are exactly
the neutral values the sailing model uses, because `75` is the divisor at which the
seamanship factor is 1 and `1` is the rank at which the skill factor is `(10+1)/10`.
The neutral factors of this project are therefore the starting character's own, not
an arbitrary choice.

**The clock.** Time is measured in minutes from the start of the game, which is
08:00 on 17 May 1522 — `timePassed = 480`. A step of the world costs 20 minutes; an
action inside a port costs 40, 60 or 80 minutes at random; waiting costs the
difference to the next 08:00. The wind and the current are re-read every 240 minutes
(four hours), the day rolls over at midnight, and a day lasts 1440 minutes. Daylight
runs from 04:00 to 20:00, and the people of a port appear at dawn and vanish at
dusk.

**The provisions.** A hold carries four kinds of supply, not two: water, food,
**lumber** for repairs and **shot** for the guns. This project models the first two.

**The shipyard.** A port has an `industry` value and an `industryId` naming one of
eleven shipbuilding traditions. The tradition decides which hulls the yard can lay
down, each with an industry requirement, and a hull is available only when the
port's industry reaches that requirement. The number of used hulls on offer follows
the industry as well: two below 400, two to four to 600, two to six to 800, and two
to eight above. A used hull is sold at 85% of its durability, and any hull is bought
back at half its price. A rating costs 40 gold.

**The bank.** The credit line is 1000 gold: the debt may not exceed it.

**The shops.** An item shop sells at the listed price and buys at half of it. Some
ports keep a secret item that is only on the shelf between 02:00 and 03:00.

**The politics.** A port is not owned by one nation but shared between six, as a
vector of six percentages that sums to a hundred: Lisbon is 100% Portuguese,
Seville and Barcelona 100% Spanish, Istanbul 100% Ottoman, Algiers 20% held, and
Veracruz is held by nobody at all. Each port also carries an `economy` and an
`industry` value between 35 and 810, and a list of the items its shop stocks.

**What the reference does not have.** The market, the palace, the guild and the
fortune teller are declared in the building table with their menus but are not
implemented, so there is no trade price formula to compare against this project's
own. The same is true of naval combat.

### The graphics layout

The graphics of the DOS releases are **planar**: the colour of a pixel is not
stored as a byte, it is stored as one bit in each of several *planes*. A three plane
image therefore holds three bits per pixel and eight colours, and a four plane image
holds sixteen.

Two details of the layout matter and both are part of the specification of a sprite
sheet:

* **The plane order.** The EGA adapters of the era wired the planes in an order that
does not match their bit number. The reference analysis of the first Uncharted Waters
documents the order `1, 4, 2` for its three plane art, which is written in a
specification as `"plane_order": [0, 2, 1]`: the first stored plane is bit 0, the
second is bit 2, the third is bit 1.
* **The interleaving.** In the interleaved layout the bytes of the planes of one group
of eight pixels follow each other; in the planar layout a whole plane is stored before
the next one begins. `"interleaved": true` selects the first.

The palette is the bright half of the EGA palette (`"ega_palette": true`), which is
held in `pkg/importer/graphics.go`.

A specification can therefore describe a sprite sheet completely:

```json
{
  "graphics": [{
    "name": "ega_sprite", "offset": 288,
    "width": 24, "height": 12, "planes": 3, "columns": 1, "count": 1,
    "plane_order": [0, 2, 1], "interleaved": true, "ega_palette": true
  }]
}
```

and the importer writes an indexed PNG with the eight colour palette. See
`assets/specs/examples/planar_ega.json`. The decoder is covered by
`TestPlanarDecodeMatchesTheReference`, which builds a sprite in the documented layout
and checks that every pixel and the palette survive the round trip.

### The towns and the portraits

The game draws the pictures of the original game. It draws them out of derived data,
never out of the files of the release: the import step converts the pictures into
`assets/sprites`, and the game reads that.

| File | What it holds |
|---|---|
| `portraits.png` | the 128 portraits of the characters, 64 × 80 pixels, eight to a row |
| `portchips.png` | the seven sets of tiles of the ports, 240 tiles of 16 × 16 to a set |
| `townmaps.bin` | the 101 maps of the towns, a grid of 96 × 96 tile indices each |
| `chip_sets.csv` | the set of tiles of every town |

Three parts of the layout had to be worked out, and none of them is the general
planar layout of the section above.

**The tiles of the ports are stored in segments.** `PORTCHIP.LZW` holds one block of
30,720 bytes to a set, which is a picture of 16 × 3,840 pixels in four planes. The
four planes are not stored one after the other: the picture is cut into segments of
`align_length` bytes to a plane — 32 bytes — and inside a segment the planes follow
each other. A segment holds 256 pixels of a plane, which is exactly one tile:

```
segment s, pixel j:  byte = plane * 32 + j / 8,  bit = 7 - j % 8
```

so a set of 240 tiles is 240 segments, and the tiles follow each other down the
picture. The geometry is the one that the reference viewer of the series lists for the
file, and the decode agrees with its published rendering of `PORTCHIP.LZW` to the
tenth of a percent on the covered pixels.

**A map of a town is not a picture.** `PORTMAP.LZW` holds one block of 9,216 bytes to
a town, and a block is a grid of 96 × 96 one byte cells. Each byte is the index of a
tile of the set of the town, so the town is drawn by putting the tiles in their place
— which is why the grid is square: a width of 48 or 64 scrambles the rows, while 96
lays out a facade, a water front, the streets of the town and the open sea in order.

**The fourth plane of the tiles is the intensity bit.** The tiles name sixteen
colours while the palette of the release, `HORIZON_KAO.PAL`, holds eight. The tiles
use the indices 9, 11, 13 and 15 about as often as 1, 3, 5 and 7, which is how a wall
is drawn once in the shade and once in the light: the fourth plane is the intensity
bit of the EGA adapters. The bright half of the table is not in the palette file, so
the game folds the two halves together and draws the towns in the eight colours that
the release actually ships instead of inventing the other eight.

`CHIP_NO.DAT` is the table that says which set a town is built from, and it agrees
with the map without being told: the ports of northern Europe take set 1, the
Caribbean and Africa set 3, India set 6, China set 4 and Japan set 5. Bergen is drawn
in timber and a fjord, Lisbon in stone and a harbour, Sakai in tatami roofs and green
tiles.

The exporter is `ExportTownData` in `pkg/importer/towndata.go`, and the reader is
`internal/asset`. `TestExportTownData` converts the release and checks every derived
file, and `TestPortSceneUsesTheRightSet` walks the whole table of the towns.

When `assets/sprites` is missing the game draws its own procedural interface and says
nothing: the pictures of the original are an enrichment, not a dependency.

### The sound of the original game

The game plays the music of the original release and its sound effects. There
was no recording of the game to copy, so the sound is read out of the files of
the release and synthesised again from the same notes.

**The music is a command language.** `D2.MML` begins with a table of the
offsets of the tracks, and the table is a list of songs, and a song is a list
of tracks. Every command is one byte, and the byte says how many bytes follow
it:

| Byte | What it is |
|---|---|
| `0x00`–`0x5F` | a command with two parameters: the length of the notes and the volume |
| `0x60`–`0xBF` | a note, the pitch in the lower six bits |
| `0xC0`–`0xFF` | a command with one parameter |

A note is held for as long as the length that the last command with two
parameters set, and the lengths are the multiples of twelve of the language:
twelve for one sixteenth note, twenty four for two, forty eight for four.

**The sound driver is the ground truth.** The driver of the release,
`FMDRV.COM`, is a program of the era for the Yamaha chip of the sound card,
and it was read to settle every question the files left open: the command
lengths, the meaning of the parameters, the way the pitch is built and the
table of the F-numbers.

**The pitch comes from the table of the chip.** The driver holds the
F-numbers of the twelve notes of an octave — `343, 363, 385, 408, 432, 458,
485, 514, 544, 577, 611, 647` — and they are within a tenth of a percent of
the equal temperament, which is what a chip with a clock of 3.579545 megahertz
does. The F-number 343 at the block four is the middle C, and it sounds at
260.2 hertz.

**The octave is folded.** The driver divides the pitch of a note by twelve and
uses the quotient as the block of the chip, and the block of a chip is a
number from zero to seven. A note whose block is higher is folded into the
seventh block, and the note within the octave is kept.

**The effects are the same language.** `SNR0.DAT` to `SNR6.DAT` are banks of
sound effects. A bank begins with the magic `SNDT` and the number of the
bank, and the offsets of the effects follow, and the effects end with the two
bytes `0xF8 0xF2`, which the driver reads as a pointer back into the stream.

| File | What it holds |
|---|---|
| `music.csv` | the twenty three songs and their four thousand five hundred and ninety five notes |
| `effects.csv` | the forty eight effects of the seven banks |

The game plays the music of the sea and the music of the port, and it plays
the effect of a key, of a choice, of money, of a ship that leaves a harbour
and of bad news. The system menu has the sound and the volume.

Everything is optional. A machine without a sound device leaves the game
silent and working, and a build without the sound library is the same.

The reader is `pkg/importer/mml.go` and `pkg/importer/sndt.go`, the
synthesiser is `internal/audio`, and the exporter is `ExportSound` in
`pkg/importer/sound.go`.

### The LS10 and LS11 container

The `.LZW` files are not compressed with LZW, and the community name for the
codec — "the KOEI LZW variant" — is a misnomer. There is no dictionary of
phrases and no sliding window over the input. A block is a stream of variable
length codes, and each code either names a byte out of a 256 entry substitution
table or points back into the output that has already been produced.

The layout of a file is

```text
0..4      the magic, "LS10" or "LS11"
4..16     twelve zero bytes
16..272   a 256 byte substitution table, a permutation of 0..255
272..     the block table
```

The block table is a list of twelve byte records of three big endian 32 bit
numbers — the compressed size, the uncompressed size and the offset of the
block — and it ends with four zero bytes. The blocks tile the payload exactly:
the offset of a block is the offset of its predecessor plus the compressed size
of that predecessor. A block whose compressed size equals its uncompressed size
is not compressed at all.

The codes of a compressed block are read from a bitstream that starts at the
first byte of the block and is read most significant bit first. A code is a run
of one bits, a zero, and then the run length in bits:

```text
run    = the run of one bits, read as a number
factor = the run length in bits, most significant first
code   = run + factor
```

A code below 256 is an index into the substitution table. A code of 256 or more
is a back reference: the offset into the output is `code - 256` and the length
is a second code plus three. The blocks are independent of each other — each
has its own bitstream and its own output, and the back references of a block
point into that block's own output.

Two things are worth writing down, because both cost a lot of time to find:

* **the bit order is the one that looks wrong.** The bit index starts at 7 and
  counts *down*, and the byte index moves forward. That is plain most
  significant bit first, but it reads like least significant bit first if the
  two counters are confused.
* **the run of one bits is part of the number.** The zero that ends the run is
  the last bit of the run's value, not a separate marker, so a run of three
  ones followed by a zero is the number `0b1110`.

The graphics of the game are planar and are stored group interleaved: the
pixels come in groups of eight and each group is stored as one byte per plane,
the first byte holding the highest bit of the colour index. The portraits of
`KAO.LZW` are 64 by 80 pixels in three planes, which is 1920 bytes a piece.

The proof that all of this is right is `TestLSPortraitPixelsMatchTheReference`,
which decodes the portraits of `KAO.LZW` and compares them with the images of
a third party implementation pixel by pixel: all 128 of them are identical. The
byte level test is `TestLSDecodesEveryBlock`, which checks that all 675 blocks
of the fourteen files of the release decode to exactly the size their record
asks for.

### What the importer does not read (yet)

Some of the asset files could not be converted, because their container or their
encoding is not confirmed. They are listed here so that the gap is visible:

| Files | Content | Why it is not converted |
|---|---|---|
| `*.LZW` | the graphics: character portraits, the world map, the port chips, the opening | the container, the codec and the pixels are all recovered (see above). The portraits and the port chips are written out; the world map is a table of positions whose layout is not yet understood |
| `GRAPH.DAT`, `GRAPH2.DAT` | the tiles and icons | the offset table is decoded (a big-endian table of 65 blocks), but the blocks themselves use a codec that has not been identified |
| `EVENT*.DAT`, `ENDGRP.DAT`, `END_PUT.DAT` | the event and ending graphics | offset tables followed by planar graphics of an unknown geometry |
| `WINDCUR.DAT` | the wind and current table | now read: see [the wind of the original game](#the-wind-of-the-original-game) |
| `ZA_DAT.DAT` | an unknown table | the records are not understood, and guessing their meaning would be worse than leaving them out |
| `TRANSIT.DAT`, `MONSTER.DAT` | an animation and a small table | the records are not understood, and guessing their meaning would be worse than leaving them out |
| `D2.MML` | the music | now read: see [the sound](#the-sound-of-the-original-game) |
| `SNR*.DAT` | the sound effects | now read: see [the sound](#the-sound-of-the-original-game) |

Adding any of them means writing a specification for the container (for the offset
tables) and a decoder for the codec; the importer is built to take both without a
change to its code.

### Legal note

The importer is for personal reference conversion only. No original copyrighted asset
is included in this repository, in the release binary, or in the runtime of the game.
`assets/raw/` is gitignored and is meant to hold the files you supply yourself.
The imported tables hold **names and text** from the original game so that the
reimplementation can be compared with it; the rules, the balance and the code are
this project's own.

---

## References

The formats described above were recovered from the data itself and from three
public sources, none of which contributes code to this project:

* the data of the English DOS release (*New Horizons*), read as data only;
* [osgameclones.com/uncharted-waters](https://osgameclones.com/uncharted-waters/),
  which lists the open reimplementations of the game;
* [onionmixer/Water1-MapViewer](https://github.com/onionmixer/Water1-MapViewer)
  (MIT), whose `docs/data-provenance.md` documents the engine of the *first*
  Uncharted Waters: the EGA planar layout with the plane order `1, 4, 2`, the bright
  EGA palette, the twenty byte port record, and the meaning of the bits of the port
  attribute byte. The engine of the second game is its successor and shares the port
  record layout, which is why the same attribute byte was found at the same offset.

### The second remake

The port coordinates, the hulls and the sailing model of this project were
independently confirmed against
[JohanLi/uncharted-waters-2](https://github.com/JohanLi/uncharted-waters-2) (MIT),
a remake in TypeScript. The comparison is worth spelling out, because most of
it confirms what this project had already worked out on its own.

| What | The result |
|---|---|
| The map of the towns | `portTilemaps.wasm` of that project is **byte for byte** the payload of this project's `townmaps.bin`, which is an independent confirmation of the grid of ninety six by ninety six and of the two hundred and forty tiles of a set |
| The table of the towns | the tileset of every port of that project equals this project's `CHIP_NO.DAT`, port for port, and the counts agree: 38, 18, 18, 15, 5, 4 and 2 |
| The port coordinates | its `x` is this project's `x` plus 720, a third of the map width, and the `y` is the same for all one hundred and thirty ports. That remake wraps the map so that Europe is centred, and the twenty five ports of the Americas wrap past the edge |
| The hulls | the same twenty five ships in the same order, and every field of every ship agrees: the durability, the crew, the capacity, the tacking, the power, the minimum crew and the sail type |
| The sailing model | the same formula and the same table of the wind factors, sail type against the number of octants from the wind |

That project also settles two things this project had not worked out.

**The map of the world is 2160 by 1080 tiles of thirty two pixels.** Its
`worldTilemap.wasm` holds one byte for every cell of that grid, and a value of
fifty or more is land. The value is the index of a tile of a world tileset of
a hundred and twenty eight tiles, which the tileset of the release provides.

**The wind and the current are read from a grid of sea areas.** The sea area of
a position is the column divided by seventy two and the row divided by seventy
two, and the tables of the wind and the current are indexed by that number:
the direction of the wind in the summer and in the winter, the speed in the
summer and in the winter, and the direction and the speed of the current. The
file of the release, `WINDCUR.DAT`, holds the same data, and it is read now.

### The wind of the original game

The wind of the original game is not derived and not guessed: it is a table
of the release, and the format was recovered from the executable itself.

The file `WINDCUR.DAT` holds 1350 bytes and has no header. It is three tables
of 450 bytes, and every table holds one byte for every sea area of the world
map. The world map is 2160 columns by 1080 rows of tiles, and the sea areas
are 72 by 72 tiles, so there are 30 columns and 15 rows of sea areas and 450
of them in all.

The byte of a sea area packs three things into one byte:

    bits 0 to 2   the direction the wind blows to, a number from 0 to 7
    bit  3        the flag of a strong wind
    bits 4 to 7   the strength of the wind, a number from 0 to 15

The eight directions run clockwise from the north, so a direction is a quarter
turn of 45 degrees. The layout was read out of the fourth segment of
`MAIN.EXE`: the routine at the offset 0x9703 takes the byte of the record of
a sea area, masks it with `0xf8` to keep the strength and the flag, and
writes a new direction into the lower three bits; the routine at the offset
0x7b01 uses the same three bits to index the table of the eight unit vectors
of the compass.

The file holds the wind of the summer in its first table, the wind of the
winter in its second one and the currents in its third one. The loader at the
offset 0x6763 reads the three tables in that order, and the game tells the
two seasons apart by the month: the months from March to August are the
summer.

The importer writes the table to `assets/config/wind_current.csv`, and the
simulation uses it when it is there: the wind of a fleet is the wind of the sea
area that the fleet is in, for the season the calendar is in, with the weather
of the moment added on top of it. Without the table the game falls back on the
sea zones of the configuration.

The second remake of the game holds a table of the wind as well, and its
tables do not agree with the file of the release: the directions differ by
about two of the eight points on average, where the small random change that
the remake admits to would move a direction by only one. Its tables are a
reconstruction; the file of the release is the truth.

The executable is the ground truth for the rest of the release as well, and
[the notes on the executable](docs/executable-notes.md) record what it says:
the complete list of the files of the release, the loader of the world map, and
the geometry of the map.

The codec of the `.LZW` files was recovered from
[gcjjyy/koei_viewer](https://github.com/gcjjyy/koei_viewer) (MIT), a viewer for
the image data of the KOEI games of the DOS era. Its `ls11-decoder.ts` gives the
reading of the codes and the back references, and its `koei-image.ts` the layout
of the planar data. Its `config.json` records which file holds what: `KAO.LZW`
is the face data of this game at 64×80 in three planes, and the same file name
appears in several other games of the series with four planes and a different
size, so the geometry is a property of the file and not of the container. The
decoder of this project was written against that description and then checked
against the data: all 675 blocks of the fourteen files of the release decode to
exactly the size their record asks for, and all 128 portraits of `KAO.LZW` come
out pixel for pixel identical to the images of that viewer.

`WORLDMAP.LZW` is a different thing: it holds no graphics at all but a table of
numbers. It is a file of the LS11 codec with three blocks, of 25,394, 21,973
and 23,907 bytes, and every block is a list of sixteen bit numbers. The
executable loads the file at the offset 0xb3e8 of its first segment, and the
routine at the offset 0xdafa of the fourth segment turns the data into the
picture of a row of the map.

What the numbers mean is still open. 65535 of them is a sentinel, and the rest
reach as high as 65534, while the neighbours of a number differ by 1, 256 or
1024, which is not the shape of a list of row and column pairs. What is
certain is the geometry of the map the numbers describe: 2160 columns by 1080
rows, with the west and the east edges joined because the world wraps around
the globe.

The coordinates of the world come from
[BB9z/Uncharted-Waters](https://github.com/BB9z/Uncharted-Waters), which holds a
trainer for the game and the notes of its author. The trainer reads the save
files and prints the position of the fleet as a bearing, and that conversion is
the one of the original game: the map is 2160 pixels wide and 1076 high, the
equator runs at the row 640, and the longitude is not centred on the prime
meridian — the western edge is 29 degrees west, the middle of the map is 179
pixels into the eastern hemisphere, and the eastern edge runs back down to 29
degrees west, because the map wraps around the globe. Lisbon therefore comes
out at N39 W9, Seville at N37 W6 and Istanbul at N41 E28, which is within a
degree of the real places. The latitude of this project had been calibrated by
eye and was about four degrees too far south; it now follows the original.

The same trainer confirms the rest of the import. Its list of the twenty five
hulls, in Chinese, runs in exactly the order of the twenty five hulls of this
project: the Balsa is the 单桅渔船, the Hansa Cog the 汉萨柯克船, the Kansen the 关船.
Its list of the ports runs in the order of the imported port table as well, and
it names a sixth nation, the pirates, beside the five of the countries.

That remake is also the reference for the *rules* of the game, which the data
files do not spell out:

* **the hull table** — the names, and the power, tacking, sail type, capacity,
  crew, durability and price of all 25 hulls. The same 25 names appear in
  `KOUKAI2.DAT` at offset `0x4c47` as fixed length records in the same order, so
  the ordering is independently confirmed; the numeric fields of those records are
  not yet decoded, and until they are the figures come from the reference. The
  columns that the original does not state are computed from the ones it does,
  rather than chosen:

  ```text
  class             small under 150 tons, medium under 600, large above
  repair_cost_per_hp  price · 0.06 / durability   (6% of the price for a wreck)
  upkeep_per_day      price · 0.0005
  consumable_capacity capacity · 0.8
  weapon_level        max_guns / 25, capped at 5
  armor_level         durability / 18, capped at 5
  turn_speed_deg     45 + tacking / 2
  required_sailor_level   by price: 1 under 2 000, 2 under 10 000, 3 under
                          50 000, 4 under 150 000, 5 above
  has_oars            the seven rowed hulls, which the reference groups as the
                      last seven of the table
  ```
* **the sailing model** — the speed formula and the wind factor table above,
  including the constant of 150, the cap of 30 and the seamanship divisor of 75.
  `internal/sail` reproduces the arithmetic and `internal/sail/sail_test.go` pins
  it to a case worked out by hand;
* **the wind and current field** — a sea area is 72×72 pixels on a 30 column grid,
  and `WINDCUR.DAT` holds a direction and a strength per area for summer, for
  winter and for the current. This project derives its 16 sea zones rather than
  reading that file, because its encoding has not been recovered;
* **the buildings** — the twelve buildings of a port and the menu each offers: the
  market, the pub, the shipyard, the harbour, the lodge, the palace, the guild, the
  bank, the item shop, the church and the fortune teller;
* **the supply ports** — the port table holds 100 towns followed by 30 anchorages,
  which offer a harbour and nothing else. The split is positional, and the thirty
  names the reference lists as supply ports are exactly the last thirty records of
  this project's imported table;
* **the geography of the world** — eight regions for the sphere of influence of a
  palace, and thirteen markets that set the price of goods. This project derives
  its own regions from the map position, so these are recorded as the original
  design rather than reproduced;
* **the opening of the story** — the Lisbon prologue is a chain of quest states
  keyed on which building the player is in and which scenes have already played:
  the Duke's house, the pub, the church with Brother Enrico, the harbour, the
  shipyard and the market. The cast (João, Duke Franco, Duchess Christiana, Old Sea
  Hand Rocco, Brother Enrico, Carlotta, Lucia) is present in this project's
  imported dialogue, and the records of `original_story_npcs.csv` are indexed one
  lower than the reference's, which is the same table read from a different base.

## Testing

```sh
go test ./...                    # the whole suite
go test ./internal/world/ -v     # the simulation
./game validate                  # the configuration tables
```

The suite covers the ECS lifecycle, the economic model, the sailing physics, the
AI decision model, the event system, the Lua sandbox, the save/load round trip and
the importer.

---

## Licence

The code of this project is released under the terms of the `LICENSE` file.
