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
// at, under workspace/state/ — deliberately OUTSIDE
// workspace/artifacts/research/ entirely (critic Finding 2, 2026-07-28):
// that directory also holds report.md, which is deliberately unsanitized,
// so a salt sitting inside it is a silent de-anonymization risk — an
// invisible dotfile a naive "share the research folder" tar/zip/Finder copy
// includes without the user ever seeing it, making every service_hash ever
// shared reversible by dictionary attack. The salt is generated once per
// repo and reused: reuse is what makes a service's hashed identity STABLE
// across report runs (same service -> same hash), which is the whole point
// of "counts can be distinguished without being identifiable" — a fresh
// salt per run would make even repeat counts of the same service look like
// unrelated services.
const SaltRelPath = "workspace/state/research.salt"

// legacySaltRelPath is where the salt lived before Finding 2's fix — inside
// the shareable artifacts tree. LoadOrCreateSalt migrates a salt found here
// to SaltRelPath (preserving its value, so an existing repo's hash lineage
// survives the relocation instead of silently resetting) and removes the
// old file.
const legacySaltRelPath = "workspace/artifacts/research/.salt"

// SaltPath resolves the current salt file path under root.
func SaltPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(SaltRelPath))
}

func legacySaltPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(legacySaltRelPath))
}

// LoadOrCreateSalt resolves the salt for the repo at root: the current path
// (SaltPath) if it already has one; else a legacy pre-Finding-2 salt,
// migrated in place and then removed from the old location; else a freshly
// generated 32-byte random salt. The salt itself must NEVER be written into
// a bundle (Build never takes it as an output field, only as an input to
// HashService) — it exists solely to make locally-computed service hashes
// stable across runs, on this machine only.
func LoadOrCreateSalt(root string) (string, error) {
	path := SaltPath(root)
	if b, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read salt %s: %w", path, err)
	}

	if s, migrated, err := migrateLegacySalt(root, path); err != nil {
		return "", err
	} else if migrated {
		return s, nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	salt := hex.EncodeToString(buf)
	if err := writeSalt(path, salt); err != nil {
		return "", err
	}
	return salt, nil
}

// migrateLegacySalt moves a pre-Finding-2 salt from legacySaltPath(root) to
// newPath, preserving its value, then deletes the old file. ok is false
// when no legacy salt exists (the ordinary fresh-generation path in
// LoadOrCreateSalt then runs).
func migrateLegacySalt(root, newPath string) (salt string, ok bool, err error) {
	old := legacySaltPath(root)
	b, rerr := os.ReadFile(old)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read legacy salt %s: %w", old, rerr)
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", false, nil
	}
	if err := writeSalt(newPath, s); err != nil {
		return "", false, err
	}
	// The migration already succeeded (the salt is safe at its new home);
	// a failed removal of the old copy would leave a harmless-but-residual
	// duplicate rather than block the run — but attempt it unconditionally
	// so the pre-fix leak surface (a salt inside the shareable directory)
	// is actually closed, not just superseded.
	_ = os.Remove(old)
	return s, true, nil
}

func writeSalt(path, salt string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	// 0o600: the salt is a local secret in the sense that anyone who has it
	// can correlate this machine's hashed service counts across bundles;
	// it is never meant to leave the machine.
	if err := os.WriteFile(path, []byte(salt+"\n"), 0o600); err != nil {
		return fmt.Errorf("write salt %s: %w", path, err)
	}
	return nil
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
