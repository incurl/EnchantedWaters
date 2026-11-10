# Reading the Sea

## A forensic study of *Uncharted Waters*

---

### What this book is

This is a book about taking an old game apart. Not to copy it, but to
understand it — and, in understanding it, to learn two crafts at once: how
games are designed, and how programs are read when their source is gone.

It is written for someone who has never done either. If you can read a
little code and are curious about how a thing works, that is enough. The
book moves from the smallest possible question — what is in this file? — to
the largest — why does this game feel the way it does? — and tries to show
that these are the same question asked at different zooms.

The subject is *Uncharted Waters: New Horizons*, the 1995 English DOS
release of KOEI's 大航海時代II. It is a game about the age of exploration:
you command a small fleet out of Lisbon in 1523, and you trade, explore,
fight, and eventually discover a route to the East. It is famous for being
slow, atmospheric, and unexpectedly deep — a game where the wind matters,
where a voyage is a commitment, and where the map is not a menu but a place.

That feel is not an accident. It is the product of thousands of small
decisions, most of which are still recorded in the game's executable and
data files, waiting to be read. That is what we are going to read.

### Who it is for

Three kinds of reader, and the book tries to serve all of them:

- **The game designer** who wants to understand why a thirty-year-old game
  still feels better than many modern ones, and what specific mechanisms
  produce that feeling.
- **The reverse engineer** who wants to learn the discipline on a real,
  well-behaved, non-adversarial target — no anti-debugging, no packing, no
  obfuscation. Just a program and a puzzle.
- **The curious player** who has wondered what is actually inside the files
  and would like to be shown, rather than told.

### How to read it

The book has five parts. Part I describes the artefact — the era, the
machine, the toolchain. Part II is about method: how to form a hypothesis
about a binary, how to test it, and how to know when you are wrong. Part III
is four case studies, each a complete piece of forensics from first contact
to verified result. Part IV turns from the code to the design, and asks what
the mechanisms teach. Part V is practice: a worksheet, and a discussion of
the ethics.

You can read Part III first if you like — it is the most concrete. But the
method in Part II is what makes Part III comprehensible rather than magical.

### A note on honesty

Every technical claim in this book is either something the author verified
against the artefact, or is explicitly flagged as unsure. That distinction is
the whole discipline. A confident wrong answer is worse than an honest
question, and this book contains at least one story about exactly that
mistake — made, caught, and corrected — because you will make it too, and it
is better to see it coming.

Where a number is given — an offset, a size, a byte — it was read from the
file. Where a mechanism is described, it was read from the code that
implements it. Where something is inference, the word *probably* appears, and
it appears on purpose.


# Part I — The Artefact

## Chapter 1. What we have

Before we read anything, we should look at what is actually in the box.

The release is a directory of files. Two of them matter most:

- one huge executable, `MAIN.EXE`, of 293,537 bytes;
- a pile of data files, most of them under a hundred kilobytes.

There is no installer, no configuration, no documentation worth the name.
The files carry a modification date of 15 July 1995. That date is not the
date the game was designed — the Japanese original shipped two years
earlier — but it is the date this English edition was assembled, and it
gives us our era.

Here is the directory, sorted by size, which is already informative:

```
  476225  KAO.LZW          the character portraits
  457397  OPGRAPH.LZW      the opening graphics
  442349  GRAPH.DAT        the tiles and icons
  396400  ENDGRP.DAT       the ending graphics
  319591  KOUKAI2.DAT      a large table, content unknown
  138223  PORTMAP.LZW      the town maps
  123698  PORTCHIP.LZW     the town tiles
  104598  DATA1.LZW        a large table
   78875  GRAPH2.DAT       more tiles
   52338  IAP1.LZW         ship and item pictures
   ... and so on
```

Two things stand out immediately, and both are worth pausing on.

**First, the extensions are a taxonomy.** `.LZW` files hold pictures and
compressed tables. `.DAT` files hold raw tables. `.MML` holds music. `.MES`
holds text. This is a game whose authors thought of their data by *kind*,
and gave each kind a container. That is a design decision, and a good one:
it means that once you crack one `.LZW` file you have cracked all of them.

**Second, the sizes are suspiciously round in places.** `KAO.LZW` is 476,225
bytes. `PORTMAP.LZW` is 138,223. Neither is a round number, because both are
compressed. But we will later find that the *decompressed* sizes are
extremely round: `KAO.LZW` expands to exactly 368,448 bytes, which is 192
pictures of exactly 1,920 bytes each. And 1,920 bytes is exactly 64 × 80 × 3
÷ 8 — a portrait 64 pixels wide, 80 tall, in three bit-planes.

That is the first real lesson of forensics, and it is worth stating plainly:

> **Structure hides in arithmetic.** When a number decomposes cleanly into
> width × height × depth, you have found a picture. When it does not, you
> have either found something else, or you have not found the right number
> yet.

### The executable in one line

`MAIN.EXE` is a DOS MZ executable. The first two bytes are `4D 5A` — the
letters `MZ`, for Mark Zbikowski, who designed the format at Microsoft in
1981. Every DOS program that needed more than 64 kilobytes began this way
for fifteen years.

We will open it properly in Chapter 3. For now, just note the shape: a
header, then a lump of code and data, then nothing else. No overlay, no
compression, no packing. The whole program is *there*, in the clear,
293,537 bytes of it.

That last point deserves emphasis, because it is why this book can exist:

> This program was not written to resist you. It was written to run on a
> machine with 640 kilobytes of memory, in 1995, by people who had no reason
> to imagine anyone would take it apart. There is no anti-debugging, no
> encryption, no obfuscation. What there is, is *age*: a foreign instruction
> set, a foreign memory model, and a foreign compiler's conventions. Those
> are the obstacles, and they are all learnable.

### The machine it expected

To read old code you must know what machine it was written for, because the
machine's constraints explain most of its oddities.

This game expected:

- **An IBM PC or compatible**, in *real mode*. Real mode means the processor
  behaves like a very fast 1978 Intel 8086: 16-bit registers, one megabyte of
  addressable memory, and a strange segmentation scheme we will meet in
  Chapter 2.
- **640 kilobytes of conventional memory**, and probably expanded memory
  beyond it, because the graphics alone are large.
- **EGA graphics**: 320 × 200 pixels, 16 colours from a palette of 64. We
  know it is EGA and not VGA because the game's shipped palette file,
  `HORIZON_KAO.PAL`, contains only *eight* RGB triples, not 256 — the
  hallmark of a display with a fixed 16-colour attribute palette, where the
  sixteenth colour is the "bright" version of the eighth.
- **A YM3812 sound chip** — Yamaha's OPL2, the chip inside the Ad Lib and
  Sound Blaster cards. We know this because the music driver's note table
  contains the twelve *F-numbers* of that chip, the register values that
  produce each semitone. A number like 343 is not a frequency; it is an
  OPL2 register value for the C below middle C.

Hold those four facts. They will explain nearly every strange thing we find.

### Why this matters for design

It is tempting to treat the hardware as an embarrassing limitation that a
modern game has escaped. Resist that. The hardware is the *material*, and the
design is what the designers made of it.

A machine with 640 kilobytes, sixteen colours, and eight sound channels
cannot render a living ocean. So the designers did not try. They built a
world of *cells* — sea areas and map tiles — and gave each cell a wind, a
current, and a terrain. The ocean became a grid of numbers, and the sensation
of sailing became the sensation of moving between those numbers.

That abstraction is the heart of the game. We will spend Part IV on it. But
keep it in view from the beginning, because almost everything we reverse
engineer in Part III is *machinery for that abstraction*: a 1350-byte file
that says how the wind blows, a 16-kilobyte block that says what the world
looks like.

---


## Chapter 2. The machine, and why its memory smells

This chapter is the one people skip. Do not skip it. Almost every confusing
thing in the rest of the book comes directly from the segmentation model, and
once you have it in your bones the executable stops looking arbitrary and
starts looking *inevitable*.

### Sixteen bits is not enough

An 8086 register is sixteen bits wide. A sixteen-bit number can hold 65,536
distinct values. So the largest single number the processor can name is
65,535.

But the machine can address one megabyte — 1,048,576 bytes — which needs
twenty bits. There is a four-bit gap between what a register can say and what
the address bus can reach. The 8086's solution was blunt and brilliant:

> **An address is two sixteen-bit numbers, and you combine them by adding one
> to sixteen times the other.**

The high half is called a *segment*. The low half is called an *offset*. The
real address is:

```
address = segment × 16 + offset
```

Because of that ×16, a segment is thought of as a "paragraph": sixteen bytes.
The notation you will see everywhere is `SEGMENT:OFFSET`, written in hex, for
example `0x2DFF:0x387C`.

Two consequences, both of which we will meet:

**Segments overlap.** There are only 65,536 possible segments and each covers
65,536 bytes, so the same byte has thousands of valid names. `0x1000:0x0010`
and `0x1001:0x0000` are the same address. This is why a debugger sometimes
seems to disagree with itself.

**An offset alone is meaningless.** The number `0x387C` is just a number. It
becomes an address only when you know which segment it is used with.

### What a compiler does with that

A compiler targeting this machine has to make a decision it does not have to
make on a flat-address machine: *what goes in which segment?*

Turbo Pascal, the compiler used here, made a clean and rather elegant choice.
It put each *unit* of the program — each separately compiled module — into its
own segment. Code and data for a unit live together. When the unit needs to
call something in another unit, the call is a *far call*: it carries both a
segment and an offset, four bytes instead of two.

That decision is the reason this executable has the shape it does. Let us look
at it.

### Reading the header

Here is the first 48 bytes of `MAIN.EXE`:

```
00000000: 4d5a a100 3e02 2e15 6005 0604 ffff d042  MZ..>...`......B
00000010: 0038 0000 4433 0000 1e00 0000 0100 2916  .8..D3........).
00000020: 0000 5d16 0000 d016 0000 2017 0000 7d46  ..]....... ...}F
```

The MZ header is a fixed table of sixteen-bit fields. Reading it:

| Offset | Field | Value | Meaning |
|--------|-------|-------|---------|
| 0x00 | `e_magic` | `4D5A` | "MZ" |
| 0x02 | `e_cblp` | 0x00A1 = 161 | bytes used in the last page |
| 0x04 | `e_cp` | 0x023E = 574 | pages of 512 bytes |
| 0x06 | `e_crlc` | 0x152E = 5422 | number of relocations |
| 0x08 | `e_cparhdr` | 0x0560 = 1376 | header size, in paragraphs |
| 0x0A | `e_minalloc` | 0x0406 = 1030 | minimum extra paragraphs |
| 0x0C | `e_maxalloc` | 0xFFFF | maximum extra paragraphs |
| 0x0E | `e_ss` | 0x42D0 | stack segment |
| 0x10 | `e_sp` | 0x3800 | stack pointer |
| 0x14 | `e_ip` | 0x0000 | entry offset |
| 0x16 | `e_cs` | 0x0001 | entry segment |
| 0x18 | `e_lfarlc` | 0x001E | offset of the relocation table |

The first check you should always make is whether the arithmetic closes. Does
the file size match what the header promises?

```
(574 − 1) × 512 + 161 = 293,376 + 161 = 293,537
```

And `ls -l MAIN.EXE` says 293,537 bytes. It closes exactly. That tells us
there is no overlay, no appended data, nothing hidden after the end — the
header describes the entire file.

Now the header size: 1,376 paragraphs × 16 = **22,016 bytes**. That is much
larger than the 28-byte fixed part, and the difference is the *relocation
table*: 5,422 entries of four bytes each, which is 21,688 bytes. So:

```
28 (fixed header) + 21,688 (relocations) = 21,716, padded to 22,016
```

The code and data — the *load module* — therefore begins at file offset
22,016. Everything before that is bookkeeping.


### What relocations are for

Here is the beautiful part, and the part that tells you where the code is.

A far call encodes a segment and an offset. But the segment is only known at
*load time*, because DOS decides where to put the program in memory. So the
compiler cannot write the real segment into the instruction. Instead it writes
a placeholder and records the *location* of that placeholder in the relocation
table. At load time, DOS walks the table, finds each placeholder, adds the
load address, and patches it.

The relocation table is therefore a **map of every place in the program that
mentions a segment** — which is to say, every place that references data or
code in another segment.

We can use that map. Here it is, grouped by segment:

```
segment     0:  293 relocations, offsets 0x0000 .. 0xFB1B
segment  4036: 1851 relocations, offsets 0x0036 .. 0xE9AD
segment  7775: 2253 relocations, offsets 0x0013 .. 0xF9DA
segment 11775:  829 relocations, offsets 0x0010 .. 0xBDC9
segment 13915:  195 relocations, offsets 0x0000 .. 0xF9FC
segment 16954:    1 relocation,  offsets 0x3352
```

Six segments. The highest relocation offset in each tells us roughly how big
that segment is (the real end is the next paragraph boundary). Stacking them
from the start of the load module gives the layout:

| Segment | File offset | Size | Relocations |
|---------|-------------|------|-------------|
| 0 | 22,016 | 64,288 | 293 |
| 4036 | 86,304 | 59,824 | 1,851 |
| 7775 | 146,128 | 63,968 | 2,253 |
| 11775 | 210,096 | 48,592 | 829 |
| 13915 | 258,688 | 64,000 | 195 |
| 16954 | 322,688 | 13,152 | 1 |

Check: 322,688 + 13,152 = 335,840, which is past the file end of 293,537.
That is fine — the last segment's computed size is an over-estimate, because
the "end of segment" is inferred from the highest relocation, not from a
length field. The MZ format does not record segment lengths at all.

> **Forensic note.** This is the single most useful piece of reverse
> engineering you can do on a Turbo Pascal binary: derive the segment table
> from the size of the relocation table. It costs ten lines of code and turns
> a 293-kilobyte blob into six addressable regions. Without it, every offset
> you see in a disassembly is ambiguous. With it, `0x387C` has a home.

### The segments have personalities

We can already guess what each segment is for, before reading a single
instruction, from the *number* of relocations:

- **Segment 16954 has exactly one relocation.** A segment with one reference
  into it is not code — it is a data blob, referenced once as a whole. (We
  will later find it holds strings.)
- **Segments 7775 and 4036 have the most relocations** — 2,253 and 1,851.
  Lots of cross-unit references means lots of code. These are the engine.
- **Segment 13915 has only 195 relocations but is 64,000 bytes.** A big
  segment with few references is mostly *data*. Its content will turn out to
  be the user-interface strings: format templates like `%3d`, `%-14s %5d`,
  and messages like `Commodore, we found the port of %s!`.

That is already a working hypothesis about the program's architecture, derived
purely from arithmetic on a header. This is what forensics feels like when it
is going well: you are not guessing, you are *narrowing*.

### Where the entry point is, and why it does not help

`e_cs` is 1 and `e_ip` is 0. So execution begins at `0001:0000` — which is
file offset 22,016 + 16 = 22,032, the very first byte of segment 0.

But here is a thing that will bite you: disassembling from there produces
garbage, because Turbo Pascal puts a small startup stub first and then jumps
into the real initialisation, crossing into other segments. Linear
disassembly from the entry point reaches only a handful of instructions before
it walks into data.

We will come back to this in Chapter 5. For now, note the shape of the problem:
**the entry point is not where the interesting code is.**

### The map we carry from here on

From this point in the book, when an offset is given it is *always* an offset
within a named segment, and the segment is always one of these six. To convert
to a file offset:

```
file offset = segment start + offset
```

So segment 4036 offset `0x6763` is at file offset 86,304 + 26,467 = 112,771.
This one line is the key to every citation in Part III. Write it down.

---


## Chapter 3. What the compiler leaves behind

We did not find a disassembly or a map file. We have only the binary. But
compilers are creatures of habit, and a compiler leaves fingerprints. Learning
to recognise them is half of reading old code.

### The fingerprints of Turbo Pascal

Four habits give the compiler away, and all four are visible in the first
minute of looking.

**Strings are Pascal strings, not C strings.** A C string is bytes followed by
a zero. A Pascal string is a *length byte* followed by the bytes, with no
terminator — and that is not how the file names look. But the file name table
is plain ASCII with zero terminators, so those are string *constants* that the
compiler converted, not runtime strings. That distinction matters: it tells you
the name table is data, not a variable.

**Local variables live below the frame pointer, and the offsets are
negative.** A routine begins:

```
push bp
mov  bp, sp
sub  sp, 0x0a        ; room for ten bytes of locals
```

and then refers to locals as `[bp − 8]`, `[bp − 0x0a]`. Once you can read
`[bp − n]` as "a local variable" and `[bp + n]` as "a parameter", a function
stops being a wall of instructions and becomes a *shape*: how many locals, how
many parameters, how big the frame.

**Far calls to other units are `lcall seg, off`.** A near call (`call`) stays
inside a unit. A far call crosses. In a disassembly you will see both
`call 0x47a4` (same segment) and `lcall 0x2dff, 0x843f` (segment `0x2DFF`,
offset `0x843F`). The far ones tell you where the unit boundaries are and which
segment owns the routine being called.

**There is a runtime library, and it is at the bottom of segment 0.** Every
string comparison, every `Write`, every file operation is a call into the
compiler's runtime. In this program those calls cluster in the low offsets of
segments 0 and `0x2DFF`.

We can name a few of them, because we watched what they did:

```
lcall 0, 0xAA          read from a file     (handle, count, offset)
lcall 0, 0x5B          seek                 (handle, seg, offset, count)
lcall 0, 0x30          close a file
lcall 0, 0x3D2         fill a rectangle on the screen
lcall 0, 0x3CA         draw a sprite
lcall 0, 0x4B98        a random number
lcall 0x2DFF, 0x843F   the segment of a memory block
lcall 0x2DFF, 0x836C   allocate a memory block
lcall 0x2DFF, 0x387C   is this point on land?
```

That last list is not a guess. Each was identified by watching the arguments
pushed before the call and the result used after it. That is how you bootstrap a
dictionary of an unknown runtime: read the call sites.

### A worked example: the shape of a function

Here is a real routine, at segment 4036 offset `0x672E`, trimmed to its bones:

```
672e  push ax
672f  lea  ax, [bp - 0x104]     ; a 260-byte local buffer
6733  push ax
6734  mov  ax, si
6736  lea  bx, [0xC719]         ; a format string
673a  mov  dx, 0x51
673d  lcall 0x2DFF, 0x7B2F      ; format into the buffer
6742  mov  ax, 0x400            ; 1024
6745  push ax
6746  mov  ax, 0x12             ; 18
6749  push ax
674a  lea  ax, [bp - 0x104]
674e  push ax
674f  mov  ax, si
6751  mov  dx, [0x600]          ; a far pointer's segment
6755  sub  bx, bx
6757  lcall 0x2DFF, 0x798A      ; blit the formatted text
675c  push si
675d  lcall 0, 0x30             ; close a file
```

Read that and you can *describe* the routine: it takes a file handle in `si`,
formats a string into a stack buffer, draws it, and closes the file.

Notice `[bp − 0x104]`. That is 260 bytes. A round, slightly-too-big number for a
text buffer. Turbo Pascal's maximum string length is 255, and a Pascal string
needs one byte for the length, so 260 is "a bit more than the largest string
plus a little". Compilers do this constantly, and those padding numbers are
another fingerprint.

### The `sub sp` trick, and why it is not a trick

You will also see something like:

```
6b44  sub sp, 0x98
6b48  push ax
6b49  push di
6b4a  push si
```

The `sub sp` reserves 152 bytes. Then the pushes *also* move the stack
pointer. So the routine's frame grows in two stages. That is normal: the
compiler reserves the locals it knows about, then saves the registers it is
about to clobber.

The consequence for you is that `[bp − n]` may name either a local or a saved
register, and you distinguish them by watching whether the value is written
before it is read. A saved register is read *first* (in the epilogue); a local
is written first.


### Segmented data pointers

One more habit, because it appears everywhere and confuses everyone.

A pointer to data in another segment is stored as **two words**: an offset and
a segment. Turbo Pascal keeps them in consecutive globals. So when you see:

```
db04  mov ax, [0x5EE]
db07  mov dx, [0x5F0]
db0b  mov [bp - 0x0A], ax
db0e  mov es, dx
```

you are looking at a far pointer being loaded: `[0x5EE]` is the offset,
`[0x5F0]` is the segment, and `es` becomes the segment register so that
`es:[bx]` can address the block.

Once you recognise the pattern, a whole class of globals becomes readable: they
come in *pairs*, offset then segment, two bytes apart.

> **This is the highest-leverage skill in the whole book.** Learning to see a
> far pointer as a pair of globals — `0x5EE/0x5F0`, `0x5F6/0x5F8`,
> `0x602/0x604` — is what lets you find the data structures. In Part III the
> entire world-map investigation turns on recognising that `0x5EE/0x5F0` is
> "the world map", and it turns on that recognition because the pair is
> *allocated together* and *used together*.

### Exercise

Open the executable in Python and find the byte pattern `8B EC` (`mov bp, sp`)
— the start of almost every function. Count them. You will find several
hundred. That number is your map of the program's function count: not exact, but
close, and it gives you a feel for the density of code in each segment.

---


# Part II — The Discipline

## Chapter 4. How to know you are right

Here is the uncomfortable truth about reverse engineering: **most of your
hypotheses will be wrong, and wrong in a way that still looks right.**

This chapter is about not fooling yourself. It is the most important chapter in
the book, and it contains almost no code.

### The three kinds of evidence

Not all evidence is equal. Rank it, always.

**Tier 1 — the artefact itself.** The bytes in the file. The instruction the
disassembler prints. A mask in the code. This cannot lie, though it can be
misread.

**Tier 2 — behaviour.** What the program does when run. A sprite appears. A
value changes. Strong, but *indirect*: it tells you what a mechanism achieves,
not what it is.

**Tier 3 — someone else's claim.** A wiki, a forum post, a reimplementation on
GitHub. Weakest of all, and the most seductive, because it is already written
down and sounds confident.

The rule is simple and absolute:

> **Tier 3 is a pointer, never a proof. Tier 2 can suggest a structure. Only
> Tier 1 settles a dispute.**

### The oracle doctrine

An *oracle* is an independent source of truth about the same thing. If you can
find one you can test a hypothesis instead of arguing about it. We had three,
and it is worth seeing what each was good for.

**Oracle one: arithmetic closure.** If a hypothesis says a file holds 192
pictures of 1,920 bytes, the product must equal the file size. It nearly does:
192 × 1,920 = 368,640, and the real decompressed size is 368,448 — 192 bytes
short, which is exactly one byte per picture. The arithmetic told us we had the
right *idea* and the wrong *depth*. That is a real test, and it cost nothing.

**Oracle two: internal consistency.** If a table of sea-area winds is real,
adjacent sea areas should have similar winds, because wind is a physical field
and does not jump from north to south between two neighbouring cells. This is a
*statistical* oracle. It cannot prove a decode, but it can rank candidates. We
used it in Part III to choose between two ways of splitting a byte.

**Oracle three: the program's own output.** If you can make the program convert
its data into something you can see — a map, a screen, a printed value — you
have an oracle with very high resolution. We did not have this for the wind,
which is why the statistical oracle mattered.

### Falsification, not confirmation

The temptation, when you have a theory, is to look for evidence *for* it. Do
the opposite. Ask: **what would I see if this were false?**

Worked example. We believed the wind byte packed a direction in its low three
bits. The confirmation-seeking move is to check that the low bits are always
0–7; they are; done. Useless — that is true by construction of a mask.

The falsifying move: *if the direction really lived in the low three bits, and
the wind is a smooth field, then the low three bits should be smoother across
neighbouring cells than any other three-bit window.* So we measured every
window:

```
the table 0:  bits 0: 0.848   bits 1: 1.166   bits 2: 0.884
              bits 3: 0.680   bits 4: 0.549   bits 5: 0.574
```

Bits 4–6 scored *better* than bits 0–2. That contradicts the hypothesis, and it
is exactly the kind of contradiction you must chase rather than explain away.
(What resolved it is in Chapter 8; the point here is that the contradiction was
real and had to be faced.)


### The story of a wrong answer

This actually happened in this project, and it is worth telling properly,
because the failure mode is subtle.

**The claim.** The decoded `WORLDMAP.LZW` is a list of sixteen-bit numbers.
Reading them as `(row, column)` pairs produced a plausible-looking structure:
the list rose steadily, with runs of consecutive values. Conclusion: "the
numbers are the positions of the cells that differ from the tile before them;
the high byte is the row and the low byte is the column."

**The problem.** The numbers reached 65,534. If the high byte were a row, the
map would have 256 rows — but the executable says 45. And 65,535 appeared 48
times in one block, which is not what a list of positions looks like.

**The discovery.** Two of the three decompressed blocks were an **odd** number
of bytes long: 21,973 and 23,907. A list of sixteen-bit numbers cannot be odd.
The entire reading was built on a misalignment.

**The correction.** The claim was removed and replaced with what the data
actually shows: a sentinel, a table that breaks at a specific index, long runs
of `0x0F`. The notes now say the meaning is open.

**The moral.** The wrong claim was *plausible*. It explained the data at the
level of "these numbers go up". It survived a casual look, and it would have
survived a reader who trusted the author. It did not survive one question:
*does the total size divide cleanly into the record size I am proposing?*

> **Ask the divisibility question every single time.** If a file is 21,973
> bytes and your model says records of 2 bytes, you are wrong, and no amount of
> plausible output fixes that.

### The other failure mode: trusting a confident neighbour

The second remake of this game is on GitHub. It is serious work, and it
contains a file of wind and current data with exactly the right shape: 2,700
bytes, six tables of 450, one per direction and speed for summer, winter and
current. It looks authoritative.

It is not. Its own comments admit the direction is "guesswork through observing
the game". And when compared to the file in the release, no bit window,
rotation or scale produced it: the mean disagreement was about 1.9 compass
points out of 8, where the remake's own admitted ±1 jitter would allow about
0.2.

So the remake's data is a *reconstruction*: one person's best attempt to make
something plausible. It is genuinely useful for geometry — its sea-area formula
matches the executable exactly — and useless for data.

> **A reimplementation is evidence about the reimplementer, not about the
> original.** Use it to generate hypotheses. Never to settle them.

### A checklist you can reuse

Before you believe anything you have derived, run this:

1. **Does the arithmetic close?** File size ÷ record size, with no remainder.
2. **Is the endianness consistent everywhere?** An odd-sized array of 16-bit
   words is an impossibility that reveals a misalignment.
3. **Does it survive perturbation?** Change one assumption; does the result
   break or bend?
4. **Is there a second, independent route to the same answer?** A mask in the
   code and a statistical property of the data are two routes. Two routes that
   agree is strong. One route is a hypothesis.
5. **Can I state what would falsify it?** If not, you are not testing.


---

## Chapter 5. Reading a binary with a disassembler

Now the tools. Nothing exotic: a hex viewer, a disassembler, and Python to hold
it all together.

### The one property of disassembly you must understand

A disassembler is not an oracle. It is a *decoder*, and it will decode anything
you point it at, including data, producing confident nonsense.

What that looks like. Disassembling segment 0 from offset `0`:

```
0000  0000        add byte ptr [bx + si], al
0002  0000        add byte ptr [bx + si], al
```

Two instructions, then it walks off into data. Disassembling segment 7775 from
offset 0 gives 4,088 instructions before it derails; segment 4036 gives one.

The lesson:

> **A disassembler has no idea what is code. You have to tell it where to
> start.**

### The technique that works: pattern, then disassemble

The workflow that actually functions on a real binary:

1. **Search for a byte pattern you have a reason to expect.** Not a guess — a
   reason. If you know the code reads a file, search for the DOS interrupt that
   reads a file.
2. **Disassemble *at* the offset where the pattern was found**, not before it.
3. **Read forward and backward.** Forward tells you what happens next;
   backward tells you what set it up.
4. **Note the addresses you land on.** They become the anchors for the next
   search.

Where does this game open its files? Mostly through the compiler's runtime — but
the runtime itself must eventually touch the disk. So search for the classic DOS
`open`: `mov ah, 3Dh` within a few bytes of `int 21h`.

```
pattern:  B4 3D ... CD 21
hits:     one, at file offset 0x5625
```

One hit. That is the single place the program calls the DOS `open`. Everything
else goes through it. We now have a *hub*, and its callers are the interesting
ones.

Finding one hub collapses an enormous search space. The same technique found the
file-read hub, the file-close hub, the allocate-memory hub and the draw-sprite
hub — and those five together gave a skeleton of the whole engine in an
afternoon.

### Reading around a hit

The `open` call sits inside a routine. Read outward from it:

```
55f0  push si
55f5  push cs
55f6  call 0x52B4           ; a helper
55f9  cmp  byte ptr [bp - 1], 0x2B     ; '+'
55fd  je   0x560E
55ff  cmp  byte ptr [bp - 1], 0x2D     ; '-'
5603  je   0x560E
5605  cmp  byte ptr [bp - 1], 0x3D     ; '='
5609  je   0x560E
560b  jmp  0x56CB
```

Three comparisons against `+`, `-` and `=` before anything opens. That is not a
file name parser — it is an *argument parser*. This routine is the command-line
option handler, which is why it sits next to the `open` call: parse a flag, then
open a file.

Read outward and a routine tells you its purpose. Read inward and it tells you
its mechanics. Do both.

### The other trap: a clean disassembly of a data region

Some segments are mostly data with code sprinkled in. Segment 4036 has an array
at `0x87BB` that disassembles into perfectly valid, perfectly meaningless
instructions:

```
87bb  0000        add byte ptr [bx + si], al
87bd  80cc80      or ah, 0x80
```

It decoded. It is not code. The defence is to ask, of every suspicious region:
*does this make sense as a program?* A run of `add byte ptr [bx+si], al` is the
signature of zeros being decoded — the classic tell that you are looking at an
array of small numbers, not instructions.

That particular mistake cost real time in this project, and the tell was there
all along.

### A minimal toolkit

You do not need expensive software. What was actually used:

- **Python** for everything structural: reading the MZ header, computing the
  segment table, searching byte patterns, decoding records.
- **Capstone**, a disassembly library, for turning offsets into instructions.
  One line to set it up: `capstone.Cs(CS_ARCH_X86, CS_MODE_16)`.
- **A hex viewer** (`xxd`, or any editor) for eyeballing structure.
- **The program itself**, in an emulator (DOSBox), when behaviour must be
  confirmed.

That is the entire stack. The hard part is never the tools.

---


# Part III — Four Case Studies

Each of the following chapters is a complete piece of forensics: we start from
nothing and end with something verified. Read them as worked examples of the
method in Part II.

## Chapter 6. Finding the game's vocabulary

**Question:** what files does this program expect to find on disk?

This sounds trivial and is not, because a DOS program with no documentation
gives you no list. The directory tells you what shipped. It does not tell you
which files the program can *reach* — and some may be optional, or loaded only
in a late-game branch.

### Finding the strings

The first tool for any binary is a string search: find runs of printable ASCII.
In an English DOS binary you will find thousands, most of them uninteresting. So
search with a purpose: look for things that look like filenames.

```
search for:  a run of 4..14 printable characters containing '.'
```

Filtering by that gives a manageable list, and in this executable they are all
in one place: segment 11775, starting at offset `0x9224`.

```
00009220: 5a 57 00 00 43 3a 44 41 54 41 31 2e 4c 5a 57 00   'ZW\0\0C:DATA1.LZW\0'
00009230: 43 3a 4b 41 4f 2e 4c 5a 57 00 43 3a 50 4f 52 54   'C:KAO.LZW\0C:PORT'
00009240: 43 48 49 50 2e 4c 5a 57 00 00 43 3a 57 4f 52 4c   'CHIP.LZW\0\0C:WORL'
```

Split at the zeros and you have the manifest. Note the `C:` prefix: the
programmer assumed the game would run from a floppy with the data on drive C and
hard-coded the letter. Every name carries it.

The complete list, with segment-relative offsets:

```
0x9224 C:DATA1.LZW      0x928E C:EVENT.LZW
0x9230 C:KAO.LZW        0x929A C:IKKI.LZW
0x923A C:PORTCHIP.LZW   0x92A6 C:CHIP_NO.DAT
0x924A C:WORLDMAP.LZW   0x92B4 C:COLONY.DAT
0x925A C:ITEM.MES       0x92C2 C:EVENT*.DAT
0x9266 C:SNRDAT.LZW     0x92D0 C:GRAPH.DAT
0x9274 C:SNRMES.LZW     0x92DC C:KOUKAI2.DAT
0x9282 C:DATA2.LZW      0x92EA C:MONSTER.DAT
0x92F8 C:SNR*.DAT       0x932E C:IAE1.LZW
0x9304 C:TRANSIT.DAT    0x933A C:IAP*.LZW
0x9312 C:WINDCUR.DAT    0x9346 C:PORTMAP.LZW
0x9320 C:ZA_DAT.DAT     0x9354 C:SNR*.MES
0x9360 C:HDAT.PUT       0x9378 C:USERNAME.DAT
0x936C C:NAME.TBL       0x9388 C:D2.MML
```

Twenty-eight names, twenty-two of which shipped. That is a complete inventory of
the game's data vocabulary, obtained in ten minutes.

### What the list tells you before you open a file

**The wildcards are a loader mechanism.** `EVENT*.DAT`, `SNR*.DAT`, `IAP*.LZW`
and `SNR*.MES` are not names; they are patterns the program expands over the
directory. So the number of scenario files is not fixed in the code — add a
file and the program finds it. That is a design decision, and it tells you the
game was built to be extended: `EVENT0.DAT` … `EVENT6.DAT` are the seven
scenarios.

**The suffixes are shared.** Look at the name space around `0x9230`:

```
0x9230  C:KAO.LZW
0x9234          O.LZW          <- the tail of "KAO.LZW"
0x923A  C:PORTCHIP.LZW
0x923C          PORTCHIP.LZW
0x9240          CHIP.LZW       <- the tail of "PORTCHIP.LZW"
0x924A  C:WORLDMAP.LZW
```

Offsets `0x9234`, `0x923C` and `0x9240` point *inside* other strings. That is
not an accident: the code builds names by appending a suffix to a base, and the
suffix pointers are simply addresses a few bytes into an existing name. The
routine at segment 0 offset `0x4977` is a `sprintf`-alike that joins them.

Here is a real and reusable discovery: **you can tell that a string table is
used for name construction because some pointers land mid-string.** If every
reference pointed at the start of a name, the code would be loading names
directly. It is assembling them.

**The taxonomy is by content, not by loader.** `.LZW` holds compressed data
(pictures and tables); `.DAT` holds raw tables; `.MES` holds text; `.MML` holds
music. `.PUT` and `.TBL` are singletons with their own meanings.

### The name that gave the game away

One line is the reason the next three chapters exist:

```
0x9312  C:WINDCUR.DAT
```

Wind and current. In a game about sailing. We take that file apart in Chapter 8.


---

## Chapter 7. Case study: a container that is not what it says

**Question:** what is inside the `.LZW` files, and how do you get it out?

### The name is a lie

Every `.LZW` file begins with four ASCII bytes: `LS10` or `LS11`. Not "LZW".
The extension says one thing; the magic says another.

This is the first thing to check in any format investigation: **look at the
first four bytes.** Extensions are chosen by packagers; magics are written by
the code that made the file.

```
$ xxd -l 16 KAO.LZW
00000000: 4c53 3131 0000 0000 0000 0000 0000 0000  LS11............
```

`LS11`, then twelve zero bytes. That is a signature: a magic, some reserved
space, and then something at offset 16.

### Structuring the guess

From offset 16 the bytes stop being zero and become noise:

```
00000010: 00ff aa55 8001 027f 2a40 fea8 a00a 5415  ...U....*@....T.
00000020: 0550 3ffc 08bf fd20 1f04 10c0 f803 07e0  .P?.... ........
```

`00 FF AA 55 80 01 02 7F 2A 40 FE A8 ...`. A permutation of 0–255 has each value
once, and this could be one. That is testable:

```
is the sorted set of bytes at 16..272 exactly {0..255}?
```

If yes, it is a 256-byte *substitution table* — a permutation used to remap code
values. And 16 + 256 = 272, so the block table should start at 272.

### The block table

Offset 272 (`0x110`):

```
00000110: 0000 06ad 0000 0780 0000 0dbc 0000 064b  ...............K
00000120: 0000 0780 0000 1469 0000 0713 0000 0780  .......i........
```

Three big-endian 32-bit numbers, repeating:

```
06ad = 1709    0780 = 1920    0dbc = 3516
064b = 1611    0780 = 1920    1469 = 5225
0713 = 1811    0780 = 1920    1ab4 = 6836
```

Read as (compressed size, uncompressed size, offset):

- block 0: 1,709 → 1,920 bytes, at offset 3,516
- block 1: 1,611 → 1,920 bytes, at offset 5,225
- block 2: 1,811 → 1,920 bytes, at offset 6,836

Check the arithmetic: 3,516 + 1,709 = 5,225 ✓. 5,225 + 1,611 = 6,836 ✓. The
blocks tile the file exactly. **The structure closes.** That is our first
confident result of the chapter.

And 1,920 is the number from Chapter 1: 64 × 80 × 3 ÷ 8 = 1,920. Every block is
one portrait — 64 pixels wide, 80 tall, three bit-planes deep.

### What "LS11" compression actually is

The payload of a compressed block is a stream of variable-length codes, and each
code either names a byte from the substitution table or points backwards into
the output already produced.

That is *dictionary-free LZ77 with a code-length trick*, not LZW. The name on the
file is wrong, and the community name ("the KOEI LZW variant") is also wrong. The
tell is the shape of the code word:

> A run of one-bits, then a zero, then as many bits as the run was long.

Why design such a thing in 1993? Because it is cheap to decode. Reading a
variable-length prefix code of this shape needs no table and no lookup — it is a
loop of shifts:

```
run = 0
do:
    bit = next_bit()
    run = (run << 1) | bit
until bit == 0
factor = read_bits(number_of_bits_read_so_far)
code = run + factor
```

The length of the run *is* the number of bits in the factor, so the code is
self-delimiting. A decoder needs about twelve instructions. On a machine where
every byte of code is precious, that mattered.

A code below 256 indexes the substitution table. A code of 256 or more is a back
reference: offset `code − 256`, and the length is a *second* code plus three.

> **Design note.** That "+3" is the giveaway that the author was thinking about
> compression ratio, not decode speed. A two-byte match could never pay for its
> own code, so the format simply forbids them. Encoders love this trick; it is
> the same reason DEFLATE has a minimum match length of three.

### The bug that would have cost days

**Every block is independent.** Each has its own bitstream and its own output
buffer, and a back reference points only into that block's own output.

Why does that matter? Because the blocks *tile the payload byte for byte* — they
sit end to end with no gaps — and it is enormously tempting to read them as one
continuous stream. Do that and the back references point into the wrong windows,
the output is garbage, and the format looks broken. It is not broken; you are
reading it wrong.

> **When a decoder produces garbage, suspect your framing before you suspect the
> codec.** Chunk boundaries that tile perfectly are not the same as chunk
> boundaries that continue each other.

### Validation

How do we know the decoder is right? Arithmetic, again:

```
all 675 blocks of the fourteen .LZW files decode to exactly
the size their record asks for
```

No block overruns. No block underruns. 675 independent chances to be wrong, and
none taken. Then the second check: all 128 portraits of `KAO.LZW` come out
pixel-for-pixel identical to the images extracted by an independent viewer
(`gcjjyy/koei_viewer`). Two independent implementations agreeing on
128 × 8,192 pixels is about as strong as evidence gets in this field.

### The lesson

> **The container is generic; the content is not.** The same codec carries the
> portraits, the town tiles, the town maps and the world map. Once the container
> was cracked, the remaining work was entirely about *what the decompressed
> bytes mean* — a different and harder problem. This is the normal shape of a
> reverse-engineering project: a short, decisive victory on the container, then
> a long campaign on the semantics.

---


## Chapter 8. Case study: the wind, or how a mystery dissolves

This is the centrepiece of the book: a complete investigation of one
1,350-byte file — from a confident community belief that it was encrypted, to a
verified decode that fits in a dozen lines of code.

### The belief

The project's own notes said:

> `WINDCUR.DAT` — the wind and current table. The bytes are obfuscated and the
> key is not recoverable from the data alone.

That belief was sincere, and it was wrong. Hold on to how plausible it sounded;
the lesson at the end of this chapter is about that plausibility.

### Step one: look at the file

```
$ ls -l WINDCUR.DAT
-rw-rw-r-- 1 peter peter 1350  Jul 15  1995 WINDCUR.DAT

$ xxd -l 48 WINDCUR.DAT
00000000: 292b 2b2b 2b2a 2222 2222 2a2a 2a2a 2222  )++++*""""****""
00000010: 2222 0a0a 0909 0909 090a 0c0b 0909 092a  "".............*
```

No header. No magic. 1,350 bytes of small values, ranging roughly 0x00–0xF2,
with heavy concentration in 0x09, 0x0A, 0x22, 0x2A, 0x2B.

Now the first piece of forensic arithmetic: **1,350 = 3 × 450**. Three tables of
450 entries is a strong shape, because 450 is itself suspicious — it will turn
out to be 30 × 15, and the world map is divided into exactly 30 columns and 15
rows of sea areas.

But a shape is not a proof. Hold it as a hypothesis and go find the code.

### Step two: find the loader

We want the routine that reads this file. Searching for the string offset
`0x9312` ("C:WINDCUR.DAT") finds nothing, because names are assembled from base
and suffix (Chapter 6). So search on *behaviour* instead: a routine that reads
450 bytes, three times.

Here is what turned up, at segment 4036 offset `0x6763`:

```
6763  lea  bx, [0xC52]         ; the name buffer
6768  push cs
6769  call 0x5F6E              ; open  -> si = handle
676e  cmp  byte ptr [0x735], 3 ; the month
6773  jb   0x6786
6775  cmp  byte ptr [0x735], 9
677a  jae  0x6786
677c  sub  ax, ax              ; summer: read nothing here
...
6786  mov  word ptr [bp - 4], 0x1C2   ; 450
678b  mov  word ptr [bp - 2], 0       ; offset 0
6790  sub  ax, ax
6792  push ax                  ; 0
6793  push word ptr [bp - 2]   ; the offset in the file
6796  push word ptr [bp - 4]   ; the count
6799  push si                  ; the handle
679a  lcall 0, 0xAA            ; seek
67a2  mov  ax, 0x1C2           ; 450
67a6  push word ptr [0x602]    ; a buffer
67aa  push word ptr [0x604]
67ae  push si
67af  lcall 0, 0x5B            ; read 450 bytes
67b7  mov  ax, 0x1C2           ; 450
67c1  lcall 0, 0xAA            ; seek to 900
67c9  mov  ax, 0x1C2
67cd  push word ptr [0x606]    ; a second buffer
67d1  push word ptr [0x608]
67d6  lcall 0, 0x5B            ; read 450 more
```

Read that carefully. It settles three things at once.

**It is not obfuscated.** There is no XOR, no rotation, no key. The code seeks to
offset 0, reads 450 bytes, seeks to 900, reads 450 bytes. It is a plain array
read. Whatever made the file look encrypted was not encryption.

**The three tables are at 0, 450 and 900.** The routine reads two directly; the
offsets (0, then 900) and the 450-byte chunk imply the third.

**The month selects a season.** `[0x735]` is compared against 3 and 9: months 3
through 8 take one branch, everything else the other. A designer would call that
summer and winter.

> **Forensic note.** Notice how much the *shape* of this routine told us. We found
> it by searching for "three reads of 450 bytes", not by finding the file name.
> Searching on behaviour rather than text is a general and powerful technique, and
> it works even when the program never mentions the thing you are looking for.


### Step three: find the byte layout

Now the hard part: what does a byte mean?

Search the code for a *consumer* of the wind. A routine at segment 4036 offset
`0x7AC6` blends the wind at the corners of a ship's hull. Inside it:

```
7b01  mov al, byte ptr [di + 0x13]    ; the wind byte of a sea area
7b04  and ax, 7                       ; the direction
7b07  mov word ptr [bp - 2], ax
7b0a  mov cx, word ptr [si + 4]
7b16  mov cl, 3
7b18  shl bx, cl                      ; direction * 8 -> an index
```

There it is: **`and ax, 7`.** The low three bits are a direction, 0–7. It is then
multiplied by 8 to index a table of the eight compass unit vectors, which
confirms the eight directions.

Now find the *writer*, which shows what the other five bits are. At segment 4036
offset `0x9703`:

```
9703  mov al, byte ptr [bx + 0x13]
9706  and al, 0xF8                    ; keep everything above the low three bits
9708  or  al, byte ptr [bp + 6]       ; replace the low three bits
970b  mov byte ptr [bx + 0x13], al
```

`and al, 0xF8` then `or` in a new value. So bits 0–2 are the direction, and bits
3–7 are something that must be *preserved* when the direction changes.

And a few instructions later:

```
971f  or byte ptr [bx + 0x13], 8      ; set bit 3
```

Bit 3 is set on its own, as a flag. **So bit 3 is not part of the strength.** That
leaves bits 4–7 — a clean four-bit field — as the strength, ranging 0–15.

### The layout

```
bits 0-2   direction, 0-7, clockwise from north
bit  3     flag: a strong wind
bits 4-7   strength, 0-15
```

Three independent pieces of code agree: the reader masks with 7, the writer masks
with `0xF8`, and the flag routine touches bit 3 alone. Tier-1 evidence, three
directions at once.

### The contradiction, and how it was resolved

Recall from Chapter 4 that the statistical test on the *file* preferred bits 4–6
over bits 0–2 as a "direction". That looked like a contradiction. It was not; it
was a warning that the test was measuring the wrong thing.

Here is the resolution. A byte holds a direction (bits 0–2) and a strength (bits
4–7). The *strength* field is smoother across neighbouring sea areas than the
*direction* field, because wind strength varies gently while wind direction can
swing. A test that asks "which three-bit window looks smoothest?" will therefore
prefer a window that straddles the strength field.

The mistake was assuming that "smoothest" implies "direction". It does not. The
test was sound; the *interpretation* was wrong.

> **Lesson.** A statistical measure does not know what it is measuring. Use it to
> *rank* hypotheses you have already grounded in the code, never to *invent* them.
> When a statistic disagrees with the code, believe the code, and then find out
> what the statistic was actually seeing.

### The decode, verified

The beginning of the file again:

```
00000000: 29 2b 2b 2b 2b 2a 22 22 22 22 2a 2a 2a 2a 22 22 22 22 0a 0a 09 09 09 09 09 0a 0c 0b 09 09 09 2a
```

Decode the first few:

```
0x29 = 0010 1001   direction 1   strength 2
0x2B = 0010 1011   direction 3   strength 2
0x2B =             direction 3   strength 2
0x2B =             direction 3   strength 2
0x22 = 0010 0010   direction 2   strength 2
0x0A = 0000 1010   direction 2   strength 0
0x09 = 0000 1001   direction 1   strength 0
```

And the first bytes of the *third* table, at offset 900:

```
00000384: 20 22 2a 32 11 11 10 00 00 00 10 10 10 10 10 00
```

```
0x20 = 0010 0000   direction 0   strength 2
0x22 =             direction 2   strength 2
0x2A =             direction 2   strength 2
0x32 = 0011 0010   direction 2   strength 3
0x11 = 0001 0001   direction 1   strength 1
0x00 =             direction 0   strength 0
```

Every value is a legal direction and a legal strength — no exceptions across all
1,350 bytes. The strengths cluster in 0–3 with occasional spikes to 12 and above,
which is exactly how you would model an ocean: mostly gentle, with storms in
patches. And the first table's directions differ visibly from the third table's,
because the wind and the current are different fields. **The data behaves like an
ocean.** That is a second, independent confirmation.

### What was shipped

The importer writes:

```
column,row,summer_direction,summer_strength,winter_direction,winter_strength,current_direction,current_strength
0,0,1,2,1,2,0,2
1,0,3,2,2,2,2,2
```

450 rows, eight columns, generated from the release file. The simulation reads
the row for the season the calendar is in, turns the direction into a bearing
(×45°), scales the strength onto the sailing model's 0–7 scale, and applies it.

### The lesson, stated plainly

> **"Obfuscated" is usually a hypothesis, not an observation.** A file of small
> numbers with no header looks like ciphertext because it has no structure you
> recognise. This one had no header because it needed none, and no structure
> because it was exactly what it appeared to be: an array.

The community note was not stupid. It was *unexamined*. Someone looked at 1,350
bytes of small values, could not see an obvious meaning, and wrote down
"obfuscated". That note then deterred everyone who read it — including, for a
while, this project.

> **Write down what you observed, not what you concluded.** "The bytes range
> 0x00–0xF2 with heavy clustering at 0x09, 0x0A, 0x22, 0x2A, 0x2B" is
> observation. "The bytes are obfuscated" is a conclusion — and being wrong, it
> closed a door for two years.

---


## Chapter 9. Case study: the world, from arithmetic alone

**Question:** how big is the world, and how is it divided?

This chapter is about a technique that feels like cheating: reading a program's
*memory allocations*. Note what we are not doing — tracing pixels, or watching a
map scroll. We are reading constants out of instructions. It is faster, it is
certain, and it teaches you how the designers thought.

### Finding the size of the world

Search the code for instructions that load plausible map dimensions. In 16-bit
code a constant arrives as a two-byte immediate, so searching for the
little-endian encoding of 2160 (`0x0870`) finds every place it appears.

Two hits matter.

At segment 4036 offset `0xDC82`:

```
dc82  mov cx, 0x870        ; 2160
```

At segment 4036 offset `0xA399`:

```
a399  cmp ax, 0x438        ; 1080
a39c  jl  0xA3AF
a3a1  cmp word ptr [bx], di
a3a3  jle 0xA3AB
a3a5  add di, 0x870        ; 2160
a3a9  jmp 0xA3AF
a3ab  sub di, 0x870        ; 2160
```

That second snippet is beautiful. It compares a distance against 1080, and then,
depending on which side of the world you are on, it *adds or subtracts 2160* from
a position.

> **That is a wrap-around.** The world is a cylinder: sail off the west edge and
> you arrive at the east. And the code implements it exactly as you would — by
> moving the position one full width.

So the map is 2160 wide and 1080 tall. Two constants, two contexts, one
conclusion. That is the strongest kind of evidence: the numbers appear where their
meaning is unambiguous.

### The technique that unlocked the grid

Inside the map-drawing code there is a sequence of memory allocations, at segment
4036 offset `0x72C3`:

```
72c3  mov ax, [0xE14]
72c6  lcall 0x2DFF, 0x843F    ; the segment of a block
72cb  mov [0x5F0], ax         ; -> the segment of the world map
72ce  mov ax, 0x400           ; 0x400 paragraphs
72d4  lcall 0x2DFF, 0x836C    ; allocate
72d9  mov [0xE0E], ax
72e8  mov ax, 0x240           ; 0x240 paragraphs
72ee  lcall 0x2DFF, 0x836C    ; allocate
72f3  mov [0xE16], ax
730a  mov [0x5F8], ax         ; -> the wind records
730d  mov ax, 0x700           ; 0x700 paragraphs
7313  lcall 0x2DFF, 0x836C    ; allocate
7318  mov [0xE18], ax
```

Three things to notice.

**The unit is paragraphs.** `0x400` is not 1,024 bytes; it is 1,024 *paragraphs*,
and a paragraph is sixteen bytes. So `0x400` = **16,384 bytes**. Getting that
wrong by a factor of sixteen is a classic mistake, and it is why the far-pointer
architecture of Chapter 2 matters even when you are reading a simple constant.

**Each block is immediately assigned to a far pointer.** `[0x5F0]` is the
*segment* half of the map pointer; its offset half is `[0x5EE]`. `[0x5F8]` is the
segment of the wind records, whose offset half is `[0x5F6]`. We can now name these
globals:

```
[0x5EE]/[0x5F0]  the world map          16,384 bytes
[0x5F6]/[0x5F8]  the sea-area records    9,216 bytes
```

**The size alone constrains the format.** A 9,216-byte block for 450 sea areas
gives 20.48 bytes each. Twenty is the obvious fit — 450 × 20 = 9,000, leaving 216
spare. And twenty bytes means the record spans `0x00`–`0x13`, so the wind byte at
`0x13` is the record's **last byte**. That is a small, independent detail from
Chapter 8, and it *agrees* with it.

> **Forensic note.** This is the most productive thing you can look for in an
> unknown program: its allocation calls. They appear together, near start-up, with
> size constants, and each is attached to a named global. In an afternoon you get a
> table of the program's major data structures and their sizes, and those sizes
> constrain every format you will later decode.


### The grid falls out of a division

Now the map's row index. At segment 4036 offset `0xDD25`:

```
dd25  mov bx, [0xC76E]
dd29  mov ax, word ptr [bx + 2]   ; the Y position of the fleet
dd2c  mov cx, 0x18               ; 24
dd30  idiv cx                    ; Y / 24
dd32  mov si, 0x5A               ; 90
dd35  imul si                    ; (Y / 24) * 90
dd37  mov dx, ax
dd39  mov al, byte ptr [0x8D7]
dd3e  mov si, dx
```

**`(Y / 24) * 90`.** That is a row index into a grid 90 columns wide, where each
row is 24 pixels tall.

Check it against the known dimensions:

```
2160 / 90 = 24        1080 / 24 = 45
```

So the world is a **90 × 45 grid of 24 × 24 pixel tiles**. Not 2160 × 1080
individual cells — a grid of 4,050 tiles, each drawn as a 24-pixel square.

Everything clicks into place at once:

```
90 * 45   = 4,050 tiles
4050 * 4  = 16,200 bytes      <= the 16,384-byte map block
45 rows * 12 bytes = 540      <= the land bitmap, 96 bits per row, 90 used
```

The map block and the land bitmap are the *same grid*. We did not go looking for
that agreement; it emerged, and that is what makes the result trustworthy.

### The land bitmap, and why it is 540 bytes

At segment 4036 offset `0x67EA` a routine clears a bitmap:

```
67ea  push di
67eb  push si
67ec  sub dx, dx
67ee  mov bx, 0xF6C            ; the bitmap
67f1  jmp 0x67F9
67f3  inc bx
67f4  mov byte ptr [bx - 1], 0
67f8  inc dx
67f9  cmp dx, 0x21C            ; 540
67fd  jl  0x67F3
```

Then it sets bits in a rectangle. The bitmap is 540 bytes. Why?

```
540 = 45 rows × 12 bytes
12 bytes = 96 bits, of which 90 are used
```

Forty-five rows of ninety columns, with six bits of padding per row so each row
starts on a byte boundary. That padding is not waste; it is *arithmetic
convenience*. Shifting by 96 bits is easier to arrange than shifting by 90.

> **Design note.** That is a design smell in the good sense: the author traded six
> bits of memory per row for simpler indexing code. On a machine with 64 kilobytes
> of working space, six bits is nothing, and simplifying the inner loop is
> everything. You will see this trade over and over in nineties code.

### Still open: what the map file actually says

We know the world's *shape*: 90 × 45 tiles, 24 pixels each, 4,050 cells, a
16,200-byte footprint. We do **not** yet know what the decompressed
`WORLDMAP.LZW` means.

What the data does show, honestly:

- It decompresses to 71,274 bytes in three blocks of 25,394, 21,973 and 23,907.
- Two of those sizes are **odd**, so the data is not an array of sixteen-bit
  words, whatever else it is.
- In each block, a run of ascending sixteen-bit values breaks at exactly word
  1351 — byte 2702 — in all three blocks. After the break come a short header and
  long runs of the byte `0x0F`.
- 65,535 appears as a value 48, 32 and 18 times, which is the shape of a sentinel.

What we do not know is how those numbers map onto the grid. The 16,200-byte figure
is now an *oracle* for the next attempt.

> **Lesson.** It is entirely normal to close a case study with the shape solved and
> the content open. Write down what you know in a form precise enough that the next
> person can test a hypothesis in five minutes, then stop. The worst outcome is not
> "unfinished"; it is "finished and wrong", which is what Chapter 4 is about.

---


## Chapter 10. Case study: a language hiding in the sound

**Question:** how does the game store music, and why is that a lesson in design?

### The file that is not sheet music

`D2.MML` holds the music. The extension says MML — Music Macro Language, a
notation that goes back to the 1980s BASICs, where you write `C4 D4 E4 F2` and a
chip plays it.

It is not ASCII. It is a binary command language, and the way to read it is to
look at the *driver* — the program that interprets it. The driver is in the
executable, and it is the definition of the format (Tier 1 again).

### The structure in one paragraph

The file begins with a table of sixteen-bit offsets to tracks. A zero ends the
tracks of a song, so the table is a list of songs and a song is a list of tracks.

Every command is one byte, and *the byte's value decides how many bytes follow*:

```
0x00 – 0x5F    a command with two parameters
0x60 – 0xBF    a note
0xC0 – 0xFF    a command with one parameter
```

That single rule is the whole format. It is the same trick the compression
format used in Chapter 7: encode the *length* in the *opcode*, so the decoder
needs no table. It is a design idiom of the era, and once you have seen it twice
you will start seeing it everywhere.

### The notes are not frequencies

Here is the interesting part, and the part that makes this a lesson in design
rather than just a format.

A note byte carries a pitch in its lower six bits. The driver takes the byte,
subtracts `0x60`, and adds two transpositions: one for the channel and one for
the driver itself. The result is a number of semitones.

Why subtract and add rather than store a frequency? Because the hardware does not
want a frequency. The YM3812 chip is programmed with an *F-number* — a register
value — plus an octave and a key-scale. The driver's note table is literally the
chip's register values for the twelve semitones of an octave:

```
343, 363, 385, 408, 432, 458, 485, 514, 544, 577, 611, 647
```

Look at the numbers. They are not evenly spaced, because the
equal-tempered scale is logarithmic and the chip's phase accumulator is linear.
The ratio between neighbours alternates around 1.0595 (the twelfth root of two).

> **Design note.** The author did not compute these. They were *tabulated*, once,
> by whoever wrote the first KOEI sound driver, and then reused for years. That is
> why the same twelve numbers appear in game after game: they are a piece of
> institutional knowledge, frozen into a table.

### Why the transposition exists

Two transpositions — one per channel, one global — look redundant. They are not;
they are a *workflow* feature.

A composer writing for eight channels will want to move a melody up a third
without rewriting it, and will want to shift the whole piece into a comfortable
key without touching the individual tracks. Two transposition layers give exactly
those two operations, and they cost two bytes each.

> **Design note.** That is a small thing that tells you a lot. Somebody at KOEI
> thought about the person who would *use* this format, not just the person who
> would write the driver. Formats designed for their users survive; formats
> designed only for their writers get replaced.

### The game's music and its effects share a language

The sound effects live in banks whose magic is `SNDT`. A bank begins with the
magic, the bank number, and a table of offsets. The effects themselves are written
in *the same command language as the music*, and end with the two bytes `0xF8
0xF2`, which the driver reads as a pointer back into the stream.

So one parser reads both music and effects. That is not laziness; it is
architecture. A single interpreter, a single command set, two content files.

> **Lesson.** When two subsystems share a format, they share a *debugger*, a
> *documentation burden*, and a *mental model*. Designers who unify formats are
> buying simplicity with expressiveness, and in 1993 that was almost always the
> right trade.

### The honest limit

This project does not reproduce the original's exact timbre, and it is worth
being clear about why. The YM3812 is not a synthesiser in the modern sense; it is
a specific FM chip with two operators per channel, four waveforms, a feedback
path, and an envelope generator with its own quirks. Reproducing it exactly means
*emulating it*, cycle for cycle, which is a project in itself.

What the project does instead is synthesise the same notes, with the same
durations, the same volumes and the same transpositions, through a modern
synthesiser. The *music* is faithful. The *voice* is an interpretation.

> **That distinction — faithful structure, approximate surface — is the honest
> position for most reimplementation work.** Saying so explicitly is better than
> claiming an authenticity you have not achieved.

---


# Part IV — What the Design Teaches

We have spent nine chapters on machinery. This part asks the question the
machinery exists to answer: *why does this game feel the way it does?*

## Chapter 11. Quantisation as a design language

The single most important idea in this game's design is that **everything is
quantised**, and the quantisation is coarse on purpose.

The wind is not a vector. It is eight directions and a strength from 0 to 15. The
sea is not water; it is 450 rectangles, each with one wind and one current. The
world is not a landscape; it is 4,050 tiles of 24 pixels. The land is a bitmap of
45 rows of 90 bits.

A modern designer looking at that would see loss. The original designers saw
*gain*, and they were right, for three reasons.

### Reason one: quantisation makes information legible

Consider what the player needs to know when deciding whether to sail. They need to
know roughly which way the wind blows, roughly how hard, and roughly how far they
can get today.

A continuous wind field gives them a number they cannot see and cannot use. Eight
directions and a strength give them something they can *see on the chart* — an
arrow, of a certain size. The quantisation is not a limitation of the simulation;
it is the *interface*.

> **Design principle.** When a system's output must be read by a human, quantise
> it to the resolution the human can actually perceive. Extra precision is not
> extra fidelity; it is noise the player must filter.

### Reason two: quantisation creates decisions

Here is the clever part, and where the sailing model earns its keep.

A ship's drive depends on the number of compass octants between its heading and
the wind — an integer from 0 to 4. The drive for a given sail type is a small
table:

| sail type | running | | | | dead to windward |
|---|---|---|---|---|---|
| 1 (square) | 10 | 10 | 8 | 6 | 4 |
| 2 | 8 | 9 | 9 | 7 | 6 |
| 3 | 10 | 10 | 10 | 8 | 6 |
| 4 | 9 | 10 | 10 | 9 | 7 |
| 5 | 9 | 10 | 10 | 9 | 7 |
| 6 (lateen) | 8 | 10 | 10 | 10 | 8 |

Read the first row against the last. A square rig keeps four tenths of its drive
dead to windward; a lateen rig keeps eight. That single difference is the entire
reason the Mediterranean matters in this game: the ships that can beat to
windward open a different map.

And the table is not dominated by one entry. The square rig is best dead downwind
(10 versus 8); the lateen is best to windward (8 versus 4). No sail type is best
everywhere. That is a real choice with a real cost, expressed in eighteen small
integers.

> **Design principle.** A good balance table has no dominant strategy and no
> useless option. You can check both by eye, which is why tables beat formulas in
> design work.

### Reason three: quantisation makes the world comprehensible

The map being 90 × 45 tiles means the world has 4,050 *places*. Not infinitely
many. That is a number a player can develop a mental model of — "the Caribbean is
about twenty tiles across" — and a number a designer can reason about.

Compare a modern open world with a continuous heightfield. Its designers cannot
tell you how many places it has, because the question is meaningless. Here the
question has an answer, and the answer fits in a designer's head.

> **Design principle.** A world the designer can hold in mind is a world whose
> pacing the designer can control.

### The cost, honestly

Quantisation is not free. The sailing model has a documented absurdity: **a
stronger headwind makes a ship faster.** The wind speed multiplies the drive
regardless of angle, so a stiff contrary wind beats a light one. That is not a
modelling choice; it is an oversight. And a second: a rowed hull still cares about
wind *direction* when the wind is calm.

The project this book accompanies reproduces both quirks rather than fixing them.

> **Design principle.** When reimplementing a classic, distinguish *identity* from
> *defect*. The headwind quirk is a defect, but players built strategies around it.
> Fixing it silently changes the game. Fixing it *optionally*, and saying so, is
> the honest move.


---

## Chapter 12. The economy, and the shape of a voyage

The wind is the physics. The economy is the reason to move.

### The price model in outline

Each port has a base price for each commodity, and each commodity has a base
value, a floor, a ceiling, a cycle period and amplitude, and a seasonal component
with its own phase. Prices wander within those bounds. A port has a *specialty* —
the one thing it makes cheap — and a discount for it.

That is a remarkably small model. It fits in a CSV. And it produces the whole of
the game's trading behaviour:

- Buy where a thing is made, sell where it is not. The specialty discount *is* the
  "where is it made" signal.
- Prices revert, so a route cannot be run forever without the margin decaying.
- Seasons move prices, so the calendar is an economic variable, not decoration.
- Perishable goods spoil, so the fastest route and the most valuable cargo are in
  tension.

> **Design principle.** A trading game needs four numbers per good — base, floor,
> ceiling, and how fast it returns — and one per port, the specialty. Everything
> else is presentation. Complexity beyond that is usually the designer amusing
> themselves.

### Why the voyage has weight

Here is what this game does that most trading games do not: **it makes the journey
expensive in time, and time is the scarcest resource.**

The wind means a voyage has a duration you cannot fully control. The sea areas
mean the wind changes as you cross them. The seasons mean the trade winds you
learned in summer are not the ones you get in winter. And the economy's cycles are
keyed to days, so by the time you arrive, the price has moved.

The result is that a trade route is a *plan*, not a *transaction*, and a plan can
go wrong. That is where the drama is.

> **Design principle.** If you want a journey to feel significant, make it cost a
> resource the player cannot buy more of. Here it is days.

### The port as a place

The port screens — exchange, shipyard, tavern, governor, church — are the game's
social layer. And the data shows how much of the design lives there: each port
carries a level for the shipyard, the tavern, the governor, the church and the
inn, plus a tax rate and a ship discount.

Those are five small integers per port, and they do an enormous amount of work.
They make Lisbon feel different from Istanbul before you have bought anything —
and the *sameness* of the great capitals, where every level is 5, is itself
information: the interesting ports are the ones where one of those numbers is low.

> **Design principle.** Character is often a *profile* of small numbers rather than
> a large amount of bespoke content. Five integers per location, arranged
> thoughtfully, reads as a personality.

---

## Chapter 13. Three scales of abstraction

The most instructive thing about this game's architecture is that it uses **three
different grids at three different scales**, and the boundaries between them are
where the design lives.

**Scale one: the tile.** 24 × 24 pixels. The world is 4,050 of them. This is the
scale at which the world is *drawn*.

**Scale two: the sea area.** 72 × 72 tiles — three tiles on a side. The world is
450 of them. This is the scale at which the world *behaves*: one wind, one
current, one strength per area.

**Scale three: the port.** A single point, with a name and a profile. This is the
scale at which the world is *narrated*.

Now notice the ratios. A sea area is 3 × 3 tiles — nine tiles to one behaviour.
The wind is constant across a nine-tile patch, which is exactly coarse enough that
the player perceives a boundary without feeling a grid.

And notice what is *absent*: there is no scale between the port and the sea area. A
port is a point, not a region. That is a deliberate simplification with a
consequence — ports cannot be fought over spatially, only entered. The game's
conflict is therefore about *routes*, not *territory*, which is the correct choice
for a game about trade.

> **Design lesson.** Choose abstraction scales so that the *behaviour* grid is
> coarser than the *visual* grid by a small integer factor. Nine to one is
> comfortable. Two to one feels jittery; a hundred to one feels like a board game.
> And do not add a scale you do not need: the missing port-region is a feature.

### The half lesson: the wrap-around

The world is a cylinder. Sail west far enough and you arrive in the east.

Implementation: `add di, 2160` or `sub di, 2160`. Four bytes of code.

Design value: enormous. The map has no edge, so there is no "corner of the world"
to hide in, and the age-of-exploration fantasy — that the ocean continues — is
preserved. The cheapest possible trick, bought for four bytes.

> **Design lesson.** When a fiction demands an unbounded space and your hardware
> gives you a bounded one, *wrap* rather than *wall*. Players forgive a seam they
> cannot see; they do not forgive an invisible wall.

---


# Part V — Practice

## Chapter 14. Your first reversal, as a worksheet

Theory is cheap. Here is a sequence of exercises, in order of difficulty, that
takes you from "I have a file" to "I have a decoded format". Each has a
deliverable, so you know when you are done.

### Exercise 0 — Confirm your tools (30 minutes)

Take any DOS executable you own. Parse its MZ header with ten lines of Python.
Print the header fields. Verify that the file size matches
`(e_cp − 1) × 512 + e_cblp`.

**Deliverable:** a script that prints the header, and a sentence saying whether
the arithmetic closed.

If it did not close, the file is either corrupt, or has an overlay, or is not what
you think it is. All three are worth knowing.

### Exercise 1 — Find the segments (1 hour)

Compute the segment table from the relocation table, as in Chapter 2. Print one
row of `segment, start, size, relocation count` per segment.

**Deliverable:** the table, and a guess at which segments are code and which are
data, with your reasoning. Hint: count relocations per kilobyte — code has many,
data has few.

### Exercise 2 — Find one hub (1 hour)

Search for a byte pattern you have a *reason* to expect — a DOS interrupt, a string
you know exists, a known instruction sequence. Find one routine that everything
calls.

**Deliverable:** the offset, the first ten instructions, and a sentence describing
what the routine does. Do not guess from a name; it has none. Work it out from the
instructions and the arguments.

### Exercise 3 — Decode a header (half a day)

Pick a data file. Determine:

- Is there a magic? What are the first four bytes?
- Is there a length field? Does it agree with the file size?
- Is there an offset table? Do the offsets close — does each offset plus its
  length equal the next offset?

**Deliverable:** a written format description, with the arithmetic that confirms
it.

### Exercise 4 — Decode a record (a day)

Take the decompressed content of a file with a known record count. Work out the
record size by divisibility. Then determine what each field means by finding the
code that reads it.

**Deliverable:** a table of fields, with, for each, the line of assembly that
proves it.

This exercise teaches the most, and it is where you will make the mistake from
Chapter 4. When you do, notice it, and write down what told you.

### Exercise 5 — Falsify yourself (a day)

Take your decode from Exercise 4. Write down three predictions it makes about data
you have not looked at. Then check them.

**Deliverable:** three predictions and their outcomes. If any fail, your decode is
wrong or incomplete, and you now know where.

> **This exercise is the whole discipline.** Everything before it is preparation.

---

## Chapter 15. Ethics, law, and the clean room

This book has shown you how to take apart a commercial game. That deserves a frank
discussion, because "how" and "may I" are different questions.

### What this project actually did

Three rules, and they are worth adopting.

**One: the original files never enter the repository.** The importer reads them
from a directory the user supplies. Nothing from the release is committed. Clone
the project and you get code and derived data, not the game.

**Two: the derived data is data, not code.** A table of 450 wind directions is a
fact about the world, not an expression of anyone's creativity. Facts are not
copyrightable in most jurisdictions; the *selection and arrangement* of a creative
database can be, which is why the derived files are structured by the project's own
needs rather than copied wholesale.

**Three: the runtime never reads the release.** The game reads `wind_current.csv`,
not `WINDCUR.DAT`. That is not only a legal precaution; it is good engineering,
because the game then runs without the original present.

### What would not be acceptable

- Shipping the original data files, or verbatim extractions of the art.
- Shipping a disassembly of the original executable.
- Presenting a reimplementation as the original, or as licensed.
- Using the original's trademarks as your game's name.

> **The general shape of the rule:** you may learn from a work and build on what you
> learned. You may not redistribute the work. The line is drawn by the derived
> artefacts, which is why the clean-room discipline of "read the binary, write new
> code" matters.

### The clean room, honestly described

A strict clean room has two teams: one reads the original and writes a
specification, the other reads only the specification and writes the code. The
separation exists to prove that no copied expression crossed over.

Almost no hobby project does this, and this one does not. What it does is weaker
but honest: it works from *observed behaviour and data formats*, writes all code
fresh, and never copies bytes of the original's code or art into the repository.

> **Be honest about which you are doing.** Claiming a clean room you did not run is
> worse than admitting you worked from the artefact, because the claim is the thing
> that gets tested.

### The one thing that is never acceptable

Do not use what you learn to circumvent copy protection so that people can play a
game they have not bought. This game has none, but the technique generalises, and
the line is not subtle: preservation and study are one thing; piracy is another.

---


## Chapter 16. Closing

### What we found, in one page

A 293-kilobyte DOS executable, written in Turbo Pascal, in six segments, with a
complete and unobfuscated data vocabulary of twenty-eight file names.

A container format (`LS11`) that is not the LZW its extension claims, decoding 675
blocks across fourteen files to exactly the sizes their headers promise.

A 1,350-byte file that the community believed was encrypted, and that turned out to
be three tables of 450 bytes: the direction and strength of the wind in summer,
the same in winter, and the currents — two numbers packed into each byte.

A world 2,160 by 1,080 pixels, wrapping at its edges, drawn as a 90 × 45 grid of
24 × 24 pixel tiles, with a 540-byte land bitmap of the same grid and a
16,200-byte map that fits a 16,384-byte allocation exactly.

A music format whose opcode encodes its own length, whose note table is the
register map of a 1985 sound chip, and which is shared verbatim with the sound
effects.

And a design built on three coarse grids and a handful of small integers per
location, producing a game about patience and planning rather than reflexes.

### What is still open

The meaning of the numbers inside `WORLDMAP.LZW`. Two unexamined files,
`ZA_DAT.DAT` and `GRAPH.DAT`. The exact timbre of the original's synthesis.

That is the correct state for a project like this: the interesting parts are done,
and the remaining parts are *written down precisely enough to be resumed*.

### The three things to carry away

**First: structure hides in arithmetic, and arithmetic never lies.** Every
confident result in this book came from a number dividing cleanly. Every mistake
came from a number that did not, and was ignored.

**Second: believe the code, not the story.** The file was not encrypted. The
reimplementation was not authoritative. Both were stories, and both were wrong.
The instructions were right.

**Third: write down what you observed, and mark what you guessed.** The most
valuable artefact this project produced is not the decoder. It is a document that
says, line by line, which claims are verified and which are open — because that
document is what lets the next person start in five minutes instead of two years.

> A game is a machine for producing feelings. Take it apart carefully, and you find
> the feelings were made of integers all along — and that this makes them more
> impressive, not less.

---

# Appendices

## Appendix A. The file manifest

Twenty-eight names, from segment 11775 at `0x9224`. Files marked with a star were
present in the release examined.

| Offset | Name | Purpose |
|--------|------|---------|
| `0x9224` | `C:DATA1.LZW` | a large compressed table |
| `0x9230` | `C:KAO.LZW` | the 128 character portraits |
| `0x923A` | `C:PORTCHIP.LZW` | seven sets of 240 port tiles |
| `0x924A` | `C:WORLDMAP.LZW` | the world map |
| `0x925A` | `C:ITEM.MES` | item descriptions |
| `0x9266` | `C:SNRDAT.LZW` | the sound-effect banks |
| `0x9274` | `C:SNRMES.LZW` | sound-effect names |
| `0x9282` | `C:DATA2.LZW` | a second compressed table |
| `0x928E` | `C:EVENT.LZW` | the scenario event data |
| `0x929A` | `C:IKKI.LZW` | an unexamined picture set |
| `0x92A6` | `C:CHIP_NO.DAT` | port → tileset mapping |
| `0x92B4` | `C:COLONY.DAT` | colonies and wonders |
| `0x92C2` | `C:EVENT*.DAT` | the seven scenario files |
| `0x92D0` | `C:GRAPH.DAT` | tiles and icons |
| `0x92DC` | `C:KOUKAI2.DAT` | a large table, content unknown |
| `0x92EA` | `C:MONSTER.DAT` | a small table |
| `0x92F8` | `C:SNR*.DAT` | the sound effects, per bank |
| `0x9304` | `C:TRANSIT.DAT` | an animation |
| `0x9312` | `C:WINDCUR.DAT` | the wind and the currents |
| `0x9320` | `C:ZA_DAT.DAT` | an unexamined table |
| `0x932E` | `C:IAE1.LZW` | item pictures |
| `0x933A` | `C:IAP*.LZW` | ship and item pictures |
| `0x9346` | `C:PORTMAP.LZW` | 101 town maps of 96 × 96 tiles |
| `0x9354` | `C:SNR*.MES` | sound-effect text |
| `0x9360` | `C:HDAT.PUT` | unexamined |
| `0x936C` | `C:NAME.TBL` | the name pool |
| `0x9378` | `C:USERNAME.DAT` | the player's name |
| `0x9388` | `C:D2.MML` | the music |

---


## Appendix B. The segment map

`MAIN.EXE`, 293,537 bytes, MZ, `e_cparhdr` = 1376 (header 22,016 bytes).

| Segment | File offset | Size | Relocations | Character |
|---------|-------------|------|-------------|-----------|
| 0 | 22,016 | 64,288 | 293 | start-up, file I/O, the name table |
| 4036 | 86,304 | 59,824 | 1,851 | the world map and the wind |
| 7775 | 146,128 | 63,968 | 2,253 | the rendering of the map |
| 11775 | 210,096 | 48,592 | 829 | the data of the game |
| 13915 | 258,688 | 64,000 | 195 | the user-interface strings |
| 16954 | 322,688 | 13,152 | 1 | string constants |

Entry point: CS:IP = `0001:0000`, file offset 22,032.

To convert a segment offset to a file offset:

```
file offset = segment start + offset
```

### Landmark offsets used in this book

| Segment | Offset | What |
|---------|--------|------|
| 0 | `0x4977` | the string joiner (`sprintf`-alike) |
| 0 | `0x5625` | the single DOS `open` call |
| 0 | `0x5F6E` | open a file by name |
| 4036 | `0x6763` | the wind loader |
| 4036 | `0x67EA` | clear the land bitmap |
| 4036 | `0x72C3` | the memory allocations |
| 4036 | `0x7AC6` | the wind interpolation |
| 4036 | `0x816F` | read a wind direction |
| 4036 | `0x9703` | write a wind direction |
| 4036 | `0xA399` | the wrap-around |
| 4036 | `0xD95E` | the wind lookup for a position |
| 4036 | `0xDAFA` | draw a row of the map |
| 4036 | `0xDC82` | the map width, 2160 |
| 4036 | `0xDD25` | the map row index, `(Y/24)*90` |
| 11775 | `0x9224` | the file-name table |

---

## Appendix C. The formats, in one page each

### `WINDCUR.DAT` — 1,350 bytes, no header

```
bytes   0..450    the wind of the summer
bytes 450..900    the wind of the winter
bytes 900..1350   the currents
```

One byte per sea area, 450 sea areas in a 30 × 15 grid of 72 × 72 tile areas:

```
bits 0-2   direction, 0-7, clockwise from north
bit  3     flag of a strong wind
bits 4-7   strength, 0-15
```

### `LS11` / `LS10` — the container

```
0..4      "LS10" or "LS11"
4..16     twelve zero bytes
16..272   a 256-byte substitution table, a permutation of 0..255
272..     the block table
```

Each block-table record is twelve bytes: three big-endian 32-bit numbers —
compressed size, uncompressed size, offset. The table ends with four zero bytes. A
block whose two sizes are equal is stored uncompressed.

A compressed block is a bitstream read from the most significant bit of each byte
downwards. A code is a run of one-bits, a zero, then as many bits as the run was
long; the value is run + factor. A code below 256 indexes the substitution table;
256 or more is a back reference, offset `code − 256`, length the second code plus
three. **Blocks are independent.**

### `KAO.LZW` content — the portraits

128 blocks of 1,920 bytes: 64 × 80 pixels, three bit-planes, eight colours.

### `PORTCHIP.LZW` — the town tiles

Seven sets of 240 tiles of 16 × 16 pixels in four bit-planes.

### `PORTMAP.LZW` — the town maps

101 maps, each 96 × 96 bytes of tile indices into the set named by `CHIP_NO.DAT`.

### `D2.MML` — the music

A table of sixteen-bit offsets to tracks; a zero ends a song. One byte per command:

```
0x00-0x5F   a command with two parameters
0x60-0xBF   a note, pitch in the low six bits
0xC0-0xFF   a command with one parameter
```

Pitch is `(byte − 0x60) + channel transpose + driver transpose` semitones. The
frequency table is the twelve F-numbers of the YM3812:

```
343 363 385 408 432 458 485 514 544 577 611 647
```

### `SNDT` — the sound-effect banks

Magic `SNDT`, bank number, offset table, then effects in the same command language,
each ending with `0xF8 0xF2`.

---


## Appendix D. Glossary

**8086 / real mode** — the processor model this game was written for; 16-bit
registers, one megabyte of memory addressed as `segment:offset`.

**block** — one independently decompressed chunk of an `LS11` file. 675 of them
across fourteen files.

**Capstone** — a disassembly library; the one used in this book.

**far pointer** — a `segment:offset` pair naming memory outside the current
segment. Stored as two consecutive words in Turbo Pascal.

**F-number** — the register value that sets a YM3812 channel's pitch.

**LS10 / LS11** — the compression container used by every `.LZW` file here. Not
LZW.

**MML** — Music Macro Language; here, a binary command language, not text.

**MZ header** — the DOS executable header, beginning `4D 5A`.

**offset** — the low half of a far pointer; meaningless without its segment.

**OPL2 / YM3812** — Yamaha's FM sound chip; the game's synthesiser.

**oracle** — an independent source of truth used to test a hypothesis.

**paragraph** — sixteen bytes; the unit of x86 memory allocation. `0x400`
paragraphs is 16,384 bytes, not 1,024.

**plane** — one bit-depth layer of an indexed image. Four planes give sixteen
colours; three give eight.

**relocation** — a placeholder in an executable that the loader patches with a real
segment at run time. The relocation table is a map of every cross-segment
reference.

**sea area** — one cell of the 30 × 15 behavioural grid; 72 × 72 tiles; holds one
wind and one current.

**segment** — the high half of a far pointer; a 64-kilobyte window into memory,
stepping in units of sixteen bytes.

**sentinel** — a value reserved to mean "not a real value", such as `0xFFFF`.

**tile** — one 24 × 24 pixel cell of the world's visual grid; 4,050 of them.

---

## Appendix E. Colophon

This book was written from the artefact: the executable and data files of the 1995
English DOS release of *Uncharted Waters: New Horizons*, examined with Python,
Capstone, `xxd`, and a number of DOSBox sessions.

Every offset cited was read from the file. Every mechanism described was read from
the code that implements it. The places where inference outran evidence are
marked, and one of them is a mistake that was made, caught and corrected in
public, because that is how the work actually goes.

The companion project is a clean-room reimplementation in Go with the Ebitengine
game library. It reads only derived assets, never the release. Its notes —
`docs/executable-notes.md` and `docs/session-handoff.md` — are the primary sources
for this book and are worth reading alongside it.

*Fin.*

---
