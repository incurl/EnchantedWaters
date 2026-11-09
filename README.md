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
| `F1` | open the system menu (save / load / speed) |
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

```
v_ground = v_hull · f(θ) · weatherPenalty + v_current
```

`f(θ)` is the wind efficiency, with `θ` the angle between the heading and the wind:

```
f(θ) = cos(θ)                for θ ≤ 60°
f(θ) = blend(cos, linear)     for 60° < θ ≤ 90°   (a smooth transition)
f(θ) = 0                     for θ > 90°          (the no-go zone)
```

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
| `original_characters` | 120 | the six protagonists and their companions, with abilities |
| `original_story_npcs` | 21 | the cartographers, professors and nobles |

The runtime loads these as **optional reference data**. The wording service of the
interface resolves its sentences through the original text and falls back to the
wording of this project when a line is absent, and the port menu is built from the
twelve facilities of the original game. A data set without the imported tables stays
perfectly valid.

### Deriving the playable world

`./game derive` turns the imported reference data into the playable tables:

```sh
./game derive --config=assets/config
# derived 130 ports, 2730 port prices, 16 sea zones and 1578 land cells
```

**What is taken from the original:** the name and the position of every port.

**What this project decides:** the class of a port (and with it its tax and its
facilities), the goods a region produces, the supply and demand of every good at
every port, and the wind and the currents of each sea zone. Every rule is written
down next to the code that applies it, and the derivation is deterministic, so the
result can be regenerated and reviewed at any time.

The land and sea mask (`world_terrain.csv`) is derived from the ports alone: a cell
becomes land when it lies close to a port, and the mask is then dilated and eroded
so that the outline follows the clusters of ports. It is a schematic world, not a
cartographic one.

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

### What the importer does not read (yet)

Some of the asset files could not be converted, because their container or their
encoding is not confirmed. They are listed here so that the gap is visible:

| Files | Content | Why it is not converted |
|---|---|---|
| `*.LZW` | the graphics: character portraits, the world map, the port chips, the opening | the `LS10` / `LS11` container has a sixteen byte header of zeros. The planar layout and the palette are known (see above), but the payload is compressed and neither the codec nor the geometry is recorded in the header; LZW in every bit order and width, LZSS in every flag polarity and field split, RLE and byte oriented LZ77 were all tried against it without a plausible output |
| `GRAPH.DAT`, `GRAPH2.DAT` | the tiles and icons | the offset table is decoded (a big-endian table of 65 blocks), but the blocks themselves use the same unknown codec |
| `EVENT*.DAT`, `ENDGRP.DAT`, `END_PUT.DAT` | the event and ending graphics | offset tables followed by planar graphics of an unknown geometry |
| `WINDCUR.DAT`, `ZA_DAT.DAT` | the wind and current table, an unknown table | the bytes are obfuscated and the key is not recoverable from the data alone |
| `TRANSIT.DAT`, `MONSTER.DAT` | an animation and a small table | the records are not understood, and guessing their meaning would be worse than leaving them out |
| `SNR*.DAT` | the sound effects and the music | the `SNDT` container is a sound bank, which the project does not use |

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

The port coordinates of this project were independently confirmed against
[JohanLi/uncharted-waters-2](https://github.com/JohanLi/uncharted-waters-2) (MIT),
which holds the same 130 ports: its `x` is this project's `x` plus 720, a third of
the map width, because that remake wraps the map so that Europe is centred.

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
