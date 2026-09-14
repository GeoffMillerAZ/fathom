package main

// promptcandidates.go — extracting the paths, packages and bare symbol
// names a UserPromptSubmit prompt names (O57 items 4/5), for prompt-hook
// to resolve through the index. Deliberately conservative: a false
// negative costs one fewer rule surfaced, a false positive wastes an RPC
// on garbage — never a wrong governing rule, since every candidate still
// has to resolve through the real index (fetchObligations / find_symbol)
// before anything is served.

import "strings"

// promptCandidateKind classifies one extracted token.
type promptCandidateKind int

const (
	candidatePath promptCandidateKind = iota
	candidatePackage
	candidateSymbol
)

// promptCandidate is one token extracted from a prompt, classified but not
// yet resolved.
type promptCandidate struct {
	kind promptCandidateKind
	text string
}

// promptTokenCutset is the punctuation a token gets trimmed of at both
// ends before classification — prose wraps a path in backticks, quotes,
// or trailing punctuation ("fix internal/store/backend.go.", "the file
// `internal/mcp/tools.go`,") and none of it is part of the name.
const promptTokenCutset = "`'\"“”‘’.,;:!?()[]{}<>"

// extractPromptCandidates scans prompt for path-, package- and symbol-
// shaped tokens, in first-seen order, deduplicated, capped at
// maxPromptCandidates. It never reads the index — classification is
// purely lexical; resolution (and therefore the final word on whether a
// token means anything) happens downstream.
func extractPromptCandidates(prompt string) []promptCandidate {
	seen := map[string]bool{}
	var out []promptCandidate
	for _, raw := range strings.Fields(prompt) {
		if len(out) >= maxPromptCandidates {
			break
		}
		// A prompt marks code two ways Fields+Trim alone can't preserve on
		// tok itself: wrapped in backticks ("`core.Edge`"), or immediately
		// called ("registerTool(ctx)" — the "(" is cut off here, before
		// promptTokenCutset would otherwise mangle it into the middle of
		// the token instead of trimming a clean end).
		backticked := strings.HasPrefix(raw, "`") && strings.Count(raw, "`") >= 2
		work := raw
		calledLike := false
		if idx := strings.IndexByte(raw, '('); idx > 0 {
			work = raw[:idx]
			calledLike = true
		}
		tok := strings.Trim(work, promptTokenCutset)
		if tok == "" || seen[tok] {
			continue
		}
		switch {
		case looksLikeFilePath(tok):
			// No slash requirement here (unlike looksLikePackagePath
			// below): a bare same-directory filename ("README.md" with no
			// path prefix) is still a file, not a directory.
			seen[tok] = true
			out = append(out, promptCandidate{candidatePath, tok})
		case looksLikePackagePath(tok):
			seen[tok] = true
			out = append(out, promptCandidate{candidatePackage, tok})
		case looksLikeSymbol(tok, backticked, calledLike):
			seen[tok] = true
			out = append(out, promptCandidate{candidateSymbol, tok})
		}
	}
	return out
}

// looksLikeFilePath reports whether tok's last path segment has a short,
// alphanumeric extension — "internal/store/backend.go", "cmd/fathom/
// CLAUDE.md" — and is not itself a URL (http(s):// tokens are never file
// candidates; nothing in this repo's obligations scope matches one).
func looksLikeFilePath(tok string) bool {
	if strings.Contains(tok, "://") {
		return false
	}
	base := tok
	if i := strings.LastIndexByte(tok, '/'); i >= 0 {
		base = tok[i+1:]
	}
	dot := strings.LastIndexByte(base, '.')
	if dot <= 0 || dot == len(base)-1 {
		return false
	}
	ext := base[dot+1:]
	if len(ext) == 0 || len(ext) > 8 {
		return false
	}
	for _, r := range ext {
		if r >= 'A' && r <= 'Z' {
			// An uppercase "extension" is never a real one ("go", "md",
			// "json" — every extension in this repo's own tree is
			// lowercase); it reads as Type.Method instead ("Store.Get"),
			// which is a symbol shape (looksLikeSymbol), not a file one —
			// this is what keeps a bare, no-slash dotted token like
			// "README.md" a file without also swallowing "Store.Get".
			return false
		}
		if !isASCIILetterOrDigit(r) {
			return false
		}
	}
	return true
}

// looksLikePackagePath reports whether tok is a bare, extension-free,
// multi-segment slash path ("internal/store", "cmd/fathom") — a package
// or directory mention rather than a file. At least two segments so a
// single lowercase word ("docs", "and/or") is never mistaken for one.
func looksLikePackagePath(tok string) bool {
	if !strings.Contains(tok, "/") || strings.Contains(tok, "://") {
		return false
	}
	if looksLikeFilePath(tok) {
		return false // already classified as a file
	}
	segs := strings.Split(strings.Trim(tok, "/"), "/")
	if len(segs) < 2 {
		return false
	}
	for _, s := range segs {
		if s == "" {
			return false
		}
		for _, r := range s {
			if !(isASCIILetterOrDigit(r) || r == '_' || r == '-' || r == '.') {
				return false
			}
		}
	}
	return true
}

// looksLikeSymbol reports whether tok reads as a code identifier rather
// than an English word. A single leading capital is NOT enough — every
// capitalized sentence word ("Update", "Edge", "Fixed", or "README" in
// "README.md") satisfies that trivially, and a prompt exploring a design
// is FULL of them; admitting all of them starves the real symbols a
// prompt names out of maxPromptSymbolLookups' few slots. tok counts as a
// symbol only when the PROMPT marked it as code some other way:
// backticked or immediately called (both decided by the caller from the
// original token, since trimming has already erased backticks and a
// trailing "(...)" by the time tok reaches here), package-qualified
// (dotted, "pkg.Name" or "Type.Method" shaped), CamelCase/mixedCase with
// an internal capital — one not merely the first rune ("FindSymbol",
// "registerTool") — or snake_case with a letter on either side of an
// underscore ("max_rules"). At least 4 characters, no slash, no more
// than one embedded dot (a URL fragment or a sentence with a mid-token
// period is not this shape).
func looksLikeSymbol(tok string, backticked, calledLike bool) bool {
	if len(tok) < 4 || strings.ContainsAny(tok, "/@#") {
		return false
	}
	if strings.Count(tok, ".") > 1 {
		return false
	}
	for _, r := range tok {
		if !(isASCIILetterOrDigit(r) || r == '_' || r == '.') {
			return false
		}
	}
	if backticked || calledLike {
		return true
	}
	if isDotQualifiedSymbol(tok) {
		return true
	}
	if hasInternalCapital(tok) {
		return true
	}
	return hasUnderscoreLowerShape(tok)
}

// isDotQualifiedSymbol reports whether tok is a single-dot, two-segment
// identifier shape ("core.Edge", "Store.Get") — a package- or
// type-qualified reference, regardless of which side carries the capital.
func isDotQualifiedSymbol(tok string) bool {
	i := strings.IndexByte(tok, '.')
	if i <= 0 || i == len(tok)-1 {
		return false
	}
	return isIdentifierShape(tok[:i]) && isIdentifierShape(tok[i+1:])
}

func isIdentifierShape(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 && !(isASCIILetterOrDigit(r) || r == '_') {
			return false
		}
		if !(isASCIILetterOrDigit(r) || r == '_') {
			return false
		}
	}
	return true
}

// hasInternalCapital reports whether tok has at least one lowercase
// letter AND at least one uppercase letter that is not merely tok's
// first rune — the shape that separates "FindSymbol"/"registerTool" from
// "Update"/"Edge", which carry a capital only because they open a
// sentence-cased word.
func hasInternalCapital(tok string) bool {
	hasLower, hasUpperAfterFirst := false, false
	for i, r := range tok {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z' && i > 0:
			hasUpperAfterFirst = true
		}
	}
	return hasLower && hasUpperAfterFirst
}

// hasUnderscoreLowerShape reports the snake_case admission this
// heuristic already covered before this defect: an underscore with a
// lowercase letter somewhere in tok too.
func hasUnderscoreLowerShape(tok string) bool {
	hasLower, hasUnderscore := false, false
	for _, r := range tok {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r == '_':
			hasUnderscore = true
		}
	}
	return hasUnderscore && hasLower
}

func isASCIILetterOrDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || isASCIIDigit(r)
}

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }
