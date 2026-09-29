package sourcedprose

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	prose "github.com/geoffmilleraz/agentic-kit/kits/governed-prose-kit"
	"github.com/geoffmilleraz/agentic-kit/kits/hitl"
)

func TestRunSourcedProseApprovalAndRecovery(t *testing.T) {
	t.Parallel()
	r, err := Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.LocalEffects != 1 || r.RemoteEffects != 1 || r.GeneratorCalls != 3 {
		t.Fatalf("counts: %+v", r)
	}
	if r.Export.Markdown != "Count increased from 10 to 12, a difference of 2\\.\n\nStatus: verified\\." || len(r.Export.SourceMap) != 2 {
		t.Fatalf("export: %+v", r.Export)
	}
	if err := prose.VerifyProseExport(r.Export); err != nil {
		t.Fatal(err)
	}
	for _, entry := range r.Export.SourceMap {
		if len(entry.Claims) != 1 || r.Export.Markdown[entry.Start:entry.End] != strings.ReplaceAll(entry.Claims[0].Assertion, ".", `\.`) {
			t.Fatal("rendered claim/source mapping lost")
		}
	}
	first := r.Export.SourceMap[0]
	if len(first.Facts) != 3 {
		t.Fatalf("comparison closure = %d facts", len(first.Facts))
	}
	versions := map[string]string{}
	for _, f := range first.Facts {
		versions[f.Span.ID] = f.Source.Version
	}
	if versions["current"] != "v2" || versions["historical"] != "v1" || versions["delta"] != "" {
		t.Fatalf("source versions: %v", versions)
	}
	if len(r.Export.SourceMap[1].Facts) != 1 || r.Export.SourceMap[1].Facts[0].Span.ID != "status" {
		t.Fatal("independent unit got blanket bibliography")
	}
	if !reflect.DeepEqual(r.Invalidated.InvalidFacts, []string{"current", "delta", "historical"}) || !reflect.DeepEqual(r.Invalidated.InvalidUnits, []string{"comparison-unit"}) {
		t.Fatalf("invalidation: %+v", r.Invalidated)
	}
	if r.Invalidated.Draft.Units[1].Text != statusText || r.Invalidated.Receipt.Integrity != "invalidated" {
		t.Fatal("unaffected wording or invalidation lost")
	}
	if r.Receipt.Actor != "editor" || r.Receipt.Binding.EvidenceDigest != r.Export.EvidenceDigest || r.Remote.Status != hitl.AttemptSucceeded || r.Remote.Sends != 1 {
		t.Fatal("approval/recovery binding lost")
	}
	want := []string{"unsupported-14", "local-replay", "stale-approval", "invalidated-export", "stale-source", "unknown-resend", "duplicate-prepare", "reconciled-resend"}
	if !reflect.DeepEqual(r.Denied, want) {
		t.Fatalf("denials=%v", r.Denied)
	}
	inspectable, err := json.MarshalIndent(r.Export, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Historical checked export and citation sidecar:\n%s", inspectable)
}

func TestRunDeterministicAndCanceled(t *testing.T) {
	t.Parallel()
	a, err := Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("synthetic replay drifted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := Run(ctx)
	if !errors.Is(err, context.Canceled) || r.LocalEffects != 0 || r.RemoteEffects != 0 {
		t.Fatal("cancellation did not fail before effects")
	}
}

func TestIndependentReviewRejectsFalseWordingWithValidLinks(t *testing.T) {
	for _, wording := range []string{"14", "Count increased from 10 to 14, a difference of 4.", "Count decreased from 12 to 10, a difference of 2."} {
		t.Run(wording, func(t *testing.T) {
			f, err := newFixture()
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.WithValue(context.Background(), actorKey{}, "editor")
			s, err := f.store.View(ctx)
			if err != nil {
				t.Fatal(err)
			}
			d := draft(false) // Keep the valid delta→12/10 support graph.
			d.Claims[0].Assertion = wording
			d.Units[0].Text = wording
			d.Units[0].Claims[0].End = len(wording)
			p, _, err := f.pipeline(d)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Generate(ctx, request(s)); !errors.Is(err, prose.ErrUnsupported) {
				t.Fatalf("false wording with valid source links passed: %v", err)
			}
		})
	}
}

func TestUnresolvedEvidenceCannotReachPublication(t *testing.T) {
	for _, kind := range []string{"source", "claim-edge"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), actorKey{}, "editor")
			f, err := newFixture()
			if err != nil {
				t.Fatal(err)
			}
			s, err := f.store.View(ctx)
			if err != nil {
				t.Fatal(err)
			}
			before := digest(s)
			req, d := request(s), draft(false)
			if kind == "source" {
				req.Facts[0].Source.Version = "missing"
				req.Facts[0].Span.RecordID = "missing"
			} else {
				d.Claims[0].FactIDs = []string{"missing"}
			}
			p, mock, err := f.pipeline(d)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Generate(ctx, req); err == nil {
				t.Fatal("unresolved evidence passed generation")
			}
			if kind == "source" && mock.CallCount() != 0 {
				t.Fatal("unresolved source reached generator")
			}
			after, err := f.store.View(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if before != digest(after) {
				t.Fatal("unresolved evidence mutated publication or approval state")
			}
		})
	}
}

func TestPreparedExportCannotBypassApprovalOrCorrection(t *testing.T) {
	ctx := context.WithValue(context.Background(), actorKey{}, "editor")
	f, err := newFixture()
	if err != nil {
		t.Fatal(err)
	}
	op, _, err := f.prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Decisions) != 0 || s.Target.Value.LocalEffects != 0 {
		t.Fatal("export authorizer consumed approval or applied")
	}
	c, err := f.committer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Commit(ctx, "fabricated", hitl.Binding{}, op); err == nil {
		t.Fatal("export treated as authority")
	}
	changed := op
	changed.Export.Markdown = "14"
	if _, _, err := c.Propose(ctx, "publish", changed, time.Minute); err == nil {
		t.Fatal("changed bytes reached approver")
	}
	if err := f.correct(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Propose(ctx, "publish", op, time.Minute); err == nil {
		t.Fatal("old export proposed after correction")
	}
	s, err = f.store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Target.Value.LocalEffects != 0 || len(s.Decisions) != 0 {
		t.Fatal("denial mutated publication state")
	}
}

func TestSourceProviderUsesAuthenticatedTransactionalState(t *testing.T) {
	f, err := newFixture()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), actorKey{}, "editor")
	op, _, err := f.prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p := storedSources{f.store, func() time.Time { return f.epoch }}
	if _, err := p.Current(context.Background(), "editor", "count"); !errors.Is(err, prose.ErrEvidence) {
		t.Fatal("caller string authenticated itself")
	}
	before, err := p.Current(ctx, "editor", "count")
	if err != nil {
		t.Fatal(err)
	}
	expired := storedSources{f.store, func() time.Time { return f.epoch.Add(2 * time.Hour) }}
	if _, err := expired.Current(ctx, "editor", "count"); !errors.Is(err, prose.ErrEvidence) {
		t.Fatal("expired current pointer remained authorized")
	}
	if _, err := expired.Resolve(ctx, "editor", before); !errors.Is(err, prose.ErrEvidence) {
		t.Fatal("expired source remained authorized")
	}
	if err := f.correct(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := p.Current(ctx, "editor", "count")
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before.Version != "v2" || after.Version != "v3" || s.Target.EvidenceDigest == op.Export.EvidenceDigest || s.Target.Value.Prepared.ArtifactDigest != "" {
		t.Fatal("correction did not atomically update source/evidence/preparation")
	}
}

func TestCorrectionBetweenExportAndInstallRefusesWithoutEffects(t *testing.T) {
	ctx := context.WithValue(context.Background(), actorKey{}, "editor")
	f, err := newFixture()
	if err != nil {
		t.Fatal(err)
	}
	op, _, err := f.prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before, err := f.store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.correct(ctx); err != nil {
		t.Fatal(err)
	}
	corrected, err := f.store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.install(ctx, before, before.Target.Value.Artifact, op.Export); !errors.Is(err, hitl.ErrApproval) {
		t.Fatalf("stale checked-export installation = %v", err)
	}
	after, err := f.store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(corrected, after) {
		t.Fatal("stale installation mutated correction or approval state")
	}
}
