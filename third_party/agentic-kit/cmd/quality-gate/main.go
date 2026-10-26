package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, evaluateModels)) }

const usage = `quality-gate: exact Git snapshots, deterministic rules, optional judgment holds
Commands: init, validate, site-digests, doctor, check, pre-push, lefthook
Start: quality-gate init --dir .quality-gate
Enforce: quality-gate check --boundary pre-commit --staged
Bootstrap: add --observe --policy-file .quality-gate/policy.json
CI: quality-gate check --boundary ci --base BASE --revision SHA --policy-ref BASE
Diagnose: quality-gate doctor --boundary pre-commit [--format json]
Exit: 0 pass/explicit observation or doctor configuration ready, 1 policy hold, 2 invalid/unknown.
Doctor never proves enforcement, hook firing or provider availability.
Remote disclosure requires --allow-remote. See docs/quality-boundaries.md.
`

const noRequiredBoundaryReason = "boundary has no required rules or hold sites"

type options struct {
	repo, index, boundary, base, revision, policyRef, policyPath, policyFile string
	gitBin                                                                   string
	format, cacheDir                                                         string
	staged, observe, allowRemote                                             bool
	deferModels                                                              bool // internal structural preflight, never a caller flag
	shadowObservation                                                        bool // explicit observe intent during internal preflight
	budget                                                                   *modelBudget
	now                                                                      func() time.Time
}

func run(args []string, in io.Reader, out, errout io.Writer, model modelEvaluator) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(out, usage)
		return 0
	}
	command := args[0]
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(errout)
	var o options
	dir := f.String("dir", ".quality-gate", "new scaffold directory (init only)")
	f.StringVar(&o.repo, "repo", ".", "repository directory")
	f.StringVar(&o.index, "index-file", "", "explicit alternate index, relative to repo")
	f.StringVar(&o.gitBin, "git-bin", "", "trusted absolute Git executable (default: resolve PATH outside candidate repository)")
	f.StringVar(&o.boundary, "boundary", "", "pre-commit, pre-push, ci, or pre-publish")
	f.StringVar(&o.base, "base", "", "base commit (empty means full tree)")
	f.StringVar(&o.revision, "revision", "", "proposed commit")
	f.StringVar(&o.policyRef, "policy-ref", "HEAD", "trusted policy commit or ref")
	f.StringVar(&o.policyPath, "policy-path", ".quality-gate/policy.json", "tracked policy path")
	f.StringVar(&o.policyFile, "policy-file", "", "local untrusted policy: validate/digests/doctor/observe only")
	f.StringVar(&o.format, "format", "text", "text or json")
	f.StringVar(&o.cacheDir, "cache-dir", "", "optional private shadow cache directory")
	f.BoolVar(&o.staged, "staged", false, "read the index, never working files")
	f.BoolVar(&o.observe, "observe", false, "report observation without enforcement")
	f.BoolVar(&o.allowRemote, "allow-remote", false, "explicitly authorize selected source disclosure")
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if f.NArg() != 0 || (o.format != "json" && o.format != "text") {
		fmt.Fprintln(errout, "invalid arguments or output format")
		return 2
	}
	// A proposed checkout's HEAD is not a safe implicit policy authority in CI.
	// The caller must supply the independently trusted policy revision explicitly.
	explicitPolicyRef := false
	f.Visit(func(v *flag.Flag) {
		if v.Name == "policy-ref" {
			explicitPolicyRef = true
		}
	})
	allowed := map[string][]string{
		"init": {"dir"}, "lefthook": {},
		"validate":     {"repo", "git-bin", "policy-file", "policy-ref", "policy-path"},
		"site-digests": {"repo", "git-bin", "policy-file", "policy-ref", "policy-path"},
		"doctor":       {"repo", "git-bin", "boundary", "policy-ref", "policy-path", "policy-file", "format"},
		"check":        {"repo", "git-bin", "index-file", "boundary", "base", "revision", "policy-ref", "policy-path", "policy-file", "format", "cache-dir", "staged", "observe", "allow-remote"},
		"pre-push":     {"repo", "git-bin", "policy-ref", "policy-path", "policy-file", "format", "cache-dir", "observe", "allow-remote"},
	}
	invalidFlag := false
	f.Visit(func(v *flag.Flag) {
		if !includes(allowed[command], v.Name) {
			invalidFlag = true
		}
	})
	if invalidFlag {
		fmt.Fprintln(errout, "flag is not supported by this command")
		return 2
	}
	if command == "doctor" {
		if o.boundary == "" {
			o.boundary = "pre-commit"
		}
		if !includes([]string{"pre-commit", "pre-push", "ci", "pre-publish"}, o.boundary) {
			fmt.Fprintln(errout, "doctor requires a valid boundary")
			return 2
		}
		return runDoctor(out, o, explicitPolicyRef)
	}
	if command == "check" && !o.observe && (o.boundary == "ci" || o.boundary == "pre-publish") {
		if o.staged || !explicitPolicyRef {
			fmt.Fprintln(errout, "CI/publish enforcement requires --revision and explicit --policy-ref from a trusted source")
			return 2
		}
	}
	switch command {
	case "init":
		if err := scaffold(*dir); err != nil {
			fmt.Fprintln(errout, "cannot create scaffold:", err)
			return 2
		}
		fmt.Fprintln(out, "Created policy, schema, and Lefthook fragment. Review before adoption; no hooks installed.")
		return 0
	case "lefthook":
		fmt.Fprint(out, lefthookFragment)
		return 0
	case "validate", "site-digests", "check", "pre-push":
	default:
		fmt.Fprintln(errout, "unknown command")
		return 2
	}
	if command == "pre-push" {
		o.boundary = "pre-push"
	}
	if command == "check" && (!includes([]string{"pre-commit", "pre-push", "ci", "pre-publish"}, o.boundary) || o.staged == (o.revision != "") || (o.staged && o.base != "")) {
		fmt.Fprintln(errout, "check requires a valid --boundary and exactly one of --staged or --revision")
		return 2
	}
	if command == "pre-push" && (o.staged || o.revision != "" || o.base != "" || o.index != "") {
		fmt.Fprintln(errout, "pre-push reads exact update rows from stdin")
		return 2
	}
	if command == "check" && o.index != "" && !o.staged {
		fmt.Fprintln(errout, "--index-file requires --staged")
		return 2
	}
	if o.policyFile != "" && command != "validate" && command != "site-digests" && !o.observe {
		fmt.Fprintln(errout, "enforcement requires committed policy; local files require --observe")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := gitSource{Dir: o.repo, IndexFile: o.index, GitBin: o.gitBin}
	if o.policyFile == "" || command == "check" || command == "pre-push" {
		resolved, e := trustedGitPath(o.repo, o.gitBin)
		if e != nil {
			fmt.Fprintln(errout, "cannot establish trusted Git runtime:", e)
			return 2
		}
		g.GitBin = resolved
	}
	data, commit, err := loadPolicy(ctx, g, o)
	if err != nil {
		fmt.Fprintln(errout, "cannot load policy:", err)
		return 2
	}
	p, err := decodePolicy(data)
	if err != nil {
		fmt.Fprintln(errout, "invalid policy:", err)
		return 2
	}
	if command == "validate" {
		fmt.Fprintln(out, "Policy valid:", p.ID)
		return 0
	}
	if command == "site-digests" {
		digests := map[string]string{}
		for _, s := range p.Model.Sites {
			digests[s.ID] = siteDigest(s, p.Model.Sites...)
		}
		return emitJSON(out, digests)
	}
	// The invocation deadline bounds all ref snapshots and provider calls together.
	ctx, finish := context.WithTimeout(context.Background(), time.Duration(p.Limits.TimeoutSeconds)*time.Second)
	defer finish()
	o.budget = newModelBudget(p.Model)
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])
	if command == "check" {
		var s snapshot
		if o.staged {
			s, err = g.staged(ctx, p.Limits)
		} else {
			s, err = g.revision(ctx, o.base, o.revision, p.Limits)
		}
		r, code := assess(ctx, g, p, o, commit, digest, s, err, model)
		if writeReport(out, o.format, r) != 0 {
			return 2
		}
		return code
	}
	updates, err := parsePushUpdates(in)
	if err != nil || len(updates) == 0 {
		fmt.Fprintln(errout, "missing or invalid pre-push update rows")
		return 2
	}
	reports := make([]report, 0, len(updates))
	preflight := o
	preflight.observe, preflight.deferModels = false, true
	preflight.shadowObservation = o.observe
	code := 0
	preflightBlocked := false
	for _, u := range updates {
		var s snapshot
		var captureErr error
		if isZeroOID(u.LocalOID) {
			s.Ref = u.RemoteRef
			r := report{Version: 1, PolicyID: p.ID, PolicyCommit: commit, PolicySHA256: digest, Boundary: o.boundary, Observe: o.observe, Snapshot: s, State: "pass", Reason: "ref deletion; no proposed tree", Rules: []ruleResult{}, Findings: []finding{}, Model: modelResult{Provider: p.Model.Provider, Cache: "none", Sites: []siteResult{}}}
			r.Coverage = coverageFor(p, o.boundary, s)
			if len(r.Coverage.RequiredRules)+len(r.Coverage.RequiredSites) == 0 {
				r.State, r.Reason, code = "unknown", noRequiredBoundaryReason, 2
				if !o.observe {
					preflightBlocked = true
				}
			}
			reports = append(reports, r)
			continue
		}
		base := u.RemoteOID
		if isZeroOID(base) {
			base = ""
		}
		s, captureErr = g.revision(ctx, base, u.LocalOID, p.Limits)
		// The advertised local OID is the input even if its ref moves later.
		s.Ref = ""
		r, rc := assess(ctx, g, p, preflight, commit, digest, s, captureErr, model)
		r.Snapshot.Ref = u.RemoteRef
		if rc > code {
			code = rc
		}
		if rc != 0 && !(o.observe && r.Reason == noRequiredBoundaryReason) {
			preflightBlocked = true
		}
		reports = append(reports, r)
	}
	// Preflight every ref before any disclosure or paid dispatch. One hard
	// failure makes model review of the other refs unnecessary.
	if !preflightBlocked {
		judgment := o
		judgment.observe = false
		judgment.shadowObservation = o.observe
		for i := range reports {
			if reports[i].Snapshot.Revision == "" {
				continue
			} // deletion has no tree
			s := reports[i].Snapshot
			s.Ref = "" // advertised OIDs, never remote ref names, are checked
			r, rc := assess(ctx, g, p, judgment, commit, digest, s, nil, model)
			r.Snapshot.Ref = reports[i].Snapshot.Ref
			reports[i] = r
			if rc > code {
				code = rc
			}
		}
	} else {
		for i := range reports {
			reports[i].Model.Reason = "push-structural-preflight-failed"
			for j := range reports[i].Model.Sites {
				site := &reports[i].Model.Sites[j]
				if site.Outcome == "pending" {
					site.Outcome, site.Reason, site.Held = "unknown", "push-preflight-failed", site.Mode == "hold"
				}
			}
			if reports[i].State == "pass" && hasRequiredModelMatch(p, o.boundary, reports[i].Snapshot) {
				reports[i].State, reports[i].Reason = "unknown", "required model sites skipped after push preflight failure"
				code = 2
			}
		}
	}
	// Recheck the trusted policy even on an all-deletion push.
	if commit != "" {
		if current, e := g.resolveCommit(ctx, o.policyRef); e != nil || current != commit {
			fmt.Fprintln(errout, "trusted policy ref changed")
			return 2
		}
	}
	if o.observe {
		for i := range reports {
			observeReport(&reports[i], stateExitCode(reports[i].State))
		}
		code = 0
	}
	if o.format == "json" {
		if emitJSON(out, reports) != 0 {
			return 2
		}
	} else {
		for _, r := range reports {
			if writeReport(out, o.format, r) != 0 {
				return 2
			}
		}
	}
	return code
}

func loadPolicy(ctx context.Context, g gitSource, o options) ([]byte, string, error) {
	if o.policyFile == "" {
		return g.policyBlob(ctx, o.policyRef, o.policyPath)
	}
	f, err := os.Open(o.policyFile)
	if err != nil {
		return nil, "", errors.New("local policy unavailable")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, policyBlobCap+1))
	if err != nil || len(b) > policyBlobCap {
		return nil, "", errors.New("local policy exceeds limit or cannot be read")
	}
	return b, "", nil
}

func assess(ctx context.Context, g gitSource, p policy, o options, commit, digest string, s snapshot, captureErr error, model modelEvaluator) (report, int) {
	r := report{Version: 1, PolicyID: p.ID, PolicyCommit: commit, PolicySHA256: digest, Boundary: o.boundary, Observe: o.observe, Snapshot: s, State: "pass", Rules: []ruleResult{}, Findings: []finding{}, Model: modelResult{Provider: p.Model.Provider, Cache: "not-run", Sites: []siteResult{}}}
	code := 0
	r.Coverage = coverageFor(p, o.boundary, s)
	if captureErr != nil {
		r.State = "unknown"
		r.Reason = "Git snapshot unavailable or exceeds limits"
		code = 2
	} else {
		r.Rules, r.Findings = evaluateRules(p, o.boundary, s)
		noRequired := len(r.Coverage.RequiredRules)+len(r.Coverage.RequiredSites) == 0
		if noRequired {
			r.State, r.Reason, code = "unknown", noRequiredBoundaryReason, 2
		}
		for _, v := range r.Rules {
			if v.State == "fail" && code < 1 {
				code = 1
				r.State = "hold"
			}
			if v.State == "unknown" {
				code = 2
				r.State = "unknown"
			}
		}
		if code == 0 || (noRequired && (o.observe || o.shadowObservation)) {
			if o.deferModels {
				r.Model = evaluateModelsWith(ctx, p, o.boundary, digest, s, modelOptions{PrepareOnly: true, Budget: o.budget, Now: o.now}, nil)
			} else {
				r.Model = model(ctx, p, o.boundary, digest, s, modelOptions{AllowRemote: o.allowRemote, CacheDir: o.cacheDir, Budget: o.budget, Now: o.now})
			}
			for _, v := range r.Model.Sites {
				if v.Held {
					if code < 1 {
						code = 1
						r.State = "hold"
					}
					if v.Outcome == "unknown" {
						code = 2
						r.State = "unknown"
					}
				}
			}
			if !o.deferModels {
				if reason := requiredSiteProblem(p, o, s, r.Model); reason != "" {
					r.State, r.Reason, code = "unknown", reason, 2
				}
			}
		} else {
			r.Model.Reason = "deterministic-check-failed"
		}
		if err := g.unchanged(ctx, s, o.staged); err != nil {
			r.State = "unknown"
			r.Reason = "Git input changed during evaluation"
			code = 2
		}
	}
	if commit != "" {
		if current, err := g.resolveCommit(ctx, o.policyRef); err != nil || current != commit {
			r.State = "unknown"
			r.Reason = "trusted policy ref changed"
			code = 2
		}
	}
	if ctx.Err() != nil {
		r.State = "unknown"
		r.Reason = "invocation deadline exceeded"
		code = 2
	}
	if o.observe {
		observeReport(&r, code)
		code = 0
	}
	return r, code
}

func observeReport(r *report, code int) {
	r.Observe, r.State = true, "observed"
	if r.Reason == "" {
		r.Reason = fmt.Sprintf("enforcement exit would be %d", code)
	}
}

func stateExitCode(state string) int {
	if state == "pass" {
		return 0
	}
	if state == "hold" {
		return 1
	}
	return 2
}

func hasRequiredModelMatch(p policy, boundary string, s snapshot) bool {
	for _, site := range p.Model.Sites {
		if site.Mode != "hold" || !includes(site.Boundaries, boundary) {
			continue
		}
		for _, c := range s.Changes {
			if matches(site.Select, c.Path) {
				return true
			}
		}
	}
	return false
}

func coverageFor(p policy, boundary string, s snapshot) boundaryCoverage {
	c := boundaryCoverage{RequiredRules: []string{}, RequiredSites: []string{}, ShadowSites: []string{}, UnselectedPaths: []string{}}
	for _, r := range p.Rules {
		if includes(r.Boundaries, boundary) {
			c.RequiredRules = append(c.RequiredRules, r.ID)
		}
	}
	for _, site := range p.Model.Sites {
		if includes(site.Boundaries, boundary) {
			if site.Mode == "hold" {
				c.RequiredSites = append(c.RequiredSites, site.ID)
			} else {
				c.ShadowSites = append(c.ShadowSites, site.ID)
			}
		}
	}
	for _, change := range s.Changes {
		selected := false
		for _, r := range p.Rules {
			if includes(r.Boundaries, boundary) && matches(r.Select, change.Path) {
				selected = true
			}
		}
		for _, site := range p.Model.Sites {
			if includes(site.Boundaries, boundary) && matches(site.Select, change.Path) {
				selected = true
			}
		}
		if !selected {
			c.UnselectedPaths = append(c.UnselectedPaths, change.Path)
		}
	}
	slices.Sort(c.UnselectedPaths)
	return c
}

// Required sites are enumerated from trusted policy, not from whatever the
// evaluator happens to return. A missing receipt cannot silently remove a floor.
func requiredSiteProblem(p policy, o options, s snapshot, r modelResult) string {
	for _, site := range p.Model.Sites {
		if site.Mode != "hold" || !includes(site.Boundaries, o.boundary) {
			continue
		}
		matched := false
		for _, c := range s.Changes {
			if matches(site.Select, c.Path) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		var result siteResult
		count := 0
		for _, got := range r.Sites {
			if got.ID == site.ID {
				result = got
				count++
			}
		}
		if count != 1 || result.Mode != "hold" {
			return "required model site receipt missing or ambiguous"
		}
		switch result.Outcome {
		case "concern", "insufficient":
			if !result.Held {
				return "required model site hold was not retained"
			}
		case "unknown":
			return "required model site could not be checked"
		case "clear":
			if result.Held || result.Confidence == nil || *result.Confidence < site.MinConfidence || result.Calibration != "pass" || r.Provider != p.Model.Provider || r.Model == "" || r.Model != os.Getenv(p.Model.ExpectedModelEnv) {
				return "required model clear lacks bound evidence"
			}
			now := o.now
			if now == nil {
				now = time.Now
			}
			if calibrationStatus(p.ID, o.boundary, site, expectedProvider(r.Provider), r.Model, "clear", *result.Confidence, now(), p.Model.Sites...) != "pass" {
				return "required model calibration is not current"
			}
		default:
			return "required model site returned an invalid outcome"
		}
	}
	return ""
}

func emitJSON(w io.Writer, v any) int {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	if e.Encode(v) != nil {
		return 2
	}
	return 0
}
func writeReport(w io.Writer, format string, r report) int {
	if format == "json" {
		return emitJSON(w, r)
	}
	var text strings.Builder
	fmt.Fprintf(&text, "quality-gate %s boundary=%s policy=%s tree=%s\n", r.State, r.Boundary, r.PolicyID, r.Snapshot.Tree)
	if r.Reason != "" {
		fmt.Fprintln(&text, r.Reason)
	}
	fmt.Fprintf(&text, "  coverage: %d rules, %d required sites, %d shadow sites; %d changed paths outside selectors\n", len(r.Coverage.RequiredRules), len(r.Coverage.RequiredSites), len(r.Coverage.ShadowSites), len(r.Coverage.UnselectedPaths))
	for _, path := range r.Coverage.UnselectedPaths[:min(20, len(r.Coverage.UnselectedPaths))] {
		fmt.Fprintf(&text, "    unselected: %q\n", path)
	}
	if len(r.Coverage.UnselectedPaths) > 20 {
		fmt.Fprintf(&text, "    %d more unselected paths; use --format json for the full list\n", len(r.Coverage.UnselectedPaths)-20)
	}
	if r.Model.Reason != "" {
		fmt.Fprintln(&text, "  model:", r.Model.Reason)
	}
	for _, f := range r.Findings {
		fmt.Fprintf(&text, "  %s %s %q:%d %s\n", f.RuleID, f.Kind, f.Path, f.Line, f.Reason)
	}
	for _, s := range r.Model.Sites {
		fmt.Fprintf(&text, "  site=%s mode=%s outcome=%s calibration=%s %s\n", s.ID, s.Mode, s.Outcome, s.Calibration, s.Reason)
	}
	if _, err := io.WriteString(w, text.String()); err != nil {
		return 2
	}
	return 0
}
