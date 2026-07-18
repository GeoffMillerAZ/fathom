package integrityevaluator

import (
	"fmt"
	"sort"
	"strings"
)

// Evaluate checks output against the knownRefs map and returns a Report.
//
// Parameters:
//   - output is the text produced by the model.
//   - knownRefs maps reference identifiers (e.g. "[1]", "doc:foo") to their
//     resolved values. A ref is "resolvable" when its value is non-empty.
//
// A reference is detected in the output by scanning for every key in
// knownRefs. References that appear in the output but are NOT in knownRefs
// are reported as ViolationFabricatedRef. References whose resolved value is
// empty are reported as ViolationUnresolvableRef.
//
// The report is deterministic: violations are ordered by ref string then kind.
func Evaluate(output string, knownRefs map[string]string) Report {
	var violations []Violation

	// Collect refs that appear in the output.
	for ref, resolved := range knownRefs {
		if !strings.Contains(output, ref) {
			continue
		}
		if resolved == "" {
			violations = append(violations, Violation{
				Kind:   ViolationUnresolvableRef,
				Ref:    ref,
				Detail: fmt.Sprintf("reference %q is present in knownRefs but resolves to an empty value", ref),
			})
		}
	}

	// Detect any refs that appear in the output but are absent from knownRefs.
	// We use a simple bracket-based heuristic: scan for [word] and doc:word
	// tokens and check if they look like citation keys. This is intentionally
	// conservative — it only flags tokens that follow a recognised pattern.
	citedRefs := extractCitedRefs(output)
	for _, ref := range citedRefs {
		if _, known := knownRefs[ref]; !known {
			violations = append(violations, Violation{
				Kind:   ViolationFabricatedRef,
				Ref:    ref,
				Detail: fmt.Sprintf("reference %q appears in output but is not present in knownRefs", ref),
			})
		}
	}

	// Sort for determinism: primary key = Ref, secondary = Kind.
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Ref != violations[j].Ref {
			return violations[i].Ref < violations[j].Ref
		}
		return violations[i].Kind < violations[j].Kind
	})

	return Report{
		Passed:     len(violations) == 0,
		Violations: violations,
	}
}

// extractCitedRefs returns the unique set of reference-like tokens found in
// text. It recognises two patterns:
//
//   - Bracket citations: [<token>] where <token> contains no spaces or brackets.
//   - Prefixed citations: doc:<token> where <token> is alphanumeric with dashes/underscores.
func extractCitedRefs(text string) []string {
	seen := make(map[string]struct{})

	// Scan for [token] patterns.
	for i := 0; i < len(text); i++ {
		if text[i] != '[' {
			continue
		}
		end := strings.IndexByte(text[i+1:], ']')
		if end < 0 {
			continue
		}
		token := text[i+1 : i+1+end]
		if token != "" && !strings.ContainsAny(token, "[ \t\n") {
			seen["["+token+"]"] = struct{}{}
		}
		i += end + 1
	}

	// Scan for doc:<token> patterns.
	rest := text
	for {
		idx := strings.Index(rest, "doc:")
		if idx < 0 {
			break
		}
		after := rest[idx+4:]
		end := strings.IndexFunc(after, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
				r >= '0' && r <= '9' || r == '-' || r == '_')
		})
		var token string
		if end < 0 {
			token = after
		} else {
			token = after[:end]
		}
		if token != "" {
			seen["doc:"+token] = struct{}{}
		}
		rest = rest[idx+4+len(token):]
		if len(rest) == 0 {
			break
		}
	}

	refs := make([]string, 0, len(seen))
	for r := range seen {
		refs = append(refs, r)
	}
	sort.Strings(refs)
	return refs
}
