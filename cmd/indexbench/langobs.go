package main

// langobs.go — wave c3 (LB-Q-alertmanager, LB-Q-grafana-dashboard):
// `indexbench lang-obs`, lang-oracle's measurement of one subject plus its
// observability pairs, mirroring lang-routes. An observability pair is a
// cross-file reference the doc-xref pass's hook resolves
// (fusion.ResolveObsXrefs, the resolver ingest stores those edges from) over
// the files ingest reads (fusion.Pipeline.ObsXrefsForBench: every doc file
// through ingest's dispatch, the documents Kubernetes objects embed, the
// compose bind mounts): an Alertmanager route's or inhibit rule's alertname
// to the Prometheus alerting rule, an alerting rule to the receivers its
// static labels route to, a Grafana panel's metric to the recording rule, a
// uid link, a datasource uid, a provider's directory. Pairs cross dialects
// (a rule file is prometheus'), so lang-oracle's own xref scoring, which
// keeps both ends in the measured dialect, cannot score them.
//
// The oracle's `obs_references` are the truth, scored at its tier into the
// xref family (the union with any cross_references): a pair counts when one
// of its ends is a file the measured dialect owns (-only restricts that
// side, never the rule files a pair joins to). An alerting rule's routing is
// scored only where the oracle names it decided (`obs_decided`: every
// matcher Alertmanager evaluates reads a static label), and there fathom's
// routes_to edges alone are its answer, so a may it should not have written
// costs recall; an undecided rule's routing (`obs_undecided`) is disclosed,
// with fathom's certain edges checked against Alertmanager's answer for the
// static labels and that answer against fathom's routes_to and may_route_to
// together. `obs_crosscheck` (amtool run per routing) is disclosed, never
// scored. The level is langbench's computation; nobody types it.

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

// The observability counts a reading carries beside the xref family's.
const (
	cntObsTruth        = "obs_truth"
	cntObsFathom       = "obs_fathom"
	cntObsTruthFound   = "obs_truth_found"
	cntObsFathomInTrue = "obs_fathom_in_truth"
)

type obsOracleRef struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to"`
	Edge   string `json:"edge"`
}

type obsOracleRule struct {
	Path string `json:"path"`
	From string `json:"from"`
}

type obsOracleUndecided struct {
	Path      string   `json:"path"`
	Rule      string   `json:"rule"`
	ToPath    string   `json:"to_path"`
	Receivers []string `json:"receivers"`
	Labels    []string `json:"labels"`
}

type obsCrossCheck struct {
	Tool     string   `json:"tool"`
	Tier     string   `json:"tier"`
	Checked  int      `json:"checked"`
	Agree    int      `json:"agree"`
	Disagree []string `json:"disagree"`
}

// obsOracle is the observability half of an oracle file.
type obsOracle struct {
	path       string
	tier       string
	refs       []obsOracleRef
	decided    []obsOracleRule
	undecided  []obsOracleUndecided
	crossCheck *obsCrossCheck
}

// obsFields are the bench-only fields lang-oracle's decode refuses.
var obsFields = []string{"obs_references", "obs_decided", "obs_undecided", "obs_crosscheck"}

// splitObsOracle reads an oracle file, returns its observability half (nil
// when it carries no obs_references) and writes the rest to a file in dir.
func splitObsOracle(i int, path, dir string) (rest string, oo *obsOracle, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return "", nil, fmt.Errorf("%s: %w", path, err)
	}
	if raw, ok := m["obs_references"]; ok {
		oo = &obsOracle{path: path}
		var tier string
		_ = json.Unmarshal(m["tier"], &tier)
		oo.tier = orDefault(tier, "A")
		if err := json.Unmarshal(raw, &oo.refs); err != nil {
			return "", nil, fmt.Errorf("%s: obs_references: %w", path, err)
		}
		for field, dst := range map[string]any{"obs_decided": &oo.decided, "obs_undecided": &oo.undecided} {
			if v, ok := m[field]; ok {
				if err := json.Unmarshal(v, dst); err != nil {
					return "", nil, fmt.Errorf("%s: %s: %w", path, field, err)
				}
			}
		}
		if c, ok := m["obs_crosscheck"]; ok {
			oo.crossCheck = &obsCrossCheck{}
			if err := json.Unmarshal(c, oo.crossCheck); err != nil {
				return "", nil, fmt.Errorf("%s: obs_crosscheck: %w", path, err)
			}
		}
	}
	for _, f := range obsFields {
		delete(m, f)
	}
	out, err := json.Marshal(m)
	if err != nil {
		return "", nil, err
	}
	rest = filepath.Join(dir, fmt.Sprintf("%d-%s", i, filepath.Base(path)))
	return rest, oo, os.WriteFile(rest, out, 0o644)
}

// obsSubject is one subject's observability resolution.
type obsSubject struct {
	owned map[string]bool // the measured dialect's files
	res   fusion.ObsResolution
}

// resolveSubjectObs runs the hook's reading over a subject. owned names the
// measured dialect's files (lang-oracle's ownership, -only applied).
func resolveSubjectObs(ctx context.Context, spec langbench.BenchSpec, root string, only []string) (obsSubject, error) {
	const service = "langbench"
	osub := obsSubject{owned: map[string]bool{}}
	refs, err := connector.NewLocalConnector().Discover(ctx, core.ServiceRef{ID: service, Path: root})
	if err != nil {
		return osub, fmt.Errorf("discover %s: %w", root, err)
	}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	for _, r := range ownedSourceRefs(spec.Manifest, root, refs, only, contentDialectOf(pipe, refs)) {
		osub.owned[r.Path] = true
	}
	_, osub.res = pipe.ObsXrefsForBench(service, refs, func(rel string) ([]byte, error) {
		return readSubjectFile(root, rel)
	})
	return osub, nil
}

func readSubjectFile(root, rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
}

// isRoutingEdge reports whether an observability edge is an alerting rule's
// routing.
func isRoutingEdge(edge string) bool {
	return edge == fusion.EdgeRoutesTo || edge == fusion.EdgeMayRouteTo
}

// scoreObsOracle scores one oracle's observability pairs into sub and
// returns the notes that disclose what was not scored.
func scoreObsOracle(oo *obsOracle, osub obsSubject, name string, sub *langbench.Subject) []string {
	var notes []string
	decided := map[[2]string]bool{}
	for _, d := range oo.decided {
		decided[[2]string{d.Path, d.From}] = true
	}
	inScope := func(path, toPath string) bool { return osub.owned[path] || osub.owned[toPath] }
	dropped := 0
	var truth []langbench.XrefPair
	for _, r := range oo.refs {
		if !inScope(r.Path, r.ToPath) {
			dropped++
			continue
		}
		truth = append(truth, langbench.XrefPair{Path: r.Path, From: r.From, ToPath: r.ToPath, To: r.To})
	}
	var found []langbench.XrefPair
	mayOnDecided := 0
	for _, x := range osub.res.Refs {
		if !inScope(x.FromFile, x.ToFile) {
			continue
		}
		if isRoutingEdge(x.Edge) {
			if !decided[[2]string{x.FromFile, x.From}] {
				continue // an undecided rule: disclosed below
			}
			if x.Edge == fusion.EdgeMayRouteTo {
				mayOnDecided++
				continue
			}
		}
		found = append(found, langbench.XrefPair{Path: x.FromFile, From: x.From, ToPath: x.ToFile, To: x.To})
	}
	own := map[string]int{}
	langbench.ScoreXrefs(truth, found, own)
	for xref, obs := range map[string]string{
		langbench.CntXrefTruth: cntObsTruth, langbench.CntXrefFathom: cntObsFathom,
		langbench.CntXrefTruthFound: cntObsTruthFound, langbench.CntXrefFathomInTrue: cntObsFathomInTrue,
	} {
		sub.Counts[xref] += own[xref]
		sub.Counts[obs] += own[xref]
	}
	sub.Families[langbench.FamilyXref] = langbench.WeakerTier(sub.Families[langbench.FamilyXref], oo.tier)
	for _, u := range osub.res.Unresolved {
		if osub.owned[u.File] {
			sub.Counts["obs_unresolved_"+u.Reason]++
		}
	}
	if dropped > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d oracle observability pairs with neither end in the measured files were left out of scope (§1.4)", name, dropped))
	}
	if mayOnDecided > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d may_route_to edges on rules the oracle names decided were not counted as found", name, mayOnDecided))
	}
	if len(oo.undecided) > 0 {
		sub.Counts["obs_undecided_rules"] += len(oo.undecided)
		certainOutside, staticMissed := undecidedCheck(oo.undecided, osub.res.Refs)
		notes = append(notes, fmt.Sprintf("%s: %d alerting rules route through a matcher on a label they do not set statically (undecided, not scored): fathom's routes_to edges outside Alertmanager's answer for their static labels %d, that answer's receivers fathom reaches by neither routes_to nor may_route_to %d", name, len(oo.undecided), certainOutside, staticMissed))
	}
	notes = append(notes, noteObsRefs)
	if c := oo.crossCheck; c != nil && c.Checked > 0 {
		note := fmt.Sprintf("%s: routing cross-check by %s (tier %s, not scored): %d of %d decided routings agree with the oracle's dispatch", name, c.Tool, c.Tier, c.Agree, c.Checked)
		if len(c.Disagree) > 0 {
			note += "; disagree: " + strings.Join(c.Disagree, ", ")
		}
		notes = append(notes, note)
	}
	return notes
}

// undecidedCheck compares fathom's routing of the undecided rules with
// Alertmanager's answer for their static labels: a routes_to edge outside it
// is a certainty fathom should not have claimed; a receiver of it fathom
// reaches by no edge is a routing fathom missed.
func undecidedCheck(us []obsOracleUndecided, refs []fusion.DocXref) (certainOutside, staticMissed int) {
	type k struct{ path, rule, toPath string }
	got := map[k]map[string]string{}
	for _, x := range refs {
		if !isRoutingEdge(x.Edge) {
			continue
		}
		kk := k{x.FromFile, x.From, x.ToFile}
		if got[kk] == nil {
			got[kk] = map[string]string{}
		}
		got[kk][x.To] = x.Edge
	}
	for _, u := range us {
		kk := k{u.Path, u.Rule, u.ToPath}
		static := map[string]bool{}
		for _, r := range u.Receivers {
			static[r] = true
			if got[kk][r] == "" {
				staticMissed++
			}
		}
		for r, edge := range got[kk] {
			if edge == fusion.EdgeRoutesTo && !static[r] {
				certainOutside++
			}
		}
	}
	return certainOutside, staticMissed
}

// noteObsRefs discloses what the observability counts compare.
const noteObsRefs = "observability pairs (obs_* counts, wave c3) are scored into xref_precision and xref_recall with the oracle's cross_references: an Alertmanager route's or inhibit rule's alertname to the Prometheus alerting rule, an alerting rule to the receivers Alertmanager's dispatch routes its static labels to (decided rules only; fathom's routes_to edges), a Grafana panel's or alert rule's metric to the recording rule, a uid link, a datasource uid, a provider's directory, resolved by fusion.ResolveObsXrefs (the resolver ingest stores those doc-xref edges from) over the files ingest reads; obs_unresolved_<reason> counts the measured files' references that joined nothing (undeclared: a metric no recording rule records, a templated uid, a datasource or contact point the repository does not provision)"

// cmdLangObs is `indexbench lang-obs`.
func cmdLangObs(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-obs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dialect := fs.String("dialect", "", "dialect id whose reading takes the observability pairs (required)")
	manifestPath := fs.String("manifest", "", "the dialect's dialect.json (default: internal/extractor/dialects/<id>/dialect.json, as lang-oracle)")
	subject := fs.String("subject", "", "subject checkout to measure (required)")
	subjectName := fs.String("subject-name", "", "subject name recorded in the reading (default: the directory name)")
	ctagsPath := fs.String("ctags", "", "universal-ctags binary (as lang-oracle)")
	var toolOracles multiFlag
	fs.Var(&toolOracles, "tool-oracle", "oracle JSON in the neutral shape; its obs_references are the observability truth (repeatable)")
	only := fs.String("only", "", "comma-separated globs restricting the measured dialect's files (as lang-oracle)")
	out := fs.String("out", "", "reading to write; an existing reading of the same dialect is merged (required)")
	queuePath := fs.String("queue", "docs/plans/language-breadth.queue.json", "the queue (for the dialect's target level)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "measured_at")
	commit := fs.String("commit", "", "fathom commit measured (default: git rev-parse --short HEAD)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dialect == "" || *subject == "" || *out == "" || len(toolOracles) == 0 || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: indexbench lang-obs -dialect <id> -subject <dir> -tool-oracle <json> -out <reading.json>")
		return 2
	}
	tmp, err := os.MkdirTemp("", "lang-obs-")
	if err != nil {
		fmt.Fprintln(stderr, "lang-obs:", err)
		return 1
	}
	defer os.RemoveAll(tmp)
	var rest []string
	var obs []*obsOracle
	for i, p := range toolOracles {
		r, oo, err := splitObsOracle(i, p, tmp)
		if err != nil {
			fmt.Fprintln(stderr, "lang-obs:", err)
			return 1
		}
		rest = append(rest, r)
		if oo != nil {
			obs = append(obs, oo)
		}
	}
	spec, source, err := resolveBenchSpec(*dialect, *manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "lang-obs:", err)
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
	var with extractor.Extractor
	if source == "dialect" {
		x, xerr := dialectExtractor(spec.Manifest)
		if xerr != nil {
			fmt.Fprintln(stderr, "lang-obs:", xerr)
			return 1
		}
		with = x
	}
	m, err := measureSubject(ctx, spec, *subject, name, oracleOpts{ctags: *ctagsPath, tools: rest, only: onlyGlobs}, with)
	if err != nil {
		fmt.Fprintln(stderr, "lang-obs:", err)
		return 1
	}
	if len(obs) > 0 {
		osub, err := resolveSubjectObs(ctx, spec, *subject, onlyGlobs)
		if err != nil {
			fmt.Fprintln(stderr, "lang-obs:", err)
			return 1
		}
		for _, oo := range obs {
			m.notes = append(m.notes, scoreObsOracle(oo, osub, name, &m.subject)...)
		}
	}
	r, err := loadOrNewReading(*out, spec.Manifest, target)
	if err != nil {
		fmt.Fprintln(stderr, "lang-obs:", err)
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
		fmt.Fprintln(stderr, "lang-obs: reading refused:", err)
		return 1
	}
	b, err := langbench.MarshalReading(r)
	if err != nil {
		fmt.Fprintln(stderr, "lang-obs:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, "lang-obs:", err)
		return 1
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(stderr, "lang-obs:", err)
		return 1
	}
	c := m.subject.Counts
	var keys []string
	for k := range c {
		if strings.HasPrefix(k, "obs_") {
			keys = append(keys, fmt.Sprintf("%s %d", k, c[k]))
		}
	}
	sort.Strings(keys)
	fmt.Fprintf(stdout, "%s: %s defs fathom %d oracle %d matched %d; xref truth %d fathom %d agree %d/%d; [%s]; level_met %s grade %+v -> %s\n",
		r.Dialect, name, c["fathom_defs"], c["oracle_defs"], c["matched"], c[langbench.CntXrefTruth], c[langbench.CntXrefFathom], c[langbench.CntXrefTruthFound], c[langbench.CntXrefFathomInTrue],
		strings.Join(keys, ", "), r.LevelMet, r.Grade, *out)
	return 0
}
