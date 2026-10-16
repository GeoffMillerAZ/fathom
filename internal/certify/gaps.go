package certify

// gaps.go — Gaps (TC-11, design doc §5): the target universe `fathom
// certify gaps` lists — every declared certification target (the
// certify_declarations table, TC-5), every invariant row and every
// incident record in the intent corpus (internal/intent) — each with its
// compiled certifications-layer certificates and a remedy line naming the
// marker or manifest field to add when it has no live one.
//
// Deliberately NOT sourced here (design doc §5 lists it for a later
// stage; the plan row scopes TC-11 to declared + invariant + incident
// only): every spec id declared in plans (still deferred). TC-20 (port
// gaps) and TC-19 (incident-outcome certificates) are both wired below —
// TC-20 extends the target universe itself (port.go's own PortGaps);
// TC-19 changes how an "incident" target's liveness is DECIDED, not
// what target universe is sourced.
import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"

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
	// Source is where the target itself is declared — a `// certifies:`
	// marker's own position, an invariant row, a port interface's
	// declaration, or an incident heading — when Gaps could place it
	// cheaply from data it already read. Zero value (File == "") when no
	// source names a location for this target's kind (RK-36 anchors: a
	// caller must never invent one).
	Source Source
}

// Source is a target's own declared location (repo-relative File, 1-based
// Line; Line 0 means "the file, no more specific line").
type Source struct {
	File string
	Line int
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
	// Unread names every source the target universe is read from (the
	// intent corpus, incidents.md, a port's source file) that exists and
	// could not be read, with the operating system's reason (I38). When
	// it is non-empty the targets are a lower bound, never the universe:
	// an unreadable incidents.md once made every incident target vanish
	// without a word, and a wholly unreadable repository read as "nothing
	// declared". A caller must say the counts are a lower bound whenever
	// it is non-empty (ADR-027).
	Unread []string
}

// incidentHeadingPath is docs/intent/incidents.md's own repo-relative
// path — the one and only place `Gaps` sources an "incident" target from,
// so citing it as that target's Source.File is never a guess.
const incidentHeadingPath = "docs/intent/incidents.md"

// incidentHeading is one "### INC-N" heading incidentHeadings found: its
// id and the 1-based line the heading itself sits on.
type incidentHeading struct {
	id   string
	line int
}

// incidentHeadings reads corpusRoot's docs/intent/incidents.md (when
// present) and returns every "### INC-N" heading's id and line, sorted by
// id — the SAME incidentHeadingRe resolveIncident (resolve.go) already
// uses to resolve one id, walked here for every id the file names. A
// missing file yields none (intent.Load itself never treats absence as an
// error); an unreadable one yields none too, and reads records why, so
// Gaps names it rather than reporting fewer incidents (I38). The line is
// derived from the match's own byte offset (bytes.Count of newlines
// before it) — free given the file is already read and matched here.
func incidentHeadings(corpusRoot string, reads *readmanifest.Set) []incidentHeading {
	body, ok := reads.ReadFile(filepath.Join(corpusRoot, "docs", "intent", "incidents.md"))
	if !ok {
		return nil
	}
	var out []incidentHeading
	for _, m := range incidentHeadingRe.FindAllSubmatchIndex(body, -1) {
		id := string(body[m[2]:m[3]])
		line := bytes.Count(body[:m[0]], []byte("\n")) + 1
		out = append(out, incidentHeading{id: id, line: line})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
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
	sources := map[key]Source{}
	var order []key
	// add records k in declaration order (first add wins, exactly like
	// seen already did) and, when src names a real location and none was
	// recorded yet for k, its Source — so a target sourced from more than
	// one place (e.g. a certifies marker declaring "invariant I5", which
	// also has its own row in invariants.md) keeps whichever location was
	// offered FIRST, matching add's own existing "first considered wins"
	// rule for everything else about the target.
	add := func(kind, id string, src Source) {
		if !include(kind) {
			return
		}
		k := key{kind, id}
		if seen[k] {
			return
		}
		seen[k] = true
		order = append(order, k)
		if src.File != "" {
			sources[k] = src
		}
	}

	reads := readmanifest.NewSet()
	reads.RelativeTo(corpusRoot)

	decls, err := st.CertifyDeclarationsForService(ctx, service)
	if err != nil {
		return GapReport{}, fmt.Errorf("Gaps: CertifyDeclarationsForService: %w", err)
	}
	for _, d := range decls {
		add(d.Kind, d.TargetID, Source{File: d.File, Line: d.Line})
	}
	if corpusRoot != "" {
		if corpus, cerr := intent.Load(corpusRoot); cerr == nil {
			reads.Merge(corpus.Coverage)
			for _, inv := range corpus.Invariants {
				add("invariant", inv.ID, Source{File: inv.File, Line: inv.Line})
			}
		} else {
			reads.Record(filepath.Join(corpusRoot, "docs", "intent"), readmanifest.Unreadable(cerr))
		}
		for _, h := range incidentHeadings(corpusRoot, reads) {
			add("incident", h.id, Source{File: incidentHeadingPath, Line: h.line})
		}
	}

	// TC-20 (design doc §5): every exported interface with at least one
	// indexed implementation is a "port" target, sourced from the index
	// itself rather than from CertifyDeclarationsForService/intent.Load —
	// there is nothing to declare a port target's EXISTENCE against, only
	// whether it carries a conformance certificate.
	portImpls := map[string][]PortImplementation{}
	if corpusRoot != "" && include("port") {
		ports, perr := portGaps(ctx, st, service, corpusRoot, reads)
		if perr != nil {
			// The port scan itself failed: its targets are unknown and
			// named so, never read as a repository with no ports (I38).
			reads.Record("port targets", readmanifest.Unreadable(perr))
		}
		for _, p := range ports {
			add("port", p.ID, Source{File: p.File, Line: p.Line})
			portImpls[p.ID] = p.Implementations
		}
	}

	claims, err := st.ClaimsForService(ctx, service, Layer)
	if err != nil {
		return GapReport{}, fmt.Errorf("Gaps: ClaimsForService: %w", err)
	}
	bySubject := map[string][]Record{}
	byTestName := map[string][]Record{}
	for _, r := range Records(claims) {
		bySubject[r.Subject] = append(bySubject[r.Subject], r)
		byTestName[r.Test] = append(byTestName[r.Test], r)
	}

	// TC-19 (design doc §5): an incident's OWN Outcome: line may name a
	// live certificate independent of anything declared for
	// "incident:<ID>" itself.
	var incidentCerts map[string]string
	if corpusRoot != "" && include("incident") {
		incidentCerts = IncidentCertificates(corpusRoot)
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
			// A "port" target needs specifically a conformance-strength
			// live certificate (design doc §5/§1: "a conformance
			// certificate must sit in a suite of class... naming a
			// port") — a live certificate at a weaker strength still
			// leaves it a gap.
			if r.State == string(StateLive) && (k.kind != "port" || r.Strength == "conformance") {
				hasLive = true
			}
			states = append(states, r.State)
		}

		// TC-19 (design doc §5): an incident closed by its OWN
		// Outcome: line, independent of any certifications-layer
		// record for "incident:<ID>" itself.
		var outcomeTest string
		if k.kind == "incident" && !hasLive {
			if outcomeTest = incidentCerts[k.id]; outcomeTest != "" {
				for _, r := range byTestName[outcomeTest] {
					certs = append(certs, GapCertificate{
						Test: r.Test, Suite: r.Suite, Class: r.Class,
						Strength: r.Strength, State: r.State, Tier: r.Tier, RunDigest: r.RunDigest,
					})
					if r.State == string(StateLive) {
						hasLive = true
					}
					states = append(states, r.State)
				}
			}
		}

		t := GapTarget{Kind: k.kind, ID: k.id, Gap: !hasLive, Certificates: certs, Source: sources[k]}
		if t.Gap {
			gaps++
			switch {
			case k.kind == "port":
				t.Remedy = portRemedy(k.id, portImpls[k.id])
			case k.kind == "incident" && outcomeTest != "":
				if len(states) == 0 {
					t.Remedy = fmt.Sprintf("Outcome: names certificate %s, but no certifications-layer record exists for it at all — declare it with a \"// certifies:\" marker and attest a passing run", outcomeTest)
				} else {
					t.Remedy = fmt.Sprintf("Outcome: names certificate %s, but it has no live certifications-layer record (declared state(s): %s) — fix or re-attest %s", outcomeTest, strings.Join(states, ", "), outcomeTest)
				}
			case len(certs) == 0:
				t.Remedy = markerRemedy(k.kind, k.id)
			default:
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

	var unread []string
	for _, e := range reads.Manifest().Unreadable() {
		unread = append(unread, e.Path+": "+e.Outcome.Detail)
	}
	return GapReport{
		Targets: targets,
		Summary: GapSummary{
			Declared: len(targets), Live: live, Gaps: gaps,
			JoinedCases: casesJoined, Cases: casesTotal,
		},
		Unread: unread,
	}, nil
}
