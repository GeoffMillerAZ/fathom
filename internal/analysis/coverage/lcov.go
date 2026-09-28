package coverage

// lcov.go — ParseLcov (TQ-10): the lcov `.info` tracefile format (genhtml/
// lcov's own line-oriented grammar), producing ForeignBlock (foreign.go)
// at DA-line granularity.

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseLcov parses an lcov tracefile (`.info`): a sequence of per-source
// records, each an optional `TN:<test name>` line, an `SF:<path>` line
// naming the source file, then zero or more `DA:<line>,<hits>[,<checksum>]`
// lines (one per instrumented line), ended by `end_of_record`. Every other
// line kind lcov defines (FN/FNDA/FNF/FNH function data, BRDA/BRF/BRH
// branch data, comments) is function- or branch-level, not the line-level
// statement coverage ForeignBlock carries, so ParseLcov skips them rather
// than erroring — lcov producers vary widely in which optional sections
// they emit, and none of them changes what a DA line means.
//
// Each DA line becomes one ForeignBlock with StartLine == EndLine == its
// line number, Statements == 1, Covered == 1 if hits > 0. When the
// enclosing record named a non-empty TN, every block from that record
// carries it as its sole entry in Tests — lcov's one facility for
// per-test attribution (a tracefile merged from several per-test runs via
// `lcov -a`, each keeping its own TN); a bare TN: with no value, or no TN
// line at all (the overwhelming common case: one full-suite run), leaves
// Tests nil.
//
// A DA line outside any SF record, or a malformed DA/hits value, is a
// reportable parse error — never silently skipped — since either means
// lcov's own grammar was violated and every hit count downstream would be
// suspect.
func ParseLcov(r io.Reader) ([]ForeignBlock, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var out []ForeignBlock
	curFile := ""
	curTest := ""
	inRecord := false
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "TN:"):
			curTest = strings.TrimSpace(strings.TrimPrefix(line, "TN:"))
		case strings.HasPrefix(line, "SF:"):
			curFile = strings.TrimSpace(strings.TrimPrefix(line, "SF:"))
			inRecord = true
		case strings.HasPrefix(line, "DA:"):
			if !inRecord {
				return nil, fmt.Errorf("lcov: line %d: DA record before any SF:", lineNo)
			}
			fields := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
			if len(fields) < 2 {
				return nil, fmt.Errorf("lcov: line %d: malformed DA record (%d field(s), want at least 2)", lineNo, len(fields))
			}
			ln, err := strconv.Atoi(strings.TrimSpace(fields[0]))
			if err != nil {
				return nil, fmt.Errorf("lcov: line %d: malformed DA line number: %w", lineNo, err)
			}
			hits, err := strconv.Atoi(strings.TrimSpace(fields[1]))
			if err != nil {
				return nil, fmt.Errorf("lcov: line %d: malformed DA hit count: %w", lineNo, err)
			}
			covered := 0
			if hits > 0 {
				covered = 1
			}
			var tests []string
			if curTest != "" {
				tests = []string{curTest}
			}
			out = append(out, ForeignBlock{
				File:       curFile,
				StartLine:  ln,
				EndLine:    ln,
				Statements: 1,
				Covered:    covered,
				Tests:      tests,
			})
		case line == "end_of_record":
			inRecord = false
			curFile = ""
		default:
			// FN/FNDA/FNF/FNH, BRDA/BRF/BRH, and any other lcov section:
			// function- or branch-level, not this parser's concern.
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("lcov: scan: %w", err)
	}
	return out, nil
}
