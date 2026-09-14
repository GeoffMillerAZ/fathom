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
//   - go-mutesting's own log output (github.com/zimmski/go-mutesting): each
//     mutant is announced with a line containing
//     "Executing mutation for <file>:<startLine>.<startCol>,<endLine>.<endCol>"
//     and later closed by a line containing "Mutation is killed" or
//     "Mutation is NOT killed" (case-insensitive) — go-mutesting's own
//     runner messages, timestamp-prefixed by the standard log package, so
//     matching is substring/regexp, never a full-line anchor.
//   - fathom's own JSON shape, for any OTHER mutation tool a caller adapts
//     to it: {"mutants":[{"file":"path/to/x.go","line":42,"killed":true}]}
//     (a bare top-level JSON array of the same records is accepted too).
//
// Detection is by content, not a flag: input starting with '{' or '[' (once
// leading whitespace is trimmed) is parsed as JSON; anything else is parsed
// as go-mutesting text. A mutation report carries no statement-level block
// data (unlike a cover profile) — one line, one mutant.
import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// MutantResult is one mutant's outcome at a file:line location — the
// mutation-testing analogue of a cover Block, but a single point rather
// than a statement span (neither input shape below carries end-of-mutation
// span data reliably enough to trust).
type MutantResult struct {
	File   string
	Line   int
	Killed bool // true: the test suite caught the mutation; false: it survived
}

// MutationReport is a parsed mutation-testing run, ready to join against
// indexed symbols via JoinMutationSymbols.
type MutationReport struct {
	Mutants []MutantResult
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

// execMutationRe matches go-mutesting's "Executing mutation for
// <file>:<startLine>.<startCol>,<endLine>.<endCol>" runner message —
// substring match against a (possibly timestamp-prefixed) log line, never a
// full-line anchor.
var execMutationRe = regexp.MustCompile(`Executing mutation for (\S+):(\d+)\.\d+,\d+\.\d+`)

// killedMutationRe matches go-mutesting's "Mutation is killed" / "Mutation
// is NOT killed" runner message, case-insensitively.
var killedMutationRe = regexp.MustCompile(`(?i)Mutation is (NOT )?killed`)

// ParseMutationReport parses a mutation-testing run from r — see the
// package-level doc comment for the two accepted shapes. A go-mutesting
// "Executing mutation for" line with no matching "Mutation is (NOT) killed"
// line before EOF (a truncated log, or a run go-mutesting itself aborted) is
// dropped silently: an unresolved mutant asserts nothing either way, so it
// is never fabricated as killed or survived.
func ParseMutationReport(r io.Reader) (MutationReport, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return MutationReport{}, fmt.Errorf("read mutation report: %w", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return parseMutationJSON([]byte(trimmed))
	}
	return parseGoMutestingText(trimmed), nil
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
		return rep, nil
	}
	var bare []jsonMutant
	if err := json.Unmarshal(data, &bare); err != nil {
		return MutationReport{}, fmt.Errorf("parse mutation report JSON: %w", err)
	}
	for _, m := range bare {
		rep.Mutants = append(rep.Mutants, MutantResult{File: m.File, Line: m.Line, Killed: m.Killed})
	}
	return rep, nil
}

func parseGoMutestingText(text string) MutationReport {
	var rep MutationReport
	var pendingFile string
	var pendingLine int
	havePending := false

	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if m := execMutationRe.FindStringSubmatch(line); m != nil {
			if n, err := strconv.Atoi(m[2]); err == nil {
				pendingFile, pendingLine, havePending = m[1], n, true
			}
			continue
		}
		if m := killedMutationRe.FindStringSubmatch(line); m != nil && havePending {
			rep.Mutants = append(rep.Mutants, MutantResult{File: pendingFile, Line: pendingLine, Killed: m[1] == ""})
			havePending = false
		}
	}
	return rep
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
// tally. A symbol with no mutant inside it is omitted entirely (like
// JoinSymbols, never a fabricated zero score). Output is sorted by (File,
// Line): deterministic given identical inputs.
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
		return out[i].Line < out[j].Line
	})
	return out
}
