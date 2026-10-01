// Command terraform-oracle is the language-breadth harness's oracle for the
// terraform native dialect (design §1.3, LB-Q-terraform). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10), tier A:
//
//	go run . -subject <checkout> -scratch <dir outside the subject> -mirror <provider mirror> [-tofu tofu] [-files-out <file>]
//
// The tool is OpenTofu itself, run on every root module of a scratch copy of
// the subject: every directory holding a .tf file (the connector's own
// exclusions apply: vendor, testdata, node_modules and dot directories), each
// one in turn — the subject root, examples/*, and every module and wrapper,
// which a caller also reaches. Per root:
//
//   - `tofu init -backend=false -input=false`, providers from the shared
//     offline filesystem mirror only (a CLI configuration with one
//     filesystem_mirror and no direct installation; TF_DATA_DIR and the
//     plugin cache under -scratch). A registry module a root calls is
//     downloaded by init; it is not a file of the subject and leaves the
//     measurement.
//   - `tofu show -json -config`: the tool's own reading of the configuration
//     tree (every module call's configuration nested in its caller's): the
//     resources, data sources, variables, outputs and module calls each module
//     declares, and every expression's references, count and for_each, and
//     depends_on.
//   - `tofu graph -type=plan`: the plan graph, for what the configuration
//     JSON leaves out: local values (a node each, and their edges) and the
//     references of a dynamic block's content (the JSON renders expressions
//     by the provider schema, which names no dynamic block).
//   - .terraform/modules/modules.json (TF_DATA_DIR/modules): the directory
//     init installed each module call into, which places a module path in the
//     subject (or outside it, for a downloaded one).
//
// The tool prints no source positions, so each definition's file, line and
// end come from an independent line reader (its own scanner: a top-level
// block's header line, its closing brace at column 0, a heredoc skipped; a
// local value is an attribute at the locals block's own indentation), which
// is no HCL parser and shares no code with the dialect.
//
// What it writes (names are the dialect's: aws_vpc.this, data.T.N, module.m,
// var.x, local.x, output.o; repo-relative slash paths):
//
//   - definitions: every resource ("resource"), data source ("data_source"),
//     module call ("module"), variable ("variable_decl"), output ("output")
//     and local value ("variable") of every in-subject module a root reaches.
//   - references: (definition, referenced definition of the same module),
//     keyed by the referring definition's file wherever the target is
//     declared, as lang-oracle compares a dialect's Calls (by name, per
//     referring file; the kubernetes and taskfile oracles' references are the
//     same): an expression's references, count/for_each and depends_on from
//     the configuration JSON, and the plan graph's edges (a local's, a dynamic
//     block's), read as references to the definition each address names
//     (module.m.o names the call module.m). The plan graph is transitively
//     reduced, so a local's reference to something another of its
//     references already depends on is not in it.
//   - cross_references, file granularity: the references whose ends are in
//     different files (to_path the referenced definition's file), plus a
//     module call to each input variable its arguments set (in the called
//     module's directory) and a reference to module.m.o to output o in the
//     called module's directory. A module instance reached from several roots
//     is one set of files: every entry is deduplicated.
//
// A root the tool cannot initialize or graph contributes nothing and is named
// on stderr; -files-out lists the .tf files of the modules the measured roots
// reached, for lang-oracle -only.
//
// Bench-only; never a product dependency. The scratch directory (a copy of
// the subject, TF_DATA_DIR per root, the unpacked provider cache) is deleted
// after measuring; the mirror is kept for the next terraform measurement.
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
	"regexp"
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

type ref struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}

func main() {
	subject := flag.String("subject", "", "subject checkout (read-only; a scratch copy is measured)")
	scratch := flag.String("scratch", "", "scratch directory outside the subject")
	mirror := flag.String("mirror", "", "offline provider mirror (tofu providers mirror -platform=<os_arch>)")
	tofu := flag.String("tofu", "tofu", "OpenTofu binary")
	filesOut := flag.String("files-out", "", "write the .tf files of the modules the measured roots reached, one per line")
	flag.Parse()
	if *subject == "" || *scratch == "" || *mirror == "" {
		fmt.Fprintln(os.Stderr, "usage: terraform-oracle -subject <checkout> -scratch <dir> -mirror <provider mirror> [-tofu tofu] [-files-out <file>]")
		os.Exit(2)
	}
	o, files, err := measure(context.Background(), *subject, *scratch, *mirror, *tofu)
	if err != nil {
		fmt.Fprintln(os.Stderr, "terraform-oracle:", err)
		os.Exit(1)
	}
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(files, "\n")+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "terraform-oracle:", err)
			os.Exit(1)
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		fmt.Fprintln(os.Stderr, "terraform-oracle:", err)
		os.Exit(1)
	}
}

// ── running the tool ──────────────────────────────────────────────────────

func tofuEnv(cliConfig, dataDir string) []string {
	var env []string
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if strings.HasPrefix(k, "TF_") {
			continue
		}
		env = append(env, e)
	}
	return append(env, "TF_CLI_CONFIG_FILE="+cliConfig, "TF_DATA_DIR="+dataDir, "TF_IN_AUTOMATION=1", "TF_INPUT=0", "CHECKPOINT_DISABLE=1")
}

// run runs the tool, up to three times: init, show and graph are idempotent,
// and a host that briefly refuses a read (a provider package, a module file)
// fails a run that a second one completes. Each failed attempt is named on
// stderr; the last error is returned.
func run(ctx context.Context, bin, dir string, env []string, args ...string) ([]byte, error) {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		var b []byte
		if b, err = runOnce(ctx, bin, dir, env, args...); err == nil {
			return b, nil
		}
		fmt.Fprintf(os.Stderr, "attempt %d: %v\n", attempt, err)
		time.Sleep(2 * time.Second)
	}
	return nil, err
}

func runOnce(ctx context.Context, bin, dir string, env []string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tofu %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(tail(stderr.String(), 600)))
	}
	return stdout.Bytes(), nil
}

// readFile and readDir retry a refused read (EPERM) a few times: the host
// this runs on can refuse a read for a moment, and a definition or a module
// manifest silently missing would bias the measurement.
func readFile(p string) ([]byte, error) {
	var b []byte
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if b, err = os.ReadFile(p); err == nil || !os.IsPermission(err) {
			return b, err
		}
		time.Sleep(time.Second)
	}
	return b, err
}

func readDir(p string) ([]os.DirEntry, error) {
	var ents []os.DirEntry
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if ents, err = os.ReadDir(p); err == nil || !os.IsPermission(err) {
			return ents, err
		}
		time.Sleep(time.Second)
	}
	return ents, err
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// ── the measurement ───────────────────────────────────────────────────────

// entity is one definition: the module directory (subject-relative) and its
// address in that module.
type entity struct{ dir, name string }

// collector accumulates every root's findings, deduplicated.
type collector struct {
	defs     map[entity]string // entity → kind
	refs     map[[2]entity]bool
	inputs   map[[2]entity]bool // module call → the called module's variable
	dirs     map[string]bool
	graphRef int // references only the plan graph gave a non-local definition (a dynamic block's)
}

func measure(ctx context.Context, subject, scratch, mirror, tofu string) (out, []string, error) {
	subject, err := filepath.Abs(subject)
	if err != nil {
		return out{}, nil, err
	}
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		return out{}, nil, err
	}
	scratch, err = filepath.Abs(scratch)
	if err != nil {
		return out{}, nil, err
	}
	if rel, err := filepath.Rel(subject, scratch); err == nil && !strings.HasPrefix(rel, "..") {
		return out{}, nil, fmt.Errorf("-scratch %s is inside the subject", scratch)
	}
	src := filepath.Join(scratch, "src")
	if err := os.RemoveAll(src); err != nil {
		return out{}, nil, err
	}
	if err := copyTree(subject, src); err != nil {
		return out{}, nil, err
	}
	cliConfig := filepath.Join(scratch, "tofurc")
	cache := filepath.Join(scratch, "plugin-cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return out{}, nil, err
	}
	rc := fmt.Sprintf("provider_installation {\n  filesystem_mirror {\n    path    = %q\n    include = [\"*/*/*\"]\n  }\n}\nplugin_cache_dir = %q\n", mirror, cache)
	if err := os.WriteFile(cliConfig, []byte(rc), 0o644); err != nil {
		return out{}, nil, err
	}
	version := "unknown"
	if b, err := run(ctx, tofu, scratch, tofuEnv(cliConfig, filepath.Join(scratch, "data", "version")), "version", "-json"); err == nil {
		var v struct {
			Version string `json:"terraform_version"`
		}
		if json.Unmarshal(b, &v) == nil && v.Version != "" {
			version = v.Version
		}
	}

	roots, err := tfDirs(src)
	if err != nil {
		return out{}, nil, err
	}
	c := &collector{defs: map[entity]string{}, refs: map[[2]entity]bool{}, inputs: map[[2]entity]bool{}, dirs: map[string]bool{}}
	failed := 0
	for i, root := range roots {
		dataDir := filepath.Join(scratch, "data", fmt.Sprintf("%03d", i))
		env := tofuEnv(cliConfig, dataDir)
		wd := filepath.Join(src, filepath.FromSlash(root))
		if _, err := run(ctx, tofu, wd, env, "init", "-backend=false", "-input=false", "-no-color"); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "root %s: init failed: %v\n", root, err)
			continue
		}
		cfg, err := run(ctx, tofu, wd, env, "show", "-json", "-config", "-no-color")
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "root %s: show -config failed: %v\n", root, err)
			continue
		}
		dot, err := run(ctx, tofu, wd, env, "graph", "-type=plan", "-no-color")
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "root %s: graph failed: %v\n", root, err)
			continue
		}
		mods, err := moduleDirs(filepath.Join(dataDir, "modules", "modules.json"), src, wd)
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "root %s: modules.json: %v\n", root, err)
			continue
		}
		if err := c.addRoot(cfg, dot, mods); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "root %s: %v\n", root, err)
			continue
		}
	}

	o := out{Tool: "opentofu (tofu show -json -config, tofu graph -type=plan)", Version: version, Tier: "A"}
	where := map[string]map[string]located{} // dir → name → position
	locate := func(e entity) (located, bool) {
		m, ok := where[e.dir]
		if !ok {
			m = scanDir(filepath.Join(src, filepath.FromSlash(e.dir)), e.dir)
			where[e.dir] = m
		}
		l, ok := m[e.name]
		return l, ok
	}
	unlocated := 0
	for e, kind := range c.defs {
		l, ok := locate(e)
		if !ok {
			unlocated++
			fmt.Fprintf(os.Stderr, "unlocated: %s %s\n", e.dir, e.name)
			continue
		}
		o.Definitions = append(o.Definitions, def{Path: l.path, Name: e.name, Line: l.line, End: l.end, Kind: kind})
	}
	refSet := map[ref]bool{}
	xrefSet := map[xref]bool{}
	pair := func(from, to entity) {
		lf, ok1 := locate(from)
		lt, ok2 := locate(to)
		if !ok1 || !ok2 {
			return
		}
		if lf.path != lt.path {
			xrefSet[xref{Path: lf.path, From: from.name, ToPath: lt.path}] = true
		}
	}
	for p := range c.refs {
		pair(p[0], p[1])
	}
	for p := range c.inputs {
		pair(p[0], p[1])
	}
	// references: every reference a definition makes to a definition of its
	// own module, keyed by the referring file and named by address, wherever
	// the target is declared (lang-oracle compares a dialect's Calls by name
	// per referring file, as the kubernetes and taskfile oracles' references
	// are; the cross-file subset is cross_references, L3's).
	for p := range c.refs {
		if p[0].dir != p[1].dir {
			continue
		}
		if lf, ok := locate(p[0]); ok {
			if _, ok := locate(p[1]); ok {
				refSet[ref{Path: lf.path, From: p[0].name, To: p[1].name}] = true
			}
		}
	}
	for r := range refSet {
		o.References = append(o.References, r)
	}
	for x := range xrefSet {
		o.CrossReferences = append(o.CrossReferences, x)
	}
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
	sort.Slice(o.References, func(i, j int) bool {
		a, b := o.References[i], o.References[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath < b.Path+"\x00"+b.From+"\x00"+b.ToPath
	})
	var files []string
	for d := range c.dirs {
		ents, _ := readDir(filepath.Join(src, filepath.FromSlash(d)))
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".tf") {
				files = append(files, path.Join(d, e.Name()))
			}
		}
	}
	for i, f := range files {
		files[i] = strings.TrimPrefix(f, "./")
	}
	sort.Strings(files)
	fmt.Fprintf(os.Stderr, "terraform-oracle: opentofu %s: %d roots (%d failed), %d module dirs, %d definitions (%d unlocated), %d references, %d cross-references, %d references only the plan graph gave a resource, data source, output or module call\n",
		version, len(roots), failed, len(c.dirs), len(o.Definitions), unlocated, len(o.References), len(o.CrossReferences), c.graphRef)
	return o, files, nil
}

// tfDirs is every directory of the tree holding a .tf file (slash paths,
// "." for the root), skipping the connector's exclusions.
func tfDirs(root string) ([]string, error) {
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tf") {
			rel, _ := filepath.Rel(root, filepath.Dir(p))
			seen[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	var dirs []string
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs, err
}

func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "node_modules"
}

func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			if p != from && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(dst, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := readFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
}

// moduleDirs reads init's module manifest: module path ("a.b", "" for the
// root) → subject-relative directory, "" for one outside the subject.
func moduleDirs(manifest, src, wd string) (map[string]string, error) {
	b, err := readFile(manifest)
	if os.IsNotExist(err) {
		// A root that calls no module: init writes no manifest.
		rel, _ := filepath.Rel(src, wd)
		return map[string]string{"": filepath.ToSlash(rel)}, nil
	}
	if err != nil {
		return nil, err
	}
	var m struct {
		Modules []struct{ Key, Dir string }
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	res := map[string]string{}
	for _, mod := range m.Modules {
		dir := mod.Dir
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(wd, dir)
		}
		rel, err := filepath.Rel(src, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			res[mod.Key] = ""
			continue
		}
		res[mod.Key] = filepath.ToSlash(rel)
	}
	if _, ok := res[""]; !ok {
		rel, _ := filepath.Rel(src, wd)
		res[""] = filepath.ToSlash(rel)
	}
	return res, nil
}

// ── the configuration JSON ────────────────────────────────────────────────

type exprs = map[string]json.RawMessage

type cfgModule struct {
	Resources []struct {
		Address          string          `json:"address"`
		Mode             string          `json:"mode"`
		Expressions      json.RawMessage `json:"expressions"`
		CountExpression  json.RawMessage `json:"count_expression"`
		ForEachExpresion json.RawMessage `json:"for_each_expression"`
		DependsOn        []string        `json:"depends_on"`
	} `json:"resources"`
	Outputs map[string]struct {
		Expression json.RawMessage `json:"expression"`
		DependsOn  []string        `json:"depends_on"`
	} `json:"outputs"`
	Variables   map[string]json.RawMessage `json:"variables"`
	ModuleCalls map[string]struct {
		Expressions       exprs           `json:"expressions"`
		CountExpression   json.RawMessage `json:"count_expression"`
		ForEachExpression json.RawMessage `json:"for_each_expression"`
		DependsOn         []string        `json:"depends_on"`
		Module            cfgModule       `json:"module"`
	} `json:"module_calls"`
}

// references collects every "references" list under an expression tree.
func references(raw json.RawMessage, into *[]string) {
	if len(raw) == 0 {
		return
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, x := range t {
				if k == "references" {
					if l, ok := x.([]any); ok {
						for _, s := range l {
							if r, ok := s.(string); ok {
								*into = append(*into, r)
							}
						}
					}
					continue
				}
				walk(x)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	walk(v)
}

var indexRE = regexp.MustCompile(`\[[^\]]*\]`)

// target is what a reference names: the definition its address names in
// the referring module, and for module.m.o also output o of the called
// module (child is "" otherwise). ok is false for a reference to no
// definition (count.index, each.value, self, path.module, terraform.workspace).
func target(r string) (name, childCall, childOutput string, ok bool) {
	p := strings.Split(indexRE.ReplaceAllString(r, ""), ".")
	switch p[0] {
	case "count", "each", "self", "path", "terraform":
		return "", "", "", false
	case "var", "local":
		if len(p) < 2 {
			return "", "", "", false
		}
		return p[0] + "." + p[1], "", "", true
	case "module":
		if len(p) < 2 {
			return "", "", "", false
		}
		if len(p) > 2 {
			return "module." + p[1], p[1], p[2], true
		}
		return "module." + p[1], "", "", true
	case "data":
		if len(p) < 3 {
			return "", "", "", false
		}
		return "data." + p[1] + "." + p[2], "", "", true
	}
	if len(p) < 2 {
		return "", "", "", false
	}
	return p[0] + "." + p[1], "", "", true
}

func (c *collector) addRoot(cfgJSON, dot []byte, mods map[string]string) error {
	var cfg struct {
		RootModule cfgModule `json:"root_module"`
	}
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return fmt.Errorf("show -config: %w", err)
	}
	cfgRefs := map[[2]entity]bool{}
	c.addConfig(cfg.RootModule, "", mods, cfgRefs)
	if err := c.addGraph(dot, mods, cfgRefs); err != nil {
		return err
	}
	for d := range mods {
		if mods[d] != "" {
			c.dirs[mods[d]] = true
		}
	}
	return nil
}

func childKey(key, call string) string {
	if key == "" {
		return call
	}
	return key + "." + call
}

// addConfig records one module's definitions and references from the
// configuration JSON, then its calls' modules.
func (c *collector) addConfig(m cfgModule, key string, mods map[string]string, cfgRefs map[[2]entity]bool) {
	dir := mods[key]
	refer := func(from string, rs []string) {
		for _, r := range rs {
			name, call, output, ok := target(r)
			if !ok {
				continue
			}
			c.ref(dir, from, name, cfgRefs)
			if call != "" {
				if cd := mods[childKey(key, call)]; cd != "" && dir != "" {
					c.refs[[2]entity{{dir, from}, {cd, "output." + output}}] = true
				}
			}
		}
	}
	for _, r := range m.Resources {
		kind := "resource"
		if r.Mode == "data" {
			kind = "data_source"
		}
		if dir != "" {
			c.defs[entity{dir, r.Address}] = kind
		}
		var rs []string
		references(r.Expressions, &rs)
		references(r.CountExpression, &rs)
		references(r.ForEachExpresion, &rs)
		rs = append(rs, r.DependsOn...)
		refer(r.Address, rs)
	}
	for name, o := range m.Outputs {
		if dir != "" {
			c.defs[entity{dir, "output." + name}] = "output"
		}
		var rs []string
		references(o.Expression, &rs)
		rs = append(rs, o.DependsOn...)
		refer("output."+name, rs)
	}
	for name := range m.Variables {
		if dir != "" {
			c.defs[entity{dir, "var." + name}] = "variable_decl"
		}
	}
	for name, mc := range m.ModuleCalls {
		from := "module." + name
		if dir != "" {
			c.defs[entity{dir, from}] = "module"
		}
		var rs []string
		for _, e := range mc.Expressions {
			references(e, &rs)
		}
		references(mc.CountExpression, &rs)
		references(mc.ForEachExpression, &rs)
		rs = append(rs, mc.DependsOn...)
		refer(from, rs)
		ck := childKey(key, name)
		if cd := mods[ck]; cd != "" && dir != "" {
			for arg := range mc.Expressions {
				if _, declared := mc.Module.Variables[arg]; declared {
					c.inputs[[2]entity{{dir, from}, {cd, "var." + arg}}] = true
				}
			}
		}
		c.addConfig(mc.Module, ck, mods, cfgRefs)
	}
}

func (c *collector) ref(dir, from, to string, cfgRefs map[[2]entity]bool) {
	if dir == "" || from == to {
		return
	}
	p := [2]entity{{dir, from}, {dir, to}}
	c.refs[p] = true
	if cfgRefs != nil {
		cfgRefs[p] = true
	}
}

// ── the plan graph ────────────────────────────────────────────────────────

// gnode is one plan-graph node: its module path ("a.b"), its address in
// that module ("" for a module call's own node: then call names it), and the
// node's annotation ("expand", "close", "expand, input", "expand, reference").
type gnode struct {
	key, addr, call, note string
}

var edgeRE = regexp.MustCompile(`^\s*"((?:[^"\\]|\\.)*)"\s*->\s*"((?:[^"\\]|\\.)*)"`)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9_-]+`)

// parseNode reads a node name: `[root] module.a.aws_vpc.this (expand)`,
// `[root] module.a.module.b (close)`, `[root] module.a.var.x (expand, input)`.
// ok is false for a provider, the root and a meta node.
func parseNode(s string) (gnode, bool) {
	s = strings.TrimPrefix(s, "[root] ")
	var n gnode
	if i := strings.LastIndex(s, " ("); i >= 0 && strings.HasSuffix(s, ")") {
		n.note = s[i+2 : len(s)-1]
		s = s[:i]
	}
	if s == "root" || strings.HasPrefix(s, "meta.") || strings.Contains(s, "provider[") {
		return gnode{}, false
	}
	var path []string
	for strings.HasPrefix(s, "module.") {
		rest := s[len("module."):]
		name := nameRE.FindString(rest)
		if name == "" {
			return gnode{}, false
		}
		rest = rest[len(name):]
		if rest == "" {
			n.key = strings.Join(path, ".")
			n.call = name
			return n, true
		}
		if !strings.HasPrefix(rest, ".") {
			return gnode{}, false
		}
		path = append(path, name)
		s = rest[1:]
	}
	n.key = strings.Join(path, ".")
	switch {
	case strings.HasPrefix(s, "var."), strings.HasPrefix(s, "local."), strings.HasPrefix(s, "output."), strings.HasPrefix(s, "data."):
	case strings.HasPrefix(s, "provider"), strings.HasPrefix(s, "check."), strings.HasPrefix(s, "ephemeral."):
		return gnode{}, false
	default:
		if !strings.Contains(s, ".") {
			return gnode{}, false
		}
	}
	n.addr = s
	return n, true
}

func parentKey(key string) (string, string) {
	i := strings.LastIndex(key, ".")
	if i < 0 {
		return "", key
	}
	return key[:i], key[i+1:]
}

// addGraph records the plan graph's local values and every edge that is a
// reference: from a definition (a module call's argument node is the call's)
// to a definition of the same module, to output o of a module it calls
// (module.m.o: the call and the output), or to a call's own node (depends_on
// a module). Edges to providers, to a module's expand and close nodes from
// inside it, and between a variable's reference and input nodes are the
// graph's own structure.
func (c *collector) addGraph(dot []byte, mods map[string]string, cfgRefs map[[2]entity]bool) error {
	for _, line := range strings.Split(string(dot), "\n") {
		m := edgeRE.FindStringSubmatch(line)
		if m == nil {
			// A node declaration: a local value is a definition.
			if i := strings.Index(line, `"[root] `); i >= 0 && !strings.Contains(line, "->") {
				end := strings.Index(line[i+1:], `"`)
				if end > 0 {
					if n, ok := parseNode(line[i+1 : i+1+end]); ok && strings.HasPrefix(n.addr, "local.") {
						if d := mods[n.key]; d != "" {
							c.defs[entity{d, n.addr}] = "variable"
						}
					}
				}
			}
			continue
		}
		a, okA := parseNode(unquote(m[1]))
		b, okB := parseNode(unquote(m[2]))
		for _, n := range []gnode{a, b} {
			if strings.HasPrefix(n.addr, "local.") {
				if d := mods[n.key]; d != "" {
					c.defs[entity{d, n.addr}] = "variable"
				}
			}
		}
		if !okA || !okB {
			continue
		}
		// The referring definition.
		var fromKey, from string
		switch {
		case a.call != "":
			if a.note != "expand" {
				continue
			}
			fromKey, from = a.key, "module."+a.call
		case strings.HasPrefix(a.addr, "var.") && strings.Contains(a.note, "input"):
			pk, call := parentKey(a.key)
			if a.key == "" {
				continue
			}
			fromKey, from = pk, "module."+call
		default:
			fromKey, from = a.key, a.addr
		}
		dir := mods[fromKey]
		if dir == "" {
			continue
		}
		switch {
		case b.call != "":
			// module.m's own node: a reference to the call (depends_on).
			if b.key == fromKey {
				c.graphEdge(dir, from, "module."+b.call, cfgRefs)
			}
		case b.key == fromKey && b.addr != "":
			if strings.HasPrefix(b.addr, "var.") && strings.Contains(b.note, "input") {
				continue
			}
			c.graphEdge(dir, from, b.addr, cfgRefs)
		case strings.HasPrefix(b.addr, "output."):
			pk, call := parentKey(b.key)
			if b.key != "" && pk == fromKey {
				c.graphEdge(dir, from, "module."+call, cfgRefs)
				if cd := mods[b.key]; cd != "" {
					c.refs[[2]entity{{dir, from}, {cd, b.addr}}] = true
				}
			}
		}
	}
	return nil
}

func (c *collector) graphEdge(dir, from, to string, cfgRefs map[[2]entity]bool) {
	if from == to {
		return
	}
	p := [2]entity{{dir, from}, {dir, to}}
	if !cfgRefs[p] && !strings.HasPrefix(from, "local.") && !c.refs[p] {
		c.graphRef++
	}
	c.refs[p] = true
}

func unquote(s string) string {
	return strings.ReplaceAll(s, `\"`, `"`)
}

// ── the independent line reader ───────────────────────────────────────────

type located struct {
	path      string
	line, end int
}

var (
	headerRE  = regexp.MustCompile(`^(resource|data)\s+"([^"]+)"\s+"([^"]+)"\s*\{(.*)$`)
	header1RE = regexp.MustCompile(`^(module|variable|output|locals|provider|terraform|moved|import|removed|check)\b(?:\s+"([^"]+)")?\s*\{(.*)$`)
	localRE   = regexp.MustCompile(`^  ([A-Za-z_][A-Za-z0-9_-]*)\s*=[^=]`)
	heredocRE = regexp.MustCompile(`<<-?([A-Za-z_][A-Za-z0-9_]*)\s*$`)
)

// scanDir locates every definition the .tf files of one directory declare,
// by name. A top-level block starts at column 0 with its header and ends at
// the first later line that is exactly "}" (or on its own line when the
// header closes it); a local value is an attribute at two spaces inside a
// locals block, ending at the last non-blank, non-comment line before the
// next one or the block's end. Heredoc bodies are skipped.
func scanDir(abs, rel string) map[string]located {
	res := map[string]located{}
	ents, err := readDir(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan %s: %v\n", rel, err)
		return res
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
			continue
		}
		b, err := readFile(filepath.Join(abs, e.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "scan %s/%s: %v\n", rel, e.Name(), err)
			continue
		}
		p := path.Join(rel, e.Name())
		p = strings.TrimPrefix(p, "./")
		for name, l := range scanFile(string(b)) {
			l.path = p
			res[name] = l
		}
	}
	return res
}

func scanFile(src string) map[string]located {
	res := map[string]located{}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var heredoc string
	inComment := false
	block, blockName := "", ""
	blockStart := 0
	var local string
	localStart := 0
	lastContent := 0 // last non-blank, non-comment line inside the current local
	closeLocal := func() {
		if local != "" {
			end := lastContent
			if end < localStart {
				end = localStart
			}
			res["local."+local] = located{line: localStart, end: end}
			local = ""
		}
	}
	for i, raw := range lines {
		n := i + 1
		line := strings.TrimRight(raw, " \t")
		trim := strings.TrimSpace(line)
		if heredoc != "" {
			if trim == heredoc {
				heredoc = ""
			}
			if local != "" {
				lastContent = n
			}
			continue
		}
		if inComment {
			if strings.Contains(line, "*/") {
				inComment = false
			}
			continue
		}
		if strings.HasPrefix(trim, "/*") && !strings.Contains(trim, "*/") {
			inComment = true
			continue
		}
		if block == "" {
			if m := headerRE.FindStringSubmatch(line); m != nil {
				name := m[2] + "." + m[3]
				if m[1] == "data" {
					name = "data." + name
				}
				if oneLine(m[4]) {
					res[name] = located{line: n, end: n}
					continue
				}
				block, blockName, blockStart = m[1], name, n
			} else if m := header1RE.FindStringSubmatch(line); m != nil {
				name := ""
				switch m[1] {
				case "module":
					name = "module." + m[2]
				case "variable":
					name = "var." + m[2]
				case "output":
					name = "output." + m[2]
				}
				if oneLine(m[3]) {
					if name != "" {
						res[name] = located{line: n, end: n}
					}
					continue
				}
				block, blockName, blockStart = m[1], name, n
			}
			if hd := heredocRE.FindStringSubmatch(line); hd != nil {
				heredoc = hd[1]
			}
			continue
		}
		if line == "}" {
			if block == "locals" {
				closeLocal()
			} else if blockName != "" {
				res[blockName] = located{line: blockStart, end: n}
			}
			block, blockName = "", ""
			continue
		}
		if block == "locals" {
			if m := localRE.FindStringSubmatch(line + " "); m != nil {
				closeLocal()
				local, localStart, lastContent = m[1], n, n
			} else if local != "" && trim != "" && !strings.HasPrefix(trim, "#") && !strings.HasPrefix(trim, "//") {
				lastContent = n
			}
		}
		if hd := heredocRE.FindStringSubmatch(line); hd != nil {
			heredoc = hd[1]
		}
	}
	return res
}

// oneLine: the rest of a header line after its "{" closes the block.
func oneLine(rest string) bool {
	rest = strings.TrimSpace(rest)
	return strings.HasSuffix(rest, "}") && strings.Count(rest, "{") < strings.Count(rest, "}")
}
