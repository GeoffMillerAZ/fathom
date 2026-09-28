package extractor

import "sync"

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
