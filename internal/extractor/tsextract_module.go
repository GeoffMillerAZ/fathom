package extractor

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// tsextract_module.go — the module symbol (K18, ADR-048 amendment 2026-10-10,
// wave lf2, item python): a tags-engine capability a dialect manifest turns on
// with module_symbol, naming the kind the engine mints. Every file of such a
// dialect gets one definition of that kind, named core.ModuleSymbolName, from
// line 1 to the file's last line: the file's top level executes code (a
// Python module, a JavaScript module, a Ruby, Lua or shell script), and the
// module symbol is what that code is made from and what an import of the
// module targets.
//
// It owns the calls no definition owns: a call no definition's node encloses
// and no call attachment (@calls.from/@calls.to) gives to a definition — a
// module-level statement's, an `if __name__ == "__main__":` block's, a
// module-level loop's. A call inside a function stays the function's, a
// decorator's the definition it decorates, an assignment's the variable it
// defines, and a call in a class body outside any member stays unowned as
// before: the class, not the module, encloses it. The file's imports are made
// from it by the file resolver, which reads definitions by line
// (fusion.enclosingAt): the module symbol spans every line no narrower
// definition does.
//
// A dialect whose manifest names no module_symbol skips the pass and its
// output is unchanged (TestModuleSymbolOnlyWhereTheManifestNamesIt). An
// injected range (a host file's embedded language) mints none: the host file
// is the module.

// moduleKindOf is d's module_symbol kind compiled for the engine, nil when the
// manifest names none (checkModuleSymbol vets the name at load).
func moduleKindOf(d Dialect) *tsDefKind {
	k, ok := d.Kinds[d.ModuleSymbol]
	if d.ModuleSymbol == "" || !ok {
		return nil
	}
	return &tsDefKind{Kind: core.SourceKind(k.SourceKind)}
}

// mintTSModule appends the file's module symbol to defs when rules name a
// module kind and whole is set (the file itself, not an injected range); it
// returns defs unchanged otherwise. calls and attaches are the file's call
// sites and call attachments, as attachTSCalls read them.
func mintTSModule(defs []*tsDef, calls []tsCallSite, attaches []tsCallAttach, src []byte, rules *tsDialectRules, whole bool) []*tsDef {
	if rules.moduleKind == nil || !whole {
		return defs
	}
	m := &tsDef{
		kind:     rules.moduleKind.Kind,
		name:     core.ModuleSymbolName,
		qualName: core.ModuleSymbolName,
		start:    0,
		end:      uint32(len(src)),
		line:     1,
		defLine:  1,
		parent:   -1,
	}
	owned := tsOwnedSpans(defs, attaches)
	set := map[string]struct{}{}
	for _, c := range calls {
		if !owned.covers(c.start, c.end) {
			set[c.text] = struct{}{}
		}
	}
	for text := range set {
		m.calls = append(m.calls, text)
	}
	sort.Strings(m.calls)
	return append(defs, m)
}

// tsSpans is a set of byte ranges sorted by start, with the running maximum
// of their ends, so whether any of them encloses a range is one binary search.
type tsSpans struct {
	starts, maxEnd []uint32
}

// tsOwnedSpans is the ranges inside which a call has an owner other than the
// module: every definition's node, and every call attachment's from-range
// whose to-range is a definition's node (attachTSCalls' rule for a call no
// callable encloses).
func tsOwnedSpans(defs []*tsDef, attaches []tsCallAttach) tsSpans {
	type span struct{ start, end uint32 }
	at := make(map[span]bool, len(defs))
	spans := make([]span, 0, len(defs)+len(attaches))
	for _, d := range defs {
		at[span{d.start, d.end}] = true
		spans = append(spans, span{d.start, d.end})
	}
	for _, a := range attaches {
		if at[span{a.toStart, a.toEnd}] {
			spans = append(spans, span{a.fromStart, a.fromEnd})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var out tsSpans
	var hi uint32
	for i, s := range spans {
		if i == 0 || s.end > hi {
			hi = s.end
		}
		out.starts = append(out.starts, s.start)
		out.maxEnd = append(out.maxEnd, hi)
	}
	return out
}

// covers reports whether some range of s encloses [start, end).
func (s tsSpans) covers(start, end uint32) bool {
	i := sort.Search(len(s.starts), func(i int) bool { return s.starts[i] > start }) - 1
	return i >= 0 && s.maxEnd[i] >= end
}
