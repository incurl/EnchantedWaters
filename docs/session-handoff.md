# Handoff: reverse-engineering the DOS executable

This is the state of the work on `MAIN.EXE` of the original release, and the
prompt to resume it.

## The goal

The project is a clean-room Go/Ebitengine reimplementation of *Uncharted
Waters II*. Runtime code reads only derived assets, never the release
binaries. The release's data formats are being recovered from the
executable, which is the ground truth: the files were written by that code.

## State: clean, committed, green

The working tree is clean. `go build ./...`, `go vet ./...` and `go test
./...` all pass (12 packages). `go run ./cmd validate` reports no problems,
and `go run ./cmd run --shot=/tmp/x.png --world=1` exits in under a second.

Commits from this work:

    f4b9a5c docs: keep the notes on the world map to what was verified
    afdad08 Correct the layout and the order of the tables of the wind
    d5cc0a4 docs: record what the executable says about the world map
    56483e8 Read the wind and the currents from the original release

Findings are written up in `docs/executable-notes.md`.

## Tooling, and how to read the executable again

`MAIN.EXE` is at `/home/peter/Downloads/Uncharted2/MAIN.EXE` (293,537
bytes). It is a Turbo Pascal MZ program in six segments:

| Segment | File offset | Size  |
|---------|-------------|-------|
| 0       | 22016       | 64288 |
| 4036    | 86304       | 59824 |
| 7775    | 146128      | 63968 |
| 11775   | 210096      | 48592 |
| 13915   | 258688      | 64000 |
| 16954   | 322688      | 13152 |

`e_cparhdr` is 1376, so the header is 22016 bytes and the load module
starts there. The entry point is CS:IP = 0001:0000.

There is a working disassembler at `/tmp/uz.py`, run with
`/tmp/disenv/bin/python /tmp/uz.py <segment> <offset> <hexlen>` (capstone
lives in that venv). The segment mapping is:

    HDR  = u16(8) * 16
    REL  = HDR - (0x1c + u16(6)*4)        # the relocation table
    BYSEG[seg] = [relocation offsets]     # a segment ends at max(offset)+16
    FBASE = cumulative file offsets from HDR

Two traps that cost time:

- **Linear disassembly from a segment start does not work.** Each Turbo
  Pascal unit interleaves code and data, so a linear pass stops at the
  first bad byte. Search for the byte pattern of an instruction you
  expect, then disassemble *at* that offset.
- **`os.IsNotExist` does not unwrap a `%w`-wrapped error; `errors.Is`
  does.** The optional-table loaders had this bug and only worked by luck.

## Settled: the complete file manifest

The only place the release's filenames appear is segment 11775, offsets
`0x9224`–`0x9388`. Full table in `docs/executable-notes.md`. Files the
project has still not examined: `DATA1.LZW`, `DATA2.LZW`, `EVENT.LZW`,
`IKKI.LZW`, `ITEM.MES`, `HDAT.PUT`, `NAME.TBL`, `KOUKAI2.DAT`,
`MONSTER.DAT`, `TRANSIT.DAT`, `ZA_DAT.DAT`.


## Settled and implemented: the wind and the currents

`WINDCUR.DAT` was never obfuscated. It is 1350 bytes: three tables of 450,
in this order — summer wind, winter wind, currents. The loader at segment
4036 offset `0x6763` reads 450 bytes into one buffer, 450 into a second,
then the last 450; the month at `[0x735]` picks the season (months 3–8 are
summer, checked at `0x676e`).

The byte of a sea area:

    bits 0-2   direction, 0-7, clockwise from north
    bit  3     flag of a strong wind
    bits 4-7   strength, 0-15

Evidence: `0x9703` does `and al,0xf8` then `or al,<new direction>`;
`0x816f` does `and ax,7` for the direction; `0x971f` sets bit 3 with an
`or`; `0x7b01` uses the three bits to index the eight compass unit
vectors. The in-memory record is **20 bytes** and the wind lives at its
last byte, offset 0x13.

Grid: **30 × 15 sea areas of 72 × 72 tiles** over the 2160 × 1080 map.

Implemented end to end: `pkg/importer/windcur.go` →
`assets/config/wind_current.csv` (450 rows + header, 8 columns) →
`config.WorldWind` (in `internal/config/original.go`) → `authenticWindAt`,
`windKnots` and `isSummer` in `internal/world/navigation.go`. The sea
zones of the configuration remain the fallback when the CSV is absent.

## New, NOT yet implemented: the map is a 90 × 45 grid of 24 × 24 tiles

This was found at the very end of the session and is not in the code.

The map data block is allocated at segment 4036 offset `0x72ce` as
`mov ax,0x400` — **0x400 paragraphs = 16,384 bytes** — and becomes the
segment of the map pointer at `[0x5f0]` (the offset is `[0x5ee]`).

The row index of the map is computed at segment 4036 `0xdd25`–`0xdd3e`:

    (Y / 24) * 90 + [0x8d7]

So the map is **90 columns** of 24-pixel cells, and:

    2160 / 90 = 24        1080 / 24 = 45
    90 * 45  = 4,050 cells
    4050 * 4 = 16,200 bytes   <= fits the 16,384-byte block

This also explains the land bitmap at segment 4036 offset `0xf6c`, which
is 540 bytes = **45 rows of 12 bytes** = 45 rows of 96 bits, i.e. 90
columns with six bits of padding. Same grid.

The memory allocation map of the same routine:

| Size | Paragraphs | Bytes | Pointer |
|------|-----------|-------|---------|
| 0x400 | 1024 | 16,384 | `[0x5ee]/[0x5f0]` world map |
| 0x240 | 576  | 9,216  | `[0x5f6]/[0x5f8]` wind records |
| 0x700 | 1792 | 28,672 | `[0x8d0]` |
| 0x20  | 32   | 512    | `[0x8d4]` |
| 0x20  | 32   | 512    | `[0x5fc]` |
| 0x40  | 64   | 1,024  | `[0x600]` |
| 0x39  | 57   | 912    | — |

450 × 20 = 9,000, which fits the wind block's 9,216.

The drawing routine is at segment 4036 `0xdafa`; it is called from
`0xdd1e` with `ax = 0x80` (the 128 tiles of the world tileset) and `bx` a
local buffer, and it reads the map from `[0x5ee]` two bytes at a time,
splitting each pair into four nibbles.

## Still open

1. **The meaning of `WORLDMAP.LZW`.** LS11, three blocks of 25,394 /
   21,973 / 23,907 bytes, decoding to 71,274 bytes. Two of the sizes are
   **odd**, so it is not a pure 16-bit array. In each block a 16-bit
   ascending table breaks at exactly **word 1351 (byte 2702)** — in all
   three blocks — after which come a short header and long runs of `0x0F`.
   The numbers look like `(row << 8) | col` reaching row 252–255, and
   65535 appears 48 / 32 / 18 times as a sentinel. This is the next thing
   to crack, and the 16,200-byte map size above is now an oracle for it.
2. `ZA_DAT.DAT` and `GRAPH.DAT` are unexamined.
3. The world-map tile index source.

## Do not trust the second remake's data

`/tmp/uw2ref` is `JohanLi/uncharted-waters-2`. Its `windsCurrent.wasm`
(2700 bytes = 6 × 450) and `worldTilemap.wasm` (2160 × 1080 bytes) are
**reconstructions, not decodes**. The remake admits to guesswork in its own
source comments, and every bit window, rotation and scale of the release
file fails to reproduce it: the mean direction disagreement is ~1.9 of 8
compass points where the remake's own ±1 jitter would allow ~0.2. Use it
for geometry and for cross-checks only; the executable is the only
authority on the data.

## House rules for this repository

- Runtime code reads derived assets, never the release binaries.
- Comments and prose use British spelling, no contractions, and explain
  *why* rather than restating the code.
- After each change: `go build ./...`, `go vet ./...`, `go test ./...`,
  `gofmt -l internal pkg cmd`, and `go run ./cmd validate`.
- Do not claim more than the evidence supports. One round of this work
  ended with a corrected overclaim about `WORLDMAP.LZW`; keep the notes
  honest about what is settled and what is open.

## A suggested first move for the next session

The 16,200-byte map is the oracle. Take the decoded `WORLDMAP.LZW`, try to
lay it out as 4,050 cells of 4 bytes, and check the result against
`assets/config/world_terrain.csv` (the derived land mask the project
already ships) and against the 130 port coordinates, which are known to be
correct. A layout that puts the ports on land and the oceans on water is
the layout the file uses.

