// Command cue-oracle is the language-breadth harness's oracle for the CUE
// dialect (LB-Q-cue, design §1.3 and §18.3: the oracle is the cue binary,
// never fathom's loader). It writes indexbench lang-oracle's tool-oracle JSON
// (-tool-oracle, design §10), tier A:
//
//	go run . -subject <checkout> -cue <cue binary> -cache <CUE_CACHE_DIR outside the subject> [-files-out <file>] > cue-<subject>.json
//
// The tool is the cue binary (v0.16.1, the version fathom's go.mod pins),
// run as separate processes; this program never loads a CUE library:
//
//   - `cue vet -c=false <package>` per package instance (its import path, run
//     at its module root; a module-less file by its own path) is the gate: a
//     package the binary does not load contributes nothing, and -files-out
//     lists only the files of packages it loads. `cue def <package>` runs
//     beside it and its outcome is counted on stderr.
//   - `cue lsp serve`, the binary's language server, one session per CUE
//     module (rooted at the directory holding cue.mod), every accepted file
//     opened:
//     textDocument/documentSymbol gives each field and definition the binary
//     declares, with its range, named by the path of the field labels that
//     enclose it (the definitions);
//     textDocument/definition, asked at every identifier a lexical scan of
//     the file proposes (lex.go: the scan proposes positions, the server
//     decides what, if anything, each one refers to), gives each reference's
//     declarations (the calls and the cross-file references);
//     textDocument/definition at an import's path gives the package clauses
//     of the files the import loads (the imports, and the import entity's
//     cross-file references).
//
// What it writes:
//
//   - definitions: one per symbol the server lists, at its range; `name` is
//     the bare name (§1.4: the last segment of the label path, a definition's
//     leading '#' or '_#' dropped, as BareName compares it), `kind`
//     `definition` for a #-label, `field` otherwise, and `pattern` (a `[x]:`
//     constraint) or `dynamic` (an interpolated or parenthesized label) for
//     the declarations with no static name, which no dialect kind maps.
//   - calls: (file, referring field, referenced field or definition), each
//     named by its full label path (`#Deploy.svc`); the referring entity is
//     the innermost enclosing declaration with a static name, so a reference
//     in a pattern constraint's or a dynamic field's value belongs to the
//     field around it. A reference with no enclosing field (a file-level
//     embedding) has no caller and is counted on stderr.
//   - cross_references, entity granularity: a call whose declaration is in
//     another file of the subject (to its full label path), and an import to
//     each file of the subject the server resolves it to (from the import
//     entity, named by the import path as written, to the file's package
//     entity, named `package/<name>`).
//   - imports: (file, import line, file of the subject it loads, path).
//
// Standard-library imports (the server resolves them to no file) and
// imports of modules outside the subject (a registry dependency in the CUE
// cache) are counted on stderr, never written.
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end"`
	Kind string `json:"kind"`
}

type pair struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to,omitempty"`
}

type imp struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	ToPath string `json:"to_path"`
	Module string `json:"module,omitempty"`
}

type output struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	Calls           []pair `json:"calls"`
	CrossReferences []xref `json:"cross_references"`
	Imports         []imp  `json:"imports"`
}

// skipDirs are the directories fathom's discovery never reads
// (internal/connector), so the oracle does not spend the server's time on
// them.
var skipDirs = map[string]bool{
	"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true,
	".idea": true, ".vscode": true, ".claude": true,
}

// cueFile is one .cue file of the subject.
type cueFile struct {
	rel    string // subject-relative, slash
	abs    string
	src    []byte
	scan   scanResult
	module *module // nil: outside every module
}

type module struct {
	root string // absolute directory holding cue.mod
	rel  string // subject-relative ("" for the subject root)
	path string // module path, major version suffix dropped
}

type stats struct {
	files, accepted, packages, packagesRejected, defOK, defFailed      int
	refsAsked, refsResolved, refsNoCaller, refsNotEntity, refsExternal int
	importsStd, importsExternal, importsResolved, importsUnresolved    int
	symbolsOutside                                                     int
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	cueBin := flag.String("cue", "cue", "the cue binary (v0.16.1)")
	cache := flag.String("cache", "", "CUE_CACHE_DIR for registry modules (outside the subject; required when a module has registry dependencies)")
	filesOut := flag.String("files-out", "", "write the files of the packages the binary loads, one per line (for lang-oracle -only)")
	timeout := flag.Duration("timeout", 2*time.Minute, "per-request language-server timeout")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: cue-oracle -subject <checkout> -cue <cue> -cache <dir> [-files-out <file>]")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fail(err)
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	ctx := context.Background()
	env := cueEnv(*cache)
	version := cueVersion(ctx, *cueBin, env)
	out, st, accepted, err := run(ctx, root, *cueBin, env, *timeout)
	if err != nil {
		fail(err)
	}
	out.Tool, out.Version, out.Tier = "cue lsp (cue vet gate)", version, "A"
	fmt.Fprintf(os.Stderr, "cue-oracle: %d files, %d in %d of %d packages the binary loads (%d rejected; cue def ok %d, failed %d); %d definitions, %d calls, %d cross-references, %d imports\n",
		st.files, st.accepted, st.packages-st.packagesRejected, st.packages, st.packagesRejected, st.defOK, st.defFailed,
		len(out.Definitions), len(out.Calls), len(out.CrossReferences), len(out.Imports))
	fmt.Fprintf(os.Stderr, "cue-oracle: references: %d identifiers asked, %d resolved to a field or definition, %d with no enclosing field, %d to a non-entity (import alias, let, comprehension variable, package), %d to a declaration outside the subject\n",
		st.refsAsked, st.refsResolved, st.refsNoCaller, st.refsNotEntity, st.refsExternal)
	fmt.Fprintf(os.Stderr, "cue-oracle: imports: %d to files of the subject, %d standard library, %d outside the subject (registry or cache), %d resolved by the server to nothing\n",
		st.importsResolved, st.importsStd, st.importsExternal, st.importsUnresolved)
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(accepted, "\n")+"\n"), 0o644); err != nil {
			fail(err)
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "cue-oracle:", err)
	os.Exit(1)
}

// cueEnv is the environment for every cue run: the cache pinned, nothing of
// the operator's CUE configuration leaking in.
func cueEnv(cache string) []string {
	env := []string{}
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		switch k {
		case "CUE_CACHE_DIR", "CUE_EXPERIMENT", "CUE_REGISTRY", "CUE_CONFIG_DIR":
			continue
		}
		env = append(env, e)
	}
	if cache != "" {
		env = append(env, "CUE_CACHE_DIR="+cache)
	}
	return env
}

func cueVersion(ctx context.Context, bin string, env []string) string {
	b, err := runCue(ctx, bin, ".", env, "version")
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "cue version "); ok {
			return v
		}
	}
	return ""
}

func runCue(ctx context.Context, bin, dir string, env []string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("cue %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(tail(stderr.String(), 400)))
	}
	return stdout.Bytes(), nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// discover lists the subject's .cue files and the modules they belong to.
func discover(root string) ([]*cueFile, error) {
	var files []*cueFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".cue") || !d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, &cueFile{rel: filepath.ToSlash(rel), abs: p, src: src, scan: scan(src)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	mods := map[string]*module{}
	for _, f := range files {
		f.module = findModule(root, f.rel, mods)
	}
	return files, nil
}

// findModule is the module a file belongs to: the nearest directory at or
// above the file's own (inside the subject) holding cue.mod/module.cue; a
// file under a cue.mod directory belongs to the module that directory is in.
func findModule(root, rel string, mods map[string]*module) *module {
	dir := path.Dir(rel)
	if i := strings.Index("/"+dir+"/", "/cue.mod/"); i >= 0 {
		dir = strings.TrimSuffix(("/" + dir + "/")[1:i+1], "/")
		if dir == "" {
			dir = "."
		}
	}
	for {
		if m, ok := mods[dir]; ok {
			if m != nil {
				return m
			}
		} else {
			mf := filepath.Join(root, filepath.FromSlash(dir), "cue.mod", "module.cue")
			if b, err := os.ReadFile(mf); err == nil {
				r := ""
				if dir != "." {
					r = dir
				}
				m := &module{root: filepath.Join(root, filepath.FromSlash(dir)), rel: r, path: modulePath(b)}
				mods[dir] = m
				return m
			}
			mods[dir] = nil
		}
		if dir == "." || dir == "/" || dir == "" {
			return nil
		}
		dir = path.Dir(dir)
	}
}

// modulePath reads module.cue's `module: "path@vN"` lexically, the major
// version suffix dropped.
func modulePath(src []byte) string {
	toks := lex(src)
	for i := 0; i+2 < len(toks); i++ {
		if toks[i].Kind == tkIdent && toks[i].Text == "module" && toks[i+1].Kind == tkPunct && toks[i+1].Text == ":" && toks[i+2].Kind == tkString {
			p := toks[i+2].Value
			if at := strings.LastIndex(p, "@"); at >= 0 {
				p = p[:at]
			}
			return p
		}
	}
	return ""
}

// instance is the package argument `cue vet` loads a file through, and the
// directory it runs in.
func instance(f *cueFile, subject string) (arg, dir string) {
	pkg := f.scan.Package
	if f.module == nil || pkg == "" || pkg == "_" {
		return "./" + path.Base(f.rel), filepath.Join(subject, filepath.FromSlash(path.Dir(f.rel)))
	}
	modRel := f.rel
	if f.module.rel != "" {
		modRel = strings.TrimPrefix(f.rel, f.module.rel+"/")
	}
	d := path.Dir(modRel)
	var ip string
	switch {
	case strings.HasPrefix(d, "cue.mod/pkg/") || strings.HasPrefix(d, "cue.mod/gen/") || strings.HasPrefix(d, "cue.mod/usr/"):
		ip = d[len("cue.mod/pkg/"):]
	case d == ".":
		ip = f.module.path
	default:
		ip = f.module.path + "/" + d
	}
	if path.Base(ip) != pkg {
		ip += ":" + pkg
	}
	return ip, f.module.root
}

func run(ctx context.Context, root, cueBin string, env []string, timeout time.Duration) (output, stats, []string, error) {
	var out output
	var st stats
	files, err := discover(root)
	if err != nil {
		return out, st, nil, err
	}
	st.files = len(files)

	// the gate: one cue vet (and cue def) per package instance
	type gate struct{ ok bool }
	gates := map[string]*gate{}
	var accepted []*cueFile
	var moduleFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.rel, "cue.mod/module.cue") {
			// the module file is the module's configuration, no package, and
			// the language server lists no symbol for it: `cue export` is the
			// binary's word on its fields, the scan's on their lines
			// (moduleDefs)
			b, err := runCue(ctx, cueBin, filepath.Dir(filepath.Dir(f.abs)), env, "export", "--out", "json", "./cue.mod/module.cue")
			if err != nil {
				fmt.Fprintf(os.Stderr, "cue-oracle: rejected %s: %v\n", f.rel, err)
				continue
			}
			var v map[string]any
			if err := json.Unmarshal(b, &v); err != nil {
				continue
			}
			out.Definitions = append(out.Definitions, moduleDefs(f, v)...)
			moduleFiles = append(moduleFiles, f.rel)
			continue
		}
		arg, dir := instance(f, root)
		tool := strings.HasSuffix(f.rel, "_tool.cue")
		if tool {
			// cue vet and cue def leave a package's _tool.cue files out; the
			// binary loads them for `cue cmd`, and `cue help cmd`, run in the
			// package's directory, lists the commands they define only when
			// the package loads with them
			arg, dir = "(tool files)", filepath.Join(root, filepath.FromSlash(path.Dir(f.rel)))
		}
		key := dir + "\x00" + arg
		g, seen := gates[key]
		if !seen {
			g = &gate{}
			gates[key] = g
			st.packages++
			if tool {
				b, err := runCue(ctx, cueBin, dir, env, "help", "cmd")
				g.ok = err == nil && bytes.Contains(b, []byte("Available Commands:"))
			} else if _, err := runCue(ctx, cueBin, dir, env, "vet", "-c=false", arg); err == nil {
				g.ok = true
			} else {
				fmt.Fprintf(os.Stderr, "cue-oracle: rejected %s (in %s): %v\n", arg, dir, err)
			}
			if !g.ok {
				st.packagesRejected++
				if tool {
					fmt.Fprintf(os.Stderr, "cue-oracle: rejected the tool files in %s: cue help cmd lists no command\n", dir)
				}
			} else if !tool {
				if _, err := runCue(ctx, cueBin, dir, env, "def", arg); err == nil {
					st.defOK++
				} else {
					st.defFailed++
				}
			}
		}
		if g.ok {
			accepted = append(accepted, f)
		}
	}
	st.accepted = len(accepted)
	acceptedSet := map[string]bool{}
	acceptedRels := append([]string(nil), moduleFiles...)
	for _, f := range accepted {
		acceptedSet[f.rel] = true
		acceptedRels = append(acceptedRels, f.rel)
	}
	sort.Strings(acceptedRels)
	st.accepted += len(moduleFiles)

	// one language-server session per module (module-less files share one
	// rooted at the subject)
	byModule := map[*module][]*cueFile{}
	var order []*module
	for _, f := range accepted {
		if _, ok := byModule[f.module]; !ok {
			order = append(order, f.module)
		}
		byModule[f.module] = append(byModule[f.module], f)
	}
	sort.Slice(order, func(i, j int) bool { return modRoot(order[i], root) < modRoot(order[j], root) })
	byRel := map[string]*cueFile{}
	for _, f := range files {
		byRel[f.rel] = f
	}
	for _, m := range order {
		if err := measureModule(ctx, root, cueBin, env, timeout, m, byModule[m], byRel, acceptedSet, &out, &st); err != nil {
			return out, st, nil, fmt.Errorf("module %s: %w", modRoot(m, root), err)
		}
	}
	sortOutput(&out)
	return out, st, acceptedRels, nil
}

// moduleDefs are a module file's fields: each key path `cue export` gives,
// at the line the scan finds its label (a key the scan does not find is left
// out, and counted on stderr).
func moduleDefs(f *cueFile, exported map[string]any) []def {
	type at struct{ line, end int }
	found := map[string]at{}
	toks := lex(f.src)
	var stack []string // the label path each open brace belongs to
	var chain []string // labels written `a: b: c` since the last value
	depthPath := func() []string {
		if len(stack) == 0 {
			return nil
		}
		return strings.Split(stack[len(stack)-1], "\x00")
	}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		isLabel := (t.Kind == tkIdent || t.Kind == tkString) && i+1 < len(toks) && toks[i+1].Kind == tkPunct && toks[i+1].Text == ":"
		switch {
		case isLabel:
			name := t.Text
			if t.Kind == tkString {
				name = t.Value
			}
			chain = append(chain, name)
			p := append(append([]string(nil), depthPath()...), chain...)
			found[strings.Join(p, "\x00")] = at{t.Line, t.Line}
			i++ // the ':'
		case t.Kind == tkPunct && t.Text == "{":
			p := append(append([]string(nil), depthPath()...), chain...)
			stack = append(stack, strings.Join(p, "\x00"))
			chain = nil
		case t.Kind == tkPunct && t.Text == "}":
			if len(stack) > 0 {
				key := stack[len(stack)-1]
				// every label path the brace closes ends here
				for k, v := range found {
					if k == key {
						found[k] = at{v.line, t.Line}
					}
				}
				stack = stack[:len(stack)-1]
			}
			chain = nil
		default:
			chain = nil
		}
	}
	var out []def
	var walk func(v map[string]any, prefix []string)
	walk = func(v map[string]any, prefix []string) {
		for k, sub := range v {
			p := append(append([]string(nil), prefix...), k)
			if a, ok := found[strings.Join(p, "\x00")]; ok {
				out = append(out, def{Path: f.rel, Name: bareName(strings.Join(p, ".")), Line: a.line, End: a.end, Kind: "field"})
			} else {
				fmt.Fprintf(os.Stderr, "cue-oracle: %s: exported key %q has no label the scan finds\n", f.rel, strings.Join(p, "."))
			}
			if m, ok := sub.(map[string]any); ok {
				walk(m, p)
			}
		}
	}
	walk(exported, nil)
	return out
}

func modRoot(m *module, subject string) string {
	if m == nil {
		return subject
	}
	return m.root
}

// sym is one flattened document symbol.
type sym struct {
	full     string // label path, aliases dropped
	kind     string // definition, field, pattern, dynamic
	static   bool   // has a static name (not pattern or dynamic)
	rng      lspRange
	sel      lspPos
	selEnd   lspPos
	parent   *sym
	children []*sym
}

type fileSyms struct {
	all   []*sym
	bySel map[lspPos]*sym
	pkg   string
}

// at is the symbol a declaration position names: the one whose selection
// range starts there, else the innermost whose selection range holds it (an
// aliased field's selection range starts at its alias, `X=name`, and the
// server answers a reference with the label's own position).
func (fs *fileSyms) at(p lspPos) *sym {
	if s := fs.bySel[p]; s != nil {
		return s
	}
	var best *sym
	for _, s := range fs.all {
		sel := lspRange{Start: s.sel, End: s.selEnd}
		if !within(sel, p) {
			continue
		}
		if best == nil || within(lspRange{Start: best.sel, End: best.selEnd}, s.sel) {
			best = s
		}
	}
	return best
}

func measureModule(ctx context.Context, subject, cueBin string, env []string, timeout time.Duration, m *module, files []*cueFile, byRel map[string]*cueFile, accepted map[string]bool, out *output, st *stats) error {
	c, err := startLSP(ctx, cueBin, modRoot(m, subject), env, timeout)
	if err != nil {
		return err
	}
	defer c.close()
	opened := map[string]bool{}
	symCache := map[string]*fileSyms{}
	// symbolsOf opens (once) and lists the symbols of any file by absolute
	// path: an accepted file, a generated one under cue.mod, a registry
	// module's in the cache.
	symbolsOf := func(abs string) (*fileSyms, error) {
		if fsy, ok := symCache[abs]; ok {
			return fsy, nil
		}
		if !opened[abs] {
			b, err := os.ReadFile(abs)
			if err != nil {
				symCache[abs] = nil
				return nil, nil
			}
			if err := c.open(abs, string(b)); err != nil {
				return nil, err
			}
			opened[abs] = true
		}
		ds, err := c.symbols(abs)
		if err != nil {
			return nil, err
		}
		fsy := &fileSyms{bySel: map[lspPos]*sym{}}
		var walk func(ds []docSymbol, parent *sym)
		walk = func(ds []docSymbol, parent *sym) {
			for _, d := range ds {
				label, kind, static := normalizeLabel(d.Name)
				s := &sym{kind: kind, static: static, rng: d.Range, sel: d.SelectionRange.Start, selEnd: d.SelectionRange.End, parent: parent}
				s.full = label
				if parent != nil {
					s.full = parent.full + "." + label
				}
				fsy.all = append(fsy.all, s)
				fsy.bySel[s.sel] = s
				if parent != nil {
					parent.children = append(parent.children, s)
				}
				walk(d.Children, s)
			}
		}
		walk(ds, nil)
		if b, err := os.ReadFile(abs); err == nil {
			fsy.pkg = scan(b).Package
		}
		symCache[abs] = fsy
		return fsy, nil
	}
	for _, f := range files {
		if err := c.open(f.abs, string(f.src)); err != nil {
			return err
		}
		opened[f.abs] = true
	}
	relOf := func(uri string) (string, bool) {
		p := strings.TrimPrefix(uri, "file://")
		if r, err := filepath.Rel(subject, p); err == nil && !strings.HasPrefix(r, "..") {
			return filepath.ToSlash(r), true
		}
		return "", false
	}
	col := func(t token) int {
		if c.encoding == "utf-8" {
			return t.Col
		}
		return t.Col16
	}
	for _, f := range files {
		fsy, err := symbolsOf(f.abs)
		if err != nil {
			return err
		}
		if fsy == nil {
			continue
		}
		for _, s := range fsy.all {
			name := bareName(s.full)
			out.Definitions = append(out.Definitions, def{Path: f.rel, Name: name, Line: s.rng.Start.Line + 1, End: s.rng.End.Line + 1, Kind: s.kind})
		}
		// references
		for _, id := range f.scan.Idents {
			if id.Role != roleRef {
				continue
			}
			st.refsAsked++
			pos := lspPos{Line: id.Line - 1, Character: col(id.token)}
			locs, err := c.definition(f.abs, pos.Line, pos.Character)
			if err != nil {
				return fmt.Errorf("%s:%d: %w", f.rel, id.Line, err)
			}
			caller := enclosing(fsy, pos)
			for _, l := range locs {
				if strings.TrimPrefix(l.URI, "file://") == f.abs && l.Range.Start == pos {
					continue // the identifier itself
				}
				tAbs := strings.TrimPrefix(l.URI, "file://")
				tsy, err := symbolsOf(tAbs)
				if err != nil {
					return err
				}
				var target *sym
				if tsy != nil {
					target = tsy.at(l.Range.Start)
				}
				if target == nil || !target.static {
					st.refsNotEntity++
					continue
				}
				if caller == nil {
					st.refsNoCaller++
					continue
				}
				if caller == target {
					continue
				}
				st.refsResolved++
				out.Calls = append(out.Calls, pair{Path: f.rel, From: caller.full, To: target.full})
				tRel, inSubject := relOf(l.URI)
				if !inSubject {
					st.refsExternal++
					continue
				}
				if tRel != f.rel {
					out.CrossReferences = append(out.CrossReferences, xref{Path: f.rel, From: caller.full, ToPath: tRel, To: target.full})
				}
			}
		}
		// imports
		for _, im := range f.scan.Imports {
			// an unaliased import resolves at its path, an aliased one at its
			// alias (the server answers nothing at an aliased import's path)
			p := lspPos{Line: im.Line - 1, Character: im.Col16 + 1}
			if c.encoding == "utf-8" {
				p.Character = im.Col + 1
			}
			if im.Alias != "" {
				p = lspPos{Line: im.AliasAt.Line - 1, Character: col(im.AliasAt)}
			}
			locs, err := c.definition(f.abs, p.Line, p.Character)
			if err != nil {
				return fmt.Errorf("%s:%d: import %q: %w", f.rel, im.Line, im.Path, err)
			}
			if len(locs) == 0 {
				if !strings.Contains(strings.SplitN(im.Path, "/", 2)[0], ".") {
					st.importsStd++
				} else {
					st.importsUnresolved++
				}
				continue
			}
			any := false
			for _, l := range locs {
				tRel, inSubject := relOf(l.URI)
				if !inSubject {
					continue
				}
				any = true
				tf := byRel[tRel]
				pkg := ""
				if tf != nil {
					pkg = tf.scan.Package
				}
				out.Imports = append(out.Imports, imp{Path: f.rel, Line: im.Line, ToPath: tRel, Module: im.Path})
				if pkg != "" {
					out.CrossReferences = append(out.CrossReferences, xref{Path: f.rel, From: im.Path, ToPath: tRel, To: "package/" + pkg})
				}
			}
			if any {
				st.importsResolved++
			} else {
				st.importsExternal++
			}
		}
	}
	_ = accepted
	return nil
}

// enclosing is the innermost symbol with a static name whose range holds p.
func enclosing(fsy *fileSyms, p lspPos) *sym {
	var best *sym
	for _, s := range fsy.all {
		if !within(s.rng, p) {
			continue
		}
		if best == nil || within(best.rng, s.rng.Start) && within(best.rng, s.rng.End) {
			best = s
		}
	}
	for best != nil && !best.static {
		best = best.parent
	}
	return best
}

func within(r lspRange, p lspPos) bool {
	before := func(a, b lspPos) bool { return a.Line < b.Line || (a.Line == b.Line && a.Character <= b.Character) }
	return before(r.Start, p) && before(p, r.End)
}

// normalizeLabel is a document symbol's name as a label: an alias (`X=name`)
// names the field `name`; a quoted simple label is its content; a pattern
// constraint (`[string]`) and an interpolated or parenthesized label have no
// static name.
func normalizeLabel(name string) (label, kind string, static bool) {
	switch {
	case strings.HasPrefix(name, "["):
		return name, "pattern", false
	case strings.HasPrefix(name, "("), strings.Contains(name, `\(`):
		return name, "dynamic", false
	}
	if i := strings.Index(name, "="); i > 0 && !strings.HasPrefix(name, `"`) {
		name = name[i+1:]
	}
	if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
		name = name[1 : len(name)-1]
	}
	kind = "field"
	if strings.HasPrefix(name, "#") || strings.HasPrefix(name, "_#") {
		kind = "definition"
	}
	return name, kind, true
}

// bareName is §1.4's comparison form of a label path: its last segment, a
// definition's '#' dropped (the harness's BareName rule: the text after the
// last of . : # / \ space ).
func bareName(full string) string {
	i := strings.LastIndexAny(full, ".:#/\\ )")
	return full[i+1:]
}

func sortOutput(o *output) {
	sort.Slice(o.Definitions, func(i, j int) bool {
		a, b := o.Definitions[i], o.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	o.Calls = dedupPairs(o.Calls)
	o.CrossReferences = dedupXrefs(o.CrossReferences)
	sort.Slice(o.Imports, func(i, j int) bool {
		a, b := o.Imports[i], o.Imports[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.ToPath < b.ToPath
	})
	seen := map[imp]bool{}
	var imps []imp
	for _, x := range o.Imports {
		if !seen[x] {
			seen[x] = true
			imps = append(imps, x)
		}
	}
	o.Imports = imps
	if o.Definitions == nil {
		o.Definitions = []def{}
	}
	if o.Calls == nil {
		o.Calls = []pair{}
	}
	if o.CrossReferences == nil {
		o.CrossReferences = []xref{}
	}
	if o.Imports == nil {
		o.Imports = []imp{}
	}
}

func dedupPairs(ps []pair) []pair {
	sort.Slice(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	var out []pair
	for i, p := range ps {
		if i == 0 || p != ps[i-1] {
			out = append(out, p)
		}
	}
	return out
}

func dedupXrefs(xs []xref) []xref {
	sort.Slice(xs, func(i, j int) bool {
		a, b := xs[i], xs[j]
		for _, p := range [][2]string{{a.Path, b.Path}, {a.From, b.From}, {a.ToPath, b.ToPath}, {a.To, b.To}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
	var out []xref
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}
