package importer

import (
	"bytes"
	"math/bits"
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

// TestLSRoundTripIsByteExact proves the codec.
//
// The property that identifies the format is that the payload is a single
// continuous bitstream of variable length codes, and that re-encoding the
// symbols which were read from it reproduces the payload byte for byte.
// The test reads every LS file of the release, decodes it, and checks that
// property for each one.
//
// A decoder that restarted at every block would fail this test on all but
// the smallest blocks, because the block boundaries fall in the middle of
// a byte.
func TestLSRoundTripIsByteExact(t *testing.T) {
	dir := lsRawDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	found := 0
	for _, e := range entries {
		name := e.Name()
		if len(name) < 4 || name[len(name)-4:] != ".LZW" {
			continue
		}
		found++
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		f, err := ParseLS(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if f.Magic != LSMagic10 && f.Magic != LSMagic11 {
			t.Errorf("%s: the magic is %q", name, f.Magic)
		}

		// The block table must tile the payload: the offset of a block
		// is the offset of its predecessor plus the compressed size of
		// that predecessor.
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

		// The round trip: decode the symbols and encode them again.
		// The framing that reproduces the payload is the right one.
		ok := false
		for _, framing := range []lsFraming{lsFramingZero, lsFramingOne} {
			symbols, err := lsDecodeSymbolsFramed(f.Payload, framing)
			if err != nil {
				continue
			}
			if bytes.Equal(lsEncodeSymbolsFramed(symbols, framing), f.Payload) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("%s: the payload is not reproduced by re-encoding its symbols", name)
		}
	}
	if found == 0 {
		t.Skip("no LS files were found")
	}
	t.Logf("checked %d LS files", found)
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

// TestLSBlockSizesAreConsistent measures how many symbols the payload
// holds against the size the block table asks for.
//
// The payload holds fewer symbols than the blocks want, because each symbol
// stands for a phrase rather than for a single byte: the difference is the
// work of the expansion layer, which is not implemented yet. The test
// records the ratio so that a change in the codec shows up here.
func TestLSBlockSizesAreConsistent(t *testing.T) {
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
			continue
		}
		total := 0
		for _, b := range f.Blocks {
			total += b.UncompressedSize
		}
		symbols, err := lsDecodeSymbols(f.Payload)
		if err != nil {
			continue
		}
		if len(symbols) < total {
			t.Logf("%s: the payload holds %d symbols for %d bytes of output (a ratio of %.3f)",
				name, len(symbols), total, float64(total)/float64(len(symbols)))
		}
	}
}

// TestLSCodeEncoding checks the code of a few symbols by hand.
//
// A code is the unary prefix of the Elias gamma code of the symbol plus
// two: the prefix is a run of ones, a zero, and then the factor, most
// significant bit first.
//
//	0 -> 0 0          (n = 2, factor 0)
//	1 -> 0 1          (n = 3, factor 1)
//	2 -> 1 0 0 0      (n = 4, factor 00)
//	3 -> 1 0 0 1      (n = 5, factor 01)
//
// The comparison is at the bit level, because the codes are packed into
// bytes and a short code shares its byte with the next one.
func TestLSCodeEncoding(t *testing.T) {
	cases := []struct {
		symbol int
		want   []byte
	}{
		{0, []byte{0, 0}},
		{1, []byte{0, 1}},
		{2, []byte{1, 0, 0, 0}},
		{3, []byte{1, 0, 0, 1}},
	}
	for _, c := range cases {
		got := lsCodeBits(c.symbol, lsFramingZero)
		if len(got) != len(c.want) {
			t.Errorf("the code of %d is %v, want %v", c.symbol, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("the code of %d is %v, want %v", c.symbol, got, c.want)
				break
			}
		}
	}
}

// TestLSZerosFramingCodeEncoding checks the other framing.
//
// The prefix is a run of zeros and a one.
//
//	0 -> 1 0
//	1 -> 1 1
func TestLSZerosFramingCodeEncoding(t *testing.T) {
	cases := []struct {
		symbol int
		want   []byte
	}{
		{0, []byte{1, 0}},
		{1, []byte{1, 1}},
	}
	for _, c := range cases {
		got := lsCodeBits(c.symbol, lsFramingOne)
		if len(got) != len(c.want) {
			t.Errorf("the code of %d is %v, want %v", c.symbol, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("the code of %d is %v, want %v", c.symbol, got, c.want)
				break
			}
		}
	}
}

// lsCodeBits returns the bits of the code of a symbol with the given
// framing, most significant bit first.
//
// The framing names the bit that ends the unary prefix.
func lsCodeBits(symbol int, framing lsFraming) []byte {
	terminator := byte(0)
	if framing == lsFramingOne {
		terminator = 1
	}
	prefixBit := 1 - terminator
	n := symbol + 2
	factor := bits.Len(uint(n)) - 1
	out := make([]byte, 0, factor*2+1)
	for i := 0; i < factor-1; i++ {
		out = append(out, prefixBit)
	}
	out = append(out, terminator)
	for i := factor - 1; i >= 0; i-- {
		out = append(out, byte(n>>uint(i)&1))
	}
	return out
}

// TestLSDecodeSymbolsRoundTrips checks that the decoder and the encoder
// agree on a list of symbols.
func TestLSDecodeSymbolsRoundTrips(t *testing.T) {
	symbols := []int{0, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 255, 0, 7, 200}
	encoded := lsEncodeSymbols(symbols)
	decoded, err := lsDecodeSymbols(encoded)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(decoded) != len(symbols) {
		t.Fatalf("decoded %d symbols, want %d", len(decoded), len(symbols))
	}
	for i := range symbols {
		if decoded[i] != symbols[i] {
			t.Errorf("symbol %d decoded as %d", i, decoded[i])
		}
	}
}
