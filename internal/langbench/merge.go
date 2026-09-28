package langbench

import (
	"math"
	"slices"
	"sort"
)

// SubjectTenancy is one subject's raw resource numbers.
type SubjectTenancy struct {
	Faults        int      `json:"faults"`
	FaultingFiles []string `json:"faulting_files"`
	ExtractMS     float64  `json:"extract_ms"`
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
	FamilyRefs:      {MRefRecall, MRefPrecision},
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
	set(FamilyL1, MSymPrecision, ratio(counts[CntMatched], counts[CntFathomDefs]))
	set(FamilyL1, MKindAgreement, ratio(counts[CntKindEqual], counts[CntMatched]))
	set(FamilyL1, MRangeAgreement, ratio(counts[CntRangeOK], counts[CntRangeEligible]))
	set(FamilyL1, MSignatureCoverage, ratio(counts[CntCallablesSigned], counts[CntCallables]))
	set(FamilyCalls, MCallRecall, ratio(counts[CntCallTruthFound], counts[CntCallTruth]))
	set(FamilyCalls, MCallPrecision, ratio(counts[CntCallFathomInTrue], counts[CntCallFathom]))
	set(FamilyDefMap, MDefMapping, ratio(counts[CntScipDefsMapped], counts[CntScipDefs]))
	set(FamilyTyped, MTypedRecall, ratio(counts[CntScipEdgesTyped], counts[CntScipEdges]))
	set(FamilyFileEdges, MFileEdgePrecision, ratio(counts[CntFileEdgesAgree], counts[CntFileEdgesFathom]))
	set(FamilyFileEdges, MFileEdgeRecall, ratio(counts[CntFileEdgesAgree], counts[CntFileEdgesTruth]))
	set(FamilyRefs, MRefRecall, ratio(counts[CntRefTruthFound], counts[CntRefTruth]))
	set(FamilyRefs, MRefPrecision, ratio(counts[CntRefFathomInTrue], counts[CntRefFathom]))
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
	r.Finalize()
}
