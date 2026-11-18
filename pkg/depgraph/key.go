package depgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"

	"github.com/geoffmilleraz/fathom/pkg/depgraph/hashw"
)

// InputKey returns the lowercase-hex SHA-256 of the framed inputs.
// Field order and each sort are part of the key format. Changing them
// rekeys all inputs and requires a format migration and an Epoch bump.
// Sorts operate on copies of the caller's slices.
func InputKey(in KeyInputs) Key {
	w := hashw.New()
	// Format v2 frames collection boundaries as well as scalar values.
	// The explicit format prefix partitions even an old caller's Epoch
	// from all v1 proofs: never fall back to the ambiguous v1 encoding.
	w.Str("depgraph/input-key/v2")
	w.Str(in.Domain)
	w.U32(in.Epoch)
	w.Str(in.Task)
	w.Str(in.Platform)

	srcs := append([]SourceRef(nil), in.Sources...)
	sort.Slice(srcs, func(i, j int) bool { return srcs[i].Path < srcs[j].Path })
	w.U32(uint32(len(srcs)))
	for _, s := range srcs {
		w.Str(s.Path)
		w.U32(s.Mode)
		w.Bytes(s.Digest)
	}

	w.U32(uint32(len(in.Deps)))
	for _, d := range sortedStrings(in.Deps) {
		w.Str(d)
	}

	w.Str(in.ToolchainLockDigest)
	w.Str(in.CueLang)
	w.U32(uint32(len(in.ToolchainPackages)))
	for _, name := range sortedKeys(in.ToolchainPackages) {
		w.Str(name)
		w.Str(in.ToolchainPackages[name])
	}

	w.U32(uint32(len(in.Env)))
	for _, k := range sortedKeys(in.Env) {
		w.Str(k)
		w.Str(in.Env[k])
	}

	// Args are positional: order is meaning, so no sort.
	w.U32(uint32(len(in.Args)))
	for _, a := range in.Args {
		w.Str(a)
	}

	w.U32(uint32(len(in.SecretNames)))
	for _, n := range sortedStrings(in.SecretNames) {
		w.Str(n)
	}

	return Key(hex.EncodeToString(w.Sum()))
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Materialize canonicalizes files under fsys into SourceRefs, applying
// these rules: a digest is of content bytes only (never mtime or size),
// bytes are not line-ending normalized, empty directories yield nothing,
// the exec bit and symlink-ness are the only mode bits that survive, and
// a symlink's digest is the sha256 of its target path string — the bytes
// behind it belong to the target's own ref. Paths must already be
// repo-relative and NFC. Callers normalize paths on NFD filesystems.
// Output paths are slash-separated as fs.FS defines. Each element of paths may
// be a file or a directory to walk. The result is sorted by path bytes.
func Materialize(fsys fs.FS, paths []string) ([]SourceRef, error) {
	var out []SourceRef
	for _, p := range paths {
		info, err := lstat(fsys, p)
		if err != nil {
			return nil, fmt.Errorf("depgraph: materialize %s: %w", p, err)
		}
		if !info.IsDir() {
			ref, err := refFor(fsys, p, info.Mode())
			if err != nil {
				return nil, err
			}
			out = append(out, ref)
			continue
		}
		err = fs.WalkDir(fsys, p, func(fp string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			ref, err := refFor(fsys, fp, fi.Mode())
			if err != nil {
				return err
			}
			out = append(out, ref)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("depgraph: materialize %s: %w", p, err)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// lstat never follows a symlink — a link is its own ref. Filesystems
// without ReadLinkFS cannot represent symlinks, so plain Stat is exact
// there, not a fallback approximation.
func lstat(fsys fs.FS, p string) (fs.FileInfo, error) {
	if rlf, ok := fsys.(fs.ReadLinkFS); ok {
		return rlf.Lstat(p)
	}
	return fs.Stat(fsys, p)
}

func refFor(fsys fs.FS, p string, mode fs.FileMode) (SourceRef, error) {
	// "." as a walk root would leak the artificial root name into the
	// key; fs.FS paths are already clean, so Clean only strips that.
	cp := path.Clean(p)

	if mode&fs.ModeSymlink != 0 {
		rlf, ok := fsys.(fs.ReadLinkFS)
		if !ok {
			return SourceRef{}, fmt.Errorf("depgraph: %s is a symlink but the filesystem cannot read links", cp)
		}
		target, err := rlf.ReadLink(cp)
		if err != nil {
			return SourceRef{}, fmt.Errorf("depgraph: readlink %s: %w", cp, err)
		}
		sum := sha256.Sum256([]byte(target))
		return SourceRef{Path: cp, Mode: ModeSymlink, Digest: sum[:]}, nil
	}

	f, err := fsys.Open(cp)
	if err != nil {
		return SourceRef{}, fmt.Errorf("depgraph: open %s: %w", cp, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return SourceRef{}, fmt.Errorf("depgraph: read %s: %w", cp, err)
	}
	var m uint32
	if mode.Perm()&0o111 != 0 {
		m |= ModeExec
	}
	return SourceRef{Path: cp, Mode: m, Digest: h.Sum(nil)}, nil
}
