package coverage

// coveragepy_istanbul.go — ParseCoveragePyIstanbul (TQ-12): Python's
// `coverage json` output and JavaScript/TypeScript's Istanbul
// `coverage-final.json`, both JSON but shaped nothing alike — one
// exported function, sniffing which by top-level keys the same way
// ParseCoberturaJaCoCo sniffs an XML root element, since neither format
// self-declares a schema name a caller could pass instead.

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// coveragePyDoc is the subset of coverage.py's `coverage json` schema
// ParseCoveragePyIstanbul reads: per file, the line numbers it executed
// and the ones its summary says it missed — coverage.py's own two
// disjoint sets, together the full instrumented-line list (branch detail,
// when `--show-contexts`/branch coverage is on, is not read here).
type coveragePyDoc struct {
	Meta  json.RawMessage `json:"meta"`
	Files map[string]struct {
		ExecutedLines []int `json:"executed_lines"`
		MissingLines  []int `json:"missing_lines"`
	} `json:"files"`
}

// istanbulFileEntry is one file's entry in an Istanbul coverage-final.json
// (top-level: a map keyed by the file's own path, no wrapper object)
// ParseCoveragePyIstanbul reads: statementMap gives each statement id's
// source span, s its hit count by the same id.
type istanbulFileEntry struct {
	StatementMap map[string]struct {
		Start struct {
			Line int `json:"line"`
		} `json:"start"`
		End struct {
			Line int `json:"line"`
		} `json:"end"`
	} `json:"statementMap"`
	S map[string]int `json:"s"`
}

// ParseCoveragePyIstanbul parses a coverage.py `coverage json` report or
// an Istanbul coverage-final.json report, detected from the document's
// own top-level keys: both "meta" and "files" present means coverage.py
// (coverage.py always emits both); their absence means Istanbul, whose
// top level is itself a map of file path to that file's statement data —
// there is no third shape either format can produce, so anything else is
// a reportable "empty or unrecognized" error rather than a silent empty
// result.
//
// Output is sorted by (File, StartLine): unlike the line-ordered XML/lcov
// formats, a JSON object's key order is not part of either format's own
// grammar (encoding/json, and every producer, treats it as unordered), so
// determinism here requires an explicit sort rather than "preserve
// document order".
func ParseCoveragePyIstanbul(r io.Reader) ([]ForeignBlock, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("coverage.py/istanbul: read: %w", err)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("coverage.py/istanbul: unmarshal: %w", err)
	}
	_, hasMeta := probe["meta"]
	_, hasFiles := probe["files"]
	if hasMeta && hasFiles {
		return parseCoveragePy(data)
	}
	if len(probe) == 0 {
		return nil, fmt.Errorf("coverage.py/istanbul: empty JSON object (want coverage.py's {meta,files,...} or an Istanbul file-path-keyed map)")
	}
	return parseIstanbul(data)
}

func parseCoveragePy(data []byte) ([]ForeignBlock, error) {
	var doc coveragePyDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("coverage.py: unmarshal: %w", err)
	}
	files := make([]string, 0, len(doc.Files))
	for f := range doc.Files {
		files = append(files, f)
	}
	sort.Strings(files)

	var out []ForeignBlock
	for _, f := range files {
		entry := doc.Files[f]
		covered := make(map[int]bool, len(entry.ExecutedLines)+len(entry.MissingLines))
		for _, ln := range entry.ExecutedLines {
			covered[ln] = true
		}
		for _, ln := range entry.MissingLines {
			if _, ok := covered[ln]; !ok {
				covered[ln] = false
			}
		}
		lines := make([]int, 0, len(covered))
		for ln := range covered {
			lines = append(lines, ln)
		}
		sort.Ints(lines)
		for _, ln := range lines {
			c := 0
			if covered[ln] {
				c = 1
			}
			out = append(out, ForeignBlock{File: f, StartLine: ln, EndLine: ln, Statements: 1, Covered: c})
		}
	}
	return out, nil
}

func parseIstanbul(data []byte) ([]ForeignBlock, error) {
	var doc map[string]istanbulFileEntry
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("istanbul: unmarshal: %w", err)
	}
	files := make([]string, 0, len(doc))
	for f := range doc {
		files = append(files, f)
	}
	sort.Strings(files)

	var out []ForeignBlock
	for _, f := range files {
		entry := doc[f]
		ids := make([]string, 0, len(entry.StatementMap))
		for id := range entry.StatementMap {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool {
			ni, erri := strconv.Atoi(ids[i])
			nj, errj := strconv.Atoi(ids[j])
			if erri == nil && errj == nil {
				return ni < nj
			}
			return ids[i] < ids[j]
		})
		for _, id := range ids {
			span := entry.StatementMap[id]
			covered := 0
			if entry.S[id] > 0 {
				covered = 1
			}
			out = append(out, ForeignBlock{
				File:       f,
				StartLine:  span.Start.Line,
				EndLine:    span.End.Line,
				Statements: 1,
				Covered:    covered,
			})
		}
	}
	return out, nil
}
