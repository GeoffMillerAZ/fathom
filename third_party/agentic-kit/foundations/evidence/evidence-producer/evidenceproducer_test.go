package evidenceproducer_test

import (
	"testing"

	ep "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-producer"
	scoring "github.com/geoffmilleraz/agentic-kit/foundations/review/scoring"
)

// engineeringLadder mirrors testdata/valid.cue's engineeringCatalog ladder
// and foundations/review/scoring's Tier* constants.
var engineeringLadder = []string{
	scoring.TierVerified,
	scoring.TierPreScan,
	scoring.TierDocClaimed,
	scoring.TierSMEStated,
	scoring.TierAssumed,
}

func validCatalog() ep.Catalog {
	return ep.Catalog{
		Profile: "engineering",
		Ladder:  engineeringLadder,
		Producers: []ep.Producer{
			{ID: "native-analyzer", Kind: ep.KindDeterministic, Origin: ep.OriginExternal, TierCap: scoring.TierPreScan},
			{ID: "review-kit-model", Kind: ep.KindModel, Origin: ep.OriginExternal, TierCap: scoring.TierSMEStated, Requires: []string{"review-kit"}, Version: "review-kit-v3"},
			{ID: "human-reviewer", Kind: ep.KindHuman, Origin: ep.OriginExternal, TierCap: scoring.TierSMEStated},
		},
	}
}

// ---------------------------------------------------------------------------
// Catalog.Validate
// ---------------------------------------------------------------------------

func TestCatalogValidate(t *testing.T) {
	t.Parallel()

	mutate := func(f func(*ep.Catalog)) ep.Catalog {
		c := validCatalog()
		f(&c)
		return c
	}

	cases := []struct {
		name    string
		catalog ep.Catalog
		wantErr bool
	}{
		{"valid catalog", validCatalog(), false},
		{"empty profile", mutate(func(c *ep.Catalog) { c.Profile = "" }), true},
		{"empty ladder", mutate(func(c *ep.Catalog) { c.Ladder = nil }), true},
		{"ladder with empty entry", mutate(func(c *ep.Catalog) { c.Ladder = []string{"verified", ""} }), true},
		{"duplicate ladder entry", mutate(func(c *ep.Catalog) { c.Ladder = []string{"verified", "verified"} }), true},
		{"single-tier ladder (boundary)", ep.Catalog{
			Profile: "single",
			Ladder:  []string{"only"},
			Producers: []ep.Producer{
				{ID: "p1", Kind: ep.KindHuman, Origin: ep.OriginExternal, TierCap: "only"},
			},
		}, false},
		{"empty producer id", mutate(func(c *ep.Catalog) { c.Producers[0].ID = "" }), true},
		{"duplicate producer id", mutate(func(c *ep.Catalog) {
			c.Producers[1].ID = c.Producers[0].ID
		}), true},
		{"unknown kind", mutate(func(c *ep.Catalog) { c.Producers[0].Kind = "quantum" }), true},
		{"unknown origin", mutate(func(c *ep.Catalog) { c.Producers[0].Origin = "orbit" }), true},
		{"tier_cap not in ladder", mutate(func(c *ep.Catalog) { c.Producers[0].TierCap = "not-a-tier" }), true},
		{"empty requires entry", mutate(func(c *ep.Catalog) {
			c.Producers[0].Requires = []string{""}
		}), true},
		{"no producers (boundary, still valid)", mutate(func(c *ep.Catalog) { c.Producers = nil }), false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.catalog.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Catalog.Lookup
// ---------------------------------------------------------------------------

func TestCatalogLookup(t *testing.T) {
	t.Parallel()
	c := validCatalog()

	if p, ok := c.Lookup("review-kit-model"); !ok || p.Kind != ep.KindModel {
		t.Errorf("Lookup(review-kit-model) = %+v, %v; want KindModel, true", p, ok)
	}
	if _, ok := c.Lookup("nonexistent"); ok {
		t.Error("Lookup(nonexistent) ok=true; want false")
	}
	if _, ok := (ep.Catalog{}).Lookup("anything"); ok {
		t.Error("Lookup on empty Catalog ok=true; want false")
	}
}

// ---------------------------------------------------------------------------
// Catalog.Rank
// ---------------------------------------------------------------------------

func TestCatalogRank(t *testing.T) {
	t.Parallel()
	c := validCatalog()

	cases := []struct {
		tier     string
		wantRank int
		wantOK   bool
	}{
		{scoring.TierVerified, 0, true}, // strongest = rank 0
		{scoring.TierPreScan, 1, true},
		{scoring.TierDocClaimed, 2, true},
		{scoring.TierSMEStated, 3, true},
		{scoring.TierAssumed, 4, true}, // weakest = last rank (boundary)
		{"not-a-tier", 0, false},
	}
	for _, tc := range cases {
		rank, ok := c.Rank(tc.tier)
		if ok != tc.wantOK || (ok && rank != tc.wantRank) {
			t.Errorf("Rank(%q) = %d, %v; want %d, %v", tc.tier, rank, ok, tc.wantRank, tc.wantOK)
		}
	}
}

// ---------------------------------------------------------------------------
// Catalog.Cap — served tier = min(claimed, producer cap); tier never rises.
// ---------------------------------------------------------------------------

func TestCatalogCap(t *testing.T) {
	t.Parallel()
	c := validCatalog()
	preScanCapped, _ := c.Lookup("native-analyzer") // TierCap = pre-scan
	smeCapped, _ := c.Lookup("human-reviewer")      // TierCap = sme-stated

	cases := []struct {
		name     string
		claimed  string
		producer ep.Producer
		want     string
		wantErr  bool
	}{
		{"claimed stronger than cap: capped down", scoring.TierVerified, preScanCapped, scoring.TierPreScan, false},
		{"claimed weaker than cap: claimed wins", scoring.TierAssumed, preScanCapped, scoring.TierAssumed, false},
		{"claimed equals cap: no change (boundary)", scoring.TierPreScan, preScanCapped, scoring.TierPreScan, false},
		{"claimed stronger than sme cap", scoring.TierDocClaimed, smeCapped, scoring.TierSMEStated, false},
		{"claimed not in ladder", "bogus", preScanCapped, "", true},
		{"producer cap not in ladder", scoring.TierVerified, ep.Producer{ID: "x", TierCap: "bogus"}, "", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := c.Cap(tc.claimed, tc.producer)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Cap() error=%v, wantErr=%v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("Cap(%q, %+v) = %q; want %q", tc.claimed, tc.producer, got, tc.want)
			}
		})
	}
}

// TestCatalogCapNeverRisesAboveClaimed is a property check: across every
// pairing of claimed tier and producer cap in the engineering ladder, the
// served tier's rank is always >= both input ranks (i.e. never stronger
// than either) — served tier = min(claimed, cap), never a promotion.
func TestCatalogCapNeverRisesAboveClaimed(t *testing.T) {
	t.Parallel()
	c := ep.Catalog{Profile: "engineering", Ladder: engineeringLadder}
	for _, claimed := range engineeringLadder {
		for _, cap := range engineeringLadder {
			p := ep.Producer{ID: "p", TierCap: cap}
			got, err := c.Cap(claimed, p)
			if err != nil {
				t.Fatalf("Cap(%q, TierCap=%q) unexpected error: %v", claimed, cap, err)
			}
			gotRank, _ := c.Rank(got)
			claimedRank, _ := c.Rank(claimed)
			capRank, _ := c.Rank(cap)
			if gotRank < claimedRank || gotRank < capRank {
				t.Errorf("Cap(%q, TierCap=%q) = %q (rank %d); must be >= both input ranks (%d, %d) — tier rose", claimed, cap, got, gotRank, claimedRank, capRank)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Catalog.Independent — corroboration counts only across independent
// producers, never the same producer twice.
// ---------------------------------------------------------------------------

func TestCatalogIndependent(t *testing.T) {
	t.Parallel()
	c := validCatalog()
	a, _ := c.Lookup("native-analyzer")
	b, _ := c.Lookup("review-kit-model")

	if !c.Independent(a, b) {
		t.Error("Independent(a, b) = false for distinct IDs; want true")
	}
	if c.Independent(a, a) {
		t.Error("Independent(a, a) = true; want false — same producer is not a second witness")
	}
	// Same ID, different struct values (e.g. re-fetched/stale copy): still
	// not independent — identity is by ID, not by struct equality.
	aStale := a
	aStale.Version = "different-version"
	if c.Independent(a, aStale) {
		t.Error("Independent(a, aStale) = true for same ID with differing fields; want false")
	}
}

// ---------------------------------------------------------------------------
// Catalog.Admit — a producer whose origin is the consuming system itself is
// refused on import.
// ---------------------------------------------------------------------------

func TestCatalogAdmit(t *testing.T) {
	t.Parallel()
	c := validCatalog()

	external := ep.Producer{ID: "ext", Origin: ep.OriginExternal}
	if err := c.Admit(external); err != nil {
		t.Errorf("Admit(external) = %v; want nil", err)
	}

	self := ep.Producer{ID: "native-store", Origin: ep.OriginSelf}
	err := c.Admit(self)
	if err == nil {
		t.Fatal("Admit(self) = nil; want circular-import error")
	}
	const want = `circular: producer "native-store" originates from the consuming system`
	if err.Error() != want {
		t.Errorf("Admit(self) error = %q; want %q", err.Error(), want)
	}
}

// ---------------------------------------------------------------------------
// DefaultKindCaps — documentation helper, never enforced by Validate.
// ---------------------------------------------------------------------------

func TestDefaultKindCaps(t *testing.T) {
	t.Parallel()
	caps := ep.DefaultKindCaps()

	want := map[ep.Kind]string{
		ep.KindDeterministic: scoring.TierPreScan,
		ep.KindModel:         scoring.TierSMEStated,
		ep.KindHuman:         scoring.TierSMEStated,
		ep.KindExternalTool:  scoring.TierPreScan,
	}
	if len(caps) != len(want) {
		t.Fatalf("DefaultKindCaps() has %d entries; want %d", len(caps), len(want))
	}
	for k, wantTier := range want {
		if got := caps[k]; got != wantTier {
			t.Errorf("DefaultKindCaps()[%q] = %q; want %q", k, got, wantTier)
		}
	}

	// It is a helper, never enforced: a catalog may declare a producer of
	// KindDeterministic with a stronger cap than the default without
	// Validate objecting.
	strongerThanDefault := ep.Catalog{
		Profile: "engineering",
		Ladder:  engineeringLadder,
		Producers: []ep.Producer{
			{ID: "oracle-backed", Kind: ep.KindDeterministic, Origin: ep.OriginExternal, TierCap: scoring.TierVerified},
		},
	}
	if err := strongerThanDefault.Validate(); err != nil {
		t.Errorf("Validate() rejected a TierCap stronger than DefaultKindCaps' recommendation: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CUE schema vet (invoked externally via `cue vet ./...`)
// ---------------------------------------------------------------------------
// This test is a marker: `cue vet ./foundations/evidence/evidence-producer/...`
// is run as part of the cue-vet task and verifies that testdata/valid.cue
// satisfies the schema. We add a compile-time guard here so the presence of
// the schema-mirroring Go types is at least referenced; CUE conformance
// itself is validated by `cue vet`, not this test.

func TestSchemaExists(t *testing.T) {
	t.Parallel()
	_ = ep.KindDeterministic
	_ = ep.KindModel
	_ = ep.KindHuman
	_ = ep.KindExternalTool
	_ = ep.OriginSelf
	_ = ep.OriginExternal
}
