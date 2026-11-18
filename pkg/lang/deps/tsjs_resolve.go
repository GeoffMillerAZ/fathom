package deps

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"
)

type tsjsManifest struct {
	file, name, kind, main, types string
	raw                           tsjsPackageJSON
}

type tsjsPackageJSON struct {
	Name, Type, Main, Types, Typings                                      string
	Exports, Imports, TypesVersions                                       json.RawMessage
	Dependencies, DevDependencies, OptionalDependencies, PeerDependencies map[string]string
}

func (g *tsjsRead) manifest(dir string) (tsjsManifest, UnknownCode) {
	file := path.Join(dir, "package.json")
	data, exists, code := g.metadata(file)
	if !exists || code != "" {
		return tsjsManifest{}, code
	}
	var cfg tsjsPackageJSON
	if len(data) == 0 || json.Unmarshal(data, &cfg) != nil || strings.TrimSpace(string(data))[0] != '{' {
		return tsjsManifest{}, ParseError
	}
	dts := cfg.Types
	if dts == "" {
		dts = cfg.Typings
	}
	if cfg.Type != "" && cfg.Type != "module" && cfg.Type != "commonjs" {
		return tsjsManifest{}, UnsupportedConstruct
	}
	return tsjsManifest{file, cfg.Name, cfg.Type, cfg.Main, dts, cfg}, ""
}

func (g *tsjsRead) nearestManifest() tsjsManifest {
	for dir := path.Dir(g.out.Path); ; dir = path.Dir(dir) {
		if g.ctx.Err() != nil {
			return tsjsManifest{}
		}
		p, code := g.manifest(dir)
		if code != "" {
			g.unknown(code, path.Join(dir, "package.json"), 1, ConfigFile)
			g.config.unprovable = true
			return tsjsManifest{}
		}
		if p.file != "" || dir == "." {
			return p
		}
	}
}

func (g *tsjsRead) resolve(r tsjsRequest, esm bool) {
	raw := r.raw
	require := g.config.requireMode(r.mode, g.out.Path, g.sourceManifest.kind, esm)
	extensions := require || g.config.resolution == "bundler" || g.config.resolution == "node10"
	if raw == "." || raw == ".." || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
		base := tsjsJoin(path.Dir(g.out.Path), raw)
		if base == "" {
			g.unknown(UnsupportedConstruct, raw, r.line, ConfigFile, Toolchain)
			return
		}
		to, code := g.localTarget(base, extensions, true)
		g.finishTarget(r, to, code)
		return
	}
	if strings.HasPrefix(raw, "#") {
		g.unknown(UnsupportedConstruct, raw, r.line, ConfigFile, Toolchain)
		return
	}
	if strings.HasPrefix(raw, "node:") && validExternal(raw) {
		g.edge(Import, raw, raw, External, "", r.line)
		return
	}
	pkg, sub := tsjsPackageName(raw)
	if pkg == "" {
		g.unknown(UnsupportedConstruct, raw, r.line, Toolchain)
		return
	}
	if g.config.unprovable {
		g.unknown(UnresolvedAlias, raw, r.line, Toolchain, ConfigFile)
		return
	}
	if to, code := g.alias(raw, extensions); to != "" || code != "" {
		g.finishTarget(r, to, code)
		return
	}
	if pkg == g.sourceManifest.name && len(g.sourceManifest.raw.Exports) != 0 && g.config.resolution != "node10" {
		to, code := g.packageExport(path.Dir(g.sourceManifest.file), g.sourceManifest.raw.Exports, sub, require)
		g.finishTarget(r, to, code)
		return
	}
	g.external(r, pkg, sub, require)
}

func (g *tsjsRead) finishTarget(r tsjsRequest, to string, code UnknownCode) {
	if to != "" {
		g.edge(Import, r.raw, to, File, "", r.line)
		return
	}
	if code == "" {
		code = UnresolvedAlias
	}
	g.unknown(code, r.raw, r.line, Toolchain, ConfigFile)
}

func (g *tsjsRead) localTarget(base string, extensionless, directory bool) (string, UnknownCode) {
	stem, suffixes := tsjsSuffixes(base, g.config.json)
	if !strings.HasSuffix(base, "/") {
		for _, suffix := range suffixes {
			name := stem + suffix
			if ok, code := g.probe(name, false); ok || code != "" {
				return tsjsProbeResult(name, ok, code)
			}
		}
	}
	if !extensionless {
		return "", ""
	}
	for _, ext := range tsjsStandardExtensions {
		name := base + ext
		if ok, code := g.probe(name, false); ok || code != "" {
			return tsjsProbeResult(name, ok, code)
		}
	}
	if !directory {
		return "", ""
	}
	isDir, code := g.probe(base, true)
	if !isDir || code != "" {
		return "", code
	}
	p, code := g.manifest(base)
	if code != "" || len(p.raw.TypesVersions) != 0 {
		if code == "" {
			code = UnsupportedConstruct
		}
		return "", code
	}
	entry := p.types
	if entry == "" {
		entry = p.main
	}
	if entry != "" {
		target := tsjsJoin(base, entry)
		if target == "" || !tsjsWithin(base, target) {
			return "", UnsupportedConstruct
		}
		// TypeScript may also apply an index fallback to a package entry
		// pointing to a directory. That ambiguous shape needs the compiler.
		if to, code := g.localTarget(target, true, false); to != "" || code != "" {
			return to, code
		}
		if isDir, code := g.probe(target, true); isDir || code != "" {
			return "", UnsupportedConstruct
		}
	}
	return g.localTarget(path.Join(base, "index"), true, false)
}

func tsjsProbeResult(name string, exists bool, code UnknownCode) (string, UnknownCode) {
	if exists {
		return name, ""
	}
	return "", code
}

var tsjsStandardExtensions = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx"}

func tsjsSuffixes(name string, jsonAllowed bool) (string, []string) {
	ext := path.Ext(name)
	for _, dts := range []string{".d.ts", ".d.mts", ".d.cts"} {
		if strings.HasSuffix(name, dts) {
			ext = dts
			break
		}
	}
	stem := strings.TrimSuffix(name, ext)
	switch ext {
	case ".ts", ".d.ts", ".js":
		return stem, tsjsStandardExtensions
	case ".tsx", ".jsx":
		return stem, []string{".tsx", ".ts", ".d.ts", ".jsx", ".js"}
	case ".mts", ".d.mts", ".mjs":
		return stem, []string{".mts", ".d.mts", ".mjs"}
	case ".cts", ".d.cts", ".cjs":
		return stem, []string{".cts", ".d.cts", ".cjs"}
	case ".json":
		if jsonAllowed {
			return stem, []string{".d.json.ts", ".json"}
		}
		return stem, []string{".d.json.ts"}
	case "":
		return name, nil
	default:
		return stem, []string{".d" + ext + ".ts"}
	}
}

func (g *tsjsRead) alias(raw string, extensions bool) (string, UnknownCode) {
	c := g.config
	match, replacement := "", ""
	prefixLen, ambiguous := -1, false
	if _, exact := c.paths[raw]; exact && !strings.Contains(raw, "*") {
		match = raw
	} else {
		for key := range c.paths {
			prefix, suffix, ok := strings.Cut(key, "*")
			if !ok || strings.Contains(suffix, "*") || !strings.HasPrefix(raw, prefix) ||
				!strings.HasSuffix(raw, suffix) || len(raw) < len(prefix)+len(suffix) {
				continue
			}
			switch {
			case len(prefix) > prefixLen:
				match, prefixLen, ambiguous = key, len(prefix), false
				replacement = raw[len(prefix) : len(raw)-len(suffix)]
			case len(prefix) == prefixLen:
				ambiguous = true
			}
		}
	}
	if ambiguous {
		return "", UnresolvedAlias
	}
	root := c.dir
	if c.base != "" {
		root = c.base
	}
	if match != "" {
		targets := c.paths[match]
		if len(targets) == 0 || len(targets) > maxPackageEntries {
			return "", UnsupportedConstruct
		}
		for _, candidate := range targets {
			if strings.Count(candidate, "*") > 1 || c.base == "" && !strings.HasPrefix(candidate, "./") && !strings.HasPrefix(candidate, "../") {
				return "", UnsupportedConstruct
			}
			base := tsjsJoin(root, strings.Replace(candidate, "*", replacement, 1))
			if base == "" {
				return "", UnsupportedConstruct
			}
			if to, code := g.localTarget(base, extensions, true); to != "" || code != "" {
				return to, code
			}
		}
	}
	if c.base != "" {
		if to, code := g.localTarget(tsjsJoin(c.base, raw), extensions, true); to != "" || code != "" {
			return to, code
		}
	}
	if match != "" {
		return "", UnresolvedAlias
	}
	return "", ""
}

var tsjsPackageToken = regexp.MustCompile(`^(?:@[A-Za-z0-9_~-][A-Za-z0-9._~-]*/)?[A-Za-z0-9_~-][A-Za-z0-9._~-]*$`)

func tsjsPackageName(raw string) (name, sub string) {
	if !validPath(raw) || strings.Contains(raw, " ") {
		return "", ""
	}
	name, sub, _ = strings.Cut(raw, "/")
	if strings.HasPrefix(raw, "@") {
		scope := name
		name, sub, _ = strings.Cut(sub, "/")
		name = scope + "/" + name
	}
	if !tsjsPackageToken.MatchString(name) {
		return "", ""
	}
	return name, sub
}

func tsjsWithin(dir, file string) bool {
	return dir == "." && validPath(file) || strings.HasPrefix(file, dir+"/")
}
