package wiring

// testmain_test.go — receiptsPath (receipt.go, O63 item 5) defaults to
// internal/statedir.Resolve(root), which falls back to the user's state
// home; a bare `go test` would otherwise write real receipt files under
// the developer's home. testisolate.Main (RT-19, §10.4) gives the binary
// a private HOME, state dir and call log and an unroutable daemon URL, or
// refuses to run (exit 2); tests that want their own fresh override still
// use t.Setenv, which saves/restores relative to this isolated baseline.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

func TestMain(m *testing.M) { testisolate.Main(m) }
