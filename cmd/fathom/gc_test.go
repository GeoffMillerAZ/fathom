package main

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// gcTreeDigest hashes every path, size, mode and content under roots.
func gcTreeDigest(t *testing.T, roots ...string) string {
	t.Helper()
	var lines []string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			st, _ := d.Info()
			line := fmt.Sprintf("%s %v %d", p, d.IsDir(), st.Size())
			if !d.IsDir() {
				b, _ := os.ReadFile(p)
				line += fmt.Sprintf(" %x", sha256.Sum256(b))
			}
			lines = append(lines, line)
			return nil
		})
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func gcTouch(t *testing.T, p string, size int64, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if size > 0 {
		_ = f.Truncate(size)
	}
	f.Close()
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatal(err)
	}
}

// RT-125 (design §12.1): `fathom gc` is a dry run by default — it reports
// every class's reclaimable bytes and changes nothing — and --apply
// applies per-class retention, archiving evidence (the call log) and
// deleting only what is not evidence.
func TestGCDryRunDeletesNothing(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	old := now.Add(-40 * 24 * time.Hour)
	home, tmp, ws := t.TempDir(), t.TempDir(), t.TempDir()
	fh := filepath.Join(home, ".fathom")
	callLog := filepath.Join(home, "state", "logs", "mcp-calls.jsonl")

	gcTouch(t, callLog, callLogRotateBytes+1, now)
	callLogPending := callLog + ".20260926T000000Z.pending"
	gcTouch(t, callLogPending, 4096, now)
	gcTouch(t, filepath.Join(fh, "session-ledger.jsonl"), gcLedgerRotateBytes+1, now)
	gcTouch(t, filepath.Join(fh, "session-stamps", "old.json"), 10, old)
	gcTouch(t, filepath.Join(fh, "session-stamps", "new.json"), 10, now)
	// A dead daemon's record and a live one's; the lifecycle ledger stays.
	for id, pid := range map[string]int{"addr-9-1": 111, "fathom-live": 222} {
		d := filepath.Join(fh, "daemon", id)
		gcTouch(t, filepath.Join(d, "lifecycle.jsonl"), 5, now)
		if err := os.WriteFile(filepath.Join(d, "record.json"), []byte(fmt.Sprintf(`{"schema_version":2,"addr":"127.0.0.1:9","pid":%d}`, pid)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 12; i++ {
		gcTouch(t, filepath.Join(fh, "daemon", "fathom-live", "diag", fmt.Sprintf("20260926T20%02d00Z-refuse", i), "heap.pb.gz"), 10, now)
	}
	// wazero's compilation cache (LB-F26): another version's directory and
	// a current entry unwritten for 40 days go; a fresh entry stays.
	wz := filepath.Join(fh, "cache", "wazero")
	gcTouch(t, filepath.Join(wz, "v0.0.1", "stale"), 10, now)
	gcTouch(t, filepath.Join(wz, tsengine.WazeroVersion(), "old"), 10, old)
	gcTouch(t, filepath.Join(wz, tsengine.WazeroVersion(), "fresh"), 10, now)
	gcTouch(t, filepath.Join(tmp, "fathom-edit-set-abc"), 10, now.Add(-48*time.Hour))
	gcTouch(t, filepath.Join(tmp, "fathom-mcp-test-123", "x"), 10, now.Add(-48*time.Hour))
	os.Chtimes(filepath.Join(tmp, "fathom-mcp-test-123"), now.Add(-48*time.Hour), now.Add(-48*time.Hour))
	gcTouch(t, filepath.Join(tmp, "fathom-edit-set-fresh"), 10, now)
	gcTouch(t, filepath.Join(tmp, "unrelated"), 10, old)
	for i := 0; i < 8; i++ {
		gcTouch(t, filepath.Join(ws, "artifacts", "scenariobench", "worktrees", fmt.Sprintf("run-%d", i), "f"), 10, old.Add(time.Duration(i)*time.Hour))
		os.Chtimes(filepath.Join(ws, "artifacts", "scenariobench", "worktrees", fmt.Sprintf("run-%d", i)), old.Add(time.Duration(i)*time.Hour), old.Add(time.Duration(i)*time.Hour))
	}
	gcTouch(t, filepath.Join(ws, "artifacts", "scenariobench", "worktrees", "run-0", ".pin"), 0, old)
	os.Chtimes(filepath.Join(ws, "artifacts", "scenariobench", "worktrees", "run-0"), old, old)
	for i := 0; i < 5; i++ {
		gcTouch(t, filepath.Join(ws, "tmp", fmt.Sprintf("fathom-bin-%d", i)), 10, now.Add(time.Duration(i)*time.Minute))
	}
	alive := func(pid int) bool { return pid == 222 }

	before := gcTreeDigest(t, home, tmp, ws)
	dry := runGC(gcOptions{Home: home, TmpDir: tmp, Workspace: ws, CallLog: callLog, Now: now, Alive: alive})
	if after := gcTreeDigest(t, home, tmp, ws); after != before {
		t.Fatal("a dry run changed the disk")
	}
	if dry.Applied || len(dry.Errors) > 0 {
		t.Fatalf("dry run: applied %v errors %v", dry.Applied, dry.Errors)
	}
	want := map[string]int{"call-log": 2, "ledgers": 1, "session-stamps": 1, "daemon-records": 1, "diag": 2, "tmp": 2, "scenariobench-worktrees": 2, "workspace-bins": 2, "wazero-cache": 2}
	got := map[string]int{}
	for _, it := range dry.Items {
		got[it.Class]++
		if it.Done {
			t.Fatalf("a dry run did %v", it)
		}
	}
	for c, n := range want {
		if got[c] != n {
			t.Errorf("class %s: %d item(s), want %d (items %+v)", c, got[c], n, dry.Items)
		}
	}
	if dry.Reclaimable["call-log"] == 0 || dry.Reclaimable["tmp"] == 0 {
		t.Fatalf("reclaimable by class not reported: %v", dry.Reclaimable)
	}
	if _, ok := dry.Reclaimable["ledgers"]; ok {
		t.Fatalf("a reported ledger is not reclaimable: %v", dry.Reclaimable)
	}

	// --apply: evidence archived, never deleted; the rest removed.
	rep := runGC(gcOptions{Home: home, TmpDir: tmp, Workspace: ws, CallLog: callLog, Now: now, Alive: alive, Apply: true})
	if len(rep.Errors) > 0 {
		t.Fatalf("apply errors: %v", rep.Errors)
	}
	if _, err := os.Stat(filepath.Join(wz, tsengine.WazeroVersion(), "fresh")); err != nil {
		t.Fatalf("gc deleted a fresh compilation-cache entry: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wz, "v0.0.1")); !os.IsNotExist(err) {
		t.Fatalf("another wazero version's cache survived --apply: %v", err)
	}
	if st, err := os.Stat(callLog); err != nil || st.Size() != callLogRotateBytes+1 {
		t.Fatalf("gc never renames the live call log (the daemon's writer rotates it): %v", err)
	}
	if _, err := os.Stat(callLog + ".20260926T000000Z.jsonl.gz"); err != nil {
		t.Fatalf("the pending generation is archived, never deleted: %v", err)
	}
	if st, err := os.Stat(filepath.Join(fh, "session-ledger.jsonl")); err != nil || st.Size() != gcLedgerRotateBytes+1 {
		t.Fatalf("a ledger is never rotated away or deleted: %v", err)
	}
	for _, gone := range []string{
		filepath.Join(fh, "session-stamps", "old.json"), filepath.Join(fh, "daemon", "addr-9-1", "record.json"),
		filepath.Join(tmp, "fathom-edit-set-abc"), filepath.Join(tmp, "fathom-mcp-test-123"),
		filepath.Join(ws, "artifacts", "scenariobench", "worktrees", "run-1"), filepath.Join(ws, "tmp", "fathom-bin-0"),
	} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s survived --apply", gone)
		}
	}
	for _, kept := range []string{
		filepath.Join(fh, "session-stamps", "new.json"), filepath.Join(fh, "daemon", "fathom-live", "record.json"),
		filepath.Join(fh, "daemon", "addr-9-1", "lifecycle.jsonl"), filepath.Join(tmp, "fathom-edit-set-fresh"), filepath.Join(tmp, "unrelated"),
		filepath.Join(ws, "artifacts", "scenariobench", "worktrees", "run-0"), filepath.Join(ws, "artifacts", "scenariobench", "worktrees", "run-7"),
		filepath.Join(ws, "tmp", "fathom-bin-4"),
	} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s was removed: %v", kept, err)
		}
	}
	if ents, _ := os.ReadDir(filepath.Join(fh, "daemon", "fathom-live", "diag")); len(ents) != diagKeep {
		t.Errorf("diag kept %d captures, want %d", len(ents), diagKeep)
	}
}
