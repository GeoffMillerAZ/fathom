package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boundaryCall(t *testing.T, s *Server, name string, args map[string]any) (map[string]any, *rpcError) {
	t.Helper()
	tool, ok := s.findTool(name)
	if !ok {
		t.Fatal(name)
	}
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	out, rpcErr := tool.handler(context.Background(), raw)
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	return body, rpcErr
}

func TestCloneBoundaryF2UntrackedFiles(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	b, clone := reviewMCPClone(t)
	// No ignore rules: ordinary untracked files must still never be served.
	if err := os.Remove(filepath.Join(clone, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	s := NewServer(b)
	for _, file := range []string{"id_rsa", "keys.go"} {
		writeFlagFixture(t, clone, file, "package a\n// private-secret\nfunc Secret() {}\n")
		for _, name := range []string{"search", "read_source", "find_symbol", "symbol_context", "blast_radius"} {
			t.Run(file+"/"+name, func(t *testing.T) {
				args := map[string]any{"service": "svc", "name": "Secret", "symbol": "Secret", "path": filepath.Join(clone, file), "file": filepath.Join(clone, file), "changed_files": []string{filepath.Join(clone, file)}, "root": clone, "pattern": "private-secret"}
				if name == "search" {
					delete(args, "service")
					delete(args, "path")
				}
				out, e := boundaryCall(t, s, name, args)
				if e != nil {
					t.Fatal(e)
				}
				data, _ := json.Marshal(out)
				if strings.Contains(string(data), "private-secret") || strings.Contains(string(data), "func Secret") {
					t.Errorf("untracked bytes served: %s", data)
				}
				if name != "search" && out["status"] != "unknown" {
					t.Errorf("untracked path must be unknown: %s", data)
				}
			})
		}
	}
	for _, file := range []string{"id_rsa", "keys.go"} {
		out, e := boundaryCall(t, s, "blast_radius", map[string]any{"service": "svc", "path": clone, "changed_files": []string{file}})
		if e != nil || out["status"] != "unknown" {
			t.Errorf("untracked reach must be unknown without bytes: %v, %+v", out, e)
		}
	}
	out, e := boundaryCall(t, s, "read_source", map[string]any{"service": "svc", "file": filepath.Join(clone, "a.go")})
	data, _ := json.Marshal(out)
	if e != nil || !strings.Contains(string(data), "func Stable") {
		t.Fatalf("tracked bytes withheld: %s, %+v", data, e)
	}
	// A directory identity hint must not expose a snippet for an untracked file.
	b.nodes[0].FilePath = "keys.go"
	out, e = boundaryCall(t, NewServer(b), "symbol_context", map[string]any{"service": "svc", "path": clone, "symbol": "Stable"})
	data, _ = json.Marshal(out)
	if e != nil || strings.Contains(string(data), "private-secret") {
		t.Fatalf("untracked snippet served: %s, %+v", data, e)
	}
}

func TestCloneBoundaryF3MissingHome(t *testing.T) {
	b, clone := reviewMCPClone(t)
	t.Setenv("HOME", "")
	out, e := boundaryCall(t, NewServer(b), "read_source", map[string]any{"service": "svc", "file": filepath.Join(clone, "a.go")})
	if e == nil || !strings.Contains(e.Message, "home directory unavailable") {
		t.Fatalf("missing home must refuse with reason: %v, %+v", out, e)
	}
}

func TestCloneBoundaryF6AbsoluteAliases(t *testing.T) {
	b, clone := reviewMCPClone(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(clone, alias); err != nil {
		t.Fatal(err)
	}
	for _, hint := range []bool{false, true} {
		t.Run(map[bool]string{false: "inferred", true: "explicit"}[hint], func(t *testing.T) {
			h := clonePathHandler("blast_radius", b, func(_ context.Context, raw json.RawMessage) (any, *rpcError) {
				var args struct {
					Files []string `json:"changed_files"`
				}
				if err := json.Unmarshal(raw, &args); err != nil {
					t.Fatal(err)
				}
				if len(args.Files) != 2 || args.Files[0] != "a.go" || args.Files[1] != "a.go" {
					t.Errorf("absolute aliases not canonicalized: %v", args.Files)
				}
				return map[string]any{}, nil
			})
			args := map[string]any{"service": "svc", "changed_files": []string{filepath.Join(clone, "a.go"), filepath.Join(alias, "a.go")}}
			if hint {
				args["path"] = clone
			}
			raw, _ := json.Marshal(args)
			if out, e := h(context.Background(), raw); e != nil {
				t.Fatalf("same-checkout alias refused: %v, %+v", out, e)
			}
		})
	}
	outside := t.TempDir()
	writeFlagFixture(t, outside, "a.go", "package private\n")
	if err := os.Symlink(filepath.Join(outside, "a.go"), filepath.Join(clone, "outside.go")); err != nil {
		t.Fatal(err)
	}
	cloneGit(t, clone, "add", "outside.go")
	_, e := boundaryCall(t, NewServer(b), "blast_radius", map[string]any{"service": "svc", "path": clone, "changed_files": []string{filepath.Join(clone, "a.go"), filepath.Join(clone, "outside.go")}})
	if e == nil {
		t.Fatal("changed_files symlink escaping checkout accepted")
	}
}
