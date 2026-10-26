package extractor

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// A probe without a pre-pass must not describe malformed source as error-free.
func TestProbeParseReportsWrittenErrorsWithoutPreprocess(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		bad  bool
	}{
		{"clean", "int f(void) { return 1; }", false},
		{"error", "void broken( {", true},
		{"missing", "int value", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := tsengine.NewParser(context.Background(), "c", "tree_sitter_c")
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			res, err := ProbeParse(p, ProbeTarget{}, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Tree.Close()
			if got := res.WrittenErrors > 0; got != tc.bad {
				t.Errorf("WrittenErrors=%d, want errors=%v", res.WrittenErrors, tc.bad)
			}
		})
	}
}
