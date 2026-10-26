package fusion

import (
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"testing"
)

// The namespace setting changes binding, not path lookup. Existing strategies
// retain their wildcard behavior unless their manifest explicitly opts in.
func TestWholeBindingsNamespaceIsOptIn(t *testing.T) {
	for _, strategy := range []string{"include-path", "file-symbol", "swift-module"} {
		r := &importResolver{strategy: strategy}
		if !r.wholeIsWildcard() {
			t.Errorf("%s changed its default", strategy)
		}
		r.params = extractor.ImportParams{WholeBindings: "namespace"}
		if r.wholeIsWildcard() {
			t.Errorf("%s ignored namespace binding", strategy)
		}
	}
}
