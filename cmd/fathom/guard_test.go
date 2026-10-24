package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

func writeTestGuardFile(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guard.cue"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadGuardSet_Absent(t *testing.T) {
	root := t.TempDir()
	if _, ok := loadGuardSet(root); ok {
		t.Fatal("expected ok=false when .fathom/guard.cue is absent")
	}
}

func TestLoadGuardSet_Malformed_FailsOpen(t *testing.T) {
	root := t.TempDir()
	writeTestGuardFile(t, root, `guards: [{}]`) // neither scopes nor command_scopes -- invalid
	if _, ok := loadGuardSet(root); ok {
		t.Fatal("expected ok=false for a malformed guard.cue")
	}
}

func TestLoadGuardSet_Valid(t *testing.T) {
	root := t.TempDir()
	writeTestGuardFile(t, root, `guards: [{scopes: ["internal/store/**"], note: "spec-governed"}]`)
	gs, ok := loadGuardSet(root)
	if !ok || len(gs.Guards) != 1 {
		t.Fatalf("loadGuardSet = %v, %v", gs, ok)
	}
}

func TestMatchingFileGuard(t *testing.T) {
	gs := guardspec.GuardSet{Guards: []guardspec.Guard{
		{CommandScopes: []string{"terraform *"}}, // command-only, must be skipped for file matching
		{Scopes: []string{"internal/store/**"}, Note: "store"},
		{Scopes: []string{"**/*.go"}, Note: "go"},
	}}
	g, ok := matchingFileGuard(gs, "internal/store/db.go")
	if !ok || g.Note != "store" {
		t.Fatalf("matchingFileGuard = %+v, %v, want the store guard (first match wins)", g, ok)
	}
	g2, ok2 := matchingFileGuard(gs, "cmd/main.go")
	if !ok2 || g2.Note != "go" {
		t.Fatalf("matchingFileGuard = %+v, %v, want the go guard", g2, ok2)
	}
	if _, ok3 := matchingFileGuard(gs, "README.md"); ok3 {
		t.Error("expected no match for README.md")
	}
}

func TestMatchingCommandGuard(t *testing.T) {
	gs := guardspec.GuardSet{Guards: []guardspec.Guard{
		{Scopes: []string{"**/*.go"}}, // file-only, must be skipped for command matching
		{CommandScopes: []string{"terraform apply*", "kubectl *"}, Note: "infra"},
	}}
	g, ok := matchingCommandGuard(gs, "terraform apply -auto-approve")
	if !ok || g.Note != "infra" {
		t.Fatalf("matchingCommandGuard = %+v, %v", g, ok)
	}
	g2, ok2 := matchingCommandGuard(gs, "kubectl apply -f infra/prod.yaml")
	if !ok2 || g2.Note != "infra" {
		t.Fatalf("matchingCommandGuard(kubectl with slash arg) = %+v, %v — '*' must cross '/' for command patterns", g2, ok2)
	}
	if _, ok3 := matchingCommandGuard(gs, "ls -la"); ok3 {
		t.Error("expected no match for an unrelated command")
	}
}

func TestCommandMatches_StarCrossesSlash(t *testing.T) {
	if _, ok := guardspec.MatchCommandGuard(guardspec.GuardSet{Guards: []guardspec.Guard{{CommandScopes: []string{"*/migrate *"}}}}, "bin/migrate up"); !ok {
		t.Error("pattern */migrate * should match \"bin/migrate up\"")
	}
	if _, ok := guardspec.MatchCommandGuard(guardspec.GuardSet{Guards: []guardspec.Guard{{CommandScopes: []string{"kubectl *"}}}}, "kubectl apply -f a/b/c.yaml"); !ok {
		t.Error("'*' in a command pattern must match across '/' (this is a command string, not a filepath)")
	}
}

func TestMatchingCommandGuardSegmentsAndFallback(t *testing.T) {
	gs := guardspec.GuardSet{Guards: []guardspec.Guard{{CommandScopes: []string{"kubectl delete ns *"}, Note: "repo", Enforce: true}}}
	for _, command := range []string{`true && kubectl delete ns demo`, `kubectl delete ns $NS`, `kubectl delete ns demo; eval true`} {
		if g, ok := matchingCommandGuard(gs, command); !ok || g.Note != "repo" {
			t.Errorf("%q: want repo guard, got %+v, %v", command, g, ok)
		}
	}
}
