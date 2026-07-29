package main

// module_doctor.go — `fathom module doctor`: the readiness scorecard +
// ladder (docs/vision/13-expert-suite.md §4 step 6): draft -> eval-passed
// -> servable -> published. Every criterion is machine-checkable; doctor
// NEVER auto-promotes — it reports the highest level all criteria support,
// naming every criterion pass/fail with its evidence path (law 11: the
// advisor recommends, the gate decides — there is no advisor in this
// phase, only deterministic criteria).

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

// Criterion is one named, machine-checked doctor check.
type Criterion struct {
	Level    string `json:"level"`
	Name     string `json:"name"`
	Pass     bool   `json:"pass"`
	Evidence string `json:"evidence"`
}

// DoctorReport is `module doctor`'s full verdict.
type DoctorReport struct {
	Module   string      `json:"module"`
	Verdict  string      `json:"verdict"`
	Criteria []Criterion `json:"criteria"`
}

func cmdModuleDoctor(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("module doctor", stderr)
	manifestPath := fs.String("manifest", "", "path to module.cue (required)")
	root := fs.String("root", "", "corpus root (default: module.cue's own directory)")
	storePath := fs.String("store", "", "local store path")
	service := fs.String("service", "", "service ID in --store (default: manifest name)")
	packDir := fs.String("pack-dir", "", "a `fathom module pack` output dir, to check the published-level pack-artifact criterion")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "error: --manifest is required")
		fs.Usage()
		return 1
	}

	report := runDoctor(*manifestPath, *root, *storePath, *service, *packDir)

	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "module:  %s\n", report.Module)
	fmt.Fprintf(stdout, "verdict: %s\n\n", report.Verdict)
	for _, c := range report.Criteria {
		mark := "FAIL"
		if c.Pass {
			mark = "PASS"
		}
		fmt.Fprintf(stdout, "[%s] %-11s %-40s %s\n", mark, c.Level, c.Name, c.Evidence)
	}
	return 0
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

	corpusRoot := moduleServiceRoot(manifestPath, root)
	inv, invErr := resolveSourceInventory(context.Background(), corpusRoot, m)
	atLeastOneSourceResolves := invErr == nil && inv.TotalMatched > 0
	sourcesEvidence := fmt.Sprintf("%d files matched under %s", inv.TotalMatched, corpusRoot)
	if invErr != nil {
		sourcesEvidence = invErr.Error()
	}
	add(LevelDraft, "at least one source resolves", atLeastOneSourceResolves, sourcesEvidence)

	// ── eval-passed ──
	evalEvidencePresent := false
	evalEvidenceEvidence := "eval_evidence.evaled is false"
	if m.EvalEvidence.Evaled {
		p := m.EvalEvidence.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(filepath.Dir(manifestPath), p)
		}
		if _, statErr := os.Stat(p); statErr == nil {
			evalEvidencePresent = true
			evalEvidenceEvidence = p
		} else {
			evalEvidenceEvidence = fmt.Sprintf("eval_evidence.path %q does not exist on disk: %v", m.EvalEvidence.Path, statErr)
		}
	}
	add(LevelEvalPassed, "eval evidence present and on disk", evalEvidencePresent, evalEvidenceEvidence)

	// ── servable ──
	storeConforms := false
	storeConformsEvidence := "no --store given"
	zeroBrokenAnchors := false
	anchorsEvidence := "no --store given"
	watermarkFresh := false
	watermarkEvidence := "no --store given"

	if storePath != "" {
		svc := service
		if svc == "" {
			svc = m.Name
		}
		if _, mErr := store.MountReadOnly(storePath, store.LevelL0); mErr != nil {
			storeConformsEvidence = mErr.Error()
		} else {
			storeConforms = true
			storeConformsEvidence = fmt.Sprintf("%s conforms to store-format-v0 L0", storePath)
		}

		mapReport, mapErr := computeMap(context.Background(), corpusRoot, storePath, svc, m)
		if mapErr != nil {
			anchorsEvidence = mapErr.Error()
			watermarkEvidence = mapErr.Error()
		} else {
			brokenAnchors := 0
			hasWatermarkDrift := false
			hasStoreNote := false
			for _, g := range mapReport.Gaps {
				switch g.Code {
				case GapLegacyEmptyAnchor, GapAnchorDrifted:
					brokenAnchors++
				case GapWatermarkDrift:
					hasWatermarkDrift = true
				case GapStoreNotProvided, GapLayerNotCompiled:
					hasStoreNote = true
				}
			}
			zeroBrokenAnchors = brokenAnchors == 0 && !hasStoreNote
			if hasStoreNote {
				anchorsEvidence = "layer not compiled yet in this store"
			} else {
				anchorsEvidence = fmt.Sprintf("%d broken anchor(s) found by `module map`", brokenAnchors)
			}
			watermarkFresh = !hasWatermarkDrift
			if hasWatermarkDrift {
				watermarkEvidence = "manifest watermark does not match the corpus's current watermark (see `module map`)"
			} else {
				watermarkEvidence = fmt.Sprintf("manifest watermark %s matches the corpus", m.Watermark)
			}
		}
	}
	add(LevelServable, "store conforms (store.MountReadOnly, L0)", storeConforms, storeConformsEvidence)
	add(LevelServable, "zero broken anchors", zeroBrokenAnchors, anchorsEvidence)
	add(LevelServable, "watermark fresh vs corpus", watermarkFresh, watermarkEvidence)

	// ── published ──
	licenseReal := m.License != "" && m.License != "unspecified"
	licenseEvidence := fmt.Sprintf("license = %q", m.License)
	if !licenseReal {
		licenseEvidence = "license is the honest default \"unspecified\" — a published module must name a real license"
	}
	add(LevelPublished, "license is a real, non-default value", licenseReal, licenseEvidence)

	provenanceComplete := m.Provenance.CompiledBy != "" && m.Provenance.CompiledAt != ""
	add(LevelPublished, "provenance complete", provenanceComplete, fmt.Sprintf("compiled_by=%q compiled_at=%q", m.Provenance.CompiledBy, m.Provenance.CompiledAt))

	packExists := false
	packEvidence := "no --pack-dir given"
	if packDir != "" {
		sqlitePath := filepath.Join(packDir, "module.sqlite")
		cuePath := filepath.Join(packDir, "module.cue")
		_, sErr := os.Stat(sqlitePath)
		_, cErr := os.Stat(cuePath)
		if sErr == nil && cErr == nil {
			packExists = true
			packEvidence = packDir
		} else {
			packEvidence = fmt.Sprintf("%s missing module.sqlite and/or module.cue", packDir)
		}
	}
	add(LevelPublished, "pack artifact exists", packExists, packEvidence)

	report.Criteria = criteria
	report.Verdict = highestSupportedLevel(criteria)
	return report
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

// Note: runDoctor deliberately does NOT resolve/validate the module's
// layer pack (semantic.LoadLayerIn) as a criterion — a servable/published
// module needs a conforming STORE, not necessarily a live layer pack still
// on disk (docs/layers.md's own "orphaned layer" doctrine: claims keep
// serving after a pack directory is removed).
