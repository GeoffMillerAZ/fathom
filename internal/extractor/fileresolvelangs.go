package extractor

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// FileResolveLang is one language the L3 file resolver runs for (LB-F11,
// design §6): a TSExtractor registry key (a tags dialect id, or the
// extractor group its dialects share), the extensions it covers, and the
// import strategy its manifest selects.
type FileResolveLang struct {
	Lang       string
	Extensions []string // lowercased, sorted
	Strategy   string
	Params     ImportParams
}

// FileSpecExtractor is what the file resolver (fusion's file-resolved pass
// and import resolution, the harness's L3 scoring) needs of a language's
// extractor: its symbols and, per file, its import specs with their
// statement spans, its references and its re-exports. The tags engine's
// TSExtractor is one; a native dialect whose own reader parses its imports
// (Rego, LB-F131) is another.
type FileSpecExtractor interface {
	Extractor
	Name() string
	ExtractFileSpecs(relPath string, src []byte) (FileSpecs, error)
	ExtractImportSpans(relPath string, src []byte) ([]ImportSpan, error)
	Close()
}

// NewFileSpecExtractor is lang's file-resolve extractor: a native dialect's
// own when it parses imports, else the tags engine's for lang.
func NewFileSpecExtractor(lang string) (FileSpecExtractor, error) {
	if e, ok := nativeFileSpecExtractor(lang); ok {
		return e, nil
	}
	ts, err := NewTSExtractor(lang)
	if err != nil {
		return nil, err
	}
	return ts, nil
}

func nativeFileSpecExtractor(id string) (FileSpecExtractor, bool) {
	e, ok := NativeDialectExtractor(id)
	if !ok {
		return nil, false
	}
	f, ok := e.(FileSpecExtractor)
	return f, ok
}

// FileResolveLangs lists, sorted by Lang, every tags dialect whose manifest
// names both an imports query and an import strategy other than Go's native
// go-package, and every native dialect whose manifest names a strategy and
// whose own reader parses its imports (a FileSpecExtractor). Dialects in one
// extractor group (JavaScript and TypeScript) fold into the group and must
// agree on strategy and parameters.
func FileResolveLangs() ([]FileResolveLang, error) {
	ds, err := LoadDialects()
	if err != nil {
		return nil, err
	}
	groupOf := map[string]string{}
	for g, members := range tsExtractorGroups {
		for _, m := range members {
			groupOf[m] = g
		}
	}
	byLang := map[string]*FileResolveLang{}
	for _, d := range ds {
		if d.Imports == nil || d.Imports.Strategy == "go-package" {
			continue
		}
		switch d.Engine {
		case "tags", "tags+doc":
			if d.Queries.Imports == "" {
				continue
			}
		case "native":
			if _, ok := nativeFileSpecExtractor(d.ID); !ok {
				continue
			}
		default:
			continue
		}
		lang := d.ID
		if g, ok := groupOf[d.ID]; ok {
			lang = g
		}
		fl, ok := byLang[lang]
		if !ok {
			fl = &FileResolveLang{Lang: lang, Strategy: d.Imports.Strategy, Params: d.Imports.Params}
			byLang[lang] = fl
		} else if fl.Strategy != d.Imports.Strategy || !reflect.DeepEqual(fl.Params, d.Imports.Params) {
			return nil, fmt.Errorf("extractor group %q: dialect %q's imports differ from its group's", lang, d.ID)
		}
		for _, e := range dialectExtensions(d) {
			fl.Extensions = append(fl.Extensions, strings.ToLower(e))
		}
	}
	out := make([]FileResolveLang, 0, len(byLang))
	for _, fl := range byLang {
		sort.Strings(fl.Extensions)
		out = append(out, *fl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Lang < out[j].Lang })
	return out, nil
}
