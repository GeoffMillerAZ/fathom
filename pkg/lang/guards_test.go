package lang_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// A TOML document of n keys costs go-toml n*n/2 moves: 60,000 keys took 4 s and
// 180,000 took 27 s inside the 2 MiB limit. A document past the key bound is
// refused whole with the reason, and one under it reads.
func TestTOMLKeyCountIsBounded(t *testing.T) {
	r := newReader(t)
	keys := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "k%d = 1\n", i)
		}
		return b.String()
	}
	for _, path := range []string{"data.toml", "crate/Cargo.toml"} {
		var got lang.Result
		within(t, 20*time.Second, path+" with 180,000 keys", func() { got = extract(t, r, path, keys(180_000)) })
		if got.Skipped != "" || !got.Incomplete || len(got.Notes) != 1 || !strings.Contains(got.Notes[0], "keys") || len(got.Symbols) != 0 {
			t.Errorf("%s: skipped %q incomplete=%v notes=%v symbols=%d", path, got.Skipped, got.Incomplete, got.Notes, len(got.Symbols))
		}
	}
	if ok := extract(t, r, "data.toml", keys(5_000)); ok.Incomplete || len(ok.Symbols) != 5_000 {
		t.Errorf("5,000 keys: incomplete=%v symbols=%d notes=%v", ok.Incomplete, len(ok.Symbols), ok.Notes)
	}
	// Dots in a float and in strings are not key parts, and values are not keys.
	flat := "a = [" + strings.Repeat("1.5, ", 20_000) + "2.5]\n" + "s = \"a.b.c = d\"\nt = \"x.y\"\n"
	if ok := extract(t, r, "data.toml", flat); ok.Incomplete {
		t.Errorf("a long array of floats reads incomplete: %v", ok.Notes)
	}
}

// emicklei/proto's service and message loops have no exit at the end of the
// input: a 50-byte file cut at an open `{` never returned, and a real file cut at
// a bracket read the same. A file with a bracket open at its end is refused with
// the reason, and a file cut at every bracket of the fixtures returns.
func TestProtoBracketOpenAtEOFIsRefused(t *testing.T) {
	r := newReader(t)
	for _, src := range []string{
		"syntax=\"proto3\";\nservice s{ rpc a(b) returns (c) {",
		"syntax=\"proto3\";\nmessage m {",
		"syntax=\"proto3\";\nmessage m { option (a) = { b: [1,",
		"syntax=\"proto3\";\nmessage m { map<string, int32 x = 1;",
	} {
		var got lang.Result
		within(t, 20*time.Second, src, func() { got = extract(t, r, "api/x.proto", src) })
		if !got.Incomplete || len(got.Notes) != 1 || !strings.Contains(got.Notes[0], "bracket is still open") {
			t.Errorf("%q: incomplete=%v notes=%v", src, got.Incomplete, got.Notes)
		}
	}
	// A balanced file still reads.
	if ok := extract(t, r, "api/x.proto", "syntax=\"proto3\";\nmessage m { int32 a = 1; }\n"); ok.Incomplete || len(ok.Symbols) == 0 {
		t.Errorf("a balanced file: incomplete=%v notes=%v", ok.Incomplete, ok.Notes)
	}
	dir := filepath.Join("..", "..", "internal", "extractor", "dialects", "protobuf", "testdata")
	for _, name := range []string{"greeter.proto", "shapes.proto", "legacy.proto"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		cuts := 0
		within(t, 120*time.Second, name+" cut at every bracket", func() {
			for i, c := range raw {
				if c == '{' || c == '(' || c == '[' || c == '<' {
					extract(t, r, "api/"+name, string(raw[:i+1]))
					cuts++
				}
			}
		})
		if cuts == 0 {
			t.Errorf("%s: no bracket to cut at", name)
		}
	}
}

// The locator that gives a TOML key its line hashed every dotted prefix of the
// key's full name, so a table header of 9,000 segments with 8,000 keys under it
// (81 KB) took 27 s and 2.1 GB. A header of more than 64 segments is refused, and
// a dotted key of thousands of segments is located by its first 64.
func TestTOMLDeepHeaderIsRefusedAndDeepKeysAreBounded(t *testing.T) {
	r := newReader(t)
	var h strings.Builder
	h.WriteString("[" + strings.Repeat("a.", 9_000) + "a]\n")
	for i := 0; i < 8_000; i++ {
		fmt.Fprintf(&h, "k%d = 1\n", i)
	}
	var got lang.Result
	within(t, 20*time.Second, "a 9,000 segment header", func() { got = extract(t, r, "crate/Cargo.toml", h.String()) })
	if !got.Incomplete || len(got.Notes) != 1 || !strings.Contains(got.Notes[0], "table header of more than 64 segments") {
		t.Errorf("deep header: incomplete=%v notes=%v", got.Incomplete, got.Notes)
	}
	var k strings.Builder
	k.WriteString("[package]\n")
	for i := 0; i < 3_000; i++ {
		fmt.Fprintf(&k, "%sk%d = 1\n", strings.Repeat("a.", 9_000), i)
	}
	within(t, 20*time.Second, "3,000 keys of 9,000 segments", func() { extract(t, r, "data.toml", k.String()) })
	// A header at the bound reads.
	ok := "[" + strings.TrimSuffix(strings.Repeat("a.", 64), ".") + "]\nk = 1\n"
	if got := extract(t, r, "crate/Cargo.toml", ok); got.Incomplete {
		t.Errorf("a 64 segment header is refused: %v", got.Notes)
	}
}
