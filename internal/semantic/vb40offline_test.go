package semantic

// vb40offline_test.go — RS-M2's reading harness (docs/plans/rule-selection.md,
// docs/bench/readings/rule-selection/vb40-offline.md). VB-40 offline: for each
// of the 8 guards tasks of testdata/scenariobench/guards-fathom.json, is the
// task's governing rule in the served set (the 6-rule cap) at the first touch
// of its target path? Measured the way h1governance_test.go measures the
// what-governs lookups: offline, no model, scored on the served row's own
// text and citation by keyword (decision-port-ab-2026-09-23.md's "matched by
// keyword against its own citation/topic").
//
// It needs real compiled claims, which only a store holds, so it reads a
// COPY of one named by FATHOM_RS_STORE and skips without it: a reading
// instrument, not a gate. The matchers below were written from each task's
// own rule block before any served list was read.

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type vb40Task struct {
	id, path string
	governs  func(text string) bool // lower-cased rule + " " + src + " " + subject
}

func vb40Has(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func vb40Tasks() []vb40Task {
	return []vb40Task{
		{"guards-i1-packagedeps-sorted", "internal/analysis/packagedeps/packagedeps.go", func(s string) bool {
			return vb40Has(s, "map iteration") || (strings.Contains(s, "sort") && vb40Has(s, "output", "determinis"))
		}},
		{"guards-i2-store-purego", "internal/store/sqlite.go", func(s string) bool {
			return vb40Has(s, "cgo", "pure go", "pure-go") && vb40Has(s, "store", "sqlite")
		}},
		{"guards-i11-mcp-tool-doc", "internal/mcp/server.go", func(s string) bool {
			return vb40Has(s, "mcp-tools.md")
		}},
		{"guards-i16-raw-cp-deploy", "CLAUDE.md", func(s string) bool {
			return vb40Has(s, "raw cp", "raw-cp", "raw `cp`", "task deploy")
		}},
		{"guards-i19-org-agnostic-fixture", "internal/connector/testdata/multiremote/README.md", func(s string) bool {
			return vb40Has(s, "org-agnostic", "org-specific", "adr-011")
		}},
		{"guards-i28-duckdb-path", "internal/store/backend.go", func(s string) bool {
			return strings.Contains(s, "duckdb") && vb40Has(s, "refus", "backendforpath")
		}},
		{"guards-i31-workspace-artifact", "cmd/fathom/defaultpaths.go", func(s string) bool {
			return strings.Contains(s, "workspace/")
		}},
		{"guards-i36-usecase-maintained-line", "docs/use-cases/incident-investigation.md", func(s string) bool {
			return strings.Contains(s, "maintained")
		}},
	}
}

func TestVB40Offline_GuardsFirstTouch(t *testing.T) {
	path := os.Getenv("FATHOM_RS_STORE")
	if path == "" {
		t.Skip("reading harness: set FATHOM_RS_STORE to a copy of a store holding service fathom's compiled claims")
	}
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer st.Close()
	ctx := context.Background()
	served, servedGoverns, servedInScope, servedClaimsRole := 0, 0, 0, 0
	for _, task := range vb40Tasks() {
		obl, err := ObligationsFor(ctx, st, "fathom", task.path, MaxObligations)
		if err != nil {
			t.Fatalf("%s: %v", task.id, err)
		}
		pos := 0
		for i, r := range obl.Rules {
			if task.governs(strings.ToLower(r.Rule + " " + r.Src + " " + r.Subject)) {
				pos = i + 1
				break
			}
		}
		ex, err := ExplainObligations(ctx, st, "fathom", task.path, MaxObligations, ObligationLayerAll)
		if err != nil {
			t.Fatalf("%s explain: %v", task.id, err)
		}
		// Fixture precision: of the served rules, those whose stored scope
		// covers the edited path (a claims-role rule's scope is re-derived
		// from its plan row's contract at serve time, so it is counted
		// apart).
		for _, c := range ex.Candidates {
			if !c.Served {
				continue
			}
			if c.Role == RoleClaims {
				servedClaimsRole++
				continue
			}
			servedGoverns++
			if fileScopeMatch(task.path)(core.SemanticClaim{}, c.Scope) >= 0 {
				servedInScope++
			}
		}
		bestRank, bestTier, bestSrc := 0, "", ""
		for _, c := range ex.Candidates {
			if task.governs(strings.ToLower(c.Rule + " " + c.Src)) {
				bestRank, bestTier, bestSrc = c.Rank, c.ScopeTier, c.Src
				break
			}
		}
		verdict := "MISS"
		if pos > 0 {
			verdict = fmt.Sprintf("SERVED pos %d", pos)
			served++
		}
		t.Logf("%s %s: %s of %d (eligible %d, superseded withheld %d); first governing candidate rank %d tier %s %s",
			task.id, task.path, verdict, len(obl.Rules), ex.Eligible, obl.Superseded, bestRank, bestTier, bestSrc)
		var tiers []string
		for _, tr := range ex.Tiers {
			tiers = append(tiers, fmt.Sprintf("%s=%d", tr.Tier, tr.Candidates))
		}
		t.Logf("    candidates by scope tier: %s", strings.Join(tiers, " "))
		for i, r := range obl.Rules {
			rule := r.Rule
			if len(rule) > 90 {
				rule = rule[:90]
			}
			t.Logf("    %d. [%s] %s — %s", i+1, r.Layer, r.Src, rule)
		}
	}
	t.Logf("VB-40 offline: governing rule served at first touch on %d of 8", served)
	t.Logf("fixture precision: %d of %d served governs-role rules scoped to their path (%d claims-role rules contract-scoped)", servedInScope, servedGoverns, servedClaimsRole)

	// Added hook latency, CPU time: the median and p95 of the serve call over
	// the 8 paths, repeated. User+system CPU of this process, so a loaded
	// host's wall-clock noise does not read as cost.
	const reps = 40
	var samples []time.Duration
	for r := 0; r < reps; r++ {
		for _, task := range vb40Tasks() {
			before := processCPU()
			if _, err := ObligationsFor(ctx, st, "fathom", task.path, MaxObligations); err != nil {
				t.Fatalf("%s: %v", task.id, err)
			}
			samples = append(samples, processCPU()-before)
		}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	t.Logf("serve CPU over %d calls: p50 %v p95 %v", len(samples), samples[len(samples)/2], samples[len(samples)*95/100])
}

// TestRSM3InForceListing is RS-M3's listing (docs/bench/readings/
// rule-selection/in-force-check.md): every served-status claim citing the
// decision log, with the standing claimInForce gives it, for a hand check.
func TestRSM3InForceListing(t *testing.T) {
	path := os.Getenv("FATHOM_RS_STORE")
	if path == "" {
		t.Skip("reading harness: set FATHOM_RS_STORE to a copy of a store holding service fathom's compiled claims")
	}
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer st.Close()
	ctx := context.Background()
	claims, err := st.ClaimsForServe(ctx, "fathom", nil, servedClaimStatuses, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(claims))
	for i, cl := range claims {
		ids[i] = cl.ClaimID
	}
	links, err := st.ClaimLinks(ctx, "fathom", ids)
	if err != nil {
		t.Fatal(err)
	}
	cite := map[string]string{}
	for _, l := range links {
		if l.Rel == core.RelCites {
			if _, ok := cite[l.ClaimID]; !ok {
				cite[l.ClaimID] = l.TargetRef
			}
		}
	}
	standing := inForceResolver(serviceRoot(ctx, st, "fathom"))
	counts := map[string]int{}
	sort.Slice(claims, func(i, j int) bool { return cite[claims[i].ClaimID] < cite[claims[j].ClaimID] })
	for _, cl := range claims {
		c := cite[cl.ClaimID]
		if !strings.HasPrefix(c, "ADR.md#") {
			continue
		}
		s, rec := standing(c)
		counts[s]++
		if s != inForce || strings.Contains(cl.Rule, "ADR-003") || strings.Contains(cl.Rule, "ADR-004") || strings.Contains(cl.Rule, "ADR-013") {
			t.Logf("%-18s %-8s %-7s %s %s | %s", "standing="+s, rec, cl.Layer[:3], cl.Status, c, cl.Rule)
		}
	}
	t.Logf("ADR.md-citing served-status claims by standing: %v", counts)
}
