package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/cost"
	"github.com/geoffmilleraz/fathom/internal/memory"
)

// runEditHookWithStdin runs the PostToolUse edit-hook with input on the
// process stdin (cmdEditHook reads os.Stdin directly, so a pipe is the only
// way to feed it) and returns the exit code and stdout.
func runEditHookWithStdin(t *testing.T, input, url string) (int, *bytes.Buffer) {
	t.Helper()
	origStdin := os.Stdin
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = pr
	_, _ = pw.WriteString(input)
	_ = pw.Close()
	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + url}, pr, &out, &errBuf)
	os.Stdin = origStdin
	_ = pr.Close()
	return code, &out
}

// TestMemoryInjectionOrderAndBudget (AM-6): memories join the payload as a
// separate labeled block, ordered tier → specificity → id regardless of
// input order, retired/orphaned never render, and the block keeps only the
// highest-ranked prefix the budget allows — counting what it dropped.
func TestMemoryInjectionOrderAndBudget(t *testing.T) {
	evs := []memory.Evaluation{
		{ID: "ccc", Claim: "pre-scan, symbol anchor.", Tier: core.TierPreScan, State: memory.StateFresh, Specificity: 0, Surfaces: true},
		{ID: "aaa", Claim: "assumed, symbol anchor.", Tier: core.TierAssumed, State: memory.StateFresh, Specificity: 0, Surfaces: true},
		{ID: "bbb", Claim: "pre-scan, path anchor.", Tier: core.TierPreScan, State: memory.StateSuspect, Specificity: 4, Surfaces: true},
		{ID: "rrr", Claim: "retired, must never render.", Tier: core.TierVerified, State: memory.StateRetired, Specificity: 0, Surfaces: false},
		{ID: "ooo", Claim: "orphaned, must never render.", Tier: core.TierVerified, State: memory.StateOrphaned, Specificity: 0, Surfaces: false},
		{ID: "ddd", Claim: "verified, path anchor.", Tier: core.TierVerified, State: memory.StateFresh, Specificity: 4, Surfaces: true},
		{ID: "eee", Claim: "pre-scan, symbol anchor, later id.", Tier: core.TierPreScan, State: memory.StateFresh, Specificity: 0, Surfaces: true},
	}

	// Generous budget: everything surfacing renders, in order, labeled.
	var sb strings.Builder
	sb.WriteString("⚖️ fathom obligations — pkg/foo.go:\n• Sort outputs. [CLAUDE.md#L4-L4, doc-claimed]")
	shown, displaced := injectMemories(&sb, "pkg/foo.go", evs, 10_000)
	if shown != 5 || displaced != 0 {
		t.Fatalf("shown/displaced = %d/%d, want 5/0", shown, displaced)
	}
	out := sb.String()
	if !strings.Contains(out, "\n🧠 fathom memory — pkg/foo.go:") {
		t.Fatalf("missing labeled memory block header:\n%s", out)
	}
	if strings.Contains(out, "retired") || strings.Contains(out, "orphaned") {
		t.Fatalf("retired/orphaned memory rendered:\n%s", out)
	}
	block := out[strings.Index(out, "🧠"):]
	lines := strings.Split(block, "\n")[1:]
	wantOrder := []string{"ddd", "ccc", "eee", "bbb", "aaa"}
	wantLabels := []string{"[verified · fresh]", "[pre-scan · fresh]", "[pre-scan · fresh]", "[pre-scan · suspect ⚠]", "[assumed · fresh]"}
	byID := map[string]memory.Evaluation{}
	for _, e := range evs {
		byID[e.ID] = e
	}
	if len(lines) != len(wantOrder) {
		t.Fatalf("block has %d lines, want %d:\n%s", len(lines), len(wantOrder), block)
	}
	for i, id := range wantOrder {
		if !strings.HasPrefix(lines[i], "• "+wantLabels[i]+" "+byID[id].Claim) {
			t.Errorf("line %d = %q, want %s labeled %s", i, lines[i], id, wantLabels[i])
		}
	}
	// The obligations block above is untouched and the memory block follows it.
	if !strings.HasPrefix(out, "⚖️ fathom obligations — pkg/foo.go:") || strings.Index(out, "⚖️") > strings.Index(out, "🧠") {
		t.Fatalf("memory block must follow the obligations block:\n%s", out)
	}

	// Deterministic: a second call over a differently-ordered copy renders
	// byte-identical text.
	rev := make([]memory.Evaluation, len(evs))
	for i, e := range evs {
		rev[len(evs)-1-i] = e
	}
	var sb2 strings.Builder
	sb2.WriteString("⚖️ fathom obligations — pkg/foo.go:\n• Sort outputs. [CLAUDE.md#L4-L4, doc-claimed]")
	injectMemories(&sb2, "pkg/foo.go", rev, 10_000)
	if sb2.String() != out {
		t.Fatalf("injection is order-sensitive:\n%s\n---\n%s", sb2.String(), out)
	}

	// Tight budget: the highest-ranked prefix survives, the rest is counted.
	var sb3 strings.Builder
	sb3.WriteString("⚖️ fathom obligations — pkg/foo.go:\n• Sort outputs. [CLAUDE.md#L4-L4, doc-claimed]")
	prefixCost := cost.Estimate(sb3.String() + "\n🧠 fathom memory — pkg/foo.go:" + "\n• [verified · fresh] verified, path anchor." + "\n• [pre-scan · fresh] pre-scan, symbol anchor.")
	shown, displaced = injectMemories(&sb3, "pkg/foo.go", evs, prefixCost)
	if shown != 2 || displaced != 3 {
		t.Fatalf("budgeted shown/displaced = %d/%d, want 2/3:\n%s", shown, displaced, sb3.String())
	}
	if !strings.Contains(sb3.String(), "ddd") && !strings.Contains(sb3.String(), "verified, path anchor.") {
		t.Fatalf("highest-ranked memory dropped under budget:\n%s", sb3.String())
	}
	if !strings.Contains(sb3.String(), "⚠️ 3 more memory(ies) not shown (budget)") {
		t.Fatalf("displaced count not named:\n%s", sb3.String())
	}
	if strings.Contains(sb3.String(), "assumed") {
		t.Fatalf("lowest-ranked memory rendered under budget:\n%s", sb3.String())
	}

	// Budget already spent: nothing renders but the count, and only when
	// even that fits.
	var sb4 strings.Builder
	sb4.WriteString(strings.Repeat("• a long obligations payload that spends the whole budget. ", 12))
	spent := cost.Estimate(sb4.String())
	before := sb4.String()
	shown, displaced = injectMemories(&sb4, "pkg/foo.go", evs, spent)
	if shown != 0 || displaced != 5 {
		t.Fatalf("spent shown/displaced = %d/%d, want 0/5", shown, displaced)
	}
	if sb4.String() != before {
		t.Fatalf("a spent budget must not grow the payload:\n%s", sb4.String())
	}

	// No memories: sb is untouched, nothing extra.
	var sb5 strings.Builder
	shown, displaced = injectMemories(&sb5, "pkg/foo.go", nil, 10_000)
	if shown != 0 || displaced != 0 || sb5.Len() != 0 {
		t.Fatalf("no memories should print nothing, got %q", sb5.String())
	}
	// Only non-surfacing memories: same.
	shown, displaced = injectMemories(&sb5, "pkg/foo.go", evs[3:5], 10_000)
	if shown != 0 || displaced != 0 || sb5.Len() != 0 {
		t.Fatalf("retired/orphaned only should print nothing, got %q", sb5.String())
	}

	// Empty payload before injection: the header carries no leading newline.
	var sb6 strings.Builder
	injectMemories(&sb6, "pkg/foo.go", evs[:1], 10_000)
	if !strings.HasPrefix(sb6.String(), "🧠 fathom memory — pkg/foo.go:\n• [pre-scan · fresh]") {
		t.Fatalf("standalone block = %q", sb6.String())
	}
}

// TestEditHookInjectsMemoriesAfterObligations drives the whole hook: a fake
// daemon serves obligations for the file AND file-scoped memory_search
// results; the payload carries the obligations block first and the labeled
// memory block after it.
func TestEditHookInjectsMemoriesAfterObligations(t *testing.T) {
	dir := t.TempDir()
	// The hook only asks for memories when the project has a memories file.
	if err := os.MkdirAll(filepath.Join(dir, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memory.Path(dir), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// The handler runs on the httptest server's goroutines; the test reads
	// memCalls afterwards with no happens-before edge through the HTTP
	// round-trip, so the slice is mutex-guarded.
	var (
		memMu    sync.Mutex
		memCalls []map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var payload map[string]any
		switch req.Params.Name {
		case "obligations":
			payload = map[string]any{
				"service": filepath.Base(dir), "file": "pkg/foo.go",
				"rules": []any{map[string]any{"rule": "Sort all outputs.", "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed"}},
			}
		case "memory_search":
			memMu.Lock()
			memCalls = append(memCalls, req.Params.Arguments)
			memMu.Unlock()
			payload = map[string]any{
				"service": filepath.Base(dir), "file": "pkg/foo.go", "total": 2, "shown": 2,
				"memories": []any{
					map[string]any{"id": "bbb", "claim": "foo.go is read once at startup.", "tier": "doc-claimed", "state": "fresh", "specificity": 1, "surfaces": true},
					map[string]any{"id": "aaa", "claim": "Foo validates before returning.", "tier": "verified", "state": "fresh", "specificity": 0, "surfaces": true},
					map[string]any{"id": "zzz", "claim": "unlabelled — must be dropped.", "specificity": 0, "surfaces": true},
				},
			}
		default:
			payload = map[string]any{}
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": mustJSON(payload)}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, filepath.Join(dir, "pkg", "foo.go"), dir)
	code, out := runEditHookWithStdin(t, hookInput, srv.URL)
	if code != 0 {
		t.Fatalf("edit-hook exit %d", code)
	}
	var env struct {
		Hook struct {
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decode hook output %q: %v", out.String(), err)
	}
	ctx := env.Hook.Context
	memMu.Lock()
	gotCalls := append([]map[string]any(nil), memCalls...)
	memMu.Unlock()
	if len(gotCalls) != 1 || gotCalls[0]["file"] != "pkg/foo.go" {
		t.Fatalf("memory_search calls = %v, want one file-scoped call", gotCalls)
	}
	obl := strings.Index(ctx, "⚖️ fathom obligations — pkg/foo.go:")
	mem := strings.Index(ctx, "\n🧠 fathom memory — pkg/foo.go:")
	if obl != 0 || mem < 0 || mem < obl {
		t.Fatalf("expected obligations then memory block:\n%s", ctx)
	}
	memBlock := ctx[mem:]
	if !strings.Contains(memBlock, "\n• [verified · fresh] Foo validates before returning.\n• [doc-claimed · fresh] foo.go is read once at startup.") {
		t.Fatalf("memory block not ordered/labeled:\n%s", ctx)
	}
	if strings.Contains(ctx, "unlabelled") {
		t.Fatalf("an unlabelled memory reached the payload:\n%s", ctx)
	}
	if cost.Estimate(ctx) > specLineTokenBudget {
		t.Fatalf("payload exceeds the budget: %d tokens", cost.Estimate(ctx))
	}
}

// TestEditHookNoMemoriesFileNoMemoryCall: without a project memories file
// the hook never asks the daemon for memories (the no-memories case costs
// no RPC) and the clean-file contract stays silent.
func TestEditHookNoMemoriesFileNoMemoryCall(t *testing.T) {
	dir := t.TempDir()
	var (
		memMu    sync.Mutex
		memCalls int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name == "memory_search" {
			memMu.Lock()
			memCalls++
			memMu.Unlock()
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": mustJSON(map[string]any{"service": filepath.Base(dir), "file": "pkg/foo.go"})}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, filepath.Join(dir, "pkg", "foo.go"), dir)
	code, out := runEditHookWithStdin(t, hookInput, srv.URL)
	if code != 0 {
		t.Fatalf("edit-hook exit %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("clean file with no memories file: expected silence, got %q", out.String())
	}
	memMu.Lock()
	n := memCalls
	memMu.Unlock()
	if n != 0 {
		t.Fatalf("memory_search called %d times without a memories file", n)
	}
}

// TestEditHookMemoryFetchFailureEmitsErrorBeat: a memory_search failure
// keeps the PAYLOAD silent (hook doctrine — the obligations block still
// renders, no memory block, exit 0) but is never invisible to the
// instrument: the hook emits the same edit-hook/error beat a failed
// obligations fetch does, tagged target=memories.
func TestEditHookMemoryFetchFailureEmitsErrorBeat(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memory.Path(dir), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	beatLog := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name == "memory_search" {
			http.Error(w, "memory index unavailable", http.StatusInternalServerError)
			return
		}
		payload := map[string]any{
			"service": filepath.Base(dir), "file": "pkg/foo.go",
			"rules": []any{map[string]any{"rule": "Sort all outputs.", "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed"}},
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": mustJSON(payload)}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, filepath.Join(dir, "pkg", "foo.go"), dir)
	code, out := runEditHookWithStdin(t, hookInput, srv.URL)
	if code != 0 {
		t.Fatalf("edit-hook exit %d", code)
	}
	var env struct {
		Hook struct {
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decode hook output %q: %v", out.String(), err)
	}
	if !strings.HasPrefix(env.Hook.Context, "⚖️ fathom obligations — pkg/foo.go:") || strings.Contains(env.Hook.Context, "🧠") {
		t.Fatalf("payload must keep the obligations block and stay silent on memories:\n%s", env.Hook.Context)
	}

	beats, err := readRetroBeats(beatLog)
	if err != nil {
		t.Fatalf("reading beat log: %v", err)
	}
	errorBeats := 0
	for _, b := range beats {
		if b.Channel == "edit-hook" && b.Kind == "error" {
			errorBeats++
			if b.Meta["target"] != "memories" || b.Meta["phase"] != "post" {
				t.Errorf("error beat meta = %+v, want target=memories phase=post", b.Meta)
			}
		}
	}
	if errorBeats != 1 {
		t.Fatalf("edit-hook/error beats = %d, want exactly 1 (the memories fetch)", errorBeats)
	}
}
