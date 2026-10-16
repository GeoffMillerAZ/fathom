package gitro

// blob.go — one file's committed bytes at a revision, for the two readers
// that must never answer from the working tree: the findings importer,
// hashing a finding's cited lines at the revision its producer observed
// (WAE-1), and read_source's `ref` (WAE-3). Three read-only git commands in
// the corpus root, through Command (no lock, no lazy fetch, no prompt):
// rev-parse peels the revision to a commit, ls-tree names the entry's mode
// and size without reading it, and cat-file prints the blob. A symlink, a
// submodule or a directory at the revision is refused by its entry mode,
// never followed, and a blob over the caller's cap is refused before it is
// read.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// Errors BlobAt returns, each a distinct reason a caller can name.
var (
	// ErrNoRevision: the revision is empty. `git show :path` would read the
	// index, which is the working tree's staging area, not a commit.
	ErrNoRevision = errors.New("no revision given")
	// ErrBadRevision: the revision starts with '-' (it would parse as an
	// option) or carries whitespace, a control character or a ':'.
	ErrBadRevision = errors.New("revision refused")
	// ErrUnknownRevision: the repository has no commit by that name.
	ErrUnknownRevision = errors.New("unknown revision")
	// ErrBadPath: the path is empty, absolute, or climbs out of the root.
	ErrBadPath = errors.New("path must be relative to the corpus root")
	// ErrPathAbsent: the commit has no entry at the path.
	ErrPathAbsent = errors.New("path absent at revision")
	// ErrNotBlob: the entry at the path is not a regular file (an
	// *EntryError names which kind it is).
	ErrNotBlob = errors.New("not a regular file at revision")
	// ErrBlobTooLarge: the blob is over the caller's cap (a *SizeError
	// carries both sizes).
	ErrBlobTooLarge = errors.New("blob over the read cap")
)

// EntryError is BlobAt's refusal of an entry that is not a regular file:
// Kind is "symlink", "submodule" or "directory". It matches ErrNotBlob.
type EntryError struct {
	Path string
	Kind string
}

func (e *EntryError) Error() string {
	return fmt.Sprintf("%s is a %s at the revision, not a regular file", e.Path, e.Kind)
}

// Is makes an *EntryError match ErrNotBlob.
func (e *EntryError) Is(target error) bool { return target == ErrNotBlob }

// SizeError is BlobAt's refusal of a blob over the cap. It matches
// ErrBlobTooLarge.
type SizeError struct {
	Path string
	Size int64
	Max  int64
}

func (e *SizeError) Error() string {
	return fmt.Sprintf("%s is %d bytes at the revision, over the %d-byte cap", e.Path, e.Size, e.Max)
}

// Is makes a *SizeError match ErrBlobTooLarge.
func (e *SizeError) Is(target error) bool { return target == ErrBlobTooLarge }

// BlobAt returns the bytes of rel (slash-separated, relative to root) as
// committed at rev, and the commit rev names. root is a directory inside a
// git work tree (a monorepo service's subdirectory included: rel is read
// relative to it, as git reads a path relative to its working directory).
// maxBytes > 0 refuses a larger blob before reading it. It never reads the
// working tree or the index, never fetches and never prompts.
//
// The errors name why: ErrNoRevision, ErrBadRevision, ErrNotRepository,
// ErrUnknownRevision, ErrBadPath, ErrPathAbsent, an *EntryError
// (ErrNotBlob) or a *SizeError (ErrBlobTooLarge); any other error is git
// failing to read an object the commit names (a blob missing from a partial
// clone).
func BlobAt(ctx context.Context, root, rev, rel string, maxBytes int64) (data []byte, commit string, err error) {
	if rev == "" {
		return nil, "", ErrNoRevision
	}
	if !revisionAllowed(rev) {
		return nil, "", fmt.Errorf("%w: %q", ErrBadRevision, rev)
	}
	clean, ok := cleanRel(rel)
	if !ok {
		return nil, "", fmt.Errorf("%w: %q", ErrBadPath, rel)
	}
	if _, _, _, ferr := findGitDirs(root); ferr != nil {
		return nil, "", ErrNotRepository
	}

	out, rerr := Output(ctx, root, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if rerr != nil {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", fmt.Errorf("%w: %q", ErrUnknownRevision, rev)
	}
	commit = strings.TrimSpace(string(out))
	if !isObjectName(commit) {
		return nil, "", fmt.Errorf("%w: %q", ErrUnknownRevision, rev)
	}

	listing, lerr := Output(ctx, root, "--literal-pathspecs", "ls-tree", "-l", "-z", commit, "--", clean)
	if lerr != nil {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", fmt.Errorf("ls-tree %s: %w", clean, lerr)
	}
	mode, object, size, found := lsTreeEntry(listing, clean)
	if !found {
		return nil, "", fmt.Errorf("%w: %s at %s", ErrPathAbsent, clean, rev)
	}
	switch mode {
	case "100644", "100755", "100664":
	case "120000":
		return nil, "", &EntryError{Path: clean, Kind: "symlink"}
	case "160000":
		return nil, "", &EntryError{Path: clean, Kind: "submodule"}
	case "040000":
		return nil, "", &EntryError{Path: clean, Kind: "directory"}
	default:
		return nil, "", &EntryError{Path: clean, Kind: "mode " + mode + " entry"}
	}
	if maxBytes > 0 && size > maxBytes {
		return nil, "", &SizeError{Path: clean, Size: size, Max: maxBytes}
	}

	data, cerr := Output(ctx, root, "cat-file", "blob", object)
	if cerr != nil {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", fmt.Errorf("read %s at %s: %w", clean, rev, cerr)
	}
	return data, commit, nil
}

// revisionAllowed refuses a revision git could read as an option, or one
// carrying whitespace, a control character or a ':' (`rev:path` would name
// a tree path, not a commit).
func revisionAllowed(rev string) bool {
	if strings.HasPrefix(rev, "-") {
		return false
	}
	for _, r := range rev {
		if r <= ' ' || r == 0x7f || r == ':' {
			return false
		}
	}
	return true
}

// cleanRel cleans a slash-separated relative path, refusing an empty,
// absolute or climbing one.
func cleanRel(rel string) (string, bool) {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.ContainsRune(rel, 0) {
		return "", false
	}
	clean := path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

// lsTreeEntry finds name's entry in `git ls-tree -l -z` output: records
// "<mode> SP <type> SP <object> SP+ <size> TAB <path>" separated by NUL,
// size "-" for a tree or a submodule.
func lsTreeEntry(listing []byte, name string) (mode, object string, size int64, found bool) {
	for _, rec := range bytes.Split(listing, []byte{0}) {
		tab := bytes.IndexByte(rec, '\t')
		if tab < 0 || string(rec[tab+1:]) != name {
			continue
		}
		fields := strings.Fields(string(rec[:tab]))
		if len(fields) != 4 {
			continue
		}
		size = -1
		if n, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
			size = n
		}
		return fields[0], fields[2], size, true
	}
	return "", "", 0, false
}
