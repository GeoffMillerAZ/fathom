package indexbench

import (
	"context"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// claims.go seeds the fixture fleet's semantic layer.
//
// L1's first replay entry is `obligations` on a fixture file, because that
// is the tool O23 caught serving six C++ plan rows and displacing sixty-five
// others on a Go file — and a store with no claims serves nothing, so a
// replay over a bare fixture store would have pinned the empty payload and
// proved nothing. The layer below is deliberately SHAPED like the real one:
// more than one layer (obligations, adr-rules, spec-bindings), a mix of
// narrow and universal scopes, and one drifted claim, so the golden pins
// ranking and freshness and not just a byte count.
//
// It is deliberately UNDER the serving cap: every replayed file matches at
// most four claims against a cap of six, so a `displaced` count in the
// golden means a real regression in scoping or ranking, never fixture
// crowding. TestPayloadReplay fails on any displaced count for exactly that
// reason.

// fixtureClaim is one seeded claim, authored here so the fixture layer reads
// as a list rather than as a loop with a switch in it.
type fixtureClaim struct {
	id     string
	layer  string
	rule   string
	scope  []string
	status core.ClaimStatus
	cite   string
}

// fixtureClaims is the committed fixture layer. Service ids are assigned per
// service at seed time: the same rules are seeded into every Go service so a
// replay can ask any of them the same question.
var fixtureClaims = []fixtureClaim{
	{
		id: "fx-sorted", layer: semantic.LayerObligations,
		rule:  "Sort every output slice; map iteration order must never reach a result.",
		scope: []string{"**"}, status: core.ClaimFresh, cite: "docs/determinism.md#L1-L4",
	},
	{
		id: "fx-validate", layer: semantic.LayerObligations,
		rule:  "Validate an entry before applying it; a zero amount is never posted.",
		scope: []string{"ledger-service/**"}, status: core.ClaimFresh, cite: "docs/ledger.md#L10-L14",
	},
	{
		id: "fx-balance", layer: semantic.LayerObligations,
		rule:  "Balance refuses an empty account id rather than returning zero.",
		scope: []string{"ledger-service/ledger.go"}, status: core.ClaimFresh, cite: "docs/ledger.md#L20-L23",
	},
	{
		id: "fx-charge", layer: "adr-rules",
		rule:  "Reject a charge whose amount is not positive.",
		scope: []string{"payment-service/**"}, status: core.ClaimFresh, cite: "ADR.md#L40-L42",
	},
	{
		id: "fx-drifted", layer: "spec-bindings",
		rule:  "Plan row FX-1 claims double-entry posting for every service.",
		scope: []string{"**/*.go"}, status: core.ClaimStale, cite: "docs/plans/fx.md#L7-L8",
	},
}

// SeedFixtureClaims writes the fixture semantic layer for every Go service of
// the fixture fleet. compiledAt is fixed, never time.Now(): a claim stamped
// with the real clock makes a committed golden a time bomb (internal/
// CLAUDE.md's determinism contract).
func SeedFixtureClaims(ctx context.Context, st store.Store) error {
	compiledAt := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for _, svc := range FauxtownGoServices {
		var claims []core.SemanticClaim
		var links []core.ClaimLink
		for _, fc := range fixtureClaims {
			id := svc + ":" + fc.id
			claims = append(claims, core.SemanticClaim{
				ClaimID: id, ServiceID: svc,
				Layer: fc.layer, LayerVersion: "v1", Kind: semantic.KindObligation,
				Subject: "indexbench/fixture#" + fc.id, Rule: fc.rule, Scope: fc.scope,
				Tier: core.TierDocClaimed, TierProfile: "engineering", Status: fc.status,
				PromptVersion: "indexbench-fixture-v1", ReproKey: id, InputDigest: "fixture",
				CompiledAtWatermark: "manifest:indexbench-fixture", CompiledAt: compiledAt,
			})
			links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelCites, TargetRef: fc.cite, TargetHash: "fixture"})
			for _, g := range fc.scope {
				links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopes, TargetRef: g})
			}
		}
		if err := st.PutClaims(ctx, claims, links); err != nil {
			return fmt.Errorf("indexbench: seed claims for %s: %w", svc, err)
		}
	}
	return nil
}
