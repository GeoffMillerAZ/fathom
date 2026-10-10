package extractor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	ts "github.com/tree-sitter/go-tree-sitter"

	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// TestNativeSpecsProbe (EW-53, off unless FATHOM_PROBE_DIR is set) compares a
// dialect's FileSpecs from the wasm path with the native-tree path over a
// checkout: the per-dialect file counts the EW-54 reading quotes
// (docs/bench/readings/efficiency-wave/one-parse.md).
func TestNativeSpecsProbe(t *testing.T) {
	dir := os.Getenv("FATHOM_PROBE_DIR")
	if dir == "" {
		t.Skip("FATHOM_PROBE_DIR not set")
	}
	cases := []struct {
		dialect string
		exts    []string
		lang    *ts.Language
	}{
	}
	only := os.Getenv("FATHOM_PROBE_DIALECT")
	for _, c := range cases {
		if only != "" && only != c.dialect {
			continue
		}
		t.Run(c.dialect, func(t *testing.T) {
			e, err := NewTSExtractor(c.dialect)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			var files, diffs int
			var wasmT, nativeT time.Duration
			shown := 0
			filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				ok := false
				for _, x := range c.exts {
					if strings.EqualFold(filepath.Ext(p), x) {
						ok = true
					}
				}
				if !ok {
					return nil
				}
				src, err := os.ReadFile(p)
				if err != nil || len(src) > 2<<20 {
					return nil
				}
				rel, _ := filepath.Rel(dir, p)
				t0 := time.Now()
				want, werr := e.ExtractFileSpecs(rel, src)
				wasmT += time.Since(t0)
				var got FileSpecs
				var gerr error
				t0 = time.Now()
				perr := treesitter.WithTree(c.lang, src, func(root *ts.Node) {
					got, gerr = nativeFileSpecs(c.lang, root, e.variantFor(rel).rules, src)
				})
				nativeT += time.Since(t0)
				files++
				if (werr != nil) != (gerr != nil || perr != nil) || !reflect.DeepEqual(got, want) {
					diffs++
					if shown < 3 {
						shown++
						t.Logf("DIFF %s\n wasm %+v (%v)\n nat  %+v (%v %v)", rel, want, werr, got, gerr, perr)
					}
				}
				return nil
			})
			t.Logf("%s: files %d, differing %d, wasm %v, native %v", c.dialect, files, diffs, wasmT, nativeT)
		})
	}
}
