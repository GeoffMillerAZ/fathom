package fusion

// alertroute.go — the observability joins of wave c3 (LB-F362, LB-F363;
// docs/design/language-breadth.md §4 "Observability documents"). One
// resolver, ResolveObsXrefs, serves ingest (the doc-xref pass's one hook,
// Pipeline.obsXrefs) and the language-breadth harness (indexbench lang-obs),
// so the bench never scores an edge ingest drops. Its edges are stored with
// the doc-xref pass's: file-tier typed edges (core.EdgeFileResolved,
// pre-scan, never a gate, I57) under the typed_edges key doc-xref; the edge
// kind is a data label on DocXref (Edge), never a new key kind or table.
//
//   - entity-name references (a doc spec's `resolve: entity-name` with a
//     `to_kind`): every entity of that source kind and name, in any file (a
//     Grafana panel's metric name to the recording rule that records it, a
//     `/d/<uid>` link to the dashboard of that uid, a datasource uid, a
//     policy's contact point). A metric name is never joined to emitter code
//     (Stage B is gated); a name no entity of the kind has joins nothing.
//   - repo-tree references: every dialect file under a directory whose root
//     entity is of to_kind (a Grafana dashboard provider's options.path); an
//     absolute container path reads through a compose bind mount.
//   - Alertmanager: a route's or an inhibit rule's `alertname` equality
//     matcher to every Prometheus alerting rule of that name; and every
//     alerting rule to the receivers its alert name and static labels route
//     to (routeAlert), `routes_to` when the static labels decide it,
//     `may_route_to` when a matcher reads a label the rule does not set
//     statically (its runtime labels are unknown), never a guess presented
//     as certain. K19: no identity-label join to a deploy: key here.

import (
	"context"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/promql"
	"gopkg.in/yaml.v3"
)

// The observability edge labels (DocXref.Edge) the resolver writes beyond
// the doc specs' own.
const (
	EdgeAlertName     = "alertname"      // a route's alertname matcher to the alerting rule
	EdgeInhibitSource = "inhibit_source" // an inhibit rule's source alertname to the alerting rule
	EdgeInhibitTarget = "inhibit_target" // an inhibit rule's target alertname to the alerting rule
	EdgeRoutesTo      = "routes_to"      // an alerting rule to a receiver its static labels decide
	EdgeMayRouteTo    = "may_route_to"   // an alerting rule to a receiver a runtime label may decide
)

// obsFanOut caps one reference's targets: an entity-name joins every entity
// of its kind and name, and a repo-tree every dashboard under a directory.
const obsFanOut = 256

// BindMount is a compose bind mount: a repository directory (Source, a
// repo-relative path) mounted at an absolute container path (Target).
type BindMount struct {
	Source, Target string
}

// ObsResolution is ResolveObsXrefs' answer with its disclosure.
type ObsResolution struct {
	Refs []DocXref
	// Unresolved is every reference that joined nothing, with its reason:
	// "undeclared" (no entity of the kind and name, no file under the
	// directory), "fan-out" (more targets than obsFanOut), "unmounted" (an
	// absolute container path no bind mount maps).
	Unresolved []ObsUnresolved
	// MayRoutes counts the may_route_to edges, Rules the alerting rules
	// routed, Configs the Alertmanager configurations read.
	MayRoutes, Rules, Configs int
}

// ObsUnresolved is one reference that joined nothing.
type ObsUnresolved struct {
	File, From, Edge, Raw, Reason string
}

// ResolveObsXrefs resolves the observability joins over a set of doc files
// (deterministic, sorted).
func ResolveObsXrefs(files []DocFile, mounts []BindMount) ObsResolution {
	var res ObsResolution
	ix := newObsIndex(files)
	add := func(from, to obsNode, edge, strategy string) {
		if from.key == to.key {
			return
		}
		res.Refs = append(res.Refs, DocXref{FromFile: from.file, From: from.name, FromKey: from.key, ToFile: to.file, To: to.name, ToKey: to.key, Edge: edge, Strategy: strategy})
	}
	res.Unresolved = resolveEntityNames(ix, files, mounts, add)
	routes := resolveAlertRoutes(ix, files, add)
	res.MayRoutes, res.Rules, res.Configs = routes.may, routes.rules, routes.configs
	res.Refs = dedupeXrefs(res.Refs)
	return res
}

// obsNode is one entity a join reads.
type obsNode struct {
	file, name, kind, key string
	line, end             int
}

// obsIndex indexes a doc set's entities by kind and name.
type obsIndex struct {
	byKindName map[[2]string][]obsNode
	byFileName map[[2]string]obsNode
	roots      map[string][]obsNode // a file's root entities
}

func newObsIndex(files []DocFile) *obsIndex {
	ix := &obsIndex{byKindName: map[[2]string][]obsNode{}, byFileName: map[[2]string]obsNode{}, roots: map[string][]obsNode{}}
	for _, f := range files {
		isRoot := map[string]bool{}
		for _, r := range f.Result.Roots {
			isRoot[r] = true
		}
		for _, n := range f.Result.Nodes {
			on := obsNode{file: f.Path, name: n.Name, kind: string(n.Kind), key: n.Key(), line: n.Line, end: n.EndLine}
			k := [2]string{f.Path, n.Name}
			if _, dup := ix.byFileName[k]; dup {
				continue
			}
			ix.byFileName[k] = on
			ix.byKindName[[2]string{on.kind, n.Name}] = append(ix.byKindName[[2]string{on.kind, n.Name}], on)
			if isRoot[n.Name] {
				ix.roots[f.Path] = append(ix.roots[f.Path], on)
			}
		}
	}
	return ix
}

// origin is the entity a reference leaves: its From, else the file's first
// root.
func (ix *obsIndex) origin(file, from string) (obsNode, bool) {
	if from == "" {
		if rs := ix.roots[file]; len(rs) > 0 {
			return rs[0], true
		}
		return obsNode{}, false
	}
	n, ok := ix.byFileName[[2]string{file, from}]
	return n, ok
}

// resolveEntityNames resolves the entity-name and repo-tree references.
func resolveEntityNames(ix *obsIndex, files []DocFile, mounts []BindMount, add func(from, to obsNode, edge, strategy string)) []ObsUnresolved {
	var unresolved []ObsUnresolved
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	sort.Strings(paths)
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			miss := func(reason string) {
				unresolved = append(unresolved, ObsUnresolved{File: f.Path, From: c.From, Edge: c.Edge, Raw: c.Raw, Reason: reason})
			}
			switch c.Strategy {
			case "entity-name":
				from, ok := ix.origin(f.Path, c.From)
				if !ok {
					continue
				}
				to := ix.byKindName[[2]string{c.ToKind, c.Raw}]
				switch {
				case len(to) == 0:
					miss("undeclared")
				case len(to) > obsFanOut:
					miss("fan-out")
				default:
					for _, t := range to {
						add(from, t, c.Edge, c.Strategy)
					}
				}
			case "repo-tree":
				from, ok := ix.origin(f.Path, c.From)
				if !ok {
					continue
				}
				dir := c.Target
				if strings.HasPrefix(c.Raw, "/") {
					d, ok := mountedDir(c.Raw, mounts)
					if !ok {
						miss("unmounted")
						continue
					}
					dir = d
				}
				var hit []obsNode
				for _, p := range paths {
					if dir == "" || !(dir == "." || strings.HasPrefix(p, dir+"/")) {
						continue
					}
					for _, r := range ix.roots[p] {
						if c.ToKind == "" || r.kind == c.ToKind {
							hit = append(hit, r)
						}
					}
				}
				switch {
				case len(hit) == 0:
					miss("undeclared")
				case len(hit) > obsFanOut:
					miss("fan-out")
				default:
					for _, t := range hit {
						add(from, t, c.Edge, c.Strategy)
					}
				}
			}
		}
	}
	return unresolved
}

// mountedDir maps an absolute container path through the bind mounts: the
// mount whose target is its longest prefix, "" and false when none (or two
// different directories at that length) maps it.
func mountedDir(p string, mounts []BindMount) (string, bool) {
	p = path.Clean(p)
	best, bestLen, ambiguous := "", -1, false
	for _, m := range mounts {
		t := path.Clean(m.Target)
		if p != t && !strings.HasPrefix(p, strings.TrimSuffix(t, "/")+"/") {
			continue
		}
		rest := strings.TrimPrefix(strings.TrimPrefix(p, t), "/")
		d := path.Clean(path.Join(m.Source, rest))
		switch {
		case len(t) > bestLen:
			best, bestLen, ambiguous = d, len(t), false
		case len(t) == bestLen && d != best:
			ambiguous = true
		}
	}
	if bestLen < 0 || ambiguous || best == ".." || strings.HasPrefix(best, "../") {
		return "", false
	}
	return best, true
}

// ── Alertmanager ────────────────────────────────────────────────────────────

// amRoute is one route of an Alertmanager configuration's tree.
type amRoute struct {
	node     obsNode
	receiver string // its own; "" inherits
	cont     bool
	matchers []amMatcher
	unparsed bool // a matcher did not read: the route's match is never decided
	children []*amRoute
}

// amMatcher is a route matcher with its anchored regular expression (nil for
// = and !=, or for one that does not compile: Alertmanager refuses that
// configuration, and the match stays undecided).
type amMatcher struct {
	promql.Matcher
	re *regexp.Regexp
}

func newAMMatcher(m promql.Matcher) amMatcher {
	am := amMatcher{Matcher: m}
	if m.Op == promql.OpRegex || m.Op == promql.OpNotRegex {
		am.re, _ = regexp.Compile("^(?:" + m.Value + ")$")
	}
	return am
}

// amConfig is one Alertmanager configuration: its route tree and receivers.
type amConfig struct {
	file      string
	root      *amRoute
	receivers map[string]obsNode
}

// amRule is one Prometheus alerting rule with its static labels.
type amRule struct {
	node   obsNode
	labels map[string]string // static: written in the rule or its group, never templated
}

// routeNameRe reads a route entity's name: an optional embedding prefix
// (k8sembed.go qualifies an embedded document's entities `<key>/`), then the
// path from `route`.
var routeNameRe = regexp.MustCompile(`^(.*/)?route((?:\.routes\[\d+\])*)$`)

var routeStepRe = regexp.MustCompile(`\.routes\[(\d+)\]`)

type alertRouteStats struct{ may, rules, configs int }

// resolveAlertRoutes joins alertname matchers to alerting rules and routes
// every alerting rule through every Alertmanager configuration.
func resolveAlertRoutes(ix *obsIndex, files []DocFile, add func(from, to obsNode, edge, strategy string)) alertRouteStats {
	var st alertRouteStats
	configs := amConfigs(ix, files)
	rules := amRules(ix, files)
	byName := map[string][]amRule{}
	for _, r := range rules {
		byName[r.node.name] = append(byName[r.node.name], r)
	}
	// alertname matchers: a route's and an inhibit rule's
	for _, f := range files {
		for _, ms := range f.Result.Matchers {
			from, ok := ix.byFileName[[2]string{f.Path, ms.Entity}]
			if !ok {
				continue
			}
			edge := ""
			switch {
			case from.kind == "AlertmanagerRoute" && ms.Edge == "match":
				edge = EdgeAlertName
			case from.kind == "AlertmanagerInhibitRule" && ms.Edge == "source":
				edge = EdgeInhibitSource
			case from.kind == "AlertmanagerInhibitRule" && ms.Edge == "target":
				edge = EdgeInhibitTarget
			default:
				continue
			}
			for _, m := range ms.Matchers {
				if m.Name != "alertname" || m.Op != promql.OpEqual {
					continue
				}
				for _, r := range byName[m.Value] {
					add(from, r.node, edge, "entity-name")
				}
			}
		}
	}
	if len(configs) == 0 || len(rules) == 0 {
		return st
	}
	st.rules, st.configs = len(rules), len(configs)
	for _, c := range configs {
		for _, r := range rules {
			for _, h := range routeAlert(c, r) {
				to, ok := c.receivers[h.receiver]
				if !ok {
					continue
				}
				edge := EdgeRoutesTo
				if h.state == matchMay {
					edge = EdgeMayRouteTo
					st.may++
				}
				add(r.node, to, edge, "alert-route")
			}
		}
	}
	return st
}

// amConfigs reads every Alertmanager configuration's route tree from the
// doc set: the routes (named by path), their facts (receiver, continue) and
// matcher sets, and the receivers beside them.
func amConfigs(ix *obsIndex, files []DocFile) []amConfig {
	var out []amConfig
	for _, f := range files {
		trees := map[string]map[string]*amRoute{} // prefix -> route path -> route
		recv := map[string]map[string]obsNode{}
		for _, n := range f.Result.Nodes {
			switch n.Kind {
			case "AlertmanagerRoute":
				m := routeNameRe.FindStringSubmatch(n.Name)
				if m == nil {
					continue
				}
				if trees[m[1]] == nil {
					trees[m[1]] = map[string]*amRoute{}
				}
				trees[m[1]]["route"+m[2]] = &amRoute{node: ix.byFileName[[2]string{f.Path, n.Name}]}
			case "AlertmanagerReceiver":
				prefix, name := "", n.Name
				if i := strings.LastIndex(n.Name, "/"); i >= 0 {
					prefix, name = n.Name[:i+1], n.Name[i+1:]
				}
				if recv[prefix] == nil {
					recv[prefix] = map[string]obsNode{}
				}
				recv[prefix][prefix+name] = ix.byFileName[[2]string{f.Path, n.Name}]
			}
		}
		if len(trees) == 0 {
			continue
		}
		routeOf := func(name string) *amRoute {
			m := routeNameRe.FindStringSubmatch(name)
			if m == nil {
				return nil
			}
			return trees[m[1]]["route"+m[2]]
		}
		for _, fact := range f.Result.Facts {
			r := routeOf(fact.Entity)
			if r == nil {
				continue
			}
			switch fact.Key {
			case "receiver":
				r.receiver = fact.Value
			case "continue":
				r.cont = fact.Value == "true"
			}
		}
		for _, ms := range f.Result.Matchers {
			if ms.Edge != "match" {
				continue
			}
			if r := routeOf(ms.Entity); r != nil {
				for _, m := range ms.Matchers {
					r.matchers = append(r.matchers, newAMMatcher(m))
				}
				r.unparsed = r.unparsed || ms.Unparsed > 0
			}
		}
		prefixes := make([]string, 0, len(trees))
		for p := range trees {
			prefixes = append(prefixes, p)
		}
		sort.Strings(prefixes)
		for _, prefix := range prefixes {
			tree := trees[prefix]
			root := tree["route"]
			if root == nil {
				continue
			}
			// parents before children: a path's parent is the path without
			// its last step, a child's place its index
			paths := make([]string, 0, len(tree))
			for p := range tree {
				paths = append(paths, p)
			}
			sort.Slice(paths, func(i, j int) bool { return routeLess(paths[i], paths[j]) })
			for _, p := range paths {
				if p == "route" {
					continue
				}
				i := strings.LastIndex(p, ".routes[")
				if parent := tree[p[:i]]; parent != nil {
					parent.children = append(parent.children, tree[p])
				}
			}
			// receiver names are qualified by the prefix, as the routes are
			qualify(root, prefix)
			out = append(out, amConfig{file: f.Path, root: root, receivers: recv[prefix]})
		}
	}
	return out
}

// qualify prefixes every route's own receiver with the embedding prefix, so
// it names the receiver entity of the same document.
func qualify(r *amRoute, prefix string) {
	if r.receiver != "" {
		r.receiver = prefix + r.receiver
	}
	for _, c := range r.children {
		qualify(c, prefix)
	}
}

// routeLess orders route paths by their index sequence (a parent before its
// children, siblings by index).
func routeLess(a, b string) bool {
	ia, ib := routeIndexes(a), routeIndexes(b)
	for k := 0; k < len(ia) && k < len(ib); k++ {
		if ia[k] != ib[k] {
			return ia[k] < ib[k]
		}
	}
	return len(ia) < len(ib)
}

func routeIndexes(p string) []int {
	var out []int
	for _, m := range routeStepRe.FindAllStringSubmatch(p, -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	return out
}

// amRules reads every Prometheus alerting rule with its static labels: the
// rule's own and its group's, a templated value ({{ }}) left out (decided at
// evaluation), and its name as alertname.
func amRules(ix *obsIndex, files []DocFile) []amRule {
	var out []amRule
	for _, f := range files {
		var groups []obsNode
		var rules []obsNode
		for _, n := range f.Result.Nodes {
			switch n.Kind {
			case "PrometheusRuleGroup":
				groups = append(groups, obsNode{file: f.Path, name: n.Name, kind: string(n.Kind), key: n.Key(), line: n.Line, end: n.EndLine})
			case "PrometheusAlertingRule":
				rules = append(rules, obsNode{file: f.Path, name: n.Name, kind: string(n.Kind), key: n.Key(), line: n.Line, end: n.EndLine})
			}
		}
		if len(rules) == 0 {
			continue
		}
		type labelAt struct {
			entity, key, value string
			line               int
		}
		var facts []labelAt
		for _, fact := range f.Result.Facts {
			if k, ok := strings.CutPrefix(fact.Key, "labels."); ok {
				facts = append(facts, labelAt{fact.Entity, k, fact.Value, fact.Line})
			}
		}
		for _, r := range rules {
			labels := map[string]string{}
			templated := map[string]bool{}
			set := func(k, v string) {
				if strings.Contains(v, "{{") {
					templated[k] = true
					delete(labels, k)
					return
				}
				if !templated[k] {
					labels[k] = v
				}
			}
			for _, g := range groups {
				if r.line < g.line || r.line > g.end {
					continue
				}
				for _, fa := range facts {
					if fa.entity == g.name && fa.line >= g.line && fa.line < r.line {
						set(fa.key, fa.value)
					}
				}
			}
			for _, fa := range facts {
				if fa.entity == r.name && fa.line >= r.line && fa.line <= r.end {
					set(fa.key, fa.value)
				}
			}
			labels["alertname"] = r.name
			out = append(out, amRule{node: r, labels: labels})
		}
	}
	return out
}

// matchState is a three-valued match: decided no, undecided, decided yes.
type matchState int

const (
	matchNo matchState = iota
	matchMay
	matchYes
)

func minState(a, b matchState) matchState {
	if a < b {
		return a
	}
	return b
}

func maxState(a, b matchState) matchState {
	if a > b {
		return a
	}
	return b
}

// routeHit is a receiver an alert reaches, and how certainly.
type routeHit struct {
	receiver string
	state    matchState
}

// routeAlert routes one alerting rule through one configuration by
// Alertmanager's own semantics (dispatch.Route.Match): a route matches when
// every matcher holds; its children are tried in order and the first that
// matches ends the search unless it sets continue; a route none of whose
// children matched is itself a match, with its receiver or the nearest
// ancestor's. A matcher on a static label is decided (=, !=, and =~, !~ as
// anchored RE2); one on any other label is undecided, since the alert's
// runtime labels (its series', the evaluating Prometheus' external labels)
// are unknown: an undecided match makes every receiver it can reach a
// may, never a certainty. Each receiver is returned once, at its strongest.
func routeAlert(c amConfig, r amRule) []routeHit {
	best := map[string]matchState{}
	var walk func(rt *amRoute, reach matchState, inherited string) matchState
	walk = func(rt *amRoute, reach matchState, inherited string) matchState {
		reach = minState(reach, rt.state(r.labels))
		if reach == matchNo {
			return matchNo
		}
		recv := inherited
		if rt.receiver != "" {
			recv = rt.receiver
		}
		remaining, childMatched := reach, matchNo
		for _, ch := range rt.children {
			if remaining == matchNo {
				break
			}
			cs := walk(ch, remaining, recv)
			if cs == matchNo {
				continue
			}
			childMatched = maxState(childMatched, cs)
			if !ch.cont {
				if cs == matchYes {
					remaining = matchNo
				} else {
					remaining = minState(remaining, matchMay)
				}
			}
		}
		// the route is itself a match when no child matched
		self := reach
		switch childMatched {
		case matchYes:
			self = matchNo
		case matchMay:
			self = minState(reach, matchMay)
		}
		if self != matchNo && recv != "" {
			best[recv] = maxState(best[recv], self)
		}
		return reach
	}
	walk(c.root, matchYes, "")
	out := make([]routeHit, 0, len(best))
	for recv, s := range best {
		out = append(out, routeHit{recv, s})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].receiver < out[j].receiver })
	return out
}

// state is the route's own match under a static label set.
func (rt *amRoute) state(labels map[string]string) matchState {
	if rt.unparsed {
		return matchMay
	}
	s := matchYes
	for _, m := range rt.matchers {
		v, ok := labels[m.Name]
		if !ok {
			s = minState(s, matchMay)
			continue
		}
		hold, decided := matcherHolds(m, v)
		switch {
		case !decided:
			s = minState(s, matchMay)
		case !hold:
			return matchNo
		}
	}
	return s
}

// matcherHolds tests a matcher against a label's value; decided is false
// for a regular expression that does not compile (Alertmanager refuses the
// configuration; nothing is guessed).
func matcherHolds(m amMatcher, v string) (hold, decided bool) {
	switch m.Op {
	case promql.OpEqual:
		return v == m.Value, true
	case promql.OpNotEqual:
		return v != m.Value, true
	}
	if m.re == nil {
		return false, false
	}
	if m.Op == promql.OpRegex {
		return m.re.MatchString(v), true
	}
	return !m.re.MatchString(v), true
}

// dedupeXrefs sorts and deduplicates resolved references.
func dedupeXrefs(xs []DocXref) []DocXref {
	sort.Slice(xs, func(i, j int) bool {
		a, b := xs[i], xs[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.ToFile, b.ToFile}, {a.To, b.To}, {a.Edge, b.Edge}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
	out := xs[:0]
	for i, x := range xs {
		if i > 0 && x == xs[i-1] {
			continue
		}
		out = append(out, x)
	}
	return out
}

// ── ingest ──────────────────────────────────────────────────────────────────

// obsXrefs is the doc-xref pass's hook for the observability joins
// (docxref.go's one append line): the pass's doc files, the documents
// Kubernetes objects embed (k8sembed.go: a ConfigMap's dashboard, a
// GrafanaDashboard's spec.json, a Secret's alertmanager.yaml), and the
// compose bind mounts a dashboard provider's container path reads through,
// resolved by ResolveObsXrefs. A Kubernetes manifest is no doc file, so it is
// read once more here, only when the pass already runs.
func (p *Pipeline) obsXrefs(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, files []DocFile) []DocXref {
	have := make(map[string]bool, len(files))
	compose := map[string]bool{}
	for _, f := range files {
		have[f.Path] = true
		compose[f.Path] = isComposeResult(f.Result)
	}
	all := append([]DocFile(nil), files...)
	var mounts []BindMount
	fetch := func(r core.SourceRef) []byte {
		content, err := p.conn.Fetch(ctx, r)
		if err != nil || len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(r.Path, content).excluded != "" {
			return nil
		}
		return content
	}
	for _, r := range refs {
		ext := strings.ToLower(path.Ext(r.Path))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		if !have[r.Path] {
			if content := fetch(r); content != nil && k8sEmbedCandidate(content) {
				if res, ok := k8sEmbeddedResult(svc.ID, r.Path, content); ok {
					all = append(all, DocFile{Path: r.Path, Result: res})
				}
			}
			continue
		}
		if compose[r.Path] {
			if content := fetch(r); content != nil {
				mounts = append(mounts, composeBindMounts(r.Path, content)...)
			}
		}
	}
	return ResolveObsXrefs(all, mounts).Refs
}

// isComposeResult reports whether a doc file is a Compose file (its root is
// a ComposeFile).
func isComposeResult(res extractor.DocResult) bool {
	for _, n := range res.Nodes {
		if n.Kind == "ComposeFile" {
			return true
		}
	}
	return false
}

// composeBindMounts reads a Compose file's bind mounts: each service's
// volumes, the short form `./src:/target[:mode]` and the long form
// `{type: bind, source, target}`, whose source is a path relative to the
// file (a named volume, an absolute host path and an interpolated source,
// `${X}`, mount nothing of the repository).
func composeBindMounts(relPath string, content []byte) []BindMount {
	var out []BindMount
	dir := path.Dir(relPath)
	for _, doc := range yamlDocs(content) {
		services := ymap(doc, "services")
		if services == nil || services.Kind != yaml.MappingNode {
			continue
		}
		for i := 1; i < len(services.Content); i += 2 {
			vols := ymap(services.Content[i], "volumes")
			if vols == nil || vols.Kind != yaml.SequenceNode {
				continue
			}
			for _, v := range vols.Content {
				var src, dst string
				switch v.Kind {
				case yaml.ScalarNode:
					parts := strings.Split(v.Value, ":")
					if len(parts) < 2 {
						continue
					}
					src, dst = parts[0], parts[1]
				case yaml.MappingNode:
					if t := yscalar(ymap(v, "type")); t != "" && t != "bind" {
						continue
					}
					src, dst = yscalar(ymap(v, "source")), yscalar(ymap(v, "target"))
				}
				if dst == "" || !strings.HasPrefix(dst, "/") || strings.Contains(src, "$") || !(src == "." || strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../")) {
					continue
				}
				s := path.Clean(path.Join(dir, src))
				if s == ".." || strings.HasPrefix(s, "../") {
					continue
				}
				out = append(out, BindMount{Source: s, Target: path.Clean(dst)})
			}
		}
	}
	return out
}

// ObsXrefsForBench is ResolveObsXrefs over the files ingest's doc-xref pass
// and its hook read (the harness's indexbench lang-obs): every doc-engine
// file of refs through ingest's dispatch, the documents Kubernetes objects
// embed, and the compose bind mounts.
func (p *Pipeline) ObsXrefsForBench(service string, refs []core.SourceRef, read func(rel string) ([]byte, error)) ([]DocFile, ObsResolution) {
	idx := buildSiblingIndex(refs)
	var files []DocFile
	var mounts []BindMount
	for _, r := range refs {
		if !isDocCandidate(r.Path) {
			continue
		}
		content, err := read(r.Path)
		if err != nil || len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(r.Path, content).excluded != "" {
			continue
		}
		e, _, ok, _ := p.resolveExtractor(r.Path, content, idx)
		if doc, isDoc := e.(extractor.DocResulter); ok && isDoc {
			if res, err := doc.ExtractDoc(service, r.Path, content); err == nil {
				files = append(files, DocFile{Path: r.Path, Result: res})
				if isComposeResult(res) {
					mounts = append(mounts, composeBindMounts(r.Path, content)...)
				}
			}
			continue
		}
		ext := strings.ToLower(path.Ext(r.Path))
		if (ext == ".yaml" || ext == ".yml") && k8sEmbedCandidate(content) {
			if res, ok := k8sEmbeddedResult(service, r.Path, content); ok {
				files = append(files, DocFile{Path: r.Path, Result: res})
			}
		}
	}
	return files, ResolveObsXrefs(files, mounts)
}
