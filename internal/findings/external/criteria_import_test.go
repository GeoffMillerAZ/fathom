package external

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Criteria imports are advisory child observations. The review kit emits
// unknown for a satisfied or unknown child, never a whole-rule pass. Retain
// that lifecycle, identity and evidence tier when accepting its producer IDs.
func TestImport_ReviewKitCriteriaProducers(t *testing.T) {
	for _, tc := range []struct {
		kind string
		cap  core.EvidenceTier
	}{
		{"deterministic", core.TierPreScan},
		{"human", core.TierAssumed},
		{"model", core.TierAssumed},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			ctx := context.Background()
			st := store.NewMemoryStore()
			im := newTestImporter(t, st)
			producer := "reviewkit.criteria-import." + tc.kind
			event := strings.NewReplacer(
				`"name": "reviewkit.system-review"`, `"name": "`+producer+`"`,
				`"version": "1.0.0"`, `"version": "report-v1"`,
				`"tier": "pre-scan"`, `"tier": "verified"`,
				`"coverage": "complete"`, `"coverage": "incomplete"`,
			).Replace(validEventJSON("criteria-" + tc.kind))
			event = strings.Replace(event, `"version": "report-v1"`, `"version": "report-v1", "kind": "`+tc.kind+`", "tierCap": "`+string(tc.cap)+`"`, 1)
			p := parseOne(t, event)
			r, err := im.Import(ctx, testService, producer, p, false)
			if err != nil || r.Delivered != 1 || len(r.Rejected) != 0 {
				t.Fatalf("Import = %+v, %v; want accepted criteria producer", r, err)
			}
			claims, err := st.ClaimsForService(ctx, testService, "findings")
			if err != nil || len(claims) != 1 {
				t.Fatalf("ClaimsForService = %+v, %v", claims, err)
			}
			claim := claims[0]
			if claim.Tier != tc.cap || claim.AssertedBy != core.AssertedBy("import", producer+"@report-v1") {
				t.Fatalf("claim tier/identity = %q/%q, want %q/%s@report-v1", claim.Tier, claim.AssertedBy, tc.cap, producer)
			}
			cursor, found, err := st.ExternalFinding(ctx, testService, claim.ClaimID)
			if err != nil || !found || cursor.Producer != producer || cursor.SourceRevision != "abc123" {
				t.Fatalf("criteria cursor lost provenance: %+v, found=%v err=%v", cursor, found, err)
			}
			// A later incomplete/unknown child observation cannot close the
			// failed finding, even if its text mentions raw=satisfied.
			unknown := strings.NewReplacer(
				`"outcome": "fail"`, `"outcome": "unknown"`,
				`"sourceVersion": 1`, `"sourceVersion": 2`,
				`"sourceStatus": "open"`, `"sourceStatus": "raw=satisfied; authority=advisory-child-only"`,
			).Replace(event)
			r, err = im.Import(ctx, testService, producer, parseOne(t, unknown), false)
			if err != nil || r.Delivered != 1 || len(r.Rejected) != 0 {
				t.Fatalf("Import unknown = %+v, %v", r, err)
			}
			claims, err = st.ClaimsForService(ctx, testService, "findings")
			if err != nil || len(claims) != 1 || claims[0].Status != core.ClaimFresh || !claims[0].ValidTo.IsZero() {
				t.Fatalf("unknown child closed finding: %+v, %v", claims, err)
			}
			r, err = im.Import(ctx, testService, producer, parseOne(t, unknown), false)
			if err != nil || r.Duplicate != 1 || r.Delivered != 0 {
				t.Fatalf("duplicate criteria receipt = %+v, %v", r, err)
			}
		})
	}
}

func TestImport_DeclaredProducerMetadataMustMatchCatalog(t *testing.T) {
	for _, tc := range []struct {
		name, metadata, reason string
	}{
		{"kind", `"kind": "deterministic", "tierCap": "assumed"`, "producer-kind-mismatch"},
		{"higher cap", `"kind": "model", "tierCap": "verified"`, "producer-tier-cap-mismatch"},
		{"lower cap", `"kind": "deterministic", "tierCap": "assumed"`, "producer-tier-cap-mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			producer := "reviewkit.criteria-import.model"
			if tc.name == "lower cap" {
				producer = "reviewkit.criteria-import.deterministic"
			}
			event := strings.NewReplacer(
				`"name": "reviewkit.system-review"`, `"name": "`+producer+`"`,
				`"version": "1.0.0"`, `"version": "report-v1", `+tc.metadata,
			).Replace(validEventJSON("criteria-metadata-" + tc.name))
			st := store.NewMemoryStore()
			im := newTestImporter(t, st)
			r, err := im.Import(context.Background(), testService, "", parseOne(t, event), false)
			if err != nil || r.Delivered != 0 || len(r.Rejected) != 1 || r.Rejected[0].Reason != tc.reason {
				t.Fatalf("Import conflicting metadata = %+v, %v; want %s", r, err, tc.reason)
			}
			claims, err := st.ClaimsForService(context.Background(), testService, "findings")
			if err != nil || len(claims) != 0 {
				t.Fatalf("metadata mismatch wrote claims: %+v, %v", claims, err)
			}
		})
	}
}

func TestImport_ReviewKitCriteriaProducerTyposRemainUnknown(t *testing.T) {
	for _, producer := range []string{"reviewkit.criteria-import", "reviewkit.criteria-import.verified", "reviewkit.criteria-import.fixture"} {
		t.Run(producer, func(t *testing.T) {
			st := store.NewMemoryStore()
			im := newTestImporter(t, st)
			event := strings.Replace(validEventJSON(producer), "reviewkit.system-review", producer, 1)
			r, err := im.Import(context.Background(), testService, "", parseOne(t, event), false)
			if err != nil || r.Delivered != 0 || len(r.Rejected) != 1 || r.Rejected[0].Reason != "unknown-producer" {
				t.Fatalf("Import unknown criteria producer = %+v, %v", r, err)
			}
			claims, err := st.ClaimsForService(context.Background(), testService, "findings")
			if err != nil || len(claims) != 0 {
				t.Fatalf("unknown producer wrote claims: %+v, %v", claims, err)
			}
		})
	}
}
