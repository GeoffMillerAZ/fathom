// Package usedset computes the used closure of a dependency directory (a
// vendored or mirrored module such as third_party/app-kit) from a consumer
// module's product entry points (SV-6, ADR-060 decision 6, vision 41 §4.5).
//
// A unit is one of:
//
//   - a Go package of the directory's module reached by the import graph from
//     the consumer's main packages: the files go/build would compile for this
//     platform, never a test file, so the set equals `go list -deps` of the same
//     mains restricted to the module (asserted on a fixture and on this
//     repository). A package keeps every file in its directory, testdata
//     included: pruning files inside a Go package is unsafe (a type reference
//     is not a call edge) and refused;
//   - a file a reached package embeds (`//go:embed` patterns);
//   - a declared CSS root: an `@source` glob or `@import` in one of the
//     consumer's CSS entry files (a Tailwind source list) that lands inside
//     the directory;
//   - a TypeScript or JavaScript module reached by relative import edges from
//     the files above;
//   - the module's go.mod and go.sum, which the consumer's replace needs (a
//     mirror that is a plain directory of the consumer module, ADR-070, has
//     none: its import paths are the consumer's module path plus its
//     directory).
//
// Each unit carries its entry point and its first importer (breadth first
// from the sorted entries); the packages and modules nothing reaches are
// listed unreached. The package reads files only: it runs no go command,
// fetches nothing and never copies, prunes or vendors (SV-X4).
package usedset

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

// Unit kinds.
const (
	KindGoPackage  = "go-package"
	KindEmbed      = "embed"
	KindCSSSource  = "css-source"
	KindCSSImport  = "css-import"
	KindJSModule   = "js-module"
	KindModuleFile = "module-file"
)

// Options configures UsedUnits.
type Options struct {
	// Root is the consumer module's root (its go.mod).
	Root string
	// Dir is the dependency directory, absolute or relative to Root.
	Dir string
	// Mains are the product entry points, package directories relative to
	// Root; nil is every main package of Root's module outside Dir and
	// workspace/.
	Mains []string
	// CSS are the consumer's CSS entry files relative to Root; nil is every
	// .css file of Root's module outside Dir and workspace/ that declares an
	// @source or @import landing in Dir.
	CSS []string
	// Build is the build context files are matched against; nil is
	// build.Default (this platform, CGO_ENABLED and GOFLAGS tags as Go reads
	// them).
	Build *build.Context
}

// Unit is one used or unreached unit of the directory.
type Unit struct {
	Kind string `json:"kind"`
	// ID is the import path of a Go package, else the path relative to Root
	// (a glob for an @source).
	ID string `json:"id"`
	// Dir is the unit's directory relative to Root.
	Dir string `json:"dir"`
	// Entry is the product entry point that reaches the unit (a main
	// package's import path, or the CSS entry file).
	Entry string `json:"entry,omitempty"`
	// FirstImporter is the package, module or file through which the unit
	// was first reached.
	FirstImporter string `json:"first_importer,omitempty"`
	Files         int    `json:"files"`
	Bytes         int64  `json:"bytes"`
}

// Totals sizes the directory and its used part.
type Totals struct {
	Files     int   `json:"files"`
	Bytes     int64 `json:"bytes"`
	UsedFiles int   `json:"used_files"`
	UsedBytes int64 `json:"used_bytes"`
	// GoPackages counts the module's packages (nested modules excluded).
	GoPackages     int `json:"go_packages"`
	UsedGoPackages int `json:"used_go_packages"`
}

// Result is a directory's used closure.
type Result struct {
	Root      string   `json:"root"`
	Dir       string   `json:"dir"`
	Module    string   `json:"module"`
	Entries   []string `json:"entries"`
	CSS       []string `json:"css_entries,omitempty"`
	Used      []Unit   `json:"used"`
	Unreached []Unit   `json:"unreached"`
	Totals    Totals   `json:"totals"`
	// UsedFiles lists every file of the used units, relative to Root: what a
	// pruning script reads (it stays a script, SV-X4).
	UsedFiles []string `json:"used_files"`
}

// localModule is a module whose source is a directory on disk: the consumer,
// or a module its go.mod replaces with a local path.
type localModule struct {
	path string // module path
	dir  string // absolute directory
}

type walker struct {
	opt     Options
	root    string
	dir     string // absolute dependency directory
	dirRel  string
	ctx     *build.Context
	mods    []localModule // longest path first
	mod     string        // the dependency directory's module path
	pkgs    map[string]*pkgInfo
	nested  map[string]bool // absolute dirs holding a go.mod other than a local module's own
	fileSet *token.FileSet
}

type pkgInfo struct {
	importPath string
	dir        string
	isMain     bool
	imports    []string
	embeds     []string // patterns
	err        error
}

// UsedUnits computes the used closure of opt.Dir from opt.Root's entry points.
func UsedUnits(opt Options) (Result, error) {
	root, err := filepath.Abs(opt.Root)
	if err != nil {
		return Result{}, err
	}
	dir := opt.Dir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	dir = filepath.Clean(dir)
	dirRel, err := filepath.Rel(root, dir)
	if err != nil || strings.HasPrefix(dirRel, "..") {
		return Result{}, fmt.Errorf("usedset: %s is not inside %s", opt.Dir, root)
	}
	w := &walker{opt: opt, root: root, dir: dir, dirRel: filepath.ToSlash(dirRel), pkgs: map[string]*pkgInfo{}, nested: map[string]bool{}, fileSet: token.NewFileSet()}
	w.ctx = opt.Build
	if w.ctx == nil {
		c := build.Default
		w.ctx = &c
	}
	if err := w.loadModules(); err != nil {
		return Result{}, err
	}
	if w.mod == "" {
		return Result{}, fmt.Errorf("usedset: %s is neither a directory of the consumer module nor a module %s replaces with a local path", w.dirRel, filepath.Join(root, "go.mod"))
	}

	mains, err := w.entries()
	if err != nil {
		return Result{}, err
	}
	res := Result{Root: root, Dir: w.dirRel, Module: w.mod}
	for _, m := range mains {
		res.Entries = append(res.Entries, m.importPath)
	}

	used := map[string]Unit{} // kind+id → unit
	addUnit := func(u Unit) bool {
		k := u.Kind + "\x00" + u.ID
		if _, ok := used[k]; ok {
			return false
		}
		used[k] = u
		return true
	}

	// Go: breadth first from the sorted entries over the local modules.
	type qe struct {
		pkg          *pkgInfo
		entry, first string
	}
	seen := map[string]bool{}
	var queue []qe
	for _, m := range mains {
		seen[m.importPath] = true
		queue = append(queue, qe{pkg: m, entry: m.importPath})
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if w.inDir(cur.pkg.dir) && w.ownedBy(cur.pkg.importPath) {
			files, bytes := w.packageFiles(cur.pkg.dir)
			addUnit(Unit{Kind: KindGoPackage, ID: cur.pkg.importPath, Dir: w.rel(cur.pkg.dir), Entry: cur.entry, FirstImporter: cur.first, Files: len(files), Bytes: bytes})
			for _, f := range w.embedFiles(cur.pkg) {
				addUnit(Unit{Kind: KindEmbed, ID: w.rel(f), Dir: w.rel(filepath.Dir(f)), Entry: cur.entry, FirstImporter: cur.pkg.importPath, Files: 1, Bytes: size(f)})
			}
		}
		for _, imp := range cur.pkg.imports {
			if seen[imp] {
				continue
			}
			p := w.load(imp)
			if p == nil {
				continue
			}
			seen[imp] = true
			queue = append(queue, qe{pkg: p, entry: cur.entry, first: cur.pkg.importPath})
		}
	}

	// The module files the consumer's replace needs.
	for _, f := range []string{"go.mod", "go.sum"} {
		p := filepath.Join(w.dir, f)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			addUnit(Unit{Kind: KindModuleFile, ID: w.rel(p), Dir: w.dirRel, Entry: "go.mod", FirstImporter: "replace " + w.mod, Files: 1, Bytes: st.Size()})
		}
	}

	// CSS: declared roots in the consumer's CSS entries.
	cssEntries, err := w.cssEntries()
	if err != nil {
		return Result{}, err
	}
	res.CSS = cssEntries
	for _, css := range cssEntries {
		for _, d := range w.cssDeclarations(css) {
			abs := filepath.Clean(filepath.Join(filepath.Dir(filepath.Join(w.root, css)), filepath.FromSlash(d.target)))
			if !w.inDir(abs) {
				continue
			}
			matches, _ := filepath.Glob(abs)
			var n int
			var b int64
			for _, m := range matches {
				if st, err := os.Stat(m); err == nil && st.Mode().IsRegular() {
					n++
					b += st.Size()
				}
			}
			kind := KindCSSSource
			if d.directive == "import" {
				kind = KindCSSImport
			}
			addUnit(Unit{Kind: kind, ID: w.rel(abs), Dir: w.rel(filepath.Dir(abs)), Entry: css, FirstImporter: css, Files: n, Bytes: b})
		}
	}

	// TS/JS: relative import edges from every used file that is a module.
	jsAll := w.jsModules()
	var jsQueue []string
	jsFirst := map[string]Unit{}
	for _, u := range used {
		for _, f := range w.unitFiles(u) {
			if isJS(f) {
				jsQueue = append(jsQueue, f)
				jsFirst[f] = u
			}
		}
	}
	sort.Strings(jsQueue)
	jsSeen := map[string]bool{}
	for _, f := range jsQueue {
		jsSeen[f] = true
	}
	for len(jsQueue) > 0 {
		f := jsQueue[0]
		jsQueue = jsQueue[1:]
		from := jsFirst[f]
		for _, t := range jsImports(filepath.Join(w.root, filepath.FromSlash(f))) {
			abs := resolveJS(filepath.Dir(filepath.Join(w.root, filepath.FromSlash(f))), t)
			if abs == "" || !w.inDir(abs) {
				continue
			}
			rel := w.rel(abs)
			if jsSeen[rel] {
				continue
			}
			jsSeen[rel] = true
			entry := from.Entry
			addUnit(Unit{Kind: KindJSModule, ID: rel, Dir: w.rel(filepath.Dir(abs)), Entry: entry, FirstImporter: f, Files: 1, Bytes: size(abs)})
			jsFirst[rel] = Unit{Entry: entry}
			jsQueue = append(jsQueue, rel)
		}
	}

	for _, u := range used {
		res.Used = append(res.Used, u)
	}
	sortUnits(res.Used)

	// Unreached: the module's packages and the JS/TS modules no unit covers.
	usedFiles := map[string]bool{}
	for _, u := range res.Used {
		for _, f := range w.unitFiles(u) {
			usedFiles[f] = true
		}
	}
	allPkgs := w.modulePackages()
	res.Totals.GoPackages = len(allPkgs)
	for _, pd := range allPkgs {
		ip := w.importPathOf(pd)
		if _, ok := used[KindGoPackage+"\x00"+ip]; ok {
			res.Totals.UsedGoPackages++
			continue
		}
		files, bytes := w.packageFiles(pd)
		res.Unreached = append(res.Unreached, Unit{Kind: KindGoPackage, ID: ip, Dir: w.rel(pd), Files: len(files), Bytes: bytes})
	}
	for _, f := range jsAll {
		if usedFiles[f] {
			continue
		}
		abs := filepath.Join(w.root, filepath.FromSlash(f))
		res.Unreached = append(res.Unreached, Unit{Kind: KindJSModule, ID: f, Dir: path.Dir(f), Files: 1, Bytes: size(abs)})
	}
	sortUnits(res.Unreached)

	// Totals over every file of the directory, nested modules included (they
	// are on disk under it), and the used files.
	_ = filepath.WalkDir(w.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Type().IsRegular() {
			res.Totals.Files++
			res.Totals.Bytes += size(p)
		}
		return nil
	})
	for f := range usedFiles {
		res.UsedFiles = append(res.UsedFiles, f)
		res.Totals.UsedFiles++
		res.Totals.UsedBytes += size(filepath.Join(w.root, filepath.FromSlash(f)))
	}
	sort.Strings(res.UsedFiles)
	return res, nil
}

func sortUnits(us []Unit) {
	order := map[string]int{KindGoPackage: 0, KindEmbed: 1, KindCSSSource: 2, KindCSSImport: 3, KindJSModule: 4, KindModuleFile: 5}
	sort.SliceStable(us, func(i, j int) bool {
		if order[us[i].Kind] != order[us[j].Kind] {
			return order[us[i].Kind] < order[us[j].Kind]
		}
		return us[i].ID < us[j].ID
	})
}

// loadModules reads the consumer's go.mod: its module and every replace
// with a local directory. The dependency directory's module is the one whose
// directory it is.
func (w *walker) loadModules() error {
	gomod := filepath.Join(w.root, "go.mod")
	raw, err := os.ReadFile(gomod)
	if err != nil {
		return fmt.Errorf("usedset: %w", err)
	}
	mf, err := modfile.Parse(gomod, raw, nil)
	if err != nil {
		return fmt.Errorf("usedset: %s: %w", gomod, err)
	}
	if mf.Module == nil {
		return fmt.Errorf("usedset: %s has no module line", gomod)
	}
	w.mods = append(w.mods, localModule{path: mf.Module.Mod.Path, dir: w.root})
	for _, r := range mf.Replace {
		if r.New.Version != "" || !(strings.HasPrefix(r.New.Path, "./") || strings.HasPrefix(r.New.Path, "../") || filepath.IsAbs(r.New.Path)) {
			continue
		}
		d := r.New.Path
		if !filepath.IsAbs(d) {
			d = filepath.Join(w.root, filepath.FromSlash(d))
		}
		d = filepath.Clean(d)
		w.mods = append(w.mods, localModule{path: r.Old.Path, dir: d})
		if d == w.dir {
			w.mod = r.Old.Path
		}
	}
	sort.SliceStable(w.mods, func(i, j int) bool { return len(w.mods[i].path) > len(w.mods[j].path) })
	// A mirror that is a plain directory of the consumer module (ADR-070:
	// fathom's kit mirrors keep their go.mod as go.mod.upstream) owns the
	// import paths under the consumer's module path plus its directory. A
	// directory holding its own go.mod that nothing replaces stays refused.
	if w.mod == "" && w.dirRel != "." && !w.hasGoMod(w.dir) {
		w.mod = mf.Module.Mod.Path + "/" + w.dirRel
	}
	return nil
}

func (w *walker) isModuleDir(d string) bool {
	for _, m := range w.mods {
		if m.dir == d {
			return true
		}
	}
	return false
}

// resolve maps an import path to a package directory in a local module, or
// "" for the standard library and modules resolved elsewhere.
func (w *walker) resolve(imp string) string {
	for _, m := range w.mods {
		if imp != m.path && !strings.HasPrefix(imp, m.path+"/") {
			continue
		}
		d := filepath.Join(m.dir, filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(imp, m.path), "/")))
		// A package under a nested go.mod belongs to another module, which
		// the go command resolves from its own requirement, not from here.
		for p := d; p != m.dir && strings.HasPrefix(p, m.dir); p = filepath.Dir(p) {
			if w.hasGoMod(p) {
				return ""
			}
		}
		return d
	}
	return ""
}

func (w *walker) hasGoMod(d string) bool {
	if v, ok := w.nested[d]; ok {
		return v
	}
	_, err := os.Stat(filepath.Join(d, "go.mod"))
	v := err == nil
	w.nested[d] = v
	return v
}

// importPathOf names the package in directory d by its local module.
func (w *walker) importPathOf(d string) string {
	best := localModule{}
	for _, m := range w.mods {
		if (d == m.dir || strings.HasPrefix(d, m.dir+string(filepath.Separator))) && len(m.dir) > len(best.dir) {
			best = m
		}
	}
	if best.dir == "" {
		return ""
	}
	rel, _ := filepath.Rel(best.dir, d)
	if rel == "." {
		return best.path
	}
	return best.path + "/" + filepath.ToSlash(rel)
}

// load parses the package at import path imp, once.
func (w *walker) load(imp string) *pkgInfo {
	if p, ok := w.pkgs[imp]; ok {
		return p
	}
	d := w.resolve(imp)
	if d == "" {
		w.pkgs[imp] = nil
		return nil
	}
	p := w.parseDir(imp, d)
	w.pkgs[imp] = p
	return p
}

// parseDir reads the files go/build would compile in d for this platform,
// tests excluded: their imports, package name and embed patterns.
func (w *walker) parseDir(imp, d string) *pkgInfo {
	ents, err := os.ReadDir(d)
	if err != nil {
		return nil
	}
	p := &pkgInfo{importPath: imp, dir: d}
	imports := map[string]bool{}
	any := false
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if ok, err := w.ctx.MatchFile(d, name); err != nil || !ok {
			continue
		}
		f, err := parser.ParseFile(w.fileSet, filepath.Join(d, name), nil, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			continue
		}
		any = true
		if f.Name.Name == "main" {
			p.isMain = true
		}
		embeds := false
		for _, is := range f.Imports {
			v, err := strconv.Unquote(is.Path.Value)
			if err != nil || v == "C" {
				continue
			}
			imports[v] = true
			embeds = embeds || v == "embed"
		}
		if embeds {
			p.embeds = append(p.embeds, embedPatterns(filepath.Join(d, name))...)
		}
	}
	if !any {
		return nil
	}
	for v := range imports {
		p.imports = append(p.imports, v)
	}
	sort.Strings(p.imports)
	return p
}

// entries returns the product entry points: opt.Mains, or every main
// package of the consumer's own module outside Dir and workspace/.
func (w *walker) entries() ([]*pkgInfo, error) {
	var dirs []string
	if w.opt.Mains != nil {
		for _, m := range w.opt.Mains {
			d := m
			if !filepath.IsAbs(d) {
				d = filepath.Join(w.root, filepath.FromSlash(d))
			}
			dirs = append(dirs, filepath.Clean(d))
		}
	} else {
		_ = filepath.WalkDir(w.root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if p != w.root && (skipDir(d.Name()) || p == w.dir || p == filepath.Join(w.root, "workspace") || w.isModuleDir(p) || w.hasGoMod(p)) {
				return filepath.SkipDir
			}
			dirs = append(dirs, p)
			return nil
		})
	}
	var out []*pkgInfo
	for _, d := range dirs {
		ip := w.importPathOf(d)
		if ip == "" {
			continue
		}
		p := w.parseDir(ip, d)
		if p == nil || !p.isMain {
			if w.opt.Mains != nil {
				return nil, fmt.Errorf("usedset: entry %s is not a main package", w.rel(d))
			}
			continue
		}
		w.pkgs[ip] = p
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].importPath < out[j].importPath })
	if len(out) == 0 {
		return nil, errors.New("usedset: no main package to start from")
	}
	return out, nil
}

// skipDir is the go command's rule for ./...: testdata, vendor, and names
// beginning with a dot or an underscore.
func skipDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// modulePackages lists every package directory of the dependency's module:
// a directory with a non-test .go file, nested modules and the ./... skips
// excluded.
func (w *walker) modulePackages() []string {
	var out []string
	_ = filepath.WalkDir(w.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if p != w.dir && (skipDir(d.Name()) || w.hasGoMod(p)) {
			return filepath.SkipDir
		}
		ents, _ := os.ReadDir(p)
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				out = append(out, p)
				break
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// packageFiles lists the files a package directory owns: its own files and
// its testdata tree (never a subpackage).
func (w *walker) packageFiles(d string) ([]string, int64) {
	var files []string
	var total int64
	ents, _ := os.ReadDir(d)
	for _, e := range ents {
		p := filepath.Join(d, e.Name())
		if e.Type().IsRegular() {
			files = append(files, w.rel(p))
			total += size(p)
		}
		if e.IsDir() && e.Name() == "testdata" {
			_ = filepath.WalkDir(p, func(q string, de fs.DirEntry, err error) error {
				if err == nil && de.Type().IsRegular() {
					files = append(files, w.rel(q))
					total += size(q)
				}
				return nil
			})
		}
	}
	sort.Strings(files)
	return files, total
}

// unitFiles lists a unit's files relative to Root.
func (w *walker) unitFiles(u Unit) []string {
	switch u.Kind {
	case KindGoPackage:
		files, _ := w.packageFiles(filepath.Join(w.root, filepath.FromSlash(u.Dir)))
		return files
	case KindCSSSource, KindCSSImport:
		matches, _ := filepath.Glob(filepath.Join(w.root, filepath.FromSlash(u.ID)))
		var out []string
		for _, m := range matches {
			if st, err := os.Stat(m); err == nil && st.Mode().IsRegular() {
				out = append(out, w.rel(m))
			}
		}
		sort.Strings(out)
		return out
	default:
		return []string{u.ID}
	}
}

var embedLine = regexp.MustCompile(`^//go:embed\s+(.+)$`)

// embedPatterns reads a file's //go:embed patterns (quoted or bare).
func embedPatterns(file string) []string {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		m := embedLine.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil {
			continue
		}
		out = append(out, splitEmbedPatterns(m[1])...)
	}
	return out
}

func splitEmbedPatterns(s string) []string {
	var out []string
	s = strings.TrimSpace(s)
	for s != "" {
		if s[0] == '"' || s[0] == '`' {
			q := s[0]
			end := strings.IndexByte(s[1:], q)
			if end < 0 {
				break
			}
			v := s[1 : end+1]
			if q == '"' {
				if u, err := strconv.Unquote(s[:end+2]); err == nil {
					v = u
				}
			}
			out = append(out, v)
			s = strings.TrimSpace(s[end+2:])
			continue
		}
		i := strings.IndexAny(s, " \t")
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i])
		s = strings.TrimSpace(s[i:])
	}
	return out
}

// embedFiles resolves a package's embed patterns to files, as the go
// command does: a matched directory contributes its files recursively except
// those whose names begin with "." or "_" (unless the pattern says all:).
func (w *walker) embedFiles(p *pkgInfo) []string {
	seen := map[string]bool{}
	var out []string
	for _, pat := range p.embeds {
		all := strings.HasPrefix(pat, "all:")
		pat = strings.TrimPrefix(pat, "all:")
		matches, _ := filepath.Glob(filepath.Join(p.dir, filepath.FromSlash(pat)))
		for _, m := range matches {
			st, err := os.Stat(m)
			if err != nil {
				continue
			}
			if st.Mode().IsRegular() {
				if !seen[m] {
					seen[m] = true
					out = append(out, m)
				}
				continue
			}
			_ = filepath.WalkDir(m, func(q string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				name := d.Name()
				if q != m && !all && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if d.Type().IsRegular() && !seen[q] {
					seen[q] = true
					out = append(out, q)
				}
				return nil
			})
		}
	}
	sort.Strings(out)
	return out
}

type cssDecl struct {
	directive string // "source" | "import"
	target    string
}

var cssDeclRE = regexp.MustCompile(`^@(source|import)\s+(?:url\()?["']([^"']+)["']`)

// cssDeclarations reads a CSS file's @source and @import targets.
func (w *walker) cssDeclarations(rel string) []cssDecl {
	raw, err := os.ReadFile(filepath.Join(w.root, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	var out []cssDecl
	for _, ln := range strings.Split(string(raw), "\n") {
		m := cssDeclRE.FindStringSubmatch(strings.TrimSpace(ln))
		if m == nil {
			continue
		}
		out = append(out, cssDecl{directive: m[1], target: m[2]})
	}
	return out
}

// cssEntries returns opt.CSS, or every .css file of the consumer outside Dir
// and workspace/ with a declaration landing in Dir.
func (w *walker) cssEntries() ([]string, error) {
	if w.opt.CSS != nil {
		out := append([]string{}, w.opt.CSS...)
		sort.Strings(out)
		return out, nil
	}
	var out []string
	_ = filepath.WalkDir(w.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != w.root && (skipDir(d.Name()) || d.Name() == "node_modules" || p == w.dir || p == filepath.Join(w.root, "workspace") || w.isModuleDir(p)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".css") {
			return nil
		}
		rel := w.rel(p)
		for _, decl := range w.cssDeclarations(rel) {
			if w.inDir(filepath.Clean(filepath.Join(filepath.Dir(p), filepath.FromSlash(decl.target)))) {
				out = append(out, rel)
				break
			}
		}
		return nil
	})
	sort.Strings(out)
	return out, nil
}

var jsExts = []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}

func isJS(f string) bool {
	for _, e := range jsExts {
		if strings.HasSuffix(f, e) && !strings.HasSuffix(f, ".d.ts") {
			return true
		}
	}
	return false
}

// jsModules lists the TS/JS modules in Dir (node_modules and the ./...
// skips excluded), relative to Root.
func (w *walker) jsModules() []string {
	var out []string
	_ = filepath.WalkDir(w.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != w.dir && (skipDir(d.Name()) || d.Name() == "node_modules" || w.hasGoMod(p)) {
				return filepath.SkipDir
			}
			return nil
		}
		if isJS(d.Name()) {
			out = append(out, w.rel(p))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

var jsImportRE = regexp.MustCompile(`(?:\bimport\s*(?:[^'"()]*?\bfrom\s*)?|\bexport\s+[^'"]*?\bfrom\s*|\brequire\s*\(\s*|\bimport\s*\(\s*)["']([^"']+)["']`)

// jsImports lists a module's import specifiers.
func jsImports(file string) []string {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range jsImportRE.FindAllStringSubmatch(string(raw), -1) {
		out = append(out, m[1])
	}
	return out
}

// resolveJS resolves a relative specifier from dir: the path itself, with a
// module extension, or a directory's index; "" for a bare specifier.
func resolveJS(dir, spec string) string {
	if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
		return ""
	}
	base := filepath.Join(dir, filepath.FromSlash(spec))
	cands := []string{base}
	for _, e := range jsExts {
		cands = append(cands, base+e)
	}
	// A .js specifier written for a TS source (the NodeNext convention).
	if strings.HasSuffix(base, ".js") {
		stem := strings.TrimSuffix(base, ".js")
		cands = append(cands, stem+".ts", stem+".tsx")
	}
	for _, e := range jsExts {
		cands = append(cands, filepath.Join(base, "index"+e))
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && st.Mode().IsRegular() {
			return filepath.Clean(c)
		}
	}
	return ""
}

func (w *walker) inDir(abs string) bool {
	return abs == w.dir || strings.HasPrefix(abs, w.dir+string(filepath.Separator))
}

// ownedBy reports whether an import path belongs to the dependency's module.
func (w *walker) ownedBy(imp string) bool {
	return imp == w.mod || strings.HasPrefix(imp, w.mod+"/")
}

func (w *walker) rel(abs string) string {
	r, err := filepath.Rel(w.root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(r)
}

func size(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.Size()
}
