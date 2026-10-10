// Command prometheus-oracle is the language-breadth harness's tier-A oracle
// for the `prometheus` doc-engine dialect (LB-Q-prometheus, design §1.3,
// §10): Prometheus' own tools decide what a rule file, a configuration and a
// promtool test file declare. A separate Go module, never imported by
// fathom, never a product dependency (ADR-020, ADR-025).
//
//	prometheus-oracle -subject <checkout> -promtool <promtool> \
//	    -refs-out <refs.json> -routes-out <routes.json> -files-out <files.txt> > <defs.json>
//	prometheus-oracle -derive-prometheusrules <out> -subject <checkout>
//
// The instrument: `promtool check rules`, `promtool check config
// --syntax-only` and `promtool test rules` accept or refuse each file;
// model/rulefmt (the rule groups and rules), config (scrape jobs, static
// targets, rule_files) and promql/parser (ExtractSelectors) name what an
// accepted file declares and references. Lines come from gopkg.in/yaml.v3
// (an item's first key, "where it is written"): a decoder detail, never an
// entity decision.
//
// The derive mode writes each PrometheusRule custom resource's spec (the
// same rule grammar) as a plain rule file at the same relative path, so a
// kube-prometheus checkout is measured as extracted specs; the custom
// resource itself is the kubernetes side pass's (item mesh).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	_ "github.com/prometheus/prometheus/discovery/install" // every service discovery a configuration may name, as promtool registers them
	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/promql"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end,omitempty"`
	Kind string `json:"kind"`
}

type pair struct {
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

type defsFile struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
}

type refsFile struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []pair `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}

type routesFile struct {
	Tool            string            `json:"tool"`
	Version         string            `json:"version"`
	Tier            string            `json:"tier"`
	Definitions     []def             `json:"definitions"`
	RouteReferences []routeRef        `json:"route_references"`
	RouteUnresolved []routeUnresolved `json:"route_unresolved"`
}

// promParser is the reference parser with every optional syntax on.
var promParser = parser.NewParser(parser.Options{EnableExperimentalFunctions: true, ExperimentalDurationExpr: true, EnableExtendedRangeSelectors: true, EnableBinopFillModifiers: true})

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// skipDirs are the directories fathom's connector never reads (connector.go).
var skipDirs = map[string]bool{"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true, ".idea": true, ".vscode": true, ".claude": true}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	promtool := flag.String("promtool", "", "promtool binary (required unless -derive-prometheusrules)")
	refsOut := flag.String("refs-out", "", "write references and cross_references here")
	routesOut := flag.String("routes-out", "", "write route_references and route_unresolved here")
	filesOut := flag.String("files-out", "", "write the accepted files, one per line")
	derive := flag.String("derive-prometheusrules", "", "write each PrometheusRule's spec as a rule file under this directory and exit")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: prometheus-oracle -subject <dir> -promtool <bin> [-refs-out f] [-routes-out f] [-files-out f]")
		os.Exit(2)
	}
	if *derive != "" {
		n, err := deriveRules(*subject, *derive)
		if err != nil {
			fmt.Fprintln(os.Stderr, "derive:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "derived %d PrometheusRule specs\n", n)
		return
	}
	if *promtool == "" {
		fmt.Fprintln(os.Stderr, "-promtool is required")
		os.Exit(2)
	}
	r := run(*subject, *promtool)
	version := promtoolVersion(*promtool)
	emit := func(p string, v any) {
		b, _ := json.MarshalIndent(v, "", " ")
		if p == "" {
			os.Stdout.Write(append(b, '\n'))
			return
		}
		if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	emit("", defsFile{Tool: "promtool", Version: version, Tier: "A", Definitions: r.defs})
	if *refsOut != "" {
		emit(*refsOut, refsFile{Tool: "promtool", Version: version, Tier: "A", Definitions: []def{}, References: r.refs, CrossReferences: r.xrefs})
	}
	if *routesOut != "" {
		emit(*routesOut, routesFile{Tool: "promtool+routes", Version: version, Tier: routesTier(r.routes), Definitions: []def{}, RouteReferences: r.routes, RouteUnresolved: r.unresolved})
	}
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(r.files, "\n")+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "files accepted %d (rules %d, configs %d, tests %d), refused %d %v; definitions %d, references %d, cross_references %d, route_references %d, route_unresolved %d\n",
		len(r.files), r.n["rules"], r.n["config"], r.n["tests"], len(r.refused), r.refused, len(r.defs), len(r.refs), len(r.xrefs), len(r.routes), len(r.unresolved))
}

// promtoolVersion is the binary's version and revision ("3.15.0 (5241a27f)").
func promtoolVersion(bin string) string {
	out, _ := exec.Command(bin, "--version").CombinedOutput()
	line, _, _ := strings.Cut(string(out), "\n")
	f := strings.Fields(line) // promtool, version 3.15.0 (branch: HEAD, revision: <sha>)
	if len(f) < 3 {
		return strings.TrimSpace(line)
	}
	v := f[2]
	if _, rev, ok := strings.Cut(line, "revision: "); ok && len(rev) >= 8 {
		v += " (" + rev[:8] + ")"
	}
	return v
}

// result is the oracle's whole answer for a subject.
type result struct {
	files      []string
	refused    []string
	n          map[string]int
	defs       []def
	refs       []pair
	xrefs      []xref
	routes     []routeRef
	unresolved []routeUnresolved
	pending    []target // route targets before resolution
}

// candidate is one YAML file whose top level has a Prometheus shape.
type candidate struct {
	rel, kind string
	root      *yaml.Node
	content   []byte
}

func run(subject, promtool string) result {
	r := result{n: map[string]int{}}
	var cands []candidate
	err := filepath.WalkDir(subject, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if p != subject && skipDirs[e.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yml" && ext != ".yaml" || !e.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil || len(b) > 2<<20 {
			return err
		}
		root := firstDoc(b)
		if root == nil || root.Kind != yaml.MappingNode {
			return nil
		}
		rel, _ := filepath.Rel(subject, p)
		c := candidate{rel: filepath.ToSlash(rel), root: root, content: b}
		switch {
		case get(root, "apiVersion") != nil && get(root, "kind") != nil:
			return nil // a Kubernetes object: the kubernetes dialect's
		case get(root, "groups") != nil:
			c.kind = "rules"
		case get(root, "tests") != nil && get(root, "rule_files") != nil:
			c.kind = "tests"
		case get(root, "scrape_configs") != nil || get(root, "rule_files") != nil:
			c.kind = "config"
		default:
			return nil
		}
		cands = append(cands, c)
		return nil
	})
	if err != nil {
		panic(err)
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].rel < cands[j].rel })
	accepted := map[string]bool{}
	ruleFiles := map[string]map[string]bool{} // rel -> record names declared
	for _, c := range cands {
		abs := filepath.Join(subject, filepath.FromSlash(c.rel))
		var args []string
		switch c.kind {
		case "rules":
			args = []string{"check", "rules", abs}
		case "config":
			args = []string{"check", "config", "--syntax-only", abs}
		case "tests":
			args = []string{"test", "rules", abs}
		}
		out, err := exec.Command(promtool, args...).CombinedOutput()
		ok := err == nil
		if !ok && c.kind == "tests" {
			// a failing unit test is still a test file promtool read; a file
			// it cannot load is not
			ok = !bytes.Contains(out, []byte("Unmarshalling")) && !bytes.Contains(out, []byte("yaml:")) && strings.Contains(string(out), "FAILED")
		}
		if !ok {
			r.refused = append(r.refused, c.rel)
			continue
		}
		accepted[c.rel] = true
		r.files = append(r.files, c.rel)
		r.n[c.kind]++
		r.defs = append(r.defs, def{Path: c.rel, Name: path.Base(c.rel), Line: c.root.Line, End: endLine(c.root), Kind: "package"})
		switch c.kind {
		case "rules":
			records := r.readRules(c)
			ruleFiles[c.rel] = records
		case "config":
			r.readConfig(c)
		case "tests":
			r.readTests(c)
		}
	}
	// rule_files of configurations and test files, to an accepted rule file
	for _, c := range cands {
		if !accepted[c.rel] || c.kind == "rules" {
			continue
		}
		for _, pat := range ruleFilesOf(c) {
			for _, t := range globRel(subject, path.Dir(c.rel), pat) {
				if _, ok := ruleFiles[t]; ok {
					r.xrefs = append(r.xrefs, xref{Path: c.rel, From: path.Base(c.rel), ToPath: t, To: path.Base(t)})
				}
			}
		}
	}
	r.resolveRoutes(subject)
	sortAll(&r)
	return r
}

// readRules adds an accepted rule file's groups and rules (rulefmt's names,
// yaml.v3's lines) and the recording rules each rule's expr reads, and
// collects its route targets. It returns the recording rule names.
func (r *result) readRules(c candidate) map[string]bool {
	rgs, errs := rulefmt.Parse(c.content, false, model.UTF8Validation, promParser, quiet)
	if len(errs) > 0 || rgs == nil {
		panic(fmt.Sprintf("%s: promtool accepted, rulefmt refused: %v", c.rel, errs))
	}
	groupNodes := items(get(c.root, "groups"))
	records := map[string]bool{}
	for _, g := range rgs.Groups {
		for _, rl := range g.Rules {
			if rl.Record != "" {
				records[rl.Record] = true
			}
		}
	}
	seen := map[pair]bool{}
	for gi, g := range rgs.Groups {
		gn := groupNodes[gi]
		r.defs = append(r.defs, def{Path: c.rel, Name: g.Name, Line: gn.Line, End: endLine(gn), Kind: "section"})
		ruleNodes := items(get(gn, "rules"))
		for ri, rl := range g.Rules {
			rn := ruleNodes[ri]
			name := rl.Alert
			if rl.Record != "" {
				name = rl.Record
			}
			r.defs = append(r.defs, def{Path: c.rel, Name: name, Line: rn.Line, End: endLine(rn), Kind: "rule"})
			e, err := promParser.ParseExpr(rl.Expr)
			if err != nil {
				panic(fmt.Sprintf("%s: rulefmt accepted an expr the parser refuses: %v", c.rel, err))
			}
			for _, set := range parser.ExtractSelectors(e) {
				for _, m := range set {
					if m.Name == model.MetricNameLabel && m.Type.String() == "=" && records[m.Value] && !(rl.Record != "" && m.Value == rl.Record) {
						p := pair{Path: c.rel, From: name, To: m.Value}
						if !seen[p] {
							seen[p] = true
							r.refs = append(r.refs, p)
						}
					}
				}
			}
			r.ruleTargets(c.rel, name, e)
		}
	}
	return records
}

// readConfig adds an accepted configuration's scrape jobs (each a job and
// its telemetry identity, the config package's names) and their static
// targets.
func (r *result) readConfig(c candidate) {
	cfg, err := config.Load(string(c.content), quiet)
	if err != nil {
		panic(fmt.Sprintf("%s: promtool accepted, config refused: %v", c.rel, err))
	}
	jobNodes := items(get(c.root, "scrape_configs"))
	for i, sc := range cfg.ScrapeConfigs {
		n := jobNodes[i]
		r.defs = append(r.defs, def{Path: c.rel, Name: sc.JobName, Line: n.Line, End: endLine(n), Kind: "job"},
			def{Path: c.rel, Name: sc.JobName, Line: n.Line, End: endLine(n), Kind: "key"})
		for _, sd := range sc.ServiceDiscoveryConfigs {
			st, ok := sd.(discovery.StaticConfig)
			if !ok {
				continue
			}
			for _, tg := range st {
				for _, t := range tg.Targets {
					r.pending = append(r.pending, target{path: c.rel, from: sc.JobName, raw: string(t[model.AddressLabel])})
				}
			}
		}
	}
}

// unitTestFile is promtool's own test file shape (cmd/promtool/unittest.go),
// read strictly as promtool reads it.
type unitTestFile struct {
	RuleFiles          []string       `yaml:"rule_files"`
	EvaluationInterval model.Duration `yaml:"evaluation_interval,omitempty"`
	GroupEvalOrder     []string       `yaml:"group_eval_order"`
	Tests              []struct {
		Name string `yaml:"name"`
	} `yaml:"tests"`
	FuzzyCompare bool `yaml:"fuzzy_compare,omitempty"`
}

func (r *result) readTests(c candidate) {
	var f unitTestFile
	if err := yaml.Unmarshal(c.content, &f); err != nil {
		panic(fmt.Sprintf("%s: %v", c.rel, err))
	}
	nodes := items(get(c.root, "tests"))
	for i, t := range f.Tests {
		name := t.Name
		if name == "" {
			name = fmt.Sprint(i)
		}
		r.defs = append(r.defs, def{Path: c.rel, Name: "tests/" + name, Line: nodes[i].Line, End: endLine(nodes[i]), Kind: "test"})
	}
}

func ruleFilesOf(c candidate) []string {
	var out []string
	for _, n := range items(get(c.root, "rule_files")) {
		if n.Kind == yaml.ScalarNode && n.Value != "" {
			out = append(out, n.Value)
		}
	}
	return out
}

// globRel expands a rule_files pattern as Prometheus does (relative to the
// referring file's directory, a glob allowed) and returns the subject files
// it names; an absolute path is outside the subject.
func globRel(subject, dir, pat string) []string {
	if filepath.IsAbs(pat) {
		return nil
	}
	full := filepath.Join(subject, filepath.FromSlash(dir), filepath.FromSlash(pat))
	ms, err := filepath.Glob(full)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range ms {
		rel, err := filepath.Rel(subject, m)
		if err == nil && !strings.HasPrefix(rel, "..") {
			out = append(out, filepath.ToSlash(rel))
		}
	}
	sort.Strings(out)
	return out
}

// ── YAML helpers ──────────────────────────────────────────────────────────

func firstDoc(b []byte) *yaml.Node {
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(b)).Decode(&doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	return doc.Content[0]
}

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

func items(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

// endLine is the last line any node under n is written on.
func endLine(n *yaml.Node) int {
	end := n.Line
	for _, c := range n.Content {
		if e := endLine(c); e > end {
			end = e
		}
	}
	return end
}

// ── routes ────────────────────────────────────────────────────────────────

// target is one route target before resolution: a scrape job's static
// address, or an identifying label's value (scoped by a namespace).
type target struct {
	path, from, raw string
	value, ns       string // a label matcher's value and namespace ("" for an address)
	label           bool
}

// ruleTargets collects an expression's identifying-label matchers as route
// targets (equality only; the regex and negative ones are the oracle's
// unresolved too).
func (r *result) ruleTargets(rel, from string, e parser.Expr) {
	idents := map[string]bool{}
	for _, l := range promql.IdentityLabels() {
		idents[l] = true
	}
	for _, set := range parser.ExtractSelectors(e) {
		ns, nsFixed := "", true
		for _, m := range set {
			if m.Name == promql.NamespaceLabel {
				if m.Type.String() == "=" {
					ns = m.Value
				} else {
					nsFixed = false
				}
			}
		}
		for _, m := range set {
			if !idents[m.Name] || m.Name == promql.NamespaceLabel {
				continue
			}
			op := m.Type.String()
			switch {
			case op == "!=":
				r.unresolved = append(r.unresolved, routeUnresolved{Path: rel, From: from, Raw: fmt.Sprintf("%s!=%q", m.Name, m.Value), Reason: "negative-matcher"})
			case op != "=":
				r.unresolved = append(r.unresolved, routeUnresolved{Path: rel, From: from, Raw: fmt.Sprintf("%s%s%q", m.Name, op, m.Value), Reason: "regex-matcher"})
			case m.Value == "":
			case !nsFixed:
				r.unresolved = append(r.unresolved, routeUnresolved{Path: rel, From: from, Raw: m.Name + "=" + m.Value, Reason: "regex-matcher"})
			default:
				r.pending = append(r.pending, target{path: rel, from: from, raw: m.Value, value: m.Value, ns: ns, label: true})
			}
		}
	}
}

func sortAll(r *result) {
	sort.Slice(r.defs, func(i, j int) bool {
		a, b := r.defs[i], r.defs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Kind < b.Kind
	})
	sort.Slice(r.refs, func(i, j int) bool {
		a, b := r.refs[i], r.refs[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	sort.Slice(r.xrefs, func(i, j int) bool {
		a, b := r.xrefs[i], r.xrefs[j]
		return a.Path+"\x00"+a.ToPath < b.Path+"\x00"+b.ToPath
	})
	sort.Slice(r.routes, func(i, j int) bool {
		a, b := r.routes[i], r.routes[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To+"\x00"+a.Raw < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To+"\x00"+b.Raw
	})
	sort.Slice(r.unresolved, func(i, j int) bool {
		a, b := r.unresolved[i], r.unresolved[j]
		return a.Path+"\x00"+a.From+"\x00"+a.Raw+"\x00"+a.Reason < b.Path+"\x00"+b.From+"\x00"+b.Raw+"\x00"+b.Reason
	})
}
