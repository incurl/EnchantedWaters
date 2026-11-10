# What MAIN.EXE tells us

These notes record what the executable of the original release holds, and
where in it every fact was found. The executable is the ground truth for
the data formats: the files of the release were written by the code in it,
and the code that reads them is the definition of their meaning.

The addresses below are the offsets inside the segments of `MAIN.EXE`. The
program is a Turbo Pascal program in six segments, and the segments are
laid out in the file one after the other, starting at the offset 22016.

| Segment | File offset | Size   | Contents                        |
| ------- | ----------- | ------ | ------------------------------- |
| 0       | 22016       | 64288  | the start-up and the file names |
| 4036    | 86304       | 59824  | the world map and the wind      |
| 7775    | 146128      | 63968  | the rendering of the map        |
| 11775   | 210096      | 48592  | the data of the game            |
| 13915   | 258688      | 64000  | the texts of the user interface |
| 16954   | 322688      | 13152  | the strings of the programme    |

## The list of the files of the release

The names of the files are in the fourth segment, and they are the only
place in the executable where the names of the release appear. They are
plain ASCII with the drive letter of the original, and the offsets are
the offsets inside the segment:

    0x9224  C:DATA1.LZW        0x928e  C:EVENT.LZW
    0x9230  C:KAO.LZW          0x929a  C:IKKI.LZW
    0x923a  C:PORTCHIP.LZW     0x92a6  C:CHIP_NO.DAT
    0x924a  C:WORLDMAP.LZW     0x92b4  C:COLONY.DAT
    0x925a  C:ITEM.MES         0x92c2  C:EVENT*.DAT
    0x9266  C:SNRDAT.LZW       0x92d0  C:GRAPH.DAT
    0x9274  C:SNRMES.LZW       0x92dc  C:KOUKAI2.DAT
    0x9282  C:DATA2.LZW        0x92ea  C:MONSTER.DAT
                                0x92f8  C:SNR*.DAT
                                0x9304  C:TRANSIT.DAT
                                0x9312  C:WINDCUR.DAT
                                0x9320  C:ZA_DAT.DAT
                                0x932e  C:IAE1.LZW
                                0x933a  C:IAP*.LZW
                                0x9346  C:PORTMAP.LZW
                                0x9354  C:SNR*.MES
                                0x9360  C:HDAT.PUT
                                0x936c  C:NAME.TBL
                                0x9378  C:USERNAME.DAT
                                0x9388  C:D2.MML

The names that hold a star are opened with the wildcard helper of the
program, which expands them over the files of the directory. The names
that are built by the code are built from a base and a suffix that lie
next to each other in the table, so the offsets of the suffixes fall
inside the names that precede them.

Files that the project has not touched yet and that are worth a look:
`DATA1.LZW`, `DATA2.LZW`, `EVENT.LZW`, `IKKI.LZW`, `ITEM.MES`,
`HDAT.PUT`, `NAME.TBL`, `KOUKAI2.DAT`, `MONSTER.DAT`, `TRANSIT.DAT` and
`ZA_DAT.DAT`.

## The world map

The world map is loaded by the routine at the offset 0xb3e8 of the first
segment. The routine reads a byte out of the record of the port at the
offset 0x0c, and a byte out of the record of the scenario at the offset
0x0d:

    0xb3e8  test byte ptr [bx + 0x0c], 0x80   the flag of the record
    0xb3f1  test byte ptr [bx + 0x0d], 0x20   the scenario
    0xb3f7  mov ax, 0x9240                    C:PORTCHIP.LZW
    0xb3fc  mov ax, 0x924a                    C:WORLDMAP.LZW

The map of the world is `WORLDMAP.LZW`, and it is a file of the LS11
codec. It is three blocks of 25394, 21973 and 23907 bytes, and every
block is a list of sixteen bit numbers.

The map itself is 2160 columns by 1080 rows of tiles, and the sea areas
are 72 by 72 tiles. That gives 30 columns and 15 rows of sea areas, and
450 sea areas in all. The division by 72 is confirmed by the code that
looks up the wind of a position, and by the size of the wind file.

The size of the map is confirmed twice: the routine at the offset 0xdc82
of the fourth segment holds the width 2160 in a register, and the routine
at the offset 0xa399 compares the height against 1080 and moves the
position by 2160 when the fleet passes the edge of the world, because the
map wraps around the globe.

The routine at the offset 0xdafa of the fourth segment draws a row of the
map. It reads two bytes of the buffer at 0x5ee for every cell, splits them
into four nibbles, and turns each nibble into part of one output byte, so
a cell of the map is one pixel of the picture and not a tile. The routine
also compares a value against 0x80, which is the 128 tiles of the world
tileset, so the same routine serves both the tiles and the pixels.

What the numbers inside `WORLDMAP.LZW` mean is not settled. They are
sixteen bit numbers and 65535 of them is a sentinel: 48 of the 12697
numbers of the first block, 32 of the second and 18 of the third, and the
rest reach as high as 65534. The high byte of a number repeats, and the
largest differences between two neighbours are 1, 256 and 1024, which is
not the shape of a list of row and column pairs. The file is therefore
still open.

The land and the sea are also held in a bitmap in the fourth segment, at
the offset 0xf6c. The routine at the offset 0x67ea clears 540 bytes of it
and then sets the bits of the rectangle of the map that the screen shows,
so the bitmap is the land and sea mask of the visible part of the map and
not of the whole world.

## The wind and the currents

`WINDCUR.DAT` holds 1350 bytes, and it is three tables of 450 bytes.
There is no header and no magic.

The loader is at the offset 0x6763 of the fourth segment. It reads 450
bytes into one buffer, then 450 bytes into a second one, and then the last
450 bytes, so the file holds:

    bytes   0..450   the wind of the summer
    bytes 450..900   the wind of the winter
    bytes 900..1350  the currents

The game tells the two seasons apart by the month. The month is at the
offset 0x735 of the data, and the routine at the offset 0x676e of the
segment reads the tables only when the month lies from three to eight,
which is the summer.

The byte of a sea area packs three things into one byte:

    bits 0 to 2   the direction, a number from 0 to 7
    bit  3        the flag of a strong wind
    bits 4 to 7   the strength, a number from 0 to 15

This is the layout of the record of a sea area, which the game keeps in
memory: the record is 96 bytes long and holds the wind at its offset
0x13. The routine at the offset 0x9703 of the segment takes the byte of
the record, masks it with 0xf8 to keep the strength and the flag, and
writes a new direction into the lower three bits. The routine at the
offset 0x816f masks the lower three bits to read the direction, and the
routine at the offset 0x971f sets the flag with an "or" of the bit 3.

The eight directions run clockwise from the north. The routine at the
offset 0x7b01 takes the three bits and uses them to index the table of
the eight unit vectors of the compass, so a direction is a quarter turn
of 45 degrees.

The wind is not read from the file as it is. The game keeps a record of
96 bytes for every sea area, and it fills the records from the file. The
four bits of the byte at the offset 0x12 that are not used hold the
number of the arrows that are drawn on the map.

The wind of a ship is not the wind of the sea area that the ship is in.
The routine at the offset 0xd95e samples the four corners of the hull and
weighs the results, and the routine at the offset 0x7ac6 does the same
for the currents.

The second remake of the game holds a table of the wind and the currents
as well, and its tables do not agree with the file of the release: the
mean distance between the direction of the file and the direction of the
remake is about two of the eight points, where the small random change
that the remake admits to would only move the direction by one. The
tables of the remake are a reconstruction, and the file of the release is
the truth.

## What is still missing

- The three blocks of `WORLDMAP.LZW` are a list of positions, and the cell
  itself is two bytes of four nibbles that name the sixteen colours of a
  tile. What is not settled yet is how the positions of the list are laid
  out over the map, and what the tile of a position is: the file holds the
  colours of the cell, but the index of the tile of the tileset is not in
  it. It is probably in `GRAPH.DAT` or in one of the files of the pictures.
- The order of the three tables of the wind is settled: the summer, the
  winter and the currents. The three tables of the release are read in that
  order by the loader at the offset 0x6763, and the two seasons are two
  tables of the same file rather than two readings of it.
- The texts of the story are in `EVENT*.DAT`, `ITEM.MES` and `NAME.TBL`,
  and the routine that reads them has not been found yet.
