package main

// obligations.go — `fathom obligations` (OC-32, ADR-045 consequence
// "obligations gains an additive --explain"): what governs a file, as the
// obligations tool serves it to an agent — the raw-fallback rows for an
// uncompiled governing guide first, then the compiled rules — and, with
// --explain, why: every scope-matched candidate by scope tier, the ranking
// and the cap, and each displaced rule's reason (semantic.
// ExplainObligations, run through servingExplanation in
// console_explain.go, the same function /explain renders).
//
// CLI only, by the surface rules: the agent surface's argument schemas are
// golden-pinned (TestAgentSurfaceArgSchemaUnchanged) and every byte of
// them is paid by every connected session, while the explanation is read
// by a person. The MCP obligations tool is unchanged; its default payload
// and this verb's default output are the same served list.
//
// Beside each served rule the verb shows what the edit hook recorded of it
// — served, top_changed, still_served over the last 30 days (SR-61, F-27/U4;
// ruleoutcome_read.go reads the hook's signal log). That record and the
// rule's claim id are read here, by a person; the MCP payload an agent
// reads carries neither (the id rides only to the edit hook itself).
//
// Beside the compiled rules it shows the `.fathom/guard.cue` layer rules
// that govern the path, each marked checkable or not (SR-45w, F-26:
// guardspec.LayerRuleIsCheckable — a deterministic pre-write check exists
// for its kind), and `fathom obligations promote <rule>` compiles a
// checkable one to an enforce:true guard (guardspec.GuardFromCheckableRule):
// a dry run by default, `--write` to apply, a not-checkable rule refused
// with its reason. CLI only, like the rest of this verb's additions.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// servedObligationsJSON is the default --json: the list an agent is served.
type servedObligationsJSON struct {
	Service         string                                `json:"service"`
	File            string                                `json:"file"`
	Layer           string                                `json:"layer"`
	Rules           []semantic.Obligation                 `json:"rules"`
	StaleWithheld   int                                   `json:"stale_withheld,omitempty"`
	RawWhy          string                                `json:"raw_why,omitempty"`
	Displaced       int                                   `json:"displaced,omitempty"`
	DisplacedByTier map[string]int                        `json:"displaced_by_tier,omitempty"`
	Deduped         int                                   `json:"deduped,omitempty"`
	RankedBy        string                                `json:"ranked_by"`
	Watermark       string                                `json:"compiled_at_watermark,omitempty"`
	Coverage        console.Value[semantic.LayerCoverage] `json:"coverage"`
	Note            string                                `json:"note,omitempty"`
	// Outcomes is the edit hook's record of each served rule, keyed by
	// claim id (SR-61); absent, never zero, when no hook has recorded yet.
	Outcomes console.Value[console.RuleOutcomeSet] `json:"outcomes"`
	// LayerRules are the `.fathom/guard.cue` layer rules governing the
	// path, each marked checkable (SR-45w).
	LayerRules servedLayerRules `json:"layer_rules"`
}

func servedJSON(se console.ServingExplanation, lr servedLayerRules) servedObligationsJSON {
	return servedObligationsJSON{
		Service: se.Service, File: se.File, Layer: se.Layer, Rules: se.AgentSees, StaleWithheld: se.StaleWithheld, RawWhy: se.RawWhy,
		Displaced: se.Served.Displaced, DisplacedByTier: se.Served.DisplacedByTier, Deduped: se.Deduped,
		RankedBy: se.Served.RankedBy, Watermark: se.Served.Watermark, Coverage: se.Coverage, Note: se.CoverageNote,
		Outcomes: se.Outcomes, LayerRules: lr,
	}
}

// servedLayerRule is one guard.cue layer rule as the answer marks it.
// Why is the reason a not-checkable rule stays advisory, or, for a
// checkable one not yet enforced, the command that promotes it.
type servedLayerRule struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Sentence  string `json:"sentence"`
	Severity  string `json:"severity"`
	Enforce   bool   `json:"enforce"`
	Checkable bool   `json:"checkable"`
	Why       string `json:"why,omitempty"`
}

// servedLayerRules is the answer's layer-rule section: the file read and
// the rules governing the path, or Note naming why none were read (never
// an empty list standing in for "no rule governs this path").
type servedLayerRules struct {
	Source string            `json:"source,omitempty"`
	Rules  []servedLayerRule `json:"rules"`
	Note   string            `json:"note,omitempty"`
}

// layerRulesServed reads <root>/.fathom/guard.cue's layer rules and keeps
// those governing file of service (guardspec.LayerRuleGoverns), marking
// each with guardspec.LayerRuleIsCheckable.
func layerRulesServed(root, service, file string) servedLayerRules {
	out := servedLayerRules{Rules: []servedLayerRule{}}
	if root == "" {
		out.Note = "not read: " + service + " has no local root on this machine; pass --root"
		return out
	}
	path := projectdir.Guard.Path(root)
	if _, err := os.Stat(path); err != nil {
		out.Note = "no layer rules: " + path + " does not exist"
		return out
	}
	out.Source = path
	set, err := guardspec.LoadLayerRules(path)
	if err != nil {
		out.Note = "not read: " + err.Error()
		return out
	}
	for _, r := range set.Rules {
		if !guardspec.LayerRuleGoverns(r, service, file) {
			continue
		}
		sr := servedLayerRule{
			ID: r.ID, Kind: string(r.Kind), Sentence: r.Sentence, Severity: string(r.Severity),
			Enforce: r.Enforce, Checkable: guardspec.LayerRuleIsCheckable(r),
		}
		switch {
		case !sr.Checkable:
			sr.Why = guardspec.NotCheckableReason(r)
		case !r.Enforce:
			sr.Why = "fathom obligations promote " + r.ID + " --root " + root + " --write"
		}
		out.Rules = append(out.Rules, sr)
	}
	return out
}

// obligationsRoot is the directory whose guard.cue the answer reads: the
// --root flag, else the service's recorded ingest root when it is a
// directory on this machine, else "" (the answer then says so).
func obligationsRoot(ctx context.Context, flagRoot string, st store.Store, service string) string {
	if flagRoot != "" {
		return flagRoot
	}
	meta, found, err := st.GetServiceMeta(ctx, service)
	if err != nil || !found || meta.Path == "" {
		return ""
	}
	if fi, err := os.Stat(meta.Path); err != nil || !fi.IsDir() {
		return ""
	}
	return meta.Path
}

// cmdObligationsPromote implements `fathom obligations promote <rule>`
// (SR-45w): compile the layer rule to an enforce:true guard and print the
// stanza (dry run), or write it into <root>/.fathom/guard.cue (--write).
func cmdObligationsPromote(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("obligations promote", stderr)
	root := fs.String("root", ".", "repository root whose .fathom/guard.cue holds the rule")
	write := fs.Bool("write", false, "write the promoted rule into .fathom/guard.cue (default: a dry run that prints it)")
	if err := fs.Parse(reorderFlagsFirst(fs, args)); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: fathom obligations promote <layer-rule-id> [--root DIR] [--write]")
		return 1
	}
	id := fs.Arg(0)
	path := projectdir.Guard.Path(*root)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v%s\n", err, tccWedgeHint(path))
		return 1
	}
	p, err := guardspec.PromoteLayerRuleSource(path, data, id)
	if err != nil {
		var nc *guardspec.NotCheckableError
		if errors.As(err, &nc) {
			fmt.Fprintf(stderr, "refused: %v\n", err)
		} else {
			fmt.Fprintf(stderr, "error: %v\n", err)
		}
		return 1
	}
	const effect = "the PreToolUse guard hook then denies an edit that breaks it (FATHOM_ENFORCE=off or [guards] enforce = false keeps it advisory)"
	switch {
	case !p.Changed:
		fmt.Fprintf(stdout, "%s is already enforce: true in %s; nothing to write\n\n%s\n", id, path, p.Stanza)
	case !*write:
		fmt.Fprintf(stdout, "would promote %s to enforce: true in %s (dry run; --write applies it):\n\n%s\n\n%s\n", id, path, p.Stanza, effect)
	default:
		fi, serr := os.Stat(path)
		mode := os.FileMode(0o644)
		if serr == nil {
			mode = fi.Mode().Perm()
		}
		if err := os.WriteFile(path, p.Source, mode); err != nil {
			fmt.Fprintf(stderr, "error: write %s: %v\n", path, err)
			return 1
		}
		fmt.Fprintf(stdout, "promoted %s to enforce: true in %s:\n\n%s\n\n%s\n", id, path, p.Stanza, effect)
	}
	return 0
}

// printLayerRules is the text answer's layer-rule section.
func printLayerRules(w io.Writer, lr servedLayerRules) {
	if lr.Note != "" && len(lr.Rules) == 0 {
		fmt.Fprintf(w, "\nlayer rules: %s\n", lr.Note)
		return
	}
	if len(lr.Rules) == 0 {
		fmt.Fprintf(w, "\nlayer rules (%s): none governs this path\n", lr.Source)
		return
	}
	fmt.Fprintf(w, "\nlayer rules (%s):\n", lr.Source)
	for _, r := range lr.Rules {
		state := "advisory"
		if r.Enforce {
			state = "enforced"
		}
		mark := "checkable"
		if !r.Checkable {
			mark = "not checkable"
		}
		fmt.Fprintf(w, "  %s [%s] %s · %s\n      %s\n", r.ID, r.Kind, state, mark, r.Sentence)
		if r.Why != "" {
			if r.Checkable {
				fmt.Fprintf(w, "      promote: %s\n", r.Why)
			} else {
				fmt.Fprintf(w, "      %s\n", r.Why)
			}
		}
	}
}

func cmdObligations(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "promote" {
		return cmdObligationsPromote(args[1:], stdout, stderr)
	}
	fs := newFlagSet("obligations", stderr)
	rootFlag := fs.String("root", "", "repository root whose .fathom/guard.cue layer rules the answer marks checkable (default: the service's recorded root)")
	service := fs.String("service", "", "service ID (required)")
	file := fs.String("file", "", "repo-relative file being edited; omit for the root-scope question")
	layer := fs.String("layer", "rules", `which claims to serve: "rules" (the obligations tool's default), "claims" (plan-row bindings) or "all"`)
	explain := fs.Bool("explain", false, "explain the answer: every candidate by scope tier, the ranking, the cap, and each displaced rule's reason")
	format := fs.String("format", "", "output format: text (default) | sarif — SARIF 2.1.0 of the served rules, one rule per tier, located at each rule's own citation (overrides --json, --explain unread)")
	kf := addKnowledgeFlags(fs)
	if err := fs.Parse(reorderFlagsFirst(fs, args)); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		return 1
	}
	switch semantic.ObligationLayerFilter(*layer) {
	case semantic.ObligationLayerRules, semantic.ObligationLayerClaims, semantic.ObligationLayerAll:
	default:
		fmt.Fprintf(stderr, "error: --layer must be rules, claims or all, got %q\n", *layer)
		return 1
	}
	switch *format {
	case "", "sarif":
	default:
		fmt.Fprintf(stderr, "error: --format %q: want sarif\n", *format)
		return 1
	}
	ks := resolveKnowledgeStore(*kf.store, *kf.url)
	st, err := openKnowledgeStore(ks)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	c := knowledgeClient(st)
	defer c.Close()
	se, err := servingExplanation(context.Background(), c, *service, *file, semantic.ObligationLayerFilter(*layer), absStorePath(ks.Path))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	se = withRuleOutcomes(se, ruleOutcomeNow())
	oblRoot := obligationsRoot(context.Background(), *rootFlag, c.Store(), *service)
	layerRules := layerRulesServed(oblRoot, *service, *file)
	switch {
	case *format == "sarif":
		// EVX-3: anchored at the service root's commit against the
		// index's stored fingerprints (the rules were served from it).
		run := receipt.Run{Dir: oblRoot, Service: *service, Stored: func(p string) (string, bool) {
			fp, found, ferr := c.SourceFingerprint(context.Background(), *service, p)
			return fp, ferr == nil && found
		}}
		_, _ = stdout.Write(cliSARIF(run, "fathom obligations", obligationsSARIF(se.AgentSees)))
		return 0
	case *kf.json && *explain:
		return jsonExit(writeKnowledgeJSON(stdout, se), stderr)
	case *kf.json:
		return jsonExit(writeKnowledgeJSON(stdout, servedJSON(se, layerRules)), stderr)
	}
	subject := "governs " + se.File
	if se.File == "" {
		subject = "governs the repository root"
	}
	fmt.Fprintf(stdout, "what %s in %s (layer %s; store %s)\n\n", subject, se.Service, se.Layer, ks)
	if len(se.AgentSees) == 0 && se.StaleWithheld == 0 {
		fmt.Fprintln(stdout, "no rule is served — nothing compiled governs this path")
	}
	for i, r := range se.AgentSees {
		flag := ""
		if r.Status != "" {
			flag = " [" + r.Status + "]"
		}
		fmt.Fprintf(stdout, "%2d. [%s]%s %s\n      %s (%s)\n", i+1, r.Tier, flag, r.Rule, r.Src, r.Layer)
		if r.Why != "" {
			fmt.Fprintf(stdout, "      %s\n", r.Why)
		}
		if line := ruleOutcomeText(se.Outcomes, r.ClaimID); line != "" {
			fmt.Fprintf(stdout, "      %s\n", line)
		}
	}
	if se.StaleWithheld > 0 {
		fmt.Fprintf(stdout, "\n%s\n", staleWithheldLine(se.StaleWithheld))
	}
	if !se.Outcomes.Known() && len(se.AgentSees) > 0 {
		fmt.Fprintf(stdout, "\nhook outcomes: %s\n", se.Outcomes.Reason)
	}
	if se.CoverageNote != "" {
		fmt.Fprintf(stdout, "\nnote: %s\n", se.CoverageNote)
	}
	printLayerRules(stdout, layerRules)
	if !*explain {
		if se.Served.Displaced > 0 {
			fmt.Fprintf(stdout, "\n%d eligible rule(s) displaced by the cap — add --explain to see which and why\n", se.Served.Displaced)
		}
		return 0
	}
	fmt.Fprintf(stdout, "\ncap %d · %d eligible · %d served · %d displaced · %d duplicate(s) not repeated · ranked by %s\n",
		se.Cap, se.Eligible, len(se.Served.Rules), se.Displaced, se.Deduped, strings.Join(se.Ranking, " → "))
	fmt.Fprintf(stdout, "re-rank: %s\n", se.Rerank)
	for _, t := range se.Tiers {
		fmt.Fprintf(stdout, "\n%s — %d candidate(s): %d served, %d displaced, %d duplicate(s)\n", strings.ToUpper(strings.ReplaceAll(t.Tier, "_", " ")), t.Candidates, t.Served, t.Displaced, t.Duplicates)
		for _, cand := range se.Candidates {
			if cand.ScopeTier != t.Tier {
				continue
			}
			state := fmt.Sprintf("served #%d", cand.Position)
			if cand.Reserved {
				state += " (reserved slot)"
			}
			if cand.Displaced != nil {
				state = cand.Displaced.Reason + ": " + cand.Displaced.Why
			}
			fmt.Fprintf(stdout, "  #%-3d %s\n       %s · %s\n", cand.Rank, cand.Rule, cand.Src, state)
		}
	}
	return 0
}

// staleWithheldLine names the compiled rules the I38 withhold took out of
// the served list (O1149), as the verb and /explain print it.
func staleWithheldLine(n int) string {
	return plural(n, "compiled rule") + " withheld: the section each was compiled from changed, or its file is gone, since the compile (I38); recompile the layer to serve them again"
}

// ruleOutcomeText is one served rule's hook record as the CLI prints it:
// "hook, last 30 days: served 3 · top changed 1 · still served 1", with the
// human-verdict counts only when there are any and GL-11's class — a
// proposal, never an action — when one applies. "" when the record is not
// known or the rule has no claim id (a raw-fallback row has none).
func ruleOutcomeText(v console.Value[console.RuleOutcomeSet], claimID string) string {
	if !v.Known() || claimID == "" {
		return ""
	}
	c, ok := v.V.ByClaim(claimID)
	if !ok {
		return ""
	}
	line := fmt.Sprintf("hook, last %d days: served %d · top changed %d · still served %d", v.V.WindowDays, c.Served, c.TopChanged, c.StillServed)
	if c.Blocked > 0 {
		line += fmt.Sprintf(" · blocked %d", c.Blocked)
	}
	if c.Refuted > 0 || c.Corroborated > 0 {
		line += fmt.Sprintf(" · refuted %d · corroborated %d", c.Refuted, c.Corroborated)
	}
	if c.Class != "" {
		line += fmt.Sprintf(" [%s — proposal: %s]", strings.ReplaceAll(c.Class, "_", " "), c.Remedy)
	}
	return line
}

// obligationsRunner runs `fathom obligations` (registered below; EW-45).
type obligationsRunner struct{}

func (obligationsRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdObligations(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "obligations", Group: "plans", Order: 70, Run: obligationsRunner{},
		Summary: "What governs a file, as served to an agent: --service S [--file F] [--layer rules|claims|all] | --explain: every candidate by scope tier, the ranking, the cap, and why each displaced rule lost; marks each guard.cue layer rule checkable | promote <rule> [--write]: compile a checkable layer rule to an enforce:true guard"})
}
