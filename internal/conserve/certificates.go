package conserve

// certificates.go — the "certificates" plane (ADR-044 §2.4): a certifies
// declaration lost between base and head (certify.LostDeclarations), plus
// a compiled certification whose test or reach falls inside the change
// range and whose current State is anything but live — unproven, broken,
// or unreachable — reported with that state's own reason. A "behaviour"
// certificate's own target resolution (CV-22, whether its id names a real
// capability-inventory item) is certify.ResolveTarget's concern at
// COMPILE time, not this plane's: a contradicted-at-compile behaviour
// certificate already shows up here as a non-live state once its test or
// reach is in range, exactly like any other kind.
//
// Registers itself from init() (conserve.go's own doc comment).

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/certify"
)

func init() {
	Register(certificatesPlane{})
}

type certificatesPlane struct{}

func (certificatesPlane) Name() string { return "certificates" }

// Check reports two kinds of finding. "lost_declaration": a certifies
// (kind, id) target declared in a changed Go file at base, undeclared at
// head — certify.LostDeclarations' own pure base/head fact, excused here
// via in.Declarations (I45) exactly like the surface plane's removed/
// changed findings. "certificate_<state>": a compiled certifications-
// layer claim whose test file or reach (Meta.TestFile / the claim's own
// Scope) falls inside in.ChangedFiles, and whose current Meta.State is
// unproven, broken, or unreachable — never live findings, since a live
// certificate is exactly what conservation wants to see and is not a
// defect to report. Unproven (I44) when either Input cannot read file
// content, or in.Store is nil: this plane's whole mechanism needs both.
func (certificatesPlane) Check(ctx context.Context, in Input) ([]Finding, []string) {
	if in.BaseFile == nil || in.HeadFile == nil {
		return nil, []string{"no base/head file reader"}
	}
	if in.Store == nil {
		return nil, []string{"no store"}
	}

	var findings []Finding
	for _, lost := range certify.LostDeclarations(in.Service, in.ChangedFiles, in.BaseFile, in.HeadFile) {
		key := lost.Kind + ":" + lost.ID
		findings = append(findings, Finding{
			Plane: "certificates", Kind: "lost_declaration", Key: key,
			Detail:   fmt.Sprintf("certifies %s declared at base, no longer declared at head", key),
			Declared: in.Declarations.Declared(key),
			Remedy:   fmt.Sprintf("add `retires: %s` to the row that removed it (or restore the certifies marker)", key),
		})
	}

	claims, err := in.Store.ClaimsForService(ctx, in.Service, certify.Layer)
	if err != nil {
		return findings, []string{fmt.Sprintf("certifications layer: %v", err)}
	}
	changed := toSet(in.ChangedFiles)
	for _, c := range claims {
		if len(c.Meta) == 0 {
			continue
		}
		var m certify.Meta
		if jerr := json.Unmarshal(c.Meta, &m); jerr != nil {
			continue
		}
		if !nonLiveState(m.State) {
			continue
		}
		if !changed[m.TestFile] && !anyChanged(changed, c.Scope) {
			continue
		}
		findings = append(findings, Finding{
			Plane: "certificates", Kind: "certificate_" + m.State, Key: c.Subject,
			Detail:   fmt.Sprintf("%s certifies %s: %s", m.Test, c.Subject, m.State),
			Head:     citeOf(m.TestFile, 0),
			Declared: in.Declarations.Declared(c.Subject),
			Remedy:   fmt.Sprintf("re-attest %s (task tests:attest / fathom tests ingest), or fix/retire the certifies declaration", m.Test),
		})
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Key < findings[j].Key
	})
	return findings, nil
}

// nonLiveState reports whether state is one of the certifications layer's
// non-satisfying states (design doc §3: only "live" satisfies a gate).
// "flaky" is deliberately excluded — a flake still carries a passing last
// run and is not itself the kind of defect this plane exists to surface.
func nonLiveState(state string) bool {
	switch certify.State(state) {
	case certify.StateUnproven, certify.StateBroken, certify.StateUnreachable:
		return true
	default:
		return false
	}
}

func anyChanged(changed map[string]bool, files []string) bool {
	for _, f := range files {
		if changed[f] {
			return true
		}
	}
	return false
}
