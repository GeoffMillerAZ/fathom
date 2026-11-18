package deps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"strings"
)

type tsjsRead struct {
	ctx            context.Context
	repo           repositoryFS
	out            *result
	config         tsjsConfig
	sourceManifest tsjsManifest
}

type tsjsMode uint8

const (
	tsjsStatic tsjsMode = iota
	tsjsImport
	tsjsRequire
)

type tsjsConfig struct {
	resolution, module string
	dir, base          string
	paths              map[string][]string
	unprovable         bool
	json               bool
	automaticJSX       bool
}

func (g *tsjsRead) unknown(code UnknownCode, raw string, line int, by ...DecidableBy) {
	g.out.addUnknown(Unknown{Code: code, Raw: raw, Line: line, DecidableBy: by})
}

func (g *tsjsRead) edge(kind Kind, raw, to string, resolution Resolution, version string, line int) {
	g.out.addEdge(Edge{Kind: kind, Raw: raw, To: to, Resolution: resolution, Version: version, Line: line, Provenance: ProvenanceBuiltin})
}

// A failed higher-priority probe (including a symlink) must never be
// mistaken for absence and followed by a successful lower-priority probe.
func (g *tsjsRead) probe(name string, dir bool) (bool, UnknownCode) {
	if name == "." && dir {
		return true, ""
	}
	if !validPath(name) {
		return false, UnknownUnsupportedConstruct
	}
	info, err := g.repo.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, ""
	}
	if err != nil {
		return false, UnknownUnsupportedConstruct
	}
	if dir {
		return info.IsDir(), ""
	}
	return info.Mode().IsRegular(), ""
}

func (g *tsjsRead) metadata(name string) ([]byte, bool, UnknownCode) {
	exists, code := g.probe(name, false)
	if !exists || code != "" {
		return nil, exists, code
	}
	g.edge(KindConfigRef, name, name, ResolvedFile, "", 0)
	data, code := readSource(g.repo, name)
	return data, true, code
}

func (g *tsjsRead) loadConfig() {
	g.config = tsjsConfig{resolution: "node16", module: "node16"}
	for dir := path.Dir(g.out.Path); ; dir = path.Dir(dir) {
		if g.ctx.Err() != nil {
			return
		}
		for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
			file := path.Join(dir, name)
			data, exists, code := g.metadata(file)
			if code != "" {
				g.unknown(code, file, 0, DecideConfigFile)
				g.config.unprovable = true
				return
			}
			if exists {
				g.parseConfig(dir, file, data)
				return
			}
		}
		if dir == "." {
			return
		}
	}
}

func (g *tsjsRead) parseConfig(dir, file string, data []byte) {
	var cfg struct {
		Extends, References json.RawMessage
		CompilerOptions     struct {
			Module, ModuleResolution, BaseURL, JSX string
			Paths                                  map[string][]string
			ResolveJSONModule                      *bool
		}
	}
	clean := tsjsJSONComments(data)
	var raw struct{ CompilerOptions map[string]json.RawMessage }
	if !bytes.HasPrefix(bytes.TrimSpace(clean), []byte("{")) ||
		json.Unmarshal(clean, &cfg) != nil || json.Unmarshal(clean, &raw) != nil {
		g.config.unprovable = true
		g.unknown(UnknownParseError, file, 0, DecideConfigFile)
		return
	}
	opts := cfg.CompilerOptions
	c := tsjsConfig{dir: dir, paths: opts.Paths, module: strings.ToLower(opts.Module)}
	c.resolution = strings.ToLower(opts.ModuleResolution)
	if c.resolution == "" {
		switch c.module {
		case "node16", "node18", "node20":
			c.resolution = "node16"
		case "nodenext":
			c.resolution = "nodenext"
		case "", "commonjs", "esnext", "preserve", "es2022", "es2020", "es2015", "es6":
			c.resolution = "bundler"
		}
	}
	switch c.resolution {
	case "node", "node10":
		c.resolution = "node10"
	case "node16", "nodenext", "bundler":
	default:
		c.unprovable = true
	}
	if opts.BaseURL != "" {
		c.base = tsjsJoin(dir, opts.BaseURL)
		c.unprovable = c.unprovable || c.base == ""
	}
	c.json = c.resolution == "bundler" || c.resolution == "nodenext" || c.module == "node20"
	if opts.ResolveJSONModule != nil {
		c.json = *opts.ResolveJSONModule
	}
	c.automaticJSX = opts.JSX == "react-jsx" || opts.JSX == "react-jsxdev"
	for key, value := range raw.CompilerOptions {
		switch strings.ToLower(key) {
		case "rootdirs", "modulesuffixes", "customconditions", "resolvemoduleimportsatpackagejson",
			"resolvepackagejsonexports", "resolvepackagejsonimports", "allowarbitraryextensions",
			"jsximportsource", "typeroots", "pathsbasepath", "moduledetection":
			c.unprovable = true
		case "noresolve", "importhelpers":
			c.unprovable = c.unprovable || string(value) == "true"
		case "types":
			c.unprovable = c.unprovable || string(value) != "[]" && string(value) != "null"
		}
	}
	if len(cfg.Extends) != 0 || len(cfg.References) != 0 && string(cfg.References) != "[]" ||
		len(c.paths) > maxPackageEntries {
		c.unprovable = true
	}
	if c.unprovable {
		g.unknown(UnknownUnsupportedConstruct, file, 0, DecideToolchain, DecideConfigFile)
	}
	g.config = c
}

// Remove JSONC comments and optional trailing commas outside quoted strings.
// Keep line breaks and replace comments with spaces so adjacent tokens never
// become a different valid token.
func tsjsJSONComments(data []byte) []byte {
	b := bytes.Clone(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	for i := 0; i < len(b); i++ {
		if b[i] == '"' {
			i = tsjsJSONStringEnd(b, i)
			continue
		}
		if b[i] != '/' || i+1 >= len(b) {
			continue
		}
		switch b[i+1] {
		case '/':
			for ; i < len(b) && b[i] != '\n' && b[i] != '\r'; i++ {
				b[i] = ' '
			}
		case '*':
			end := bytes.Index(b[i+2:], []byte("*/"))
			if end < 0 {
				return nil
			}
			end += i + 4
			for ; i < end; i++ {
				if b[i] != '\n' && b[i] != '\r' {
					b[i] = ' '
				}
			}
			i--
		}
	}
	for i := 0; i < len(b); i++ {
		switch b[i] {
		case '"':
			i = tsjsJSONStringEnd(b, i)
		case ',':
			for _, c := range b[i+1:] {
				if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
					continue
				}
				if c == '}' || c == ']' {
					b[i] = ' '
				}
				break
			}
		}
	}
	return b
}

func tsjsJSONStringEnd(b []byte, i int) int {
	for i++; i < len(b); i++ {
		if b[i] == '\\' {
			i++
		} else if b[i] == '"' {
			break
		}
	}
	return i
}

// Join confines candidates before they are presented to the repository view.
func tsjsJoin(dir, operand string) string {
	if operand == "" || strings.HasPrefix(operand, "/") || strings.ContainsAny(operand, "\\:%?#") {
		return ""
	}
	name := path.Join(dir, operand)
	if name == "." || validPath(name) {
		return name
	}
	return ""
}

func (c tsjsConfig) requireMode(req tsjsMode, file, packageType string, esm bool) bool {
	switch req {
	case tsjsRequire:
		return true
	case tsjsImport:
		return false
	}
	switch path.Ext(file) {
	case ".mts", ".mjs":
		return false
	case ".cts", ".cjs":
		return true
	}
	if c.resolution == "bundler" {
		return c.module == "commonjs" || !esm && packageType != "module"
	}
	return packageType != "module"
}
