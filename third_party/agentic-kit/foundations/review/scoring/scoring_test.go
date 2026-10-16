package scoring_test

import (
	"testing"

	sc "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/scoring"
)

// ---- NormalizeResult --------------------------------------------------------

func TestNormalizeResult(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"PASS", "pass"},
		{"FAIL", "fail"},
		{"PARTIAL", "partial"},
		{"CONTRADICTS", "contradicts"},
		{"CONTRADICTION", "contradicts"},
		{"unknown", "partial"},
		{"", "partial"},
		{"  PASS  ", "pass"},
	}
	for _, tc := range cases {
		got := sc.NormalizeResult(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeResult(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---- NormalizeEvidenceTier --------------------------------------------------

func TestNormalizeEvidenceTier(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"code_verified", "verified"},
		{"code_reviewed", "verified"},
		{"code_seen", "verified"},
		{"code", "verified"},
		{"direct", "verified"},
		{"observed", "verified"},
		{"config_found", "verified"},
		{"config_verified", "verified"},
		{"sme-verified", "verified"},
		{"sme_verified", "verified"},
		{"red", "pre-scan"},
		{"doc_claimed", "doc-claimed"},
		{"documentation", "doc-claimed"},
		{"sme_stated", "sme-stated"},
		{"verified", "verified"},
		{"pre-scan", "pre-scan"},
		{"doc-claimed", "doc-claimed"},
		{"sme-stated", "sme-stated"},
		{"assumed", "assumed"},
		{"totally-unknown-xyz", "assumed"},
		{"", "assumed"},
	}
	for _, tc := range cases {
		got := sc.NormalizeEvidenceTier(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeEvidenceTier(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---- ApplyCeiling -----------------------------------------------------------

func TestApplyCeiling(t *testing.T) {
	cases := []struct {
		confidence int
		tier       string
		want       int
	}{
		{5, "verified", 5},
		{4, "pre-scan", 4},
		{5, "doc-claimed", 2},
		{3, "doc-claimed", 2},
		{1, "doc-claimed", 1},
		{5, "sme-stated", 2},
		{2, "assumed", 1},
		{1, "assumed", 1},
	}
	for _, tc := range cases {
		got := sc.ApplyCeiling(tc.confidence, tc.tier)
		if got != tc.want {
			t.Errorf("ApplyCeiling(%d, %q) = %d, want %d", tc.confidence, tc.tier, got, tc.want)
		}
	}
}

// ---- TierRank ---------------------------------------------------------------

func TestTierRank(t *testing.T) {
	// verified > pre-scan > doc-claimed > sme-stated > assumed
	if sc.TierRank("verified") <= sc.TierRank("pre-scan") {
		t.Error("verified should rank higher than pre-scan")
	}
	if sc.TierRank("pre-scan") <= sc.TierRank("doc-claimed") {
		t.Error("pre-scan should rank higher than doc-claimed")
	}
	if sc.TierRank("doc-claimed") <= sc.TierRank("sme-stated") {
		t.Error("doc-claimed should rank higher than sme-stated")
	}
	if sc.TierRank("sme-stated") <= sc.TierRank("assumed") {
		t.Error("sme-stated should rank higher than assumed")
	}
}

// ---- TierCeiling ------------------------------------------------------------

func TestTierCeiling(t *testing.T) {
	cases := []struct {
		tier string
		want int
	}{
		{"verified", 5},
		{"pre-scan", 4},
		{"pre_scan", 4},
		{"doc-claimed", 2},
		{"doc_claimed", 2},
		{"sme-stated", 2},
		{"sme_stated", 2},
		{"assumed", 1},
		{"unknown", 1},
	}
	for _, tc := range cases {
		got := sc.TierCeiling(tc.tier)
		if got != tc.want {
			t.Errorf("TierCeiling(%q) = %d, want %d", tc.tier, got, tc.want)
		}
	}
}

// ---- DeriveConfidenceFromTier -----------------------------------------------

func TestDeriveConfidenceFromTier(t *testing.T) {
	cases := []struct {
		tier         string
		corroborated bool
		want         int
	}{
		{"verified", true, 5},
		{"verified", false, 4},
		{"pre-scan", true, 4},
		{"pre-scan", false, 3},
		{"doc-claimed", true, 2},
		{"doc-claimed", false, 1},
		{"assumed", true, 1},
		{"assumed", false, 1}, // floor at 1
	}
	for _, tc := range cases {
		got := sc.DeriveConfidenceFromTier(tc.tier, tc.corroborated)
		if got != tc.want {
			t.Errorf("DeriveConfidenceFromTier(%q, %v) = %d, want %d",
				tc.tier, tc.corroborated, got, tc.want)
		}
	}
}

// ---- DeriveDisposition ------------------------------------------------------

func TestDeriveDisposition(t *testing.T) {
	cases := []struct {
		result         string
		confidence     int
		state          string
		hasEvidenceGap bool
		want           string
	}{
		// contradiction always wins
		{"contradiction", 5, "converged", false, "contradiction"},
		{"contradicts", 5, "converged", false, "contradiction"},
		// evidence_gap result → lean_fail
		{"evidence_gap", 0, "converged", false, "lean_fail"},
		// not_applicable paths
		{"pass", 0, "needs_sources", false, "not_applicable"},
		{"pass", 0, "exploring", false, "not_applicable"},
		{"pass", 1, "exploring", false, "lean_pass"}, // exploring with confidence>0 falls through to result switch
		// truncated (state not in terminal set)
		{"pass", 5, "running", false, "lean_fail"},
		// pass paths
		{"pass", 4, "converged", false, "pass"},
		{"pass", 5, "converged", false, "pass"},
		{"pass", 3, "converged", false, "lean_pass"},
		{"pass", 1, "converged", false, "lean_pass"},
		// fail paths
		{"fail", 5, "converged", false, "fail"},
		{"fail", 5, "converged", true, "lean_fail"},
		// partial paths
		{"partial", 3, "converged", false, "mixed"},
		{"partial", 2, "converged", false, "lean_fail"},
		{"partial", 5, "converged", true, "lean_fail"},
		// abandoned state
		{"pass", 4, "abandoned", false, "pass"},
	}
	for _, tc := range cases {
		got := sc.DeriveDisposition(tc.result, tc.confidence, tc.state, tc.hasEvidenceGap)
		if got != tc.want {
			t.Errorf("DeriveDisposition(%q, %d, %q, %v) = %q, want %q",
				tc.result, tc.confidence, tc.state, tc.hasEvidenceGap, got, tc.want)
		}
	}
}

// ---- DeriveCompleteness -----------------------------------------------------

func TestDeriveCompleteness(t *testing.T) {
	cases := []struct {
		state          string
		confidence     int
		hasEvidenceGap bool
		want           string
	}{
		{"converged", 3, false, "conclusive"},
		{"converged", 5, false, "conclusive"},
		{"converged", 2, false, "evidence_gap"},
		{"converged", 1, false, "evidence_gap"},
		{"converged", 3, true, "evidence_gap"},
		{"abandoned", 5, false, "evidence_gap"},
		{"exploring", 5, false, "evidence_gap"},
		{"needs_sources", 3, false, "evidence_gap"},
	}
	for _, tc := range cases {
		got := sc.DeriveCompleteness(tc.state, tc.confidence, tc.hasEvidenceGap)
		if got != tc.want {
			t.Errorf("DeriveCompleteness(%q, %d, %v) = %q, want %q",
				tc.state, tc.confidence, tc.hasEvidenceGap, got, tc.want)
		}
	}
}

// ---- DeriveCoverage ---------------------------------------------------------

func TestDeriveCoverage_WithCriteria(t *testing.T) {
	criteria := []sc.Criterion{
		{Met: true, Weight: 0.4},
		{Met: true, Weight: 0.3},
		{Met: false, Weight: 0.3},
	}
	got := sc.DeriveCoverage(5, "pass", criteria)
	// 0.4 + 0.3 = 0.70 rounded to 2dp
	if got != 0.70 {
		t.Errorf("DeriveCoverage with criteria = %v, want 0.70", got)
	}
}

func TestDeriveCoverage_CriteriaCapAt1(t *testing.T) {
	criteria := []sc.Criterion{
		{Met: true, Weight: 0.6},
		{Met: true, Weight: 0.6},
	}
	got := sc.DeriveCoverage(5, "pass", criteria)
	if got != 1.0 {
		t.Errorf("DeriveCoverage with over-weight criteria = %v, want 1.0", got)
	}
}

func TestDeriveCoverage_FallbackPass(t *testing.T) {
	got := sc.DeriveCoverage(4, "pass", nil)
	// 4/5 = 0.80
	if got != 0.80 {
		t.Errorf("DeriveCoverage fallback pass = %v, want 0.80", got)
	}
}

func TestDeriveCoverage_FallbackFail(t *testing.T) {
	got := sc.DeriveCoverage(5, "fail", nil)
	if got != 0.0 {
		t.Errorf("DeriveCoverage fallback fail = %v, want 0.0", got)
	}
}

func TestDeriveCoverage_FallbackPartial(t *testing.T) {
	got := sc.DeriveCoverage(5, "partial", nil)
	// 5/5 * 0.6 = 0.60
	if got != 0.60 {
		t.Errorf("DeriveCoverage fallback partial = %v, want 0.60", got)
	}
}

// ---- Tier alias & constants ---------------------------------------------------

// Tier is a type ALIAS for string, so string-typed call sites (the original
// consumer contract) must keep compiling without conversion in either direction.
func TestTierIsStringAlias(t *testing.T) {
	var plain string = sc.TierVerified // constant assigns to plain string
	var tier sc.Tier = plain           // plain string assigns to Tier
	if sc.TierRank(plain) != 5 || sc.TierRank(tier) != 5 {
		t.Error("TierRank must accept plain strings and Tier values identically")
	}
}

func TestTierConstantsAreCanonical(t *testing.T) {
	cases := []struct {
		constant sc.Tier
		want     string
	}{
		{sc.TierVerified, "verified"},
		{sc.TierPreScan, "pre-scan"},
		{sc.TierDocClaimed, "doc-claimed"},
		{sc.TierSMEStated, "sme-stated"},
		{sc.TierAssumed, "assumed"},
	}
	for _, tc := range cases {
		if tc.constant != tc.want {
			t.Errorf("tier constant = %q, want %q", tc.constant, tc.want)
		}
	}
}

// Round-trip: every exported constant normalizes to itself (fixed point), and
// known aliases normalize onto the constants. Normalization is idempotent.
func TestNormalizeEvidenceTierRoundTrip(t *testing.T) {
	constants := []sc.Tier{
		sc.TierVerified, sc.TierPreScan, sc.TierDocClaimed,
		sc.TierSMEStated, sc.TierAssumed,
	}
	for _, c := range constants {
		if got := sc.NormalizeEvidenceTier(c); got != c {
			t.Errorf("NormalizeEvidenceTier(%q) = %q, want fixed point", c, got)
		}
	}
	aliases := []struct {
		input string
		want  sc.Tier
	}{
		{"code_verified", sc.TierVerified},
		{"red", sc.TierPreScan},
		{"doc_claimed", sc.TierDocClaimed},
		{"documentation", sc.TierDocClaimed},
		{"sme_stated", sc.TierSMEStated},
		{"", sc.TierAssumed},
		{"garbage-tier", sc.TierAssumed},
	}
	for _, tc := range aliases {
		got := sc.NormalizeEvidenceTier(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeEvidenceTier(%q) = %q, want %q", tc.input, got, tc.want)
		}
		if again := sc.NormalizeEvidenceTier(got); again != got {
			t.Errorf("NormalizeEvidenceTier not idempotent: %q → %q → %q", tc.input, got, again)
		}
	}
}

func TestTierRankExactValues(t *testing.T) {
	cases := []struct {
		tier sc.Tier
		want int
	}{
		{sc.TierVerified, 5},
		{sc.TierPreScan, 4},
		{"pre_scan", 4},
		{sc.TierDocClaimed, 3},
		{"doc_claimed", 3},
		{sc.TierSMEStated, 2},
		{"sme_stated", 2},
		{sc.TierAssumed, 0},
		{"unknown", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := sc.TierRank(tc.tier); got != tc.want {
			t.Errorf("TierRank(%q) = %d, want %d", tc.tier, got, tc.want)
		}
	}
}

// TierRank and TierCeiling are different scales by design: rank is the trust
// ordering (doc-claimed outranks sme-stated as a source), ceiling is the
// confidence cap (both unverified tiers cap at 2). This test pins the
// documented asymmetry so it cannot silently drift.
func TestTierRankVsCeilingAsymmetryIsIntentional(t *testing.T) {
	if got := sc.TierRank(sc.TierDocClaimed); got != 3 {
		t.Errorf("TierRank(doc-claimed) = %d, want 3 (ordering scale)", got)
	}
	if got := sc.TierCeiling(sc.TierDocClaimed); got != 2 {
		t.Errorf("TierCeiling(doc-claimed) = %d, want 2 (confidence cap)", got)
	}
	// The cap is shared with sme-stated: unverified hearsay never exceeds 2 …
	if sc.TierCeiling(sc.TierDocClaimed) != sc.TierCeiling(sc.TierSMEStated) {
		t.Error("doc-claimed and sme-stated must share the same confidence ceiling")
	}
	// … yet the ordering still distinguishes them.
	if sc.TierRank(sc.TierDocClaimed) <= sc.TierRank(sc.TierSMEStated) {
		t.Error("doc-claimed must outrank sme-stated in the trust ordering")
	}
	// ApplyCeiling must agree with TierCeiling for the tiers it caps
	// (verified and pre-scan are deliberately uncapped by ApplyCeiling).
	for _, tier := range []sc.Tier{sc.TierDocClaimed, sc.TierSMEStated, sc.TierAssumed} {
		if got, want := sc.ApplyCeiling(5, tier), sc.TierCeiling(tier); got != want {
			t.Errorf("ApplyCeiling(5, %q) = %d, want TierCeiling %d", tier, got, want)
		}
	}
}

func TestDeriveCoverage_Deterministic(t *testing.T) {
	criteria := []sc.Criterion{{Met: true, Weight: 0.5}, {Met: false, Weight: 0.5}}
	v1 := sc.DeriveCoverage(3, "partial", criteria)
	v2 := sc.DeriveCoverage(3, "partial", criteria)
	if v1 != v2 {
		t.Errorf("DeriveCoverage is not deterministic: %v vs %v", v1, v2)
	}
}
