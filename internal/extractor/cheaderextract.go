package extractor

import (
	"strings"

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

// NewCHeaderExtractor returns a header router over fresh CGO C and C++
// extractors: the incumbent lang-parity measures the C and C++ ports against
// (cmd/indexbench langparity.go). Fusion dispatches NewCHeaderRouter.
func NewCHeaderExtractor() *CHeaderExtractor {
	return &CHeaderExtractor{c: NewCExtractor(), cpp: NewCppExtractor()}
}

// NewCHeaderRouter returns the header router over the given C and C++
// extractors, the ones fusion serves .c and .cpp with (LB-F119): since the C
// tags port replaced cextract.go in ingest (lang-parity MERGE, ADR-048
// amendment D1), a header the sniff reads as C goes to the port.
func NewCHeaderRouter(c, cpp Extractor) *CHeaderExtractor {
	return &CHeaderExtractor{c: c, cpp: cpp}
}

// Route is the extractor Extract would delegate src to (the one-parse stage
// asks whether it is a tags extractor).
func (h *CHeaderExtractor) Route(src []byte) Extractor {
	if looksLikeCpp(src) {
		return h.cpp
	}
	return h.c
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

// LooksLikeCppHeader is the header router's verdict (looksLikeCpp), exported
// for the measurement harness: indexbench lang-oracle measures the C++
// dialect on the .h files this sniff sends to C++ (no dialect claims .h).
func LooksLikeCppHeader(src []byte) bool { return looksLikeCpp(src) }

// looksLikeCpp reports whether src contains a C++-only signal: a signal
// keyword at a word boundary, or the scope operator "::".
func looksLikeCpp(src []byte) bool {
	s := codeOnly(src)
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

// codeOnly returns src with every comment, string literal and character
// literal blanked to spaces (newlines kept), so the signal sniff reads code
// only: a header whose comment says "a class of handles" is not C++ (O365).
// A quote with no closing quote on its line (an apostrophe in a #error
// message) and a digit separator (1'000) open no literal.
func codeOnly(src []byte) string {
	b := make([]byte, len(src))
	copy(b, src)
	blank := func(from, to int) {
		for k := from; k < to; k++ {
			if b[k] != '\n' {
				b[k] = ' '
			}
		}
	}
	for i := 0; i < len(b); {
		switch {
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '/':
			j := i
			for j < len(b) && b[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '*':
			j := i + 2
			for j+1 < len(b) && !(b[j] == '*' && b[j+1] == '/') {
				j++
			}
			j = min(j+2, len(b))
			blank(i, j)
			i = j
		case b[i] == '#' && directiveText(b, i):
			j := i
			for j < len(b) && b[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j
		case b[i] == '"' || (b[i] == '\'' && (i == 0 || !identByte(b[i-1]))):
			q := b[i]
			j := i + 1
			for j < len(b) && b[j] != q && b[j] != '\n' {
				if b[j] == '\\' && j+1 < len(b) && b[j+1] != '\n' {
					j++
				}
				j++
			}
			if j < len(b) && b[j] == q {
				blank(i, j+1)
				i = j + 1
			} else {
				i++
			}
		default:
			i++
		}
	}
	return string(b)
}

// directiveText reports whether the '#' at b[i] opens a #error or #warning
// line: its rest is a message, never code.
func directiveText(b []byte, i int) bool {
	for k := i - 1; k >= 0 && b[k] != '\n'; k-- {
		if b[k] != ' ' && b[k] != '\t' {
			return false
		}
	}
	rest := strings.TrimLeft(string(b[i+1:min(i+16, len(b))]), " \t")
	return strings.HasPrefix(rest, "error") || strings.HasPrefix(rest, "warning")
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
