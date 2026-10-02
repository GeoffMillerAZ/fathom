// Command gator-oracle is the language-breadth harness's oracle for the
// gatekeeper doc-engine dialect (LB-Q-gatekeeper), tier A: gator (the
// Gatekeeper project's own CLI, `gator verify`) runs every suite of the
// subject, and a suite test that passes attests that its ConstraintTemplate
// compiled and that its constraint is an instance of that template — gator
// refuses a constraint whose kind no loaded template defines. What a passing
// test attests is the oracle's answer, read for names and lines with
// gopkg.in/yaml.v3:
//
//   - definitions: each attested template (named by spec.crd.spec.names.kind)
//     and its spec.targets[].target, each attested constraint (Kind/name),
//     each suite with a passing test (Suite/name);
//   - references: a constraint to its template's kind;
//   - cross_references: a constraint to its template, a suite to the
//     template and constraint files a passing test names (the file's first
//     entity), entity granularity.
//
// -files-out lists the attested files and the case objects their tests read,
// the scope a reading is taken on (lang-policies -only).
//
// -policies-out writes the policy half, tier C (targets.go: each attested
// constraint's spec.match kinds × apiGroups, labelSelector and namespaces
// joined to the subject's Kubernetes objects), with a cross-check gator
// attests: a case that asserts violations (yes, or a count above 0) and
// passes proves the constraint matched that object, so the walk must join
// them. Embedded Rego is not read here (LB-X-gatekeeper-rego). Bench-only;
// never a product dependency.
//
//	go build -o <bin> . && <bin> -subject <checkout> -gator <gator> -files-out <files.txt> -policies-out <policies.json> > <out.json>
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

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
	To     string `json:"to"`
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}

type policyOut struct {
	Tool             string             `json:"tool"`
	Version          string             `json:"version"`
	Tier             string             `json:"tier"`
	PolicyReferences []policyRef        `json:"policy_references"`
	PolicyUnresolved []policyUnresolved `json:"policy_unresolved"`
	PolicyCrossCheck *crossCheck        `json:"policy_crosscheck,omitempty"`
}

func gatekeeperDoc(root *yaml.Node) bool {
	av, kind := scalar(get(root, "apiVersion")), scalar(get(root, "kind"))
	g, _, ok := strings.Cut(av, "/")
	if !ok {
		return false
	}
	switch g {
	case "templates.gatekeeper.sh", "constraints.gatekeeper.sh":
		return true
	case "test.gatekeeper.sh":
		return kind == "Suite"
	}
	return false
}

func claimed(f yamlFile) bool {
	if len(f.docs) == 0 {
		return false
	}
	for _, d := range f.docs {
		if d.Kind != yaml.MappingNode || !gatekeeperDoc(d) {
			return false
		}
	}
	return true
}

// entityOf is a claimed document's entity name and canonical kind.
func entityOf(d *yaml.Node) (string, string) {
	av, kind, name := scalar(get(d, "apiVersion")), scalar(get(d, "kind")), scalar(get(d, "metadata", "name"))
	switch {
	case kind == "ConstraintTemplate":
		return scalar(get(d, "spec", "crd", "spec", "names", "kind")), "policy"
	case strings.HasPrefix(av, "constraints.gatekeeper.sh/") && kind != "" && name != "":
		return kind + "/" + name, "resource"
	case kind == "Suite" && name != "":
		return "Suite/" + name, "test"
	}
	return "", ""
}

// verifyResult is one suite's gator verify outcome: test name -> passed,
// and per test the cases that passed.
type verifyResult struct {
	tests map[string]bool
	cases map[string]map[string]bool
}

func runVerify(bin, suite string) verifyResult {
	res := verifyResult{tests: map[string]bool{}, cases: map[string]map[string]bool{}}
	b, _ := exec.Command(bin, "verify", suite, "-v").CombinedOutput()
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	var current string
	var pendingCases []string
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)
		indented := strings.HasPrefix(line, " ")
		for _, st := range []string{"--- PASS: ", "--- FAIL: "} {
			rest, ok := strings.CutPrefix(trim, st)
			if !ok {
				continue
			}
			name := rest
			if i := strings.LastIndex(rest, "\t"); i >= 0 {
				name = rest[:i]
			}
			name = strings.TrimSpace(name)
			pass := st == "--- PASS: "
			if indented {
				pendingCases = append(pendingCases, map[bool]string{true: "+", false: "-"}[pass]+name)
				continue
			}
			current = name
			res.tests[current] = pass
			res.cases[current] = map[string]bool{}
			for _, c := range pendingCases {
				res.cases[current][c[1:]] = c[0] == '+'
			}
			pendingCases = nil
		}
	}
	return res
}

func violates(assertions *yaml.Node) bool {
	for _, a := range items(assertions) {
		v := get(a, "violations")
		if v == nil {
			continue
		}
		switch s := strings.ToLower(scalar(v)); s {
		case "yes", "true":
			return true
		case "no", "false", "0", "":
		default:
			return true
		}
	}
	return false
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	gator := flag.String("gator", "", "gator binary (required)")
	filesOut := flag.String("files-out", "", "write the attested files and their case objects here")
	policiesOut := flag.String("policies-out", "", "write the policy half (tier C, with gator's cross-check) here")
	flag.Parse()
	if *subject == "" || *gator == "" {
		fmt.Fprintln(os.Stderr, "usage: gator-oracle -subject <dir> -gator <bin> [-files-out <file>] [-policies-out <json>]")
		os.Exit(2)
	}
	ver, _ := exec.Command(*gator, "--version").Output()
	version := strings.TrimSpace(string(ver))
	files, err := walkYAML(*subject)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gator-oracle:", err)
		os.Exit(1)
	}
	byRel := map[string]yamlFile{}
	for _, f := range files {
		byRel[f.rel] = f
	}
	o := out{Tool: "gator verify (" + version + ") + gopkg.in/yaml.v3 for names and lines", Version: version, Tier: "A",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}}
	attested := map[string]bool{} // template and constraint files a passing test loaded
	scope := map[string]bool{}    // + suites and case objects
	suites := map[string]string{} // suite file -> Suite entity, when a test passed
	type pair struct{ suite, template, constraint string }
	var pairs []pair
	type hit struct{ constraintFile, objectFile string }
	var violations []hit
	nSuites, nTests, nPassed := 0, 0, 0
	for _, f := range files {
		if !claimed(f) {
			continue
		}
		for _, d := range f.docs {
			if scalar(get(d, "kind")) != "Suite" {
				continue
			}
			nSuites++
			res := runVerify(*gator, filepath.Join(*subject, filepath.FromSlash(f.rel)))
			dir := path.Dir(f.rel)
			for _, t := range items(get(d, "tests")) {
				nTests++
				name := scalar(get(t, "name"))
				if !res.tests[name] {
					continue
				}
				nPassed++
				tp := path.Clean(path.Join(dir, scalar(get(t, "template"))))
				cp := path.Clean(path.Join(dir, scalar(get(t, "constraint"))))
				attested[tp], attested[cp] = true, true
				scope[tp], scope[cp], scope[f.rel] = true, true, true
				ent, _ := entityOf(d)
				suites[f.rel] = ent
				pairs = append(pairs, pair{f.rel, tp, cp})
				for _, c := range items(get(t, "cases")) {
					op := path.Clean(path.Join(dir, scalar(get(c, "object"))))
					scope[op] = true
					if res.cases[name][scalar(get(c, "name"))] && violates(get(c, "assertions")) {
						violations = append(violations, hit{cp, op})
					}
				}
			}
		}
	}
	firstEntity := func(rel string) string {
		for _, d := range byRel[rel].docs {
			if e, _ := entityOf(d); e != "" {
				return e
			}
		}
		return ""
	}
	var targets []target
	for rel := range attested {
		f, ok := byRel[rel]
		if !ok || !claimed(f) {
			continue
		}
		for _, d := range f.docs {
			ent, kind := entityOf(d)
			if ent == "" {
				continue
			}
			o.Definitions = append(o.Definitions, def{rel, ent, d.Line, kind})
			if scalar(get(d, "kind")) == "ConstraintTemplate" {
				for _, t := range items(get(d, "spec", "targets")) {
					if tn := scalar(get(t, "target")); tn != "" && t.Kind == yaml.MappingNode {
						o.Definitions = append(o.Definitions, def{rel, tn, t.Line, "rule"})
					}
				}
				continue
			}
			o.References = append(o.References, ref{rel, ent, scalar(get(d, "kind"))})
			targets = append(targets, constraintTargets(rel, ent, d)...)
		}
	}
	for rel, ent := range suites {
		for _, d := range byRel[rel].docs {
			if e, _ := entityOf(d); e == ent {
				o.Definitions = append(o.Definitions, def{rel, ent, d.Line, "test"})
			}
		}
	}
	xseen := map[xref]bool{}
	addX := func(x xref) {
		if x.To != "" && x.From != "" && !xseen[x] {
			xseen[x] = true
			o.CrossReferences = append(o.CrossReferences, x)
		}
	}
	for _, p := range pairs {
		tmpl := firstEntity(p.template)
		for _, d := range byRel[p.constraint].docs {
			if e, k := entityOf(d); k == "resource" && scalar(get(d, "kind")) == tmpl {
				addX(xref{p.constraint, e, p.template, tmpl})
			}
		}
		addX(xref{p.suite, suites[p.suite], p.template, tmpl})
		addX(xref{p.suite, suites[p.suite], p.constraint, firstEntity(p.constraint)})
	}
	sortAll(&o)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "gator-oracle: %d suites, %d of %d tests pass; %d definitions, %d references, %d cross-references\n",
		nSuites, nPassed, nTests, len(o.Definitions), len(o.References), len(o.CrossReferences))
	if *filesOut != "" {
		var list []string
		for f := range scope {
			if _, ok := byRel[f]; ok {
				list = append(list, f)
			}
		}
		sort.Strings(list)
		if err := os.WriteFile(*filesOut, []byte(strings.Join(list, "\n")+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "gator-oracle:", err)
			os.Exit(1)
		}
	}
	if *policiesOut == "" {
		return
	}
	charts := chartDirsOf(files)
	var objs []object
	for _, f := range files {
		if !claimed(f) {
			objs = append(objs, objectsOf(f, charts)...)
		}
	}
	po := policyOut{Tool: "gator-oracle policy walk (independent gopkg.in/yaml.v3)", Version: "gopkg.in/yaml.v3 v3.0.1", Tier: "C"}
	po.PolicyReferences, po.PolicyUnresolved = resolveTargets(targets, objs)
	have := map[[2]string]bool{}
	for _, r := range po.PolicyReferences {
		have[[2]string{r.Path, r.ToPath}] = true
	}
	cc := &crossCheck{Tool: "gator verify (" + version + ")", Tier: "A", Disagree: []string{}}
	unplaced := 0
	for _, v := range violations {
		objs := objectsOf(byRel[v.objectFile], charts)
		if len(objs) == 0 {
			unplaced++ // an AdmissionReview wrapper, or no object
			continue
		}
		cc.Checked++
		if have[[2]string{v.constraintFile, v.objectFile}] {
			cc.Agree++
		} else {
			cc.Disagree = append(cc.Disagree, v.constraintFile+" -> "+v.objectFile)
		}
	}
	sort.Strings(cc.Disagree)
	cc.Note = fmt.Sprintf("a passing case asserting violations proves the constraint matched its object; %d such cases name an object file the walk reads no object from (an AdmissionReview wrapper)", unplaced)
	po.PolicyCrossCheck = cc
	b, _ := json.MarshalIndent(po, "", " ")
	if err := os.WriteFile(*policiesOut, append(b, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gator-oracle:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "gator-oracle: %d policy pairs, %d unresolved; gator cross-check %d of %d\n", len(po.PolicyReferences), len(po.PolicyUnresolved), cc.Agree, cc.Checked)
}

// constraintTargets reads a constraint's spec.match: kinds × apiGroups (a
// "*" or absent group is any group, "" the core group), narrowed by
// labelSelector.matchLabels and scoped by namespaces; a selector alone; a
// constraint with neither matches every object.
func constraintTargets(rel, ent string, d *yaml.Node) []target {
	m := get(d, "spec", "match")
	var scope []string
	for _, n := range scalars(get(m, "namespaces")) {
		scope = append(scope, n.Value)
	}
	sel := get(m, "labelSelector", "matchLabels")
	labels := labelMap(sel)
	var out []target
	seen := map[string]bool{}
	for _, ks := range items(get(m, "kinds")) {
		groups := scalars(get(ks, "apiGroups"))
		gs := []string{}
		for _, g := range groups {
			gs = append(gs, g.Value)
		}
		if len(gs) == 0 {
			gs = []string{"*"}
		}
		for _, k := range scalars(get(ks, "kinds")) {
			for _, g := range gs {
				kt := k.Value
				if kt != "*" && g != "" && g != "*" {
					kt = g + "/" + kt
				}
				if kt == "*" && labels != nil {
					kt = ""
				}
				if seen[kt] {
					continue
				}
				seen[kt] = true
				t := target{path: rel, from: ent, raw: k.Value, kind: kt, scope: scope, labels: labels, line: k.Line}
				if kt == "" {
					t.raw = canon(labels)
				}
				out = append(out, t)
			}
		}
	}
	if len(out) == 0 {
		if labels != nil {
			return []target{{path: rel, from: ent, raw: canon(labels), labels: labels, scope: scope, line: sel.Line}}
		}
		return []target{{path: rel, from: ent, raw: "*", kind: "*", line: d.Line}}
	}
	return out
}

func sortAll(o *out) {
	sort.Slice(o.Definitions, func(i, j int) bool {
		a, b := o.Definitions[i], o.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Line < b.Line
	})
	sort.Slice(o.References, func(i, j int) bool {
		a, b := o.References[i], o.References[j]
		return a.Path+"\x00"+a.From < b.Path+"\x00"+b.From
	})
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath < b.Path+"\x00"+b.From+"\x00"+b.ToPath
	})
}
