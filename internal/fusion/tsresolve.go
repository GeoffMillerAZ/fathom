package fusion

// tsresolve.go — TypeScript's module resolution for relative-path's TS/JS
// rule (LB-F223..LB-F227, wave lh): what the compiler's moduleNameResolver
// does with a spec once a project's configuration is in view, over the repo's
// own files only. The compiler is the behaviour reference; nothing here is
// copied from it or from a bundler's resolver.
//
//   - The project (LB-F223): an importer takes the nearest tsconfig.json or
//     jsconfig.json whose files/include/exclude hold it, else the first of
//     that config's references that does, else the next one up (tsserver's
//     rule); `extends` chains (a relative path, an in-repo package
//     specifier, an array: later bases win, the config's own options last)
//     are merged option by option, each path option relative to the config
//     that declared it, `${configDir}` to the importer's config. With none,
//     the compiler's defaults apply (node10, no paths).
//   - compilerOptions (LB-F224): `paths` (an exact key, else the pattern
//     with the longest prefix before its `*`; its substitutions tried in
//     order), relative to baseUrl, else to the declaring config's directory;
//     `baseUrl` alone; `rootDirs` merged into one virtual directory for a
//     relative spec.
//   - Packages (LB-F225, LB-F226): a bare spec naming an in-repo workspace
//     package — npm/yarn `workspaces` (an array or {packages}),
//     pnpm-workspace.yaml `packages`, lerna.json `packages` — as a hoisted
//     node_modules link would, visible below the workspace root that
//     declares it; a package importing its own name through its `exports`.
//     Under node16, nodenext and bundler a package's `exports` (a string, a
//     subpath map, `*` patterns, conditions in the object's own key order,
//     matched against the compiler's set: types, import or require, node,
//     then customConditions; "default" always; a condition whose target is
//     missing falls through to the next) and `imports` (`#x`) are read; under
//     node10 (CommonJS, and with no config) typings, types, main, then the
//     index file.
//   - Outputs (LB-F227): a package target or `paths` target under a
//     tsconfig's outDir or declarationDir that is not in the repo names the
//     source it is built from — the same path under rootDir (else the
//     config's src/, else its directory) with the source extension — when
//     that source exists, as the workspace sees the package before a build.
//
// A bare spec that leaves the repository resolves to nothing. Every manifest
// is read through the env's reader and memoized for the env's life (one
// resolve); nothing is stored. Known deviations from the compiler: the
// classic resolution mode reads as node10, and `typesVersions` and versioned
// `types@` conditions are not read.

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/pathglob"
)

// Resolution modes, as the compiler names them (classic reads as node10).
const (
	tsModeNode10  = "node10"
	tsModeNode16  = "node16"
	tsModeBundler = "bundler"
)

// maxExtendsDepth bounds a tsconfig `extends` chain.
const maxExtendsDepth = 16

// tsEnv is one resolveEnv's TypeScript state, built on first use.
type tsEnv struct {
	raws    map[string]*tsRawConfig
	configs map[string]*tsConfig
	pkgs    map[string]*pkgManifest
	forFile map[string]*tsConfig
	scopes  map[string]scopeHit
	outSrc  map[string]string
	wsDone  bool
	ws      []tsWorkspace
}

type scopeHit struct {
	dir string
	ok  bool
}

func (e *resolveEnv) tsState() *tsEnv {
	if e.ts == nil {
		e.ts = &tsEnv{raws: map[string]*tsRawConfig{}, configs: map[string]*tsConfig{}, pkgs: map[string]*pkgManifest{},
			forFile: map[string]*tsConfig{}, scopes: map[string]scopeHit{}, outSrc: map[string]string{}}
	}
	return e.ts
}

// resolveTSModule is relative-path's TS/JS rule (resolveRelativePath with no
// suffixes, index files or relative): a relative spec by Node's rule with
// TypeScript's extension mapping (resolveNodeModule), then the importer's
// rootDirs; a bare or `#` spec by its project's paths, baseUrl, the
// package's own `imports` and name, then the workspace's packages.
func resolveTSModule(env *resolveEnv, importer, spec string) []string {
	if isRelativeSpec(spec) {
		if out := resolveNodeModule(env, importer, spec); out != nil {
			return out
		}
		if c := env.tsConfigFor(importer); c != nil && len(c.rootDirs) > 0 {
			if f, ok := env.tsRootDirs(c, importer, spec); ok {
				return []string{f}
			}
		}
		return nil
	}
	if strings.HasPrefix(spec, "/") || strings.HasPrefix(spec, "\\") {
		return nil
	}
	if f, ok := env.tsBare(importer, relDir(importer), spec); ok && f != importer {
		return []string{f}
	}
	return nil
}

func isRelativeSpec(spec string) bool {
	return spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")
}

// tsReq is what one resolution carries into a package: its mode, the
// condition set exports and imports are matched against, and whether the
// importer is TypeScript (tsExtensionMap first).
type tsReq struct {
	mode  string
	conds map[string]bool
	ts    bool
}

// tsBare resolves a non-relative spec for importer: its config's paths, then
// baseUrl, then (tsPackageSpec) a `#` import, the package's own name, a
// workspace package.
func (e *resolveEnv) tsBare(importer, from, spec string) (string, bool) {
	c := e.tsConfigFor(importer)
	req := &tsReq{mode: tsModeNode10, ts: isTSSource(importer)}
	if c != nil {
		req.mode = c.mode
		if f, ok := e.tsPaths(c, spec, req.ts); ok {
			return f, true
		}
		if c.hasBaseURL {
			if f, ok := e.tsLoad(joinRel(c.baseURL, spec), req.ts); ok {
				return f, true
			}
		}
	}
	req.conds = tsConditions(e, c, importer, req.mode)
	return e.tsPackageSpec(from, spec, req, 0)
}

// tsPackageSpec resolves a bare or `#` spec from directory from as a
// package lookup does: a `#` spec through the package scope's imports
// (node16, nodenext, bundler), a package's own name through its exports
// (the same modes), then the workspace package the name names.
func (e *resolveEnv) tsPackageSpec(from, spec string, req *tsReq, depth int) (string, bool) {
	if depth > 4 {
		return "", false
	}
	if strings.HasPrefix(spec, "#") {
		if req.mode == tsModeNode10 {
			return "", false
		}
		scope, ok := e.packageScope(from)
		if !ok {
			return "", false
		}
		pkg := e.packageAt(scope)
		if pkg == nil || pkg.imports == nil || pkg.imports.kind != 'o' {
			return "", false
		}
		return e.tsMatchMap(scope, pkg.imports, spec, req, true, depth)
	}
	name, sub, ok := splitPackageSpec(spec)
	if !ok {
		return "", false
	}
	if req.mode != tsModeNode10 {
		// A package importing its own name, through its exports.
		if scope, ok := e.packageScope(from); ok {
			if pkg := e.packageAt(scope); pkg != nil && pkg.name == name && pkg.exports != nil {
				if f, ok := e.tsExports(scope, pkg.exports, sub, req, depth); ok {
					return f, true
				}
			}
		}
	}
	dir := e.workspaceDir(name, from)
	if dir == "" {
		return "", false
	}
	return e.tsPackage(dir, sub, req, depth)
}

// tsLoad is cand as a file, then as a directory (its package.json typings,
// types or main, then its index), then as the source of an output.
func (e *resolveEnv) tsLoad(cand string, ts bool) (string, bool) {
	if cand == "" || cand == ".." || strings.HasPrefix(cand, "../") {
		return "", false
	}
	if f, ok := nodeFile(e, cand, ts); ok {
		return f, true
	}
	if f, ok := e.tsPackageDir(cand, ts); ok {
		return f, true
	}
	return e.tsOutDirSource(cand)
}

// tsPackage resolves sub ("." or "./x") of the package at dir: its exports
// (node16, nodenext, bundler), else its fields and index, or the file or
// directory sub names.
func (e *resolveEnv) tsPackage(dir, sub string, req *tsReq, depth int) (string, bool) {
	if pkg := e.packageAt(dir); req.mode != tsModeNode10 && pkg != nil && pkg.exports != nil {
		return e.tsExports(dir, pkg.exports, sub, req, depth)
	}
	if sub == "." {
		return e.tsPackageDir(dir, req.ts)
	}
	return e.tsLoad(joinRel(dir, strings.TrimPrefix(sub, "./")), req.ts)
}

// tsPackageDir is a directory's package.json fields (a TypeScript importer
// reads typings, types, then main; a JavaScript one main), each a file, a
// directory's index or an output's source, then the directory's index.
func (e *resolveEnv) tsPackageDir(dir string, ts bool) (string, bool) {
	if pkg := e.packageAt(dir); pkg != nil {
		fields := []string{pkg.main}
		if ts {
			fields = []string{pkg.typings, pkg.types, pkg.main}
		}
		for _, v := range fields {
			v = strings.TrimSpace(v)
			if v == "" || path.IsAbs(v) {
				continue
			}
			m := cleanRel(path.Join(dir, v))
			if m == "" || m == ".." || strings.HasPrefix(m, "../") || m == dir {
				continue
			}
			if f, ok := nodeFile(e, m, ts); ok {
				return f, true
			}
			if f, ok := nodeIndex(e, m); ok {
				return f, true
			}
			if f, ok := e.tsOutDirSource(m); ok {
				return f, true
			}
		}
	}
	return nodeIndex(e, dir)
}

// splitPackageSpec splits a bare spec into its package name and subpath
// ("." for the package itself, "./x" for a file or directory in it).
func splitPackageSpec(spec string) (name, sub string, ok bool) {
	segs := strings.Split(spec, "/")
	n := 1
	if strings.HasPrefix(spec, "@") {
		if len(segs) < 2 || segs[0] == "@" || segs[1] == "" {
			return "", "", false
		}
		n = 2
	}
	name = strings.Join(segs[:n], "/")
	if segs[0] == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "#") {
		return "", "", false
	}
	if rest := strings.Join(segs[n:], "/"); rest != "" {
		return name, "./" + rest, true
	}
	return name, ".", true
}

// tsResolveManifest reports a manifest the TS/JS rule reads (LB-F228): a
// tsconfig*.json or jsconfig*.json, a package.json, a pnpm-workspace.yaml or
// a lerna.json. An edit to one alone changes what a spec resolves to, so it
// is an input of the file-resolved pass (fileResolveInput); a base config
// named otherwise is read when the pass runs, never what triggers it.
func tsResolveManifest(p string) bool {
	b := path.Base(p)
	switch b {
	case "package.json", "pnpm-workspace.yaml", "lerna.json":
		return true
	}
	return strings.HasSuffix(b, ".json") && (strings.HasPrefix(b, "tsconfig") || strings.HasPrefix(b, "jsconfig"))
}

// ─── the project ────────────────────────────────────────────────────────────

// tsConfig is one tsconfig.json or jsconfig.json with its `extends` chain
// merged, every path repo-relative.
type tsConfig struct {
	path, dir  string
	mode       string
	baseURL    string
	hasBaseURL bool
	paths      []tsPathRule
	rootDirs   []string
	conditions []string
	outDirs    []string
	rootDir    string
	hasRootDir bool
	allowJS    bool
	files      map[string]bool
	include    []*regexp.Regexp
	exclude    []*regexp.Regexp
	references []string
}

// tsPathRule is one `paths` key: its prefix and suffix around the `*` (an
// exact key has none) and its substitutions, each repo-relative.
type tsPathRule struct {
	key            string
	star           bool
	prefix, suffix string
	subs           []string
}

// tsRawConfig is one config file as written.
type tsRawConfig struct {
	dir        string
	extends    []string
	options    map[string]json.RawMessage // compilerOptions, keys lower-cased
	paths      *jsonNode                  // compilerOptions.paths, in key order
	files      *[]string
	include    *[]string
	exclude    *[]string
	references []string
}

func (e *resolveEnv) tsRaw(p string) *tsRawConfig {
	st := e.tsState()
	if r, ok := st.raws[p]; ok {
		return r
	}
	st.raws[p] = nil
	b := e.manifest(p)
	if b == nil {
		return nil
	}
	b = stripJSONComments(b)
	var doc struct {
		Extends         json.RawMessage            `json:"extends"`
		CompilerOptions map[string]json.RawMessage `json:"compilerOptions"`
		Files           *[]string                  `json:"files"`
		Include         *[]string                  `json:"include"`
		Exclude         *[]string                  `json:"exclude"`
		References      []struct {
			Path string `json:"path"`
		} `json:"references"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return nil
	}
	r := &tsRawConfig{dir: relDir(p), options: map[string]json.RawMessage{}, files: doc.Files, include: doc.Include, exclude: doc.Exclude}
	var one string
	var many []string
	switch {
	case json.Unmarshal(doc.Extends, &one) == nil && one != "":
		r.extends = []string{one}
	case json.Unmarshal(doc.Extends, &many) == nil:
		r.extends = many
	}
	for k, v := range doc.CompilerOptions {
		r.options[strings.ToLower(k)] = v
	}
	if raw, ok := r.options["paths"]; ok {
		if n, ok := parseOrderedJSON(raw); ok && n.kind == 'o' {
			r.paths = n
		}
	}
	for _, ref := range doc.References {
		if ref.Path == "" || path.IsAbs(ref.Path) {
			continue
		}
		rp := joinRel(r.dir, ref.Path)
		if !strings.HasSuffix(rp, ".json") {
			rp = joinRel(rp, "tsconfig.json")
		}
		r.references = append(r.references, rp)
	}
	st.raws[p] = r
	return r
}

// tsOpt is one merged compiler option and the directory of the config that
// set it; tsList one merged files/include/exclude list.
type tsOpt struct {
	raw   json.RawMessage
	paths *jsonNode
	dir   string
}

type tsList struct {
	items []string
	dir   string
	set   bool
}

// tsLayers applies p's `extends` chain then p itself onto opts and the
// lists: bases first, in order, so a later base and then the config win.
func (e *resolveEnv) tsLayers(p string, depth int, seen map[string]bool, opts map[string]tsOpt, lists *[3]tsList) {
	r := e.tsRaw(p)
	if r == nil || seen[p] || depth > maxExtendsDepth {
		return
	}
	seen[p] = true
	for _, ext := range r.extends {
		if bp := e.tsExtendsPath(r.dir, ext); bp != "" {
			e.tsLayers(bp, depth+1, seen, opts, lists)
		}
	}
	for k, v := range r.options {
		o := tsOpt{raw: v, dir: r.dir}
		if k == "paths" {
			o.paths = r.paths
		}
		opts[k] = o
	}
	for i, l := range []*[]string{r.files, r.include, r.exclude} {
		if l != nil {
			lists[i] = tsList{items: *l, dir: r.dir, set: true}
		}
	}
}

// tsExtendsPath is the config an `extends` entry names: a relative path
// (".json" added when the file has none), or an in-repo package's config
// (its tsconfig.json, or the file its subpath names).
func (e *resolveEnv) tsExtendsPath(from, spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" || path.IsAbs(spec) {
		return ""
	}
	var cands []string
	if isRelativeSpec(spec) {
		c := joinRel(from, spec)
		cands = []string{c, c + ".json", joinRel(c, "tsconfig.json")}
	} else {
		name, sub, ok := splitPackageSpec(spec)
		if !ok {
			return ""
		}
		dir := e.workspaceDir(name, from)
		if dir == "" {
			return ""
		}
		if sub == "." {
			cands = []string{joinRel(dir, "tsconfig.json")}
		} else {
			c := joinRel(dir, strings.TrimPrefix(sub, "./"))
			cands = []string{c, c + ".json", joinRel(c, "tsconfig.json")}
		}
	}
	for _, c := range cands {
		if c != "" && !strings.HasPrefix(c, "../") && e.manifest(c) != nil {
			return c
		}
	}
	return ""
}

// tsConfigAt is the config file p, merged; nil when absent or unreadable.
func (e *resolveEnv) tsConfigAt(p string) *tsConfig {
	st := e.tsState()
	if c, ok := st.configs[p]; ok {
		return c
	}
	st.configs[p] = nil
	raw := e.tsRaw(p)
	if raw == nil {
		return nil
	}
	opts := map[string]tsOpt{}
	var lists [3]tsList
	e.tsLayers(p, 0, map[string]bool{}, opts, &lists)
	c := &tsConfig{path: p, dir: raw.dir, files: map[string]bool{}, references: raw.references}
	resolve := func(v, dir string) (string, bool) {
		v = strings.TrimSpace(v)
		if rest, ok := strings.CutPrefix(v, "${configDir}"); ok {
			return joinRel(c.dir, "./"+rest), true
		}
		if path.IsAbs(v) || filepath.IsAbs(v) {
			if e.root != "" {
				if rel, err := filepath.Rel(e.root, filepath.FromSlash(v)); err == nil && !strings.HasPrefix(filepath.ToSlash(rel), "../") {
					return cleanRel(filepath.ToSlash(rel)), true
				}
			}
			return "", false
		}
		return joinRel(dir, v), true
	}
	str := func(k string) (string, string, bool) {
		o, ok := opts[k]
		var s string
		if !ok || json.Unmarshal(o.raw, &s) != nil {
			return "", "", false
		}
		return s, o.dir, true
	}
	strs := func(k string) ([]string, string) {
		o, ok := opts[k]
		var ss []string
		if !ok || json.Unmarshal(o.raw, &ss) != nil {
			return nil, ""
		}
		return ss, o.dir
	}
	moduleRes, _, _ := str("moduleresolution")
	module, _, _ := str("module")
	c.mode = tsModeFor(strings.ToLower(moduleRes), strings.ToLower(module))
	if v, dir, ok := str("baseurl"); ok {
		c.baseURL, c.hasBaseURL = resolve(v, dir)
	}
	if o, ok := opts["paths"]; ok && o.paths != nil {
		base := c.baseURL
		if !c.hasBaseURL {
			base = o.dir
		}
		for i, k := range o.paths.keys {
			v := o.paths.vals[i]
			if v.kind != 'a' || strings.Count(k, "*") > 1 {
				continue
			}
			rule := tsPathRule{key: k}
			if j := strings.IndexByte(k, '*'); j >= 0 {
				rule.star, rule.prefix, rule.suffix = true, k[:j], k[j+1:]
			}
			for _, s := range v.vals {
				if s.kind != 's' {
					continue
				}
				sub := s.str
				if rest, ok := strings.CutPrefix(sub, "${configDir}"); ok {
					rule.subs = append(rule.subs, joinRel(c.dir, "./"+rest))
				} else if !path.IsAbs(sub) {
					rule.subs = append(rule.subs, joinRel(base, sub))
				}
			}
			c.paths = append(c.paths, rule)
		}
	}
	if ss, dir := strs("rootdirs"); len(ss) > 0 {
		for _, s := range ss {
			if r, ok := resolve(s, dir); ok {
				c.rootDirs = append(c.rootDirs, r)
			}
		}
	}
	c.conditions, _ = strs("customconditions")
	for _, k := range []string{"outdir", "declarationdir"} {
		if v, dir, ok := str(k); ok {
			if r, ok := resolve(v, dir); ok {
				c.outDirs = append(c.outDirs, r)
			}
		}
	}
	if v, dir, ok := str("rootdir"); ok {
		c.rootDir, c.hasRootDir = resolve(v, dir)
	}
	// allowJs defaults to checkJs, and a jsconfig.json allows JavaScript.
	var allow, check bool
	switch o, ok := opts["allowjs"]; {
	case ok && json.Unmarshal(o.raw, &allow) == nil:
		c.allowJS = allow
	case strings.HasPrefix(path.Base(p), "jsconfig"):
		c.allowJS = true
	default:
		if o, ok := opts["checkjs"]; ok && json.Unmarshal(o.raw, &check) == nil {
			c.allowJS = check
		}
	}
	files, include, exclude := lists[0], lists[1], lists[2]
	if files.set {
		for _, f := range files.items {
			if r, ok := resolve(f, files.dir); ok {
				c.files[r] = true
			}
		}
	}
	switch {
	case include.set:
		for _, s := range include.items {
			if r, ok := resolve(s, include.dir); ok {
				if re := tsSpecRegexp(r, false); re != nil {
					c.include = append(c.include, re)
				}
			}
		}
	case !files.set:
		c.include = []*regexp.Regexp{tsSpecRegexp(joinRel(c.dir, "**/*"), false)}
	}
	if exclude.set {
		for _, s := range exclude.items {
			if r, ok := resolve(s, exclude.dir); ok {
				if re := tsSpecRegexp(r, true); re != nil {
					c.exclude = append(c.exclude, re)
				}
			}
		}
	} else {
		for _, s := range []string{"node_modules", "bower_components", "jspm_packages"} {
			c.exclude = append(c.exclude, tsSpecRegexp(joinRel(c.dir, s), true))
		}
		for _, o := range c.outDirs {
			if o != c.dir && within(o, c.dir) { // an outDir of "." would exclude the project itself
				c.exclude = append(c.exclude, tsSpecRegexp(o, true))
			}
		}
	}
	st.configs[p] = c
	return c
}

// tsModeFor is the resolution mode moduleResolution names, else the one
// module implies (the compiler's defaults; classic reads as node10).
func tsModeFor(moduleResolution, module string) string {
	switch moduleResolution {
	case "node16", "nodenext":
		return tsModeNode16
	case "bundler":
		return tsModeBundler
	case "node", "node10", "classic":
		return tsModeNode10
	}
	switch module {
	case "node16", "node18", "node20", "nodenext":
		return tsModeNode16
	case "preserve":
		return tsModeBundler
	}
	return tsModeNode10
}

// includes reports whether file is in the config's project: listed in
// files, or matched by include (an extension the project compiles) and not
// by exclude.
func (c *tsConfig) includes(file string) bool {
	if c.files[file] {
		return true
	}
	switch scriptExt(file) {
	case ".ts", ".tsx", ".mts", ".cts", ".d.ts", ".d.mts", ".d.cts":
	case ".js", ".jsx", ".mjs", ".cjs":
		if !c.allowJS {
			return false
		}
	default:
		return false
	}
	in := false
	for _, re := range c.include {
		if re.MatchString(file) {
			in = true
			break
		}
	}
	if !in {
		return false
	}
	for _, re := range c.exclude {
		if re.MatchString(file) {
			return false
		}
	}
	return true
}

// tsSpecRegexp compiles one repo-relative include or exclude spec: `*` and
// `?` within a segment, `**/` any directories; an include whose last segment
// has no `.`, `*` or `?` names a directory (everything under it), and an
// exclude matches a path or anything below one it matches.
func tsSpecRegexp(spec string, exclude bool) *regexp.Regexp {
	var comps []string
	if spec != "" {
		comps = strings.Split(spec, "/")
	}
	if !exclude {
		switch last := ""; {
		case len(comps) > 0 && comps[len(comps)-1] == "**":
			comps = append(comps, "*")
		case len(comps) == 0:
			comps = []string{"**", "*"}
		default:
			last = comps[len(comps)-1]
			if !strings.ContainsAny(last, ".*?") {
				comps = append(comps, "**", "*")
			}
		}
	} else if len(comps) > 0 && comps[len(comps)-1] == "**" {
		comps = comps[:len(comps)-1]
	}
	var b strings.Builder
	b.WriteString("^")
	for i, c := range comps {
		if c == "**" {
			b.WriteString("(?:[^/]+/)*")
			continue
		}
		for _, r := range c {
			switch r {
			case '*':
				b.WriteString("[^/]*")
			case '?':
				b.WriteString("[^/]")
			default:
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
		}
		if i < len(comps)-1 {
			b.WriteString("/")
		}
	}
	if exclude {
		if len(comps) == 0 {
			b.WriteString(".*")
		} else {
			b.WriteString("(?:/.*)?")
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return re
}

// tsConfigFor is the config whose project holds importer (LB-F223): in
// each directory from the importer's up, tsconfig.json then jsconfig.json,
// if it includes the importer, else its first reference that does; nil
// when none does (the compiler's defaults).
func (e *resolveEnv) tsConfigFor(importer string) *tsConfig {
	st := e.tsState()
	if c, ok := st.forFile[importer]; ok {
		return c
	}
	var found *tsConfig
search:
	for _, d := range ancestors(relDir(importer)) {
		for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
			c := e.tsConfigAt(joinRel(d, name))
			if c == nil {
				continue
			}
			if c.includes(importer) {
				found = c
				break search
			}
			for _, r := range c.references {
				if rc := e.tsConfigAt(r); rc != nil && rc.includes(importer) {
					found = rc
					break search
				}
			}
		}
	}
	st.forFile[importer] = found
	return found
}

// tsPaths resolves spec through the config's `paths` (LB-F224): an exact
// key, else the pattern with the longest prefix; its substitutions in order.
func (e *resolveEnv) tsPaths(c *tsConfig, spec string, ts bool) (string, bool) {
	var rule *tsPathRule
	star := ""
	for i := range c.paths {
		if r := &c.paths[i]; !r.star && r.key == spec {
			rule = r
			break
		}
	}
	if rule == nil {
		best := -1
		for i := range c.paths {
			r := &c.paths[i]
			if !r.star || len(spec) < len(r.prefix)+len(r.suffix) || !strings.HasPrefix(spec, r.prefix) || !strings.HasSuffix(spec, r.suffix) {
				continue
			}
			if len(r.prefix) > best {
				best, rule, star = len(r.prefix), r, spec[len(r.prefix):len(spec)-len(r.suffix)]
			}
		}
	}
	if rule == nil {
		return "", false
	}
	for _, s := range rule.subs {
		if rule.star {
			s = cleanRel(strings.Replace(s, "*", star, 1))
		}
		if f, ok := e.tsLoad(s, ts); ok {
			return f, true
		}
	}
	return "", false
}

// tsRootDirs resolves a relative spec against the config's rootDirs, one
// virtual directory (LB-F224): the spec's path under the rootDir that holds
// it, tried under each other rootDir in order.
func (e *resolveEnv) tsRootDirs(c *tsConfig, importer, spec string) (string, bool) {
	cand := path.Clean(path.Join(relDir(importer), spec))
	if cand == ".." || strings.HasPrefix(cand, "../") {
		return "", false
	}
	cand = cleanRel(cand)
	matched, best := "", -1
	for _, r := range c.rootDirs {
		if within(cand, r) && len(r) > best {
			matched, best = r, len(r)
		}
	}
	if best < 0 {
		return "", false
	}
	suffix := strings.TrimPrefix(strings.TrimPrefix(cand, matched), "/")
	ts := isTSSource(importer)
	for _, r := range c.rootDirs {
		if r == matched {
			continue
		}
		if f, ok := e.tsLoad(joinRel(r, suffix), ts); ok {
			return f, true
		}
	}
	return "", false
}

// within reports whether p is dir or below it ("" is the repository root).
func within(p, dir string) bool {
	if dir == "" {
		return !strings.HasPrefix(p, "../") && p != ".."
	}
	return p == dir || strings.HasPrefix(p, dir+"/")
}

// tsConditions is the condition set a package's exports and imports are
// matched against: types, then import (an ES module importer, or bundler)
// or require, node (node16/nodenext), then customConditions; "default"
// matches always.
func tsConditions(e *resolveEnv, c *tsConfig, importer, mode string) map[string]bool {
	conds := map[string]bool{"types": true}
	switch mode {
	case tsModeBundler:
		conds["import"] = true
	case tsModeNode16:
		conds["node"] = true
		if e.isESM(importer) {
			conds["import"] = true
		} else {
			conds["require"] = true
		}
	default:
		conds["require"], conds["node"] = true, true
	}
	if c != nil {
		for _, k := range c.conditions {
			conds[k] = true
		}
	}
	return conds
}

// isESM reports an ES-module importer under node16/nodenext: .mts, .mjs (a
// .d.mts), else a .ts, .tsx, .js or .jsx whose package scope says
// "type": "module".
func (e *resolveEnv) isESM(importer string) bool {
	switch scriptExt(importer) {
	case ".mts", ".mjs", ".d.mts":
		return true
	case ".cts", ".cjs", ".d.cts":
		return false
	}
	if scope, ok := e.packageScope(relDir(importer)); ok {
		if pkg := e.packageAt(scope); pkg != nil {
			return pkg.typ == "module"
		}
	}
	return false
}

// ─── package.json ───────────────────────────────────────────────────────────

// pkgManifest is the fields of one package.json resolution reads.
type pkgManifest struct {
	name, typ            string
	main, types, typings string
	exports, imports     *jsonNode
	workspaces           []string
}

// packageAt is dir's package.json, nil when it has none.
func (e *resolveEnv) packageAt(dir string) *pkgManifest {
	st := e.tsState()
	if p, ok := st.pkgs[dir]; ok {
		return p
	}
	st.pkgs[dir] = nil
	b := e.manifest(joinRel(dir, "package.json"))
	if b == nil {
		return nil
	}
	doc, ok := parseOrderedJSON(b)
	if !ok || doc.kind != 'o' {
		return nil
	}
	p := &pkgManifest{
		name: doc.str2("name"), typ: doc.str2("type"),
		main: doc.str2("main"), types: doc.str2("types"), typings: doc.str2("typings"),
		exports: doc.get("exports"), imports: doc.get("imports"),
	}
	if w := doc.get("workspaces"); w != nil {
		if w.kind == 'o' {
			w = w.get("packages")
		}
		p.workspaces = w.strings()
	}
	st.pkgs[dir] = p
	return p
}

// packageScope is the nearest directory from dir up holding a package.json.
func (e *resolveEnv) packageScope(dir string) (string, bool) {
	st := e.tsState()
	if h, ok := st.scopes[dir]; ok {
		return h.dir, h.ok
	}
	h := scopeHit{}
	for _, d := range ancestors(dir) {
		if e.packageAt(d) != nil {
			h = scopeHit{d, true}
			break
		}
	}
	st.scopes[dir] = h
	return h.dir, h.ok
}

// tsExports resolves sub ("." or "./x") through a package's exports (LB-F226).
func (e *resolveEnv) tsExports(dir string, exp *jsonNode, sub string, req *tsReq, depth int) (string, bool) {
	if exp.kind == 'o' && len(exp.keys) > 0 && strings.HasPrefix(exp.keys[0], ".") {
		return e.tsMatchMap(dir, exp, sub, req, false, depth)
	}
	if sub != "." {
		return "", false
	}
	return e.tsTarget(dir, exp, "", req, false, 0, depth)
}

// tsMatchMap resolves key through an exports subpath map or an imports map:
// the exact key (one with no `*`), else the first pattern key in the
// compiler's order (the longest part before `*` first, then the longer key)
// that matches; a key ending in "/" maps a folder.
func (e *resolveEnv) tsMatchMap(dir string, m *jsonNode, key string, req *tsReq, isImports bool, depth int) (string, bool) {
	if !strings.HasSuffix(key, "/") && !strings.Contains(key, "*") {
		if v := m.get(key); v != nil {
			return e.tsTarget(dir, v, "", req, isImports, 0, depth)
		}
	}
	var expanding []string
	for _, k := range m.keys {
		if strings.Count(k, "*") == 1 || strings.HasSuffix(k, "/") {
			expanding = append(expanding, k)
		}
	}
	sort.SliceStable(expanding, func(i, j int) bool { return comparePatternKeys(expanding[i], expanding[j]) < 0 })
	for _, k := range expanding {
		if i := strings.IndexByte(k, '*'); i >= 0 {
			prefix, suffix := k[:i], k[i+1:]
			if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, suffix) && len(key) >= len(prefix)+len(suffix) {
				return e.tsTarget(dir, m.get(k), key[len(prefix):len(key)-len(suffix)], req, isImports, 1, depth)
			}
			continue
		}
		if strings.HasPrefix(key, k) {
			return e.tsTarget(dir, m.get(k), key[len(k):], req, isImports, 2, depth)
		}
	}
	return "", false
}

// comparePatternKeys orders exports/imports keys as the compiler does.
func comparePatternKeys(a, b string) int {
	ai, bi := strings.IndexByte(a, '*'), strings.IndexByte(b, '*')
	baseA, baseB := len(a), len(b)
	if ai >= 0 {
		baseA = ai + 1
	}
	if bi >= 0 {
		baseB = bi + 1
	}
	switch {
	case baseA > baseB:
		return -1
	case baseB > baseA:
		return 1
	case ai < 0:
		return 1
	case bi < 0:
		return -1
	case len(a) > len(b):
		return -1
	case len(b) > len(a):
		return 1
	}
	return 0
}

// tsTarget resolves one exports/imports target (kind 0: exact, 1: a `*`
// pattern whose match is star, 2: a folder whose remainder is star): a
// "./" path in the package (an imports target may also be a bare spec), an
// array (the first that resolves), a conditions object (its keys in order,
// a matched condition whose target is missing falling through), or null.
func (e *resolveEnv) tsTarget(dir string, t *jsonNode, star string, req *tsReq, isImports bool, kind, depth int) (string, bool) {
	if t == nil || depth > 16 {
		return "", false
	}
	switch t.kind {
	case 's':
		s := t.str
		if !strings.HasPrefix(s, "./") {
			if isImports && !strings.HasPrefix(s, "../") && !strings.HasPrefix(s, "/") {
				if kind == 1 {
					s = strings.ReplaceAll(s, "*", star)
				} else if kind == 2 {
					s += star
				}
				return e.tsPackageSpec(dir, s, req, depth+1)
			}
			return "", false
		}
		switch kind {
		case 1:
			s = strings.ReplaceAll(s, "*", star)
		case 2:
			if !strings.HasSuffix(s, "/") {
				return "", false
			}
			s += star
		}
		p := joinRel(dir, s)
		if !within(p, dir) {
			return "", false
		}
		for _, seg := range strings.Split(strings.TrimPrefix(p, dir), "/") {
			if seg == "node_modules" {
				return "", false
			}
		}
		if f, ok := nodeFile(e, p, req.ts); ok {
			return f, true
		}
		return e.tsOutDirSource(p)
	case 'a':
		for _, v := range t.vals {
			if f, ok := e.tsTarget(dir, v, star, req, isImports, kind, depth+1); ok {
				return f, true
			}
		}
	case 'o':
		for i, k := range t.keys {
			if k == "default" || req.conds[k] {
				if f, ok := e.tsTarget(dir, t.vals[i], star, req, isImports, kind, depth+1); ok {
					return f, true
				}
			}
		}
	}
	return "", false
}

// tsOutDirSource maps a missing output to the source it is built from
// (LB-F227): under a tsconfig.json's or tsconfig.build.json's outDir or
// declarationDir in one of p's directories, the same path under the
// config's rootDir (else its src/, else its directory) with each source
// extension of p's own (.js and .d.ts: .tsx, .ts, .jsx, .js; .mjs and
// .d.mts: .mts, .mjs; .cjs and .d.cts: .cts, .cjs), when that file exists.
func (e *resolveEnv) tsOutDirSource(p string) (string, bool) {
	if e.set[p] {
		return "", false // not missing: the caller already had it
	}
	st := e.tsState()
	if f, ok := st.outSrc[p]; ok {
		return f, f != ""
	}
	st.outSrc[p] = ""
	ext := ""
	for _, x := range []string{".d.ts", ".d.mts", ".d.cts", ".js", ".mjs", ".cjs"} {
		if strings.HasSuffix(p, x) {
			ext = x
			break
		}
	}
	var inputs []string
	switch ext {
	case ".d.mts", ".mjs":
		inputs = []string{".mts", ".mjs"}
	case ".d.cts", ".cjs":
		inputs = []string{".cts", ".cjs"}
	default:
		inputs = []string{".tsx", ".ts", ".jsx", ".js"}
	}
	stem := strings.TrimSuffix(p, ext)
	for _, d := range ancestors(relDir(p)) {
		for _, name := range []string{"tsconfig.json", "tsconfig.build.json"} {
			c := e.tsConfigAt(joinRel(d, name))
			if c == nil {
				continue
			}
			for _, out := range c.outDirs {
				if !within(stem, out) || strings.HasPrefix(out, "../") {
					continue
				}
				rel := strings.TrimPrefix(strings.TrimPrefix(stem, out), "/")
				roots := []string{joinRel(c.dir, "src"), c.dir}
				if c.hasRootDir {
					roots = []string{c.rootDir}
				}
				for _, r := range roots {
					for _, x := range inputs {
						if cand := joinRel(r, rel) + x; e.set[cand] && cand != p {
							st.outSrc[p] = cand
							return cand, true
						}
					}
				}
			}
		}
	}
	return "", false
}

// ─── workspaces ─────────────────────────────────────────────────────────────

// tsWorkspace is one workspace root and its member packages by name (a
// name two members share is left out: never a guess).
type tsWorkspace struct {
	root   string
	byName map[string]string
}

// workspaceDir is the directory of the workspace package called name that
// an importer in from sees (LB-F225): the nearest workspace root above from
// whose members include it, as a hoisted node_modules link resolves; ""
// when none does.
func (e *resolveEnv) workspaceDir(name, from string) string {
	for _, ws := range e.workspaces() {
		if !within(from, ws.root) {
			continue
		}
		if d, ok := ws.byName[name]; ok {
			return d
		}
	}
	return ""
}

// workspaces is every workspace root the repo declares — a package.json
// `workspaces`, a pnpm-workspace.yaml, a lerna.json — deepest first.
func (e *resolveEnv) workspaces() []tsWorkspace {
	st := e.tsState()
	if st.wsDone {
		return st.ws
	}
	st.wsDone = true
	globs := map[string][]string{}
	pkgDirs := map[string]bool{}
	for _, m := range e.manifestPaths("package.json") {
		d := relDir(m)
		pkgDirs[d] = true
		if p := e.packageAt(d); p != nil && len(p.workspaces) > 0 {
			globs[d] = append(globs[d], p.workspaces...)
		}
	}
	for _, m := range e.manifestsNamed("pnpm-workspace.yaml") {
		var doc struct {
			Packages []string `yaml:"packages"`
		}
		if b := e.manifest(m); b != nil && yaml.Unmarshal(b, &doc) == nil {
			globs[relDir(m)] = append(globs[relDir(m)], doc.Packages...)
		}
	}
	for _, m := range e.manifestsNamed("lerna.json") {
		var doc struct {
			Packages      *[]string `json:"packages"`
			UseWorkspaces bool      `json:"useWorkspaces"`
		}
		if b := e.manifest(m); b != nil && json.Unmarshal(stripJSONComments(b), &doc) == nil && !doc.UseWorkspaces {
			ps := []string{"packages/*"}
			if doc.Packages != nil {
				ps = *doc.Packages
			}
			globs[relDir(m)] = append(globs[relDir(m)], ps...)
		}
	}
	dirs := make([]string, 0, len(pkgDirs))
	for d := range pkgDirs {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for root, gs := range globs {
		var pos, neg []string
		for _, g := range gs {
			g = strings.TrimSpace(g)
			not := strings.HasPrefix(g, "!")
			g = strings.TrimSuffix(cleanRel(strings.TrimPrefix(g, "!")), "/")
			if g == "" {
				continue
			}
			if not {
				neg = append(neg, g)
			} else {
				pos = append(pos, g)
			}
		}
		named := map[string][]string{}
		for _, d := range dirs {
			if d == root || !within(d, root) {
				continue
			}
			rel := strings.TrimPrefix(strings.TrimPrefix(d, root), "/")
			if !pathglob.MatchAny(pos, rel) || pathglob.MatchAny(neg, rel) {
				continue
			}
			if p := e.packageAt(d); p != nil && p.name != "" {
				named[p.name] = append(named[p.name], d)
			}
		}
		ws := tsWorkspace{root: root, byName: map[string]string{}}
		for n, ds := range named {
			if len(ds) == 1 {
				ws.byName[n] = ds[0]
			}
		}
		st.ws = append(st.ws, ws)
	}
	depth := func(root string) int {
		if root == "" {
			return 0
		}
		return strings.Count(root, "/") + 1
	}
	sort.Slice(st.ws, func(i, j int) bool {
		if a, b := depth(st.ws[i].root), depth(st.ws[j].root); a != b {
			return a > b
		}
		return st.ws[i].root < st.ws[j].root
	})
	return st.ws
}

// manifestPaths is every repo path called name the env can see: those in
// the universe's directories and their ancestors (manifestsNamed), and every
// one the caller listed (an ingest's discovered files) or, for a repository
// on disk with no list, a walk of it that skips what discovery skips — a
// workspace package with no source file of the universe (a shared tsconfig
// package) is still a member.
func (e *resolveEnv) manifestPaths(name string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if !seen[p] && e.manifest(p) != nil {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range e.manifestsNamed(name) {
		add(p)
	}
	for _, p := range e.listedFiles() {
		if path.Base(p) == name {
			add(p)
		}
	}
	sort.Strings(out)
	return out
}

// maxListedFiles bounds the disk walk listedFiles makes.
const maxListedFiles = 200000

// listedFiles is the repository's files as the caller listed them, else
// (root on disk, no list) its walk, read once.
func (e *resolveEnv) listedFiles() []string {
	if e.listed != nil || e.listDone {
		return e.listed
	}
	e.listDone = true
	if e.root == "" || !e.walkRoot {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(e.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != e.root && connector.IsSkippedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= maxListedFiles {
			return filepath.SkipAll
		}
		if rel, rerr := filepath.Rel(e.root, p); rerr == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	e.listed = out
	return out
}

// ─── JSON ───────────────────────────────────────────────────────────────────

// jsonNode is a JSON value with its object keys in document order (exports
// and imports conditions are matched in the order they are written).
type jsonNode struct {
	kind byte // 's' string, 'o' object, 'a' array, 'n' null, 'v' any other scalar
	str  string
	keys []string
	vals []*jsonNode
}

// get is an object's value for key (the last, as JSON.parse keeps), nil if
// absent or not an object.
func (n *jsonNode) get(key string) *jsonNode {
	if n == nil || n.kind != 'o' {
		return nil
	}
	for i := len(n.keys) - 1; i >= 0; i-- {
		if n.keys[i] == key {
			return n.vals[i]
		}
	}
	return nil
}

// str2 is an object's string field, "" when absent or not a string.
func (n *jsonNode) str2(key string) string {
	if v := n.get(key); v != nil && v.kind == 's' {
		return v.str
	}
	return ""
}

// strings is an array's string elements.
func (n *jsonNode) strings() []string {
	if n == nil || n.kind != 'a' {
		return nil
	}
	var out []string
	for _, v := range n.vals {
		if v.kind == 's' {
			out = append(out, v.str)
		}
	}
	return out
}

// parseOrderedJSON parses JSON (comments and trailing commas allowed) into
// a jsonNode; ok is false for malformed input.
func parseOrderedJSON(b []byte) (*jsonNode, bool) {
	dec := json.NewDecoder(bytes.NewReader(stripJSONComments(b)))
	dec.UseNumber()
	n, err := decodeOrdered(dec, 0)
	if err != nil {
		return nil, false
	}
	return n, true
}

type jsonDepthError struct{}

func (jsonDepthError) Error() string { return "json: nested too deeply" }

func decodeOrdered(dec *json.Decoder, depth int) (*jsonNode, error) {
	if depth > 64 {
		return nil, jsonDepthError{}
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			n := &jsonNode{kind: 'o'}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, _ := kt.(string)
				v, err := decodeOrdered(dec, depth+1)
				if err != nil {
					return nil, err
				}
				n.keys, n.vals = append(n.keys, k), append(n.vals, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return n, nil
		case '[':
			n := &jsonNode{kind: 'a'}
			for dec.More() {
				v, err := decodeOrdered(dec, depth+1)
				if err != nil {
					return nil, err
				}
				n.vals = append(n.vals, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return n, nil
		}
	case string:
		return &jsonNode{kind: 's', str: t}, nil
	case nil:
		return &jsonNode{kind: 'n'}, nil
	}
	return &jsonNode{kind: 'v'}, nil
}

// stripJSONComments blanks JSONC's comments and trailing commas (a
// tsconfig.json is JSONC), keeping strings intact.
func stripJSONComments(src []byte) []byte {
	if !bytes.Contains(src, []byte("/")) && !bytes.Contains(src, []byte(",")) {
		return src
	}
	out := append([]byte(nil), src...)
	inStr := false
	for i := 0; i < len(out); i++ {
		c := out[i]
		switch {
		case inStr:
			if c == '\\' {
				i++
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
		case c == '/' && i+1 < len(out) && out[i+1] == '/':
			for ; i < len(out) && out[i] != '\n'; i++ {
				out[i] = ' '
			}
		case c == '/' && i+1 < len(out) && out[i+1] == '*':
			out[i], out[i+1] = ' ', ' '
			for i += 2; i < len(out); i++ {
				if out[i] == '*' && i+1 < len(out) && out[i+1] == '/' {
					out[i], out[i+1] = ' ', ' '
					i++
					break
				}
				if out[i] != '\n' {
					out[i] = ' '
				}
			}
		case c == ',':
			j := i + 1
			for j < len(out) {
				if out[j] == ' ' || out[j] == '\t' || out[j] == '\r' || out[j] == '\n' {
					j++
					continue
				}
				if out[j] == '/' && j+1 < len(out) && out[j+1] == '/' {
					for j < len(out) && out[j] != '\n' {
						j++
					}
					continue
				}
				if out[j] == '/' && j+1 < len(out) && out[j+1] == '*' {
					j += 2
					for j+1 < len(out) && !(out[j] == '*' && out[j+1] == '/') {
						j++
					}
					j += 2
					continue
				}
				break
			}
			if j < len(out) && (out[j] == '}' || out[j] == ']') {
				out[i] = ' '
			}
		}
	}
	return out
}

// diskListedRoot reports whether root is a directory on disk a resolve may
// walk for its manifests (a resolve over an ingest's connector lists its
// files instead).
func diskListedRoot(root string) bool {
	if root == "" {
		return false
	}
	fi, err := os.Stat(root)
	return err == nil && fi.IsDir()
}
