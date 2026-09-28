package extractor

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// updateGoldens rewrites every dialects/<id>/testdata/<case>.golden.json from
// the extractor's current output: `go test ./internal/extractor -run
// 'TestTagsDialectParity|TestDialectGoldens' -update`. A regenerated golden is
// reviewed as a diff, never accepted blind.
var updateGoldens = flag.Bool("update", false, "rewrite dialect golden files from current output")

// migratedTagsDialects are the five languages TSExtractor served before
// dialect manifests (LB-F1). Their goldens were first written by the
// pre-manifest engine at 56c18502; TestTagsDialectParity holds every later
// engine change to them.
var migratedTagsDialects = []string{"javascript", "kotlin", "python", "ruby", "typescript"}

// goldenSymbol is one SymbolNode as a golden file records it. Service and
// FilePath are fixed by the harness ("svc", "src/<case>.<ext>").
type goldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line,omitempty"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type goldenImport struct {
	Local    string `json:"local,omitempty"`
	Exported string `json:"exported,omitempty"`
	Module   string `json:"module"`
	Kind     string `json:"kind"`
	Line     int    `json:"line"`
}

type goldenFile struct {
	Symbols []goldenSymbol `json:"symbols"`
	Imports []goldenImport `json:"imports,omitempty"`
}

// legacyProjection drops the fields LB-F3 adds (EndLine, Signature): the
// part of a golden the five migrated dialects must never change, whatever
// the engine learns later — no symbol regressions.
func (g goldenFile) legacyProjection() goldenFile {
	out := goldenFile{Imports: g.Imports}
	for _, s := range g.Symbols {
		s.EndLine, s.Signature = 0, ""
		out.Symbols = append(out.Symbols, s)
	}
	return out
}

func dialectFixtures(t *testing.T, id string) []string {
	t.Helper()
	dir := filepath.Join("dialects", id, "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

func goldenPathFor(fixture string) string {
	return strings.TrimSuffix(fixture, filepath.Ext(fixture)) + filepath.Ext(fixture) + ".golden.json"
}

// extractGolden runs e over fixture exactly as fusion would and renders the
// result in golden form.
func extractGolden(t *testing.T, e *TSExtractor, fixture string) goldenFile {
	t.Helper()
	src, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	rel := "src/" + filepath.Base(fixture)
	nodes, err := e.Extract("svc", rel, src)
	if err != nil {
		t.Fatalf("Extract %s: %v", fixture, err)
	}
	var g goldenFile
	g.Symbols = []goldenSymbol{}
	for _, n := range nodes {
		if n.Service != "svc" || n.FilePath != rel {
			t.Fatalf("%s: plumbing changed: service %q file %q", fixture, n.Service, n.FilePath)
		}
		g.Symbols = append(g.Symbols, goldenSymbol{
			Name: n.Name, Kind: string(n.Kind), Package: n.Package, Line: n.Line,
			EndLine: n.EndLine, Signature: n.Signature, Calls: n.Calls, Tier: tierName(n.Tier),
		})
	}
	imps, err := e.ExtractImports(rel, src)
	if err != nil {
		t.Fatalf("ExtractImports %s: %v", fixture, err)
	}
	for _, s := range imps {
		kind := "named"
		switch s.Kind {
		case ImportWhole:
			kind = "whole"
		case ImportWildcard:
			kind = "wildcard"
		}
		g.Imports = append(g.Imports, goldenImport{Local: s.LocalName, Exported: s.ExportedName, Module: s.Module, Kind: kind, Line: s.Line})
	}
	return g
}

func tierName(t core.EvidenceTier) string {
	if t == core.TierVerified {
		return "verified"
	}
	return "other"
}

func marshalGolden(g goldenFile) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(g)
	return buf.Bytes()
}

// TestTagsDialectParity (LB-F1) holds the five migrated languages to their
// pre-manifest output: every fixture's symbols and imports, compared on the
// legacy projection (everything except EndLine and Signature, which LB-F3
// adds). The goldens were written by the pre-manifest engine; a change here
// is a symbol regression, not a golden to refresh.
func TestTagsDialectParity(t *testing.T) {
	for _, id := range migratedTagsDialects {
		t.Run(id, func(t *testing.T) {
			e, err := NewTSExtractor(id)
			if err != nil {
				t.Fatalf("NewTSExtractor(%s): %v", id, err)
			}
			defer e.Close()
			for _, fx := range dialectFixtures(t, id) {
				got := extractGolden(t, e, fx)
				gp := goldenPathFor(fx)
				if *updateGoldens {
					if err := os.WriteFile(gp, marshalGolden(got), 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				raw, err := os.ReadFile(gp)
				if err != nil {
					t.Fatalf("read golden: %v (run with -update to create)", err)
				}
				var want goldenFile
				if err := json.Unmarshal(raw, &want); err != nil {
					t.Fatalf("%s: %v", gp, err)
				}
				gb, wb := marshalGolden(got.legacyProjection()), marshalGolden(want.legacyProjection())
				if !bytes.Equal(gb, wb) {
					t.Errorf("%s: parity broken\n--- got\n%s\n--- want\n%s", fx, gb, wb)
				}
			}
		})
	}
}

// TestDialectGoldens (design §13.2 step 10) is every tags-engine dialect's
// own golden check: unlike TestTagsDialectParity (which holds the five
// pre-manifest languages to a legacy projection dropping EndLine and
// Signature), this compares the extractor's FULL output — the shape any
// new dialect (CSS onward) is actually held to. migratedTagsDialects stay
// covered by the parity test above and are skipped here so a fixture is
// never graded twice under two different projections.
func TestDialectGoldens(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.Engine != "tags" || slices.Contains(migratedTagsDialects, d.ID) {
			continue
		}
		dir := filepath.Join("dialects", d.ID, "testdata")
		if _, err := os.Stat(dir); err != nil {
			continue // no fixtures committed yet for this dialect
		}
		t.Run(d.ID, func(t *testing.T) {
			e, err := NewTSExtractor(d.ID)
			if err != nil {
				t.Fatalf("NewTSExtractor(%s): %v", d.ID, err)
			}
			defer e.Close()
			for _, fx := range dialectFixtures(t, d.ID) {
				got := extractGolden(t, e, fx)
				gp := goldenPathFor(fx)
				if *updateGoldens {
					if err := os.WriteFile(gp, marshalGolden(got), 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				raw, err := os.ReadFile(gp)
				if err != nil {
					t.Fatalf("read golden: %v (run with -update to create)", err)
				}
				var want goldenFile
				if err := json.Unmarshal(raw, &want); err != nil {
					t.Fatalf("%s: %v", gp, err)
				}
				gb, wb := marshalGolden(got), marshalGolden(want)
				if !bytes.Equal(gb, wb) {
					t.Errorf("%s: golden mismatch\n--- got\n%s\n--- want\n%s", fx, gb, wb)
				}
			}
		})
	}
}
