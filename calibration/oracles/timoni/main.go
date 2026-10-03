// timoni-oracle: the language-breadth harness's oracle for the timoni native
// dialect (design §1.3, LB-Q-timoni). Bench-only; never a product dependency.
// It writes `indexbench lang-oracle`'s tool-oracle JSON in two files, one per
// tier, which lang-oracle takes as repeated -tool-oracle flags:
//
//	-out        tier C: definitions and references, from this program's own walk
//	            of the CUE syntax tree (cuelang.org/go/cue/parser, the parser the
//	            dialect also reads with: the contract is re-implemented here, the
//	            parser library is shared), gated by the tool: a module enters only
//	            if `timoni mod vet` accepts it, an object template only if its kind
//	            is one `timoni mod vet` (default and --debug values) renders, a
//	            bundle only if `timoni bundle vet` accepts it;
//	-xref-out   tier A: cross_references, from the timoni CLI itself.
//
// What the tool decides (tier A), all on a private working copy of the subject,
// offline: no cluster, no registry pull, TIMONI_CACHING off, a cache dir inside
// the scratch directory.
//
//   - a module's object templates: for each top-level definition D that declares
//     a literal Kubernetes kind, the module is vetted again with D's label
//     renamed in the one file that declares it. CUE reports every reference
//     the rename breaks as `<path>: reference "#D" not found:` with the position
//     of each use, so the pair is (the using file, the top-level definition that
//     holds the use, D's file): the tool names who uses D and where;
//   - a bundle's modules: a bundle `timoni bundle vet --print-value` accepts is
//     evaluated by the tool (defaults applied, a companion <stem>_*.cue file
//     passed beside it); each instance whose module.url is `file://<path>` is
//     rebuilt alone with `timoni bundle build` in the bundle's directory, and the
//     instance named in the output resolved to <dir of the bundle>/<path>, the
//     module's timoni.cue. oci:// instances need a registry: they are counted
//     external in the summary and have no truth.
//
// What the walk decides (tier C): which entities exist and where (the contract
// of internal/extractor/dialects/timoni/PROVENANCE.md), and the references:
// every `#Def` a field's value names (CUE's unification, field type and list
// element type, and an import selector), the keys of a struct-literal `values:`
// to the module's #Config fields, and a template's `#config: #Config`.
//
//	timoni-oracle -subject <checkout> -scratch <dir outside it> -timoni <bin> \
//	    -out <defs.json> -xref-out <xref.json> [-files-out <txt>] [-summary-out <json>]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
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
}

type oracleFile struct {
	Tool        string  `json:"tool"`
	Version     string  `json:"version,omitempty"`
	Tier        string  `json:"tier"`
	Source      string  `json:"source,omitempty"`
	Licence     string  `json:"licence,omitempty"`
	Definitions []def   `json:"definitions,omitempty"`
	References  []pair  `json:"references,omitempty"`
	Xrefs       *[]xref `json:"cross_references,omitempty"`
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// tool runs the timoni binary offline in dir.
type tool struct {
	bin, cache string
}

func (t tool) run(dir string, args ...string) (string, error) {
	cmd := exec.Command(t.bin, append([]string{"--cache-dir", t.cache}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TIMONI_CACHING=false", "NO_COLOR=1", "HOME="+t.cache)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return ansi.ReplaceAllString(out.String(), ""), err
}

func main() {
	subject := flag.String("subject", "", "the subject checkout (read only)")
	scratch := flag.String("scratch", "", "a scratch directory outside the subject")
	bin := flag.String("timoni", "timoni", "the timoni binary")
	out := flag.String("out", "", "tier C definitions and references")
	xout := flag.String("xref-out", "", "tier A cross_references")
	filesOut := flag.String("files-out", "", "the measured files, one per line")
	sumOut := flag.String("summary-out", "", "the summary JSON")
	include := flag.String("include", "", "comma-separated subject directories to copy into the working copy (default: all but .git); a module's cue.mod/pkg links reach ../../../../schemas, so name it beside the measured directories")
	flag.Parse()
	if *subject == "" || *scratch == "" || *out == "" || *xout == "" {
		fmt.Fprintln(os.Stderr, "usage: timoni-oracle -subject <dir> -scratch <dir> -timoni <bin> -out <json> -xref-out <json> [-files-out f] [-summary-out f]")
		os.Exit(2)
	}
	if rel, err := filepath.Rel(*subject, *scratch); err == nil && !strings.HasPrefix(rel, "..") {
		fatal(fmt.Errorf("-scratch must lie outside the subject"))
	}
	if strings.Contains(*bin, "/") {
		if a, err := filepath.Abs(*bin); err == nil {
			*bin = a
		}
	}
	if a, err := filepath.Abs(*scratch); err == nil {
		*scratch = a
	}
	r, err := newRun(*subject, *scratch, *bin, *include)
	if err != nil {
		fatal(err)
	}
	r.discover()
	r.truth()
	r.ablate()
	r.bundles()
	r.write(*out, *xout, *filesOut, *sumOut)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "timoni-oracle:", err)
	os.Exit(1)
}

type module struct {
	dir      string // repo-relative
	vetOK    bool
	vetErr   string
	kinds    map[string]bool
	objects  []object
	instFile string // the file that declares #Instance
}

type object struct {
	name, kind, file string
	off, n           int // the definition label's byte offset and length
}

type run struct {
	t           tool
	work        string
	version     string
	mods        []*module
	files       map[string]bool // measured files
	defs        []def
	refs        []pair
	xrefs       []xref
	sum         summary
	bundleFiles []string
	runtime     []string
}

type summary struct {
	Modules         int               `json:"modules"`
	ModulesVetted   int               `json:"modules_vetted"`
	ModulesRejected map[string]string `json:"modules_rejected,omitempty"`
	ObjectDefs      int               `json:"object_defs"`
	ObjectsRendered int               `json:"object_defs_of_rendered_kinds"`
	Ablations       int               `json:"ablations"`
	AblationBroke   int               `json:"ablations_that_broke_the_build"`
	AblationOther   int               `json:"ablations_failing_without_a_reference_error,omitempty"`
	ObjectXrefs     int               `json:"module_object_pairs"`
	Bundles         int               `json:"bundles"`
	BundlesRejected map[string]string `json:"bundles_rejected,omitempty"`
	Instances       int               `json:"bundle_instances"`
	InstancesLocal  int               `json:"bundle_instances_local"`
	InstancesBuilt  int               `json:"bundle_instances_built_by_the_tool"`
	InstancesOCI    int               `json:"bundle_instances_oci_external"`
	InstancesOther  int               `json:"bundle_instances_unresolved_other,omitempty"`
	BundleXrefs     int               `json:"bundle_module_pairs"`
	Runtimes        int               `json:"runtimes"`
	Tool            string            `json:"tool"`
	Files           int               `json:"files"`
}

func newRun(subject, scratch, bin, include string) (*run, error) {
	work := filepath.Join(scratch, "work")
	if err := os.RemoveAll(work); err != nil {
		return nil, err
	}
	for _, d := range strings.Split(include, ",") {
		d = strings.TrimSpace(d)
		if err := copyTree(filepath.Join(subject, d), filepath.Join(work, d)); err != nil {
			return nil, err
		}
	}
	cache := filepath.Join(scratch, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return nil, err
	}
	r := &run{t: tool{bin: bin, cache: cache}, work: work, files: map[string]bool{}}
	r.sum.ModulesRejected = map[string]string{}
	r.sum.BundlesRejected = map[string]string{}
	v, err := r.t.run(work, "version")
	if err != nil {
		return nil, fmt.Errorf("%s version: %v: %s", bin, err, v)
	}
	var cl, cu string
	for _, l := range strings.Split(v, "\n") {
		if s, ok := strings.CutPrefix(l, "client:"); ok {
			cl = strings.TrimSpace(s)
		}
		if s, ok := strings.CutPrefix(l, "cue:"); ok {
			cu = strings.TrimSpace(s)
		}
	}
	r.version = "timoni " + cl + " (cue " + cu + ")"
	r.sum.Tool = r.version
	return r, nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		if d.Type()&fs.ModeSymlink != 0 {
			// a module's cue.mod/pkg/timoni.sh is a link into the subject's schemas/
			l, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(l, to)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, 0o644)
	})
}

func (r *run) abs(rel string) string { return filepath.Join(r.work, filepath.FromSlash(rel)) }

// cueFiles lists the .cue files under dir (repo-relative), never under cue.mod.
func (r *run) cueFiles(dir string) []string {
	var out []string
	_ = filepath.WalkDir(r.abs(dir), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == "cue.mod" || d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".cue") {
			rel, _ := filepath.Rel(r.work, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

var renderedRe = regexp.MustCompile(`(?m)^\S*\s*INF (\S+)/(\S+)/(\S+) valid resource$`)

// discover finds each module, vets it with the default and the debug values,
// and keeps the kinds the tool renders.
func (r *run) discover() {
	var dirs []string
	_ = filepath.WalkDir(r.work, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == "cue.mod" || d.Name() == ".git") {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == "timoni.cue" {
			if _, err := os.Stat(filepath.Join(filepath.Dir(p), "cue.mod", "module.cue")); err != nil {
				return nil // a schema or fixture beside no module: not a module
			}
			rel, _ := filepath.Rel(r.work, filepath.Dir(p))
			dirs = append(dirs, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(dirs)
	for _, d := range dirs {
		m := &module{dir: d, kinds: map[string]bool{}}
		r.sum.Modules++
		abs := r.abs(d)
		o, err := r.t.run(abs, "mod", "vet", ".")
		if err != nil {
			m.vetErr = firstErr(o)
			r.sum.ModulesRejected[d] = m.vetErr
			r.mods = append(r.mods, m)
			continue
		}
		for _, mm := range renderedRe.FindAllStringSubmatch(o, -1) {
			m.kinds[mm[1]] = true
		}
		if _, statErr := os.Stat(filepath.Join(abs, "debug_values.cue")); statErr == nil {
			if o2, err2 := r.t.run(abs, "mod", "vet", ".", "--debug"); err2 == nil {
				for _, mm := range renderedRe.FindAllStringSubmatch(o2, -1) {
					m.kinds[mm[1]] = true
				}
			}
		}
		m.vetOK = true
		r.sum.ModulesVetted++
		r.mods = append(r.mods, m)
	}
}

func firstErr(o string) string {
	for _, l := range strings.Split(o, "\n") {
		if strings.Contains(l, "ERR") {
			return strings.TrimSpace(l)
		}
	}
	return strings.TrimSpace(o)
}

// parse reads one repo-relative file; nil when the parser recovers nothing.
func (r *run) parse(rel string) (*ast.File, []byte) {
	b, err := os.ReadFile(r.abs(rel))
	if err != nil {
		return nil, nil
	}
	f, _ := parser.ParseFile(rel, b)
	return f, b
}

func labelOf(fd *ast.Field) string {
	n, _, err := ast.LabelName(fd.Label)
	if err != nil {
		return ""
	}
	return n
}

func str(e ast.Expr) (string, bool) {
	b, ok := e.(*ast.BasicLit)
	if !ok || b.Kind != token.STRING {
		return "", false
	}
	if t := strings.TrimLeft(b.Value, "#"); len(t) == 0 || (t[0] != '"' && t[0] != '\'') {
		return "", false
	}
	s, err := literal.Unquote(b.Value)
	return s, err == nil
}

func structs(e ast.Expr, acc *[]*ast.StructLit) {
	switch v := e.(type) {
	case *ast.StructLit:
		*acc = append(*acc, v)
	case *ast.BinaryExpr:
		if v.Op == token.AND {
			structs(v.X, acc)
			structs(v.Y, acc)
		}
	case *ast.ParenExpr:
		structs(v.X, acc)
	}
}

func literalOf(e ast.Expr, field string) string {
	var ss []*ast.StructLit
	structs(e, &ss)
	for _, s := range ss {
		for _, el := range s.Elts {
			if fd, ok := el.(*ast.Field); ok && labelOf(fd) == field {
				if v, ok := str(fd.Value); ok {
					return v
				}
			}
		}
	}
	return ""
}

func takesConfig(e ast.Expr) bool {
	var ss []*ast.StructLit
	structs(e, &ss)
	for _, s := range ss {
		for _, el := range s.Elts {
			if fd, ok := el.(*ast.Field); ok && labelOf(fd) == "#config" {
				if id, ok := fd.Value.(*ast.Ident); ok && id.Name == "#Config" {
					return true
				}
			}
		}
	}
	return false
}

func importIdents(f *ast.File) map[string]string {
	m := map[string]string{}
	for imp := range f.ImportSpecs() {
		if imp.Path == nil || imp.Path.Value == "" {
			continue
		}
		p, err := literal.Unquote(imp.Path.Value)
		if err != nil || p == "" {
			continue
		}
		id := path.Base(p)
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" {
			id = imp.Name.Name
		}
		m[id] = path.Base(p)
	}
	return m
}

// defRefs are the #Def names a field's value reaches the way the CUE
// extraction reads references: unification (&), the value itself, a list's
// element type, an import selector.
func defRefs(v ast.Expr, imports map[string]string) []string {
	seen := map[string]bool{}
	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.Ident:
			if strings.HasPrefix(x.Name, "#") {
				seen[x.Name] = true
			}
		case *ast.SelectorExpr:
			n, _, err := ast.LabelName(x.Sel)
			if err != nil || !strings.HasPrefix(n, "#") {
				return
			}
			if id, ok := x.X.(*ast.Ident); ok {
				if _, ok := imports[id.Name]; ok {
					seen[n] = true
				}
			}
		case *ast.BinaryExpr:
			if x.Op == token.AND {
				walk(x.X)
				walk(x.Y)
			}
		case *ast.ListLit:
			for _, el := range x.Elts {
				if el2, ok := el.(*ast.Ellipsis); ok && el2.Type != nil {
					walk(el2.Type)
				}
			}
		}
	}
	if v != nil {
		walk(v)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (r *run) addDef(file, name, kind string, n ast.Node) {
	r.defs = append(r.defs, def{Path: file, Name: name, Line: n.Pos().Line(), End: n.End().Line(), Kind: kind})
}

// fields walks the CUE field tree: every field, dotted, to depth 6, descending
// only into a direct struct-literal value; each carries its #Def references.
func fields(f *ast.File, file string, imports map[string]string, emit func(name string, fd *ast.Field, refs []string)) {
	var visit func(d ast.Decl, parent string, depth int)
	visit = func(d ast.Decl, parent string, depth int) {
		if depth > 6 {
			return
		}
		fd, ok := d.(*ast.Field)
		if !ok {
			return
		}
		n := labelOf(fd)
		if n == "" {
			return
		}
		dotted := n
		if parent != "" {
			dotted = parent + "." + n
		}
		emit(dotted, fd, defRefs(fd.Value, imports))
		if depth == 6 {
			return
		}
		if lit, ok := fd.Value.(*ast.StructLit); ok {
			for _, e := range lit.Elts {
				visit(e, dotted, depth+1)
			}
		}
	}
	for _, d := range f.Decls {
		visit(d, "", 1)
	}
}

// moduleFiles are a module's own files: every .cue under it but cue.mod/.
func (r *run) moduleFiles(m *module) []string { return r.cueFiles(m.dir) }

// truth writes the definitions and references for each vetted module and each
// vetted bundle and runtime file, and the measured file list.
func (r *run) truth() {
	for _, m := range r.mods {
		if !m.vetOK {
			continue
		}
		for _, file := range r.moduleFiles(m) {
			f, _ := r.parse(file)
			if f == nil {
				continue
			}
			r.files[file] = true
			r.entities(m, file, f)
		}
	}
	// Bundles and runtimes anywhere in the tree, outside a rejected module.
	for _, file := range r.cueFiles(".") {
		if r.files[file] || r.inRejected(file) {
			continue
		}
		f, b := r.parse(file)
		if f == nil {
			continue
		}
		top := map[string]*ast.Field{}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.Field); ok {
				top[labelOf(fd)] = fd
			}
		}
		switch {
		case top["bundle"] != nil:
			r.bundleFiles = append(r.bundleFiles, file)
		case top["runtime"] != nil && bytes.Contains(b, []byte(`"v1alpha1"`)):
			r.runtime = append(r.runtime, file)
		}
	}
}

func (r *run) inRejected(file string) bool {
	for _, m := range r.mods {
		if !m.vetOK && (file == m.dir || strings.HasPrefix(file, m.dir+"/")) {
			return true
		}
	}
	return false
}

// entities derives one file's Timoni entities and every reference pair of its
// CUE fields. m is nil for a bundle or runtime file.
func (r *run) entities(m *module, file string, f *ast.File) {
	imports := importIdents(f)
	// the field tree: references
	fields(f, file, imports, func(name string, fd *ast.Field, refs []string) {
		for _, to := range refs {
			r.refs = append(r.refs, pair{file, name, to})
		}
	})
	for _, d := range f.Decls {
		fd, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		name := labelOf(fd)
		switch name {
		case "timoni":
			r.addDef(file, "timoni/module", "package", fd)
		case "values":
			r.addDef(file, "timoni/values", "variable", fd)
			if lit, ok := fd.Value.(*ast.StructLit); ok {
				seen := map[string]bool{}
				for _, e := range lit.Elts {
					kf, ok := e.(*ast.Field)
					if !ok {
						continue
					}
					k := labelOf(kf)
					if k == "" || strings.HasPrefix(k, "#") || strings.HasPrefix(k, "_") || seen[k] {
						continue
					}
					seen[k] = true
					r.refs = append(r.refs, pair{file, "timoni/values", "#Config." + k})
				}
			}
		case "bundle":
			r.addDef(file, "timoni/bundle", "package", fd)
		case "runtime":
			r.addDef(file, "timoni/runtime", "package", fd)
		default:
			if !strings.HasPrefix(name, "#") {
				continue
			}
			if name == "#Instance" && m != nil && m.instFile == "" {
				m.instFile = file
			}
			if takesConfig(fd.Value) {
				r.refs = append(r.refs, pair{file, name, "#Config"})
			}
			if kind := literalOf(fd.Value, "kind"); kind != "" && m != nil {
				r.sum.ObjectDefs++
				if m.kinds[kind] {
					r.sum.ObjectsRendered++
					r.addDef(file, "timoni/object/"+name, "resource", fd)
					m.objects = append(m.objects, object{name: name, kind: kind, file: file, off: fd.Label.Pos().Offset(), n: len(name)})
				}
			}
		}
	}
}

// refErr is CUE's word for a use of a definition that is gone: a bare
// reference, or a field selected from an import.
var refErr = regexp.MustCompile(`(?m)^(.*): (?:reference "(#[^"]+)" not found|undefined field: (#[^:\s]+)):\s*$`)
var posLine = regexp.MustCompile(`^\s+\./(\S+?):(\d+):(\d+)\s*$`)

// ablate renames each object template's label in its file and vets the module:
// the tool names every use the rename breaks.
func (r *run) ablate() {
	seenPair := map[xref]bool{}
	for _, m := range r.mods {
		if !m.vetOK || len(m.objects) == 0 {
			continue
		}
		for _, o := range m.objects {
			orig, err := os.ReadFile(r.abs(o.file))
			if err != nil {
				continue
			}
			mut := append(append(append([]byte{}, orig[:o.off]...), []byte(o.name+"__ablated")...), orig[o.off+o.n:]...)
			if err := os.WriteFile(r.abs(o.file), mut, 0o644); err != nil {
				continue
			}
			out, verr := r.t.run(r.abs(m.dir), "mod", "vet", ".")
			_ = os.WriteFile(r.abs(o.file), orig, 0o644)
			r.sum.Ablations++
			if verr == nil {
				continue // nothing names the template: no edge
			}
			lines := strings.Split(out, "\n")
			found := false
			for i, l := range lines {
				mm := refErr.FindStringSubmatch(l)
				if mm == nil || (mm[2] != o.name && mm[3] != o.name) {
					continue
				}
				for k := i + 1; k < len(lines); k++ {
					pm := posLine.FindStringSubmatch(lines[k])
					if pm == nil {
						break
					}
					use := path.Join(m.dir, pm[1])
					ln, _ := strconv.Atoi(pm[2])
					from := r.enclosing(use, ln)
					if from == "" {
						continue
					}
					x := xref{Path: use, From: from, ToPath: o.file}
					if !seenPair[x] {
						seenPair[x] = true
						r.xrefs = append(r.xrefs, x)
						r.sum.ObjectXrefs++
					}
					found = true
				}
			}
			if found {
				r.sum.AblationBroke++
			} else {
				r.sum.AblationOther++
			}
		}
	}
}

// enclosing is the top-level field of file that holds line: the entity the
// tool's position names.
func (r *run) enclosing(file string, line int) string {
	f, _ := r.parse(file)
	if f == nil {
		return ""
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.Field); ok && fd.Pos().Line() <= line && line <= fd.End().Line() {
			return labelOf(fd)
		}
	}
	return ""
}

// bundles vets each bundle, reads the instances the tool evaluated, and
// rebuilds each local-module instance alone.
func (r *run) bundles() {
	for _, file := range r.bundleFiles {
		dir := path.Dir(file)
		stem := strings.TrimSuffix(path.Base(file), ".cue")
		args := []string{"bundle", "vet", "-f", path.Base(file)}
		ents, _ := os.ReadDir(r.abs(dir))
		for _, e := range ents {
			if n := e.Name(); strings.HasPrefix(n, stem+"_") && strings.HasSuffix(n, ".cue") {
				args = append(args, "-f", n)
			}
		}
		out, err := r.t.run(r.abs(dir), append(args, "--print-value")...)
		r.sum.Bundles++
		if err != nil {
			r.sum.BundlesRejected[file] = firstErr(out)
			continue
		}
		r.files[file] = true
		f, _ := r.parse(file)
		if f != nil {
			fields(f, file, importIdents(f), func(name string, fd *ast.Field, refs []string) {
				for _, to := range refs {
					r.refs = append(r.refs, pair{file, name, to})
				}
			})
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.Field); ok && labelOf(fd) == "bundle" {
					r.addDef(file, "timoni/bundle", "package", fd)
					r.refs = append(r.refs, r.bundleInstanceDefs(file, fd)...)
				}
			}
		}
		pf, perr := parser.ParseFile("print", []byte(out), parser.ParseComments)
		if perr != nil && pf == nil {
			continue
		}
		var evalBundle *ast.Field
		for _, d := range pf.Decls {
			if fd, ok := d.(*ast.Field); ok && labelOf(fd) == "bundle" {
				evalBundle = fd
			}
		}
		if evalBundle == nil {
			continue
		}
		insts, ok := member(evalBundle, "instances")
		if !ok {
			continue
		}
		for _, in := range members(insts) {
			name := labelOf(in)
			r.sum.Instances++
			mod, ok1 := member(in, "module")
			if !ok1 {
				continue
			}
			uf, ok2 := member(mod, "url")
			if !ok2 {
				continue
			}
			url, ok3 := str(uf.Value)
			switch {
			case !ok3:
				r.sum.InstancesOther++
				continue
			case strings.HasPrefix(url, "oci://"):
				r.sum.InstancesOCI++
				continue
			case !strings.HasPrefix(url, "file://"):
				r.sum.InstancesOther++
				continue
			}
			r.sum.InstancesLocal++
			target := path.Join(dir, strings.TrimPrefix(url, "file://"), "timoni.cue")
			if r.built(file, dir, in) {
				r.sum.InstancesBuilt++
				if _, err := os.Stat(r.abs(target)); err == nil {
					r.xrefs = append(r.xrefs, xref{Path: file, From: "timoni/bundle/instance/" + name, ToPath: target})
					r.sum.BundleXrefs++
				}
			}
		}
	}
	for _, file := range r.runtime {
		f, _ := r.parse(file)
		if f == nil {
			continue
		}
		r.files[file] = true
		r.sum.Runtimes++
		for _, d := range f.Decls {
			fd, ok := d.(*ast.Field)
			if !ok || labelOf(fd) != "runtime" {
				continue
			}
			r.addDef(file, "timoni/runtime", "package", fd)
			if cl, ok := member(fd, "clusters"); ok {
				for _, c := range members(cl) {
					r.addDef(file, "timoni/runtime/cluster/"+labelOf(c), "target", c)
				}
			}
		}
	}
}

// bundleInstanceDefs are the bundle's instance entities as the walk reads
// them (every instance of the written file, whatever its module url).
func (r *run) bundleInstanceDefs(file string, fd *ast.Field) []pair {
	if insts, ok := member(fd, "instances"); ok {
		for _, in := range members(insts) {
			if n := labelOf(in); n != "" {
				r.addDef(file, "timoni/bundle/instance/"+n, "target", in)
			}
		}
	}
	return nil
}

func member(fd *ast.Field, name string) (*ast.Field, bool) {
	for _, m := range members(fd) {
		if labelOf(m) == name {
			return m, true
		}
	}
	return nil, false
}

func members(fd *ast.Field) []*ast.Field {
	lit, ok := fd.Value.(*ast.StructLit)
	if !ok {
		return nil
	}
	var out []*ast.Field
	for _, e := range lit.Elts {
		if f, ok := e.(*ast.Field); ok {
			out = append(out, f)
		}
	}
	return out
}

// built rebuilds one instance alone, in the bundle's directory, and reports
// whether the tool's output names it.
func (r *run) built(file, dir string, in *ast.Field) bool {
	name := labelOf(in)
	val, err := format.Node(in.Value)
	if err != nil {
		return false
	}
	tmp := fmt.Sprintf("zz_oracle_%s.cue", strconv.Itoa(len(name)))
	src := fmt.Sprintf("bundle: {\n\tapiVersion: \"v1alpha1\"\n\tname: \"oracle\"\n\tinstances: %q: %s\n}\n", name, val)
	if err := os.WriteFile(filepath.Join(r.abs(dir), tmp), []byte(src), 0o644); err != nil {
		return false
	}
	defer os.Remove(filepath.Join(r.abs(dir), tmp))
	out, err := r.t.run(r.abs(dir), "bundle", "build", "-f", tmp)
	if err != nil {
		return false
	}
	return strings.Contains(out, "# Instance: "+name+"\n")
}

func (r *run) write(out, xout, filesOut, sumOut string) {
	sort.Slice(r.defs, func(i, j int) bool {
		a, b := r.defs[i], r.defs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	r.defs = dedupDefs(r.defs)
	for i := range r.defs {
		// lang-oracle compares an oracle's definition name with the dialect's
		// last segment (design §1.4): the entity timoni/object/#Deployment is
		// written as the segment the harness keeps.
		r.defs[i].Name = bareName(r.defs[i].Name)
	}
	refs := dedupPairs(r.refs, r.files)
	sort.Slice(r.xrefs, func(i, j int) bool {
		a, b := r.xrefs[i], r.xrefs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.ToPath < b.ToPath
	})
	xr := r.xrefs
	if xr == nil {
		xr = []xref{}
	}
	writeJSON(out, oracleFile{Tool: "timoni oracle: a CUE syntax walk gated by " + r.version, Version: r.version, Tier: "C",
		Definitions: r.defs, References: refs})
	writeJSON(xout, oracleFile{Tool: r.version, Version: r.version, Tier: "A", Xrefs: &xr})
	var files []string
	for f := range r.files {
		files = append(files, f)
	}
	sort.Strings(files)
	r.sum.Files = len(files)
	if filesOut != "" {
		_ = os.WriteFile(filesOut, []byte(strings.Join(files, "\n")+"\n"), 0o644)
	}
	if sumOut != "" {
		writeJSON(sumOut, r.sum)
	}
	fmt.Fprintf(os.Stderr, "timoni-oracle: %d modules (%d vetted), %d files, %d definitions, %d references, %d cross_references\n",
		r.sum.Modules, r.sum.ModulesVetted, len(files), len(r.defs), len(refs), len(xr))
}

func dedupDefs(ds []def) []def {
	seen := map[[2]string]bool{}
	var out []def
	for _, d := range ds {
		k := [2]string{d.Path, d.Name}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, d)
	}
	return out
}

func dedupPairs(ps []pair, in map[string]bool) []pair {
	seen := map[pair]bool{}
	var out []pair
	for _, p := range ps {
		if !in[p.Path] || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	return out
}

func writeJSON(p string, v any) {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil {
		fatal(err)
	}
}

// bareName is a qualified name's last segment: the text after the last of
// . : # / \ space and ).
func bareName(q string) string {
	return q[strings.LastIndexAny(q, ".:#/\\ )")+1:]
}
