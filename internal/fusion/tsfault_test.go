package fusion_test

// tsfault_test.go — critic M1: a tree-sitter engine fault (parse timeout or
// trap, ADR-021 §4) must be VISIBLE per ingest, not a silent zero. Before
// this counter existed, an engine-level parse failure returned nil,nil with
// no log and no counter, the file was fingerprinted (never retried), and the
// poisoned parser was reused — one trap could zero a whole language for the
// daemon's lifetime. Now: the fault is counted on fusion.Report
// (TSEngineFaults), one stderr line is emitted, and the extractor re-creates
// the parser so the NEXT file of that language extracts normally.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_TSEngineFaultCountedAndRecovered(t *testing.T) {
	saved := tsengine.ParseTimeout
	tsengine.ParseTimeout = 300 * time.Millisecond
	defer func() { tsengine.ParseTimeout = saved }()

	dir := t.TempDir()
	// aaa_bundle.min.js sorts before good.js, so the fault happens FIRST and
	// the healthy file proves parser re-creation within the same ingest.
	// 150KB, under the 2MB cap. Broken into short lines: one 150KB line
	// is minified, and the LB-F10 gate would skip it before the engine.
	pathological := bytes.Repeat([]byte("({[({[({[({[({[\n"), 10000)
	if err := os.WriteFile(filepath.Join(dir, "aaa_bundle.min.js"), pathological, 0o644); err != nil {
		t.Fatalf("write pathological file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.js"), []byte("function ok() { return 1; }\n"), 0o644); err != nil {
		t.Fatalf("write good.js: %v", err)
	}

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	svc := core.ServiceRef{ID: "tsfault-svc", Path: dir}
	ctx := context.Background()

	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.TSEngineFaults != 1 {
		t.Errorf("Report.TSEngineFaults = %d, want 1", rep.TSEngineFaults)
	}

	// The healthy file must have extracted normally on the re-created parser.
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawOK bool
	for _, o := range obs {
		if o.SourceKind == core.KindJSFunc && o.Symbol == "ok" {
			sawOK = true
		}
	}
	if !sawOK {
		t.Error("good.js did not extract after the engine fault — parser was not re-created")
	}

	// The faulted file is fingerprinted (existing non-fatal contract), so a
	// clean re-ingest skips both files and reports zero new faults.
	rep2, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("re-Ingest: %v", err)
	}
	if rep2.TSEngineFaults != 0 {
		t.Errorf("re-ingest Report.TSEngineFaults = %d, want 0 (file fingerprinted)", rep2.TSEngineFaults)
	}
	if rep2.Skipped != 2 {
		t.Errorf("re-ingest Skipped = %d, want 2", rep2.Skipped)
	}
}
