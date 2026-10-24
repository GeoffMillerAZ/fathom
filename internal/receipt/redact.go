package receipt

import (
	"sort"
	"strings"
)

// RedactReceipt removes service identities in place unless keepService is
// explicitly set. Apply it after BuildReceipt and before writing the export.
// Source paths, excerpts and digests remain the evidence's original bytes.
func RedactReceipt(r *ExportedReceipt, keepService bool) {
	if !keepService {
		redactReceipt(r)
	}
}

// redactReceipt covers the current envelope's service parameter and every
// symbol-bearing field, including keys belonging to another service.
// Redaction can map distinct keys to the same @:: key, so the redacted lists
// may hold duplicates; they are sorted, not deduplicated.
func redactReceipt(r *ExportedReceipt) {
	if r.Service != "" {
		r.Service = "@"
	}
	r.Symbol = redactSymbolKey(r.Symbol)
	r.Test = redactSymbolKey(r.Test)
	r.Target = redactSymbolKey(r.Target)
	for _, keys := range [][]string{r.Callers, r.Reached} {
		for i := range keys {
			keys[i] = redactSymbolKey(keys[i])
		}
		sort.Strings(keys)
	}
	for i := range r.Facts {
		r.Facts[i].Subject = redactSymbolKey(r.Facts[i].Subject)
	}
	sort.SliceStable(r.Facts, func(i, j int) bool {
		if r.Facts[i].Kind != r.Facts[j].Kind {
			return r.Facts[i].Kind < r.Facts[j].Kind
		}
		return r.Facts[i].Subject < r.Facts[j].Subject
	})
}

func redactSymbolKey(key string) string {
	if _, name, ok := strings.Cut(key, "::"); ok {
		return "@::" + name
	}
	return key
}
