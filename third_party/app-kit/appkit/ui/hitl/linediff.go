package hitl

// linediff.go — a deterministic line diff (Myers' O(ND) shortest edit script)
// and unified-hunk grouping for RenderUnifiedDiff. Pure Go, no dependencies,
// bounded input and bounded work.

import (
	"fmt"
	"strconv"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/internal/linetext"
)

// DiffLineKind classifies one line of a line diff.
type DiffLineKind string

const (
	DiffContext DiffLineKind = "context"
	DiffAdded   DiffLineKind = "added"
	DiffRemoved DiffLineKind = "removed"
)

// DiffLine is one line of a line diff. OldNumber is its 1-based line number in
// the old text (0 for an added line); NewNumber is its number in the new text
// (0 for a removed line). NoNewline marks the last line of a text that does not
// end with a newline, so "a" and "a\n" differ exactly as they do in git.
type DiffLine struct {
	Kind      DiffLineKind
	OldNumber int
	NewNumber int
	Text      string
	NoNewline bool
}

// LineDiff is a complete edit script: every line of both texts, in order,
// with removals before additions inside each changed block.
type LineDiff struct {
	Lines []DiffLine
	// Replaced reports that the edit distance exceeded the comparison budget.
	// The script is still correct (common leading and trailing lines are kept),
	// but the differing middle is one removal block followed by one addition
	// block rather than a minimal diff.
	Replaced bool
}

const (
	// MaxDiffLines bounds each side of a comparison; larger input is refused.
	MaxDiffLines = 10000
	// DefaultContext is the unified-diff context used when Context is 0.
	DefaultContext = 3
	// NoContext requests hunks with no unchanged lines around changes.
	NoContext = -1
	// editBudget bounds the Myers search (time O((N+M)·D), memory O(D²)).
	editBudget = 1000
)

type lineKey struct {
	text      string
	noNewline bool
}

type opKind uint8

const (
	opEqual opKind = iota
	opDelete
	opInsert
)

func keys(text string) []lineKey {
	lines, final := linetext.Split(text)
	out := make([]lineKey, len(lines))
	for i, line := range lines {
		out[i] = lineKey{text: line, noNewline: i == len(lines)-1 && !final}
	}
	return out
}

// DiffText compares two texts line by line. The result is deterministic for
// the same input. Texts longer than MaxDiffLines lines are refused.
func DiffText(oldText, newText string) (LineDiff, error) {
	a, b := keys(oldText), keys(newText)
	if len(a) > MaxDiffLines || len(b) > MaxDiffLines {
		return LineDiff{}, fmt.Errorf("hitl: diff input has %d and %d lines; at most %d per side", len(a), len(b), MaxDiffLines)
	}
	ops, replaced := editScript(a, b, editBudget)
	lines := make([]DiffLine, 0, len(ops))
	x, y := 0, 0
	for _, op := range ops {
		switch op {
		case opEqual:
			lines = append(lines, DiffLine{Kind: DiffContext, OldNumber: x + 1, NewNumber: y + 1, Text: a[x].text, NoNewline: a[x].noNewline})
			x++
			y++
		case opDelete:
			lines = append(lines, DiffLine{Kind: DiffRemoved, OldNumber: x + 1, Text: a[x].text, NoNewline: a[x].noNewline})
			x++
		case opInsert:
			lines = append(lines, DiffLine{Kind: DiffAdded, NewNumber: y + 1, Text: b[y].text, NoNewline: b[y].noNewline})
			y++
		}
	}
	return LineDiff{Lines: lines, Replaced: replaced}, nil
}

// editScript trims the common prefix and suffix, runs Myers on the middle and
// orders each changed block as removals then additions.
func editScript(a, b []lineKey, budget int) ([]opKind, bool) {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	middle, ok := myers(a[prefix:len(a)-suffix], b[prefix:len(b)-suffix], budget)
	replaced := !ok
	if !ok {
		middle = middle[:0]
		for range len(a) - prefix - suffix {
			middle = append(middle, opDelete)
		}
		for range len(b) - prefix - suffix {
			middle = append(middle, opInsert)
		}
	}
	ops := make([]opKind, 0, prefix+len(middle)+suffix)
	for range prefix {
		ops = append(ops, opEqual)
	}
	ops = append(ops, middle...)
	for range suffix {
		ops = append(ops, opEqual)
	}
	// Removals before additions inside every run of changes: the script stays
	// valid because deletes and inserts within one run are independent.
	for i := 0; i < len(ops); {
		if ops[i] == opEqual {
			i++
			continue
		}
		j, deletes := i, 0
		for j < len(ops) && ops[j] != opEqual {
			if ops[j] == opDelete {
				deletes++
			}
			j++
		}
		for k := i; k < j; k++ {
			if k-i < deletes {
				ops[k] = opDelete
			} else {
				ops[k] = opInsert
			}
		}
		i = j
	}
	return ops, replaced
}

// myers returns the shortest edit script from a to b, or false when it needs
// more than budget edits. Each step keeps only the diagonal window it can read
// back, so memory is O(D²) rather than O(D·(N+M)).
func myers(a, b []lineKey, budget int) ([]opKind, bool) {
	n, m := len(a), len(b)
	if n == 0 || m == 0 {
		ops := make([]opKind, 0, n+m)
		for range n {
			ops = append(ops, opDelete)
		}
		for range m {
			ops = append(ops, opInsert)
		}
		return ops, true
	}
	limit := min(n+m, budget)
	offset := limit + 1
	v := make([]int, 2*limit+3)
	var trace [][]int
	for d := 0; d <= limit; d++ {
		// Snapshot the state before step d: diagonals -(d+1)..d+1.
		trace = append(trace, append([]int(nil), v[offset-d-1:offset+d+2]...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1] // down: insert b[y]
			} else {
				x = v[offset+k-1] + 1 // right: delete a[x]
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				return backtrack(trace, n, m), true
			}
		}
	}
	return nil, false
}

func backtrack(trace [][]int, n, m int) []opKind {
	var ops []opKind
	x, y := n, m
	for d := len(trace) - 1; d > 0; d-- {
		snapshot := trace[d]
		at := func(k int) int { return snapshot[k+d+1] }
		k := x - y
		prevK := k - 1
		if k == -d || (k != d && at(k-1) < at(k+1)) {
			prevK = k + 1
		}
		prevX := at(prevK)
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			ops = append(ops, opEqual)
			x--
			y--
		}
		if x == prevX {
			ops = append(ops, opInsert)
		} else {
			ops = append(ops, opDelete)
		}
		x, y = prevX, prevY
	}
	for x > 0 && y > 0 {
		ops = append(ops, opEqual)
		x--
		y--
	}
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}
	return ops
}

// Hunk is one unified-diff hunk: a contiguous run of lines with its old and
// new ranges. When a count is 0 the start is the line before the hunk, as in
// git ("@@ -0,0 +1,2 @@" adds two lines to an empty text).
type Hunk struct {
	OldStart, OldCount int
	NewStart, NewCount int
	Lines              []DiffLine
}

// Header returns the unified hunk header, e.g. "@@ -12,7 +12,8 @@". A count of
// one is omitted, as git does.
func (h Hunk) Header() string {
	return "@@ -" + hunkRange(h.OldStart, h.OldCount) + " +" + hunkRange(h.NewStart, h.NewCount) + " @@"
}

func hunkRange(start, count int) string {
	if count == 1 {
		return strconv.Itoa(start)
	}
	return strconv.Itoa(start) + "," + strconv.Itoa(count)
}

func effectiveContext(context int) int {
	switch {
	case context == 0:
		return DefaultContext
	case context < 0:
		return 0
	}
	return context
}

// Hunks groups the script into unified hunks. context 0 means DefaultContext;
// NoContext (any negative value) shows changed lines only. Identical texts
// have no hunks.
func (d LineDiff) Hunks(context int) []Hunk {
	var hunks []Hunk
	for _, segment := range d.segments(effectiveContext(context)) {
		if segment.hunk != nil {
			hunks = append(hunks, *segment.hunk)
		}
	}
	return hunks
}

// segment is either a visible hunk or a run of hidden unchanged lines.
type segment struct {
	hunk   *Hunk
	hidden []DiffLine
}

// segments partitions every line: an unchanged line is visible only within
// context lines of a change. Identical texts yield no segments.
func (d LineDiff) segments(context int) []segment {
	lines := d.Lines
	visible := make([]bool, len(lines))
	changed := false
	lastChange := -1 << 30
	for i, line := range lines {
		if line.Kind != DiffContext {
			changed = true
			lastChange = i
		}
		visible[i] = i-lastChange <= context
	}
	if !changed {
		return nil
	}
	nextChange := 1 << 30
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i].Kind != DiffContext {
			nextChange = i
		}
		visible[i] = visible[i] || nextChange-i <= context
	}
	var out []segment
	oldSeen, newSeen := 0, 0
	for i := 0; i < len(lines); {
		j := i
		for j < len(lines) && visible[j] == visible[i] {
			j++
		}
		run := lines[i:j]
		if visible[i] {
			h := Hunk{Lines: run}
			for _, line := range run {
				if line.Kind != DiffAdded {
					h.OldCount++
				}
				if line.Kind != DiffRemoved {
					h.NewCount++
				}
			}
			h.OldStart, h.NewStart = oldSeen, newSeen
			if h.OldCount > 0 {
				h.OldStart++
			}
			if h.NewCount > 0 {
				h.NewStart++
			}
			out = append(out, segment{hunk: &h})
			oldSeen += h.OldCount
			newSeen += h.NewCount
		} else {
			out = append(out, segment{hidden: run})
			oldSeen += len(run)
			newSeen += len(run)
		}
		i = j
	}
	return out
}
