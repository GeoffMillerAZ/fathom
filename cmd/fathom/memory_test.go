package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memory"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// memory_test.go — the `fathom memory` verb (AM-1/AM-2/AM-5).

// seedMemoryStore writes one symbol observation for parseConfig with the
// given file content hash, plus the service metadata that supplies the index
// generation, and returns the store path.
func seedMemoryStore(t *testing.T, dir, contentHash, commit string) string {
	t.Helper()
	path := filepath.Join(dir, "memory.sqlite")
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: "svc", Package: "cfg", Symbol: "parseConfig", SourceKind: core.KindGoFunc,
		SourceRef: "internal/cfg/cfg.go", Claim: "func parseConfig(b []byte) (Config, error)",
		ContentHash: contentHash,
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := st.SetFingerprint(ctx, "svc", "internal/cfg/cfg.go", contentHash); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: dir, IndexedCommit: commit, IndexedAt: 1}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return path
}

// reseedMemoryStore rewrites the same service at a new content hash and
// commit — the "the code changed under the memory" step.
func reseedMemoryStore(t *testing.T, storePath, dir, contentHash, commit string) {
	t.Helper()
	st, err := store.NewSQLiteStore(storePath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	if err := st.PurgeSource(ctx, "svc", "internal/cfg/cfg.go"); err != nil {
		t.Fatalf("PurgeSource: %v", err)
	}
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: "svc", Package: "cfg", Symbol: "parseConfig", SourceKind: core.KindGoFunc,
		SourceRef: "internal/cfg/cfg.go", Claim: "func parseConfig(b []byte) (Config, error)",
		ContentHash: contentHash,
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := st.SetFingerprint(ctx, "svc", "internal/cfg/cfg.go", contentHash); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: dir, IndexedCommit: commit, IndexedAt: 2}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func addMemory(t *testing.T, storePath, root string, extra ...string) string {
	t.Helper()
	args := append([]string{
		"--store", storePath, "--service", "svc", "--root", root,
		"--claim", "parseConfig validates the schema before returning.",
		"--anchor", "symbol:parseConfig",
		"--evidence", "index:svc::cfg.parseConfig",
		"--by", "human:alex",
		"--json",
	}, extra...)
	var out, errBuf bytes.Buffer
	if code := cmdMemoryAdd(args, &out, &errBuf); code != 0 {
		t.Fatalf("memory add exit = %d, stderr: %s", code, errBuf.String())
	}
	var m memory.Memory
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	return m.ID
}

// TestReviewReconfirmationTiers proves AM-5 through the CLI: `fathom memory
// review` resolves a suspect memory, records who confirmed it and against
// which commit, and a MODEL-confirmed memory lands at a lower tier than a
// HUMAN-confirmed one — with neither invocation naming a tier.
func TestReviewReconfirmationTiers(t *testing.T) {
	resolveBy := func(t *testing.T, by string) memoryReviewReport {
		t.Helper()
		dir := t.TempDir()
		root := t.TempDir()
		storePath := seedMemoryStore(t, dir, "file-hash-v1", "aaa111")
		id := addMemory(t, storePath, root)

		// The anchored code changes.
		reseedMemoryStore(t, storePath, dir, "file-hash-v2", "bbb222")

		// It is in the queue, suspect.
		var qOut, qErr bytes.Buffer
		if code := cmdMemoryReview([]string{"--store", storePath, "--service", "svc", "--root", root, "--json"}, &qOut, &qErr); code != 0 {
			t.Fatalf("memory review (queue) exit = %d, stderr: %s", code, qErr.String())
		}
		var queue memoryReviewReport
		if err := json.Unmarshal(qOut.Bytes(), &queue); err != nil {
			t.Fatalf("unmarshal queue: %v\n%s", err, qOut.String())
		}
		if len(queue.Queue) != 1 || queue.Queue[0].State != string(memory.StateSuspect) {
			t.Fatalf("queue = %+v, want one suspect memory", queue.Queue)
		}

		var out, errBuf bytes.Buffer
		code := cmdMemoryReview([]string{
			"--store", storePath, "--service", "svc", "--root", root,
			"--id", id, "--resolve", "still-true", "--by", by, "--json",
		}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("memory review --resolve exit = %d, stderr: %s", code, errBuf.String())
		}
		var report memoryReviewReport
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
		}
		if report.Resolved == nil {
			t.Fatalf("no resolution in %s", out.String())
		}
		return report
	}

	human := resolveBy(t, "human:alex")
	model := resolveBy(t, "model:some-model")

	if human.Resolved.State != string(memory.StateFresh) {
		t.Errorf("human-resolved state = %q, want %q", human.Resolved.State, memory.StateFresh)
	}
	if model.Resolved.State != string(memory.StateFresh) {
		t.Errorf("model-resolved state = %q, want %q", model.Resolved.State, memory.StateFresh)
	}
	if human.Resolved.Tier != string(core.TierSMEStated) {
		t.Errorf("human-confirmed tier = %q, want %q", human.Resolved.Tier, core.TierSMEStated)
	}
	if model.Resolved.Tier != string(core.TierAssumed) {
		t.Errorf("model-confirmed tier = %q, want %q", model.Resolved.Tier, core.TierAssumed)
	}
	if model.Resolved.Confidence >= human.Resolved.Confidence {
		t.Errorf("model confidence %d is not below human confidence %d",
			model.Resolved.Confidence, human.Resolved.Confidence)
	}

	// Who, and against which commit — both recorded, neither stated as a
	// tier by the caller.
	if !strings.Contains(human.Resolved.By, "alex") {
		t.Errorf("By = %q, want the human's name", human.Resolved.By)
	}
	if human.Resolved.Commit != "bbb222" {
		t.Errorf("Commit = %q, want the service's current indexed commit bbb222", human.Resolved.Commit)
	}
	if human.Resolved.LastConfirmedAt == "" {
		t.Error("LastConfirmedAt was not updated")
	}
	if !model.Resolved.Capped || model.Resolved.CapReason == "" {
		t.Error("a model confirmation weakened the tier and must say why")
	}
}

// TestCmdMemoryAddRefusesUnanchored: the refusal reaches the operator with
// its reason and its remedy, and nothing is written.
func TestCmdMemoryAddRefusesUnanchored(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	storePath := seedMemoryStore(t, dir, "file-hash-v1", "aaa111")

	var out, errBuf bytes.Buffer
	code := cmdMemoryAdd([]string{
		"--store", storePath, "--service", "svc", "--root", root,
		"--claim", "we prefer tabs over spaces",
		"--evidence", "human:alex",
		"--by", "human:alex",
	}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("memory add accepted an unanchored memory; stdout: %s", out.String())
	}
	msg := errBuf.String()
	if !strings.Contains(msg, string(memory.RefusalNoAnchor)) {
		t.Errorf("stderr %q does not name the refusal", msg)
	}
	if !strings.Contains(msg, "CLAUDE.md") {
		t.Errorf("stderr %q does not name where the note belongs", msg)
	}
	if _, err := memory.NewFileStore(memory.Path(root)).List(context.Background(), ""); err != nil {
		t.Fatalf("List: %v", err)
	}
	all, _ := memory.NewFileStore(memory.Path(root)).List(context.Background(), "")
	if len(all) != 0 {
		t.Errorf("a refused add wrote %d memories", len(all))
	}
}

// TestCmdMemoryAddCapsALyingCaller: the same law-14 cap AM-2 proves in the
// package must also be visible on the surface an operator actually uses.
func TestCmdMemoryAddCapsALyingCaller(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	storePath := seedMemoryStore(t, dir, "file-hash-v1", "aaa111")

	var out, errBuf bytes.Buffer
	code := cmdMemoryAdd([]string{
		"--store", storePath, "--service", "svc", "--root", root,
		"--claim", "parseConfig validates the schema before returning.",
		"--anchor", "symbol:parseConfig",
		"--evidence", "model:some-model",
		"--tier", "verified", "--confidence", "5",
		"--by", "model:some-model", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var m memory.Memory
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if m.Tier != core.TierAssumed || m.Confidence != 1 {
		t.Errorf("stored tier/confidence = %q/%d, want assumed/1", m.Tier, m.Confidence)
	}
	if !m.Capped || m.CapReason == "" {
		t.Error("the cap must be recorded with its reason")
	}
}

// TestCmdMemoryListLabelsEveryEntry: every surfaced memory carries its tier
// and state. An unlabelled memory would be a claim fathom is making on its
// own authority (§9), so this is a test failure, not a style preference.
func TestCmdMemoryListLabelsEveryEntry(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	storePath := seedMemoryStore(t, dir, "file-hash-v1", "aaa111")
	addMemory(t, storePath, root)
	reseedMemoryStore(t, storePath, dir, "file-hash-v2", "bbb222")

	var out, errBuf bytes.Buffer
	if code := cmdMemoryList([]string{"--store", storePath, "--service", "svc", "--root", root, "--json"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var report memoryListReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if len(report.Entries) != 1 {
		t.Fatalf("entries = %+v, want 1", report.Entries)
	}
	e := report.Entries[0]
	if e.State != string(memory.StateSuspect) {
		t.Errorf("state = %q, want %q", e.State, memory.StateSuspect)
	}
	if e.Tier == "" || !strings.Contains(e.Line, e.Tier) || !strings.Contains(e.Line, e.State) {
		t.Errorf("rendered line %q must carry both tier %q and state %q", e.Line, e.Tier, e.State)
	}
	if len(e.Drift) == 0 {
		t.Error("a suspect entry must say what changed")
	}
}

// TestCmdMemoryRetiredStopsSurfacingButShowStillFinds: §3.2's third key, on
// the surface. Retired is gone from the list and still readable by id.
func TestCmdMemoryRetiredStopsSurfacingButShowStillFinds(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	storePath := seedMemoryStore(t, dir, "file-hash-v1", "aaa111")
	id := addMemory(t, storePath, root)
	reseedMemoryStore(t, storePath, dir, "file-hash-v2", "bbb222")

	var out, errBuf bytes.Buffer
	if code := cmdMemoryReview([]string{
		"--store", storePath, "--service", "svc", "--root", root,
		"--id", id, "--resolve", "no-longer-true", "--by", "human:alex",
		"--note", "the schema check moved to the caller", "--json",
	}, &out, &errBuf); code != 0 {
		t.Fatalf("retire exit = %d, stderr: %s", code, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	if code := cmdMemoryList([]string{"--store", storePath, "--service", "svc", "--root", root, "--state", "fresh", "--json"}, &out, &errBuf); code != 0 {
		t.Fatalf("list exit = %d, stderr: %s", code, errBuf.String())
	}
	var report memoryListReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if len(report.Entries) != 0 {
		t.Errorf("a retired memory still surfaces: %+v", report.Entries)
	}

	out.Reset()
	errBuf.Reset()
	if code := cmdMemoryShow([]string{"--root", root, "--id", id, "--json"}, &out, &errBuf); code != 0 {
		t.Fatalf("show exit = %d, stderr: %s", code, errBuf.String())
	}
	var m memory.Memory
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if !m.Retired() {
		t.Error("show did not report the memory as retired")
	}
	if m.RetiredReason == "" {
		t.Error("a retired memory must carry why")
	}
}

// TestCmdMemoryUsageAndArgErrors covers the operator-facing guardrails.
func TestCmdMemoryUsageAndArgErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no verb", nil},
		{"unknown verb", []string{"forget"}},
		{"add without store", []string{"add", "--service", "svc"}},
		{"list without service", []string{"list", "--store", "x"}},
		{"review with id but no resolve", []string{"review", "--store", "x", "--service", "svc", "--id", "abc"}},
		{"show without id", []string{"show"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			if code := cmdMemory(tc.args, &out, &errBuf); code == 0 {
				t.Fatalf("exit = 0 for %v; stdout: %s", tc.args, out.String())
			}
			if errBuf.Len() == 0 {
				t.Error("no diagnostic written to stderr")
			}
		})
	}
}

// TestCmdMemoryReviewRefusesResolveWithoutBy: WHO resolved is what decides
// the tier, so it cannot be optional.
func TestCmdMemoryReviewRefusesResolveWithoutBy(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	storePath := seedMemoryStore(t, dir, "file-hash-v1", "aaa111")
	id := addMemory(t, storePath, root)

	var out, errBuf bytes.Buffer
	code := cmdMemoryReview([]string{
		"--store", storePath, "--service", "svc", "--root", root,
		"--id", id, "--resolve", "still-true",
	}, &out, &errBuf)
	if code == 0 {
		t.Fatal("review accepted a resolution with no --by")
	}
	if !strings.Contains(errBuf.String(), "--by") {
		t.Errorf("stderr %q does not name the missing flag", errBuf.String())
	}
}

// TestMemoryVerbIsRegistered: the dispatch in main.go actually routes
// "memory", so the verb is reachable from the real entry point.
func TestMemoryVerbIsRegistered(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"memory"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (usage)", code)
	}
	if !strings.Contains(errBuf.String(), "add | list | review | show") {
		t.Errorf("stderr %q is not the memory usage; the verb may not be wired", errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	if code := run([]string{"help"}, &out, &errBuf); code != 0 {
		t.Fatalf("help exit = %d", code)
	}
	if !strings.Contains(out.String(), "memory") {
		t.Error("`fathom help` does not list the memory verb")
	}
}
