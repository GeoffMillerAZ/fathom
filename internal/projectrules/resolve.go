package projectrules

import (
	"errors"
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
)

// SourceInline names arch_conformance's rule_source field when the caller
// passed inline rules (as opposed to SourceProject / SourceNone, both
// defined alongside Load). Kept here so every caller of ResolveRules shares
// the exact same three-value vocabulary instead of redeclaring "inline"
// as a local string constant.
const SourceInline = "inline"

// ResolveRules decides which architecture-conformance rules a caller should
// check: inline rules win when non-empty; otherwise corpusRoot's project
// rule pack (Load) is consulted. It returns the same three-way honesty
// split arch_conformance's rule_source/checked/reason contract needs
// (C4-2/C4-3, docs/plans/review-verb.md), factored out so a second composed
// caller (review_diff, C1-1) gets byte-identical behavior instead of a
// second hand-rolled copy that could drift:
//
//   - source is SourceInline, SourceProject, or SourceNone.
//   - checked is false only when neither an inline rule nor a usable
//     project pack exists — reason then explains why. A found-but-empty
//     rule pack and an absent rule pack both read as SourceNone/checked
//     false, worded differently in reason.
//   - A malformed rule pack, or any Load error OTHER than
//     ErrRelativeCorpusRoot, is returned via err and must be surfaced
//     loudly by the caller (an RPC/CLI error) — never silently downgraded
//     to checked:false. ErrRelativeCorpusRoot itself is folded into the
//     honest checked:false/SourceNone case (err is nil): a pre-O10 relative
//     ServiceMeta path is a data-staleness condition the caller can fix by
//     re-ingesting, not a malformed request.
func ResolveRules(corpusRoot string, inline []archconformance.Rule, denyByDefault bool) (rules []archconformance.Rule, source string, effectiveDeny bool, checked bool, reason string, err error) {
	if len(inline) > 0 {
		return inline, SourceInline, denyByDefault, true, "", nil
	}
	if corpusRoot == "" {
		return nil, SourceNone, denyByDefault, false,
			"no rules configured: no inline rules were passed and the service's corpus root is unknown, so no .fathom/rules/archconformance.cue could be located", nil
	}

	result, loadErr := Load(corpusRoot)
	if errors.Is(loadErr, ErrRelativeCorpusRoot) {
		// A relative ServiceMeta path (pre-O10 meta) would resolve against
		// the daemon's CWD — loading a foreign repo's rule pack and
		// attributing it to this service. Refuse to check rather than
		// answer from the wrong repo's rules.
		return nil, SourceNone, denyByDefault, false,
			fmt.Sprintf("no rules configured: the service's corpus root %q is recorded relative (pre-O10 meta) and cannot be resolved safely — re-ingest the service so ServiceMeta records an absolute path", corpusRoot), nil
	}
	if loadErr != nil {
		return nil, "", false, false, "", loadErr
	}
	if !result.Found || len(result.Rules) == 0 {
		r := fmt.Sprintf("no rules configured: no inline rules were passed and no project rule pack exists at %s", result.Path)
		if result.Found {
			r = fmt.Sprintf("no rules configured: %s exists but declares zero rules", result.Path)
		}
		return nil, SourceNone, denyByDefault, false, r, nil
	}
	return result.Rules, SourceProject, denyByDefault || result.DenyByDefault, true, "", nil
}
