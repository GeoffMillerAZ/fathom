// Command docker-compose-oracle is the language-breadth harness's oracle for
// the docker-compose doc-engine dialect (design §1.3, owner decision D4). It
// writes indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10)
// in one of two modes:
//
//	go run . -subject <checkout>          # tier A: `docker compose config`
//	go run . -subject <checkout> -xref    # tier C: cross-file references
//
// Tier A. Each compose file is handed to the real tool —
// `docker compose config --format json` — which evaluates it the way
// `docker compose up` would (YAML merge keys, anchors, `include:` and
// `extends:` resolved).
// It runs with --no-normalize, so no dependency is inferred from links:,
// volumes_from: or network_mode: service:x — those are real dependencies the
// tool would add to `depends_on`, but they are not references the file spells
// as depends_on, the only ones the dialect models. What the tool answers is which services the
// file defines and what each one depends on; `definitions` are those service
// names and `references` the depends_on pairs. The tool's JSON carries no
// source lines, and `include:`/`extends:` merge other files' services into
// the answer, so a yaml.v3 walk of the file itself supplies the line of each
// service key and drops any name the file does not itself declare (the
// merged-in ones belong to their own file's entry). The tool names no file
// entity, so none is emitted: the dialect's root entity (ComposeFile, one per
// file) stays outside the L1 kind intersection and is checked through the
// cross-file references, every one of which lands on one. A file the tool
// rejects (an invalid Compose file — a schema violation, a missing include)
// yields no definition and is counted on stderr; fathom still extracts what
// it can from such a file, which the reading scores as it stands.
//
// The tier-A answer also carries the cross-file references the tool attests
// (`cross_references`, file granularity): an `include:` entry (the file, a
// `path:`, or each of a `path:` list) whose included file declares a service
// that appears in the tool's answer for the including file; an
// `extends: {file: f}` of a file the tool accepted (an extends whose file or
// service does not resolve fails the whole `config`); and a `build:` as the
// tool evaluated it (context, then `dockerfile:` relative to it, default
// `Dockerfile`) naming a Dockerfile of the subject. When the tool rejects the
// including file, each include and extends is probed alone: a scratch file
// beside it holding only that entry (an include with the included file's
// .env unread, as config's) is handed to the tool, and the entry is
// attested when the tool accepts it (and, for an include, answers one of the
// included file's services). The build edges land on files a doc-engine
// dialect never reads (a Dockerfile), so the harness drops them as out of
// scope (§1.4) and counts them.
//
// Tier C (-xref). The same edges from the raw YAML alone, unattested:
// `include:` (the including file -> each included file), `extends: {file: f}`
// (service -> file f, relative to the referring file's directory) and
// `build:` (service -> the Dockerfile the context and `dockerfile:` name, when
// it exists in the subject).
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
	"regexp"
	"sort"
	"strconv"
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
}

// includes is a file's include: entries as written, with their lines: a
// string, a `path:` string, or each string of a `path:` list.
func includes(root *yaml.Node) []string {
	var out []string
	inc := get(root, "include")
	if inc == nil || inc.Kind != yaml.SequenceNode {
		return nil
	}
	for _, e := range inc.Content {
		switch e.Kind {
		case yaml.ScalarNode:
			out = append(out, e.Value)
		case yaml.MappingNode:
			p := get(e, "path")
			if p == nil {
				continue
			}
			if p.Kind == yaml.ScalarNode {
				out = append(out, p.Value)
			} else if p.Kind == yaml.SequenceNode {
				for _, q := range p.Content {
					if q.Kind == yaml.ScalarNode {
						out = append(out, q.Value)
					}
				}
			}
		}
	}
	return out
}

// subjectFile is p (relative to the referring file rel) as a file of the
// subject, repo-relative; "" when it leaves the subject, is templated or
// does not exist.
func subjectFile(root, rel, p string) string {
	if p == "" || strings.Contains(p, "$") || strings.Contains(p, "://") {
		return ""
	}
	t := path.Clean(path.Join(path.Dir(rel), p))
	if t == "." || t == ".." || strings.HasPrefix(t, "../") {
		return ""
	}
	if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(t))); err != nil || st.IsDir() {
		return ""
	}
	return t
}

// declaredServices is the services a subject file itself declares.
func declaredServices(root, rel string) []string {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	var names []string
	for _, s := range services(parse(b)) {
		names = append(names, s.name)
	}
	return names
}

// probe hands the tool a scratch file beside file holding body alone and
// returns its answer (nil when the tool rejects it).
func probe(ctx context.Context, bin []string, file, body string) map[string]any {
	pf := filepath.Join(filepath.Dir(file), ".fathom-oracle-probe.yml")
	if err := os.WriteFile(pf, []byte(body), 0o644); err != nil {
		return nil
	}
	defer os.Remove(pf)
	j, err := config(ctx, bin, pf)
	if err != nil {
		return nil
	}
	return j
}

// configFile is config on file, or, when it includes other files, on a
// scratch copy beside it whose include entries leave the included files' .env
// unread (each entry in its long form with `env_file: /dev/null`), the same
// rule as --env-file for the including project: with --no-interpolate nothing
// in the answer depends on a .env, and an included project's .env requiring an
// unset variable (`${X?}`) would otherwise fail the including file.
func configFile(ctx context.Context, bin []string, file string, root *yaml.Node) (map[string]any, error) {
	inc := get(root, "include")
	if inc == nil || inc.Kind != yaml.SequenceNode {
		return config(ctx, bin, file)
	}
	for i, e := range inc.Content {
		switch e.Kind {
		case yaml.ScalarNode:
			inc.Content[i] = &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "path"}, {Kind: yaml.ScalarNode, Value: e.Value},
				{Kind: yaml.ScalarNode, Value: "env_file"}, {Kind: yaml.ScalarNode, Value: os.DevNull},
			}}
		case yaml.MappingNode:
			if get(e, "env_file") == nil {
				e.Content = append(e.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "env_file"}, &yaml.Node{Kind: yaml.ScalarNode, Value: os.DevNull})
			}
		}
	}
	b, err := yaml.Marshal(root)
	if err != nil {
		return config(ctx, bin, file)
	}
	pf := filepath.Join(filepath.Dir(file), ".fathom-oracle-probe.yml")
	if err := os.WriteFile(pf, b, 0o644); err != nil {
		return nil, err
	}
	defer os.Remove(pf)
	return config(ctx, bin, pf)
}

// anyIn reports whether one of names is a key of the tool's services.
func anyIn(names []string, toolSvcs map[string]any) bool {
	for _, n := range names {
		if _, ok := toolSvcs[n]; ok {
			return true
		}
	}
	return false
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references,omitempty"`
}

// composeName reports whether base names a file the dialect could own; it
// over-generates on purpose (the harness drops entries on files fathom does
// not read and counts them).
var composeName = regexp.MustCompile(`^(docker-)?compose([.-].*)?\.ya?ml$`)

var skipDirs = map[string]bool{".git": true, "vendor": true, "testdata": true, "generated": true, "node_modules": true, ".idea": true, ".vscode": true, ".claude": true}

func get(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

type declared struct {
	name string
	line int
	node *yaml.Node
}

// services is the file's own `services:` keys, in file order.
func services(root *yaml.Node) []declared {
	var out []declared
	sv := get(root, "services")
	if sv == nil || sv.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(sv.Content); i += 2 {
		k, v := sv.Content[i], sv.Content[i+1]
		if v.Kind == yaml.AliasNode && v.Alias != nil {
			v = v.Alias
		}
		out = append(out, declared{k.Value, k.Line, v})
	}
	return out
}

func parse(b []byte) *yaml.Node {
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(b)).Decode(&doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind == yaml.AliasNode && root.Alias != nil {
		root = root.Alias
	}
	if root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}

func composeBin() []string {
	if p, err := exec.LookPath("docker"); err == nil {
		return []string{p, "compose"}
	}
	if p, err := exec.LookPath("docker-compose"); err == nil {
		return []string{p}
	}
	return nil
}

func toolVersion(bin []string) string {
	args := append(append([]string(nil), bin[1:]...), "version", "--short")
	b, err := exec.Command(bin[0], args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

// config runs `docker compose config --format json` on one file. The
// project's .env is not read (--env-file os.DevNull): with --no-interpolate
// nothing in the answer depends on it, and a .env whose own values require an
// unset variable (`${X?}`) would otherwise fail the file.
func config(ctx context.Context, bin []string, file string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	args := append(append([]string(nil), bin[1:]...),
		"--env-file", os.DevNull, "--profile", "*", "-f", file, "--project-directory", filepath.Dir(file),
		"config", "--format", "json",
		"--no-interpolate", "--no-env-resolution", "--no-path-resolution", "--no-consistency", "--no-normalize")
	cmd := exec.CommandContext(ctx, bin[0], args...)
	cmd.Dir = filepath.Dir(file)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if i := strings.LastIndex(msg, "\n"); i >= 0 {
			msg = msg[i+1:]
		}
		return nil, fmt.Errorf("%v: %s", err, msg)
	}
	var j map[string]any
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	return j, nil
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	xrefMode := flag.Bool("xref", false, "emit the tier-C cross-file references instead of the tier-A tool answer")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: docker-compose-oracle -subject <dir> [-xref]")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bin := composeBin()
	if bin == nil && !*xrefMode {
		fmt.Fprintln(os.Stderr, "docker-compose-oracle: neither `docker compose` nor docker-compose on PATH")
		os.Exit(1)
	}
	o := out{Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}}
	if *xrefMode {
		o.Tool, o.Version, o.Tier = "docker-compose-xref-oracle (independent yaml.v3 walk of include, extends.file and build)", "gopkg.in/yaml.v3 v3.0.1", "C"
	} else {
		o.Tool, o.Version, o.Tier = "docker compose config --format json", toolVersion(bin), "A"
	}
	var files, rejected, unparsable, probed int
	ctx := context.Background()
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !composeName.MatchString(d.Name()) {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rootNode := parse(src)
		if rootNode == nil {
			unparsable++
			return nil
		}
		svcs := services(rootNode)
		if *xrefMode {
			for _, inc := range includes(rootNode) {
				if t := subjectFile(root, rel, inc); t != "" {
					o.CrossReferences = append(o.CrossReferences, xref{rel, d.Name(), t})
				}
			}
			for _, s := range svcs {
				if s.node.Kind != yaml.MappingNode {
					continue
				}
				if f := scalar(get(get(s.node, "extends"), "file")); f != "" {
					o.CrossReferences = append(o.CrossReferences, xref{rel, s.name, path.Clean(path.Join(path.Dir(rel), f))})
				}
				if t := buildTarget(root, rel, s.node); t != "" {
					o.CrossReferences = append(o.CrossReferences, xref{rel, s.name, t})
				}
			}
			files++
			return nil
		}
		j, err := configFile(ctx, bin, p, rootNode)
		if err != nil {
			rejected++
			fmt.Fprintf(os.Stderr, "rejected by the tool: %s: %v\n", rel, err)
			// each include and extends is probed alone
			for _, inc := range includes(rootNode) {
				t := subjectFile(root, rel, inc)
				if t == "" {
					continue
				}
				// the included file's own .env is not read either (config's rule)
				if pj := probe(ctx, bin, p, "include:\n  - path: "+strconv.Quote(inc)+"\n    env_file: "+strconv.Quote(os.DevNull)+"\n"); pj != nil {
					ps, _ := pj["services"].(map[string]any)
					if anyIn(declaredServices(root, t), ps) {
						o.CrossReferences = append(o.CrossReferences, xref{rel, d.Name(), t})
						probed++
					}
				}
			}
			for _, s := range svcs {
				ext := get(s.node, "extends")
				f, svc := scalar(get(ext, "file")), scalar(get(ext, "service"))
				t := subjectFile(root, rel, f)
				if t == "" || svc == "" {
					continue
				}
				body := "services:\n  " + strconv.Quote(s.name) + ":\n    extends:\n      file: " + strconv.Quote(f) + "\n      service: " + strconv.Quote(svc) + "\n"
				if probe(ctx, bin, p, body) != nil {
					o.CrossReferences = append(o.CrossReferences, xref{rel, s.name, t})
					probed++
				}
			}
			return nil
		}
		files++
		toolSvcs, _ := j["services"].(map[string]any)
		for _, inc := range includes(rootNode) {
			if t := subjectFile(root, rel, inc); t != "" && anyIn(declaredServices(root, t), toolSvcs) {
				o.CrossReferences = append(o.CrossReferences, xref{rel, d.Name(), t})
			}
		}
		for _, s := range svcs {
			sj, ok := toolSvcs[s.name].(map[string]any)
			if !ok {
				continue
			}
			o.Definitions = append(o.Definitions, def{rel, s.name, s.line, "service"})
			names := dependsOn(sj["depends_on"])
			for _, n := range names {
				o.References = append(o.References, ref{rel, s.name, n})
			}
			// the tool resolved the extends (it fails the file otherwise)
			if t := subjectFile(root, rel, scalar(get(get(s.node, "extends"), "file"))); t != "" {
				o.CrossReferences = append(o.CrossReferences, xref{rel, s.name, t})
			}
			if t := toolBuildTarget(root, rel, sj["build"]); t != "" {
				o.CrossReferences = append(o.CrossReferences, xref{rel, s.name, t})
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "docker-compose-oracle:", err)
		os.Exit(1)
	}
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath < b.Path+"\x00"+b.From+"\x00"+b.ToPath
	})
	fmt.Fprintf(os.Stderr, "docker-compose-oracle: %d compose files answered, %d rejected by the tool (%d of their cross-file references attested by a probe), %d unparsable, %d cross-file references\n", files, rejected, probed, unparsable, len(o.CrossReferences))
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
	}
}

// dependsOn is a service's declared dependencies as the tool evaluated them:
// with --no-normalize the short list form stays a list and the long form a
// map keyed by service name (normalising would also add the dependencies the
// tool infers from links:, volumes_from: and network_mode: service:x — real
// dependencies, but not references the file spells as depends_on).
func dependsOn(v any) []string {
	var names []string
	switch d := v.(type) {
	case []any:
		for _, e := range d {
			if s, ok := e.(string); ok {
				names = append(names, s)
			}
		}
	case map[string]any:
		for n := range d {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// toolBuildTarget is the Dockerfile a service's build names as the tool
// evaluated it (the build object of its answer), repo-relative, when that
// file exists in the subject; "" otherwise.
func toolBuildTarget(root, rel string, b any) string {
	m, ok := b.(map[string]any)
	if !ok {
		return ""
	}
	ctxDir, _ := m["context"].(string)
	dockerfile, _ := m["dockerfile"].(string)
	n := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "build"},
		{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "context"}, {Kind: yaml.ScalarNode, Value: ctxDir},
			{Kind: yaml.ScalarNode, Value: "dockerfile"}, {Kind: yaml.ScalarNode, Value: dockerfile},
		}},
	}}
	return buildTarget(root, rel, n)
}

// buildTarget is the Dockerfile a service's build names (context, then
// dockerfile:, default "Dockerfile"), repo-relative, when that file exists in
// the subject; "" otherwise (a remote or templated context, a missing file).
func buildTarget(root, rel string, svc *yaml.Node) string {
	b := get(svc, "build")
	if b == nil {
		return ""
	}
	ctxDir, dockerfile := scalar(b), ""
	if b.Kind == yaml.MappingNode {
		ctxDir, dockerfile = scalar(get(b, "context")), scalar(get(b, "dockerfile"))
	}
	if ctxDir == "" {
		ctxDir = "."
	}
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	if strings.Contains(ctxDir+dockerfile, "${") || strings.Contains(ctxDir, "://") || strings.HasPrefix(ctxDir, "git@") {
		return ""
	}
	target := path.Clean(path.Join(path.Dir(rel), ctxDir, dockerfile))
	if target == ".." || strings.HasPrefix(target, "../") {
		return ""
	}
	if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err != nil || st.IsDir() {
		return ""
	}
	return target
}
