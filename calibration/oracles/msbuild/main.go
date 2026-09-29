// Command msbuild-oracle is the language-breadth harness's oracle for the msbuild
// doc-engine dialect (design §1.3, owner decision D4). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10), tier A:
//
//	go run . -subject <checkout> [-under <dir>] [-dotnet <dotnet>] [-scratch <dir>] [-j 2]
//
// The tool is MSBuild itself, run through the .NET SDK's `dotnet msbuild` on every
// project file (.csproj, .fsproj, .vbproj) under -under. MSBuild evaluates a project
// (its Sdk, the imports it pulls in, every condition and property) and reports each
// item with the file that defined it, so what a file itself declares is the tool's
// word and not a decode of the raw XML:
//
//   - definitions: one `package` per project file the tool evaluated, and one per
//     file inside the subject that an evaluation imported (a Directory.Build.props,
//     an eng/ props or targets file), at its <Project> start tag. The imports are
//     read from `-preprocess`, which brackets each imported file between an
//     `<Import …>` and a `</Import>` marker naming it.
//   - references: (file, package id) for every PackageReference item the tool lists
//     whose defining file is a file of the subject, whichever project evaluated it.
//     A file that only imports another's items (a props with no ItemGroup of its
//     own) has none.
//   - cross_references, entity granularity: a ProjectReference item, from the file
//     that defined it, to the project file MSBuild resolves it to (the item's
//     FullPath: properties expanded, separators normalised) when that file is one of
//     the subject's own.
//
// One evaluation sees one configuration and one target framework, and a condition
// that is false there hides its items. Each project is therefore evaluated once per
// configuration (-configs, default Debug and Release), for the default target
// framework and for every framework its TargetFrameworks names, and the items are the
// union; the imports are read from the Debug evaluations. A condition on anything else
// (an environment variable, a property a caller passes, a file that does not exist)
// stays false, and the file's declaration it hides shows up as a dialect-only
// reference; the independent decode below counts them (`declared-not-in-tool`).
//
// A project the tool cannot evaluate (an SDK it cannot resolve, a missing import)
// is listed on stderr and left out: its file is then a dialect-only definition.
//
// Every project file's PackageReference and ProjectReference entries are also read
// with an independent `encoding/xml` decode (the dialect's own walk over the doc
// engine is not reused) and compared with what the tool listed: `declared-not-in-tool`
// counts what a file writes that no evaluation listed, `tool-not-declared` what the
// tool lists that no file of the subject writes there (both printed on stderr, not
// scored).
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
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

// ── running the tool ──────────────────────────────────────────────────────

type runner struct {
	dotnet  string
	scratch string
	subject string
	timeout time.Duration
}

func (r runner) run(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	full := append([]string{"-n", "10", r.dotnet, "msbuild", "-nologo", "-nodeReuse:false", "-verbosity:quiet"}, args...)
	cmd := exec.CommandContext(ctx, "nice", full...)
	cmd.Dir = r.scratch
	cmd.Env = append(os.Environ(), "DOTNET_CLI_USE_MSBUILD_SERVER=0", "MSBUILDDISABLENODEREUSE=1", "DOTNET_CLI_TELEMETRY_OPTOUT=1", "DOTNET_NOLOGO=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if len(msg) > 400 {
			msg = msg[:400]
		}
		return nil, fmt.Errorf("%v: %s", err, msg)
	}
	return stdout.Bytes(), nil
}

type evalItem struct {
	Identity            string `json:"Identity"`
	FullPath            string `json:"FullPath"`
	DefiningProjectFull string `json:"DefiningProjectFullPath"`
}

type evalResult struct {
	Properties map[string]string     `json:"Properties"`
	Items      map[string][]evalItem `json:"Items"`
}

// evaluate is one `dotnet msbuild -getItem -getProperty` evaluation of a project.
func (r runner) evaluate(project string, props map[string]string) (evalResult, error) {
	args := []string{project, "-getProperty:TargetFramework,TargetFrameworks", "-getItem:PackageReference,ProjectReference"}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-p:"+k+"="+props[k])
	}
	b, err := r.run(args...)
	if err != nil {
		return evalResult{}, err
	}
	if i := bytes.IndexByte(b, '{'); i > 0 {
		b = b[i:]
	}
	var res evalResult
	if err := json.Unmarshal(b, &res); err != nil {
		return evalResult{}, fmt.Errorf("decode evaluation of %s: %w", project, err)
	}
	return res, nil
}

var markerRe = regexp.MustCompile(`(?ms)^={20,}\n(.*?)\n={20,}\n\s*-->`)

var targetRe = regexp.MustCompile(`<Target\s+Name="([^"]+)"`)

// imports reads `-preprocess` output: the first marker names the project, then each
// `<Import …>` marker names the file it opens and each `</Import>` marker the
// importing file it returns to. It returns every file that appeared and the name of
// every target the preprocessed project holds (the tool's own view of the targets
// its files declare, used to cross-check them, not to score them).
func (r runner) imports(project string, props map[string]string, sink string) (files, targets []string, err error) {
	args := []string{project, "-preprocess:" + sink}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-p:"+k+"="+props[k])
	}
	if _, err := r.run(args...); err != nil {
		return nil, nil, err
	}
	defer os.Remove(sink)
	b, err := os.ReadFile(sink)
	if err != nil {
		return nil, nil, err
	}
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	for _, m := range targetRe.FindAllSubmatch(b, -1) {
		targets = append(targets, string(m[1]))
	}
	seen := map[string]bool{}
	for _, m := range markerRe.FindAllSubmatch(b, -1) {
		body := string(m[1])
		lines := strings.Split(body, "\n")
		last := strings.TrimSpace(lines[len(lines)-1])
		if strings.HasPrefix(last, "/") && !strings.HasPrefix(last, "</") {
			seen[last] = true
		}
	}
	files = make([]string, 0, len(seen))
	for p := range seen {
		files = append(files, p)
	}
	sort.Strings(files)
	return files, targets, nil
}

// ── the subject ───────────────────────────────────────────────────────────

var projectExts = map[string]bool{".csproj": true, ".fsproj": true, ".vbproj": true}
var msbuildExts = map[string]bool{".csproj": true, ".fsproj": true, ".vbproj": true, ".props": true, ".targets": true}

func walk(root, under string, exts map[string]bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(filepath.Join(root, under), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if exts[strings.ToLower(filepath.Ext(p))] {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// projectLine is the 1-based line of a file's root start tag, found with a
// tolerant token scan (comments and declarations before it do not count).
func projectLine(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 1
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	dec.Strict = false
	for {
		off := int(dec.InputOffset())
		tok, err := dec.Token()
		if err != nil {
			return 1
		}
		if _, ok := tok.(xml.StartElement); ok {
			for off < len(b) && b[off] != '<' {
				off++
			}
			return 1 + bytes.Count(b[:off], []byte{'\n'})
		}
	}
}

// declared is the independent decode: the PackageReference and ProjectReference
// entries a file writes as children of an ItemGroup that is a child of the root.
type declared struct {
	packages, projects, targets []string
}

func declaredItems(path string) declared {
	b, err := os.ReadFile(path)
	if err != nil {
		return declared{}
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	dec.Strict = false
	var d declared
	var stack []string
	for {
		tok, err := dec.Token()
		if err == io.EOF || err != nil {
			return d
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) == 2 && stack[1] == "ItemGroup" {
				inc := ""
				for _, a := range t.Attr {
					if a.Name.Local == "Include" {
						inc = a.Value
					}
				}
				if inc != "" {
					switch t.Name.Local {
					case "PackageReference":
						d.packages = append(d.packages, inc)
					case "ProjectReference":
						d.projects = append(d.projects, inc)
					}
				}
			}
			if len(stack) == 1 && t.Name.Local == "Target" {
				for _, a := range t.Attr {
					if a.Name.Local == "Name" && a.Value != "" {
						d.targets = append(d.targets, a.Value)
					}
				}
			}
			stack = append(stack, t.Name.Local)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
}

// ── main ──────────────────────────────────────────────────────────────────

func main() {
	subject := flag.String("subject", "", "the checkout to read (required)")
	under := flag.String("under", ".", "a directory of the subject whose project files are evaluated")
	dotnet := flag.String("dotnet", "dotnet", "the dotnet muxer")
	scratch := flag.String("scratch", "", "a scratch directory outside the subject (default: a temporary one)")
	configs := flag.String("configs", "Debug,Release", "configurations to evaluate each project under")
	jobs := flag.Int("j", 2, "projects evaluated at once")
	timeout := flag.Duration("timeout", 10*time.Minute, "per evaluation")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "msbuild-oracle: -subject is required")
		os.Exit(2)
	}
	root, err := filepath.EvalSymlinks(*subject)
	if err != nil {
		fatal(err)
	}
	if *scratch == "" {
		d, err := os.MkdirTemp("", "msbuild-oracle-")
		if err != nil {
			fatal(err)
		}
		defer os.RemoveAll(d)
		*scratch = d
	}
	if err := os.MkdirAll(*scratch, 0o755); err != nil {
		fatal(err)
	}
	r := runner{dotnet: *dotnet, scratch: *scratch, subject: root, timeout: *timeout}

	rel := func(p string) (string, bool) {
		x, err := filepath.Rel(root, p)
		if err != nil || strings.HasPrefix(x, "..") {
			return "", false
		}
		return filepath.ToSlash(x), true
	}
	exists := func(p string) bool {
		st, err := os.Stat(p)
		return err == nil && !st.IsDir()
	}

	projects, err := walk(root, *under, projectExts)
	if err != nil {
		fatal(err)
	}
	if len(projects) == 0 {
		fatal(fmt.Errorf("no project file under %s", filepath.Join(root, *under)))
	}
	cfgs := strings.Split(*configs, ",")

	type projectResult struct {
		project  string
		err      error
		packages map[[2]string]bool // defining file, package id
		refs     map[[2]string]bool // defining file, resolved project
		imported map[string]bool
		targets  map[string]bool
		evals    int
	}
	results := make([]*projectResult, len(projects))
	var wg sync.WaitGroup
	sem := make(chan struct{}, *jobs)
	for i, p := range projects {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, p string) {
			defer wg.Done()
			defer func() { <-sem }()
			pr := &projectResult{project: p, packages: map[[2]string]bool{}, refs: map[[2]string]bool{}, imported: map[string]bool{}, targets: map[string]bool{}}
			results[i] = pr
			absorb := func(res evalResult) {
				for _, it := range res.Items["PackageReference"] {
					pr.packages[[2]string{it.DefiningProjectFull, it.Identity}] = true
				}
				for _, it := range res.Items["ProjectReference"] {
					pr.refs[[2]string{it.DefiningProjectFull, it.FullPath}] = true
				}
			}
			for ci, cfg := range cfgs {
				base := map[string]string{"Configuration": cfg}
				first, err := r.evaluate(p, base)
				if err != nil {
					if ci == 0 {
						pr.err = err
						return
					}
					continue
				}
				pr.evals++
				absorb(first)
				tfs := map[string]bool{}
				for _, tf := range strings.Split(first.Properties["TargetFrameworks"], ";") {
					if tf = strings.TrimSpace(tf); tf != "" {
						tfs[tf] = true
					}
				}
				delete(tfs, first.Properties["TargetFramework"])
				names := make([]string, 0, len(tfs))
				for tf := range tfs {
					names = append(names, tf)
				}
				sort.Strings(names)
				variants := []map[string]string{nil}
				for _, tf := range names {
					variants = append(variants, map[string]string{"TargetFramework": tf})
				}
				for vi, v := range variants {
					props := map[string]string{"Configuration": cfg}
					for k, val := range v {
						props[k] = val
					}
					if vi > 0 {
						res, err := r.evaluate(p, props)
						if err != nil {
							continue
						}
						pr.evals++
						absorb(res)
					}
					if ci == 0 {
						sink := filepath.Join(*scratch, fmt.Sprintf("pp-%d-%d.xml", i, vi))
						files, targets, err := r.imports(p, props, sink)
						if err == nil {
							for _, f := range files {
								pr.imported[f] = true
							}
							for _, t := range targets {
								pr.targets[t] = true
							}
						}
					}
				}
			}
		}(i, p)
	}
	wg.Wait()

	// Merge.
	evaluated := map[string]bool{}
	imported := map[string]bool{}
	toolTargets := map[string]bool{}
	packages := map[[2]string]bool{}
	refs := map[[2]string]bool{}
	var failed []string
	evals := 0
	for _, pr := range results {
		if pr.err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", pr.project, pr.err))
			continue
		}
		evals += pr.evals
		evaluated[pr.project] = true
		for f := range pr.imported {
			imported[f] = true
		}
		for t := range pr.targets {
			toolTargets[t] = true
		}
		for k := range pr.packages {
			packages[k] = true
		}
		for k := range pr.refs {
			refs[k] = true
		}
	}

	o := out{Tool: "dotnet msbuild (-getItem, -getProperty and -preprocess)", Tier: "A",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}}
	if v, err := r.run("-version"); err == nil {
		lines := strings.Split(strings.TrimSpace(string(v)), "\n")
		o.Version = strings.TrimSpace(lines[len(lines)-1])
	}
	if b, err := exec.Command(*dotnet, "--version").Output(); err == nil {
		o.Version = "MSBuild " + o.Version + "; .NET SDK " + strings.TrimSpace(string(b))
	}

	defFiles := map[string]bool{}
	for p := range evaluated {
		defFiles[p] = true
	}
	for f := range imported {
		if _, ok := rel(f); ok && exists(f) && msbuildExts[strings.ToLower(filepath.Ext(f))] {
			defFiles[f] = true
		}
	}
	defList := make([]string, 0, len(defFiles))
	for f := range defFiles {
		defList = append(defList, f)
	}
	sort.Strings(defList)
	for _, f := range defList {
		rp, ok := rel(f)
		if !ok {
			continue
		}
		o.Definitions = append(o.Definitions, def{Path: rp, Name: filepath.Base(f), Line: projectLine(f), Kind: "package"})
	}

	toolPkgs := map[string]map[string]bool{}
	for k := range packages {
		rp, ok := rel(k[0])
		if !ok || !exists(k[0]) || k[1] == "" {
			continue
		}
		o.References = append(o.References, ref{Path: rp, From: filepath.Base(k[0]), To: k[1]})
		if toolPkgs[rp] == nil {
			toolPkgs[rp] = map[string]bool{}
		}
		toolPkgs[rp][k[1]] = true
	}
	sort.Slice(o.References, func(i, j int) bool {
		a, b := o.References[i], o.References[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.To < b.To
	})
	for k := range refs {
		from, ok1 := rel(k[0])
		to, ok2 := rel(k[1])
		if !ok1 || !ok2 || !exists(k[0]) || !exists(k[1]) || !msbuildExts[strings.ToLower(filepath.Ext(k[1]))] {
			continue
		}
		o.CrossReferences = append(o.CrossReferences, xref{Path: from, From: filepath.Base(k[0]), ToPath: to, To: filepath.Base(k[1])})
	}
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.ToPath < b.ToPath
	})

	// The independent decode and its cross-check.
	all, _ := walk(root, *under, msbuildExts)
	declNotTool, toolNotDecl := 0, 0
	var examples []string
	declaredPkgs := map[string]map[string]bool{}
	targetsDeclared, targetsInTool := 0, 0
	for _, f := range all {
		rp, _ := rel(f)
		d := declaredItems(f)
		if defFiles[f] {
			for _, t := range d.targets {
				targetsDeclared++
				if toolTargets[t] {
					targetsInTool++
				}
			}
		}
		set := map[string]bool{}
		for _, id := range d.packages {
			set[id] = true
			if !toolPkgs[rp][id] && defFiles[f] {
				declNotTool++
				if len(examples) < 8 {
					examples = append(examples, rp+" declares "+id)
				}
			}
		}
		declaredPkgs[rp] = set
	}
	for rp, ids := range toolPkgs {
		if _, walked := declaredPkgs[rp]; !walked {
			continue // a file outside -under: the decode did not read it
		}
		for id := range ids {
			if !declaredPkgs[rp][id] {
				toolNotDecl++
			}
		}
	}
	unattested := 0
	for _, f := range all {
		if !defFiles[f] {
			unattested++
		}
	}
	fmt.Fprintf(os.Stderr, "msbuild-oracle: %d projects (%d evaluated, %d failed), %d evaluations, %d in-subject imports\n",
		len(projects), len(evaluated), len(failed), evals, len(defList)-len(evaluated))
	fmt.Fprintf(os.Stderr, "msbuild-oracle: %d definitions, %d references, %d cross-references; %d msbuild files under %s are not attested (dialect-only definitions)\n",
		len(o.Definitions), len(o.References), len(o.CrossReferences), unattested, *under)
	fmt.Fprintf(os.Stderr, "msbuild-oracle: cross-check: declared-not-in-tool %d, tool-not-declared %d; targets declared %d, in the tool's preprocessed projects %d (not scored)\n", declNotTool, toolNotDecl, targetsDeclared, targetsInTool)
	for _, e := range examples {
		fmt.Fprintf(os.Stderr, "  declared-not-in-tool: %s\n", e)
	}
	for _, f := range failed {
		fmt.Fprintf(os.Stderr, "  failed: %s\n", f)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(o); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "msbuild-oracle:", err)
	os.Exit(1)
}
