package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const purgeCanary = "AKIA-NOT-A-REAL-KEY-0123456789"

// purgeRepo writes a tiny two-directory Go corpus: one file that must be
// purged and ignored forever after, one that must survive both.
func purgeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	must := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	must("secrets/token.go", "package secrets\n\n// Token is the leaked value.\nconst Token = \""+purgeCanary+"\"\n\nfunc Use() string { return Token }\n")
	must("internal/keep.go", "package internal\n\nfunc Keep() string { return \"harmless\" }\n")
	return root
}

// countRefs counts what is REALLY in the store for svc under prefix — via
// the raw per-location resolver, deliberately below the suppression gate,
// because a purge writes a suppression and a filtered count would hide a
// failed delete behind the read filter.
func countRefs(t *testing.T, storePath, svc, prefix string) int {
	t.Helper()
	st, err := store.BackendForPath(storePath)
	if err != nil {
		t.Fatalf("BackendForPath: %v", err)
	}
	defer st.Close()
	sq, ok := st.(*store.SQLiteStore)
	if !ok {
		t.Fatalf("store is %T, want *SQLiteStore", st)
	}
	plan, err := sq.ResolvePurge(context.Background(), svc, filter.New([]string{prefix + "**"}))
	if err != nil {
		t.Fatalf("ResolvePurge: %v", err)
	}
	for _, c := range plan.Counts {
		if c.Location == string(store.LocObservations) {
			return c.Rows
		}
	}
	return 0
}

// TestPurgeSurvivesForcedReingest is SP-8 and §3.5: purge writes an
// INGEST-time exclusion as well as a suppression, because the purge itself
// deletes the fingerprint row (re-arming extraction) and autorefresh
// re-ingests on git-HEAD drift with no human action — a query-layer filter
// cannot stop the bytes coming back. The exclusion is SERVICE-SCOPED
// (V2): it lands in ~/.fathom/ignore.d/<service>, which Discover unions
// only for that service, and never in the shared ~/.fathom/ignore.
func TestPurgeSurvivesForcedReingest(t *testing.T) {
	home := scopeHome(t)
	root := purgeRepo(t)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	const svc = "svc"

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", svc, "--path", root, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest rc=%d stderr=%s", rc, errOut.String())
	}
	if n := countRefs(t, storePath, svc, "secrets/"); n == 0 {
		t.Fatalf("fixture premise broken: nothing was indexed under secrets/ (%s)", out.String())
	}

	out.Reset()
	errOut.Reset()
	rc := run([]string{
		"purge", "secrets/**", "--service", svc, "--store", storePath,
		"--reason", "leaked credential", "--probe", purgeCanary, "--yes", "--json",
	}, &out, &errOut)
	if rc != 0 {
		t.Fatalf("purge rc=%d stdout=%s stderr=%s", rc, out.String(), errOut.String())
	}
	if n := countRefs(t, storePath, svc, "secrets/"); n != 0 {
		t.Fatalf("%d row(s) under secrets/ survived the purge", n)
	}

	// The ingest-time exclusion is the load-bearing part, and it is
	// SERVICE-SCOPED: ignore.d/<svc> carries it, the shared file does not.
	scopedPath, err := connector.ServiceIgnorePath(svc)
	if err != nil {
		t.Fatal(err)
	}
	scoped := readFileString(t, scopedPath)
	if !strings.Contains(scoped, "secrets/**") {
		t.Fatalf("%s does not carry the purged pattern; a query filter cannot stop autorefresh re-indexing it.\n%s", scopedPath, scoped)
	}
	if shared := readFileString(t, filepath.Join(home, ".fathom", "ignore")); strings.Contains(shared, "secrets/**") {
		t.Fatalf("the purged pattern leaked into the SHARED ~/.fathom/ignore, which unions into every repo (V2):\n%s", shared)
	}
	// It must be an EXCLUSION, never a re-include.
	for _, line := range strings.Split(scoped, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "!") {
			t.Fatalf("purge wrote a re-include into the scoped ignore file: %q", line)
		}
	}
	if m, _, err := connector.LoadIgnoreScopedFor(root, svc); err != nil {
		t.Fatalf("LoadIgnoreScopedFor refuses the file purge wrote: %v", err)
	} else if m == nil || !m.Match("secrets/token.go") {
		t.Fatal("the pattern purge wrote does not actually match the purged file at Discover")
	}
	// And it does NOT reach any other service (V2/V4).
	if m, _, err := connector.LoadIgnoreScopedFor(root, "other-svc"); err != nil {
		t.Fatal(err)
	} else if m != nil && m.Match("secrets/token.go") {
		t.Fatal("the exclusion written for one service applies to another — it is not service-scoped")
	}

	// Forced FULL re-ingest — what autorefresh does on git-HEAD drift.
	out.Reset()
	errOut.Reset()
	if rc := run([]string{"ingest", "--service", svc, "--path", root, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("re-ingest rc=%d stderr=%s", rc, errOut.String())
	}
	if n := countRefs(t, storePath, svc, "secrets/"); n != 0 {
		t.Fatalf("%d row(s) under secrets/ came BACK on re-ingest — the ingest-time exclusion is not in effect", n)
	}
	if n := countRefs(t, storePath, svc, "internal/"); n == 0 {
		t.Fatal("the re-ingest lost the unrelated file too — the exclusion is too wide")
	}

	// Appending the same pattern twice must not duplicate the line: purge
	// is run repeatedly and the file is user-visible.
	out.Reset()
	errOut.Reset()
	if rc := run([]string{
		"purge", "secrets/**", "--service", svc, "--store", storePath,
		"--reason", "leaked credential", "--probe", purgeCanary, "--yes",
	}, &out, &errOut); rc != 0 {
		t.Fatalf("second purge rc=%d stdout=%s stderr=%s", rc, out.String(), errOut.String())
	}
	if n := strings.Count(readFileString(t, scopedPath), "secrets/**"); n != 1 {
		t.Fatalf("%s carries the pattern %d times, want 1", scopedPath, n)
	}
}

// TestPurgeAllIsServiceScoped is V2's exact catastrophe, replayed: the
// first build's `purge --service A --all` wrote a bare `**` into the
// SHARED ignore file, which unions into EVERY repo at Discover — one
// service-scoped purge destroyed the whole fleet index on the next
// refresh. Here: two services live in one store; purging A with --all
// leaves B's rows, B's discovery and B's future ingests completely intact.
func TestPurgeAllIsServiceScoped(t *testing.T) {
	home := scopeHome(t)
	rootA := purgeRepo(t)
	rootB := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootB, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "lib", "b.go"), []byte("package lib\n\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", "svc-a", "--path", rootA, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest A rc=%d stderr=%s", rc, errOut.String())
	}
	if rc := run([]string{"ingest", "--service", "svc-b", "--path", rootB, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest B rc=%d stderr=%s", rc, errOut.String())
	}
	if n := countRefs(t, storePath, "svc-b", "lib/"); n == 0 {
		t.Fatal("fixture premise broken: svc-b indexed nothing")
	}

	out.Reset()
	errOut.Reset()
	if rc := run([]string{
		"purge", "--service", "svc-a", "--all", "--store", storePath,
		"--reason", "decommissioned", "--probe", purgeCanary, "--yes",
	}, &out, &errOut); rc != 0 {
		t.Fatalf("purge --all rc=%d stdout=%s stderr=%s", rc, out.String(), errOut.String())
	}

	// A is gone; B is intact.
	if n := countRefs(t, storePath, "svc-a", ""); n != 0 {
		t.Fatalf("svc-a still holds %d row(s)", n)
	}
	if n := countRefs(t, storePath, "svc-b", "lib/"); n == 0 {
		t.Fatal("purging svc-a took svc-b's rows with it")
	}

	// The `**` landed ONLY in svc-a's scoped file. The shared ignore file
	// — the one that unions into every repository — is untouched.
	scopedA, err := connector.ServiceIgnorePath("svc-a")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFileString(t, scopedA), "**") {
		t.Fatalf("svc-a's scoped ignore file does not carry the exclusion")
	}
	if shared := readFileString(t, filepath.Join(home, ".fathom", "ignore")); strings.Contains(shared, "**") {
		t.Fatalf("`--all` wrote into the SHARED ignore file — the fleet-index-destroying accident is back:\n%s", shared)
	}
	scopedB, err := connector.ServiceIgnorePath("svc-b")
	if err != nil {
		t.Fatal(err)
	}
	if readFileString(t, scopedB) != "" {
		t.Fatal("svc-b gained an exclusion file from svc-a's purge")
	}

	// B still ingests fully afterwards.
	out.Reset()
	errOut.Reset()
	if rc := run([]string{"ingest", "--service", "svc-b", "--path", rootB, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("re-ingest B rc=%d stderr=%s", rc, errOut.String())
	}
	if n := countRefs(t, storePath, "svc-b", "lib/"); n == 0 {
		t.Fatal("svc-b can no longer ingest after svc-a's purge — the exclusion is not service-scoped")
	}
}

// TestPurgeDisclosesExclusionScope is V4: even a narrow exclusion changes
// future ingests, so the report states — in words — which service it
// reaches and that every other service is unaffected.
func TestPurgeDisclosesExclusionScope(t *testing.T) {
	scopeHome(t)
	root := purgeRepo(t)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	const svc = "svc"

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", svc, "--path", root, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest rc=%d stderr=%s", rc, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if rc := run([]string{
		"purge", "secrets/**", "--service", svc, "--store", storePath,
		"--reason", "leaked credential", "--probe", purgeCanary, "--yes",
	}, &out, &errOut); rc != 0 {
		t.Fatalf("purge rc=%d stderr=%s", rc, errOut.String())
	}
	text := out.String()
	if !strings.Contains(text, "scoped to this service only") {
		t.Fatalf("the report does not disclose the exclusion's scope in words:\n%s", text)
	}
	if !strings.Contains(text, "every other service is unaffected") {
		t.Fatalf("the report does not state that other services are unaffected:\n%s", text)
	}
	if !strings.Contains(text, "ignore.d") {
		t.Fatalf("the report does not name the scoped file the exclusion landed in:\n%s", text)
	}
}

// TestPurgeBytesUncheckedExitsDistinct is V1 at the verb level: when the
// raw-bytes probe cannot run (here: the purged path is still live under a
// second service, so every needle is skipped), the run must NOT print
// VERIFIED, must name the unchecked bytes, and must exit with its own
// non-zero code — distinct from both success and outright failure.
func TestPurgeBytesUncheckedExitsDistinct(t *testing.T) {
	scopeHome(t)
	rootA := purgeRepo(t)
	rootB := purgeRepo(t) // the SAME relative layout under a second service
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", "svc-a", "--path", rootA, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest A rc=%d stderr=%s", rc, errOut.String())
	}
	if rc := run([]string{"ingest", "--service", "svc-b", "--path", rootB, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest B rc=%d stderr=%s", rc, errOut.String())
	}

	out.Reset()
	errOut.Reset()
	rc := run([]string{
		"purge", "secrets/**", "--service", "svc-a", "--store", storePath,
		"--reason", "leaked credential", "--yes",
	}, &out, &errOut)
	combined := out.String() + errOut.String()
	if rc == 0 {
		t.Fatalf("a purge whose probe never ran exited 0:\n%s", combined)
	}
	if rc == 1 {
		t.Fatalf("bytes-unchecked was flattened into generic failure (rc=1) — it is its own named state:\n%s", combined)
	}
	if strings.Contains(out.String(), "\nVERIFIED") {
		t.Fatalf("VERIFIED was printed while the raw-bytes probe never ran:\n%s", out.String())
	}
	if !strings.Contains(combined, "BYTES UNCHECKED") {
		t.Fatalf("the distinct state is not named:\n%s", combined)
	}
	// The SQL rows for svc-a are genuinely gone; svc-b is untouched.
	if n := countRefs(t, storePath, "svc-a", "secrets/"); n != 0 {
		t.Fatalf("svc-a rows survived: %d", n)
	}
	if n := countRefs(t, storePath, "svc-b", "secrets/"); n == 0 {
		t.Fatal("svc-b lost its rows to svc-a's purge")
	}
	// History records the tri-state verdict, not a flattened boolean.
	out.Reset()
	errOut.Reset()
	if rc := run([]string{"purge", "history", "--json"}, &out, &errOut); rc != 0 {
		t.Fatalf("history rc=%d stderr=%s", rc, errOut.String())
	}
	if !strings.Contains(out.String(), string(store.PurgeBytesUnchecked)) {
		t.Fatalf("purge history does not carry the bytes-unchecked verdict:\n%s", out.String())
	}
}

// TestPurgeFailsLoudlyOnSurvivor is SP-7 at the verb level: verifyPurge
// turns the store's report into an exit code and a message. An
// artificially-surviving row must make the run FAIL, name the table, and
// never report partial success — "a partial purge that reports success is
// worse than a failed one, because the user stops looking" (§3.3).
func TestPurgeFailsLoudlyOnSurvivor(t *testing.T) {
	scopeHome(t)
	root := purgeRepo(t)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	const svc = "svc"

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", svc, "--path", root, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest rc=%d stderr=%s", rc, errOut.String())
	}

	purgeBetweenExecuteAndVerify = func() {
		st, err := store.BackendForPath(storePath)
		if err != nil {
			t.Errorf("inject: %v", err)
			return
		}
		defer st.Close()
		if err := st.PutKnowledgeCards(context.Background(), []store.KnowledgeCard{
			{ServiceID: svc, Symbol: "Resurrected", File: "secrets/token.go", Summary: "survived"},
		}); err != nil {
			t.Errorf("inject: %v", err)
		}
	}
	t.Cleanup(func() { purgeBetweenExecuteAndVerify = nil })

	out.Reset()
	errOut.Reset()
	rc := run([]string{
		"purge", "secrets/**", "--service", svc, "--store", storePath,
		"--reason", "leaked credential", "--yes",
	}, &out, &errOut)
	if rc == 0 {
		t.Fatalf("a surviving row exited 0:\n%s", out.String())
	}
	if rc != 1 {
		t.Fatalf("a surviving row exited %d, want the FAILED code 1 — a survivor must never be softened into another state:\n%s%s", rc, out.String(), errOut.String())
	}
	combined := out.String() + errOut.String()
	// The report table always lists every location, so the assertion must
	// be on the survivor line specifically, not the bare table name.
	if !strings.Contains(combined, "survivor: knowledge_cards") {
		t.Fatalf("the failure does not name the surviving table:\n%s", combined)
	}
	if strings.Contains(strings.ToLower(combined), "partial success") {
		t.Fatalf("the run reported partial success:\n%s", combined)
	}
}

// TestClaimEventsRetentionDisclosed is SP-9 at the verb level: the default
// run states, with a count, that claim history was RETAINED, and
// --include-history is refused without --force.
func TestClaimEventsRetentionDisclosed(t *testing.T) {
	scopeHome(t)
	root := purgeRepo(t)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	const svc = "svc"

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", svc, "--path", root, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest rc=%d stderr=%s", rc, errOut.String())
	}
	// Give the purged path a claim with a history event.
	func() {
		st, err := store.BackendForPath(storePath)
		if err != nil {
			t.Fatalf("BackendForPath: %v", err)
		}
		defer st.Close()
		ctx := context.Background()
		if err := st.PutClaims(
			ctx,
			[]core.SemanticClaim{{ServiceID: svc, ClaimID: "c1", Layer: "l", Subject: "secrets", Rule: "r"}},
			[]core.ClaimLink{{ClaimID: "c1", Rel: core.RelCites, TargetRef: core.CiteRef("secrets/token.go", 1, 2), AnchorText: "x"}},
		); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		if err := st.AppendClaimEvent(ctx, core.ClaimEvent{ClaimID: "c1", Verdict: "corroborated", Author: "a", At: time.Now()}); err != nil {
			t.Fatalf("AppendClaimEvent: %v", err)
		}
	}()

	out.Reset()
	errOut.Reset()
	if rc := run([]string{
		"purge", "secrets/**", "--service", svc, "--store", storePath,
		"--reason", "leaked credential", "--probe", purgeCanary, "--yes",
	}, &out, &errOut); rc != 0 {
		t.Fatalf("purge rc=%d stdout=%s stderr=%s", rc, out.String(), errOut.String())
	}
	text := out.String()
	if !strings.Contains(text, "claim_events") || !strings.Contains(strings.ToLower(text), "retained") {
		t.Fatalf("the default run did not disclose the claim_events retention:\n%s", text)
	}
	if !strings.Contains(text, "1") {
		t.Fatalf("the retention was disclosed without its count:\n%s", text)
	}

	// --include-history alone is refused, and names --force.
	out.Reset()
	errOut.Reset()
	if rc := run([]string{
		"purge", "secrets/**", "--service", svc, "--store", storePath,
		"--reason", "r", "--yes", "--include-history",
	}, &out, &errOut); rc == 0 {
		t.Fatal("--include-history without --force was accepted")
	}
	if !strings.Contains(errOut.String(), "--force") {
		t.Fatalf("the refusal does not name --force: %s", errOut.String())
	}
}

// TestPurgeNamesALeftoverMemoriesFile is AM-12 after SR-13 retired anchored
// memory (surface reckoning F-45, ADR-053): nothing writes or reads
// <repo>/.fathom/memories.jsonl any more, but a file left over from before
// the retirement may still QUOTE purged content. A purge with no leftover
// file reports the memories location as checked; one WITH a leftover file
// fails loudly, naming it, rather than reporting a verified purge over a
// file it did not filter (law 3).
func TestPurgeNamesALeftoverMemoriesFile(t *testing.T) {
	scopeHome(t)
	root := purgeRepo(t)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	const svc = "svc"

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", svc, "--path", root, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest rc=%d stderr=%s", rc, errOut.String())
	}
	purge := func() int {
		out.Reset()
		errOut.Reset()
		return run([]string{
			"purge", "secrets/**", "--service", svc, "--store", storePath,
			"--reason", "leaked credential", "--probe", purgeCanary, "--yes",
		}, &out, &errOut)
	}

	// No leftover file: the location is checked and the purge verifies.
	if rc := purge(); rc != 0 {
		t.Fatalf("purge rc=%d stdout=%s stderr=%s", rc, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), string(store.LocMemories)) {
		t.Fatalf("the report does not mention the memories location:\n%s", out.String())
	}

	// A leftover file from before SR-13: the purge refuses to call itself
	// verified and names the file.
	memPath := filepath.Join(root, ".fathom", "memories.jsonl")
	if err := os.MkdirAll(filepath.Dir(memPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memPath, []byte(`{"id":"m-quote","claim":"the token is `+purgeCanary+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc := purge(); rc == 0 {
		t.Fatalf("purge over a leftover memories file verified; stdout=%s", out.String())
	}
	if !strings.Contains(errOut.String()+out.String(), "memories.jsonl") {
		t.Fatalf("the failure does not name the leftover file:\nstdout=%s\nstderr=%s", out.String(), errOut.String())
	}
}

// TestPurgeDryRunChangesNothing: `--dry-run` resolves and shows the file
// list and per-table counts, and leaves the store, the scoped ignore
// directory and the suppression file exactly as they were. Resolution
// before confirmation is phase 1 of §3.3.
func TestPurgeDryRunChangesNothing(t *testing.T) {
	home := scopeHome(t)
	root := purgeRepo(t)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	const svc = "svc"

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", svc, "--path", root, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest rc=%d stderr=%s", rc, errOut.String())
	}
	before := countRefs(t, storePath, svc, "secrets/")

	out.Reset()
	errOut.Reset()
	if rc := run([]string{
		"purge", "secrets/**", "--service", svc, "--store", storePath,
		"--reason", "looking first", "--dry-run", "--json",
	}, &out, &errOut); rc != 0 {
		t.Fatalf("dry-run rc=%d stderr=%s", rc, errOut.String())
	}
	var plan struct {
		Files  []string       `json:"files"`
		Counts map[string]int `json:"counts"`
		Dry    bool           `json:"dry_run"`
	}
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("dry-run json: %v (%s)", err, out.String())
	}
	if !plan.Dry {
		t.Error("the dry-run report does not say it was a dry run")
	}
	if len(plan.Files) == 0 {
		t.Fatalf("the dry run resolved nothing: %s", out.String())
	}
	if plan.Counts["observations"] == 0 {
		t.Errorf("the dry run reported no per-table counts: %+v", plan.Counts)
	}
	if n := countRefs(t, storePath, svc, "secrets/"); n != before {
		t.Fatalf("the dry run deleted rows: %d -> %d", before, n)
	}
	scopedPath, err := connector.ServiceIgnorePath(svc)
	if err != nil {
		t.Fatal(err)
	}
	if s := readFileString(t, scopedPath); s != "" {
		t.Fatalf("the dry run wrote an ingest-time exclusion: %q", s)
	}
	if s := readFileString(t, filepath.Join(home, ".fathom", "suppressions.jsonl")); s != "" {
		t.Fatalf("the dry run wrote a suppression: %q", s)
	}
}

// TestPurgeHistoryRecordsEveryRun: §3.3 phase 4 — the run is written to
// the purge log and `fathom purge history` reads it back, so "what has
// been purged, when, and whether verified" outlives the terminal
// scrollback.
func TestPurgeHistoryRecordsEveryRun(t *testing.T) {
	home := scopeHome(t)
	root := purgeRepo(t)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	const svc = "svc"

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", svc, "--path", root, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest rc=%d stderr=%s", rc, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if rc := run([]string{
		"purge", "secrets/**", "--service", svc, "--store", storePath,
		"--reason", "leaked credential", "--probe", purgeCanary, "--yes",
	}, &out, &errOut); rc != 0 {
		t.Fatalf("purge rc=%d stderr=%s", rc, errOut.String())
	}

	if s := readFileString(t, filepath.Join(home, ".fathom", "purges.jsonl")); s == "" {
		t.Fatal("no purge record was written")
	}
	out.Reset()
	errOut.Reset()
	if rc := run([]string{"purge", "history", "--json"}, &out, &errOut); rc != 0 {
		t.Fatalf("purge history rc=%d stderr=%s", rc, errOut.String())
	}
	var hist []struct {
		Service  string   `json:"service"`
		Patterns []string `json:"patterns"`
		Reason   string   `json:"reason"`
		Verdict  string   `json:"verdict"`
		Files    int      `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &hist); err != nil {
		t.Fatalf("history json: %v (%s)", err, out.String())
	}
	if len(hist) != 1 {
		t.Fatalf("history has %d entries: %s", len(hist), out.String())
	}
	if hist[0].Service != svc || hist[0].Reason != "leaked credential" || hist[0].Verdict != string(store.PurgeVerified) {
		t.Fatalf("history entry is wrong: %+v", hist[0])
	}
	if hist[0].Files == 0 {
		t.Errorf("history records no file count: %+v", hist[0])
	}
}

// TestPurgeRefusesWithoutReason: a purge with no reason is refused before
// anything is touched — the same discipline suppression enforces, applied
// to the operation that cannot be undone.
func TestPurgeRefusesWithoutReason(t *testing.T) {
	scopeHome(t)
	root := purgeRepo(t)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	const svc = "svc"

	var out, errOut bytes.Buffer
	if rc := run([]string{"ingest", "--service", svc, "--path", root, "--store", storePath}, &out, &errOut); rc != 0 {
		t.Fatalf("ingest rc=%d stderr=%s", rc, errOut.String())
	}
	before := countRefs(t, storePath, svc, "secrets/")

	out.Reset()
	errOut.Reset()
	if rc := run([]string{"purge", "secrets/**", "--service", svc, "--store", storePath, "--yes"}, &out, &errOut); rc == 0 {
		t.Fatal("purge without --reason was accepted")
	}
	if n := countRefs(t, storePath, svc, "secrets/"); n != before {
		t.Fatal("a refused purge still deleted rows")
	}

	// And without --yes it must not execute either: §3.3 phase 1 requires
	// confirmation, and a non-interactive stdin is not consent.
	out.Reset()
	errOut.Reset()
	if rc := run([]string{"purge", "secrets/**", "--service", svc, "--store", storePath, "--reason", "r"}, &out, &errOut); rc == 0 {
		t.Fatal("purge executed without confirmation")
	}
	if n := countRefs(t, storePath, svc, "secrets/"); n != before {
		t.Fatal("an unconfirmed purge still deleted rows")
	}
}
