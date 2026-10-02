package fusion

// compdb.go — LB-Q-c-rest step 1: the compile databases an include-path
// strategy reads. A compile_commands.json names, per translation unit, the
// directories its compiler searches for an #include: -iquote (quoted only), -I,
// -isystem and -idirafter (quoted and angled), each spelled joined (-Idir) or
// separate (-I dir), relative to the entry's own directory. The search order
// is the compiler's: a quoted include looks beside the including file, then
// -iquote, then -I, -isystem, -idirafter; an angled one skips the file's own
// directory and -iquote.
//
// A unit's own roots come first; a file no entry compiles (a header) takes the
// union of every unit's, in first-seen order. Only a directory inside the
// repository names anything. The databases read are the conventional in-repo
// ones (the repo root, build/, builddir/, _build/, cmake-build-*/ and
// out/build/*/) wherever the connector has them or the disk does: a build
// directory is usually gitignored, so discovery never lists it.

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// maxCompDBBytes caps one compile database. A real one is one line per
// translation unit: curl's is 1 MiB, a large tree's tens of MiB.
const maxCompDBBytes = 64 << 20

// compEntry is one translation unit: its repo-relative file ("" when outside
// the repository) and its in-repo include roots in search order.
type compEntry struct {
	file  string
	quote []string // roots a quoted include searches after the file's directory
	angle []string // roots an angled include searches
}

// compDB is every database's entries and their unions.
type compDB struct {
	names      []string // the repo-relative databases read
	byFile     map[string][]int
	entries    []compEntry
	quoteUnion []string
	angleUnion []string
	compiled   map[string]bool // every file an entry compiles, repo-relative
}

// compDBDirs are the repo-relative directories a compile database is looked
// for, in order; a glob names a directory per match.
var compDBDirs = []string{"", "build", "builddir", "_build", "cmake-build-*", "out/build/*"}

// compDatabase reads the compile databases once.
func (e *resolveEnv) compDatabase() *compDB {
	if e.cdb != nil {
		return e.cdb
	}
	db := &compDB{byFile: map[string][]int{}, compiled: map[string]bool{}}
	e.cdb = db
	seenQ, seenA := map[string]bool{}, map[string]bool{}
	for _, dir := range e.compDBCandidates() {
		rel := joinRel(dir, "compile_commands.json")
		raw := e.readCompDB(rel)
		if raw == nil {
			continue
		}
		var entries []struct {
			Directory string   `json:"directory"`
			Command   string   `json:"command"`
			Arguments []string `json:"arguments"`
			File      string   `json:"file"`
		}
		if json.Unmarshal(raw, &entries) != nil {
			continue
		}
		db.names = append(db.names, rel)
		for _, en := range entries {
			args := en.Arguments
			if len(args) == 0 {
				args = splitCommand(en.Command)
			}
			ce := compEntry{}
			if en.File != "" {
				f := en.File
				if !filepath.IsAbs(f) {
					f = filepath.Join(en.Directory, f)
				}
				ce.file, _ = e.repoRel(f)
			}
			var quote, system, after []string
			for i := 0; i < len(args); i++ {
				flag, val := includeFlag(args, &i)
				if val == "" {
					continue
				}
				if !filepath.IsAbs(val) {
					val = filepath.Join(en.Directory, val)
				}
				r, ok := e.repoRel(val)
				if !ok {
					continue
				}
				switch flag {
				case "-iquote":
					quote = append(quote, r)
				case "-I":
					ce.angle = append(ce.angle, r)
				case "-isystem":
					system = append(system, r)
				case "-idirafter":
					after = append(after, r)
				}
			}
			ce.quote = dedupeKeep(append(append([]string(nil), quote...), append(append(append([]string(nil), ce.angle...), system...), after...)...))
			ce.angle = dedupeKeep(append(append(ce.angle, system...), after...))
			if ce.file != "" {
				db.byFile[ce.file] = append(db.byFile[ce.file], len(db.entries))
				db.compiled[ce.file] = true
			}
			db.entries = append(db.entries, ce)
			for _, r := range ce.quote {
				if !seenQ[r] {
					seenQ[r] = true
					db.quoteUnion = append(db.quoteUnion, r)
				}
			}
			for _, r := range ce.angle {
				if !seenA[r] {
					seenA[r] = true
					db.angleUnion = append(db.angleUnion, r)
				}
			}
		}
	}
	return db
}

// includeFlag reads the include-directory flag at args[*i] in either spelling
// and advances *i past a separate value; ("", "") for any other argument.
func includeFlag(args []string, i *int) (flag, val string) {
	a := strings.Trim(args[*i], `"'`)
	for _, f := range []string{"-iquote", "-isystem", "-idirafter", "-I"} {
		if a == f {
			if *i+1 < len(args) {
				*i++
				return f, strings.Trim(args[*i], `"'`)
			}
			return "", ""
		}
		if v, ok := strings.CutPrefix(a, f); ok && v != "" {
			if f != "-I" && strings.HasPrefix(v, "=") {
				continue
			}
			return f, v
		}
	}
	return "", ""
}

// splitCommand splits a compile command the way a shell does for the flags
// this reads: whitespace separates, a quote or a backslash protects.
func splitCommand(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	started := false
	flush := func() {
		if started {
			out = append(out, cur.String())
			cur.Reset()
			started = false
		}
	}
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else if c == '\\' && quote == '"' && i+1 < len(rs) {
				i++
				cur.WriteRune(rs[i])
			} else {
				cur.WriteRune(c)
			}
		case c == '"' || c == '\'':
			quote, started = c, true
		case c == '\\' && i+1 < len(rs):
			i++
			cur.WriteRune(rs[i])
			started = true
		case c == ' ' || c == '\t' || c == '\n':
			flush()
		default:
			cur.WriteRune(c)
			started = true
		}
	}
	flush()
	return out
}

func dedupeKeep(xs []string) []string {
	seen := make(map[string]bool, len(xs))
	out := xs[:0]
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// repoRel is abs as a clean repo-relative slash path, false when it lies
// outside the repository (or the repository root is unknown).
func (e *resolveEnv) repoRel(abs string) (string, bool) {
	if e.root == "" {
		return "", false
	}
	if !e.rootDone {
		e.rootDone = true
		if real, err := filepath.EvalSymlinks(e.root); err == nil && real != e.root {
			e.rootReal = real
		}
	}
	roots := []string{e.root}
	if e.rootReal != "" {
		roots = append(roots, e.rootReal)
	}
	for _, root := range roots {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		rel = cleanRel(filepath.ToSlash(rel))
		if rel == ".." || strings.HasPrefix(rel, "../") {
			continue
		}
		return rel, true
	}
	return "", false
}

// compDBCandidates are the repo-relative directories that hold a
// compile_commands.json this run may read: the fixed ones, then each match of
// the globbed ones found on disk, sorted.
func (e *resolveEnv) compDBCandidates() []string {
	var out []string
	for _, d := range compDBDirs {
		if !strings.Contains(d, "*") {
			out = append(out, d)
			continue
		}
		if e.root == "" {
			continue
		}
		parent, pat := path.Split(d)
		ents, err := os.ReadDir(filepath.Join(e.root, filepath.FromSlash(parent)))
		if err != nil {
			continue
		}
		var names []string
		for _, ent := range ents {
			if ok, _ := path.Match(pat, ent.Name()); ok && ent.IsDir() {
				names = append(names, ent.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			out = append(out, joinRel(parent, n))
		}
	}
	return out
}

// readCompDB reads one database: through the env's reader when it has it,
// else from the disk under the repository root (a build directory is never in
// a connector's file list). nil when absent, unreadable or over the cap.
func (e *resolveEnv) readCompDB(rel string) []byte {
	if b := e.manifest(rel); b != nil {
		return b
	}
	if e.root == "" {
		return nil
	}
	f, err := os.Open(filepath.Join(e.root, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxCompDBBytes+1))
	if err != nil || len(b) > maxCompDBBytes {
		return nil
	}
	return b
}

// includeRoots are the include directories importer's compile command names,
// in the compiler's search order, then the rest of the union: a quoted include
// searches -iquote then the -I family, an angled one the -I family only.
func (e *resolveEnv) includeRoots(importer string, angle bool) []string {
	key := importer
	if angle {
		key = "<" + importer
	}
	if out, ok := e.rootsMemo[key]; ok {
		return out
	}
	out := e.includeRootsOf(importer, angle)
	if e.rootsMemo == nil {
		e.rootsMemo = map[string][]string{}
	}
	e.rootsMemo[key] = out
	return out
}

func (e *resolveEnv) includeRootsOf(importer string, angle bool) []string {
	db := e.compDatabase()
	var out []string
	seen := map[string]bool{}
	add := func(rs []string) {
		for _, r := range rs {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	for _, i := range db.byFile[importer] {
		if angle {
			add(db.entries[i].angle)
		} else {
			add(db.entries[i].quote)
		}
	}
	if angle {
		add(db.angleUnion)
	} else {
		add(db.quoteUnion)
	}
	return out
}

// compiledFiles is every file a compile database compiles, nil when no
// database named one (the build is unknown, so nothing is filtered by it).
func (e *resolveEnv) compiledFiles() map[string]bool {
	if db := e.compDatabase(); len(db.compiled) > 0 {
		return db.compiled
	}
	return nil
}

// noteAncestorInclude records that importer's include of spec resolved only
// through an ancestor directory (resolveIncludePath's no-database fallback).
func (e *resolveEnv) noteAncestorInclude(importer, spec string) {
	if e.ancHits == nil {
		e.ancHits = map[string]bool{}
	}
	e.ancHits[importer+"\x00"+spec] = true
}

// ancestorIncludes is the number of distinct includes that resolved through
// an ancestor directory.
func (e *resolveEnv) ancestorIncludes() int { return len(e.ancHits) }
