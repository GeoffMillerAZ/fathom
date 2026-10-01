// Command lefthook-dump-oracle is the language-breadth harness's oracle for the
// git-hooks-config doc-engine dialect (design §1.3, §13.4 step 19). It writes
// indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10), one file per
// tier:
//
//	go run . -subject <checkout> -scratch <dir outside the subject> \
//	    [-lefthook <bin>] [-yq <yq>] [-ruby <ruby>] \
//	    [-out-a <file>] [-out-c <file>] [-files-out <file>]
//
// Tier A (-out-a): lefthook itself. `lefthook dump --format json` (lefthook 1.13.6) is
// run on every lefthook config of the subject (lefthook.yml, .lefthook.yml,
// lefthook.yaml, .lefthook.yaml and the four -local spellings), each copied alone into a
// fresh scratch git repository as its lefthook.yml, so each is the root of its own run and
// nothing else merges in. The dump is the config as lefthook loaded it: its hooks, each
// hook's commands and scripts by name and its jobs (named or not, nested groups
// included). Which top-level keys are lefthook's own options (min_version, colors,
// templates, remotes, ...) and not hooks is read from the JSON schema the lefthook binary
// embeds (its `validate` schema): every property that is not a `#/$defs/Hook`. The tool
// writes no line, so each name is joined to the line written for it by an independent
// decode (yq v4's `line`, not fathom's gopkg.in/yaml.v3 walk): a hook and a command or
// script at its key's line, a job at its list item's first line. Emitted:
//
//   - definitions: a `stage` per hook, a `job` per command, script and named job (a job
//     with no `name:` has no name to match, in lefthook's output or fathom's, and is
//     counted on stderr).
//
// A config the tool rejects (it cannot load it) is left out of -files-out and counted on
// stderr with its reason, as is a name whose written line the decode cannot find (a
// command reached through a YAML merge key).
//
// Tier C (-out-c): pre-commit is not installed on the measuring machine, so a
// .pre-commit-config.yaml and a .pre-commit-hooks.yaml are read by a Ruby Psych reference
// parse (precommit_psych.rb, libyaml): a `job` per `repos[*].hooks[*]` entry carrying an
// `id`, and per hook of a manifest's root list, at the entry's first line.
//
// One subject gives one L1 oracle (lang-oracle refuses a second definitions file on the
// same subject), so a subject holding both formats fails here: split it with -only.
// Directories the connector never reads (testdata, vendor, node_modules, generated, .git
// and the editor directories) are not walked.
//
// Bench-only; never a product dependency. Standard library only; `lefthook`, `git`, `yq`
// and `ruby` are external programs.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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

type out struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
}

var lefthookNames = map[string]bool{
	"lefthook.yml": true, "lefthook.yaml": true, ".lefthook.yml": true, ".lefthook.yaml": true,
	"lefthook-local.yml": true, "lefthook-local.yaml": true, ".lefthook-local.yml": true, ".lefthook-local.yaml": true,
}

var preCommitNames = map[string]string{
	".pre-commit-config.yaml": "config",
	".pre-commit-hooks.yaml":  "manifest",
}

var skipDirs = map[string]bool{
	"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true,
	".idea": true, ".vscode": true, ".claude": true,
}

type runner struct {
	lefthook, yq, ruby, psych string
	timeout                   time.Duration
}

func (r runner) run(ctx context.Context, dir string, stdin []byte, name string, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LEFTHOOK=1", "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1")
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
	fmt.Fprintln(os.Stderr, "lefthook-dump-oracle:", err)
	os.Exit(1)
}

func findConfigs(root string) ([]string, error) {
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
		if _, pc := preCommitNames[d.Name()]; lefthookNames[d.Name()] || pc {
			rel, _ := filepath.Rel(root, p)
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(found)
	return found, err
}

// schemaMarker opens the JSON schema the lefthook binary embeds (the one
// `lefthook validate` checks a config against).
var schemaMarker = []byte("{\n  \"$schema\": \"https://json-schema.org/draft/2020-12/schema\",\n  \"$id\": \"https://json.schemastore.org/lefthook.json\"")

// optionKeys is the set of top-level keys lefthook's schema declares that are not
// hooks: every property whose schema is not a reference to `#/$defs/Hook`.
func optionKeys(bin []byte) (map[string]bool, error) {
	i := bytes.Index(bin, schemaMarker)
	if i < 0 {
		return nil, errors.New("no embedded lefthook JSON schema found in the binary")
	}
	var schema struct {
		Properties map[string]struct {
			Ref string `json:"$ref"`
		} `json:"properties"`
	}
	if err := json.NewDecoder(bytes.NewReader(bin[i:])).Decode(&schema); err != nil {
		return nil, fmt.Errorf("embedded schema: %w", err)
	}
	opts := map[string]bool{}
	hooks := 0
	for k, p := range schema.Properties {
		if p.Ref == "#/$defs/Hook" {
			hooks++
			continue
		}
		opts[k] = true
	}
	if hooks == 0 || len(opts) == 0 {
		return nil, fmt.Errorf("embedded schema lists %d hooks and %d options", hooks, len(opts))
	}
	return opts, nil
}

// dumpJob is one job of a hook's `jobs:` list, as lefthook dumps it.
type dumpJob struct {
	Name  string `json:"name"`
	Group *struct {
		Jobs []dumpJob `json:"jobs"`
	} `json:"group"`
}

type dumpHook struct {
	Commands map[string]json.RawMessage `json:"commands"`
	Scripts  map[string]json.RawMessage `json:"scripts"`
	Jobs     []dumpJob                  `json:"jobs"`
}

// entity is one name the dump attests, with the decode path its line is read at.
type entity struct {
	name, kind string
	path       []any
	itemLine   bool // a list item: its own first line, not a key's line
}

// dumpEntities reads a dump: every top-level key that is not an option is a hook.
func dumpEntities(dump []byte, opts map[string]bool) ([]entity, int, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(dump, &top); err != nil {
		return nil, 0, fmt.Errorf("dump is not a JSON object: %w", err)
	}
	var hooks []string
	for k := range top {
		if !opts[k] {
			hooks = append(hooks, k)
		}
	}
	sort.Strings(hooks)
	var ents []entity
	unnamed := 0
	for _, h := range hooks {
		var dh dumpHook
		if err := json.Unmarshal(top[h], &dh); err != nil {
			return nil, 0, fmt.Errorf("hook %s: %w", h, err)
		}
		ents = append(ents, entity{name: h, kind: "stage", path: []any{h}})
		for _, c := range sortedKeys(dh.Commands) {
			ents = append(ents, entity{name: c, kind: "job", path: []any{h, "commands", c}})
		}
		for _, s := range sortedKeys(dh.Scripts) {
			ents = append(ents, entity{name: s, kind: "job", path: []any{h, "scripts", s}})
		}
		var walk func(jobs []dumpJob, at []any)
		walk = func(jobs []dumpJob, at []any) {
			for i, j := range jobs {
				p := append(append([]any(nil), at...), i)
				if j.Name != "" {
					ents = append(ents, entity{name: j.Name, kind: "job", path: p, itemLine: true})
				} else {
					unnamed++
				}
				if j.Group != nil {
					walk(j.Group.Jobs, append(append([]any(nil), p...), "group", "jobs"))
				}
			}
		}
		walk(dh.Jobs, []any{h, "jobs"})
	}
	return ents, unnamed, nil
}

func sortedKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// pathKey renders a decode path (string keys, int indices) as one map key.
func pathKey(p []any) string {
	parts := make([]string, len(p))
	for i, x := range p {
		switch v := x.(type) {
		case string:
			parts[i] = "s:" + v
		case int:
			parts[i] = "i:" + strconv.Itoa(v)
		case float64:
			parts[i] = "i:" + strconv.Itoa(int(v))
		default:
			parts[i] = fmt.Sprint(v)
		}
	}
	return strings.Join(parts, "\x00")
}

// lines is yq's decode of a file: every node's path, its line and its key's line.
// yq's header preprocessing (on by default) slurps a file's leading comments and
// `---` before parsing, which shifts every line it reports by their count; the
// decode turns it off so a line is the file's.
type lines map[string]struct{ line, keyLine int }

const yqLines = `[.. | select(path | length > 0) | {"p": path, "l": line, "kl": (key | line)}]`

func (r runner) decodeLines(ctx context.Context, src []byte) (lines, error) {
	so, se, err := r.run(ctx, "", src, r.yq, "--header-preprocess=false", "-o=json", "-I=0", yqLines, "-")
	if err != nil {
		return nil, fmt.Errorf("yq: %v: %s", err, firstLine(se))
	}
	var rows []struct {
		P  []any `json:"p"`
		L  int   `json:"l"`
		KL int   `json:"kl"`
	}
	if err := json.Unmarshal(so, &rows); err != nil {
		return nil, fmt.Errorf("yq output: %w", err)
	}
	ls := lines{}
	for _, row := range rows {
		ls[pathKey(row.P)] = struct{ line, keyLine int }{row.L, row.KL}
	}
	return ls, nil
}

func (e entity) lineIn(ls lines) (int, bool) {
	n, ok := ls[pathKey(e.path)]
	if !ok {
		return 0, false
	}
	if e.itemLine {
		return n.line, n.line > 0
	}
	return n.keyLine, n.keyLine > 0
}

func (r runner) dump(ctx context.Context, scratch string, i int, src []byte) ([]byte, error) {
	dir := filepath.Join(scratch, fmt.Sprintf("cfg-%03d", i))
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if _, se, err := r.run(ctx, dir, nil, "git", "init", "-q", "."); err != nil {
		return nil, fmt.Errorf("git init: %v: %s", err, firstLine(se))
	}
	if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), src, 0o644); err != nil {
		return nil, err
	}
	so, se, err := r.run(ctx, dir, nil, r.lefthook, "--colors", "off", "dump", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("lefthook dump: %v: %s", err, firstLine(append(se, so...)))
	}
	if t := bytes.TrimSpace(so); len(t) == 0 || t[0] != '{' {
		return nil, fmt.Errorf("lefthook dump: %s", firstLine(append(so, se...)))
	}
	return so, nil
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	scratch := flag.String("scratch", "", "scratch directory outside the subject (required)")
	lefthookBin := flag.String("lefthook", "lefthook", "lefthook 1.13.6 (the tier-A tool)")
	yq := flag.String("yq", "yq", "mikefarah yq v4 (the independent decode for lines)")
	ruby := flag.String("ruby", "ruby", "ruby (Psych, the tier-C reference parse)")
	outA := flag.String("out-a", "", "write the tier-A (lefthook) oracle here")
	outC := flag.String("out-c", "", "write the tier-C (pre-commit, Psych) oracle here")
	filesOut := flag.String("files-out", "", "write the accepted config paths here, one per line")
	flag.Parse()
	if *subject == "" || *scratch == "" {
		fatal(errors.New("-subject and -scratch are required"))
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fatal(err)
	}
	sc, err := filepath.Abs(*scratch)
	if err != nil {
		fatal(err)
	}
	if rel, err := filepath.Rel(root, sc); err == nil && !strings.HasPrefix(rel, "..") {
		fatal(fmt.Errorf("-scratch %s is inside the subject", sc))
	}
	self, err := os.Executable()
	if err != nil {
		fatal(err)
	}
	psych := filepath.Join(filepath.Dir(self), "precommit_psych.rb")
	if _, err := os.Stat(psych); err != nil {
		// `go run` builds into a temporary directory: fall back to the source tree.
		if wd, werr := os.Getwd(); werr == nil {
			psych = filepath.Join(wd, "precommit_psych.rb")
		}
	}
	r := runner{lefthook: *lefthookBin, yq: *yq, ruby: *ruby, psych: psych, timeout: 2 * time.Minute}
	ctx := context.Background()

	files, err := findConfigs(root)
	if err != nil {
		fatal(err)
	}
	a := out{Tool: "lefthook dump", Tier: "A", Definitions: []def{}}
	c := out{Tool: "ruby psych", Tier: "C", Definitions: []def{}}
	var accepted []string
	rejected := map[string][]string{}
	unnamedJobs, unplaced := 0, 0
	var opts map[string]bool
	for i, rel := range files {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			fatal(err)
		}
		base := filepath.Base(rel)
		if mode, ok := preCommitNames[base]; ok {
			so, se, err := r.run(ctx, "", nil, r.ruby, r.psych, filepath.Join(root, filepath.FromSlash(rel)), mode)
			if err != nil {
				rejected[firstLine(se)] = append(rejected[firstLine(se)], rel)
				continue
			}
			var ds []def
			if err := json.Unmarshal(so, &ds); err != nil {
				fatal(fmt.Errorf("%s: psych output: %w", rel, err))
			}
			for _, d := range ds {
				c.Definitions = append(c.Definitions, def{Path: rel, Name: d.Name, Line: d.Line, Kind: d.Kind})
			}
			accepted = append(accepted, rel)
			continue
		}
		if opts == nil {
			bin, err := exec.LookPath(r.lefthook)
			if err != nil {
				fatal(err)
			}
			raw, err := os.ReadFile(bin)
			if err != nil {
				fatal(err)
			}
			if opts, err = optionKeys(raw); err != nil {
				fatal(err)
			}
			v, _, err := r.run(ctx, "", nil, r.lefthook, "version")
			if err != nil {
				fatal(fmt.Errorf("lefthook version: %w", err))
			}
			a.Version = firstLine(v)
		}
		dump, err := r.dump(ctx, sc, i, src)
		if err != nil {
			rejected[err.Error()] = append(rejected[err.Error()], rel)
			continue
		}
		ents, unnamed, err := dumpEntities(dump, opts)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", rel, err))
		}
		unnamedJobs += unnamed
		ls, err := r.decodeLines(ctx, src)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", rel, err))
		}
		for _, e := range ents {
			line, ok := e.lineIn(ls)
			if !ok {
				unplaced++
				fmt.Fprintf(os.Stderr, "%s: %s %q has no written line in the decode (left out)\n", rel, e.kind, e.name)
				continue
			}
			a.Definitions = append(a.Definitions, def{Path: rel, Name: e.name, Line: line, Kind: e.kind})
		}
		accepted = append(accepted, rel)
	}
	if v, _, err := r.run(ctx, "", nil, r.ruby, "-e", `require "psych"; print "#{Psych::VERSION} (ruby #{RUBY_VERSION})"`); err == nil {
		c.Version = string(v)
	}
	if len(a.Definitions) > 0 && len(c.Definitions) > 0 {
		fatal(errors.New("the subject holds lefthook and pre-commit configs: one L1 oracle per subject, split it with -only"))
	}
	fmt.Fprintf(os.Stderr, "lefthook-dump-oracle: %d configs, %d accepted, %d lefthook definitions, %d pre-commit definitions, %d unnamed jobs, %d names with no written line\n",
		len(files), len(accepted), len(a.Definitions), len(c.Definitions), unnamedJobs, unplaced)
	whys := make([]string, 0, len(rejected))
	for why := range rejected {
		whys = append(whys, why)
	}
	sort.Strings(whys)
	for _, why := range whys {
		fmt.Fprintf(os.Stderr, "  rejected (%s): %s\n", why, strings.Join(rejected[why], ", "))
	}
	write := func(path string, o out) {
		if path == "" || len(o.Definitions) == 0 {
			return
		}
		sort.SliceStable(o.Definitions, func(i, j int) bool {
			x, y := o.Definitions[i], o.Definitions[j]
			if x.Path != y.Path {
				return x.Path < y.Path
			}
			if x.Line != y.Line {
				return x.Line < y.Line
			}
			return x.Name < y.Name
		})
		b, err := json.MarshalIndent(o, "", " ")
		if err != nil {
			fatal(err)
		}
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			fatal(err)
		}
	}
	write(*outA, a)
	write(*outC, c)
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(accepted, "\n")+"\n"), 0o644); err != nil {
			fatal(err)
		}
	}
}
