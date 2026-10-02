package extractor

// planrowsdialect.go — the fathom-plan-rows native dialect (LB-Q-fathom-plan-rows,
// wave fx item fmt): fathom's own plan grammar v2 (docs/plans/README.md) read as
// entities and references, so a plan's rows join the index they gate.
//
// The document engine has no Markdown decoder, so this is a native extractor
// that COMPOSES the Markdown extractor (a plan's headings stay the doc chunks
// they always were) and reads each row with verify.ParseMarkdownPlan, the
// gate's own parser: the rows the extractor indexes are the rows the gate
// proves. The calibration oracle (calibration/oracles/fathom-plan-rows) never
// uses that parser; it reads the grammar again with a parser of its own.
//
// Entities: a PlanRow per row that carries an author-owned id (a positional
// md-NNN id renumbers when a line is inserted, so it names nothing durable),
// named by its id, spanning the row's lines, signed by its title.
//
// References:
//   - needs: a row id written bare, or as <this plan>.md#ID, that another row
//     of the same plan declares is an in-file reference (the entity's Calls);
//   - needs: <other>.md#ID is a cross-file reference (strategy plan-row, edge
//     needs) that fusion's plan-row resolver joins to that row of that plan;
//   - symbol and reachable_from: cross-file references (strategy plan-symbol,
//     edges plan_symbol and plan_reachable_from) that fusion's plan-symbol
//     resolver joins to the code symbol the row anchors, in its package;
//   - package: a cross-file reference to the repository directory (strategy
//     repo-path, edge plan_package).

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

const (
	// KindPlanRow is a plan row's entity kind. It has no connector
	// classification use, so no constant in internal/core (the Cargo.toml and
	// go.work precedent).
	KindPlanRow core.SourceKind = "PlanRow"

	// PlanStrategyRow, PlanStrategySymbol are the cross-file strategies this
	// dialect introduces; internal/fusion/planrowxref.go resolves them.
	PlanStrategyRow    = "plan-row"
	PlanStrategySymbol = "plan-symbol"
)

// positionalPlanID is the id ParseMarkdownPlan assigns a row with no id:.
var positionalPlanID = regexp.MustCompile(`^md-[0-9]{3,}$`)

// planNeedID is one row id as the grammar writes it; planNeedRef is the
// cross-plan form <plan>.md#<id>.
var (
	planNeedID  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	planNeedRef = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._/-]*\.md)#([A-Za-z0-9][A-Za-z0-9._-]*)$`)
)

// PlanRowsExtractor implements Extractor and DocResulter for the plan files
// the fathom-plan-rows dialect owns (docs/plans/*.md).
type PlanRowsExtractor struct{ md *MarkdownExtractor }

var (
	_ Extractor   = (*PlanRowsExtractor)(nil)
	_ DocResulter = (*PlanRowsExtractor)(nil)
)

// NewPlanRowsExtractor returns the fathom-plan-rows dialect's extractor.
func NewPlanRowsExtractor() *PlanRowsExtractor { return &PlanRowsExtractor{md: NewMarkdownExtractor()} }

// Extract implements Extractor.
func (p *PlanRowsExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := p.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// LooksLikePlan is the dispatch's content sniff: a file with at least one
// checkbox row whose annotation block carries an author-owned id, the one
// signal a plan has that README.md and every other Markdown file in the
// directory lacks.
func LooksLikePlan(src []byte) bool {
	for _, t := range verify.ParseMarkdownPlan("", src).Tasks {
		if !positionalPlanID.MatchString(t.ID) {
			return true
		}
	}
	return false
}

// ExtractDoc implements DocResulter: the headings, then a node per row, and
// the references the rows carry.
func (p *PlanRowsExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	var res DocResult
	heads, err := p.md.Extract(service, relPath, src)
	if err != nil {
		return res, err
	}
	res.Nodes = append(res.Nodes, heads...)
	plan := verify.ParseMarkdownPlan(service, src)
	pkg := pkgFromPath(relPath)
	dir := path.Dir(relPath)
	declared := map[string]bool{}
	for _, t := range plan.Tasks {
		if !positionalPlanID.MatchString(t.ID) {
			declared[t.ID] = true
		}
	}
	for _, t := range plan.Tasks {
		if !declared[t.ID] {
			continue
		}
		n := core.SymbolNode{
			Service: service, Package: pkg, Name: t.ID, Kind: KindPlanRow, FilePath: relPath,
			Line: t.Line, EndLine: t.EndLine, Signature: capString(t.Title, 200), Tier: core.TierPreScan,
		}
		var calls []string
		seen := map[string]bool{}
		for _, need := range t.Needs {
			switch {
			case planNeedRef.MatchString(need):
				m := planNeedRef.FindStringSubmatch(need)
				target := path.Join(dir, m[1])
				if target == relPath {
					if declared[m[2]] && !seen[m[2]] && m[2] != t.ID {
						seen[m[2]] = true
						calls = append(calls, m[2])
					}
					continue
				}
				res.CrossFile = append(res.CrossFile, DocCrossFileRef{From: t.ID, Edge: "needs", Raw: need, Target: target, Strategy: PlanStrategyRow, Line: t.Line})
			case planNeedID.MatchString(need):
				if declared[need] && !seen[need] && need != t.ID {
					seen[need] = true
					calls = append(calls, need)
				}
			}
		}
		sort.Strings(calls)
		n.Calls = calls
		res.Nodes = append(res.Nodes, n)
		pkgDir := cleanRepoDir(t.Expect.Package)
		if sym := strings.TrimSpace(t.Expect.Symbol); sym != "" {
			res.CrossFile = append(res.CrossFile, DocCrossFileRef{From: t.ID, Edge: "plan_symbol", Raw: sym, Target: pkgDir, Strategy: PlanStrategySymbol, Line: t.Line})
		}
		if rf := strings.TrimSpace(t.Expect.ReachableFrom); rf != "" {
			res.CrossFile = append(res.CrossFile, DocCrossFileRef{From: t.ID, Edge: "plan_reachable_from", Raw: rf, Target: pkgDir, Strategy: PlanStrategySymbol, Line: t.Line})
		}
		if pkgDir != "" {
			res.CrossFile = append(res.CrossFile, DocCrossFileRef{From: t.ID, Edge: "plan_package", Raw: t.Expect.Package, Target: pkgDir, Strategy: "repo-path", Line: t.Line})
		}
	}
	sort.SliceStable(res.Nodes, func(i, j int) bool {
		a, b := res.Nodes[i], res.Nodes[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.SliceStable(res.CrossFile, func(i, j int) bool {
		a, b := res.CrossFile[i], res.CrossFile[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Edge != b.Edge {
			return a.Edge < b.Edge
		}
		return a.Raw < b.Raw
	})
	return res, nil
}

// cleanRepoDir is a package anchor as a repository-relative directory: ""
// for none, for an absolute path or for one that climbs out of the tree.
func cleanRepoDir(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, `\`, "/"))
	if p == "" || strings.HasPrefix(p, "/") {
		return ""
	}
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}
