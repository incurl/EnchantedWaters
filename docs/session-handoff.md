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
./...` all pass. `go run ./cmd validate` reports no problems, and
`go run ./cmd run --shot=/tmp/x.png --world=1` exits in under a second.

Findings are written up in `docs/executable-notes.md`. That file was
corrected in this session: every address it quoted for a segment other than
the first was 0x120 bytes too high, and the segment table was wrong. See
the next section for the cause.

## Tooling, and how to read the executable again

`MAIN.EXE` is at `/home/peter/Downloads/Uncharted2/MAIN.EXE` (293,537
bytes). It is a Turbo Pascal MZ program. The load module starts at the
offset 22016. The relocation table is at `e_lfarlc` = 0x1E, 5,422 records
of `(offset, segment)`, and the file offset of a unit is

    FBASE = 22016 + segment * 16

| Segment | File offset | Bytes | Contents                          |
| ------- | ----------- | ----- | --------------------------------- |
| 0       | 22016       | 64576 | the start-up and the main program |
| 4036    | 86592       | 59824 | the map, the wind and the sailing |
| 7775    | 146416      | 64000 | the user interface                |
| 11775   | 210416      | 34240 | the runtime library and constants |
| 13915   | 244656      | 48624 | the global data segment           |
| 16954   | 293280      | 257   | a last, short unit                |

The data segment is at `load + 0x3657`; its image begins at the file offset
0x3BB70. The globals are at `DS:offset`, that is, file `0x3BB70 + offset`.

The disassembler at `/tmp/uz.py` has been fixed to parse the relocation
table properly; run it with
`/tmp/disenv/bin/python /tmp/uz.py <segment> <offset> <hexlen>`. The old
version read the relocation table backwards and looked for it just before
the load module, which is why it printed every address of segments 4036,
7775 and 11775 as 0x120 too high and why it placed the last two segments
beyond the end of the file. Do not trust any address that was copied from
the old output.

Two traps that cost time:

- **Linear disassembly from a segment start does not work.** Each Turbo
  Pascal unit interleaves code and data, so a linear pass stops at the
  first bad byte. Search for the byte pattern of an instruction you
  expect, then disassemble *at* that offset.
- **`os.IsNotExist` does not unwrap a `%w`-wrapped error; `errors.Is`
  does.** The optional-table loaders had this bug and only worked by luck.

## Settled: the complete file manifest

The names are typed constants of segment 11775 at `0x90A2`–`0x9250`, and
again in the data segment at `0xB22`–`0xCD0`. The code uses the data
segment copy. Full table in `docs/executable-notes.md`. Files the project
has still not examined: `EVENT.LZW`, `IKKI.LZW`, `ITEM.MES`, `HDAT.PUT`,
`NAME.TBL`, `KOUKAI2.DAT`, `MONSTER.DAT`, `TRANSIT.DAT`, `ZA_DAT.DAT`.

## Settled and implemented: the wind and the currents

`WINDCUR.DAT` was never obfuscated. It is 1350 bytes: three tables of 450,
in this order — summer wind, winter wind, currents. The loader at segment
4036 offset `0x6643` opens `DS:0xC52`, picks the section by the month at
`[0x735]` (months 3–8 are summer, the test is at `0x664E` and `0x6655`),
reads 450 bytes at offset 0 or 450, seeks to 900 and reads the currents
into a second buffer.

The byte of a sea area:

    bits 0-2   direction, 0-7, clockwise from north
    bit  3     flag of a strong wind
    bits 4-7   strength, 0-15

Evidence: `0x95E3` does `and al,0xf8` then `or al,<new direction>`;
`0x804F` does `and ax,7` for the direction; `0x95FF` sets bit 3 with an
`or`; `0x79E1` uses the three bits to index the eight compass unit
vectors. The record of a sea area is 96 bytes and the wind lives at its
offset 0x13.

Grid: **30 × 15 sea areas of 72 × 72 tiles** over the 2160 × 1080 map.

Implemented end to end: `pkg/importer/windcur.go` →
`assets/config/wind_current.csv` (450 rows + header, 8 columns) →
`config.WorldWind` (in `internal/config/original.go`) → `authenticWindAt`,
`windKnots` and `isSummer` in `internal/world/navigation.go`. The sea
zones of the configuration remain the fallback when the CSV is absent.

## Settled, NOT yet implemented: the map is three strips of 30 × 45 cells

The map is 2160 × 1080 pixels, 90 columns and 45 rows of 24 × 24 pixels.
`WORLDMAP.LZW` is its graphics: an LS11 file of three blocks, each one
vertical strip of 30 columns × 45 rows, 720 × 1080 pixels. The loader at
segment 0 offset `0x81F1` opens the file and reads block `[0x8cc]` into the
buffer at `[0x8d0]`; the caller at `0x8303` sets `[0x8cc]` to `x / 720`.

A block is

    bytes 0..2700     1,350 little-endian u16 offsets
    bytes 2700..      the record of each cell

so cell `i` (row `i / 30`, column `i % 30`) starts at `table[i] + 2700`.
The decoder is at segment 7775 offset `0x46BA`; it adds `0xA8C` (2,700) to
the table entry and reads through the far pointer `[0x8ce]`/`[0x8d0]`.

A cell record is compressed into three sub-records. Each begins with a
control byte: a low seven bits value of 0x78 or more picks one of eight
144-byte base patterns at `DS:0x6C94`; bit 7 set means the whole patch is
the pattern; bit 7 clear means 18 mask bytes follow, then one literal byte
per set mask bit. Open sea is a one-byte record (`0xFC` or `0xFD`, base
patterns 4 and 5). The placement of the three patches in the 24 × 24 cell
is still open, and so is the origin of the base patterns.

The 540-byte bitmap at `DS:0xF6C` is not the terrain: it is the explored
area, cleared by segment 4036 offset `0x66CA` and filled as the fleet
moves.

The other map graphics are `DATA1.LZW` blocks 0, 2 and 3 (23,040 + 4,320 +
4,320 bytes), which the routine at segment 4036 offset `0x29B1` reads into
the buffer at `[0x5f0]`. The buffer is the 0x900-paragraph allocation,
36,864 bytes. The routine at `0xD9DA` turns 128 records of 128 bytes into
32-byte two-bit tiles through the table at `DS:0xAC68`.

The allocation sizes are paragraphs, not bytes: the size is the word in
`ax`, and `dx = 1` is a flag. The earlier handoff read `mov ax,0x400` as
the map buffer, but the map buffer is the `0x900` allocation; `0x400`
paragraphs belongs to `[0xe0e]`.

## Still open

1. **The eight base patterns of the cell decoder.** The decoder reads them
   at `DS:0x6C94`, but the image of the data segment holds ordinary text
   there, so the program must fill the table at run time. Finding where is
   the next step, and it would let the whole of `WORLDMAP.LZW` be decoded
   to pixels.
2. **The placement of the three patches in a cell.** The decoder adds
   `12 * (0x24 * outer + sub)` to the destination and writes 12 rows with a
   stride of 24 bytes; which corner each `sub` belongs to is not settled.
3. **The graphics of `DATA1.LZW`.** Blocks 2 and 3 are read into the same
   buffer as block 0 and drawn with the same rectangle routine; their
   format has not been worked out. `ZA_DAT.DAT`, `GRAPH.DAT` and
   `PORTMAP.LZW` are unexamined.
4. **The tile index.** `WORLDMAP.LZW` holds the picture of a cell, not a
   tile number, so the tileset of the big map, if there is one, is
   somewhere else.

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
- Do not claim more than the evidence supports. This session corrected two
  overclaims: the routine at segment 0 `0xB3E8` is a dialogue routine and
  not the loader of the map files, and the "16,384-byte map block" was the
  wrong allocation. Keep the notes honest about what is settled and what is
  open.

## A suggested first move for the next session

Find the code that fills the eight base patterns at `DS:0x6C94`. It is the
last piece of the cell decoder: with the patterns, `WORLDMAP.LZW` can be
decoded cell by cell into a 2160 × 1080 picture, and that picture is the
real world map. Compare the result against the 130 port coordinates, which
are known to be correct, and against the two sea shades `0xFC` and `0xFD`.

The patterns are 1,152 bytes, eight blocks of 144. The image of the data
segment holds ordinary text at that offset, so either the table is written
at run time or the decoder runs with a different DS; look for a routine
that writes a run of 0x480 bytes through a pointer, most likely from the
decoded `WORLDMAP.LZW` itself or from `DATA1.LZW`.
