package certify

// gaps.go — Gaps (TC-11, design doc §5): the target universe `fathom
// certify gaps` lists — every declared certification target (the
// certify_declarations table, TC-5), every invariant row and every
// incident record in the intent corpus (internal/intent) — each with its
// compiled certifications-layer certificates and a remedy line naming the
// marker or manifest field to add when it has no live one.
//
// Deliberately NOT sourced here (design doc §5 lists them for a later
// stage; the plan row scopes TC-11 to declared + invariant + incident
// only): every spec id declared in plans, every exported interface (kind
// "port"). A later worker adding TC-19 (incident-outcome certificates) or
// TC-20 (port gaps) extends the target universe and GapCertificate's own
// fields additively — the shape here is deliberately not tied to any one
// kind's resolution.
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// GapCertificate is one compiled certifications-layer claim served on a
// GapTarget — the Review Kit adapter's own certificate shape (the fixed
// field set docs/design/test-certifications.md's consumer, platform-
// engineering's review-kit, parses).
type GapCertificate struct {
	Test      string
	Suite     string
	Class     string
	Strength  string
	State     string
	Tier      string
	RunDigest string
}

// GapTarget is one target `certify gaps` considered. Remedy is non-empty
// exactly when Gap is true (never the reverse): a target WITH a live
// certificate carries no remedy to print. Severity is reserved ("" in
// this stage — fathom asserts liveness, never adequacy or priority; see
// ADR-043 decision 3) for a later worker to populate from a source that
// actually carries one (e.g. an invariant's own enforcement rung).
type GapTarget struct {
	Kind         string
	ID           string
	Severity     string
	Gap          bool
	Remedy       string
	Certificates []GapCertificate
}

// GapSummary totals GapReport's own targets and the service-wide
// case-to-symbol join rate (TestSuiteStatuses' own fact, TC-3/TC-4).
// Declared here means "targets considered" (Live + Gaps), matching the
// Review Kit adapter's schema — not "targets with at least one
// certificate declared" (a target sourced only from the intent corpus,
// with nothing ever declared for it, still counts, and is a Gap).
type GapSummary struct {
	Declared    int
	Live        int
	Gaps        int
	JoinedCases int
	Cases       int
}

// GapReport is Gaps' full return.
type GapReport struct {
	Targets []GapTarget
	Summary GapSummary
}

// incidentIDs reads corpusRoot's docs/intent/incidents.md (when present)
// and returns every "### INC-N" heading's id, sorted — the SAME
// incidentHeadingRe resolveIncident (resolve.go) already uses to resolve
// one id, walked here for every id the file names. A missing or
// unreadable file yields none — the SAME "nothing to report" honesty an
// absent docs/intent/ already carries throughout this package (intent.Load
// itself never treats absence as an error).
func incidentIDs(corpusRoot string) []string {
	body, err := os.ReadFile(filepath.Join(corpusRoot, "docs", "intent", "incidents.md"))
	if err != nil {
		return nil
	}
	var ids []string
	for _, m := range incidentHeadingRe.FindAllStringSubmatch(string(body), -1) {
		ids = append(ids, m[1])
	}
	sort.Strings(ids)
	return ids
}

// markerRemedy is the remedy line for a target with NO certificate
// declared at all: the exact marker grammar (design doc §1) or the
// suite-level manifest field, either satisfies it.
func markerRemedy(kind, id string) string {
	return fmt.Sprintf(
		"add \"// certifies: %s %s (asserts)\" above a Test* function, or a #Certifies entry naming %s %s in the suite's tests/*.cue manifest",
		kind, id, kind, id,
	)
}

// Gaps computes the target universe (declared certifications, invariant
// rows, incident records — filtered to kinds when non-empty), each with
// its compiled certificates and remedy, plus the service-wide summary.
// corpusRoot is the service's indexed path (ServiceMeta.Path); "" degrades
// the invariant/incident sources to none without failing the call (the
// SAME best-effort discipline the review-input builders already use per
// file — TC-10's own wiring).
func Gaps(ctx context.Context, st store.Store, service, corpusRoot string, kinds []string) (GapReport, error) {
	want := toSet(kinds)
	include := func(kind string) bool { return len(want) == 0 || want[kind] }

	type key struct{ kind, id string }
	seen := map[key]bool{}
	var order []key
	add := func(kind, id string) {
		if !include(kind) {
			return
		}
		k := key{kind, id}
		if seen[k] {
			return
		}
		seen[k] = true
		order = append(order, k)
	}

	decls, err := st.CertifyDeclarationsForService(ctx, service)
	if err != nil {
		return GapReport{}, fmt.Errorf("Gaps: CertifyDeclarationsForService: %w", err)
	}
	for _, d := range decls {
		add(d.Kind, d.TargetID)
	}
	if corpusRoot != "" {
		if corpus, cerr := intent.Load(corpusRoot); cerr == nil {
			for _, inv := range corpus.Invariants {
				add("invariant", inv.ID)
			}
		}
		for _, id := range incidentIDs(corpusRoot) {
			add("incident", id)
		}
	}

	claims, err := st.ClaimsForService(ctx, service, Layer)
	if err != nil {
		return GapReport{}, fmt.Errorf("Gaps: ClaimsForService: %w", err)
	}
	bySubject := map[string][]Record{}
	for _, r := range Records(claims) {
		bySubject[r.Subject] = append(bySubject[r.Subject], r)
	}

	targets := make([]GapTarget, 0, len(order))
	var live, gaps int
	for _, k := range order {
		subject := k.kind + ":" + k.id
		recs := append([]Record(nil), bySubject[subject]...)
		sort.Slice(recs, func(i, j int) bool { return recs[i].Test < recs[j].Test })

		certs := make([]GapCertificate, 0, len(recs))
		hasLive := false
		var states []string
		for _, r := range recs {
			certs = append(certs, GapCertificate{
				Test: r.Test, Suite: r.Suite, Class: r.Class,
				Strength: r.Strength, State: r.State, Tier: r.Tier, RunDigest: r.RunDigest,
			})
			if r.State == string(StateLive) {
				hasLive = true
			}
			states = append(states, r.State)
		}

		t := GapTarget{Kind: k.kind, ID: k.id, Gap: !hasLive, Certificates: certs}
		if t.Gap {
			gaps++
			if len(certs) == 0 {
				t.Remedy = markerRemedy(k.kind, k.id)
			} else {
				t.Remedy = fmt.Sprintf("no live certificate for %s %s: declared state(s) %s — fix the failing/unreachable test or attest a passing run", k.kind, k.id, strings.Join(states, ", "))
			}
		} else {
			live++
		}
		targets = append(targets, t)
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Kind != targets[j].Kind {
			return targets[i].Kind < targets[j].Kind
		}
		return targets[i].ID < targets[j].ID
	})

	casesTotal, casesJoined := 0, 0
	if statuses, serr := st.TestSuiteStatuses(ctx, service); serr == nil {
		for _, s := range statuses {
			casesTotal += s.CasesTotal
			casesJoined += s.CasesJoined
		}
	}

	return GapReport{
		Targets: targets,
		Summary: GapSummary{
			Declared: len(targets), Live: live, Gaps: gaps,
			JoinedCases: casesJoined, Cases: casesTotal,
		},
	}, nil
}
