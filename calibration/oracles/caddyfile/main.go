// Command caddyfile-oracle is the language-breadth harness's oracle for the
// caddyfile native dialect (design §1.3, §13.4 step 19; LB-Q-caddyfile). It
// writes indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10),
// tier A:
//
//	go run . -subject <dir> -caddy <caddy binary> -out <file> [-routes-out <file>]
//	    [-materialize-from <caddyfile_adapt dir>] [-env KEY=VALUE]... [-files-out <file>]
//	    [-summary-out <file>]
//
// Tier A is Caddy itself, v2.11.6: `caddy adapt --config <file> --adapter
// caddyfile` accepts the file (one it rejects is counted and left out) and its
// JSON names every reverse_proxy upstream it dials; the reference lexer,
// caddyfile.Tokenize from github.com/caddyserver/caddy/v2 v2.11.6 (the
// format's reference parser as a library), gives every token with its line
// and whether it was quoted; caddyfile.Parse, the reference parser, gives each
// server block's keys, which cross-check the site blocks read here (stderr
// and -summary-out count agreement, and the upstreams against the adapted
// JSON's dials). The statements are read from the reference tokens with the
// Caddyfile's own structure rules: a statement starts on a new line, a
// statement whose last token is an unquoted { opens a block, a statement that
// is a lone unquoted } closes one; at the top a bare { is the global options
// block, (name) a snippet, &(name) a named route, anything else a site's
// addresses (a file whose first statement opens no block is one site).
// Emitted, under the dialect's naming contract (its PROVENANCE.md):
//
//   - definitions: a package per file (its base name, line 1 to its last line);
//     a service per site block (its first address); a mixin per snippet and a
//     route per named route (the name inside the parentheses); a selector per
//     named matcher (@name, qualified by its site or snippet and a space); a
//     route per handle, handle_path, handle_errors and route block and per
//     reverse_proxy, php_fastcgi and forward_auth directive (the directive and
//     its matcher token, *, /path or @name, when it has one, qualified by its
//     site). A block's end is its closing brace's line, a statement's its last
//     token's.
//   - references: an import of a snippet the file declares, and an invoke of a
//     named route it declares, from the enclosing entity.
//   - cross_references (file granularity): an import that names no snippet, a
//     file or a glob relative to the importing file's directory, to each
//     matching Caddyfile of the subject.
//
// Route targets (-routes-out, bench-only; lang-oracle's tool-oracle decoder
// refuses unknown fields): each upstream of a reverse_proxy, php_fastcgi or
// forward_auth (its arguments after the matcher, and its to subdirective's),
// resolved against the nearest compose file at or above the Caddyfile
// (`docker compose config --format json`, Docker Compose 5.1.4): a service name,
// or a container_name, hostname or network alias of exactly one service, is a
// route_reference; otherwise a route_unresolved with its reason: variable ({ or
// $), unix-socket, ip-literal (an IP address or localhost), ambiguous,
// external (a dotted host) or undeclared.
//
// -env sets an environment variable for caddy adapt (a deployment's {$NAME}
// placeholder, which Caddy substitutes before parsing: an empty one can leave a
// site with no address); the tokens read here are the file's, placeholders as
// written, as the dialect reads them. -files-out lists the files caddy adapt
// accepted, for lang-oracle's -only (a subject built with plugins, such as
// picosh/pico's cloudflare DNS and postgres storage, is adapted by a caddy built
// the way its Dockerfile builds one).
//
// -materialize-from turns caddy's caddytest/integration/caddyfile_adapt into a
// subject: each .caddyfiletest's Caddyfile half (the lines before its first
// line of dashes) is written to <subject>/<test>.caddyfile and committed in a
// fresh repository at a fixed date, so the reading names a reproducible SHA.
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end"`
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
}

type route struct {
	from, raw string
}

var skipDirs = map[string]bool{"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true, ".idea": true, ".vscode": true, ".claude": true}

func isCaddyfile(name string) bool {
	return name == "Caddyfile" || strings.HasPrefix(name, "Caddyfile.") || strings.EqualFold(path.Ext(name), ".caddyfile")
}

func main() {
	subject := flag.String("subject", "", "subject directory (required)")
	caddy := flag.String("caddy", "", "caddy binary (required)")
	out := flag.String("out", "", "tool-oracle JSON to write (required)")
	routesOut := flag.String("routes-out", "", "route_references/route_unresolved JSON to write")
	materialize := flag.String("materialize-from", "", "a caddyfile_adapt directory to materialize into -subject first")
	summaryOut := flag.String("summary-out", "", "agreement summary JSON to write")
	filesOut := flag.String("files-out", "", "the files caddy adapt accepted, one per line")
	var env multi
	flag.Var(&env, "env", "KEY=VALUE for caddy adapt (repeatable)")
	flag.Parse()
	if *subject == "" || *caddy == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *materialize != "" {
		if err := materializeTests(*materialize, *subject); err != nil {
			fmt.Fprintln(os.Stderr, "caddyfile-oracle:", err)
			os.Exit(1)
		}
	}
	root, _ := filepath.Abs(*subject)
	var files []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if isCaddyfile(d.Name()) {
			rel, _ := filepath.Rel(root, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)

	var defs []def
	var refs []ref
	var xrefs []xref
	rejected := map[string]string{}
	routes := map[string][]route{}
	agree := map[string]int{}
	for _, rel := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		dials, err := adapt(*caddy, abs, env)
		if err != nil {
			rejected[rel] = err.Error()
			continue
		}
		toks, err := caddyfile.Tokenize(data, abs)
		if err != nil {
			rejected[rel] = "tokenize: " + err.Error()
			continue
		}
		w := &walk{rel: rel, root: path.Base(rel), eof: lastLine(data), snippets: map[string]bool{}, named: map[string]bool{}}
		w.read(statements(toks))
		defs = append(defs, w.defs...)
		refs = append(refs, w.refs...)
		for _, im := range w.imports {
			for _, t := range importTargets(root, rel, im.raw) {
				xrefs = append(xrefs, xref{Path: rel, From: im.from, ToPath: t})
			}
		}
		routes[rel] = w.routes
		// Cross-checks: the reference parser's server-block keys, the adapted
		// JSON's dials.
		if sbs, perr := caddyfile.Parse(abs, data); perr == nil {
			for _, sb := range sbs {
				if len(sb.Keys) == 0 {
					continue
				}
				k := strings.TrimSuffix(sb.Keys[0].Text, ",")
				if sb.IsNamedRoute {
					k = strings.TrimSuffix(strings.TrimPrefix(k, "&("), ")")
				}
				agree["parser_blocks"]++
				if w.sites[k] {
					agree["parser_blocks_read"]++
				}
			}
		}
		for _, r := range w.routes {
			agree["upstreams"]++
			if dials[r.raw] || dials[strings.TrimPrefix(strings.TrimPrefix(r.raw, "http://"), "https://")] {
				agree["upstreams_dialed"]++
			}
		}
	}
	version := strings.Fields(run(*caddy, "", "version"))
	v := ""
	if len(version) > 0 {
		v = version[0]
	}
	writeJSON(*out, map[string]any{"tool": "caddy", "version": v, "tier": "A", "definitions": nonNil(defs), "references": nonNil(refs), "cross_references": nonNil(xrefs)})

	summary := map[string]any{"files": len(files) - len(rejected), "rejected": rejected, "agreement": agree}
	if *routesOut != "" {
		var resolved, unresolved []map[string]string
		cache := map[string]map[string][]string{}
		for _, rel := range files {
			for _, r := range routes[rel] {
				svc, composeFiles, reason := classify(r.raw, func(host string) []composeHit {
					return resolveCompose(root, rel, host, func(c string) map[string][]string { return composeNamesOf(root, c, cache) })
				})
				if svc != "" {
					for _, compose := range composeFiles {
						resolved = append(resolved, map[string]string{"path": rel, "from": r.from, "to_path": compose, "to": svc, "raw": r.raw})
					}
				} else {
					unresolved = append(unresolved, map[string]string{"path": rel, "from": r.from, "raw": r.raw, "reason": reason})
				}
			}
		}
		writeJSON(*routesOut, map[string]any{"tool": "caddy+docker-compose", "version": v, "tier": "A", "route_references": nonNil(resolved), "route_unresolved": nonNil(unresolved)})
		summary["routes"] = map[string]int{"resolved": len(resolved), "unresolved": len(unresolved)}
	}
	fmt.Fprintf(os.Stderr, "caddyfile-oracle: %d files, %d rejected, %d definitions, %d references, %d cross-references; agreement %v\n",
		len(files)-len(rejected), len(rejected), len(defs), len(refs), len(xrefs), agree)
	for _, rel := range sortedKeys(rejected) {
		fmt.Fprintf(os.Stderr, "caddyfile-oracle: rejected %s: %s\n", rel, rejected[rel])
	}
	if *summaryOut != "" {
		writeJSON(*summaryOut, summary)
	}
	if *filesOut != "" {
		var ok []string
		for _, rel := range files {
			if _, bad := rejected[rel]; !bad {
				ok = append(ok, rel)
			}
		}
		if err := os.WriteFile(*filesOut, []byte(strings.Join(ok, "\n")+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "caddyfile-oracle:", err)
			os.Exit(1)
		}
	}
}

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// adapt runs caddy adapt on file and returns the dial addresses its JSON names.
func adapt(caddy, file string, env []string) (map[string]bool, error) {
	cmd := exec.Command(caddy, "adapt", "--config", file, "--adapter", "caddyfile")
	cmd.Dir = filepath.Dir(file)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
		return nil, fmt.Errorf("caddy adapt: %s", lines[len(lines)-1])
	}
	var doc any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		return nil, fmt.Errorf("caddy adapt output: %w", err)
	}
	dials := map[string]bool{}
	var visit func(any)
	visit = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			if d, ok := v["dial"].(string); ok {
				dials[d] = true
			}
			for _, c := range v {
				visit(c)
			}
		case []any:
			for _, c := range v {
				visit(c)
			}
		}
	}
	visit(doc)
	return dials, nil
}

// stmt is one statement: its tokens, whether it opens a block, and its
// block's statements and closing line.
type stmt struct {
	toks     []caddyfile.Token
	block    bool
	children []*stmt
	end      int
}

func (s *stmt) name() string { return s.toks[0].Text }

func (s *stmt) args() []caddyfile.Token { return s.toks[1:] }

func (s *stmt) argsText() string {
	var parts []string
	for _, t := range s.args() {
		parts = append(parts, t.Text)
	}
	return strings.Join(parts, " ")
}

// statements groups the reference tokens into statements and blocks.
func statements(toks []caddyfile.Token) []*stmt {
	var lines [][]caddyfile.Token
	nextLine := 0
	for _, t := range toks {
		if len(lines) == 0 || t.Line > nextLine {
			lines = append(lines, nil)
		}
		lines[len(lines)-1] = append(lines[len(lines)-1], t)
		nextLine = t.Line + t.NumLineBreaks()
	}
	i := 0
	var block func(depth int) ([]*stmt, int)
	block = func(depth int) ([]*stmt, int) {
		var out []*stmt
		for i < len(lines) {
			ln := lines[i]
			i++
			if len(ln) == 1 && ln[0].Text == "}" && !ln[0].Quoted() {
				if depth > 0 {
					return out, ln[0].Line
				}
				continue
			}
			for depth == 0 && strings.HasSuffix(ln[len(ln)-1].Text, ",") && i < len(lines) {
				ln = append(append([]caddyfile.Token(nil), ln...), lines[i]...)
				i++
			}
			last := ln[len(ln)-1]
			s := &stmt{toks: ln, end: last.Line + last.NumLineBreaks()}
			if last.Text == "{" && !last.Quoted() {
				s.block = true
				s.toks = ln[:len(ln)-1]
				if len(s.toks) == 0 {
					s.toks = []caddyfile.Token{{Line: last.Line}}
				}
				s.children, s.end = block(depth + 1)
			}
			out = append(out, s)
		}
		return out, -1
	}
	out, _ := block(0)
	return out
}

type importRef struct{ from, raw string }

type walk struct {
	rel, root       string
	eof             int
	defs            []def
	refs            []ref
	imports         []importRef
	routes          []route
	snippets, named map[string]bool
	sites           map[string]bool
}

func (w *walk) add(name string, line, end int, kind string) {
	if end < 0 || end < line {
		end = line
	}
	w.defs = append(w.defs, def{Path: w.rel, Name: name, Line: line, End: end, Kind: kind})
}

func qualified(q, n string) string {
	if q == "" {
		return n
	}
	return q + " " + n
}

func parenName(s, open string) (string, bool) {
	if strings.HasPrefix(s, open) && strings.HasSuffix(s, ")") && len(s) > len(open)+1 {
		return s[len(open) : len(s)-1], true
	}
	return "", false
}

func addresses(s *stmt) []string {
	var out []string
	for _, t := range s.toks {
		for _, a := range strings.Split(t.Text, ",") {
			if a = strings.TrimSpace(a); a != "" {
				out = append(out, a)
			}
		}
	}
	return out
}

func (w *walk) read(top []*stmt) {
	w.sites = map[string]bool{}
	w.add(w.root, 1, w.eof, "package")
	for _, s := range top {
		if !s.block {
			continue
		}
		if n, ok := parenName(s.name(), "("); ok {
			w.snippets[n] = true
		}
		if n, ok := parenName(s.name(), "&("); ok {
			w.named[n] = true
		}
	}
	if len(top) > 0 && top[0].block && top[0].name() == "" {
		top = top[1:]
	}
	if len(top) > 0 && !top[0].block && top[0].name() != "import" {
		site := addresses(top[0])[0]
		w.sites[site] = true
		w.add(site, top[0].toks[0].Line, w.eof, "service")
		w.walk(top[1:], site, site)
		return
	}
	for _, s := range top {
		switch {
		case !s.block:
			w.statement(s, w.root)
		case strings.HasPrefix(s.name(), "&("):
			n, _ := parenName(s.name(), "&(")
			w.sites[n] = true
			w.add(n, s.toks[0].Line, s.end, "route")
			w.walk(s.children, n, n)
		case strings.HasPrefix(s.name(), "("):
			n, _ := parenName(s.name(), "(")
			w.add(n, s.toks[0].Line, s.end, "mixin")
			w.walk(s.children, n, n)
		default:
			site := addresses(s)[0]
			w.sites[site] = true
			w.add(site, s.toks[0].Line, s.end, "service")
			w.walk(s.children, site, site)
		}
	}
}

var routeBlocks = map[string]bool{"handle": true, "handle_path": true, "handle_errors": true, "route": true}
var proxies = map[string]bool{"reverse_proxy": true, "php_fastcgi": true, "forward_auth": true}

func isMatcher(t string) bool {
	return t == "*" || strings.HasPrefix(t, "/") || strings.HasPrefix(t, "@")
}

func (w *walk) routeName(qual string, s *stmt) string {
	n := s.name()
	if a := s.args(); len(a) > 0 && isMatcher(a[0].Text) {
		n += " " + a[0].Text
	}
	return qualified(qual, n)
}

func (w *walk) walk(ss []*stmt, qual, owner string) {
	for _, s := range ss {
		n := s.name()
		switch {
		case strings.HasPrefix(n, "@") && len(n) > 1:
			w.add(qualified(qual, n), s.toks[0].Line, s.end, "selector")
		case routeBlocks[n] && s.block:
			rn := w.routeName(qual, s)
			w.add(rn, s.toks[0].Line, s.end, "route")
			w.walk(s.children, qual, rn)
		case proxies[n]:
			rn := w.routeName(qual, s)
			w.add(rn, s.toks[0].Line, s.end, "route")
			args := s.args()
			if len(args) > 0 && isMatcher(args[0].Text) {
				args = args[1:]
			}
			for _, a := range args {
				w.routes = append(w.routes, route{rn, a.Text})
			}
			for _, sub := range s.children {
				if sub.name() == "to" {
					for _, a := range sub.args() {
						w.routes = append(w.routes, route{rn, a.Text})
					}
				}
			}
		case s.block:
			w.walk(s.children, qual, owner)
		default:
			w.statement(s, owner)
		}
	}
}

func (w *walk) statement(s *stmt, owner string) {
	a := s.args()
	if len(a) == 0 {
		return
	}
	switch s.name() {
	case "import":
		if w.snippets[a[0].Text] {
			w.refs = append(w.refs, ref{Path: w.rel, From: owner, To: a[0].Text})
			return
		}
		w.imports = append(w.imports, importRef{from: owner, raw: a[0].Text})
	case "invoke":
		if w.named[a[0].Text] {
			w.refs = append(w.refs, ref{Path: w.rel, From: owner, To: a[0].Text})
		}
	}
}

// importTargets resolves an imported file or glob from the importing file's
// directory to the Caddyfiles of the subject it names.
func importTargets(root, rel, raw string) []string {
	p := raw
	if strings.HasPrefix(p, "/") {
		const pre = "/etc/caddy/"
		if !strings.HasPrefix(p, pre) {
			return nil
		}
		p = strings.TrimPrefix(p, pre)
	}
	pattern := filepath.Join(root, filepath.FromSlash(path.Dir(rel)), filepath.FromSlash(p))
	matches, _ := filepath.Glob(pattern)
	var out []string
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil && !fi.IsDir() {
			r, _ := filepath.Rel(root, m)
			r = filepath.ToSlash(r)
			if !strings.HasPrefix(r, "../") {
				out = append(out, r)
			}
		}
	}
	sort.Strings(out)
	return out
}

func lastLine(b []byte) int {
	n := 1 + bytes.Count(b, []byte("\n"))
	if len(b) > 0 && b[len(b)-1] == '\n' {
		n--
	}
	return max(n, 1)
}

// composeName matches the compose-named files of one directory: compose.y[a]ml,
// docker-compose.y[a]ml and their override or environment variants. A compose
// project is a directory.
var composeName = regexp.MustCompile(`(?i)^(docker-)?compose([.-].+)?\.ya?ml$`)

// composeFilesIn lists the compose-named files of the directory dir (relative).
func composeFilesIn(root, dir string) []string {
	ents, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && composeName.MatchString(e.Name()) {
			out = append(out, path.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

// composeHit is one compose file that declares a host: the services behind it.
type composeHit struct {
	file string
	svcs []string
}

// resolveCompose is the nearest compose project that declares host: the
// directory of the Caddyfile, then each ancestor. Every compose file of that
// directory that declares the name is one hit (an override file is the same
// service, compose merges them).
func resolveCompose(root, rel, host string, names func(string) map[string][]string) []composeHit {
	for d := path.Dir(rel); ; d = path.Dir(d) {
		var hits []composeHit
		for _, c := range composeFilesIn(root, d) {
			if svcs := names(c)[host]; len(svcs) > 0 {
				hits = append(hits, composeHit{c, svcs})
			}
		}
		if len(hits) > 0 {
			return hits
		}
		if d == "." || d == "/" || d == "" {
			return nil
		}
	}
}

// routeHits is the one service behind a host's hits and the files declaring it:
// a pair per file for one service, "ambiguous" for several behind one name.
func routeHits(hits []composeHit) (svc string, files []string, reason string) {
	seen := map[string]bool{}
	for _, h := range hits {
		for _, sv := range h.svcs {
			seen[sv] = true
		}
	}
	switch len(seen) {
	case 0:
		return "", nil, ""
	case 1:
		for sv := range seen {
			svc = sv
		}
		for _, h := range hits {
			files = append(files, h.file)
		}
		return svc, files, ""
	}
	return "", nil, "ambiguous"
}

// composeNamesOf maps every name `docker compose config` gives one compose file
// (service key, container_name, hostname, network aliases) to its services.
func composeNamesOf(root, cand string, cache map[string]map[string][]string) map[string][]string {
	if names, ok := cache[cand]; ok {
		return names
	}
	names := map[string][]string{}
	b, err := exec.Command("docker", "compose", "-f", filepath.Join(root, filepath.FromSlash(cand)), "config", "--format", "json",
		"--no-interpolate", "--no-path-resolution", "--no-consistency", "--no-normalize").Output()
	if err == nil {
		var cfg struct {
			Services map[string]struct {
				ContainerName string `json:"container_name"`
				Hostname      string `json:"hostname"`
				Networks      map[string]*struct {
					Aliases []string `json:"aliases"`
				} `json:"networks"`
			} `json:"services"`
		}
		if json.Unmarshal(b, &cfg) == nil {
			add := func(alias, svc string) {
				if alias != "" && !contains(names[alias], svc) {
					names[alias] = append(names[alias], svc)
				}
			}
			for svc, spec := range cfg.Services {
				add(svc, svc)
				add(spec.ContainerName, svc)
				add(spec.Hostname, svc)
				for _, n := range spec.Networks {
					if n != nil {
						for _, al := range n.Aliases {
							add(al, svc)
						}
					}
				}
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "caddyfile-oracle: docker compose config %s: %v\n", cand, err)
	}
	cache[cand] = names
	return names
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// classify is the service, its declaring compose files and the reason a
// reverse_proxy address is not one; lookup(host) is the nearest project's hits.
func classify(raw string, lookup func(host string) []composeHit) (string, []string, string) {
	switch {
	case strings.ContainsAny(raw, "{$"):
		return "", nil, "variable"
	case strings.HasPrefix(raw, "unix/") || strings.HasPrefix(raw, "unix//") || strings.HasPrefix(raw, "unix+"):
		return "", nil, "unix-socket"
	}
	s := raw
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	host := s
	if h, _, err := net.SplitHostPort(s); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" || net.ParseIP(host) != nil {
		return "", nil, "ip-literal"
	}
	if svc, files, reason := routeHits(lookup(host)); svc != "" || reason != "" {
		return svc, files, reason
	}
	if strings.Contains(host, ".") {
		return "", nil, "external"
	}
	return "", nil, "undeclared"
}

// materializeTests writes each .caddyfiletest's Caddyfile half into dst and
// commits it in a fresh repository.
func materializeTests(src, dst string) error {
	ents, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".caddyfiletest") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		var kept []string
		for _, ln := range strings.Split(string(b), "\n") {
			if t := strings.TrimSpace(ln); len(t) >= 3 && strings.Trim(t, "-") == "" {
				break
			}
			kept = append(kept, ln)
		}
		name := strings.TrimSuffix(e.Name(), ".caddyfiletest") + ".caddyfile"
		if err := os.WriteFile(filepath.Join(dst, name), []byte(strings.TrimRight(strings.Join(kept, "\n"), "\n")+"\n"), 0o644); err != nil {
			return err
		}
		n++
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=caddyfile-oracle", "-c", "user.email=oracle@localhost", "commit", "-qm", fmt.Sprintf("materialize %d caddyfile_adapt tests", n)}} {
		cmd := exec.Command("git", append([]string{"-C", dst}, args...)...)
		// A fixed date: the same tests materialize to the same commit, so a
		// reading's SHA names the derived subject reproducibly.
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-09-28T00:00:00Z", "GIT_COMMITTER_DATE=2026-09-28T00:00:00Z")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %v: %s", args, err, out)
		}
	}
	fmt.Fprintf(os.Stderr, "caddyfile-oracle: materialized %d tests into %s\n", n, dst)
	return nil
}

func run(bin, dir string, args ...string) string {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	b, _ := cmd.Output()
	return string(b)
}

func writeJSON(p string, v any) {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "caddyfile-oracle:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "caddyfile-oracle:", err)
		os.Exit(1)
	}
}

func nonNil[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

func sortedKeys(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
