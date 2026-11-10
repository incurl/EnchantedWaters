# What MAIN.EXE tells us

These notes record what the executable of the original release holds, and
where in it every fact was found. The executable is the ground truth for
the data formats: the files of the release were written by the code in it,
and the code that reads them is the definition of their meaning.

The addresses below are offsets inside a segment of `MAIN.EXE`. The program
is a Turbo Pascal program, and its units are separate segments.

## How the program is laid out

The MZ header occupies 1376 paragraphs, so the load module starts at the
file offset 22016. The relocation table sits at `e_lfarlc`, 0x1E, and holds
5422 records of four bytes: a sixteen bit offset and a sixteen bit segment.
The segment numbers are paragraph addresses relative to the load module, so
the file offset of a unit is

    file = 22016 + segment * 16

The units tile the load module exactly, which is a useful check on the
arithmetic:

| Segment | File offset | Bytes | Contents                          |
| ------- | ----------- | ----- | --------------------------------- |
| 0       | 22016       | 64576 | the start-up and the main program |
| 4036    | 86592       | 59824 | the map, the wind and the sailing |
| 7775    | 146416      | 64000 | the user interface                |
| 11775   | 210416      | 34240 | the runtime library and constants |
| 13915   | 244656      | 48624 | the global data segment           |
| 16954   | 293280      | 257   | a last, short unit                |

The data segment is at paragraph 0x3657. The start-up of segment 0 holds the
word 0x3657 at the offset 0x3352, and the relocation table has an entry for
exactly that word, so the value is patched to `load + 0x3657` and later
copied into DS. The image of the data segment therefore starts at the file
offset 0x3BB70, which is 64 bytes before the segment table's data segment
at 0x365B; those 64 bytes are the prefix of the runtime and the globals
begin after them. Every global that these notes mention, such as `[0x5ee]`,
`[0x5f0]` and `[0xf6c]`, is at `DS:offset` and therefore at the file offset
`0x3BB70 + offset`.

### A trap that cost time

An earlier disassembler read the relocation table backwards, that is, it
treated the offset field as the segment, and it looked for the table just
before the load module instead of at 0x1E. As a result every address it
printed for a segment other than the first was 0x120 bytes too high, and
the segment sizes it derived were wrong. The addresses in the earlier
version of these notes inherited that error. The addresses below are the
corrected ones.

## The list of the files of the release

The names of the files are typed constants of the unit at segment 11775, at
0x90A2 to 0x9250. The code itself uses a second copy, in the data segment at
**0xB22 to 0xCD0**, and that is the copy to follow when the code does
`lea bx, [0xb64]`. The offsets below are the offsets of the data segment
copy:

    0xB22  C:MENU.DAT         0xC10  C:GRAPH.DAT
    0xB30  C:MESSAGE2.DAT     0xC1C  C:KOUKAI2.DAT
    0xB40  C:MESSAGE.DAT      0xC2A  C:MONSTER.DAT
    0xB4C  C:MAIN.EXE         0xC38  C:SNR*.DAT
    0xB56  C:CHAR.LZW         0xC46  C:TRANSIT.DAT
    0xB64  C:DATA1.LZW        0xC52  C:WINDCUR.DAT
    0xB72  C:KAO.LZW          0xC60  C:ZA_DAT.DAT
    0xB7A  C:PORTCHIP.LZW     0xC6C  C:IAE1.LZW
    0xB8A  C:WORLDMAP.LZW     0xC76  C:IAP*.LZW
    0xB9A  C:ITEM.MES         0xC86  C:PORTMAP.LZW
    0xBA6  C:SNRDAT.LZW       0xC94  C:SNR*.MES
    0xBB4  C:SNRMES.LZW       0xCA0  C:HDAT.PUT
    0xBC2  C:DATA2.LZW        0xCAC  C:NAME.TBL
    0xBCE  C:EVENT.LZW        0xCB8  C:USERNAME.DAT
    0xBDA  C:IKKI.LZW         0xCC8  C:D2.MML
    0xBE6  C:CHIP_NO.DAT
    0xBF4  C:COLONY.DAT
    0xC02  C:EVENT*.DAT

The names that hold a star are expanded by the wildcard helper of the
program. The names that are built by the code are built from a base and a
suffix, and the suffixes are the tail of a name that stands next to them.

Files that the project has not touched yet and that are worth a look:
`EVENT.LZW`, `IKKI.LZW`, `ITEM.MES`, `HDAT.PUT`, `NAME.TBL`, `KOUKAI2.DAT`,
`MONSTER.DAT`, `TRANSIT.DAT` and `ZA_DAT.DAT`.

## The world map

The map is 2160 by 1080 pixels, divided into 90 columns and 45 rows of
24 by 24 pixels. Three independent facts fix this:

- the routine that draws a cell at the offset 0xD83E of segment 4036
  divides a column by 30 and a row by 45, which is the extent of one chip;
- the routine at the offset 0x8303 of segment 0 sets the chip number to
  `x / 720`, and 720 is 30 columns of 24 pixels;
- the small map drawn at the offset 0xDAC9 of segment 4036 walks 45 rows of
  90 six-pixel blocks.

### The three chips

The graphics of the map are `WORLDMAP.LZW`, an LS11 file of three blocks
that decode to 25394, 21973 and 23907 bytes. Each block is one vertical
strip of the world, 30 columns wide and 45 rows tall, so the three strips
make the 90 columns.

A block is

    bytes 0..2700        1350 little endian sixteen bit numbers
    bytes 2700..         the records of the cells

The numbers are the offsets of the 1350 cell records, relative to the end of
the table: the record of cell `i` starts at `table[i] + 2700`. The cells run
row by row, 30 to a row, so cell `i` is at column `i mod 30` and row
`i div 30` of the strip. The decoder at the offset 0x46BA of segment 7775
does exactly this sum; it adds the constant 0xA8C, which is 2700, to the
table entry and reads the base of the strip from `[0x8ce]` and `[0x8d0]`.

The loader is at the offset 0x81F1 of segment 0. It opens `C:WORLDMAP.LZW`
and reads block number `[0x8cc]` into the buffer whose segment is `[0x8d0]`.
The caller at 0x8303 sets `[0x8cc]` to `x / 720` before the call, so only
the strip that the screen shows is in memory at any one time. The buffer is
the allocation of 0x700 paragraphs, 28672 bytes, which is large enough for
the largest strip.

### A cell record

The record of a cell is compressed. The decoder at 0x46BA reads the record
as three sub-records. Each sub-record begins with a control byte:

- the low seven bits, when they are 0x78 or more, select one of eight
  144-byte base patterns of twelve by twelve pixels at `DS:0x6C94`, entry
  `low - 0x78`;
- bit 7 set means that the whole patch is the base pattern and nothing more
  follows;
- bit 7 clear means that eighteen bytes of a mask follow, and then one
  literal byte for every set bit of the mask, in reading order, most
  significant bit first;
- a control byte whose low seven bits are below 0x78 leaves the sub-record
  empty.

The two sea shades are the base patterns 4 and 5, and open sea is therefore
a one-byte record, `0xFC` or `0xFD`. That is why so many records are a single
byte. The record is read through the chip segment while the pattern table is
reached through `DS`, so the patterns are in the data segment and not in the
file. Where the program fills that table is not settled yet.

The exact placement of the three patches inside the 24 by 24 cell is not
settled either. The decoder multiplies the outer step by 0x24, adds the
sub-record step, multiplies the sum by twelve and adds the destination
pointer, and then writes twelve rows with a stride of 24 bytes, but which
corner of the cell each sub-record lands in is still open.

### The explored area, not the terrain

The 540-byte bitmap at `DS:0xF6C` is 45 rows of twelve bytes, that is, 45
rows of 90 bits with six bits of padding. The routine at the offset 0x66CA
of segment 4036 clears it and then sets the cells of rows 8 to 16 and
columns 4 to 16, the rectangle around the starting position. The routine at
0xD83E tests a bit before it draws a cell, and other routines set bits as
the fleet moves. The bitmap is therefore the part of the map that the
player has explored, and not the land and sea of the world.

### What the earlier notes got wrong

The earlier notes read the routine at 0xB3E8 of segment 0 as the place where
the game chooses between `PORTCHIP.LZW` and `WORLDMAP.LZW`. It does not. The
two constants it loads, 0x9240 and 0x924A, are offsets into the data
segment, where they hold the strings `" Support "` and `" Enemy "`, and the
table at `DS:0xA8C` that follows holds the offsets of the names of the
sixteen sides of a battle. The routine is a dialogue routine, and the
filename table has an unrelated copy of the same offsets.

## The graphics of the map

The other graphics that the map uses are in `DATA1.LZW`. The routine at the
offset 0x29B1 of segment 4036 opens the file and reads three of its blocks
into the buffer whose segment is `[0x5f0]`:

    block 0   23040 bytes   at offset 0
    block 2    4320 bytes   at offset 0x5A00
    block 3    4320 bytes   at offset 0x6B00

The sizes in that list are the uncompressed sizes in the block table of
`DATA1.LZW`, and they match the counts that the code passes to the block
reader exactly. The buffer is the allocation of 0x900 paragraphs, 36864
bytes, which fits 31680 bytes with room to spare.

The routine at the offset 0xD9DA of segment 4036 then turns the first part
of the buffer into 128 small tiles on the stack. It reads four bytes of the
buffer for every output byte, uses the first two and skips the last two,
splits the two nibbles of each used byte and looks each nibble up in a
sixteen-entry table at `DS:0xAC68`. The table is

    0 1 0 1 0 1 0 1 2 3 2 3 2 3 2 3

so the sixteen colours of the source become two bits per pixel. The routine
builds 32 bytes of output for every 128 bytes of input, and the second
argument of the routine, 0x80, is the number of tiles.

The allocation sizes in these notes are paragraphs, as the allocator wants
them, and the word that follows the size in the call is a flag rather than
the high half of a size. The check is `[0xe12]`: the program allocates 0x30
paragraphs, 768 bytes, for it, and the routine at 0x6883 fills exactly 768
bytes of it. The earlier notes attributed the 0x400-paragraph allocation to
the map buffer, but the map buffer is the 0x900-paragraph one; 0x400
paragraphs is a different block, at `[0xe0e]`.

## The wind and the currents

`WINDCUR.DAT` holds 1350 bytes, and it is three tables of 450 bytes. There
is no header and no magic.

The loader is at the offset 0x6643 of segment 4036. It opens
`C:WINDCUR.DAT` at `DS:0xC52` and picks a table by the month at `[0x735]`:

    0x664E  cmp byte ptr [0x735], 3      a month before March
    0x6655  cmp byte ptr [0x735], 9      or from September
    0x6666  mov word ptr [bp - 4], 0x1C2 the wind of the winter, at 450

Months three to eight are the summer and use the offset 0. The loader then
reads 450 bytes at that offset into the buffer at `[0x602]` and `[0x604]`,
seeks to 900, reads 450 bytes into `[0x606]` and `[0x608]`, and closes the
file. The third table is therefore the currents and the first two are the
wind of the two seasons.

The byte of a sea area packs three things into one byte:

    bits 0 to 2   the direction, a number from 0 to 7
    bit  3        the flag of a strong wind
    bits 4 to 7   the strength, a number from 0 to 15

The evidence is in the record of a sea area, which is 96 bytes long and
holds the wind at its offset 0x13. The routine at the offset 0x95E3 masks
the byte with 0xF8 to keep the strength and the flag and writes a new
direction into the lower three bits. The routine at 0x95FF sets the flag
with an `or` of the bit 3. The routine at 0x804F masks the lower three bits
to read the direction. The routine at 0x79E1 uses the three bits to index
the eight unit vectors of the compass, so a direction is a quarter turn of
45 degrees clockwise from the north.

The wind is not read from the file as it is. The game keeps a record of 96
bytes for every sea area, and it fills the records from the file. The four
bits of the byte at the offset 0x12 that are not used hold the number of
the arrows that are drawn on the map.

The wind of a ship is not the wind of the sea area that the ship is in. The
routine at the offset 0xD83E samples the four corners of the hull and
weighs the results, and the routine at 0x79A6 does the same for the
currents.

The second remake of the game holds a table of the wind and the currents as
well, and its tables do not agree with the file of the release: the mean
distance between the direction of the file and the direction of the remake
is about two of the eight points, where the small random change that the
remake admits to would only move the direction by one. The tables of the
remake are a reconstruction, and the file of the release is the truth.

## What is still missing

- The eight base patterns of the cell decoder are filled in at run time and
  the place where they come from has not been found.
- The placement of the three twenty-four-pixel patches inside a cell is not
  settled.
- The tile index of a cell, if the map has one at all, is not in
  `WORLDMAP.LZW`; the file holds the picture of the cell and nothing that
  names a tile of a tileset.
- The texts of the story are in `EVENT*.DAT`, `ITEM.MES` and `NAME.TBL`,
  and the routine that reads them has not been found yet.
- The graphics of `DATA1.LZW` blocks 2 and 3, and the remaining blocks of
  the file, are not understood.

