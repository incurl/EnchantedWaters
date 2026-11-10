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

The numbers of a block of `WORLDMAP.LZW` are the positions of the cells
that differ from the tile before them. The high byte of a number is the
row of the cell and the low byte is the column. The rows repeat, because
the list is sorted by the row of the page and not by the row of the map.

The routine at the offset 0xdafa of the fourth segment reads the map out
of the buffer at 0x5ee and unpacks it: a cell of the map is two bytes that
hold four nibbles, and every nibble is looked up in a table of sixteen
entries at the offset 0x87bb of the segment. That is the planar format of
the original game: four planes of four bits, so a cell names sixteen
colours.

The land and the sea are also held in a bitmap of 540 bytes in the fourth
segment, at the offset 0xf6c. The bitmap has 45 rows of twelve bytes,
so 96 columns of one bit each. The routine at the offset 0x67ea clears
the bitmap and then sets the bits of the rectangle of the screen that is
shown.

## The wind and the currents

`WINDCUR.DAT` holds 1350 bytes, and it is three tables of 450 bytes.
There is no header and no magic.

The loader is at the offset 0x78ef of the fourth segment. It reads 450
bytes and then seeks and reads 450 bytes more, and the byte at the offset
0x735 of the data decides whether the tables are read at all: that byte
holds the month, and the map is only drawn in the summer and in the
winter.

The byte of a sea area packs two numbers:

    bits 0 to 2   the direction, a number from 0 to 7
    bits 3 to 7   the strength, a number from 0 to 31

The routine at the offset 0x816f masks the lower three bits of the byte
to get the direction and masks the byte with 0xf8 to keep the strength
when it writes a new direction into the record of the sea area. The
routine at the offset 0x7b01 takes the same three bits and uses them to
index the table of the eight unit vectors of the compass, so a direction
is a quarter turn of 45 degrees, counted clockwise from the north.

The wind is not read from the file as it is. The game keeps a record of
96 bytes for every sea area, and it fills the records from the file. The
record holds the direction and the strength at the offset 0x13, and the
four bits of the byte at the offset 0x12 that are not used hold the
number of the arrows that are drawn on the map.

The wind of a ship is not the wind of the sea area that the ship is in.
The routine at the offset 0xd95e samples the four corners of the hull and
weighs the results, and the routine at the offset 0x7ac6 does the same
for the currents.

## What is still missing

- The three blocks of `WORLDMAP.LZW` are a list of positions, and the cell
  itself is two bytes of four nibbles that name the sixteen colours of a
  tile. What is not settled yet is how the positions of the list are laid
  out over the map, and what the tile of a position is: the file holds the
  colours of the cell, but the index of the tile of the tileset is not in
  it. It is probably in `GRAPH.DAT` or in one of the files of the pictures.
- The order of the three tables of the wind is not settled yet. The bytes
  of the file are 450 for the wind, 450 for the strength and 450 for the
  currents, but which of them is which is not written down in the
  executable.
- The texts of the story are in `EVENT*.DAT`, `ITEM.MES` and `NAME.TBL`,
  and the routine that reads them has not been found yet.
