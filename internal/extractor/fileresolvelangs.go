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

// FileResolveLangs lists, sorted by Lang, every tags dialect whose manifest
// names both an imports query and an import strategy other than Go's native
// go-package. Dialects in one extractor group (JavaScript and TypeScript)
// fold into the group and must agree on strategy and parameters.
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
		if (d.Engine != "tags" && d.Engine != "tags+doc") || d.Queries.Imports == "" || d.Imports == nil || d.Imports.Strategy == "go-package" {
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
