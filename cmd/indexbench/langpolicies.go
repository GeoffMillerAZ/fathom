package main

// langpolicies.go — LB-F137 (wave pc): `indexbench lang-policies`,
// lang-oracle's measurement of one subject plus its policy references,
// mirroring lang-routes. A policy pair (a policy, rule, exception or
// constraint entity to the Kubernetes object or declared resource its target
// names) is resolved by fusion.ResolvePolicyRefs — the resolver ingest stores
// policy_ref edges from — over the files ingest's policy pass reads
// (fusion.IsPolicyCandidate, through ingest's own dispatch), and scored
// against the oracle's policy_references at its tier into the xref family,
// the union with any cross_references the same oracle scored there
// (langbench.ScorePolicyRefs). The level is langbench's computation; nobody
// types it.
//
// A content-claimed policy dialect (Kyverno, Gatekeeper: a manifest naming no
// extension, basename or glob, since a .yaml is Kubernetes's) is owned by the
// predicate ingest claims it with, fusion.PolicyDialect: lang-oracle's
// ownership asks classify.go's ContentDialect, which knows only
// CloudFormation and OpenAPI, so this command names the claimed files as the
// measured set (each path a literal glob) and says so in the reading.
//
// The policy fields are bench-only and lang-oracle's strict decode refuses
// them, so each oracle file is passed to the shared measurement with
// policy_references, policy_unresolved and policy_crosscheck removed.

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

// policyOracleRef is one oracle policy pair: the policy entity (path, from)
// and the target entity (to_path, to) its target (raw, by strategy) names.
type policyOracleRef struct {
	Path     string `json:"path"`
	From     string `json:"from"`
	ToPath   string `json:"to_path"`
	To       string `json:"to"`
	Raw      string `json:"raw"`
	Strategy string `json:"strategy,omitempty"`
}

// policyOracleUnresolved is one oracle policy reference that names no
// declared target, with the reason (fusion.PolicyReasons).
type policyOracleUnresolved struct {
	Path     string `json:"path"`
	From     string `json:"from"`
	Raw      string `json:"raw"`
	Reason   string `json:"reason"`
	Strategy string `json:"strategy,omitempty"`
}

// policyCrossCheck is a stronger tool's agreement with the oracle on part of
// its answer (kyverno test's evaluated resources, gator verify's violating
// objects), disclosed in the reading's notes, never scored.
type policyCrossCheck struct {
	Tool     string   `json:"tool"`
	Tier     string   `json:"tier"`
	Checked  int      `json:"checked"`
	Agree    int      `json:"agree"`
	Disagree []string `json:"disagree"`
	Note     string   `json:"note,omitempty"`
}

// policyOracle is the policy half of an oracle file.
type policyOracle struct {
	path       string
	tier       string
	refs       []policyOracleRef
	unresolved []policyOracleUnresolved
	crossCheck *policyCrossCheck
}

// policyFields are the bench-only policy fields lang-oracle's decode refuses.
var policyFields = []string{"policy_references", "policy_unresolved", "policy_crosscheck"}

// splitPolicyOracle reads an oracle file, returns its policy half (nil when
// it carries no policy_references) and writes the rest to a file in dir for
// the shared measurement.
func splitPolicyOracle(i int, path, dir string) (rest string, po *policyOracle, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return "", nil, fmt.Errorf("%s: %w", path, err)
	}
	if raw, ok := m["policy_references"]; ok {
		po = &policyOracle{path: path}
		var tier string
		_ = json.Unmarshal(m["tier"], &tier)
		po.tier = orDefault(tier, "A")
		if err := json.Unmarshal(raw, &po.refs); err != nil {
			return "", nil, fmt.Errorf("%s: policy_references: %w", path, err)
		}
		if u, ok := m["policy_unresolved"]; ok {
			if err := json.Unmarshal(u, &po.unresolved); err != nil {
				return "", nil, fmt.Errorf("%s: policy_unresolved: %w", path, err)
			}
		}
		if c, ok := m["policy_crosscheck"]; ok {
			po.crossCheck = &policyCrossCheck{}
			if err := json.Unmarshal(c, po.crossCheck); err != nil {
				return "", nil, fmt.Errorf("%s: policy_crosscheck: %w", path, err)
			}
		}
	}
	for _, f := range policyFields {
		delete(m, f)
	}
	out, err := json.Marshal(m)
	if err != nil {
		return "", nil, err
	}
	rest = filepath.Join(dir, fmt.Sprintf("%d-%s", i, filepath.Base(path)))
	return rest, po, os.WriteFile(rest, out, 0o644)
}

// policySubject is one subject's policy resolution, from the files ingest's
// policy pass reads.
type policySubject struct {
	files map[string]bool
	res   fusion.PolicyResolution
}

// resolveSubjectPolicies runs ingest's policy reading and
// fusion.ResolvePolicyRefs over a subject.
func resolveSubjectPolicies(ctx context.Context, root string, refs []core.SourceRef, only []string) (policySubject, error) {
	const service = "langbench"
	ps := policySubject{files: map[string]bool{}}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	var files []fusion.DocFile
	for _, ref := range refs {
		if !fusion.IsPolicyCandidate(ref.Path) || !onlyMatch(only, ref.Path) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil {
			return ps, err
		}
		ps.files[ref.Path] = true
		if f, ok := pipe.PolicyFileForBench(service, refs, ref.Path, content); ok {
			files = append(files, f)
		}
	}
	ps.res = fusion.ResolvePolicyRefs(files)
	return ps, nil
}

// scorePolicyOracle scores one oracle's policy pairs against the subject's
// resolved policy references into sub (the xref family at the weaker of its
// tier and any already scored there, and the policy_* counts) and returns
// the notes that disclose what was not scored.
func scorePolicyOracle(po *policyOracle, ps policySubject, name string, sub *langbench.Subject) []string {
	var notes []string
	dropped := 0
	var truth []langbench.XrefPair
	for _, r := range po.refs {
		if !ps.files[r.Path] || !ps.files[r.ToPath] {
			dropped++
			continue
		}
		truth = append(truth, langbench.XrefPair{Path: r.Path, From: r.From, ToPath: r.ToPath, To: r.To})
	}
	var found []langbench.XrefPair
	for _, r := range ps.res.Refs {
		found = append(found, langbench.XrefPair{Path: r.FromFile, From: r.From, ToPath: r.ToFile, To: r.To})
	}
	langbench.ScorePolicyRefs(truth, found, sub.Counts)
	sub.Families[langbench.FamilyXref] = langbench.WeakerTier(sub.Families[langbench.FamilyXref], po.tier)
	for reason, n := range ps.res.UnresolvedByReason() {
		sub.Counts[langbench.CntPolicyUnresolvedPrefix+reason] += n
	}
	for _, u := range po.unresolved {
		if ps.files[u.Path] {
			sub.Counts[langbench.CntPolicyOracleUnresolvedPrefix+u.Reason]++
		}
	}
	if dropped > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d oracle policy references on files the policy pass does not read were left out of scope (§1.4)", name, dropped))
	}
	notes = append(notes, notePolicyRefs)
	if c := po.crossCheck; c != nil && c.Checked > 0 {
		note := fmt.Sprintf("%s: policy cross-check by %s (tier %s, not scored): %d of %d checked pairs agree with the oracle", name, c.Tool, c.Tier, c.Agree, c.Checked)
		if c.Note != "" {
			note += " (" + c.Note + ")"
		}
		if len(c.Disagree) > 0 {
			shown := c.Disagree
			if len(shown) > 10 {
				shown = append(append([]string{}, shown[:10]...), fmt.Sprintf("and %d more", len(c.Disagree)-10))
			}
			note += "; disagree: " + strings.Join(shown, ", ")
		}
		notes = append(notes, note)
	}
	return notes
}

// notePolicyRefs discloses what the policy counts compare.
const notePolicyRefs = "policy references (policy_* counts, LB-F137) are scored into xref_precision and xref_recall with the oracle's cross_references: a policy, rule, exception or constraint entity to the Kubernetes object or declared resource its target names, resolved by fusion.ResolvePolicyRefs (the resolver ingest stores policy_ref edges from; the nearest targets, a fan-out over 256 refused) over the files ingest's policy pass reads; policy_unresolved_<reason> counts fathom's unresolved references, policy_oracle_unresolved_<reason> the oracle's"

// contentOwned is the subject's files a content-claimed policy dialect owns:
// the paths fusion.PolicyDialect, ingest's claim, gives to dialect.
func contentOwned(root string, refs []core.SourceRef, only []string, dialect string) ([]string, error) {
	var out []string
	for _, ref := range refs {
		if !onlyMatch(only, ref.Path) {
			continue
		}
		switch strings.ToLower(filepath.Ext(ref.Path)) {
		case ".yaml", ".yml":
		default:
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil {
			return nil, err
		}
		if fusion.PolicyDialect(ref.Path, b) == dialect {
			out = append(out, ref.Path)
		}
	}
	sort.Strings(out)
	return out, nil
}

// literalGlob escapes a path for a manifest glob (path.Match per segment).
func literalGlob(p string) string {
	r := strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`)
	return r.Replace(p)
}

// cmdLangPolicies is `indexbench lang-policies` (LB-F137).
func cmdLangPolicies(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-policies", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dialect := fs.String("dialect", "", "dialect id whose reading takes the policy pairs (required)")
	manifestPath := fs.String("manifest", "", "the dialect's dialect.json (default: internal/extractor/dialects/<id>/dialect.json, as lang-oracle)")
	subject := fs.String("subject", "", "subject checkout to measure (required)")
	subjectName := fs.String("subject-name", "", "subject name recorded in the reading (default: the directory name)")
	ctagsPath := fs.String("ctags", "", "universal-ctags binary (as lang-oracle)")
	var toolOracles multiFlag
	fs.Var(&toolOracles, "tool-oracle", "oracle JSON in the neutral shape; its policy_references and policy_unresolved are the policy truth (repeatable)")
	only := fs.String("only", "", "comma-separated globs restricting the subject's files (as lang-oracle)")
	out := fs.String("out", "", "reading to write; an existing reading of the same dialect is merged (required)")
	queuePath := fs.String("queue", "docs/plans/language-breadth.queue.json", "the queue (for the dialect's target level)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "measured_at")
	commit := fs.String("commit", "", "fathom commit measured (default: git rev-parse --short HEAD)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dialect == "" || *subject == "" || *out == "" || len(toolOracles) == 0 || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: indexbench lang-policies -dialect <id> -subject <dir> -tool-oracle <json> -out <reading.json>")
		return 2
	}
	tmp, err := os.MkdirTemp("", "lang-policies-")
	if err != nil {
		fmt.Fprintln(stderr, "lang-policies:", err)
		return 1
	}
	defer os.RemoveAll(tmp)
	var rest []string
	var policies []*policyOracle
	for i, p := range toolOracles {
		r, po, err := splitPolicyOracle(i, p, tmp)
		if err != nil {
			fmt.Fprintln(stderr, "lang-policies:", err)
			return 1
		}
		rest = append(rest, r)
		if po != nil {
			policies = append(policies, po)
		}
	}
	if len(policies) == 0 {
		fmt.Fprintln(stderr, "lang-policies: no -tool-oracle carries policy_references")
		return 2
	}

	spec, source, err := resolveBenchSpec(*dialect, *manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "lang-policies:", err)
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
	refs, err := connector.NewLocalConnector().Discover(ctx, core.ServiceRef{ID: "langbench", Path: *subject})
	if err != nil {
		fmt.Fprintln(stderr, "lang-policies:", err)
		return 1
	}
	var ownNote string
	mt := spec.Manifest.Match
	if spec.Manifest.Engine == "doc" && len(mt.Extensions)+len(mt.Basenames)+len(mt.Globs) == 0 && mt.Sniff == nil {
		owned, err := contentOwned(*subject, refs, onlyGlobs, spec.Manifest.ID)
		if err != nil {
			fmt.Fprintln(stderr, "lang-policies:", err)
			return 1
		}
		if len(owned) == 0 {
			fmt.Fprintf(stderr, "lang-policies: subject %s has no file fusion.PolicyDialect gives to %s\n", *subject, spec.Manifest.ID)
			return 1
		}
		for _, p := range owned {
			spec.Manifest.Match.Globs = append(spec.Manifest.Match.Globs, literalGlob(p))
		}
		ownNote = fmt.Sprintf("%s: the %d measured files are the ones ingest's claim (fusion.PolicyDialect, every object document of the dialect's API groups) gives to %s; a file mixing them with other objects stays Kubernetes and is not measured", name, len(owned), spec.Manifest.ID)
	}
	// The dialect's own extractor, as lang-oracle and lang-routes measure it.
	var with extractor.Extractor
	if source == "dialect" {
		x, xerr := dialectExtractor(spec.Manifest)
		if xerr != nil {
			fmt.Fprintln(stderr, "lang-policies:", xerr)
			return 1
		}
		with = x
		if c, ok := x.(interface{ Close() }); ok {
			defer c.Close()
		}
	}
	m, err := measureSubject(ctx, spec, *subject, name, oracleOpts{ctags: *ctagsPath, tools: rest, only: onlyGlobs}, with)
	if err != nil {
		fmt.Fprintln(stderr, "lang-policies:", err)
		return 1
	}
	if ownNote != "" {
		m.notes = append(m.notes, ownNote)
	}
	ps, err := resolveSubjectPolicies(ctx, *subject, refs, onlyGlobs)
	if err != nil {
		fmt.Fprintln(stderr, "lang-policies:", err)
		return 1
	}
	for _, po := range policies {
		m.notes = append(m.notes, scorePolicyOracle(po, ps, name, &m.subject)...)
	}

	r, err := loadOrNewReading(*out, spec.Manifest, target)
	if err != nil {
		fmt.Fprintln(stderr, "lang-policies:", err)
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
		fmt.Fprintln(stderr, "lang-policies: reading refused:", err)
		return 1
	}
	b, err := langbench.MarshalReading(r)
	if err != nil {
		fmt.Fprintln(stderr, "lang-policies:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, "lang-policies:", err)
		return 1
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(stderr, "lang-policies:", err)
		return 1
	}
	c := m.subject.Counts
	var unresolved []string
	for _, reason := range fusion.PolicyReasons {
		if n := c[langbench.CntPolicyUnresolvedPrefix+reason]; n > 0 {
			unresolved = append(unresolved, fmt.Sprintf("%s %d", reason, n))
		}
	}
	fmt.Fprintf(stdout, "%s: %s policies truth %d fathom %d agree %d/%d; unresolved [%s]; level_met %s grade %+v -> %s\n",
		r.Dialect, name, c[langbench.CntPolicyTruth], c[langbench.CntPolicyFathom], c[langbench.CntPolicyTruthFound], c[langbench.CntPolicyFathomInTrue],
		strings.Join(unresolved, ", "), r.LevelMet, r.Grade, *out)
	return 0
}
