package external

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// prettySprint renders a deterministic snapshot of the given values for the
// dry-run store-state equality pin — both inputs are already returned in a
// sorted, deterministic order by their Store methods.
func prettySprint(vs ...any) string {
	return fmt.Sprintf("%+v", vs)
}

const (
	testService = "svc-findings"
	testSubject = "acme/widget/api"
)

func fixedNow() time.Time { return time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC) }

func newTestImporter(t *testing.T, st store.Store) Importer {
	t.Helper()
	return Importer{
		Store:   st,
		Now:     fixedNow,
		Catalog: semantic.DefaultProducers(),
		Subject: testSubject,
		FileContent: func(path string) ([]byte, bool) {
			return []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\n"), true
		},
		Fingerprint: func(path string) (string, bool) {
			return "fp-" + path, true
		},
	}
}

func parseOne(t *testing.T, eventJSON string) Projection {
	t.Helper()
	p, err := ParseProjection(wrapProjection(eventJSON))
	if err != nil {
		t.Fatalf("ParseProjection: %v\n%s", err, eventJSON)
	}
	return p
}

func TestImport_Happy(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	p := parseOne(t, validEventJSON("happy"))

	r, err := im.Import(context.Background(), testService, "", p, false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if r.Delivered != 1 || r.Duplicate != 0 || r.Stale != 0 || len(r.Rejected) != 0 {
		t.Fatalf("receipt = %+v", r)
	}

	claimID := ClaimFromEvent(testService, p.Events[0], core.TierPreScan).ClaimID
	claims, err := st.ClaimsForService(context.Background(), testService, "findings")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v %+v", err, claims)
	}
	if claims[0].ClaimID != claimID || claims[0].Status != core.ClaimFresh || claims[0].Tier != core.TierPreScan {
		t.Errorf("claim = %+v", claims[0])
	}
	if claims[0].PromptVersion != "external-import/v1" {
		t.Errorf("PromptVersion = %q", claims[0].PromptVersion)
	}

	ef, found, err := st.ExternalFinding(context.Background(), testService, claimID)
	if err != nil || !found {
		t.Fatalf("ExternalFinding: found=%v err=%v", found, err)
	}
	if ef.SourceVersion != 1 || ef.Producer != "reviewkit.system-review" {
		t.Errorf("ef = %+v", ef)
	}
}

func TestImport_Duplicate(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	p := parseOne(t, validEventJSON("dup"))
	ctx := context.Background()

	if _, err := im.Import(ctx, testService, "", p, false); err != nil {
		t.Fatalf("first import: %v", err)
	}
	r, err := im.Import(ctx, testService, "", p, false)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if r.Duplicate != 1 || r.Delivered != 0 {
		t.Errorf("receipt = %+v, want duplicate=1 delivered=0", r)
	}
}

func TestImport_OutOfOrder_Stale(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ctx := context.Background()

	e2 := strings.NewReplacer(`"sourceVersion": 1`, `"sourceVersion": 2`).Replace(validEventJSON("ooo"))
	p2 := parseOne(t, e2)
	if _, err := im.Import(ctx, testService, "", p2, false); err != nil {
		t.Fatalf("import v2: %v", err)
	}

	p1 := parseOne(t, validEventJSON("ooo")) // sourceVersion 1, same findingId/subject
	r, err := im.Import(ctx, testService, "", p1, false)
	if err != nil {
		t.Fatalf("import v1 (stale): %v", err)
	}
	if r.Stale != 1 || r.Delivered != 0 {
		t.Errorf("receipt = %+v, want stale=1", r)
	}
}

func TestImport_ConflictingReplay(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ctx := context.Background()

	p := parseOne(t, validEventJSON("conflict"))
	if _, err := im.Import(ctx, testService, "", p, false); err != nil {
		t.Fatalf("first import: %v", err)
	}

	// Same findingId + same sourceVersion, different payloadDigest.
	altered := validEventJSON("conflict")
	altered = strings.NewReplacer(hex64("conflict-payload"), hex64("conflict-payload-v2")).Replace(altered)
	p2 := parseOne(t, altered)

	r, err := im.Import(ctx, testService, "", p2, false)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if len(r.Rejected) != 1 || r.Rejected[0].Reason != "conflicting-replay" {
		t.Errorf("receipt.Rejected = %+v, want one conflicting-replay", r.Rejected)
	}
}

func TestImport_ClosureSupersedesFresh(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ctx := context.Background()

	p1 := parseOne(t, validEventJSON("closure"))
	if _, err := im.Import(ctx, testService, "", p1, false); err != nil {
		t.Fatalf("observation import: %v", err)
	}

	e2 := strings.NewReplacer(
		`"kind": "observation"`, `"kind": "closure"`,
		`"sourceVersion": 1`, `"sourceVersion": 2`,
	).Replace(validEventJSON("closure"))
	p2 := parseOne(t, e2)
	r, err := im.Import(ctx, testService, "", p2, false)
	if err != nil {
		t.Fatalf("closure import: %v", err)
	}
	if r.Delivered != 1 {
		t.Fatalf("receipt = %+v", r)
	}

	cID := ClaimFromEvent(testService, p2.Events[0], core.TierPreScan).ClaimID
	claims, _ := st.ClaimsForService(ctx, testService, "findings")
	var got core.SemanticClaim
	for _, c := range claims {
		if c.ClaimID == cID {
			got = c
		}
	}
	if got.Status != core.ClaimSuperseded {
		t.Errorf("status after closure = %q, want superseded", got.Status)
	}
	events, err := st.ClaimEvents(ctx, cID)
	if err != nil || len(events) != 1 {
		t.Fatalf("ClaimEvents: %v %+v", err, events)
	}
	if events[0].Verdict != "superseded" || events[0].ResolutionKind != core.ResolutionExternalClosure {
		t.Errorf("event = %+v", events[0])
	}
}

func TestImport_Revocation(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ctx := context.Background()

	p1 := parseOne(t, validEventJSON("revoke"))
	if _, err := im.Import(ctx, testService, "", p1, false); err != nil {
		t.Fatalf("observation import: %v", err)
	}

	e2 := strings.NewReplacer(
		`"kind": "observation"`, `"kind": "revocation"`,
		`"sourceVersion": 1`, `"sourceVersion": 2`,
	).Replace(validEventJSON("revoke"))
	p2 := parseOne(t, e2)
	if _, err := im.Import(ctx, testService, "", p2, false); err != nil {
		t.Fatalf("revocation import: %v", err)
	}

	cID := ClaimFromEvent(testService, p2.Events[0], core.TierPreScan).ClaimID
	claims, _ := st.ClaimsForService(ctx, testService, "findings")
	var got core.SemanticClaim
	for _, c := range claims {
		if c.ClaimID == cID {
			got = c
		}
	}
	if got.Status != core.ClaimSuperseded {
		t.Errorf("status after revocation = %q, want superseded", got.Status)
	}
	events, _ := st.ClaimEvents(ctx, cID)
	found := false
	for _, e := range events {
		if e.Verdict == "superseded" && e.ResolutionKind == core.ResolutionRevoked {
			found = true
		}
		if e.Verdict == "refuted" {
			t.Error("revocation must never write a refuted verdict")
		}
	}
	if !found {
		t.Errorf("no superseded/revoked event among %+v", events)
	}
}

func TestImport_Annotation_StatusPreserving(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ctx := context.Background()

	p1 := parseOne(t, validEventJSON("annot"))
	if _, err := im.Import(ctx, testService, "", p1, false); err != nil {
		t.Fatalf("observation import: %v", err)
	}
	cID := ClaimFromEvent(testService, p1.Events[0], core.TierPreScan).ClaimID

	e2 := strings.NewReplacer(`"kind": "observation"`, `"kind": "annotation"`, `"sourceVersion": 1`, `"sourceVersion": 2`).Replace(validEventJSON("annot"))
	e2 = strings.Replace(e2, `"sourceStatus": "open",`, `"sourceStatus": "open", "annotation": "disputed by the team",`, 1)
	p2 := parseOne(t, e2)
	r, err := im.Import(ctx, testService, "", p2, false)
	if err != nil {
		t.Fatalf("annotation import: %v", err)
	}
	if r.Delivered != 1 {
		t.Fatalf("receipt = %+v", r)
	}

	claims, _ := st.ClaimsForService(ctx, testService, "findings")
	for _, c := range claims {
		if c.ClaimID == cID && c.Status != core.ClaimFresh {
			t.Errorf("annotation changed status to %q, want unchanged (fresh)", c.Status)
		}
	}
	events, _ := st.ClaimEvents(ctx, cID)
	sawAnnotated := false
	for _, e := range events {
		if e.Verdict == "annotated" {
			sawAnnotated = true
			if e.Note != "disputed by the team" {
				t.Errorf("annotation note = %q", e.Note)
			}
		}
		if e.Verdict == "refuted" {
			t.Error("annotation must never write a refuted verdict")
		}
	}
	if !sawAnnotated {
		t.Error("no annotated event recorded")
	}
}

func TestImport_FathomOriginRefused(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	e := strings.NewReplacer(`"name": "reviewkit.system-review"`, `"name": "reviewkit.review-import"`).Replace(validEventJSON("circular"))
	p := parseOne(t, e)

	r, err := im.Import(context.Background(), testService, "", p, false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if r.Delivered != 0 || len(r.Rejected) != 1 || r.Rejected[0].Reason != "circular: fathom-origin producer" {
		t.Errorf("receipt = %+v", r)
	}
}

func TestImport_UnknownProducerRejected(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	e := strings.NewReplacer(`"name": "reviewkit.system-review"`, `"name": "nobody.review"`).Replace(validEventJSON("unknownprod"))
	p := parseOne(t, e)

	r, err := im.Import(context.Background(), testService, "", p, false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(r.Rejected) != 1 || r.Rejected[0].Reason != "unknown-producer" {
		t.Errorf("receipt = %+v", r)
	}
}

func TestImport_ProducerFlagMismatch(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	p := parseOne(t, validEventJSON("mismatch"))

	r, err := im.Import(context.Background(), testService, "human.review", p, false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(r.Rejected) != 1 || r.Rejected[0].Reason != "producer-mismatch" {
		t.Errorf("receipt = %+v", r)
	}
}

func TestImport_NoSubjectProvisioned_Refuses(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	im.Subject = ""
	p := parseOne(t, validEventJSON("nosubj"))

	_, err := im.Import(context.Background(), testService, "", p, false)
	if err == nil {
		t.Fatal("expected refusal for unprovisioned subject")
	}
}

func TestImport_SubjectMismatch_Refuses(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	im.Subject = "other/repo/component"
	p := parseOne(t, validEventJSON("subjmismatch"))

	before, _ := st.ClaimsForService(context.Background(), testService, "")
	_, err := im.Import(context.Background(), testService, "", p, false)
	if err == nil {
		t.Fatal("expected refusal for subject mismatch")
	}
	after, _ := st.ClaimsForService(context.Background(), testService, "")
	if len(before) != len(after) {
		t.Error("subject-mismatch refusal must write nothing")
	}
}

func TestImport_DryRun_StoreStateEquality(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ctx := context.Background()
	p := parseOne(t, validEventJSON("dryrun"))

	before, err := dumpStore(ctx, st, testService)
	if err != nil {
		t.Fatal(err)
	}

	r, err := im.Import(ctx, testService, "", p, true)
	if err != nil {
		t.Fatalf("dry-run import: %v", err)
	}
	if !r.DryRun || r.Delivered != 1 {
		t.Errorf("receipt = %+v, want DryRun=true Delivered=1", r)
	}

	after, err := dumpStore(ctx, st, testService)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("dry run wrote to the store:\nbefore=%s\nafter=%s", before, after)
	}
}

// dumpStore renders a comparable snapshot of every table the importer could
// touch, for the dry-run store-state equality pin.
func dumpStore(ctx context.Context, st store.Store, service string) (string, error) {
	claims, err := st.ClaimsForService(ctx, service, "")
	if err != nil {
		return "", err
	}
	ext, err := st.ExternalFindingsForService(ctx, service)
	if err != nil {
		return "", err
	}
	return prettySprint(claims, ext), nil
}

func TestImport_Isolation_ServiceScoped(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ctx := context.Background()
	p := parseOne(t, validEventJSON("iso"))

	if _, err := im.Import(ctx, "svc-A", "", p, false); err != nil {
		t.Fatalf("import: %v", err)
	}
	claimsB, err := st.ClaimsForService(ctx, "svc-B", "findings")
	if err != nil {
		t.Fatal(err)
	}
	if len(claimsB) != 0 {
		t.Errorf("service B sees claims imported for service A: %+v", claimsB)
	}
	extB, err := st.ExternalFindingsForService(ctx, "svc-B")
	if err != nil {
		t.Fatal(err)
	}
	if len(extB) != 0 {
		t.Errorf("service B sees external_findings rows imported for service A: %+v", extB)
	}
}

func TestImport_Disagreement_BothServedSideBySide(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ctx := context.Background()

	// Two producers, same rule/anchor, one fail one pass: distinct findingIds
	// (each producer emits its own finding), both served.
	failEvt := validEventJSON("disagree-fail")
	passEvt := strings.NewReplacer(
		`"name": "reviewkit.system-review"`, `"name": "reviewkit.security-review"`,
		`"outcome": "fail"`, `"outcome": "pass"`,
	).Replace(validEventJSON("disagree-pass"))

	p, err := ParseProjection(wrapProjection(failEvt, passEvt))
	if err != nil {
		t.Fatalf("ParseProjection: %v", err)
	}
	r, err := im.Import(ctx, testService, "", p, false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if r.Delivered != 2 {
		t.Fatalf("receipt = %+v, want Delivered=2", r)
	}
	claims, _ := st.ClaimsForService(ctx, testService, "findings")
	fresh, superseded := 0, 0
	for _, c := range claims {
		switch c.Status {
		case core.ClaimFresh:
			fresh++
		case core.ClaimSuperseded:
			superseded++
		}
	}
	if fresh != 1 || superseded != 1 {
		t.Errorf("claims = %+v, want one fresh (fail) and one superseded (pass)", claims)
	}
}

// TestImport_UnanchoredProcessSubject proves FL-5's other named case: an
// event with no anchor at all produces a claim with an EMPTY Scope (never a
// fabricated one), and its cursor row is recorded Unanchored — served as
// process context, never attached to a file.
func TestImport_UnanchoredProcessSubject(t *testing.T) {
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	ctx := context.Background()

	e := validEventJSON("unanchored-proc")
	i := strings.Index(e, `"anchor":`)
	j := strings.Index(e[i:], "},") + i + 2
	e = e[:i] + e[j:]
	p := parseOne(t, e)

	r, err := im.Import(ctx, testService, "", p, false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if r.Delivered != 1 || r.NoFingerprint != 0 {
		t.Fatalf("receipt = %+v, want Delivered=1 NoFingerprint=0 (nothing to fingerprint)", r)
	}

	claimID := ClaimFromEvent(testService, p.Events[0], core.TierPreScan).ClaimID
	claims, _ := st.ClaimsForService(ctx, testService, "findings")
	if len(claims) != 1 || claims[0].Scope != nil {
		t.Errorf("claims = %+v, want exactly one claim with a nil/empty Scope", claims)
	}
	ef, found, err := st.ExternalFinding(ctx, testService, claimID)
	if err != nil || !found {
		t.Fatalf("ExternalFinding: found=%v err=%v", found, err)
	}
	if !ef.Unanchored || ef.AnchorPath != "" {
		t.Errorf("ef = %+v, want Unanchored=true AnchorPath=\"\"", ef)
	}
}

func TestMissingRequiredField(t *testing.T) {
	e := strings.Replace(validEventJSON("req"), `"closure": "manual review",`, `"closure": "x",`, 1)
	p := parseOne(t, e)
	if got := missingRequiredField(p.Events[0], []string{"rule.closure"}); got != "" {
		t.Errorf("missingRequiredField = %q, want \"\" (closure present)", got)
	}
	if got := missingRequiredField(p.Events[0], []string{"unknown.field"}); got != "unknown.field" {
		t.Errorf("missingRequiredField for unrecognized name = %q, want fail-closed \"unknown.field\"", got)
	}
}
