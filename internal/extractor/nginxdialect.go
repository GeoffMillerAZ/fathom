package extractor

// nginxdialect.go — the nginx native dialect (LB-Q-nginx, docs/design/
// language-breadth.md §13, engine "native": grammar_source native:pure-go).
// confsyntax.go reads a configuration file with crossplane's token rules
// into a statement tree; this walks it into entities, intra-file references
// and cross-file references (dialects/nginx/PROVENANCE.md says what is and
// is not modelled).
//
// Entities:
//
//   - the file (NginxConfig, named by its base name): every file's root, what
//     an include of it resolves to;
//   - a server block (NginxServer, a service), named by its first
//     server_name, else its first listen address, else "server";
//   - a location (NginxLocation, a route), named by its arguments as written
//     (modifier, URI or @name) and qualified by its server's name;
//   - an upstream (NginxUpstream, a service) and each of its servers
//     (NginxUpstreamServer, a target, qualified by the upstream's name).
//
// Intra-file references (L2): a proxy_pass, grpc_pass, fastcgi_pass,
// uwsgi_pass, scgi_pass or memcached_pass whose host, written without a
// port, names an upstream of the file is a call of the location (in stream,
// the server) to that upstream.
//
// Cross-file references (L3):
//
//   - include: one repo-path candidate per directory from the including
//     file's up to the repository root (nginx resolves a relative path
//     against the configuration prefix, the main nginx.conf's directory,
//     which one file cannot see; the resolver keeps the candidates that are
//     files of the repository). A file named nginx.conf is the prefix: its
//     own directory only. An absolute path under a conventional prefix
//     (/etc/nginx/, /usr/local/nginx/conf/, …) is read relative to it.
//   - a route target (the route contract, ADR-050): a *_pass host that names
//     no upstream of the file, or names one with a port, from the location
//     (or stream server); an upstream's server address, from the upstream.
//     Edge route_ref, strategy route-host, Raw and Target the argument as
//     written; fusion.ResolveRouteRefs resolves it or discloses why not.

import (
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const (
	kindNginxConfig         core.SourceKind = "NginxConfig"
	kindNginxServer         core.SourceKind = "NginxServer"
	kindNginxLocation       core.SourceKind = "NginxLocation"
	kindNginxUpstream       core.SourceKind = "NginxUpstream"
	kindNginxUpstreamServer core.SourceKind = "NginxUpstreamServer"

	// Edge names of the cross-file references the proxy dialects emit.
	edgeIncludes      = "includes"
	edgeRouteRef      = "route_ref"
	strategyRepoPath  = "repo-path"
	strategyRouteHost = "route-host"
)

// nginxPassDirectives are the directives that hand a request to an upstream
// or a host.
var nginxPassDirectives = map[string]bool{
	"proxy_pass": true, "grpc_pass": true, "fastcgi_pass": true,
	"uwsgi_pass": true, "scgi_pass": true, "memcached_pass": true,
}

// nginxDataBlocks hold data, not directives: their children are never
// entities or references (an include inside one still is).
var nginxDataBlocks = map[string]bool{
	"map": true, "geo": true, "types": true, "split_clients": true,
	"charset_map": true, "match": true,
}

// nginxPrefixes are the conventional configuration prefixes an absolute
// include is read against.
var nginxPrefixes = []string{"/etc/nginx/", "/usr/local/nginx/conf/", "/usr/local/etc/nginx/", "/opt/homebrew/etc/nginx/"}

// NginxExtractor implements Extractor and DocResulter for nginx
// configuration files.
type NginxExtractor struct{}

var (
	_ Extractor   = (*NginxExtractor)(nil)
	_ DocResulter = (*NginxExtractor)(nil)
)

// NewNginxExtractor returns the nginx dialect's extractor.
func NewNginxExtractor() *NginxExtractor { return &NginxExtractor{} }

// Extract implements Extractor.
func (x *NginxExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := x.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// ExtractDoc implements DocResulter. Malformed input yields what its
// well-formed statements declare and no error.
func (x *NginxExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	tree, _ := parseNginxTree(src)
	w := &confWalk{service: service, relPath: relPath, pkg: pkgFromPath(relPath)}
	root := path.Base(relPath)
	w.node(root, kindNginxConfig, "", 1, confLastLine(src))
	w.res.Roots = append(w.res.Roots, root)
	n := &nginxWalk{confWalk: w, root: root, upstreams: map[string]bool{}}
	n.collectUpstreams(tree)
	n.walk(tree, nginxScope{})
	return w.finish(), nil
}

// confWalk is the entity and reference collection the three proxy dialects
// share.
type confWalk struct {
	service, relPath, pkg string
	res                   DocResult
}

func (w *confWalk) node(name string, kind core.SourceKind, sig string, line, end int) int {
	w.res.Nodes = append(w.res.Nodes, core.SymbolNode{
		Service: w.service, Package: w.pkg, Name: name, Kind: kind, FilePath: w.relPath,
		Line: line, EndLine: max(end, line), Signature: capString(sig, 200), Tier: core.TierPreScan,
	})
	return len(w.res.Nodes) - 1
}

func (w *confWalk) call(i int, to string) {
	if i < 0 {
		return
	}
	for _, c := range w.res.Nodes[i].Calls {
		if c == to {
			return
		}
	}
	w.res.Nodes[i].Calls = append(w.res.Nodes[i].Calls, to)
}

func (w *confWalk) cross(from, edge, raw, target, strategy string, line int) {
	w.res.CrossFile = append(w.res.CrossFile, DocCrossFileRef{From: from, Edge: edge, Raw: raw, Target: target, Strategy: strategy, Line: line})
}

// route records a route target (the route contract).
func (w *confWalk) route(from, raw string, line int) {
	if raw != "" {
		w.cross(from, edgeRouteRef, raw, raw, strategyRouteHost, line)
	}
}

// finish sorts the result deterministically (by line, then name; cross-file
// references by line, edge, origin and target) and sorts each entity's calls.
func (w *confWalk) finish() DocResult {
	for i := range w.res.Nodes {
		sort.Strings(w.res.Nodes[i].Calls)
	}
	sort.SliceStable(w.res.Nodes, func(i, j int) bool {
		a, b := w.res.Nodes[i], w.res.Nodes[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.SliceStable(w.res.CrossFile, func(i, j int) bool {
		a, b := w.res.CrossFile[i], w.res.CrossFile[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Edge != b.Edge {
			return a.Edge < b.Edge
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.Target < b.Target
	})
	return w.res
}

// qualified joins a qualifier and a name with one space ("" qualifier: the
// name alone).
func qualified(q, name string) string {
	if q == "" {
		return name
	}
	return q + " " + name
}

type nginxWalk struct {
	*confWalk
	root      string
	upstreams map[string]bool
}

// nginxScope is where a statement sits: the nearest server and location
// (their names and entity indexes; -1 when none) and whether it is inside a
// data block.
type nginxScope struct {
	server, location       string
	serverIdx, locationIdx int
	inUpstream             string
	upstreamIdx            int
	data, set              bool
}

func (s nginxScope) init() nginxScope {
	if !s.set {
		s = nginxScope{serverIdx: -1, locationIdx: -1, upstreamIdx: -1, set: true}
	}
	return s
}

// owner is the entity a statement in this scope belongs to: the location,
// else the server, else the upstream; "" and -1 at the file's top.
func (s nginxScope) owner() (string, int) {
	switch {
	case s.locationIdx >= 0:
		return s.location, s.locationIdx
	case s.serverIdx >= 0:
		return s.server, s.serverIdx
	case s.upstreamIdx >= 0:
		return s.inUpstream, s.upstreamIdx
	}
	return "", -1
}

func (n *nginxWalk) collectUpstreams(nodes []*confNode) {
	for _, c := range nodes {
		if c.block && c.name == "upstream" && len(c.args) > 0 {
			n.upstreams[c.args[0].text] = true
		}
		if c.block && !nginxDataBlocks[c.name] {
			n.collectUpstreams(c.children)
		}
	}
}

func (n *nginxWalk) walk(nodes []*confNode, sc nginxScope) {
	sc = sc.init()
	for _, c := range nodes {
		switch {
		case c.name == "include" && len(c.args) > 0:
			from, _ := sc.owner()
			n.include(from, c.args[0].text, c.line)
		case sc.data:
			if c.block {
				n.walk(c.children, sc)
			}
		case c.block && nginxDataBlocks[c.name]:
			inner := sc
			inner.data = true
			n.walk(c.children, inner)
		case c.block && c.name == "server" && sc.upstreamIdx < 0:
			name := nginxServerName(c)
			i := n.node(name, kindNginxServer, nginxServerSig(c), c.line, c.end)
			inner := sc
			inner.server, inner.serverIdx = name, i
			inner.location, inner.locationIdx = "", -1
			n.walk(c.children, inner)
		case c.block && c.name == "location":
			name := qualified(sc.server, c.argsText())
			i := n.node(name, kindNginxLocation, "location "+c.argsText(), c.line, c.end)
			inner := sc
			inner.location, inner.locationIdx = name, i
			n.walk(c.children, inner)
		case c.block && c.name == "upstream" && len(c.args) > 0:
			name := c.args[0].text
			i := n.node(name, kindNginxUpstream, "upstream "+c.argsText(), c.line, c.end)
			inner := sc
			inner.inUpstream, inner.upstreamIdx = name, i
			n.walk(c.children, inner)
		case !c.block && c.name == "server" && sc.upstreamIdx >= 0 && len(c.args) > 0:
			addr := c.args[0].text
			n.node(qualified(sc.inUpstream, addr), kindNginxUpstreamServer, "server "+c.argsText(), c.line, c.end)
			n.route(sc.inUpstream, addr, c.line)
		case !c.block && nginxPassDirectives[c.name] && len(c.args) > 0:
			n.pass(sc, c)
		case c.block:
			n.walk(c.children, sc)
		}
	}
}

// pass records a *_pass directive: a call to an upstream of the file, or a
// route target, from the location (else the server, else the file).
func (n *nginxWalk) pass(sc nginxScope, c *confNode) {
	from, i := sc.owner()
	if i < 0 {
		// A snippet's top-level pass (the file is included into a location):
		// the file's own entity holds it.
		from, i = n.root, 0
	}
	arg := c.args[0].text
	host, port := nginxPassHost(arg)
	if !port && n.upstreams[host] {
		n.call(i, host)
		return
	}
	n.route(from, arg, c.line)
}

// nginxPassHost is a *_pass argument's host and whether a port follows it:
// the scheme and any path are dropped (http://backend:8080/x → backend,
// true); a unix: socket has no host.
func nginxPassHost(arg string) (string, bool) {
	s := arg
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if strings.HasPrefix(s, "unix:") {
		return "", false
	}
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]"); i >= 0 {
			return s[:i+1], strings.HasPrefix(s[i+1:], ":")
		}
		return s, false
	}
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		return s[:i], true
	}
	return s, false
}

// nginxServerName names a server block: its first server_name (an empty
// one skipped), else its first listen address, else "server".
func nginxServerName(c *confNode) string {
	for _, d := range c.children {
		if d.name == "server_name" {
			for _, a := range d.args {
				if a.text != "" {
					return a.text
				}
			}
		}
	}
	for _, d := range c.children {
		if d.name == "listen" && len(d.args) > 0 {
			return d.args[0].text
		}
	}
	return "server"
}

// nginxServerSig is a server's listen and server_name arguments.
func nginxServerSig(c *confNode) string {
	var parts []string
	for _, d := range c.children {
		if d.name == "listen" || d.name == "server_name" {
			parts = append(parts, strings.TrimSpace(d.name+" "+d.argsText()))
		}
	}
	return strings.Join(parts, "; ")
}

// include records an include's repo-path candidates (see the file comment).
func (n *nginxWalk) include(from, raw string, line int) {
	p := raw
	abs := strings.HasPrefix(p, "/")
	if abs {
		mapped := false
		for _, pre := range nginxPrefixes {
			if strings.HasPrefix(p, pre) {
				p, mapped = strings.TrimPrefix(p, pre), true
				break
			}
		}
		if !mapped {
			n.cross(from, edgeIncludes, raw, "", strategyRepoPath, line)
			return
		}
	}
	dir := path.Dir(n.relPath)
	if path.Base(n.relPath) == "nginx.conf" {
		n.cross(from, edgeIncludes, raw, repoJoin(dir, p), strategyRepoPath, line)
		return
	}
	seen := map[string]bool{}
	for {
		t := repoJoin(dir, p)
		if t != "" && !seen[t] {
			seen[t] = true
			n.cross(from, edgeIncludes, raw, t, strategyRepoPath, line)
		}
		if dir == "." || dir == "/" || dir == "" {
			return
		}
		dir = path.Dir(dir)
	}
}

// repoJoin joins a repository directory and a relative path; "" when the
// result climbs out of the repository.
func repoJoin(dir, rel string) string {
	t := path.Clean(path.Join(dir, rel))
	if t == ".." || strings.HasPrefix(t, "../") || strings.HasPrefix(t, "/") {
		return ""
	}
	return t
}
