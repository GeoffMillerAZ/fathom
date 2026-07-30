package main

// module_doctor.go — `fathom module doctor`: the readiness scorecard +
// ladder (docs/vision/13-expert-suite.md §4 step 6): draft -> eval-passed
// -> servable -> published. Every criterion is machine-checkable AGAINST
// THE STORE (Phase J critic finding 12 — not against author-typed text);
// doctor NEVER auto-promotes — it reports the highest level all criteria
// support, naming every criterion pass/fail with its evidence path (law
// 11: the advisor recommends, the gate decides — there is no advisor in
// this phase, only deterministic criteria).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/modspec"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Readiness levels, in ladder order.
const (
	LevelDraft      = "draft"
	LevelEvalPassed = "eval-passed"
	LevelServable   = "servable"
	LevelPublished  = "published"
)

// ladderRank orders the levels for --require-level comparisons (finding 9);
// "none" (an unloadable manifest) ranks below everything.
var ladderRank = map[string]int{
	"none":          0,
	LevelDraft:      1,
	LevelEvalPassed: 2,
	LevelServable:   3,
	LevelPublished:  4,
}

// Criterion is one named, machine-checked doctor check. Facts carries
// optional STRUCTURED values alongside Evidence's free-text prose (e.g.
// eval precision/recall, compile lane, model, watermark) — additive only,
// never a replacement for the prose: a consumer that only reads Evidence
// (every consumer before this field existed) is unaffected, and a
// verb-JSON-only consumer that needs to render a value (not parse a
// sentence) reads Facts instead. Populated only for criteria whose
// computation already has strongly-typed values on hand; nil (omitted)
// everywhere else, never a fabricated/parsed-out-of-prose value.
type Criterion struct {
	Level    string            `json:"level"`
	Name     string            `json:"name"`
	Pass     bool              `json:"pass"`
	Evidence string            `json:"evidence"`
	Facts    map[string]string `json:"facts,omitempty"`
}

// ManifestSummary is the human-relevant subset of a loaded module.cue
// manifest, surfaced on DoctorReport so a verb-JSON-only consumer (e.g.
// fathom-studio, law 10: it may only consume verb JSON, never reach into
// module.cue's CUE internals itself) can render a module's identity
// without a manifest dump verb of its own. `module doctor --json` is the
// ONE place this lives (docs/mcp-tools.md's verb catalog is CLI-verb-only
// and unaffected; see module.go's doc comment) — chosen over a separate
// `module show --json` because doctor already loads the manifest on every
// invocation, so this is a zero-extra-read addition, not a new code path.
type ManifestSummary struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	License     string `json:"license,omitempty"`
	TierProfile string `json:"tier_profile,omitempty"`
	// ProvenanceCompiledBy is provenance.compiled_by exactly as authored in
	// module.cue — the manifest's OWN claim, not the store-cross-checked
	// identity the "compile lane" published criterion verifies (see that
	// criterion's Facts for the store-verified value).
	ProvenanceCompiledBy string `json:"provenance_compiled_by,omitempty"`
	ProvenanceCompiledAt string `json:"provenance_compiled_at,omitempty"`
	// CorpusSelectorCount is len(include)+len(exclude) — the total glob
	// count declared in the manifest's corpus block.
	CorpusSelectorCount int    `json:"corpus_selector_count"`
	Watermark           string `json:"watermark,omitempty"`
}

// DoctorReport is `module doctor`'s full verdict. Manifest is nil when the
// manifest itself fails to load (the "manifest loads" draft criterion
// already reports that failure; there is nothing to summarize).
type DoctorReport struct {
	Module   string           `json:"module"`
	Verdict  string           `json:"verdict"`
	Criteria []Criterion      `json:"criteria"`
	Manifest *ManifestSummary `json:"manifest,omitempty"`
}

func cmdModuleDoctor(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("module doctor", stderr)
	manifestPath := fs.String("manifest", "", "path to module.cue (required)")
	root := fs.String("root", "", "corpus root (default: module.cue's own directory)")
	storePath := fs.String("store", "", "local store path")
	service := fs.String("service", "", "service ID in --store (default: manifest name)")
	packDir := fs.String("pack-dir", "", "a `fathom module pack` output dir, to check the published-level pack-artifact criterion")
	requireLevel := fs.String("require-level", "", "exit non-zero when the computed verdict ranks below this level (draft|eval-passed|servable|published)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "error: --manifest is required")
		fs.Usage()
		return 1
	}
	if *requireLevel != "" {
		if _, ok := ladderRank[*requireLevel]; !ok || *requireLevel == "none" {
			fmt.Fprintf(stderr, "error: --require-level %q is not a known level (want draft|eval-passed|servable|published)\n", *requireLevel)
			return 1
		}
	}

	report := runDoctor(*manifestPath, *root, *storePath, *service, *packDir)

	if *jsonOut {
		jsonPrint(stdout, report)
	} else {
		fmt.Fprintf(stdout, "module:  %s\n", report.Module)
		fmt.Fprintf(stdout, "verdict: %s\n\n", report.Verdict)
		for _, c := range report.Criteria {
			mark := "FAIL"
			if c.Pass {
				mark = "PASS"
			}
			fmt.Fprintf(stdout, "[%s] %-11s %-45s %s\n", mark, c.Level, c.Name, c.Evidence)
		}
	}

	if *requireLevel != "" && ladderRank[report.Verdict] < ladderRank[*requireLevel] {
		if !*jsonOut {
			fmt.Fprintf(stderr, "error: verdict %q ranks below required level %q\n", report.Verdict, *requireLevel)
		}
		return 1
	}
	return 0
}

// storeState is the store data every level above draft cross-checks
// against — opened ONCE via store.MountReadOnly (finding 5: never
// store.Open, which would stamp fathom's schema onto a third-party
// artifact) and closed before runDoctor returns.
type storeState struct {
	conforms     bool
	conformsEvid string
	claims       []core.SemanticClaim
	claimsErr    error
	runs         []core.CompileRun
	runsErr      error
}

// loadStoreState opens storePath read-only and fetches everything the
// eval-passed/servable/published criteria need in one pass. err fields are
// individually populated (never a single all-or-nothing error) so a mount
// failure still lets draft-level criteria print a precise reason.
func loadStoreState(storePath, service, layer string) storeState {
	noStoreErr := errors.New("no --store given")
	st := storeState{conformsEvid: "no --store given", claimsErr: noStoreErr, runsErr: noStoreErr}
	if storePath == "" {
		return st
	}
	mounted, mErr := store.MountReadOnly(storePath, store.LevelL0)
	if mErr != nil {
		st.conformsEvid = mErr.Error()
		st.claimsErr = mErr
		st.runsErr = mErr
		return st
	}
	defer mounted.Close()
	st.conforms = true
	st.conformsEvid = fmt.Sprintf("%s conforms to store-format-v0 L0", storePath)
	st.claims, st.claimsErr = mounted.ClaimsForService(context.Background(), service, layer)
	st.runs, st.runsErr = mounted.CompileRuns(context.Background(), service, layer)
	return st
}

// runDoctor computes every criterion at every level and the resulting
// verdict — the highest level all of ITS OWN AND EVERY LOWER level's
// criteria pass. A level is never skipped even if a lower one fails: every
// criterion is still computed and reported (the honest scorecard doctrine —
// "names every criterion pass/fail," not just the first failure).
func runDoctor(manifestPath, root, storePath, service, packDir string) DoctorReport {
	report := DoctorReport{Module: manifestPath}
	var criteria []Criterion
	add := func(level, name string, pass bool, evidence string) {
		criteria = append(criteria, Criterion{Level: level, Name: name, Pass: pass, Evidence: evidence})
	}
	// addFacts is add's twin for criteria whose computation already has
	// strongly-typed values on hand (Criterion.Facts's doc comment) — the
	// prose in evidence is unchanged, facts is purely additive.
	addFacts := func(level, name string, pass bool, evidence string, facts map[string]string) {
		criteria = append(criteria, Criterion{Level: level, Name: name, Pass: pass, Evidence: evidence, Facts: facts})
	}

	// ── draft ──
	m, err := modspec.LoadManifest(manifestPath)
	if err != nil {
		add(LevelDraft, "manifest loads", false, err.Error())
		report.Criteria = criteria
		report.Verdict = "none (manifest does not even load)"
		return report
	}
	add(LevelDraft, "manifest loads", true, manifestPath)
	report.Module = m.Name
	report.Manifest = &ManifestSummary{
		Name:                 m.Name,
		Version:              m.Version,
		Description:          m.Description,
		License:              m.License,
		TierProfile:          m.TierProfile,
		ProvenanceCompiledBy: m.Provenance.CompiledBy,
		ProvenanceCompiledAt: m.Provenance.CompiledAt,
		CorpusSelectorCount:  len(m.Corpus.Include) + len(m.Corpus.Exclude),
		Watermark:            m.Watermark,
	}

	corpusRoot := moduleServiceRoot(manifestPath, root)
	inv, invErr := resolveSourceInventory(context.Background(), corpusRoot, m)
	atLeastOneSourceResolves := invErr == nil && inv.TotalMatched > 0
	sourcesEvidence := fmt.Sprintf("%d files matched under %s", inv.TotalMatched, corpusRoot)
	if invErr != nil {
		sourcesEvidence = invErr.Error()
	}
	add(LevelDraft, "at least one source resolves", atLeastOneSourceResolves, sourcesEvidence)

	svc := service
	if svc == "" {
		svc = m.Name
	}
	ss := loadStoreState(storePath, svc, m.Name)

	// ── eval-passed ──
	// Finding 1: os.Stat on author-typed text is no longer accepted as
	// evidence. eval_evidence.path must be a structured EvalArtifact (JSON,
	// written by `module eval --record`) whose PassesGate is true AND whose
	// PromptVersion matches the STORE's own currently-compiled claims — an
	// artifact recorded under a stale prompt identity (the layer was
	// recompiled since) does not count, no matter how recently the file was
	// written. Lane (mock vs. real) is reported but NOT gated here — see
	// EvalArtifact's own doc comment for why the mock/real distinction is
	// enforced at the published level instead (compileLane's sibling
	// criterion below), not here: a scripted/CI eval must still be able to
	// prove the scoring machinery works without spending real tokens.
	evalPassed := false
	evalEvidence := "eval_evidence.evaled is false"
	var evalFacts map[string]string // typed twin of evalEvidence's prose (precision/recall/lane/prompt_version), set only once an artifact actually parses
	if m.EvalEvidence.Evaled {
		p := m.EvalEvidence.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(filepath.Dir(manifestPath), p)
		}
		raw, readErr := os.ReadFile(p)
		switch {
		case readErr != nil:
			evalEvidence = fmt.Sprintf("eval_evidence.path %q could not be read: %v", m.EvalEvidence.Path, readErr)
		default:
			var artifact EvalArtifact
			if jsonErr := json.Unmarshal(raw, &artifact); jsonErr != nil {
				evalEvidence = fmt.Sprintf("eval_evidence.path %q is not a valid eval artifact (author-typed text is not accepted as evidence — record one via `module eval --record`): %v", m.EvalEvidence.Path, jsonErr)
			} else {
				evalFacts = map[string]string{
					"precision":      fmt.Sprintf("%.4f", artifact.Precision),
					"recall":         fmt.Sprintf("%.4f", artifact.Recall),
					"lane":           artifact.Lane,
					"prompt_version": artifact.PromptVersion,
				}
				switch {
				case !artifact.PassesGate:
					evalEvidence = fmt.Sprintf("eval artifact reports passes_gate=false (precision=%.2f recall=%.2f, lane=%s)", artifact.Precision, artifact.Recall, artifact.Lane)
				case ss.claimsErr != nil:
					evalEvidence = fmt.Sprintf("could not cross-check the eval artifact's prompt_version %q against the store's compiled claims: %v", artifact.PromptVersion, ss.claimsErr)
				case len(ss.claims) == 0:
					evalEvidence = fmt.Sprintf("eval artifact passes its own gate (lane=%s), but no claims are compiled yet to cross-check prompt_version %q against", artifact.Lane, artifact.PromptVersion)
				case !anyClaimPromptVersion(ss.claims, artifact.PromptVersion):
					evalEvidence = fmt.Sprintf("eval artifact was recorded under prompt_version %q, but the store's compiled claims for layer %q carry a different prompt_version — this eval evidence is stale relative to the current compile", artifact.PromptVersion, m.Name)
				default:
					evalPassed = true
					evalEvidence = fmt.Sprintf("eval artifact: precision=%.2f recall=%.2f lane=%s prompt_version=%q matches the store's compiled claims", artifact.Precision, artifact.Recall, artifact.Lane, artifact.PromptVersion)
				}
			}
		}
	}
	addFacts(LevelEvalPassed, "eval evidence present, gate-passing, and prompt_version-fresh (store-checked)", evalPassed, evalEvidence, evalFacts)

	// ── servable ──
	// Anchor health + the full three-way watermark check both live in
	// computeMap (module_map.go) — the single source of truth `module map`
	// itself uses, so the two verbs can never disagree. GapNoCitations
	// (finding 10) counts as a broken-anchor-class failure here too: a
	// citation-free store is not "0 broken anchors," it's unverifiable.
	zeroBrokenAnchors := false
	anchorsEvidence := "no --store given"
	watermarkFresh := false
	watermarkEvidence := "no --store given"
	if storePath != "" {
		mapReport, mapErr := computeMap(context.Background(), corpusRoot, storePath, svc, m)
		if mapErr != nil {
			anchorsEvidence = mapErr.Error()
			watermarkEvidence = mapErr.Error()
		} else {
			brokenAnchors := 0
			hasWatermarkDrift := false
			hasStoreNote := false
			hasNoCitations := false
			for _, g := range mapReport.Gaps {
				switch g.Code {
				case GapLegacyEmptyAnchor, GapAnchorDrifted:
					brokenAnchors++
				case GapNoCitations:
					hasNoCitations = true
				case GapWatermarkDrift:
					hasWatermarkDrift = true
				case GapStoreNotProvided, GapLayerNotCompiled:
					hasStoreNote = true
				}
			}
			zeroBrokenAnchors = brokenAnchors == 0 && !hasStoreNote && !hasNoCitations
			switch {
			case hasStoreNote:
				anchorsEvidence = "layer not compiled yet in this store"
			case hasNoCitations:
				anchorsEvidence = "claims are compiled but carry ZERO citation links — anchors cannot be verified (see `module map`)"
			default:
				anchorsEvidence = fmt.Sprintf("%d broken anchor(s) found by `module map`", brokenAnchors)
			}
			watermarkFresh = !hasWatermarkDrift
			if hasWatermarkDrift {
				watermarkEvidence = "manifest/store/corpus watermarks disagree — see `module map`'s watermark_drift gap for exactly which pair"
			} else {
				watermarkEvidence = fmt.Sprintf("manifest watermark %s matches the store's recorded compile AND the corpus's current watermark", m.Watermark)
			}
		}
	}
	add(LevelServable, "store conforms (store.MountReadOnly, L0)", ss.conforms, ss.conformsEvid)
	add(LevelServable, "zero broken anchors", zeroBrokenAnchors, anchorsEvidence)
	addFacts(LevelServable, "watermark fresh: manifest == store's recorded compile == current corpus", watermarkFresh, watermarkEvidence, map[string]string{"manifest_watermark": m.Watermark})

	// ── published ──
	licenseReal := m.License != "" && m.License != "unspecified"
	licenseEvidence := fmt.Sprintf("license = %q", m.License)
	if !licenseReal {
		licenseEvidence = "license is the honest default \"unspecified\" — a published module must name a real license"
	}
	addFacts(LevelPublished, "license is a real, non-default value", licenseReal, licenseEvidence, map[string]string{"license": m.License})

	provenanceComplete := m.Provenance.CompiledBy != "" && m.Provenance.CompiledAt != ""
	addFacts(LevelPublished, "provenance complete", provenanceComplete, fmt.Sprintf("compiled_by=%q compiled_at=%q", m.Provenance.CompiledBy, m.Provenance.CompiledAt),
		map[string]string{"compiled_by": m.Provenance.CompiledBy, "compiled_at": m.Provenance.CompiledAt})

	// Finding 2: the store's OWN claims — never what the manifest merely
	// claims about itself — decide whether this was a real compile. A
	// mock/dev model, or an ambiguous mix, refuses published outright; a
	// real or deterministic compile must additionally match
	// provenance.compiled_by (init's "unspecified" default fails this until
	// a real compile stamps something else).
	laneOK := false
	laneEvidence := "no --store given"
	var laneFacts map[string]string
	if ss.claimsErr != nil {
		laneEvidence = ss.claimsErr.Error()
	} else {
		lane := classifyCompileLane(ss.claims)
		laneFacts = map[string]string{"lane": lane.Lane, "model": lane.Model}
		switch lane.Lane {
		case "none":
			laneEvidence = "no claims compiled yet"
		case "mock":
			laneEvidence = fmt.Sprintf("published refuses a mock/dev compile: %s", lane.Detail)
		case "mixed":
			laneEvidence = fmt.Sprintf("published refuses an ambiguous compile: %s", lane.Detail)
		default: // "real" or "deterministic"
			expected := lane.Model
			if m.Provenance.CompiledBy == expected {
				laneOK = true
				laneEvidence = fmt.Sprintf("provenance.compiled_by=%q matches the store's actual compile identity (%s)", m.Provenance.CompiledBy, lane.Detail)
			} else {
				laneEvidence = fmt.Sprintf("provenance.compiled_by=%q does not match the store's actual compile identity %q", m.Provenance.CompiledBy, expected)
			}
		}
	}
	addFacts(LevelPublished, "compile lane is real (not mock/dev) and matches provenance.compiled_by", laneOK, laneEvidence, laneFacts)

	// Finding 1 (the published-level half): the eval evidence itself must
	// also be a REAL run, not the scripted/mock lane — eval-passed above
	// deliberately does not gate on this so a scripted CI eval can still
	// prove the scoring machinery; published is where mock evidence, like a
	// mock compile, is refused outright.
	evalLaneOK := false
	evalLaneEvidence := "eval_evidence.evaled is false"
	if m.EvalEvidence.Evaled {
		p := m.EvalEvidence.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(filepath.Dir(manifestPath), p)
		}
		if raw, readErr := os.ReadFile(p); readErr != nil {
			evalLaneEvidence = fmt.Sprintf("eval_evidence.path %q could not be read: %v", m.EvalEvidence.Path, readErr)
		} else {
			var artifact EvalArtifact
			if jsonErr := json.Unmarshal(raw, &artifact); jsonErr != nil {
				evalLaneEvidence = fmt.Sprintf("eval_evidence.path %q is not a valid eval artifact: %v", m.EvalEvidence.Path, jsonErr)
			} else if artifact.Lane == "real" {
				evalLaneOK = true
				evalLaneEvidence = "eval artifact lane=real"
			} else {
				evalLaneEvidence = fmt.Sprintf("published refuses a mock/dev eval: eval artifact lane=%q", artifact.Lane)
			}
		}
	}
	add(LevelPublished, "eval lane is real (not mock/dev)", evalLaneOK, evalLaneEvidence)

	// Finding 4: two filename Stats are not a pack-conformance check. The
	// packed copy must independently mount read-only, its own module.cue
	// must independently load, and its identity (name/version/watermark)
	// must match the manifest under review — plus, when --store was given,
	// its claim-ID SET must equal --store's (proving the pack is current,
	// not stale or foreign).
	packOK := false
	packEvidence := "no --pack-dir given"
	if packDir != "" {
		packOK, packEvidence = evaluatePackArtifact(packDir, svc, m, ss)
	}
	add(LevelPublished, "pack artifact conforms and matches this manifest", packOK, packEvidence)

	report.Criteria = criteria
	report.Verdict = highestSupportedLevel(criteria)
	return report
}

// anyClaimPromptVersion reports whether at least one claim carries the
// given prompt version — "at least one" rather than "all," since a corpus
// mid-recompile (digest-gated: only changed sections re-run) may legitimately
// carry a mix of old and new PromptVersion values for unrelated sections;
// the eval artifact only needs to be fresh relative to SOME of what's
// currently compiled, not every claim ever emitted.
func anyClaimPromptVersion(claims []core.SemanticClaim, promptVersion string) bool {
	for _, cl := range claims {
		if cl.PromptVersion == promptVersion {
			return true
		}
	}
	return false
}

// evaluatePackArtifact is the finding-4 pack-conformance check. Returns
// (pass, evidence).
func evaluatePackArtifact(packDir, service string, m modspec.Manifest, ss storeState) (bool, string) {
	sqlitePath := filepath.Join(packDir, "module.sqlite")
	cuePath := filepath.Join(packDir, "module.cue")

	packedStore, mErr := store.MountReadOnly(sqlitePath, store.LevelL0)
	if mErr != nil {
		return false, fmt.Sprintf("packed store %s failed conformance: %v", sqlitePath, mErr)
	}
	defer packedStore.Close()

	packedManifest, lErr := modspec.LoadManifest(cuePath)
	if lErr != nil {
		return false, fmt.Sprintf("packed manifest %s failed to load: %v", cuePath, lErr)
	}
	if packedManifest.Name != m.Name || packedManifest.Version != m.Version || packedManifest.Watermark != m.Watermark {
		return false, fmt.Sprintf("packed manifest (name=%q version=%q watermark=%q) does not match the manifest under review (name=%q version=%q watermark=%q)",
			packedManifest.Name, packedManifest.Version, packedManifest.Watermark, m.Name, m.Version, m.Watermark)
	}

	packedClaims, pcErr := packedStore.ClaimsForService(context.Background(), service, m.Name)
	if pcErr != nil {
		return false, fmt.Sprintf("could not read the packed store's claims: %v", pcErr)
	}
	if ss.claimsErr == nil {
		if !sameClaimIDSet(ss.claims, packedClaims) {
			return false, fmt.Sprintf("packed store's claim set (%d claims) does not match --store's (%d claims) — the pack is stale or was built from a different compile", len(packedClaims), len(ss.claims))
		}
		return true, fmt.Sprintf("%s conforms, its manifest matches, and its %d claim(s) match --store", packDir, len(packedClaims))
	}
	return true, fmt.Sprintf("%s conforms and its manifest matches (no --store given to cross-check claim IDs against)", packDir)
}

// highestSupportedLevel walks the ladder in order and returns the highest
// level whose own criteria, AND every lower level's, all pass — doctor
// never auto-promotes past a failing lower rung.
func highestSupportedLevel(criteria []Criterion) string {
	byLevel := map[string]bool{
		LevelDraft:      true,
		LevelEvalPassed: true,
		LevelServable:   true,
		LevelPublished:  true,
	}
	for _, c := range criteria {
		if !c.Pass {
			byLevel[c.Level] = false
		}
	}
	ladder := []string{LevelPublished, LevelServable, LevelEvalPassed, LevelDraft}
	cumulative := map[string]bool{}
	ok := true
	for i := len(ladder) - 1; i >= 0; i-- {
		lvl := ladder[i]
		ok = ok && byLevel[lvl]
		cumulative[lvl] = ok
	}
	for _, lvl := range ladder {
		if cumulative[lvl] {
			return lvl
		}
	}
	return "none"
}
