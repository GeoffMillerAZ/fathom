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
	"sync/atomic"
	"testing"
)

// ─── OP-3: edit-hook refuses files outside every candidate root ─────────────

// hookStdin pipes hookInput into os.Stdin for the duration of the edit-hook
// call and restores the original afterward — the same dance every hook test
// in this package uses (cmdEditHook reads os.Stdin directly; runWithStdin's
// injected reader is for other subcommands).
func hookStdin(t *testing.T, hookInput string) {
	t.Helper()
	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdin = r
	if _, err := w.WriteString(hookInput); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	w.Close()
	t.Cleanup(func() {
		os.Stdin = orig
		r.Close()
	})
}

// TestEditHookSilentWhenFileOutsideAllCandidateRoots is the O8 addendum
// regression: a file that does not relativize under ANY service-candidate
// root (here, a path with no relation whatsoever to the project dir) must
// never reach the daemon with its raw absolute path — it must exit silently,
// exactly like every other hook-doctrine silent path. Before OP-3 the
// candidate loop fell back to the raw path and still fired an obligations
// lookup for a file outside every indexed repo.
func TestEditHookSilentWhenFileOutsideAllCandidateRoots(t *testing.T) {
	// The premise "no repo above these paths" must hold even when TMPDIR
	// points inside a git work tree (the Taskfile routes TMPDIR to
	// workspace/tmp/gobuild): pin git's upward discovery at the temp root so
	// serviceCandidates' rev-parse can never resolve the enclosing repo.
	ceil := os.TempDir()
	if resolved, err := filepath.EvalSymlinks(ceil); err == nil {
		ceil = resolved
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", ceil)

	dir := t.TempDir() // no .git here: serviceCandidates yields exactly {name: base(dir), root: dir}

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "the daemon must never be queried for an out-of-repo file", 500)
	}))
	defer srv.Close()

	// A path that shares no ancestor with dir beyond the OS temp root, so
	// filepath.Rel(dir, outside) always yields a "../.." prefix.
	outside := filepath.Join(os.TempDir(), "definitely-unrelated-"+filepath.Base(t.TempDir()), "notes.md")
	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, outside, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)

	if code != 0 {
		t.Fatalf("edit-hook on an out-of-repo file should exit 0, got %d", code)
	}
	if out.Len() != 0 {
		t.Errorf("edit-hook on an out-of-repo file should print nothing, got %q", out.String())
	}
	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Errorf("edit-hook queried the daemon %d time(s) for an out-of-repo file, want 0", n)
	}
}

// TestEditHookAbsolutePathInsideCandidateRootStillResolves is the positive
// twin of the above: OP-3 must not regress the ordinary case, where an
// absolute file path DOES live under a candidate root — it must still
// relativize and query the daemon with the repo-relative path (not the raw
// absolute one).
func TestEditHookAbsolutePathInsideCandidateRootStillResolves(t *testing.T) {
	dir := t.TempDir()
	absFile := filepath.Join(dir, "internal", "core", "engine.go")

	var gotFile string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Arguments struct {
					File string `json:"file"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotFile = req.Params.Arguments.File
		cannedPayload := map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    req.Params.Arguments.File,
			"rules": []any{
				map[string]any{"rule": "Sort all outputs.", "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed"},
			},
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, absFile, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	if gotFile != "internal/core/engine.go" {
		t.Errorf("daemon received file=%q, want the repo-relative path, not the raw absolute one", gotFile)
	}
	if !strings.Contains(out.String(), "Sort all outputs") {
		t.Errorf("expected the obligations rule in output, got %q", out.String())
	}
}

// TestEditHookRendersDisplacedAndNonDefaultLayer is T5-2/O14's edit-hook
// coverage: the obligations payload's displaced count and each rule's layer
// label must reach the injected context text — displaced as a single
// integer note (never the evicted rules themselves, which would reopen the
// unbounded-growth risk the ≤150-token spec-line budget above already
// guards against), and a NON-default layer (here "adr-rules", blended in by
// a recompile — the O14 reproduction shape) called out inline so an agent
// sees which layer a rule came from without a second lookup.
func TestEditHookRendersDisplacedAndNonDefaultLayer(t *testing.T) {
	dir := t.TempDir()
	absFile := filepath.Join(dir, "internal", "query", "engine.go")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Arguments struct {
					File string `json:"file"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		cannedPayload := map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    req.Params.Arguments.File,
			"rules": []any{
				map[string]any{"rule": "Sort all outputs.", "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed", "layer": "obligations"},
				map[string]any{"rule": "ADR-021 §3: typed edges only.", "src": "ADR.md#L10-L10", "tier": "doc-claimed", "layer": "adr-rules"},
			},
			"displaced": 1,
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, absFile, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	got := out.String()
	if !strings.Contains(got, "Sort all outputs") {
		t.Errorf("expected the obligations rule in output, got %q", got)
	}
	if strings.Contains(got, "obligations]") || strings.Contains(got, ", obligations,") {
		t.Errorf("default-layer rule must NOT be labeled (elided for compactness), got %q", got)
	}
	if !strings.Contains(got, "adr-rules") {
		t.Errorf("non-default-layer rule must carry its layer label, got %q", got)
	}
	if !strings.Contains(got, "1 more eligible rule") {
		t.Errorf("expected the displaced count in output, got %q", got)
	}
}

// TestEditHookResolvesThroughSymlinkedRoot: the root-refusal check must not
// be purely lexical. gitToplevel returns the PHYSICAL work-tree path while a
// session may address files through a symlink (~/code → /Volumes/…/code) —
// or vice versa. Here the project dir is the symlink-resolved (physical)
// form of a temp dir while the file path uses the raw (on macOS,
// /var → /private/var symlinked) form: lexically the file ..-escapes the
// root, so without symlink resolution OP-3 would silently refuse every
// in-repo file of such a session — a fatal recall loss.
func TestEditHookResolvesThroughSymlinkedRoot(t *testing.T) {
	dir := t.TempDir()
	physDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", dir, err)
	}
	rel := "internal/core/engine.go"
	abs := writeEditedFile(t, dir, rel, "package core\n\nfunc Engine() {}\n") // raw (symlinked) path

	var gotFile string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Arguments struct {
					File string `json:"file"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotFile = req.Params.Arguments.File
		cannedPayload := map[string]any{
			"service": strings.ToLower(filepath.Base(physDir)),
			"file":    req.Params.Arguments.File,
			"rules": []any{
				map[string]any{"rule": "Sort all outputs.", "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed"},
			},
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(cannedPayload)},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Project dir = physical path, file path = raw path.
	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, abs, physDir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	if gotFile != rel {
		t.Errorf("daemon received file=%q, want %q (symlink-mismatched root must still relativize)", gotFile, rel)
	}
	if !strings.Contains(out.String(), "Sort all outputs") {
		t.Errorf("expected the obligations rule in output, got %q", out.String())
	}
}

// ─── OP-4: spec-line injection ───────────────────────────────────────────────

// specRefsFakeDaemon serves a canned "obligations" MCP response. SR-27
// (surface reckoning F-35) folded the retired spec_refs tool's
// information into obligations' own additive `spec_refs` field, so one
// canned response now covers what used to take two (an "obligations"
// call plus a per-ID "spec_refs" call) — obligationsOut may carry a
// "spec_refs" key shaped exactly like the real one:
// {spec_id: [{file, line}]}.
type specRefsFakeDaemon struct {
	t              *testing.T
	obligationsOut map[string]any // canned obligations payload (may carry "spec_refs")
}

func (f *specRefsFakeDaemon) start() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Params.Name != "obligations" {
			http.Error(w, "unexpected tool "+req.Params.Name, 400)
			return
		}
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": mustJSON(f.obligationsOut)},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func writeEditedFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return abs
}

// TestEditHookInjectsSpecLineForCitedID: editing a file that cites a
// bracketed spec ID appends "▸ implements [ID] — defined at <file:line>" for
// the location OUTSIDE the edited file (the spec doc side) — the citing
// file's own location must never be echoed back as its own definition.
func TestEditHookInjectsSpecLineForCitedID(t *testing.T) {
	dir := t.TempDir()
	rel := "internal/core/engine.go"
	abs := writeEditedFile(t, dir, rel, "package core\n\n// implements [US-042]\nfunc Engine() {}\n")

	fake := &specRefsFakeDaemon{
		t: t,
		obligationsOut: map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    rel,
			"rules": []any{
				map[string]any{"rule": "Sort all outputs.", "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed"},
			},
			"spec_refs": map[string]any{
				"US-042": []any{
					map[string]any{"file": rel, "line": 3},                     // the citing file itself — must be skipped
					map[string]any{"file": "docs/specs/us-042.md", "line": 10}, // the definition — must be appended
				},
			},
		},
	}
	srv := fake.start()
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, abs, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	outStr := out.String()
	if !strings.Contains(outStr, "Sort all outputs") {
		t.Fatalf("expected the obligations rule in output, got %q", outStr)
	}
	if !strings.Contains(outStr, "▸ implements [US-042] — defined at docs/specs/us-042.md:10") {
		t.Errorf("expected the spec-line, got %q", outStr)
	}
	if strings.Contains(outStr, rel+":3") {
		t.Errorf("spec line must not point back at the citing file itself: %q", outStr)
	}
}

// TestEditHookSpecLineDroppedWhenBudgetSpent: the ≤150-token discipline
// applies to the WHOLE payload; if the obligations rules already spend the
// budget, spec lines are dropped first (never truncating the obligations
// rules themselves).
func TestEditHookSpecLineDroppedWhenBudgetSpent(t *testing.T) {
	dir := t.TempDir()
	rel := "internal/core/engine.go"
	abs := writeEditedFile(t, dir, rel, "package core\n\n// implements [US-042]\nfunc Engine() {}\n")

	// A single rule long enough (chars/4 ~= tokens) to blow the whole-payload
	// budget on its own — sized from specLineTokenBudget, never a literal, so
	// the test follows ADR-016's ceiling (5 chars per "word " = 1.25 tokens).
	longRule := strings.Repeat("word ", specLineTokenBudget)
	fake := &specRefsFakeDaemon{
		t: t,
		obligationsOut: map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    rel,
			"rules": []any{
				map[string]any{"rule": longRule, "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed"},
			},
			"spec_refs": map[string]any{
				"US-042": []any{
					map[string]any{"file": "docs/specs/us-042.md", "line": 10},
				},
			},
		},
	}
	srv := fake.start()
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, abs, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	outStr := out.String()
	if !strings.Contains(outStr, strings.TrimSpace(longRule)) {
		t.Fatalf("the obligations rule must survive even when it alone exceeds budget, got %q", outStr)
	}
	if strings.Contains(outStr, "▸ implements") {
		t.Errorf("spec line must be dropped once the token budget is already spent, got %q", outStr)
	}
}

// TestEditHookCapsSpecIDLookupsAtThree: a file citing more than three
// distinct spec IDs must render at most three spec lines — SR-27 folded
// the per-ID spec_refs RPC into the SAME obligations call, so the cap
// (maxSpecIDsPerEdit) now bounds how many of the IDs the obligations
// answer's spec_refs map gets consulted for, not a separate RPC count.
func TestEditHookCapsSpecIDLookupsAtThree(t *testing.T) {
	dir := t.TempDir()
	rel := "internal/core/engine.go"
	abs := writeEditedFile(t, dir, rel,
		"package core\n// [US-001] [US-002] [US-003] [US-004] [US-005]\nfunc Engine() {}\n")

	fake := &specRefsFakeDaemon{
		t: t,
		obligationsOut: map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    rel,
			"rules": []any{
				map[string]any{"rule": "Sort all outputs.", "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed"},
			},
			"spec_refs": map[string]any{
				"US-001": []any{map[string]any{"file": "docs/specs/a.md", "line": 1}},
				"US-002": []any{map[string]any{"file": "docs/specs/b.md", "line": 1}},
				"US-003": []any{map[string]any{"file": "docs/specs/c.md", "line": 1}},
				"US-004": []any{map[string]any{"file": "docs/specs/d.md", "line": 1}},
				"US-005": []any{map[string]any{"file": "docs/specs/e.md", "line": 1}},
			},
		},
	}
	srv := fake.start()
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, abs, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	outStr := out.String()
	for _, id := range []string{"US-001", "US-002", "US-003"} {
		if !strings.Contains(outStr, "implements ["+id+"]") {
			t.Errorf("expected a spec line for the first three IDs, missing %s: %q", id, outStr)
		}
	}
	for _, id := range []string{"US-004", "US-005"} {
		if strings.Contains(outStr, "implements ["+id+"]") {
			t.Errorf("expected the cap to drop IDs past the first three, got %s: %q", id, outStr)
		}
	}
}

// TestEditHookSpecLinesWithoutObligations: spec lines fire INDEPENDENTLY of
// obligations. A file citing [US-042] whose edit matches zero rules still
// gets its traceability line, as a spec-lines-only payload — post-recompile,
// prompt v3's narrower scopes make rule-less files the common case, and the
// plan's "silent on none" means no spec IDs, not no obligations.
func TestEditHookSpecLinesWithoutObligations(t *testing.T) {
	dir := t.TempDir()
	rel := "internal/core/engine.go"
	abs := writeEditedFile(t, dir, rel, "package core\n\n// implements [US-042]\nfunc Engine() {}\n")

	fake := &specRefsFakeDaemon{
		t: t,
		obligationsOut: map[string]any{ // zero rules, not stale
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    rel,
			"spec_refs": map[string]any{
				"US-042": []any{
					map[string]any{"file": "docs/specs/us-042.md", "line": 10},
				},
			},
		},
	}
	srv := fake.start()
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, abs, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	outStr := out.String()
	if outStr == "" {
		t.Fatal("expected a spec-lines-only payload, got nothing")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(outStr)), &payload); err != nil {
		t.Fatalf("output not valid JSON: %v (output=%q)", err, outStr)
	}
	hook, _ := payload["hookSpecificOutput"].(map[string]any)
	ctx, _ := hook["additionalContext"].(string)
	if !strings.Contains(ctx, "⚖️ fathom spec refs — "+rel+":") {
		t.Errorf("spec-lines-only payload missing its header, got %q", ctx)
	}
	if !strings.Contains(ctx, "▸ implements [US-042] — defined at docs/specs/us-042.md:10") {
		t.Errorf("expected the spec line without any obligations, got %q", ctx)
	}
	if strings.Contains(ctx, "fathom obligations") {
		t.Errorf("no obligations header should appear when there are no rules, got %q", ctx)
	}
}

// TestEditHookSilentWhenNoRulesAndNoSpecIDs pins the zero-output-on-clean
// contract after the OP-4 restructure: an in-root file with no matching
// rules AND no cited spec IDs produces nothing.
func TestEditHookSilentWhenNoRulesAndNoSpecIDs(t *testing.T) {
	dir := t.TempDir()
	rel := "internal/core/engine.go"
	abs := writeEditedFile(t, dir, rel, "package core\n\nfunc Engine() {}\n")

	fake := &specRefsFakeDaemon{
		t: t,
		obligationsOut: map[string]any{ // zero rules, not stale
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    rel,
		},
	}
	srv := fake.start()
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, abs, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("clean file with no spec IDs: expected no output, got %q", out.String())
	}
}

// TestEditHookSpecLinesOmittedWhenObligationsCarriesNone: SR-27 folded
// spec_refs into the SAME obligations call, so there is no longer a
// separate RPC that can fail independently — the equivalent degrade is an
// obligations answer whose spec_refs key is simply absent (exactly what
// obligationsSpecRefs itself returns on any server-side degrade, per its
// own doc comment: never an error, only nil). The edit hook must render
// the obligations payload normally and show no spec line, never fail the
// hook over an additive section that isn't there.
func TestEditHookSpecLinesOmittedWhenObligationsCarriesNone(t *testing.T) {
	dir := t.TempDir()
	rel := "internal/core/engine.go"
	abs := writeEditedFile(t, dir, rel, "package core\n\n// implements [US-042]\nfunc Engine() {}\n")

	fake := &specRefsFakeDaemon{
		t: t,
		obligationsOut: map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    rel,
			"rules": []any{
				map[string]any{"rule": "Sort all outputs.", "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed"},
			},
			// no "spec_refs" key at all.
		},
	}
	srv := fake.start()
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, abs, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0 (an absent spec_refs section must not fail the hook)", code)
	}
	outStr := out.String()
	if !strings.Contains(outStr, "Sort all outputs") {
		t.Errorf("obligations payload must render fine with no spec_refs section, got %q", outStr)
	}
	if strings.Contains(outStr, "▸ implements") {
		t.Errorf("no spec line should be present when obligations carries no spec_refs, got %q", outStr)
	}
}
