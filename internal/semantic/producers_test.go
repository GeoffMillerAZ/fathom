package semantic

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestProducersSchemaMatchesSource is the drift gate FL-1 requires: the
// embedded producers.cue (internal/semantic, go:embed'd into the binary)
// must be byte-identical to schema/semantic/producers.cue, which stays the
// single authoritative source — mirrors TestManifestSchemaMatchesSource.
func TestProducersSchemaMatchesSource(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(file), "..", "..", "schema", "semantic", "producers.cue")
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source producers.cue %s: %v", src, err)
	}
	if string(want) != string(producersSchema) {
		t.Fatalf(
			"internal/semantic/producers.cue has drifted from %s — "+
				"copy the source file verbatim (schema/semantic/producers.cue is authoritative)",
			src,
		)
	}
}

// TestDefaultProducers_ShippedCatalog proves the embedded catalog loads and
// contains every producer vision 30 §4 names, with the right kind/origin/cap.
func TestDefaultProducers_ShippedCatalog(t *testing.T) {
	cat := DefaultProducers()

	cases := []struct {
		id      string
		kind    string
		origin  string
		tierCap core.EvidenceTier
	}{
		{"fathom.review", "deterministic", "fathom", core.TierVerified},
		{"reviewkit.review-import", "deterministic", "fathom", core.TierVerified},
		{"reviewkit.system-review", "deterministic", "external", core.TierPreScan},
		{"reviewkit.security-review", "deterministic", "external", core.TierPreScan},
		{"human.review", "human", "external", core.TierSMEStated},
		{"model.review", "model", "external", core.TierSMEStated},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			p, ok := cat.Lookup(tc.id)
			if !ok {
				t.Fatalf("producer %q not found in default catalog", tc.id)
			}
			if p.Kind != tc.kind {
				t.Errorf("kind = %q, want %q", p.Kind, tc.kind)
			}
			if p.Origin != tc.origin {
				t.Errorf("origin = %q, want %q", p.Origin, tc.origin)
			}
			if p.TierCap != tc.tierCap {
				t.Errorf("tierCap = %q, want %q", p.TierCap, tc.tierCap)
			}
		})
	}

	if _, ok := cat.Lookup("nonexistent.producer"); ok {
		t.Error("Lookup(nonexistent.producer) = found, want not found")
	}

	sysReview, _ := cat.Lookup("reviewkit.system-review")
	if len(sysReview.Requires) != 1 || sysReview.Requires[0] != "rule.closure" {
		t.Errorf("reviewkit.system-review.Requires = %v, want [rule.closure]", sysReview.Requires)
	}
}

// TestLoadProducers_Refusals proves every named validation refusal in
// vision 30 §4 item 1 / FL-1: unknown kind, unknown origin, a cap above
// verified paired with a non-deterministic kind, and a duplicate id.
func TestLoadProducers_Refusals(t *testing.T) {
	cases := []struct {
		name string
		cue  string
	}{
		{
			"unknown kind",
			`package semantic
#Producer: {id: string, kind: string, origin: "fathom" | "external", tier_cap: string, requires: [...string]}
producers: [...#Producer] & [{id: "x.y", kind: "quantum", origin: "external", tier_cap: "assumed", requires: []}]`,
		},
		{
			"unknown origin",
			`package semantic
#Producer: {id: string, kind: string, origin: string, tier_cap: string, requires: [...string]}
producers: [...#Producer] & [{id: "x.y", kind: "human", origin: "moon", tier_cap: "sme-stated", requires: []}]`,
		},
		{
			"non-deterministic producer capped at verified",
			`package semantic
#Producer: {id: string, kind: string, origin: string, tier_cap: string, requires: [...string]}
producers: [...#Producer] & [{id: "x.y", kind: "human", origin: "external", tier_cap: "verified", requires: []}]`,
		},
		{
			"duplicate id",
			`package semantic
#Producer: {id: string, kind: string, origin: string, tier_cap: string, requires: [...string]}
producers: [...#Producer] & [
	{id: "x.y", kind: "human", origin: "external", tier_cap: "sme-stated", requires: []},
	{id: "x.y", kind: "model", origin: "external", tier_cap: "sme-stated", requires: []},
]`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadProducers([]byte(tc.cue)); err == nil {
				t.Error("LoadProducers: expected refusal, got nil error")
			}
		})
	}
}

// TestCapTier is FL-2's table test: served tier = min(event tier, producer
// cap), over every catalog kind, pinning that tier never rises on import.
func TestCapTier(t *testing.T) {
	cases := []struct {
		name    string
		event   core.EvidenceTier
		capTier core.EvidenceTier
		want    core.EvidenceTier
	}{
		{"event weaker than cap: event wins", core.TierAssumed, core.TierVerified, core.TierAssumed},
		{"event stronger than cap: cap wins", core.TierVerified, core.TierPreScan, core.TierPreScan},
		{"event equal to cap: unchanged", core.TierSMEStated, core.TierSMEStated, core.TierSMEStated},
		{"deterministic external producer caps pre-scan", core.TierVerified, core.TierPreScan, core.TierPreScan},
		{"human producer caps sme-stated", core.TierVerified, core.TierSMEStated, core.TierSMEStated},
		{"model producer caps sme-stated", core.TierDocClaimed, core.TierSMEStated, core.TierSMEStated},
		{"fathom-origin producer at verified never lowers a verified event", core.TierVerified, core.TierVerified, core.TierVerified},
		{"assumed event under any cap stays assumed", core.TierAssumed, core.TierSMEStated, core.TierAssumed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CapTier(tc.event, Producer{TierCap: tc.capTier})
			if got != tc.want {
				t.Errorf("CapTier(%q, cap=%q) = %q, want %q", tc.event, tc.capTier, got, tc.want)
			}
		})
	}
}

// TestCapTier_EveryCatalogKind runs CapTier over every shipped producer,
// proving the cap never rises above what the catalog declares.
func TestCapTier_EveryCatalogKind(t *testing.T) {
	cat := DefaultProducers()
	for _, id := range cat.IDs() {
		p, _ := cat.Lookup(id)
		t.Run(id, func(t *testing.T) {
			got := CapTier(core.TierVerified, p)
			if got != p.TierCap {
				t.Errorf("CapTier(verified, %s) = %q, want producer's own cap %q", id, got, p.TierCap)
			}
			got2 := CapTier(core.TierAssumed, p)
			if got2 != core.TierAssumed {
				t.Errorf("CapTier(assumed, %s) = %q, want assumed (weakest always wins)", id, got2)
			}
		})
	}
}

// TestProducerCatalogsAgree is SU-25's cross-check (ADR-041 decision 8):
// fathom's own CUE producer catalog, mapped through CatalogFromKit's origin
// translation (fathom -> self), must satisfy agentic-kit's own #Catalog CUE
// schema — proving the two catalogs describe the same shape rather than
// merely agreeing by construction of the Go structs.
func TestProducerCatalogsAgree(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	kitSchemaPath := filepath.Join(repoRoot, "third_party", "agentic-kit", "foundations", "evidence", "evidence-producer", "schema.cue")
	kitSchema, err := os.ReadFile(kitSchemaPath)
	if err != nil {
		t.Fatalf("read kit schema %s: %v", kitSchemaPath, err)
	}

	cctx := cuecontext.New()
	schemaVal := cctx.CompileBytes(kitSchema, cue.Filename("schema.cue"))
	if err := schemaVal.Err(); err != nil {
		t.Fatalf("compile kit schema: %v", err)
	}
	catalogDef := schemaVal.LookupPath(cue.ParsePath("#Catalog"))
	if !catalogDef.Exists() {
		t.Fatal("kit schema.cue has no #Catalog definition")
	}

	cat := DefaultProducers()
	type wireProducer struct {
		ID       string   `json:"id"`
		Kind     string   `json:"kind"`
		Origin   string   `json:"origin"`
		TierCap  string   `json:"tier_cap"`
		Requires []string `json:"requires"`
	}
	type wireCatalog struct {
		Profile   string         `json:"profile"`
		Ladder    []string       `json:"ladder"`
		Producers []wireProducer `json:"producers"`
	}
	wc := wireCatalog{Profile: "engineering", Ladder: engineeringLadder}
	for _, id := range cat.IDs() {
		p, _ := cat.Lookup(id)
		origin := "external"
		if p.Origin == "fathom" {
			origin = "self"
		}
		reqs := p.Requires
		if reqs == nil {
			reqs = []string{}
		}
		wc.Producers = append(wc.Producers, wireProducer{
			ID: p.ID, Kind: p.Kind, Origin: origin, TierCap: string(p.TierCap), Requires: reqs,
		})
	}

	instance := cctx.Encode(wc)
	if err := instance.Err(); err != nil {
		t.Fatalf("encode fathom catalog for cue vet: %v", err)
	}

	unified := catalogDef.Unify(instance)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("fathom's producers catalog does not satisfy agentic-kit's #Catalog: %v", err)
	}
}
