package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func reviewMCPClone(t *testing.T) (clonePathBackend, string) {
	t.Helper()
	repo := t.TempDir()
	parent := t.TempDir()
	clone := filepath.Join(parent, "clone")
	cloneGit(t, repo, "init", "-q")
	writeFlagFixture(t, repo, "a.go", "package a\nfunc Stable() {}\n")
	writeFlagFixture(t, repo, ".gitignore", ".ssh/\nignored.go\n")
	cloneGit(t, repo, "add", "a.go", ".gitignore")
	cloneGit(t, repo, "commit", "-qm", "base")
	commit := cloneGit(t, repo, "rev-parse", "HEAD")
	cloneGit(t, parent, "clone", "-q", repo, clone)
	return clonePathBackend{root: repo, commit: commit, nodes: []core.SymbolNode{{Service: "svc", Name: "Stable", Kind: core.KindGoFunc, FilePath: "a.go", Line: 2, EndLine: 2}}}, clone
}

func TestReviewF1CloneVisiblePaths(t *testing.T) {
	b, clone := reviewMCPClone(t)
	writeFlagFixture(t, clone, "ignored.go", "package a\n// private-secret\n")
	writeFlagFixture(t, clone, ".ssh/keys.go", "package a\n// private-secret\n")
	s := NewServer(b)
	for _, name := range []string{"search", "read_source", "find_symbol", "symbol_context", "blast_radius"} {
		t.Run(name, func(t *testing.T) {
			tool, _ := s.findTool(name)
			args := map[string]any{"service": "svc", "name": "Stable", "symbol": "Stable", "path": filepath.Join(clone, ".ssh/keys.go"), "file": filepath.Join(clone, ".ssh/keys.go"), "root": clone, "pattern": "private-secret", "changed_files": []string{".ssh/keys.go"}}
			if name == "search" {
				delete(args, "service")
			}
			raw, _ := json.Marshal(args)
			out, e := tool.handler(context.Background(), raw)
			if name == "search" {
				if e != nil {
					t.Fatal(e)
				}
				data, _ := json.Marshal(out)
				if strings.Contains(string(data), "private-secret") {
					t.Fatalf("ignored bytes served: %s", data)
				}
			} else if e == nil {
				t.Fatalf("private path accepted by %s: %v", name, out)
			}
		})
	}
	tool, _ := s.findTool("read_source")
	for _, file := range []string{filepath.Join(clone, "ignored.go"), filepath.Join(clone, "../outside.go"), filepath.Join(clone, "link.go")} {
		if strings.HasSuffix(file, "link.go") {
			if err := os.Symlink(filepath.Join(b.root, "a.go"), file); err != nil {
				t.Fatal(err)
			}
		}
		raw, _ := json.Marshal(map[string]any{"service": "svc", "file": file})
		if out, e := tool.handler(context.Background(), raw); e == nil {
			t.Errorf("disallowed file accepted: %q %v", file, out)
		}
	}
}

type reviewResolverBackend struct {
	clonePathBackend
	err         error
	sawDeadline *bool
}

func (b reviewResolverBackend) Services(ctx context.Context) ([]string, error) {
	if b.sawDeadline != nil {
		_, *b.sawDeadline = ctx.Deadline()
	}
	if b.err != nil {
		return nil, b.err
	}
	return b.clonePathBackend.Services(ctx)
}
func TestReviewF6OriginalHandlerFallback(t *testing.T) {
	b, clone := reviewMCPClone(t)
	for _, tc := range []struct {
		name, service, target string
		backend               QueryInterface
	}{
		{"resolver error", "svc", clone, reviewResolverBackend{clonePathBackend: b, err: errors.New("registry unavailable")}},
		{"explicit elsewhere", "svc", t.TempDir(), b},
		{"direct other service", "other", b.root, b},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			raw, _ := json.Marshal(map[string]any{"service": tc.service, "root": tc.target})
			h := clonePathHandler("search", tc.backend, func(_ context.Context, got json.RawMessage) (any, *rpcError) {
				called = true
				if string(got) != string(raw) {
					t.Error("arguments changed")
				}
				return "original", nil
			})
			out, e := h(context.Background(), raw)
			if e != nil || !called || out != "original" {
				t.Fatalf("fallback: %v %+v called=%v", out, e, called)
			}
		})
	}
	// A verified clone still intercepts explicit-service requests.
	raw, _ := json.Marshal(map[string]any{"service": "other", "root": clone})
	h := clonePathHandler("search", b, func(context.Context, json.RawMessage) (any, *rpcError) {
		t.Fatal("clone mismatch fell through")
		return nil, nil
	})
	if _, e := h(context.Background(), raw); e == nil {
		t.Fatal("clone service mismatch accepted")
	}
}
func TestReviewF3ResolverSubdeadline(t *testing.T) {
	b, clone := reviewMCPClone(t)
	hasDeadline := false
	_, _, err := resolveMCPPath(context.Background(), reviewResolverBackend{clonePathBackend: b, sawDeadline: &hasDeadline}, clone)
	if err != nil || !hasDeadline {
		t.Fatalf("resolver omitted subdeadline: %v %v", err, hasDeadline)
	}
}

type reviewStateBackend struct {
	clonePathBackend
	checks *int
}

func (b reviewStateBackend) SourceFingerprint(context.Context, string, string) (string, bool, error) {
	*b.checks++
	return "", false, nil
}
func TestReviewF3OneFileStateCheck(t *testing.T) {
	b, clone := reviewMCPClone(t)
	calls := reviewGitCounter(t)
	checks := 0
	backend := reviewStateBackend{b, &checks}
	h := clonePathHandler("find_symbol", backend, func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
		r, ok := clonePath(ctx, "")
		if !ok {
			t.Fatal("no clone context")
		}
		for i := 0; i < 3; i++ {
			if same, why := cloneFileState(ctx, backend, r, "a.go"); !same {
				t.Fatalf("state %s", why)
			}
		}
		return map[string]any{"matches": []any{map[string]any{"file_path": "a.go"}, map[string]any{"file_path": "a.go"}}}, nil
	})
	raw, _ := json.Marshal(map[string]any{"service": "svc", "path": clone})
	if _, e := h(context.Background(), raw); e != nil {
		t.Fatal(e)
	}
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "ls-files -z --cached --others --exclude-standard\n") != 1 {
		t.Fatalf("visible path calls: %s", data)
	}
	if checks != 1 {
		t.Fatalf("file state checks=%d, want 1", checks)
	}
}
func TestReviewF5CloneInputSchemas(t *testing.T) {
	s := NewServer(nil)
	for _, name := range []string{"find_symbol", "symbol_context", "blast_radius"} {
		tool, _ := s.findTool(name)
		props := tool.desc.InputSchema["properties"].(map[string]any)
		if props["path"] == nil {
			t.Errorf("%s omits clone path", name)
		}
	}
	tool, _ := s.findTool("read_source")
	desc := tool.desc.InputSchema["properties"].(map[string]any)["file"].(map[string]any)["description"].(string)
	if !strings.Contains(desc, "clone") {
		t.Fatalf("read_source file description: %s", desc)
	}
}

func TestReviewF8UnknownReplacesAbsence(t *testing.T) {
	b, clone := reviewMCPClone(t)
	writeFlagFixture(t, clone, "a.go", "package changed\n")
	h := clonePathHandler("blast_radius", b, func(context.Context, json.RawMessage) (any, *rpcError) {
		return map[string]any{"summary": "nothing found", "not_found": true, "not_indexed": true, "hint": "try another service"}, nil
	})
	raw, _ := json.Marshal(map[string]any{"service": "svc", "path": clone, "changed_files": []string{"a.go"}})
	out, e := h(context.Background(), raw)
	if e != nil {
		t.Fatal(e)
	}
	body := out.(map[string]any)
	for _, key := range []string{"not_found", "not_indexed", "hint"} {
		if _, ok := body[key]; ok {
			t.Errorf("unknown answer retains %s: %v", key, body)
		}
	}
	if body["status"] != "unknown" || !strings.Contains(body["summary"].(string), "unknown") {
		t.Fatal(body)
	}
}

func reviewGitCounter(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + quote(log) + "\nexec " + quote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func TestReviewF6ExplicitServiceRootKeepsValidation(t *testing.T) {
	b, _ := reviewMCPClone(t)
	tool, _ := NewServer(b).findTool("search")
	raw, _ := json.Marshal(map[string]any{"service": "svc", "root": t.TempDir(), "pattern": "func"})
	_, e := tool.handler(context.Background(), raw)
	if e == nil || e.Code != ErrCodeParams || e.Message != "search: give exactly one of service or root" {
		t.Fatalf("original service/root validation changed: %+v", e)
	}
}
