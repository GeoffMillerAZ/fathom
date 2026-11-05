// Package cuelang is the CUE family of native dialects: cue, timoni,
// fathom-concerns and fathom-layer-packs. It lives apart so internal/extractor
// compiles without cuelang.org/go (ADR-092, LL-7). A binary that reads CUE
// links this package, and its init registers the four dialects with the
// extractor registry before main or any test runs, so registration always
// precedes the first dialect lookup.
package cuelang

import (
	"sync"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

var registerOnce sync.Once

func init() { RegisterCUE() }

// RegisterCUE registers the four CUE-family dialects with internal/extractor.
// It is idempotent: the product reaches it through this package's init and a
// caller may call it again.
func RegisterCUE() {
	registerOnce.Do(func() {
		extractor.RegisterNativeDialect("cue", func() extractor.Extractor { return NewCUEExtractor() })
		for _, id := range []string{"timoni", "fathom-concerns", "fathom-layer-packs"} {
			extractor.RegisterNativeDialect(id, func() extractor.Extractor {
				if e, ok := TimoniDialectExtractor(id); ok {
					return e
				}
				e, _ := FathomCUEDialectExtractor(id)
				return e
			})
		}
	})
}
