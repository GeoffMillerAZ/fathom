package fusion

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// DispatchTable is the read-only view of the per-key extractor dispatch that
// New wires (LB-F356): for every extension or basename key, a short label of
// the extractor that serves it. It is computed from a freshly constructed
// Pipeline's own registry (p.exts), the sniff-only dialects (p.sniffExts) and
// refusedDialects, so docs/languages.md's "Engine / dispatch" column is
// generated from what ingest does and cannot be hand-typed into disagreement.
// A key no extractor serves is absent.
//
// The label is one of: tags dialect `id`; doc dialect `id`; native Go; native
// CUE; Go walker `Type`; "<label> (by content sniff)" for a sniff-only
// extension; "refused (no extractor)" for a refused dialect's keys.
func DispatchTable() map[string]string {
	p := New(nil, nil, Config{})
	defer p.Close()
	docKeys := docDialectIDsByKey()
	out := map[string]string{}
	for k, e := range p.exts {
		out[k] = dispatchLabel(k, e, docKeys)
	}
	dialects, err := extractor.LoadDialects()
	if err != nil {
		return out
	}
	for _, d := range dialects {
		keys := append([]string(nil), d.Match.Extensions...)
		keys = append(keys, d.Match.Basenames...)
		for _, v := range d.Variants {
			keys = append(keys, v.Extensions...)
		}
		if d.Engine == "tags" && refusedDialects[d.ID] {
			for _, k := range keys {
				if _, claimed := p.exts[k]; !claimed {
					out[k] = "refused (no extractor)"
				}
			}
			continue
		}
		if d.Engine == "tags" && d.Match.Sniff != nil {
			for _, x := range d.Match.Sniff.Extensions {
				if _, claimed := p.exts[x]; !claimed {
					out[x] = fmt.Sprintf("tags dialect `%s` (by content sniff)", d.ID)
				}
			}
		}
	}
	return out
}

// docDialectIDsByKey maps a key to the doc-engine dialects whose manifest
// claims it: the engine's extractor does not expose its dialect id, the
// manifest is the one place it is named.
func docDialectIDsByKey() map[string][]string {
	out := map[string][]string{}
	dialects, err := extractor.LoadDialects()
	if err != nil {
		return out
	}
	for _, d := range dialects {
		if d.Engine == "tags" {
			continue
		}
		keys := append([]string(nil), d.Match.Extensions...)
		keys = append(keys, d.Match.Basenames...)
		for _, v := range d.Variants {
			keys = append(keys, v.Extensions...)
		}
		for _, k := range keys {
			out[k] = append(out[k], d.ID)
		}
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

func dispatchLabel(key string, e extractor.Extractor, docKeys map[string][]string) string {
	switch x := e.(type) {
	case *extractor.TSExtractor:
		return fmt.Sprintf("tags dialect `%s`", x.Name())
	case *extractor.GoExtractor:
		return "native Go (`go/parser` + `go/types`)"
	case *extractor.CUEExtractor:
		return "native CUE (`cue/load`)"
	case *extractor.DocExtractor, *extractor.TagsDocExtractor:
		ids := docKeys[key]
		if len(ids) == 0 {
			return "doc dialect"
		}
		return fmt.Sprintf("doc dialect `%s`", strings.Join(ids, "`, `"))
	}
	t := reflect.TypeOf(e)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return fmt.Sprintf("Go walker `%s`", t.Name())
}
