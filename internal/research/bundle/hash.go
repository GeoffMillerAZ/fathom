package bundle

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SaltRelPath is the project-relative path a local, gitignored salt lives
// at (under workspace/, per CLAUDE.md's workspace/ convention). The salt is
// generated once per repo and reused: reuse is what makes a service's
// hashed identity STABLE across report runs (same service -> same hash),
// which is the whole point of "counts can be distinguished without being
// identifiable" — a fresh salt per run would make even repeat counts of the
// same service look like unrelated services.
const SaltRelPath = "workspace/artifacts/research/.salt"

// SaltPath resolves the salt file path under root.
func SaltPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(SaltRelPath))
}

// LoadOrCreateSalt reads the salt at path, generating and persisting a
// fresh 32-byte random salt on first use. The salt itself must NEVER be
// written into a bundle (Build never takes it as an output field, only as
// an input to HashService) — it exists solely to make locally-computed
// service hashes stable across runs, on this machine only.
func LoadOrCreateSalt(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read salt %s: %w", path, err)
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	salt := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	// 0o600: the salt is a local secret in the sense that anyone who has it
	// can correlate this machine's hashed service counts across bundles;
	// it is never meant to leave the machine.
	if err := os.WriteFile(path, []byte(salt+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write salt %s: %w", path, err)
	}
	return salt, nil
}

// HashService returns a stable, salted, non-reversible identifier for a
// service name: sha256(salt + "\x00" + service), hex-encoded (64 lowercase
// hex chars, matching schema/research-bundle/schema.cue's #ServiceHashCount
// pattern). The same service name with the same salt always hashes
// identically (so per-service COUNTS stay distinguishable across report
// runs), while two different salts produce unrelated hashes for the same
// name (so a bundle shared from two different machines cannot be
// correlated by service identity even if both happen to index the same
// repo).
func HashService(salt, service string) string {
	h := sha256.Sum256([]byte(salt + "\x00" + service))
	return hex.EncodeToString(h[:])
}
