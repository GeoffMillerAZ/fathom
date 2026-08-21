package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFrozenTotalsNeverSummed pins IT-12: no aggregate may sum a
// whole-session token figure from a row whose transcript read hit the
// 16MB LimitReader cap (transcript_truncated: true, sessionledger.go —
// transcriptSummaryFrom sums every harness-usage field, including
// tokens_cache_read, from the SAME parse pass, so a truncated read freezes
// them all together, independent of which field a reader happens to look
// at). buildImpactModel's cache-read average (CacheReadMedianCtx,
// observatory_render.go's "cache reads (context)" tile) is the one plain
// SUM/avg of a whole-session token figure in the model — proven here to
// exclude a truncated row's tokens_cache_read, and to name the exclusion.
func TestFrozenTotalsNeverSummed(t *testing.T) {
	dir := hermeticObsEnv(t)
	rows := []string{
		// Untruncated: a real, trustworthy cache-read figure.
		`{"schema_version":2,"session_id":"clean-1","arm":"on","at":"2026-08-08T10:00:00Z","tokens_note":"","tokens_cache_read":1000,"transcript_truncated":false}`,
		// Truncated: the harness-summed tokens_cache_read is a FROZEN
		// partial figure (whatever accumulated before the 16MB cap hit),
		// never the session's true total — must not enter the sum/average.
		`{"schema_version":2,"session_id":"frozen-1","arm":"on","at":"2026-08-08T11:00:00Z","tokens_note":"","tokens_cache_read":999999999,"transcript_truncated":true}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	imp := buildImpactModel(time.Date(2026, 8, 8, 14, 0, 0, 0, time.UTC))

	if imp.CacheReadMedianCtx == nil {
		t.Fatal("CacheReadMedianCtx absent — the clean row alone should still produce a figure")
	}
	if *imp.CacheReadMedianCtx != 1000 {
		t.Errorf("CacheReadMedianCtx = %d, want 1000 (the clean row alone) — the truncated row's frozen 999999999 must never enter the sum/average", *imp.CacheReadMedianCtx)
	}
	if imp.CacheReadFrozenExcluded == nil || *imp.CacheReadFrozenExcluded != 1 {
		t.Fatalf("CacheReadFrozenExcluded = %v, want 1 — the exclusion must be named, not silent", imp.CacheReadFrozenExcluded)
	}
}
