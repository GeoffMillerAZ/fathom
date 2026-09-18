package fusion_test

// mjs_extraction_test.go — J1 (app-kit O18/O32/O33): after a clean ingest,
// find_symbol never found an exported .mjs function at all — not a parser
// gap (internal/extractor's "javascript" TSExtractor already spans .mjs/
// .cjs), but a DISPATCH gap: fusion.New never routed these extensions to
// any extractor, so the files were fingerprinted and otherwise silently
// dropped. This proves the fix end to end, through the same Ingest path a
// real service uses — not just the extractor package in isolation.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type mjsFixtureConn struct {
	refs    []core.SourceRef
	content map[string][]byte
}

func (c *mjsFixtureConn) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	out := make([]core.SourceRef, len(c.refs))
	for i, r := range c.refs {
		r.Service = svc.ID
		out[i] = r
	}
	return out, nil
}

func (c *mjsFixtureConn) Fetch(_ context.Context, ref core.SourceRef) ([]byte, error) {
	return c.content[ref.Path], nil
}

func (c *mjsFixtureConn) Watermark(context.Context, core.ServiceRef) (string, error) { return "", nil }

func TestIngest_MjsCjsMtsCtsSymbolsIndexed(t *testing.T) {
	svc := core.ServiceRef{ID: "app-kit"}
	conn := &mjsFixtureConn{
		refs: []core.SourceRef{
			{Path: "apps/styleguide/browser/memory.mjs"},
			{Path: "apps/styleguide/browser/owned_process.cjs"},
			{Path: "src/probe.mts"},
			{Path: "src/types.cts"},
		},
		content: map[string][]byte{
			"apps/styleguide/browser/memory.mjs": []byte(
				"export async function createMemoryMeasurements() {\n  return { heap: 0 };\n}\n\n" +
					"export function trackOwnedChild(pid) {\n  return pid;\n}\n",
			),
			"apps/styleguide/browser/owned_process.cjs": []byte(
				"function trackOwnedChild(pid) {\n  return pid;\n}\n\nmodule.exports = { trackOwnedChild };\n",
			),
			"src/probe.mts": []byte("export function run(): void {}\n"),
			"src/types.cts": []byte("export interface Probe {\n  id: string;\n}\n"),
		},
	}
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, fusion.Config{})

	report, err := p.Ingest(context.Background(), svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if report.Discovered != 4 {
		t.Fatalf("Discovered = %d, want 4", report.Discovered)
	}
	if report.Extracted != 4 {
		t.Fatalf("Extracted = %d, want 4 (before J1 these fell into no-extractor and were never extracted)", report.Extracted)
	}

	obs, err := st.QueryObservations(context.Background(), svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	names := map[string]bool{}
	for _, o := range obs {
		names[o.Symbol] = true
	}
	for _, want := range []string{"createMemoryMeasurements", "trackOwnedChild", "run", "Probe"} {
		if !names[want] {
			t.Errorf("symbol %q not indexed; observed %v", want, names)
		}
	}
	if _, ok := report.UnsupportedLanguages["js"]; ok {
		t.Errorf("mjs/cjs must never be reported as an unsupported language: %v", report.UnsupportedLanguages)
	}
	if _, ok := report.UnsupportedLanguages["ts"]; ok {
		t.Errorf("mts/cts must never be reported as an unsupported language: %v", report.UnsupportedLanguages)
	}
}
