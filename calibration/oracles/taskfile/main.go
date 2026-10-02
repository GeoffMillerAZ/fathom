// Command taskfile-oracle is the language-breadth harness's oracle for the
// taskfile doc-engine dialect (design §1.3, owner decision D4). It writes
// indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10) in two
// files from one run:
//
//	go run . -subject <checkout> [-task <task>] [-yq <yq>] [-scratch <dir>] \
//	    [-files-out <file>] [-refs-out <file>] > tier-a.json
//
// Tier A (stdout). The tool is go-task itself, `task --list-all --json
// --taskfile <file>` (go-task 3.52.0), run on every Taskfile of a scratch copy of the
// subject, each as the root of its own run. It evaluates the Taskfile (its version, its
// includes, its tasks and their names) and reports every task with the file, line and
// column of its key:
//
//   - definitions: one `package` per Taskfile the tool accepted (named by the file, at its
//     first key), and one `task` per task the tool lists whose location is that file,
//     named as the file writes it (the key at the tool's line and column: an included
//     task is `website:build` to the includer and `build` in website/Taskfile.yml, and
//     a key with a colon in it is not a namespace) and at the tool's line.
//     `--list-all` leaves an `internal: true` task out, so the run is made on a copy
//     with that line blanked (every other line stays where it was), and the tool lists
//     them: an internal task is a task the tool attests.
//   - cross_references, file granularity: an `includes:` entry whose file (or the file
//     its directory holds, by the tool's own search order) contributes at least one task
//     to the tool's listing, from the including Taskfile to that file. An include the tool
//     listed nothing of (a file with no tasks) and one written with a template are
//     counted on stderr and left out of the truth; a Taskfile with a remote include is
//     rejected by the tool as a whole (remote Taskfiles are an experiment, off by
//     default) and excluded.
//
// Tier A (-refs-out -ablate, ablate.go): each reference below is cut from a scratch copy
// whose sh: and if: values are all `true`, and kept when `task --dry --force` then names
// another set of tasks (or refuses the run).
//
// Tier C (-refs-out). go-task lists nothing of what a task depends on, and `--summary`
// renders (and so evaluates) a task's variables, dynamic ones included, so no run of the
// tool is made to read them. The references are a decode of the file (yq: an independent
// YAML reader, not the doc engine's walk, with anchors and merge keys expanded): the
// task names in a task's `deps`, its `cmds` `task:` calls, a `defer` that calls one, and
// the same in a task written as a list of commands. Each is checked against the tool's
// listing (the name is a task the tool knows, an alias, a namespaced task or a wildcard
// match) and the ones it does not know are counted on stderr, not scored.
//
// A Taskfile the tool rejects (a version it does not read, a cycle, a missing include)
// contributes nothing and is left out of -files-out, counted on stderr with its reason.
// Measure with `lang-oracle -only "$(paste -sd, <files-out>)"`. Directories the connector
// never reads (testdata, vendor, node_modules, generated, .git and the editor
// directories) are not walked, so the oracle does not evaluate what fathom would not see.
//
// Bench-only; never a product dependency. Standard library only; `task` and `yq`
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
	"strconv"
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
	Definitions     []def  `json:"definitions,omitempty"`
	References      []ref  `json:"references,omitempty"`
	CrossReferences []xref `json:"cross_references,omitempty"`
}

// taskfileNames is go-task's own search order for the Taskfile a directory holds.
var taskfileNames = []string{
	"Taskfile.yml", "taskfile.yml", "Taskfile.yaml", "taskfile.yaml",
	"Taskfile.dist.yml", "taskfile.dist.yml", "Taskfile.dist.yaml", "taskfile.dist.yaml",
}

var skipDirs = map[string]bool{
	"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true,
	".idea": true, ".vscode": true, ".claude": true,
}

type runner struct {
	task, yq string
	timeout  time.Duration
	env      []string
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

func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "taskfile-oracle:", err)
	os.Exit(1)
}

// listing is `task --list-all --json` for one Taskfile.
type listing struct {
	Tasks []struct {
		Name     string   `json:"name"`
		Aliases  []string `json:"aliases"`
		Location struct {
			Line     int    `json:"line"`
			Column   int    `json:"column"`
			Taskfile string `json:"taskfile"`
		} `json:"location"`
	} `json:"tasks"`
}

func (r runner) list(ctx context.Context, abs string) (listing, error) {
	var l listing
	so, se, err := r.run(ctx, filepath.Dir(abs), []byte{}, r.task, "--list-all", "--json", "--taskfile", abs)
	if err != nil {
		return l, fmt.Errorf("%s", firstLine(se))
	}
	if err := json.Unmarshal(so, &l); err != nil {
		return l, fmt.Errorf("task output: %w", err)
	}
	return l, nil
}

func (r runner) yqJSON(ctx context.Context, expr string, src []byte) ([]byte, error) {
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

func isTaskfileName(n string) bool {
	for _, t := range taskfileNames {
		if n == t {
			return true
		}
	}
	return false
}

func findTaskfiles(root string) ([]string, error) {
	var found []string
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
		if isTaskfileName(d.Name()) {
			rel, _ := filepath.Rel(root, p)
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(found)
	return found, err
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

// keyAt reads the mapping key written at a 1-based line and column: a quoted key
// without its quotes, a plain one up to the first colon that ends it (followed by a
// space or the end of the line), so `generate:mocks:` is `generate:mocks` and
// `release:*:` is `release:*`.
func keyAt(lines []string, line, col int) (string, bool) {
	if line < 1 || line > len(lines) || col < 1 || col > len(lines[line-1])+1 {
		return "", false
	}
	s := lines[line-1][col-1:]
	if s == "" {
		return "", false
	}
	if s[0] == '"' || s[0] == '\'' {
		q := s[0]
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' && q == '"' {
				i++
				continue
			}
			if s[i] == q {
				if q == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				k := s[1:i]
				if q == '\'' {
					k = strings.ReplaceAll(k, "''", "'")
				} else if u, err := strconv.Unquote(s[:i+1]); err == nil {
					k = u
				}
				return k, true
			}
		}
		return "", false
	}
	for i := 0; i < len(s); i++ {
		if s[i] == ':' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t') {
			return strings.TrimSpace(s[:i]), true
		}
	}
	return "", false
}

// decoded is what a Taskfile writes, as the decode reads it.
type decoded struct {
	Tasks    map[string]json.RawMessage `json:"tasks"`
	Includes map[string]json.RawMessage `json:"includes"`
}

// taskRefs are the task names a task's deps, cmds and defer call, from the decoded task.
func taskRefs(raw json.RawMessage) []string {
	var names []string
	add := func(v any) {
		switch x := v.(type) {
		case string:
			names = append(names, x)
		case map[string]any:
			if s, ok := x["task"].(string); ok {
				names = append(names, s)
			}
		}
	}
	cmd := func(v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		if s, ok := m["task"].(string); ok {
			names = append(names, s)
		}
		if d, ok := m["defer"].(map[string]any); ok {
			if s, ok := d["task"].(string); ok {
				names = append(names, s)
			}
		}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	switch t := v.(type) {
	case []any: // a task written as a list of commands
		for _, c := range t {
			cmd(c)
		}
	case map[string]any:
		if deps, ok := t["deps"].([]any); ok {
			for _, d := range deps {
				add(d)
			}
		}
		if cmds, ok := t["cmds"].([]any); ok {
			for _, c := range cmds {
				cmd(c)
			}
		}
	}
	return names
}

// includeTarget is the path an `includes:` entry names: the string, or its taskfile key.
func includeTarget(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var m struct {
		Taskfile string `json:"taskfile"`
	}
	if json.Unmarshal(raw, &m) == nil {
		return m.Taskfile
	}
	return ""
}

var remoteRE = regexp.MustCompile(`^(?:[a-z][a-z0-9+.-]*://|git@|git::)`)

func main() {
	subject := flag.String("subject", "", "the checkout to read (required)")
	taskBin := flag.String("task", "task", "go-task")
	yq := flag.String("yq", "yq", "mikefarah yq v4 (the independent decode)")
	scratch := flag.String("scratch", "", "a scratch directory outside the subject (default: a temporary one)")
	filesOut := flag.String("files-out", "", "write the Taskfiles the tool accepted, one per line, for lang-oracle -only")
	refsOut := flag.String("refs-out", "", "write the references file here (tier C; tier A with -ablate)")
	ablate := flag.Bool("ablate", false, "attest each reference by ablation: cut it from a neutralized scratch copy and see `task --dry --force` change (ablate.go); the references file is then tier A")
	timeout := flag.Duration("timeout", time.Minute, "per run")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "taskfile-oracle: -subject is required")
		os.Exit(2)
	}
	root, err := filepath.EvalSymlinks(*subject)
	if err != nil {
		fatal(err)
	}
	if *scratch == "" {
		d, err := os.MkdirTemp("", "taskfile-oracle-")
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
	if work, err = filepath.EvalSymlinks(work); err != nil {
		fatal(err)
	}
	env := append(os.Environ(),
		"HTTP_PROXY=http://127.0.0.1:9", "HTTPS_PROXY=http://127.0.0.1:9", "ALL_PROXY=http://127.0.0.1:9",
		"http_proxy=http://127.0.0.1:9", "https_proxy=http://127.0.0.1:9", "all_proxy=http://127.0.0.1:9",
		"NO_PROXY=", "no_proxy=", "GIT_TERMINAL_PROMPT=0", "CI=true")
	r := runner{task: *taskBin, yq: *yq, timeout: *timeout, env: env}
	ctx := context.Background()

	files, err := findTaskfiles(work)
	if err != nil {
		fatal(err)
	}
	if len(files) == 0 {
		fatal(fmt.Errorf("no Taskfile under %s", root))
	}
	fileSet := map[string]bool{}
	for _, f := range files {
		fileSet[f] = true
	}

	version := taskVersion(ctx, r)
	tierA := out{Tool: "task --list-all --json", Version: version, Tier: "A", Definitions: []def{}, CrossReferences: []xref{}}
	refsFile := out{Tool: "task --list-all --json (names) with a decode of deps, cmds and defer", Version: version, Tier: "C", References: []ref{}}
	stats := map[string]int{}
	excluded := map[string][]string{}
	var accepted []string

	for _, rel := range files {
		abs := filepath.Join(work, filepath.FromSlash(rel))
		orig, err := os.ReadFile(abs)
		if err != nil {
			fatal(err)
		}
		exclude := func(why string) {
			stats["excluded"]++
			excluded[why] = append(excluded[why], rel)
		}
		// the run as the file is written first: a file the tool rejects contributes nothing
		if _, err := r.list(ctx, abs); err != nil {
			exclude("the tool rejects it: " + err.Error())
			continue
		}
		// then with `internal: true` removed, so the tool lists what it hides
		lst, note := internalToo(ctx, r, abs, orig)
		if note != "" {
			stats["internal-variant-failed"]++
		}
		accepted = append(accepted, rel)
		stats["accepted"]++
		base := path.Base(rel)
		lines := strings.Split(string(orig), "\n")
		tierA.Definitions = append(tierA.Definitions, def{Path: rel, Name: base, Line: leadingLines(orig) + 1, Kind: "package"})

		selfAbs, _ := filepath.EvalSymlinks(abs)
		own := map[string]bool{}
		known := map[string]bool{}
		var wild []string
		fileTasks := map[string]bool{} // files (absolute) that contribute a task
		prefixFiles := map[string]map[string]bool{}
		for _, t := range lst.Tasks {
			tf, _ := filepath.EvalSymlinks(t.Location.Taskfile)
			fileTasks[tf] = true
			known[t.Name] = true
			for _, a := range t.Aliases {
				known[a] = true
			}
			if i := strings.IndexByte(t.Name, ':'); i > 0 {
				ns := t.Name[:i]
				if prefixFiles[ns] == nil {
					prefixFiles[ns] = map[string]bool{}
				}
				prefixFiles[ns][tf] = true
			}
			if strings.Contains(t.Name, "*") {
				wild = append(wild, t.Name)
			}
			if tf != selfAbs {
				continue
			}
			key, ok := keyAt(lines, t.Location.Line, t.Location.Column)
			if !ok {
				stats["task-key-unreadable"]++
				fmt.Fprintf(os.Stderr, "task-key-unreadable: %s:%d:%d %q\n", rel, t.Location.Line, t.Location.Column, t.Name)
				continue
			}
			if key != t.Name {
				// a task with a `label:` is listed under its label; the key is the name a
				// deps or task: entry writes
				stats["task-labelled-differently"]++
				fmt.Fprintf(os.Stderr, "task-labelled-differently: %s:%d tool %q key %q\n", rel, t.Location.Line, t.Name, key)
			}
			own[key] = true
			known[key] = true
			tierA.Definitions = append(tierA.Definitions, def{Path: rel, Name: key, Line: t.Location.Line, Kind: "task"})
			stats["tasks"]++
		}

		// references: a decode, checked against the tool's names
		var dec decoded
		if raw, err := r.yqJSON(ctx, "explode(.) | {\"tasks\": (.tasks // {}), \"includes\": (.includes // {})}", orig); err == nil {
			_ = json.Unmarshal(raw, &dec)
		} else {
			stats["undecodable"]++
		}
		declared := map[string]bool{}
		for name, raw := range dec.Tasks {
			declared[name] = true
			if !own[name] {
				stats["declared-not-in-tool"]++
				fmt.Fprintf(os.Stderr, "declared-not-in-tool: %s task %q\n", rel, name)
				continue
			}
			seen := map[string]bool{}
			for _, to := range taskRefs(raw) {
				if seen[to] {
					continue
				}
				seen[to] = true
				refsFile.References = append(refsFile.References, ref{Path: rel, From: name, To: to})
				stats["refs"]++
				if !known[to] && !matchesWildcard(to, wild) && !strings.Contains(to, "{{") {
					stats["ref-unknown-to-tool"]++
					fmt.Fprintf(os.Stderr, "ref-unknown-to-tool: %s %q -> %q\n", rel, name, to)
				}
			}
		}
		for name := range own {
			if !declared[name] && len(dec.Tasks) > 0 {
				stats["tool-not-declared"]++
				fmt.Fprintf(os.Stderr, "tool-not-declared: %s task %q\n", rel, name)
			}
		}

		// includes: the tool's listing is the evidence
		nss := make([]string, 0, len(dec.Includes))
		for ns := range dec.Includes {
			nss = append(nss, ns)
		}
		sort.Strings(nss)
		for _, ns := range nss {
			target := includeTarget(dec.Includes[ns])
			switch {
			case target == "":
				stats["include-without-taskfile"]++
				continue
			case remoteRE.MatchString(target):
				stats["include-remote"]++
				continue
			case strings.Contains(target, "{{"):
				stats["include-templated"]++
				continue
			}
			to, ok := includedFile(work, path.Dir(rel), target)
			if !ok {
				stats["include-names-no-file"]++
				continue
			}
			toAbs := filepath.Join(work, filepath.FromSlash(to))
			toReal, _ := filepath.EvalSymlinks(toAbs)
			if toReal == selfAbs || !fileTasks[toReal] {
				if toReal != selfAbs {
					stats["include-not-evidenced"]++
					fmt.Fprintf(os.Stderr, "include-not-evidenced: %s %q -> %s (the tool lists no task of it)\n", rel, ns, to)
				}
				continue
			}
			if pf := prefixFiles[ns]; pf != nil && !pf[toReal] && !flattened(dec.Includes[ns]) {
				stats["include-prefix-mismatch"]++
				fmt.Fprintf(os.Stderr, "include-prefix-mismatch: %s %q -> %s\n", rel, ns, to)
			}
			tierA.CrossReferences = append(tierA.CrossReferences, xref{Path: rel, From: base, ToPath: to})
			stats["xref"]++
		}
	}

	sort.Slice(tierA.Definitions, func(i, j int) bool {
		a, b := tierA.Definitions[i], tierA.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.Slice(refsFile.References, func(i, j int) bool {
		a, b := refsFile.References[i], refsFile.References[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	sort.Slice(tierA.CrossReferences, func(i, j int) bool {
		a, b := tierA.CrossReferences[i], tierA.CrossReferences[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.ToPath < b.ToPath
	})
	if *ablate {
		// A second copy, every accepted Taskfile neutralized (no subject shell runs).
		abl := filepath.Join(*scratch, "ablate")
		if err := os.RemoveAll(abl); err != nil {
			fatal(err)
		}
		if err := copyTree(work, abl); err != nil {
			fatal(err)
		}
		for _, rel := range accepted {
			if err := r.neutralize(ctx, filepath.Join(abl, filepath.FromSlash(rel))); err != nil {
				fatal(err)
			}
		}
		ab := newAblator(r)
		kept := refsFile.References[:0]
		for _, x := range refsFile.References {
			file := filepath.Join(abl, filepath.FromSlash(x.Path))
			ok, err := ab.attest(ctx, filepath.Dir(file), file, x.From, x.To)
			if err != nil {
				fatal(err)
			}
			if !ok {
				stats["ref-not-attested"]++
				fmt.Fprintf(os.Stderr, "ref-not-attested (cutting it changes no task the dry run names): %s %q -> %q\n", x.Path, x.From, x.To)
				continue
			}
			stats["ref-attested"]++
			kept = append(kept, x)
		}
		refsFile.References = kept
		refsFile.Tool = "task --dry --force ablation (each deps entry and task: call cut from a scratch copy with every sh: and if: set to true)"
		refsFile.Tier = "A"
		stats["dry-runs"] = ab.runs
	}
	if *filesOut != "" {
		sort.Strings(accepted)
		if err := os.WriteFile(*filesOut, []byte(strings.Join(accepted, "\n")+"\n"), 0o644); err != nil {
			fatal(err)
		}
	}
	if *refsOut != "" {
		b, err := json.MarshalIndent(refsFile, "", " ")
		if err != nil {
			fatal(err)
		}
		if err := os.WriteFile(*refsOut, append(b, '\n'), 0o644); err != nil {
			fatal(err)
		}
	}
	summary(os.Stderr, len(files), stats, excluded)
	b, err := json.MarshalIndent(tierA, "", " ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(b))
}

func flattened(raw json.RawMessage) bool {
	var m struct {
		Flatten bool `json:"flatten"`
	}
	return json.Unmarshal(raw, &m) == nil && m.Flatten
}

// includedFile resolves an include as the tool does: a file, or the Taskfile a
// directory holds by go-task's search order. Only a file of the subject counts. The
// directory is read, not stat-ed by candidate name, so the path returned is spelled as
// on disk (a case-insensitive filesystem answers to any spelling).
func includedFile(work, dir, target string) (string, bool) {
	p := path.Clean(path.Join(dir, target))
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	st, err := os.Stat(filepath.Join(work, filepath.FromSlash(p)))
	if err != nil {
		return "", false
	}
	if !st.IsDir() {
		return p, true
	}
	ents, err := os.ReadDir(filepath.Join(work, filepath.FromSlash(p)))
	if err != nil {
		return "", false
	}
	have := map[string]bool{}
	for _, e := range ents {
		if !e.IsDir() {
			have[e.Name()] = true
		}
	}
	for _, n := range taskfileNames {
		if have[n] {
			return path.Join(p, n), true
		}
	}
	return "", false
}

func matchesWildcard(name string, wild []string) bool {
	for _, w := range wild {
		if ok, _ := path.Match(w, name); ok {
			return true
		}
	}
	return false
}

// internalRE is a block-style `internal: true` line of a task.
var internalRE = regexp.MustCompile(`^\s+internal:\s*(?:true|True|TRUE|yes|on)\s*(?:#.*)?$`)

// withoutInternal blanks every block-style `internal: true` line, keeping every other
// line where it was: a line the tool reports for the copy is the line of the file. It
// reports how many it blanked.
func withoutInternal(src []byte) ([]byte, int) {
	lines := strings.Split(string(src), "\n")
	n := 0
	for i, l := range lines {
		if internalRE.MatchString(strings.TrimRight(l, "\r")) {
			lines[i] = ""
			n++
		}
	}
	return []byte(strings.Join(lines, "\n")), n
}

// internalToo lists a Taskfile with every task's `internal: true` removed, so the tool
// lists the tasks it hides (`--list-all` leaves an internal task out); on any failure it
// falls back to the plain listing.
func internalToo(ctx context.Context, r runner, abs string, orig []byte) (listing, string) {
	plain, perr := r.list(ctx, abs)
	variant, n := withoutInternal(orig)
	if n == 0 {
		return plain, ""
	}
	if werr := os.WriteFile(abs, variant, 0o644); werr != nil {
		fatal(werr)
	}
	defer func() {
		if werr := os.WriteFile(abs, orig, 0o644); werr != nil {
			fatal(werr)
		}
	}()
	l, err := r.list(ctx, abs)
	if err != nil {
		if perr != nil {
			return listing{}, err.Error()
		}
		return plain, err.Error()
	}
	return l, ""
}

func summary(w io.Writer, total int, stats map[string]int, excluded map[string][]string) {
	fmt.Fprintf(w, "taskfiles: %d found, %d accepted by the tool, %d tasks listed as declared in their own file\n", total, stats["accepted"], stats["tasks"])
	var whys []string
	for why := range excluded {
		whys = append(whys, why)
	}
	sort.Strings(whys)
	for _, why := range whys {
		fmt.Fprintf(w, "excluded (%d): %s: %s\n", len(excluded[why]), why, strings.Join(excluded[why], ", "))
	}
	fmt.Fprintf(w, "references decoded: %d (unknown to the tool: %d); declared-not-in-tool: %d; tool-not-declared: %d; undecodable: %d\n",
		stats["refs"], stats["ref-unknown-to-tool"], stats["declared-not-in-tool"], stats["tool-not-declared"], stats["undecodable"])
	fmt.Fprintf(w, "includes evidenced by the tool: %d; remote: %d; templated: %d; no taskfile key: %d; names no file: %d; the tool lists no task of it: %d; prefix mismatch: %d\n",
		stats["xref"], stats["include-remote"], stats["include-templated"], stats["include-without-taskfile"], stats["include-names-no-file"], stats["include-not-evidenced"], stats["include-prefix-mismatch"])
	if n := stats["internal-variant-failed"]; n > 0 {
		fmt.Fprintf(w, "internal-task variant failed on %d Taskfiles (their internal tasks are not listed)\n", n)
	}
	if n := stats["task-key-unreadable"]; n > 0 {
		fmt.Fprintf(w, "task keys the oracle could not read at the tool's line and column: %d (left out)\n", n)
	}
	if n := stats["task-labelled-differently"]; n > 0 {
		fmt.Fprintf(w, "tasks the tool lists under a label that is not their key: %d (named by their key)\n", n)
	}
}

var taskVersionRE = regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)

func taskVersion(ctx context.Context, r runner) string {
	so, _, err := r.run(ctx, "", nil, r.task, "--version")
	if err != nil {
		return "unknown"
	}
	if m := taskVersionRE.FindSubmatch(so); m != nil {
		return "go-task " + string(m[1])
	}
	return "unknown"
}
