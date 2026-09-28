package specdialect

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// The converge block's markers (design §5, law 15): fathom writes a
// foreign spec only between these two lines, and only when asked.
const (
	ConvergeBegin = "<!-- fathom:begin converge -->"
	ConvergeEnd   = "<!-- fathom:end converge -->"
)

// FixupLines renders the converge report's claimed-unproven and unclaimed
// items as open tasks in the dialect's own checkbox shape, numbered after
// the highest task number outside the converge block (so a re-run numbers
// the same way):
//
//   - speckit: "- [ ] T006 …"
//   - openspec: "- [ ] 4.1 …", a new top-level number
//   - native and anything else: "- [ ] …"
//
// A claimed-unproven task's fix-up carries the task's own contract, so it
// verifies exactly when the task does; an unclaimed symbol's fix-up carries
// {symbol, package} naming it, so claiming it is a provable act.
func FixupLines(dialect string, content []byte, plan verify.Plan, r ConvergeReport) []string {
	byID := map[string]verify.Task{}
	for _, t := range plan.Tasks {
		byID[t.ID] = t
	}
	var texts []string
	for _, it := range r.ClaimedUnproven {
		block := ""
		if t, ok := byID[it.TaskID]; ok {
			t.ID = ""
			if pairs := synthPairs(t); len(pairs) > 0 {
				block = " {" + strings.Join(pairs, ", ") + "}"
			}
		}
		texts = append(texts, fmt.Sprintf("Make %s pass — %s%s", it.TaskID, braceless(it.Criterion), block))
	}
	for _, s := range r.Unclaimed {
		texts = append(texts, fmt.Sprintf("Claim or remove %s (%s:%d), changed on this branch with no task anchoring it {symbol: %s, package: %s}",
			s.Name, s.File, s.Line, synthValue(s.Name), synthValue(path.Dir(s.File))))
	}

	outside := string(content)
	if b, e, ok := blockBounds(content); ok {
		outside = string(content[:b]) + string(content[e:])
	}
	lines := make([]string, 0, len(texts))
	switch dialect {
	case "speckit":
		n := maxNumber(outside, regexp.MustCompile(`(?m)^\s*[-*]\s+\[[ xX]\]\s+T(\d+)\b`))
		for i, tx := range texts {
			lines = append(lines, fmt.Sprintf("- [ ] T%03d %s", n+i+1, tx))
		}
	case "openspec":
		n := maxNumber(outside, regexp.MustCompile(`(?m)^\s*[-*]\s+\[[ xX]\]\s+(\d+)(?:\.\d+)*\.?\s`))
		for i, tx := range texts {
			lines = append(lines, fmt.Sprintf("- [ ] %d.%d %s", n+1, i+1, tx))
		}
	default:
		for _, tx := range texts {
			lines = append(lines, "- [ ] "+tx)
		}
	}
	return lines
}

// braceless keeps a criterion's detail from opening an annotation block.
func braceless(s string) string {
	return strings.NewReplacer("{", "(", "}", ")").Replace(s)
}

// maxNumber is the highest first capture group re finds in s (0 if none).
func maxNumber(s string, re *regexp.Regexp) int {
	max := 0
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > max {
			max = n
		}
	}
	return max
}

// blockBounds locates the converge block: b is the offset of the begin
// marker's line, e the offset just past the end marker's line (its
// newline included). ok is false when there is no complete, ordered pair.
func blockBounds(content []byte) (b, e int, ok bool) {
	s := string(content)
	bi := markerLine(s, ConvergeBegin, 0)
	if bi < 0 {
		return 0, 0, false
	}
	ei := markerLine(s, ConvergeEnd, bi)
	if ei < 0 {
		return 0, 0, false
	}
	end := ei + len(ConvergeEnd)
	for end < len(s) && s[end] != '\n' {
		end++
	}
	if end < len(s) {
		end++
	}
	return bi, end, true
}

// markerLine is the offset (at or after from) of the first line whose
// trimmed text is exactly marker, or -1.
func markerLine(s, marker string, from int) int {
	off := from
	for _, line := range strings.SplitAfter(s[from:], "\n") {
		if strings.TrimSpace(line) == marker {
			return off + strings.Index(line, marker)
		}
		off += len(line)
	}
	return -1
}

// writeConvergeBlock returns content with the converge block's contents
// replaced by lines (law 15): every byte outside the markers is kept
// exactly. With no block, one is appended at the end (after a newline when
// the file lacks a final one). A lone or misordered marker is refused —
// fathom never guesses where its region ends.
func writeConvergeBlock(content []byte, lines []string) ([]byte, error) {
	s := string(content)
	hasBegin, hasEnd := markerLine(s, ConvergeBegin, 0) >= 0, markerLine(s, ConvergeEnd, 0) >= 0
	body := ""
	if len(lines) > 0 {
		body = strings.Join(lines, "\n") + "\n"
	}
	if bi := markerLine(s, ConvergeBegin, 0); bi >= 0 {
		if ei := markerLine(s, ConvergeEnd, bi); ei >= 0 {
			// Replace only what lies strictly between the two marker
			// lines; the markers' own lines stay as written.
			afterBegin := bi + strings.IndexByte(s[bi:], '\n') + 1
			endLine := strings.LastIndexByte(s[:ei], '\n') + 1
			return []byte(s[:afterBegin] + body + s[endLine:]), nil
		}
	}
	if hasBegin || hasEnd {
		return content, fmt.Errorf("the converge block's markers are incomplete or out of order (want %s … %s); fix them by hand — fathom writes only inside a complete block", ConvergeBegin, ConvergeEnd)
	}
	prefix := s
	if prefix != "" && !strings.HasSuffix(prefix, "\n") {
		prefix += "\n"
	}
	return []byte(prefix + "\n" + ConvergeBegin + "\n" + body + ConvergeEnd + "\n"), nil
}

// WriteFixups is `--write-fixups`: the spec's content with its converge
// block holding one open task per claimed-unproven and unclaimed item, in
// the dialect's own checkbox shape. Nothing outside the markers changes.
func WriteFixups(dialect string, content []byte, plan verify.Plan, r ConvergeReport) ([]byte, error) {
	return writeConvergeBlock(content, FixupLines(dialect, content, plan, r))
}
