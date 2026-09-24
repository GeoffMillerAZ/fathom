package main

// certify_gaps.go — "fathom certify gaps" (TC-11, design doc §5): every
// declared certification target, invariant row and incident record with
// no live certificate, each with a remedy line naming the marker or
// manifest field to add. --json emits the versioned
// fathom.certify.gaps/v1 shape the Review Kit adapter (platform-
// engineering's internal/evidence/certify.go) parses — the schema
// string, field names and sort order (kind then id) are a contract, not
// a rendering choice: changing any of them breaks that adapter.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// certifyGapsSchema is the Review Kit adapter's exact schema string.
// Bumping it is a breaking change to that adapter's contract.
const certifyGapsSchema = "fathom.certify.gaps/v1"

func cmdCertifyGaps(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("certify gaps", stderr)
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	kind := fs.String("kind", "", "filter to one target kind: invariant|incident|spec|port|rule|behaviour")
	jsonOut := fs.Bool("json", false, "emit the versioned fathom.certify.gaps/v1 JSON shape (the Review Kit adapter's contract) instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
		if svc == "" {
			fmt.Fprintln(stderr, "error: certify gaps requires --service (could not guess one from the current directory)")
			return 1
		}
	}

	var kinds []string
	if *kind != "" {
		kinds = []string{*kind}
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	ctx := context.Background()

	corpusRoot := ""
	indexCommit := ""
	if meta, found, merr := st.GetServiceMeta(ctx, svc); merr == nil && found {
		corpusRoot = meta.Path
		indexCommit = meta.IndexedCommit
	}

	report, err := certify.Gaps(ctx, st, svc, corpusRoot, kinds)
	if err != nil {
		fmt.Fprintf(stderr, "error: certify gaps for %q: %v\n", svc, err)
		return 1
	}

	if *jsonOut {
		return renderCertifyGapsJSON(stdout, svc, indexCommit, time.Now(), report)
	}
	// Suggestions (TC-24) are advisory only and never touch the versioned
	// fathom.certify.gaps/v1 JSON the Review Kit adapter parses — read and
	// rendered ONLY on the table path, best effort (a missing file means
	// certify suggest has simply never run here).
	renderCertifyGapsTable(stdout, report, loadCertifySuggestions(workspacePath("artifacts/certify-suggestions.jsonl")))
	return 0
}

// certifyGapsPayload/certifyGapsTarget/certifyGapsCert/certifyGapsSummary
// are the fathom.certify.gaps/v1 wire shape, field for field. Every slice
// is initialized non-nil so it marshals as `[]`, never `null` — an
// adapter parsing this JSON with a fixed struct would fail on null where
// it expects an array.
type certifyGapsPayload struct {
	Schema      string              `json:"schema"`
	Service     string              `json:"service"`
	IndexCommit string              `json:"index_commit"`
	GeneratedAt string              `json:"generated_at"`
	Kinds       []string            `json:"kinds"`
	Targets     []certifyGapsTarget `json:"targets"`
	Summary     certifyGapsSummary  `json:"summary"`
	// Skip names why the target universe is empty because nothing was
	// ever ingested/declared here — never set for a genuinely-checked
	// service that simply has zero targets right now. Mirrors `certify
	// status --json`'s and the doctor row's own "no manifests found"
	// skip (never a silent pass): without this, a never-ingested service
	// and a checked, zero-gap one both serialize to an empty `targets`
	// array, and the Review Kit adapter treats empty targets as an
	// outright pass. Additive and omitempty — the schema string, field
	// names and sort order stay exactly what that adapter parses.
	Skip string `json:"skip,omitempty"`
}

type certifyGapsTarget struct {
	Kind         string            `json:"kind"`
	ID           string            `json:"id"`
	Severity     string            `json:"severity"`
	Gap          bool              `json:"gap"`
	Remedy       string            `json:"remedy"`
	Certificates []certifyGapsCert `json:"certificates"`
}

type certifyGapsCert struct {
	Test      string `json:"test"`
	Suite     string `json:"suite"`
	Class     string `json:"class"`
	Strength  string `json:"strength"`
	State     string `json:"state"`
	Tier      string `json:"tier"`
	RunDigest string `json:"run_digest"`
}

type certifyGapsSummary struct {
	Declared    int `json:"declared"`
	Live        int `json:"live"`
	Gaps        int `json:"gaps"`
	JoinedCases int `json:"joined_cases"`
	Cases       int `json:"cases"`
}

func renderCertifyGapsJSON(w io.Writer, service, indexCommit string, now time.Time, report certify.GapReport) int {
	kindSet := map[string]bool{}
	targets := make([]certifyGapsTarget, 0, len(report.Targets))
	for _, t := range report.Targets {
		kindSet[t.Kind] = true
		certs := make([]certifyGapsCert, 0, len(t.Certificates))
		for _, c := range t.Certificates {
			certs = append(certs, certifyGapsCert{
				Test: c.Test, Suite: c.Suite, Class: c.Class,
				Strength: c.Strength, State: c.State, Tier: c.Tier, RunDigest: c.RunDigest,
			})
		}
		targets = append(targets, certifyGapsTarget{
			Kind: t.Kind, ID: t.ID, Severity: t.Severity, Gap: t.Gap, Remedy: t.Remedy,
			Certificates: certs,
		})
	}
	kinds := make([]string, 0, len(kindSet))
	for k := range kindSet {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	payload := certifyGapsPayload{
		Schema:      certifyGapsSchema,
		Service:     service,
		IndexCommit: indexCommit,
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Kinds:       kinds,
		Targets:     targets,
		Summary: certifyGapsSummary{
			Declared: report.Summary.Declared, Live: report.Summary.Live, Gaps: report.Summary.Gaps,
			JoinedCases: report.Summary.JoinedCases, Cases: report.Summary.Cases,
		},
	}
	if report.Summary.Declared == 0 {
		// Same condition and wording `certify status` already treats as
		// a named skip, never a pass (cmdCertifyStatus).
		payload.Skip = "no manifests found — nothing declared"
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return 1
	}
	return 0
}

func renderCertifyGapsTable(w io.Writer, report certify.GapReport, suggestions []certifySuggestion) {
	if len(report.Targets) == 0 {
		fmt.Fprintln(w, "certify gaps: no targets found (nothing declared, and no invariant/incident corpus read)")
	} else {
		fmt.Fprintf(w, "%-10s %-24s %-5s %s\n", "KIND", "ID", "GAP", "REMEDY")
		for _, t := range report.Targets {
			gap := "no"
			remedy := ""
			if t.Gap {
				gap = "yes"
				remedy = t.Remedy
			}
			fmt.Fprintf(w, "%-10s %-24s %-5s %s\n", t.Kind, t.ID, gap, remedy)
		}
		fmt.Fprintf(w, "%d target(s), %d live, %d gap(s), joined %d of %d cases\n",
			report.Summary.Declared, report.Summary.Live, report.Summary.Gaps,
			report.Summary.JoinedCases, report.Summary.Cases)
	}
	// TC-24: an unregistered test the decision port judged proceed-band —
	// never a claim, never counted in Summary, purely a hint that a human
	// still has to accept by adding the marker.
	for _, s := range suggestions {
		fmt.Fprintf(w, "suggested: %s may certify something (strength: %s, confidence %.2f, asserts %.2f) — add a \"// certifies:\" marker to accept\n", s.Test, s.Strength, s.Confidence, s.AssertsNoul)
	}
}
