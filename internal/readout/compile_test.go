package readout

import (
	"strings"
	"testing"
)

// TestCompileErrorLineVetBudgetAndAmbiguousDefs: vet's prefixed type errors
// are records, the compiler's "too many errors" notice and have/want
// continuations are not; two changed definitions with the same last name
// place no root; roots past the budget are counted, never cut mid-root.
func TestCompileErrorLineVetBudgetAndAmbiguousDefs(t *testing.T) {
	out := strings.Join([]string{
		"# example.com/shop/cart",
		"# [example.com/shop/cart]",
		"vet: ./cart.go:10:20: not enough arguments in call to Add",
		"\thave (int)",
		"\twant (int, int)",
		"cart.go:12:2: fmt.Printf format %d has arg s of wrong type string",
		"cart.go:14:2: too many errors",
	}, "\n")
	recs := CompileErrors(out, nil)
	if len(recs) != 2 || recs[0].Kind != RecordCompileError || field(recs[0], "file") != "cart.go" || field(recs[0], "ident") != "Add" || recs[0].Line != 3 {
		t.Fatalf("records = %+v", recs)
	}
	defs := []CompileDef{{Name: "Ledger.Add", File: "ledger.go", Line: 4}, {Name: "Cart.Add", File: "cart.go", Line: 9}}
	line := CompileErrorLine("go vet", recs, defs, func(string) bool { return false }, 700)
	want := "go vet: 2 errors, 2 roots: the signature of Add, cart.go:10 (1 error); cart.go:12: fmt.Printf format %d has arg s of wrong type string"
	if line != want {
		t.Fatalf("line\n got %q\nwant %q", line, want)
	}
	if got := CompileErrorLine("go vet", recs, defs, nil, 90); got != "go vet: 2 errors, 2 roots: the signature of Add, cart.go:10 (1 error); and 1 more root" {
		t.Fatalf("budget 90: %q", got)
	}
	if got := CompileErrorLine("go vet", recs, defs, nil, 85); len(got) > 85 || !strings.HasSuffix(got, compileCut+"; and 1 more root") {
		t.Fatalf("budget 85: the leading root is cut and says so: %q (%d bytes)", got, len(got))
	}
	if got := CompileErrorLine("go build", nil, nil, nil, 700); got != "" {
		t.Fatalf("no records: %q", got)
	}
	// Same input, same bytes.
	if a, b := CompileErrorLine("go vet", recs, defs, nil, 700), CompileErrorLine("go vet", recs, defs, nil, 700); a != b {
		t.Fatalf("not deterministic: %q vs %q", a, b)
	}
}
