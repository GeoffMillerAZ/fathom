package langroute

// door.go — which dialect read a file, for the language library's door
// (ADR-092, LL-11). The dispatch picks an extractor and a tool kind; neither
// names the dialect on every path, so DialectOf reads the signals in order of
// specificity and returns "" when none names one.

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// The ids of the generic readers: the JSON and TOML extractors that read a
// file no dialect's own extractor claimed. They are readers of an extension,
// not dialects with a manifest, so the language library names them so that a
// file they read has a Dialect.
const (
	GenericJSON = extractor.GenericJSONDialect
	GenericTOML = extractor.GenericTOMLDialect
)

// Templ is the id of the a-h/templ reader, a line scanner of the .templ
// extension with no manifest dialect, named for the same reason.
const Templ = "templ"

// toolKindDialect names the dialect behind a tool kind whose spelling is not
// the dialect's id.
var toolKindDialect = map[ToolKind]string{
	ToolK8sManifest:      "kubernetes",
	ToolK8sCRD:           "kubernetes",
	ToolCompose:          "docker-compose",
	ToolHelmChart:        "helm",
	ToolHelmValues:       "helm",
	ToolHelmTemplateYAML: "helm",
	ToolGHA:              "github-actions",
	ToolCUEModule:        "cue",
	ToolCUEPlain:         "cue",
}

var (
	dialectIndexOnce sync.Once
	dialectIDs       map[string]bool   // every manifest dialect id
	keyOwner         map[string]string // extension or basename -> the one dialect id that claims it ("" when shared)
	kindOwner        map[string]string // source kind -> the one dialect id that emits it ("" when shared)
)

func buildDialectIndex() {
	dialectIDs = map[string]bool{}
	keyOwner = map[string]string{}
	kindOwner = map[string]string{}
	ds, err := extractor.LoadDialects()
	if err != nil {
		return
	}
	claim := func(m map[string]string, k, id string) {
		if prev, ok := m[k]; ok && prev != id {
			m[k] = ""
			return
		}
		m[k] = id
	}
	for _, d := range ds {
		dialectIDs[d.ID] = true
		for _, e := range d.Match.Extensions {
			claim(keyOwner, strings.ToLower(e), d.ID)
		}
		for _, v := range d.Variants {
			for _, e := range v.Extensions {
				claim(keyOwner, strings.ToLower(e), d.ID)
			}
		}
		for _, b := range d.Match.Basenames {
			claim(keyOwner, b, d.ID)
		}
		if d.Match.Sniff != nil {
			for _, e := range d.Match.Sniff.Extensions {
				keyOwner[strings.ToLower(e)] = "" // a sniffed extension is settled per file
			}
		}
		for _, k := range d.Kinds {
			claim(kindOwner, k.SourceKind, d.ID)
		}
	}
}

// DialectOfContent is DialectOf for a caller that has the file's bytes: a .h
// file is read by the header router, whose own sniff (extractor.HeaderDialect)
// names c, cpp or objective-c even when the header holds nothing to emit.
func DialectOfContent(e extractor.Extractor, tk ToolKind, relPath string, content []byte, nodes []core.SymbolNode) string {
	if _, ok := e.(*extractor.CHeaderExtractor); ok {
		return extractor.HeaderDialect(content)
	}
	return DialectOf(e, tk, relPath, nodes)
}

// DialectOf is the id of the dialect that read relPath, or "" when no signal
// names one. e and tk are the extractor and tool kind the dispatch chose and
// nodes what it returned. The order is the tool kind (the most specific
// signal), the generic JSON and TOML readers, the path's extension or basename
// when one dialect owns it, the extractor's own name, the kinds of the nodes,
// then the native Go extractor.
func DialectOf(e extractor.Extractor, tk ToolKind, relPath string, nodes []core.SymbolNode) string {
	dialectIndexOnce.Do(buildDialectIndex)
	if tk != "" && tk != ToolUnclassified {
		if id, ok := toolKindDialect[tk]; ok {
			return id
		}
		if dialectIDs[string(tk)] {
			return string(tk)
		}
	}
	// The extractor that read the file speaks before the file's name does: a
	// basename a dialect owns (envoy.json) whose content check failed was read by
	// the generic reader, and is labelled by it.
	switch e.(type) {
	case *extractor.JSONExtractor:
		return GenericJSON
	case *extractor.TOMLExtractor:
		return GenericTOML
	case *extractor.TemplExtractor:
		return Templ
	}
	if id := keyOwner[filepath.Base(relPath)]; id != "" {
		return id
	}
	if id := keyOwner[strings.ToLower(filepath.Ext(relPath))]; id != "" {
		return id
	}
	if n, ok := e.(interface{ Name() string }); ok && dialectIDs[n.Name()] {
		return n.Name()
	}
	for _, n := range nodes {
		if id := kindOwner[string(n.Kind)]; id != "" {
			return id
		}
	}
	if _, ok := e.(*extractor.GoExtractor); ok {
		return "go"
	}
	return ""
}
