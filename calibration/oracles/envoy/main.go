// Command envoy-oracle is the language-breadth harness's oracle for the envoy
// doc-engine dialect (design §1.3, LB-Q-envoy). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10):
//
//	go run . -subject <checkout> [-envoy <envoy>] [-refs-out <file>] [-files-out <file>] > definitions.json
//
// Two instruments read each file, and a file is the oracle's only when BOTH
// accept it:
//
//   - Envoy itself: `envoy --mode validate -c <file>` (the brew build, 1.39.1)
//     is the acceptance. It loads the bootstrap as the proxy would, so a file
//     it rejects (an unknown field, a missing extension, a bad address) is not
//     an Envoy configuration, contributes nothing and is counted with its first
//     error line. An xDS resource file (a DiscoveryResponse: `resources:` of
//     `@type` objects, which no bootstrap names as a bootstrap) is validated by
//     the bootstrap that loads it, so it is accepted when it decodes strictly
//     as a DiscoveryResponse.
//   - Envoy's own API schema: go-control-plane's v3 messages (generated from the
//     same .proto files Envoy's C++ parser reads), decoded with protojson
//     STRICTLY (no unknown field is discarded) into a Bootstrap or a
//     DiscoveryResponse. Every typed_config the registry knows (the
//     HttpConnectionManager, which carries the route configuration, and every
//     message of the API's core packages) is resolved and decoded; an extension
//     the registry does not carry (a filter, a transport socket) is wrapped as a
//     google.protobuf.Struct, so it is skipped, not guessed. Entities are read
//     off the decoded messages, never off the YAML.
//
// Tier A: the entities' existence, names and references are the schema's own
// reading, cross-checked by the proxy's acceptance; the argument is in README.md
// (design §1.3: "the format's reference parser" is A, an independent library is
// C; here the schema is the reference, and its two decoders, the proxy's and
// protojson's, must agree on every file). Source lines are not parser output in
// either: they come from gopkg.in/yaml.v3, the library the dialect's engine
// reads lines with too, by the field path of each decoded message; a line is a
// decoder detail, never an entity decision.
//
// stdout (tier A) carries the definitions: a `package` per file, a `resource`
// per listener, a `section` per route configuration, a `selector` per virtual
// host, a `route` per route that has a name (or a match prefix, path or regex)
// and a `service` per cluster. A listener without a name is identified by its
// bind port, then its pipe path, as the dialect names it.
//
// -refs-out (tier A) carries references (a route to the cluster it names or
// weights or mirrors to, a listener to the route configuration its rds names),
// each only when the target is declared in the same file, cross_references
// (the file-based xDS edges: a ConfigSource path that names a file of the
// subject, file granularity; and a route's cluster or a listener's rds name
// declared in a file a bootstrap's xDS paths load, entity granularity), and,
// for the route_ref layer (bench-only, lang-oracle ignores them),
// route_references (a cluster endpoint's address that joins a docker-compose
// service or Kubernetes Service of the subject) and route_unresolved (the rest,
// with the reason).
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
	"strconv"
	"strings"
	"time"

	_ "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	_ "google.golang.org/protobuf/types/known/structpb"
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

// seg is one step of a message's field path: a field name and, for a repeated
// field, the index; a map field is not used by the entities read.
type seg struct {
	field string
	idx   int // -1: not a list element
}

// fileFacts is everything the typed walk learned about one file.
type fileFacts struct {
	rel        string
	defs       []def
	clusters   map[string]bool
	routeCfgs  map[string]bool
	routeRefs  []fromTo // route -> cluster
	rdsRefs    []fromTo // listener -> route configuration
	xdsPaths   []string // file-based xDS paths as written
	endpoints  []hostEnt
	listenerOf string
}

type fromTo struct{ from, to string }

type hostEnt struct{ cluster, raw string }

func camel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func mapGetKV(n *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, nil
	}
	alias := camel(key)
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i].Value; k == key || k == alias {
			return n.Content[i], n.Content[i+1]
		}
	}
	return nil, nil
}

func mapGet(n *yaml.Node, key string) *yaml.Node {
	_, v := mapGetKV(n, key)
	return v
}

// nodeAt walks the YAML tree by a field path. The YAML key is the proto field
// name or its lowerCamelCase JSON name; a document node is looked through. It
// returns the node and the line the entity is defined at: a list item's own
// first line, and for a message held in a field (not a list) the line of the
// key that holds it, "where its key is written" as the dialect's engine reads
// a map value.
func nodeAt(root *yaml.Node, p []seg) (*yaml.Node, int) {
	n := root
	line := 0
	for _, s := range p {
		if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
			n = n.Content[0]
		}
		key, val := mapGetKV(n, s.field)
		if val == nil {
			return nil, 0
		}
		n, line = val, key.Line
		if s.idx >= 0 {
			if n.Kind != yaml.SequenceNode || s.idx >= len(n.Content) {
				return nil, 0
			}
			n = n.Content[s.idx]
			line = n.Line
		}
	}
	return n, line
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

// wrapUnknown rewrites every @type object whose message the registry does not
// carry as a google.protobuf.Struct, so a strict decode skips an extension
// instead of failing on it. It returns how many it wrapped.
func wrapUnknown(v any) (any, int) {
	n := 0
	switch t := v.(type) {
	case map[string]any:
		if u, ok := t["@type"].(string); ok {
			if _, err := protoregistry.GlobalTypes.FindMessageByURL(u); err != nil {
				cp := map[string]any{}
				for k, x := range t {
					if k != "@type" {
						cp[k] = x
					}
				}
				return map[string]any{"@type": "type.googleapis.com/google.protobuf.Struct", "value": cp}, 1
			}
		}
		for k, x := range t {
			nx, c := wrapUnknown(x)
			t[k], n = nx, n+c
		}
	case []any:
		for i, x := range t {
			nx, c := wrapUnknown(x)
			t[i], n = nx, n+c
		}
	}
	return v, n
}

// jsonable converts yaml.v3's decoded value into one encoding/json accepts.
func jsonable(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = jsonable(x)
		}
		return t
	case map[any]any:
		m := map[string]any{}
		for k, x := range t {
			m[fmt.Sprint(k)] = jsonable(x)
		}
		return m
	case []any:
		for i, x := range t {
			t[i] = jsonable(x)
		}
		return t
	}
	return v
}

type walker struct {
	root  *yaml.Node
	facts *fileFacts
	// listener is the name of the listener the walk is inside ("" outside).
	listener string
	cluster  string
}

func scalarString(v protoreflect.Value, fd protoreflect.FieldDescriptor) string {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return v.String()
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		return strconv.FormatUint(v.Uint(), 10)
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		return strconv.FormatInt(v.Int(), 10)
	}
	return ""
}

func str(m protoreflect.Message, field string) string {
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(field))
	if fd == nil || !m.Has(fd) {
		return ""
	}
	return scalarString(m.Get(fd), fd)
}

func sub(m protoreflect.Message, field string) protoreflect.Message {
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(field))
	if fd == nil || fd.Message() == nil || fd.IsList() || fd.IsMap() || !m.Has(fd) {
		return nil
	}
	return m.Get(fd).Message()
}

func (w *walker) define(kind string, name string, p []seg) {
	if name == "" {
		return
	}
	n, line := nodeAt(w.root, p)
	if n == nil {
		return // a message with no YAML node (a default the schema filled): not written
	}
	w.facts.defs = append(w.facts.defs, def{Path: w.facts.rel, Name: name, Line: line, End: lastLine(n), Kind: kind})
}

func routeName(r protoreflect.Message) string {
	if n := str(r, "name"); n != "" {
		return n
	}
	m := sub(r, "match")
	if m == nil {
		return ""
	}
	for _, f := range []string{"prefix", "path"} {
		if s := str(m, f); s != "" {
			return s
		}
	}
	if sr := sub(m, "safe_regex"); sr != nil {
		return str(sr, "regex")
	}
	return ""
}

func listenerName(l protoreflect.Message) string {
	if n := str(l, "name"); n != "" {
		return n
	}
	if a := sub(l, "address"); a != nil {
		if sa := sub(a, "socket_address"); sa != nil {
			if p := str(sa, "port_value"); p != "" {
				return p
			}
		}
		if pp := sub(a, "pipe"); pp != nil {
			return str(pp, "path")
		}
	}
	return ""
}

// visit walks m, a message at field path p, in document order.
func (w *walker) visit(m protoreflect.Message, p []seg) {
	full := string(m.Descriptor().FullName())
	savedL, savedC := w.listener, w.cluster
	defer func() { w.listener, w.cluster = savedL, savedC }()
	switch full {
	case "envoy.config.listener.v3.Listener":
		name := listenerName(m)
		w.define("resource", name, p)
		w.listener = name
	case "envoy.config.route.v3.RouteConfiguration":
		name := str(m, "name")
		w.define("section", name, p)
		if name != "" {
			w.facts.routeCfgs[name] = true
		}
	case "envoy.config.route.v3.VirtualHost":
		w.define("selector", str(m, "name"), p)
	case "envoy.config.route.v3.Route":
		name := routeName(m)
		w.define("route", name, p)
		if name != "" {
			if act := sub(m, "route"); act != nil {
				if c := str(act, "cluster"); c != "" {
					w.facts.routeRefs = append(w.facts.routeRefs, fromTo{name, c})
				}
				if wc := sub(act, "weighted_clusters"); wc != nil {
					if fd := wc.Descriptor().Fields().ByName("clusters"); fd != nil {
						l := wc.Get(fd).List()
						for i := 0; i < l.Len(); i++ {
							if c := str(l.Get(i).Message(), "name"); c != "" {
								w.facts.routeRefs = append(w.facts.routeRefs, fromTo{name, c})
							}
						}
					}
				}
				if fd := act.Descriptor().Fields().ByName("request_mirror_policies"); fd != nil {
					l := act.Get(fd).List()
					for i := 0; i < l.Len(); i++ {
						if c := str(l.Get(i).Message(), "cluster"); c != "" {
							w.facts.routeRefs = append(w.facts.routeRefs, fromTo{name, "mirror:" + c})
						}
					}
				}
			}
		}
	case "envoy.config.cluster.v3.Cluster":
		name := str(m, "name")
		w.define("service", name, p)
		if name != "" {
			w.facts.clusters[name] = true
		}
		w.cluster = name
	case "envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager":
		if rds := sub(m, "rds"); rds != nil && w.listener != "" {
			if n := str(rds, "route_config_name"); n != "" {
				w.facts.rdsRefs = append(w.facts.rdsRefs, fromTo{w.listener, n})
			}
		}
	case "envoy.config.core.v3.ConfigSource":
		if s := str(m, "path"); s != "" {
			w.facts.xdsPaths = append(w.facts.xdsPaths, s)
		}
		if pcs := sub(m, "path_config_source"); pcs != nil {
			if s := str(pcs, "path"); s != "" {
				w.facts.xdsPaths = append(w.facts.xdsPaths, s)
			}
		}
	case "envoy.config.core.v3.SocketAddress":
		if w.cluster != "" && len(p) > 0 && p[len(p)-1].field == "socket_address" {
			if a := str(m, "address"); a != "" {
				w.facts.endpoints = append(w.facts.endpoints, hostEnt{w.cluster, a})
			}
		}
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := string(fd.Name())
		switch {
		case fd.IsMap():
			return true
		case fd.Message() == nil:
			return true
		case fd.IsList():
			l := v.List()
			for i := 0; i < l.Len(); i++ {
				w.child(l.Get(i).Message(), append(append([]seg(nil), p...), seg{name, i}))
			}
		default:
			w.child(v.Message(), append(append([]seg(nil), p...), seg{name, -1}))
		}
		return true
	})
}

// child visits a message; a google.protobuf.Any is unpacked when the registry
// knows its type (the fields of the packed message sit beside @type in the
// document, so the path does not grow).
func (w *walker) child(m protoreflect.Message, p []seg) {
	if m.Descriptor().FullName() == "google.protobuf.Any" {
		url := m.Get(m.Descriptor().Fields().ByName("type_url")).String()
		mt, err := protoregistry.GlobalTypes.FindMessageByURL(url)
		if err != nil || mt.Descriptor().FullName() == "google.protobuf.Struct" {
			return
		}
		inner := mt.New().Interface()
		if err := proto.Unmarshal(m.Get(m.Descriptor().Fields().ByName("value")).Bytes(), inner); err != nil {
			return
		}
		w.visit(inner.ProtoReflect(), p)
		return
	}
	w.visit(m, p)
}

type runner struct {
	envoy   string
	timeout time.Duration
}

func (r runner) validate(ctx context.Context, file string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.envoy, "--mode", "validate", "-c", file, "--log-level", "error", "--disable-hot-restart")
	// A file-based xDS path is read relative to where Envoy runs; the dialect
	// resolves it from the referring file's directory, so Envoy runs there.
	cmd.Dir = filepath.Dir(file)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.String() + se.String(), err
}

func firstError(s string) string {
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "[") && strings.Contains(l, "] [") && strings.Contains(l, "info]") {
			continue
		}
		if i := strings.Index(l, "] "); strings.HasPrefix(l, "[") && i > 0 {
			l = l[i+2:]
		}
		if i := strings.Index(l, "error initializing configuration '"); i >= 0 {
			l = l[i+len("error initializing configuration '"):]
			if j := strings.Index(l, "': "); j >= 0 {
				l = l[j+3:]
			}
		}
		if len(l) > 110 {
			l = l[:110]
		}
		return l
	}
	return ""
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	envoy := flag.String("envoy", "envoy", "the envoy binary")
	refsOut := flag.String("refs-out", "", "write the references, cross_references and route targets here")
	routesOut := flag.String("routes-out", "", "write the route_ref layer's truth (route_references, route_unresolved) here")
	filesOut := flag.String("files-out", "", "write the files both instruments accepted, one per line, for lang-oracle -only")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: envoy-oracle -subject <checkout> [-envoy <envoy>] [-refs-out <file>] [-files-out <file>]")
		os.Exit(2)
	}
	if abs, err := filepath.Abs(*subject); err == nil {
		*subject = abs
	}
	r := runner{envoy: *envoy, timeout: 2 * time.Minute}
	vso, err := exec.Command(*envoy, "--version").Output()
	if err != nil {
		fatal(fmt.Errorf("envoy --version: %v", err))
	}
	version := "unknown"
	for _, f := range strings.Fields(string(vso)) {
		if strings.Count(f, "/") >= 2 {
			version = strings.Split(f, "/")[1]
		}
	}

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
			return nil
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

	ctx := context.Background()
	var facts []*fileFacts
	rejected := map[string]int{}
	var accepted []string
	wrapped := 0
	for _, f := range files {
		b, err := readFile(f)
		if err != nil {
			fatal(err)
		}
		if !bytes.Contains(b, []byte("static_resources")) && !bytes.Contains(b, []byte("dynamic_resources")) && !bytes.Contains(b, []byte("type.googleapis.com/envoy")) &&
			!bytes.Contains(b, []byte("staticResources")) && !bytes.Contains(b, []byte("dynamicResources")) {
			continue
		}
		rel, _ := filepath.Rel(*subject, f)
		rel = filepath.ToSlash(rel)
		var node yaml.Node
		if err := yaml.Unmarshal(b, &node); err != nil || len(node.Content) == 0 || node.Content[0].Kind != yaml.MappingNode {
			rejected["not decodable as YAML or JSON"]++
			continue
		}
		var generic any
		if err := yaml.Unmarshal(b, &generic); err != nil {
			rejected["not decodable as YAML or JSON"]++
			continue
		}
		generic = jsonable(generic)
		generic, nw := wrapUnknown(generic)
		js, err := json.Marshal(generic)
		if err != nil {
			rejected["not JSON-expressible"]++
			continue
		}
		isXDS := mapGet(node.Content[0], "resources") != nil && mapGet(node.Content[0], "static_resources") == nil && mapGet(node.Content[0], "dynamic_resources") == nil
		var msg proto.Message
		if isXDS {
			msg = &discoveryv3.DiscoveryResponse{}
		} else {
			msg = &bootstrapv3.Bootstrap{}
		}
		if err := protojson.Unmarshal(js, msg); err != nil {
			rejected["protojson (go-control-plane v3) rejects: "+reasonKey(err.Error())]++
			continue
		}
		if !isXDS {
			if out, err := r.validate(ctx, f); err != nil {
				rejected["envoy --mode validate rejects: "+firstError(out)]++
				continue
			}
		}
		wrapped += nw
		ff := &fileFacts{rel: rel, clusters: map[string]bool{}, routeCfgs: map[string]bool{}}
		root := node.Content[0]
		ff.defs = append(ff.defs, def{Path: rel, Name: path.Base(rel), Line: root.Line, End: lastLine(root), Kind: "package"})
		w := &walker{root: &node, facts: ff}
		w.visit(msg.ProtoReflect(), nil)
		facts = append(facts, ff)
		accepted = append(accepted, rel)
	}

	dep := scanDeployed(*subject)
	out := output{Tool: "envoy", Version: version, Tier: "A", Definitions: []def{}, References: []pair{}}
	refs := output{Tool: "envoy", Version: version, Tier: "A", Definitions: []def{}, References: []pair{}}
	byRel := map[string]*fileFacts{}
	for _, ff := range facts {
		byRel[ff.rel] = ff
	}
	for _, ff := range facts {
		out.Definitions = append(out.Definitions, ff.defs...)
		seen := map[string]bool{}
		for _, rr := range ff.routeRefs {
			to := strings.TrimPrefix(rr.to, "mirror:")
			if !ff.clusters[to] || seen[rr.from+"\x00"+to] {
				continue
			}
			seen[rr.from+"\x00"+to] = true
			refs.References = append(refs.References, pair{ff.rel, rr.from, to})
		}
		for _, rr := range ff.rdsRefs {
			if ff.routeCfgs[rr.to] && !seen["rds"+rr.from+"\x00"+rr.to] {
				seen["rds"+rr.from+"\x00"+rr.to] = true
				refs.References = append(refs.References, pair{ff.rel, rr.from, rr.to})
			}
		}
		// the files this file's xDS paths load (repo-path: a leading / is the repository root)
		var loaded []string
		seenPath := map[string]bool{}
		for _, p := range ff.xdsPaths {
			t := p
			if strings.HasPrefix(t, "/") {
				t = path.Clean(strings.TrimPrefix(t, "/"))
			} else {
				t = path.Clean(path.Join(path.Dir(ff.rel), t))
			}
			if _, ok := byRel[t]; ok && t != ff.rel && !seenPath[t] {
				seenPath[t] = true
				loaded = append(loaded, t)
				refs.CrossReferences = append(refs.CrossReferences, xref{Path: ff.rel, From: path.Base(ff.rel), To: path.Base(t), ToPath: t})
			}
		}
		// names a file's routes and listeners leave to the files it loads
		avail := map[string]string{}
		for _, t := range loaded {
			for c := range byRel[t].clusters {
				if _, dup := avail["c:"+c]; dup {
					avail["c:"+c] = "\x00ambiguous"
				} else {
					avail["c:"+c] = t
				}
			}
			for c := range byRel[t].routeCfgs {
				if _, dup := avail["r:"+c]; dup {
					avail["r:"+c] = "\x00ambiguous"
				} else {
					avail["r:"+c] = t
				}
			}
		}
		for _, rr := range ff.routeRefs {
			to := strings.TrimPrefix(rr.to, "mirror:")
			if ff.clusters[to] {
				continue
			}
			if t, ok := avail["c:"+to]; ok && t != "\x00ambiguous" {
				refs.CrossReferences = append(refs.CrossReferences, xref{Path: ff.rel, From: rr.from, To: to, ToPath: t})
			}
		}
		for _, rr := range ff.rdsRefs {
			if ff.routeCfgs[rr.to] {
				continue
			}
			if t, ok := avail["r:"+rr.to]; ok && t != "\x00ambiguous" {
				refs.CrossReferences = append(refs.CrossReferences, xref{Path: ff.rel, From: rr.from, To: rr.to, ToPath: t})
			}
		}
		seenHost := map[string]bool{}
		for _, h := range ff.endpoints {
			if seenHost[h.cluster+"\x00"+h.raw] {
				continue
			}
			seenHost[h.cluster+"\x00"+h.raw] = true
			if tp, to, reason := dep.classify(h.raw); reason == "" {
				refs.RouteReferences = append(refs.RouteReferences, routeRef{Path: ff.rel, From: h.cluster, ToPath: tp, To: to, Raw: h.raw})
			} else {
				refs.RouteUnresolved = append(refs.RouteUnresolved, routeUnresolved{Path: ff.rel, From: h.cluster, Raw: h.raw, Reason: reason})
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
		sort.Strings(accepted)
		if err := writeFile(*filesOut, []byte(strings.Join(accepted, "\n")+"\n")); err != nil {
			fatal(err)
		}
	}
	fmt.Fprintf(os.Stderr, "envoy-oracle: %d files accepted, %d definitions, %d references, %d cross_references, %d route_references, %d route_unresolved, %d extension messages wrapped as Struct\n",
		len(accepted), len(out.Definitions), len(refs.References), len(refs.CrossReferences), len(refs.RouteReferences), len(refs.RouteUnresolved), wrapped)
	var reasons []string
	for k, n := range rejected {
		reasons = append(reasons, fmt.Sprintf("  %5d  %s", n, k))
	}
	sort.Strings(reasons)
	for _, l := range reasons {
		fmt.Fprintln(os.Stderr, l)
	}
}

// reasonKey trims an error to the part that repeats across files.
func reasonKey(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if i := strings.Index(s, "proto:"); i >= 0 {
		s = strings.TrimSpace(s[i+6:])
	}
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "envoy-oracle:", err)
	os.Exit(1)
}
