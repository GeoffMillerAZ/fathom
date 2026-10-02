// Command pulumi-yaml-oracle is the language-breadth harness's oracle for the
// pulumi-yaml doc-engine dialect (LB-Q-pulumi-yaml-rest), tier A: every entity
// and reference is one Pulumi's own code decides, linked offline as Go
// libraries — nothing is read off the YAML by a rule written here.
//
//   - The project: Pulumi's project loader, the one the pulumi CLI loads every
//     Pulumi.yaml with (github.com/pulumi/pulumi/sdk/v3 go/common/workspace
//     LoadProject: the project JSON schema, then the typed Project's
//     Validate). A file it refuses yields no entity. It names the project (a
//     `package`, by `name`, when the typed project has a runtime: the
//     dialect's rule) and each `config` key (`variable_decl`).
//   - The program: pulumi-yaml's own template parse, the one the YAML language
//     host runs (github.com/pulumi/pulumi-yaml pkg/pulumiyaml LoadYAMLBytes,
//     v1.36.0, the language host the pulumi CLI 3.247.0 ships): each
//     `variables` entry (`variable`), each `resources` entry with a `type`
//     (`resource`), each `outputs` entry (`output`). A template it refuses
//     yields no program entity.
//   - References: pulumi-yaml's interpolation grammar (its parsed
//     SymbolExpr and InterpolateExpr nodes, `$${` a literal) in every
//     expression of a resource, a variable or an output, from that entity to
//     the resource, variable or config key its root names; the builtin
//     `pulumi` and an undeclared root are not references, nor is a
//     declaration naming itself (an output may).
//
// Lines come from an independent YAML reader (github.com/goccy/go-yaml, its
// own scanner and parser): pulumi-yaml's YAML layer is gopkg.in/yaml.v3, the
// decoder the dialect's doc engine reads with, while its program model and
// interpolation grammar are Pulumi's. Each key line is cross-checked against
// pulumi-yaml's own syntax range (mismatches on stderr).
//
// Stack files (Pulumi.<stack>.yaml) are counted and not attested.
// Bench-only; never a product dependency.
//
//	go build -o <bin> . && <bin> -subject <checkout> [-files-out <txt>] > <out.json>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	gast "github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	yast "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end,omitempty"`
	Kind string `json:"kind"`
}

type ref struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type stats struct {
	files, projects, stacks, refused, programRefused, lineMismatch int
	refusedFiles                                                   []string
}

type output struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
	References  []ref  `json:"references"`
	stats       stats
	accepted    []string
}

var (
	skipDirs = map[string]bool{"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true, ".idea": true, ".vscode": true, ".claude": true}
	project  = regexp.MustCompile(`^Pulumi\.ya?ml$`)
	stack    = regexp.MustCompile(`^Pulumi\.[^/]+\.ya?ml$`)
)

func modVersion(path string) string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == path {
				return d.Version
			}
		}
	}
	return "(unknown)"
}

// readRetry reads a file, retrying a transient EPERM of the external volume.
func readRetry(p string) ([]byte, error) {
	var b []byte
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		if b, err = os.ReadFile(p); err == nil || !strings.Contains(err.Error(), "not permitted") {
			return b, err
		}
		time.Sleep(time.Second)
	}
	return b, err
}

// keyLines is the independent reader's view of one file: each top-level
// key's line, each section entry's key line and last line.
type keyLines struct {
	first, last int
	top         map[string]int
	section     map[string]map[string][2]int
}

func lastLine(n gast.Node) int {
	max := 0
	gast.Walk(visitor(func(x gast.Node) {
		if t := x.GetToken(); t != nil && t.Position != nil {
			l := t.Position.Line
			if s, ok := x.(*gast.LiteralNode); ok && s.Value != nil {
				l += strings.Count(strings.TrimRight(s.Value.Value, "\n"), "\n") + 1
			}
			if l > max {
				max = l
			}
		}
	}), n)
	return max
}

type visitor func(gast.Node)

func (v visitor) Visit(n gast.Node) gast.Visitor {
	v(n)
	return v
}

func mappingValues(n gast.Node) []*gast.MappingValueNode {
	switch m := n.(type) {
	case *gast.MappingNode:
		return m.Values
	case *gast.MappingValueNode:
		return []*gast.MappingValueNode{m}
	case *gast.TagNode:
		return mappingValues(m.Value)
	case *gast.AnchorNode:
		return mappingValues(m.Value)
	}
	return nil
}

func keyName(mv *gast.MappingValueNode) string {
	if mv.Key == nil {
		return ""
	}
	if s, ok := mv.Key.(gast.ScalarNode); ok {
		return fmt.Sprint(s.GetValue())
	}
	return mv.Key.String()
}

func readLines(b []byte) (keyLines, error) {
	kl := keyLines{top: map[string]int{}, section: map[string]map[string][2]int{}}
	f, err := parser.ParseBytes(b, 0)
	if err != nil {
		return kl, err
	}
	if len(f.Docs) == 0 || f.Docs[0].Body == nil {
		return kl, fmt.Errorf("no document")
	}
	for i, mv := range mappingValues(f.Docs[0].Body) {
		k := keyName(mv)
		line := mv.Key.GetToken().Position.Line
		if i == 0 {
			kl.first = line
		}
		if _, ok := kl.top[k]; !ok {
			kl.top[k] = line
		}
		if l := lastLine(mv); l > kl.last {
			kl.last = l
		}
		sec := map[string][2]int{}
		for _, e := range mappingValues(mv.Value) {
			name := keyName(e)
			if _, ok := sec[name]; !ok {
				el := e.Key.GetToken().Position.Line
				sec[name] = [2]int{el, max(el, lastLine(e))}
			}
		}
		if _, ok := kl.section[k]; !ok {
			kl.section[k] = sec
		}
	}
	return kl, nil
}

// roots collects every interpolation root an expression tree names, as
// pulumi-yaml parsed it.
func roots(v reflect.Value, out map[string]bool) {
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			roots(v.Elem(), out)
		}
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		switch x := v.Interface().(type) {
		case *yast.SymbolExpr:
			if x.Property != nil {
				out[x.Property.RootName()] = true
			}
		case *yast.InterpolateExpr:
			for _, p := range x.Parts {
				if p.Value != nil {
					out[p.Value.RootName()] = true
				}
			}
		}
		roots(v.Elem(), out)
	case reflect.Struct:
		if !strings.HasSuffix(v.Type().PkgPath(), "pulumiyaml/ast") {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				roots(v.Field(i), out)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			roots(v.Index(i), out)
		}
	}
}

func syntaxLine(e *yast.StringExpr) int {
	if e == nil || e.Syntax() == nil || e.Syntax().Syntax() == nil {
		return 0
	}
	if r := e.Syntax().Syntax().Range(); r != nil {
		return r.Start.Line
	}
	return 0
}

// sentinelName and sentinelDescription stand for ${PROJECT} and
// ${DESCRIPTION} when the file is instantiated the way `pulumi new` does.
const (
	sentinelName        = "fathom-oracle-project"
	sentinelDescription = "fathom oracle description"
)

// instantiate is `pulumi new`'s own step for a template: the pulumi CLI's
// workspace.CopyTemplateFiles copies the file's directory to a scratch
// directory, replacing ${PROJECT} and ${DESCRIPTION} (a file without them is
// copied unchanged). It returns the instantiated Pulumi.yaml and its bytes.
// The substitution can move lines (a description is written as a YAML
// string), so lines are read from the file as written.
func instantiate(root, path string) (string, []byte, error) {
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		dest, err := os.MkdirTemp(root, "t")
		if err != nil {
			return "", nil, err
		}
		inst := filepath.Join(dest, "t")
		if err := os.Mkdir(inst, 0o700); err != nil {
			return "", nil, err
		}
		src := filepath.Join(dest, "src")
		if err := os.Mkdir(src, 0o700); err != nil {
			return "", nil, err
		}
		b, err := readRetry(path)
		if err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(filepath.Join(src, filepath.Base(path)), b, 0o644); err != nil {
			return "", nil, err
		}
		if err := workspace.CopyTemplateFiles(src, inst, true, sentinelName, sentinelDescription); err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "not permitted") {
				time.Sleep(time.Second)
				continue
			}
			return "", nil, err
		}
		out := filepath.Join(inst, filepath.Base(path))
		ib, err := readRetry(out)
		return out, ib, err
	}
	return "", nil, lastErr
}

func run(subject string) (*output, error) {
	o := &output{Tier: "A", Definitions: []def{}, References: []ref{}}
	var files []string
	err := filepath.WalkDir(subject, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && p != subject {
				return filepath.SkipDir
			}
			return nil
		}
		if project.MatchString(d.Name()) || stack.MatchString(d.Name()) {
			rel, _ := filepath.Rel(subject, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	scratch, err := os.MkdirTemp("", "pulumi-yaml-oracle-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	refSeen := map[ref]bool{}
	for _, rel := range files {
		o.stats.files++
		if !project.MatchString(filepath.Base(rel)) {
			o.stats.stacks++
			continue
		}
		o.stats.projects++
		orig, err := readRetry(filepath.Join(subject, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		abs, b, err := instantiate(scratch, filepath.Join(subject, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		var proj *workspace.Project
		for attempt := 0; attempt < 8; attempt++ {
			if proj, err = workspace.LoadProject(abs); err == nil || !strings.Contains(err.Error(), "not permitted") {
				break
			}
			time.Sleep(time.Second)
		}
		if err != nil {
			o.stats.refused++
			o.stats.refusedFiles = append(o.stats.refusedFiles, rel+": "+firstLine(err.Error()))
			continue
		}
		kl, err := readLines(orig) // lines of the file as written
		// pulumi-yaml's own ranges cross-check the lines where instantiation
		// changed nothing.
		checkLines := string(orig) == string(b)
		if err != nil {
			o.stats.refused++
			o.stats.refusedFiles = append(o.stats.refusedFiles, rel+": the line reader: "+firstLine(err.Error()))
			continue
		}
		o.accepted = append(o.accepted, rel)
		if proj.Runtime.Name() != "" { // the dialect's rule: a project with a runtime is a package
			name := proj.Name.String()
			if name == sentinelName { // pulumi new's substitution, undone: the name as written
				name = "${PROJECT}"
			}
			o.Definitions = append(o.Definitions, def{rel, name, kl.first, kl.last, "package"})
		}
		declared := map[string]bool{}
		var configKeys []string
		for k := range proj.Config {
			configKeys = append(configKeys, k)
		}
		sort.Strings(configKeys)
		for _, k := range configKeys {
			if l, ok := kl.section["config"][k]; ok {
				o.Definitions = append(o.Definitions, def{rel, k, l[0], l[1], "variable_decl"})
				declared[k] = true
			}
		}
		tmpl, diags, err := pulumiyaml.LoadYAMLBytes(rel, b)
		if err != nil || tmpl == nil || diags.HasErrors() {
			o.stats.programRefused++
			msg := fmt.Sprint(err)
			if err == nil {
				msg = firstLine(strings.TrimSpace(diags.Error()))
			}
			o.stats.refusedFiles = append(o.stats.refusedFiles, rel+" (program): "+msg)
			continue
		}
		type entity struct {
			name, kind string
			key        *yast.StringExpr
			value      any
			section    string
		}
		var ents []entity
		for _, e := range tmpl.Variables.Entries {
			ents = append(ents, entity{e.Key.Value, "variable", e.Key, e.Value, "variables"})
		}
		for _, e := range tmpl.Resources.Entries {
			if e.Value == nil || e.Value.Type == nil || e.Value.Type.Value == "" {
				continue
			}
			ents = append(ents, entity{e.Key.Value, "resource", e.Key, e.Value, "resources"})
		}
		for _, e := range tmpl.Outputs.Entries {
			ents = append(ents, entity{e.Key.Value, "output", e.Key, e.Value, "outputs"})
		}
		for _, e := range tmpl.Config.Entries {
			if sl := syntaxLine(e.Key); sl > 0 && checkLines {
				if l, ok := kl.section["config"][e.Key.Value]; ok && l[0] != sl {
					o.stats.lineMismatch++
				}
			}
		}
		for _, e := range ents {
			if e.kind != "output" {
				declared[e.name] = true
			}
		}
		for _, e := range ents {
			l, ok := kl.section[e.section][e.name]
			if !ok {
				continue
			}
			if sl := syntaxLine(e.key); checkLines && sl > 0 && sl != l[0] {
				o.stats.lineMismatch++
			}
			o.Definitions = append(o.Definitions, def{rel, e.name, l[0], l[1], e.kind})
			rs := map[string]bool{}
			roots(reflect.ValueOf(e.value), rs)
			var names []string
			for r := range rs {
				names = append(names, r)
			}
			sort.Strings(names)
			for _, r := range names {
				if !declared[r] || (r == e.name && e.kind != "output") {
					continue
				}
				x := ref{rel, e.name, r}
				if !refSeen[x] {
					refSeen[x] = true
					o.References = append(o.References, x)
				}
			}
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
	return o, nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func main() {
	subj := flag.String("subject", "", "subject checkout")
	filesOut := flag.String("files-out", "", "write the project files the loaders accept here (lang-oracle -only)")
	flag.Parse()
	if *subj == "" {
		fmt.Fprintln(os.Stderr, "usage: pulumi-yaml-oracle -subject <dir> [-files-out <txt>] > <out.json>")
		os.Exit(2)
	}
	o, err := run(*subj)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pulumi-yaml-oracle:", err)
		os.Exit(1)
	}
	o.Version = "pulumi-yaml " + modVersion("github.com/pulumi/pulumi-yaml") + ", pulumi sdk " + modVersion("github.com/pulumi/pulumi/sdk/v3") + ", goccy/go-yaml " + modVersion("github.com/goccy/go-yaml")
	o.Tool = "Pulumi's own loaders (workspace.LoadProject, pulumiyaml.LoadYAMLBytes and its interpolation parse) + an independent YAML reader for lines"
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
	}
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(o.accepted, "\n")+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "pulumi-yaml-oracle:", err)
			os.Exit(1)
		}
	}
	s := o.stats
	fmt.Fprintf(os.Stderr, "pulumi-yaml-oracle: %d files (%d project, %d stack, not attested); %d refused by the project loader, %d programs refused by pulumi-yaml; %d definitions, %d references; %d key lines disagree with pulumi-yaml's own ranges\n",
		s.files, s.projects, s.stacks, s.refused, s.programRefused, len(o.Definitions), len(o.References), s.lineMismatch)
	for _, r := range s.refusedFiles {
		fmt.Fprintln(os.Stderr, "  refused", r)
	}
}
