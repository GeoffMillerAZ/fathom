package conserve

// pinloss_cite.go — PR-12 (ADR-057's K4 revisit clause, fired by O161): a
// retired pin, one whose naming sources all dropped the test in the range, is
// exempt from the pre-push block only when the range also cites a decision
// that names it. The r1 and r2 i28 cells of the PR-M0 replay rewrote I28's row
// to name their new test, which PR-9 read as a retirement; none of them added
// a decision naming the lost test, so each blocks here.
//
// A decision cite is a line ADDED in base..head that names the lost test's
// function name as a whole word and either
//
//  1. sits in a decision record (ADR.md, docs/adr/**, docs/intent/decisions/**), or
//  2. is a dated decision line: an ISO date (YYYY-MM-DD) and one of the words
//     decision, decided, retire, retired or retires on the same line, in any
//     changed file.
//
// A line under docs/plans/** never counts: the plan rows an agent writes for
// its own change are not a decision (O161). "Added" is a line-multiset
// difference: a head line counts only past the number of identical lines the
// file had at base, so a line merely moved is not added. The scan is
// storeless, deterministic (changed files in sorted order, then head line
// order; the first qualifying line wins) and bounded: it reads only the
// changed files, through PinLossInput.Read, only when a retirement needs a
// cite, skips a head blob that names none of the retired tests, is over
// pinCiteMaxBytes or holds a NUL byte, and reads each file at most once a side.

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// pinCiteMaxBytes bounds one file the cite scan reads. A larger file is
// skipped, so a cite inside it is missed and the retirement blocks: the
// conservative direction.
const pinCiteMaxBytes = 8 << 20

var (
	pinCiteDate = regexp.MustCompile(`\b(19|20)\d\d-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])\b`)
	pinCiteWord = regexp.MustCompile(`(?i)\b(decision|decided|retire|retired|retires)\b`)
)

// isDecisionRecordPath reports whether p is a decision record: ADR.md at the
// root, or a file under docs/adr/ or docs/intent/decisions/.
func isDecisionRecordPath(p string) bool {
	return p == "ADR.md" || strings.HasPrefix(p, "docs/adr/") || strings.HasPrefix(p, "docs/intent/decisions/")
}

// isPlanPath reports whether p is under docs/plans/, whose lines never cite.
func isPlanPath(p string) bool { return strings.HasPrefix(p, "docs/plans/") }

// isDatedDecisionLine reports whether line carries an ISO date and a decision
// word (decision, decided, retire, retired, retires).
func isDatedDecisionLine(line string) bool {
	return pinCiteDate.MatchString(line) && pinCiteWord.MatchString(line)
}

// decisionCites returns, for each test in tests, "path:line" of its first
// decision cite in the range, or no entry. Each changed file outside
// docs/plans/ is read at most once at head, and at base only when its head
// names one of the tests.
func decisionCites(base, head func(string) ([]byte, bool, error), changed, tests []string) map[string]string {
	out := map[string]string{}
	if len(tests) == 0 {
		return out
	}
	names := make(map[string]*regexp.Regexp, len(tests))
	for _, t := range tests {
		names[t] = regexp.MustCompile(`\b` + regexp.QuoteMeta(t) + `\b`)
	}
	files := make([]string, 0, len(changed))
	for _, p := range changed {
		if !isPlanPath(p) {
			files = append(files, p)
		}
	}
	sort.Strings(files)
	for _, p := range files {
		if len(out) == len(names) {
			break
		}
		h, ok, err := head(p)
		if err != nil || !ok || len(h) > pinCiteMaxBytes || bytes.IndexByte(h, 0) >= 0 {
			continue
		}
		var want []string // the tests still without a cite that p's head names
		for _, t := range tests {
			if _, done := out[t]; !done && bytes.Contains(h, []byte(t)) {
				want = append(want, t)
			}
		}
		if len(want) == 0 {
			continue
		}
		record := isDecisionRecordPath(p)
		for _, a := range addedLines(base, p, h) {
			if !record && !isDatedDecisionLine(a.text) {
				continue
			}
			for _, t := range want {
				if _, done := out[t]; !done && names[t].MatchString(a.text) {
					out[t] = fmt.Sprintf("%s:%d", p, a.line)
				}
			}
		}
	}
	return out
}

type addedLine struct {
	line int // 1-based, at head
	text string
}

// addedLines returns head's lines (p at head) past the count of identical
// lines p had at base: a moved line is not added; a file new in the range is
// all added.
func addedLines(base func(string) ([]byte, bool, error), p string, head []byte) []addedLine {
	baseCount := map[string]int{}
	if b, ok, err := base(p); err == nil && ok {
		for _, line := range strings.Split(string(b), "\n") {
			baseCount[line]++
		}
	}
	var out []addedLine
	for i, line := range strings.Split(string(head), "\n") {
		if baseCount[line] > 0 {
			baseCount[line]--
			continue
		}
		out = append(out, addedLine{line: i + 1, text: line})
	}
	return out
}
