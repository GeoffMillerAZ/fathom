package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/adr"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const inForceADRLog = `# Decisions

Status: accepted unless noted.

## ADR-001 — Old storage engine

**Status.** Superseded by ADR-003 (2026-07-22). Kept as written.

**Decision.** Persist observations in engine X.

## ADR-002 — Extraction via runtime Y

**Status.** Superseded in mechanism by ADR-003; revived in part by
ADR-004 §2. Kept as written.

**Decision.** Run grammars in runtime Y.

## ADR-003 — Current storage

**Decision.** Persist observations in engine Z.

## ADR-004 — A proposal

**Status.** Proposed 2026-10-01.

**Decision.** Maybe later.
`

// adrLineOf is the 1-based line of the first line of text starting with prefix.
func adrLineOf(t *testing.T, text, prefix string) int {
	t.Helper()
	for i, ln := range strings.Split(text, "\n") {
		if strings.HasPrefix(ln, prefix) {
			return i + 1
		}
	}
	t.Fatalf("no line starting %q", prefix)
	return 0
}

func seedCited(t *testing.T, st store.Store, id, rule, cite string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: LayerObligations, LayerVersion: "v1", Kind: KindObligation,
		Subject: "ADR.md#" + id, Rule: rule, Scope: []string{"**"},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: core.ClaimFresh,
		PromptVersion: obligationsPromptVersion(), ReproKey: id, InputDigest: "d",
		CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{
		{ClaimID: id, Rel: core.RelCites, TargetRef: cite, TargetHash: "h"},
		{ClaimID: id, Rel: core.RelScopes, TargetRef: "**"},
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// TestClaimCitingSupersededADRIsWithheldAndCounted pins RS-2 (ADR-077 d3):
// a claim citing a fully superseded ADR section is withheld from the cap and
// counted on the payload; a partly superseded one stays, tagged; a section
// without a status line is accepted; valid_to stays empty.
func TestClaimCitingSupersededADRIsWithheldAndCounted(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte(inForceADRLog), 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}
	cite := func(prefix string) string {
		l := adrLineOf(t, inForceADRLog, prefix)
		return fmt.Sprintf("ADR.md#L%d-L%d", l, l)
	}
	seedCited(t, st, "full", "Persist observations in engine X.", cite("**Decision.** Persist observations in engine X."))
	seedCited(t, st, "part", "Run grammars in runtime Y.", cite("**Decision.** Run grammars in runtime Y."))
	seedCited(t, st, "bare", "Persist observations in engine Z.", cite("**Decision.** Persist observations in engine Z."))
	seedCited(t, st, "prop", "Maybe later.", cite("**Decision.** Maybe later."))
	seedCited(t, st, "guide", "A guide rule.", "docs/g.md#L1-L2")

	got, err := ObligationsFor(ctx, st, "svc", "internal/x/y.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	byID := map[string]Obligation{}
	for _, r := range got.Rules {
		byID[r.ClaimID] = r
	}
	if _, ok := byID["full"]; ok {
		t.Errorf("a claim citing a fully superseded section was served: %+v", got.Rules)
	}
	for _, id := range []string{"part", "bare", "guide"} {
		if _, ok := byID[id]; !ok {
			t.Errorf("%s withheld, want served: %+v", id, got.Rules)
		}
	}
	if _, ok := byID["prop"]; ok {
		t.Error("proposed claim served under ADR-080 d2")
	}
	if got.InForceWithheld != 2 {
		t.Errorf("InForceWithheld = %d, want 2", got.InForceWithheld)
	}
	if got.Superseded != 1 {
		t.Errorf("Superseded = %d, want 1", got.Superseded)
	}
	if byID["part"].InForce != partlySuperseded {
		t.Errorf("partly superseded rule tag = %q, want %q", byID["part"].InForce, partlySuperseded)
	}
	for _, id := range []string{"bare", "guide"} {
		if byID[id].InForce != "" {
			t.Errorf("%s tagged %q, want in force", id, byID[id].InForce)
		}
	}
	b, _ := json.Marshal(got)
	if !strings.Contains(string(b), `"superseded_withheld":1`) {
		t.Errorf("payload does not disclose the withhold: %s", b)
	}
	// Withheld at read, never written: valid_to stays empty.
	all, err := st.ClaimsForService(ctx, "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, cl := range all {
		if !cl.ValidTo.IsZero() {
			t.Errorf("%s valid_to = %v, want empty", cl.ClaimID, cl.ValidTo)
		}
	}

	// Against this repository's own log, the cases ADR-077 names: ADR-003
	// and ADR-013 are fully superseded, ADR-004 partly, ADR-020 in force.
	data, err := os.ReadFile(filepath.Join("..", "..", "ADR.md"))
	if err != nil {
		t.Fatal(err)
	}
	secs := adr.ParseSections(data)
	for _, c := range []struct{ id, want string }{
		{"ADR-003", fullySuperseded}, {"ADR-013", fullySuperseded}, {"ADR-004", partlySuperseded}, {"ADR-020", inForce},
	} {
		line := 0
		for _, s := range secs {
			if s.ID == c.id {
				line = s.Line + 2
				break
			}
		}
		if line == 0 {
			t.Fatalf("%s not in ADR.md", c.id)
		}
		if got, rec := claimInForce(fmt.Sprintf("ADR.md#L%d-L%d", line, line), secs); got != c.want {
			t.Errorf("%s: standing %q (record %s), want %q", c.id, got, rec, c.want)
		}
	}
}
