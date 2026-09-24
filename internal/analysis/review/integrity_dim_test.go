package review

import "testing"

// TestReview_IntegrityDimension is CV-12's own anchor: without a base ref
// the "integrity" dimension says so (design §1.6's exact wording); with
// one, every caller-supplied IntegrityFinding renders as a Finding, sorted
// and severity-classified.
func TestReview_IntegrityDimension(t *testing.T) {
	t.Run("no base ref", func(t *testing.T) {
		d := DimIntegrity(Input{})
		if d.Checked {
			t.Fatalf("Checked = true with no Base, want false")
		}
		if d.Reason != "needs a base ref" {
			t.Errorf("Reason = %q, want %q", d.Reason, "needs a base ref")
		}
		if len(d.Findings) != 0 || d.Count != 0 {
			t.Errorf("got findings on an unchecked dimension: %+v", d)
		}
	})

	t.Run("with base ref", func(t *testing.T) {
		in := Input{
			Base: "deadbeef",
			IntegrityFindings: []IntegrityFinding{
				{Kind: "deleted", Test: "TestGuarded", File: "pkg/x_test.go", Line: 5, Detail: "gone at head"},
				{Kind: "edited", Test: "TestOther", File: "pkg/x_test.go", Line: 20, Detail: "body changed"},
			},
		}
		d := DimIntegrity(in)
		if !d.Checked {
			t.Fatalf("Checked = false with a Base set, want true (reason: %s)", d.Reason)
		}
		if d.Count != 2 || len(d.Findings) != 2 {
			t.Fatalf("got %d finding(s), want 2: %+v", len(d.Findings), d.Findings)
		}
		// Sorted by (File, Kind, Message); both share a file, so Kind
		// decides: "deleted" < "edited".
		if d.Findings[0].Kind != "deleted" || d.Findings[0].Symbol != "TestGuarded" {
			t.Errorf("Findings[0] = %+v, want kind=deleted symbol=TestGuarded", d.Findings[0])
		}
		if d.Findings[0].Severity != "error" {
			t.Errorf("deleted severity = %q, want error", d.Findings[0].Severity)
		}
		if d.Findings[1].Kind != "edited" || d.Findings[1].Severity != "warn" {
			t.Errorf("Findings[1] = %+v, want kind=edited severity=warn", d.Findings[1])
		}
		if d.Findings[0].Citation == "" {
			t.Errorf("Citation is empty, want a file:line")
		}
	})

	t.Run("wired into Run", func(t *testing.T) {
		v := Run(Input{Service: "svc", Base: "deadbeef", IntegrityFindings: []IntegrityFinding{
			{Kind: "weakened", Test: "TestX", File: "a_test.go", Line: 1, Detail: "d"},
		}})
		var found *Dimension
		for i := range v.Dimensions {
			if v.Dimensions[i].Name == dimIntegrityName {
				found = &v.Dimensions[i]
			}
		}
		if found == nil {
			t.Fatalf("Run's Verdict has no %q dimension: %+v", dimIntegrityName, v.Dimensions)
		}
		if !found.Checked || found.Count != 1 {
			t.Errorf("integrity dimension in Run() = %+v, want checked with 1 finding", found)
		}
	})
}
