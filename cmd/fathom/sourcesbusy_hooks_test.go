package main

// sourcesbusy_hooks_test.go — RT-146 end to end at the hooks (I38, I9): a
// daemon that skipped a hook's whole-tree read says so, and the hook
// renders that named skip — the edit hook as a warning line, the Stop hook
// as an advisory that never blocks.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestRenderPostEdit_NamesSkippedCoverage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "busysvc")
	absFile := filepath.Join(dir, "pkg", "run.go")
	if err := os.MkdirAll(filepath.Dir(absFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absFile, []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					File string `json:"file"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "list_services":
			inner = fmt.Sprintf(`{"services":[{"service":"busysvc","symbol_count":5,"path":%q}]}`, dir)
		case "obligations":
			inner = fmt.Sprintf(`{"service":"busysvc","file":%q,"rules":[],"coverage_skipped":%q,"layer_coverage":{"status":"skipped"}}`,
				req.Params.Arguments.File, core.ErrSourcesBusy.Error())
		default:
			http.Error(w, "unknown tool", 400)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	}))
	defer srv.Close()

	text, _, ok := renderPostEdit(srv.URL, dir, absFile, "sess-rt146", "")
	if !ok || !strings.Contains(text, "coverage skipped: whole-tree read in progress") {
		t.Fatalf("post-edit render with a skipped coverage read = (%q, %v), want the named skip — never silence (I38)", text, ok)
	}
}

func TestRunStopHook_SourcesBusyIsAdvisory(t *testing.T) {
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	plans := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plans, "p.md")
	if err := os.WriteFile(planPath, []byte("- [ ] One {id: T-1, symbol: Real, spec: [S-1]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "."},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(planPath, []byte("- [x] One {id: T-1, symbol: Real, spec: [S-1]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Params.Name {
		case "list_services":
			inner := fmt.Sprintf(`{"services":[{"service":%q,"symbol_count":10}]}`, svc)
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}})
		case "verify_plan":
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": -32603, "message": "verify_plan: " + core.ErrSourcesBusy.Error()}})
		default:
			http.Error(w, "unknown tool", 400)
		}
	}))
	defer srv.Close()
	hc := &http.Client{Timeout: 2 * time.Second}

	for _, block := range []bool{false, true} {
		var out bytes.Buffer
		if code, _ := runStopHook(dir, hc, srv.URL, block, &out); code != 0 {
			t.Fatalf("block=%v: exit %d, want 0 (fail open)", block, code)
		}
		if !strings.Contains(out.String(), "coverage skipped: whole-tree read in progress") {
			t.Fatalf("block=%v: output %q, want the named skip, never silence", block, out.String())
		}
		var blk struct {
			Decision string `json:"decision"`
		}
		_ = json.Unmarshal(out.Bytes(), &blk)
		if blk.Decision == "block" {
			t.Fatalf("block=%v: a skipped read produced decision:block — infrastructure never blocks", block)
		}
	}
}

// The prompt hook merges obligations across files: a skip on any of them
// rides the merge so the rendered payload can name it.
func TestMergeObligations_CarriesSkippedCoverage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner := fmt.Sprintf(`{"service":"s","file":"a.go","rules":[{"rule":"R","src":"CLAUDE.md#L1-L1","tier":"doc-claimed","layer":"obligations"}],"coverage_skipped":%q}`, core.ErrSourcesBusy.Error())
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}})
	}))
	defer srv.Close()
	m := mergeObligationsAcrossFiles(srv.URL, "s", []string{"a.go"}, time.Now().Add(2*time.Second), nil)
	if m.coverageSkipped != core.ErrSourcesBusy.Error() {
		t.Fatalf("merged coverageSkipped = %q, want the named skip", m.coverageSkipped)
	}
}
