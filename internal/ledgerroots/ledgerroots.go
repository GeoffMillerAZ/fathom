// Package ledgerroots is the one answer every reader of the user-level
// ledgers asks: which repository roots are the fleet's (PE-23,
// docs/design/progress-and-economics.md §11, observation O121).
//
// Scratch-clone runs of `fathom verify` and `fathom intent audit` appended
// rows keyed by their throwaway roots to ~/.fathom's ledgers: 13,514 of
// 19,639 verify rows and 91 of 180 intent rows on 2026-09-28. A reader that
// counts them reports proof history, burn-up and economics for work that is
// not the fleet's. So a reader keeps a row only when its root is a
// registered service root, and names how many rows it ignored; the file
// itself is never rewritten by a reader.
//
// The rule is exact canonical equality (absolute, clean, symlinks resolved,
// no trailing slash). A worktree or a subdirectory under a registered root
// is not that root: its rows describe a branch in flight, not the
// registered checkout (the owner's archive rule of 2026-09-29). A root
// under an OS temp directory is scratch and is ignored even when the
// registry lists it. An unread registry is not an empty one (I38): it
// excludes nothing and says why.
package ledgerroots

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The classes a row root falls in.
const (
	// ClassRegistered is a registered service root, not under a scratch dir.
	ClassRegistered = "registered"
	// ClassScratch is a root under a scratch (OS temp) directory, registered
	// or not.
	ClassScratch = "scratch"
	// ClassWorktree is a root strictly under a registered root: a linked
	// worktree, a push clone under workspace/, any subdirectory.
	ClassWorktree = "worktree"
	// ClassUnregistered is any other root, or a row that names none.
	ClassUnregistered = "unregistered"
	// ClassUnchecked is every root when the registry could not be read.
	ClassUnchecked = "unchecked"
)

// Roots is the fleet's registered service roots as a ledger reader sees
// them. Read is false when the registry could not be read: no row can then
// be checked, so none is excluded, and Note says why.
type Roots struct {
	Read bool
	Note string
	// roots are the canonical registered roots.
	roots map[string]bool
	// scratch are the scratch directories in every spelling a row root may
	// carry (cleaned and symlink-resolved).
	scratch []string
}

// RegisteredRoots is a registry that was read and lists paths, with the OS
// temp directories (TempDirs) as the scratch dirs.
func RegisteredRoots(paths []string) Roots {
	return RegisteredRootsWith(paths, TempDirs())
}

// RegisteredRootsWith is RegisteredRoots with the scratch dirs named: a
// root under any of them is ignored even when paths lists it.
func RegisteredRootsWith(paths, scratchDirs []string) Roots {
	r := Roots{Read: true, roots: make(map[string]bool, len(paths))}
	for _, p := range paths {
		if c := Canonical(p); c != "" {
			r.roots[c] = true
		}
	}
	seen := map[string]bool{}
	for _, d := range scratchDirs {
		for _, s := range []string{cleanAbs(d), Canonical(d)} {
			if s != "" && s != string(filepath.Separator) && !seen[s] {
				seen[s] = true
				r.scratch = append(r.scratch, s)
			}
		}
	}
	sort.Strings(r.scratch)
	return r
}

// Unread is a registry that could not be read, and why.
func Unread(note string) Roots { return Roots{Note: note} }

// TempDirs are the OS temp directories a scratch clone lives under:
// os.TempDir() (TMPDIR) and /tmp.
func TempDirs() []string {
	return []string{os.TempDir(), "/tmp"}
}

// Canonical is path absolute and clean, its symlinks resolved when it
// resolves (else the cleaned path), with no trailing slash; "" for "".
func Canonical(path string) string {
	c := cleanAbs(path)
	if c == "" {
		return ""
	}
	if r, err := filepath.EvalSymlinks(c); err == nil {
		return r
	}
	return c
}

// cleanAbs is path absolute and clean (Clean drops a trailing slash).
func cleanAbs(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if a, err := filepath.Abs(path); err == nil {
		return a
	}
	return filepath.Clean(path)
}

// List is the canonical registered roots, sorted.
func (r Roots) List() []string {
	out := make([]string, 0, len(r.roots))
	for k := range r.roots {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Key names this registry for a memo key: two registries with the same
// canonical roots and scratch dirs share it.
func (r Roots) Key() string {
	if !r.Read {
		return "unread|" + r.Note
	}
	return "read|" + strings.Join(r.List(), "\x00") + "|" + strings.Join(r.scratch, "\x00")
}

// Registered reports whether root is a registered service root: the
// registry was read, root's canonical path equals a registered root's, and
// it is not under a scratch dir.
func (r Roots) Registered(root string) bool {
	return r.classOf(Canonical(root)) == ClassRegistered
}

// Excludes reports whether a reader leaves a row under root out: only when
// the registry was read and root is not registered. An unread registry
// excludes nothing.
func (r Roots) Excludes(root string) bool {
	return r.Read && !r.Registered(root)
}

// Class names which class root falls in (the Class* constants).
func (r Roots) Class(root string) string { return r.classOf(Canonical(root)) }

// classOf classifies a canonical root.
func (r Roots) classOf(c string) string {
	if !r.Read {
		return ClassUnchecked
	}
	if c == "" {
		return ClassUnregistered
	}
	if under(c, r.scratch) {
		return ClassScratch
	}
	if r.roots[c] {
		return ClassRegistered
	}
	for reg := range r.roots {
		if isUnder(c, reg) {
			return ClassWorktree
		}
	}
	return ClassUnregistered
}

// under reports whether c is one of dirs or below one.
func under(c string, dirs []string) bool {
	for _, d := range dirs {
		if c == d || isUnder(c, d) {
			return true
		}
	}
	return false
}

// isUnder reports whether c is strictly below dir.
func isUnder(c, dir string) bool {
	return strings.HasPrefix(c, strings.TrimSuffix(dir, string(filepath.Separator))+string(filepath.Separator))
}

// Filter is one reader's pass over its rows: it canonicalizes each distinct
// row root once (a map local to the pass, never a package-level cache) and
// counts what it ignored.
type Filter struct {
	roots   Roots
	seen    map[string]verdict
	ignored Ignored
}

// verdict is one distinct row root's canonical path and class.
type verdict struct{ canon, class string }

// NewFilter starts one reader's pass under r.
func (r Roots) NewFilter() *Filter {
	f := &Filter{roots: r, seen: map[string]verdict{}}
	if !r.Read {
		f.ignored.RegistryNote = r.Note
	}
	return f
}

// Keep reports whether the reader keeps a row under root, counting it as
// ignored when not.
func (f *Filter) Keep(root string) bool {
	if !f.roots.Read {
		return true
	}
	v, ok := f.seen[root]
	if !ok {
		v.canon = Canonical(root)
		v.class = f.roots.classOf(v.canon)
		f.seen[root] = v
	}
	if v.class == ClassRegistered {
		return true
	}
	f.ignored.Add(v.canon, v.class == ClassScratch)
	return false
}

// Ignored is what the pass left out so far.
func (f *Filter) Ignored() Ignored { return f.ignored }

// Ignored is what a reader left out under the registered-roots rule.
type Ignored struct {
	// Rows is how many rows sat under a root that is not registered.
	Rows int `json:"rows"`
	// Roots are those rows' canonical roots, distinct and sorted ("" for a
	// row that names none).
	Roots []string `json:"roots,omitempty"`
	// ScratchRows is how many of Rows sat under a scratch (temp) dir.
	ScratchRows int `json:"scratch_rows,omitempty"`
	// RegistryNote is set when the registry could not be read: no row was
	// checked against it, so none was ignored.
	RegistryNote string `json:"registry_note,omitempty"`
}

// Add counts one ignored row under the canonical root.
func (i *Ignored) Add(root string, scratch bool) {
	i.Rows++
	if scratch {
		i.ScratchRows++
	}
	i.addRoot(root)
}

// Merge adds o's rows and roots to i.
func (i *Ignored) Merge(o Ignored) {
	i.Rows += o.Rows
	i.ScratchRows += o.ScratchRows
	for _, r := range o.Roots {
		i.addRoot(r)
	}
	if i.RegistryNote == "" {
		i.RegistryNote = o.RegistryNote
	}
}

// addRoot keeps Roots sorted and distinct.
func (i *Ignored) addRoot(root string) {
	k := sort.SearchStrings(i.Roots, root)
	if k < len(i.Roots) && i.Roots[k] == root {
		return
	}
	i.Roots = append(i.Roots, "")
	copy(i.Roots[k+1:], i.Roots[k:])
	i.Roots[k] = root
}

// Phrase names the exclusion for a denominator or a reason, noun singular:
// "5 verify-ledger rows under 4 unregistered roots (1 under a scratch dir)
// ignored". An unread registry phrases why nothing was ignored; a read one
// that ignored nothing phrases as "".
func (i Ignored) Phrase(noun string) string {
	if i.Rows == 0 {
		if i.RegistryNote != "" {
			return "no " + noun + " checked against the registered roots, none ignored: " + i.RegistryNote
		}
		return ""
	}
	s := count(i.Rows, noun) + " under " + count(len(i.Roots), "unregistered root")
	if i.ScratchRows > 0 {
		s += fmt.Sprintf(" (%s under a scratch dir)", thousands(i.ScratchRows))
	}
	return s + " ignored"
}

// count is "1 row" or "1,234 rows".
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return thousands(n) + " " + noun + "s"
}

// thousands is n with comma separators.
func thousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		return "-" + s
	}
	return s
}
