package main

// routeoracle.go — LB-F324 (wave c2): `indexbench route-oracle`, framework
// routes scored against a running framework's own route dump
// (calibration/oracles/routes/README.md). Each subject is ingested into a
// scratch store exactly as a product ingest stores it, and its routes are
// read back through serviceref.RoutesFromRows, the composition
// BuildFromStore joins on, so the bench never scores a route ingest drops
// or a mount it would not apply (the lang-routes precedent, LB-F98).
//
// route_recall and route_precision count (method, normalized path) pairs;
// bound_recall and bound_precision also require the handler to equal the
// oracle's (fathom's naming: Class.method, or the function name). A subject
// reads PASS when both bound metrics are at least .90; the reading reads
// PASS when every subject does.

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

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// routeOracleFile is the oracle shape (calibration/oracles/routes/README.md).
type routeOracleFile struct {
	Tool        string             `json:"tool"`
	ToolVersion string             `json:"tool_version"`
	Subject     string             `json:"subject"`
	SHA         string             `json:"sha"`
	Licence     string             `json:"licence"`
	Routes      []routeOracleRoute `json:"routes"`
}

// routeOracleRoute is one route the framework serves: method upper-case or
// ANY, path as the framework renders it, handler in fathom's naming.
type routeOracleRoute struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
}

// routeScore is one subject's (or the pool's) reading.
type routeScore struct {
	Subject        string  `json:"subject,omitempty"`
	SHA            string  `json:"sha,omitempty"`
	Tool           string  `json:"tool,omitempty"`
	ToolVersion    string  `json:"tool_version,omitempty"`
	Licence        string  `json:"licence,omitempty"`
	OracleRoutes   int     `json:"oracle_routes"`
	FathomRoutes   int     `json:"fathom_routes"`
	RouteHits      int     `json:"route_hits"`
	RouteFound     int     `json:"route_found_in_oracle"`
	BoundHits      int     `json:"bound_hits"`
	BoundFound     int     `json:"bound_found_in_oracle"`
	OracleBound    int     `json:"oracle_bound"`
	FathomBound    int     `json:"fathom_bound"`
	RouteRecall    float64 `json:"route_recall"`
	RoutePrecision float64 `json:"route_precision"`
	BoundRecall    float64 `json:"bound_recall"`
	BoundPrecision float64 `json:"bound_precision"`
	IngestSeconds  float64 `json:"ingest_seconds,omitempty"`
	// HandlersIndexed counts fathom's routes whose handler is the name of
	// a symbol the same ingest indexed (the route binds to its handler);
	// Handlers counts fathom's routes with a handler at all.
	HandlersIndexed int      `json:"handlers_indexed"`
	Handlers        int      `json:"handlers"`
	Missed          []string `json:"missed,omitempty"`
	Extra           []string `json:"extra,omitempty"`
	MissedMore      int      `json:"missed_more,omitempty"`
	ExtraMore       int      `json:"extra_more,omitempty"`
	Verdict         string   `json:"verdict"`
}

// routeReading is the file -out writes.
type routeReading struct {
	MeasuredAt   string       `json:"measured_at"`
	FathomCommit string       `json:"fathom_commit"`
	Only         []string     `json:"only,omitempty"`
	Threshold    float64      `json:"threshold"`
	Subjects     []routeScore `json:"subjects"`
	Pooled       routeScore   `json:"pooled"`
	Verdict      string       `json:"verdict"`
}

// routeOracleThreshold is the bound recall and precision a subject needs.
const routeOracleThreshold = 0.90

// routeOracleMissCap bounds each subject's missed and extra lists.
const routeOracleMissCap = 25

// cmdRouteOracle is `indexbench route-oracle` (LB-F324).
func cmdRouteOracle(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("route-oracle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var oracles, repos multiFlag
	fs.Var(&oracles, "oracle", "route oracle JSON (repeatable; paired with -repo by position)")
	fs.Var(&repos, "repo", "subject checkout at the oracle's sha (repeatable)")
	only := fs.String("only", "", "comma-separated globs restricting the files fathom's routes may come from")
	out := fs.String("out", "", "reading to write (required)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "measured_at")
	commit := fs.String("commit", "", "fathom commit measured (default: git rev-parse --short HEAD)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(oracles) == 0 || len(oracles) != len(repos) || *out == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: indexbench route-oracle -oracle F.json -repo DIR [-oracle F2.json -repo DIR2 ...] [-only GLOB] -out R.json")
		return 2
	}
	var globs []string
	for _, g := range strings.Split(*only, ",") {
		if g = strings.TrimSpace(g); g != "" {
			globs = append(globs, g)
		}
	}
	ctx := context.Background()
	if *commit == "" {
		*commit = gitOut(ctx, ".", "rev-parse", "--short", "HEAD")
	}
	reading := routeReading{MeasuredAt: *date, FathomCommit: *commit, Only: globs, Threshold: routeOracleThreshold}
	var pool routeScore
	for i, op := range oracles {
		of, err := readRouteOracle(op)
		if err != nil {
			fmt.Fprintln(stderr, "route-oracle:", err)
			return 1
		}
		found, indexed, secs, err := ingestedRoutes(ctx, repos[i], globs)
		if err != nil {
			fmt.Fprintln(stderr, "route-oracle:", err)
			return 1
		}
		s := scoreRoutes(of.Routes, found)
		for _, r := range found {
			if r.Handler != "" {
				s.Handlers++
				if indexed[r.Handler] {
					s.HandlersIndexed++
				}
			}
		}
		s.Subject, s.SHA, s.Tool, s.ToolVersion, s.Licence, s.IngestSeconds = of.Subject, of.SHA, of.Tool, of.ToolVersion, of.Licence, secs
		reading.Subjects = append(reading.Subjects, s)
		pool.add(s)
		fmt.Fprintf(stdout, "%s: route recall %.4f precision %.4f; bound recall %.4f precision %.4f (oracle %d, fathom %d; handlers indexed %d of %d) %s\n",
			s.Subject, s.RouteRecall, s.RoutePrecision, s.BoundRecall, s.BoundPrecision, s.OracleRoutes, s.FathomRoutes, s.HandlersIndexed, s.Handlers, s.Verdict)
	}
	pool.finish()
	reading.Pooled = pool
	reading.Verdict = "PASS"
	for _, s := range reading.Subjects {
		if s.Verdict != "PASS" {
			reading.Verdict = "FAIL"
		}
	}
	b, err := json.MarshalIndent(reading, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, "route-oracle:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, "route-oracle:", err)
		return 1
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		fmt.Fprintln(stderr, "route-oracle:", err)
		return 1
	}
	fmt.Fprintf(stdout, "pooled: bound recall %.4f precision %.4f; verdict %s -> %s\n", pool.BoundRecall, pool.BoundPrecision, reading.Verdict, *out)
	return 0
}

// readRouteOracle reads and checks one oracle file.
func readRouteOracle(path string) (routeOracleFile, error) {
	var of routeOracleFile
	b, err := os.ReadFile(path)
	if err != nil {
		return of, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&of); err != nil {
		return of, fmt.Errorf("%s: %w", path, err)
	}
	if of.Subject == "" || of.SHA == "" || of.Tool == "" {
		return of, fmt.Errorf("%s: subject, sha and tool are required", path)
	}
	for _, r := range of.Routes {
		if r.Method == "" || r.Method != strings.ToUpper(r.Method) || r.Path == "" {
			return of, fmt.Errorf("%s: route %+v: method must be upper-case or ANY and path non-empty", path, r)
		}
	}
	return of, nil
}

// ingestedRoutes ingests repo into a scratch store as a product ingest
// does and returns its routes as RoutesFromRows composes them, restricted
// to files the globs match; the names of the symbols the same ingest
// indexed; and the ingest's wall seconds.
func ingestedRoutes(ctx context.Context, repo string, globs []string) ([]routeOracleRoute, map[string]bool, float64, error) {
	dir, err := os.MkdirTemp("", "route-oracle-")
	if err != nil {
		return nil, nil, 0, err
	}
	defer os.RemoveAll(dir)
	st, err := store.NewSQLiteStore(filepath.Join(dir, "routes.sqlite"))
	if err != nil {
		return nil, nil, 0, err
	}
	defer st.Close()
	const service = "route-oracle"
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})
	defer p.Close()
	started := time.Now()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: service, Path: repo}); err != nil {
		return nil, nil, 0, fmt.Errorf("ingest %s: %w", repo, err)
	}
	secs := time.Since(started).Seconds()
	refs, _, err := st.ServiceRefsFor(ctx, service)
	if err != nil {
		return nil, nil, 0, err
	}
	obs, err := st.QueryObservations(ctx, service, "")
	if err != nil {
		return nil, nil, 0, err
	}
	indexed := make(map[string]bool, len(obs))
	for _, o := range obs {
		indexed[o.Symbol] = true
	}
	var out []routeOracleRoute
	for _, r := range serviceref.RoutesFromRows(refs.Routes) {
		if !onlyMatch(globs, r.File) {
			continue
		}
		out = append(out, routeOracleRoute{Method: r.Method, Path: r.Path, Handler: r.Handler})
	}
	return out, indexed, secs, nil
}

// scoreRoutes scores fathom's routes against the oracle's: route pairs on
// (method, normalized path), bound triples with the handler too.
func scoreRoutes(truth, found []routeOracleRoute) routeScore {
	pair := func(r routeOracleRoute) string { return r.Method + " " + serviceref.NormalizeTemplate(r.Path) }
	triple := func(r routeOracleRoute) string { return pair(r) + " " + r.Handler }
	tp, tb, fp, fb := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, r := range truth {
		tp[pair(r)], tb[triple(r)] = true, true
	}
	for _, r := range found {
		fp[pair(r)], fb[triple(r)] = true, true
	}
	var s routeScore
	s.OracleRoutes, s.FathomRoutes, s.OracleBound, s.FathomBound = len(tp), len(fp), len(tb), len(fb)
	s.RouteHits, s.RouteFound = routeCountIn(tp, fp), routeCountIn(fp, tp)
	s.BoundHits, s.BoundFound = routeCountIn(tb, fb), routeCountIn(fb, tb)
	s.Missed, s.MissedMore = routeCapped(routeMissing(tb, fb))
	s.Extra, s.ExtraMore = routeCapped(routeMissing(fb, tb))
	s.finish()
	return s
}

// add pools another subject's counts.
func (s *routeScore) add(o routeScore) {
	s.OracleRoutes += o.OracleRoutes
	s.FathomRoutes += o.FathomRoutes
	s.RouteHits += o.RouteHits
	s.RouteFound += o.RouteFound
	s.OracleBound += o.OracleBound
	s.FathomBound += o.FathomBound
	s.BoundHits += o.BoundHits
	s.BoundFound += o.BoundFound
	s.Handlers += o.Handlers
	s.HandlersIndexed += o.HandlersIndexed
}

// finish computes the ratios and the verdict from the counts.
func (s *routeScore) finish() {
	s.RouteRecall = routeRatio(s.RouteHits, s.OracleRoutes)
	s.RoutePrecision = routeRatio(s.RouteFound, s.FathomRoutes)
	s.BoundRecall = routeRatio(s.BoundHits, s.OracleBound)
	s.BoundPrecision = routeRatio(s.BoundFound, s.FathomBound)
	s.Verdict = "FAIL"
	if s.BoundRecall >= routeOracleThreshold && s.BoundPrecision >= routeOracleThreshold {
		s.Verdict = "PASS"
	}
}

// routeRatio is n/d rounded to four places; 0 when d is 0.
func routeRatio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(int(float64(n)/float64(d)*10000+0.5)) / 10000
}

// routeCountIn counts a's keys present in b.
func routeCountIn(a, b map[string]bool) int {
	n := 0
	for k := range a {
		if b[k] {
			n++
		}
	}
	return n
}

// routeMissing is a's keys absent from b, sorted.
func routeMissing(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// routeCapped keeps the first routeOracleMissCap entries and counts the rest.
func routeCapped(list []string) ([]string, int) {
	if len(list) <= routeOracleMissCap {
		return list, 0
	}
	return list[:routeOracleMissCap], len(list) - routeOracleMissCap
}
