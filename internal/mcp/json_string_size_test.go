package mcp

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Encoding the response is the independent oracle for a dispatch byte budget:
// every single byte, including malformed UTF-8, must cost exactly its encoding.
func TestJSONStringSizeAllBytesAndMalformedUTF8(t *testing.T) {
	for i := 0; i <= 255; i++ {
		t.Run(fmt.Sprintf("byte-%02x", i), func(t *testing.T) {
			assertJSONStringSize(t, string([]byte{byte(i)}))
		})
	}
	for _, s := range []string{"é日本", "\u2028\u2029", "\xff日本", "a\xffz", "\xc0\xaf", "\xed\xa0\x80", "\xf0\x9f", "\xef\xbf\xbd"} {
		assertJSONStringSize(t, s)
	}
}

func assertJSONStringSize(t *testing.T, s string) {
	t.Helper()
	for _, value := range []any{s, map[string]any{s: []string{s}}} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		n, err := jsonSize(value, 1<<30)
		if err != nil || n != len(b) {
			t.Errorf("input %q: size = %d, %v; Marshal = %d: %s", s, n, err, len(b), b)
		}
	}
}
