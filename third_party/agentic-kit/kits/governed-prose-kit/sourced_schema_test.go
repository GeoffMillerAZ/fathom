package governedprosekit

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
)

func TestSourcedMarshaledValuesMatchCUE(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE tool unavailable; task cue-vet has the same optional prerequisite")
	}
	s, req, draft := sourcedFixture(t, "12", "10")
	derived := req.Facts[0]
	derived.Span.ID, derived.Span.Value, derived.Span.Field = "delta", "2", "difference"
	derived.Source, derived.Span.Locator = SourceRef{}, evidencespan.Locator{}
	derived.Span.SourceID, derived.Span.RecordID, derived.Span.RecordHash = "", "", ""
	derived.Derivation = &Derivation{Version: "arithmetic/v1", Operation: "difference", Inputs: []string{"fact-a", "fact-b"}}
	req.Facts = append(req.Facts, derived)
	draft.Claims = append(draft.Claims, ProseClaim{ID: "delta", Assertion: "2", FactIDs: []string{"delta"}})
	draft.Units = append(draft.Units, ProseUnit{ID: "delta", Text: "2", Claims: []ClaimRange{{ClaimID: "delta", End: 1}}})
	p, _ := sourcedPipeline(t, s, ExactSupportReviewer{}, draft)
	artifact, err := p.CheckDraft(context.Background(), req, draft)
	if err != nil {
		t.Fatal(err)
	}
	assertSchema := func(name string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "actual.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, cue, "vet", "schema.cue", path, "-d", name).CombinedOutput(); err != nil {
			t.Fatalf("actual Go JSON violates %s: %v\n%s", name, err, out)
		}
	}
	assertSchema("#SourcedArtifact", artifact)
	for _, private := range []bool{false, true} {
		if private {
			ref := req.Facts[0].Source
			snapshot := s.items[ref]
			snapshot.Access = "restricted"
			s.items[ref] = snapshot
		}
		export, err := p.Export(context.Background(), artifact, "public", allowPublication)
		if err != nil {
			t.Fatal(err)
		}
		assertSchema("#ProseExport", export)
	}
}
