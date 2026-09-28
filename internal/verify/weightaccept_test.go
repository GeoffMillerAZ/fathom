package verify

// weightaccept_test.go — SI-7/SI-8 (docs/design/spec-interop.md §4, ADR-049):
// the two additive grammar keys. `weight:` is a positive integer, default 1,
// and lint refuses anything else; `accept:` is quoted acceptance text that
// never gates by itself and joins annotationKeys so a mangled block stays
// loud (I22).

import (
	"strings"
	"testing"
)

func TestParseWeightKey(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ann     map[string]string
		want    int
		invalid string
	}{
		{"absent defaults to 1", map[string]string{}, 1, ""},
		{"nil map defaults to 1", nil, 1, ""},
		{"explicit 3", map[string]string{"weight": "3"}, 3, ""},
		{"spaces trimmed", map[string]string{"weight": " 5 "}, 5, ""},
		{"zero refused", map[string]string{"weight": "0"}, 1, `"0"`},
		{"negative refused", map[string]string{"weight": "-2"}, 1, `"-2"`},
		{"fraction refused", map[string]string{"weight": "1.5"}, 1, `"1.5"`},
		{"word refused", map[string]string{"weight": "heavy"}, 1, `"heavy"`},
		{"empty refused", map[string]string{"weight": ""}, 1, `""`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, invalid := parseWeight(tc.ann)
			if got != tc.want || invalid != tc.invalid {
				t.Fatalf("parseWeight(%v) = (%d, %q), want (%d, %q)", tc.ann, got, invalid, tc.want, tc.invalid)
			}
		})
	}

	md := "- [ ] default {id: W-1, symbol: Foo}\n" +
		"- [ ] heavy {id: W-2, symbol: Foo, weight: 4}\n" +
		"- [ ] broken {id: W-3, symbol: Foo, weight: 0}\n" +
		"- [ ] no annotation at all\n"
	p := ParseMarkdownPlan("svc", []byte(md))
	want := []int{1, 4, 1, 1}
	for i, tk := range p.Tasks {
		if tk.Weight != want[i] {
			t.Errorf("task %s Weight = %d, want %d", tk.ID, tk.Weight, want[i])
		}
	}
	if p.Tasks[2].WeightInvalid != `"0"` {
		t.Errorf("W-3 WeightInvalid = %q, want %q", p.Tasks[2].WeightInvalid, `"0"`)
	}

	lint := strings.Join(LintPlan(p), "\n")
	if !strings.Contains(lint, "W-3: weight:") {
		t.Errorf("lint did not refuse weight: 0; got:\n%s", lint)
	}
	if strings.Contains(lint, "W-2: weight") || strings.Contains(lint, "W-1: weight") {
		t.Errorf("lint refused a valid weight; got:\n%s", lint)
	}
}

func TestParseAcceptKey(t *testing.T) {
	if got := parseAccept(nil); got != "" {
		t.Fatalf("parseAccept(nil) = %q, want empty", got)
	}
	if got := parseAccept(map[string]string{"accept": "  WHEN x THE SYSTEM SHALL y  "}); got != "WHEN x THE SYSTEM SHALL y" {
		t.Fatalf("parseAccept trims: got %q", got)
	}

	md := `- [x] retry {id: A-1, symbol: Foo, accept: "WHEN the client times out, THE SYSTEM SHALL retry {three} times"}` + "\n" +
		"- [ ] no accept {id: A-2, symbol: Foo}\n"
	p := ParseMarkdownPlan("svc", []byte(md))
	if got := p.Tasks[0].Accept; got != "WHEN the client times out, THE SYSTEM SHALL retry {three} times" {
		t.Fatalf("A-1 Accept = %q", got)
	}
	if p.Tasks[0].ID != "A-1" || p.Tasks[0].Expect.Symbol != "Foo" {
		t.Fatalf("accept: disturbed the rest of the block: %+v", p.Tasks[0])
	}
	if p.Tasks[1].Accept != "" {
		t.Fatalf("A-2 Accept = %q, want empty", p.Tasks[1].Accept)
	}

	// Never gating by itself: a row whose ONLY annotation beyond its id is
	// accept: carries no contract — it verifies no-contract, not pass.
	only := ParseMarkdownPlan("svc", []byte(`- [x] prose {id: A-3, accept: "THE SYSTEM SHALL work"}`+"\n"))
	if !claimedWithoutContract(only.Tasks[0]) {
		t.Fatalf("an accept:-only claimed row must read as carrying no contract")
	}

	// I22: accept joins annotationKeys, so a mangled accept block is loud.
	found := false
	for _, k := range annotationKeys {
		if k == "accept" {
			found = true
		}
	}
	if !found {
		t.Fatalf("accept is missing from annotationKeys (I22)")
	}
	mangled := ParseMarkdownPlan("svc", []byte("- [ ] x {accept}\n"))
	if mangled.Tasks[0].AnnotationSuspect == "" {
		t.Fatalf("a mangled accept block must be suspect, not silent prose")
	}
}
