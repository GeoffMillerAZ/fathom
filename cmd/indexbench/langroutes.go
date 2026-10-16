package main

// langroutes.go — LB-F98 (wave px): `indexbench lang-routes`, lang-oracle's
// measurement of one subject plus its route references. A route pair (a
// route, upstream or mesh rule to the Kubernetes or compose service its
// target host names) is resolved by fusion.ResolveRouteRefs — the resolver
// ingest stores route_ref edges from — over the files ingest's route pass
// reads (fusion.IsRouteCandidate: the measured dialect's files and the
// subject's Kubernetes and compose files, through ingest's own dispatch),
// and scored against the oracle's route_references at its tier into the xref
// family, the union with any cross_references the same oracle scored there
// (langbench.ScoreRouteRefs). The level is langbench's computation; nobody
// types it.
//
// The route fields are bench-only and lang-oracle's strict decode refuses
// them (scoreToolOracle disallows unknown fields), so each oracle file is
// passed to the shared measurement with route_references, route_unresolved
// and route_crosscheck removed.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// routeOracleRef is one oracle route pair: the route entity (path, from)
// and the provider entity (to_path, to) its target (raw) names.
type routeOracleRef struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to"`
	Raw    string `json:"raw"`
}

// routeOracleUnresolved is one oracle route reference that names no
// provider, with the reason (fusion.RouteReasons).
type routeOracleUnresolved struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
}

// routeCrossCheck is a stronger tool's agreement with the oracle on part of
// its answer (istioctl analyze on Istio destination hosts), disclosed in the
// reading's notes, never scored.
type routeCrossCheck struct {
	Tool     string   `json:"tool"`
	Tier     string   `json:"tier"`
	Checked  int      `json:"checked"`
	Agree    int      `json:"agree"`
	Disagree []string `json:"disagree"`
}

// routeOracle is the route half of an oracle file.
type routeOracle struct {
	path       string
	tier       string
	refs       []routeOracleRef
	unresolved []routeOracleUnresolved
	crossCheck *routeCrossCheck
}

// routeFields are the bench-only route fields lang-oracle's decode refuses.
var routeFields = []string{"route_references", "route_unresolved", "route_crosscheck"}

// splitRouteOracle reads an oracle file, returns its route half (nil when
// it carries no route_references) and writes the rest to a file in dir for
// the shared measurement.
func splitRouteOracle(i int, path, dir string) (rest string, ro *routeOracle, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return "", nil, fmt.Errorf("%s: %w", path, err)
	}
	if raw, ok := m["route_references"]; ok {
		ro = &routeOracle{path: path}
		var tier string
		_ = json.Unmarshal(m["tier"], &tier)
		ro.tier = orDefault(tier, "A")
		if err := json.Unmarshal(raw, &ro.refs); err != nil {
			return "", nil, fmt.Errorf("%s: route_references: %w", path, err)
		}
		if u, ok := m["route_unresolved"]; ok {
			if err := json.Unmarshal(u, &ro.unresolved); err != nil {
				return "", nil, fmt.Errorf("%s: route_unresolved: %w", path, err)
			}
		}
		if c, ok := m["route_crosscheck"]; ok {
			ro.crossCheck = &routeCrossCheck{}
			if err := json.Unmarshal(c, ro.crossCheck); err != nil {
				return "", nil, fmt.Errorf("%s: route_crosscheck: %w", path, err)
			}
		}
	}
	for _, f := range routeFields {
		delete(m, f)
	}
	out, err := json.Marshal(m)
	if err != nil {
		return "", nil, err
	}
	rest = filepath.Join(dir, fmt.Sprintf("%d-%s", i, filepath.Base(path)))
	return rest, ro, os.WriteFile(rest, out, 0o644)
}

// routeSubject is one subject's route resolution, from the files ingest's
// route pass reads.
type routeSubject struct {
	files map[string]bool
	only  []string
	res   fusion.RouteResolution
}

// inScope reports whether a route's own file is one the measurement covers:
// -only restricts the measured dialect's files (the route side), never the
// compose and Kubernetes files a route target joins to (LB-F168).
func (rs routeSubject) inScope(path string) bool { return onlyMatch(rs.only, path) }

// resolveSubjectRoutes runs ingest's route reading and fusion.ResolveRouteRefs
// over a subject.
func resolveSubjectRoutes(ctx context.Context, root string, only []string) (routeSubject, error) {
	const service = "langbench"
	rs := routeSubject{files: map[string]bool{}, only: only}
	refs, err := connector.NewLocalConnector().Discover(ctx, core.ServiceRef{ID: service, Path: root})
	if err != nil {
		return rs, fmt.Errorf("discover %s: %w", root, err)
	}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	var files []fusion.DocFile
	for _, ref := range refs {
		if !fusion.IsRouteCandidate(ref.Path) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil {
			return rs, err
		}
		rs.files[ref.Path] = true
		if f, ok := pipe.RouteFileForBench(service, refs, ref.Path, content); ok {
			files = append(files, f)
		}
	}
	rs.res = fusion.ResolveRouteRefs(files)
	return rs, nil
}

// isServiceEntity reports whether a route reference starts at a core
// Service (`Service/<ns>/<name>` or `Service/<name>`): the only side-pass
// reference from a Service is its spec.selector to the workloads it fronts
// (routeref_mesh.go's service_selector, TL-15), never a route host.
func isServiceEntity(from string) bool { return strings.HasPrefix(from, "Service/") }

// scoreRouteOracle scores one oracle's route pairs against the subject's
// resolved routes into sub (the xref family at the weaker of its tier and
// any already scored there, and the route_* counts) and returns the notes
// that disclose what was not scored.
func scoreRouteOracle(ro *routeOracle, rs routeSubject, name string, sub *langbench.Subject) []string {
	var notes []string
	dropped := 0
	var truth []langbench.XrefPair
	for _, r := range ro.refs {
		if !rs.inScope(r.Path) || !rs.files[r.Path] || !rs.files[r.ToPath] {
			dropped++
			continue
		}
		truth = append(truth, langbench.XrefPair{Path: r.Path, From: r.From, ToPath: r.ToPath, To: r.To})
	}
	var found []langbench.XrefPair
	selectorJoins, selectorUnmatched := 0, 0
	for _, r := range rs.res.Refs {
		if !rs.inScope(r.FromFile) {
			continue
		}
		if r.Via == fusion.RouteSelectorStrategy && isServiceEntity(r.From) {
			selectorJoins++
			continue
		}
		found = append(found, langbench.XrefPair{Path: r.FromFile, From: r.From, ToPath: r.ToFile, To: r.To})
	}
	langbench.ScoreRouteRefs(truth, found, sub.Counts)
	sub.Families[langbench.FamilyXref] = langbench.WeakerTier(sub.Families[langbench.FamilyXref], ro.tier)
	for _, u := range rs.res.Unresolved {
		if rs.inScope(u.FromFile) {
			if isServiceEntity(u.From) {
				selectorUnmatched++
				continue
			}
			sub.Counts[langbench.CntRouteUnresolvedPrefix+u.Reason]++
		}
	}
	if selectorJoins+selectorUnmatched > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d core Service selector route references (observed-by's hop, TL-15) and %d that select no workload the corpus holds (O1049) are not route pairs: the same join is the dialect's label-selector cross-reference, graded in the xref family (K19, LB-F551), so it is scored once there", name, selectorJoins, selectorUnmatched))
	}
	for _, u := range ro.unresolved {
		if rs.inScope(u.Path) && rs.files[u.Path] {
			sub.Counts[langbench.CntRouteOracleUnresolvedPrefix+u.Reason]++
		}
	}
	if dropped > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d oracle route references on files the route pass does not read were left out of scope (§1.4)", name, dropped))
	}
	notes = append(notes, noteRouteRefs)
	if c := ro.crossCheck; c != nil && c.Checked > 0 {
		note := fmt.Sprintf("%s: route cross-check by %s (tier %s, not scored): %d of %d checked route targets agree with the oracle", name, c.Tool, c.Tier, c.Agree, c.Checked)
		if len(c.Disagree) > 0 {
			note += "; disagree: " + strings.Join(c.Disagree, ", ")
		}
		notes = append(notes, note)
	}
	return notes
}

// noteRouteRefs discloses what the route counts compare.
const noteRouteRefs = "route references (route_* counts, LB-F98) are scored into xref_precision and xref_recall with the oracle's cross_references: a route, upstream or mesh rule's entity to the Kubernetes Service or compose service its target host names, resolved by fusion.ResolveRouteRefs (the resolver ingest stores route_ref edges from) over the files ingest's route pass reads; route_unresolved_<reason> counts fathom's unresolved references, route_oracle_unresolved_<reason> the oracle's"

// cmdLangRoutes is `indexbench lang-routes` (LB-F98).
func cmdLangRoutes(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-routes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dialect := fs.String("dialect", "", "dialect id whose reading takes the route pairs (required)")
	manifestPath := fs.String("manifest", "", "the dialect's dialect.json (default: internal/extractor/dialects/<id>/dialect.json, as lang-oracle)")
	subject := fs.String("subject", "", "subject checkout to measure (required)")
	subjectName := fs.String("subject-name", "", "subject name recorded in the reading (default: the directory name)")
	ctagsPath := fs.String("ctags", "", "universal-ctags binary (as lang-oracle)")
	var toolOracles multiFlag
	fs.Var(&toolOracles, "tool-oracle", "oracle JSON in the neutral shape; its route_references and route_unresolved are the route truth (repeatable)")
	only := fs.String("only", "", "comma-separated globs restricting the subject's files (as lang-oracle)")
	out := fs.String("out", "", "reading to write; an existing reading of the same dialect is merged (required)")
	queuePath := fs.String("queue", "docs/plans/language-breadth.queue.json", "the queue (for the dialect's target level)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "measured_at")
	commit := fs.String("commit", "", "fathom commit measured (default: git rev-parse --short HEAD)")
	labelJoins := fs.String("label-joins", "", "the kubernetes oracle's -labeljoins-out truth: with -label-joins-out, the Stage A static reading (langroutes_telemetry.go)")
	labelJoinsOut := fs.String("label-joins-out", "", "where the Stage A static reading goes (docs/bench/readings/telemetry/stage-a-static.json)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dialect == "" || *subject == "" || *out == "" || len(toolOracles) == 0 || fs.NArg() != 0 || (*labelJoins == "") != (*labelJoinsOut == "") {
		fmt.Fprintln(stderr, "usage: indexbench lang-routes -dialect <id> -subject <dir> -tool-oracle <json> -out <reading.json>")
		return 2
	}
	tmp, err := os.MkdirTemp("", "lang-routes-")
	if err != nil {
		fmt.Fprintln(stderr, "lang-routes:", err)
		return 1
	}
	defer os.RemoveAll(tmp)
	var rest []string
	var routes []*routeOracle
	for i, p := range toolOracles {
		r, ro, err := splitRouteOracle(i, p, tmp)
		if err != nil {
			fmt.Fprintln(stderr, "lang-routes:", err)
			return 1
		}
		rest = append(rest, r)
		if ro != nil {
			routes = append(routes, ro)
		}
	}
	if len(routes) == 0 {
		fmt.Fprintln(stderr, "lang-routes: no -tool-oracle carries route_references")
		return 2
	}

	spec, source, err := resolveBenchSpec(*dialect, *manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "lang-routes:", err)
		return 2
	}
	target := spec.Manifest.TargetLevel
	if q, qerr := langbench.LoadQueue(*queuePath); qerr == nil {
		for _, e := range q {
			if e.ID == spec.Manifest.ID {
				target = e.TargetLevel
			}
		}
	}
	if target == "" {
		target = "L1"
	}
	ctx := context.Background()
	if *commit == "" {
		*commit = gitOut(ctx, ".", "rev-parse", "--short", "HEAD")
	}
	name := *subjectName
	if name == "" {
		name = filepath.Base(filepath.Clean(*subject))
	}
	var onlyGlobs []string
	for _, g := range strings.Split(*only, ",") {
		if g = strings.TrimSpace(g); g != "" {
			onlyGlobs = append(onlyGlobs, g)
		}
	}
	// The dialect's own extractor, as lang-oracle measures it (a native
	// dialect with no stand-alone extractor is measured through dispatch).
	var with extractor.Extractor
	if source == "dialect" {
		x, xerr := dialectExtractor(spec.Manifest)
		switch {
		case xerr == nil:
			with = x
			if c, ok := x.(interface{ Close() }); ok {
				defer c.Close()
			}
		case spec.Manifest.Engine != "native":
			fmt.Fprintln(stderr, "lang-routes:", xerr)
			return 1
		}
	}
	m, err := measureSubject(ctx, spec, *subject, name, oracleOpts{ctags: *ctagsPath, tools: rest, only: onlyGlobs}, with)
	if err != nil {
		fmt.Fprintln(stderr, "lang-routes:", err)
		return 1
	}
	rs, err := resolveSubjectRoutes(ctx, *subject, onlyGlobs)
	if err != nil {
		fmt.Fprintln(stderr, "lang-routes:", err)
		return 1
	}
	for _, ro := range routes {
		m.notes = append(m.notes, scoreRouteOracle(ro, rs, name, &m.subject)...)
	}
	if *labelJoins != "" {
		sa, err := writeStageA(*labelJoins, *labelJoinsOut, name, *date, *commit, rs.res)
		if err != nil {
			fmt.Fprintln(stderr, "lang-routes: stage A:", err)
			return 1
		}
		fmt.Fprintf(stdout, "stage A static: %s: label_join_recall %.4f (joinable %d of %d expressions, recall there %.4f) label_join_precision %.4f (%d of %d joins); kill fired %v -> %s\n",
			name, sa.LabelJoinRecall, sa.Partition.Joinable.Exprs, sa.Exprs, sa.RecallJoinable, sa.LabelJoinPrecision, sa.JoinsInTruth, sa.FathomJoins, sa.Kill.Fired, *labelJoinsOut)
	}

	r, err := loadOrNewReading(*out, spec.Manifest, target)
	if err != nil {
		fmt.Fprintln(stderr, "lang-routes:", err)
		return 1
	}
	r.TargetLevel = target
	r.MeasuredAt = *date
	r.FathomCommit = *commit
	r.Extractor = source
	r.Oracles = mergeOracles(r.Oracles, m.oracles)
	r.Notes = mergeNotes(r.Notes, append(append([]string(nil), spec.Notes...), m.notes...))
	langbench.MergeSubject(&r, m.subject)
	if err := r.Validate(); err != nil {
		fmt.Fprintln(stderr, "lang-routes: reading refused:", err)
		return 1
	}
	b, err := langbench.MarshalReading(r)
	if err != nil {
		fmt.Fprintln(stderr, "lang-routes:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, "lang-routes:", err)
		return 1
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(stderr, "lang-routes:", err)
		return 1
	}
	c := m.subject.Counts
	var unresolved []string
	for _, reason := range fusion.RouteReasons {
		if n := c[langbench.CntRouteUnresolvedPrefix+reason]; n > 0 {
			unresolved = append(unresolved, fmt.Sprintf("%s %d", reason, n))
		}
	}
	sort.Strings(unresolved)
	fmt.Fprintf(stdout, "%s: %s routes truth %d fathom %d agree %d/%d; unresolved [%s]; level_met %s grade %+v -> %s\n",
		r.Dialect, name, c[langbench.CntRouteTruth], c[langbench.CntRouteFathom], c[langbench.CntRouteTruthFound], c[langbench.CntRouteFathomInTrue],
		strings.Join(unresolved, ", "), r.LevelMet, r.Grade, *out)
	return 0
}
