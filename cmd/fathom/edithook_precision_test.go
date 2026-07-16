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

// ─── OP-4: spec-line injection ───────────────────────────────────────────────

// specRefsFakeDaemon serves canned "obligations" and "spec_refs" MCP
// responses, recording every spec_refs query it receives (for the ≤3 cap
// assertion) behind a mutex-free atomic counter map is overkill here — a
// plain slice guarded by the test's single-goroutine http round trips is
// fine since net/http/httptest serves requests sequentially in these tests.
type specRefsFakeDaemon struct {
	t              *testing.T
	obligationsOut map[string]any   // canned obligations payload
	specRefs       map[string][]any // spec ID -> canned []{file,line} locations
	specQueries    []string         // every spec ID queried, in order
	specRefsFail   bool             // force spec_refs to error
}

func (f *specRefsFakeDaemon) start() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					Service string `json:"service"`
					File    string `json:"file"`
					Spec    string `json:"spec"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch req.Params.Name {
		case "obligations":
			resp := map[string]any{
				"jsonrpc": "2.0", "id": 1,
				"result": map[string]any{"content": []any{
					map[string]any{"type": "text", "text": mustJSON(f.obligationsOut)},
				}},
			}
			json.NewEncoder(w).Encode(resp)
		case "spec_refs":
			f.specQueries = append(f.specQueries, req.Params.Arguments.Spec)
			if f.specRefsFail {
				http.Error(w, "spec_refs unavailable", 500)
				return
			}
			specs := f.specRefs[req.Params.Arguments.Spec]
			payload := map[string]any{
				"specs":         map[string]any{req.Params.Arguments.Spec: specs},
				"total_refs":    len(specs),
				"files_scanned": 1,
			}
			resp := map[string]any{
				"jsonrpc": "2.0", "id": 1,
				"result": map[string]any{"content": []any{
					map[string]any{"type": "text", "text": mustJSON(payload)},
				}},
			}
			json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "unexpected tool "+req.Params.Name, 400)
		}
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
		},
		specRefs: map[string][]any{
			"US-042": {
				map[string]any{"file": rel, "line": 3},                     // the citing file itself — must be skipped
				map[string]any{"file": "docs/specs/us-042.md", "line": 10}, // the definition — must be appended
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

	// A single rule long enough (chars/4 ~= tokens) to blow the 150-token
	// budget on its own.
	longRule := strings.Repeat("word ", 200)
	fake := &specRefsFakeDaemon{
		t: t,
		obligationsOut: map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    rel,
			"rules": []any{
				map[string]any{"rule": longRule, "src": "CLAUDE.md#L4-L4", "tier": "doc-claimed"},
			},
		},
		specRefs: map[string][]any{
			"US-042": {
				map[string]any{"file": "docs/specs/us-042.md", "line": 10},
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
// distinct spec IDs must trigger at most three spec_refs lookups — a file
// citing a dozen specs must not turn one edit into a dozen RPCs.
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
		},
		specRefs: map[string][]any{
			"US-001": {map[string]any{"file": "docs/specs/a.md", "line": 1}},
			"US-002": {map[string]any{"file": "docs/specs/b.md", "line": 1}},
			"US-003": {map[string]any{"file": "docs/specs/c.md", "line": 1}},
			"US-004": {map[string]any{"file": "docs/specs/d.md", "line": 1}},
			"US-005": {map[string]any{"file": "docs/specs/e.md", "line": 1}},
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
	if len(fake.specQueries) > 3 {
		t.Errorf("spec_refs queried %d times, want at most 3 (cap): %v", len(fake.specQueries), fake.specQueries)
	}
}

// TestEditHookSilentOnSpecRefsError: spec_refs failing must not disturb the
// already-built obligations payload (hook doctrine: silent on any error, and
// only the additive part is at risk).
func TestEditHookSilentOnSpecRefsError(t *testing.T) {
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
		},
		specRefsFail: true,
	}
	srv := fake.start()
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, abs, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("edit-hook exit = %d, want 0 (spec_refs errors must not fail the hook)", code)
	}
	outStr := out.String()
	if !strings.Contains(outStr, "Sort all outputs") {
		t.Errorf("obligations payload must survive a spec_refs error, got %q", outStr)
	}
	if strings.Contains(outStr, "▸ implements") {
		t.Errorf("no spec line should be present when spec_refs errors, got %q", outStr)
	}
}
