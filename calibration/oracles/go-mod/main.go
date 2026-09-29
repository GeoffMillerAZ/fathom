// Command go-mod-oracle is the language-breadth harness's oracle for the go-mod
// native dialect (design §1.3, owner decision D4). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10), tier A:
//
//	go run . -subject <checkout> -modcache <GOMODCACHE outside the subject> [-go <go>]
//
// The tool is the go command itself, run at every go.mod and go.work the
// subject holds (the connector's own exclusions: vendor, testdata,
// node_modules, .git):
//
//   - `go mod edit -json <go.mod>` and `go work edit -json <go.work>`, the
//     command's own parse of the file, for what each file declares: the module,
//     its requires, its replaces, a workspace's uses. The parse is
//     golang.org/x/mod/modfile, the package the dialect reads with, so it does
//     not catch a parser bug; it attests what the dialect models (every
//     directive, every block form) and the positions, which the tool does not
//     print and this program finds by an independent line scan (its own
//     tokenizer, no modfile), aligned entry for entry with the tool's list and
//     cross-checked (scan-mismatch counts on stderr, zero when the file's own
//     words are what the tool parsed);
//   - `go list -e -m -json all`, the command's own evaluation of the module
//     graph, for what a declaration resolves to: run at each go.mod with
//     GOWORK=off (its own replaces applied) and, where a go.work reaches the
//     module, in workspace mode. -e keeps the evaluation of a module the graph
//     cannot resolve in full (a requirement no proxy serves and nothing
//     redirects is reported with an error and resolves to nothing, while its
//     other requirements and replaces are still evaluated). A module the graph
//     resolves to a directory inside the subject (a replace to a local path, a
//     workspace member) is an edge.
//
// What it writes:
//
//   - definitions: a `module` per go.mod (at its module directive), a `package`
//     per require (at its line), a `key` per replace (replace/<old>[@<version>])
//     of a go.mod or go.work, and per go.work a `namespace` (workspace, at its
//     first directive) and a `key` per use (use/<directory as written>).
//   - references: (module, required module) for every require.
//   - cross_references, entity granularity: a require whose module the tool
//     resolves to a directory of the subject holding a go.mod (the requiring
//     module to the required one), a replace to a local directory the tool
//     evaluated as the module's replacement (the replace entity to that
//     directory's module), and a go.work use the tool lists as a workspace
//     module (the use entity to that module).
//
// A go.mod whose evaluation fails outright still has its declarations attested
// by the parse; it contributes no cross-references, and is counted on stderr.
//
// Bench-only; never a product dependency. The module cache is scratch to
// delete after measuring (it is written with -modcacherw so it can be).
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
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
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
	To     string `json:"to,omitempty"`
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}

// ── the go command's parse ────────────────────────────────────────────────

type modVersion struct{ Path, Version string }

type modEdit struct {
	Module  struct{ Path string }
	Go      string
	Require []struct {
		Path, Version string
		Indirect      bool
	}
	Replace []struct{ Old, New modVersion }
}

type workEdit struct {
	Go      string
	Use     []struct{ DiskPath, ModulePath string }
	Replace []struct{ Old, New modVersion }
}

// goEnv is the environment for one run: nothing of the operator's Go
// configuration leaks in.
func goEnv(modcache, workspace string, flags string) []string {
	env := []string{}
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		switch k {
		case "GOWORK", "GOFLAGS", "GOMODCACHE", "GOTOOLCHAIN", "GONOSUMDB", "GONOSUMCHECK":
			continue
		}
		env = append(env, e)
	}
	env = append(env, "GOMODCACHE="+modcache, "GOTOOLCHAIN=local", "GONOSUMDB=*")
	if workspace == "off" {
		env = append(env, "GOWORK=off")
	}
	if flags != "" {
		env = append(env, "GOFLAGS="+flags)
	}
	return env
}

func run(ctx context.Context, goBin, dir string, env []string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nice", append([]string{"-n", "10", goBin}, args...)...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(tail(stderr.String(), 600)))
	}
	return stdout.Bytes(), nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// ── go list -m ────────────────────────────────────────────────────────────

type listMod struct {
	Path    string
	Version string
	Main    bool
	Dir     string
	Replace *struct {
		Path, Version, Dir string
	}
	// Error is set, with -e, on a module the tool could not resolve: a
	// requirement no proxy serves and nothing redirects.
	Error *struct{ Err string }
}

func goList(ctx context.Context, goBin, modcache, dir string, workspace bool) (map[string]listMod, error) {
	var b []byte
	var err error
	if workspace {
		b, err = run(ctx, goBin, dir, goEnv(modcache, "", "-modcacherw"), "list", "-e", "-m", "-json", "all")
	} else {
		b, err = run(ctx, goBin, dir, goEnv(modcache, "off", "-modcacherw -mod=mod"), "list", "-e", "-m", "-json", "all")
	}
	if err != nil {
		return nil, err
	}
	res := map[string]listMod{}
	dec := json.NewDecoder(bytes.NewReader(b))
	for dec.More() {
		var m listMod
		if err := dec.Decode(&m); err != nil {
			return nil, err
		}
		res[m.Path] = m
	}
	return res, nil
}

// ── the independent line scan ─────────────────────────────────────────────

type directive struct {
	verb   string
	tokens []string
	line   int
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '`') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// stripComment removes a // comment that is not inside a quoted string.
func stripComment(s string) string {
	inQ := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQ != 0:
			if c == inQ {
				inQ = 0
			}
		case c == '"' || c == '`':
			inQ = c
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			return s[:i]
		}
	}
	return s
}

// scan lists a file's directives in order, block entries flattened, with the
// 1-based line each begins on. It is its own tokenizer: whitespace-separated
// words, quotes stripped, `//` comments dropped, `verb (` opening a block.
func scan(src string) []directive {
	var out []directive
	block := ""
	for i, raw := range strings.Split(src, "\n") {
		toks := strings.Fields(stripComment(raw))
		if len(toks) == 0 {
			continue
		}
		for j := range toks {
			toks[j] = unquote(toks[j])
		}
		if block != "" {
			if toks[0] == ")" {
				block = ""
				continue
			}
			out = append(out, directive{block, toks, i + 1})
			continue
		}
		if len(toks) >= 2 && toks[1] == "(" && len(toks) == 2 {
			block = toks[0]
			continue
		}
		if len(toks) == 1 && strings.HasSuffix(toks[0], "(") && len(toks[0]) > 1 {
			block = strings.TrimSuffix(toks[0], "(")
			continue
		}
		out = append(out, directive{toks[0], toks[1:], i + 1})
	}
	return out
}

func lines(ds []directive, verb string) []directive {
	var out []directive
	for _, d := range ds {
		if d.verb == verb {
			out = append(out, d)
		}
	}
	return out
}

// firstLine is the first directive's line: the anchor of a go.work's own entity.
func firstLine(ds []directive) int {
	m := 0
	for _, d := range ds {
		if m == 0 || d.line < m {
			m = d.line
		}
	}
	return m
}

func replaceName(old modVersion) string {
	n := "replace/" + old.Path
	if old.Version != "" {
		n += "@" + old.Version
	}
	return n
}

// replaceOld reads the left-hand side of a scanned replace: its path and, when
// written, its version.
func replaceOld(d directive) modVersion {
	v := modVersion{Path: d.tokens[0]}
	if len(d.tokens) > 1 && d.tokens[1] != "=>" {
		v.Version = d.tokens[1]
	}
	return v
}

// ── the oracle ────────────────────────────────────────────────────────────

type fileInfo struct {
	rel, dir string // subject-relative slash path, absolute directory
	isWork   bool
	mod      modEdit
	work     workEdit
	parsed   bool
	src      string
}

type counts struct {
	files, parsed, parseRejected, scanMismatch int
	evalModule, evalWorkspace, evalRejected    int
	evalWorkFiles, unresolved                  int
	requires, requiresNotInBuild               int
	replaces, replacesLocal, replacesInactive  int
	uses, usesNotMain                          int
	requireEdges, replaceEdges, useEdges       int
	notes                                      []string
}

func evalDir(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	goBin := flag.String("go", "go", "the go binary")
	modcache := flag.String("modcache", "", "GOMODCACHE for the runs, outside the subject (required; scratch)")
	flag.Parse()
	if *subject == "" || *modcache == "" {
		fmt.Fprintln(os.Stderr, "usage: go-mod-oracle -subject <checkout> -modcache <scratch dir> [-go <go>]")
		os.Exit(2)
	}
	abs, err := filepath.Abs(*subject)
	if err != nil {
		fatal(err)
	}
	subj := evalDir(abs)
	mc, err := filepath.Abs(*modcache)
	if err != nil {
		fatal(err)
	}
	ctx := context.Background()

	ver, _ := run(ctx, *goBin, subj, goEnv(mc, "off", ""), "version")
	var files []*fileInfo
	skip := map[string]bool{".git": true, "vendor": true, "testdata": true, "node_modules": true, ".idea": true, ".vscode": true, ".claude": true, "generated": true}
	err = filepath.WalkDir(subj, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != subj && skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" && d.Name() != "go.work" {
			return nil
		}
		r, _ := filepath.Rel(subj, p)
		files = append(files, &fileInfo{rel: filepath.ToSlash(r), dir: filepath.Dir(p), isWork: d.Name() == "go.work"})
		return nil
	})
	if err != nil {
		fatal(err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })

	var c counts
	o := out{Tool: "go mod edit -json, go work edit -json, go list -e -m -json all", Version: strings.TrimPrefix(strings.TrimSpace(string(ver)), "go version "), Tier: "A",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}}
	byRel := map[string]*fileInfo{}

	// Pass 1: what each file declares, by the command's own parse.
	for _, f := range files {
		c.files++
		b, err := os.ReadFile(filepath.Join(subj, filepath.FromSlash(f.rel)))
		if err != nil {
			fatal(err)
		}
		f.src = string(b)
		var raw []byte
		if f.isWork {
			raw, err = run(ctx, *goBin, f.dir, goEnv(mc, "off", ""), "work", "edit", "-json", filepath.Join(f.dir, "go.work"))
		} else {
			raw, err = run(ctx, *goBin, f.dir, goEnv(mc, "off", ""), "mod", "edit", "-json", filepath.Join(f.dir, "go.mod"))
		}
		if err == nil {
			if f.isWork {
				err = json.Unmarshal(raw, &f.work)
			} else {
				err = json.Unmarshal(raw, &f.mod)
			}
		}
		if err != nil {
			c.parseRejected++
			c.notes = append(c.notes, f.rel+": the tool rejected the file: "+firstLineOf(err.Error()))
			continue
		}
		f.parsed = true
		c.parsed++
		byRel[f.rel] = f
	}

	// Definitions and references, with the scan's positions.
	for _, f := range files {
		if !f.parsed {
			continue
		}
		ds := scan(f.src)
		if f.isWork {
			uses, reps := lines(ds, "use"), lines(ds, "replace")
			if len(uses) != len(f.work.Use) || len(reps) != len(f.work.Replace) {
				c.scanMismatch++
				c.notes = append(c.notes, f.rel+": the scan found a different count of use/replace directives than the tool")
				f.parsed = false
				continue
			}
			o.Definitions = append(o.Definitions, def{f.rel, "workspace", firstLine(ds), "namespace"})
			for i, u := range f.work.Use {
				if uses[i].tokens[0] != u.DiskPath {
					c.scanMismatch++
				}
				o.Definitions = append(o.Definitions, def{f.rel, "use/" + u.DiskPath, uses[i].line, "key"})
				c.uses++
			}
			for i, r := range f.work.Replace {
				o.Definitions = append(o.Definitions, def{f.rel, replaceName(r.Old), reps[i].line, "key"})
				c.replaces++
			}
			continue
		}
		mods, reqs, reps := lines(ds, "module"), lines(ds, "require"), lines(ds, "replace")
		if len(mods) != 1 || len(reqs) != len(f.mod.Require) || len(reps) != len(f.mod.Replace) {
			c.scanMismatch++
			c.notes = append(c.notes, f.rel+": the scan found a different count of module/require/replace directives than the tool")
			f.parsed = false
			continue
		}
		mp := f.mod.Module.Path
		o.Definitions = append(o.Definitions, def{f.rel, mp, mods[0].line, "module"})
		seen := map[string]bool{}
		for i, r := range f.mod.Require {
			if reqs[i].tokens[0] != r.Path {
				c.scanMismatch++
			}
			o.Definitions = append(o.Definitions, def{f.rel, r.Path, reqs[i].line, "package"})
			c.requires++
			if !seen[r.Path] {
				seen[r.Path] = true
				o.References = append(o.References, ref{f.rel, mp, r.Path})
			}
		}
		for i, r := range f.mod.Replace {
			if got := replaceOld(reps[i]); got != r.Old {
				c.scanMismatch++
			}
			o.Definitions = append(o.Definitions, def{f.rel, replaceName(r.Old), reps[i].line, "key"})
			c.replaces++
		}
	}

	// Pass 2: what the declarations resolve to, by the command's evaluation.
	dirHasMod := map[string]*fileInfo{} // evaluated absolute dir -> its go.mod
	for _, f := range files {
		if f.parsed && !f.isWork {
			dirHasMod[evalDir(f.dir)] = f
		}
	}
	local := func(dir string) (*fileInfo, bool) { // a directory of the subject holding an accepted go.mod
		t, ok := dirHasMod[evalDir(dir)]
		return t, ok
	}
	seenX := map[xref]bool{}
	addX := func(x xref) {
		if !seenX[x] {
			seenX[x] = true
			o.CrossReferences = append(o.CrossReferences, x)
		}
	}
	for _, f := range files {
		if !f.parsed {
			continue
		}
		if f.isWork {
			ws, err := goList(ctx, *goBin, mc, f.dir, true)
			if err != nil {
				c.evalRejected++
				c.notes = append(c.notes, f.rel+": the tool could not evaluate the workspace: "+firstLineOf(err.Error()))
				continue
			}
			c.evalWorkFiles++
			mainDirs := map[string]bool{}
			for _, m := range ws {
				if m.Main && m.Dir != "" {
					mainDirs[evalDir(m.Dir)] = true
				}
			}
			for _, u := range f.work.Use {
				d := filepath.Join(f.dir, filepath.FromSlash(u.DiskPath))
				t, ok := local(d)
				if !ok {
					continue
				}
				if !mainDirs[evalDir(d)] {
					c.usesNotMain++
					continue
				}
				addX(xref{f.rel, "use/" + u.DiskPath, t.rel, t.mod.Module.Path})
				c.useEdges++
			}
			continue
		}
		mp := f.mod.Module.Path
		modL, modErr := goList(ctx, *goBin, mc, f.dir, false)
		var wsL map[string]listMod
		if b, err := run(ctx, *goBin, f.dir, goEnv(mc, "", ""), "env", "GOWORK"); err == nil {
			if w := strings.TrimSpace(string(b)); w != "" && w != "off" {
				wsL, _ = goList(ctx, *goBin, mc, f.dir, true)
			}
		}
		switch {
		case modErr == nil:
			c.evalModule++
			for _, m := range modL {
				if m.Error != nil {
					c.unresolved++
				}
			}
		case wsL != nil:
			c.evalWorkspace++
		default:
			c.evalRejected++
			c.notes = append(c.notes, f.rel+": the tool could not evaluate the module (no xrefs attested): "+firstLineOf(modErr.Error()))
			continue
		}
		// A requirement resolved to a directory of the subject: by this module's
		// own replace (module mode) or as a workspace module.
		for _, r := range f.mod.Require {
			inBuild := false
			var target string
			if m, ok := modL[r.Path]; ok {
				inBuild = true
				if m.Replace != nil && m.Replace.Dir != "" {
					target = m.Replace.Dir
				}
			}
			if m, ok := wsL[r.Path]; ok {
				inBuild = true
				if target == "" && m.Main && m.Dir != "" && r.Path != mp {
					target = m.Dir
				}
			}
			if !inBuild {
				c.requiresNotInBuild++
			}
			if target == "" {
				continue
			}
			if t, ok := local(target); ok && t.rel != f.rel {
				addX(xref{f.rel, mp, t.rel, t.mod.Module.Path})
				c.requireEdges++
			}
		}
		// A replace to a local directory that the tool evaluated as the
		// module's replacement.
		for _, r := range f.mod.Replace {
			if !isDirectoryPath(r.New.Path) {
				continue
			}
			c.replacesLocal++
			dir := filepath.Join(f.dir, filepath.FromSlash(strings.ReplaceAll(r.New.Path, `\`, "/")))
			t, ok := local(dir)
			if !ok {
				continue
			}
			active := false
			if m, ok := modL[r.Old.Path]; ok && m.Replace != nil && m.Replace.Dir != "" && evalDir(m.Replace.Dir) == evalDir(dir) {
				active = true
			}
			if m, ok := wsL[r.Old.Path]; ok && m.Main && evalDir(m.Dir) == evalDir(dir) {
				active = true // the workspace carries the module at that very directory
			}
			if !active {
				c.replacesInactive++
				continue
			}
			addX(xref{f.rel, replaceName(r.Old), t.rel, t.mod.Module.Path})
			c.replaceEdges++
		}
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
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "go-mod-oracle: %d files (%d parsed, %d rejected by the tool's parse, %d scan-mismatch)\n", c.files, c.parsed, c.parseRejected, c.scanMismatch)
	fmt.Fprintf(os.Stderr, "  evaluated: %d go.mod in module mode (%d modules of the build lists unresolved), %d go.mod in workspace mode only, %d go.work in workspace mode, %d not at all\n", c.evalModule, c.unresolved, c.evalWorkspace, c.evalWorkFiles, c.evalRejected)
	fmt.Fprintf(os.Stderr, "  requires %d (%d not in the build list), replaces %d (%d to a local directory, %d of them inactive), uses %d (%d not a workspace module)\n",
		c.requires, c.requiresNotInBuild, c.replaces, c.replacesLocal, c.replacesInactive, c.uses, c.usesNotMain)
	fmt.Fprintf(os.Stderr, "  cross_references: %d require, %d replace, %d use\n", c.requireEdges, c.replaceEdges, c.useEdges)
	for _, n := range c.notes {
		fmt.Fprintln(os.Stderr, "  note:", n)
	}
}

// isDirectoryPath is cmd/go's rule for a replace's right-hand side: relative
// and rooted paths are directories, the rest are module paths.
func isDirectoryPath(ns string) bool {
	return ns == "." || strings.HasPrefix(ns, "./") || strings.HasPrefix(ns, `.\`) ||
		ns == ".." || strings.HasPrefix(ns, "../") || strings.HasPrefix(ns, `..\`) ||
		strings.HasPrefix(ns, "/") || strings.HasPrefix(ns, `\`) ||
		len(ns) >= 2 && ('A' <= ns[0] && ns[0] <= 'Z' || 'a' <= ns[0] && ns[0] <= 'z') && ns[1] == ':'
}

func firstLineOf(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "go-mod-oracle:", err)
	os.Exit(1)
}
