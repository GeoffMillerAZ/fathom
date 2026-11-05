package lang_test

import (
	"fmt"
	"github.com/geoffmilleraz/fathom/pkg/lang"
	"strings"
	"testing"
	"time"
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
