package main

// statusz.go — Q-25/Q-26 (docs/plans/expert-suite-q.md, docs/design/
// observability-surfaces.md §8 item 6): `GET /statusz` on serve-http —
// daemon health, per-service index freshness, channel liveness, last
// compile per layer. Loopback-only, read-only, gated inside the hosted-
// mode allowlist when `--modules-token-env` is configured
// (servehttp.go's modulesAuthMiddleware, extended there — the
// phase-O-critic-finding-#2 class this closes). The report is built here;
// the console page renders it on the shell (console_statusz.go, OC-6) and
// answers ?format=json with this report unchanged (schema v3).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// statuszSchemaVersion identifies the /statusz payload contract. "2" adds
// the daemon's own build stamp (binary_commit, binary_dirty, built_at —
// PC-1, vision 26 §1.1) so a CLI can tell which binary built the store it
// is about to trust; v1 carried no schema_version at all. "3" adds each
// service's typed_edges block (BP-35): whether the daemon's own typed Go
// call-edge pass ran, and why not when it did not — the fact that was
// invisible while a broken daemon environment served a syntax-only graph
// (2026-09-12). Additive: every v1/v2 field keeps its name and meaning.
const statuszSchemaVersion = 3

// StatuszBuildInfo is the daemon binary's own identity as /statusz reports
// it — the daemon's stamp, never the CLI's or the repo's HEAD (O5).
type StatuszBuildInfo struct {
	BinaryCommit string `json:"binary_commit"`
	BinaryDirty  bool   `json:"binary_dirty"`
	BuiltAt      string `json:"built_at,omitempty"`
}

// statuszBuildInfo reads the running daemon binary's VCS stamp from
// internal/buildinfo. An unstamped binary (built without -buildvcs) yields
// an empty commit, which readers treat as "unknown", never as fresh.
func statuszBuildInfo() StatuszBuildInfo {
	st := buildinfo.Read()
	return StatuszBuildInfo{BinaryCommit: st.Revision, BinaryDirty: st.Dirty, BuiltAt: st.BuiltAt}
}

// StatuszTypedEdges is one service's typed Go call-edge pass outcome as of
// the last pass recorded for it (BP-35, internal/fusion/typedenv.go): by
// this daemon process, or — Persisted — by an earlier one, replayed from
// the store (DG-10). Omitted entirely when no pass is recorded anywhere —
// which readers must treat as "unknown", never as a pass: the whole defect
// class here is a degradation that reads healthy. (A service with no Go
// module never records one; the page says so, the v3 JSON does not.)
type StatuszTypedEdges struct {
	Packages int    `json:"packages"`
	Failed   bool   `json:"failed"`
	Reason   string `json:"reason,omitempty"`
	// Persisted/RecordedAt (finding [4]/DG-10) name a store-replayed
	// (pre-restart) answer distinctly from one this daemon process just
	// computed -- omitted (false/zero) for a fresh in-process result.
	Persisted  bool   `json:"persisted,omitempty"`
	RecordedAt string `json:"recorded_at,omitempty"`
}

// StatuszServiceRow is one indexed service's freshness + last-compile
// summary.
type StatuszServiceRow struct {
	Service       string `json:"service"`
	IndexedCommit string `json:"indexed_commit,omitempty"`
	FreshnessNote string `json:"freshness_note,omitempty"`
	// TypedEdges is nil when no typed-edge pass is recorded for this
	// service — neither by this daemon process nor in the store (DG-10's
	// fallback) — see StatuszTypedEdges.
	TypedEdges *StatuszTypedEdges `json:"typed_edges,omitempty"`
	// TypedEdgesAbsence says why TypedEdges is nil, for the page only: the
	// pass does not apply (no Go module at the service root, so
	// enrichTypedEdges never runs and never records), or no pass was ever
	// recorded, or whether it applies could not be checked. json:"-" keeps
	// the v3 contract byte-for-byte (OC-6).
	TypedEdgesAbsence statuszTypedAbsence `json:"-"`
	// LastCompilePerLayer maps layer name -> a human-readable summary of
	// its most recent compile_runs row ("ran_at, model, cost").
	LastCompilePerLayer map[string]string `json:"last_compile_per_layer,omitempty"`
}

// StatuszReport is /statusz's payload — both the JSON and HTML renderings
// come from the exact same value.
type StatuszReport struct {
	SchemaVersion int    `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
	StatuszBuildInfo
	UptimeSec    float64             `json:"uptime_seconds"`
	ChannelsNote string              `json:"channels_note"`
	Services     []StatuszServiceRow `json:"services"`
	// Quarantined names every service the exit rung quarantined (RT-155):
	// tenancy.Quarantine.Attention's line. Omitted when none is.
	Quarantined []string `json:"quarantined,omitempty"`
	// Tenancy is the serving daemon's ladder (design §2, §13.3; RT-32):
	// level, marks, the last sample and every reading that is unknown —
	// "footprint unknown (ladder off)" included. Omitted where no governor
	// runs (a CLI verb).
	Tenancy *governorSnapshot `json:"tenancy,omitempty"`
}

// buildStatuszReport is the pure-given-its-client core: it reads whatever
// the already-open client exposes (Services, ServiceMeta, CompileRunsFor)
// — no new store connection, no daemon round trip beyond what the caller
// already has open. Channel liveness reuses fathomChannelsDisabled's OWN
// env read — the SAME honesty caveat checkChannelsEnv already carries:
// this reports the SERVER process's own environment, not the harness's
// (the harness's hooks run in a different process entirely).
func buildStatuszReport(ctx context.Context, c *client.Client, startTime, now time.Time) StatuszReport {
	r := StatuszReport{
		SchemaVersion:    statuszSchemaVersion,
		GeneratedAt:      now.UTC().Format(time.RFC3339),
		StatuszBuildInfo: statuszBuildInfo(),
		UptimeSec:        now.Sub(startTime).Seconds(),
		Quarantined:      statuszQuarantined(),
	}
	if g := activeGovernor.Load(); g != nil {
		snap := g.snapshot()
		r.Tenancy = &snap
	}
	if fathomChannelsDisabled() {
		r.ChannelsNote = "FATHOM_CHANNELS=off in this SERVER process's own environment — this does not necessarily reflect the harness's own hook environment"
	} else {
		r.ChannelsNote = "FATHOM_CHANNELS is not \"off\" in this server process's own environment"
	}

	services, err := c.Services(ctx)
	if err != nil {
		return r
	}
	sort.Strings(services)
	for _, svc := range services {
		row := StatuszServiceRow{Service: svc}
		servicePath := ""
		if path, commit, found, merr := c.ServiceMeta(ctx, svc); merr == nil && found {
			row.IndexedCommit = commit
			servicePath = path
		} else {
			row.FreshnessNote = "not measurable: no ServiceMeta recorded for this service"
		}
		if st, known := c.TypedEdgeStatus(svc); known {
			row.TypedEdges = &StatuszTypedEdges{Packages: st.Packages, Failed: st.Failed, Reason: st.Reason}
			if st.Persisted {
				row.TypedEdges.Persisted = true
				row.TypedEdges.RecordedAt = st.RecordedAt.UTC().Format(time.RFC3339)
			}
		} else {
			row.TypedEdgesAbsence = statuszTypedEdgesAbsence(servicePath)
		}
		if runs, rerr := c.CompileRunsFor(ctx, svc); rerr == nil && len(runs) > 0 {
			// runs is newest-first (Store.CompileRuns' own contract) —
			// the FIRST run seen per layer is that layer's most recent.
			seen := map[string]bool{}
			perLayer := map[string]string{}
			for _, run := range runs {
				if seen[run.Layer] {
					continue
				}
				seen[run.Layer] = true
				perLayer[run.Layer] = fmt.Sprintf("ran_at=%s model=%s cost=$%.4f",
					run.RanAt.UTC().Format(time.RFC3339), run.Model, run.CostUSD)
			}
			row.LastCompilePerLayer = perLayer
		}
		r.Services = append(r.Services, row)
	}
	return r
}

// statuszTypedAbsence is why a service has no typed-edge status.
type statuszTypedAbsence struct {
	// NotApplicable is true when the pass cannot run for this service: no
	// go.mod at its root (enrichTypedEdges returns before recording
	// anything, so no status will ever exist). Not an unknown.
	NotApplicable bool
	// Reason says it in words, for either case.
	Reason string
}

// statuszTypedEdgesAbsence classifies a service with no recorded typed-edge
// pass. The pass only runs for a service whose root holds a go.mod
// (internal/fusion/typedcalls.go enrichTypedEdges); without one it never
// records a status, so "unknown" would be false forever. A root that cannot
// be read stays unknown, with the read error named (ADR-027: an unreadable
// source never reads as an answer).
func statuszTypedEdgesAbsence(root string) statuszTypedAbsence {
	if root == "" {
		return statuszTypedAbsence{Reason: "no service root is recorded, so whether the Go pass applies cannot be checked"}
	}
	_, err := os.Stat(filepath.Join(root, "go.mod"))
	switch {
	case err == nil:
		return statuszTypedAbsence{Reason: "no typed-edge pass is recorded for this service — not by this daemon, not in the store"}
	case os.IsNotExist(err):
		return statuszTypedAbsence{NotApplicable: true, Reason: "does not apply — no go.mod at the service root, so the typed Go pass never runs"}
	}
	return statuszTypedAbsence{Reason: "the service root could not be read (" + err.Error() + "), so whether the Go pass applies is unknown"}
}

// statuszBinaryLabel renders the build stamp for the HTML view: the commit
// (short), "-dirty" when built from a modified tree, or "unstamped".
func statuszBinaryLabel(b StatuszBuildInfo) string {
	if b.BinaryCommit == "" {
		return "unstamped (built without -buildvcs)"
	}
	label := b.BinaryCommit
	if len(label) > 12 {
		label = label[:12]
	}
	if b.BinaryDirty {
		label += "-dirty"
	}
	if b.BuiltAt != "" {
		label += " built " + b.BuiltAt
	}
	return label
}
