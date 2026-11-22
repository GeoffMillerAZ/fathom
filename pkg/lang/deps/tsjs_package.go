package deps

import (
	"bytes"
	"encoding/json"
	"path"
	"regexp"
	"strings"
)

type tsjsProperty struct {
	key   string
	value json.RawMessage
}

func tsjsObject(raw json.RawMessage) ([]tsjsProperty, bool) {
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return nil, false
	}
	var props []tsjsProperty
	seen := make(map[string]bool)
	for d.More() {
		key, err := d.Token()
		k, ok := key.(string)
		if err != nil || !ok || seen[k] || len(props) == maxPackageEntries {
			return nil, false
		}
		seen[k] = true
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, false
		}
		props = append(props, tsjsProperty{k, value})
	}
	return props, true
}

func (g *tsjsRead) packageExport(dir string, raw json.RawMessage, sub string, require bool) (string, UnknownCode) {
	wanted := "."
	if sub != "" {
		wanted = "./" + sub
	}
	props, object := tsjsObject(raw)
	if !object && len(raw) > 0 && raw[0] == '{' {
		return "", UnknownUnsupportedConstruct
	}
	subpaths, conditions := false, false
	for _, p := range props {
		if strings.HasPrefix(p.key, ".") {
			subpaths = true
		} else {
			conditions = true
		}
	}
	switch {
	case subpaths && conditions:
		return "", UnknownUnsupportedConstruct
	case subpaths:
		var pattern json.RawMessage
		wildcard := ""
		bestPrefix, bestLen := -1, -1
		for _, p := range props {
			if p.key == wanted {
				return g.exportBranch(dir, p.value, "", require, 0)
			}
			pre, post, has := strings.Cut(p.key, "*")
			if !has || len(wanted) <= len(pre)+len(post) ||
				!strings.HasPrefix(wanted, pre) || !strings.HasSuffix(wanted, post) {
				continue
			}
			if strings.Contains(post, "*") {
				return "", UnknownUnsupportedConstruct
			}
			if len(pre) > bestPrefix || len(pre) == bestPrefix && len(p.key) > bestLen {
				pattern, wildcard = p.value, wanted[len(pre):len(wanted)-len(post)]
				bestPrefix, bestLen = len(pre), len(p.key)
			}
		}
		if pattern != nil {
			return g.exportBranch(dir, pattern, wildcard, require, 0)
		}
		return "", UnknownUnresolvedAlias
	case sub == "":
		return g.exportBranch(dir, raw, "", require, 0)
	default:
		return "", UnknownUnresolvedAlias
	}
}

func (g *tsjsRead) exportBranch(dir string, raw json.RawMessage, wildcard string, require bool, depth int) (string, UnknownCode) {
	if depth >= 16 {
		return "", UnknownReadLimit
	}
	if string(raw) == "null" {
		return "", ""
	}
	var target string
	if json.Unmarshal(raw, &target) == nil {
		if !strings.HasPrefix(target, "./") || wildcard == "" && strings.Contains(target, "*") {
			return "", UnknownUnsupportedConstruct
		}
		operand := strings.ReplaceAll(strings.TrimPrefix(target, "./"), "*", wildcard)
		if !validPath(operand) || strings.HasSuffix(target, "/") {
			return "", UnknownUnsupportedConstruct
		}
		for _, part := range strings.Split(operand, "/") {
			if part == "node_modules" {
				return "", UnknownUnsupportedConstruct
			}
		}
		name := tsjsJoin(dir, operand)
		return g.localTarget(name, false, false)
	}
	props, ok := tsjsObject(raw)
	if !ok {
		return "", UnknownUnsupportedConstruct
	}
	for _, p := range props {
		switch {
		case p.key == "default", p.key == "types", p.key == "require" && require, p.key == "import" && !require,
			p.key == "node" && g.config.resolution != "bundler":
			if found, code := g.exportBranch(dir, p.value, wildcard, require, depth+1); found != "" || code != "" {
				return found, code
			}
		case strings.HasPrefix(p.key, "types@"):
			// Version-qualified compiler conditions need a compiler version.
			return "", UnknownUnsupportedConstruct
		}
	}
	return "", ""
}

// Node built-ins that accept a bare specifier. Prefix-only modules are
// handled by the node: branch without treating them as npm aliases.
var tsjsBareNodeBuiltins = func() map[string]bool {
	names := strings.Fields(`
assert assert/strict async_hooks buffer child_process cluster console constants
crypto dgram diagnostics_channel dns dns/promises domain events fs fs/promises
http http2 https inspector inspector/promises module net os path path/posix
path/win32 perf_hooks process punycode querystring readline readline/promises
repl stream stream/consumers stream/promises stream/web string_decoder sys
timers timers/promises tls trace_events tty url util util/types v8 vm wasi
worker_threads zlib
`)
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[name] = true
	}
	return out
}()

func (g *tsjsRead) external(r tsjsRequest, pkg, sub string, require bool) {
	types := strings.TrimPrefix(strings.Replace(pkg, "/", "__", 1), "@")
	for dir := path.Dir(g.out.Path); ; dir = path.Dir(dir) {
		if exists, code := g.probe(path.Join(dir, "node_modules", "@types", types), true); exists || code != "" {
			// A visible declaration package can change the selected file even
			// when the corresponding runtime package has an exact version.
			g.finishTarget(r, "", UnknownUnsupportedConstruct)
			return
		}
		base := path.Join(dir, "node_modules", pkg)
		if exists, code := g.probe(base, true); exists || code != "" {
			if code != "" {
				g.finishTarget(r, "", code)
				return
			}
			p, code := g.manifest(base)
			if code != "" {
				g.finishTarget(r, "", code)
				return
			}
			if len(p.raw.Exports) != 0 && g.config.resolution != "node10" {
				to, code := g.packageExport(base, p.raw.Exports, sub, require)
				g.finishTarget(r, to, code)
				return
			}
			if len(p.raw.TypesVersions) != 0 {
				g.finishTarget(r, "", UnknownUnsupportedConstruct)
				return
			}
			to, code := g.localTarget(path.Join(base, sub), true, true)
			g.finishTarget(r, to, code)
			return
		}
		if dir == "." {
			break
		}
	}
	if g.out.Dialect == "typescript" {
		if code := g.provePackageAbsent(pkg); code != "" {
			g.finishTarget(r, "", code)
			return
		}
	}
	pin := g.packagePin(pkg)
	g.edge(KindImport, r.raw, "npm:"+r.raw, ResolvedExternal, pin, r.line)
	if sub != "" && pin != "" {
		g.unknown(UnknownUnresolvedAlias, r.raw, r.line, DecideToolchain, DecideConfigFile)
	}
}

var tsjsExactVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[A-Za-z0-9.-]+)?(\+[A-Za-z0-9.-]+)?$`)

func (g *tsjsRead) packagePin(pkg string) string {
	m := g.sourceManifest
	if m.file == "" {
		return ""
	}
	spec := tsjsDependencySpec(m.raw, pkg)
	if spec == "" || strings.ContainsAny(spec, ":/\\") {
		return ""
	}
	file := path.Join(path.Dir(m.file), "package-lock.json")
	data, exists, code := g.metadata(file)
	if code != "" {
		g.unknown(code, file, 0, DecideLockfile)
		return ""
	}
	if !exists {
		if tsjsExactVersion.MatchString(spec) {
			return spec
		}
		return ""
	}
	var lock struct {
		LockfileVersion int
		Packages        map[string]struct {
			tsjsPackageJSON
			Version, Resolved string
			Link              bool
		}
	}
	if json.Unmarshal(data, &lock) != nil {
		g.unknown(UnknownParseError, file, 0, DecideLockfile)
		return ""
	}
	root, hasRoot := lock.Packages[""]
	pin, hasPin := lock.Packages["node_modules/"+pkg]
	if lock.LockfileVersion != 2 && lock.LockfileVersion != 3 || !hasRoot ||
		tsjsDependencySpec(root.tsjsPackageJSON, pkg) != spec || !hasPin || pin.Link ||
		pin.Name != "" && pin.Name != pkg || !tsjsExactVersion.MatchString(pin.Version) ||
		pin.Resolved != "" && !strings.HasPrefix(pin.Resolved, "https://") ||
		tsjsExactVersion.MatchString(spec) && spec != pin.Version {
		return ""
	}
	return pin.Version
}

func tsjsDependencySpec(p tsjsPackageJSON, pkg string) string {
	for _, source := range []map[string]string{p.OptionalDependencies, p.Dependencies, p.DevDependencies, p.PeerDependencies} {
		if value := source[pkg]; value != "" {
			return value
		}
	}
	return ""
}
