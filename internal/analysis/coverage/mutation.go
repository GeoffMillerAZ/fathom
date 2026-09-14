package coverage

// mutation.go — VB-34: a parsed mutation-testing run joined to indexed
// symbols, the same shape of join JoinSymbols performs for a Go cover
// profile. Statement coverage (join.go) answers "did SOME test execute
// this line"; mutation score answers the stronger question "would a test
// have NOTICED if this line broke" — a symbol can be 100% covered and
// still have every mutant survive (nothing in the reaching test actually
// asserts on the mutated behavior). ParseMutationReport accepts two input
// shapes:
//
//   - go-mutesting's own log output (github.com/zimmski/go-mutesting): for
//     the BUILT-IN exec command (the default; a custom `--exec` carries no
//     diff and so no file/line evidence at all — see below), each mutant
//     ends with one record line
//
//     PASS "<tmpdir>/<file>.<n>" with checksum <hex>   // exec exit 1: tests failed -> mutant KILLED
//     FAIL "<tmpdir>/<file>.<n>" with checksum <hex>   // exec exit 0: tests passed -> mutant SURVIVED
//     SKIP "<tmpdir>/<file>.<n>" with checksum <hex>   // exec exit 2: mutation did not compile
//
//     (cmd/go-mutesting/main.go's mutateExec exit-code switch: 0->FAIL,
//     1->PASS, 2->SKIP — the labels name whether the exec step matched
//     go-mutesting's own expectation, not the mutant's fate directly).
//     Immediately before a FAIL record, `diff -u <original> <mutated>`
//     output is ALWAYS printed (unconditionally, so file/line are always
//     recoverable for a survived mutant); before a PASS or SKIP record it
//     is printed ONLY under `--debug`. The diff's own header
//     ("--- <path>\t<mtime>", tab-separated, standard `diff -u` framing)
//     names the source file, and the first "-" (removed) or "+" (inserted)
//     line inside the following "@@ -oldStart,oldCount +newStart,newCount
//     @@" hunk — counting context lines from oldStart, NOT oldStart itself,
//     which is usually a few lines of context short of the actual change —
//     names the mutated line in the ORIGINAL file. A record with no diff
//     (a non-debug PASS/SKIP) still yields a FILE: the mutant path's tail
//     after its "go-mutesting-<digits>" temp-dir segment (ioutil.TempDir's
//     own naming — main.go's `ioutil.TempDir("", "go-mutesting-")`) mirrors
//     the source tree exactly, so stripping that prefix and the trailing
//     ".<n>" mutation-index suffix recovers the relative path; only the
//     LINE is lost. Such a record is still kept (Line 0) — MutationReport
//     counts it in Unattributed rather than dropping it, and
//     JoinMutationSymbols only attributes records with Line>0. The
//     recommended invocation for symbol-level attribution is therefore
//     `go-mutesting --debug ./... > workspace/artifacts/mutation.txt`. A
//     SKIP record (did not compile) carries no killed/survived evidence
//     either way and is counted in MutationReport.Skipped, never joined.
//     The trailing "The mutation score is F (P passed, F failed, D
//     duplicated, S skipped, total is T)" line is cross-checked against
//     the parsed PASS/FAIL/SKIP counts; a mismatch (or its absence when
//     records were found) is a parse error, never a silent partial.
//   - fathom's own JSON shape, for any OTHER mutation tool a caller adapts
//     to it: {"mutants":[{"file":"path/to/x.go","line":42,"killed":true}]}
//     (a bare top-level JSON array of the same records is accepted too).
//
// Detection is by content, not a flag: input starting with '{' or '[' (once
// leading whitespace is trimmed) is parsed as JSON; anything else is parsed
// as go-mutesting text. A mutation report carries no statement-level block
// data (unlike a cover profile) — one line, one mutant.
import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// MutantResult is one mutant's outcome — the mutation-testing analogue of
// a cover Block, but a single point rather than a statement span. File and
// Line are both "" / 0 when a non-debug PASS/SKIP record's location could
// not be recovered at all (see the package doc comment); File alone can
// still be "" while Line is 0 in that case even when the mutant path's
// tail WAS recoverable, since a location with no line can't be joined to a
// symbol regardless.
type MutantResult struct {
	File   string
	Line   int
	Killed bool // true: the test suite caught the mutation; false: it survived
}

// MutationReport is a parsed mutation-testing run, ready to join against
// indexed symbols via JoinMutationSymbols.
type MutationReport struct {
	Mutants []MutantResult
	// Skipped counts go-mutesting SKIP records (mutation did not compile) —
	// no killed/survived evidence either way, so never added to Mutants.
	Skipped int
	// Unattributed counts entries in Mutants whose Line is 0 (a non-debug
	// PASS record with no diff to recover a line from) — so a caller can
	// see that mutants exist which JoinMutationSymbols could not attribute
	// to any symbol, rather than have them silently vanish from the count.
	Unattributed int
}

// SymbolMutation is one symbol's joined mutation-testing result.
type SymbolMutation struct {
	Symbol   string
	File     string
	Line     int
	Mutants  int
	Killed   int
	Survived int
	// Score is Killed/Mutants as a 0.0-1.0 fraction (go-mutesting's own
	// convention), 0 when Mutants is 0 — which cannot occur for an emitted
	// entry: JoinMutationSymbols, like JoinSymbols, only speaks for symbols
	// the report actually has a mutant inside.
	Score float64
}

// jsonMutant is one record of fathom's own mutation-report JSON shape.
type jsonMutant struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Killed bool   `json:"killed"`
}

// mutestingRecordRe matches go-mutesting's per-mutant outcome line: `PASS
// "<path>" with checksum <hex>` (and FAIL / SKIP) — cmd/go-mutesting's own
// `fmt.Sprintf("%q with checksum %s", mutationFile, checksum)` framing.
var mutestingRecordRe = regexp.MustCompile(`^(PASS|FAIL|SKIP) "(.+)" with checksum [0-9a-fA-F]+$`)

// mutestingDiffFileRe matches the `diff -u` unified-diff header's "---"
// line: "--- <path>\t<mtime>" — the tab is load-bearing (it is what
// distinguishes this from `go test`'s own "--- FAIL: TestName (0.00s)"
// line, which is space-separated and appears in the SAME transcript under
// --debug when the mutated build's test output is echoed).
var mutestingDiffFileRe = regexp.MustCompile("^--- (\\S+)\t")

// mutestingHunkRe matches a unified-diff hunk header: "@@ -oldStart,oldCount
// +newStart,newCount @@" (counts default to 1 when omitted, per the
// unified-diff format).
var mutestingHunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+\d+(?:,\d+)? @@`)

// mutestingSummaryRe matches go-mutesting's trailing summary line, used to
// cross-check the parsed PASS/FAIL/SKIP counts against the tool's own tally.
var mutestingSummaryRe = regexp.MustCompile(`^The mutation score is [\d.]+ \((\d+) passed, (\d+) failed, (\d+) duplicated, (\d+) skipped, total is (\d+)\)$`)

// mutestingTmpDirRe matches go-mutesting's temp-directory naming
// (`ioutil.TempDir("", "go-mutesting-")`, which appends a purely numeric
// suffix) inside a mutant's quoted path.
var mutestingTmpDirRe = regexp.MustCompile(`go-mutesting-\d+/`)

// mutestingIndexSuffixRe strips the trailing ".<mutationID>" go-mutesting
// appends to a mutated file's copy (`fmt.Sprintf("%s.%d", tmpFile,
// mutationID)`).
var mutestingIndexSuffixRe = regexp.MustCompile(`\.\d+$`)

// ParseMutationReport parses a mutation-testing run from r — see the
// package-level doc comment for the two accepted shapes and go-mutesting's
// exact wire format. Returns an error for input that is non-empty but
// yields zero recognised records, or whose parsed PASS/FAIL/SKIP counts
// disagree with go-mutesting's own trailing summary line — never a
// silently empty or partial report.
func ParseMutationReport(r io.Reader) (MutationReport, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return MutationReport{}, fmt.Errorf("read mutation report: %w", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return MutationReport{}, nil
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return parseMutationJSON([]byte(trimmed))
	}
	return parseGoMutestingText(trimmed)
}

func parseMutationJSON(data []byte) (MutationReport, error) {
	var wrapped struct {
		Mutants []jsonMutant `json:"mutants"`
	}
	var rep MutationReport
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Mutants != nil {
		for _, m := range wrapped.Mutants {
			rep.Mutants = append(rep.Mutants, MutantResult{File: m.File, Line: m.Line, Killed: m.Killed})
		}
		return finishMutationReport(rep), nil
	}
	var bare []jsonMutant
	if err := json.Unmarshal(data, &bare); err != nil {
		return MutationReport{}, fmt.Errorf("parse mutation report JSON: %w", err)
	}
	for _, m := range bare {
		rep.Mutants = append(rep.Mutants, MutantResult{File: m.File, Line: m.Line, Killed: m.Killed})
	}
	return finishMutationReport(rep), nil
}

// finishMutationReport computes Unattributed from the final Mutants slice.
func finishMutationReport(rep MutationReport) MutationReport {
	for _, m := range rep.Mutants {
		if m.Line <= 0 {
			rep.Unattributed++
		}
	}
	return rep
}

// recoverFileFromMutantPath recovers a mutant's relative source path from
// its quoted temp-file path (see the package doc comment): everything
// after the "go-mutesting-<digits>/" segment, minus the trailing
// ".<mutationID>" suffix. Returns "" when the temp-dir marker isn't found
// (an unexpected go-mutesting version or a custom TMPDIR layout) — never a
// guess.
func recoverFileFromMutantPath(path string) string {
	loc := mutestingTmpDirRe.FindStringIndex(path)
	if loc == nil {
		return ""
	}
	rel := mutestingIndexSuffixRe.ReplaceAllString(path[loc[1]:], "")
	return rel
}

// firstChangedOldLine walks a unified-diff hunk body (lines[start:], the
// lines immediately after an "@@" header whose old-file start is oldStart)
// and returns the ORIGINAL file's line number of the first actually-changed
// line: the first "-" (removed) or "+" (inserted, reported at its
// insertion point) line, counting every context (" ") and removed ("-")
// line against the old file's numbering (an inserted "+" line has no old-
// file line of its own). oldStart itself is USUALLY just where the diff's
// context window begins, a few lines short of the real change — this walks
// forward to find it rather than reporting oldStart blindly. Returns 0 (an
// honest "could not determine") if the hunk body is empty, malformed, or
// contains no changed line before the diff ends.
func firstChangedOldLine(lines []string, start, oldStart int) int {
	oldLine := oldStart
	for i := start; i < len(lines); i++ {
		l := lines[i]
		if l == "" {
			return 0 // blank separator line: end of the diff, no change found
		}
		switch l[0] {
		case '-':
			return oldLine
		case '+':
			return oldLine
		case ' ':
			oldLine++
		case '\\':
			// "\ No newline at end of file" — no line-count effect.
		default:
			return 0 // not diff content — end of the hunk/diff
		}
	}
	return 0
}

// mutestingRecord is one fully-resolved PASS/FAIL/SKIP line plus whatever
// diff immediately preceded it.
type mutestingRecord struct {
	status   string // PASS | FAIL | SKIP
	path     string // the raw quoted mutant path
	diffFile string // "" when no diff was recovered
	diffLine int    // 0 when no diff was recovered (or it had no changed line)
}

// scanMutestingRecords walks lines once, pairing each PASS/FAIL/SKIP record
// with the diff (if any) that immediately preceded it — go-mutesting emits
// at most one diff per mutant, directly before its record line.
func scanMutestingRecords(lines []string) []mutestingRecord {
	var records []mutestingRecord
	var pendingFile string
	var pendingLine int
	havePendingDiff := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if m := mutestingDiffFileRe.FindStringSubmatch(line); m != nil {
			// go-mutesting's diff is always exactly "--- ...\n+++ ...\n@@ ... @@\n<body>":
			// the "+++" line is always immediately next, the hunk header
			// immediately after that.
			if i+2 < len(lines) {
				if hm := mutestingHunkRe.FindStringSubmatch(lines[i+2]); hm != nil {
					oldStart, _ := strconv.Atoi(hm[1])
					pendingFile = m[1]
					pendingLine = firstChangedOldLine(lines, i+3, oldStart)
					havePendingDiff = true
				}
			}
			continue
		}
		if m := mutestingRecordRe.FindStringSubmatch(line); m != nil {
			rec := mutestingRecord{status: m[1], path: m[2]}
			if havePendingDiff {
				rec.diffFile, rec.diffLine = pendingFile, pendingLine
			}
			records = append(records, rec)
			pendingFile, pendingLine, havePendingDiff = "", 0, false
			continue
		}
	}
	return records
}

// parseGoMutestingText parses go-mutesting's log transcript — see the
// package doc comment for the exact wire format this implements.
func parseGoMutestingText(text string) (MutationReport, error) {
	lines := strings.Split(text, "\n")
	records := scanMutestingRecords(lines)

	var passed, failed, skipped int
	var mutants []MutantResult
	for _, rec := range records {
		switch rec.status {
		case "PASS":
			passed++
			file, line := rec.diffFile, rec.diffLine
			if file == "" {
				file = recoverFileFromMutantPath(rec.path)
			}
			mutants = append(mutants, MutantResult{File: file, Line: line, Killed: true})
		case "FAIL":
			failed++
			file, line := rec.diffFile, rec.diffLine
			if file == "" {
				file = recoverFileFromMutantPath(rec.path)
			}
			mutants = append(mutants, MutantResult{File: file, Line: line, Killed: false})
		case "SKIP":
			skipped++
		}
	}

	if len(records) == 0 {
		return MutationReport{}, fmt.Errorf("no go-mutesting records recognised")
	}

	var summary []string
	for _, line := range lines {
		if m := mutestingSummaryRe.FindStringSubmatch(line); m != nil {
			summary = m
			break
		}
	}
	if summary == nil {
		return MutationReport{}, fmt.Errorf("go-mutesting summary line not found — cannot cross-check %d parsed record(s)", len(records))
	}
	summaryPassed, _ := strconv.Atoi(summary[1])
	summaryFailed, _ := strconv.Atoi(summary[2])
	summarySkipped, _ := strconv.Atoi(summary[4])
	if summaryPassed != passed || summaryFailed != failed || summarySkipped != skipped {
		return MutationReport{}, fmt.Errorf(
			"go-mutesting summary mismatch: parsed %d passed/%d failed/%d skipped, summary line says %d/%d/%d",
			passed, failed, skipped, summaryPassed, summaryFailed, summarySkipped)
	}

	return finishMutationReport(MutationReport{Mutants: mutants, Skipped: skipped}), nil
}

// resolveMutationFile finds the repo-relative file in repoFiles (longest
// first, matching joinBlocks' own tie-break) that path names — exact match
// or path is a longer, absolute-ish string ending in "/"+file — or ""
// when none matches.
func resolveMutationFile(path string, repoFiles []string) string {
	for _, rf := range repoFiles {
		if path == rf || strings.HasSuffix(path, "/"+rf) {
			return rf
		}
	}
	return ""
}

// JoinMutationSymbols joins report's mutant outcomes to symbols: for every
// function/method symbol whose [Line, EndLine] span contains a mutant's
// Line, the mutant is added to that symbol's Mutants/Killed/Survived
// tally. A mutant with Line<=0 (see MutationReport.Unattributed) can never
// match any symbol span and is skipped here, same as one whose File
// resolves to no indexed symbol. A symbol with no mutant inside it is
// omitted entirely (like JoinSymbols, never a fabricated zero score).
// Output is sorted by (File, Line, Symbol): deterministic given identical
// inputs, with Symbol as the final tiebreak for two symbols sharing a
// (File, Line) — e.g. a synthesized EndLine edge case.
func JoinMutationSymbols(report MutationReport, symbols []core.SymbolNode) []SymbolMutation {
	fileSyms := make(map[string][]core.SymbolNode)
	var repoFiles []string
	for _, s := range symbols {
		if !coverageEligible(s.Kind) {
			continue
		}
		if _, ok := fileSyms[s.FilePath]; !ok {
			repoFiles = append(repoFiles, s.FilePath)
		}
		fileSyms[s.FilePath] = append(fileSyms[s.FilePath], s)
	}
	// EndLine synthesis mirrors joinBlocks exactly (see JoinSymbols' doc
	// comment) so a mutation report and a cover profile agree on which
	// symbol a given line belongs to.
	for fp, syms := range fileSyms {
		sort.Slice(syms, func(i, j int) bool { return syms[i].Line < syms[j].Line })
		for i := range syms {
			if syms[i].EndLine > 0 {
				continue
			}
			if i+1 < len(syms) {
				syms[i].EndLine = syms[i+1].Line - 1
			} else {
				syms[i].EndLine = 1<<31 - 1
			}
		}
		fileSyms[fp] = syms
	}
	sort.Slice(repoFiles, func(i, j int) bool { return len(repoFiles[i]) > len(repoFiles[j]) })

	resolved := make(map[string]string)
	resolve := func(path string) string {
		if fp, ok := resolved[path]; ok {
			return fp
		}
		fp := resolveMutationFile(path, repoFiles)
		resolved[path] = fp
		return fp
	}

	type accum struct {
		sym      core.SymbolNode
		mutants  int
		killed   int
		survived int
	}
	byKey := make(map[string]*accum)
	for _, m := range report.Mutants {
		if m.Line <= 0 {
			continue
		}
		fp := resolve(m.File)
		if fp == "" {
			continue
		}
		for _, s := range fileSyms[fp] {
			if m.Line < s.Line || m.Line > s.EndLine {
				continue
			}
			key := s.Key()
			a, ok := byKey[key]
			if !ok {
				a = &accum{sym: s}
				byKey[key] = a
			}
			a.mutants++
			if m.Killed {
				a.killed++
			} else {
				a.survived++
			}
		}
	}

	out := make([]SymbolMutation, 0, len(byKey))
	for key, a := range byKey {
		score := 0.0
		if a.mutants > 0 {
			score = float64(a.killed) / float64(a.mutants)
		}
		out = append(out, SymbolMutation{
			Symbol: key, File: a.sym.FilePath, Line: a.sym.Line,
			Mutants: a.mutants, Killed: a.killed, Survived: a.survived, Score: score,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}
