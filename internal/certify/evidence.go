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
