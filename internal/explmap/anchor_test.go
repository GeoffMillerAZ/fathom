package explmap

import (
	"errors"
	"math/rand"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func anchorNode(pkg, name, file string, line, end int) core.SymbolNode {
	return core.SymbolNode{Service: "fathom", Package: pkg, Name: name, FilePath: file, Line: line, EndLine: end, Kind: core.KindGoFunc}
}

func anchorNodes() []core.SymbolNode {
	return []core.SymbolNode{
		anchorNode("fusion", "ResolveImportSpecs", "internal/fusion/importtargets.go", 55, 97),
		anchorNode("explmap", "resolveEntryFresh", "internal/explmap/fresh.go", 74, 120),
		anchorNode("main", "run", "cmd/a/main.go", 3, 9),
		anchorNode("main", "run", "cmd/b/main.go", 4, 8),
		anchorNode("registry", "Registry.CloseCase", "internal/registry/reg.go", 10, 20),
		anchorNode("cache", "Cache.CloseCase", "internal/cache/c.go", 11, 15),
		anchorNode("cfg", "Load", "internal/cfg/load.go", 5, 30),
		anchorNode("cfg", "load", "internal/cfg/load.go", 40, 50),
		anchorNode("root", "Main", "main.go", 1, 4),
	}
}

// TestMapAnchorResolvesLikeFindSymbol pins XM-31's resolver: a symbol anchor
// resolves the way find_symbol does, by tiers (exact, then a bare method name,
// then package.Name, then case-insensitive), the first tier that matches anything
// deciding; one distinct symbol resolves, several list their candidates and
// resolve nothing, none is a miss; the stored key is one canonical spelling
// whatever spelling the caller used.
func TestMapAnchorResolvesLikeFindSymbol(t *testing.T) {
	const fusionKey = "internal/fusion.ResolveImportSpecs"
	resolved := []struct {
		query string
		key   string
		tier  AnchorTier
		file  string
		line  int
	}{
		{"internal/fusion.ResolveImportSpecs", fusionKey, AnchorExact, "internal/fusion/importtargets.go", 55},
		{"ResolveImportSpecs", fusionKey, AnchorExact, "internal/fusion/importtargets.go", 55},
		{"fathom::internal/fusion.ResolveImportSpecs", fusionKey, AnchorExact, "internal/fusion/importtargets.go", 55},
		{"fathom::internal/fusion/fusion.ResolveImportSpecs", fusionKey, AnchorExact, "internal/fusion/importtargets.go", 55},
		{"internal/fusion/fusion.ResolveImportSpecs", fusionKey, AnchorExact, "internal/fusion/importtargets.go", 55},
		{"fusion.ResolveImportSpecs", fusionKey, AnchorPackage, "internal/fusion/importtargets.go", 55},
		{"resolveimportspecs", fusionKey, AnchorCaseInsensitive, "internal/fusion/importtargets.go", 55},
		{"Internal/Fusion.resolveimportspecs", fusionKey, AnchorCaseInsensitive, "internal/fusion/importtargets.go", 55},
		{"  ResolveImportSpecs  ", fusionKey, AnchorExact, "internal/fusion/importtargets.go", 55},
		{"internal/registry.Registry.CloseCase", "internal/registry.Registry.CloseCase", AnchorExact, "internal/registry/reg.go", 10},
		{"(*Registry).CloseCase", "internal/registry.Registry.CloseCase", AnchorExact, "internal/registry/reg.go", 10},
		{"registry.Registry.CloseCase", "internal/registry.Registry.CloseCase", AnchorPackage, "internal/registry/reg.go", 10},
		{"Main", "root.Main", AnchorExact, "main.go", 1},
		{"root.Main", "root.Main", AnchorExact, "main.go", 1},
		// The exact tier decides before case-insensitive is consulted: Load
		// and load both exist, so each name is its own symbol.
		{"Load", "internal/cfg.Load", AnchorExact, "internal/cfg/load.go", 5},
		{"load", "internal/cfg.load", AnchorExact, "internal/cfg/load.go", 40},
	}
	for _, c := range resolved {
		got, err := ResolveAnchorSymbol(anchorNodes(), c.query)
		if err != nil {
			t.Errorf("ResolveAnchorSymbol(%q): %v", c.query, err)
			continue
		}
		if got.Key != c.key || got.Tier != c.tier || got.Node.FilePath != c.file || got.Node.Line != c.line {
			t.Errorf("ResolveAnchorSymbol(%q) = key %q tier %q at %s:%d; want %q %q at %s:%d",
				c.query, got.Key, got.Tier, got.Node.FilePath, got.Node.Line, c.key, c.tier, c.file, c.line)
		}
	}

	// Ambiguity lists the candidates and resolves nothing, never first-match-wins.
	for _, c := range []struct {
		query string
		tier  AnchorTier
		want  []string
	}{
		{"run", AnchorExact, []string{"cmd/a.run (cmd/a/main.go:3)", "cmd/b.run (cmd/b/main.go:4)"}},
		{"CloseCase", AnchorMember, []string{"internal/cache.Cache.CloseCase (internal/cache/c.go:11)", "internal/registry.Registry.CloseCase (internal/registry/reg.go:10)"}},
		{"main.run", AnchorPackage, []string{"cmd/a.run (cmd/a/main.go:3)", "cmd/b.run (cmd/b/main.go:4)"}},
	} {
		_, err := ResolveAnchorSymbol(anchorNodes(), c.query)
		var amb *AmbiguousAnchorError
		if !errors.As(err, &amb) {
			t.Errorf("ResolveAnchorSymbol(%q) err = %v, want an ambiguity error", c.query, err)
			continue
		}
		if amb.Tier != c.tier || strings.Join(amb.Candidates, "|") != strings.Join(c.want, "|") || amb.Total != len(c.want) {
			t.Errorf("ResolveAnchorSymbol(%q) ambiguity = tier %q %v (total %d); want %q %v", c.query, amb.Tier, amb.Candidates, amb.Total, c.tier, c.want)
		}
		for _, w := range c.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("the ambiguity message for %q must name %q: %s", c.query, w, err)
			}
		}
	}

	// No match at any tier is a miss.
	for _, q := range []string{"NoSuchThing", "fusion.NoSuchThing", "", "   ", "a b"} {
		if _, err := ResolveAnchorSymbol(anchorNodes(), q); !errors.Is(err, ErrAnchorNotFound) {
			t.Errorf("ResolveAnchorSymbol(%q) err = %v, want ErrAnchorNotFound", q, err)
		}
	}

	// Two nodes that share one canonical key (a build-tag pair in one
	// directory) are one symbol, and the earlier file and line is the span.
	twin := append(anchorNodes(),
		anchorNode("cfg", "Twin", "internal/cfg/twin_linux.go", 9, 12),
		anchorNode("cfg", "Twin", "internal/cfg/twin_darwin.go", 7, 10))
	got, err := ResolveAnchorSymbol(twin, "Twin")
	if err != nil || got.Node.FilePath != "internal/cfg/twin_darwin.go" || got.Key != "internal/cfg.Twin" {
		t.Errorf("a build-tag pair: %+v, %v; want one symbol at the earlier file", got, err)
	}

	// The order the index lists its symbols in decides nothing.
	shuffled := anchorNodes()
	rand.New(rand.NewSource(7)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	_, err = ResolveAnchorSymbol(shuffled, "run")
	var amb *AmbiguousAnchorError
	if !errors.As(err, &amb) || amb.Candidates[0] != "cmd/a.run (cmd/a/main.go:3)" {
		t.Errorf("shuffled input changed the ambiguity list: %v", err)
	}
}

// TestAmbiguousAnchorListIsBounded keeps the message a line a terminal shows:
// past the cap the candidates are counted, not all printed.
func TestAmbiguousAnchorListIsBounded(t *testing.T) {
	var nodes []core.SymbolNode
	for i := 0; i < 25; i++ {
		nodes = append(nodes, anchorNode("p", "Common", "pkg"+string(rune('a'+i))+"/x.go", i+1, i+2))
	}
	_, err := ResolveAnchorSymbol(nodes, "Common")
	var amb *AmbiguousAnchorError
	if !errors.As(err, &amb) {
		t.Fatalf("err = %v, want an ambiguity error", err)
	}
	if amb.Total != 25 || len(amb.Candidates) != AnchorCandidateCap {
		t.Errorf("total %d, listed %d; want 25 counted and %d listed", amb.Total, len(amb.Candidates), AnchorCandidateCap)
	}
	if !strings.Contains(err.Error(), "17 more") {
		t.Errorf("the message must count what it left out: %s", err)
	}
}
