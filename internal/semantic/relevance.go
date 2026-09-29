package semantic

// relevance.go — wave O2 (docs/bench/readings/vs-pilot-6-second-reading.md
// §5, second reading of vs-pilot-6). VB-8's governing-scope ranker
// (governingscope.go) gets a claim/section to the right SCOPE TIER — but
// every rule inside a tier still fell back to document order (rawfallback's
// own StartLine tie-break) or a raw byte-length/alphabetical tie-break
// (obligationsCore's specificity/rule-text tie-break) once it got there.
// vs-pilot-6 found the exact shape this produces: internal/AGENTS.md's
// "Store purity" section (an unrelated decoy) served ahead of its own
// "Adding things" section (the rule that actually names how to wire up a
// new extractor) for internal/extractor/goextract.go, on every trial the
// raw fallback fired — because both sections share the same tier and
// depth (one nested guide, one directory), and document order put Store
// purity first.
//
// relevanceScore is the missing tie-break: a small, deterministic, no-LLM
// lexical score naming how strongly a rule/section's own text reads back
// the edited path's own name (and, optionally, symbol/import names the
// caller already resolved from the index). It is used STRICTLY INSIDE a
// scope tier (VB-8's tier+depth proximity still decides the tier itself —
// this never promotes a farther-tier rule over a nearer one; see
// serve.go's less() and rawfallback.go's candidate sort for exactly where
// it sits in each ordering).
//
// Deliberately NOT a length-normalised density (hits / len(text)): an
// early version of this file scored internal/AGENTS.md's "Adding things"
// section (19 lines, four different addition recipes, only one of which —
// "Extractor" — is about goextract.go) BELOW the much shorter "Store
// purity" section, purely because the other three recipes diluted the
// ratio — exactly backwards for the fixture this exists to fix. Instead: a
// hit COUNT capped at hitCap (repeating one word nine times is not nine
// times as relevant) times a per-token SPECIFICITY weight (the file's own
// stem/symbol names outweigh its immediate package directory, which
// outweighs every shallower ancestor — "internal" alone barely
// discriminates within a guide that governs all of internal/*), with a
// heading hit weighted again on top. No division, so a thorough section is
// never punished for being thorough.
//
// wave O3 adds two more pieces, both still strictly deterministic/no-LLM:
// relevanceStrength (this file) reports, alongside the score, whether any
// hit was STRONG (packageWeight or above) — the signal crossTierCloser
// (governingscope.go) needs to ever promote a farther-tier rule ahead of a
// nearer one, and only when the nearer one is a total lexical blank (see
// that function's doc comment for the exact, deliberately narrow rule).
// BehaviorTokens (this file) is the other half: a root-wide convention
// (workspace/, the gate) names no file/package/symbol of its own to be
// "strong" about, so a file that BEHAVES the way the convention governs
// (writes output, runs the gate) is scanned for a small fixed table of
// signals and gets the convention's own keyword folded in as an extra,
// stem-weight token — the same extraTokens path a caller-resolved symbol
// name already uses (pkg/client.RawFallbackObligations wires both).
import "strings"

// weightedToken is one lexical token relevanceScore matches against,
// carrying how specific a signal it is (stemWeight down to
// extensionWeight) — see this file's doc comment.
type weightedToken struct {
	tok    string // already lower-cased
	weight float64
}

const (
	// stemWeight is the file's own name (and any caller-supplied
	// symbol/import token) — the most specific signal a rule can echo back.
	stemWeight = 3.0
	// packageWeight is the file's own immediate directory (its Go package,
	// by convention) — specific, but shared by every sibling file.
	packageWeight = 2.0
	// ancestorWeight is every directory ABOVE the file's own package —
	// "internal" alone governs so much of a typical repo that it barely
	// discriminates among a nested guide's own sections.
	ancestorWeight = 1.0
	// extensionWeight is the bare file extension (".go" -> "go") — the
	// weakest signal kept at all, present on every sibling file in the repo.
	extensionWeight = 0.5
	// headingRelevanceWeight multiplies a hit found in text's own first
	// line (an ATX heading for a raw section, or the heading words folded
	// into a compiled claim's synthetic scoring text) — a rule's own title
	// naming the file is a stronger signal than an incidental body mention.
	headingRelevanceWeight = 2.0
	// hitCap bounds how many times one token's repetition can count —
	// past this, more repeats buy no more score.
	hitCap = 3
)

// pathRelevanceTokens derives the weighted lexical tokens relevanceScore
// scores a candidate rule/section against, from the edited path ALONE (no
// index lookup — see symbolRelevanceTokens for the optional index-backed
// half of item 1's design): every directory segment (ancestorWeight, or
// packageWeight for the file's own immediate directory), the file's own
// stem (stemWeight, further split on case/underscore boundaries when it
// has any), and its bare extension (extensionWeight). Deterministic and
// dedupes by token text (first weight wins — the loop visits stem/package
// tokens, the more specific ones, before ancestor tokens).
func pathRelevanceTokens(file string) []weightedToken {
	var out []weightedToken
	seen := map[string]bool{}
	add := func(tok string, weight float64) {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if len(tok) < 2 || seen[tok] {
			return
		}
		seen[tok] = true
		out = append(out, weightedToken{tok: tok, weight: weight})
	}

	dir := dirOf(file)
	base := file[len(dir):]
	base = strings.TrimPrefix(base, "/")
	ext := ""
	stem := base
	if i := strings.LastIndex(base, "."); i > 0 {
		stem, ext = base[:i], base[i+1:]
	}
	add(stem, stemWeight)
	for _, part := range splitIdent(stem) {
		add(part, stemWeight)
	}
	if ext != "" {
		add(ext, extensionWeight)
	}

	if dir != "" {
		segs := strings.Split(dir, "/")
		for i, seg := range segs {
			w := ancestorWeight
			if i == len(segs)-1 {
				w = packageWeight // the file's own immediate directory
			}
			add(seg, w)
			for _, part := range splitIdent(seg) {
				add(part, w)
			}
		}
	}
	return out
}

// symbolRelevanceTokens wraps caller-supplied symbol/import names — the
// optional index-backed half of item 1's design: a governing rule that
// names the file's own declared type or an import it depends on is as
// strong a signal as one that names its stem. The ranking functions in
// this package never touch a store directly (RawFallbackRules stays a pure
// function of its inputs); a caller that has resolved these from the index
// (pkg/client.RawFallbackObligations) passes them in as plain strings.
func symbolRelevanceTokens(names []string) []weightedToken {
	var out []weightedToken
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if len(n) < 2 || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, weightedToken{tok: n, weight: stemWeight})
	}
	return out
}

// splitIdent breaks an identifier-shaped token on underscores/hyphens/dots
// and camelCase/PascalCase boundaries into lower-cased words: "goExtract"
// and "go_extract" both -> ["go", "extract"]; "goextract" (no boundary at
// all) returns nil — the caller already added the whole token via add()
// before calling this, so a word-less identifier is never lost, only never
// artificially split.
func splitIdent(s string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case r == '_' || r == '-' || r == '.':
			flush()
		case r >= 'A' && r <= 'Z' && i > 0 && !(runes[i-1] >= 'A' && runes[i-1] <= 'Z'):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	if len(words) <= 1 {
		return nil
	}
	return words
}

// relevanceScore is the deterministic, no-LLM lexical tie-break: how
// strongly text — split into its own first LINE (heading) and the rest
// (body) — reads back tokens. See this file's doc comment for the
// capped-hit-count, weighted-by-specificity, no-length-division design and
// why. Zero when tokens or text is empty — an honest "nothing to score",
// never a fabricated tie-break.
func relevanceScore(text string, tokens []weightedToken) float64 {
	score, _ := relevanceStrength(text, tokens)
	return score
}

// relevanceStrength is relevanceScore's full answer (wave O3, item 1): the
// same score, plus whether ANY hit came from a token at packageWeight or
// above (stemWeight, packageWeight, or a caller-supplied symbol/behaviour
// token — symbolRelevanceTokens and BehaviorTokens both carry stemWeight).
// A hit confined to ancestorWeight/extensionWeight tokens ("internal",
// ".go") is never strong: both are shared by every sibling file in the
// repo and fire on unrelated prose constantly (a guide that merely
// mentions "internal/store" in passing, or uses the word "go" as an
// English verb). "Strong" is what crossTierCloser (governingscope.go)
// requires before it will promote a farther-tier candidate ahead of a
// nearer one — see that function's doc comment for the full rule.
func relevanceStrength(text string, tokens []weightedToken) (score float64, strong bool) {
	if len(tokens) == 0 || text == "" {
		return 0, false
	}
	heading, body, _ := strings.Cut(text, "\n")
	heading = strings.ToLower(heading)
	body = strings.ToLower(body)
	for _, t := range tokens {
		hHits := countWordHits(heading, t.tok)
		bHits := countWordHits(body, t.tok)
		if hHits > hitCap {
			hHits = hitCap
		}
		if bHits > hitCap {
			bHits = hitCap
		}
		if hHits+bHits == 0 {
			continue
		}
		score += t.weight * (headingRelevanceWeight*float64(hHits) + float64(bHits))
		if t.weight >= packageWeight {
			strong = true
		}
	}
	return score, strong
}

// behaviorConventionTable (wave O3, item 2) is the small, deterministic
// behaviour -> convention-keyword table for root-wide conventions that
// name no file, package, or symbol of their own — H1-T08's workspace/ row
// is the case this exists for: "ALL build artifacts, logs, temp files,
// local DBs go under gitignored workspace/" reads back nothing about any
// one edited file, so pathRelevanceTokens/symbolRelevanceTokens can never
// give it a lexical tie no matter which file is being edited. What DOES
// tie a file to that rule is the file's own BEHAVIOUR: a file that writes
// output the convention governs. Each row is a set of literal signals
// (matched as a plain substring of the file's own source text — no
// parsing, no AST) mapped to the convention keyword(s) to add as extra
// relevance tokens, at the same stem weight a caller-resolved symbol name
// carries (BehaviorTokens below feeds them through the exact same
// extraTokens path RawFallbackObligations already uses for symbol names).
var behaviorConventionTable = []struct {
	signals  []string
	keywords []string
}{
	{
		// Writing a file outside the working tree, or a flag that names an
		// output path — the shape the root workspace/ convention governs.
		signals: []string{
			"os.WriteFile(", "os.Create(", "os.OpenFile(", "os.MkdirAll(",
			"ioutil.WriteFile(", "ioutil.TempFile(", "os.TempDir(",
		},
		keywords: []string{"workspace"},
	},
	{
		// Running the test/vet/build gate, or a raw shell command — the
		// shape the root Gate convention governs.
		signals: []string{
			"exec.Command(\"go\"", "exec.Command(\"sh\"", "exec.Command(\"bash\"",
			"\"go\", \"test\"", "\"go\", \"vet\"", "\"go\", \"build\"",
		},
		keywords: []string{"gate", "validate"},
	},
}

// BehaviorTokens scans fileText — the edited file's own source, when the
// caller has it (pkg/client.RawFallbackObligations already loads every
// source file's content to build the layer's corpus; the edited file is
// almost always among them) — for the signals behaviorConventionTable
// declares, and returns the matched rows' convention keywords, deduplicated
// in table order. A pure function of the text handed to it — no store
// access, no AST parse, just the same substring/word-boundary machinery
// this file already uses. Returns nil when fileText is empty (nothing to
// scan — an honest "no signal available", the exact shape a synthetic or
// unreadable path degrades to) or when nothing in the table matches.
// Deterministic: same text, same tokens, every call.
func BehaviorTokens(fileText string) []string {
	if fileText == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, row := range behaviorConventionTable {
		matched := false
		for _, sig := range row.signals {
			if strings.Contains(fileText, sig) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		for _, kw := range row.keywords {
			if seen[kw] {
				continue
			}
			seen[kw] = true
			out = append(out, kw)
		}
	}
	return out
}

// countWordHits counts non-overlapping, word-bounded occurrences of tok
// (already lower-cased) in text (already lower-cased) — "go" matches the
// standalone word "go" but never the "go" inside "cgo" or "golden", so a
// short, common token cannot fire on an unrelated word that merely
// contains it as a substring.
func countWordHits(text, tok string) int {
	if tok == "" {
		return 0
	}
	count := 0
	for idx := 0; idx < len(text); {
		i := strings.Index(text[idx:], tok)
		if i < 0 {
			break
		}
		pos := idx + i
		beforeOK := pos == 0 || !isWordByte(text[pos-1])
		end := pos + len(tok)
		afterOK := end >= len(text) || !isWordByte(text[end])
		if beforeOK && afterOK {
			count++
		}
		idx = pos + len(tok)
	}
	return count
}

// isWordByte reports whether b can be part of an identifier-shaped word —
// the boundary test countWordHits uses on both sides of a candidate match.
func isWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
