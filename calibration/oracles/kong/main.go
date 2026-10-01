// Command kong-oracle is the language-breadth harness's oracle for the kong
// doc-engine dialect (design §1.3, LB-Q-kong). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10):
//
//	go run . -subject <checkout> [-deck <deck>] [-scratch <dir>] [-refs-out <file>] [-files-out <file>] > definitions.json
//
// The tool is decK (Kong's own declarative-configuration tool, Apache-2.0),
// the release binary, run offline on every file of the subject that declares
// `_format_version`:
//
//   - `deck file validate <file>` is the acceptance: a file decK rejects is not
//     a Kong configuration it can read, contributes nothing and is left out of
//     -files-out, counted on stderr with decK's first error line.
//   - `deck file render --format json <file>` is the entity truth: decK's
//     normalised state, with defaults filled in and every foreign key resolved
//     into nesting (a route under the service it names, a plugin under the
//     service or route it is scoped to). An entity decK's state holds is a
//     definition; one it does not hold (an unnamed route, a consumer, which the
//     dialect does not model) is not.
//
// stdout (tier A) carries the definitions: a service, a route, an upstream (a
// `resource`), a target (an `element`) and a plugin (a `policy`) per entity of
// the rendered state that has a name, at the line the file writes it. decK
// prints no source lines, so each rendered entity is paired with the written
// one of the same kind, name and effective scope (a plugin's scope is the
// service or route it names, or the entity it is nested under) through
// gopkg.in/yaml.v3, the library the dialect's engine reads lines with too: a
// line is a decoder detail, never an entity decision, and a rendered entity no
// written one pairs with drops the whole file (counted as unpairable).
//
// -refs-out (tier C) carries what decK's state cannot say by itself, whether a
// reference was WRITTEN: references (a route's `service`, a plugin's `service`
// or `route`, a service's `host` naming an upstream, each confirmed by decK's
// own nesting or by the rendered upstream), cross_references (a reference
// whose name is declared in no entity of its own file and in exactly one entity
// of another file of the same directory, the files decK merges; entity
// granularity), and, for the route_ref layer (bench-only, lang-oracle ignores
// them), route_references (a service host or url that is not an upstream, and
// an upstream target, joined to a docker-compose service or a Kubernetes
// Service of the subject) and route_unresolved (the same addresses that join
// nothing, with the reason).
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
	"sort"
	"strings"
	"time"

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

type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	To     string `json:"to"`
	ToPath string `json:"to_path"`
}

type output struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []pair `json:"references,omitempty"`
	CrossReferences []xref `json:"cross_references,omitempty"`
	// RouteReferences and RouteUnresolved are the route_ref layer's truth
	// (bench-only). lang-oracle's tool-oracle decode refuses unknown fields,
	// so they are written to -routes-out, never into a file it reads.
	RouteReferences []routeRef        `json:"route_references,omitempty"`
	RouteUnresolved []routeUnresolved `json:"route_unresolved,omitempty"`
}

// routesFile is -routes-out's content.
type routesFile struct {
	Tool            string            `json:"tool"`
	Version         string            `json:"version"`
	RouteReferences []routeRef        `json:"route_references"`
	RouteUnresolved []routeUnresolved `json:"route_unresolved"`
}

// entity is one named thing of a Kong configuration, written or rendered.
type entity struct {
	kind, name, scope string
	line, end         int
	node              *yaml.Node
}

// canonical kinds the dialect manifest's oracle_kinds.tool names.
var oracleKind = map[string]string{
	"service": "service", "route": "route", "upstream": "resource", "target": "element", "plugin": "policy",
}

func scalarOf(n *yaml.Node, key string) string {
	if n == nil || n.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key && n.Content[i+1].Kind == yaml.ScalarNode {
			return n.Content[i+1].Value
		}
	}
	return ""
}

func child(n *yaml.Node, key string) *yaml.Node {
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

// refName reads a foreign key written either as a name or as {name: ...}/{id: ...}.
func refName(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	return scalarOf(n, "name")
}

func lastLine(n *yaml.Node) int {
	m := n.Line
	for _, c := range n.Content {
		if l := lastLine(c); l > m {
			m = l
		}
	}
	return m
}

func items(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	var out []*yaml.Node
	for _, c := range n.Content {
		if c.Kind == yaml.MappingNode {
			out = append(out, c)
		}
	}
	return out
}

// walk collects the entities under n: kind says what the mapping n is.
// scope is the entity n is nested under ("" at the root). explicit reports an
// entity's FK (kind, name) when it is written on the entity itself.
func walkWritten(root *yaml.Node) (ents []entity, refs []fk) {
	var plugins func(n *yaml.Node, scope string)
	plugins = func(n *yaml.Node, scope string) {
		for _, p := range items(child(n, "plugins")) {
			name := scalarOf(p, "name")
			if name == "" {
				continue
			}
			eff := scope
			if s := refName(child(p, "service")); s != "" {
				eff = "service:" + s
				refs = append(refs, fk{"plugin", name, eff, "service", s, p.Line})
			}
			if r := refName(child(p, "route")); r != "" {
				eff = "route:" + r
				refs = append(refs, fk{"plugin", name, eff, "route", r, p.Line})
			}
			ents = append(ents, entity{"plugin", name, eff, p.Line, lastLine(p), p})
		}
	}
	route := func(r *yaml.Node, scope string) {
		name := scalarOf(r, "name")
		if name != "" {
			eff := scope
			if s := refName(child(r, "service")); s != "" {
				eff = "service:" + s
				refs = append(refs, fk{"route", name, eff, "service", s, r.Line})
			}
			ents = append(ents, entity{"route", name, eff, r.Line, lastLine(r), r})
			plugins(r, "route:"+name)
		}
	}
	for _, s := range items(child(root, "services")) {
		name := scalarOf(s, "name")
		if name == "" {
			continue
		}
		ents = append(ents, entity{"service", name, "", s.Line, lastLine(s), s})
		if h := scalarOf(s, "host"); h != "" {
			refs = append(refs, fk{"service", name, "", "upstream", h, s.Line})
		}
		for _, r := range items(child(s, "routes")) {
			route(r, "service:"+name)
		}
		plugins(s, "service:"+name)
	}
	for _, r := range items(child(root, "routes")) {
		route(r, "")
	}
	for _, u := range items(child(root, "upstreams")) {
		name := scalarOf(u, "name")
		if name == "" {
			continue
		}
		ents = append(ents, entity{"upstream", name, "", u.Line, lastLine(u), u})
		for _, t := range items(child(u, "targets")) {
			if tn := scalarOf(t, "target"); tn != "" {
				ents = append(ents, entity{"target", tn, "upstream:" + name, t.Line, lastLine(t), t})
			}
		}
	}
	plugins(root, "")
	for _, c := range items(child(root, "consumers")) {
		plugins(c, "consumer:"+scalarOf(c, "username"))
	}
	for _, g := range items(child(root, "consumer_groups")) {
		plugins(g, "consumer_group:"+scalarOf(g, "name"))
	}
	return ents, refs
}

// fk is one written reference: the referring entity, the kind of target and its name.
type fk struct {
	fromKind, from, fromScope, toKind, to string
	line                                  int
}

// walkRendered collects decK's rendered entities: foreign keys are nesting.
func walkRendered(r map[string]any) []entity {
	var ents []entity
	list := func(v any) []map[string]any {
		var out []map[string]any
		if a, ok := v.([]any); ok {
			for _, x := range a {
				if m, ok := x.(map[string]any); ok {
					out = append(out, m)
				}
			}
		}
		return out
	}
	str := func(m map[string]any, k string) string { s, _ := m[k].(string); return s }
	var plugins func(m map[string]any, scope string)
	plugins = func(m map[string]any, scope string) {
		for _, p := range list(m["plugins"]) {
			if n := str(p, "name"); n != "" {
				ents = append(ents, entity{kind: "plugin", name: n, scope: scope})
			}
		}
	}
	route := func(rt map[string]any, scope string) {
		if n := str(rt, "name"); n != "" {
			ents = append(ents, entity{kind: "route", name: n, scope: scope})
			plugins(rt, "route:"+n)
		}
	}
	for _, s := range list(r["services"]) {
		n := str(s, "name")
		if n == "" {
			continue
		}
		ents = append(ents, entity{kind: "service", name: n})
		for _, rt := range list(s["routes"]) {
			route(rt, "service:"+n)
		}
		plugins(s, "service:"+n)
	}
	for _, rt := range list(r["routes"]) {
		route(rt, "")
	}
	for _, u := range list(r["upstreams"]) {
		n := str(u, "name")
		if n == "" {
			continue
		}
		ents = append(ents, entity{kind: "upstream", name: n})
		for _, t := range list(u["targets"]) {
			if tn := str(t, "target"); tn != "" {
				ents = append(ents, entity{kind: "target", name: tn, scope: "upstream:" + n})
			}
		}
	}
	plugins(r, "")
	for _, c := range list(r["consumers"]) {
		plugins(c, "consumer:"+str(c, "username"))
	}
	for _, g := range list(r["consumer_groups"]) {
		plugins(g, "consumer_group:"+str(g, "name"))
	}
	return ents
}

func key(e entity) string { return e.kind + "\x00" + e.name + "\x00" + e.scope }

type runner struct {
	deck    string
	timeout time.Duration
	env     []string
}

func (r runner) run(ctx context.Context, dir string, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.deck, args...)
	cmd.Dir = dir
	cmd.Env = r.env
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.Bytes(), se.Bytes(), err
}

func firstErr(b []byte) string {
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "Error: ") {
			return l
		}
	}
	return strings.TrimSpace(string(b))
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	deck := flag.String("deck", "deck", "the decK binary")
	scratch := flag.String("scratch", "", "scratch directory outside the subject (default: a temp dir)")
	refsOut := flag.String("refs-out", "", "write the tier C references, cross_references and route targets here")
	routesOut := flag.String("routes-out", "", "write the route_ref layer's truth (route_references, route_unresolved) here")
	filesOut := flag.String("files-out", "", "write the files decK accepted, one per line, for lang-oracle -only")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: kong-oracle -subject <checkout> [-deck <deck>] [-scratch <dir>] [-refs-out <file>] [-files-out <file>]")
		os.Exit(2)
	}
	if abs, err := filepath.Abs(*subject); err == nil {
		*subject = abs
	}
	ctx := context.Background()
	work := *scratch
	if work == "" {
		w, err := os.MkdirTemp("", "kong-oracle-")
		if err != nil {
			fatal(err)
		}
		defer os.RemoveAll(w)
		work = w
	}
	_ = os.MkdirAll(work, 0o755)
	r := runner{deck: *deck, timeout: 2 * time.Minute, env: []string{"HOME=" + work, "PATH=" + os.Getenv("PATH"), "DECK_ANALYTICS=false"}}
	vso, _, verr := r.run(ctx, work, "version")
	if verr != nil {
		fatal(fmt.Errorf("deck version: %v", verr))
	}
	version := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(vso)), "decK"))

	var files []string
	if err := filepath.WalkDir(*subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // a symlink or device: not a file of the subject
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".yaml", ".yml", ".json":
			files = append(files, p)
		}
		return nil
	}); err != nil {
		fatal(err)
	}
	sort.Strings(files)

	var truths []*fileTruth
	rejected := map[string]int{}
	var dbless []*fileTruth // decodable declarative files decK rejects: route targets at tier C only
	var accepted []string
	unpairable := 0
	for _, f := range files {
		b, err := readFile(f)
		if err != nil {
			fatal(err)
		}
		if !bytes.Contains(b, []byte("_format_version")) {
			continue
		}
		rel, _ := filepath.Rel(*subject, f)
		rel = filepath.ToSlash(rel)
		var doc yaml.Node
		if err := yaml.Unmarshal(b, &doc); err != nil || len(doc.Content) == 0 || child(doc.Content[0], "_format_version") == nil {
			rejected["not decodable as a Kong file: "+firstLine(err)]++
			continue
		}
		if _, se, err := r.run(ctx, filepath.Dir(f), "file", "validate", f); err != nil {
			rejected["decK rejects: "+reasonKey(firstErr(se))]++
			dbless = append(dbless, &fileTruth{rel: rel, hosts: hostRefs(doc.Content[0])})
			continue
		}
		so, se, err := r.run(ctx, filepath.Dir(f), "file", "render", "--format", "json", f)
		if err != nil {
			rejected["decK render fails: "+reasonKey(firstErr(se))]++
			continue
		}
		var rendered map[string]any
		if json.Unmarshal(so, &rendered) != nil {
			rejected["decK render is not JSON"]++
			continue
		}
		written, refs := walkWritten(doc.Content[0])
		rend := walkRendered(rendered)
		wk := map[string][]entity{}
		for _, e := range written {
			wk[key(e)] = append(wk[key(e)], e)
		}
		rk := map[string]int{}
		for _, e := range rend {
			rk[key(e)]++
		}
		paired := true
		var ents []entity
		for k, n := range rk {
			if len(wk[k]) != n {
				paired = false
				break
			}
		}
		if paired {
			for k, es := range wk {
				if rk[k] != len(es) {
					paired = false
					break
				}
			}
		}
		if !paired {
			unpairable++
			continue
		}
		for _, e := range written {
			ents = append(ents, e)
		}
		accepted = append(accepted, rel)
		truths = append(truths, &fileTruth{rel: rel, ents: ents, refs: refs, hosts: hostRefs(doc.Content[0])})
		_ = rend
	}

	dep := scanDeployed(*subject)
	out := output{Tool: "deck", Version: version, Tier: "A", Definitions: []def{}}
	refsDoc := output{Tool: "deck", Version: version, Tier: "C"}
	// names declared per file, per directory, for the cross-file uniqueness.
	declared := map[string]map[string]bool{}
	for _, t := range truths {
		declared[t.rel] = map[string]bool{}
		for _, e := range t.ents {
			declared[t.rel][e.name] = true
		}
	}
	for _, t := range truths {
		for _, e := range t.ents {
			out.Definitions = append(out.Definitions, def{Path: t.rel, Name: e.name, Line: e.line, End: e.end, Kind: oracleKind[e.kind]})
		}
		own := map[string]map[string]bool{}
		for _, e := range t.ents {
			if own[e.kind] == nil {
				own[e.kind] = map[string]bool{}
			}
			own[e.kind][e.name] = true
		}
		seen := map[string]bool{}
		for _, ref := range t.refs {
			if ref.toKind == "upstream" && !own["upstream"][ref.to] {
				// not an upstream of this file: an address, or another file's upstream
				if other, ok := uniqueElsewhere(declared, t.rel, ref.to); ok {
					refsDoc.CrossReferences = append(refsDoc.CrossReferences, xref{Path: t.rel, From: ref.from, To: ref.to, ToPath: other})
				}
				continue
			}
			if ref.toKind != "upstream" && !own[ref.toKind][ref.to] {
				if other, ok := uniqueElsewhere(declared, t.rel, ref.to); ok {
					refsDoc.CrossReferences = append(refsDoc.CrossReferences, xref{Path: t.rel, From: ref.from, To: ref.to, ToPath: other})
				}
				continue
			}
			id := ref.from + "\x00" + ref.to
			if !seen[id] {
				seen[id] = true
				refsDoc.References = append(refsDoc.References, pair{Path: t.rel, From: ref.from, To: ref.to})
			}
		}
		for _, h := range t.hosts {
			if h.kind == "service" && own["upstream"][h.raw] {
				continue // an upstream's name is an L2 reference, never a route target
			}
			if tp, to, reason := dep.classify(h.raw); reason == "" {
				refsDoc.RouteReferences = append(refsDoc.RouteReferences, routeRef{Path: t.rel, From: h.from, ToPath: tp, To: to, Raw: h.raw})
			} else {
				refsDoc.RouteUnresolved = append(refsDoc.RouteUnresolved, routeUnresolved{Path: t.rel, From: h.from, Raw: h.raw, Reason: reason})
			}
		}
	}
	// Route targets of the files decK rejects (the DB-less dialect of the format:
	// an acl, a credential or a comment decK's stricter schema refuses): written
	// text joined to the subject's services, tier C.
	for _, t := range dbless {
		ups := map[string]bool{}
		for _, h := range t.hosts {
			if h.kind == "upstream" {
				ups[h.from] = true
			}
		}
		for _, h := range t.hosts {
			if h.kind == "service" && ups[h.raw] {
				continue
			}
			if tp, to, reason := dep.classify(h.raw); reason == "" {
				refsDoc.RouteReferences = append(refsDoc.RouteReferences, routeRef{Path: t.rel, From: h.from, ToPath: tp, To: to, Raw: h.raw, Tier: "C"})
			} else {
				refsDoc.RouteUnresolved = append(refsDoc.RouteUnresolved, routeUnresolved{Path: t.rel, From: h.from, Raw: h.raw, Reason: reason, Tier: "C"})
			}
		}
	}
	sort.Slice(out.Definitions, func(i, j int) bool {
		a, b := out.Definitions[i], out.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		fatal(err)
	}
	if *refsOut != "" {
		rf := refsDoc
		rf.RouteReferences, rf.RouteUnresolved = nil, nil
		if rf.References == nil {
			rf.References = []pair{}
		}
		b, _ := json.MarshalIndent(rf, "", " ")
		if err := writeFile(*refsOut, append(b, '\n')); err != nil {
			fatal(err)
		}
	}
	if *routesOut != "" {
		rt := routesFile{Tool: refsDoc.Tool, Version: refsDoc.Version, RouteReferences: refsDoc.RouteReferences, RouteUnresolved: refsDoc.RouteUnresolved}
		if rt.RouteReferences == nil {
			rt.RouteReferences = []routeRef{}
		}
		if rt.RouteUnresolved == nil {
			rt.RouteUnresolved = []routeUnresolved{}
		}
		b, _ := json.MarshalIndent(rt, "", " ")
		if err := writeFile(*routesOut, append(b, '\n')); err != nil {
			fatal(err)
		}
	}
	if *filesOut != "" {
		sort.Strings(accepted)
		if err := writeFile(*filesOut, []byte(strings.Join(accepted, "\n")+"\n")); err != nil {
			fatal(err)
		}
	}
	fmt.Fprintf(os.Stderr, "kong-oracle: %d files accepted, %d definitions, %d references, %d cross_references, %d route_references, %d route_unresolved, %d unpairable\n",
		len(accepted), len(out.Definitions), len(refsDoc.References), len(refsDoc.CrossReferences), len(refsDoc.RouteReferences), len(refsDoc.RouteUnresolved), unpairable)
	var reasons []string
	for k, n := range rejected {
		reasons = append(reasons, fmt.Sprintf("  %5d  %s", n, k))
	}
	sort.Strings(reasons)
	for _, l := range reasons {
		fmt.Fprintln(os.Stderr, l)
	}
}

// fileTruth is one accepted file's truth.
type fileTruth struct {
	rel   string
	ents  []entity
	refs  []fk
	hosts []hostRefEnt
}

type hostRefEnt struct {
	kind, from, raw string
}

// hostRefs lists the addresses a file's services and upstreams name, as written.
func hostRefs(root *yaml.Node) []hostRefEnt {
	var out []hostRefEnt
	for _, s := range items(child(root, "services")) {
		name := scalarOf(s, "name")
		if name == "" {
			continue
		}
		if h := scalarOf(s, "host"); h != "" {
			out = append(out, hostRefEnt{"service", name, h})
		}
		if u := scalarOf(s, "url"); u != "" {
			out = append(out, hostRefEnt{"service", name, u})
		}
	}
	for _, u := range items(child(root, "upstreams")) {
		name := scalarOf(u, "name")
		if name == "" {
			continue
		}
		for _, t := range items(child(u, "targets")) {
			if tn := scalarOf(t, "target"); tn != "" {
				out = append(out, hostRefEnt{"upstream", name, tn})
			}
		}
	}
	return out
}

// uniqueElsewhere: the one file of from's directory, other than from, that
// declares name (a decK merge unit), when exactly one does.
func uniqueElsewhere(declared map[string]map[string]bool, from, name string) (string, bool) {
	dir := path.Dir(from)
	var hit []string
	for f, names := range declared {
		if f != from && path.Dir(f) == dir && names[name] {
			hit = append(hit, f)
		}
	}
	if len(hit) == 1 {
		return hit[0], true
	}
	return "", false
}

func firstLine(err error) string {
	if err == nil {
		return "empty or no _format_version"
	}
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// reasonKey trims a decK message to the part that repeats across files.
func reasonKey(s string) string {
	if i := strings.Index(s, "err="); i >= 0 {
		s = s[i+4:]
	}
	if len(s) > 110 {
		s = s[:110]
	}
	return s
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "kong-oracle:", err)
	os.Exit(1)
}
