// Command grafana-oracle is the language-breadth harness's oracle for the
// `grafana-dashboard` doc-engine dialect (LB-Q-grafana-dashboard, design
// §1.3, §10). Tier C, and it says so: no Grafana tool reads a dashboard
// offline (Grafana's own source is AGPL-3.0 and is a behaviour reference
// only; nothing of it is copied), so this program walks the dashboard JSON
// model and the provisioning files by Grafana's documented shapes, a reading
// of its own that shares no code with fathom's dialect, and grades the PromQL
// a panel sends with the Prometheus parser's own ExtractSelectors
// (prometheus v0.315.0, as calibration/oracles/prometheus). A tier-C oracle
// caps every level it grades at good (design §1.3).
//
//	grafana-oracle -subject <checkout> -refs-out <refs.json> -obs-out <obs.json> -files-out <files.txt> > <defs.json>
//	grafana-oracle -derive-embedded <out> -subject <manifests>
//
// What it reads:
//   - a dashboard: a JSON or YAML document whose top level (or a `dashboard`
//     wrapper's) has schemaVersion with panels or rows, or a
//     dashboard.grafana.app resource of kind Dashboard; its rows, panels
//     (nested in rows, a library panel by uid), library panel elements,
//     template variables and links;
//   - a provisioning file: a YAML document with `apiVersion: 1` and a
//     providers, datasources, groups, contactPoints, policies, muteTimes or
//     templates section;
//   - the Prometheus rule files' recording rules, which a panel's metric
//     names.
//
// References: a panel, row or variable to every declared template variable
// its strings interpolate ($name, ${name...}, [[name...]]), and a panel to
// the library panel element it reuses.
//
// Pairs (obs_references): a panel's or alert rule's PromQL metric (its
// datasource Prometheus-typed: the target's, else the panel's, else every
// datasource variable and input of the dashboard Prometheus; Grafana's
// macros substituted, a template variable a placeholder that names no
// metric) to the recording rule; a `/d/<uid>` link or dashboardUid to the
// dashboard; a datasource uid to the provisioned datasource; a policy's
// receiver to the contact point, its mute time interval to the mute timing;
// a provider's options.path (relative, or a container path a compose bind
// mount maps) to every dashboard under it. Lines come from yaml.v3: a map
// value's line is its key's, a list item's its own.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
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

type obsRef struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to"`
	Edge   string `json:"edge"`
}

// promParser is the reference parser with every optional syntax on.
var promParser = parser.NewParser(parser.Options{EnableExperimentalFunctions: true, ExperimentalDurationExpr: true, EnableExtendedRangeSelectors: true, EnableBinopFillModifiers: true})

// skipDirs are the directories fathom's connector never reads (connector.go).
var skipDirs = map[string]bool{"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true, ".idea": true, ".vscode": true, ".claude": true}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	refsOut := flag.String("refs-out", "", "write references here")
	obsOut := flag.String("obs-out", "", "write the observability pairs here")
	filesOut := flag.String("files-out", "", "write the dashboards and provisioning files read, one per line")
	derive := flag.String("derive-embedded", "", "write every dashboard and provisioning file a Kubernetes ConfigMap, Secret or GrafanaDashboard embeds under <out>/<manifest without extension>/<key>, removing the manifest from out, and exit")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: grafana-oracle -subject <dir> [-refs-out f] [-obs-out f] [-files-out f]")
		os.Exit(2)
	}
	if *derive != "" {
		n, err := deriveEmbedded(*subject, *derive)
		if err != nil {
			fmt.Fprintln(os.Stderr, "derive:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "derived %d embedded documents\n", n)
		return
	}
	r := run(*subject)
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
	const tool, version = "grafana-model-walk", "1 (prometheus/promql/parser v0.315.0 ExtractSelectors)"
	emit("", map[string]any{"tool": tool, "version": version, "tier": "C", "definitions": r.defs})
	if *refsOut != "" {
		emit(*refsOut, map[string]any{"tool": tool, "version": version, "tier": "C", "definitions": []def{}, "references": r.refs})
	}
	if *obsOut != "" {
		emit(*obsOut, map[string]any{"tool": tool, "version": version, "tier": "C", "definitions": []def{}, "obs_references": r.obs, "obs_decided": []pair{}, "obs_undecided": []any{}})
	}
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(r.files, "\n")+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "files %d (dashboards %d, provisioning %d), rule files %d (%d recording rules); definitions %d, references %d, pairs %d; queries scanned %d, refused by the parser %d, not Prometheus %d, datasource undecided %d\n",
		len(r.files), r.dashboards, r.provisioning, r.ruleFiles, len(r.records), len(r.defs), len(r.refs), len(r.obs), r.scanned, r.refused, r.notProm, r.undecidedDS)
}

type node struct {
	n    *yaml.Node
	line int // a map value's key line, a list item's own
}

type dashFile struct {
	rel  string
	root *yaml.Node // the document
	dash *yaml.Node // the dashboard object (root, or its wrapper's)
	v2   bool
	uid  string // the root entity's name
}

type result struct {
	files                                  []string
	dashboards, provisioning, ruleFiles    int
	records                                map[string][]string // recording rule -> files
	defs                                   []def
	refs                                   []pair
	obs                                    []obsRef
	scanned, refused, notProm, undecidedDS int
	uidFile                                map[string][]string // dashboard uid -> files
	dsFile                                 map[string][]string // datasource name -> files
	cpFile                                 map[string][]string // contact point -> files
	muteFile                               map[string][]string
	mounts                                 [][2]string // source dir, container target
}

func run(subject string) *result {
	r := &result{records: map[string][]string{}, uidFile: map[string][]string{}, dsFile: map[string][]string{}, cpFile: map[string][]string{}, muteFile: map[string][]string{}}
	var dashes []dashFile
	type prov struct {
		rel  string
		root *yaml.Node
	}
	var provs []prov
	_ = filepath.WalkDir(subject, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() {
			if p != subject && (skipDirs[e.Name()] || strings.HasPrefix(e.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".json" && ext != ".yml" && ext != ".yaml" {
			return nil
		}
		rel, _ := filepath.Rel(subject, p)
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		var doc yaml.Node
		if yaml.Unmarshal(b, &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
			return nil
		}
		root := doc.Content[0]
		if d, ok := dashboardOf(root); ok {
			d.rel = rel
			dashes = append(dashes, d)
			return nil
		}
		if ext == ".json" {
			return nil
		}
		if get(root, "apiVersion") != nil && get(root, "kind") != nil {
			return nil // a Kubernetes object
		}
		if isProvisioning(root) {
			provs = append(provs, prov{rel, root})
			return nil
		}
		if g := get(root, "groups"); g != nil && g.Kind == yaml.SequenceNode {
			n := 0
			for _, grp := range g.Content {
				for _, ru := range items(get(grp, "rules")) {
					if rec := scalar(get(ru, "record")); rec != "" {
						r.records[rec] = appendOnce(r.records[rec], rel)
						n++
					}
				}
			}
			if n > 0 {
				r.ruleFiles++
			}
		}
		if s := get(root, "services"); s != nil && s.Kind == yaml.MappingNode {
			r.mounts = append(r.mounts, composeMounts(rel, s)...)
		}
		return nil
	})
	// declarations first, then references
	for _, d := range dashes {
		r.uidFile[d.uid] = appendOnce(r.uidFile[d.uid], d.rel)
	}
	for _, p := range provs {
		for _, ds := range items(get(p.root, "datasources")) {
			if name := first(scalar(get(ds, "uid")), scalar(get(ds, "name"))); name != "" {
				r.dsFile[name] = appendOnce(r.dsFile[name], p.rel)
			}
		}
		for _, cp := range items(get(p.root, "contactPoints")) {
			if name := scalar(get(cp, "name")); name != "" {
				r.cpFile[name] = appendOnce(r.cpFile[name], p.rel)
			}
		}
		for _, mt := range items(get(p.root, "muteTimes")) {
			if name := scalar(get(mt, "name")); name != "" {
				r.muteFile[name] = appendOnce(r.muteFile[name], p.rel)
			}
		}
	}
	for _, d := range dashes {
		r.files = append(r.files, d.rel)
		r.dashboards++
		r.readDashboard(d)
	}
	for _, p := range provs {
		r.files = append(r.files, p.rel)
		r.provisioning++
		r.readProvisioning(p.rel, p.root, dashes)
	}
	r.sort()
	return r
}

func appendOnce(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func first(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
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

func keyLine(n *yaml.Node, key string) int {
	for i := 0; n != nil && i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i].Line
		}
	}
	return 0
}

func items(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

func endLine(n *yaml.Node) int {
	if n == nil {
		return 0
	}
	end := n.Line
	for _, c := range n.Content {
		if e := endLine(c); e > end {
			end = e
		}
	}
	return end
}

// dashboardOf recognizes a dashboard document.
func dashboardOf(root *yaml.Node) (dashFile, bool) {
	has := func(n *yaml.Node, k string) bool { return get(n, k) != nil }
	switch {
	case has(root, "schemaVersion") && (has(root, "panels") || has(root, "rows")):
		return dashFile{root: root, dash: root, uid: first(scalar(get(root, "uid")), scalar(get(root, "title")))}, true
	case has(root, "dashboard") && get(root, "dashboard").Kind == yaml.MappingNode && has(get(root, "dashboard"), "schemaVersion"):
		d := get(root, "dashboard")
		return dashFile{root: root, dash: d, uid: first(scalar(get(d, "uid")), scalar(get(d, "title")))}, true
	case scalar(get(root, "kind")) == "Dashboard" && strings.HasPrefix(scalar(get(root, "apiVersion")), "dashboard.grafana.app/"):
		return dashFile{root: root, dash: get(root, "spec"), v2: true, uid: first(scalar(get(get(root, "metadata"), "name")), scalar(get(get(root, "spec"), "title")))}, true
	}
	return dashFile{}, false
}

func isProvisioning(root *yaml.Node) bool {
	if scalar(get(root, "apiVersion")) != "1" {
		return false
	}
	for _, k := range []string{"providers", "datasources", "groups", "contactPoints", "policies", "muteTimes", "templates"} {
		if get(root, k) != nil {
			return true
		}
	}
	return false
}

func (r *result) def(rel, name, kind string, line int, n *yaml.Node) {
	r.defs = append(r.defs, def{Path: rel, Name: name, Line: line, End: max(endLine(n), line), Kind: kind})
}

func (r *result) pairTo(rel, from string, targets map[string][]string, name, edge string) {
	for _, f := range targets[name] {
		r.obs = append(r.obs, obsRef{Path: rel, From: from, ToPath: f, To: name, Edge: edge})
	}
}

var uidLinkRe = regexp.MustCompile(`/d/([A-Za-z0-9_-]+)`)

// varUseRe is a template variable a string interpolates: $name, ${name...},
// [[name...]].
var varUseRe = regexp.MustCompile(`(?:\$\{|\$|\[\[)([A-Za-z0-9_]+)`)

// isPanelEntity reports whether a panel list item is a definition (a row,
// a typed panel, a library panel reference): the walk for a containing
// definition's strings stops there.
func isPanelEntity(p *yaml.Node) bool {
	return scalar(get(p, "type")) != "" || get(p, "libraryPanel") != nil
}

// entityStrings is every key and scalar under n, not entering a nested
// panel list's definitions.
func entityStrings(n *yaml.Node, out *[]string) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.ScalarNode:
		*out = append(*out, n.Value)
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			*out = append(*out, n.Content[i].Value)
			v := n.Content[i+1]
			if n.Content[i].Value == "panels" && v.Kind == yaml.SequenceNode {
				for _, it := range v.Content {
					if !isPanelEntity(it) {
						entityStrings(it, out)
					}
				}
				continue
			}
			entityStrings(v, out)
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			entityStrings(c, out)
		}
	}
}

// varRefs is every declared variable an entity's strings interpolate, other
// than itself.
func varRefs(rel, from string, n *yaml.Node, declared map[string]bool, self string) []pair {
	var strs []string
	entityStrings(n, &strs)
	var out []pair
	seen := map[string]bool{}
	for _, s := range strs {
		for _, m := range varUseRe.FindAllStringSubmatch(s, -1) {
			v := m[1]
			if !declared[v] || v == self || seen[v] {
				continue
			}
			seen[v] = true
			out = append(out, pair{Path: rel, From: from, To: v})
		}
	}
	return out
}

// strings under n, not entering nested panels (another panel's).
func panelStrings(n *yaml.Node, out *[]string, top bool) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.ScalarNode:
		*out = append(*out, n.Value)
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "panels" && !top {
				continue
			}
			if n.Content[i].Value == "panels" && top {
				continue // a row's panels are their own
			}
			*out = append(*out, n.Content[i].Value)
			panelStrings(n.Content[i+1], out, false)
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			panelStrings(c, out, false)
		}
	}
}

// panelsOf walks every panel of a panel list, rows' nested panels after
// their row, with its indexed path.
func panelsOf(list *yaml.Node, ipath string, fn func(p *yaml.Node, ipath string)) {
	for i, p := range items(list) {
		ip := fmt.Sprintf("%s[%d]", ipath, i)
		fn(p, ip)
		if nested := get(p, "panels"); nested != nil {
			panelsOf(nested, ip+".panels", fn)
		}
	}
}

func (r *result) readDashboard(d dashFile) {
	r.def(d.rel, d.uid, "package", d.root.Line, d.root)
	dash := d.dash
	prefix := ""
	if dash != d.root && !d.v2 {
		prefix = "dashboard."
	}
	if d.v2 {
		for i := 0; dash != nil && i+1 < len(dash.Content); i += 2 {
			if dash.Content[i].Value != "elements" {
				continue
			}
			els := dash.Content[i+1]
			for j := 0; j+1 < len(els.Content); j += 2 {
				name := "panel/" + els.Content[j].Value
				el := els.Content[j+1]
				r.def(d.rel, name, "element", els.Content[j].Line, el)
				r.refs = append(r.refs, varRefs(d.rel, name, el, v2Vars(dash), "")...)
				for _, q := range items(get(get(get(get(el, "spec"), "data"), "spec"), "queries")) {
					query := get(get(q, "spec"), "query")
					expr := scalar(get(get(query, "spec"), "expr"))
					r.queryPairs(d, name, expr, v2Datasource(query), get(query, "spec"))
				}
			}
		}
		for _, v := range items(get(dash, "variables")) {
			if name := scalar(get(get(v, "spec"), "name")); name != "" {
				r.def(d.rel, name, "variable_decl", v.Line, v)
				r.refs = append(r.refs, varRefs(d.rel, name, v, v2Vars(dash), name)...)
			}
		}
		for i, l := range items(get(dash, "links")) {
			r.def(d.rel, fmt.Sprintf("spec.links[%d]", i), "import", l.Line, l)
		}
		return
	}
	// library panel elements, variables, links
	if els := get(d.root, "__elements"); els != nil && els.Kind == yaml.MappingNode {
		for j := 0; j+1 < len(els.Content); j += 2 {
			r.def(d.rel, els.Content[j].Value, "element", els.Content[j].Line, els.Content[j+1])
		}
	}
	declared := map[string]bool{}
	for _, v := range items(get(get(dash, "templating"), "list")) {
		if name := scalar(get(v, "name")); name != "" {
			declared[name] = true
		}
	}
	for _, v := range items(get(get(dash, "templating"), "list")) {
		if name := scalar(get(v, "name")); name != "" {
			r.def(d.rel, name, "variable_decl", v.Line, v)
			r.refs = append(r.refs, varRefs(d.rel, name, v, declared, name)...)
		}
	}
	for i, l := range items(get(dash, "links")) {
		name := fmt.Sprintf("%slinks[%d]", prefix, i)
		r.def(d.rel, name, "import", l.Line, l)
		if m := uidLinkRe.FindStringSubmatch(scalar(get(l, "url"))); m != nil {
			r.pairTo(d.rel, name, r.uidFile, m[1], "links_to")
		}
	}
	// legacy rows
	for i, row := range items(get(dash, "rows")) {
		name := fmt.Sprintf("row/%srows[%d]", prefix, i)
		r.def(d.rel, name, "section", row.Line, row)
		r.refs = append(r.refs, varRefs(d.rel, name, row, declared, "")...)
	}
	visit := func(p *yaml.Node, ip string) {
		typ := scalar(get(p, "type"))
		id := scalar(get(p, "id"))
		if id == "" {
			id = ip
		}
		if typ == "row" {
			r.def(d.rel, "row/"+id, "section", p.Line, p)
			r.refs = append(r.refs, varRefs(d.rel, "row/"+id, p, declared, "")...)
			return
		}
		if typ == "" && get(p, "libraryPanel") == nil {
			return
		}
		name := "panel/" + id
		r.def(d.rel, name, "element", p.Line, p)
		r.refs = append(r.refs, varRefs(d.rel, name, p, declared, "")...)
		if lp := scalar(get(get(p, "libraryPanel"), "uid")); lp != "" {
			if els := get(d.root, "__elements"); get(els, lp) != nil {
				r.refs = append(r.refs, pair{Path: d.rel, From: name, To: lp})
			}
		}
		var strs []string
		panelStrings(p, &strs, true)
		seen := map[string]bool{}
		for _, s := range strs {
			for _, m := range uidLinkRe.FindAllStringSubmatch(s, -1) {
				if !seen[m[1]] {
					seen[m[1]] = true
					r.pairTo(d.rel, name, r.uidFile, m[1], "links_to")
				}
			}
		}
		if u := scalar(get(get(p, "datasource"), "uid")); u != "" {
			r.pairTo(d.rel, name, r.dsFile, u, "datasource")
		}
		for _, t := range items(get(p, "targets")) {
			if u := scalar(get(get(t, "datasource"), "uid")); u != "" {
				r.pairTo(d.rel, name, r.dsFile, u, "datasource")
			}
			expr := scalar(get(t, "expr"))
			r.queryPairs(d, name, expr, r.datasourceKind(d, t, p), nil)
		}
	}
	panelsOf(get(dash, "panels"), prefix+"panels", visit)
	for i, row := range items(get(dash, "rows")) {
		panelsOf(get(row, "panels"), fmt.Sprintf("%srows[%d].panels", prefix, i), visit)
	}
}

// v2Vars is a v2 dashboard's declared variables.
func v2Vars(spec *yaml.Node) map[string]bool {
	out := map[string]bool{}
	for _, v := range items(get(spec, "variables")) {
		if name := scalar(get(get(v, "spec"), "name")); name != "" {
			out[name] = true
		}
	}
	return out
}

func v2Datasource(query *yaml.Node) string {
	switch k := scalar(get(query, "kind")); {
	case k == "DataQuery":
		return strings.ToLower(scalar(get(query, "group")))
	case k != "":
		return strings.ToLower(k)
	}
	return ""
}

var varRefRe = regexp.MustCompile(`^(?:\$\{([A-Za-z0-9_]+)(?:[:.][^}]*)?\}|\$([A-Za-z0-9_]+)|\[\[([A-Za-z0-9_]+)(?::[^\]]*)?\]\])$`)

// datasourceKind decides a panel target's datasource plugin: the target's,
// else the panel's (a mixed panel's targets carry their own), else the
// dashboard default (every datasource variable and input Prometheus).
func (r *result) datasourceKind(d dashFile, target, panel *yaml.Node) string {
	if k, set := r.dsRef(d, get(target, "datasource")); set {
		return k
	}
	if k, set := r.dsRef(d, get(panel, "datasource")); set && k != "mixed" {
		return k
	}
	var ids []string
	for _, v := range items(get(get(d.dash, "templating"), "list")) {
		if scalar(get(v, "type")) == "datasource" {
			ids = append(ids, strings.ToLower(varQuery(v)))
		}
	}
	for _, in := range items(get(d.root, "__inputs")) {
		if scalar(get(in, "type")) == "datasource" {
			ids = append(ids, strings.ToLower(scalar(get(in, "pluginId"))))
		}
	}
	if len(ids) == 0 {
		return ""
	}
	for _, id := range ids {
		if id != "prometheus" {
			return ""
		}
	}
	return "prometheus"
}

func varQuery(v *yaml.Node) string {
	q := get(v, "query")
	if q != nil && q.Kind == yaml.MappingNode {
		return scalar(get(q, "query"))
	}
	return scalar(q)
}

func (r *result) dsRef(d dashFile, ds *yaml.Node) (string, bool) {
	if ds == nil || ds.Tag == "!!null" {
		return "", false
	}
	ref := ""
	switch ds.Kind {
	case yaml.MappingNode:
		if t := scalar(get(ds, "type")); t != "" && !strings.Contains(t, "$") {
			if t == "datasource" {
				switch scalar(get(ds, "uid")) {
				case "-- Mixed --":
					return "mixed", true
				case "-- Dashboard --":
					return "dashboard", true
				case "grafana", "-- Grafana --":
					return "grafana", true
				}
				return "", true
			}
			return strings.ToLower(t), true
		}
		ref = scalar(get(ds, "uid"))
		if ref == "" {
			return "", true
		}
	case yaml.ScalarNode:
		ref = ds.Value
	default:
		return "", true
	}
	switch ref {
	case "-- Mixed --":
		return "mixed", true
	case "-- Grafana --", "grafana":
		return "grafana", true
	case "-- Dashboard --":
		return "dashboard", true
	}
	if m := varRefRe.FindStringSubmatch(strings.TrimSpace(ref)); m != nil {
		name := first(m[1], m[2], m[3])
		for _, v := range items(get(get(d.dash, "templating"), "list")) {
			if scalar(get(v, "name")) == name && scalar(get(v, "type")) == "datasource" {
				return strings.ToLower(varQuery(v)), true
			}
		}
		for _, in := range items(get(d.root, "__inputs")) {
			if scalar(get(in, "name")) == name && scalar(get(in, "type")) == "datasource" {
				return strings.ToLower(scalar(get(in, "pluginId"))), true
			}
		}
		return "", true
	}
	return "", true
}

// macroRe is Grafana's global variables in a query: $__name, ${__name},
// ${__name:fmt}, [[__name]].
var macroRe = regexp.MustCompile(`\$\{(__[A-Za-z0-9_]+)(?::[^}]*)?\}|\$(__[A-Za-z0-9_]+)|\[\[(__[A-Za-z0-9_]+)(?::[^\]]*)?\]\]`)

// varRe is any other template variable.
var varRe = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)(?:[:.][^}]*)?\}|\$([A-Za-z0-9_]+)|\[\[([A-Za-z0-9_]+)(?::[^\]]*)?\]\]`)

const placeholder = "grafana_template_variable_"

// interpolate rewrites a query as Grafana would send it, for the parser:
// macros a fixed duration or number; a variable inside a range's brackets a
// duration (an interval variable), any other outside a string a name no
// metric has; inside a string, left as written.
func interpolate(expr string) string {
	var b strings.Builder
	var q byte
	depth := 0 // inside a range or subquery's brackets
	i := 0
	for i < len(expr) {
		c := expr[i]
		if q == 0 && c == '[' && !strings.HasPrefix(expr[i:], "[[") {
			depth++
		}
		if q == 0 && c == ']' && depth > 0 && !strings.HasPrefix(expr[i:], "]]") {
			depth--
		}
		if q != 0 {
			b.WriteByte(c)
			i++
			if c == '\\' && q != '`' && i < len(expr) {
				b.WriteByte(expr[i])
				i++
			} else if c == q {
				q = 0
			}
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			q = c
			b.WriteByte(c)
			i++
			continue
		}
		if loc := macroRe.FindStringSubmatchIndex(expr[i:]); loc != nil && loc[0] == 0 {
			name := ""
			for g := 2; g < len(loc); g += 2 {
				if loc[g] >= 0 {
					name = expr[i+loc[g] : i+loc[g+1]]
				}
			}
			switch {
			case strings.HasSuffix(name, "_ms"):
				b.WriteString("300000")
			case strings.HasSuffix(name, "_s"):
				b.WriteString("300")
			case name == "__from" || name == "__to":
				b.WriteString("0")
			default:
				b.WriteString("5m")
			}
			i += loc[1]
			continue
		}
		if loc := varRe.FindStringSubmatchIndex(expr[i:]); loc != nil && loc[0] == 0 {
			name := ""
			for g := 2; g < len(loc); g += 2 {
				if loc[g] >= 0 {
					name = expr[i+loc[g] : i+loc[g+1]]
				}
			}
			if depth > 0 {
				b.WriteString("5m") // a range's duration, an interval variable
			} else {
				b.WriteString(placeholder + name)
			}
			i += loc[1]
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// queryPairs grades one query: read only for a Prometheus datasource; every
// metric name the parser's ExtractSelectors gives that a recording rule
// records is a pair.
func (r *result) queryPairs(d dashFile, from, expr, ds string, _ *yaml.Node) {
	if strings.TrimSpace(expr) == "" {
		return
	}
	switch ds {
	case "prometheus":
	case "":
		r.undecidedDS++
		return
	default:
		r.notProm++
		return
	}
	r.scanned++
	e, err := promParser.ParseExpr(interpolate(expr))
	if err != nil {
		r.refused++
		return
	}
	seen := map[string]bool{}
	for _, sel := range parser.ExtractSelectors(e) {
		for _, m := range sel {
			if m.Name != labels.MetricName || m.Type != labels.MatchEqual || strings.Contains(m.Value, placeholder) || strings.Contains(m.Value, "$") || seen[m.Value] {
				continue
			}
			seen[m.Value] = true
			r.pairTo(d.rel, from, r.records, m.Value, "uses_record")
		}
	}
}

func (r *result) readProvisioning(rel string, root *yaml.Node, dashes []dashFile) {
	r.def(rel, path.Base(rel), "package", root.Line, root)
	for _, p := range items(get(root, "providers")) {
		name := scalar(get(p, "name"))
		if name == "" {
			continue
		}
		r.def(rel, name, "data_source", p.Line, p)
		dir := scalar(get(get(p, "options"), "path"))
		if dir == "" {
			continue
		}
		var repoDir string
		if strings.HasPrefix(dir, "/") {
			repoDir = r.mounted(dir)
		} else {
			repoDir = path.Clean(path.Join(path.Dir(rel), dir))
		}
		if repoDir == "" {
			continue
		}
		for _, d := range dashes {
			if strings.HasPrefix(d.rel, repoDir+"/") {
				r.obs = append(r.obs, obsRef{Path: rel, From: name, ToPath: d.rel, To: d.uid, Edge: "provides"})
			}
		}
	}
	for _, ds := range items(get(root, "datasources")) {
		if name := first(scalar(get(ds, "uid")), scalar(get(ds, "name"))); name != "" {
			r.def(rel, name, "data_source", ds.Line, ds)
		}
	}
	for _, g := range items(get(root, "groups")) {
		gname := scalar(get(g, "name"))
		if gname == "" || get(g, "rules") == nil {
			continue
		}
		r.def(rel, gname, "section", g.Line, g)
		for _, ru := range items(get(g, "rules")) {
			title := scalar(get(ru, "title"))
			if title == "" {
				continue
			}
			r.def(rel, title, "rule", ru.Line, ru)
			for _, u := range []string{scalar(get(ru, "dashboardUid")), scalar(get(get(ru, "annotations"), "__dashboardUid__"))} {
				if u != "" {
					r.pairTo(rel, title, r.uidFile, u, "dashboard")
				}
			}
			for _, q := range items(get(ru, "data")) {
				if u := scalar(get(q, "datasourceUid")); u != "" {
					r.pairTo(rel, title, r.dsFile, u, "datasource")
				}
				ds := ""
				switch {
				case get(get(get(q, "model"), "datasource"), "type") != nil:
					ds = strings.ToLower(scalar(get(get(get(q, "model"), "datasource"), "type")))
				case scalar(get(q, "datasourceUid")) == "__expr__":
					ds = "__expr__"
				}
				r.queryPairs(dashFile{rel: rel, root: root, dash: root}, title, scalar(get(get(q, "model"), "expr")), ds, nil)
			}
			if rc := scalar(get(get(ru, "notification_settings"), "receiver")); rc != "" {
				r.pairTo(rel, title, r.cpFile, rc, "receiver")
			}
		}
	}
	for _, cp := range items(get(root, "contactPoints")) {
		if name := scalar(get(cp, "name")); name != "" {
			r.def(rel, name, "target", cp.Line, cp)
		}
	}
	var policy func(p *yaml.Node, name string)
	policy = func(p *yaml.Node, name string) {
		r.def(rel, name, "route", p.Line, p)
		if rc := scalar(get(p, "receiver")); rc != "" {
			r.pairTo(rel, name, r.cpFile, rc, "receiver")
		}
		for _, mt := range items(get(p, "mute_time_intervals")) {
			r.pairTo(rel, name, r.muteFile, scalar(mt), "mute_timing")
		}
		for i, c := range items(get(p, "routes")) {
			policy(c, fmt.Sprintf("%s.routes[%d]", name, i))
		}
	}
	for i, p := range items(get(root, "policies")) {
		policy(p, fmt.Sprintf("policies[%d]", i))
	}
	for _, mt := range items(get(root, "muteTimes")) {
		if name := scalar(get(mt, "name")); name != "" {
			r.def(rel, name, "variable_decl", mt.Line, mt)
		}
	}
	for _, t := range items(get(root, "templates")) {
		if name := scalar(get(t, "name")); name != "" {
			r.def(rel, name, "macro", t.Line, t)
		}
	}
}

// composeMounts reads a compose file's relative bind mounts.
func composeMounts(rel string, services *yaml.Node) [][2]string {
	var out [][2]string
	for i := 1; i < len(services.Content); i += 2 {
		for _, v := range items(get(services.Content[i], "volumes")) {
			var src, dst string
			switch v.Kind {
			case yaml.ScalarNode:
				parts := strings.Split(v.Value, ":")
				if len(parts) < 2 {
					continue
				}
				src, dst = parts[0], parts[1]
			case yaml.MappingNode:
				if t := scalar(get(v, "type")); t != "" && t != "bind" {
					continue
				}
				src, dst = scalar(get(v, "source")), scalar(get(v, "target"))
			}
			if !strings.HasPrefix(dst, "/") || strings.Contains(src, "$") || !(src == "." || strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../")) {
				continue
			}
			out = append(out, [2]string{path.Clean(path.Join(path.Dir(rel), src)), path.Clean(dst)})
		}
	}
	return out
}

// mounted maps a container path through the longest bind mount.
func (r *result) mounted(p string) string {
	p = path.Clean(p)
	best, bestLen := "", -1
	for _, m := range r.mounts {
		if p != m[1] && !strings.HasPrefix(p, strings.TrimSuffix(m[1], "/")+"/") {
			continue
		}
		if len(m[1]) > bestLen {
			best, bestLen = path.Clean(path.Join(m[0], strings.TrimPrefix(strings.TrimPrefix(p, m[1]), "/"))), len(m[1])
		}
	}
	return best
}

func (r *result) sort() {
	sort.Slice(r.defs, func(i, j int) bool {
		a, b := r.defs[i], r.defs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.Slice(r.refs, func(i, j int) bool {
		return r.refs[i].Path+"\x00"+r.refs[i].From+"\x00"+r.refs[i].To < r.refs[j].Path+"\x00"+r.refs[j].From+"\x00"+r.refs[j].To
	})
	var refs []pair
	for i, p := range r.refs {
		if i == 0 || p != r.refs[i-1] {
			refs = append(refs, p)
		}
	}
	r.refs = refs
	key := func(o obsRef) string {
		return o.Path + "\x00" + o.From + "\x00" + o.ToPath + "\x00" + o.To + "\x00" + o.Edge
	}
	sort.Slice(r.obs, func(i, j int) bool { return key(r.obs[i]) < key(r.obs[j]) })
	var dedup []obsRef
	for i, o := range r.obs {
		if i == 0 || key(o) != key(r.obs[i-1]) {
			dedup = append(dedup, o)
		}
	}
	r.obs = dedup
	sort.Strings(r.files)
	if r.refs == nil {
		r.refs = []pair{}
	}
	if r.obs == nil {
		r.obs = []obsRef{}
	}
}

// deriveEmbedded writes the dashboards and provisioning files Kubernetes
// objects embed (a ConfigMap's or ConfigMapList item's .json, .yaml or .yml
// data, a Secret's .yaml or .yml stringData or base64 data, a
// GrafanaDashboard's spec.json) as files under out/<manifest without its
// extension>/<key>, and removes the manifest from out, so a manifest tree is
// measured as extracted documents (the manifests are ingest's k8sembed path).
func deriveEmbedded(subject, out string) (int, error) {
	n := 0
	err := filepath.WalkDir(subject, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var doc yaml.Node
		if yaml.Unmarshal(b, &doc) != nil || len(doc.Content) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(subject, p)
		dir := filepath.Join(out, strings.TrimSuffix(rel, filepath.Ext(rel)))
		wrote := 0
		write := func(key string, content []byte) error {
			var probe yaml.Node
			if yaml.Unmarshal(content, &probe) != nil || len(probe.Content) == 0 || probe.Content[0].Kind != yaml.MappingNode {
				return nil
			}
			root := probe.Content[0]
			if _, ok := dashboardOf(root); !ok && !isProvisioning(root) {
				return nil
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			wrote++
			return os.WriteFile(filepath.Join(dir, key), content, 0o644)
		}
		var objects []*yaml.Node
		root := doc.Content[0]
		if strings.HasSuffix(scalar(get(root, "kind")), "List") {
			objects = items(get(root, "items"))
		} else {
			objects = []*yaml.Node{root}
		}
		for _, o := range objects {
			kind := scalar(get(o, "kind"))
			switch kind {
			case "ConfigMap", "Secret":
				for _, field := range []string{"data", "stringData"} {
					data := get(o, field)
					for i := 0; data != nil && i+1 < len(data.Content); i += 2 {
						k, v := data.Content[i].Value, data.Content[i+1].Value
						kext := strings.ToLower(filepath.Ext(k))
						if kext != ".json" && kext != ".yaml" && kext != ".yml" {
							continue
						}
						content := []byte(v)
						if kind == "Secret" && field == "data" {
							dec, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(v), ""))
							if err != nil {
								continue
							}
							content = dec
						}
						if err := write(k, content); err != nil {
							return err
						}
					}
				}
			case "GrafanaDashboard":
				name := first(scalar(get(get(o, "metadata"), "name")), "dashboard")
				if err := write(name+".json", []byte(scalar(get(get(o, "spec"), "json")))); err != nil {
					return err
				}
			}
		}
		if wrote > 0 {
			n += wrote
			_ = os.Remove(filepath.Join(out, rel))
		}
		return nil
	})
	return n, err
}
