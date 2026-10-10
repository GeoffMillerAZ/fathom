// Command alertmanager-oracle is the language-breadth harness's tier-A oracle
// for the `alertmanager` doc-engine dialect (LB-Q-alertmanager, design §1.3,
// §10): Alertmanager's own tools decide what a configuration declares and
// where an alert goes. A separate Go module, never imported by fathom, never
// a product dependency (ADR-020, ADR-025).
//
//	alertmanager-oracle -subject <checkout> -amtool <amtool> \
//	    -refs-out <refs.json> -obs-out <obs.json> -files-out <files.txt> > <defs.json>
//	alertmanager-oracle -list-secret-fields > secret_fields.json
//	alertmanager-oracle -derive-embedded <out> -subject <checkout>
//
// The instrument: `amtool check-config` (v0.34.1) accepts or refuses each
// configuration; the config package (config.LoadFile) names what an accepted
// one declares: its route tree (each route named by its path, `route`,
// `route.routes[0]`), receivers, inhibit rules, time intervals and template
// globs; a route's receiver and time intervals are its references. The
// dispatch package (dispatch.NewRoute, Route.Match: amtool `config routes
// test`'s own code) routes every alerting rule of the subject's Prometheus
// rule files (promtool check rules acceptance when -promtool is given; a
// rule's alert name and static labels, its group's too, read as written) and
// each routing is cross-checked by running amtool config routes test.
// Lines are not Alertmanager output: they come from yaml.v3, a mapping's
// first key, and an end line is the last line any node under it is written
// on.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/dispatch"
	"github.com/prometheus/alertmanager/pkg/labels"
	"github.com/prometheus/common/model"
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

// obsRef is one cross-dialect pair: an Alertmanager entity and the
// Prometheus alerting rule its alertname names, or an alerting rule and a
// receiver its alerts route to.
type obsRef struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to"`
	Edge   string `json:"edge"`
}

// undecided is an alerting rule whose routing reads a label it does not set
// statically: Alertmanager's answer for its static labels alone, disclosed,
// never scored.
type undecided struct {
	Path      string   `json:"path"`
	Rule      string   `json:"rule"`
	ToPath    string   `json:"to_path"`
	Receivers []string `json:"receivers"`
	Labels    []string `json:"labels"`
}

type crossCheck struct {
	Tool     string   `json:"tool"`
	Tier     string   `json:"tier"`
	Checked  int      `json:"checked"`
	Agree    int      `json:"agree"`
	Disagree []string `json:"disagree"`
}

type defsFile struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
}

type refsFile struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
	References  []pair `json:"references"`
}

type obsFile struct {
	Tool          string      `json:"tool"`
	Version       string      `json:"version"`
	Tier          string      `json:"tier"`
	Definitions   []def       `json:"definitions"`
	ObsReferences []obsRef    `json:"obs_references"`
	ObsDecided    []pair      `json:"obs_decided"`
	ObsUndecided  []undecided `json:"obs_undecided"`
	ObsCrossCheck *crossCheck `json:"obs_crosscheck,omitempty"`
}

const amVersion = "v0.34.1"

// skipDirs are the directories fathom's connector never reads (connector.go).
var skipDirs = map[string]bool{"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true, ".idea": true, ".vscode": true, ".claude": true}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	amtool := flag.String("amtool", "", "amtool binary (v0.34.1)")
	promtool := flag.String("promtool", "", "promtool binary: rule files it refuses are not read (optional)")
	refsOut := flag.String("refs-out", "", "write references here")
	obsOut := flag.String("obs-out", "", "write the cross-dialect pairs here")
	filesOut := flag.String("files-out", "", "write the accepted configurations, one per line")
	listSecrets := flag.Bool("list-secret-fields", false, "print every Secret-typed field of the configuration, by reflection, and exit")
	derive := flag.String("derive-embedded", "", "write every Alertmanager configuration a Kubernetes Secret or ConfigMap embeds as a file at the manifest's path under this directory, and exit")
	flag.Parse()
	switch {
	case *listSecrets:
		b, _ := json.MarshalIndent(secretFields(), "", " ")
		os.Stdout.Write(append(b, '\n'))
		return
	case *derive != "":
		n, err := deriveEmbedded(*subject, *derive)
		if err != nil {
			fmt.Fprintln(os.Stderr, "derive:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "derived %d embedded configurations\n", n)
		return
	case *subject == "" || *amtool == "":
		fmt.Fprintln(os.Stderr, "usage: alertmanager-oracle -subject <dir> -amtool <bin> [-promtool <bin>] [-refs-out f] [-obs-out f] [-files-out f]")
		os.Exit(2)
	}
	r := run(*subject, *amtool, *promtool)
	version := amVersion + " (amtool, config and dispatch packages)"
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
	emit("", defsFile{Tool: "amtool", Version: version, Tier: "A", Definitions: r.defs})
	if *refsOut != "" {
		emit(*refsOut, refsFile{Tool: "amtool", Version: version, Tier: "A", Definitions: []def{}, References: r.refs})
	}
	if *obsOut != "" {
		emit(*obsOut, obsFile{Tool: "amtool+dispatch", Version: version, Tier: "A", Definitions: []def{}, ObsReferences: r.obs, ObsDecided: r.decided, ObsUndecided: r.undecided, ObsCrossCheck: r.cross})
	}
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(r.files, "\n")+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "configurations accepted %d, refused %d %v; rule files %d (%d alerting rules); definitions %d, references %d, obs pairs %d (decided rules %d, undecided %d); amtool routes test %d checked, %d agree\n",
		len(r.files), len(r.refused), r.refused, r.ruleFiles, len(r.rules), len(r.defs), len(r.refs), len(r.obs), len(r.decided), len(r.undecided), r.cross.Checked, r.cross.Agree)
}

type result struct {
	files, refused []string
	ruleFiles      int
	defs           []def
	refs           []pair
	obs            []obsRef
	decided        []pair
	undecided      []undecided
	rules          []rule
	cross          *crossCheck
}

// rule is one Prometheus alerting rule with its static labels.
type rule struct {
	path, name string
	labels     model.LabelSet
	dynamic    map[string]bool // labels whose value is templated
}

type amFile struct {
	rel  string
	abs  string
	cfg  *config.Config
	root *yaml.Node
}

func run(subject, amtool, promtool string) result {
	r := result{cross: &crossCheck{Tool: "amtool config routes test " + amVersion, Tier: "A", Disagree: []string{}}}
	var ams []amFile
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
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		rel, _ := filepath.Rel(subject, p)
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		root := firstMapping(b)
		if root == nil || (ymap(root, "apiVersion") != nil && ymap(root, "kind") != nil) {
			return nil
		}
		switch {
		case ymap(root, "route") != nil && ymap(root, "receivers") != nil:
			if out, err := exec.Command(amtool, "check-config", p).CombinedOutput(); err != nil {
				r.refused = append(r.refused, rel+": "+firstLine(string(out)))
				return nil
			}
			cfg, err := config.LoadFile(p)
			if err != nil {
				r.refused = append(r.refused, rel+": "+err.Error())
				return nil
			}
			ams = append(ams, amFile{rel: rel, abs: p, cfg: cfg, root: root})
			r.files = append(r.files, rel)
		case ymap(root, "groups") != nil:
			if promtool != "" {
				if err := exec.Command(promtool, "check", "rules", p).Run(); err != nil {
					return nil
				}
			}
			rs := readRules(rel, root)
			if len(rs) > 0 {
				r.ruleFiles++
				r.rules = append(r.rules, rs...)
			}
		}
		return nil
	})
	for _, f := range ams {
		r.defs = append(r.defs, definitions(f)...)
		r.refs = append(r.refs, references(f)...)
	}
	byName := map[string][]rule{}
	for _, ru := range r.rules {
		byName[ru.name] = append(byName[ru.name], ru)
	}
	for _, f := range ams {
		r.obs = append(r.obs, alertnameRefs(f, byName)...)
	}
	routeRules(&r, ams, amtool)
	sortAll(&r)
	return r
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return l
}

// firstMapping is a file's first YAML document when it is a mapping.
func firstMapping(b []byte) *yaml.Node {
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil || len(n.Content) == 0 || n.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	return n.Content[0]
}

func ymap(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == k {
				next = n.Content[i+1]
				break
			}
		}
		n = next
	}
	return n
}

// ykeyLine is the line a mapping's key is written on.
func ykeyLine(n *yaml.Node, key string) int {
	for i := 0; n != nil && i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i].Line
		}
	}
	return 0
}

func yseq(n *yaml.Node, i int) *yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode || i >= len(n.Content) {
		return nil
	}
	return n.Content[i]
}

// endLine is the last line any node under n is written on.
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

// lineOf is where a node is written: a mapping's first key.
func lineOf(n *yaml.Node) int {
	if n == nil {
		return 0
	}
	if n.Kind == yaml.MappingNode && len(n.Content) > 0 {
		return n.Content[0].Line
	}
	return n.Line
}

func definitions(f amFile) []def {
	var out []def
	add := func(name, kind string, n *yaml.Node) {
		out = append(out, def{Path: f.rel, Name: name, Line: lineOf(n), End: endLine(n), Kind: kind})
	}
	add(path.Base(f.rel), "package", f.root)
	var walk func(r *config.Route, p string, n *yaml.Node)
	walk = func(r *config.Route, p string, n *yaml.Node) {
		add(p, "route", n)
		for i, c := range r.Routes {
			cp := fmt.Sprintf("%s.routes[%d]", p, i)
			walk(c, cp, yseq(ymap(n, "routes"), i))
		}
	}
	if f.cfg.Route != nil {
		walk(f.cfg.Route, "route", ymap(f.root, "route"))
		// the root route is written where its key is (a value under a key),
		// a child route where its first key is (a sequence item)
		for i := range out {
			if out[i].Name == "route" && out[i].Kind == "route" {
				out[i].Line = ykeyLine(f.root, "route")
			}
		}
	}
	for i, rc := range f.cfg.Receivers {
		add(rc.Name, "target", yseq(ymap(f.root, "receivers"), i))
	}
	for i := range f.cfg.InhibitRules {
		add(fmt.Sprintf("inhibit_rules[%d]", i), "rule", yseq(ymap(f.root, "inhibit_rules"), i))
	}
	for i, ti := range f.cfg.TimeIntervals {
		add(ti.Name, "variable_decl", yseq(ymap(f.root, "time_intervals"), i))
	}
	for i, ti := range f.cfg.MuteTimeIntervals {
		add(ti.Name, "variable_decl", yseq(ymap(f.root, "mute_time_intervals"), i))
	}
	// config.LoadFile makes each template glob absolute: the glob as written
	// is the yaml's, the count the configuration's
	if t := ymap(f.root, "templates"); t != nil && t.Kind == yaml.SequenceNode && len(t.Content) == len(f.cfg.Templates) {
		for _, g := range t.Content {
			add(g.Value, "import", g)
		}
	}
	return out
}

func references(f amFile) []pair {
	var out []pair
	var walk func(r *config.Route, p string)
	walk = func(r *config.Route, p string) {
		if r.Receiver != "" {
			out = append(out, pair{Path: f.rel, From: p, To: r.Receiver})
		}
		for _, t := range r.MuteTimeIntervals {
			out = append(out, pair{Path: f.rel, From: p, To: t})
		}
		for _, t := range r.ActiveTimeIntervals {
			out = append(out, pair{Path: f.rel, From: p, To: t})
		}
		for i, c := range r.Routes {
			walk(c, fmt.Sprintf("%s.routes[%d]", p, i))
		}
	}
	if f.cfg.Route != nil {
		walk(f.cfg.Route, "route")
	}
	return out
}

// readRules reads a rule file's alerting rules: name, and the static labels
// (the group's, then the rule's), a templated value marked dynamic.
func readRules(rel string, root *yaml.Node) []rule {
	var out []rule
	groups := ymap(root, "groups")
	if groups == nil || groups.Kind != yaml.SequenceNode {
		return nil
	}
	for _, g := range groups.Content {
		rs := ymap(g, "rules")
		if rs == nil || rs.Kind != yaml.SequenceNode {
			continue
		}
		for _, rn := range rs.Content {
			a := ymap(rn, "alert")
			if a == nil || a.Value == "" {
				continue
			}
			ru := rule{path: rel, name: a.Value, labels: model.LabelSet{"alertname": model.LabelValue(a.Value)}, dynamic: map[string]bool{}}
			for _, ls := range []*yaml.Node{ymap(g, "labels"), ymap(rn, "labels")} {
				if ls == nil || ls.Kind != yaml.MappingNode {
					continue
				}
				for i := 0; i+1 < len(ls.Content); i += 2 {
					k, v := ls.Content[i].Value, ls.Content[i+1].Value
					if strings.Contains(v, "{{") {
						ru.dynamic[k] = true
						delete(ru.labels, model.LabelName(k))
						continue
					}
					if !ru.dynamic[k] {
						ru.labels[model.LabelName(k)] = model.LabelValue(v)
					}
				}
			}
			out = append(out, ru)
		}
	}
	return out
}

// alertnameRefs: a route's and an inhibit rule's alertname equality
// matchers (the parsed matchers, match and match_re as Alertmanager merges
// them) to every alerting rule of that name.
func alertnameRefs(f amFile, byName map[string][]rule) []obsRef {
	var out []obsRef
	emit := func(from, edge string, names []string) {
		for _, n := range names {
			for _, ru := range byName[n] {
				out = append(out, obsRef{Path: f.rel, From: from, ToPath: ru.path, To: ru.name, Edge: edge})
			}
		}
	}
	if f.cfg.Route != nil {
		var walk func(cr *config.Route, dr *dispatch.Route, p string)
		walk = func(cr *config.Route, dr *dispatch.Route, p string) {
			emit(p, "alertname", alertnames(dr.Matchers))
			for i := range cr.Routes {
				walk(cr.Routes[i], dr.Routes[i], fmt.Sprintf("%s.routes[%d]", p, i))
			}
		}
		walk(f.cfg.Route, dispatch.NewRoute(f.cfg.Route, nil), "route")
	}
	for i, ir := range f.cfg.InhibitRules {
		from := fmt.Sprintf("inhibit_rules[%d]", i)
		src := alertnames(labels.Matchers(ir.SourceMatchers))
		if v, ok := ir.SourceMatch["alertname"]; ok {
			src = append(src, v)
		}
		emit(from, "inhibit_source", src)
		tgt := alertnames(labels.Matchers(ir.TargetMatchers))
		if v, ok := ir.TargetMatch["alertname"]; ok {
			tgt = append(tgt, v)
		}
		emit(from, "inhibit_target", tgt)
	}
	return out
}

func alertnames(ms labels.Matchers) []string {
	var out []string
	for _, m := range ms {
		if m.Name == "alertname" && m.Type == labels.MatchEqual {
			out = append(out, m.Value)
		}
	}
	return out
}

// routeRules routes every alerting rule through every configuration by
// dispatch.Route.Match on its static labels. A rule is decided for a
// configuration when every matcher Match evaluates reads a static label (or
// a route already fails on one): its receivers are then its alerts', and
// scored; otherwise Alertmanager's answer for its static labels alone is
// disclosed. Each decided routing is cross-checked with amtool config
// routes test.
func routeRules(r *result, ams []amFile, amtool string) {
	type key struct{ path, name, to string }
	decided := map[[2]string]bool{}
	for _, f := range ams {
		if f.cfg.Route == nil {
			continue
		}
		root := dispatch.NewRoute(f.cfg.Route, nil)
		// a rule name's occurrences are one entity: decided only when every
		// occurrence is
		type agg struct {
			recv     map[string]bool
			undec    bool
			labels   []string
			occurred int
		}
		byRule := map[[2]string]*agg{}
		var order [][2]string
		for _, ru := range r.rules {
			k := [2]string{ru.path, ru.name}
			a := byRule[k]
			if a == nil {
				a = &agg{recv: map[string]bool{}}
				byRule[k] = a
				order = append(order, k)
			}
			a.occurred++
			var recvs []string
			for _, m := range root.Match(ru.labels) {
				recvs = append(recvs, m.RouteOpts.Receiver)
				a.recv[m.RouteOpts.Receiver] = true
			}
			if !routeDecided(root, ru.labels) {
				a.undec = true
				for l := range readLabels(root, ru.labels) {
					if _, ok := ru.labels[model.LabelName(l)]; !ok {
						a.labels = append(a.labels, l)
					}
				}
				continue
			}
			// the amtool cross-check
			args := []string{"config", "routes", "test", "--config.file=" + f.abs}
			for ln, lv := range ru.labels {
				args = append(args, string(ln)+"="+string(lv))
			}
			out, err := exec.Command(amtool, args...).Output()
			got := strings.Split(strings.TrimSpace(string(out)), ",")
			sort.Strings(got)
			want := dedupe(recvs)
			r.cross.Checked++
			if err == nil && reflect.DeepEqual(got, want) {
				r.cross.Agree++
			} else {
				r.cross.Disagree = append(r.cross.Disagree, fmt.Sprintf("%s %s: amtool %v, dispatch %v", f.rel, ru.name, got, want))
			}
		}
		for _, k := range order {
			a := byRule[k]
			var recvs []string
			for rc := range a.recv {
				recvs = append(recvs, rc)
			}
			sort.Strings(recvs)
			if a.undec {
				r.undecided = append(r.undecided, undecided{Path: k[0], Rule: k[1], ToPath: f.rel, Receivers: recvs, Labels: dedupe(a.labels)})
				continue
			}
			decided[[2]string{k[0], k[1]}] = true
			for _, rc := range recvs {
				r.obs = append(r.obs, obsRef{Path: k[0], From: k[1], ToPath: f.rel, To: rc, Edge: "routes_to"})
			}
		}
	}
	for k := range decided {
		r.decided = append(r.decided, pair{Path: k[0], From: k[1]})
	}
	_ = key{}
}

// routeDecided replays Route.Match, reporting whether every route outcome it
// reads is decided by the static labels: a route's outcome is decided when
// all its matchers read static labels, or one that reads a static label
// fails.
func routeDecided(r *dispatch.Route, ls model.LabelSet) bool {
	state := func(rt *dispatch.Route) (match, decided bool) {
		match, decided = true, true
		for _, m := range rt.Matchers {
			v, ok := ls[model.LabelName(m.Name)]
			if !ok {
				decided = false
				continue
			}
			if !m.Matches(string(v)) {
				return false, true
			}
		}
		if !decided {
			return false, false
		}
		return true, true
	}
	var walk func(rt *dispatch.Route) (matched, ok bool)
	walk = func(rt *dispatch.Route) (bool, bool) {
		m, d := state(rt)
		if !d {
			return false, false
		}
		if !m {
			return false, true
		}
		for _, c := range rt.Routes {
			cm, ok := walk(c)
			if !ok {
				return false, false
			}
			if cm && !c.Continue {
				break
			}
		}
		return true, true
	}
	_, ok := walk(r)
	return ok
}

// readLabels is every label a matcher of the tree reads.
func readLabels(r *dispatch.Route, _ model.LabelSet) map[string]bool {
	out := map[string]bool{}
	var walk func(rt *dispatch.Route)
	walk = func(rt *dispatch.Route) {
		for _, m := range rt.Matchers {
			out[m.Name] = true
		}
		for _, c := range rt.Routes {
			walk(c)
		}
	}
	walk(r)
	return out
}

func dedupe(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	if out == nil {
		out = []string{}
	}
	return out
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
		return a.Name < b.Name
	})
	sort.Slice(r.refs, func(i, j int) bool {
		a, b := r.refs[i], r.refs[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	sort.Slice(r.obs, func(i, j int) bool {
		a, b := r.obs[i], r.obs[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To+"\x00"+a.Edge < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To+"\x00"+b.Edge
	})
	sort.Slice(r.decided, func(i, j int) bool {
		return r.decided[i].Path+"\x00"+r.decided[i].From < r.decided[j].Path+"\x00"+r.decided[j].From
	})
	sort.Slice(r.undecided, func(i, j int) bool {
		a, b := r.undecided[i], r.undecided[j]
		return a.Path+"\x00"+a.Rule+"\x00"+a.ToPath < b.Path+"\x00"+b.Rule+"\x00"+b.ToPath
	})
	sort.Strings(r.files)
	sort.Strings(r.refused)
	if r.refs == nil {
		r.refs = []pair{}
	}
	if r.obs == nil {
		r.obs = []obsRef{}
	}
	if r.decided == nil {
		r.decided = []pair{}
	}
	if r.undecided == nil {
		r.undecided = []undecided{}
	}
}

// deriveEmbedded writes each Alertmanager configuration a Kubernetes Secret
// (stringData, or base64 data) or ConfigMap embeds under an
// alertmanager.yaml/.yml key as a plain file at the manifest's relative path
// under out, so a manifest tree is measured as extracted configurations (the
// manifest itself is ingest's k8sembed path, LB-F366).
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
		root := firstMapping(b)
		kind := ymap(root, "kind")
		if kind == nil || (kind.Value != "Secret" && kind.Value != "ConfigMap") {
			return nil
		}
		for _, field := range []string{"stringData", "data"} {
			data := ymap(root, field)
			if data == nil || data.Kind != yaml.MappingNode {
				continue
			}
			for i := 0; i+1 < len(data.Content); i += 2 {
				k, v := data.Content[i].Value, data.Content[i+1].Value
				if k != "alertmanager.yaml" && k != "alertmanager.yml" {
					continue
				}
				content := []byte(v)
				if field == "data" && kind.Value == "Secret" {
					dec, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(v), ""))
					if err != nil {
						continue
					}
					content = dec
				}
				rel, _ := filepath.Rel(subject, p)
				dst := filepath.Join(out, rel)
				if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(dst, content, 0o644); err != nil {
					return err
				}
				n++
			}
		}
		return nil
	})
	return n, err
}
