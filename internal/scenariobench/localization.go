// localization.go is VB-25's "files" oracle (vision 29 Wave G,
// docs/vision/29-verbs-not-lookups.md §3 row G; docs/plans/verbs.md
// VB-25): scoring a bug-localization answer's own file set against
// Task.Gold.Files. RunOracle calls ScoreLocalization for any task
// carrying Oracle.Localization; both are exported so a caller (a test, or
// `scenariobench score`) can score an answer directly.
package scenariobench

import (
	"regexp"
	"strings"
)

// codeFileExtensions is the fixed allow-list a BARE filename (no "/") must
// match to be accepted by ParseAnswerFiles — chosen to admit ordinary
// source, config and doc extensions across the many languages Long Code
// Arena spans, while excluding prose that would otherwise parse as a
// nonexistent one-letter extension ("e.g." -> "e", extension "g"; "i.e."
// -> extension "e"). A token that also contains a "/" is accepted
// regardless of extension — a path with a directory component is strong
// enough evidence on its own.
var codeFileExtensions = map[string]bool{
	"go": true, "py": true, "js": true, "jsx": true, "ts": true, "tsx": true,
	"java": true, "kt": true, "kts": true, "rb": true, "rs": true, "c": true,
	"h": true, "hpp": true, "cc": true, "cpp": true, "cxx": true, "cs": true,
	"php": true, "swift": true, "scala": true, "sh": true, "bash": true,
	"md": true, "json": true, "yaml": true, "yml": true, "toml": true,
	"sql": true, "proto": true, "rst": true, "txt": true, "cue": true,
	"mod": true, "sum": true, "lock": true,
}

// pathTokenRE matches a candidate path-shaped token: a leading word
// character, then path characters (word characters, "/" and "-"), a
// literal ".", then an extension (letters/digits, one or more segments).
// It is deliberately loose — codeFileExtensions and the "contains a
// slash" rule in ParseAnswerFiles narrow it down to real paths — since no
// regex parses free-form natural language perfectly; this is a
// deterministic heuristic, not a guarantee.
var pathTokenRE = regexp.MustCompile(`[A-Za-z0-9_][A-Za-z0-9_/-]*\.[A-Za-z0-9]+(?:\.[A-Za-z0-9]+)*`)

// negationMarkers, each checked case-insensitively against a path
// mention's own CLAUSE (back to the nearest sentence boundary or comma,
// clauseIsNegated): present there, the mention is demoted below every
// non-negated one — the answer named this file only to rule it out
// ("not in", "isn't", "rather than", …), kept for recall (it still
// counts toward Recall/Precision), never for HitAt1.
var negationMarkers = []string{
	"not in", "isn't", "is not", "rather than", "instead of",
	"unrelated", "no longer", "exclude",
}

// listLineRE matches a numbered ("1.", "2)") or bulleted ("-", "*", "•")
// marker opening a line (after leading whitespace), optionally followed
// by a backtick, immediately before the path itself — the shape an
// explicit ranked answer uses ("1. ledger.go", "- `errors.go`").
var listLineRE = regexp.MustCompile("^[ \t]*(?:[0-9]+[.)]|[-*•])[ \t]+`?")

// ParseAnswerFiles extracts a candidate file set from answer, RANKED —
// HitAt1/HitAt5 (ScoreLocalization) read the order directly:
//
//  1. An explicit ranked list — one or more lines each opening with a
//     numbered or bulleted marker immediately followed by a path
//     (listOrderFiles) — leads: an agent that took the trouble to rank
//     its own answer is not outranked by an incidental earlier prose
//     mention (e.g. "payment.go was also touched recently" ahead of a
//     numbered list further down). Every other path mentioned in prose
//     (never on a list line) is appended after, in first-mention order,
//     for recall.
//  2. With no list, plain first-mention order.
//  3. Within that order, a path negated in its own clause
//     (clauseIsNegated) is demoted below every non-negated path — a
//     STABLE partition (demoteNegated): the relative order within each
//     group is unchanged.
//
// A token survives at all only when it contains "/" (a path separator —
// strong evidence on its own) or its extension is in codeFileExtensions
// (a bare filename with a recognized code/config/doc extension); this is
// the SAME shape check for every caller (never a second, looser pass for
// "surely this one counts too") — determinism over recall. Deduplicated
// throughout on the exact matched string.
func ParseAnswerFiles(answer string) []string {
	firstPos := map[string]int{}
	var prose []string
	for _, loc := range pathTokenRE.FindAllStringIndex(answer, -1) {
		tok := answer[loc[0]:loc[1]]
		if !looksLikeFilePath(tok) {
			continue
		}
		if _, ok := firstPos[tok]; !ok {
			firstPos[tok] = loc[0]
			prose = append(prose, tok)
		}
	}

	list := listOrderFiles(answer)

	seen := make(map[string]bool, len(list)+len(prose))
	ordered := make([]string, 0, len(list)+len(prose))
	for _, f := range list {
		if seen[f] {
			continue
		}
		seen[f] = true
		ordered = append(ordered, f)
	}
	for _, f := range prose {
		if seen[f] {
			continue
		}
		seen[f] = true
		ordered = append(ordered, f)
	}

	return demoteNegated(answer, ordered, firstPos)
}

// listOrderFiles returns, in line order, the first path-shaped token
// opening each list-marker line (listLineRE) — the answer's own explicit
// ranking, when it gave one; nil when the answer contains no such line.
// Deduplicated, first occurrence kept.
func listOrderFiles(answer string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(answer, "\n") {
		markerEnd := listLineRE.FindStringIndex(line)
		if markerEnd == nil {
			continue
		}
		rest := line[markerEnd[1]:]
		m := pathTokenRE.FindStringIndex(rest)
		if m == nil || m[0] != 0 {
			continue // the marker must be followed directly by the path
		}
		tok := rest[m[0]:m[1]]
		if !looksLikeFilePath(tok) || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// demoteNegated stable-partitions ordered into every non-negated path (in
// its own relative order) followed by every negated one (in its own
// relative order): clauseIsNegated checks each path's own first-
// occurrence position (firstPos) for a negationMarkers phrase before it.
func demoteNegated(answer string, ordered []string, firstPos map[string]int) []string {
	out := make([]string, 0, len(ordered))
	var negated []string
	for _, f := range ordered {
		if clauseIsNegated(answer, firstPos[f]) {
			negated = append(negated, f)
			continue
		}
		out = append(out, f)
	}
	return append(out, negated...)
}

// clauseIsNegated reports whether the clause ending at pos (a mention's
// own first-occurrence start index) — from the nearest sentence boundary
// (. ! ?) or comma strictly before pos, or the start of text when there
// is none — contains a negationMarkers phrase, checked case-
// insensitively.
func clauseIsNegated(text string, pos int) bool {
	if pos < 0 || pos > len(text) {
		return false
	}
	start := strings.LastIndexAny(text[:pos], ".!?,")
	if start < 0 {
		start = 0
	} else {
		start++
	}
	clause := strings.ToLower(text[start:pos])
	for _, m := range negationMarkers {
		if strings.Contains(clause, m) {
			return true
		}
	}
	return false
}

// looksLikeFilePath applies the slash-or-known-extension rule ParseAnswerFiles
// documents.
func looksLikeFilePath(tok string) bool {
	for i := 0; i < len(tok); i++ {
		if tok[i] == '/' {
			return true
		}
	}
	ext := ""
	for i := len(tok) - 1; i >= 0; i-- {
		if tok[i] == '.' {
			ext = tok[i+1:]
			break
		}
	}
	return codeFileExtensions[lower(ext)]
}

// lower is a tiny ASCII lowercaser — avoids pulling in strings.ToLower's
// full Unicode machinery for what is always an ASCII file extension.
func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// ScoreLocalization scores answer's own parsed file set (ParseAnswerFiles)
// against gold: HitAt1 (the top-ranked answered file matches a gold file,
// path-suffix comparison via sameFile), HitAt5 (any of the top 5), and
// set-based Recall/Precision over the FULL answered set (never truncated
// to 5) — a gold file matched by more than one answered token still
// counts once, and precision's denominator is the answered set's own
// (already deduplicated) size. NotMeasurable is set only when gold itself
// is empty (a malformed task, since BuildLCACorpus refuses one at load
// time) — an EMPTY answer is a legitimate, scoreable zero, not
// unmeasurable.
func ScoreLocalization(answer string, gold []string) LocalizationReading {
	if len(gold) == 0 {
		return LocalizationReading{NotMeasurable: "task carries no gold files to score against"}
	}
	goldSet := dedupSorted(gold)
	answered := ParseAnswerFiles(answer)

	matchesGold := func(f string) bool {
		for _, g := range goldSet {
			if sameFile(f, g) {
				return true
			}
		}
		return false
	}

	hitAt1 := len(answered) > 0 && matchesGold(answered[0])
	top5 := answered
	if len(top5) > 5 {
		top5 = top5[:5]
	}
	hitAt5 := false
	for _, f := range top5 {
		if matchesGold(f) {
			hitAt5 = true
			break
		}
	}

	matchedGold := make(map[string]bool, len(goldSet))
	for _, f := range answered {
		for _, g := range goldSet {
			if sameFile(f, g) {
				matchedGold[g] = true
			}
		}
	}
	recall := float64(len(matchedGold)) / float64(len(goldSet))
	precision := 0.0
	if len(answered) > 0 {
		precision = float64(len(matchedGold)) / float64(len(answered))
	}

	return LocalizationReading{
		HitAt1:        hitAt1,
		HitAt5:        hitAt5,
		Recall:        recall,
		Precision:     precision,
		GoldFiles:     len(goldSet),
		AnsweredFiles: len(answered),
	}
}
