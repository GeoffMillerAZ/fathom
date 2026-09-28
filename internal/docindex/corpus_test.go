package docindex

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile writes rel under root, creating parent directories as needed.
// With no extra content argument it writes a minimal placeholder Markdown
// body; a test that cares about the actual content (e.g. ingest_test.go)
// passes it explicitly.
func writeFile(t *testing.T, root, rel string, content ...string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# " + rel + "\n\nbody\n"
	if len(content) > 0 {
		body = content[0]
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDocCorpusIncludesAndFlags pins KR-3: every §3 corpus file kind is
// found and classified, generated/vendored/workspace trees are excluded
// outright, and docs/plans/archive/ + tmp/ are included but flagged
// stale_by_location — never dropped.
func TestDocCorpusIncludesAndFlags(t *testing.T) {
	root := t.TempDir()
	included := []string{
		"ADR.md",
		"docs/intent/scope.md",
		"docs/plans/knowledge-retrieval.md",
		"docs/plans/archive/old-plan.md",
		"docs/design/knowledge-retrieval.md",
		"docs/vision/38-proof-layer-roadmap.md",
		"README.md",
		"tmp/design_inspiration/notes.md",
	}
	for _, rel := range included {
		writeFile(t, root, rel)
	}
	excluded := []string{
		"vendor/thirdparty/README.md",
		"node_modules/pkg/README.md",
		".git/COMMIT_EDITMSG.md",
		"workspace/db/notes.md",
		"docs/plans/knowledge-retrieval.txt", // not Markdown
	}
	for _, rel := range excluded {
		writeFile(t, root, rel)
	}

	got, err := docCorpusFiles(root)
	if err != nil {
		t.Fatalf("docCorpusFiles: %v", err)
	}
	byPath := map[string]CorpusFile{}
	for _, f := range got {
		byPath[f.Path] = f
	}
	if len(got) != len(included) {
		var paths []string
		for _, f := range got {
			paths = append(paths, f.Path)
		}
		t.Fatalf("docCorpusFiles = %d files %v, want %d: %v", len(got), paths, len(included), included)
	}
	for _, rel := range excluded {
		if rel == "docs/plans/knowledge-retrieval.txt" {
			continue // never written as .md, just documents intent
		}
		if _, found := byPath[rel]; found {
			t.Errorf("docCorpusFiles included excluded path %q", rel)
		}
	}

	wantKind := map[string]DocKind{
		"ADR.md":                                DocKindADR,
		"docs/intent/scope.md":                  DocKindIntent,
		"docs/plans/knowledge-retrieval.md":     DocKindPlan,
		"docs/plans/archive/old-plan.md":        DocKindPlan,
		"docs/design/knowledge-retrieval.md":    DocKindOther,
		"docs/vision/38-proof-layer-roadmap.md": DocKindOther,
		"README.md":                             DocKindOther,
		"tmp/design_inspiration/notes.md":       DocKindOther,
	}
	for path, want := range wantKind {
		f, found := byPath[path]
		if !found {
			t.Errorf("docCorpusFiles missing %q", path)
			continue
		}
		if f.Kind != want {
			t.Errorf("%s Kind = %q, want %q", path, f.Kind, want)
		}
	}

	wantStale := map[string]bool{
		"ADR.md":                            false,
		"docs/plans/knowledge-retrieval.md": false,
		"docs/plans/archive/old-plan.md":    true,
		"tmp/design_inspiration/notes.md":   true,
	}
	for path, want := range wantStale {
		if got := byPath[path].StaleByLocation; got != want {
			t.Errorf("%s StaleByLocation = %v, want %v", path, got, want)
		}
	}

	// Sorted by path (I2/determinism).
	for i := 1; i < len(got); i++ {
		if got[i-1].Path >= got[i].Path {
			t.Fatalf("docCorpusFiles not sorted: %q before %q", got[i-1].Path, got[i].Path)
		}
	}
}

// TestDocCorpusUnreadableDirErrors pins I38: an unreadable directory under
// root is reported as an error, never a silently smaller result.
func TestDocCorpusUnreadableDirErrors(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "docs")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "docs/design/x.md")
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Skipf("cannot revoke directory permissions in this environment: %v", err)
	}
	defer os.Chmod(blocked, 0o755)
	if _, err := docCorpusFiles(root); err == nil {
		t.Fatal("docCorpusFiles with an unreadable subdirectory: want error, got nil")
	}
}
