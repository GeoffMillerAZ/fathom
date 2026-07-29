package bundle

import (
	"runtime"
	"sort"
)

// Input is the ONLY channel by which data reaches Build: every field here
// is already the exact scalar/aggregate value the bundle may carry. Build
// performs no further reduction of free text — there is no code path by
// which a raw map (a call-log row, an observation's symptom text, a file
// path) can reach the output Bundle, because Input simply has no field
// shaped like one (the one apparent exception, ServicePlaintextCalls, is
// consumed and discarded inside Build — see its own doc comment).
//
// Callers (cmd/fathom/research_bundle.go) are responsible for aggregating
// raw local evidence down to this shape BEFORE calling Build; Build itself
// never reads a file, a log, or a store.
type Input struct {
	WindowSince, WindowUntil, GeneratedAt string // RFC3339, caller-computed

	FathomVersion string
	CommitSHA     string
	CommitDirty   bool

	TotalCalls     int
	SurfaceTraffic []SurfaceCount
	ToolCalls      []ToolCount

	LatencyP50MS, LatencyP95MS *float64

	InjectionSeen, InjectionAnswered int
	InjectionCoverageRate            *float64
	ObligationsServed                int

	VerifyClaimed, VerifyVerified, VerifyFailing           int
	VerifyInstrumentedCalls, VerifyPredatesInstrumentation int
	ReviewActiveFindings                                   int

	StalenessReportCalls int

	TotalObservations, OpenObservations *int

	// ServicePlaintextCalls maps a plaintext service/repo ID to its call
	// count in the window — the one field in Input that carries something
	// identifying. Build hashes every key with Salt immediately and returns
	// only the hash+count pairs (ServiceActivity); the plaintext keys and
	// the salt itself never reach the returned Bundle.
	ServicePlaintextCalls map[string]int
	Salt                  string

	BenchArms []BenchArmCount
}

// Build assembles the sanitized Bundle from typed, already-aggregated
// Input. This function is the allowlist boundary
// (docs/design/research-flywheel.md's Privacy section): every field written
// to the returned Bundle is copied from a named Input field or computed
// here from Go's own runtime package — nothing is ever copied from a
// generic map/any value or a free-text field, so a source-data field this
// function doesn't explicitly know about cannot leak through by accident.
// Validate (schema.go) re-checks the result against the CUE allowlist as a
// second, independent guard.
func Build(in Input) Bundle {
	b := Bundle{
		SchemaVersion: SchemaVersionCurrent,

		WindowSince: in.WindowSince,
		WindowUntil: in.WindowUntil,
		GeneratedAt: in.GeneratedAt,

		FathomVersion: in.FathomVersion,
		CommitSHA:     in.CommitSHA,
		CommitDirty:   in.CommitDirty,
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,

		TotalCalls:     in.TotalCalls,
		SurfaceTraffic: append([]SurfaceCount(nil), in.SurfaceTraffic...),
		ToolCalls:      append([]ToolCount(nil), in.ToolCalls...),

		LatencyP50MS: in.LatencyP50MS,
		LatencyP95MS: in.LatencyP95MS,

		InjectionSeen:         in.InjectionSeen,
		InjectionAnswered:     in.InjectionAnswered,
		InjectionCoverageRate: in.InjectionCoverageRate,
		ObligationsServed:     in.ObligationsServed,

		VerifyClaimed:                 in.VerifyClaimed,
		VerifyVerified:                in.VerifyVerified,
		VerifyFailing:                 in.VerifyFailing,
		VerifyInstrumentedCalls:       in.VerifyInstrumentedCalls,
		VerifyPredatesInstrumentation: in.VerifyPredatesInstrumentation,
		ReviewActiveFindings:          in.ReviewActiveFindings,

		StalenessReportCalls: in.StalenessReportCalls,

		TotalObservations: in.TotalObservations,
		OpenObservations:  in.OpenObservations,

		BenchArms: append([]BenchArmCount(nil), in.BenchArms...),
	}

	if len(in.ServicePlaintextCalls) > 0 {
		rows := make([]ServiceHashCount, 0, len(in.ServicePlaintextCalls))
		for svc, calls := range in.ServicePlaintextCalls {
			rows = append(rows, ServiceHashCount{ServiceHash: HashService(in.Salt, svc), Calls: calls})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].ServiceHash < rows[j].ServiceHash })
		b.ServiceActivity = rows
	}

	sort.Slice(b.SurfaceTraffic, func(i, j int) bool { return b.SurfaceTraffic[i].Surface < b.SurfaceTraffic[j].Surface })
	sort.Slice(b.ToolCalls, func(i, j int) bool { return b.ToolCalls[i].Tool < b.ToolCalls[j].Tool })
	sort.Slice(b.BenchArms, func(i, j int) bool { return b.BenchArms[i].Arm < b.BenchArms[j].Arm })

	return b
}
