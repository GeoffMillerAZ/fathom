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
// extension (an extension two dialects share, MATLAB's and Objective-C's .m,
// names both); "<label> or <label> (by content sniff)" for the .h router, which
// picks between two extractors per file, three with Objective-C; "refused (no extractor)" for a
// refused dialect's keys.
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
	sniffers := map[string][]string{} // a sniff-only extension -> the dialects that claim it
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
					sniffers[x] = append(sniffers[x], d.ID)
				}
			}
		}
	}
	for x, ids := range sniffers {
		sort.Strings(ids)
		if len(ids) == 1 {
			out[x] = fmt.Sprintf("tags dialect `%s` (by content sniff)", ids[0])
			continue
		}
		// MATLAB and Objective-C share .m: the classifier picks per file.
		out[x] = fmt.Sprintf("tags dialects `%s` (by content sniff)", strings.Join(ids, "` or `"))
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
	case *extractor.CHeaderExtractor:
		// The CT-3 router picks per file between the two extractors it holds
		// (LB-F392): name them, not the router's own Go type.
		c, cpp := x.Routes()
		lc, lcpp := dispatchLabel(key, c, docKeys), dispatchLabel(key, cpp, docKeys)
		if _, ok := c.(*extractor.TSExtractor); ok {
			if t, ok := cpp.(*extractor.TSExtractor); ok {
				lcpp = fmt.Sprintf("`%s`", t.Name())
			}
		}
		if objc := x.ObjC(); objc != nil {
			lobjc := dispatchLabel(key, objc, docKeys)
			if t, ok := objc.(*extractor.TSExtractor); ok {
				lobjc = fmt.Sprintf("`%s`", t.Name())
			}
			return fmt.Sprintf("%s, %s or %s (by content sniff)", lc, lcpp, lobjc)
		}
		return fmt.Sprintf("%s or %s (by content sniff)", lc, lcpp)
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
