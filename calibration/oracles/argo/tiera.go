package main

// tiera.go — the tier-A path for the Workflow kinds (wave ox, LB-F155).
//
//   - Acceptance: argo v4.1.4's own `argo lint --offline`, which resolves a
//     templateRef or workflowTemplateRef only from the files it is given. Each
//     Workflow-kind file is linted with the subject's WorkflowTemplate and
//     ClusterWorkflowTemplate files beside it (those lint accepts on their
//     own); a file it rejects (a reference to a template the subject does not
//     hold, an invalid spec) contributes nothing, is left out of -files-out
//     and is counted.
//   - Definitions: argo-workflows v4.1.4's own v1alpha1 types decode every
//     document strictly (sigs.k8s.io/yaml.UnmarshalStrict: an unknown field
//     fails the file), and the entities are the decoded objects': the object,
//     its templates, steps, dag tasks and container images (the envoy README's
//     argument: the format's own types as a library). Lines come from the
//     gopkg.in/yaml.v3 walk: an entity is kept when the walk places one of
//     the same kind and name in that file; one the types carry and the walk
//     cannot place is unlocated, one the walk has and the types do not is
//     disagreement, both counted, never guessed.
//   - References (a step's or task's template, a task's dependencies, an
//     entrypoint, an onExit) are attested by ablation: the value at the
//     reference's site is renamed in a scratch copy and lint must fail.
//   - Cross-references (templateRef, workflowTemplateRef) are attested by
//     ablation: lint accepts the file with the target's file beside it and
//     rejects it without.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	wfv1 "github.com/argoproj/argo-workflows/v4/pkg/apis/workflow/v1alpha1"
	"gopkg.in/yaml.v3"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	sigsyaml "sigs.k8s.io/yaml"
)

// tent is one entity of the typed decode.
type tent struct{ kind, name string }

// typedEntities strictly decodes every document of a Workflow-kind file with
// argo-workflows' own types and lists the entities the dialect claims.
func typedEntities(b []byte) ([]tent, error) {
	var out []tent
	r := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(b)))
	for {
		doc, err := r.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		var probe map[string]any
		if err := sigsyaml.Unmarshal(doc, &probe); err != nil {
			return nil, err
		}
		if len(probe) == 0 {
			continue
		}
		kind, _ := probe["kind"].(string)
		var meta *wfv1.WorkflowSpec
		var name, gen, ns string
		switch kind {
		case "Workflow":
			var o wfv1.Workflow
			if err := sigsyaml.UnmarshalStrict(doc, &o); err != nil {
				return nil, err
			}
			meta, name, gen, ns = &o.Spec, o.Name, o.GenerateName, o.Namespace
		case "WorkflowTemplate":
			var o wfv1.WorkflowTemplate
			if err := sigsyaml.UnmarshalStrict(doc, &o); err != nil {
				return nil, err
			}
			meta, name, gen, ns = &o.Spec, o.Name, o.GenerateName, o.Namespace
		case "ClusterWorkflowTemplate":
			var o wfv1.ClusterWorkflowTemplate
			if err := sigsyaml.UnmarshalStrict(doc, &o); err != nil {
				return nil, err
			}
			meta, name, gen, ns = &o.Spec, o.Name, o.GenerateName, o.Namespace
		case "CronWorkflow":
			var o wfv1.CronWorkflow
			if err := sigsyaml.UnmarshalStrict(doc, &o); err != nil {
				return nil, err
			}
			meta, name, gen, ns = &o.Spec.WorkflowSpec, o.Name, o.GenerateName, o.Namespace
		default:
			return nil, fmt.Errorf("not a Workflow kind: %q", kind)
		}
		if name == "" {
			name = gen
		}
		if name != "" {
			out = append(out, tent{"resource", qual(kind, ns, name)})
		}
		var walk func(t *wfv1.Template, named bool)
		walk = func(t *wfv1.Template, named bool) {
			if named && t.Name != "" {
				out = append(out, tent{"task", t.Name})
			}
			for _, ps := range t.Steps {
				for i := range ps.Steps {
					s := &ps.Steps[i]
					if s.Name != "" {
						out = append(out, tent{"step", s.Name})
					}
					if s.Inline != nil {
						walk(s.Inline, false)
					}
				}
			}
			if t.DAG != nil {
				for i := range t.DAG.Tasks {
					k := &t.DAG.Tasks[i]
					if k.Name != "" {
						out = append(out, tent{"job", k.Name})
					}
					if k.Inline != nil {
						walk(k.Inline, false)
					}
				}
			}
			if t.Container != nil && t.Container.Image != "" {
				out = append(out, tent{"data_source", t.Container.Image})
			}
			if t.Script != nil && t.Script.Image != "" {
				out = append(out, tent{"data_source", t.Script.Image})
			}
			if t.ContainerSet != nil {
				for _, c := range t.ContainerSet.Containers {
					if c.Image != "" {
						out = append(out, tent{"data_source", c.Image})
					}
				}
			}
			for _, list := range [][]wfv1.UserContainer{t.Sidecars, t.InitContainers} {
				for _, c := range list {
					if c.Image != "" {
						out = append(out, tent{"data_source", c.Image})
					}
				}
			}
		}
		for i := range meta.Templates {
			if meta.Templates[i].Name != "" {
				walk(&meta.Templates[i], true)
			}
		}
		// a workflow-level executor plugin's sidecar container
		for _, pl := range meta.ExecutorPlugins {
			if img := pl.Spec.Sidecar.Container.Image; img != "" {
				out = append(out, tent{"data_source", img})
			}
		}
	}
}

// replaceScalar rewrites the scalar starting at (line, col), both 1-based as
// yaml.v3 reports them; a quoted scalar is replaced quotes and all.
func replaceScalar(src string, line, col int, repl string) (string, bool) {
	off := 0
	for l := 1; l < line; l++ {
		i := strings.IndexByte(src[off:], '\n')
		if i < 0 {
			return src, false
		}
		off += i + 1
	}
	off += col - 1
	if off >= len(src) {
		return src, false
	}
	end := off
	switch q := src[off]; q {
	case '"', '\'':
		j := strings.IndexByte(src[off+1:], q)
		if j < 0 {
			return src, false
		}
		end = off + 1 + j + 1
		return src[:off] + strconv.Quote(repl) + src[end:], true
	}
	for end < len(src) && src[end] != '\n' && !(src[end] == '#' && end > off && src[end-1] == ' ') {
		end++
	}
	for end > off && (src[end-1] == ' ' || src[end-1] == '\t') {
		end--
	}
	if end == off {
		return src, false
	}
	return src[:off] + repl + src[end:], true
}

// lintFiles runs `argo lint --offline` over files; an error carries the
// tool's output. An "operation not permitted" (a revoked volume grant) is
// retried after ORACLE_TCC_RESET, never read as a rejection.
func lintFiles(bin string, files []string) error {
	var last error
	for i := 0; i < 6; i++ {
		cmd := exec.Command(bin, append([]string{"lint", "--offline", "--output", "simple"}, files...)...)
		cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null")
		outb, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		last = fmt.Errorf("%v: %s", err, strings.Join(strings.Fields(string(outb)), " "))
		if !strings.Contains(strings.ToLower(string(outb)), "operation not permitted") {
			return last
		}
		if c := os.Getenv("ORACLE_TCC_RESET"); c != "" {
			_ = exec.Command("sh", "-c", c).Run()
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return last
}

// astats are the tier-A counts, printed on stderr.
type astats struct {
	files, rejected, strict                  int
	unlocated, walkOnly                      int
	refs, refsAttested, xrefs, xrefsAttested int
	refSiteMissing                           int
	contextTemplates                         int
	rejectedFiles                            []string
}

type aresult struct {
	defs  []def
	refs  []ref
	xrefs []xref
	files []string
	st    astats
}

// refSite is where a reference's value is written.
type refSite struct {
	from, to string
	node     *yaml.Node
}

// refSites lists the reference sites of one file's documents, in the walk's
// naming (main.go): a step's or task's template, a task's dependencies, an
// object's entrypoint and onExit.
func refSites(roots []*yaml.Node) []refSite {
	var out []refSite
	for _, r := range roots {
		kind := scalar(get(r, "kind"))
		name := scalar(at(r, "metadata", "name"))
		if name == "" {
			name = scalar(at(r, "metadata", "generateName"))
		}
		obj := qual(kind, scalar(at(r, "metadata", "namespace")), name)
		for _, p := range [][]string{{"spec", "entrypoint"}, {"spec", "workflowSpec", "entrypoint"}, {"spec", "onExit"}} {
			if n := at(r, p...); n != nil && n.Kind == yaml.ScalarNode && name != "" {
				out = append(out, refSite{obj, n.Value, n})
			}
		}
		var tmpls []*yaml.Node
		tmpls = append(tmpls, seq(at(r, "spec", "templates"))...)
		tmpls = append(tmpls, seq(at(r, "spec", "workflowSpec", "templates"))...)
		var walk func(t *yaml.Node)
		walk = func(t *yaml.Node) {
			for _, grp := range seq(get(t, "steps")) {
				for _, s := range items(grp) {
					if n := get(s, "template"); n != nil && n.Kind == yaml.ScalarNode {
						out = append(out, refSite{scalar(get(s, "name")), n.Value, n})
					}
					if in := get(s, "inline"); in != nil {
						walk(in)
					}
				}
			}
			for _, k := range seq(at(t, "dag", "tasks")) {
				if n := get(k, "template"); n != nil && n.Kind == yaml.ScalarNode {
					out = append(out, refSite{scalar(get(k, "name")), n.Value, n})
				}
				for _, d := range seq(get(k, "dependencies")) {
					out = append(out, refSite{scalar(get(k, "name")), d.Value, d})
				}
				if in := get(k, "inline"); in != nil {
					walk(in)
				}
			}
		}
		for _, t := range tmpls {
			walk(t)
		}
	}
	return out
}

// tierA measures the Workflow-kind files of the subject; o is the walk's
// output and lintable its Workflow-kind files.
func tierA(subject, bin, scratch string, o out, lintable []string) (aresult, error) {
	var res aresult
	abs := func(rel string) string { return filepath.Join(subject, filepath.FromSlash(rel)) }
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		return res, err
	}
	src := map[string][]byte{}
	typed := map[string][]tent{}
	for _, rel := range lintable {
		b, err := os.ReadFile(abs(rel))
		if err != nil {
			return res, err
		}
		src[rel] = b
		ts, err := typedEntities(b)
		if err != nil {
			res.st.strict++
			fmt.Fprintf(os.Stderr, "argo-oracle: strict decode refuses %s: %v\n", rel, err)
			continue
		}
		typed[rel] = ts
	}
	// context: the template files lint accepts on their own
	var ctx []string
	for _, rel := range lintable {
		isTmpl := false
		for _, t := range typed[rel] {
			if t.kind == "resource" && (strings.HasPrefix(t.name, "WorkflowTemplate/") || strings.HasPrefix(t.name, "ClusterWorkflowTemplate/")) {
				isTmpl = true
			}
		}
		if isTmpl && lintFiles(bin, []string{abs(rel)}) == nil {
			ctx = append(ctx, rel)
		}
	}
	res.st.contextTemplates = len(ctx)
	with := func(file string, skip ...string) []string {
		fs := []string{file}
		for _, c := range ctx {
			if abs(c) != file && !contains(skip, c) {
				fs = append(fs, abs(c))
			}
		}
		return fs
	}
	accepted := map[string]bool{}
	for _, rel := range lintable {
		res.st.files++
		if _, ok := typed[rel]; !ok {
			res.st.rejected++
			res.st.rejectedFiles = append(res.st.rejectedFiles, rel+" (strict decode)")
			continue
		}
		if err := lintFiles(bin, with(abs(rel))); err != nil {
			res.st.rejected++
			res.st.rejectedFiles = append(res.st.rejectedFiles, rel)
			fmt.Fprintf(os.Stderr, "argo-oracle: argo lint rejects %s: %.300s\n", rel, err)
			continue
		}
		accepted[rel] = true
		res.files = append(res.files, rel)
	}
	// definitions: the typed entities, placed by the walk
	byFile := map[string][]def{}
	for _, d := range o.Definitions {
		if accepted[d.Path] {
			byFile[d.Path] = append(byFile[d.Path], d)
		}
	}
	for _, rel := range res.files {
		want := map[tent]int{}
		for _, t := range typed[rel] {
			want[t]++
		}
		for _, d := range byFile[rel] {
			k := tent{d.Kind, d.Name}
			if want[k] > 0 {
				want[k]--
				res.defs = append(res.defs, d)
			} else {
				res.st.walkOnly++
			}
		}
		for k, n := range want {
			if n > 0 {
				res.st.unlocated += n
				fmt.Fprintf(os.Stderr, "argo-oracle: unlocated %s %s %s (%d)\n", rel, k.kind, k.name, n)
			}
		}
	}
	// references, by ablation at their site
	sites := map[string][]refSite{}
	for _, rel := range res.files {
		roots, _ := decode(src[rel])
		sites[rel] = refSites(roots)
	}
	n := 0
	for _, r := range o.References {
		if !accepted[r.Path] {
			continue
		}
		res.st.refs++
		var site *refSite
		for i := range sites[r.Path] {
			s := &sites[r.Path][i]
			if s.from == r.From && s.to == r.To {
				site = s
				break
			}
		}
		if site == nil {
			res.st.refSiteMissing++
			continue
		}
		n++
		token := fmt.Sprintf("ox-absent-%d", n)
		ablated, ok := replaceScalar(string(src[r.Path]), site.node.Line, site.node.Column, token)
		if !ok {
			res.st.refSiteMissing++
			continue
		}
		tmp := filepath.Join(scratch, fmt.Sprintf("ref-%d-%s", n, filepath.Base(r.Path)))
		if err := os.WriteFile(tmp, []byte(ablated), 0o644); err != nil {
			return res, err
		}
		err := lintFiles(bin, with(tmp, r.Path))
		_ = os.Remove(tmp)
		if err != nil {
			res.st.refsAttested++
			res.refs = append(res.refs, r)
		} else {
			fmt.Fprintf(os.Stderr, "argo-oracle: unevidenced reference %s %s -> %s\n", r.Path, r.From, r.To)
		}
	}
	// cross-references, by ablation of the target's file
	for _, x := range o.CrossReferences {
		if !accepted[x.Path] || !accepted[x.ToPath] {
			continue
		}
		res.st.xrefs++
		if lintFiles(bin, with(abs(x.Path), x.ToPath)) != nil {
			res.st.xrefsAttested++
			res.xrefs = append(res.xrefs, x)
		} else {
			fmt.Fprintf(os.Stderr, "argo-oracle: unevidenced cross-reference %s %s -> %s %s\n", x.Path, x.From, x.ToPath, x.To)
		}
	}
	sort.Strings(res.files)
	return res, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// writeTierA runs the tier-A path and writes stdout (definitions, references
// and cross_references, tier A) and filesOut.
func writeTierA(o out, lintable []string, subject, bin, scratch, filesOut string) error {
	ver, err := exec.Command(bin, "version", "--short").Output()
	if err != nil {
		return fmt.Errorf("argo version: %w", err)
	}
	res, err := tierA(subject, bin, scratch, o, lintable)
	if err != nil {
		return err
	}
	st := res.st
	fmt.Fprintf(os.Stderr, "argo-oracle: tier A (%s): %d Workflow-kind files, %d rejected (%d by the strict decode), %d WorkflowTemplate files as lint context; %d definitions (%d typed entities unlocated, %d walk-only); %d of %d references attested (%d sites not found); %d of %d cross-references attested\n",
		strings.TrimSpace(string(ver)), st.files, st.rejected, st.strict, st.contextTemplates, len(res.defs), st.unlocated, st.walkOnly, st.refsAttested, st.refs, st.refSiteMissing, st.xrefsAttested, st.xrefs)
	for _, f := range st.rejectedFiles {
		fmt.Fprintf(os.Stderr, "argo-oracle:   rejected: %s\n", f)
	}
	a := out{
		Tool:            "argo lint --offline (acceptance, reference ablation) + argo-workflows v1alpha1 types, strict (definitions); lines: gopkg.in/yaml.v3",
		Version:         strings.TrimSpace(string(ver)) + "; github.com/argoproj/argo-workflows/v4 v4.1.4",
		Tier:            "A",
		Definitions:     res.defs,
		References:      res.refs,
		CrossReferences: res.xrefs,
	}
	if a.Definitions == nil {
		a.Definitions = []def{}
	}
	if a.References == nil {
		a.References = []ref{}
	}
	if a.CrossReferences == nil {
		a.CrossReferences = []xref{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(a); err != nil {
		return err
	}
	if filesOut != "" {
		return os.WriteFile(filesOut, []byte(strings.Join(res.files, "\n")+"\n"), 0o644)
	}
	return nil
}
