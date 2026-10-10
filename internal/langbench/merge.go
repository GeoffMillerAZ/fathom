package langbench

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

// SubjectTenancy is one subject's raw resource numbers.
type SubjectTenancy struct {
	Faults        int      `json:"faults"`
	FaultingFiles []string `json:"faulting_files"`
	// FilesWithError (LB-F59, O203): the subject's measured files whose
	// tree on the dialect's own pinned grammar holds an ERROR or MISSING
	// node; nil when not measured (doc and native engines).
	FilesWithError *int    `json:"files_with_error,omitempty"`
	ExtractMS      float64 `json:"extract_ms"`
	// ExtractCPUMS is the extraction process's CPU time (user+system)
	// over the same timed extractions (owner decision D1); 0 on a
	// subject measured before D1.
	ExtractCPUMS    float64 `json:"extract_cpu_ms,omitempty"`
	MaxFileMS       int64   `json:"max_file_ms"`
	PeakModulePages int     `json:"peak_module_pages"`
	CompiledBytes   int64   `json:"compiled_bytes"`
}

// NewReading starts an empty reading for a dialect.
func NewReading(m Manifest, target string) Reading {
	kind := m.Kind
	if kind == "" {
		kind = "language"
	}
	r := Reading{Dialect: m.ID, Kind: kind, TargetLevel: target, RequiredKinds: m.RequiredCanonical()}
	r.Finalize()
	return r
}

var familyMetrics = map[string][]string{
	FamilyL1:        {MSymRecall, MSymPrecision, MKindAgreement, MRangeAgreement, MSignatureCoverage},
	FamilyCalls:     {MCallRecall, MCallPrecision},
	FamilyDefMap:    {MDefMapping},
	FamilyTyped:     {MTypedRecall},
	FamilyFileEdges: {MFileEdgePrecision, MFileEdgeRecall},
	FamilyImports:   {MImportResolution},
	FamilyRefs:      {MRefRecall, MRefPrecision},
	FamilyXref:      {MXrefPrecision, MXrefRecall},
}

var tierRank = map[string]int{"A": 4, "B": 3, "C": 2, "D": 1}

func ratio(num, den int) *float64 {
	if den == 0 {
		return nil
	}
	v := math.Round(float64(num)/float64(den)*1e4) / 1e4
	return &v
}

// MergeSubject adds (or replaces, by name) one subject's measurement and
// recomputes the reading: counts summed, metrics as ratios of the sums,
// each metric's tier the worst any contributing subject used, tenancy
// summed and maxed, then Finalize.
func MergeSubject(r *Reading, s Subject) {
	kept := r.Subjects[:0]
	for _, old := range r.Subjects {
		if old.Name != s.Name {
			kept = append(kept, old)
		}
	}
	r.Subjects = append(kept, s)
	sort.Slice(r.Subjects, func(i, j int) bool { return r.Subjects[i].Name < r.Subjects[j].Name })
	recompute(r)
}

// Regrade recomputes a reading from its recorded subjects under the
// current harness rules (thresholds, not_measured, name and kind rules that
// act on counts) without re-extracting: the subjects' raw counts are the
// measurement, the level is always derived from them.
func Regrade(r *Reading) { recompute(r) }

func recompute(r *Reading) {
	counts := map[string]int{}
	tiers := map[string]string{}
	excluded := map[string]int{}
	inter := map[string]bool{}
	var ten Tenancy
	var bytes int64
	var ms, cpuMS float64
	allCPU := len(r.Subjects) > 0
	for _, s := range r.Subjects {
		for k, v := range s.Counts {
			counts[k] += v
		}
		for fam, tier := range s.Families {
			if cur, ok := tiers[fam]; !ok || tierRank[tier] < tierRank[cur] {
				tiers[fam] = tier
			}
		}
		for k, v := range s.ExcludedByKind {
			excluded[k] += v
		}
		for _, k := range s.KindIntersection {
			inter[k] = true
		}
		bytes += s.Bytes
		if t := s.Tenancy; t != nil {
			ten.Faults += t.Faults
			for _, f := range t.FaultingFiles {
				ten.FaultingFiles = append(ten.FaultingFiles, s.Name+":"+f)
			}
			if t.FilesWithError != nil {
				ten.FilesWithError = new(*t.FilesWithError + derefInt(ten.FilesWithError))
			}
			ms += t.ExtractMS
			cpuMS += t.ExtractCPUMS
			allCPU = allCPU && t.ExtractCPUMS > 0
			ten.MaxFileMS = max(ten.MaxFileMS, t.MaxFileMS)
			ten.PeakModulePages = max(ten.PeakModulePages, t.PeakModulePages)
			ten.CompiledBytes = max(ten.CompiledBytes, t.CompiledBytes)
		}
	}
	mbs := func(ms float64) float64 { return math.Round(float64(bytes)/1e6/(ms/1000)*100) / 100 }
	switch {
	case allCPU && cpuMS > 0:
		ten.ThroughputMBs, ten.ThroughputBasis = mbs(cpuMS), ThroughputBasisCPU
		if ms > 0 {
			ten.WallThroughputMBs = mbs(ms)
		}
	case ms > 0:
		ten.ThroughputMBs, ten.ThroughputBasis = mbs(ms), ThroughputBasisWall
	}
	r.Tenancy = ten
	r.Counts = counts
	r.ExcludedByKind = excluded
	r.KindIntersection = r.KindIntersection[:0]
	for k := range inter {
		r.KindIntersection = append(r.KindIntersection, k)
	}

	all := map[string]*float64{}
	metricTiers := map[string]string{}
	var keys []string
	if r.Kind == "tooling" {
		keys = []string{MSymRecall, MSymPrecision, MRefRecall, MRefPrecision, MXrefPrecision, MXrefRecall}
	} else {
		keys = []string{MSymRecall, MSymPrecision, MKindAgreement, MRangeAgreement, MSignatureCoverage,
			MCallRecall, MCallPrecision, MImportResolution, MFileEdgePrecision, MFileEdgeRecall, MDefMapping, MTypedRecall}
	}
	for _, k := range keys {
		all[k] = nil
	}
	measured := func(fam string) bool { _, ok := tiers[fam]; return ok }
	set := func(fam, metric string, v *float64) {
		if !measured(fam) || v == nil || !slices.Contains(keys, metric) {
			return
		}
		all[metric] = v
		metricTiers[metric] = tiers[fam]
	}
	set(FamilyL1, MSymRecall, ratio(counts[CntMatched], counts[CntOracleDefs]))
	set(FamilyL1, MSymPrecision, ratio(counts[CntMatched], counts[CntFathomDefs]-counts[CntAdjudicated]))
	set(FamilyL1, MKindAgreement, ratio(counts[CntKindEqual], counts[CntMatched]))
	set(FamilyL1, MRangeAgreement, ratio(counts[CntRangeOK], counts[CntRangeEligible]))
	set(FamilyL1, MSignatureCoverage, ratio(counts[CntCallablesSigned], counts[CntCallables]))
	set(FamilyCalls, MCallRecall, ratio(counts[CntCallTruthFound], counts[CntCallTruth]))
	// LB-F203: the oracle-blind exclusion is a rule the harness computes, and
	// its cap applies per subject (owner ruling W3, 2026-10-10, O482): when a
	// subject's blind pairs exceed OracleBlindCapPct of its own fathom pairs
	// (found + blind) the oracle judges too little of that subject's calls for
	// the rule to claim a level there, so its blind pairs are back in as
	// fathom-only (still counted). A large typed subject no longer carries an
	// untyped one under the cap.
	r.Notes = slices.DeleteFunc(r.Notes, func(n string) bool {
		return strings.HasPrefix(n, noteOracleBlindCapPrefix) || strings.HasPrefix(n, noteOracleBlindSharePrefix)
	})
	fathomCalls := counts[CntCallFathom]
	if back, over := oracleBlindPerSubject(r.Subjects, CntCallsOracleBlind, CntCallFathom); back > 0 {
		fathomCalls += back
		r.Notes = append(r.Notes, fmt.Sprintf("%s%s fathom call pairs have no occurrence of the oracle at any call site written for them, more than the %d%% cap on each subject's own pairs: the oracle-blind exclusion claims no level for those subjects, so call_precision counts their %d blind pairs as fathom-only (calls_oracle_blind is still counted; owner-architect decision 2026-10-02, LB-F203; the cap per subject, owner ruling W3, 2026-10-10)", noteOracleBlindCapPrefix, strings.Join(over, ", "), OracleBlindCapPct, back))
		sort.Strings(r.Notes)
	}
	set(FamilyCalls, MCallPrecision, ratio(counts[CntCallFathomInTrue], fathomCalls))
	set(FamilyDefMap, MDefMapping, ratio(counts[CntScipDefsMapped], counts[CntScipDefs]))
	set(FamilyTyped, MTypedRecall, ratio(counts[CntScipEdgesTyped], counts[CntScipEdges]))
	// LB-F296: the file pairs the index cannot judge (CntFileEdgesOracleBlind)
	// are out of file_edge_precision under LB-F203's cap, per subject (W3): back
	// in for a subject over it on its own file pairs.
	fathomFileEdges := counts[CntFileEdgesFathom]
	r.Notes = slices.DeleteFunc(r.Notes, func(n string) bool { return strings.HasPrefix(n, noteFileEdgesBlindCapPrefix) })
	if back, over := oracleBlindPerSubject(r.Subjects, CntFileEdgesOracleBlind, CntFileEdgesFathom); back > 0 {
		fathomFileEdges += back
		r.Notes = append(r.Notes, fmt.Sprintf("%s%s fathom file pairs have no occurrence of the index at any site written for them, more than the %d%% cap on each subject's own pairs: the exclusion claims no level for those subjects, so file_edge_precision counts their %d blind pairs as fathom-only (file_edges_oracle_blind is still counted; LB-F296 under LB-F203, the cap per subject, owner ruling W3)", noteFileEdgesBlindCapPrefix, strings.Join(over, ", "), OracleBlindCapPct, back))
		sort.Strings(r.Notes)
	}
	set(FamilyFileEdges, MFileEdgePrecision, ratio(counts[CntFileEdgesAgree], fathomFileEdges))
	set(FamilyFileEdges, MFileEdgeRecall, ratio(counts[CntFileEdgesAgree], counts[CntFileEdgesTruth]))
	// LB-F40: L3 import resolution (imports.go).
	set(FamilyImports, MImportResolution, ratio(counts[CntImportsAgree], counts[CntImportsInRepo]))
	set(FamilyRefs, MRefRecall, ratio(counts[CntRefTruthFound], counts[CntRefTruth]))
	set(FamilyRefs, MRefPrecision, ratio(counts[CntRefFathomInTrue], counts[CntRefFathom]))
	// LB-F30: tool-format cross-file references (xref.go).
	set(FamilyXref, MXrefPrecision, ratio(counts[CntXrefFathomInTrue], counts[CntXrefFathom]))
	set(FamilyXref, MXrefRecall, ratio(counts[CntXrefTruthFound], counts[CntXrefTruth]))
	r.Metrics = all
	r.MetricTiers = metricTiers
	r.NotApplicable = nil
	if measured(FamilyL1) && counts[CntCallables] == 0 && r.Kind != "tooling" {
		r.NotApplicable = []string{MSignatureCoverage}
	}
	// An L1 oracle that matched definitions but reported an end line for
	// none of them cannot measure range_agreement: not_measured, disclosed,
	// and L1 capped at good (§1.2) — never a failure, never excellence.
	r.NotMeasured = nil
	r.Notes = slices.DeleteFunc(r.Notes, func(n string) bool { return n == NoteRangeNotMeasured })
	if measured(FamilyL1) && r.Kind != "tooling" && counts[CntMatched] > 0 && counts[CntRangeEligible] == 0 {
		r.NotMeasured = []string{MRangeAgreement}
		r.Notes = append(r.Notes, NoteRangeNotMeasured)
		sort.Strings(r.Notes)
	}
	r.Notes = slices.DeleteFunc(r.Notes, func(n string) bool {
		return strings.HasPrefix(n, noteImportsPrefix) || (measured(FamilyImports) && n == noteImportsNotMeasured)
	})
	if measured(FamilyImports) && r.Kind != "tooling" {
		r.Notes = append(r.Notes, importsNote(counts))
		sort.Strings(r.Notes)
	}
	r.Notes = slices.DeleteFunc(r.Notes, func(n string) bool { return strings.HasPrefix(n, noteAdjudicatedPrefix) })
	if n := counts[CntAdjudicated]; n > 0 {
		r.Notes = append(r.Notes, fmt.Sprintf("%s%d dialect-only definitions the L1 oracle is verified to miss (each listed under its subject's adjudicated, admitted by langbench.Adjudicate's deterministic file:line, name and module-scope check, owner decision D3) are left out of sym_precision's denominator", noteAdjudicatedPrefix, n))
		sort.Strings(r.Notes)
	}
	r.Finalize()
}

// derefInt is *p, or 0 for nil.
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// noteAdjudicatedPrefix opens the reading's D3 disclosure note.
const noteAdjudicatedPrefix = "adjudicated: "

// OracleBlindCapPct is the share of a reading's fathom call pairs the
// oracle may be unable to judge before the oracle-blind exclusion (LB-F175)
// stops counting toward a level (LB-F203).
const OracleBlindCapPct = 10

const noteOracleBlindCapPrefix = "oracle-blind cap fired: "
const noteFileEdgesBlindCapPrefix = "file-edge oracle-blind cap fired: "
const noteOracleBlindSharePrefix = "oracle-blind share: "

// OracleBlindCapFired reports whether blind pairs exceed the cap's share of
// fathom's pairs (found, blind excluded, plus blind); exactly the cap keeps.
func OracleBlindCapFired(blind, found int) bool {
	return blind > 0 && blind*100 > OracleBlindCapPct*(found+blind)
}

// oracleBlindPerSubject applies the oracle-blind cap per subject (owner ruling
// W3, 2026-10-10, O482): the blind pairs (blindKey) of every subject over the
// cap on its own pairs (foundKey plus blind), their sum, and each such subject
// named with its share, in subject order.
func oracleBlindPerSubject(subjects []Subject, blindKey, foundKey string) (back int, over []string) {
	for _, sub := range subjects {
		b, found := sub.Counts[blindKey], sub.Counts[foundKey]
		if OracleBlindCapFired(b, found) {
			back += b
			over = append(over, fmt.Sprintf("%s %d of %d (%.1f%%)", sub.Name, b, b+found, 100*float64(b)/float64(b+found)))
		}
	}
	return back, over
}
