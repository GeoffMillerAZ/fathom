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
// Tool/Surface/Arm string values are the caller's responsibility to
// constrain to labelPattern (or empty, for Surface) BEFORE they land in
// this struct — see cmd/fathom/research_bundle.go's sanitizeLabel and its
// own doc comment for why (critic Finding 1: these are client-controlled
// strings, not fathom-controlled ones, despite being typed as plain Go
// strings here).
//
// Callers (cmd/fathom/research_bundle.go) are responsible for aggregating
// raw local evidence down to this shape BEFORE calling Build; Build itself
// never reads a file, a log, or a store.
type Input struct {
	WindowSince, WindowUntil, GeneratedAt string // RFC3339, caller-computed

	FathomVersion string
	CommitSHA     string
	CommitDirty   bool

	// CallLogRead/HooksLogRead gate whether the CallTraffic+VerifyGate and
	// Injection groups are emitted at all (critic Finding 3): Build must
	// never synthesize an all-zero group for an instrument that was never
	// actually read — that reads as "measured zero" to a recipient who
	// cannot tell the difference. false means "instrument absent," and the
	// corresponding scalar fields below are ignored entirely regardless of
	// their value.
	CallLogRead  bool
	HooksLogRead bool

	TotalCalls           int
	SurfaceTraffic       []SurfaceCount
	ToolCalls            []ToolCount
	LatencyP50MS         *float64
	LatencyP95MS         *float64
	StalenessReportCalls int

	InjectionSeen, InjectionAnswered int
	InjectionCoverageRate            *float64
	ObligationsServed                int

	VerifyClaimed, VerifyVerified, VerifyFailing           int
	VerifyInstrumentedCalls, VerifyPredatesInstrumentation int
	ReviewActiveFindings                                   int

	TotalObservations, OpenObservations *int

	// ServicePlaintextCalls maps a plaintext service/repo ID to its call
	// count in the window — the one field in Input that carries something
	// identifying. Build hashes every key with Salt immediately and returns
	// only the hash+count pairs (ServiceActivity); the plaintext keys and
	// the salt itself never reach the returned Bundle.
	ServicePlaintextCalls map[string]int
	Salt                  string

	BenchArms []BenchArmCount

	// RetrosRead gates the Retros group (IR-5) exactly as CallLogRead
	// gates CallTraffic: false means the corpus was not read (the flag was
	// not passed, or no corpus directory exists) and Retros is ignored.
	RetrosRead bool
	Retros     RetroStats
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

		TotalObservations: in.TotalObservations,
		OpenObservations:  in.OpenObservations,

		BenchArms: append([]BenchArmCount(nil), in.BenchArms...),
	}

	if in.CallLogRead {
		b.CallTraffic = &CallTrafficStats{
			TotalCalls:           in.TotalCalls,
			SurfaceTraffic:       append([]SurfaceCount(nil), in.SurfaceTraffic...),
			ToolCalls:            append([]ToolCount(nil), in.ToolCalls...),
			LatencyP50MS:         in.LatencyP50MS,
			LatencyP95MS:         in.LatencyP95MS,
			StalenessReportCalls: in.StalenessReportCalls,
		}
		sort.Slice(b.CallTraffic.SurfaceTraffic, func(i, j int) bool {
			return b.CallTraffic.SurfaceTraffic[i].Surface < b.CallTraffic.SurfaceTraffic[j].Surface
		})
		sort.Slice(b.CallTraffic.ToolCalls, func(i, j int) bool {
			return b.CallTraffic.ToolCalls[i].Tool < b.CallTraffic.ToolCalls[j].Tool
		})

		b.VerifyGate = &VerifyGateStats{
			Claimed:                 in.VerifyClaimed,
			Verified:                in.VerifyVerified,
			Failing:                 in.VerifyFailing,
			InstrumentedCalls:       in.VerifyInstrumentedCalls,
			PredatesInstrumentation: in.VerifyPredatesInstrumentation,
			ReviewActiveFindings:    in.ReviewActiveFindings,
		}
	}

	if in.HooksLogRead {
		b.Injection = &InjectionStats{
			Seen:              in.InjectionSeen,
			Answered:          in.InjectionAnswered,
			CoverageRate:      in.InjectionCoverageRate,
			ObligationsServed: in.ObligationsServed,
		}
	}

	if len(in.ServicePlaintextCalls) > 0 {
		rows := make([]ServiceHashCount, 0, len(in.ServicePlaintextCalls))
		for svc, calls := range in.ServicePlaintextCalls {
			rows = append(rows, ServiceHashCount{ServiceHash: HashService(in.Salt, svc), Calls: calls})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].ServiceHash < rows[j].ServiceHash })
		b.ServiceActivity = rows
	}

	sort.Slice(b.BenchArms, func(i, j int) bool { return b.BenchArms[i].Arm < b.BenchArms[j].Arm })

	if in.RetrosRead {
		r := in.Retros
		r.Outcomes = append([]RetroOutcomeCount(nil), in.Retros.Outcomes...)
		sort.Slice(r.Outcomes, func(i, j int) bool {
			if r.Outcomes[i].Channel != r.Outcomes[j].Channel {
				return r.Outcomes[i].Channel < r.Outcomes[j].Channel
			}
			return r.Outcomes[i].Outcome < r.Outcomes[j].Outcome
		})
		b.Retros = &r
	}

	return b
}
