package langbench

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Metric names (design §1.2). A reading's metrics map is keyed by these;
// any other key is refused.
const (
	MSymRecall         = "sym_recall"
	MSymPrecision      = "sym_precision"
	MKindAgreement     = "kind_agreement"
	MRangeAgreement    = "range_agreement"
	MSignatureCoverage = "signature_coverage"
	MCallRecall        = "call_recall"
	MCallPrecision     = "call_precision"
	MImportResolution  = "import_resolution"
	MFileEdgePrecision = "file_edge_precision"
	MFileEdgeRecall    = "file_edge_recall"
	MDefMapping        = "def_mapping"
	MTypedRecall       = "typed_recall"
	// Tool formats: L2 intra-file references, L3 cross-file references.
	MRefRecall     = "ref_recall"
	MRefPrecision  = "ref_precision"
	MXrefPrecision = "xref_precision"
	MXrefRecall    = "xref_recall"
)

// Grades (§10). A level is met at good or excellent only.
const (
	GradeExcellent     = "excellent"
	GradeGood          = "good"
	GradeTolerated     = "tolerated" // only tier-D evidence: never a met level
	GradeUnmet         = "unmet"
	GradeNotAttempted  = "not-attempted"
	GradeNotApplicable = "not-applicable" // L4 for tool formats (§1.1)
)

// LevelNone is level_met when not even L1 is met.
const LevelNone = "none"

// Levels in ascending order.
var Levels = []string{"L1", "L2", "L3", "L4"}

// threshold is one metric's excellent and good floors.
type threshold struct {
	metric          string
	excellent, good float64
}

// languageThresholds and toolThresholds are §1.2's tables, per level. They
// are the only place a threshold lives; nothing else may weaken one.
var languageThresholds = map[string][]threshold{
	"L1": {
		{MSymRecall, .95, .85}, {MSymPrecision, .95, .90}, {MKindAgreement, .95, .85},
		{MRangeAgreement, .90, .75}, {MSignatureCoverage, .90, .70},
	},
	"L2": {{MCallRecall, .90, .75}, {MCallPrecision, .90, .85}},
	"L3": {
		{MImportResolution, .95, .85}, {MFileEdgePrecision, .95, .90}, {MFileEdgeRecall, .50, .25},
	},
	"L4": {{MDefMapping, .95, .85}, {MTypedRecall, .95, .85}},
}

var toolThresholds = map[string][]threshold{
	"L1": {{MSymRecall, .98, .95}, {MSymPrecision, .98, .95}},
	"L2": {{MRefRecall, .95, .90}, {MRefPrecision, .95, .90}},
	"L3": {{MXrefPrecision, .95, .90}, {MXrefRecall, .80, .60}},
}

// notApplicableAllowed lists the metrics a reading may declare
// not-applicable: a dialect with no callable kind has no signatures.
var notApplicableAllowed = []string{MSignatureCoverage}

// notMeasuredAllowed lists the metrics a reading may record as
// not_measured: the field exists in the language, but the only available
// L1 oracle cannot report it (universal-ctags writes no end line for
// HTML, TypeScript, JavaScript, Rust, Kotlin, Lua or PHP). A not-measured
// metric is never a failure, and never proof: the level it belongs to is
// met at most at good (§1.2).
var notMeasuredAllowed = []string{MRangeAgreement}

// NoteRangeNotMeasured is the disclosure a reading carries while
// range_agreement is not_measured.
const NoteRangeNotMeasured = "range_agreement not measured: the L1 oracle reported no end line for any matched definition, so L1 can be met at most at good, never excellent (design §1.2)"

// Tenancy floors (§1.2, always required).
const (
	MaxFaultRatio      = 0.001
	MinThroughputMBs   = 1.0
	MaxFileMS          = 2000
	MaxPeakModulePages = 1024
)

// Throughput bases (owner decision D1, 2026-09-28): the floor is judged on
// CPU time so a reading taken under contention is not failed by it.
const (
	ThroughputBasisCPU  = "cpu"
	ThroughputBasisWall = "wall"
)

// Tiers (§1.3).
var validTiers = []string{"A", "B", "C", "D"}

// Grade is the per-level grade (§10). Field order is the JSON order.
type Grade struct {
	L1 string `json:"L1"`
	L2 string `json:"L2"`
	L3 string `json:"L3"`
	L4 string `json:"L4"`
}

func (g *Grade) set(level, v string) {
	switch level {
	case "L1":
		g.L1 = v
	case "L2":
		g.L2 = v
	case "L3":
		g.L3 = v
	case "L4":
		g.L4 = v
	}
}

func (g Grade) get(level string) string {
	switch level {
	case "L1":
		return g.L1
	case "L2":
		return g.L2
	case "L3":
		return g.L3
	}
	return g.L4
}

// GrammarRef names the grammar blob a reading measured.
type GrammarRef struct {
	Blob       string `json:"blob"`
	Commit     string `json:"commit,omitempty"`
	BlobSHA256 string `json:"blob_sha256,omitempty"`
}

// Subject is one calibration subject a reading covers.
type Subject struct {
	Name  string `json:"name"`
	SHA   string `json:"sha,omitempty"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
	// The raw measurement a merged reading is recomputed from (LB-F23):
	// counts by name, the tier each oracle family was measured at, the
	// kinds excluded and intersected, and the subject's tenancy.
	Counts           map[string]int    `json:"counts,omitempty"`
	Families         map[string]string `json:"families,omitempty"`
	KindIntersection []string          `json:"kind_intersection,omitempty"`
	ExcludedByKind   map[string]int    `json:"excluded_by_kind,omitempty"`
	Tenancy          *SubjectTenancy   `json:"tenancy,omitempty"`
	// Adjudicated lists the dialect-only definitions admitted by
	// langbench.Adjudicate (D3) as "path:line name (class)".
	Adjudicated []string `json:"adjudicated,omitempty"`
}

// OracleRun names an oracle a reading used.
type OracleRun struct {
	Tier    string `json:"tier"`
	Tool    string `json:"tool"`
	Version string `json:"version,omitempty"`
	Status  string `json:"status,omitempty"` // "" (ran) or "unavailable: <why>"
	// Source, Revision and Licence name a third-party definitions query
	// run as a tier-C oracle (ADR-048's D5 extension, LB-F52): "<owner/repo>
	// <path>", the 40-hex commit it was fetched at, and its SPDX licence.
	Source   string `json:"source,omitempty"`
	Revision string `json:"revision,omitempty"`
	Licence  string `json:"licence,omitempty"`
}

// RevisionPattern is a pinned commit: 40 lowercase hex digits.
var RevisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Tenancy is §9's per-reading resource record.
type Tenancy struct {
	Faults        int      `json:"faults"`
	FaultingFiles []string `json:"faulting_files"`
	// FilesWithError (LB-F59, O203) is the measured files whose tree on the
	// dialect's own pinned grammar holds an ERROR or MISSING node: §14.2's
	// parse-error refusal line, read off the reading; for the Rego native
	// reader, the files it read only by a repair. The sum over the subjects
	// that report one; nil when none does (a doc engine, any other native
	// reader, a reading measured before the count existed), so such a
	// reading's bytes are unchanged.
	FilesWithError *int `json:"files_with_error,omitempty"`
	// ThroughputMBs is the gated number (§1.2): bytes per CPU-second of
	// the extraction process when every subject recorded CPU time
	// (ThroughputBasis "cpu", owner decision D1), else bytes per
	// wall-clock second ("wall", readings measured before D1).
	ThroughputMBs float64 `json:"throughput_mb_s"`
	// ThroughputBasis is "cpu" or "wall"; WallThroughputMBs discloses the
	// wall-clock figure alongside a cpu-basis reading.
	ThroughputBasis   string  `json:"throughput_basis,omitempty"`
	WallThroughputMBs float64 `json:"wall_throughput_mb_s,omitempty"`
	// MaxFileMS is the slowest file's time on ThroughputBasis's clock: CPU
	// time under "cpu", wall clock under "wall" (EW-3).
	MaxFileMS       int64 `json:"max_file_ms"`
	PeakModulePages int   `json:"peak_module_pages"`
	CompiledBytes   int64 `json:"compiled_bytes"`
}

// Reading is one dialect's committed measurement (§10). Nobody types
// LevelMet or Grade: Finalize computes them, Validate refuses a reading
// whose recorded values differ from the computation.
type Reading struct {
	Dialect          string              `json:"dialect"`
	Kind             string              `json:"kind"` // language | tooling
	TargetLevel      string              `json:"target_level"`
	LevelMet         string              `json:"level_met"`
	Grade            Grade               `json:"grade"`
	MeasuredAt       string              `json:"measured_at"`
	FathomCommit     string              `json:"fathom_commit"`
	Extractor        string              `json:"extractor,omitempty"`
	Grammar          *GrammarRef         `json:"grammar,omitempty"`
	Subjects         []Subject           `json:"subjects"`
	Oracles          []OracleRun         `json:"oracles"`
	Metrics          map[string]*float64 `json:"metrics"`
	MetricTiers      map[string]string   `json:"metric_tiers"`
	NotApplicable    []string            `json:"not_applicable,omitempty"`
	NotMeasured      []string            `json:"not_measured,omitempty"`
	RequiredKinds    []string            `json:"required_kinds"`
	KindIntersection []string            `json:"kind_intersection"`
	Counts           map[string]int      `json:"counts,omitempty"`
	Tenancy          Tenancy             `json:"tenancy"`
	ExcludedByKind   map[string]int      `json:"excluded_by_kind"`
	Notes            []string            `json:"notes"`
	Disposition      *string             `json:"disposition"`
}

func thresholdsFor(kind string) map[string][]threshold {
	if kind == "tooling" {
		return toolThresholds
	}
	return languageThresholds
}

// tierCeiling is the best grade a tier may prove for a metric at a level
// (§1.3): A proves anything; B proves L1 excellent and at most good above;
// C caps at good; D proves nothing (tolerated).
func tierCeiling(tier, level string) string {
	switch tier {
	case "A":
		return GradeExcellent
	case "B":
		if level == "L1" {
			return GradeExcellent
		}
		return GradeGood
	case "C":
		return GradeGood
	}
	return GradeTolerated
}

var gradeRank = map[string]int{
	GradeExcellent: 4, GradeGood: 3, GradeTolerated: 2, GradeUnmet: 1, GradeNotAttempted: 0,
}

func minGrade(a, b string) string {
	if gradeRank[a] < gradeRank[b] {
		return a
	}
	return b
}

// tenancyHolds is §1.2's always-required floor.
func tenancyHolds(r Reading) bool {
	files := 0
	for _, s := range r.Subjects {
		files += s.Files
	}
	t := r.Tenancy
	if files > 0 && float64(t.Faults)/float64(files) > MaxFaultRatio {
		return false
	}
	return t.ThroughputMBs >= MinThroughputMBs && t.MaxFileMS <= MaxFileMS &&
		t.PeakModulePages <= MaxPeakModulePages
}

// LevelMet computes a reading's level and per-level grade from its metrics,
// the §1.2 thresholds and the §1.3 tiers alone. A level is met when its own
// metrics grade good or better, every level below it is met, and the
// tenancy floor holds.
func LevelMet(r Reading) (string, Grade) {
	th := thresholdsFor(r.Kind)
	var g Grade
	levelMet := LevelNone
	chain := tenancyHolds(r)
	for _, level := range Levels {
		ts, ok := th[level]
		if !ok {
			g.set(level, GradeNotApplicable)
			chain = false
			continue
		}
		grade := gradeLevel(r, level, ts)
		g.set(level, grade)
		if chain && (grade == GradeExcellent || grade == GradeGood) {
			levelMet = level
		} else {
			chain = false
		}
	}
	return levelMet, g
}

func gradeLevel(r Reading, level string, ts []threshold) string {
	measured, missing := 0, 0
	grade := GradeExcellent
	capped := false
	for _, t := range ts {
		if slices.Contains(r.NotApplicable, t.metric) {
			continue
		}
		if slices.Contains(r.NotMeasured, t.metric) && r.Metrics[t.metric] == nil {
			capped = true
			continue
		}
		v := r.Metrics[t.metric]
		if v == nil {
			missing++
			continue
		}
		measured++
		var mg string
		switch {
		case *v >= t.excellent:
			mg = GradeExcellent
		case *v >= t.good:
			mg = GradeGood
		default:
			mg = GradeUnmet
		}
		if mg != GradeUnmet {
			mg = minGrade(mg, tierCeiling(r.MetricTiers[t.metric], level))
		}
		grade = minGrade(grade, mg)
	}
	switch {
	case measured == 0:
		return GradeNotAttempted
	case missing > 0:
		return GradeUnmet
	case capped:
		return minGrade(grade, GradeGood)
	}
	return grade
}

// Finalize computes LevelMet and Grade and normalizes the reading's slices
// (sorted, never nil) so its bytes are deterministic.
func (r *Reading) Finalize() {
	sort.Strings(r.RequiredKinds)
	sort.Strings(r.KindIntersection)
	sort.Strings(r.NotApplicable)
	sort.Strings(r.NotMeasured)
	sort.Strings(r.Tenancy.FaultingFiles)
	if r.RequiredKinds == nil {
		r.RequiredKinds = []string{}
	}
	if r.KindIntersection == nil {
		r.KindIntersection = []string{}
	}
	if r.Tenancy.FaultingFiles == nil {
		r.Tenancy.FaultingFiles = []string{}
	}
	if r.Notes == nil {
		r.Notes = []string{}
	}
	if r.Oracles == nil {
		r.Oracles = []OracleRun{}
	}
	if r.Subjects == nil {
		r.Subjects = []Subject{}
	}
	if r.Metrics == nil {
		r.Metrics = map[string]*float64{}
	}
	if r.MetricTiers == nil {
		r.MetricTiers = map[string]string{}
	}
	if r.ExcludedByKind == nil {
		r.ExcludedByKind = map[string]int{}
	}
	r.LevelMet, r.Grade = LevelMet(*r)
}

func knownMetric(name string) bool {
	for _, th := range []map[string][]threshold{languageThresholds, toolThresholds} {
		for _, ts := range th {
			for _, t := range ts {
				if t.metric == name {
					return true
				}
			}
		}
	}
	return false
}

// Validate refuses a reading that could not have come from the harness: a
// required kind outside the kind intersection (§1.4), a metric without an
// admissible tier or outside [0,1], an unknown metric, an unnamed fault, or
// a level_met/grade that differs from LevelMet's computation.
func (r Reading) Validate() error {
	if r.Dialect == "" || !QueueIDPattern.MatchString(r.Dialect) {
		return fmt.Errorf("reading: dialect %q must match %s", r.Dialect, QueueIDPattern)
	}
	if !slices.Contains(validKinds, r.Kind) {
		return fmt.Errorf("reading %s: kind %q not in %v", r.Dialect, r.Kind, validKinds)
	}
	if !slices.Contains(validLevels, r.TargetLevel) {
		return fmt.Errorf("reading %s: target_level %q", r.Dialect, r.TargetLevel)
	}
	if len(r.Subjects) == 0 {
		return fmt.Errorf("reading %s: no subjects", r.Dialect)
	}
	for _, o := range r.Oracles {
		if o.Source != "" && (!RevisionPattern.MatchString(o.Revision) || strings.TrimSpace(o.Licence) == "" || o.Tier != "C") {
			return fmt.Errorf("reading %s: oracle %s from %s needs a 40-hex revision (%q), a licence (%q) and tier C (%q) — ADR-048's D5 extension", r.Dialect, o.Tool, o.Source, o.Revision, o.Licence, o.Tier)
		}
	}
	// The refusal guards a MEASURED L1: an L1 oracle that ran with a
	// required kind outside the intersection would prove a level over a
	// hole. A reading whose L1 oracle was unavailable proves nothing at L1
	// and says so in its oracles.
	l1Measured := r.Metrics[MSymRecall] != nil || r.Metrics[MSymPrecision] != nil
	for _, k := range r.RequiredKinds {
		if l1Measured && !slices.Contains(r.KindIntersection, k) {
			return fmt.Errorf("reading %s: required kind %q is outside the kind intersection — refused (§1.4)", r.Dialect, k)
		}
	}
	for name, v := range r.Metrics {
		if !knownMetric(name) {
			return fmt.Errorf("reading %s: unknown metric %q", r.Dialect, name)
		}
		if v == nil {
			continue
		}
		if *v < 0 || *v > 1 {
			return fmt.Errorf("reading %s: metric %s = %v outside [0,1]", r.Dialect, name, *v)
		}
		if !slices.Contains(validTiers, r.MetricTiers[name]) {
			return fmt.Errorf("reading %s: metric %s has tier %q, want one of %v", r.Dialect, name, r.MetricTiers[name], validTiers)
		}
	}
	for _, name := range r.NotApplicable {
		if !slices.Contains(notApplicableAllowed, name) {
			return fmt.Errorf("reading %s: %s may not be declared not-applicable", r.Dialect, name)
		}
	}
	for _, name := range r.NotMeasured {
		if !slices.Contains(notMeasuredAllowed, name) {
			return fmt.Errorf("reading %s: %s may not be declared not_measured", r.Dialect, name)
		}
		if r.Metrics[name] != nil {
			return fmt.Errorf("reading %s: %s is declared not_measured but has a value", r.Dialect, name)
		}
	}
	if r.Tenancy.Faults != len(r.Tenancy.FaultingFiles) {
		return fmt.Errorf("reading %s: %d faults but %d faulting files named", r.Dialect, r.Tenancy.Faults, len(r.Tenancy.FaultingFiles))
	}
	if n := r.Tenancy.FilesWithError; n != nil {
		files := 0
		for _, s := range r.Subjects {
			files += s.Files
		}
		if *n < 0 || *n > files {
			return fmt.Errorf("reading %s: files_with_error %d outside 0..%d files measured", r.Dialect, *n, files)
		}
	}
	level, grade := LevelMet(r)
	if r.LevelMet != level {
		return fmt.Errorf("reading %s: level_met %q, computed %q — a level is never typed", r.Dialect, r.LevelMet, level)
	}
	if r.Grade != grade {
		return fmt.Errorf("reading %s: grade %+v, computed %+v", r.Dialect, r.Grade, grade)
	}
	return nil
}

// MarshalReading is the one writer of a reading's bytes (§10): two-space
// indent, trailing newline, so `"level_met": "L3"` is always one line.
func MarshalReading(r Reading) ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// GradeAt returns the grade recorded for level.
func (r Reading) GradeAt(level string) string { return r.Grade.get(level) }
