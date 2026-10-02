package fusion

// launchroutes.go — LB-F100 (wave px): a proxy configured only by its launch
// arguments, with no config file of its own, has its routes in the
// Kubernetes container or compose service that runs it. mongobetween
// (coinbase/mongobetween, Apache-2.0; usage `mongobetween [OPTIONS]
// address1=uri1 [address2=uri2] ...`, README at ea5a87ec253f) listens on
// each positional address and proxies it to the MongoDB deployment its URI
// names: every host of a mongodb:// URI is one route-host reference from the
// workload (the object or compose service that launches it — a listen
// address is no stored entity), a mongodb+srv:// URI is one (an SRV seed
// list, external), and a URI the arguments do not fix ($(MONGODB_URI), the
// public charts' shape) is one that resolves as a variable. The -dynamic
// file's RedirectTo (a listen address to another) is not read: neither end
// is an entity any extractor stores.

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// mongobetweenValueFlags are mongobetween's flags that take a value (its
// README usage); the rest are booleans.
var mongobetweenValueFlags = map[string]bool{
	"loglevel": true, "network": true, "password": true, "statsd": true, "username": true, "dynamic": true,
}

// launchArg is one launch argument and the line it is written on.
type launchArg struct {
	val  string
	line int
}

// launchRouteRefs returns the route-host references in a Kubernetes
// manifest's (ToolK8sManifest, ToolK8sCRD) or a compose file's (ToolCompose)
// launch arguments. Malformed YAML yields none.
func launchRouteRefs(tk ToolKind, content []byte) []extractor.DocCrossFileRef {
	if !bytes.Contains(content, []byte("mongobetween")) {
		return nil
	}
	var out []extractor.DocCrossFileRef
	dec := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			break
		}
		if len(doc.Content) == 0 {
			continue
		}
		root := doc.Content[0]
		switch tk {
		case ToolK8sManifest, ToolK8sCRD:
			out = append(out, k8sLaunchRoutes(root)...)
		case ToolCompose:
			out = append(out, composeLaunchRoutes(root)...)
		}
	}
	return out
}

func k8sLaunchRoutes(root *yaml.Node) []extractor.DocCrossFileRef {
	kind, name := yScalar(yGet(root, "kind")), yScalar(yPath(root, "metadata", "name"))
	if kind == "" || name == "" {
		return nil
	}
	entity := kind + "/" + name
	if ns := yScalar(yPath(root, "metadata", "namespace")); ns != "" {
		entity = kind + "/" + ns + "/" + name
	}
	var out []extractor.DocCrossFileRef
	for _, pod := range []*yaml.Node{yGet(root, "spec"), yPath(root, "spec", "template", "spec"), yPath(root, "spec", "jobTemplate", "spec", "template", "spec")} {
		for _, list := range []string{"containers", "initContainers"} {
			for _, c := range ySeq(yGet(pod, list)) {
				cmd, args := yArgs(yGet(c, "command")), yArgs(yGet(c, "args"))
				if !launchesMongobetween(yScalar(yGet(c, "image")), cmd) {
					continue
				}
				out = append(out, mongobetweenRoutes(entity, append(dropBinary(cmd), args...))...)
			}
		}
	}
	return out
}

func composeLaunchRoutes(root *yaml.Node) []extractor.DocCrossFileRef {
	svcs := yGet(root, "services")
	if svcs == nil || svcs.Kind != yaml.MappingNode {
		return nil
	}
	var out []extractor.DocCrossFileRef
	for i := 0; i+1 < len(svcs.Content); i += 2 {
		key, svc := svcs.Content[i].Value, svcs.Content[i+1]
		entry, cmd := yArgs(yGet(svc, "entrypoint")), yArgs(yGet(svc, "command"))
		if key == "" || !launchesMongobetween(yScalar(yGet(svc, "image")), append(append([]launchArg(nil), entry...), cmd...)) {
			continue
		}
		out = append(out, mongobetweenRoutes(key, append(dropBinary(entry), dropBinary(cmd)...))...)
	}
	return out
}

// launchesMongobetween: the image's repository, or the command's binary,
// is mongobetween.
func launchesMongobetween(image string, cmd []launchArg) bool {
	repo := image
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		repo = repo[i+1:]
	}
	if i := strings.IndexAny(repo, ":@"); i >= 0 {
		repo = repo[:i]
	}
	if strings.Contains(repo, "mongobetween") {
		return true
	}
	return len(cmd) > 0 && isMongobetweenBinary(cmd[0].val)
}

func isMongobetweenBinary(s string) bool {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s == "mongobetween"
}

// dropBinary drops a leading mongobetween binary from a command.
func dropBinary(cmd []launchArg) []launchArg {
	if len(cmd) > 0 && isMongobetweenBinary(cmd[0].val) {
		return cmd[1:]
	}
	return cmd
}

// mongobetweenRoutes reads mongobetween's arguments as Go's flag package
// does (flags first, a value flag consuming the next argument unless written
// -flag=value) and returns one reference per target of each positional
// address=uri.
func mongobetweenRoutes(entity string, args []launchArg) []extractor.DocCrossFileRef {
	var out []extractor.DocCrossFileRef
	positional := false
	for i := 0; i < len(args); i++ {
		a := args[i].val
		if !positional && strings.HasPrefix(a, "-") {
			if a == "--" {
				positional = true
				continue
			}
			f := strings.TrimLeft(a, "-")
			if !strings.Contains(f, "=") && mongobetweenValueFlags[f] {
				i++
			}
			continue
		}
		positional = true
		eq := strings.Index(a, "=")
		if eq <= 0 {
			continue
		}
		for _, raw := range mongoTargets(a[eq+1:]) {
			out = append(out, extractor.DocCrossFileRef{From: entity, Edge: RouteRefEdge, Strategy: RouteHostStrategy, Raw: raw, Target: raw, Line: args[i].line})
		}
	}
	return out
}

// mongoTargets is a MongoDB connection string's targets: each host[:port]
// of a mongodb:// seed list as written, or the whole string for a
// mongodb+srv:// URI or one the arguments do not fix.
func mongoTargets(uri string) []string {
	lower := strings.ToLower(uri)
	if !strings.HasPrefix(lower, "mongodb://") {
		if strings.HasPrefix(lower, "mongodb+srv://") || strings.Contains(uri, "$") {
			return []string{uri}
		}
		return nil
	}
	rest := uri[len("mongodb://"):]
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	var out []string
	for _, h := range strings.Split(rest, ",") {
		if h = strings.TrimSpace(h); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// yArgs is a command written as a list of scalars or as one string (split
// on whitespace, a quoted run kept whole).
func yArgs(n *yaml.Node) []launchArg {
	switch {
	case n == nil:
		return nil
	case n.Kind == yaml.SequenceNode:
		var out []launchArg
		for _, it := range n.Content {
			if it.Kind == yaml.ScalarNode {
				out = append(out, launchArg{it.Value, it.Line})
			}
		}
		return out
	case n.Kind == yaml.ScalarNode:
		var out []launchArg
		for _, w := range shellWords(n.Value) {
			out = append(out, launchArg{w, n.Line})
		}
		return out
	}
	return nil
}

// shellWords splits s on unquoted whitespace, dropping the quotes.
func shellWords(s string) []string {
	var out []string
	var b strings.Builder
	var quote rune
	in := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, in = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if in {
				out = append(out, b.String())
				b.Reset()
				in = false
			}
		default:
			b.WriteRune(r)
			in = true
		}
	}
	if in {
		out = append(out, b.String())
	}
	return out
}

func yGet(n *yaml.Node, key string) *yaml.Node {
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

func yPath(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		n = yGet(n, k)
	}
	return n
}

func yScalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

func ySeq(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}
