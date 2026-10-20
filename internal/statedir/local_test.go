package statedir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

func TestLocalPathsPreferStateDirAndFallBackToLegacy(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	t.Setenv(EnvOverride, state)

	legacy := projectdir.Leases.Path(root)
	fresh := LocalPath(root, projectdir.Leases)
	if fresh != filepath.Join(state, "leases.jsonl") {
		t.Fatalf("LocalPath = %q", fresh)
	}
	if got := ReadPath(root, projectdir.Leases); got != legacy {
		t.Fatalf("absent both: ReadPath = %q, want legacy %q", got, legacy)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadPath(root, projectdir.Leases); got != legacy {
		t.Fatalf("legacy only: ReadPath = %q", got)
	}
	if LegacyStale(root, projectdir.Leases) {
		t.Fatal("stale reported with no state copy")
	}
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fresh, []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadPath(root, projectdir.Leases); got != fresh {
		t.Fatalf("both: ReadPath = %q, want state copy", got)
	}
	if !LegacyStale(root, projectdir.Leases) {
		t.Fatal("stale not reported with both copies")
	}
}
