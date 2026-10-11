// Package receipt grows ADR-034's receipts into evidence a stranger checks
// with git (ADR-068, docs/vision/43-portable-receipts.md). Wave 1 holds the
// anchor core: AnchorAt names, for a path and a line range at a commit, the
// git object it reads, the excerpt's digest, the file's fathom fingerprint
// and whether the producer read the commit's own bytes (D3, section 6.2);
// AnchorRun gives every SARIF caller the run's provenance and each finding's
// excerpt fingerprint (D9, EVX-2, EVX-3).
//
// Git is read only: fixed argument lists (rev-parse, cat-file) through
// internal/gitro, which never writes the repository, fetches or prompts. A
// smudge filter that would download (Git LFS) is told to skip.
package receipt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// State is an anchor's state (D3).
type State string

const (
	// StateCommitted: the producer read the commit's bytes, and git's
	// filters leave them as the blob holds them. The git rung checks it.
	StateCommitted State = "committed"
	// StateUncommitted: the producer read other bytes than the commit
	// holds (an edited or new file). The anchor and its run read clean:
	// false, and the anchor carries no excerpt digest.
	StateUncommitted State = "uncommitted"
	// StateFiltered: git's filters change the bytes between the blob and
	// a checkout (autocrlf, eol, ident, working-tree-encoding, an LFS
	// pointer). The excerpt digest is over the checkout's bytes; there is
	// no git rung.
	StateFiltered State = "filtered"
	// StateWithheld: the file is suppressed or may hold secrets (ADR-065).
	// No excerpt and no digest of any of its bytes leave.
	StateWithheld State = "withheld"
)

// Commit is one repository at one commit, resolved once for every anchor a
// run cites.
type Commit struct {
	Root         string // the work tree's top level, absolute
	ID           string // the full commit object id
	ObjectFormat string // "sha1" or "sha256"
}

// Span is a repo-relative, slash-separated path and a 1-based inclusive
// line range; StartLine = EndLine = 0 is the whole file.
type Span struct {
	Path      string
	StartLine int
	EndLine   int
}

// Source is what the producer read the file from.
type Source struct {
	// Fingerprint is fusion.FingerprintHash of the bytes the producer
	// read: the index's stored fingerprint for an answer from the index.
	// "" means the producer read the work tree's file in place.
	Fingerprint string
	// Withheld is why the file's bytes may not leave (a suppression's
	// reason, or the kind that makes it secret-bearing); "" for none.
	Withheld string
}

// Anchor is one anchored span (D3). Field names are the receipt's.
type Anchor struct {
	Commit       string `json:"commit"`
	ObjectFormat string `json:"object_format"`
	Path         string `json:"path"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	// BlobID is the git blob id of Path at Commit; "" when the commit
	// holds no such file, or the anchor is withheld.
	BlobID string `json:"blob_id,omitempty"`
	// ExcerptSHA256 is the excerpt's digest (verify.ExcerptSHA256, Review
	// Kit's SpanRef.ContentHash formula) over the checkout's bytes; ""
	// when uncommitted or withheld, or the checkout's bytes are unknown.
	ExcerptSHA256 string `json:"excerpt_sha256,omitempty"`
	// FathomFingerprint is fusion.FingerprintHash of the checkout's bytes:
	// the value the index stores for the file read at Commit.
	FathomFingerprint string `json:"fathom_fingerprint,omitempty"`
	State             State  `json:"state"`
	// Clean is true when the producer read the commit's bytes (its
	// fingerprint equals FathomFingerprint).
	Clean  bool   `json:"clean"`
	Reason string `json:"reason,omitempty"`
}

// ResolveCommit resolves rev ("" is HEAD) in the repository holding dir:
// the work tree's top level, the full commit id and the object format.
func ResolveCommit(ctx context.Context, dir, rev string) (Commit, error) {
	if rev == "" {
		rev = "HEAD"
	}
	if strings.HasPrefix(rev, "-") {
		return Commit{}, fmt.Errorf("receipt: revision %q looks like an option", rev)
	}
	out, err := gitro.Output(ctx, dir, "rev-parse", "--show-toplevel", "--show-object-format")
	if err != nil {
		return Commit{}, fmt.Errorf("receipt: %s is not inside a git work tree: %w", dir, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return Commit{}, fmt.Errorf("receipt: unexpected rev-parse output %q", out)
	}
	root, format := lines[0], lines[1]
	if format != "sha1" && format != "sha256" {
		return Commit{}, fmt.Errorf("receipt: unknown object format %q", format)
	}
	out, err = gitro.Output(ctx, dir, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return Commit{}, fmt.Errorf("receipt: %q names no commit in %s", rev, root)
	}
	id := strings.TrimSpace(string(out))
	if !isObjectID(id, format) {
		return Commit{}, fmt.Errorf("receipt: unexpected commit id %q", id)
	}
	return Commit{Root: root, ID: id, ObjectFormat: format}, nil
}

var hexRE = regexp.MustCompile(`^[0-9a-f]+$`)

func isObjectID(id, format string) bool {
	n := 40
	if format == "sha256" {
		n = 64
	}
	return len(id) == n && hexRE.MatchString(id)
}

// AnchorAt anchors s at c (EVX-1): the object format, the blob id by
// read-only git, the excerpt's digest, the file's fathom fingerprint, and a
// state. src says what the producer read; it decides uncommitted (its
// fingerprint differs from the checkout's) and withheld. A range the file
// does not hold, a path that is not a clean repo-relative file path, and a
// path neither the commit nor the work tree holds are errors.
func AnchorAt(ctx context.Context, c Commit, s Span, src Source) (Anchor, error) {
	if err := checkSpan(s); err != nil {
		return Anchor{}, err
	}
	a := Anchor{Commit: c.ID, ObjectFormat: c.ObjectFormat, Path: s.Path, StartLine: s.StartLine, EndLine: s.EndLine}

	read, haveRead, err := producerFingerprint(c, s.Path, src)
	if err != nil {
		return Anchor{}, err
	}

	out, err := gitro.Output(ctx, c.Root, "rev-parse", "--verify", "--quiet", "--end-of-options", c.ID+":"+s.Path)
	if err != nil {
		// The commit holds no such file: the producer read bytes it
		// does not hold (a new or untracked file).
		if !haveRead {
			return Anchor{}, fmt.Errorf("receipt: %s is neither at %s nor in the work tree", s.Path, short(c.ID))
		}
		a.State, a.Reason = StateUncommitted, "the commit holds no such file"
		if src.Withheld != "" {
			a.State, a.Reason = StateWithheld, src.Withheld
		}
		return a, nil
	}
	blobID := strings.TrimSpace(string(out))
	blob, err := gitro.Output(ctx, c.Root, "cat-file", "blob", blobID)
	if err != nil {
		return Anchor{}, fmt.Errorf("receipt: %s at %s is not a file", s.Path, short(c.ID))
	}
	checkout, err := filteredBytes(ctx, c, s.Path)
	if err != nil {
		return Anchor{}, err
	}

	filtered, why := !bytes.Equal(checkout, blob), "git's filters change the bytes between the blob and a checkout"
	if oid, ok := lfsPointerOID(blob); ok {
		// The checkout holds the LFS object, which fathom never fetches:
		// the work tree's file stands in for it when its digest is the
		// pointer's.
		filtered, why = true, "an LFS pointer: git holds the pointer, a checkout the object"
		checkout = nil
		if wt, rerr := os.ReadFile(filepath.Join(c.Root, filepath.FromSlash(s.Path))); rerr == nil && sha256Hex(wt) == oid {
			checkout = wt
		}
	}
	if checkout != nil {
		a.FathomFingerprint = fusion.FingerprintHash(checkout)
		a.Clean = haveRead && read == a.FathomFingerprint
	}
	a.BlobID = blobID

	switch {
	case src.Withheld != "":
		a.State, a.Reason = StateWithheld, src.Withheld
		a.BlobID, a.FathomFingerprint = "", ""
		return a, nil
	case checkout == nil:
		a.State, a.Reason = StateFiltered, why+"; the object is not in the work tree"
		return a, nil
	case !a.Clean:
		// The producer's lines count other bytes: no digest, and no range
		// check against the commit's file.
		a.State, a.Reason = StateUncommitted, "the producer read other bytes than the commit holds"
		return a, nil
	case filtered:
		a.State, a.Reason = StateFiltered, why
	default:
		a.State = StateCommitted
	}
	digest, ok := verify.ExcerptSHA256(checkout, s.StartLine, s.EndLine)
	if !ok {
		return Anchor{}, fmt.Errorf("receipt: %s at %s holds no lines %d-%d", s.Path, short(c.ID), s.StartLine, s.EndLine)
	}
	a.ExcerptSHA256 = digest
	return a, nil
}

// checkSpan refuses a path that is not a clean, relative, slash-separated
// file path (it is spliced into a "<commit>:<path>" argument), and a range
// that is neither the whole file nor 1 <= start <= end.
func checkSpan(s Span) error {
	p := s.Path
	switch {
	case p == "" || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "-") || filepath.IsAbs(p):
		return fmt.Errorf("receipt: %q is not a repo-relative path", p)
	case path.Clean(p) != p || p == "." || p == ".." || strings.HasPrefix(p, "../") || strings.Contains(p, "\\"):
		return fmt.Errorf("receipt: %q is not a clean repo-relative path", p)
	}
	if s.StartLine == 0 && s.EndLine == 0 {
		return nil
	}
	if s.StartLine < 1 || s.EndLine < s.StartLine {
		return fmt.Errorf("receipt: lines %d-%d are not a range", s.StartLine, s.EndLine)
	}
	return nil
}

// producerFingerprint is the fingerprint of the bytes the producer read:
// src's, or the work tree file's when src names none. false when the work
// tree holds no such file.
func producerFingerprint(c Commit, rel string, src Source) (string, bool, error) {
	if src.Fingerprint != "" {
		return src.Fingerprint, true, nil
	}
	b, err := os.ReadFile(filepath.Join(c.Root, filepath.FromSlash(rel)))
	switch {
	case err == nil:
		return fusion.FingerprintHash(b), true, nil
	case errors.Is(err, os.ErrNotExist):
		return "", false, nil
	default:
		return "", false, fmt.Errorf("receipt: read %s: %w", rel, err)
	}
}

// filteredBytes is `git cat-file --filters <commit>:<path>`: the bytes a
// checkout of the commit writes. An LFS smudge would download the object,
// so it is told to skip (the pointer comes back unchanged).
func filteredBytes(ctx context.Context, c Commit, rel string) ([]byte, error) {
	cmd, cleanup, err := gitro.Command(ctx, c.Root, "cat-file", "--filters", c.ID+":"+rel)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	cmd.Env = append(cmd.Env, "GIT_LFS_SKIP_SMUDGE=1")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("receipt: git cat-file --filters %s: %w", rel, err)
	}
	return out, nil
}

// lfsPointerOID is the sha256 object id an LFS pointer file names, and
// whether blob is one (the Git LFS pointer format: a version line, then
// "oid sha256:<hex>").
func lfsPointerOID(blob []byte) (string, bool) {
	if len(blob) > 1024 || !bytes.HasPrefix(blob, []byte("version https://git-lfs.github.com/spec/v1\n")) {
		return "", false
	}
	for _, line := range strings.Split(string(blob), "\n") {
		if oid, ok := strings.CutPrefix(line, "oid sha256:"); ok && len(oid) == 64 && hexRE.MatchString(oid) {
			return oid, true
		}
	}
	return "", false
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
