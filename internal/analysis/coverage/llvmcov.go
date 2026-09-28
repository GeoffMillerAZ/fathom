package coverage

// llvmcov.go — ParseLLVMCov (TQ-13): `llvm-cov export -format=text`'s
// JSON export (schema "llvm.coverage.json.export" — Clang/LLVM's own
// coverage tool for C/C++/Objective-C/Swift and any other Clang-instrumented
// target), producing ForeignBlock at line granularity from its per-file
// `segments` arrays.

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// llvmCovSchema is the "type" field llvm-cov's JSON export always carries
// — checked so a caller who mis-detected the format (this repo's --format
// flag requires an explicit choice; nothing sniffs .json content the way
// ParseCoberturaJaCoCo sniffs an XML root) gets a named error instead of
// a silently empty or wrong result.
const llvmCovSchema = "llvm.coverage.json.export"

// llvmCovDoc is the subset of llvm-cov's export schema ParseLLVMCov reads:
// one or more `data` entries (one per `llvm-cov export` invocation's own
// input, ordinarily just one), each holding the files it covered. A
// file's `segments` is an array of 6-element rows
// [line, col, count, hasCount, isRegionEntry, isGapRegion] — llvm-cov's
// own region-boundary encoding, decoded here as []interface{} since JSON
// gives no fixed-arity array type.
type llvmCovDoc struct {
	Type string `json:"type"`
	Data []struct {
		Files []struct {
			Filename string          `json:"filename"`
			Segments [][]interface{} `json:"segments"`
		} `json:"files"`
	} `json:"data"`
}

// ParseLLVMCov parses an llvm-cov JSON export into ForeignBlock at line
// granularity: for each file, every segment naming a real code region
// (hasCount true, isGapRegion false — a gap region marks source with no
// instrumentable code, never a "line that didn't run") contributes its
// line to that file's covered-line set if its count > 0. Two segments on
// the same line (region boundaries mid-line are common: an `if` and its
// body starting on one physical line) merge by OR — the line counts
// covered if ANY segment on it does, the same "did this line run at all"
// convention ParseLcov/ParseCoberturaJaCoCo/ParseCoveragePyIstanbul use.
// One ForeignBlock is emitted per distinct line, Statements always 1.
//
// Output is sorted by (File, StartLine): llvm-cov's own segment order is
// (line, col) within a file, but files themselves carry no ordering
// guarantee across `data` entries, so this parser sorts rather than
// trusting document order (unlike ParseLcov/ParseCoberturaJaCoCo, whose
// formats have exactly one ordering axis to begin with).
func ParseLLVMCov(r io.Reader) ([]ForeignBlock, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("llvm-cov: read: %w", err)
	}
	var doc llvmCovDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("llvm-cov: unmarshal: %w", err)
	}
	if doc.Type != llvmCovSchema {
		return nil, fmt.Errorf("llvm-cov: unrecognized \"type\" %q (want %q)", doc.Type, llvmCovSchema)
	}

	type fileLines struct {
		covered map[int]bool
		order   []int
	}
	byFile := make(map[string]*fileLines)
	var fileOrder []string

	for _, d := range doc.Data {
		for _, f := range d.Files {
			fl, ok := byFile[f.Filename]
			if !ok {
				fl = &fileLines{covered: map[int]bool{}}
				byFile[f.Filename] = fl
				fileOrder = append(fileOrder, f.Filename)
			}
			for i, seg := range f.Segments {
				line, count, hasCount, isGapRegion, err := parseLLVMCovSegment(seg)
				if err != nil {
					return nil, fmt.Errorf("llvm-cov: file %s: segment %d: %w", f.Filename, i, err)
				}
				if !hasCount || isGapRegion {
					continue
				}
				if _, seen := fl.covered[line]; !seen {
					fl.order = append(fl.order, line)
				}
				if count > 0 {
					fl.covered[line] = true
				} else if _, seen := fl.covered[line]; !seen {
					fl.covered[line] = false
				}
			}
		}
	}
	sort.Strings(fileOrder)

	var out []ForeignBlock
	for _, name := range fileOrder {
		fl := byFile[name]
		lines := append([]int(nil), fl.order...)
		sort.Ints(lines)
		for _, ln := range lines {
			c := 0
			if fl.covered[ln] {
				c = 1
			}
			out = append(out, ForeignBlock{File: name, StartLine: ln, EndLine: ln, Statements: 1, Covered: c})
		}
	}
	return out, nil
}

// parseLLVMCovSegment extracts (line, count, hasCount, isGapRegion) from
// one raw `[line, col, count, hasCount, isRegionEntry, isGapRegion]`
// segment row.
func parseLLVMCovSegment(seg []interface{}) (line, count int, hasCount, isGapRegion bool, err error) {
	if len(seg) < 6 {
		return 0, 0, false, false, fmt.Errorf("malformed segment (%d field(s), want 6)", len(seg))
	}
	lineF, ok := seg[0].(float64)
	if !ok {
		return 0, 0, false, false, fmt.Errorf("segment field 0 (line) is not a number")
	}
	countF, ok := seg[2].(float64)
	if !ok {
		return 0, 0, false, false, fmt.Errorf("segment field 2 (count) is not a number")
	}
	hasCountB, ok := seg[3].(bool)
	if !ok {
		return 0, 0, false, false, fmt.Errorf("segment field 3 (hasCount) is not a boolean")
	}
	isGapB, ok := seg[5].(bool)
	if !ok {
		return 0, 0, false, false, fmt.Errorf("segment field 5 (isGapRegion) is not a boolean")
	}
	return int(lineF), int(countF), hasCountB, isGapB, nil
}
