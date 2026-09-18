package main

// reviewenvelope.go — FL-15/FL-16 helpers `fathom review` uses to build its
// --manifest verification and its ReviewResponse envelope's ReviewInputs
// (internal/analysis/review.ReviewInputs): an independent, cmd/fathom-local
// digest computation — NOT internal/analysis/reviewkey (FL-17), which is a
// separate contract computed on demand by `fathom review-key`. The two are
// deliberately not unified: ReviewInputs is a lightweight per-run
// fingerprint carried on every review response, while reviewkey.Key is the
// heavier, explicitly-requested reuse-eligibility token (vision 30 §3.1 vs
// §3.2).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/projectrules"
)

// reviewManifestEntry is one `--manifest FILE` row: a changed path plus the
// caller-declared sha256 it must currently match.
type reviewManifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// loadReviewManifest reads and JSON-decodes a `--manifest FILE`.
func loadReviewManifest(path string) ([]reviewManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --manifest %q: %w", path, err)
	}
	var entries []reviewManifestEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse --manifest %q: %w", path, err)
	}
	return entries, nil
}

// verifyReviewManifest resolves each manifest entry's CURRENT sha256 under
// base (the service's corpus root, or "" to resolve relative to the process
// CWD) and returns the changed-file list in manifest order. mismatch names
// the FIRST path whose current content does not match its declared sha256
// — a missing/unreadable file counts as a mismatch too. The caller refuses
// loudly (exit 2) rather than re-reading silently; never a partial changed-
// file list on a mismatch.
func verifyReviewManifest(base string, entries []reviewManifestEntry) (changedFiles []string, mismatch string) {
	for _, e := range entries {
		full := e.Path
		if base != "" {
			full = filepath.Join(base, e.Path)
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, e.Path
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != e.SHA256 {
			return nil, e.Path
		}
		changedFiles = append(changedFiles, e.Path)
	}
	return changedFiles, ""
}

// reviewPayloadDigest is the idempotency-key contract's PayloadDigest
// (core.ReviewRun): a content hash of the REQUEST this run answers —
// service, mode, and the ref/manifest identity plus the resolved changed-
// file set — never the response. A repeated --idempotency-key with the
// SAME digest replays; a different digest refuses (exit 2).
func reviewPayloadDigest(service, mode, since, manifestPath string, changedFiles []string) string {
	sorted := append([]string(nil), changedFiles...)
	sort.Strings(sorted)
	h := sha256.New()
	fmt.Fprintf(h, "service=%s\x00mode=%s\x00since=%s\x00manifest=%s\x00", service, mode, since, manifestPath)
	for _, f := range sorted {
		fmt.Fprintf(h, "file=%s\x00", f)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// reviewManifestDigest hashes the sorted "path=sha256(content)" lines for
// changedFiles, using content already resolved by the caller (the same
// source-content map the C2-1 findings bridge reads) — ReviewInputs'
// manifest_digest.
func reviewManifestDigest(changedFiles []string, content map[string][]byte) string {
	lines := make([]string, 0, len(changedFiles))
	for _, f := range changedFiles {
		sum := sha256.Sum256(content[f])
		lines = append(lines, f+"="+hex.EncodeToString(sum[:]))
	}
	sort.Strings(lines)
	return sha256Hex(strings.Join(lines, "\n"))
}

// reviewRulesDigest hashes the project's rule pack
// (.fathom/rules/archconformance.cue) + .fathom/guard.cue + any
// project-local layer manifests (.fathom/layers/*/layer.cue) under
// corpusRoot — a missing file is simply absent from the digest (no rules
// configured is a valid, honestly-empty state, matching
// projectrules.ResolveRules' own SourceNone case). "" when corpusRoot is
// unknown.
func reviewRulesDigest(corpusRoot string) string {
	if corpusRoot == "" {
		return ""
	}
	paths := []string{
		filepath.Join(corpusRoot, projectrules.RelPath),
		filepath.Join(corpusRoot, ".fathom", "guard.cue"),
	}
	if matches, _ := filepath.Glob(filepath.Join(corpusRoot, ".fathom", "layers", "*", "layer.cue")); len(matches) > 0 {
		paths = append(paths, matches...)
	}
	return digestExistingFiles(paths)
}

// reviewPlansDigest hashes every file under corpusRoot/docs/plans, sorted —
// ReviewInputs' plans_digest. "" when corpusRoot is unknown or docs/plans
// does not exist.
func reviewPlansDigest(corpusRoot string) string {
	if corpusRoot == "" {
		return ""
	}
	root := filepath.Join(corpusRoot, "docs", "plans")
	var paths []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil //nolint:nilerr — a missing docs/plans is a valid, honestly-empty state
		}
		paths = append(paths, p)
		return nil
	})
	return digestExistingFiles(paths)
}

// digestExistingFiles hashes the sorted "path\x00content\x00" bytes of every
// path in paths that can be read; an unreadable path is simply skipped
// (never hashed as empty) — the digest is best-effort over what a caller
// resolved to exist, not a refusal surface (that belongs to --manifest
// verification, which DOES refuse on a mismatch).
func digestExistingFiles(paths []string) string {
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	h := sha256.New()
	for _, p := range sorted {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
