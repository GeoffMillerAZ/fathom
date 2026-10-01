package extractor

// haproxydialect.go — the HAProxy native dialect (LB-Q-haproxy, docs/design/
// language-breadth.md §13, engine "native": grammar_source native:pure-go).
// confsyntax.go reads a configuration file with HAProxy's line rules
// (quotes, escapes, # comments); a line whose first word is a section
// keyword starts a section, every other line belongs to the section above
// it (dialects/haproxy/PROVENANCE.md says what is and is not modelled).
//
// Entities: a frontend, a backend and a listen section (HAProxyFrontend,
// HAProxyBackend, HAProxyListen; services, named by the section's name); in
// a backend or listen section, a server or server-template (HAProxyServer, a
// target, qualified by the section); in a proxy section, an acl
// (HAProxyACL, a rule, qualified by the section; each line is one) and a
// use_backend or default_backend line (HAProxyRoute, a route, named by the
// backend it names, qualified by the section).
//
// Intra-file references (L2): a route to the backend (or listen section) it
// names, when the file declares one of that name; a name built from a sample
// expression (%[...]) is no reference.
//
// Cross-file references (L3, the route contract, ADR-050): each server's
// address as written (host:port, an IP, unix@path, ${VAR}:port), a route
// target from its backend (or listen) section, edge route_ref, strategy
// route-host; fusion.ResolveRouteRefs resolves it or discloses why not.

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const (
	kindHAProxyFrontend core.SourceKind = "HAProxyFrontend"
	kindHAProxyBackend  core.SourceKind = "HAProxyBackend"
	kindHAProxyListen   core.SourceKind = "HAProxyListen"
	kindHAProxyServer   core.SourceKind = "HAProxyServer"
	kindHAProxyACL      core.SourceKind = "HAProxyACL"
	kindHAProxyRoute    core.SourceKind = "HAProxyRoute"
)

// haproxySections are the keywords that start a section.
var haproxySections = map[string]bool{
	"global": true, "defaults": true, "frontend": true, "backend": true, "listen": true,
	"peers": true, "resolvers": true, "userlist": true, "mailers": true, "cache": true,
	"program": true, "http-errors": true, "ring": true, "log-forward": true,
	"crt-store": true, "traces": true, "acme": true, "fcgi-app": true,
}

var haproxyProxyKinds = map[string]core.SourceKind{
	"frontend": kindHAProxyFrontend, "backend": kindHAProxyBackend, "listen": kindHAProxyListen,
}

// HAProxyExtractor implements Extractor and DocResulter for HAProxy
// configuration files.
type HAProxyExtractor struct{}

var (
	_ Extractor   = (*HAProxyExtractor)(nil)
	_ DocResulter = (*HAProxyExtractor)(nil)
)

// NewHAProxyExtractor returns the haproxy dialect's extractor.
func NewHAProxyExtractor() *HAProxyExtractor { return &HAProxyExtractor{} }

// Extract implements Extractor.
func (x *HAProxyExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := x.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// haproxySection is one section being read.
type haproxySection struct {
	keyword, name string
	idx           int // the entity's index, -1 for a section that is none
	line, end     int
	routes        []haproxyRoute
}

type haproxyRoute struct {
	idx     int
	backend string
}

// ExtractDoc implements DocResulter.
func (x *HAProxyExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	toks, _ := confLex(src, haproxySyntax)
	w := &confWalk{service: service, relPath: relPath, pkg: pkgFromPath(relPath)}
	declared := map[string]bool{}
	var secs []*haproxySection
	var cur *haproxySection
	for _, ln := range confLines(toks) {
		kw := ln[0].text
		if haproxySections[kw] && !ln[0].quoted {
			cur = &haproxySection{keyword: kw, idx: -1, line: ln[0].line, end: ln[len(ln)-1].line}
			if len(ln) > 1 {
				cur.name = ln[1].text
			}
			if k, ok := haproxyProxyKinds[kw]; ok && cur.name != "" {
				cur.idx = w.node(cur.name, k, strings.TrimSpace(kw+" "+cur.name), cur.line, cur.end)
				if kw != "frontend" {
					declared[cur.name] = true
				}
			}
			secs = append(secs, cur)
			continue
		}
		if cur == nil {
			continue
		}
		cur.end = ln[len(ln)-1].line
		if cur.idx < 0 {
			continue
		}
		x.line(w, cur, ln)
	}
	for _, s := range secs {
		if s.idx >= 0 {
			w.res.Nodes[s.idx].EndLine = s.end
		}
		for _, r := range s.routes {
			if declared[r.backend] {
				w.call(r.idx, r.backend)
			}
		}
	}
	return w.finish(), nil
}

// line reads one statement of a proxy section.
func (x *HAProxyExtractor) line(w *confWalk, s *haproxySection, ln []confTok) {
	kw := ln[0].text
	sig := confJoin(ln)
	line := ln[0].line
	switch kw {
	case "server", "server-template":
		if s.keyword == "frontend" || len(ln) < 3 {
			return
		}
		addr := ln[2].text
		if kw == "server-template" {
			if len(ln) < 4 {
				return
			}
			addr = ln[3].text
		}
		w.node(qualified(s.name, ln[1].text), kindHAProxyServer, sig, line, ln[len(ln)-1].line)
		w.route(s.name, addr, line)
	case "acl":
		if len(ln) < 2 {
			return
		}
		w.node(qualified(s.name, ln[1].text), kindHAProxyACL, sig, line, ln[len(ln)-1].line)
	case "use_backend", "default_backend":
		if s.keyword == "backend" || len(ln) < 2 {
			return
		}
		be := ln[1].text
		i := w.node(qualified(s.name, be), kindHAProxyRoute, sig, line, ln[len(ln)-1].line)
		if !strings.Contains(be, "%[") {
			s.routes = append(s.routes, haproxyRoute{idx: i, backend: be})
		}
	}
}

// confJoin joins a line's words with single spaces.
func confJoin(ln []confTok) string {
	parts := make([]string, len(ln))
	for i, t := range ln {
		parts[i] = t.text
	}
	return strings.Join(parts, " ")
}
