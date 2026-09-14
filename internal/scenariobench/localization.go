// localization.go is VB-25's "files" oracle (vision 29 Wave G,
// docs/vision/29-verbs-not-lookups.md §3 row G; docs/plans/verbs.md
// VB-25): scoring a bug-localization answer's own file set against
// Task.Gold.Files. RunOracle calls ScoreLocalization for any task
// carrying Oracle.Localization; both are exported so a caller (a test, or
// `scenariobench score`) can score an answer directly.
package scenariobench

import "regexp"

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

// ParseAnswerFiles extracts a candidate file set from answer, in
// FIRST-MENTION order, deduplicated on the exact matched string — the
// order a bug-localization answer names its files IS its own ranking
// (most-likely first), which HitAt1/HitAt5 (ScoreLocalization) read
// directly. A token survives only when it contains "/" (a path
// separator — strong evidence on its own) or its extension is in
// codeFileExtensions (a bare filename with a recognized code/config/doc
// extension); this is the SAME shape check for every caller (never a
// second, looser pass for "surely this one counts too") — determinism
// over recall.
func ParseAnswerFiles(answer string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, tok := range pathTokenRE.FindAllString(answer, -1) {
		if !looksLikeFilePath(tok) {
			continue
		}
		if seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
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
