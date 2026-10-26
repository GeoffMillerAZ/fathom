package proseheadless

import (
	"context"
	"errors"
	"fmt"

	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

// ErrBatch identifies an invalid variant batch request.
var ErrBatch = errors.New("proseheadless: invalid variant batch")

// GroupMode selects group readiness. Independent reports each variant on its own;
// AllRequired holds generation-cohort readiness unless all succeed. Neither mode
// grants publication authority or implements a group release transaction.
type GroupMode string

// Group modes.
const (
	Independent GroupMode = "independent"
	AllRequired GroupMode = "all-required"
)

// VariantSpec overrides presentation for one audience variant. Empty fields keep
// the base session's value and provenance.
type VariantSpec struct {
	ID             string `json:"id"`
	Audience       string `json:"audience"`
	Voice          string `json:"voice"`
	TechnicalLevel string `json:"technical_level"`
}

// SnapshotProbe returns a digest of the host's current common snapshot (source
// cut plus policy epoch). A change between calls means the batch is no longer on
// one snapshot.
type SnapshotProbe func(context.Context) (string, error)

// VariantMetrics are per-variant and never pooled: calls and tokens come from the
// recorded generation attempts of nodes generated in this variant. UsageState is
// unqualified because the shared completion type does not attest field presence.
type VariantMetrics struct {
	UsageState   string `json:"usage_state"`
	ModelCalls   int    `json:"model_calls"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	LeafTotal    int    `json:"leaf_total"`
	BytesTotal   int    `json:"bytes_total"`
}

// VariantResult statuses: "ready" (generated and checked), "failed" (reason in
// Err), "held-snapshot-changed" and "skipped" (not attempted after a snapshot change).
type VariantResult struct {
	ID         string                 `json:"id"`
	Status     string                 `json:"status"`
	Err        string                 `json:"err"`
	Version    *prose.DocumentVersion `json:"version"`
	Metrics    VariantMetrics         `json:"metrics"`
	GroupReady bool                   `json:"group_ready"`
}

// BatchResult reports a common-snapshot batch. GroupStatus is "ready", "partial"
// (independent mode, some ready), "failed" or "held" (all-required with any
// non-ready variant, or a changed snapshot).
type BatchResult struct {
	Snapshot    string          `json:"snapshot"`
	Mode        GroupMode       `json:"mode"`
	Variants    []VariantResult `json:"variants"`
	GroupStatus string          `json:"group_status"`
	Reasons     []string        `json:"reasons"`
}

func variantSession(base prose.DocumentSession, profile prose.DocumentTypeProfile, v VariantSpec) (prose.DocumentSession, error) {
	var supplied prose.PresentationDefaults
	pick := func(field, override, current string) string {
		if override != "" {
			return override
		}
		if base.DefaultsFrom[field] == "session" {
			return current
		}
		return ""
	}
	supplied.Audience = pick("audience", v.Audience, base.Presentation.Audience)
	supplied.Voice = pick("voice", v.Voice, base.Presentation.Voice)
	supplied.TechnicalLevel = pick("technical_level", v.TechnicalLevel, base.Presentation.TechnicalLevel)
	var goals []prose.Goal
	for _, g := range base.Goals {
		if g.Origin != prose.OriginProfile {
			goals = append(goals, g)
		}
	}
	return prose.NewDocumentSession(profile, prose.SessionInput{
		ID: base.ID + "/" + v.ID, Purpose: base.Purpose, Supplied: supplied, Goals: goals, ExpectedGoalIDs: base.ExpectedGoalIDs,
		ExpectedSectionIDs: base.ExpectedSectionIDs, Constraints: base.Constraints, Intent: "variant:" + v.ID,
	}, nil)
}

func metrics(v prose.DocumentVersion) VariantMetrics {
	m := VariantMetrics{UsageState: "unqualified"}
	for _, n := range v.Nodes {
		if n.Text == "" && n.Generation == nil {
			continue
		}
		m.LeafTotal++
		m.BytesTotal += len(n.Text)
		if n.Generation != nil && !n.Reused {
			m.ModelCalls++
			m.InputTokens += n.Generation.InputTokens
			m.OutputTokens += n.Generation.OutputTokens
		}
	}
	return m
}

// GenerateVariants generates audience variants from one common snapshot. The
// snapshot is probed before the batch, before each variant and after each variant;
// any change holds that variant and skips the rest, so no variant is compared or
// reported ready against a mixed snapshot. Publication is a separate authority.
// Variants are independent immutable documents
// with their own session revision and metrics. At most max (<=16) variants run.
func GenerateVariants(ctx context.Context, e *prose.DocumentEngine, base prose.DocumentTarget, variants []VariantSpec, mode GroupMode, probe SnapshotProbe, max int) (BatchResult, error) {
	out := BatchResult{Mode: mode, Variants: []VariantResult{}, Reasons: []string{}}
	if e == nil || probe == nil || (mode != Independent && mode != AllRequired) || len(variants) == 0 || max <= 0 || max > 16 || len(variants) > max {
		return out, fmt.Errorf("%w: engine, probe, mode and 1..max variants required", ErrBatch)
	}
	seen := map[string]bool{}
	for _, v := range variants {
		if v.ID == "" || seen[v.ID] {
			return out, fmt.Errorf("%w: variant IDs must be unique and non-empty", ErrBatch)
		}
		seen[v.ID] = true
	}
	snap, err := probe(ctx)
	if err != nil {
		return out, err
	}
	out.Snapshot = snap
	changed := false
	for _, v := range variants {
		res := VariantResult{ID: v.ID, Metrics: VariantMetrics{UsageState: "unqualified"}}
		if err := ctx.Err(); err != nil {
			return out, err
		}
		cur, err := probe(ctx)
		switch {
		case changed:
			res.Status = "skipped"
		case err != nil || cur != snap:
			changed, res.Status = true, "held-snapshot-changed"
			out.Reasons = append(out.Reasons, "snapshot changed before "+v.ID)
		default:
			ver, genErr := generateVariant(ctx, e, base, v)
			if cerr := ctx.Err(); cerr != nil {
				return out, cerr
			}
			after, perr := probe(ctx)
			switch {
			case genErr != nil:
				res.Status, res.Err = "failed", genErr.Error()
			case perr != nil || after != snap:
				changed, res.Status = true, "held-snapshot-changed"
				out.Reasons = append(out.Reasons, "snapshot changed during "+v.ID)
			default:
				res.Status, res.Version, res.Metrics = "ready", &ver, metrics(ver)
			}
		}
		out.Variants = append(out.Variants, res)
	}
	ready := 0
	for _, r := range out.Variants {
		if r.Status == "ready" {
			ready++
		}
	}
	switch {
	case ready == len(out.Variants):
		out.GroupStatus = "ready"
	case changed || mode == AllRequired:
		out.GroupStatus = "held"
	case ready > 0:
		out.GroupStatus = "partial"
	default:
		out.GroupStatus = "failed"
	}
	for i := range out.Variants {
		out.Variants[i].GroupReady = out.Variants[i].Status == "ready" && (mode == Independent && !changed || out.GroupStatus == "ready")
	}
	return out, nil
}

func generateVariant(ctx context.Context, e *prose.DocumentEngine, base prose.DocumentTarget, v VariantSpec) (prose.DocumentVersion, error) {
	s, err := variantSession(base.Session, base.Profile, v)
	if err != nil {
		return prose.DocumentVersion{}, err
	}
	t := base
	t.Session = s
	t.Spec.ID = base.Spec.ID + "/" + v.ID
	return e.Generate(ctx, t)
}

// ProjectionDelta reports a rebuild against its original projection: Numerator
// regenerated leaves that existed in the parent over Denominator parent leaves.
// Added, rebound, review-only and removed nodes are separate counts, not folded in.
type ProjectionDelta struct {
	Numerator   int      `json:"numerator"`
	Denominator int      `json:"denominator"`
	Added       []string `json:"added"`
	Rebound     []string `json:"rebound"`
	ReviewOnly  []string `json:"review_only"`
	Removed     []string `json:"removed"`
}

// Projection derives the original-projection delta from a rebuild plan.
func Projection(prev prose.DocumentVersion, res prose.RebuildResult) ProjectionDelta {
	old := map[string]bool{}
	d := ProjectionDelta{}
	for _, n := range prev.Nodes {
		if n.Text != "" || n.Generation != nil {
			old[n.ID] = true
			d.Denominator++
		}
	}
	for _, n := range res.Plan.Regenerate {
		if old[n.NodeID] {
			d.Numerator++
		} else {
			d.Added = append(d.Added, n.NodeID)
		}
	}
	for _, n := range res.Plan.Rebind {
		d.Rebound = append(d.Rebound, n.NodeID)
	}
	for _, n := range res.Plan.ReviewAgain {
		d.ReviewOnly = append(d.ReviewOnly, n.NodeID)
	}
	d.Removed = append(d.Removed, res.Plan.Removed...)
	return d
}
