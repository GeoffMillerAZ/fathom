package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// unexportedFixture pins LB-F60: a package-level const or var is a symbol
// whether or not its name is exported (a func and a type already were). The
// blank identifier declares nothing and is never a symbol.
const unexportedFixture = `package mypkg

const maxRetries = 3

const (
	modeA = iota
	modeB
	_
	modeD
)

const Exported, hidden = 1, 2

var defaultName = "x"

var (
	cacheSize int
	_         = "blank in a group"
	Visible   = 1
)

var left, right = 1, 2

var _ = "blank"

var _, kept = 1, 2

func f() {
	const local = 1
	var inner = 2
	_, _ = local, inner
}
`

func TestGoExtractor_UnexportedConstVar(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("mysvc", "pkg/mypkg/mypkg.go", []byte(unexportedFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if n.Kind == core.KindGoConst || n.Kind == core.KindGoVar {
			if _, dup := byName[n.Name]; dup {
				t.Errorf("symbol %q emitted twice", n.Name)
			}
			byName[n.Name] = n
		}
	}

	// name -> kind and the line the name is declared on.
	want := []struct {
		name string
		kind core.SourceKind
		line int
	}{
		{"maxRetries", core.KindGoConst, 3},
		{"modeA", core.KindGoConst, 6},
		{"modeB", core.KindGoConst, 7},
		{"modeD", core.KindGoConst, 9},
		{"Exported", core.KindGoConst, 12},
		{"hidden", core.KindGoConst, 12},
		{"defaultName", core.KindGoVar, 14},
		{"cacheSize", core.KindGoVar, 17},
		{"Visible", core.KindGoVar, 19},
		{"left", core.KindGoVar, 22},
		{"right", core.KindGoVar, 22},
		{"kept", core.KindGoVar, 26},
	}
	for _, w := range want {
		n, ok := byName[w.name]
		if !ok {
			t.Errorf("symbol %q (%s) not emitted", w.name, w.kind)
			continue
		}
		if n.Kind != w.kind {
			t.Errorf("%s: Kind %q, want %q", w.name, n.Kind, w.kind)
		}
		if n.Line != w.line {
			t.Errorf("%s: Line %d, want %d", w.name, n.Line, w.line)
		}
		if n.EndLine < n.Line {
			t.Errorf("%s: EndLine %d before Line %d", w.name, n.EndLine, n.Line)
		}
		if n.Package != "mypkg" || n.FilePath != "pkg/mypkg/mypkg.go" || n.Service != "mysvc" || n.Tier != core.TierVerified {
			t.Errorf("%s: identity fields wrong: %+v", w.name, n)
		}
	}

	// The blank identifier is never a symbol; a const or var inside a
	// function body is not package-level and is not one either.
	for _, absent := range []string{"_", "local", "inner"} {
		if _, ok := byName[absent]; ok {
			t.Errorf("symbol %q must not be emitted", absent)
		}
	}
	if len(byName) != len(want) {
		t.Errorf("emitted %d const/var symbols, want %d: %v", len(byName), len(want), byName)
	}
}
