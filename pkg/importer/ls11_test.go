package importer

import (
	"os"
	"path/filepath"
	"testing"
)

// lsRawDir returns the directory that holds the original asset files.
//
// The directory is supplied by the user; nothing of it is ever committed
// to the repository, so the tests skip themselves when it is not there.
func lsRawDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("ENCHANTED_RAW_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "..", "Downloads", "Uncharted2")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skip("the original asset files are not present")
	}
	return dir
}

// TestLSDecodesEveryBlock checks that every block of every LS file of the
// release decodes to exactly the size that its record asks for.
//
// This is the property that holds the codec together: the block table
// records the size of the output, and a decoder that reads the codes
// correctly must produce exactly that many bytes.
func TestLSDecodesEveryBlock(t *testing.T) {
	dir := lsRawDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	files := 0
	blocks := 0
	for _, e := range entries {
		name := e.Name()
		if len(name) < 4 || name[len(name)-4:] != ".LZW" {
			continue
		}
		files++
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		f, err := ParseLS(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		parts, err := DecodeLS(f, data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(parts) != len(f.Blocks) {
			t.Errorf("%s: decoded %d blocks, want %d", name, len(parts), len(f.Blocks))
			continue
		}
		for i, p := range parts {
			want := f.Blocks[i].UncompressedSize
			if len(p) != want {
				t.Errorf("%s: block %d decoded to %d bytes, want %d", name, i, len(p), want)
			}
			blocks++
		}
	}
	if files == 0 {
		t.Skip("no LS files were found")
	}
	t.Logf("decoded %d blocks of %d files", blocks, files)
}

// TestLSBlockTableTilesThePayload checks the block table.
//
// The offset of a block is the offset of its predecessor plus the
// compressed size of that predecessor, so the blocks tile the payload.
func TestLSBlockTableTilesThePayload(t *testing.T) {
	dir := lsRawDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) < 4 || name[len(name)-4:] != ".LZW" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		f, err := ParseLS(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for i := 1; i < len(f.Blocks); i++ {
			want := f.Blocks[i-1].Offset + f.Blocks[i-1].CompressedSize
			if f.Blocks[i].Offset != want {
				t.Errorf("%s: block %d starts at %d, want %d", name, i, f.Blocks[i].Offset, want)
			}
		}
		last := f.Blocks[len(f.Blocks)-1]
		if last.Offset+last.CompressedSize > len(data) {
			t.Errorf("%s: the last block runs past the end of the file", name)
		}
	}
}

// TestLSDictionaryIsAPermutation checks the substitution table.
func TestLSDictionaryIsAPermutation(t *testing.T) {
	dir := lsRawDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) < 4 || name[len(name)-4:] != ".LZW" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		f, err := ParseLS(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		var seen [256]bool
		for _, b := range f.Dictionary {
			if seen[b] {
				t.Errorf("%s: the substitution table repeats the byte %d", name, b)
				break
			}
			seen[b] = true
		}
	}
}

// TestLSCodeReading checks the reading of a code by hand.
//
// The bitstream is read most significant bit first. A code is a run of one
// bits, a zero, and then the run length in bits; the value is the run read
// as a number plus the factor.
//
// The first byte of a block, 0x3c, reads as 0, 0, 1, 1, 1, 1, 0, 0 from
// the top down. The first bit is a zero, so the run is empty and the factor
// is zero bits long: the code is 0.
func TestLSCodeReading(t *testing.T) {
	block := []byte{0x3c}
	code, ok := lsReadFirstCode(block, 8)
	if !ok {
		t.Fatal("the first code could not be read")
	}
	if code != 0 {
		t.Errorf("the first code of 0x3c is %d, want 0", code)
	}

	// A run of ones exercises the run part of the code: 0xe0 reads as
	// 1, 1, 1, 0, 0, 0, 0, 0, so the run is three ones, the number is
	// 0b1110 = 14, and the factor is the three bits that follow.
	block = []byte{0xe0, 0x00}
	code, ok = lsReadFirstCode(block, 16)
	if !ok {
		t.Fatal("the first code could not be read")
	}
	// The run read as a number is 0b1110 = 14, and the factor that
	// follows is the three bits 0b000 = 0.
	if code != 14 {
		t.Errorf("the first code of 0xe0 is %d, want 14", code)
	}
}

// lsReadFirstCode reads the first code of a bitstream, for the test.
func lsReadFirstCode(comp []byte, bitLen int) (int, bool) {
	bitPos := 0
	run := 0
	count := 0
	for {
		if bitPos >= bitLen {
			return 0, false
		}
		bit := lsBitMSB(comp, bitPos)
		bitPos++
		run = run<<1 | int(bit)
		count++
		if bit == 0 {
			break
		}
	}
	factor := 0
	for i := 0; i < count; i++ {
		if bitPos >= bitLen {
			return 0, false
		}
		factor = factor<<1 | int(lsBitMSB(comp, bitPos))
		bitPos++
	}
	return run + factor, true
}
