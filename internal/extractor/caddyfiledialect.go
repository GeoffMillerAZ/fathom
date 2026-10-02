package extractor

// caddyfiledialect.go — the Caddyfile native dialect (LB-Q-caddyfile,
// docs/design/language-breadth.md §13, engine "native": grammar_source
// native:pure-go). confsyntax.go reads a Caddyfile with Caddy's lexical
// rules (a brace delimits a block only as a word of its own, so a {host}
// placeholder is an ordinary word) into a statement tree; this walks it
// (dialects/caddyfile/PROVENANCE.md says what is and is not modelled).
//
// Entities:
//
//   - the file (CaddyfileConfig, named by its base name): its root, what an
//     import of it resolves to;
//   - a site block (CaddySite, a service), named by its first address; a
//     file whose first statement opens no block is one site;
//   - a snippet, (name) { … } (CaddySnippet, a mixin), and a named route,
//     &(name) { … } (CaddyNamedRoute, a route), named by the name;
//   - a named matcher, @name (CaddyMatcher, a selector), qualified by its
//     site (or snippet);
//   - a handle, handle_path, handle_errors or route block and a
//     reverse_proxy, php_fastcgi or forward_auth directive (CaddyRoute, a
//     route), named by the directive and its matcher token (*, /path or
//     @name) when it has one, qualified by its site.
//
// Intra-file references (L2): an import of a snippet the file declares, and
// an invoke of a named route it declares, are calls of the enclosing entity.
//
// Cross-file references (L3): an import that names no snippet is a file (or
// a glob), repo-path from the importing file's directory (an absolute path
// under /etc/caddy/ read against that directory); each upstream of a
// reverse_proxy, php_fastcgi or forward_auth (its arguments after the
// matcher and its to subdirective's) is a route target from the route
// entity (the route contract, ADR-050: edge route_ref, strategy route-host,
// Raw and Target the upstream as written).

import (
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const (
	kindCaddyfileConfig core.SourceKind = "CaddyfileConfig"
	kindCaddySite       core.SourceKind = "CaddySite"
	kindCaddySnippet    core.SourceKind = "CaddySnippet"
	kindCaddyNamedRoute core.SourceKind = "CaddyNamedRoute"
	kindCaddyMatcher    core.SourceKind = "CaddyMatcher"
	kindCaddyRoute      core.SourceKind = "CaddyRoute"
)

// caddyRouteBlocks are the directives whose block is a route.
var caddyRouteBlocks = map[string]bool{"handle": true, "handle_path": true, "handle_errors": true, "route": true}

// caddyProxies are the directives that hand a request to upstreams.
var caddyProxies = map[string]bool{"reverse_proxy": true, "php_fastcgi": true, "forward_auth": true}

// CaddyfileExtractor implements Extractor and DocResulter for Caddyfiles.
type CaddyfileExtractor struct{}

var (
	_ Extractor   = (*CaddyfileExtractor)(nil)
	_ DocResulter = (*CaddyfileExtractor)(nil)
)

// NewCaddyfileExtractor returns the caddyfile dialect's extractor.
func NewCaddyfileExtractor() *CaddyfileExtractor { return &CaddyfileExtractor{} }

// Extract implements Extractor.
func (x *CaddyfileExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := x.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

type caddyWalk struct {
	*confWalk
	snippets, namedRoutes map[string]bool
}

// ExtractDoc implements DocResulter.
func (x *CaddyfileExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	tree, _ := parseCaddyTree(src)
	w := &confWalk{service: service, relPath: relPath, pkg: pkgFromPath(relPath)}
	root := path.Base(relPath)
	eof := confLastLine(src)
	w.node(root, kindCaddyfileConfig, "", 1, eof)
	w.res.Roots = append(w.res.Roots, root)
	c := &caddyWalk{confWalk: w, snippets: map[string]bool{}, namedRoutes: map[string]bool{}}
	for _, n := range tree {
		if name, ok := caddyParenName(n.name, "("); ok && n.block {
			c.snippets[name] = true
		}
		if name, ok := caddyParenName(n.name, "&("); ok && n.block {
			c.namedRoutes[name] = true
		}
	}
	// The global options block (a bare { at the top) declares nothing.
	if len(tree) > 0 && tree[0].name == "" && tree[0].block {
		tree = tree[1:]
	}
	if len(tree) > 0 && !tree[0].block && !caddyTopLevel(tree[0]) {
		// One site with no braces: its addresses, then its directives.
		site := tree[0]
		i := c.site(site, eof)
		c.walk(tree[1:], c.siteName(site), w.res.Nodes[i].Name, i)
		return w.finish(), nil
	}
	for _, n := range tree {
		switch {
		case !n.block:
			c.statement(n, "", root, 0)
		case strings.HasPrefix(n.name, "&("):
			name, _ := caddyParenName(n.name, "&(")
			i := c.node(name, kindCaddyNamedRoute, "&("+name+")", n.line, n.end)
			c.walk(n.children, name, name, i)
		case strings.HasPrefix(n.name, "("):
			name, _ := caddyParenName(n.name, "(")
			i := c.node(name, kindCaddySnippet, "("+name+")", n.line, n.end)
			c.walk(n.children, name, name, i)
		default:
			i := c.site(n, n.end)
			c.walk(n.children, c.siteName(n), w.res.Nodes[i].Name, i)
		}
	}
	return w.finish(), nil
}

// caddyTopLevel: a top-level statement that is no site address line.
func caddyTopLevel(n *confNode) bool { return n.name == "import" || n.name == "" }

// caddyParenName is the name inside "(name)" or "&(name)".
func caddyParenName(s, open string) (string, bool) {
	if strings.HasPrefix(s, open) && strings.HasSuffix(s, ")") && len(s) > len(open)+1 {
		return s[len(open) : len(s)-1], true
	}
	return "", false
}

// caddyAddresses is a site's addresses, commas removed.
func caddyAddresses(n *confNode) []string {
	words := append([]string{n.name}, strings.Fields(n.argsText())...)
	var out []string
	for _, w := range words {
		for _, a := range strings.Split(w, ",") {
			if a = strings.TrimSpace(a); a != "" {
				out = append(out, a)
			}
		}
	}
	return out
}

func (c *caddyWalk) siteName(n *confNode) string {
	if a := caddyAddresses(n); len(a) > 0 {
		return a[0]
	}
	return n.name
}

func (c *caddyWalk) site(n *confNode, end int) int {
	return c.node(c.siteName(n), kindCaddySite, strings.Join(caddyAddresses(n), ", "), n.line, end)
}

// caddyMatcher reports whether a directive's first argument is a matcher
// token.
func caddyMatcher(t string) bool {
	return t == "*" || strings.HasPrefix(t, "/") || strings.HasPrefix(t, "@")
}

// walk reads a site's (snippet's, route's) statements: qual qualifies the
// entities it declares; owner and ownerIdx hold its imports.
func (c *caddyWalk) walk(nodes []*confNode, qual, owner string, ownerIdx int) {
	for _, n := range nodes {
		switch {
		case strings.HasPrefix(n.name, "@") && len(n.name) > 1:
			c.node(qualified(qual, n.name), kindCaddyMatcher, strings.TrimSpace(n.name+" "+n.argsText()), n.line, n.end)
		case caddyRouteBlocks[n.name] && n.block:
			name := c.routeName(qual, n)
			i := c.node(name, kindCaddyRoute, strings.TrimSpace(n.name+" "+n.argsText()), n.line, n.end)
			c.walk(n.children, qual, name, i)
		case caddyProxies[n.name]:
			c.proxy(qual, n)
		case n.block:
			c.walk(n.children, qual, owner, ownerIdx)
		default:
			c.statement(n, qual, owner, ownerIdx)
		}
	}
}

func (c *caddyWalk) routeName(qual string, n *confNode) string {
	name := n.name
	if m := n.argText(0); caddyMatcher(m) {
		name += " " + m
	}
	return qualified(qual, name)
}

// proxy records a reverse_proxy, php_fastcgi or forward_auth: the route
// entity and each upstream it names.
func (c *caddyWalk) proxy(qual string, n *confNode) {
	name := c.routeName(qual, n)
	c.node(name, kindCaddyRoute, strings.TrimSpace(n.name+" "+n.argsText()), n.line, n.end)
	args := n.args
	if len(args) > 0 && caddyMatcher(args[0].text) {
		args = args[1:]
	}
	for _, a := range args {
		c.route(name, a.text, a.line)
	}
	for _, sub := range n.children {
		if sub.name == "to" {
			for _, a := range sub.args {
				c.route(name, a.text, a.line)
			}
		}
	}
}

// statement reads an import or an invoke.
func (c *caddyWalk) statement(n *confNode, qual, owner string, ownerIdx int) {
	switch n.name {
	case "import":
		target := n.argText(0)
		if target == "" {
			return
		}
		if c.snippets[target] {
			c.call(ownerIdx, target)
			return
		}
		from := owner
		if ownerIdx == 0 {
			from = "" // the file itself
		}
		c.cross(from, edgeIncludes, target, c.importTarget(target), strategyRepoPath, n.line)
	case "invoke":
		if r := n.argText(0); c.namedRoutes[r] {
			c.call(ownerIdx, r)
		}
	}
}

// importTarget resolves an imported file or glob from the importing file's
// directory.
func (c *caddyWalk) importTarget(p string) string {
	dir := path.Dir(c.relPath)
	if strings.HasPrefix(p, "/") {
		const pre = "/etc/caddy/"
		if !strings.HasPrefix(p, pre) {
			return ""
		}
		p = strings.TrimPrefix(p, pre)
	}
	return repoJoin(dir, p)
}
