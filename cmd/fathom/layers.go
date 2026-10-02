package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// cmdLayers implements the "layers" subcommand: list/show/install semantic
// layer packs (LC-7, docs/plans/layer-catalog.md). Every verb is store-direct
// and read-mostly except install, which only ever writes to
// <root>/.fathom/layers/ — no daemon round-trip, mirroring `fathom claim`'s
// v0 store-direct-only convention.
//
// Presence = enabled — for COMPILATION (ADR-018 reconciler doctrine): a
// pack's presence is what lets `fathom compile --layer <name>` run; there is
// deliberately no "fathom layers enable/disable" verb. SERVING, however,
// derives from the STORE: claims a since-removed pack compiled keep serving
// until retired (refuted per claim via `fathom claim`, or superseded by a
// recompile) — removing a directory does not reach into the store.
// `layers list --store --service` surfaces such orphaned layers so an
// injected rule is always traceable to its (possibly departed) pack; a real
// `layers uninstall` that also retires the layer's claims is deferred
// (docs/plans/layer-catalog.md LC-13).
func cmdLayers(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: layers requires a verb: list | show | install | strata | coverage")
		printLayersUsage(stderr)
		return 1
	}
	switch args[0] {
	case "list":
		return cmdLayersList(args[1:], stdout, stderr)
	case "show":
		return cmdLayersShow(args[1:], stdout, stderr)
	case "install":
		return cmdLayersInstall(args[1:], stdout, stderr)
	case "strata":
		return cmdLayersStrata(args[1:], stdout, stderr)
	case "coverage":
		return cmdLayersCoverage(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown layers verb %q (want list | show | install | strata | coverage)\n", args[0])
		printLayersUsage(stderr)
		return 1
	}
}

func printLayersUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  fathom layers list    [--root DIR] [--store PATH --service NAME]")
	fmt.Fprintln(w, "  fathom layers show    <name> [--root DIR] | <name> --json [--store DB | --url U]")
	fmt.Fprintln(w, "  fathom layers install <dir>  [--root DIR] [--force]")
	fmt.Fprintln(w, "  fathom layers strata   [--json] [--store DB | --url U]")
	fmt.Fprintln(w, "  fathom layers coverage [--json] [--store DB | --url U]")
}

// layersRoot resolves --root, defaulting to the current working directory —
// `fathom layers` commands are run from a project's repo root the same way
// `fathom ingest`/`fathom compile --store` are.
func layersRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return wd, nil
}

func cmdLayersList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("layers list", stderr)
	root := fs.String("root", "", "project root to union with embedded packs (default: current directory)")
	storePath := fs.String("store", "", "store path, .sqlite/.db (optional — adds compiled-claims counts)")
	service := fs.String("service", "", "service ID (required with --store, for claims counts)")
	jsonOut := fs.Bool("json", false, "emit the catalog as JSON (docs/usage.md §layers)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath != "" && *service == "" {
		fmt.Fprintln(stderr, "error: --store requires --service")
		return 1
	}

	r, err := layersRoot(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	infos, err := semantic.ListLayersIn(r)
	if err != nil {
		fmt.Fprintf(stderr, "error: list layers: %v\n", err)
		return 1
	}

	var claimCounts map[string]int
	var lastCompile map[string]string
	if *storePath != "" {
		claimCounts, err = layersClaimCounts(*storePath, *service)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		lastCompile, err = layersLastCompile(*storePath, *service)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	if *jsonOut { // RP-17: layers_json.go
		return jsonExit(renderLayersListJSON(stdout, r, infos, claimCounts, lastCompile), stderr)
	}
	fmt.Fprintf(stdout, "root: %s\n\n", r)
	if len(infos) == 0 {
		fmt.Fprintln(stdout, "(no layers found)")
		return 0
	}
	// A4-1 (docs/plans/activation.md): catalog de-theater — EVAL (schema
	// data, LayerDef.Evaled) is always shown; PRODUCTION (CLAIMS +
	// LAST_COMPILE, read live from the store) appears only with --store —
	// two separate, honestly-sourced columns instead of one hand-maintained
	// "gate result" blob.
	header := "NAME                 SOURCE     KIND               TIER          STRATUM   EVAL       DRIFT  CORPUS"
	if claimCounts != nil {
		header += "  CLAIMS  LAST_COMPILE"
	}
	fmt.Fprintln(stdout, header)
	var broken []semantic.LayerInfo
	for _, info := range infos {
		if info.LoadError != "" && info.Kind == "" {
			// Project-only broken pack: no usable definition to tabulate.
			broken = append(broken, info)
			continue
		}
		source := string(info.Source)
		if info.Overrides {
			source += "*"
		}
		drift := "no"
		if info.Serving.DriftTracked {
			drift = "yes"
		}
		eval := "not-gated"
		if info.Evaled {
			eval = "pass"
		}
		// stratum (ADR-024, docs/design/pack-taxonomy.md): schema data
		// (LayerDef.Stratum), defaulted "concern" by #Layer's CUE default —
		// never empty for a successfully decoded pack.
		stratum := info.Stratum
		if stratum == "" {
			stratum = "concern"
		}
		line := fmt.Sprintf("%-20s %-10s %-18s %-13s %-9s %-10s %-6s %s",
			info.Name, source, info.Kind, info.Tier, stratum, eval, drift, strings.Join(info.CorpusInclude, ", "))
		if claimCounts != nil {
			last := lastCompile[info.Name]
			if last == "" {
				last = "never"
			}
			line += fmt.Sprintf("  %d  %s", claimCounts[info.Name], last)
		}
		fmt.Fprintln(stdout, line)
		if info.LoadError != "" {
			// A broken project pack shadowing this embedded one: the embedded
			// definition above is what serves; say why the override didn't.
			broken = append(broken, info)
		}
	}
	if hasOverride(infos) {
		fmt.Fprintln(stdout, "\n* project pack overrides an embedded pack of the same name")
	}
	for _, b := range broken {
		fmt.Fprintf(stdout, "\n! broken project pack %q (ignored; fix or remove its directory): %s\n", b.Name, b.LoadError)
	}
	if claimCounts != nil {
		known := map[string]bool{}
		for _, info := range infos {
			known[info.Name] = true
		}
		var orphans []string
		for layer := range claimCounts {
			if !known[layer] {
				orphans = append(orphans, layer)
			}
		}
		sort.Strings(orphans)
		if len(orphans) > 0 {
			fmt.Fprintln(stdout, "\norphaned layers — claims still served from the store, but no pack (embedded or project) defines them:")
			for _, o := range orphans {
				fmt.Fprintf(stdout, "  %-20s %d claims (serving derives from the store, not pack presence — retire per claim via `fathom claim refute`, or recompile; see docs/layers.md)\n", o, claimCounts[o])
			}
		}
	}
	return 0
}

func hasOverride(infos []semantic.LayerInfo) bool {
	for _, i := range infos {
		if i.Overrides {
			return true
		}
	}
	return false
}

// layersClaimCounts opens storePath directly (mirrors `fathom inspect`'s
// store-direct convention) and counts served, non-superseded/refuted claims
// per layer for service.
func layersClaimCounts(storePath, service string) (map[string]int, error) {
	st, err := store.Open(storePath)
	if err != nil {
		return nil, fmt.Errorf("open store %q: %w", storePath, err)
	}
	defer st.Close()
	claims, err := st.ClaimsForService(context.Background(), service, "")
	if err != nil {
		return nil, fmt.Errorf("load claims for %q: %w", service, err)
	}
	counts := map[string]int{}
	for _, cl := range claims {
		counts[cl.Layer]++
	}
	return counts, nil
}

// layersLastCompile opens storePath directly and returns, per layer, the
// most recent recorded compile_runs.ran_at for service (A4-1, docs/plans/
// activation.md) — "production" truth's date half, read live from the
// store, never hand-maintained. A layer with no compile_runs row for this
// service is simply absent from the map (cmdLayersList renders "never").
func layersLastCompile(storePath, service string) (map[string]string, error) {
	st, err := store.Open(storePath)
	if err != nil {
		return nil, fmt.Errorf("open store %q: %w", storePath, err)
	}
	defer st.Close()
	runs, err := st.CompileRuns(context.Background(), service, "")
	if err != nil {
		return nil, fmt.Errorf("load compile runs for %q: %w", service, err)
	}
	// CompileRuns is newest-first per (service, layer) — but layer "" spans
	// every layer interleaved, so track the max explicitly rather than
	// trusting the first hit per layer.
	latest := map[string]int64{}
	dates := map[string]string{}
	for _, r := range runs {
		nanos := r.RanAt.UnixNano()
		if prev, ok := latest[r.Layer]; ok && prev >= nanos {
			continue
		}
		latest[r.Layer] = nanos
		dates[r.Layer] = r.RanAt.UTC().Format("2006-01-02")
	}
	return dates, nil
}

func cmdLayersShow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("layers show", stderr)
	root := fs.String("root", "", "project root to union with embedded packs (default: current directory)")
	// OC-29: --json is the console layer page's projection — the pack as
	// the fleet resolves it plus its compile runs, rejection reasons and
	// uncompiled scopes, read from the daemon's store (knowledge_verbs.go).
	kf := addKnowledgeFlags(fs)
	if err := fs.Parse(reorderFlagsFirst(fs, args)); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: layers show requires exactly one layer name")
		printLayersUsage(stderr)
		return 1
	}
	name := fs.Arg(0)
	if *kf.json {
		return cmdLayersShowJSON(name, kf, stdout, stderr)
	}

	r, err := layersRoot(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	infos, err := semantic.ListLayersIn(r)
	if err != nil {
		fmt.Fprintf(stderr, "error: list layers: %v\n", err)
		return 1
	}
	var found *semantic.LayerInfo
	for i := range infos {
		if infos[i].Name == name {
			found = &infos[i]
			break
		}
	}
	if found == nil {
		names := make([]string, len(infos))
		for i, info := range infos {
			names[i] = info.Name
		}
		sort.Strings(names)
		fmt.Fprintf(stderr, "error: unknown layer %q (known layers: %s)\n", name, strings.Join(names, ", "))
		return 1
	}

	source := string(found.Source)
	if found.Overrides {
		source += " (overrides an embedded pack of the same name)"
	}
	if found.LoadError != "" {
		if found.Kind == "" {
			fmt.Fprintf(stdout, "name:              %s\n", found.Name)
			fmt.Fprintf(stdout, "source:            %s (BROKEN)\n", source)
			fmt.Fprintf(stdout, "load error:        %s\n", found.LoadError)
			fmt.Fprintln(stdout, "status:            this project pack failed to load and is ignored; fix or remove its directory")
			return 0
		}
		fmt.Fprintf(stdout, "load error:        a broken project pack of this name was ignored (the embedded definition below serves): %s\n", found.LoadError)
	}
	fmt.Fprintf(stdout, "name:              %s\n", found.Name)
	fmt.Fprintf(stdout, "version:           %s\n", found.Version)
	fmt.Fprintf(stdout, "kind:              %s\n", found.Kind)
	fmt.Fprintf(stdout, "source:            %s\n", source)
	fmt.Fprintf(stdout, "tier:              %s (profile: %s)\n", found.Tier, found.TierProfile)
	stratum := found.Stratum
	if stratum == "" {
		stratum = "concern"
	}
	fmt.Fprintf(stdout, "stratum:           %s (docs/design/pack-taxonomy.md)\n", stratum)
	evalStatus := "not-gated (no passing eval/RESULTS.md on record — fathom compile refuses this layer without --unsafe-skip-eval-gate)"
	if found.Evaled {
		evalStatus = "pass (see eval/RESULTS.md)"
	}
	fmt.Fprintf(stdout, "eval:              %s\n", evalStatus)
	fmt.Fprintf(stdout, "corpus include:    %s\n", strings.Join(found.CorpusInclude, ", "))
	if len(found.CorpusExclude) > 0 {
		fmt.Fprintf(stdout, "corpus exclude:    %s\n", strings.Join(found.CorpusExclude, ", "))
	}
	fmt.Fprintf(stdout, "prompt_file:       %s\n", found.PromptFile)
	if found.PromptText == "" {
		fmt.Fprintln(stdout, "prompt provenance: UNRESOLVED — prompt.md missing or unreadable; this layer cannot compile until it is added")
	} else {
		_, version, perr := semantic.ResolvePrompt(found.LayerDef)
		if perr != nil {
			fmt.Fprintf(stdout, "prompt provenance: error resolving compile identity: %v\n", perr)
		} else {
			fmt.Fprintf(stdout, "prompt provenance: %s (content-addressed from prompt.md, %d bytes)\n", version, len(found.PromptText))
		}
	}
	fmt.Fprintf(stdout, "budgets:           max_tokens_per_run=%d max_usd_per_run=%.4f max_rule_words=%d max_claims_per_file=%d\n",
		found.Budgets.MaxTokensPerRun, found.Budgets.MaxUSDPerRun, found.Budgets.MaxRuleWords, found.Budgets.MaxClaimsPerFile)
	fmt.Fprintf(stdout, "serving:           max_rules=%d max_tokens=%d drift_tracked=%v\n",
		found.Serving.MaxRules, found.Serving.MaxTokens, found.Serving.DriftTracked)
	return 0
}

func cmdLayersInstall(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("layers install", stderr)
	root := fs.String("root", "", "project root to install into (default: current directory)")
	force := fs.Bool("force", false, "replace an already-installed pack of the same name (whole-directory replace, never a merge)")
	// J6/O8: flags are accepted in EITHER position relative to the
	// positional pack directory (flagreorder.go) — flag.FlagSet.Parse
	// alone stops scanning for flags at the first non-flag token, which
	// silently mis-parsed the documented `<dir> --root DIR` order as three
	// positional arguments.
	if err := fs.Parse(reorderFlagsFirst(fs, args)); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: layers install requires exactly one pack directory")
		printLayersUsage(stderr)
		return 1
	}
	srcDir := fs.Arg(0)

	r, err := layersRoot(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	def, warnings, err := semantic.InstallPack(srcDir, r, *force)
	if err != nil {
		fmt.Fprintf(stderr, "error: install pack: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "installed: %s\n", def.Name)
	fmt.Fprintf(stdout, "into:      %s\n", r+"/"+semantic.ProjectLayersDir+"/"+def.Name)
	for _, w := range warnings {
		fmt.Fprintf(stdout, "warning:   %s\n", w)
	}
	return 0
}

// layersRunner runs `fathom layers` (registered below; EW-45).
type layersRunner struct{}

func (layersRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdLayers(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "layers", Group: "plans", Order: 60, Run: layersRunner{},
		Summary: "List/show/install semantic-layer packs (embedded + project .fathom/layers/) | strata: the four strata with claims, lifecycle, runs and cost | coverage: claims per project and layer, missing as not compiled | show <name> --json: compile runs, rejections and uncompiled scopes"})
}
