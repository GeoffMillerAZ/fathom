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
		tok := strings.Trim(raw, promptTokenCutset)
		if tok == "" || seen[tok] {
			continue
		}
		switch {
		case strings.Contains(tok, "/") && looksLikeFilePath(tok):
			seen[tok] = true
			out = append(out, promptCandidate{candidatePath, tok})
		case looksLikePackagePath(tok):
			seen[tok] = true
			out = append(out, promptCandidate{candidatePackage, tok})
		case looksLikeSymbol(tok):
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
// than an English word: CamelCase/mixedCase, or snake_case with a letter
// on either side of an underscore, at least 4 characters, no slash or
// dot-as-sentence-punctuation. A plain lowercase or uppercase word
// ("fix", "REFUSE") is not a symbol reference — it costs a wasted
// find_symbol call for nothing, which is exactly what maxPromptSymbolLookups
// bounds even when this heuristic lets one through.
func looksLikeSymbol(tok string) bool {
	if len(tok) < 4 || strings.ContainsAny(tok, "/@#") {
		return false
	}
	// A single embedded dot reads as Type.Method ("Store.Get"); anything
	// else (a URL fragment, a sentence with a mid-token period) is not a
	// symbol shape this heuristic accepts.
	if strings.Count(tok, ".") > 1 {
		return false
	}
	hasUpper, hasLower, hasUnderscore := false, false, false
	for _, r := range tok {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r == '_':
			hasUnderscore = true
		case r == '.':
			// allowed once, already bounded above
		case isASCIIDigit(r):
			// allowed anywhere; does not itself signal "symbol"
		default:
			return false
		}
	}
	return (hasUpper && hasLower) || (hasUnderscore && hasLower)
}

func isASCIILetterOrDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || isASCIIDigit(r)
}

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }
