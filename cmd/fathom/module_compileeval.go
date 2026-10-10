package main

// module_compileeval.go — `fathom module compile` / `fathom module eval`:
// thin wrappers over the EXISTING compile/eval machinery (docs/vision/13-
// expert-suite.md §4/§9 row J: "do not fork the engines — wire them"),
// reading identity + corpus from the manifest instead of ad-hoc flags.
//
// Design decision (documented here and in docs/plans/expert-suite-j.md):
// a module's COMPILE identity is a project-local semantic-layer pack of the
// SAME NAME as the module (.fathom/layers/<module.name>/{layer.cue,
// prompt.md}, LC-3's LoadLayerIn) — module.cue does not itself carry a
// compile prompt (schema/module/schema.cue's #Module has no such field;
// it is packaging metadata about an already-compiled store, per docs/spec/
// expert-module-v0.md §1: "a module is fathom's ordinary store format,
// packaged with a manifest"). `module compile` therefore delegates to the
// EXACT SAME compileViaStore (cmd/fathom/compile.go) the top-level `fathom
// compile` command already uses — every guard (D3-1 eval-gate, A2-1
// idempotency), every budget check, every digest gate is reused unchanged;
// this file only resolves --layer from the manifest's own name and adds
// --json.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/modspec"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// scopedWatermarkConnector wraps a connector.Connector, delegating Discover
// and Fetch unchanged but overriding Watermark to the MODULE-SCOPED
// algorithm (moduleWatermark, module.go) instead of the wrapped connector's
// own whole-directory default. pkg/client.Client.CompileLayerWithEscape
// resolves a service's compile-time watermark via c.ServiceWatermark, which
// simply calls the injected Connector's own Watermark method — so wrapping
// the connector at construction time is a legitimate, additive way to
// change WHICH watermark a compile records, without touching
// CompileLayerWithEscape/compileViaStore's own frozen logic (shared with
// the top-level `fathom compile` command for every non-module layer).
//
// Used ONLY by the mock compile lane (moduleCompileMock, moduleCompileDry
// RunJSON) — the lane every test and scripts/module-cold-run.sh actually
// exercises. Phase J critic finding 3 ("compile records connector.Watermark
// (whole-directory) while doctor computes moduleWatermark (corpus-scoped) —
// incomparable even if you tried") is closed for this lane by construction:
// a mock-compiled module's compile_runs.CorpusWatermark is the SAME
// algorithm module.cue's own watermark field uses, so module_map.go's
// "compile freshness" check can compare them directly without noise from
// sibling files (module.cue itself, eval artifacts) sharing the corpus
// root.
//
// KNOWN LIMITATION, documented rather than silently assumed away: the REAL
// (non-mock) `module compile` path calls compileViaStore directly —
// unmodified, on purpose, since it is the exact same function the
// top-level `fathom compile` command uses for every ordinary layer, and
// this phase does not fork it. A real module compile therefore still
// records the whole-directory watermark, which IS sensitive to unrelated
// sibling-file edits (e.g. hand-editing module.cue's eval_evidence after a
// passing eval) in a way the mock lane is not. See docs/plans/expert-
// suite-j.md's critic-round design note.
type scopedWatermarkConnector struct {
	connector.Connector
	root     string
	manifest modspec.Manifest
}

func (c *scopedWatermarkConnector) Watermark(ctx context.Context, _ core.ServiceRef) (string, error) {
	return moduleWatermark(ctx, c.root, c.manifest)
}

// ─── compile ────────────────────────────────────────────────────────────

func cmdModuleCompile(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("module compile", stderr)
	manifestPath := fs.String("manifest", "", "path to module.cue (required)")
	storePath := fs.String("store", "", "local store path (required — module compile is direct-store-mode only)")
	service := fs.String("service", "", "service ID already ingested in --store (default: manifest name)")
	dryRun := fs.Bool("dry-run", true, "estimate cost without calling the LLM (default true; use --dry-run=false to spend)")
	profile := fs.String("profile", "", "named LLM profile (see --profile of 'fathom compile')")
	force := fs.Bool("force", false, "override the idempotency guard (A2-1)")
	unsafeSkipEvalGate := fs.Bool("unsafe-skip-eval-gate", false, "override the eval-gate refusal (D3-1)")
	mock := fs.Bool("mock", false, "DEV/COLD-RUN LANE ONLY: use a scripted MockClient instead of a real LLM (--mock-entries-file required). Bypasses the D3-1 eval-gate and A2-1 idempotency guards compileViaStore enforces for a real compile — never use for a module you intend to publish.")
	mockEntriesFile := fs.String("mock-entries-file", "", "path to a JSON array of scripted response strings, consumed in order (--mock only)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *manifestPath == "" || *storePath == "" {
		fmt.Fprintln(stderr, "error: --manifest and --store are required")
		fs.Usage()
		return 1
	}
	if *mock && *mockEntriesFile == "" {
		fmt.Fprintln(stderr, "error: --mock requires --mock-entries-file")
		return 1
	}
	m, err := modspec.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	svc := *service
	if svc == "" {
		svc = m.Name
	}

	// Advisory cross-check (never a hard refusal — mirrors cmdCompile's own
	// "unknown at CWD" advisory pattern): a module pack's corpus should
	// match the manifest's own declared corpus; a mismatch is a doctor
	// signal, not a compile-time error, since a module author may
	// deliberately be mid-iteration on either file.
	root := filepath.Dir(*manifestPath)
	if layerDef, lerr := semantic.LoadLayerIn(root, m.Name); lerr != nil {
		fmt.Fprintf(stderr, "warning: module compile expects a project-local layer pack named %q (a %s/.fathom/layers/%s/ directory, LC-3) — a module's compile identity is its same-named layer pack, module.cue's own corpus/tier act as the servable artifact's declared identity: %v\n", m.Name, root, m.Name, lerr)
	} else if !sameCorpus(layerDef.CorpusInclude, m.Corpus.Include) {
		fmt.Fprintf(stderr, "warning: layer pack %q's corpus.include %v does not match module.cue's corpus.include %v — `fathom module doctor` will flag this\n", m.Name, layerDef.CorpusInclude, m.Corpus.Include)
	}

	if *mock {
		return moduleCompileMock(*storePath, svc, root, m, *mockEntriesFile, *dryRun, *jsonOut, stdout, stderr)
	}

	// A dry run is never recorded to compile_runs (internal/semantic.
	// Compile returns before RecordCompileRun when in.DryRun — the whole
	// point is "no store write yet"), so --json for a dry run cannot be
	// satisfied by re-reading the store afterward the way a real compile's
	// --json can. Route it through the SAME CompileLayerWithEscape entry
	// point directly and print the in-memory run — no guard logic applies
	// to a dry run either way (compileViaStore's A2-1/D3-1 guards live
	// entirely inside its `if !dryRun` branch), so this is not a second
	// copy of any decision logic, only of the plumbing to call it.
	if *dryRun && *jsonOut {
		return moduleCompileDryRunJSON(*storePath, svc, root, m, stdout, stderr)
	}

	outW := stdout
	var buf bytes.Buffer
	if *jsonOut {
		outW = &buf
	}
	rc := compileViaStore(*storePath, svc, m.Name, *profile, *dryRun, *force, *unsafeSkipEvalGate, false, nil, outW, stderr)
	if !*jsonOut {
		return rc
	}
	if rc != 0 {
		return rc
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: reopen store for JSON output: %v\n", err)
		return 1
	}
	defer st.Close()
	runs, err := st.CompileRuns(context.Background(), svc, m.Name)
	if err != nil || len(runs) == 0 {
		fmt.Fprintf(stderr, "error: could not read back the compile run just recorded: %v\n", err)
		return 1
	}
	jsonPrint(stdout, runs[0]) // runs is newest-first
	return 0
}

// moduleCompileMock is the dev/cold-run lane: it calls the EXACT SAME
// production entry point compileViaStore uses underneath —
// pkg/client.Client.CompileLayerWithEscape, itself a thin wrapper over
// internal/semantic.Compiler.Compile — with a scripted MockClient instead
// of a real provider. This is the seam that makes scripts/module-cold-
// run.sh runnable in CI with no key: same engine, same store writes, same
// claim verification, only the LLM transport differs. It deliberately does
// NOT run compileViaStore's D3-1/A2-1 CLI guards (those reason about a real
// shipping decision — an eval-gate and an idempotency check — which have no
// meaning for a scripted, throwaway cold-run compile).
func moduleCompileMock(storePath, service, root string, m modspec.Manifest, entriesFile string, dryRun, jsonOut bool, stdout, stderr io.Writer) int {
	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", storePath, err)
		return 1
	}
	defer st.Close()

	conn := &scopedWatermarkConnector{Connector: connector.NewLocalConnector(), root: root, manifest: m}
	c := client.New(conn, st)
	defer c.Close()

	if !dryRun {
		entries, merr := loadMockEntries(entriesFile)
		if merr != nil {
			fmt.Fprintf(stderr, "error: %v\n", merr)
			return 1
		}
		c.WithLLM(llmclient.NewMockClient(entries...), mockModelName, 0, 0)
	}

	run, cerr := c.CompileLayerWithEscape(context.Background(), service, m.Name, dryRun, "mock lane (scripts/module-cold-run.sh or --mock)")
	if cerr != nil {
		fmt.Fprintf(stderr, "error: compile layer %q: %v\n", m.Name, cerr)
		return 1
	}
	if jsonOut {
		jsonPrint(stdout, run)
		return 0
	}
	printCompileRunCore(stdout, run)
	return 0
}

// moduleCompileDryRunJSON serves --dry-run=true --json: a dry run is never
// persisted to compile_runs (see the call site's comment), so the estimate
// must be captured in-memory via the same CompileLayerWithEscape entry
// point compileViaStore itself calls, rather than re-read from the store
// afterward. Nothing is written to compile_runs either way, so the choice
// of connector here has no effect on doctor's watermark checks — the
// scoped wrapper is used anyway, for parity with moduleCompileMock and
// because it's simply the more correct value to REPORT in the printed
// estimate's CorpusWatermark field.
func moduleCompileDryRunJSON(storePath, service, root string, m modspec.Manifest, stdout, stderr io.Writer) int {
	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", storePath, err)
		return 1
	}
	defer st.Close()

	conn := &scopedWatermarkConnector{Connector: connector.NewLocalConnector(), root: root, manifest: m}
	c := client.New(conn, st)
	defer c.Close()

	run, err := c.CompileLayerWithEscape(context.Background(), service, m.Name, true, "")
	if err != nil {
		fmt.Fprintf(stderr, "error: compile layer %q: %v\n", m.Name, err)
		return 1
	}
	jsonPrint(stdout, run)
	return 0
}

func sameCorpus(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]bool{}
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if !set[s] {
			return false
		}
	}
	return true
}

// ─── eval ───────────────────────────────────────────────────────────────

// EvalArtifact is the structured evidence `module eval --record <path>`
// writes — replaces free-text, author-typed "evidence" (Phase J critic
// finding 1: a bare os.Stat on arbitrary text let a directory, an empty
// file, or permanently-stale prose pass as "eval evidence" forever).
// `module doctor`'s eval-passed criterion parses this JSON and
// cross-checks PromptVersion against the store's own compiled claims —
// evidence recorded under a DIFFERENT prompt identity than what's
// currently compiled is stale, regardless of how recently the file was
// written.
type EvalArtifact struct {
	Module          string  `json:"module"`
	Layer           string  `json:"layer"`
	PromptVersion   string  `json:"prompt_version"`
	CorpusWatermark string  `json:"corpus_watermark,omitempty"`
	Precision       float64 `json:"precision"`
	Recall          float64 `json:"recall"`
	PassesGate      bool    `json:"passes_gate"`
	// Lane is "real" (a genuine --profile run) or "mock" (a scripted
	// --mock run), stamped honestly by the run that produced this
	// artifact — never inferred or asserted by a caller. `module doctor`'s
	// eval-passed criterion does NOT gate on Lane (a scripted/CI eval can
	// still prove the scoring machinery works without spending real
	// tokens); the PUBLISHED level's own "eval lane is real" criterion is
	// where a mock lane is refused — see docs/plans/expert-suite-j.md's
	// critic-round design note for why the gate sits there, not here.
	Lane string `json:"lane"`
}

func cmdModuleEval(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("module eval", stderr)
	manifestPath := fs.String("manifest", "", "path to module.cue (required)")
	fixtures := fs.String("fixtures", "", "eval fixtures dir (default: sibling \"eval\" dir next to module.cue)")
	root := fs.String("root", "", "project root to resolve the module's layer pack from (default: module.cue's own directory)")
	profile := fs.String("profile", "", "named LLM profile for a real (spending) eval")
	mock := fs.Bool("mock", false, "use a scripted MockClient instead of a real LLM (--mock-entries-file required) — never spends, never needs a key")
	mockEntriesFile := fs.String("mock-entries-file", "", "path to a JSON array of scripted response strings, consumed in order (--mock only)")
	record := fs.String("record", "", "write a structured EvalArtifact (JSON) to this path — the ONLY evidence the eval-passed criterion of 'module doctor' accepts via eval_evidence.path")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "error: --manifest is required")
		fs.Usage()
		return 1
	}
	if *mock && *mockEntriesFile == "" {
		fmt.Fprintln(stderr, "error: --mock requires --mock-entries-file")
		return 1
	}
	if !*mock && *profile == "" {
		fmt.Fprintln(stderr, "error: pass --profile for a real eval, or --mock --mock-entries-file for a scripted one (never a real key in a scripted/CI run)")
		return 1
	}

	m, err := modspec.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	fixturesDir := *fixtures
	if fixturesDir == "" {
		fixturesDir = filepath.Join(filepath.Dir(*manifestPath), "eval")
	}
	if _, statErr := os.Stat(fixturesDir); statErr != nil {
		fmt.Fprintf(stderr, "error: eval fixtures dir %s: %v\n", fixturesDir, statErr)
		return 1
	}

	layerRoot := *root
	if layerRoot == "" {
		layerRoot = filepath.Dir(*manifestPath)
	}
	layerDef, err := semantic.LoadLayerIn(layerRoot, m.Name)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve layer pack %q: %v (module eval expects a project-local layer pack named after the module, .fathom/layers/%s/)\n", m.Name, err, m.Name)
		return 1
	}

	var llm llmclient.LLMClient
	model := ""
	lane := "real"
	if *mock {
		entries, merr := loadMockEntries(*mockEntriesFile)
		if merr != nil {
			fmt.Fprintf(stderr, "error: %v\n", merr)
			return 1
		}
		llm = llmclient.NewMockClient(entries...)
		model = mockModelName
		lane = "mock"
	} else {
		p, _, perr := resolveActiveProfile(*profile)
		if perr != nil {
			fmt.Fprintf(stderr, "error: resolving --profile %q: %v\n", *profile, perr)
			return 1
		}
		cl, cerr := buildLLMClientForProfile(p)
		if cerr != nil {
			fmt.Fprintf(stderr, "error: %v\n", cerr)
			return 1
		}
		llm = cl
		model = compileModelForProfile(p)
	}

	result, err := semantic.RunEval(context.Background(), llm, model, layerDef, fixturesDir, nil)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	recordedPath := ""
	if *record != "" {
		// corpus_watermark is informational only (module doctor's
		// eval-passed criterion never gates on it) — best-effort, never
		// fatal to the eval itself.
		corpusWM, _ := moduleWatermark(context.Background(), layerRoot, m)
		artifact := EvalArtifact{
			Module:          m.Name,
			Layer:           m.Name,
			PromptVersion:   result.Run.PromptVersion,
			CorpusWatermark: corpusWM,
			Precision:       result.Precision,
			Recall:          result.Recall,
			PassesGate:      result.PassesGate(),
			Lane:            lane,
		}
		raw, jerr := json.MarshalIndent(artifact, "", "  ")
		if jerr != nil {
			fmt.Fprintf(stderr, "error: marshal eval artifact: %v\n", jerr)
			return 1
		}
		if err := os.WriteFile(*record, raw, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write --record %s: %v\n", *record, err)
			return 1
		}
		recordedPath = *record
	}

	if *jsonOut {
		jsonPrint(stdout, map[string]any{
			"module":           m.Name,
			"fixtures":         fixturesDir,
			"precision":        result.Precision,
			"recall":           result.Recall,
			"tp":               result.TP,
			"fp":               result.FP,
			"fn":               result.FN,
			"missed_rule_keys": result.MissedRuleKeys,
			"passes_gate":      result.PassesGate(),
			"claims_emitted":   result.Run.ClaimsEmitted,
			"lane":             lane,
			"prompt_version":   result.Run.PromptVersion,
			"recorded":         recordedPath,
		})
		return 0
	}
	fmt.Fprintf(stdout, "module:      %s\n", m.Name)
	fmt.Fprintf(stdout, "fixtures:    %s\n", fixturesDir)
	fmt.Fprintf(stdout, "lane:        %s\n", lane)
	fmt.Fprintf(stdout, "precision:   %.2f (tp=%d fp=%d)\n", result.Precision, result.TP, result.FP)
	fmt.Fprintf(stdout, "recall:      %.2f (fn=%d)\n", result.Recall, result.FN)
	if len(result.MissedRuleKeys) > 0 {
		fmt.Fprintf(stdout, "missed:      %v\n", result.MissedRuleKeys)
	}
	fmt.Fprintf(stdout, "passes_gate: %v (>= 0.90 precision, >= 0.75 recall)\n", result.PassesGate())
	if recordedPath != "" {
		fmt.Fprintf(stdout, "recorded:    %s\n", recordedPath)
	}
	return 0
}

// loadMockEntries reads a JSON array of strings from path and returns one
// MockEntry per string, in order — the deterministic, no-key eval lane
// scripts/module-cold-run.sh drives.
func loadMockEntries(path string) ([]llmclient.MockEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --mock-entries-file %s: %w", path, err)
	}
	var texts []string
	if err := json.Unmarshal(raw, &texts); err != nil {
		return nil, fmt.Errorf("parse --mock-entries-file %s (want a JSON array of strings): %w", path, err)
	}
	entries := make([]llmclient.MockEntry, 0, len(texts))
	for _, t := range texts {
		entries = append(entries, llmclient.MockEntry{Response: llmclient.Response{Text: t}})
	}
	return entries, nil
}
