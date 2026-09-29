package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── repoIdentity (RC-31) ────────────────────────────────────────────────

func discoGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	return dir
}

func setOrigin(t *testing.T, repo, url string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", repo, "remote", "add", "origin", url).CombinedOutput(); err != nil {
		t.Fatalf("git remote add origin: %v %s", err, out)
	}
}

func TestRepoIdentity_OriginURLNormalizesAcrossForms(t *testing.T) {
	forms := []string{
		"https://github.com/Geoff/Fathom.git",
		"git@github.com:Geoff/Fathom.git",
		"ssh://git@github.com/Geoff/Fathom/",
	}
	var want string
	for i, url := range forms {
		repo := discoGitRepo(t)
		setOrigin(t, repo, url)
		got := repoIdentity(repo)
		if i == 0 {
			want = got
		} else if got != want {
			t.Errorf("repoIdentity(%q) = %q, want %q (same as form 0)", url, got, want)
		}
	}
	if want != "github.com/geoff/fathom" {
		t.Errorf("normalized identity = %q, want \"github.com/geoff/fathom\"", want)
	}
}

func TestRepoIdentity_FallsBackToLowercasedPath(t *testing.T) {
	dir := t.TempDir() // no git repo at all
	got := repoIdentity(dir)
	want := strings.ToLower(dir)
	if got != want {
		t.Errorf("repoIdentity(%q) = %q, want %q", dir, got, want)
	}
}

func TestThreadScope_IdentitySeparatesSameBasenameRepos(t *testing.T) {
	// F5 (docs/vision/28-receipts.md): two unrelated repos sharing a
	// directory basename must never merge under the old basename tag.
	parentA := t.TempDir()
	parentB := t.TempDir()
	repoA := filepath.Join(parentA, "worker-1")
	repoB := filepath.Join(parentB, "worker-1")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoB, 0o755); err != nil {
		t.Fatal(err)
	}
	idA, idB := repoIdentity(repoA), repoIdentity(repoB)
	if idA == idB {
		t.Fatalf("two unrelated repos sharing a basename got the same identity: %q", idA)
	}
	scopeA := threadScope{identity: idA, repo: "worker-1"}
	itemFromB := threadItem{Pointers: map[string]string{"repo_identity": idB}}
	if scopeA.owns(itemFromB) {
		t.Errorf("scopeA must not own an item tagged with repo B's identity")
	}
	itemFromA := threadItem{Pointers: map[string]string{"repo_identity": idA}}
	if !scopeA.owns(itemFromA) {
		t.Errorf("scopeA must own an item tagged with its own identity")
	}
}

func TestThreadScope_LegacyBasenameTagStillOwned(t *testing.T) {
	// RC-31: an item filed before this change (no repo_identity key) must
	// not be silently hidden — the old basename rule still applies.
	scope := threadScope{identity: "github.com/x/y", repo: "y"}
	legacy := threadItem{Pointers: map[string]string{"repo": "y"}}
	if !scope.owns(legacy) {
		t.Errorf("a legacy (pre-RC-31) item tagged with the old basename must still be owned")
	}
}

// ─── session windows ─────────────────────────────────────────────────────

func TestSessionWindows_RecordAndLookup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	now := time.Now().UTC()
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s1", Cwd: root, Service: "fathom", Started: now.Format(time.RFC3339)})
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s2", Cwd: root, Service: "fathom", Started: now.Add(time.Minute).Format(time.RFC3339)})

	got, ok := sessionStartFor(root, "s1")
	if !ok {
		t.Fatal("sessionStartFor(s1) not found")
	}
	if !got.Truncate(time.Second).Equal(now.Truncate(time.Second)) {
		t.Errorf("sessionStartFor(s1) = %v, want ~%v", got, now)
	}
	if _, ok := sessionStartFor(root, "unknown"); ok {
		t.Errorf("sessionStartFor(unknown) should not resolve")
	}
	windows := readSessionWindows(root)
	if len(windows) != 2 {
		t.Fatalf("readSessionWindows = %d rows, want 2", len(windows))
	}
}

// ─── gatherDiscoveryPointers ─────────────────────────────────────────────

func writeDiscoCallLogFixture(t *testing.T, path string, rows []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGatherDiscoveryPointers_RanksByRecencyThenFrequency(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "mcp-calls.jsonl")
	now := time.Now().UTC()
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Add(-time.Hour).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["internal/mcp.registerReferences"]}`,
		`{"ts":"` + now.Add(-2*time.Minute).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["internal/store.PutObservations"]}`,
		`{"ts":"` + now.Add(-time.Minute).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["internal/store.PutObservations"]}`,
		`{"ts":"` + now.Format(time.RFC3339Nano) + `","service":"other","pointers":["should/never.Appear"]}`,
	})
	scope := threadScopeFor(root)
	got := gatherDiscoveryPointers(logPath, "fathom", time.Time{}, scope, root)
	if len(got) != 2 {
		t.Fatalf("gatherDiscoveryPointers = %d pointers, want 2 (service-scoped): %+v", len(got), got)
	}
	if got[0].Pointer != "internal/store.PutObservations" || got[0].Count != 2 {
		t.Errorf("rank[0] = %+v, want internal/store.PutObservations count=2 (most recent + most frequent)", got[0])
	}
	if got[1].Pointer != "internal/mcp.registerReferences" {
		t.Errorf("rank[1] = %+v, want internal/mcp.registerReferences", got[1])
	}
}

func TestGatherDiscoveryPointers_HonoursSince(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "mcp-calls.jsonl")
	now := time.Now().UTC()
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Add(-3*time.Hour).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["too/old.Pointer"]}`,
		`{"ts":"` + now.Format(time.RFC3339Nano) + `","service":"fathom","pointers":["recent.Pointer"]}`,
	})
	scope := threadScopeFor(root)
	got := gatherDiscoveryPointers(logPath, "fathom", now.Add(-time.Hour), scope, root)
	if len(got) != 1 || got[0].Pointer != "recent.Pointer" {
		t.Fatalf("gatherDiscoveryPointers with --since = %+v, want just recent.Pointer", got)
	}
}

func TestGatherDiscoveryPointers_IncludesFiledThreadPoints(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "mcp-calls.jsonl") // absent: only thread items contribute
	scope := threadScopeFor(root)
	now := time.Now().UTC()
	if err := appendThreadItem(threadItem{
		SchemaVersion: threadSchemaVersion, ID: "th-filed", Thread: "fathom", Kind: "side-task",
		Text: "found the registration site", FiledAt: now.Format(time.RFC3339),
		Pointers: map[string]string{"repo_identity": scope.identity, discoveriesPointerKey: "internal/mcp.registerDiscoveries"},
	}); err != nil {
		t.Fatal(err)
	}
	got := gatherDiscoveryPointers(logPath, "fathom", time.Time{}, scope, root)
	if len(got) != 1 || got[0].Pointer != "internal/mcp.registerDiscoveries" {
		t.Fatalf("gatherDiscoveryPointers from filed thread items = %+v, want [internal/mcp.registerDiscoveries]", got)
	}
}

// TestGatherDiscoveryPointers_ObservedRowsFillAZeroCallLogSession is RC-56's
// load-bearing proof: reuse-pilot-2 found the wired planners reaching for
// grep and Read, never a fathom tool, so the call log held no pointers and
// every worker brief was empty. A session with ZERO MCP calls but three
// observed reads must still yield a non-empty brief.
func TestGatherDiscoveryPointers_ObservedRowsFillAZeroCallLogSession(t *testing.T) {
	pinNoEnclosingRepo(t)
	threadEnv(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "mcp-calls.jsonl") // never created: zero MCP calls
	now := time.Now().UTC()
	recordObservedDiscovery(root, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: now.Add(-3 * time.Minute).Format(time.RFC3339Nano),
		SessionID: "s1", Tool: "read", Pointers: []string{"internal/core/model.go"},
	})
	recordObservedDiscovery(root, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
		SessionID: "s1", Tool: "grep", Pointers: []string{"internal/mcp"},
	})
	recordObservedDiscovery(root, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: now.Add(-time.Minute).Format(time.RFC3339Nano),
		SessionID: "s1", Tool: "read", Pointers: []string{"cmd/fathom/main.go"},
	})

	scope := threadScopeFor(root)
	got := gatherDiscoveryPointers(logPath, "fathom", time.Time{}, scope, root)
	if len(got) != 3 {
		t.Fatalf("gatherDiscoveryPointers with zero call-log rows = %d pointers, want 3 (from observation alone): %+v", len(got), got)
	}
	for _, p := range got {
		if p.FromIndex {
			t.Errorf("pointer %q: FromIndex = true, want false (never established by an MCP call)", p.Pointer)
		}
		if p.ObservedVerb == "" {
			t.Errorf("pointer %q: ObservedVerb unset, want \"read\" or \"grepped\"", p.Pointer)
		}
	}
}

// ─── renderOneDiscoveryLine ──────────────────────────────────────────────

func TestRenderOneDiscoveryLine_ResolvesAFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model.go"), []byte("package core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	line, res := renderOneDiscoveryLine(nil, nil, "model.go", 3, true, "")
	if res != discoveryResolved || !strings.HasPrefix(line, "read model.go") || !strings.Contains(line, "(3)") {
		t.Errorf("renderOneDiscoveryLine(file) = (%q, %v), want a \"read model.go ... (3)\" line", line, res)
	}
}

func TestRenderOneDiscoveryLine_UnresolvedIsStale(t *testing.T) {
	line, res := renderOneDiscoveryLine(nil, nil, "internal/nope.GoneNow", 1, true, "")
	if res != discoveryStale {
		t.Fatalf("expected discoveryStale, got %v: %q", res, line)
	}
	if !strings.Contains(line, "stale") {
		t.Errorf("renderOneDiscoveryLine unresolved = %q, want it to say \"stale\"", line)
	}
}

// MAJOR fix: an unreachable/erroring index must never render as "stale" —
// that word is the index's own positive claim the pointer is gone, which
// it never made when it was never consulted.
func TestRenderOneDiscoveryLine_IndexUnreachableIsNotStale(t *testing.T) {
	unreachable := errors.New(`probing daemon for service "fathom": dial tcp: connection refused`)
	line, res := renderOneDiscoveryLine(nil, unreachable, "internal/nope.GoneNow", 1, true, "")
	if res != discoveryUnreachable {
		t.Fatalf("expected discoveryUnreachable, got %v: %q", res, line)
	}
	if strings.Contains(line, "stale") {
		t.Errorf("renderOneDiscoveryLine with an unreachable index = %q, must not say \"stale\"", line)
	}
	if !strings.Contains(line, "unreachable") {
		t.Errorf("renderOneDiscoveryLine with an unreachable index = %q, want it to say \"unreachable\"", line)
	}
}

// TestRenderOneDiscoveryLine_ObservedOnlyRendersWithItsVerb is RC-56 (vision
// 28 §4): a pointer the discovery OBSERVER saw but no index tool ever
// established — here, a directory a Grep searched, which resolves as
// neither a file on disk nor a symbol — renders with its own verb instead
// of the wrong "stale (unresolved)" (that word is the index's claim the
// pointer is GONE, which it never made: it was never established by the
// index in the first place).
func TestRenderOneDiscoveryLine_ObservedOnlyRendersWithItsVerb(t *testing.T) {
	line, res := renderOneDiscoveryLine(nil, nil, "pkg/cmd/run", 0, false, "grepped")
	if res != discoveryResolved {
		t.Fatalf("expected discoveryResolved, got %v: %q", res, line)
	}
	if line != "grepped pkg/cmd/run" {
		t.Errorf("renderOneDiscoveryLine(observed-only) = %q, want %q", line, "grepped pkg/cmd/run")
	}
	if strings.Contains(line, "stale") {
		t.Errorf("renderOneDiscoveryLine(observed-only) = %q, must never say \"stale\"", line)
	}
}

// ─── fathom discoveries (RC-26) ──────────────────────────────────────────

func TestCmdDiscoveries_RendersFromCallLog(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	now := time.Now().UTC()
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Format(time.RFC3339Nano) + `","service":"fathom","pointers":["some/pkg.Thing"]}`,
	})
	var out, errb bytes.Buffer
	code := cmdDiscoveries([]string{"--root", root, "--service", "fathom", "--call-log", logPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("cmdDiscoveries exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "some/pkg.Thing") {
		t.Errorf("cmdDiscoveries output = %q, want it to name the pointer", out.String())
	}
}

// TestCmdDiscoveries_RendersObservedOnlySession is RC-56's end-to-end
// proof: a session with an EMPTY call log (the planner never called an
// MCP tool) still gets a non-empty `fathom discoveries` brief from what
// the counting-only observer saw it read and grep.
func TestCmdDiscoveries_RendersObservedOnlySession(t *testing.T) {
	pinNoEnclosingRepo(t)
	threadEnv(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl") // never created
	now := time.Now().UTC()
	recordObservedDiscovery(root, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
		SessionID: "s1", Tool: "read", Pointers: []string{"internal/core/model.go"},
	})
	recordObservedDiscovery(root, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: now.Add(-time.Minute).Format(time.RFC3339Nano),
		SessionID: "s1", Tool: "grep", Pointers: []string{"pkg/cmd/run"},
	})
	var out, errb bytes.Buffer
	code := cmdDiscoveries([]string{"--root", root, "--service", "fathom", "--call-log", logPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("cmdDiscoveries exit %d: %s", code, errb.String())
	}
	rendered := out.String()
	t.Logf("fathom discoveries (observed-only session):\n%s", rendered)
	if !strings.Contains(rendered, "read internal/core/model.go") {
		t.Errorf("cmdDiscoveries output = %q, want a \"read internal/core/model.go\" line", rendered)
	}
	if !strings.Contains(rendered, "grepped pkg/cmd/run") {
		t.Errorf("cmdDiscoveries output = %q, want a \"grepped pkg/cmd/run\" line", rendered)
	}
}

func TestCmdDiscoveries_JSONShape(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	now := time.Now().UTC()
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Format(time.RFC3339Nano) + `","service":"fathom","pointers":["some/pkg.Thing"]}`,
	})
	var out, errb bytes.Buffer
	code := cmdDiscoveries([]string{"--root", root, "--service", "fathom", "--call-log", logPath, "--json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("cmdDiscoveries --json exit %d: %s", code, errb.String())
	}
	var payload struct {
		Pointers []struct {
			Pointer string `json:"pointer"`
		} `json:"pointers"`
		Rendered string `json:"rendered"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v (%s)", err, out.String())
	}
	if len(payload.Pointers) != 1 || payload.Pointers[0].Pointer != "some/pkg.Thing" {
		t.Errorf("json pointers = %+v, want [some/pkg.Thing]", payload.Pointers)
	}
}

// ─── subagentDiscoveries (RC-28) ─────────────────────────────────────────

func TestSubagentDiscoveries_EmptyWhenNothingEstablished(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	if got := subagentDiscoveries("http://127.0.0.1:1", root, "fathom", "parent-1", "", 0, time.Now()); got != "" {
		t.Errorf("subagentDiscoveries with no call log = %q, want \"\"", got)
	}
}

func TestSubagentDiscoveries_UsesParentSessionStart(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	now := time.Now().UTC()
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "parent-1", Cwd: root, Service: "fathom", Started: now.Add(-30 * time.Minute).Format(time.RFC3339)})
	logPath := callLogPathFor(root)
	writeDiscoCallLogFixture(t, logPath, []string{
		// before the parent's own start: must be excluded.
		`{"ts":"` + now.Add(-time.Hour).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["before.Start"]}`,
		// after the parent's start: must be included.
		`{"ts":"` + now.Add(-time.Minute).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["after.Start"]}`,
	})
	got := subagentDiscoveries("http://127.0.0.1:1", root, "fathom", "parent-1", "", 0, now)
	if !strings.Contains(got, "after.Start") {
		t.Errorf("subagentDiscoveries = %q, want it to include after.Start", got)
	}
	if strings.Contains(got, "before.Start") {
		t.Errorf("subagentDiscoveries = %q, must not include a pointer from before the parent's start", got)
	}
	if !strings.HasPrefix(got, "last session established:") {
		t.Errorf("subagentDiscoveries = %q, want the \"last session established:\" lead line", got)
	}
}

// fakeSlowResolverDaemon answers list_services immediately (so the index
// is reachable and the service resolves) but sleeps past every test's
// deadline on symbol_context — the CRITICAL-fix regression fixture: a
// resolver that never returns fast enough must never be allowed to block
// the whole SubagentStart hook.
func fakeSlowResolverDaemon(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Params.Name {
		case "list_services":
			inner := `{"services":[{"service":"fathom","symbol_count":10,"languages":["go"]}]}`
			env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
			_ = json.NewEncoder(w).Encode(env)
		case "symbol_context":
			// Never answers on its own (the fixture's whole point): it holds
			// until the client gives up, so a caller that returned did so
			// at its own deadline, and the server's Close() is not kept
			// waiting on this handler once the client has left.
			<-r.Context().Done()
		default:
			http.Error(w, "unknown tool", 400)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// CRITICAL fix (finding 1): subagentDiscoveries must return within its own
// deadline even when the index is reachable but every symbol lookup hangs
// — the base primer sits ahead of this call in subagentPrimer and must
// never be starved by a slow resolver.
func TestSubagentDiscoveries_ReturnsWithinDeadlineWhenIndexIsSlow(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	now := time.Now().UTC()
	logPath := callLogPathFor(root)
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Format(time.RFC3339Nano) + `","service":"fathom","pointers":["pkg.SlowSymbol"]}`,
	})
	srv := fakeSlowResolverDaemon(t)

	var got string
	returnsUnaided(t, "subagentDiscoveries against an index that never answers", func() {
		got = subagentDiscoveries(srv.URL, root, "fathom", "parent-1", "", 0, now)
	})
	if got != "" {
		t.Errorf("subagentDiscoveries past its deadline = %q, want \"\" so the constant primer ships alone", got)
	}
}

// CRITICAL fix (finding 1): the full SubagentStart primer — identity line
// included — must still ship, and on time, when the discoveries brief's
// own index resolution hangs.
func TestSubagentPrimer_ShipsAloneWithIdentityIntactWhenDiscoveriesIndexIsSlow(t *testing.T) {
	threadEnv(t)
	// indexedServiceFor derives the service name from projDir's own
	// basename absent a "path" field on list_services' response — name
	// the root "fathom" so it matches fakeSlowResolverDaemon's fixed
	// service name without needing that extra plumbing.
	root := filepath.Join(t.TempDir(), "fathom")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	now := time.Now().UTC()
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "parent-1", Cwd: root, Service: "fathom", Started: now.Add(-time.Hour).Format(time.RFC3339)})
	logPath := callLogPathFor(root)
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Format(time.RFC3339Nano) + `","service":"fathom","pointers":["pkg.SlowSymbol"]}`,
	})
	srv := fakeSlowResolverDaemon(t)

	var primer string
	returnsUnaided(t, "subagentPrimer against a discoveries index that never answers", func() {
		primer = subagentPrimer(srv.URL, root, "parent-1", "")
	})
	if !strings.Contains(primer, `indexed service "fathom"`) {
		t.Errorf("subagentPrimer with a slow discoveries index = %q, want the identity line intact", primer)
	}
	if strings.Contains(primer, "last session established:") {
		t.Errorf("subagentPrimer with a slow discoveries index = %q, must not carry a discoveries brief past its deadline", primer)
	}
}

// ─── sessionEndPointers (RC-29) ──────────────────────────────────────────

func TestSessionEndPointers_TopEightSinceSessionStart(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	now := time.Now().UTC()
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s1", Cwd: root, Service: "", Started: now.Add(-10 * time.Minute).Format(time.RFC3339)})
	logPath := callLogPathFor(root)
	var rows []string
	for i := 0; i < 10; i++ {
		rows = append(rows, `{"ts":"`+now.Add(-time.Duration(9-i)*time.Second).Format(time.RFC3339Nano)+`","pointers":["p`+string(rune('A'+i))+`"]}`)
	}
	writeDiscoCallLogFixture(t, logPath, rows)
	got := sessionEndPointers(root, "s1", now)
	if len(got) != sessionEndPointersMax {
		t.Fatalf("sessionEndPointers returned %d, want %d (capped)", len(got), sessionEndPointersMax)
	}
}

func TestSessionEndPointers_NilWhenNothingEstablished(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	if got := sessionEndPointers(root, "no-such-session", time.Now()); got != nil {
		t.Errorf("sessionEndPointers = %v, want nil", got)
	}
}

// ─── routes (RC-30) ──────────────────────────────────────────────────────

func TestRoutedPointers_RequiresTwoDistinctSessions(t *testing.T) {
	now := time.Now().UTC()
	windows := []sessionWindowRow{
		{SessionID: "s1", Started: now.Add(-2 * time.Hour).Format(time.RFC3339)},
		{SessionID: "s2", Started: now.Add(-time.Hour).Format(time.RFC3339)},
	}
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	writeDiscoCallLogFixture(t, logPath, []string{
		// established during s1 AND again during s2: a route.
		`{"ts":"` + now.Add(-90*time.Minute).Format(time.RFC3339Nano) + `","pointers":["pkg.Routed"]}`,
		`{"ts":"` + now.Add(-30*time.Minute).Format(time.RFC3339Nano) + `","pointers":["pkg.Routed"]}`,
		// established twice, but both within s2: not a route.
		`{"ts":"` + now.Add(-29*time.Minute).Format(time.RFC3339Nano) + `","pointers":["pkg.OnceOnly"]}`,
		`{"ts":"` + now.Add(-28*time.Minute).Format(time.RFC3339Nano) + `","pointers":["pkg.OnceOnly"]}`,
	})
	routed := routedPointers(logPath, "", windows, now.Add(-3*time.Hour), root)
	if len(routed) != 1 || routed[0].Pointer != "pkg.Routed" || routed[0].Sessions != 2 {
		t.Fatalf("routedPointers = %+v, want exactly [pkg.Routed sessions=2]", routed)
	}
}

// TestRoutedPointers_CountsObservedRows is RC-56 item 4 ("fathom routes
// counts observed pointers too"): a pointer named only by the discovery
// observer, in two distinct sessions (attributed directly by each row's
// own session_id — no sessionForTimestamp approximation needed), crosses
// the same two-distinct-session bar a call-log pointer does.
func TestRoutedPointers_CountsObservedRows(t *testing.T) {
	pinNoEnclosingRepo(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl") // never created: zero MCP calls
	now := time.Now().UTC()
	recordObservedDiscovery(root, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: now.Add(-2 * time.Hour).Format(time.RFC3339Nano),
		SessionID: "s1", Tool: "grep", Pointers: []string{"internal/mcp"},
	})
	recordObservedDiscovery(root, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: now.Add(-time.Hour).Format(time.RFC3339Nano),
		SessionID: "s2", Tool: "grep", Pointers: []string{"internal/mcp"},
	})
	routed := routedPointers(logPath, "", nil, now.Add(-3*time.Hour), root)
	if len(routed) != 1 || routed[0].Pointer != "internal/mcp" || routed[0].Sessions != 2 {
		t.Fatalf("routedPointers(observed) = %+v, want exactly [internal/mcp sessions=2]", routed)
	}
	if routed[0].FromIndex {
		t.Errorf("routed[0].FromIndex = true, want false (never an MCP call)")
	}
	if routed[0].ObservedVerb != "grepped" {
		t.Errorf("routed[0].ObservedVerb = %q, want \"grepped\"", routed[0].ObservedVerb)
	}
}

func TestRoutePackagePrefix_FileVsSymbol(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if got := routePackagePrefix("a.go"); got != "." && got != "(root)" {
		t.Errorf("routePackagePrefix(a.go) = %q", got)
	}
	if got := routePackagePrefix("internal/mcp.registerDiscoveries"); got != "internal/mcp" {
		t.Errorf("routePackagePrefix(symbol) = %q, want internal/mcp", got)
	}
}
