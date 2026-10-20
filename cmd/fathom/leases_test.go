package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLeaseAppendsDerivedStateOnly pins WF-5: `fathom plans lease` appends
// one JSON row to .fathom/leases.jsonl and nothing else — no status field
// anywhere, no store/verify writes (the command doesn't even accept a
// --store flag) — and leased/expired/released are all derived from the
// appended rows, never stored as their own field.
func TestLeaseAppendsDerivedStateOnly(t *testing.T) {
	root := t.TempDir()

	var out, errBuf bytes.Buffer
	code := run([]string{
		"plans", "lease", "A-1",
		"--plan", "docs/plans/foo.md", "--ttl", "2h", "--holder", "tester",
		"--root", root,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans lease exit = %d, stderr: %s", code, errBuf.String())
	}

	path := leasesPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 lease row, got %d: %q", len(lines), string(data))
	}

	// No status field anywhere in the raw JSON — the derived-state
	// contract lives in the row's own bytes, not just in Go's leaseRow
	// shape.
	var raw map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &raw); err != nil {
		t.Fatalf("lease row is not valid JSON: %v", err)
	}
	if _, ok := raw["status"]; ok {
		t.Errorf("lease row carries a status field — leased/expired must be derived, not stored: %v", raw)
	}
	for _, want := range []string{"plan", "row_id", "holder", "harness", "filed", "expires"} {
		if _, ok := raw[want]; !ok {
			t.Errorf("lease row missing %q: %v", want, raw)
		}
	}
	if raw["holder"] != "tester" || raw["harness"] != "cli" {
		t.Errorf("holder/harness = %v/%v, want tester/cli", raw["holder"], raw["harness"])
	}

	var row leaseRow
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatal(err)
	}

	// Derived freshness: fresh right after filing...
	if fresh := activeLeases([]leaseRow{row}, row.Filed.Add(time.Minute)); len(fresh) != 1 {
		t.Errorf("lease should be fresh 1m after filing, got %d active", len(fresh))
	}
	// ...expired once now passes Expires...
	if fresh := activeLeases([]leaseRow{row}, row.Expires.Add(time.Minute)); len(fresh) != 0 {
		t.Errorf("lease should be expired past its TTL, got %d active", len(fresh))
	}

	// --release appends a SECOND row (never rewrites the first) that
	// makes the row invisible to activeLeases even while still unexpired.
	var relOut, relErr bytes.Buffer
	code = run([]string{
		"plans", "lease", "A-1",
		"--plan", "docs/plans/foo.md", "--holder", "tester", "--release",
		"--root", root,
	}, &relOut, &relErr)
	if code != 0 {
		t.Fatalf("plans lease --release exit = %d, stderr: %s", code, relErr.String())
	}
	data2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines2 := strings.Split(strings.TrimSpace(string(data2)), "\n")
	if len(lines2) != 2 {
		t.Fatalf("--release should APPEND, not rewrite: want 2 lines, got %d", len(lines2))
	}
	var rows []leaseRow
	for _, l := range lines2 {
		var r leaseRow
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, r)
	}
	if fresh := activeLeases(rows, row.Filed.Add(time.Minute)); len(fresh) != 0 {
		t.Errorf("a released lease must be invisible to activeLeases even before its TTL, got %d active", len(fresh))
	}
}

// TestLeaseHolderDefaultsToSessionID pins the holder/harness defaulting
// rule: $CLAUDE_SESSION_ID set → holder defaults to it and harness is
// "claude".
func TestLeaseHolderDefaultsToSessionID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_SESSION_ID", "sess-123")

	var out, errBuf bytes.Buffer
	code := run([]string{"plans", "lease", "A-1", "--plan", "docs/plans/foo.md", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans lease exit = %d, stderr: %s", code, errBuf.String())
	}
	rows, err := readLeases(root)
	if err != nil || len(rows) != 1 {
		t.Fatalf("readLeases: %v, %d rows", err, len(rows))
	}
	if rows[0].Holder != "sess-123" || rows[0].Harness != "claude" {
		t.Errorf("holder/harness = %q/%q, want sess-123/claude", rows[0].Holder, rows[0].Harness)
	}
}

// TestLeaseRequiresHolderWithoutSession: no $CLAUDE_SESSION_ID and no
// --holder is a refusal, not a silently-empty holder.
func TestLeaseRequiresHolderWithoutSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_SESSION_ID", "")

	var out, errBuf bytes.Buffer
	code := run([]string{"plans", "lease", "A-1", "--plan", "docs/plans/foo.md", "--root", root}, &out, &errBuf)
	if code == 0 {
		t.Fatal("want non-zero exit when neither --holder nor $CLAUDE_SESSION_ID is set")
	}
}

// TestLeasesReadLegacyWriteStateDir (FD-33): leases.jsonl lives in the state
// directory. A legacy .fathom/leases.jsonl is read while the new file is
// absent, the first write seeds the new file from it, every write lands only
// in the new file, and the legacy copy is never touched (I63).
func TestLeasesReadLegacyWriteStateDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FATHOM_STATE_DIR", filepath.Join(t.TempDir(), "state"))

	legacy := filepath.Join(root, ".fathom", "leases.jsonl")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	old := leaseRow{Plan: "docs/plans/p.md", RowID: "A-1", Holder: "old", Harness: "cli", Filed: now, Expires: now.Add(time.Hour)}
	line, _ := json.Marshal(old)
	legacyBytes := append(line, '\n')
	if err := os.WriteFile(legacy, legacyBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	if leasesPath(root) == legacy {
		t.Fatal("leasesPath still names the legacy file")
	}
	rows, err := readLeases(root)
	if err != nil || len(rows) != 1 || rows[0].Holder != "old" {
		t.Fatalf("legacy rows not read: %v %+v", err, rows)
	}

	nw := leaseRow{Plan: "docs/plans/p.md", RowID: "A-2", Holder: "new", Harness: "cli", Filed: now, Expires: now.Add(time.Hour)}
	if err := appendLease(root, nw); err != nil {
		t.Fatal(err)
	}
	rows, err = readLeases(root)
	if err != nil || len(rows) != 2 {
		t.Fatalf("after append want legacy row kept and new row added: %v %+v", err, rows)
	}
	if _, err := os.Stat(leasesPath(root)); err != nil {
		t.Fatalf("write did not land in the state directory: %v", err)
	}
	after, err := os.ReadFile(legacy)
	if err != nil || string(after) != string(legacyBytes) {
		t.Fatalf("legacy copy changed or deleted: %v", err)
	}
}
