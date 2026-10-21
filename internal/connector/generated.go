package connector

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strings"
)

// goGeneratedRe is the OFFICIAL Go generated-file convention
// (https://go.dev/s/generatedcode): a `//`-comment line of exactly the form
// "// Code generated ... DO NOT EDIT." — start-anchored on the comment
// token, end-anchored on the trailing period — appearing before the first
// non-comment, non-blank text of the file (in practice: before the package
// clause; k8s's zz_generated ecosystem places it at line ~17, after a full
// license header). protoc-gen-go, mockgen, and stringer all emit this exact
// form.
var goGeneratedRe = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// generatedMarkerRe matches the marker text itself for non-Go languages
// (case-insensitive, whitespace-tolerant); the comment-token requirement in
// IsGenerated supplies the "this is a header comment, not quoted prose"
// signal that the loose regex alone cannot.
var generatedMarkerRe = regexp.MustCompile(`(?i)code generated .*\bdo not edit\b`)
var generatedStartRe = regexp.MustCompile(`(?i)code generated `)
var generatedEndRe = regexp.MustCompile(`(?i)\bdo not edit\b`)

// goGeneratedScanLimit bounds the Go package-clause scan: the marker (and
// the package clause itself) live in the file header; 4KB comfortably
// covers any license preamble without scanning megabytes of generated body.
const goGeneratedScanLimit = 4096

// generatedScanLines is the number of leading lines inspected for non-Go
// languages, whose generators put the marker at the very top.
const generatedScanLines = 3

// langCommentTokens maps a connector language tag (LanguageForPath) to the
// line-comment tokens a generated-file marker may legitimately start with.
// A language with NO tokens (json) can never be marker-flagged; markdown's
// only honored comment form is an HTML comment, so a markdown file QUOTING
// the marker as text or in a fenced code block is not flagged (critic P16).
var langCommentTokens = map[string][]string{
	"go":         {"//"},
	"js":         {"//"},
	"ts":         {"//"},
	"java":       {"//"},
	"cs":         {"//"},
	"c":          {"//", "/*", " *"},
	"cpp":        {"//", "/*", " *"},
	"rust":       {"//"},
	"scala":      {"//"},
	"php":        {"//", "#", "/*"},
	"proto":      {"//"},
	"graphql":    {"#"},
	"py":         {"#"},
	"ruby":       {"#"},
	"sh":         {"#"},
	"yaml":       {"#"},
	"toml":       {"#"},
	"dockerfile": {"#"},
	"cue":        {"//"},
	"templ":      {"//"},
	"terraform":  {"#", "//"},
	"sql":        {"--"},
	"lua":        {"--"},
	"md":         {"<!--"},
	"json":       {},
}

// unclassifiedCommentTokens is the permissive token set used for files whose
// language the registry does not know: the common line-comment forms.
var unclassifiedCommentTokens = []string{"//", "#", "--", ";", "/*", "<!--"}

// IsGenerated reports whether the file at path (content given) carries a
// "Code generated ... DO NOT EDIT" marker (ADR-021 §2 / TI-5, reworked per
// critic F4).
//
// Go files follow the official convention: the exact `// Code generated ...
// DO NOT EDIT.` line anywhere before the package clause (bounded to the
// first 4KB) — NOT just the first lines, since the k8s zz_generated
// ecosystem puts a license header first.
//
// Every other language scans only the first few lines AND requires the
// marker line to begin with one of that language's comment tokens — so
// markdown or YAML values QUOTING the marker are not flagged. Residual
// (documented, accepted): a file whose language is unknown to the registry
// uses a permissive token set, so an unrecognized text file that quotes the
// marker verbatim on a comment-looking line in its first lines (e.g. a
// fenced code sample at the very top of a .markdown file, which the
// registry does not know) can still false-positive.
//
// Generated files are still extracted normally (unlike binary content,
// which IsBinary excludes outright): IsGenerated only tags the result so
// cloc and review tooling can separate "code you write" from "code a tool
// wrote for you" without re-deriving the signal.
func IsGenerated(path string, content []byte) bool {
	if len(content) == 0 {
		return false
	}
	if strings.HasSuffix(path, ".go") {
		return isGeneratedGo(content)
	}

	lang := LanguageForPath(path)
	tokens, known := langCommentTokens[lang]
	if !known {
		tokens = unclassifiedCommentTokens
	}
	if len(tokens) == 0 {
		return false
	}

	lines := bytes.SplitN(content, []byte("\n"), generatedScanLines+1)
	limit := len(lines)
	if limit > generatedScanLines {
		limit = generatedScanLines
	}
	for i := 0; i < limit; i++ {
		line := bytes.TrimLeft(lines[i], " \t")
		if !hasAnyPrefix(line, tokens) {
			continue
		}
		if generatedMarkerRe.Match(line) {
			return true
		}
	}
	return false
}

// IsGeneratedReader makes the same decision as IsGenerated while retaining at
// most a small read buffer and marker overlap. Non-Go's first three lines may
// be arbitrarily long; they are streamed rather than truncated.
func IsGeneratedReader(path string, r io.Reader) (bool, error) {
	if strings.HasSuffix(path, ".go") {
		head, err := io.ReadAll(io.LimitReader(r, goGeneratedScanLimit))
		if err != nil {
			return false, err
		}
		return isGeneratedGo(head), nil
	}
	lang := LanguageForPath(path)
	tokens, known := langCommentTokens[lang]
	if !known {
		tokens = unclassifiedCommentTokens
	}
	if len(tokens) == 0 {
		return false, nil
	}
	reader := bufio.NewReaderSize(r, 4*1024)
	for line := 0; line < generatedScanLines; line++ {
		state := generatedLine{tokens: tokens, leading: true}
		for {
			fragment, err := reader.ReadSlice('\n')
			if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
				return false, err
			}
			last := err != bufio.ErrBufferFull
			if len(fragment) > 0 && fragment[len(fragment)-1] == '\n' {
				fragment = fragment[:len(fragment)-1]
			}
			if state.scan(fragment, last) {
				return true, nil
			}
			if err == io.EOF {
				return false, nil
			}
			if last {
				break
			}
		}
	}
	return false, nil
}

type generatedLine struct {
	tokens    []string
	leading   bool
	decided   bool
	allowed   bool
	prefix    []byte // comment token only, at most five bytes
	seenStart bool
	startEnd  int
	processed int
	tail      []byte // enough overlap for either fixed marker fragment
}

func (s *generatedLine) scan(fragment []byte, final bool) bool {
	if !s.decided {
		for i, b := range fragment {
			if s.leading && (b == ' ' || b == '\t') {
				continue
			}
			s.leading = false
			s.prefix = append(s.prefix, b)
			if hasAnyPrefix(s.prefix, s.tokens) {
				s.allowed, s.decided = true, true
				return s.marker(append(s.prefix, fragment[i+1:]...), final)
			}
			possible := false
			for _, token := range s.tokens {
				if bytes.HasPrefix([]byte(token), s.prefix) {
					possible = true
					break
				}
			}
			if !possible {
				s.decided = true
				return false
			}
		}
		return false
	}
	if !s.allowed {
		return false
	}
	return s.marker(fragment, final)
}

func (s *generatedLine) marker(fragment []byte, final bool) bool {
	windowStart := s.processed - len(s.tail)
	window := make([]byte, 0, len(s.tail)+len(fragment))
	window = append(window, s.tail...)
	window = append(window, fragment...)
	if !s.seenStart {
		if m := generatedStartRe.FindIndex(window); m != nil {
			s.seenStart = true
			s.startEnd = windowStart + m[1]
		}
	}
	if s.seenStart {
		for _, m := range generatedEndRe.FindAllIndex(window, -1) {
			if windowStart+m[0] >= s.startEnd && (m[0] > 0 || windowStart == 0) && (m[1] < len(window) || final) {
				return true
			}
		}
	}
	s.processed += len(fragment)
	if len(window) > 64 {
		window = window[len(window)-64:]
	}
	s.tail = append(s.tail[:0], window...)
	return false
}

// isGeneratedGo scans Go content line-by-line for the official convention
// line, stopping at the package clause (the marker must precede it) or the
// scan limit.
func isGeneratedGo(content []byte) bool {
	head := content
	if len(head) > goGeneratedScanLimit {
		head = head[:goGeneratedScanLimit]
	}
	for _, line := range bytes.Split(head, []byte("\n")) {
		trimmed := bytes.TrimRight(line, "\r")
		if goGeneratedRe.Match(trimmed) {
			return true
		}
		if bytes.HasPrefix(trimmed, []byte("package ")) {
			return false
		}
	}
	return false
}

// hasAnyPrefix reports whether line starts with any of the given tokens.
func hasAnyPrefix(line []byte, tokens []string) bool {
	for _, tok := range tokens {
		if bytes.HasPrefix(line, []byte(tok)) {
			return true
		}
	}
	return false
}
