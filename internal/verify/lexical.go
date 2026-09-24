package verify

// lexical.go — the lexical oracle (ADR-030, docs/plans/proof-credibility.md
// PC-17/PC-18). A plan row that is not code — a measurement file, a
// changelog entry, a design doc, a .gitignore line — may carry
// `{oracle: lexical, path: <repo path>, pattern: <regexp>}` instead of a
// symbol. The row verifies when the path exists in the working tree
// (the same tree `fathom verify` reads the plan itself from — never
// `git show HEAD:`, so a plan and its evidence are always read at the same
// moment), is non-empty, and the pattern matches at least once. It leaves
// U2 because it IS a contract: the gate can refute it.
//
// What it refuses, by construction: a markdown path with no pattern (a
// file that merely exists proves nothing about a claim), a row that also
// carries a symbol (one contract per row), a path outside the repo, and a
// pattern that does not compile. Each is a LintPlan finding AND a verify
// failure — lint stays advisory in exit codes, the gate does not.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// lexicalCriterion is the criterion name a lexical row reports under.
const lexicalCriterion = "lexical_oracle"

// lexicalContractError returns the static reason a lexical row's contract
// is malformed, or "" when its shape is sound. Pure; shared by LintPlan and
// evalLexicalOracle so the two can never disagree about what is refused.
func lexicalContractError(t Task) string {
	switch {
	case t.Oracle != "lexical":
		return fmt.Sprintf("unknown oracle %q (want lexical)", t.Oracle)
	case t.Expect.Symbol != "":
		return "one contract per row: symbol and oracle both set"
	case t.OraclePath == "":
		return "lexical oracle without a path"
	case filepath.IsAbs(t.OraclePath) || strings.HasPrefix(t.OraclePath, "../") || strings.Contains(t.OraclePath, "/../") || t.OraclePath == "..":
		return fmt.Sprintf("path %q must be repo-relative", t.OraclePath)
	case strings.HasSuffix(strings.ToLower(t.OraclePath), ".md") && t.OraclePattern == "":
		return fmt.Sprintf("markdown path %q without a pattern proves nothing about a claim (ADR-030)", t.OraclePath)
	case t.Eq != nil && (t.Max != nil || t.Min != nil):
		return "eq cannot be combined with max or min (CV-29): one bound shape per row"
	case t.Max != nil && *t.Max < 0:
		return fmt.Sprintf("max %d must be non-negative", *t.Max)
	case t.Min != nil && *t.Min < 0:
		return fmt.Sprintf("min %d must be non-negative", *t.Min)
	case t.Eq != nil && *t.Eq < 0:
		return fmt.Sprintf("eq %d must be non-negative", *t.Eq)
	case t.Max != nil && t.Min != nil && *t.Max < *t.Min:
		return fmt.Sprintf("max %d is less than min %d", *t.Max, *t.Min)
	case len(t.BoundsInvalid) > 0:
		return fmt.Sprintf("malformed integer bound(s): %s", strings.Join(t.BoundsInvalid, ", "))
	}
	if t.OraclePattern != "" {
		if _, err := regexp.Compile("(?m)" + t.OraclePattern); err != nil {
			return fmt.Sprintf("pattern does not compile: %v", err)
		}
	}
	return ""
}

// lintLexicalOracle reports the contract-shape refusals for one row (PC-18).
// A row with no oracle yields nothing; a `path:` or `pattern:` without
// `oracle:` never reaches here because the parser reads them only alongside
// it. Findings are in the same words evalLexicalOracle fails with.
func lintLexicalOracle(t Task) []string {
	if t.Oracle == "" {
		return nil
	}
	if msg := lexicalContractError(t); msg != "" {
		return []string{fmt.Sprintf("%s: lexical oracle refused — %s", t.ID, msg)}
	}
	return nil
}

// evalLexicalOracle evaluates one lexical row against files (PC-17) and
// returns the criterion plus the verdict label a failure or unknown carries.
// files == nil (an Evidence with no repo access) is Unknown with the reason,
// never a pass and never a fail: the contract is sound, the instrument
// simply cannot read the tree from here.
func evalLexicalOracle(t Task, files FileEvidence) (Criterion, string) {
	if msg := lexicalContractError(t); msg != "" {
		return Criterion{lexicalCriterion, Fail, msg}, "contract-invalid"
	}
	if files == nil {
		return Criterion{lexicalCriterion, Unknown, "no repo file access on this evidence source — run fathom verify from the repo (working tree) to evaluate the lexical oracle"}, "oracle-unevaluated"
	}
	body, err := files.ReadRepoFile(t.OraclePath)
	if err != nil {
		return Criterion{lexicalCriterion, Fail, fmt.Sprintf("path %q not readable in the working tree: %v", t.OraclePath, err)}, "path-absent"
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return Criterion{lexicalCriterion, Fail, fmt.Sprintf("path %q is empty", t.OraclePath)}, "path-empty"
	}
	if t.OraclePattern == "" {
		return Criterion{lexicalCriterion, Pass, fmt.Sprintf("%s exists in the working tree (%d bytes, non-empty)", t.OraclePath, len(body))}, ""
	}
	re := regexp.MustCompile("(?m)" + t.OraclePattern) // compiled once already in lexicalContractError
	locs := re.FindAllIndex(body, -1)
	return lexicalBoundsVerdict(t, body, locs)
}

// lexicalBoundsVerdict decides a lexical row's pass/fail from its pattern's
// match count (CV-29, ADR-044 §3.1: "gone: pattern" and every other
// lexical row now evaluate max, min and eq against the match count, not
// merely "at least one"). locs is regexp.FindAllIndex's result — nil or
// empty means zero matches.
//
// With none of Max/Min/Eq set, the oracle's ORIGINAL contract holds
// unchanged: count >= 1 ("at least one match"), reported exactly as
// before (label "pattern-unmatched" on zero, detail "matches at
// <path>:<line>" on the first match, for every existing caller/test that
// keys off those strings). Eq, when set, requires an exact count (refused
// combined with Max/Min by lexicalContractError, so this never sees both).
// Max and Min, either or both, bound the count as a range; either may be
// used alone (Max only: "at most N"; Min only: "at least N", replacing the
// bare ">=1" default with an explicit floor — CV-29's `gone: pattern` use,
// where "at least 0" i.e. Max alone proves an upper-bounded ratchet).
//
// Pure; the only I/O (reading path) already happened in evalLexicalOracle.
func lexicalBoundsVerdict(t Task, body []byte, locs [][]int) (Criterion, string) {
	count := len(locs)
	switch {
	case t.Eq != nil:
		if count != *t.Eq {
			return Criterion{lexicalCriterion, Fail, fmt.Sprintf("pattern %q matched %d time(s) in %s, want exactly %d", t.OraclePattern, count, t.OraclePath, *t.Eq)}, "bounds-unmet"
		}
	case t.Max != nil || t.Min != nil:
		if t.Min != nil && count < *t.Min {
			return Criterion{lexicalCriterion, Fail, fmt.Sprintf("pattern %q matched %d time(s) in %s, want at least %d", t.OraclePattern, count, t.OraclePath, *t.Min)}, "bounds-unmet"
		}
		if t.Max != nil && count > *t.Max {
			return Criterion{lexicalCriterion, Fail, fmt.Sprintf("pattern %q matched %d time(s) in %s, want at most %d", t.OraclePattern, count, t.OraclePath, *t.Max)}, "bounds-unmet"
		}
	default:
		if count == 0 {
			return Criterion{lexicalCriterion, Fail, fmt.Sprintf("pattern %q matches nothing in %s (%d bytes)", t.OraclePattern, t.OraclePath, len(body))}, "pattern-unmatched"
		}
	}
	if count == 0 {
		// Only reachable with an explicit Min <= 0 or Eq == 0 — "proves
		// absence" (a gone: pattern's whole point). No match location to
		// cite; the count itself is the evidence.
		return Criterion{lexicalCriterion, Pass, fmt.Sprintf("pattern %q matched 0 time(s) in %s, as required (%d bytes, non-empty)", t.OraclePattern, t.OraclePath, len(body))}, ""
	}
	line := 1 + strings.Count(string(body[:locs[0][0]]), "\n")
	return Criterion{lexicalCriterion, Pass, fmt.Sprintf("pattern matches at %s:%d, %d match(es) total (%d bytes, non-empty)", t.OraclePath, line, count, len(body))}, ""
}

// DirFiles is the working-tree FileEvidence: repo-relative reads under one
// root, refusing any path that escapes it. The CLI wraps its store-mode
// evidence with it and overlays it onto daemon verdicts in HTTP mode, so
// the oracle is always evaluated against the tree the plan was read from.
type DirFiles string

// ErrOutsideRepo is returned for a path that resolves outside the root.
var ErrOutsideRepo = errors.New("path escapes the repo root")

// ReadRepoFile implements FileEvidence.
func (d DirFiles) ReadRepoFile(path string) ([]byte, error) {
	root := filepath.Clean(string(d))
	full := filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return nil, ErrOutsideRepo
	}
	return os.ReadFile(full)
}

// OverlayLocal re-evaluates the plan's lexical rows in rep against files
// and stamps every verdict with its plan line — the HTTP-mode complement
// to VerifyPlan's own FileEvidence path: the daemon that computed rep has
// the index but not this working tree, so its lexical verdicts are
// Unknown-with-reason and the CLI replaces exactly those, then recounts
// the summary. Deterministic; verdict order is untouched.
func OverlayLocal(rep *Report, plan Plan, files FileEvidence) {
	byID := make(map[string]Task, len(plan.Tasks))
	for _, t := range plan.Tasks {
		byID[t.ID] = t
	}
	for i := range rep.Tasks {
		t, ok := byID[rep.Tasks[i].TaskID]
		if !ok {
			continue
		}
		rep.Tasks[i].Line = t.Line
		if t.Oracle == "" {
			continue
		}
		rep.Tasks[i] = verifyTask(t, noEvidence{}, files, nil)
	}
	sum := Summary{Total: len(rep.Tasks)}
	for _, v := range rep.Tasks {
		switch v.Status {
		case Pass:
			sum.Pass++
		case Fail:
			sum.Fail++
		default:
			sum.Unknown++
		}
	}
	rep.Summary = sum
}

// noEvidence is the empty index OverlayLocal evaluates lexical rows
// against: a lexical row carries no symbol by contract, so nothing here is
// ever consulted; it exists only to satisfy verifyTask's signature.
type noEvidence struct{}

func (noEvidence) FindSymbol(string) []core.SymbolNode  { return nil }
func (noEvidence) TestReaches(string) []string          { return nil }
func (noEvidence) Reachable(string, string) bool        { return false }
func (noEvidence) Contradictions() []core.Contradiction { return nil }
func (noEvidence) SpecImplementors(string) []string     { return nil }
