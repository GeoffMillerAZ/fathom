package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// buildSessionContext is sessionContext's line alone, for the tests that
// pin the primer's text.
func buildSessionContext(baseURL, projDir string) string {
	line, _ := sessionContext(baseURL, projDir)
	return line
}

// composeContinuityAndCustodian is composePrimer under an unread registry
// (every intent-ledger row kept), for the tests that pin the primer's shape.
func composeContinuityAndCustodian(identity, projDir string, now time.Time) (string, int, string) {
	return composePrimer(identity, projDir, now, unreadRoots)
}

// countingIdentityDaemon answers the SessionStart hook's requests with one
// service whose recorded root is servicePath, counting every request by
// path and every list_services call.
type countingIdentityDaemon struct {
	mu           sync.Mutex
	paths        map[string]int
	listServices int
}

func (d *countingIdentityDaemon) snapshot() (map[string]int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]int, len(d.paths))
	for k, v := range d.paths {
		out[k] = v
	}
	return out, d.listServices
}

func newCountingIdentityDaemon(t *testing.T, service, servicePath string) (*httptest.Server, *countingIdentityDaemon) {
	t.Helper()
	d := &countingIdentityDaemon{paths: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		d.mu.Lock()
		d.paths[r.URL.Path]++
		if bytes.Contains(body, []byte(`"list_services"`)) {
			d.listServices++
		}
		d.mu.Unlock()
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte("ok"))
		case "/identity":
			_ = json.NewEncoder(w).Encode(identityResult{Schema: 1, Service: service, Root: servicePath, MatchedBy: identityByRoot, Symbols: 42, Path: servicePath})
		case "/rpc":
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"services\":[{\"service\":%q,\"symbol_count\":42,\"path\":%q}]}"}]}}`, service, servicePath)
		case "/rpc/agent":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"nodes\":[{\"name\":\"BuildModel\"}]}"}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, d
}

// writeIntentLedger writes rows to a fresh intent ledger and points
// FATHOM_INTENT_LEDGER at it.
func writeIntentLedger(t *testing.T, rows []intent.LedgerRow) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "intent-ledger.jsonl")
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, r := range rows {
		r.SchemaVersion = intent.LedgerSchemaVersion
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, path, b.String())
	t.Setenv("FATHOM_INTENT_LEDGER", path)
	return path
}

// runSessionStart runs the SessionStart hook for projDir against srv and
// returns the injected context.
func runSessionStart(t *testing.T, srv *httptest.Server, projDir string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := cmdSessionHook([]string{"--url", srv.URL, "--dir", projDir}, strings.NewReader(`{"session_id":"s-pe25","source":"startup"}`), &out, &errb); code != 0 {
		t.Fatalf("session-hook exit %d: %s", code, errb.String())
	}
	var env struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decoding hook JSON: %v\n%s", err, out.String())
	}
	return env.HookSpecificOutput.AdditionalContext
}

// requestLine renders per-path counts in a stable order.
func requestLine(paths map[string]int) string {
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, paths[k]))
	}
	return strings.Join(parts, " ")
}

// TestAttentionLineReadsRegisteredRootsWithNoAddedHookWork pins PE-25's hook
// rule by counts, never a wall clock: the SessionStart hook decides whether
// projDir is a registered root from the identity it already resolved. Its
// requests to the daemon are the same, path by path, whether the intent
// ledger holds rows for projDir or none, and it never lists the services;
// the intent ledger is read by the same two fragment reads as before (no
// new scan), narrowed to projDir before the one root is canonicalized.
// A registered root's debt is spoken; a worktree's is ignored.
func TestAttentionLineReadsRegisteredRootsWithNoAddedHookWork(t *testing.T) {
	t.Setenv("FATHOM_IDENT_NO_CACHE", "1")
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	threadEnv(t)
	t.Setenv("TMPDIR", t.TempDir()) // the fixture's roots are not under the OS temp dir
	base := t.TempDir()
	reg := filepath.Join(base, "repos", "app")
	worktree := filepath.Join(reg, ".claude", "worktrees", "w1")
	for _, d := range []string{reg, worktree} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	debt := func(root string, n int) intent.LedgerRow {
		return intent.LedgerRow{Kind: intent.KindDebt, RepoRoot: root, At: now, ByKind: map[string]int{"U1": n}}
	}

	// A registered root: its recorded debt is spoken.
	srv, d := newCountingIdentityDaemon(t, "app", reg)
	writeIntentLedger(t, []intent.LedgerRow{debt(reg, 3), debt(worktree, 9)})
	got := runSessionStart(t, srv, reg)
	if !strings.Contains(got, "unproven 3") || strings.Contains(got, "unproven 9") {
		t.Fatalf("registered root: the attention line must carry its own debt (3), got:\n%s", got)
	}
	withRows, lists := d.snapshot()
	if lists != 0 {
		t.Fatalf("the hook listed the services %d time(s); it must decide from the identity it resolved", lists)
	}

	// The same hook with no intent rows makes exactly the same requests.
	srv2, d2 := newCountingIdentityDaemon(t, "app", reg)
	writeIntentLedger(t, nil)
	if got := runSessionStart(t, srv2, reg); strings.Contains(got, "unproven") {
		t.Fatalf("an empty intent ledger spoke: %s", got)
	}
	without, lists2 := d2.snapshot()
	if requestLine(withRows) != requestLine(without) || lists2 != 0 {
		t.Fatalf("requests with rows %q, without %q (list_services %d): the filter added daemon work", requestLine(withRows), requestLine(without), lists2)
	}

	// A worktree under the registered root is not registered: its debt is
	// ignored, with the same requests again.
	srv3, d3 := newCountingIdentityDaemon(t, "app", reg)
	writeIntentLedger(t, []intent.LedgerRow{debt(reg, 3), debt(worktree, 9)})
	if got := runSessionStart(t, srv3, worktree); strings.Contains(got, "unproven") {
		t.Fatalf("a worktree's debt was spoken: %s", got)
	}
	inWorktree, lists3 := d3.snapshot()
	if requestLine(inWorktree) != requestLine(without) || lists3 != 0 {
		t.Fatalf("worktree requests %q, want %q (list_services %d)", requestLine(inWorktree), requestLine(without), lists3)
	}
}

// TestAttentionLedgerRootsNeedNoDaemon pins the decision itself: it reads
// only the identity it is handed. No identity is an unread registry (every
// row kept, the reason named); an identity with no service registers
// nothing; a service's recorded root registers exactly that root.
func TestAttentionLedgerRootsNeedNoDaemon(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	base := t.TempDir()
	reg := filepath.Join(base, "app")
	if err := os.MkdirAll(reg, 0o755); err != nil {
		t.Fatal(err)
	}
	if r := attentionLedgerRoots(reg, nil); r.Read || r.Excludes(reg) || r.Note == "" {
		t.Fatalf("no identity: %+v, want unread, excluding nothing, with a reason", r)
	}
	if r := attentionLedgerRoots(reg, &identityResult{Schema: 1}); !r.Read || !r.Excludes(reg) {
		t.Fatalf("not an indexed service: %+v, want read and excluding the root", r)
	}
	if r := attentionLedgerRoots(reg, &identityResult{Schema: 1, Service: "app", Path: reg + "/"}); !r.Registered(reg) || r.Registered(filepath.Join(reg, "sub")) {
		t.Fatalf("a service's root: %v, want exactly that root", r.List())
	}
}

// TestIntentLedgerCmdReadersNameIgnoredRows pins the verb-side PE-25
// readers: doctor's intent rows read the registered roots' clock and
// baseline and name what they ignored; the audit JSON names it too.
func TestIntentLedgerCmdReadersNameIgnoredRows(t *testing.T) {
	base := t.TempDir()
	reg := filepath.Join(base, "repos", "app")
	scratch := filepath.Join(base, "scratch")
	clone := filepath.Join(scratch, "clone")
	for _, dir := range []string{reg, clone} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	writeIntentLedger(t, []intent.LedgerRow{
		{Kind: intent.KindClock, RepoRoot: clone, At: now, BaselineNote: "a scratch clone's baseline"},
		{Kind: intent.KindDebt, RepoRoot: clone, At: now, ByKind: map[string]int{"U1": 4}},
	})
	roots := ledgerroots.RegisteredRootsWith([]string{reg, clone}, []string{scratch})
	mustWrite(t, filepath.Join(clone, "docs", "intent", "README.md"), "# Router\n\ndocs/intent/invariants.md is the living doc.\n")
	mustWrite(t, filepath.Join(clone, "docs", "intent", "invariants.md"), "# Invariants\n")
	row := checkIntent(clone, roots)
	if !strings.Contains(row.Reason, "2 intent-ledger rows under 1 unregistered root (2 under a scratch dir) ignored") || !strings.Contains(row.Reason, "clock not started") {
		t.Fatalf("doctor intent row: %q", row.Reason)
	}
	payload := withLedgerIgnored(map[string]any{}, ledgerroots.Ignored{Rows: 2, Roots: []string{clone}, ScratchRows: 2})
	if payload["ledger_ignored"] != "2 intent-ledger rows under 1 unregistered root (2 under a scratch dir) ignored" {
		t.Fatalf("audit JSON: %v", payload)
	}
}
