package extractor

import (
	"github.com/geoffmilleraz/fathom/internal/core"
)

// CHeaderExtractor routes the polyglot .h extension (docs/plans/
// cpp-toolchain.md CT-3). C and C++ share it, and entire C++ codebases —
// LLVM among them — keep every header in .h; routing those to the C grammar
// silently drops classes, namespaces, templates and methods. The router
// sniffs the source for C++-only constructs and delegates to the C++
// extractor on any hit, otherwise to the C extractor.
//
// The sniff is a word-boundary token scan, not a parse: false positives are
// bounded (tree-sitter-cpp parses C acceptably — functions and structs still
// extract, under Cpp kinds), false negatives degrade to C extraction, which
// still yields functions/structs/enums. Both failure modes lose precision,
// never symbols.
type CHeaderExtractor struct {
	c   Extractor
	cpp Extractor
}

// NewCHeaderExtractor returns a header router over fresh C and C++ extractors.
// (LB-Q-c built and measured a tree-sitter "c" dialect alongside these CGO
// walkers — internal/extractor/dialects/c/ — but its L4 target is not met,
// so §11 step 2's merge gate keeps the CGO incumbent as the live dispatch
// here; see docs/bench/readings/lang/c.json and pipeline.go's own note.)
func NewCHeaderExtractor() *CHeaderExtractor {
	return &CHeaderExtractor{c: NewCExtractor(), cpp: NewCppExtractor()}
}

// Extract sniffs src and delegates to the matching grammar.
func (h *CHeaderExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	if looksLikeCpp(src) {
		return h.cpp.Extract(service, relPath, src)
	}
	return h.c.Extract(service, relPath, src)
}

// cppSignalWords are keywords that never appear in C source as keywords.
// Matched at word boundaries so identifiers like "subclass_count" or
// "template_id" never trigger the C++ route.
var cppSignalWords = []string{
	"class", "namespace", "template", "virtual",
	"public:", "private:", "protected:", "constexpr",
}

// looksLikeCpp reports whether src contains a C++-only signal: a signal
// keyword at a word boundary, or the scope operator "::".
func looksLikeCpp(src []byte) bool {
	s := string(src)
	for i := 0; i+1 < len(s); i++ {
		if s[i] == ':' && s[i+1] == ':' {
			return true
		}
	}
	for _, w := range cppSignalWords {
		if hasWord(s, w) {
			return true
		}
	}
	return false
}

// hasWord reports whether word occurs in s with non-identifier characters
// (or string edges) on both sides. The trailing-colon signals ("public:")
// end in a non-identifier byte already, so only their left edge matters.
func hasWord(s, word string) bool {
	for from := 0; ; {
		i := indexFrom(s, word, from)
		if i < 0 {
			return false
		}
		leftOK := i == 0 || !identByte(s[i-1])
		right := i + len(word)
		rightOK := right >= len(s) || !identByte(s[right])
		if leftOK && rightOK {
			return true
		}
		from = i + 1
	}
}

// indexFrom is strings.Index starting at from.
func indexFrom(s, sub string, from int) int {
	if from >= len(s) {
		return -1
	}
	for i := from; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// identByte reports whether b can appear in a C identifier.
func identByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
