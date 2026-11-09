package importer

import (
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// The original game used the DOS era Japanese code page for its text
// assets. CP932 (Shift-JIS) is the code page that the Japanese DOS
// releases of the era used.
//
// The decoding is done with golang.org/x/text, which is a pure Go library
// and therefore compatible with the CGO_ENABLED=0 build constraint. The
// conversion is lossy by design: a byte that cannot be decoded is
// replaced, so that one damaged byte never aborts the import of a whole
// table.

// cp932Decoder decodes a CP932 byte sequence into a UTF-8 string.
func decodeCP932(raw []byte) string {
	dec := japanese.ShiftJIS.NewDecoder()
	out, _, err := transform.Bytes(dec, raw)
	if err != nil {
		// The decoder only fails on a truncated sequence; the partial
		// output is still usable.
		return string(out)
	}
	// The NUL byte terminates the strings of the original data.
	if idx := strings.IndexByte(string(out), 0); idx >= 0 {
		return string(out[:idx])
	}
	return string(out)
}
