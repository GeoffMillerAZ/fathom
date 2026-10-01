// Command traefik-oracle is the language-breadth harness's oracle for the
// traefik doc-engine dialect (design §1.3, LB-Q-traefik). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10), tier A:
//
//	go run . -subject <checkout> [-traefik <traefik>] [-refs-out <file>] [-files-out <file>] > definitions.json
//
// The tool is Traefik itself (the brew build, 3.7.13) run with its file
// provider and its API on a local port: its own evaluation of the file, not a
// reading of it.
//
//   - A dynamic file (one with an http, tcp or udp key) is the only file of a
//     fresh run: `traefik --providers.file.filename=<file> --api.insecure`, with
//     one entry point of its own for the API. After the provider has loaded,
//     GET /api/rawdata lists every router, service and middleware of the HTTP,
//     TCP and UDP layers as the proxy built them, each named `<name>@file`. A
//     file whose load logs "Error while building configuration" (a YAML or TOML
//     error, a template error) is rejected: it is not a configuration Traefik
//     reads, contributes nothing and is counted. The names the proxy lists are
//     the entities; the files are read through Traefik's own template pass.
//   - A static file (one with an entryPoints or providers key) is run on a
//     scratch copy with every entry point moved to a free local port, the API
//     added and every provider but `file` removed (a docker or kubernetes
//     provider would need a daemon). Its entry points are what GET
//     /api/entrypoints lists. The file provider's filename or directory is run
//     as the static file names it, relative to the static file's directory, so
//     the dynamic files it loads are the tool's own: a dynamic file whose names
//     appear in the run's /api/rawdata is a file the static file loads.
//
// stdout carries the definitions: a `package` per accepted file (at its first
// key), an `resource` per entry point, a `route` per router, a `service` per
// service and a `rule` per middleware, the TCP and UDP layers included. Source
// lines are not Traefik output: they come from gopkg.in/yaml.v3 and go-toml's
// own parser for the written file, "where its key is written", the convention
// of the dialect's engine; a line is a decoder detail, never an entity decision.
//
// -refs-out carries, per file the tool accepted: references (a router's
// service and middlewares, a service's weighted, mirroring and failover
// services, a middleware chain; each as WRITTEN, with its `@file` provider
// suffix dropped, and only when the proxy lists the target in the same file's
// run), cross_references (a static file's providers.file to the dynamic file
// the proxy loaded through it, file granularity; a router's entry point or its
// service, middleware, declared in exactly one other file the same static file
// loaded, entity granularity), and, for the route_ref layer (bench-only,
// lang-oracle ignores them), route_references and route_unresolved (a service's
// loadBalancer server url or address joined to a docker-compose service or
// Kubernetes Service of the subject, or the reason it joins nothing).
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
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
	References      []pair `json:"references"`
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

// layers: the API's rawdata keys, the YAML/TOML section and the entity kinds.
var layers = []struct {
	api, section, group, source string
	kind                        string // canonical
}{
	{"routers", "http", "routers", "router", "route"},
	{"services", "http", "services", "service", "service"},
	{"middlewares", "http", "middlewares", "middleware", "rule"},
	{"tcpRouters", "tcp", "routers", "router", "route"},
	{"tcpServices", "tcp", "services", "service", "service"},
	{"tcpMiddlewares", "tcp", "middlewares", "middleware", "rule"},
	{"udpRouters", "udp", "routers", "router", "route"},
	{"udpServices", "udp", "services", "service", "service"},
}

// loc finds where a key path is written in one file.
type loc interface {
	// line of the key path's first appearance (0: not written), and the last
	// line the value reaches (for a map).
	find(keys ...string) (line, end int)
	rootLine() int
}

type yamlLoc struct{ root *yaml.Node }

func yget(n *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i], n.Content[i+1]
		}
	}
	return nil, nil
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

func (y yamlLoc) find(keys ...string) (int, int) {
	n := y.root
	var k *yaml.Node
	for _, key := range keys {
		k, n = yget(n, key)
		if n == nil {
			return 0, 0
		}
	}
	return k.Line, lastLine(n)
}
func (y yamlLoc) rootLine() int { return y.root.Line }

// tomlLoc records, for every key path prefix, the first and last line written.
type tomlLoc struct {
	first, last map[string]int
	root        int
}

func newTOMLLoc(src []byte) (*tomlLoc, error) {
	t := &tomlLoc{first: map[string]int{}, last: map[string]int{}}
	p := unstable.Parser{}
	p.Reset(src)
	var table []string
	note := func(keys []string, line int) {
		for i := 1; i <= len(keys); i++ {
			k := strings.Join(keys[:i], "\x00")
			if t.first[k] == 0 || line < t.first[k] {
				t.first[k] = line
			}
			if line > t.last[k] {
				t.last[k] = line
			}
		}
	}
	for p.NextExpression() {
		e := p.Expression()
		var keys []string
		var firstKeyOff int
		for it, i := e.Key(), 0; it.Next(); i++ {
			n := it.Node()
			keys = append(keys, string(n.Data))
			if i == 0 {
				firstKeyOff = int(p.Range(n.Data).Offset)
			}
		}
		var line int
		switch e.Kind {
		case unstable.Table, unstable.ArrayTable:
			table = keys
			line = 1 + bytes.Count(src[:firstKeyOff], []byte{'\n'})
			note(table, line)
		case unstable.KeyValue:
			line = 1 + bytes.Count(src[:firstKeyOff], []byte{'\n'})
			full := append(append([]string(nil), table...), keys...)
			note(full, line)
		default:
			continue
		}
		if t.root == 0 || line < t.root {
			t.root = line
		}
	}
	if err := p.Error(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *tomlLoc) find(keys ...string) (int, int) {
	k := strings.Join(keys, "\x00")
	return t.first[k], t.last[k]
}
func (t *tomlLoc) rootLine() int { return t.root }

type fileInfo struct {
	rel     string
	abs     string
	generic map[string]any
	loc     loc
	isTOML  bool
}

func decodeFile(abs string, b []byte) (*fileInfo, bool) {
	fi := &fileInfo{abs: abs, isTOML: strings.EqualFold(filepath.Ext(abs), ".toml")}
	if fi.isTOML {
		if err := toml.Unmarshal(b, &fi.generic); err != nil {
			return nil, false
		}
		l, err := newTOMLLoc(b)
		if err != nil {
			return nil, false
		}
		fi.loc = l
		return fi, true
	}
	var node yaml.Node
	if err := yaml.Unmarshal(b, &node); err != nil || len(node.Content) == 0 || node.Content[0].Kind != yaml.MappingNode {
		return nil, false
	}
	if err := yaml.Unmarshal(b, &fi.generic); err != nil {
		return nil, false
	}
	fi.loc = yamlLoc{root: node.Content[0]}
	return fi, true
}

func dig(m any, keys ...string) any {
	for _, k := range keys {
		mm, ok := m.(map[string]any)
		if !ok {
			return nil
		}
		m = mm[k]
	}
	return m
}

func keysOf(m any) []string {
	mm, _ := m.(map[string]any)
	var out []string
	for k := range mm {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func strList(v any) []string {
	var out []string
	switch t := v.(type) {
	case string:
		out = append(out, t)
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func strOf(v any) string {
	s, _ := v.(string)
	return s
}

var tmplRE = regexp.MustCompile(`\{\{`)

// ---- running traefik ------------------------------------------------------

type tr struct {
	bin     string
	version string
	ports   chan int
}

func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

type runResult struct {
	raw        map[string]map[string]json.RawMessage
	entryPoint []string
	errLog     string
	fileErr    bool
}

// run starts traefik with args in dir and reads its API on apiPort once the
// file provider has loaded. A dynamic run passes its own flags; a static run
// passes only --configFile, a file that already names the API entry point.
func (t *tr) run(ctx context.Context, dir string, args []string, apiPort int, flags bool) (*runResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if flags {
		args = append(args, "--api.insecure=true", fmt.Sprintf("--entrypoints.traefik.address=127.0.0.1:%d", apiPort),
			"--global.checknewversion=false", "--global.sendanonymoususage=false", "--log.level=ERROR", "--log.format=common")
	}
	cmd := exec.CommandContext(ctx, t.bin, args...)
	cmd.Dir = dir
	var logb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &logb, &logb
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	base := fmt.Sprintf("http://127.0.0.1:%d", apiPort)
	client := &http.Client{Timeout: 3 * time.Second}
	var body []byte
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/api/rawdata")
		if err == nil {
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if body == nil {
		return nil, fmt.Errorf("api never answered: %s", firstLogLine(logb.String()))
	}
	time.Sleep(700 * time.Millisecond) // the file provider's first load
	rr := &runResult{raw: map[string]map[string]json.RawMessage{}}
	if resp, err := client.Get(base + "/api/rawdata"); err == nil {
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
	}
	if err := json.Unmarshal(body, &rr.raw); err != nil {
		return nil, fmt.Errorf("rawdata: %v", err)
	}
	if resp, err := client.Get(base + "/api/entrypoints"); err == nil {
		var eps []struct {
			Name string `json:"name"`
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		_ = json.Unmarshal(b, &eps)
		for _, e := range eps {
			rr.entryPoint = append(rr.entryPoint, e.Name)
		}
	}
	rr.errLog = logb.String()
	rr.fileErr = strings.Contains(rr.errLog, "Error while building configuration") || strings.Contains(rr.errLog, "providerName=file") && strings.Contains(rr.errLog, "rror")
	return rr, nil
}

var (
	ansiRE    = regexp.MustCompile("\x1b\\[[0-9;]*m")
	stampRE   = regexp.MustCompile(`^\S+T\S+\s+(ERR|WRN|INF|DBG)\s+`)
	scratchRE = regexp.MustCompile(`/[^\s"]*/(dynamic|static)\.(yml|yaml|toml)`)
)

// firstLogLine is the first message of a log, without colour, time or scratch path.
func firstLogLine(s string) string {
	s = ansiRE.ReplaceAllString(s, "")
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			l = stampRE.ReplaceAllString(l, "")
			l = scratchRE.ReplaceAllString(l, "<file>")
			if len(l) > 150 {
				l = l[:150]
			}
			return l
		}
	}
	return ""
}

// names lists the `@file` entities of one API layer, provider suffix dropped.
func (rr *runResult) names(api string) []string {
	var out []string
	for n := range rr.raw[api] {
		if strings.HasSuffix(n, "@file") {
			out = append(out, strings.TrimSuffix(n, "@file"))
		}
	}
	sort.Strings(out)
	return out
}

// ---- the file's written references ---------------------------------------

type written struct {
	fromKind, from string // kind: router|service|middleware, with layer
	layer          string // http|tcp|udp
	toKind, to     string
}

func writtenRefs(fi *fileInfo) []written {
	var out []written
	for _, layer := range []string{"http", "tcp", "udp"} {
		for _, name := range keysOf(dig(fi.generic, layer, "routers")) {
			r := dig(fi.generic, layer, "routers", name)
			if s := strOf(dig(r, "service")); s != "" {
				out = append(out, written{"router", name, layer, "service", strings.TrimSuffix(s, "@file")})
			}
			for _, m := range strList(dig(r, "middlewares")) {
				out = append(out, written{"router", name, layer, "middleware", strings.TrimSuffix(m, "@file")})
			}
			for _, e := range strList(dig(r, "entryPoints")) {
				out = append(out, written{"router", name, layer, "entrypoint", e})
			}
		}
	}
	for _, name := range keysOf(dig(fi.generic, "http", "services")) {
		s := dig(fi.generic, "http", "services", name)
		if w, ok := dig(s, "weighted", "services").([]any); ok {
			for _, x := range w {
				if n := strOf(dig(x, "name")); n != "" {
					out = append(out, written{"service", name, "http", "service", strings.TrimSuffix(n, "@file")})
				}
			}
		}
		if m := strOf(dig(s, "mirroring", "service")); m != "" {
			out = append(out, written{"service", name, "http", "service", strings.TrimSuffix(m, "@file")})
		}
		if ms, ok := dig(s, "mirroring", "mirrors").([]any); ok {
			for _, x := range ms {
				if n := strOf(dig(x, "name")); n != "" {
					out = append(out, written{"service", name, "http", "service", strings.TrimSuffix(n, "@file")})
				}
			}
		}
		for _, k := range []string{"service", "fallback"} {
			if f := strOf(dig(s, "failover", k)); f != "" {
				out = append(out, written{"service", name, "http", "service", strings.TrimSuffix(f, "@file")})
			}
		}
	}
	for _, name := range keysOf(dig(fi.generic, "http", "middlewares")) {
		for _, m := range strList(dig(fi.generic, "http", "middlewares", name, "chain", "middlewares")) {
			out = append(out, written{"middleware", name, "http", "middleware", strings.TrimSuffix(m, "@file")})
		}
	}
	return out
}

type hostW struct{ from, raw string }

func writtenHosts(fi *fileInfo) []hostW {
	var out []hostW
	for _, layer := range []string{"http", "tcp", "udp"} {
		for _, name := range keysOf(dig(fi.generic, layer, "services")) {
			servers, _ := dig(fi.generic, layer, "services", name, "loadBalancer", "servers").([]any)
			for _, sv := range servers {
				key := "address"
				if layer == "http" {
					key = "url"
				}
				if raw := strOf(dig(sv, key)); raw != "" {
					out = append(out, hostW{name, raw})
				}
			}
		}
	}
	return out
}

// ---- main -----------------------------------------------------------------

type result struct {
	fi      *fileInfo
	dynamic *runResult
	static  *runResult
	reject  string
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	bin := flag.String("traefik", "traefik", "the traefik binary")
	refsOut := flag.String("refs-out", "", "write references, cross_references and route targets here")
	routesOut := flag.String("routes-out", "", "write the route_ref layer's truth (route_references, route_unresolved) here")
	filesOut := flag.String("files-out", "", "write the files the tool accepted, one per line, for lang-oracle -only")
	jobs := flag.Int("j", 6, "parallel traefik runs")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: traefik-oracle -subject <checkout> [-traefik <traefik>] [-refs-out <file>] [-files-out <file>]")
		os.Exit(2)
	}
	if abs, err := filepath.Abs(*subject); err == nil {
		*subject = abs
	}
	t := &tr{bin: *bin}
	vo, err := exec.Command(*bin, "version").Output()
	if err != nil {
		fatal(fmt.Errorf("traefik version: %v", err))
	}
	t.version = "unknown"
	for _, l := range strings.Split(string(vo), "\n") {
		if strings.HasPrefix(l, "Version:") {
			t.version = strings.TrimSpace(strings.TrimPrefix(l, "Version:"))
		}
	}

	var cands []*fileInfo
	skipped := map[string]int{}
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
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".yaml", ".yml", ".toml":
		default:
			return nil
		}
		b, err := readFile(p)
		if err != nil {
			return err
		}
		// a plausible Traefik file: a top-level http, tcp, udp, tls, entryPoints or providers key
		if !plausibleRE.Match(b) {
			return nil
		}
		rel, _ := filepath.Rel(*subject, p)
		fi, ok := decodeFile(p, b)
		if !ok {
			if tmplRE.Match(b) {
				skipped["a Go template Traefik renders before it decodes (not valid YAML or TOML as written)"]++
			} else {
				skipped["not decodable as YAML or TOML"]++
			}
			return nil
		}
		fi.rel = filepath.ToSlash(rel)
		cands = append(cands, fi)
		return nil
	}); err != nil {
		fatal(err)
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].rel < cands[j].rel })

	results := make([]*result, len(cands))
	var wg sync.WaitGroup
	sem := make(chan struct{}, *jobs)
	ctx := context.Background()
	for i, fi := range cands {
		i, fi := i, fi
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			res := &result{fi: fi}
			results[i] = res
			work, err := os.MkdirTemp("", "traefik-oracle-")
			if err != nil {
				res.reject = err.Error()
				return
			}
			defer os.RemoveAll(work)
			isDyn := dig(fi.generic, "http") != nil || dig(fi.generic, "tcp") != nil || dig(fi.generic, "udp") != nil
			isStatic := dig(fi.generic, "entryPoints") != nil || dig(fi.generic, "providers") != nil
			if isDyn {
				name := filepath.Join(work, "dynamic"+strings.ToLower(filepath.Ext(fi.abs)))
				b, _ := readFile(fi.abs)
				if err := os.WriteFile(name, b, 0o644); err != nil {
					res.reject = err.Error()
					return
				}
				rr, err := t.run(ctx, work, []string{"--providers.file.filename=" + name}, freePort(), true)
				switch {
				case err != nil:
					res.reject = "traefik: " + err.Error()
				case rr.fileErr:
					res.reject = "traefik rejects: " + firstLogLine(rr.errLog)
				default:
					res.dynamic = rr
				}
			}
			if isStatic && res.reject == "" {
				port := freePort()
				sc := staticCopy(fi, filepath.Dir(fi.abs), port)
				name := filepath.Join(work, "static.yml")
				if err := os.WriteFile(name, sc, 0o644); err != nil {
					res.reject = err.Error()
					return
				}
				rr, err := t.run(ctx, filepath.Dir(fi.abs), []string{"--configFile=" + name}, port, false)
				switch {
				case err != nil:
					if res.dynamic == nil {
						res.reject = "traefik (static): " + err.Error()
					}
				case rr.fileErr:
					if res.dynamic == nil {
						res.reject = "traefik (static) rejects: " + firstLogLine(rr.errLog)
					}
				default:
					res.static = rr
				}
			}
		}()
	}
	wg.Wait()

	dep := scanDeployed(*subject)
	out := output{Tool: "traefik", Version: t.version, Tier: "A", Definitions: []def{}, References: []pair{}}
	refs := output{Tool: "traefik", Version: t.version, Tier: "A", Definitions: []def{}, References: []pair{}}
	rejected := map[string]int{}
	var accepted []string
	byRel := map[string]*result{}
	for _, r := range results {
		if r.reject != "" {
			rejected[r.reject]++
			continue
		}
		if r.dynamic == nil && r.static == nil {
			skipped["a plausible file with neither an http/tcp/udp nor an entryPoints/providers key"]++
			continue
		}
		accepted = append(accepted, r.fi.rel)
		byRel[r.fi.rel] = r
	}
	sort.Strings(accepted)

	// entities per file: kind -> names (the tool's), for references and xrefs
	type ents map[string]map[string]bool
	entsOf := map[string]ents{}
	unlocated := 0
	for _, rel := range accepted {
		r := byRel[rel]
		fi := r.fi
		e := ents{}
		out.Definitions = append(out.Definitions, def{Path: rel, Name: path.Base(rel), Line: fi.loc.rootLine(), Kind: "package"})
		if r.static != nil {
			for _, name := range r.static.entryPoint {
				if name == "traefik" && dig(fi.generic, "entryPoints", "traefik") == nil {
					continue // the oracle's own API entry point, not the file's
				}
				if l, end := fi.loc.find("entryPoints", name); l > 0 {
					out.Definitions = append(out.Definitions, def{Path: rel, Name: name, Line: l, End: end, Kind: "resource"})
					if e["entrypoint"] == nil {
						e["entrypoint"] = map[string]bool{}
					}
					e["entrypoint"][name] = true
				} else {
					unlocated++
				}
			}
		}
		if r.dynamic != nil {
			for _, ly := range layers {
				for _, name := range r.dynamic.names(ly.api) {
					l, end := fi.loc.find(ly.section, ly.group, name)
					if l == 0 {
						unlocated++
						continue
					}
					out.Definitions = append(out.Definitions, def{Path: rel, Name: name, Line: l, End: end, Kind: ly.kind})
					k := ly.section + "." + ly.source
					if e[k] == nil {
						e[k] = map[string]bool{}
					}
					e[k][name] = true
				}
			}
		}
		entsOf[rel] = e
	}

	apiOf := map[string]string{"http.router": "routers", "http.service": "services", "http.middleware": "middlewares",
		"tcp.router": "tcpRouters", "tcp.service": "tcpServices", "tcp.middleware": "tcpMiddlewares", "udp.router": "udpRouters", "udp.service": "udpServices"}
	_ = apiOf
	// which dynamic files each static file loads, by the tool's own run
	covered := map[string][]string{} // static rel -> dynamic rels
	for _, rel := range accepted {
		r := byRel[rel]
		if r.static == nil {
			continue
		}
		for _, d := range accepted {
			if d == rel || byRel[d].dynamic == nil || !providerCovers(r.fi, d, *subject) {
				continue
			}
			hit := false
			for _, ly := range layers {
				for n := range entsOf[d][ly.section+"."+ly.source] {
					if _, ok := r.static.raw[ly.api][n+"@file"]; ok {
						hit = true
					}
				}
			}
			if hit {
				covered[rel] = append(covered[rel], d)
			}
		}
	}
	for _, rel := range accepted {
		r := byRel[rel]
		seen := map[string]bool{}
		if r.dynamic != nil {
			for _, w := range writtenRefs(r.fi) {
				if w.toKind == "entrypoint" {
					continue
				}
				k := w.layer + "." + w.toKind
				if !entsOf[rel][w.layer+"."+w.fromKind][w.from] || !entsOf[rel][k][w.to] {
					continue
				}
				id := w.from + "\x00" + w.to + "\x00" + w.layer + w.fromKind
				if !seen[id] {
					seen[id] = true
					refs.References = append(refs.References, pair{rel, w.from, w.to})
				}
			}
			for _, h := range writtenHosts(r.fi) {
				if tp, to, reason := dep.classify(h.raw); reason == "" {
					refs.RouteReferences = append(refs.RouteReferences, routeRef{Path: rel, From: h.from, ToPath: tp, To: to, Raw: h.raw})
				} else {
					refs.RouteUnresolved = append(refs.RouteUnresolved, routeUnresolved{Path: rel, From: h.from, Raw: h.raw, Reason: reason})
				}
			}
		}
	}
	for s, ds := range covered {
		for _, d := range ds {
			refs.CrossReferences = append(refs.CrossReferences, xref{Path: s, From: path.Base(s), To: path.Base(d), ToPath: d})
			for _, w := range writtenRefs(byRel[d].fi) {
				switch w.toKind {
				case "entrypoint":
					if entsOf[s]["entrypoint"][w.to] && entsOf[d][w.layer+"."+w.fromKind][w.from] {
						refs.CrossReferences = append(refs.CrossReferences, xref{Path: d, From: w.from, To: w.to, ToPath: s})
					}
				default:
					k := w.layer + "." + w.toKind
					if !entsOf[d][w.layer+"."+w.fromKind][w.from] || entsOf[d][k][w.to] {
						continue
					}
					var hit []string
					for _, o := range ds {
						if o != d && entsOf[o][k][w.to] {
							hit = append(hit, o)
						}
					}
					if len(hit) == 1 {
						refs.CrossReferences = append(refs.CrossReferences, xref{Path: d, From: w.from, To: w.to, ToPath: hit[0]})
					}
				}
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
		rf := refs
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
		rt := routesFile{Tool: refs.Tool, Version: refs.Version, RouteReferences: refs.RouteReferences, RouteUnresolved: refs.RouteUnresolved}
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
		if err := writeFile(*filesOut, []byte(strings.Join(accepted, "\n")+"\n")); err != nil {
			fatal(err)
		}
	}
	fmt.Fprintf(os.Stderr, "traefik-oracle: %d files accepted, %d definitions, %d references, %d cross_references, %d route_references, %d route_unresolved, %d entities the file does not write (unlocated)\n",
		len(accepted), len(out.Definitions), len(refs.References), len(refs.CrossReferences), len(refs.RouteReferences), len(refs.RouteUnresolved), unlocated)
	for _, m := range []map[string]int{rejected, skipped} {
		var reasons []string
		for k, n := range m {
			reasons = append(reasons, fmt.Sprintf("  %5d  %s", n, k))
		}
		sort.Strings(reasons)
		for _, l := range reasons {
			fmt.Fprintln(os.Stderr, l)
		}
	}
}

var plausibleRE = regexp.MustCompile(`(?m)^(http|tcp|udp|tls|entryPoints|providers)[ \t]*:|^\[\[?(http|tcp|udp|tls|entryPoints|providers)[.\]]`)

// providerCovers: the static file's providers.file filename or directory
// (relative to the static file's directory, as the run resolved them) names d.
func providerCovers(s *fileInfo, d, subject string) bool {
	fp := dig(s.generic, "providers", "file")
	if fp == nil {
		return false
	}
	dir := filepath.Dir(s.abs)
	target := filepath.Join(subject, filepath.FromSlash(d))
	if fn := strOf(dig(fp, "filename")); fn != "" {
		if !filepath.IsAbs(fn) {
			fn = filepath.Join(dir, fn)
		}
		return filepath.Clean(fn) == target
	}
	if dn := strOf(dig(fp, "directory")); dn != "" {
		if !filepath.IsAbs(dn) {
			dn = filepath.Join(dir, dn)
		}
		return filepath.Clean(dn) == filepath.Dir(target)
	}
	return false
}

// staticCopy is the static file reduced to what the run needs: its entry
// points on free local ports, the API entry point, and the file provider alone
// with its filename or directory made absolute against the static file's
// directory (the tool then loads the dynamic files the file names).
func staticCopy(fi *fileInfo, dir string, apiPort int) []byte {
	cfg := map[string]any{"api": map[string]any{"insecure": true}, "global": map[string]any{"checkNewVersion": false, "sendAnonymousUsage": false}, "log": map[string]any{"level": "ERROR"}}
	eps := map[string]any{"traefik": map[string]any{"address": fmt.Sprintf("127.0.0.1:%d", apiPort)}}
	for _, name := range keysOf(dig(fi.generic, "entryPoints")) {
		ep, _ := dig(fi.generic, "entryPoints", name).(map[string]any)
		cp := map[string]any{}
		for k, v := range ep {
			if k != "http" && k != "http2" && k != "http3" && k != "proxyProtocol" && k != "forwardedHeaders" && k != "transport" {
				cp[k] = v
			}
		}
		cp["address"] = fmt.Sprintf("127.0.0.1:%d", freePort())
		if name == "traefik" {
			cp["address"] = fmt.Sprintf("127.0.0.1:%d", apiPort) // the API's own entry point
		}
		eps[name] = cp
	}
	cfg["entryPoints"] = eps
	if fp, ok := dig(fi.generic, "providers", "file").(map[string]any); ok {
		pf := map[string]any{}
		if fn := strOf(fp["filename"]); fn != "" {
			if !filepath.IsAbs(fn) {
				fn = filepath.Join(dir, fn)
			}
			pf["filename"] = filepath.Clean(fn)
		}
		if dn := strOf(fp["directory"]); dn != "" {
			if !filepath.IsAbs(dn) {
				dn = filepath.Join(dir, dn)
			}
			pf["directory"] = filepath.Clean(dn)
		}
		cfg["providers"] = map[string]any{"file": pf}
	}
	b, _ := yaml.Marshal(cfg)
	return b
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "traefik-oracle:", err)
	os.Exit(1)
}
