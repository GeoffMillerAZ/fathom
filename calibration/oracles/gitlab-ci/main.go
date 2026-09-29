// Command gitlab-ci-oracle is the language-breadth harness's oracle for the
// gitlab-ci doc-engine dialect (design §1.3, owner decision D4). It writes
// indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10) in one
// of two modes:
//
//	go run . -subject <checkout>          # tier A: gitlab-ci-local --list-json
//	go run . -subject <checkout> -refs    # tier C: references and cross-file references
//
// Tier A. Each pipeline root (a .gitlab-ci.yml) is handed to the real tool,
// `gitlab-ci-local --list-json`, which loads the root and every include it can
// reach (local files from the checkout, remote templates, components and
// projects downloaded), merges them and answers which jobs the pipeline has,
// expanding parallel:matrix and parallel:N into one job per combination. The
// tool's JSON carries no source file and no line, so a yaml.v3 walk supplies
// them: the files of a pipeline are the root plus the local includes a walk of
// its `include:` reaches, a job's file is the file that declares it as a
// top-level key, its line the key's line, and a tool job name (`build:
// [go1.25]`, `test 1/3`) is folded back to the declared name. A name the tool
// lists that no file of the pipeline declares (a remote template's job) is
// dropped; a declared job the tool does not list stays out of the truth and
// counts against sym_precision, as it should. A pipeline the tool rejects
// contributes nothing and is counted on stderr; fathom still extracts what it
// can from such a file, which the reading scores as it stands. Hidden
// templates (`.name`) are not jobs, so the tool never lists them; the dialect
// emits them under the mixin kind, outside the oracle's kind intersection.
//
// Tier C (-refs). The tool evaluates a pipeline into its merged form, which
// erases `extends:` and inherits `needs:` through it, so the references the
// dialect models — what each job or template writes under `needs`,
// `dependencies` and `extends` — come from a yaml.v3 walk of the raw files,
// kept only for pipelines the tool accepted. Cross-file references are the
// subset whose target is declared in exactly one other file of the same
// pipeline (the rule fusion.ResolveDocXrefs applies to a service-name
// reference, so this oracle attests the implementation, not the rule).
// include:local and trigger:include are not emitted: they name a file, and a
// pipeline file has no root entity for the resolver to join them to.
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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
	CrossReferences []xref `json:"cross_references,omitempty"`
}

// reserved are the top-level keywords GitLab reserves; every other top-level
// key holding a map is a job. The dialect's doc.json carries the same list.
var reserved = map[string]bool{
	"default": true, "include": true, "stages": true, "variables": true, "workflow": true, "spec": true,
	"image": true, "services": true, "cache": true, "before_script": true, "after_script": true, "types": true,
}

var skipDirs = map[string]bool{".git": true, "vendor": true, "testdata": true, "node_modules": true, ".gitlab-ci-local": true, ".idea": true, ".vscode": true}

// get reads a map entry; a key written twice in one map reads as its last
// value, as Psych (GitLab's loader) and the doc engine read it.
func get(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var found *yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			found = deref(n.Content[i+1])
		}
	}
	return found
}

func deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

// items reads a field written as one value or a list: a scalar is its own
// single item, a sequence its items (the doc engine's `[*]`).
func items(n *yaml.Node) []*yaml.Node {
	n = deref(n)
	switch {
	case n == nil, n.Kind == yaml.ScalarNode && n.Tag == "!!null":
		return nil
	case n.Kind == yaml.SequenceNode:
		var out []*yaml.Node
		for _, it := range n.Content {
			out = append(out, deref(it))
		}
		return out
	}
	return []*yaml.Node{n}
}

// entry is one top-level map under an unreserved key of a pipeline file.
type entry struct {
	name   string
	line   int
	hidden bool
	node   *yaml.Node
}

// parseFile decodes every document of a YAML file (a component template has a
// `spec:` header document), returning their mapping roots.
func parseFile(abs string) ([]*yaml.Node, error) {
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var roots []*yaml.Node
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if len(roots) > 0 || errors.Is(err, io.EOF) {
				return roots, nil
			}
			return nil, err
		}
		if len(doc.Content) > 0 {
			if r := deref(doc.Content[0]); r.Kind == yaml.MappingNode {
				roots = append(roots, r)
			}
		}
	}
}

// declared lists the top-level maps of the documents under unreserved keys;
// a key written twice in one document is one entry, the last (Psych, the doc
// engine).
func declared(roots []*yaml.Node) []entry {
	type slot struct {
		e  entry
		ok bool
	}
	var out []entry
	for _, r := range roots {
		var order []string
		slots := map[string]*slot{}
		for i := 0; i+1 < len(r.Content); i += 2 {
			k, v := r.Content[i], deref(r.Content[i+1])
			sl := slots[k.Value]
			if sl == nil {
				sl = &slot{}
				slots[k.Value] = sl
				order = append(order, k.Value)
			}
			hidden := strings.HasPrefix(k.Value, ".")
			if v == nil || v.Kind != yaml.MappingNode || (!hidden && reserved[k.Value]) {
				sl.ok = false
				continue
			}
			sl.e, sl.ok = entry{k.Value, k.Line, hidden, v}, true
		}
		for _, n := range order {
			if slots[n].ok {
				out = append(out, slots[n].e)
			}
		}
	}
	return out
}

var urlRE = regexp.MustCompile(`^[a-z][a-z0-9+.-]*://`)

// localIncludes are the `include:` entries that name a file of the checkout: a
// bare path, or a `local:` map. GitLab resolves them from the repository root
// whatever the including file's directory.
func localIncludes(roots []*yaml.Node) []string {
	var out []string
	for _, r := range roots {
		for _, it := range items(get(r, "include")) {
			switch it.Kind {
			case yaml.ScalarNode:
				if s := scalar(it); s != "" && !urlRE.MatchString(s) {
					out = append(out, s)
				}
			case yaml.MappingNode:
				if s := scalar(get(it, "local")); s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

func globMatch(pattern, p string) bool {
	return globSegs(strings.Split(pattern, "/"), strings.Split(p, "/"))
}

func globSegs(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(segs); i++ {
				if globSegs(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], segs[0]); !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}

// pipeline is one root's project directory and the files its local includes
// reach (subject-relative, slash paths), root first.
type pipeline struct {
	dir   string // absolute project directory
	rel   string // its subject-relative path ("" for the subject root)
	root  string // the root file's subject-relative path
	files []string
}

func projectYAML(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && p != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if e := strings.ToLower(filepath.Ext(p)); e == ".yml" || e == ".yaml" {
			r, _ := filepath.Rel(dir, p)
			out = append(out, filepath.ToSlash(r))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func joinRel(rel, p string) string {
	if rel == "" {
		return p
	}
	return rel + "/" + p
}

func buildPipeline(subject, rootRel string) pipeline {
	dir := filepath.Dir(filepath.Join(subject, filepath.FromSlash(rootRel)))
	rel := path.Dir(rootRel)
	if rel == "." {
		rel = ""
	}
	p := pipeline{dir: dir, rel: rel, root: rootRel}
	all := projectYAML(dir)
	have := map[string]bool{}
	for _, f := range all {
		have[f] = true
	}
	seen := map[string]bool{}
	queue := []string{path.Base(rootRel)}
	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]
		if seen[f] {
			continue
		}
		seen[f] = true
		p.files = append(p.files, joinRel(rel, f))
		roots, err := parseFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		for _, pat := range localIncludes(roots) {
			pat = path.Clean(strings.TrimPrefix(pat, "/"))
			if strings.ContainsAny(pat, "*?[") {
				for _, c := range all {
					if globMatch(pat, c) {
						queue = append(queue, c)
					}
				}
			} else if have[pat] {
				queue = append(queue, pat)
			}
		}
	}
	return p
}

type toolJob struct {
	Name  string `json:"name"`
	Needs []struct {
		Job string `json:"job"`
	} `json:"needs"`
}

func toolVersion(bin string) string {
	b, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

// runTool lists a pipeline's jobs. The tool prints warnings to stdout before
// the JSON array, so the array is read from the first line that opens it. It
// downloads the remote includes a pipeline names (templates from gitlab.com's
// HEAD, components, projects) and caches them in its state directory, which
// lives outside the checkout (stateDir, an absolute path; the tool wants it
// relative to the project directory) so a later file discovery never reads a
// downloaded template and a re-run reuses the cache. A rate-limited or
// unreachable download is retried; a pipeline that fails for any other reason
// is the tool rejecting it.
func runTool(ctx context.Context, bin, dir, file, stateDir string) ([]toolJob, error) {
	rel, err := filepath.Rel(dir, stateDir)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 1; attempt <= 4; attempt++ {
		jobs, transient, err := runToolOnce(ctx, bin, dir, file, rel)
		if err == nil {
			return jobs, nil
		}
		lastErr = err
		if !transient {
			break
		}
		time.Sleep(time.Duration(attempt) * 30 * time.Second)
	}
	return nil, lastErr
}

func runToolOnce(ctx context.Context, bin, dir, file, stateRel string) ([]toolJob, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--file", file, "--state-dir", stateRel, "--list-json")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		transient := strings.Contains(msg, "429") || strings.Contains(msg, "ENOTFOUND") || strings.Contains(msg, "ETIMEDOUT") || strings.Contains(msg, "ECONNRESET") || strings.Contains(msg, "status code 5")
		if i := strings.LastIndex(msg, "\n"); i >= 0 {
			msg = msg[i+1:]
		}
		return nil, transient, fmt.Errorf("%v: %s", err, msg)
	}
	i := bytes.Index(b, []byte("\n["))
	switch {
	case bytes.HasPrefix(b, []byte("[")):
		i = 0
	case i >= 0:
		i++
	default:
		return nil, false, fmt.Errorf("no JSON array in the tool's output")
	}
	var jobs []toolJob
	if err := json.Unmarshal(b[i:], &jobs); err != nil {
		return nil, false, err
	}
	return jobs, false, nil
}

var (
	matrixSuffix   = regexp.MustCompile(`^(.+): \[.*\]$`)
	parallelSuffix = regexp.MustCompile(`^(.+) \d+/\d+$`)
)

// baseNames folds the tool's job names back to declared names: a matrix job is
// `name: [values]`, a parallel:N job `name 1/N`.
func baseNames(jobs []toolJob) map[string]bool {
	out := map[string]bool{}
	for _, j := range jobs {
		out[j.Name] = true
		if m := matrixSuffix.FindStringSubmatch(j.Name); m != nil {
			out[m[1]] = true
		}
		if m := parallelSuffix.FindStringSubmatch(j.Name); m != nil {
			out[m[1]] = true
		}
	}
	return out
}

// targets are the names a job or template writes under edge.
func targets(node *yaml.Node, edge string) []string {
	var out []string
	for _, it := range items(get(node, edge)) {
		switch {
		case it.Kind == yaml.ScalarNode:
			if s := scalar(it); s != "" {
				out = append(out, s)
			}
		case it.Kind == yaml.MappingNode && edge == "needs":
			if s := scalar(get(it, "job")); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	refsMode := flag.Bool("refs", false, "emit the tier-C references and cross-file references instead of the tier-A tool answer")
	toolBin := flag.String("tool", "gitlab-ci-local", "the gitlab-ci-local binary")
	stateBase := flag.String("state", "", "directory for the tool's state and download cache, outside the subject (default: a directory under the system temp dir)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: gitlab-ci-oracle -subject <dir> [-refs] [-tool <gitlab-ci-local>]")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bin, err := exec.LookPath(*toolBin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitlab-ci-oracle: gitlab-ci-local not found:", err)
		os.Exit(1)
	}
	version := "gitlab-ci-local " + toolVersion(bin)
	if *stateBase == "" {
		*stateBase = filepath.Join(os.TempDir(), "gitlab-ci-oracle-state")
	}
	*stateBase, err = filepath.Abs(*stateBase)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var roots []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == ".gitlab-ci.yml" || d.Name() == ".gitlab-ci.yaml" {
			r, _ := filepath.Rel(root, p)
			roots = append(roots, filepath.ToSlash(r))
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitlab-ci-oracle:", err)
		os.Exit(1)
	}
	sort.Strings(roots)

	o := out{Definitions: []def{}, References: []ref{}}
	if *refsMode {
		o.Tool, o.Version, o.Tier = "gitlab-ci-refs-oracle (independent yaml.v3 walk of needs, dependencies and extends, over the pipelines gitlab-ci-local accepts)", "gopkg.in/yaml.v3 v3.0.1", "C"
		o.CrossReferences = []xref{}
	} else {
		o.Tool, o.Version, o.Tier = "gitlab-ci-local --list-json", version, "A"
	}

	ctx := context.Background()
	var accepted, rejected, files int
	var writtenNotEvaluated, inherited int
	defSeen := map[def]bool{}
	refSeen := map[ref]bool{}
	xrefSeen := map[xref]bool{}
	for _, rootRel := range roots {
		pl := buildPipeline(root, rootRel)
		jobs, err := runTool(ctx, bin, pl.dir, path.Base(rootRel), filepath.Join(*stateBase, strings.ReplaceAll(filepath.Base(root)+"-"+rootRel, "/", "_")))
		if err != nil {
			rejected++
			fmt.Fprintf(os.Stderr, "rejected by the tool: %s: %v\n", rootRel, err)
			continue
		}
		accepted++
		names := baseNames(jobs)
		toolNeeds := map[string]map[string]bool{}
		for _, j := range jobs {
			base := j.Name
			if m := matrixSuffix.FindStringSubmatch(j.Name); m != nil {
				base = m[1]
			} else if m := parallelSuffix.FindStringSubmatch(j.Name); m != nil {
				base = m[1]
			}
			if toolNeeds[base] == nil {
				toolNeeds[base] = map[string]bool{}
			}
			for _, n := range j.Needs {
				toolNeeds[base][n.Job] = true
			}
		}

		type ent struct {
			file string
			entry
		}
		var ents []ent
		declFiles := map[string]map[string]bool{}
		for _, f := range pl.files {
			rs, err := parseFile(filepath.Join(root, filepath.FromSlash(f)))
			if err != nil {
				continue
			}
			files++
			for _, e := range declared(rs) {
				ents = append(ents, ent{f, e})
				if declFiles[e.name] == nil {
					declFiles[e.name] = map[string]bool{}
				}
				declFiles[e.name][f] = true
			}
		}
		for _, e := range ents {
			if *refsMode {
				for _, edge := range []string{"needs", "dependencies", "extends"} {
					for _, t := range targets(e.node, edge) {
						if t == e.name {
							continue
						}
						r := ref{e.file, e.name, t}
						if !refSeen[r] {
							refSeen[r] = true
							o.References = append(o.References, r)
						}
						var others []string
						for f := range declFiles[t] {
							if f != e.file {
								others = append(others, f)
							}
						}
						if len(others) == 1 {
							x := xref{e.file, e.name, others[0], t}
							if !xrefSeen[x] {
								xrefSeen[x] = true
								o.CrossReferences = append(o.CrossReferences, x)
							}
						}
					}
				}
				continue
			}
			if e.hidden || !names[e.name] {
				continue
			}
			d := def{e.file, e.name, e.line, "job"}
			if !defSeen[d] {
				defSeen[d] = true
				o.Definitions = append(o.Definitions, d)
			}
			written := map[string]bool{}
			for _, t := range targets(e.node, "needs") {
				written[t] = true
				if !toolNeeds[e.name][t] {
					writtenNotEvaluated++
				}
			}
			for t := range toolNeeds[e.name] {
				if !written[t] {
					inherited++
				}
			}
		}
	}
	sort.Slice(o.Definitions, func(i, j int) bool {
		a, b := o.Definitions[i], o.Definitions[j]
		return a.Path+"\x00"+fmt.Sprintf("%08d", a.Line)+a.Name < b.Path+"\x00"+fmt.Sprintf("%08d", b.Line)+b.Name
	})
	sort.Slice(o.References, func(i, j int) bool {
		a, b := o.References[i], o.References[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	fmt.Fprintf(os.Stderr, "gitlab-ci-oracle: %d pipelines accepted by the tool, %d rejected; %d files walked; %d definitions, %d references, %d cross-file references\n",
		accepted, rejected, files, len(o.Definitions), len(o.References), len(o.CrossReferences))
	if !*refsMode {
		fmt.Fprintf(os.Stderr, "gitlab-ci-oracle: needs the tool evaluated that the job does not write itself (inherited through extends or a merge key): %d; needs a job writes that the tool did not list: %d\n", inherited, writtenNotEvaluated)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
	}
}
