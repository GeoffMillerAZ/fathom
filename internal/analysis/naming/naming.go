// Package naming checks Go symbol naming conventions against []core.SymbolNode.
//
// Rules applied:
//   - Exported identifiers (first rune uppercase) must be PascalCase (no underscores).
//   - Unexported identifiers must not contain underscores (except leading _ blank).
//   - Acronyms in identifiers should be all-caps (e.g. ID not Id, URL not Url).
//   - Interface type names should end with "er" (idiomatic Go; advisory).
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package naming

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// Issue is a single naming-convention violation.
type Issue struct {
	Symbol  string
	Rule    string
	Message string
}

// wellKnownAcronyms is the set of common acronyms that should appear fully
// upper-cased in Go identifiers.
var wellKnownAcronyms = []string{
	"ACL", "API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML",
	"HTTP", "HTTPS", "ID", "IP", "JSON", "LHS", "QPS", "RAM", "RHS",
	"RPC", "SLA", "SMTP", "SQL", "SSH", "TCP", "TLS", "TTL", "UDP",
	"UI", "UID", "UUID", "URI", "URL", "UTF8", "VM", "XML", "XMPP",
	"XSRF", "XSS",
}

// mixedCaseAcronymVariants maps lower or title-case variants → correct form.
// e.g. "Id" → "ID", "Url" → "URL"
var mixedCaseAcronymVariants map[string]string

func init() {
	mixedCaseAcronymVariants = make(map[string]string, len(wellKnownAcronyms)*2)
	for _, a := range wellKnownAcronyms {
		// Title case variant, e.g. "Id", "Url", "Http"
		if len(a) > 0 {
			title := strings.ToUpper(a[:1]) + strings.ToLower(a[1:])
			mixedCaseAcronymVariants[title] = a
		}
	}
}

// Check inspects nodes for Go naming-convention violations and returns a
// deterministic (sorted by Symbol then Rule) slice of Issues.
func Check(nodes []core.SymbolNode) []Issue {
	var issues []Issue

	for _, n := range nodes {
		name := n.Name
		if name == "" || name == "_" {
			continue
		}

		exported := unicode.IsUpper([]rune(name)[0])

		// Rule: exported identifiers must be PascalCase — no underscores.
		if exported && strings.Contains(name, "_") {
			issues = append(issues, Issue{
				Symbol:  n.Name,
				Rule:    "exported-no-underscore",
				Message: fmt.Sprintf("exported identifier %q must not contain underscores", name),
			})
		}

		// Rule: unexported identifiers must not contain underscores (except
		// the blank identifier "_" already excluded above).
		if !exported && strings.Contains(name, "_") {
			issues = append(issues, Issue{
				Symbol:  n.Name,
				Rule:    "unexported-no-underscore",
				Message: fmt.Sprintf("unexported identifier %q should not contain underscores; use camelCase", name),
			})
		}

		// Rule: acronyms should be all-caps — detect title-cased acronym
		// segments (e.g. "GetId" should be "GetID").
		if bad, correct := findBadAcronym(name); bad != "" {
			issues = append(issues, Issue{
				Symbol:  n.Name,
				Rule:    "acronym-caps",
				Message: fmt.Sprintf("identifier %q contains %q which should be %q", name, bad, correct),
			})
		}

		// Rule: interface names should be single-word names ending with "er"
		// (idiomatic Go: Reader, Writer, Storer). Compound names like
		// UserManager or multi-segment names violate this convention.
		if n.Kind == core.KindGoInterface && exported {
			words := splitPascalWords(name)
			singleWord := len(words) == 1
			endsWithEr := strings.HasSuffix(name, "er")
			if !singleWord || !endsWithEr {
				issues = append(issues, Issue{
					Symbol:  n.Name,
					Rule:    "interface-er-suffix",
					Message: fmt.Sprintf("interface %q should be a single-word name ending with \"er\" (idiomatic Go, e.g. %ser)", name, words[len(words)-1]),
				})
			}
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Symbol != issues[j].Symbol {
			return issues[i].Symbol < issues[j].Symbol
		}
		return issues[i].Rule < issues[j].Rule
	})
	return issues
}

// findBadAcronym scans name for a title-cased acronym variant used as a
// PascalCase segment and returns the bad segment and its correct all-caps form.
// Returns ("", "") if none found.
//
// A variant matches when it appears as a complete CamelCase word-segment:
// the character immediately before (if any) must be upper-case (start of
// previous segment), and the character immediately after (if any) must be
// upper-case, a digit, or end-of-string.
func findBadAcronym(name string) (bad, correct string) {
	runes := []rune(name)
	n := len(runes)

	for variant, canon := range mixedCaseAcronymVariants {
		varRunes := []rune(variant)
		vlen := len(varRunes)
		if vlen > n {
			continue
		}
		for i := 0; i <= n-vlen; i++ {
			// Check substring match.
			match := true
			for j := 0; j < vlen; j++ {
				if runes[i+j] != varRunes[j] {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			after := i + vlen
			// The variant starts a new PascalCase word segment when:
			//   - it is at position 0, OR
			//   - the preceding rune is a lowercase letter or digit
			//     (i.e. it's the start of a new segment following a previous one).
			prevOK := i == 0 || unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])
			// The variant ends a word segment when:
			//   - it reaches end-of-string, OR
			//   - the next rune is uppercase (start of next segment).
			afterOK := after == n || unicode.IsUpper(runes[after])
			if prevOK && afterOK {
				return variant, canon
			}
		}
	}
	return "", ""
}

// splitPascalWords splits a PascalCase name into its constituent word segments.
// e.g. "UserManager" → ["User", "Manager"], "Storer" → ["Storer"].
func splitPascalWords(name string) []string {
	if name == "" {
		return nil
	}
	runes := []rune(name)
	var words []string
	start := 0
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	words = append(words, string(runes[start:]))
	return words
}
