// Command kustomize-oracle is the language-breadth harness's oracle for the
// kustomize doc-engine dialect (design §1.3, owner decision D4). It writes
// indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10), tier A:
//
//	go run . -subject <checkout> [-kubectl <kubectl>] [-yq <yq>] [-scratch <dir>] [-files-out <file>]
//
// The tool is Kustomize itself, run through `kubectl kustomize` (kubectl v1.36.1
// embeds Kustomize v5.8.1) on every kustomization.yaml and kustomization.yml of a
// scratch copy of the subject, each as the root of its own build. Kustomize lists
// nothing of what a kustomization declares, so the tool is asked one question of
// every entry, and the answer is its word:
//
//	does the build change when this entry is removed?
//
// Each entry a kustomization names (a resource, base, component or patch, an image,
// a configMapGenerator or secretGenerator) is proposed by a decode of the file
// (`yq`, an independent YAML reader with source lines: the dialect's own walk over
// the doc engine is not reused), then removed from a copy of the kustomization and the
// build run again with `buildMetadata: [originAnnotations]` set. The entry is the
// tool's when the build fails without it or its output differs; an entry whose
// removal changes nothing (an image that matches no container, a patch whose target
// matches nothing) is not evidenced, and it is neither in the truth nor hidden: it is
// counted on stderr (`unevidenced`), and the dialect that emits it pays in precision.
//
//   - definitions: one `package` per kustomization the tool built (at its first key's
//     line, named by the file), and one `resource` / `data_source` per evidenced
//     entry at its line: a resource, base, component, patch (its path), generator
//     (its name) is a `resource`, an image (its name) a `data_source`.
//   - references: the kustomization to every evidenced entry, by the name the entry
//     has as a definition.
//   - cross_references, file granularity: a resource, base, component or patch entry
//     that names a file of the subject (a directory names the kustomization it holds),
//     from the referring kustomization to that file. Those whose target is not a
//     kustomization file fall outside what the dialect reads and are dropped, and
//     counted, by lang-oracle (§1.4).
//
// A kustomization the tool cannot build (a remote base, which this oracle never
// fetches: the run sets a dead proxy and treats a reference that names no path of the
// subject as cross-repository; a Component, which is not a root; a broken
// overlay) contributes nothing and is left out of -files-out, counted on stderr with
// its reason. Measure with `lang-oracle -only "$(cat <files-out>)"`.
//
// Bench-only; never a product dependency. Standard library only; `kubectl` and `yq`
// are external programs.
package main

import (
	"bytes"
	"context"
	"encoding/json"
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
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}

// field is one list of a kustomization the oracle asks about.
type field struct {
	name string
	// key: the entry is a map and its name is this key ("" for a scalar entry).
	key string
	// kind is the oracle definition kind of its entries.
	kind string
	// paths: the entry names a file or directory of the subject.
	paths bool
}

var fields = []field{
	{"resources", "", "resource", true},
	{"bases", "", "resource", true},
	{"components", "", "resource", true},
	{"patchesStrategicMerge", "", "resource", true},
	{"patches", "path", "resource", true},
	{"patchesJson6902", "path", "resource", true},
	{"images", "name", "data_source", false},
	{"configMapGenerator", "name", "resource", false},
	{"secretGenerator", "name", "resource", false},
}

type entry struct {
	Field string `json:"-"`
	Index int    `json:"i"`
	Value string `json:"v"`
	Line  int    `json:"l"`
}

// kfile is one kustomization file of the subject.
type kfile struct {
	rel     string // subject-relative, slash-separated
	base    string
	line    int
	kind    string
	entries []entry
	// fieldOf is each entry's field, aligned with entries.
	fieldOf []field
}

// yqProgram is the one expression that reads a kustomization: its kind, its first
// key's line, and for each list the oracle asks about the entries that name
// something, at their index, value and line.
func yqProgram() string {
	var parts []string
	parts = append(parts, `"kind": (.kind // "")`, `"line": line`)
	for _, f := range fields {
		seq := fmt.Sprintf(`((.%s | select(tag == "!!seq")) // []) | to_entries | .[]`, f.name)
		var sel, val string
		if f.key == "" {
			sel = `select(.value | tag == "!!str")`
			val = `.value`
		} else {
			sel = fmt.Sprintf(`select(.value | tag == "!!map") | select(.value.%s | tag == "!!str")`, f.key)
			val = fmt.Sprintf(`.value.%s`, f.key)
		}
		parts = append(parts, fmt.Sprintf(`%q: [%s | %s | {"i": .key, "v": %s, "l": (.value | line)}]`, f.name, seq, sel, val))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

type runner struct {
	kubectl, yq string
	timeout     time.Duration
	env         []string
}

func (r runner) run(ctx context.Context, dir string, stdin []byte, name string, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = r.env
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.Bytes(), se.Bytes(), err
}

func (r runner) yqEval(ctx context.Context, expr string, src []byte) ([]byte, error) {
	so, se, err := r.run(ctx, "", src, r.yq, "-o=json", "-I=0", expr, "-")
	if err != nil {
		return nil, fmt.Errorf("yq: %v: %s", err, firstLine(se))
	}
	return so, nil
}

func (r runner) yqYAML(ctx context.Context, expr string, src []byte) ([]byte, error) {
	so, se, err := r.run(ctx, "", src, r.yq, expr, "-")
	if err != nil {
		return nil, fmt.Errorf("yq: %v: %s", err, firstLine(se))
	}
	return so, nil
}

func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "kustomize-oracle:", err)
	os.Exit(1)
}

// copyTree copies the subject's regular files, skipping .git.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
}

func findKustomizations(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "kustomization.yaml" || d.Name() == "kustomization.yml" {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

var remoteRE = regexp.MustCompile(`^(?:[a-z][a-z0-9+.-]*::|[a-z][a-z0-9+.-]*://|git@|github\.com[/:]|gitlab\.com[/:]|bitbucket\.org[/:])`)

func main() {
	subject := flag.String("subject", "", "the checkout to read (required)")
	kubectl := flag.String("kubectl", "kubectl", "kubectl (embeds Kustomize)")
	yq := flag.String("yq", "yq", "mikefarah yq v4 (the independent decode)")
	scratch := flag.String("scratch", "", "a scratch directory outside the subject (default: a temporary one)")
	filesOut := flag.String("files-out", "", "write the kustomization files the tool built, one per line, for lang-oracle -only")
	timeout := flag.Duration("timeout", 2*time.Minute, "per build")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "kustomize-oracle: -subject is required")
		os.Exit(2)
	}
	root, err := filepath.EvalSymlinks(*subject)
	if err != nil {
		fatal(err)
	}
	if *scratch == "" {
		d, err := os.MkdirTemp("", "kustomize-oracle-")
		if err != nil {
			fatal(err)
		}
		defer os.RemoveAll(d)
		*scratch = d
	}
	work := filepath.Join(*scratch, "subject")
	if err := os.RemoveAll(work); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		fatal(err)
	}
	if err := copyTree(root, work); err != nil {
		fatal(err)
	}
	// A dead proxy: a remote base or resource that reaches for the network fails at
	// once instead of being fetched, so no build depends on what a URL serves today.
	env := append(os.Environ(),
		"HTTP_PROXY=http://127.0.0.1:9", "HTTPS_PROXY=http://127.0.0.1:9", "ALL_PROXY=http://127.0.0.1:9",
		"http_proxy=http://127.0.0.1:9", "https_proxy=http://127.0.0.1:9", "all_proxy=http://127.0.0.1:9",
		"NO_PROXY=", "no_proxy=", "GIT_TERMINAL_PROMPT=0", "KUBECONFIG=/dev/null")
	r := runner{kubectl: *kubectl, yq: *yq, timeout: *timeout, env: env}
	ctx := context.Background()

	version := kustomizeVersion(ctx, r)
	kfiles, err := findKustomizations(work)
	if err != nil {
		fatal(err)
	}
	if len(kfiles) == 0 {
		fatal(fmt.Errorf("no kustomization.yaml or kustomization.yml under %s", root))
	}

	res := out{Tool: "kustomize (kubectl-embedded)", Version: version, Tier: "A",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}}
	stats := map[string]int{}
	excluded := map[string][]string{}
	var built []string
	fileSet := map[string]bool{}
	for _, k := range kfiles {
		fileSet[k] = true
	}
	prog := yqProgram()

	for _, rel := range kfiles {
		abs := filepath.Join(work, filepath.FromSlash(rel))
		orig, err := os.ReadFile(abs)
		if err != nil {
			fatal(err)
		}
		exclude := func(why string) {
			stats["excluded:"+why]++
			excluded[why] = append(excluded[why], rel)
		}
		kf, err := decode(ctx, r, prog, rel, orig)
		if err != nil {
			exclude("undecodable")
			continue
		}
		if kf.kind == "Component" {
			exclude("component (not a root the tool builds)")
			continue
		}
		dir := path.Dir(rel)
		remote, missing := false, false
		for i, e := range kf.entries {
			if !kf.fieldOf[i].paths {
				continue
			}
			if remoteRE.MatchString(e.Value) {
				remote = true
				stats["remote-entries"]++
				continue
			}
			if !pathExists(work, dir, e.Value) {
				missing = true
			}
		}
		if remote {
			exclude("remote base (cross-repository, never fetched)")
			continue
		}
		if missing {
			exclude("an entry names no path of the subject")
			continue
		}
		base, err := r.yqYAML(ctx, `.buildMetadata = (((.buildMetadata // []) + ["originAnnotations"]) | unique)`, orig)
		if err != nil {
			exclude("undecodable")
			continue
		}
		baseline, berr := build(ctx, r, work, rel, abs, base, orig)
		if berr != nil {
			exclude("the tool rejects it: " + berr.Error())
			continue
		}
		built = append(built, rel)
		stats["built"]++
		res.Definitions = append(res.Definitions, def{Path: rel, Name: kf.base, Line: kf.line, Kind: "package"})
		for i, e := range kf.entries {
			f := kf.fieldOf[i]
			stats["entries"]++
			variant, err := r.yqYAML(ctx, fmt.Sprintf(`del(.%s[%d])`, f.name, e.Index), base)
			if err != nil {
				fatal(err)
			}
			got, berr := build(ctx, r, work, rel, abs, variant, orig)
			if berr == nil && bytes.Equal(got, baseline) {
				stats["unevidenced"]++
				stats["unevidenced:"+f.name]++
				fmt.Fprintf(os.Stderr, "unevidenced: %s:%d %s %q (removing it changes nothing)\n", rel, e.Line, f.name, e.Value)
				continue
			}
			stats["evidenced"]++
			res.Definitions = append(res.Definitions, def{Path: rel, Name: e.Value, Line: e.Line, Kind: f.kind})
			res.References = append(res.References, ref{Path: rel, From: kf.base, To: e.Value})
			if !f.paths {
				continue
			}
			if to, ok := targetFile(work, dir, e.Value, fileSet); ok {
				res.CrossReferences = append(res.CrossReferences, xref{Path: rel, From: kf.base, ToPath: to})
				stats["xref"]++
			} else {
				stats["xref-not-a-kustomization"]++
			}
		}
		// leave the scratch copy as it was read
		if err := os.WriteFile(abs, orig, 0o644); err != nil {
			fatal(err)
		}
	}

	sort.Slice(res.Definitions, func(i, j int) bool {
		a, b := res.Definitions[i], res.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.Slice(res.References, func(i, j int) bool {
		a, b := res.References[i], res.References[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.To < b.To
	})
	sort.Slice(res.CrossReferences, func(i, j int) bool {
		a, b := res.CrossReferences[i], res.CrossReferences[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.ToPath < b.ToPath
	})
	if *filesOut != "" {
		sort.Strings(built)
		if err := os.WriteFile(*filesOut, []byte(strings.Join(built, "\n")+"\n"), 0o644); err != nil {
			fatal(err)
		}
	}
	summary(os.Stderr, len(kfiles), stats, excluded)
	b, err := json.MarshalIndent(res, "", " ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(b))
}

func summary(w io.Writer, total int, stats map[string]int, excluded map[string][]string) {
	fmt.Fprintf(w, "kustomizations: %d found, %d built by the tool\n", total, stats["built"])
	var whys []string
	for why := range excluded {
		whys = append(whys, why)
	}
	sort.Strings(whys)
	for _, why := range whys {
		fmt.Fprintf(w, "excluded (%d): %s: %s\n", len(excluded[why]), why, strings.Join(excluded[why], ", "))
	}
	fmt.Fprintf(w, "entries asked about: %d; evidenced by the tool: %d; unevidenced (removal changes nothing): %d\n",
		stats["entries"], stats["evidenced"], stats["unevidenced"])
	var uf []string
	for k, v := range stats {
		if strings.HasPrefix(k, "unevidenced:") {
			uf = append(uf, fmt.Sprintf("%s=%d", strings.TrimPrefix(k, "unevidenced:"), v))
		}
	}
	sort.Strings(uf)
	if len(uf) > 0 {
		fmt.Fprintf(w, "unevidenced by field: %s\n", strings.Join(uf, " "))
	}
	fmt.Fprintf(w, "cross-references: %d to a file of the subject (a kustomization, or a manifest the dialect does not read), %d evidenced path entries name no file of the subject the oracle can resolve\n",
		stats["xref"], stats["xref-not-a-kustomization"])
	fmt.Fprintf(w, "remote entries seen: %d\n", stats["remote-entries"])
}

// decode reads one kustomization with yq.
func decode(ctx context.Context, r runner, prog, rel string, src []byte) (kfile, error) {
	kf := kfile{rel: rel, base: path.Base(rel)}
	raw, err := r.yqEval(ctx, prog, src)
	if err != nil {
		return kf, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return kf, fmt.Errorf("yq output: %w", err)
	}
	if v, ok := doc["kind"]; ok {
		_ = json.Unmarshal(v, &kf.kind)
	}
	if v, ok := doc["line"]; ok {
		_ = json.Unmarshal(v, &kf.line)
	}
	// yq numbers a node's line from the first line of content: the comment, blank and
	// document-marker lines that open a file are not counted, so every line it reports is
	// short by that many. The offset is read from the source, not assumed.
	off := leadingLines(src)
	kf.line += off
	for _, f := range fields {
		var es []entry
		if v, ok := doc[f.name]; ok {
			if err := json.Unmarshal(v, &es); err != nil {
				return kf, fmt.Errorf("yq output %s: %w", f.name, err)
			}
		}
		for _, e := range es {
			e.Field = f.name
			e.Line += off
			kf.entries = append(kf.entries, e)
			kf.fieldOf = append(kf.fieldOf, f)
		}
	}
	return kf, nil
}

// leadingLines counts the lines before a file's first line of content: blank lines,
// comments, YAML directives and a document start marker.
func leadingLines(src []byte) int {
	n := 0
	for _, l := range strings.Split(string(src), "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "%") || t == "---" || strings.HasPrefix(t, "--- #") {
			n++
			continue
		}
		break
	}
	return n
}

// build runs the tool on the kustomization at abs after replacing its content with
// content, and puts orig back afterwards. A failure is an error carrying the tool's
// first line.
func build(ctx context.Context, r runner, work, rel, abs string, content, orig []byte) ([]byte, error) {
	if err := os.WriteFile(abs, content, 0o644); err != nil {
		fatal(err)
	}
	defer func() {
		if err := os.WriteFile(abs, orig, 0o644); err != nil {
			fatal(err)
		}
	}()
	so, se, err := r.run(ctx, "", nil, r.kubectl, "kustomize", filepath.Join(work, filepath.FromSlash(path.Dir(rel))))
	if err != nil {
		return nil, fmt.Errorf("%s", firstLine(se))
	}
	return so, nil
}

func pathExists(work, dir, value string) bool {
	_, err := os.Stat(filepath.Join(work, filepath.FromSlash(path.Join(dir, value))))
	return err == nil
}

// targetFile resolves an entry to the file of the subject it names: a file, or the
// kustomization a directory holds (Kustomize's own precedence, kustomization.yaml
// first). Only a file inside the subject counts.
func targetFile(work, dir, value string, kustomizations map[string]bool) (string, bool) {
	p := path.Clean(path.Join(dir, value))
	if p == "." || strings.HasPrefix(p, "../") || p == ".." {
		return "", false
	}
	st, err := os.Stat(filepath.Join(work, filepath.FromSlash(p)))
	if err != nil {
		return "", false
	}
	if !st.IsDir() {
		return p, true
	}
	for _, n := range []string{"kustomization.yaml", "kustomization.yml"} {
		if kustomizations[path.Join(p, n)] {
			return path.Join(p, n), true
		}
	}
	return "", false
}

var kustomizeVersionRE = regexp.MustCompile(`"kustomizeVersion":\s*"([^"]+)"`)

// kustomizeVersion is the Kustomize kubectl embeds.
func kustomizeVersion(ctx context.Context, r runner) string {
	so, _, err := r.run(ctx, "", nil, r.kubectl, "version", "--client", "-o", "json")
	if err != nil {
		return "unknown"
	}
	if m := kustomizeVersionRE.FindSubmatch(so); m != nil {
		return string(m[1])
	}
	return "unknown"
}
