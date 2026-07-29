package bundle

import (
	"path/filepath"
	"regexp"
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
	path := SaltPath(dir)

	s1, err := LoadOrCreateSalt(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt (create): %v", err)
	}
	if s1 == "" {
		t.Fatal("LoadOrCreateSalt returned an empty salt")
	}

	s2, err := LoadOrCreateSalt(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt (reload): %v", err)
	}
	if s1 != s2 {
		t.Errorf("salt not stable across calls: %q vs %q", s1, s2)
	}

	// A hash computed with the persisted salt must be stable across
	// "process restarts" (simulated by two independent LoadOrCreateSalt
	// calls against the same path).
	if HashService(s1, "svc") != HashService(s2, "svc") {
		t.Errorf("hash unstable despite stable salt")
	}
}

func TestLoadOrCreateSalt_TwoReposGetDifferentSalts(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	s1, err := LoadOrCreateSalt(SaltPath(dir1))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := LoadOrCreateSalt(SaltPath(dir2))
	if err != nil {
		t.Fatal(err)
	}
	if s1 == s2 {
		t.Errorf("two independently-created salts collided (astronomically unlikely, or rand.Read is broken)")
	}
}

func TestSaltPath_UnderWorkspaceArtifacts(t *testing.T) {
	got := SaltPath("/repo")
	want := filepath.Join("/repo", "workspace", "artifacts", "research", ".salt")
	if got != want {
		t.Errorf("SaltPath = %q, want %q", got, want)
	}
}
