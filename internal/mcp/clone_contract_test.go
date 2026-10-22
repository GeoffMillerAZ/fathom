package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneContractF4F5Documentation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "mcp-tools.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"integrity check, not an access control", "local user who can write files", "MCP clients that cannot write files", "path resolves to a different indexed service", "search: give exactly one of service or root", "subdirectory searches only that subdirectory"} {
		if !strings.Contains(string(data), phrase) {
			t.Errorf("clone contract lacks %q", phrase)
		}
	}
}

func TestCloneContractF5SearchSubdirectory(t *testing.T) {
	b, clone := reviewMCPClone(t)
	for _, path := range []string{"nested/inside.go", "outside.go"} {
		writeFlagFixture(t, clone, path, "package a\n// scoped-match\n")
	}
	cloneGit(t, clone, "add", "nested/inside.go", "outside.go")
	out, e := boundaryCall(t, NewServer(b), "search", map[string]any{"root": filepath.Join(clone, "nested"), "pattern": "scoped-match"})
	if e != nil {
		t.Fatal(e)
	}
	hits, _ := out["hits"].([]any)
	if len(hits) != 1 || !strings.HasPrefix(hits[0].(map[string]any)["loc"].(string), "inside.go:") {
		t.Fatalf("subdirectory search escaped scope: %v", out)
	}
	_, e = boundaryCall(t, NewServer(b), "search", map[string]any{"service": "other", "root": clone, "pattern": "scoped-match"})
	if e == nil || e.Message != "path resolves to a different indexed service" {
		t.Fatalf("clone service mismatch contract: %+v", e)
	}
}
