package console

// certhistory.go -- WI-20/WI-21 product input: the CertHistory list
// certificateRegressions and conservationRegressions replay, read from
// what the store already holds (no new capture). Each compiled
// certification claim names its (kind, target, test symbol) in its Meta;
// the test's attested case runs (store.TestCaseRunsForSymbol) are the
// history. A claim's Reached is read back from its tier, because
// internal/certify writes TierPreScan or above only for a reached
// non-behaviour target, and a behaviour target is reached when it
// resolved (a contradicted claim did not).

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// certHistoryRunLimit bounds the runs replayed per certificate.
const certHistoryRunLimit = 200

// CertHistoriesFromStore reads service's compiled certifications and each
// one's attested run history, sorted by (kind, target, test).
func CertHistoriesFromStore(ctx context.Context, st store.Store, service string) ([]CertHistory, error) {
	claims, err := st.ClaimsForService(ctx, service, certify.Layer)
	if err != nil {
		return nil, err
	}
	var out []CertHistory
	for _, c := range claims {
		if c.Kind != certify.Kind {
			continue
		}
		var m certify.Meta
		if json.Unmarshal(c.Meta, &m) != nil || m.TestSymbolKey == "" {
			continue
		}
		runs, err := st.TestCaseRunsForSymbol(ctx, service, m.TestSymbolKey, certHistoryRunLimit)
		if err != nil {
			return nil, err
		}
		reached := c.Tier == core.TierPreScan || c.Tier == core.TierVerified ||
			(m.TargetKind == "behaviour" && c.Status != core.ClaimContradicted)
		out = append(out, CertHistory{Service: service, Kind: m.TargetKind, TargetID: m.TargetID, Test: m.Test, Reached: reached, Runs: runs})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.TargetID != b.TargetID {
			return a.TargetID < b.TargetID
		}
		return a.Test < b.Test
	})
	return out, nil
}
