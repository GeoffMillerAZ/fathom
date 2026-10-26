package proseheadless

import (
	"reflect"
	"testing"

	evidencespan "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

func TestExposedFactsIncludesReviewAndRecursiveDerivations(t *testing.T) {
	doc := prose.DocumentVersion{
		Target: prose.DocumentTarget{Input: prose.DocumentInput{Facts: []prose.Fact{
			{Span: evidencespan.Span{ID: "B"}, Derivation: &prose.Derivation{Inputs: []string{"C"}}},
			{Span: evidencespan.Span{ID: "A"}, Derivation: &prose.Derivation{Inputs: []string{"B"}}},
			{Span: evidencespan.Span{ID: "C"}},
		}}},
		Nodes: []prose.DocNode{{GenManifest: &prose.ReadManifest{Facts: []prose.ManifestFact{{ID: "A"}}}, ReviewMan: &prose.ReadManifest{Facts: []prose.ManifestFact{{ID: "review-only"}}}}},
	}
	if got := exposedFacts(doc); !reflect.DeepEqual(got, []string{"A", "B", "C", "review-only"}) {
		t.Fatalf("missing transitive/review inputs: %v", got)
	}
}

// QueueMutationAvailableForTest probes the write gate inside actual effect tests.
func QueueMutationAvailableForTest(q *Queue) bool {
	if !q.mutation.TryRLock() {
		return false
	}
	q.mutation.RUnlock()
	return true
}
