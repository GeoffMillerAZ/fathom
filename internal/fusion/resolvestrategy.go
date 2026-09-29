// L3 import resolution strategies (LB-F11, docs/design/language-breadth.md
// §6). A dialect manifest selects one strategy from a closed set and gives
// its parameters (extractor.ImportParams); each strategy is a pure function
// over (importer path, module spec, repo file set) → target files. Repo
// manifests a strategy consults (composer.json, pubspec.yaml, Cargo.toml,
// Package.swift, compile_commands.json, package.json) are read through the
// env's reader, bounded and memoized. Nothing here is typed evidence: the
// edges the file resolver emits through these stay core.EdgeFileResolved,
// tier pre-scan.
//
// A new strategy is a foundation change (an Opus row), never a worker's.
package fusion

import (
	"encoding/json"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// maxManifestBytes caps one repo manifest a strategy reads.
const maxManifestBytes = 1 << 20

// resolveEnv is one resolution universe: the files a module may resolve to
// (one language's files) and a reader for the repo manifests the
// strategies consult. Lookups are memoized; an env is not shared between
// goroutines.
type resolveEnv struct {
	root  string // absolute repo root, for compile_commands.json's absolute paths
	paths []string
	set   map[string]bool
	read  func(repoRel string) ([]byte, bool)

	manifests map[string][]byte // repo-relative path -> content (nil: absent)
	named     map[string][]string
	byBase    map[string][]string // file basename -> files
	byDir     map[string][]string // directory -> files directly in it
	dirsBase  map[string][]string // directory basename -> directories
	incRoots  []string
	incDone   bool
	crates    map[string]string // crate name (underscored) -> crate dir
	swiftTgts map[string][]string
}

func newResolveEnv(root string, paths []string, read func(string) ([]byte, bool)) *resolveEnv {
	ps := append([]string(nil), paths...)
	sort.Strings(ps)
	set := make(map[string]bool, len(ps))
	for _, p := range ps {
		set[p] = true
	}
	if read == nil {
		read = func(string) ([]byte, bool) { return nil, false }
	}
	return &resolveEnv{root: root, paths: ps, set: set, read: read, manifests: map[string][]byte{}, named: map[string][]string{}}
}

// manifest reads one repo manifest, memoized; nil when absent or oversized.
func (e *resolveEnv) manifest(p string) []byte {
	if b, ok := e.manifests[p]; ok {
		return b
	}
	b, ok := e.read(p)
	if !ok || len(b) > maxManifestBytes {
		b = nil
	}
	e.manifests[p] = b
	return b
}

// manifestsNamed is every readable file called name in a directory that is
// an ancestor of (or holds) a universe file, sorted.
func (e *resolveEnv) manifestsNamed(name string) []string {
	if out, ok := e.named[name]; ok {
		return out
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range e.paths {
		for _, d := range ancestors(path.Dir(p)) {
			if seen[d] {
				break // every further ancestor was visited from this one
			}
			seen[d] = true
			if m := joinRel(d, name); e.manifest(m) != nil {
				out = append(out, m)
			}
		}
	}
	sort.Strings(out)
	e.named[name] = out
	return out
}

func (e *resolveEnv) indexes() {
	if e.byBase != nil {
		return
	}
	e.byBase, e.byDir, e.dirsBase = map[string][]string{}, map[string][]string{}, map[string][]string{}
	for _, p := range e.paths {
		e.byBase[path.Base(p)] = append(e.byBase[path.Base(p)], p)
		d := relDir(p)
		if _, ok := e.byDir[d]; !ok {
			e.dirsBase[path.Base(d)] = append(e.dirsBase[path.Base(d)], d)
		}
		e.byDir[d] = append(e.byDir[d], p)
	}
}

// ancestors is dir, its parent, … , "" (the repo root).
func ancestors(dir string) []string {
	dir = cleanRel(dir)
	out := []string{dir}
	for dir != "" {
		dir = cleanRel(path.Dir(dir))
		out = append(out, dir)
	}
	return out
}

func cleanRel(p string) string {
	p = path.Clean(p)
	if p == "." || p == "/" {
		return ""
	}
	return p
}

func relDir(p string) string { return cleanRel(path.Dir(p)) }

func joinRel(elem ...string) string { return cleanRel(path.Join(elem...)) }

// hasScheme reports a URL-like or scheme-qualified spec (https:, data:,
// mailto:, dart:) no strategy resolves in the corpus.
func hasScheme(spec string) bool {
	i := strings.IndexByte(spec, ':')
	if i <= 0 {
		return false
	}
	for _, r := range spec[:i] {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

// resolveByStrategy resolves one module spec from importer under strategy
// with params, returning the sorted target files in env (nil when nothing
// in the corpus matches, when the strategy is go-package — Go resolves
// natively — or when the strategy is outside the closed set).
func resolveByStrategy(strategy string, params extractor.ImportParams, env *resolveEnv, importer, module string) []string {
	module = strings.TrimSpace(module)
	if module == "" {
		return nil
	}
	var out []string
	switch strategy {
	case "relative-path":
		out = resolveRelativePath(params, env, importer, module)
	case "dotted-module":
		out = resolveDottedModule(params, env, importer, module)
	case "namespace-dir":
		out = resolveNamespaceDir(params, env, importer, module)
	case "include-path":
		out = resolveIncludePath(params, env, importer, module)
	case "crate-mod":
		out = resolveCrateMod(env, importer, module)
	case "swift-module":
		out = resolveSwiftModule(params, env, module)
	case "file-symbol":
		out = resolveFileSymbol(params, env, importer, module)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return dedupSorted(out)
}

func dedupSorted(xs []string) []string {
	out := xs[:0]
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

// firstFile is the first candidate, tried as itself, then with each suffix,
// then with each index file, that is in env.
func firstFile(env *resolveEnv, cands, suffixes, indexFiles []string) []string {
	for _, c := range cands {
		if c == "" || strings.HasPrefix(c, "../") || c == ".." {
			continue
		}
		if env.set[c] {
			return []string{c}
		}
		for _, s := range suffixes {
			if s != "" && env.set[c+s] {
				return []string{c + s}
			}
		}
		for _, idx := range indexFiles {
			if p := c + "/" + idx; env.set[p] {
				return []string{p}
			}
		}
	}
	return nil
}

// tsResolveExts is the extension search order for a resolved TypeScript/
// JavaScript relative import with no extension of its own (SU-11: "TS:
// .ts/.tsx/.js/index.*") — relative-path's default suffixes.
var tsResolveExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

// resolveRelativePath is relative-path: a ./ or ../ spec resolves against
// the importer's directory, tried as itself, with each suffix, then each
// index file. With params.Relative a bare spec is relative too and a
// leading / is the repo root (CSS @import, HTML src/href, Markdown links);
// a query or fragment is dropped. A manifest that sets none of suffixes,
// index_files and relative selects the TS/JS rule, resolveNodeModule.
func resolveRelativePath(p extractor.ImportParams, env *resolveEnv, importer, spec string) []string {
	if hasScheme(spec) || strings.HasPrefix(spec, "#") {
		return nil
	}
	if !p.Relative && len(p.Suffixes) == 0 && len(p.IndexFiles) == 0 {
		return resolveNodeModule(env, importer, spec)
	}
	suffixes, index := p.Suffixes, p.IndexFiles
	if len(suffixes) == 0 {
		suffixes = tsResolveExts
	}
	if len(index) == 0 {
		for _, s := range suffixes {
			index = append(index, "index"+s)
		}
	}
	var cand string
	switch {
	case strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../"):
		cand = path.Clean(path.Join(path.Dir(importer), spec))
	case !p.Relative:
		return nil // a bare specifier is a package import, outside the corpus
	default:
		if i := strings.IndexAny(spec, "?#"); i >= 0 {
			spec = spec[:i]
		}
		if spec == "" || strings.HasPrefix(spec, "//") {
			return nil
		}
		if strings.HasPrefix(spec, "/") {
			cand = path.Clean(strings.TrimLeft(spec, "/"))
		} else {
			cand = path.Clean(path.Join(path.Dir(importer), spec))
		}
	}
	return firstFile(env, []string{cand}, suffixes, index)
}

// nodeExts is the TS/JS rule's extension search order for a spec with no
// JS or TS extension of its own (TypeScript's .ts, .tsx, .d.ts, .js, .jsx,
// then the module-format extensions).
var nodeExts = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx", ".mjs", ".cjs"}

// tsExtensionMap is TypeScript's rule for a relative spec written with a
// JS or TS extension (moduleResolution node16, nodenext and bundler; the
// compiler's tryAddingExtensions): the extension is replaced by each of
// these in order, so the ESM spec `./x.js` names the source `x.ts` it is
// emitted from, `.mjs` names `.mts` and `.cjs` names `.cts`.
var tsExtensionMap = map[string][]string{
	".js": {".ts", ".tsx", ".d.ts", ".js", ".jsx"}, ".ts": {".ts", ".tsx", ".d.ts", ".js", ".jsx"}, ".d.ts": {".ts", ".tsx", ".d.ts", ".js", ".jsx"},
	".jsx": {".tsx", ".ts", ".d.ts", ".jsx", ".js"}, ".tsx": {".tsx", ".ts", ".d.ts", ".jsx", ".js"},
	".mjs": {".mts", ".d.mts", ".mjs"}, ".mts": {".mts", ".d.mts", ".mjs"}, ".d.mts": {".mts", ".d.mts", ".mjs"},
	".cjs": {".cts", ".d.cts", ".cjs"}, ".cts": {".cts", ".d.cts", ".cjs"}, ".d.cts": {".cts", ".d.cts", ".cjs"},
}

// scriptExt is p's JS or TS extension as TypeScript reads it (a
// declaration file's .d.ts, .d.mts or .d.cts whole), "" for any other.
func scriptExt(p string) string {
	for _, e := range []string{".d.ts", ".d.mts", ".d.cts"} {
		if strings.HasSuffix(p, e) {
			return e
		}
	}
	if e := path.Ext(p); tsExtensionMap[e] != nil {
		return e
	}
	return ""
}

// isTSSource reports a TypeScript importer, whose specs the compiler
// resolves (a JavaScript one is resolved by Node or a bundler).
func isTSSource(p string) bool {
	switch scriptExt(p) {
	case ".ts", ".tsx", ".mts", ".cts", ".d.ts", ".d.mts", ".d.cts":
		return true
	}
	return false
}

// resolveNodeModule is relative-path's TS/JS rule: Node's resolution of a
// relative spec (./, ../, . or ..; a bare one is a package, outside the
// corpus) with TypeScript's mapping of an emitted extension to its source.
// The spec is tried as a file, then as a directory; `.`, `..` and a spec
// ending in / (or /. or /..) name only a directory, the repository root
// among them. A file: a TypeScript importer maps a JS or TS extension first
// (tsExtensionMap: `./x.js` is x.ts when that exists, as the compiler reads
// it), a JavaScript importer takes the file as written first (Node runs it)
// and maps the extension only when it is absent (a bundler, ts-node); then
// nodeExts are appended. A directory: its package.json fields (a TypeScript
// importer reads types, typings, then main; Node reads main), each a file
// or a directory's index, then the directory's own index file.
func resolveNodeModule(env *resolveEnv, importer, spec string) []string {
	if spec != "." && spec != ".." && !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
		return nil
	}
	cand := path.Clean(path.Join(path.Dir(importer), spec))
	if cand == ".." || strings.HasPrefix(cand, "../") {
		return nil
	}
	cand = cleanRel(cand)
	ts := isTSSource(importer)
	dirOnly := spec == "." || spec == ".." || strings.HasSuffix(spec, "/") || strings.HasSuffix(spec, "/.") || strings.HasSuffix(spec, "/..")
	if !dirOnly && cand != "" {
		if f, ok := nodeFile(env, cand, ts); ok {
			return []string{f}
		}
	}
	if f, ok := nodeDir(env, cand, ts); ok {
		return []string{f}
	}
	return nil
}

// nodeFile is cand resolved as a file (resolveNodeModule's file rule).
func nodeFile(env *resolveEnv, cand string, ts bool) (string, bool) {
	ext := scriptExt(cand)
	mapped := func() (string, bool) {
		stem := strings.TrimSuffix(cand, ext)
		for _, s := range tsExtensionMap[ext] {
			if env.set[stem+s] {
				return stem + s, true
			}
		}
		return "", false
	}
	if ts && ext != "" {
		if f, ok := mapped(); ok {
			return f, true
		}
	}
	if env.set[cand] {
		return cand, true
	}
	if !ts && ext != "" {
		if f, ok := mapped(); ok {
			return f, true
		}
	}
	for _, s := range nodeExts {
		if env.set[cand+s] {
			return cand + s, true
		}
	}
	return "", false
}

// nodeDir is dir resolved as a directory (resolveNodeModule's directory
// rule); dir "" is the repository root.
func nodeDir(env *resolveEnv, dir string, ts bool) (string, bool) {
	fields := []string{"main"}
	if ts {
		fields = []string{"types", "typings", "main"}
	}
	if pkg := packageJSONFields(env.manifest(joinRel(dir, "package.json"))); pkg != nil {
		for _, field := range fields {
			v := strings.TrimSpace(pkg[field])
			if v == "" || path.IsAbs(v) {
				continue
			}
			m := cleanRel(path.Join(dir, v))
			if m == "" || m == ".." || strings.HasPrefix(m, "../") || m == dir {
				continue
			}
			if f, ok := nodeFile(env, m, ts); ok {
				return f, true
			}
			if f, ok := nodeIndex(env, m); ok {
				return f, true
			}
		}
	}
	return nodeIndex(env, dir)
}

// nodeIndex is dir's index file, index plus each of nodeExts.
func nodeIndex(env *resolveEnv, dir string) (string, bool) {
	for _, s := range nodeExts {
		if p := joinRel(dir, "index"+s); env.set[p] {
			return p, true
		}
	}
	return "", false
}

// packageJSONFields is a package.json's top-level string fields; nil when
// b is absent or not a JSON object.
func packageJSONFields(b []byte) map[string]string {
	if b == nil {
		return nil
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) != nil {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		var s string
		if json.Unmarshal(v, &s) == nil {
			out[k] = s
		}
	}
	return out
}

// resolveDottedModule is dotted-module: a dotted name maps to a path under
// each source root in order (default: the repo root); leading dots are
// relative to the importer's package, one parent per dot beyond the first.
// Defaults are the pre-LB-F11 Python rule (.py, then __init__.py).
func resolveDottedModule(p extractor.ImportParams, env *resolveEnv, importer, spec string) []string {
	sep := p.Separator
	if sep == "" {
		sep = "."
	}
	suffixes, index := p.Suffixes, p.IndexFiles
	if len(suffixes) == 0 {
		suffixes = []string{".py"}
	}
	if len(index) == 0 {
		index = []string{"__init__.py"}
	}
	toPath := func(dotted string) string {
		if dotted == "" {
			return ""
		}
		segs := strings.Split(dotted, sep)
		if p.Case == "snake" {
			for i, s := range segs {
				segs[i] = snakeCase(s)
			}
		}
		return strings.Join(segs, "/")
	}
	var cands []string
	if sep == "." && strings.HasPrefix(spec, ".") {
		dots := 0
		for dots < len(spec) && spec[dots] == '.' {
			dots++
		}
		dir := path.Dir(importer)
		for n := 1; n < dots; n++ {
			dir = path.Dir(dir)
		}
		// cleanRel("") would read as the root; a relative import at the
		// root with no name names no file (the pre-LB-F11 rule).
		c := path.Clean(path.Join(dir, toPath(spec[dots:])))
		if c == "." {
			return nil
		}
		cands = []string{c}
	} else {
		roots := p.SourceRoots
		if len(roots) == 0 {
			roots = []string{""}
		}
		rel := toPath(spec)
		for _, r := range roots {
			cands = append(cands, joinRel(r, rel))
		}
	}
	return firstFile(env, cands, suffixes, index)
}

// snakeCase maps a CamelCase segment to snake_case (MyApp -> my_app,
// HTTPServer -> http_server); an already-lowercase segment is unchanged.
func snakeCase(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1]) ||
				(i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1]))) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// resolveNamespaceDir is namespace-dir: a package/namespace-qualified name
// maps to a directory path. A class-qualified name resolves to its file
// (com.acme.Foo -> …/com/acme/Foo.java under any source root, or only the
// configured source_roots); failing that, a package name resolves to every
// file directly in its directory. PHP names resolve through composer.json's
// PSR-4 prefixes first; Dart's package:<name>/ through pubspec.yaml's name
// (lib/), and a Dart path spec relative to the importer.
func resolveNamespaceDir(p extractor.ImportParams, env *resolveEnv, importer, spec string) []string {
	suffixes := p.Suffixes
	if len(suffixes) == 0 {
		suffixes = []string{""}
	}
	if rest, ok := strings.CutPrefix(spec, "package:"); ok {
		name, sub, _ := strings.Cut(rest, "/")
		var out []string
		for _, m := range env.manifestsNamed("pubspec.yaml") {
			if pubspecName(env.manifest(m)) == name {
				if f := joinRel(relDir(m), "lib", sub); env.set[f] {
					out = append(out, f)
				}
			}
		}
		return out
	}
	if hasScheme(spec) {
		return nil
	}
	sep := p.Separator
	if sep == "" {
		sep = "."
	}
	if strings.Contains(spec, "/") || (sep != "/" && hasAnySuffix(spec, suffixes)) {
		return firstFile(env, []string{path.Clean(path.Join(path.Dir(importer), spec))}, nil, nil)
	}
	spec = strings.Trim(spec, sep)
	if spec == "" {
		return nil
	}
	if out := resolvePSR4(env, spec, sep, suffixes); len(out) > 0 {
		return out
	}
	rel := strings.Join(strings.Split(spec, sep), "/")
	env.indexes()
	var out []string
	if len(p.SourceRoots) > 0 {
		for _, r := range p.SourceRoots {
			base := joinRel(r, rel)
			for _, s := range suffixes {
				if env.set[base+s] {
					out = append(out, base+s)
				}
			}
		}
		if len(out) > 0 {
			return out
		}
		for _, r := range p.SourceRoots {
			out = append(out, filesWithSuffix(env.byDir[joinRel(r, rel)], suffixes)...)
		}
		return out
	}
	for _, s := range suffixes {
		base := path.Base(rel) + s
		for _, f := range env.byBase[base] {
			if f == rel+s || strings.HasSuffix(f, "/"+rel+s) {
				out = append(out, f)
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, d := range env.dirsBase[path.Base(rel)] {
		if d == rel || strings.HasSuffix(d, "/"+rel) {
			out = append(out, filesWithSuffix(env.byDir[d], suffixes)...)
		}
	}
	return out
}

func hasAnySuffix(s string, suffixes []string) bool {
	for _, x := range suffixes {
		if x != "" && strings.HasSuffix(s, x) {
			return true
		}
	}
	return false
}

func filesWithSuffix(files, suffixes []string) []string {
	var out []string
	for _, f := range files {
		for _, s := range suffixes {
			if strings.HasSuffix(f, s) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// pubspecName is a pubspec.yaml's top-level name.
func pubspecName(b []byte) string {
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "name:"); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// resolvePSR4 maps a namespace through every composer.json's autoload and
// autoload-dev psr-4 prefixes (the longest matching prefix per manifest).
func resolvePSR4(env *resolveEnv, spec, sep string, suffixes []string) []string {
	var out []string
	for _, m := range env.manifestsNamed("composer.json") {
		var c struct {
			Autoload struct {
				PSR4 map[string]json.RawMessage `json:"psr-4"`
			} `json:"autoload"`
			AutoloadDev struct {
				PSR4 map[string]json.RawMessage `json:"psr-4"`
			} `json:"autoload-dev"`
		}
		if json.Unmarshal(env.manifest(m), &c) != nil {
			continue
		}
		best, bestDirs := -1, []string(nil)
		for _, table := range []map[string]json.RawMessage{c.Autoload.PSR4, c.AutoloadDev.PSR4} {
			for prefix, raw := range table {
				pre := strings.Trim(prefix, sep)
				if pre != "" && spec != pre && !strings.HasPrefix(spec, pre+sep) {
					continue
				}
				if len(pre) <= best {
					continue
				}
				var dirs []string
				var one string
				if json.Unmarshal(raw, &one) == nil {
					dirs = []string{one}
				} else if json.Unmarshal(raw, &dirs) != nil {
					continue
				}
				best, bestDirs = len(pre), dirs
			}
		}
		if best < 0 {
			continue
		}
		rest := strings.TrimPrefix(strings.TrimPrefix(spec, spec[:best]), sep)
		rel := strings.Join(strings.Split(rest, sep), "/")
		for _, d := range bestDirs {
			base := joinRel(relDir(m), d, rel)
			for _, s := range suffixes {
				if env.set[base+s] {
					out = append(out, base+s)
				}
			}
		}
	}
	return out
}

// resolveIncludePath is include-path: a quoted include resolves relative to
// the including file; then (quoted or angled) under each configured
// source_roots entry and each -I/-iquote/-isystem/-idirafter root inside
// the repo from compile_commands.json (repo root, then build/); then under
// an include/ directory beside the file or any ancestor. The first hit is
// the target, as a compiler would take it.
func resolveIncludePath(p extractor.ImportParams, env *resolveEnv, importer, spec string) []string {
	angle := strings.HasPrefix(spec, "<")
	spec = strings.Trim(spec, `"<> `)
	if spec == "" || strings.HasPrefix(spec, "/") {
		return nil
	}
	var cands []string
	if !angle {
		cands = append(cands, path.Clean(path.Join(path.Dir(importer), spec)))
	}
	for _, r := range p.SourceRoots {
		cands = append(cands, joinRel(r, spec))
	}
	for _, r := range env.includeRoots() {
		cands = append(cands, joinRel(r, spec))
	}
	for _, a := range ancestors(path.Dir(importer)) {
		cands = append(cands, joinRel(a, "include", spec))
	}
	return firstFile(env, cands, nil, nil)
}

// includeRoots are the in-repo include directories compile_commands.json
// names, in first-seen order.
func (e *resolveEnv) includeRoots() []string {
	if e.incDone {
		return e.incRoots
	}
	e.incDone = true
	seen := map[string]bool{}
	for _, m := range []string{"compile_commands.json", "build/compile_commands.json"} {
		var entries []struct {
			Directory string   `json:"directory"`
			Command   string   `json:"command"`
			Arguments []string `json:"arguments"`
		}
		if json.Unmarshal(e.manifest(m), &entries) != nil {
			continue
		}
		for _, en := range entries {
			args := en.Arguments
			if len(args) == 0 {
				args = strings.Fields(en.Command)
			}
			for i := 0; i < len(args); i++ {
				a := strings.Trim(args[i], `"'`)
				var dir string
				for _, flag := range []string{"-I", "-iquote", "-isystem", "-idirafter"} {
					if a == flag && i+1 < len(args) {
						dir = strings.Trim(args[i+1], `"'`)
						i++
						break
					}
					if v, ok := strings.CutPrefix(a, flag); ok && v != "" && flag == "-I" {
						dir = v
						break
					}
				}
				if dir == "" {
					continue
				}
				if !filepath.IsAbs(dir) {
					dir = filepath.Join(en.Directory, dir)
				}
				if e.root == "" {
					continue
				}
				rel, err := filepath.Rel(e.root, dir)
				if err != nil {
					continue
				}
				rel = cleanRel(filepath.ToSlash(rel))
				if rel == ".." || strings.HasPrefix(rel, "../") || seen[rel] {
					continue
				}
				seen[rel] = true
				e.incRoots = append(e.incRoots, rel)
			}
		}
	}
	return e.incRoots
}

// resolveCrateMod is crate-mod: `mod x;` (a bare name) is x.rs or x/mod.rs
// in the declaring module's directory; a crate::/self::/super:: path, or
// one rooted at a workspace member's crate name (Cargo.toml [package]
// name, - read as _), resolves to the deepest module file the path names —
// its trailing segments may be items inside that module.
func resolveCrateMod(env *resolveEnv, importer, spec string) []string {
	if !strings.Contains(spec, "::") {
		return firstFile(env, []string{joinRel(rustModDir(importer), spec)}, []string{".rs"}, []string{"mod.rs"})
	}
	segs := strings.Split(strings.TrimPrefix(spec, "::"), "::")
	for i := range segs {
		segs[i] = strings.TrimSpace(segs[i])
	}
	var base string
	switch segs[0] {
	case "crate":
		base, segs = env.crateSrcDir(importer), segs[1:]
	case "self":
		base, segs = rustModDir(importer), segs[1:]
	case "super":
		base = rustModDir(importer)
		for len(segs) > 0 && segs[0] == "super" {
			base, segs = relDir(base), segs[1:]
		}
	default:
		dir, ok := env.crateByName()[segs[0]]
		if !ok {
			return nil // std, core, or an external crate
		}
		base, segs = rustSrcDir(dir), segs[1:]
	}
	if base == "\x00" {
		return nil
	}
	for i := len(segs); i >= 1; i-- {
		if f := firstFile(env, []string{joinRel(append([]string{base}, segs[:i]...)...)}, []string{".rs"}, []string{"mod.rs"}); f != nil {
			return f
		}
	}
	return rustModuleFile(env, base)
}

// rustModDir is the directory a Rust file's child modules live in.
func rustModDir(file string) string {
	switch path.Base(file) {
	case "lib.rs", "main.rs", "mod.rs":
		return relDir(file)
	}
	return joinRel(relDir(file), strings.TrimSuffix(path.Base(file), ".rs"))
}

// rustModuleFile is the file defining the module whose directory is dir.
func rustModuleFile(env *resolveEnv, dir string) []string {
	var cands []string
	if dir != "" {
		cands = append(cands, dir+".rs")
	}
	for _, f := range []string{"mod.rs", "lib.rs", "main.rs"} {
		cands = append(cands, joinRel(dir, f))
	}
	return firstFile(env, cands, nil, nil)
}

// crateSrcDir is the source directory of the crate holding file: the
// nearest ancestor with a Cargo.toml (its src/), else the nearest ancestor
// holding a lib.rs or main.rs; "\x00" when there is none.
func (e *resolveEnv) crateSrcDir(file string) string {
	for _, a := range ancestors(path.Dir(file)) {
		if e.manifest(joinRel(a, "Cargo.toml")) != nil {
			return rustSrcDir(a)
		}
	}
	for _, a := range ancestors(path.Dir(file)) {
		if e.set[joinRel(a, "lib.rs")] || e.set[joinRel(a, "main.rs")] {
			return a
		}
	}
	return "\x00"
}

func rustSrcDir(crateDir string) string { return joinRel(crateDir, "src") }

var cargoNameRe = regexp.MustCompile(`^\s*name\s*=\s*"([^"]+)"`)

// crateByName maps every Cargo.toml's [package] name (- read as _) to its
// crate directory.
func (e *resolveEnv) crateByName() map[string]string {
	if e.crates != nil {
		return e.crates
	}
	e.crates = map[string]string{}
	for _, m := range e.manifestsNamed("Cargo.toml") {
		section := ""
		for _, line := range strings.Split(string(e.manifest(m)), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "[") {
				section = t
				continue
			}
			if section != "[package]" {
				continue
			}
			if sm := cargoNameRe.FindStringSubmatch(t); sm != nil {
				name := strings.ReplaceAll(sm[1], "-", "_")
				if _, dup := e.crates[name]; !dup {
					e.crates[name] = relDir(m)
				}
				break
			}
		}
	}
	return e.crates
}

var (
	swiftTargetRe = regexp.MustCompile(`\.(?:target|executableTarget|testTarget|macro|plugin|systemLibrary)\s*\(\s*name:\s*"([^"]+)"`)
	swiftPathRe   = regexp.MustCompile(`\bpath:\s*"([^"]+)"`)
)

// resolveSwiftModule is swift-module: `import M` (or `import struct M.X`)
// resolves to every Swift file of the SwiftPM target named M — its path:
// in Package.swift, else Sources/M beside the manifest; without a manifest
// naming it, any Sources/M directory.
func resolveSwiftModule(p extractor.ImportParams, env *resolveEnv, spec string) []string {
	f := strings.Fields(spec)
	if len(f) == 0 {
		return nil
	}
	mod := f[len(f)-1]
	mod, _, _ = strings.Cut(mod, ".")
	suffixes := p.Suffixes
	if len(suffixes) == 0 {
		suffixes = []string{".swift"}
	}
	dirs := env.swiftTargets()[mod]
	if len(dirs) == 0 {
		env.indexes()
		for _, d := range env.dirsBase[mod] {
			if path.Base(relDir(d)) == "Sources" {
				dirs = append(dirs, d)
			}
		}
	}
	var out []string
	for _, d := range dirs {
		for _, file := range env.paths {
			if strings.HasPrefix(file, d+"/") && hasAnySuffix(file, suffixes) {
				out = append(out, file)
			}
		}
	}
	return out
}

func (e *resolveEnv) swiftTargets() map[string][]string {
	if e.swiftTgts != nil {
		return e.swiftTgts
	}
	e.swiftTgts = map[string][]string{}
	for _, m := range e.manifestsNamed("Package.swift") {
		src := string(e.manifest(m))
		locs := swiftTargetRe.FindAllStringSubmatchIndex(src, -1)
		for i, l := range locs {
			name := src[l[2]:l[3]]
			end := len(src)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			dir := joinRel(relDir(m), "Sources", name)
			if pm := swiftPathRe.FindStringSubmatch(src[l[1]:end]); pm != nil {
				dir = joinRel(relDir(m), pm[1])
			}
			e.swiftTgts[name] = append(e.swiftTgts[name], dir)
		}
	}
	return e.swiftTgts
}

// resolveFileSymbol is file-symbol: a path or separator-qualified module
// names one file — a ./ or ../ spec against the importer's directory; else
// (with params.Relative) the importer's directory first, then each source
// root — tried as itself, with each suffix, then each index file. Labels
// "bazel" reads //pkg:file (repo root) and :file (the importer's package);
// an @repo label is external.
func resolveFileSymbol(p extractor.ImportParams, env *resolveEnv, importer, spec string) []string {
	dir := path.Dir(importer)
	if p.Labels == "bazel" {
		switch {
		case strings.HasPrefix(spec, "@"):
			return nil
		case strings.HasPrefix(spec, "//"):
			return firstFile(env, []string{joinRel(strings.Replace(spec[2:], ":", "/", 1))}, p.Suffixes, p.IndexFiles)
		case strings.HasPrefix(spec, ":"):
			return firstFile(env, []string{joinRel(dir, spec[1:])}, p.Suffixes, p.IndexFiles)
		}
	}
	if strings.Contains(spec, "://") || strings.HasPrefix(spec, "/") {
		return nil
	}
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
		return firstFile(env, []string{path.Clean(path.Join(dir, spec))}, p.Suffixes, p.IndexFiles)
	}
	rel := spec
	if p.Separator != "" && p.Separator != "/" {
		rel = strings.Join(strings.Split(spec, p.Separator), "/")
	}
	var cands []string
	if p.Relative {
		cands = append(cands, path.Clean(path.Join(dir, rel)))
	}
	roots := p.SourceRoots
	if len(roots) == 0 {
		roots = []string{""}
	}
	for _, r := range roots {
		cands = append(cands, joinRel(r, rel))
	}
	return firstFile(env, cands, p.Suffixes, p.IndexFiles)
}
