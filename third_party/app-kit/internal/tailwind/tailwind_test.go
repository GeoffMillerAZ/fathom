package tailwind

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	m, err := LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if !strings.HasPrefix(m.Version, "v4.") {
		t.Errorf("Version = %q, want v4.x", m.Version)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	if _, ok := m.Binaries[platform]; !ok {
		t.Errorf("no pin for the platform running the tests (%s)", platform)
	}
	for plat, bin := range m.Binaries {
		if len(bin.SHA256) != 64 {
			t.Errorf("%s: sha256 length %d, want 64", plat, len(bin.SHA256))
		}
		if bin.Asset == "" {
			t.Errorf("%s: empty asset name", plat)
		}
	}
}

func TestVerify_MismatchIsHardFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake-binary")
	if err := os.WriteFile(path, []byte("not the pinned binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := verify(path, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("verify accepted a file with the wrong checksum")
	}
	if !strings.Contains(err.Error(), "CHECKSUM MISMATCH") {
		t.Errorf("error should name the mismatch loudly, got: %v", err)
	}
}

func TestVerify_Match(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// sha256("hello")
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if err := verify(path, want); err != nil {
		t.Errorf("verify rejected a correct checksum: %v", err)
	}
}
