package conserve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func fileReader(m map[string]string) func(path string) ([]byte, bool, error) {
	return func(path string) ([]byte, bool, error) {
		c, ok := m[path]
		if !ok {
			return nil, false, nil
		}
		return []byte(c), true, nil
	}
}

// TestSurfacePlane_RemovedKeyIsLossesUnlessDeclared exercises the
// registered "surface" plane end to end: an api: removed between base and
// head is an undeclared loss; the same removal with a matching `retires:`
// declared at base is excused (conserved).
func TestSurfacePlane_RemovedKeyIsLossesUnlessDeclared(t *testing.T) {
	base := map[string]string{"widgets/w.go": "package widgets\n\nfunc Old() {}\n"}
	head := map[string]string{"widgets/w.go": "package widgets\n\nfunc New() {}\n"}

	planes := []Plane{surfacePlane{}}

	in := Input{
		Service: "svc", Base: "base-sha", Head: HeadWorktree,
		ChangedFiles: []string{"widgets/w.go"},
		BaseFile:     fileReader(base), HeadFile: fileReader(head),
	}
	rep := Run(context.Background(), in, planes)
	pr := rep.Planes[0]
	if pr.Verdict != Losses {
		t.Fatalf("verdict = %q, want losses; findings=%+v", pr.Verdict, pr.Findings)
	}
	var sawRemoved bool
	for _, f := range pr.Findings {
		if f.Kind == "removed" && f.Key == "api:widgets.Old" {
			sawRemoved = true
			if f.Declared != DeclaredNone {
				t.Errorf("Declared = %q, want none", f.Declared)
			}
		}
	}
	if !sawRemoved {
		t.Fatalf("expected a removed api:widgets.Old finding, got %+v", pr.Findings)
	}

	// Now with a prior retires: declaration excusing it.
	inDeclared := in
	inDeclared.Declarations = NewDeclarations(
		[]verify.Plan{{Tasks: []verify.Task{{ID: "R-1", Retires: []string{"api:widgets.Old"}}}}},
		nil,
	)
	rep2 := Run(context.Background(), inDeclared, planes)
	if rep2.Planes[0].Verdict != Conserved {
		t.Fatalf("verdict with prior retires: = %q, want conserved; findings=%+v", rep2.Planes[0].Verdict, rep2.Planes[0].Findings)
	}
}

// TestSurfacePlane_ChangedKeyReportsBothCitations pins that a "changed"
// finding carries both Base and Head citations and a before->after Detail.
func TestSurfacePlane_ChangedKeyReportsBothCitations(t *testing.T) {
	base := map[string]string{"widgets/w.go": "package widgets\n\nfunc Do(id string) {}\n"}
	head := map[string]string{"widgets/w.go": "package widgets\n\nfunc Do(id string) error { return nil }\n"}

	in := Input{
		Service: "svc", Base: "base-sha", Head: HeadWorktree,
		ChangedFiles: []string{"widgets/w.go"},
		BaseFile:     fileReader(base), HeadFile: fileReader(head),
	}
	rep := Run(context.Background(), in, []Plane{surfacePlane{}})
	var found bool
	for _, f := range rep.Planes[0].Findings {
		if f.Key == "api:widgets.Do" && f.Kind == "changed" {
			found = true
			if f.Base == "" || f.Head == "" {
				t.Errorf("changed finding missing a citation: %+v", f)
			}
		}
	}
	if !found {
		t.Fatalf("expected a changed api:widgets.Do finding, got %+v", rep.Planes[0].Findings)
	}
}

// TestSurfacePlane_ApidiffCompatibleEditsAreNotLosses runs the plane end
// to end over the two CV-M4 disagreement classes that manifest as false
// POSITIVES (ADR-044 amendment 2026-09-24): an `interface{}`/`any`
// respelling and an additive struct field must both come back conserved,
// not losses, exactly as apidiff itself classifies them
// (docs/bench/readings/cv-m4-apidiff-calibration-2026-09-24.md classes A
// and B).
func TestSurfacePlane_ApidiffCompatibleEditsAreNotLosses(t *testing.T) {
	base := map[string]string{"widgets/w.go": `package widgets

type Config struct {
	Server string
}

func Do(data interface{}) error { return nil }
`}
	head := map[string]string{"widgets/w.go": `package widgets

type Config struct {
	Server    string
	Namespace string
}

func Do(data any) error { return nil }
`}

	in := Input{
		Service: "svc", Base: "base-sha", Head: HeadWorktree,
		ChangedFiles: []string{"widgets/w.go"},
		BaseFile:     fileReader(base), HeadFile: fileReader(head),
	}
	rep := Run(context.Background(), in, []Plane{surfacePlane{}})
	pr := rep.Planes[0]
	if pr.Verdict != Conserved {
		t.Fatalf("verdict = %q, want conserved; findings=%+v", pr.Verdict, pr.Findings)
	}
	if len(pr.Findings) != 0 {
		t.Fatalf("findings = %+v, want none (an additive field and an any/interface{} respelling are both apidiff-compatible)", pr.Findings)
	}
}

// TestSurfacePlane_TypeAliasIntroductionIsALoss runs the plane end to end
// over CV-M4's one false NEGATIVE: turning a defined type into an alias
// (`type X T` -> `type X = T`) is apidiff's own largest incompatible
// change in the calibration sample and must come back a loss here too.
func TestSurfacePlane_TypeAliasIntroductionIsALoss(t *testing.T) {
	base := map[string]string{"widgets/w.go": `package widgets

type Handler func(name string) error
`}
	head := map[string]string{"widgets/w.go": `package widgets

type Handler = func(name string) error
`}

	in := Input{
		Service: "svc", Base: "base-sha", Head: HeadWorktree,
		ChangedFiles: []string{"widgets/w.go"},
		BaseFile:     fileReader(base), HeadFile: fileReader(head),
	}
	rep := Run(context.Background(), in, []Plane{surfacePlane{}})
	pr := rep.Planes[0]
	if pr.Verdict != Losses {
		t.Fatalf("verdict = %q, want losses; findings=%+v", pr.Verdict, pr.Findings)
	}
	var saw bool
	for _, f := range pr.Findings {
		if f.Kind == "changed" && f.Key == "api:widgets.Handler" {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("expected a changed api:widgets.Handler finding, got %+v", pr.Findings)
	}
}

// TestSurfacePlane_MissingFileReaders reports unproven (I44).
func TestSurfacePlane_MissingFileReaders(t *testing.T) {
	rep := Run(context.Background(), Input{Service: "svc"}, []Plane{surfacePlane{}})
	if rep.Planes[0].Verdict != Unproven {
		t.Fatalf("verdict = %q, want unproven", rep.Planes[0].Verdict)
	}
	if len(rep.Planes[0].Missing) == 0 {
		t.Error("expected a missing-input reason")
	}
}

// TestConsumerLosses_RemovedRouteNamesCaller pins CV-20: a removed route
// still called by another known service's client code produces a
// consumer_loss finding naming that service and its call site.
func TestConsumerLosses_RemovedRouteNamesCaller(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	defer st.Close()

	consumerRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(consumerRoot, "client.go"), `package client

import "net/http"

func fetchWidget(id string) {
	http.Get("https://widgets.example.com/v1/widgets/" + id)
}
`)
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "consumer", Path: consumerRoot}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	// ListServiceIDs derives its set from the observations table — one row
	// is enough to make "consumer" a known service for ConsumerLosses'
	// fleet walk.
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: "consumer", SourceRef: "client.go", SourceKind: core.KindGoFunc,
		EvidenceTier: core.TierVerified, Symbol: "fetchWidget", Package: "client",
		Claim: "func fetchWidget(id string)", IngestedAt: time.Now(),
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}

	in := Input{
		Service: "widgets", Store: st,
		Declarations: NewDeclarations(nil, nil),
	}

	findings := ConsumerLosses(ctx, in, "route:GET /v1/widgets/{}")
	if len(findings) != 1 {
		t.Fatalf("ConsumerLosses = %+v, want exactly one finding", findings)
	}
	f := findings[0]
	if f.Kind != "consumer_loss" || f.Key != "route:GET /v1/widgets/{}" {
		t.Errorf("finding = %+v", f)
	}
	if f.Detail == "" || f.Head == "" {
		t.Errorf("finding missing detail/citation: %+v", f)
	}
	if !strings.Contains(f.Detail, "consumer") {
		t.Errorf("finding detail %q does not name the calling service", f.Detail)
	}
}

// TestConsumerLosses_NonRouteKeyIsNil pins the "only route: keys" scope.
func TestConsumerLosses_NonRouteKeyIsNil(t *testing.T) {
	st := store.NewMemoryStore()
	defer st.Close()
	in := Input{Service: "svc", Store: st}
	if got := ConsumerLosses(context.Background(), in, "api:foo.Bar"); got != nil {
		t.Errorf("ConsumerLosses(api: key) = %+v, want nil", got)
	}
}

// TestConsumerLosses_NilStoreIsNil pins the nil-Store no-op.
func TestConsumerLosses_NilStoreIsNil(t *testing.T) {
	got := ConsumerLosses(context.Background(), Input{Service: "svc"}, "route:GET /x")
	if got != nil {
		t.Errorf("ConsumerLosses(nil store) = %+v, want nil", got)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
