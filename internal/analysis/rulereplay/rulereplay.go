// Package rulereplay answers VB-16's question (docs/plans/verbs.md wave C;
// docs/vision/29-verbs-not-lookups.md §5 bullet 3): "a compiled layer rule
// catches nothing in the last 200 commits and nothing in thirty days of
// dogfood — governance stays advisory." A layer rule that has never fired
// might be silently useless, or might be silently perfect; nobody can tell
// without replaying it against real history. This package does that:
// evaluate TODAY's authored layer rules (.fathom/guard.cue's layer_rules)
// against every commit in a historical range, and report which commits
// would have failed the gate — and, of those, which were a NEW regression
// at that commit versus one merely inherited from its parent.
//
// Deliberately a THIRD producer of a guardspec.LayerRuleIndex, alongside
// internal/guardspec.BuildIndexFromSymbols (call-derived, live obligations)
// and internal/guardspec/layerrules.BuildIndex (go/packages, `fathom rules
// gen-test`): replaying 200 historical commits needs neither a running
// index nor a fully resolvable module at each of 200 different dependency
// states — go/packages' `go list` can fail or hang against an old go.mod
// whose requires no longer resolve offline. go/parser only needs the
// commit's own source text, so BuildImportGraphFromDir (importgraph.go)
// reads each archived commit's tree directly. No daemon, no store, no
// workspace/db — see the package's own tests for the synthetic-repo proof
// of new-vs-inherited classification.
package rulereplay

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// Options configures one Replay call.
type Options struct {
	// RepoRoot is the git repository whose history is replayed. Must be a
	// real git checkout (not a bare archive) — Replay shells out to `git
	// -C RepoRoot ...` throughout, and only ever reads it (git log, git
	// archive); it never touches RepoRoot's working tree.
	RepoRoot string
	// RuleFile is the `.fathom/guard.cue` file whose layer_rules are
	// evaluated — read ONCE, from this path (typically the current
	// checkout's committed rules), not re-read per historical commit. The
	// question VB-16 asks is "would TODAY's rules have caught anything in
	// the past", not "were yesterday's rules satisfied by yesterday's
	// code".
	RuleFile string
	// Since selects the replay range: either a bare positive integer N
	// (the last N commits reachable from HEAD) or a git ref (every commit
	// reachable from HEAD but not from the ref — the two-dot range
	// `ref..HEAD`). A value already containing ".." is passed to git
	// verbatim.
	Since string
}

// ViolationRecord is one rule violation observed at one commit.
type ViolationRecord struct {
	RuleID   string `json:"rule_id"`
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	From     string `json:"from"`
	To       string `json:"to"`
	Message  string `json:"message"`
	// New is true when this exact (RuleID, From, To) violation is absent
	// from the commit's first parent — i.e. THIS commit introduced it.
	// False means it was already present at the parent (inherited). A
	// commit with no parent (the range's own root) reports every
	// violation as New: there is no prior state to inherit from.
	New bool `json:"new"`
}

// CommitResult is one commit's replay outcome.
type CommitResult struct {
	SHA        string            `json:"sha"`
	Date       time.Time         `json:"date"`
	Subject    string            `json:"subject"`
	Violations []ViolationRecord `json:"violations,omitempty"`
	// Warnings names any production .go file under this commit's tree
	// that BuildImportGraphFromDir could not parse — reported, never
	// silently swallowed, though none are expected in a repo whose own
	// `go vet` gate is green at every commit.
	Warnings []string `json:"warnings,omitempty"`
}

// Report is one Replay call's full, deterministic output: commits in
// oldest → newest order, violations within a commit sorted by
// (RuleID, From, To).
type Report struct {
	Since     string `json:"since"`
	RangeSpec string `json:"range_spec"`
	RuleFile  string `json:"rule_file"`
	// RuleIDs are the rule ids actually evaluated at every commit
	// (everything except cross_service_direction, which needs the
	// fleet's typed cross-service edges — never available from a single
	// checkout's git history, so always reported as skipped, not
	// silently evaluated as "never violated").
	RuleIDs        []string `json:"rule_ids"`
	SkippedRuleIDs []string `json:"skipped_rule_ids,omitempty"`

	TotalCommits int `json:"total_commits"`
	// TotalViolations and NewViolations count only "error"-severity
	// violations — the ones that would actually fail `fathom rules
	// gen-test`'s generated Go test. A "warn"-severity rule's violations
	// still appear per-commit (ViolationRecord.Severity says which) but
	// are excluded from these headline numbers, matching layerrules.Check.
	TotalViolations int `json:"total_violations"`
	NewViolations   int `json:"new_violations"`

	ElapsedMS      int64   `json:"elapsed_ms"`
	PerCommitAvgMS float64 `json:"per_commit_avg_ms"`

	Commits []CommitResult `json:"commits"`
}

// violationKey identifies one violation across commits, independent of its
// prose Message (which can carry commit-independent boilerplate but never
// changes what the tuple means).
type violationKey struct{ RuleID, From, To string }

// violationSet is one commit's violations, keyed for set membership tests
// against a parent commit's set.
type violationSet map[violationKey]guardspec.Violation

// Replay evaluates RuleFile's layer_rules against every commit in the
// range Since resolves to (oldest → newest), classifying each violation
// as new or inherited against the commit's first parent. See the package
// doc comment for why go/parser (not go/packages) builds the import graph.
func Replay(ctx context.Context, opts Options) (Report, error) {
	start := time.Now()

	ruleSet, err := guardspec.LoadLayerRules(opts.RuleFile)
	if err != nil {
		return Report{}, fmt.Errorf("rulereplay: %w", err)
	}
	ruleIDs, skippedIDs, evalRules := splitRules(ruleSet.Rules)

	rangeSpec := ResolveSince(opts.Since)
	commits, err := listCommits(ctx, opts.RepoRoot, opts.Since)
	if err != nil {
		return Report{}, fmt.Errorf("rulereplay: %w", err)
	}

	cache := map[string]cachedCommit{}
	report := Report{
		Since:          opts.Since,
		RangeSpec:      rangeSpec,
		RuleFile:       displayRuleFile(opts.RepoRoot, opts.RuleFile),
		RuleIDs:        ruleIDs,
		SkippedRuleIDs: skippedIDs,
		TotalCommits:   len(commits),
	}

	get := func(sha string) (cachedCommit, error) {
		if c, ok := cache[sha]; ok {
			return c, nil
		}
		vs, warnings, err := computeViolations(ctx, opts.RepoRoot, sha, evalRules)
		if err != nil {
			return cachedCommit{}, err
		}
		c := cachedCommit{set: vs, warnings: warnings}
		cache[sha] = c
		return c, nil
	}

	for _, c := range commits {
		cur, err := get(c.SHA)
		if err != nil {
			return Report{}, fmt.Errorf("rulereplay: commit %s: %w", c.SHA, err)
		}

		var baseline violationSet
		if len(c.Parents) > 0 {
			parent, err := get(c.Parents[0])
			if err != nil {
				return Report{}, fmt.Errorf("rulereplay: parent %s of %s: %w", c.Parents[0], c.SHA, err)
			}
			baseline = parent.set
		}

		cr := CommitResult{SHA: c.SHA, Date: c.Date, Subject: c.Subject, Warnings: cur.warnings}
		for _, key := range sortedKeys(cur.set) {
			v := cur.set[key]
			_, inherited := baseline[key]
			cr.Violations = append(cr.Violations, ViolationRecord{
				RuleID: v.RuleID, Kind: string(v.Kind), Severity: string(v.Severity),
				From: v.From, To: v.To, Message: v.Message, New: !inherited,
			})
			if v.Severity == guardspec.SeverityError {
				report.TotalViolations++
				if !inherited {
					report.NewViolations++
				}
			}
		}
		report.Commits = append(report.Commits, cr)
	}

	report.ElapsedMS = time.Since(start).Milliseconds()
	if report.TotalCommits > 0 {
		report.PerCommitAvgMS = float64(report.ElapsedMS) / float64(report.TotalCommits)
	}
	return report, nil
}

type cachedCommit struct {
	set      violationSet
	warnings []string
}

// computeViolations checks out sha into a throwaway temp dir (never the
// caller's working tree), builds its import graph via go/parser, and
// evaluates every non-cross-service rule against it.
func computeViolations(ctx context.Context, repoRoot, sha string, rules []guardspec.LayerRule) (violationSet, []string, error) {
	dir, cleanup, err := archiveCommit(ctx, repoRoot, sha)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()

	idx, _, warnings, err := BuildImportGraphFromDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", sha, err)
	}

	vs := make(violationSet)
	for _, r := range rules {
		for _, v := range guardspec.EvaluateLayerRule(r, idx) {
			vs[violationKey{v.RuleID, v.From, v.To}] = v
		}
	}
	return vs, warnings, nil
}

// splitRules separates ruleSet into the ids evaluated at every commit and
// the ids skipped (cross_service_direction — a live-obligations-only
// check needing the fleet's typed edges, never derivable from one repo's
// git history), plus the rule values actually passed to EvaluateLayerRule.
func splitRules(rules []guardspec.LayerRule) (ids, skipped []string, evalRules []guardspec.LayerRule) {
	for _, r := range rules {
		if r.Kind == guardspec.CrossServiceDirection || r.Kind == guardspec.CallDirection {
			skipped = append(skipped, r.ID)
			continue
		}
		ids = append(ids, r.ID)
		evalRules = append(evalRules, r)
	}
	return
}

// displayRuleFile renders ruleFile relative to repoRoot when it lives
// under it (the common case — a repo's own .fathom/guard.cue) so a
// committed report never bakes in one machine's absolute path; falls back
// to ruleFile unchanged when it lives outside repoRoot or the relative
// path can't be computed.
func displayRuleFile(repoRoot, ruleFile string) string {
	rel, err := filepath.Rel(repoRoot, ruleFile)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ruleFile
	}
	return filepath.ToSlash(rel)
}

// sortedKeys returns vs's keys ordered by (RuleID, From, To) — the report's
// determinism contract.
func sortedKeys(vs violationSet) []violationKey {
	out := make([]violationKey, 0, len(vs))
	for k := range vs {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RuleID != out[j].RuleID {
			return out[i].RuleID < out[j].RuleID
		}
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}
