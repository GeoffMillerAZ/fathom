package main

// review_mode_test.go — FL-15 (docs/plans/findings-loop.md): `fathom
// review`'s --mode/--manifest/--idempotency-key/--expect-index-watermark
// contract.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// storeCounts is a raw row count over every table a review run could touch
// — claims, links, events, and the new review_runs table (FL-15) — read
// directly via SQL so the assertion is independent of the Store interface's
// own (service-scoped) read methods and therefore catches a write to ANY
// service, not just the one under test.
type storeCounts struct {
	claims, links, events, reviewRuns int
}

func dumpStoreCounts(t *testing.T, storePath string) storeCounts {
	t.Helper()
	db, err := sql.Open("sqlite", storePath)
	if err != nil {
		t.Fatalf("open %q for counting: %v", storePath, err)
	}
	defer db.Close()
	count := func(table string) int {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	return storeCounts{
		claims:     count("semantic_claims"),
		links:      count("claim_links"),
		events:     count("claim_events"),
		reviewRuns: count("review_runs"),
	}
}

func sha256HexOfFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// setupReviewModeFixture ingests reviewFixtureSrc under service and returns
// the store path and the fixture root.
func setupReviewModeFixture(t *testing.T, service string) (storePath, root string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}
	return storePath, root
}

// TestReview_ModeReport_NeverWrites_StoreStateEquality pins FL-15's core
// honesty guarantee: --mode report writes NOTHING, on both the happy path
// (a real finding would otherwise persist) and an error path (a manifest
// naming a missing changed file, which refuses with exit 2).
func TestReview_ModeReport_NeverWrites_StoreStateEquality(t *testing.T) {
	storePath, _ := setupReviewModeFixture(t, "review-mode-report-svc")

	t.Run("happy_path", func(t *testing.T) {
		before := dumpStoreCounts(t, storePath)
		var out, errBuf bytes.Buffer
		code := runReview(
			[]string{
				"--service", "review-mode-report-svc", "--since", "HEAD~1", "--store", storePath, "--mode", "report", "--json",
				"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
			},
			fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
		)
		if code != 0 {
			t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
		}
		if payload["mode"] != "report" {
			t.Errorf("mode = %v, want report", payload["mode"])
		}
		if payload["findings_persisted"] != false {
			t.Errorf("findings_persisted = %v, want false", payload["findings_persisted"])
		}
		after := dumpStoreCounts(t, storePath)
		if before != after {
			t.Fatalf("--mode report wrote to the store: before=%+v after=%+v", before, after)
		}
	})

	t.Run("error_path_manifest_mismatch", func(t *testing.T) {
		before := dumpStoreCounts(t, storePath)
		manifestFile := filepath.Join(t.TempDir(), "manifest.json")
		if err := os.WriteFile(manifestFile, []byte(`[{"path":"pkg/does-not-exist.go","sha256":"deadbeef"}]`), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		var out, errBuf bytes.Buffer
		code := runReview(
			[]string{"--service", "review-mode-report-svc", "--manifest", manifestFile, "--store", storePath, "--mode", "report"},
			fakeGitRunner(nil), &out, &errBuf,
		)
		if code != 2 {
			t.Fatalf("review exit = %d, want 2; stderr: %s", code, errBuf.String())
		}
		if !strings.Contains(errBuf.String(), "manifest mismatch: pkg/does-not-exist.go") {
			t.Errorf("stderr = %q, want a manifest mismatch naming the missing path", errBuf.String())
		}
		after := dumpStoreCounts(t, storePath)
		if before != after {
			t.Fatalf("the manifest-mismatch error path wrote to the store: before=%+v after=%+v", before, after)
		}
	})
}

// TestReview_Manifest_HappyPath proves --manifest accepts a caller-supplied
// changed-path digest set (instead of --since) when every entry's current
// sha256 matches, and persists exactly like --since would.
func TestReview_Manifest_HappyPath(t *testing.T) {
	storePath, root := setupReviewModeFixture(t, "review-manifest-svc")
	manifestFile := filepath.Join(t.TempDir(), "manifest.json")
	sha := sha256HexOfFile(t, filepath.Join(root, "pkg", "a.go"))
	body, _ := json.Marshal([]map[string]string{{"path": "pkg/a.go", "sha256": sha}})
	if err := os.WriteFile(manifestFile, body, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{
			"--service", "review-manifest-svc", "--manifest", manifestFile, "--store", storePath, "--json",
			"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
		},
		fakeGitRunner(nil), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if payload["mode"] != "persist" {
		t.Errorf("mode = %v, want persist (the default)", payload["mode"])
	}
	changed, _ := payload["changed_files"].([]any)
	if len(changed) != 1 || changed[0] != "pkg/a.go" {
		t.Errorf("changed_files = %v", payload["changed_files"])
	}
}

// TestReview_Manifest_SinceMutuallyExclusive proves --since and --manifest
// cannot both be given, and that omitting both is refused too.
func TestReview_Manifest_SinceMutuallyExclusive(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runReview([]string{"--service", "svc", "--since", "HEAD~1", "--manifest", "x.json"}, fakeGitRunner(nil), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected a nonzero exit when both --since and --manifest are given")
	}

	errBuf.Reset()
	code = runReview([]string{"--service", "svc"}, fakeGitRunner(nil), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected a nonzero exit when neither --since nor --manifest is given")
	}
}

// TestReview_ModeNoPersistConflict proves --mode persist and --no-persist
// together refuse rather than silently picking one.
func TestReview_ModeNoPersistConflict(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runReview([]string{"--service", "svc", "--since", "HEAD~1", "--mode", "persist", "--no-persist"}, fakeGitRunner(nil), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected a nonzero exit for --mode persist + --no-persist")
	}
}

// TestReview_IdempotencyKey_ReplaySamePayload proves that a repeated
// --idempotency-key with an unchanged request (same changed-file set)
// replays the RECORDED response and writes nothing new — a second review
// run over identical inputs, run twice, produces the identical byte-for-
// byte response and the store's claim/link/event counts never grow past
// the first run's write.
func TestReview_IdempotencyKey_ReplaySamePayload(t *testing.T) {
	storePath, _ := setupReviewModeFixture(t, "review-idem-svc")

	runOnce := func() (string, int) {
		var out, errBuf bytes.Buffer
		code := runReview(
			[]string{
				"--service", "review-idem-svc", "--since", "HEAD~1", "--store", storePath, "--json",
				"--idempotency-key", "key-1",
				"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
			},
			fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
		)
		if code != 0 {
			t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
		}
		return out.String(), code
	}

	first, _ := runOnce()
	afterFirst := dumpStoreCounts(t, storePath)

	second, _ := runOnce()
	afterSecond := dumpStoreCounts(t, storePath)

	if afterFirst != afterSecond {
		t.Fatalf("a replayed idempotency key wrote to the store: after first=%+v after second=%+v", afterFirst, afterSecond)
	}
	var firstPayload, secondPayload map[string]any
	if err := json.Unmarshal([]byte(first), &firstPayload); err != nil {
		t.Fatalf("unmarshal first: %v", err)
	}
	if err := json.Unmarshal([]byte(second), &secondPayload); err != nil {
		t.Fatalf("unmarshal second: %v", err)
	}
	if firstPayload["schema_version"] != secondPayload["schema_version"] {
		t.Errorf("replayed response differs: first=%v second=%v", first, second)
	}
	if len(second) == 0 {
		t.Fatal("replayed response is empty")
	}
}

// TestReview_IdempotencyKey_DifferentPayloadRefuses proves a reused key
// with a DIFFERENT changed-file set refuses (exit 2) rather than silently
// replaying or overwriting the recorded response.
func TestReview_IdempotencyKey_DifferentPayloadRefuses(t *testing.T) {
	storePath, root := setupReviewModeFixture(t, "review-idem-conflict-svc")
	// A second, distinct file so the second run's changed-file set differs.
	if err := os.WriteFile(filepath.Join(root, "pkg", "b.go"), []byte("package pkg\n\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatalf("write second fixture: %v", err)
	}
	var reingestOut, reingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "review-idem-conflict-svc", "--path", root, "--store", storePath}, &reingestOut, &reingestErr); code != 0 {
		t.Fatalf("re-ingest exit = %d, stderr: %s", code, reingestErr.String())
	}

	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{
			"--service", "review-idem-conflict-svc", "--since", "HEAD~1", "--store", storePath,
			"--idempotency-key", "key-1", "--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
		},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("first review exit = %d, stderr: %s", code, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	code = runReview(
		[]string{
			"--service", "review-idem-conflict-svc", "--since", "HEAD~1", "--store", storePath,
			"--idempotency-key", "key-1", "--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
		},
		fakeGitRunner([]string{"pkg/a.go", "pkg/b.go"}), &out, &errBuf,
	)
	if code != 2 {
		t.Fatalf("second review (different payload, same key) exit = %d, want 2; stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "idempotency key reused with a different payload") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

// TestReview_IdempotencyKey_RequiresPersistMode proves --idempotency-key is
// refused outside persist mode.
func TestReview_IdempotencyKey_RequiresPersistMode(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runReview([]string{"--service", "svc", "--since", "HEAD~1", "--no-persist", "--idempotency-key", "k"}, fakeGitRunner(nil), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected a nonzero exit for --idempotency-key without persist mode")
	}
}

// TestReview_ExpectIndexWatermark_Mismatch proves a mismatched
// --expect-index-watermark sets stale_index:true, still runs the
// dimensions (report-shaped, not a refusal), and refuses to persist even
// though --mode defaults to persist.
func TestReview_ExpectIndexWatermark_Mismatch(t *testing.T) {
	storePath, _ := setupReviewModeFixture(t, "review-watermark-svc")
	before := dumpStoreCounts(t, storePath)

	var out, errBuf bytes.Buffer
	code := runReview(
		[]string{
			"--service", "review-watermark-svc", "--since", "HEAD~1", "--store", storePath, "--json",
			"--expect-index-watermark", "not-the-real-commit",
			"--call-log", filepath.Join(t.TempDir(), "calls.jsonl"),
		},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("review exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if payload["stale_index"] != true {
		t.Errorf("stale_index = %v, want true", payload["stale_index"])
	}
	dims, _ := payload["dimensions"].([]any)
	if len(dims) == 0 {
		t.Error("a stale index must still run dimensions (report-shaped), not refuse the whole run")
	}
	if payload["findings_persisted"] != false {
		t.Errorf("findings_persisted = %v, want false — a stale index must refuse persist", payload["findings_persisted"])
	}
	after := dumpStoreCounts(t, storePath)
	if before.claims != after.claims || before.links != after.links {
		t.Errorf("a stale-index run must never persist claims: before=%+v after=%+v", before, after)
	}
}

// TestReview_ServiceMetaSharedWithClient is a light smoke test that
// pkg/client's ServiceMeta passthrough (used to resolve corpusRoot) behaves
// — the underlying mechanics runReview and reviewkey.ReviewKey both share.
func TestReview_ServiceMetaSharedWithClient(t *testing.T) {
	storePath, root := setupReviewModeFixture(t, "review-meta-svc")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	path, _, found, err := c.ServiceMeta(context.Background(), "review-meta-svc")
	if err != nil || !found {
		t.Fatalf("ServiceMeta: found=%v err=%v", found, err)
	}
	if path != root {
		t.Errorf("corpus root = %q, want %q", path, root)
	}
}
