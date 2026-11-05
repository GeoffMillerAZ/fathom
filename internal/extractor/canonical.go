package extractor

import (
	"sort"
	"sync"
)

var (
	canonicalOnce  sync.Once
	canonicalBySrc map[string]string
)

// CanonicalKind answers §2.3's canonical kind for a SourceKind from the
// dialect registry (LB-F24). ok is false for a SourceKind no manifest
// declares. The canonical kind is derived, never stored: SymbolNode.Kind
// keeps the dialect's SourceKind string and no store column changes.
// Dialects that share a SourceKind (sharedSourceKindGroups) share its
// canonical kind; TestCanonicalKindClosedList holds that.
func CanonicalKind(sk string) (canonical string, ok bool) {
	canonicalOnce.Do(func() {
		canonicalBySrc = map[string]string{}
		ds, err := LoadDialects()
		if err != nil {
			return
		}
		for _, d := range ds {
			for _, key := range sortedKindKeys(d) {
				k := d.Kinds[key]
				if _, dup := canonicalBySrc[k.SourceKind]; !dup {
					canonicalBySrc[k.SourceKind] = k.Canonical
				}
			}
		}
	})
	canonical, ok = canonicalBySrc[sk]
	return canonical, ok
}

// nativeCanonical is the canonical kind of each SourceKind an extractor with no
// dialect manifest emits: the native Go extractor, which has none, the
// generic JSON and TOML readers and the templ reader (a component, script or css
// block is a function-like definition). The Go rows are the Go incumbent manifest's
// (internal/langbench/incumbents.json, which TestNativeCanonicalMatchesGoIncumbent
// holds the table to). Only the language library's door reads this table:
// CanonicalKind, which the product's passes read, is unchanged.
var nativeCanonical = map[string]string{
	"GoFunc": "function", "GoMethod": "method", "GoField": "field", "GoType": "struct",
	"GoInterface": "interface", "GoConst": "constant", "GoVar": "variable",
	"JSONKey": "key", "TOMLKey": "key", "TOMLTable": "section",
	"TemplComponent": "function",
}

// noCanonical are the kinds that have no canonical meaning, each with the
// reason, so an empty Canonical is a decision on this list and never an
// omission. TestFixturesHoldTheResultContract (pkg/lang) fails on an empty one not
// listed, and on a listed one no fixture emits.
var noCanonical = map[string]string{
	"K8sFact":           "a fact a workload writes, an observation of the file and never a definition (ADR-067)",
	"TerraformProvider": "a provider configuration declares no entity a reference names",
}

// DoorCanonicalKind is the canonical kind of a SourceKind for the language
// library's door: the dialect registry's answer (CanonicalKind), then the
// native table. ok is false for a kind neither knows.
func DoorCanonicalKind(sk string) (canonical string, ok bool) {
	if c, ok := CanonicalKind(sk); ok {
		return c, true
	}
	c, ok := nativeCanonical[sk]
	return c, ok
}

// HasNoCanonical reports whether sk is a kind with no canonical meaning, and
// why.
func HasNoCanonical(sk string) (reason string, ok bool) {
	reason, ok = noCanonical[sk]
	return reason, ok
}

// NoCanonicalKinds is the kinds HasNoCanonical lists, sorted.
func NoCanonicalKinds() []string {
	out := make([]string, 0, len(noCanonical))
	for k := range noCanonical {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
