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
| Scrollable world map with land/sea collision | done |
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
./game validate                           # check the configuration tables
./game import --raw=./assets/raw --out=./assets/config
```

### Controls

| Key | Action |
|---|---|
| Arrow keys | pan the map |
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

### Legal note

The importer is for personal reference conversion only. No original copyrighted asset
is included in this repository, in the release binary, or in the runtime of the game.
`assets/raw/` is gitignored and is meant to hold the files you supply yourself.

---

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
