package logtemplate

// apis.go — the embedded logger-API registry (apis.json): universal
// defaults, data only (K15). Never a language dialect's queries: the
// dialects own extraction, this package owns one fact the dialects do not
// emit.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed apis.json
var apisJSON []byte

type apiSpec struct {
	API                string   `json:"api"`
	Default            bool     `json:"default"`
	Imports            []string `json:"imports"`
	Binds              []string `json:"binds"`
	ReceiverCallPrefix string   `json:"receiver_call_prefix"`
	Receivers          []string `json:"receivers"`
	Methods            []string `json:"methods"`
	LevelMethods       []string `json:"level_methods"`
	Style              string   `json:"style"`

	binds []*regexp.Regexp
}

type formatterSpec struct {
	Call  string `json:"call"`
	Style string `json:"style"`
}

type langSpec struct {
	Language      string            `json:"language"`
	Exts          []string          `json:"exts"`
	Receivers     []string          `json:"receivers"`
	ReceiverCalls []string          `json:"receiver_calls"`
	Annotations   map[string]string `json:"annotations"`
	APIs          []apiSpec         `json:"apis"`
	Formatters    []formatterSpec   `json:"formatters"`

	lang lang
}

type registry struct {
	langs []langSpec
	byExt map[string]int
}

// The styles a message's placeholders read in.
const (
	stylePercent       = "percent"
	stylePrintf        = "printf"
	styleBrace         = "brace"
	styleMessageFormat = "messageformat"
	stylePSR3          = "psr3"
	styleEvent         = "event"
)

var reg = mustLoadRegistry(apisJSON)

func mustLoadRegistry(b []byte) registry {
	r, err := loadRegistry(b)
	if err != nil {
		panic("logtemplate: apis.json: " + err.Error())
	}
	return r
}

func loadRegistry(b []byte) (registry, error) {
	var doc struct {
		Languages []langSpec `json:"languages"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return registry{}, err
	}
	r := registry{byExt: map[string]int{}}
	langs := map[string]lang{"python": langPython, "java": langJava, "node": langNode, "php": langPHP}
	styles := map[string]bool{stylePercent: true, stylePrintf: true, styleBrace: true, styleMessageFormat: true, stylePSR3: true, styleEvent: true}
	for i := range doc.Languages {
		ls := &doc.Languages[i]
		l, ok := langs[ls.Language]
		if !ok {
			return registry{}, fmt.Errorf("unknown language %q", ls.Language)
		}
		ls.lang = l
		defaults := 0
		for j := range ls.APIs {
			a := &ls.APIs[j]
			if !styles[a.Style] {
				return registry{}, fmt.Errorf("%s: unknown style %q", a.API, a.Style)
			}
			if a.Default {
				defaults++
			}
			for _, pat := range a.Binds {
				re, err := regexp.Compile(`([A-Za-z_$][\w$]*(?:(?:\.|->)[A-Za-z_$][\w$]*)?)\s*(?::[^=\n;]*)?=\s*(?:new\s+)?(?:` + pat + `)`)
				if err != nil {
					return registry{}, fmt.Errorf("%s: bind %q: %w", a.API, pat, err)
				}
				a.binds = append(a.binds, re)
			}
		}
		if defaults != 1 {
			return registry{}, fmt.Errorf("%s: %d default APIs, want 1", ls.Language, defaults)
		}
		for _, f := range ls.Formatters {
			if !styles[f.Style] {
				return registry{}, fmt.Errorf("%s: formatter %s: unknown style %q", ls.Language, f.Call, f.Style)
			}
		}
		for _, e := range ls.Exts {
			r.byExt[e] = len(r.langs)
		}
		r.langs = append(r.langs, *ls)
	}
	return r, nil
}

// langFor returns the language spec reading path, by extension.
func (r registry) langFor(path string) (*langSpec, bool) {
	i, ok := r.byExt[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return nil, false
	}
	return &r.langs[i], true
}

// Reads reports whether the reader reads path (by extension): the ingest
// pass's candidate predicate.
func Reads(path string) bool {
	_, ok := reg.langFor(path)
	return ok
}

// APIs lists every logger API the registry names, sorted by language then
// as written (the reading's coverage table).
func APIs() []string {
	var out []string
	for _, l := range reg.langs {
		for _, a := range l.APIs {
			out = append(out, a.API)
		}
	}
	return out
}

func (a *apiSpec) method(m string) (level bool, ok bool) {
	for _, x := range a.Methods {
		if x == m {
			return false, true
		}
	}
	for _, x := range a.LevelMethods {
		if x == m {
			return true, true
		}
	}
	return false, false
}
