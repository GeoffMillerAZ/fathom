// Command circleci-oracle is the language-breadth harness's oracle for the
// circleci doc-engine dialect (design section 1.3, owner decision D4). It
// writes indexbench lang-oracle's tool-oracle JSON (-tool-oracle, section 10)
// in two modes:
//
//	go run . -subject <checkout> -cli <circleci> -scratch <dir> [-files-out <file>]  # definitions, tier A
//	go run . -subject <checkout> -cli <circleci> -scratch <dir> -refs                # references, tier C
//
// The harness scores references per file against every pair fathom draws, so
// the references of both sources are one file and the definitions another.
//
// Tier A. The tool is CircleCI's own compiler, `circleci config process`
// (circleci-cli 1.7.0), run on every .circleci/config.yml of the subject. It
// expands orbs, commands, executors, parameters, matrices and `when`, and
// answers the jobs and workflows the pipeline compiles to. A config without
// orbs compiles with no network; an orb is fetched from the registry, so a
// config that uses one needs the network. The compiled YAML carries no source
// line, so a yaml.v3 walk of the source supplies the key line of each
// definition, and attests it only when the compiler's output holds it:
//
//   - a job is attested when the compiled jobs hold its key, or an instance of
//     it: a workflow entry that runs the job under a literal `name:`, a
//     templated `name:` or a matrix (its expansions are the compiled jobs with
//     the name's literal prefix, or `<job>-` for a matrix with no name). The
//     compiler drops a job no workflow runs, so an uncalled job is not
//     attested and counts against sym_precision, as it should;
//   - a workflow is attested when the compiled workflows hold its key (the
//     compiler drops one whose `when` is false).
//
// References: for each compiled workflow, the job each compiled entry
// instantiates (through the instance map above) and the job each `requires`
// entry names, as (workflow, job) pairs for jobs the source declares. An orb
// job, an approval and a built-in step are not declared and are left out.
//
// References (-refs, one file, tier C: the lowest of its two sources). The compiler inlines commands and executors and lists
// neither, so which command a job or command invokes, and which executor a job
// names, are read from the source by a yaml.v3 walk, kept only for configs the
// compiler accepted and only for names the file declares.
//
// A config the compiler rejects contributes nothing and is counted on stderr;
// fathom still extracts what it can from such a file. Measure with
// `lang-oracle -only "$(paste -sd, <files-out>)"`. Bench-only; never a product
// dependency.
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

type out struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
	References  []ref  `json:"references"`
}

var skipDirs = map[string]bool{".git": true, "vendor": true, "testdata": true, "node_modules": true, ".idea": true, ".vscode": true}

func deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

// get reads a map entry; a key written twice reads as its last value.
func get(n *yaml.Node, key string) *yaml.Node {
	n = deref(n)
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

func scalar(n *yaml.Node) string {
	n = deref(n)
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

// entries lists a map's (key, value, key line) in document order.
type entry struct {
	key  string
	val  *yaml.Node
	line int
}

func entries(n *yaml.Node) []entry {
	n = deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var out []entry
	for i := 0; i+1 < len(n.Content); i += 2 {
		out = append(out, entry{n.Content[i].Value, deref(n.Content[i+1]), n.Content[i].Line})
	}
	return out
}

func seq(n *yaml.Node) []*yaml.Node {
	n = deref(n)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, 0, len(n.Content))
	for _, c := range n.Content {
		out = append(out, deref(c))
	}
	return out
}

// mapEntries are the map-valued entries of a top-level section, the entities
// the dialect reads.
func mapEntries(root *yaml.Node, section string) []entry {
	var out []entry
	for _, e := range entries(get(root, section)) {
		if e.val != nil && e.val.Kind == yaml.MappingNode {
			out = append(out, e)
		}
	}
	return out
}

func parseFile(abs string) (*yaml.Node, error) {
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("empty document")
	}
	return deref(doc.Content[0]), nil
}

// compiled is what the tool answered.
type compiled struct {
	jobs      map[string]bool
	workflows map[string][]cwjob
}

type cwjob struct {
	name     string
	requires []string
}

func runTool(ctx context.Context, cli, dir, rel, home string) (*compiled, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, "config", "process", rel)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+home)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(se.String())
		if i := strings.IndexByte(msg, '\n'); i > 0 {
			msg = msg[:i]
		}
		return nil, fmt.Errorf("%v: %s", err, msg)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(so.Bytes(), &doc); err != nil || len(doc.Content) == 0 {
		return nil, fmt.Errorf("unreadable output")
	}
	root := doc.Content[0]
	c := &compiled{jobs: map[string]bool{}, workflows: map[string][]cwjob{}}
	for _, e := range entries(get(root, "jobs")) {
		c.jobs[e.key] = true
	}
	for _, w := range entries(get(root, "workflows")) {
		if w.val == nil || w.val.Kind != yaml.MappingNode {
			continue
		}
		var js []cwjob
		for _, it := range seq(get(w.val, "jobs")) {
			if s := scalar(it); s != "" {
				js = append(js, cwjob{name: s})
				continue
			}
			for _, e := range entries(it) {
				j := cwjob{name: e.key}
				for _, r := range seq(get(e.val, "requires")) {
					if s := scalar(r); s != "" {
						j.requires = append(j.requires, s)
					}
				}
				js = append(js, j)
			}
		}
		c.workflows[w.key] = js
	}
	return c, nil
}

// instances maps each compiled workflow-job name to the source job it
// instantiates, from the source workflow entries.
func instances(srcWorkflow *yaml.Node, cw []cwjob) map[string]string {
	compiledNames := map[string]bool{}
	for _, j := range cw {
		compiledNames[j.name] = true
	}
	m := map[string]string{}
	add := func(inst, job string) {
		if compiledNames[inst] {
			if _, ok := m[inst]; !ok {
				m[inst] = job
			}
		}
	}
	for _, it := range seq(get(srcWorkflow, "jobs")) {
		var job string
		var opts *yaml.Node
		if s := scalar(it); s != "" {
			add(s, s)
			continue
		}
		es := entries(it)
		if len(es) == 0 {
			continue
		}
		job, opts = es[0].key, es[0].val
		name := scalar(get(opts, "name"))
		matrix := get(opts, "matrix") != nil
		switch {
		case name != "" && !strings.Contains(name, "<<"):
			add(name, job)
		case name != "" || matrix:
			prefix := job + "-"
			if name != "" {
				prefix = name[:strings.Index(name, "<<")]
			}
			if prefix != "" {
				for _, j := range cw {
					if strings.HasPrefix(j.name, prefix) {
						add(j.name, job)
					}
				}
			}
		default:
			add(job, job)
		}
	}
	return m
}

// stepNames collects the command names a node invokes: the items of every
// `steps` list at any depth, a bare name or each key of a map item.
func stepNames(n *yaml.Node, out *[]string) {
	n = deref(n)
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			v := deref(n.Content[i+1])
			if n.Content[i].Value == "steps" && v != nil && v.Kind == yaml.SequenceNode {
				for _, it := range v.Content {
					it = deref(it)
					if s := scalar(it); s != "" {
						*out = append(*out, s)
					} else if it != nil && it.Kind == yaml.MappingNode {
						for _, e := range entries(it) {
							*out = append(*out, e.key)
						}
					}
				}
			}
			stepNames(v, out)
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			stepNames(c, out)
		}
	}
}

func executorName(job *yaml.Node) string {
	ex := get(job, "executor")
	if s := scalar(ex); s != "" {
		return s
	}
	return scalar(get(ex, "name"))
}

func toolVersion(ctx context.Context, cli, home string) string {
	cmd := exec.CommandContext(ctx, cli, "version")
	cmd.Env = append(os.Environ(), "HOME="+home)
	b, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	cli := flag.String("cli", "circleci", "the circleci binary")
	scratch := flag.String("scratch", "", "a directory used as HOME for the tool (outside the subject)")
	refs := flag.Bool("refs", false, "tier C: command and executor references from a walk of the source")
	filesOut := flag.String("files-out", "", "write the accepted configs (subject-relative) here, one per line")
	flag.Parse()
	if *subject == "" || *scratch == "" {
		fmt.Fprintln(os.Stderr, "usage: circleci-oracle -subject <dir> -scratch <dir> [-cli <bin>] [-refs] [-files-out <file>]")
		os.Exit(2)
	}
	ctx := context.Background()
	var cfgs []string
	_ = filepath.WalkDir(*subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && p != *subject {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(*subject, p)
		rel = filepath.ToSlash(rel)
		if (d.Name() == "config.yml" || d.Name() == "config.yaml") && filepath.Base(filepath.Dir(rel)) == ".circleci" {
			cfgs = append(cfgs, rel)
		}
		return nil
	})
	sort.Strings(cfgs)
	res := out{Tool: "circleci config process", Version: toolVersion(ctx, *cli, *scratch), Tier: "A", Definitions: []def{}, References: []ref{}}
	if *refs {
		res.Tool, res.Tier = "circleci config process (workflow jobs, requires) and a yaml.v3 walk of steps and executor (declared commands and executors only)", "C"
	}
	var accepted []string
	rejected := 0
	for _, rel := range cfgs {
		root, perr := parseFile(filepath.Join(*subject, filepath.FromSlash(rel)))
		if perr != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", rel, perr)
			rejected++
			continue
		}
		c, terr := runTool(ctx, *cli, *subject, rel, *scratch)
		if terr != nil {
			fmt.Fprintf(os.Stderr, "rejected %s: %v\n", rel, terr)
			rejected++
			continue
		}
		accepted = append(accepted, rel)
		jobs := mapEntries(root, "jobs")
		declaredJobs := map[string]bool{}
		for _, e := range jobs {
			declaredJobs[e.key] = true
		}
		attested := map[string]bool{}
		pairs := map[string]bool{}
		srcWf := map[string]*yaml.Node{}
		for _, e := range mapEntries(root, "workflows") {
			srcWf[e.key] = e.val
		}
		for _, e := range mapEntries(root, "workflows") {
			cw, ok := c.workflows[e.key]
			if !ok {
				continue
			}
			res.Definitions = append(res.Definitions, def{Path: rel, Name: e.key, Line: e.line, Kind: "workflow"})
			inst := instances(srcWf[e.key], cw)
			for _, j := range cw {
				if job, ok := inst[j.name]; ok && declaredJobs[job] {
					attested[job] = true
					pairs[e.key+"\x00"+job] = true
				}
				for _, r := range j.requires {
					if job, ok := inst[r]; ok && declaredJobs[job] {
						pairs[e.key+"\x00"+job] = true
					}
				}
			}
		}
		for _, e := range jobs {
			if attested[e.key] || c.jobs[e.key] {
				res.Definitions = append(res.Definitions, def{Path: rel, Name: e.key, Line: e.line, Kind: "job"})
			}
		}
		var keys []string
		for k := range pairs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := strings.SplitN(k, "\x00", 2)
			res.References = append(res.References, ref{Path: rel, From: p[0], To: p[1]})
		}
		if *refs {
			cmds := map[string]bool{}
			for _, e := range mapEntries(root, "commands") {
				cmds[e.key] = true
			}
			execs := map[string]bool{}
			for _, e := range mapEntries(root, "executors") {
				execs[e.key] = true
			}
			seen := map[string]bool{}
			addRef := func(from, to string) {
				if k := from + "\x00" + to; !seen[k] {
					seen[k] = true
					res.References = append(res.References, ref{Path: rel, From: from, To: to})
				}
			}
			for _, e := range jobs {
				if x := executorName(e.val); execs[x] {
					addRef(e.key, x)
				}
				var names []string
				stepNames(e.val, &names)
				for _, n := range names {
					if cmds[n] {
						addRef(e.key, n)
					}
				}
			}
			for _, e := range mapEntries(root, "commands") {
				var names []string
				stepNames(e.val, &names)
				for _, n := range names {
					if cmds[n] {
						addRef(e.key, n)
					}
				}
			}
		}
	}
	if *refs {
		res.Definitions = []def{}
	} else {
		res.References = []ref{}
	}
	fmt.Fprintf(os.Stderr, "configs %d, accepted %d, rejected %d, definitions %d, references %d\n", len(cfgs), len(accepted), rejected, len(res.Definitions), len(res.References))
	if *filesOut != "" {
		_ = os.WriteFile(*filesOut, []byte(strings.Join(accepted, "\n")+"\n"), 0o644)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	_ = enc.Encode(res)
}
