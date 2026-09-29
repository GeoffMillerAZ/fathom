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
// `docker compose up` would (YAML merge keys, anchors and `extends:` resolved).
// It runs with --no-normalize, so no dependency is inferred from links:,
// volumes_from: or network_mode: service:x — those are real dependencies the
// tool would add to `depends_on`, but they are not references the file spells
// as depends_on, the only ones the dialect models. What the tool answers is which services the
// file defines and what each one depends on; `definitions` are those service
// names and `references` the depends_on pairs. The tool's JSON carries no
// source lines, and `include:`/`extends:` merge other files' services into
// the answer, so a yaml.v3 walk of the file itself supplies the line of each
// service key and drops any name the file does not itself declare (the
// merged-in ones belong to their own file's entry). A file the tool rejects
// (an invalid Compose file — a schema violation, a missing include) yields
// nothing and is counted on stderr; fathom still extracts what it can from
// such a file, which the reading scores as it stands.
//
// Tier C (-xref). `config` resolves `extends:` and erases it, so the file
// edges come from the raw YAML: `extends: {file: f}` (service -> file f,
// relative to the referring file's directory) and `build:` (service -> the
// Dockerfile the context and `dockerfile:` name, when it exists in the
// subject). The build edges land on files a doc-engine dialect never reads
// (a Dockerfile), so the harness drops them as out of scope (§1.4) and
// counts them — that count is the disclosure.
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

// config runs `docker compose config --format json` on one file.
func config(ctx context.Context, bin []string, file string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	args := append(append([]string(nil), bin[1:]...),
		"--profile", "*", "-f", file, "--project-directory", filepath.Dir(file),
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
	o := out{Definitions: []def{}, References: []ref{}}
	if *xrefMode {
		o.Tool, o.Version, o.Tier = "docker-compose-xref-oracle (independent yaml.v3 walk of extends.file and build)", "gopkg.in/yaml.v3 v3.0.1", "C"
		o.CrossReferences = []xref{}
	} else {
		o.Tool, o.Version, o.Tier = "docker compose config --format json", toolVersion(bin), "A"
	}
	var files, rejected, unparsable int
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
		if len(svcs) == 0 {
			return nil
		}
		j, err := config(ctx, bin, p)
		if err != nil {
			rejected++
			fmt.Fprintf(os.Stderr, "rejected by the tool: %s: %v\n", rel, err)
			return nil
		}
		files++
		toolSvcs, _ := j["services"].(map[string]any)
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
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "docker-compose-oracle:", err)
		os.Exit(1)
	}
	if *xrefMode {
		sort.Slice(o.CrossReferences, func(i, j int) bool {
			a, b := o.CrossReferences[i], o.CrossReferences[j]
			return a.Path+"\x00"+a.From+"\x00"+a.ToPath < b.Path+"\x00"+b.From+"\x00"+b.ToPath
		})
	}
	fmt.Fprintf(os.Stderr, "docker-compose-oracle: %d compose files answered, %d rejected by the tool, %d unparsable\n", files, rejected, unparsable)
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
