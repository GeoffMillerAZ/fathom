package extractor

// tsextract_host.go — injection hosting (F2, wave lf7). A host manifest's
// injection_host makes the file the module its injected ranges are part of:
// a Vue or Svelte component's <script>, an Astro page's frontmatter. What a
// range imports and runs at its top level is the host file's:
//
//   - its file specs (imports, references, re-exports; ExtractFileSpecs'
//     answer for the range read as its own document) become the host file's,
//     at host lines, so the file resolver resolves them under the HOST
//     dialect's imports strategy (it keys a file by its extension) and draws
//     them from the host's definitions by line;
//   - its top-level calls, the ones no definition of the range encloses or
//     claims (mintTSModule's rule, run with the injected dialect's own call
//     attachments), are the host's module symbol's, and the range mints no
//     module symbol of its own;
//   - the host file's default export is its module symbol (hostDefaultExport):
//     a component file's default export is the component.
//
// A definition of the range keeps its own calls, kind and package, exactly
// as extractInjections gives it without the field. A host without the field
// (html) is unchanged: its ranges' imports and top-level calls are dropped.
// One level deep, as before: a range's own injections are not run.

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// hostedModuleKind is the module kind a hosted range of a dialect without a
// module_symbol mints, only to gather its top-level calls; it is never
// emitted.
const hostedModuleKind core.SourceKind = "fathom.hostedRange"

// hostedVariants caches, per injected dialect id, the variant a hosted range
// is read with: the dialect's own default variant (its grammar, query and
// pool key) whose rules run no injections of their own and name a module kind,
// so extractLocked's whole-file pass mints the module symbol that gathers the
// range's top-level calls.
var hostedVariants sync.Map // dialect id -> tsVariant

func hostedVariant(dialect string) tsVariant {
	if v, ok := hostedVariants.Load(dialect); ok {
		return v.(tsVariant)
	}
	v := tsDialectMap[dialect].defaultVariant
	r := *v.rules
	r.injectionsQuery = ""
	if r.moduleKind == nil {
		r.moduleKind = &tsDefKind{Kind: hostedModuleKind}
	}
	v.rules = &r
	got, _ := hostedVariants.LoadOrStore(dialect, v)
	return got.(tsVariant)
}

// peerParser is the extractor that reads a variant's own files: its pool
// owner, and the rules whose queries it compiles on the Parser it leases.
type peerParser struct {
	owner string
	rules *tsDialectRules
}

// LinkHostedParsers makes every injection host among exts read a hosted
// range on the pooled Parser of the extractor among exts that reads the
// range's dialect's own files with the same variant (LB-F550, O982, K26): an
// Astro frontmatter's TypeScript on the .ts files' parser, its <style>'s CSS
// on the .css files'. A host leasing its ranges under its own keys kept more
// variants live than tsengine.MaxLiveModules on a tree mixing components
// with .ts, .tsx, .mjs and .css files, and re-instantiated the 2 MB
// TypeScript module file after file. A range shares a Parser only when the
// peer compiles the very queries the range runs on it (tags, which carries
// imports and locals, and references), so a query cached on the Parser is
// the range's own; the hosted rules' differences (no injections query, a
// module kind) are applied in Go, never compiled. Output is unchanged: only
// which pooled module parses the range moves. The peer's Close closes the
// shared modules (RT-107; a leased one at its release). Extractors are
// visited by language name, so the first peer of a variant is the same on
// every run. The fusion pipeline calls it once its table is built.
func LinkHostedParsers(exts []Extractor) {
	var all []*TSExtractor
	seen := map[*TSExtractor]bool{}
	for _, x := range exts {
		if e, ok := x.(*TSExtractor); ok && !seen[e] {
			seen[e] = true
			all = append(all, e)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].spec.name < all[j].spec.name })
	byKey := map[string]peerParser{}
	for _, e := range all {
		vs := []tsVariant{e.spec.defaultVariant}
		xs := make([]string, 0, len(e.spec.variantForExt))
		for x := range e.spec.variantForExt {
			xs = append(xs, x)
		}
		sort.Strings(xs)
		for _, x := range xs {
			vs = append(vs, e.spec.variantForExt[x])
		}
		for _, v := range vs {
			if v.rules == nil || v.rules.injectionHost {
				continue
			}
			if _, ok := byKey[v.key()]; !ok {
				byKey[v.key()] = peerParser{owner: e.owner, rules: v.rules}
			}
		}
	}
	for _, e := range all {
		if !e.hostsRanges() {
			continue
		}
		peers := map[string]peerParser{}
		for k, p := range byKey {
			if p.owner != e.owner {
				peers[k] = p
			}
		}
		e.mu.Lock()
		e.peers = peers
		e.mu.Unlock()
	}
}

// hostsRanges reports whether any of e's variants is an injection host.
func (e *TSExtractor) hostsRanges() bool {
	if e.spec.defaultVariant.rules != nil && e.spec.defaultVariant.rules.injectionHost {
		return true
	}
	for _, v := range e.spec.variantForExt {
		if v.rules != nil && v.rules.injectionHost {
			return true
		}
	}
	return false
}

// hostedLease is v leased on its peer's pooled Parser when LinkHostedParsers
// found one that compiles the same queries, else v as is (its own key).
// Caller holds e.mu.
func (e *TSExtractor) hostedLease(v tsVariant) tsVariant {
	p, ok := e.peers[v.key()]
	if ok && p.rules.query == v.rules.query && p.rules.refsQuery == v.rules.refsQuery {
		v.leaseOwner = p.owner
	}
	return v
}

// extractHostedInjections is extractInjections for a host (rules.injectionHost):
// each range read as its own document at host lines in the host's package,
// its definitions returned, its top-level calls added to host's module symbol
// (host is the caller's node slice; its module symbol's Calls is replaced),
// and, when sp is non-nil, its file specs merged into sp's. A range whose
// extraction faults is skipped, as without the field; one whose specs could not
// be read leaves sp not ok (the caller asks ExtractFileSpecs, which reproduces
// the answer or the fault). Caller holds e.mu.
func (e *TSExtractor) extractHostedInjections(rules *tsDialectRules, injected []tsInjection, service, relPath string, src []byte, pkg string, host []core.SymbolNode, sp *tsSpecsOut) []core.SymbolNode {
	var out []core.SymbolNode
	var top []string
	for _, inj := range injected {
		v := e.hostedLease(hostedVariant(inj.dialect))
		var isp *tsSpecsOut
		if sp != nil {
			isp = &tsSpecsOut{}
		}
		nodes, _, err := e.extractLocked(v, service, relPath, src[inj.start:inj.end], pkg, inj.row, true, isp)
		if err != nil {
			continue
		}
		for _, n := range nodes {
			if n.IsModuleSymbol() && n.Kind == v.rules.moduleKind.Kind {
				top = append(top, n.Calls...)
				continue
			}
			out = append(out, n)
		}
		if sp != nil {
			switch {
			case !isp.ok:
				sp.ok = false
			case sp.ok:
				sp.specs = mergeHostedSpecs(sp.specs, isp.specs, inj.row)
			}
		}
	}
	if len(top) > 0 && rules.moduleKind != nil {
		for i := range host {
			if host[i].IsModuleSymbol() && host[i].Kind == rules.moduleKind.Kind {
				calls := append(slices.Clone(host[i].Calls), top...)
				slices.Sort(calls)
				host[i].Calls = slices.Compact(calls)
			}
		}
	}
	return out
}

// hostDefaultExport makes a host file's default export its module symbol: a
// component file's default export is the component (Vue's compiled SFC, a
// Svelte or Astro component), so an import of the file binds the module
// symbol, never a definition its <script> happens to hold alone. A range's own
// default export of a name (`export default X` in a Vue <script>) is dropped:
// the compiler merges it into the component, and two defaults would bind
// nothing.
func hostDefaultExport(fs FileSpecs) FileSpecs {
	out := []ExportSpec{{Name: core.ModuleSymbolName, As: "default", Line: 1}}
	for _, ex := range fs.Exports {
		if ex.Module == "" && ex.As == "default" {
			continue
		}
		out = append(out, ex)
	}
	fs.Exports = out
	return fs
}

// mergeHostedSpecs adds a hosted range's file specs, read at range lines, to
// the host's: every line moved down by row (the range's first row in the
// host), imports and re-exports after the host's, references merged by name
// under the same bounds refSpecsFromCaptures keeps.
func mergeHostedSpecs(dst, rng FileSpecs, row int) FileSpecs {
	for _, im := range rng.Imports {
		im.Line += row
		dst.Imports = append(dst.Imports, im)
	}
	for _, ex := range rng.Exports {
		ex.Line += row
		dst.Exports = append(dst.Exports, ex)
	}
	if len(rng.Refs) == 0 {
		return dst
	}
	lines := map[string][]int{}
	for _, r := range dst.Refs {
		lines[r.Name] = append(lines[r.Name], r.Lines...)
	}
	for _, r := range rng.Refs {
		for _, l := range r.Lines {
			lines[r.Name] = append(lines[r.Name], l+row)
		}
	}
	names := make([]string, 0, len(lines))
	for n := range lines {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > maxRefNames {
		names = names[:maxRefNames]
	}
	refs := make([]RefSpec, 0, len(names))
	for _, n := range names {
		ls := slices.Compact(slices.Sorted(slices.Values(lines[n])))
		if len(ls) > maxRefLines {
			ls = ls[:maxRefLines]
		}
		refs = append(refs, RefSpec{Name: n, Lines: ls})
	}
	dst.Refs = refs
	return dst
}

// hostedFileSpecsLocked is ExtractFileSpecs for a host: the specs one
// extraction of the file reads, its ranges' merged in (ExtractWithFileSpecs'
// answer, so the two agree). Caller holds e.mu.
func (e *TSExtractor) hostedFileSpecsLocked(variant tsVariant, relPath string, src []byte) (FileSpecs, error) {
	var out tsSpecsOut
	if _, _, err := e.extractNodesLocked(variant, "", relPath, src, &out); err != nil {
		return FileSpecs{}, err
	}
	if !out.ok {
		return FileSpecs{}, fmt.Errorf("tsextract[%s] %s: an injected range's file specs could not be read: %w", e.spec.name, relPath, ErrTSEngineFault)
	}
	return out.specs, nil
}

// hostedImportSpansLocked is the import spans of a host's injected ranges, at
// host lines, in range order: ExtractImportSpans' answer for each range read
// as its own document. tree is the host's, parsed by l. A range whose parse
// or query faults is skipped, as extraction skips it. Caller holds e.mu.
func (e *TSExtractor) hostedImportSpansLocked(variant tsVariant, l *tsengine.Lease, tree *tsengine.Tree, src []byte) ([]ImportSpan, error) {
	ranges, err := e.injectionRanges(variant, l, tree, src)
	if err != nil {
		return nil, err
	}
	var out []ImportSpan
	for _, inj := range ranges {
		spans, err := e.rangeImportSpansLocked(e.hostedLease(hostedVariant(inj.dialect)), src[inj.start:inj.end])
		if err != nil {
			continue
		}
		for _, s := range spans {
			s.Line += inj.row
			s.StartLine += inj.row
			s.EndLine += inj.row
			out = append(out, s)
		}
	}
	return out, nil
}

// rangeImportSpansLocked is ExtractImportSpans' body for one hosted range
// under v. Caller holds e.mu.
func (e *TSExtractor) rangeImportSpansLocked(v tsVariant, src []byte) ([]ImportSpan, error) {
	l, q, err := e.leaseFor(context.Background(), v)
	if err != nil {
		return nil, err
	}
	defer l.Release()
	e.countParse()
	tree, err := l.Parser().Parse(src)
	if err != nil {
		e.noteFault(l)
		return nil, fmt.Errorf("tsextract[%s]: hosted range parse: %v: %w", e.spec.name, err, ErrTSEngineFault)
	}
	defer tree.Close()
	matches, err := q.Run(tree)
	if err != nil {
		e.noteFault(l)
		return nil, fmt.Errorf("tsextract[%s]: hosted range query: %v: %w", e.spec.name, err, ErrTSEngineFault)
	}
	specs, spans := importSpecsWithSpans(matches, src, true)
	out := make([]ImportSpan, len(specs))
	for i, s := range specs {
		lo := s.Line
		if sp := spans[i][0]; sp > 0 && sp < lo {
			lo = sp
		}
		out[i] = ImportSpan{ImportSpec: s, StartLine: lo, EndLine: max(spans[i][1], s.Line)}
	}
	return out, nil
}
