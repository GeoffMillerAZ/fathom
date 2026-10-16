// Package kitrewrite keeps fathom one self-contained Go module (ADR-070).
//
// The private kits fathom builds on (agentic-kit, app-kit) are mirrored in
// tree under third_party/<name> (ADR-019). Mirrored as nested modules, they
// needed a require plus a local replace in go.mod, and `go install
// module@version` refuses any module whose go.mod carries a replace, while the
// private module paths are unreachable on the proxy. So each mirror is instead
// a plain directory of this module: its go.mod, go.sum, go.work and
// go.work.sum are kept under a ".upstream" suffix (content intact), and every
// import of the kit's upstream path is rewritten to the mirror's path inside
// this module (github.com/geoffmilleraz/app-kit/x becomes
// github.com/geoffmilleraz/fathom/third_party/app-kit/x).
//
// The rewrite is a pure byte substitution of import path literals, found by
// go/parser (ImportsOnly), in .go and .templ files: comments, strings and
// raw-string fixtures that merely name a kit are untouched, and nothing is
// reformatted, so Reverse restores the upstream form byte for byte (DS-2).
// That reverse is what lets vendor-check prove a mirror still matches its
// pinned upstream commit. testdata, nested modules (a directory below the
// tree root holding its own go.mod), dot, underscore, vendor and
// node_modules directories are never entered: none of them is compiled as a
// package of this module.
package kitrewrite

import (
	"bytes"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

// MirrorNames are the kit mirrors under third_party/, in rewrite order.
var MirrorNames = []string{"agentic-kit", "app-kit"}

// UpstreamSuffix marks a mirror's module file kept for provenance only.
const UpstreamSuffix = ".upstream"

// moduleFiles make a directory a module or a workspace root; a mirror keeps
// them as <name>.upstream.
var moduleFiles = []string{"go.mod", "go.sum", "go.work", "go.work.sum"}

// rootSkips are repository-root directories that hold scratch or clones,
// never this module's source (all gitignored).
var rootSkips = map[string]bool{"workspace": true, "ref": true, "tmp": true, "bin": true}

// Direction selects the rewrite: Forward to the in-module form, Reverse to
// the kit's upstream form.
type Direction int

const (
	// Forward rewrites upstream kit imports to this module's mirror paths
	// and moves each mirror's module files aside.
	Forward Direction = iota
	// Reverse restores the upstream form exactly.
	Reverse
)

func (d Direction) String() string {
	if d == Reverse {
		return "reverse"
	}
	return "forward"
}

// Report counts what one rewrite changed.
type Report struct {
	Files   int // .go and .templ files rewritten
	Imports int // import path literals rewritten
	Moved   int // module files renamed to or from their .upstream name
}

func (r *Report) add(o Report) {
	r.Files += o.Files
	r.Imports += o.Imports
	r.Moved += o.Moved
}

// mapping is one kit's upstream import path and its in-module path.
type mapping struct{ upstream, inModule string }

// rewritePath maps an import path through ms in direction d; ok is false when
// no mapping applies.
func rewritePath(p string, ms []mapping, d Direction) (string, bool) {
	for _, m := range ms {
		from, to := m.upstream, m.inModule
		if d == Reverse {
			from, to = to, from
		}
		if p == from {
			return to, true
		}
		if strings.HasPrefix(p, from+"/") {
			return to + p[len(from):], true
		}
	}
	return "", false
}

// ModulePath reads the module path from dir/go.mod.
func ModulePath(dir string) (string, error) {
	p := filepath.Join(dir, "go.mod")
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	mp := modfile.ModulePath(raw)
	if mp == "" {
		return "", fmt.Errorf("%s has no module line", p)
	}
	return mp, nil
}

// mirrorUpstream reads a mirror's own module path from go.mod or, once moved
// aside, go.mod.upstream; "" when the mirror is absent.
func mirrorUpstream(dir string) (string, error) {
	for _, f := range []string{"go.mod" + UpstreamSuffix, "go.mod"} {
		raw, err := os.ReadFile(filepath.Join(dir, f))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if mp := modfile.ModulePath(raw); mp != "" {
			return mp, nil
		}
		return "", fmt.Errorf("%s has no module line", filepath.Join(dir, f))
	}
	return "", nil
}

// repoMappings reads every present mirror's upstream path under root.
func repoMappings(root, module string) ([]mapping, error) {
	var ms []mapping
	for _, name := range MirrorNames {
		up, err := mirrorUpstream(filepath.Join(root, "third_party", name))
		if err != nil {
			return nil, err
		}
		if up != "" {
			ms = append(ms, mapping{upstream: up, inModule: module + "/third_party/" + name})
		}
	}
	return ms, nil
}

// RewriteRepo rewrites the repository at root in direction d: each kit
// mirror under third_party/ (module files moved, imports rewritten) and
// root's own Go and templ files (imports rewritten).
func RewriteRepo(root string, d Direction) (Report, error) {
	module, err := ModulePath(root)
	if err != nil {
		return Report{}, err
	}
	ms, err := repoMappings(root, module)
	if err != nil {
		return Report{}, err
	}
	var rep Report
	for _, name := range MirrorNames {
		dir := filepath.Join(root, "third_party", name)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		r, err := rewriteMirror(dir, ms, d)
		if err != nil {
			return rep, err
		}
		rep.add(r)
	}
	r, err := rewriteTree(root, true, ms, d)
	rep.add(r)
	return rep, err
}

// RewriteKit rewrites one kit copy at dir alone, the shape sync-kit and
// vendor-check work on: dir's base name is the mirror's name, module the
// consumer module it is (or was) mirrored into. Imports of a sibling mirror
// are mapped too, by the sibling's name under the kit's own owner path.
func RewriteKit(dir, module string, d Direction) (Report, error) {
	name := filepath.Base(dir)
	up, err := mirrorUpstream(dir)
	if err != nil {
		return Report{}, err
	}
	if up == "" {
		return Report{}, fmt.Errorf("%s holds neither go.mod nor go.mod%s: not a kit copy", dir, UpstreamSuffix)
	}
	ms := []mapping{{upstream: up, inModule: module + "/third_party/" + name}}
	owner := path.Dir(up)
	for _, other := range MirrorNames {
		if other != name {
			ms = append(ms, mapping{upstream: owner + "/" + other, inModule: module + "/third_party/" + other})
		}
	}
	return rewriteMirror(dir, ms, d)
}

func rewriteMirror(dir string, ms []mapping, d Direction) (Report, error) {
	var rep Report
	moved, err := moveModuleFiles(dir, d)
	rep.Moved = moved
	if err != nil {
		return rep, err
	}
	r, err := rewriteTree(dir, false, ms, d)
	rep.add(r)
	return rep, err
}

// moveModuleFiles renames dir's module files to their .upstream name
// (Forward) or back (Reverse). Both names present is a half-synced copy:
// refused, never resolved by picking one.
func moveModuleFiles(dir string, d Direction) (int, error) {
	n := 0
	for _, f := range moduleFiles {
		from, to := filepath.Join(dir, f), filepath.Join(dir, f+UpstreamSuffix)
		if d == Reverse {
			from, to = to, from
		}
		if _, err := os.Lstat(from); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return n, err
		}
		if _, err := os.Lstat(to); err == nil {
			return n, fmt.Errorf("%s and %s both exist: a half-synced mirror (re-run task sync-kit)", from, to)
		}
		if err := os.Rename(from, to); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// walkSources calls fn for every .go and .templ file of tree that this
// module compiles, in lexical order. A repository tree (repo true) also
// skips its root's scratch directories and its kit mirrors, which
// rewriteMirror owns; a kit copy is walked whole.
func walkSources(tree string, repo bool, fn func(p string) error) error {
	return filepath.WalkDir(tree, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() {
			if p == tree {
				return nil
			}
			name := de.Name()
			if name == "testdata" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			if repo && filepath.Dir(p) == tree && rootSkips[name] {
				return filepath.SkipDir
			}
			if repo && filepath.Dir(p) == filepath.Join(tree, "third_party") && isMirror(name) {
				return filepath.SkipDir // rewritten by rewriteMirror
			}
			if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
				return filepath.SkipDir // a nested module resolves its own imports
			}
			return nil
		}
		if !de.Type().IsRegular() || !(strings.HasSuffix(p, ".go") || strings.HasSuffix(p, ".templ")) {
			return nil
		}
		return fn(p)
	})
}

func isMirror(name string) bool {
	for _, m := range MirrorNames {
		if m == name {
			return true
		}
	}
	return false
}

// importSpan is one import path literal: its byte offset and length in the
// source, its unquoted path and its line.
type importSpan struct {
	off, end int
	path     string
	line     int
}

// imports finds src's import path literals. A .templ file's Go header
// (package clause and imports) parses as Go; ImportsOnly stops before the
// templ body.
func imports(p string, src []byte) ([]importSpan, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, p, src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var out []importSpan
	for _, is := range f.Imports {
		ip, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: import %s: %w", p, is.Path.Value, err)
		}
		pos := fset.Position(is.Path.Pos())
		out = append(out, importSpan{off: pos.Offset, end: pos.Offset + len(is.Path.Value), path: ip, line: pos.Line})
	}
	return out, nil
}

func rewriteTree(tree string, repo bool, ms []mapping, d Direction) (Report, error) {
	var rep Report
	if len(ms) == 0 {
		return rep, nil
	}
	err := walkSources(tree, repo, func(p string) error {
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		spans, err := imports(p, src)
		if err != nil {
			return err
		}
		var out bytes.Buffer
		last, n := 0, 0
		for _, s := range spans {
			to, ok := rewritePath(s.path, ms, d)
			if !ok {
				continue
			}
			q := src[s.off] // keep the literal's quote, " or `
			out.Write(src[last:s.off])
			out.WriteByte(q)
			out.WriteString(to)
			out.WriteByte(q)
			last = s.end
			n++
		}
		if n == 0 {
			return nil
		}
		out.Write(src[last:])
		st, err := os.Stat(p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(p, out.Bytes(), st.Mode().Perm()); err != nil {
			return err
		}
		rep.Files++
		rep.Imports += n
		return nil
	})
	return rep, err
}

// Check reports every way the repository at root is not one self-contained
// module (DS-1), sorted; none means `go install <module>/cmd/...@<version>`
// can resolve it from the public proxy alone:
//
//   - go.mod carries a replace or an exclude directive (go install
//     pkg@version refuses a module with either);
//   - go.mod requires a module under the module's own owner path (a private
//     sibling) or a mirror's upstream path;
//   - a kit mirror still holds its own go.mod (a nested module, so its
//     packages are not this module's);
//   - a Go or templ file imports a kit by its upstream path.
func Check(root string) ([]string, error) {
	gomod := filepath.Join(root, "go.mod")
	raw, err := os.ReadFile(gomod)
	if err != nil {
		return nil, err
	}
	mf, err := modfile.Parse(gomod, raw, nil)
	if err != nil {
		return nil, err
	}
	if mf.Module == nil {
		return nil, fmt.Errorf("%s has no module line", gomod)
	}
	module := mf.Module.Mod.Path
	ms, err := repoMappings(root, module)
	if err != nil {
		return nil, err
	}
	var vs []string
	for _, r := range mf.Replace {
		vs = append(vs, fmt.Sprintf("go.mod: replace %s => %s: go install refuses a module with a replace directive", modString(r.Old.Path, r.Old.Version), modString(r.New.Path, r.New.Version)))
	}
	for _, e := range mf.Exclude {
		vs = append(vs, fmt.Sprintf("go.mod: exclude %s %s: go install refuses a module with an exclude directive", e.Mod.Path, e.Mod.Version))
	}
	owner := path.Dir(module) + "/"
	for _, r := range mf.Require {
		_, mirrored := rewritePath(r.Mod.Path, ms, Forward)
		if strings.HasPrefix(r.Mod.Path, owner) || mirrored {
			vs = append(vs, fmt.Sprintf("go.mod: requires %s %s: a private sibling the module proxy cannot serve (mirror it under third_party/, ADR-070)", r.Mod.Path, r.Mod.Version))
		}
	}
	for _, name := range MirrorNames {
		if _, err := os.Stat(filepath.Join(root, "third_party", name, "go.mod")); err == nil {
			vs = append(vs, fmt.Sprintf("third_party/%s/go.mod: a mirror with its own go.mod is a nested module, not packages of %s", name, module))
		}
	}
	scan := func(tree string, repo bool) error {
		return walkSources(tree, repo, func(p string) error {
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			spans, err := imports(p, src)
			if err != nil {
				return err
			}
			for _, s := range spans {
				if to, ok := rewritePath(s.path, ms, Forward); ok {
					rel, _ := filepath.Rel(root, p)
					vs = append(vs, fmt.Sprintf("%s:%d: imports %s by its upstream path (want %s)", filepath.ToSlash(rel), s.line, s.path, to))
				}
			}
			return nil
		})
	}
	if len(ms) > 0 {
		for _, name := range MirrorNames {
			dir := filepath.Join(root, "third_party", name)
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				if err := scan(dir, false); err != nil {
					return nil, err
				}
			}
		}
		if err := scan(root, true); err != nil {
			return nil, err
		}
	}
	sort.Strings(vs)
	return vs, nil
}

func modString(p, v string) string {
	if v == "" {
		return p
	}
	return p + " " + v
}
