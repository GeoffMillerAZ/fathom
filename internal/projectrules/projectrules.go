// Package projectrules loads a service's project-level architecture-
// conformance rule pack — .fathom/rules/archconformance.cue under that
// service's corpus root — CUE-validates it against
// schema/rules/archconformance/schema.cue (embedded byte-identical below;
// see internal/config for the drift-gate precedent this follows), and
// returns typed internal/analysis/archconformance.Rule values plus an
// explicit source attribution.
//
// C4-2 (docs/plans/review-verb.md): the arch_conformance MCP tool (C4-3)
// consumes this package when a caller passes no inline rules, and reports
// which source it used. Two distinct "nothing to check" outcomes must never
// collapse into the same shape (C4-5's honesty audit):
//
//   - No rule file exists: not an error — Load returns Result{Found: false,
//     Source: SourceNone}. The caller decides whether that should read as
//     checked:false in its own response; this package's job is honest
//     attribution, not policy.
//   - A rule file exists but is malformed (bad CUE syntax, fails #RuleSet
//     validation, or an unknown field on a closed struct): Load returns a
//     loud error naming the offending file. Never a silently empty ruleset —
//     a typo in a rule pack must not quietly disable the check it was meant
//     to enforce.
package projectrules

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// archconformanceSchema is byte-identical to
// schema/rules/archconformance/schema.cue — kept in sync by
// TestArchconformanceSchemaMatchesSource (projectrules_test.go).
//
//go:embed archconformance_schema.cue
var archconformanceSchema []byte

// RelPath is the fixed, repo-relative location of a service's own
// architecture-conformance rule pack.
const RelPath = projectdir.Dir + "/" + string(projectdir.ArchConformance)

// ErrRelativeCorpusRoot is returned by Load for a non-absolute corpusRoot.
// A relative root would resolve against the ambient process CWD — for the
// serve-http daemon that means loading a FOREIGN repo's .fathom/rules/ pack
// and attributing it to the requested service (the pre-O10 relative-
// ServiceMeta-path class, docs/dogfood-observations.md O10). Load refuses
// instead of resolving; arch_conformance maps this error to an honest
// checked:false response rather than a foreign-rules "project" answer.
var ErrRelativeCorpusRoot = errors.New("projectrules: corpus root is not absolute")

// Source names where a Result's rules came from — the same vocabulary
// arch_conformance's rule_source field uses (C4-3), plus the loader-internal
// SourceNone case a caller maps onto its own "none" state.
const (
	// SourceProject means a valid .fathom/rules/archconformance.cue was
	// found and loaded (even if its rules list is empty — see Result.Found).
	SourceProject = "project"
	// SourceNone means no rule file exists at RelPath under the corpus
	// root. Not an error.
	SourceNone = "none"
)

// Result is the outcome of attempting to load a service's project-level
// architecture-conformance rules.
type Result struct {
	// Rules are the decoded rules, in file order. Empty (never nil-vs-empty
	// significant) when Found is false, or when a found file declares no
	// rules.
	Rules []archconformance.Rule
	// DenyByDefault is the rule pack's own deny_by_default setting
	// (defaults false per the CUE schema when the field is omitted).
	DenyByDefault bool
	// Source is SourceProject or SourceNone.
	Source string
	// Path is the rule-pack path that was checked (whether or not a file
	// was actually found there). It is always absolute: Load refuses a
	// non-absolute corpusRoot (ErrRelativeCorpusRoot) before Path is ever
	// derived, so a Result never carries a CWD-dependent path.
	Path string
	// Found reports whether a rule file existed at Path.
	Found bool
}

// Load reads and CUE-validates .fathom/rules/archconformance.cue under
// corpusRoot.
//
// corpusRoot must be absolute: a relative root is refused with
// ErrRelativeCorpusRoot (wrapped, errors.Is-able) instead of being resolved
// against the process CWD — resolving would let a stale/pre-O10 relative
// ServiceMeta path silently load whatever repo the daemon happens to be
// running in.
//
// A missing file is not an error: the returned Result has Found=false and
// Source=SourceNone, explicitly attributed rather than treated as a
// validated empty ruleset. A file that exists but fails to parse or does
// not satisfy #RuleSet (including an unrecognized field — #Rule is closed)
// returns a non-nil error naming corpusRoot's rule-pack path; Result is the
// zero value in every error case and must not be used.
func Load(corpusRoot string) (Result, error) {
	res, err := loadArchconformance(corpusRoot)
	if err != nil {
		return Result{}, err
	}
	// FD-30: the call_direction layer_rules of .fathom/guard.cue join the
	// legacy pack, so review and arch_conformance read either form.
	guard := filepath.Join(corpusRoot, projectdir.Dir, string(projectdir.Guard))
	if _, statErr := os.Stat(guard); statErr != nil {
		return res, nil
	}
	set, err := guardspec.LoadLayerRules(guard)
	if err != nil {
		return Result{}, fmt.Errorf("projectrules: %w", err)
	}
	if layer := guardspec.CallDirectionRules(set); len(layer) > 0 {
		res.Rules = append(res.Rules, layer...)
		res.Found = true
		res.Source = SourceProject
	}
	return res, nil
}

// EquivalentLayerRules renders rules as the `layer_rules` entries that say
// the same thing as a rules/archconformance.cue pack: one call_direction
// entry per (to, allow), listing every from, in first-seen order. Doctor
// prints it once so a pack can be moved by copying.
func EquivalentLayerRules(rules []archconformance.Rule) []string {
	type key struct {
		to    string
		allow bool
	}
	var order []key
	froms := map[key][]string{}
	for _, r := range rules {
		k := key{r.ToPkg, r.Allow}
		if _, ok := froms[k]; !ok {
			order = append(order, k)
		}
		froms[k] = append(froms[k], r.FromPkg)
	}
	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, fmt.Sprintf("{kind: %q, from: %s, to: [%q], allow: %t}", "call_direction", quoteList(froms[k]), k.to, k.allow))
	}
	return out
}

func quoteList(items []string) string {
	q := make([]string, len(items))
	for i, it := range items {
		q[i] = fmt.Sprintf("%q", it)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

func loadArchconformance(corpusRoot string) (Result, error) {
	if !filepath.IsAbs(corpusRoot) {
		return Result{}, fmt.Errorf("%w: %q (re-ingest the service so ServiceMeta records an absolute corpus root)", ErrRelativeCorpusRoot, corpusRoot)
	}
	path := filepath.Join(corpusRoot, RelPath)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Source: SourceNone, Path: path, Found: false}, nil
		}
		return Result{}, fmt.Errorf("projectrules: read %s: %w", path, err)
	}

	cctx := cuecontext.New()

	schema := cctx.CompileBytes(archconformanceSchema, cue.Filename("schema/rules/archconformance/schema.cue"))
	if err := schema.Err(); err != nil {
		return Result{}, fmt.Errorf("projectrules: compile embedded schema: %w", err)
	}
	def := schema.LookupPath(cue.ParsePath("#RuleSet"))
	if err := def.Err(); err != nil {
		return Result{}, fmt.Errorf("projectrules: embedded schema has no #RuleSet definition: %w", err)
	}

	// A rule pack is plain data — no package clause, no "#RuleSet &"
	// wrapper (the schema/semantic layer.cue precedent): compiled
	// independently of the schema's own package, then unified.
	instance := cctx.CompileBytes(data, cue.Filename(path))
	if err := instance.Err(); err != nil {
		return Result{}, fmt.Errorf("projectrules: %s: %w", path, err)
	}

	unified := def.Unify(instance)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return Result{}, fmt.Errorf("projectrules: %s: does not satisfy #RuleSet: %w", path, err)
	}

	var rs cueRuleSet
	if err := unified.Decode(&rs); err != nil {
		return Result{}, fmt.Errorf("projectrules: %s: decode: %w", path, err)
	}

	rules := make([]archconformance.Rule, len(rs.Rules))
	for i, r := range rs.Rules {
		rules[i] = archconformance.Rule{FromPkg: r.FromPkg, ToPkg: r.ToPkg, Allow: r.Allow}
	}

	return Result{
		Rules:         rules,
		DenyByDefault: rs.DenyByDefault,
		Source:        SourceProject,
		Path:          path,
		Found:         true,
	}, nil
}

// cueRule/cueRuleSet are the CUE decode targets — json struct tags per the
// cue.Value.Decode convention (internal/semantic/manifest.go).
type cueRule struct {
	FromPkg string `json:"from_pkg"`
	ToPkg   string `json:"to_pkg"`
	Allow   bool   `json:"allow"`
}

type cueRuleSet struct {
	Rules         []cueRule `json:"rules"`
	DenyByDefault bool      `json:"deny_by_default"`
}
