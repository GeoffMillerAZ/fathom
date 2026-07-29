package bundle

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var hexDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestHashService_Deterministic(t *testing.T) {
	a := HashService("salt1", "my-service")
	b := HashService("salt1", "my-service")
	if a != b {
		t.Errorf("HashService not deterministic: %q vs %q", a, b)
	}
	if !hexDigest.MatchString(a) {
		t.Errorf("HashService(%q) = %q, want a 64-hex-char digest", "my-service", a)
	}
}

func TestHashService_DifferentSaltsDiffer(t *testing.T) {
	a := HashService("salt1", "my-service")
	b := HashService("salt2", "my-service")
	if a == b {
		t.Errorf("HashService with different salts produced the same hash: %q", a)
	}
}

func TestHashService_DifferentServicesDiffer(t *testing.T) {
	a := HashService("salt", "service-a")
	b := HashService("salt", "service-b")
	if a == b {
		t.Errorf("HashService for different services collided: %q", a)
	}
}

func TestHashService_NeverEqualsPlaintext(t *testing.T) {
	name := "my-secret-repo-name"
	h := HashService("salt", name)
	if h == name {
		t.Fatalf("HashService returned the plaintext unchanged")
	}
}

func TestLoadOrCreateSalt_StableAcrossCalls(t *testing.T) {
	dir := t.TempDir()

	s1, err := LoadOrCreateSalt(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt (create): %v", err)
	}
	if s1 == "" {
		t.Fatal("LoadOrCreateSalt returned an empty salt")
	}

	s2, err := LoadOrCreateSalt(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt (reload): %v", err)
	}
	if s1 != s2 {
		t.Errorf("salt not stable across calls: %q vs %q", s1, s2)
	}

	// A hash computed with the persisted salt must be stable across
	// "process restarts" (simulated by two independent LoadOrCreateSalt
	// calls against the same root).
	if HashService(s1, "svc") != HashService(s2, "svc") {
		t.Errorf("hash unstable despite stable salt")
	}
}

func TestLoadOrCreateSalt_TwoReposGetDifferentSalts(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	s1, err := LoadOrCreateSalt(dir1)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := LoadOrCreateSalt(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if s1 == s2 {
		t.Errorf("two independently-created salts collided (astronomically unlikely, or rand.Read is broken)")
	}
}

func TestSaltPath_UnderWorkspaceState(t *testing.T) {
	got := SaltPath("/repo")
	want := filepath.Join("/repo", "workspace", "state", "research.salt")
	if got != want {
		t.Errorf("SaltPath = %q, want %q", got, want)
	}
}

// TestSaltPath_NotUnderResearchArtifacts is a regression guard for Finding 2
// (critic review, 2026-07-28): the salt must never live inside
// workspace/artifacts/research/, which also holds the deliberately
// unsanitized report.md — a salt there would be swept up by a naive "share
// the research folder" copy.
func TestSaltPath_NotUnderResearchArtifacts(t *testing.T) {
	got := SaltPath("/repo")
	if strings.Contains(got, filepath.Join("artifacts", "research")) {
		t.Errorf("SaltPath = %q, must not live under workspace/artifacts/research/", got)
	}
}

// TestLoadOrCreateSalt_MigratesLegacyPath proves an existing repo's salt
// (written by the pre-Finding-2 code at workspace/artifacts/research/.salt)
// is picked up, moved to the new location with its value preserved (so the
// hash lineage survives), and removed from the old, shareable-adjacent
// location.
func TestLoadOrCreateSalt_MigratesLegacyPath(t *testing.T) {
	dir := t.TempDir()
	oldPath := legacySaltPath(dir)
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const legacySalt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd"
	if err := os.WriteFile(oldPath, []byte(legacySalt+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadOrCreateSalt(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt: %v", err)
	}
	if got != legacySalt {
		t.Errorf("LoadOrCreateSalt = %q, want the migrated legacy salt %q (hash lineage must survive)", got, legacySalt)
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("legacy salt file %s should have been removed after migration, stat err = %v", oldPath, err)
	}
	newContent, err := os.ReadFile(SaltPath(dir))
	if err != nil {
		t.Fatalf("new salt file not written: %v", err)
	}
	if strings.TrimSpace(string(newContent)) != legacySalt {
		t.Errorf("new salt file content = %q, want %q", newContent, legacySalt)
	}

	// A second call must not re-migrate or change the value (idempotent).
	got2, err := LoadOrCreateSalt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got2 != legacySalt {
		t.Errorf("second LoadOrCreateSalt = %q, want stable %q", got2, legacySalt)
	}
}
