package readout

// claims.go — RO-37 (docs/plans/readouts.md; design docs/design/readouts.md
// section 20, seam H2): the test and gate claims in an assistant's final
// message, read from a closed table fixed in code. RO-39's stop hook checks
// each claim against the session's recorded test runs (TR-7); this file only
// finds the claims. No model, no I/O: the same message gives the same claims.
//
// Each claim maps to a command class, which is a declared test command (TR-1,
// matchDeclaredTestCommand in cmd/fathom): the invocation the test-run row
// logs as `invocation`, or "" for any declared test command. A claim that
// names a command (`go test ./...` passes) is a claim only when the caller's
// TR-1 matcher says the command is declared, so this package keeps no second
// list of test commands. TR-1 declares no gate of its own, so a gate claim's
// class is any declared test command.
//
// A negation, a hedge, a condition, a purpose or a question never matches
// ("did not run the tests", "should pass", "if the gate passes", "to make
// sure all tests pass", "do the tests pass?"). A missed claim is silent
// (RO-39 logs `not checked`); a matched non-claim would print a wrong line,
// so every doubt skips. Text inside code spans, fenced blocks and block
// quotes is never prose.

import (
	"regexp"
	"strings"
)

// The claim table's ids, in table order. RO-39 logs claims by these ids,
// never by message text.
const (
	claimAllTestsPass  = "all-tests-pass"
	claimGatePasses    = "gate-passes"
	claimCommandPasses = "command-passes"
	claimTestsPass     = "tests-pass"
	claimGreen         = "green"
)

// messageClaim is one claim found in a message.
type messageClaim struct {
	// ID is the claim table entry that matched.
	ID string
	// Invocation is the declared test command the claim names (TR-1's
	// invocation), or "" when any declared test command backs it.
	Invocation string
	// Start and End are the claim's byte offsets in the message.
	Start, End int
}

// claimCommandMatcher is TR-1's matcher bound to a repository: whether
// command runs a declared test command, and the declared invocation it ran.
type claimCommandMatcher func(command string) (invocation string, ok bool)

// claimEntry is one row of the closed claim table.
type claimEntry struct {
	id string
	re *regexp.Regexp
	// named: the claim names a command in a code span, which must be a
	// declared test command.
	named bool
	// bare: the phrase ("all green") is a claim only at the start of its
	// clause or in a clause that names a run, a suite or the gate.
	bare bool
}

// The table's phrase parts, over ASCII-lowered text. claimSep allows the
// markdown emphasis an agent's summary wraps a label in (**Tests:** pass).
const (
	claimSep    = `[\s*_]*`
	claimColon  = claimSep + `(?::` + claimSep + `)?`
	claimCopula = `(?:(?:is|are|has|have|been|now|still|all|both)\s+)*`
	claimVerb   = `(?:pass(?:es|ed|ing)?|green|succeed(?:s|ed)?)`
	// claimMods are the closed words a test claim's subject may carry
	// ("all 412 unit tests pass"), so a span never starts at a conjunction.
	claimMods = `(?:(?:[0-9]+|unit|integration|e2e|end-to-end|new|race|flaky|failing|affected|relevant|targeted|` +
		`existing|remaining|package|regression|golden|fixture|smoke|related|updated|added|focused|selected|full|` +
		`whole|entire|other|go|python|rust|js|java|ruby|frontend|backend|api|cli|ui)\s+){0,2}`
	claimGateMods = `(?:(?:full|local|pre-push|push|pre-commit|commit|release|quality|validation|whole)\s+)?`
	claimDet      = `(?:(?:the|my|our|these|those|both)\s+)?`
)

// claimTable is the closed table, in precedence order: of two claims that
// start at the same offset, the earlier entry wins.
var claimTable = []claimEntry{
	{id: claimAllTestsPass, re: regexp.MustCompile(`\b(?:(?:all|every)\s+(?:(?:of\s+)?the\s+)?` + claimMods + `tests?|` +
		claimDet + claimMods + `tests` + claimColon + `(?:(?:are|have)\s+)?all)\b` + claimColon + claimCopula + claimVerb + `\b`)},
	{id: claimGatePasses, re: regexp.MustCompile(`\b(?:the\s+)?` + claimGateMods + `gate(?:\s+run)?\b` + claimColon + claimCopula + claimVerb + `\b`)},
	{id: claimCommandPasses, named: true, re: regexp.MustCompile("`#+`" + `(?:\s+run)?` + claimColon + claimCopula + `(?:` + claimVerb + `|ok)\b`)},
	{id: claimTestsPass, re: regexp.MustCompile(`\b` + claimDet + claimMods + `(?:test\s+suite|tests?|suite|specs?)(?:\s+run)?\b` + claimColon + claimCopula + claimVerb + `\b`)},
	{id: claimGreen, bare: true, re: regexp.MustCompile(`\b(?:all|everything(?:'s|’s|\s+is)?)\s+green\b|\bgreen\s+across\s+the\s+board\b`)},
}

// claimSkipBefore are the words that, anywhere in a clause up to a claim's
// end, make it no claim: negations, modals and hedges, conditions,
// purposes, and CI (a remote run the session's rows cannot see).
var claimSkipBefore = claimWordSet(`not no never nor neither cannot without unable should would could might may must
	will shall if unless once until whether when whenever hopefully probably presumably likely maybe perhaps expect
	expected expecting assume assuming hope hoping want wants need needs ensure ensures make making verify check
	confirm ci`)

// claimSkipAfter are the words that, after a claim in its clause, qualify
// it into no claim.
var claimSkipAfter = claimWordSet(`if unless once when whenever until hopefully probably presumably likely maybe perhaps
	assuming except expected should eventually ci`)

// claimSkipAfterPairs are the two-word hedges after a claim.
var claimSkipAfterPairs = claimWordSet(`i_think i_believe i_expect i_assume i_hope i_guess`)

// claimBareFiller are the words a bare claim's clause may start with;
// claimRunWords name what a bare claim is about.
var (
	claimBareFiller = claimWordSet(`and now so then it it's its is ok okay yes done result results status here`)
	claimRunWords   = claimWordSet(`test tests suite gate run ran rerun reran checks spec specs`)
)

var (
	claimWordRe     = regexp.MustCompile(`[a-z0-9]+(?:['’][a-z]+)?`)
	claimContrastRe = regexp.MustCompile(`\b(?:but|however)\b`)
)

func claimWordSet(words string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(words) {
		set[strings.ReplaceAll(w, "_", " ")] = true
	}
	return set
}

// claimsInMessage returns the claims in msg in message order. declared is
// TR-1's matcher; nil means no declared test commands, so no named-command
// claim.
func claimsInMessage(msg string, declared claimCommandMatcher) []messageClaim {
	low := claimMask(msg)
	var out []messageClaim
	for _, cl := range claimClauses(low) {
		if cl.question {
			continue
		}
		start := cl.start
		for _, cut := range append(claimContrastRe.FindAllStringIndex(low[cl.start:cl.end], -1), []int{cl.end - cl.start, cl.end - cl.start}) {
			out = append(out, claimsInClause(msg, low, start, cl.start+cut[0], declared)...)
			start = cl.start + cut[1]
		}
	}
	return out
}

// claimsInClause matches the table in low[s:e], drops what a skip word or a
// missing declaration rules out, then keeps the earliest of overlapping
// claims (the earlier table entry at the same offset).
func claimsInClause(msg, low string, s, e int, declared claimCommandMatcher) []messageClaim {
	if s >= e {
		return nil
	}
	type cand struct {
		c   messageClaim
		idx int
	}
	var cands []cand
	for idx, entry := range claimTable {
		for _, loc := range entry.re.FindAllStringIndex(low[s:e], -1) {
			c := messageClaim{ID: entry.id, Start: s + loc[0], End: s + loc[1]}
			if claimSkipped(low[s:c.End], low[c.End:e]) {
				continue
			}
			if entry.bare && !claimBareAllowed(low[s:c.Start]) {
				continue
			}
			if entry.named {
				inv, ok := claimNamedCommand(msg[c.Start:c.End], declared)
				if !ok {
					continue
				}
				c.Invocation = inv
			}
			cands = append(cands, cand{c, idx})
		}
	}
	// Insertion sort by (start, table order): a clause holds a handful.
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && (cands[j].c.Start < cands[j-1].c.Start ||
			cands[j].c.Start == cands[j-1].c.Start && cands[j].idx < cands[j-1].idx); j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
	var out []messageClaim
	end := -1
	for _, cd := range cands {
		if cd.c.Start < end {
			continue
		}
		out = append(out, cd.c)
		end = cd.c.End
	}
	return out
}

// claimSkipped reports whether a skip word rules the claim out: upTo is the
// clause through the claim's end, after the rest of the clause.
func claimSkipped(upTo, after string) bool {
	for _, w := range claimWordRe.FindAllString(upTo, -1) {
		if claimSkipBefore[w] || claimNegatedWord(w) {
			return true
		}
	}
	words := claimWordRe.FindAllString(after, -1)
	for i, w := range words {
		if claimSkipAfter[w] || claimNegatedWord(w) {
			return true
		}
		if i+1 < len(words) && claimSkipAfterPairs[w+" "+words[i+1]] {
			return true
		}
	}
	return false
}

// claimNegatedWord: a contraction that negates (didn't, don’t), or a future
// or conditional one (I'll, I'd).
func claimNegatedWord(w string) bool {
	w = strings.ReplaceAll(w, "’", "'")
	return strings.HasSuffix(w, "n't") || strings.HasSuffix(w, "'ll") || strings.HasSuffix(w, "'d")
}

// claimBareAllowed reports whether a bare claim's clause prefix leaves it a
// claim: filler only, or a word that names a run.
func claimBareAllowed(prefix string) bool {
	filler := true
	for _, w := range claimWordRe.FindAllString(prefix, -1) {
		if claimRunWords[w] {
			return true
		}
		if !claimBareFiller[w] {
			filler = false
		}
	}
	return filler
}

// claimNamedCommand reads the code span that opens span (in the original
// message) and asks TR-1's matcher whether it is a declared test command.
func claimNamedCommand(span string, declared claimCommandMatcher) (string, bool) {
	if declared == nil || !strings.HasPrefix(span, "`") {
		return "", false
	}
	end := strings.IndexByte(span[1:], '`')
	if end < 0 {
		return "", false
	}
	command := strings.TrimSpace(span[1 : 1+end])
	if command == "" {
		return "", false
	}
	return declared(command)
}

// claimClause is one clause of the masked message: [start, end), and
// whether it ends in a question mark.
type claimClause struct {
	start, end int
	question   bool
}

// claimClauses splits low at newlines, semicolons, and sentence ends (a
// '.', '!' or '?' followed by space or the end). Masked code never splits.
func claimClauses(low string) []claimClause {
	var out []claimClause
	start := 0
	for i := 0; i < len(low); i++ {
		ch := low[i]
		end := ch == '\n' || ch == ';'
		if ch == '.' || ch == '!' || ch == '?' {
			end = i+1 == len(low) || low[i+1] == ' ' || low[i+1] == '\t' || low[i+1] == '\n' || low[i+1] == '\r'
		}
		if end {
			out = append(out, claimClause{start: start, end: i, question: ch == '?'})
			start = i + 1
		}
	}
	if start < len(low) {
		out = append(out, claimClause{start: start, end: len(low)})
	}
	return out
}

// claimMask returns msg ASCII-lowered, byte for byte the same length (so
// offsets index msg), with fenced blocks and block quotes blanked and every
// code span's content replaced by '#' between its backticks.
func claimMask(msg string) string {
	b := []byte(msg)
	for i, ch := range b {
		if ch >= 'A' && ch <= 'Z' {
			b[i] = ch + 'a' - 'A'
		}
	}
	blank := func(from, to int) {
		for k := from; k < to; k++ {
			if b[k] != '\n' {
				b[k] = ' '
			}
		}
	}
	inFence := false
	for ls := 0; ls < len(b); {
		le := ls
		for le < len(b) && b[le] != '\n' {
			le++
		}
		line := strings.TrimLeft(string(b[ls:le]), " \t")
		switch {
		case strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~"):
			inFence = !inFence
			blank(ls, le)
		case inFence || strings.HasPrefix(line, ">"):
			blank(ls, le)
		default:
			for k := ls; k < le; k++ {
				if b[k] != '`' {
					continue
				}
				close := k + 1
				for close < le && b[close] != '`' {
					close++
				}
				if close == le {
					break
				}
				for m := k + 1; m < close; m++ {
					b[m] = '#'
				}
				k = close
			}
		}
		ls = le + 1
	}
	return string(b)
}
