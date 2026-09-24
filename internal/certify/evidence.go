package certify

// evidence.go — ByTest (TC-9's own wiring): the shared resolution both
// cmd/fathom's cliEvidence and internal/mcp's verifyEvidence use to
// implement verify.CertificationEvidence identically, so `fathom verify`
// (CLI) and verify_plan (MCP) never drift on what a certified_by contract
// sees. Neither caller re-parses a claim's Meta blob itself.
import (
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// ByTest groups every certifications-layer claim in claims whose
// provenance test (Meta.Test) matches one of tests (bare name) into
// verify.Certification values — verify.CertificationEvidence.
// CertificationsByTest's own contract. A claim with no Meta, or Meta this
// package cannot parse, is silently excluded — the SAME "no certificate"
// outcome EvaluateCertifiedBy already reports honestly for an absent
// certificate, never a crash over a foreign or legacy claim row.
func ByTest(claims []core.SemanticClaim, tests []string) map[string][]verify.Certification {
	want := make(map[string]bool, len(tests))
	for _, t := range tests {
		want[t] = true
	}
	out := make(map[string][]verify.Certification)
	for _, c := range claims {
		if len(c.Meta) == 0 {
			continue
		}
		var m Meta
		if err := json.Unmarshal(c.Meta, &m); err != nil {
			continue
		}
		if !want[m.Test] {
			continue
		}
		out[m.Test] = append(out[m.Test], verify.Certification{
			Test: m.Test, Strength: m.Strength, State: m.State, Scope: append([]string(nil), c.Scope...),
		})
	}
	return out
}

// Record is one certifications-layer claim, flattened for a caller OUTSIDE
// this package that needs the compiled facts without depending on
// internal/certify's own types (TC-10's own wiring instruction, mirrored
// by internal/analysis/review.CertificationRecord: "review does not
// import internal/certify" — see engine.go's CertificationRecord doc
// comment) or on internal/certify's gap sourcing (TC-11's GapTarget).
// Subject is the target's own "<kind>:<id>" identity (design doc §3);
// every other field is the claim's own served shape.
type Record struct {
	Subject   string
	Test      string
	Suite     string
	Class     string
	Strength  string
	State     string
	RunDigest string
	Tier      string
	Rule      string
	Status    string
	Scope     []string
}

// Records flattens every certifications-layer claim in claims into a
// Record, parsing each claim's Meta blob (design doc §3's provenance
// block). A claim with no Meta, or Meta this package cannot parse, is
// silently excluded — the SAME "no certificate" outcome ByTest already
// reports honestly for a foreign or legacy claim row, never a crash.
func Records(claims []core.SemanticClaim) []Record {
	out := make([]Record, 0, len(claims))
	for _, c := range claims {
		if len(c.Meta) == 0 {
			continue
		}
		var m Meta
		if err := json.Unmarshal(c.Meta, &m); err != nil {
			continue
		}
		out = append(out, Record{
			Subject: c.Subject, Test: m.Test, Suite: m.Suite, Class: m.Class,
			Strength: m.Strength, State: m.State, RunDigest: m.RunDigest,
			Tier: string(c.Tier), Rule: c.Rule, Status: string(c.Status),
			Scope: append([]string(nil), c.Scope...),
		})
	}
	return out
}
