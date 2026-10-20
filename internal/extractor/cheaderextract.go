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
//
// A header that is Objective-C (looksLikeObjC) goes to the Objective-C
// extractor first, when the router holds one (WithObjC): the same router,
// never a second classifier (LB-Q-objective-c).
type CHeaderExtractor struct {
	c    Extractor
	cpp  Extractor
	objc Extractor
}

// NewCHeaderRouter returns the header router over the given C and C++
// extractors, the ones fusion serves .c and .cpp with (LB-F119, LB-F117): the
// c and cpp tags dialects, so a header the sniff reads as C goes to the C
// dialect and one it reads as C++ to the C++ dialect. The CGO router over
// cextract.go and cppextract.go that lang-parity measured them against is
// retired (LB-F392; docs/bench/readings/lang/parity/c-headers.json and
// cpp-headers.json read MERGE on the headers).
func NewCHeaderRouter(c, cpp Extractor) *CHeaderExtractor {
	return &CHeaderExtractor{c: c, cpp: cpp}
}

// WithObjC gives the router its Objective-C extractor: a header with an
// @interface, @protocol or @implementation, or an #import and no C++ signal,
// is read by it instead of by C or C++. It returns the router.
func (h *CHeaderExtractor) WithObjC(objc Extractor) *CHeaderExtractor {
	h.objc = objc
	return h
}

// ObjC is the router's Objective-C extractor, nil when it holds none.
func (h *CHeaderExtractor) ObjC() Extractor { return h.objc }

// Routes is the router's two extractors, the C one and the C++ one, for a
// caller that names what a .h dispatches to (fusion.DispatchTable).
func (h *CHeaderExtractor) Routes() (c, cpp Extractor) { return h.c, h.cpp }

// Route is the extractor Extract would delegate src to (the one-parse stage
// asks whether it is a tags extractor).
func (h *CHeaderExtractor) Route(src []byte) Extractor {
	if h.objc != nil && looksLikeObjC(src) {
		return h.objc
	}
	if looksLikeCpp(src) {
		return h.cpp
	}
	return h.c
}

// Extract sniffs src and delegates to the matching grammar.
func (h *CHeaderExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	return h.Route(src).Extract(service, relPath, src)
}

// cppSignalWords are keywords that never appear in C source as keywords.
// Matched at word boundaries so identifiers like "subclass_count" or
// "template_id" never trigger the C++ route.
var cppSignalWords = []string{
	"class", "namespace", "template", "virtual",
	"public:", "private:", "protected:", "constexpr",
}

// HeaderDialect is the dialect the header router reads a .h with: "objective-c"
// when looksLikeObjC says so, else "cpp" on a C++ signal, else "c". It is the
// one verdict the router, the file resolver and the measurement harness share.
func HeaderDialect(src []byte) string {
	switch {
	case looksLikeObjC(src):
		return "objective-c"
	case looksLikeCpp(src):
		return "cpp"
	}
	return "c"
}

// looksLikeObjC reports whether a header is Objective-C: its code (comments,
// strings and #error lines blanked) carries an @interface, @protocol or
// @implementation, or an #import directive and no C++ signal (an MSVC
// `#import <comdef.h>` header is C++; a header of C declarations that
// #imports Foundation is Objective-C).
func looksLikeObjC(src []byte) bool {
	s := codeOnly(src)
	for _, kw := range []string{"@interface", "@protocol", "@implementation"} {
		for from := 0; ; {
			i := indexFrom(s, kw, from)
			if i < 0 {
				break
			}
			if end := i + len(kw); end >= len(s) || !identByte(s[end]) {
				return true
			}
			from = i + 1
		}
	}
	return hasImportDirective(s) && !looksLikeCpp(src)
}

// hasImportDirective reports whether s has a line whose first token is
// #import (whitespace allowed around the #).
func hasImportDirective(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(t, "#") {
			continue
		}
		t = strings.TrimLeft(t[1:], " \t")
		if strings.HasPrefix(t, "import") && (len(t) == len("import") || !identByte(t[len("import")])) {
			return true
		}
	}
	return false
}

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
