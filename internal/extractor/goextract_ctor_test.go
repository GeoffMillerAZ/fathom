package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// The evidence-ladder test pattern:
//
//	l := ladder.New(tier1, tier2, nil, ladder.WithClock(fixedClock))
//	res := l.Run(ctx, sig)
//
// The receiver variable's initializer is a selector call into an imported
// package — a syntactically-present fact within the same function body. FIX 3
// records the method call WITH that constructor-package provenance as
// "<import-path>#.Method" (documented on Extract), alongside the legacy
// "<var>.Method" form which keeps the global unique-suffix fallback alive.
const ctorFixture = `package evidenceladder_test

import (
	"context"
	"testing"

	ladder "example.com/kit/foundations/evidence/evidence-ladder"
)

func TestLadderRun(t *testing.T) {
	l := ladder.New(nil, nil, nil, ladder.WithClock(nil))
	res := l.Run(context.Background(), nil)
	_ = res
}
`

// TestGoExtractor_ConstructorScopedMethodCall pins FIX 3 (extractor half).
func TestGoExtractor_ConstructorScopedMethodCall(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("kit", "foundations/evidence/evidence-ladder/x_test.go", []byte(ctorFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	calls := callsOf(t, nodes, "TestLadderRun")

	want := "example.com/kit/foundations/evidence/evidence-ladder#.Run"
	if !contains(calls, want) {
		t.Errorf("Calls = %v, want constructor-scoped form %q", calls, want)
	}
	if !contains(calls, "l.Run") {
		t.Errorf("Calls = %v, want legacy %q kept (secondary global-fallback path)", calls, "l.Run")
	}
	// The constructor calls themselves ride FIX 1 canonicalization.
	if !contains(calls, "evidence-ladder.New") || !contains(calls, "evidence-ladder.WithClock") {
		t.Errorf("Calls = %v, want canonicalized constructor calls evidence-ladder.{New,WithClock}", calls)
	}
}
