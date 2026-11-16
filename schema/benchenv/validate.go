// Package benchenv validates authored environment data, never empirical evidence.
package benchenv

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
)

//go:embed schema.cue
var schemaSource string

type Tier string

const (
	TierPublic      Tier = "public"
	TierConstructed Tier = "constructed"
	TierSmoke       Tier = "smoke"
)

type Split string

const (
	SplitDevelopment Split = "dev"
	SplitHeldOut     Split = "held-out"
)

type Axis string

const (
	AxisLines        Axis = "lines"
	AxisFiles        Axis = "files"
	AxisRepositories Axis = "repositories"
	AxisHistory      Axis = "history"
)

type ScaleClass string

const (
	ClassS  ScaleClass = "S"
	ClassM  ScaleClass = "M"
	ClassL  ScaleClass = "L"
	ClassXL ScaleClass = "XL"
)

type Prediction string

const (
	PredictionEffect Prediction = "effect"
	PredictionNull   Prediction = "null"
)

type Polarity string

const (
	PolarityPositive Polarity = "positive"
	PolarityNegative Polarity = "negative"
)

type ExclusionKind string

const (
	ExclusionVendored  ExclusionKind = "vendored"
	ExclusionGenerated ExclusionKind = "generated"
	ExclusionTest      ExclusionKind = "test"
	ExclusionBuild     ExclusionKind = "build"
)

type Ref struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type RepositoryPin struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Commit string `json:"commit"`
}
type Pin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}
type ExcludedCount struct {
	Kind  ExclusionKind `json:"kind"`
	Lines int64         `json:"lines"`
	Files int64         `json:"files"`
}
type Scale struct {
	Lines        *int64          `json:"lines"`
	Files        *int64          `json:"files"`
	Repositories *int64          `json:"repositories"`
	History      *int64          `json:"history"`
	Excluded     []ExcludedCount `json:"excluded"`
	Receipt      Ref             `json:"receipt"`
}
type ScaleClaim struct {
	Axis  Axis       `json:"axis"`
	Class ScaleClass `json:"class"`
}
type EnvironmentHead struct {
	ID           string          `json:"id"`
	Tier         Tier            `json:"tier"`
	Repositories []RepositoryPin `json:"repositories"`
	Subjects     []Ref           `json:"subjects"`
	Licence      Ref             `json:"licence"`
	Toolchains   []Pin           `json:"toolchains"`
	Images       []Pin           `json:"images"`
	SourcePolicy Ref             `json:"source_policy"`
	Scale        Scale           `json:"scale"`
	Split        Split           `json:"split"`
	PriorReads   []Ref           `json:"prior_reads"`
	Oracles      []Ref           `json:"oracles"`
	Receipts     []Ref           `json:"receipts"`
}
type Stratum struct {
	Subtype    string       `json:"subtype"`
	Language   string       `json:"language"`
	Scale      []ScaleClaim `json:"scale"`
	Prediction Prediction   `json:"prediction"`
}
type Requirement struct {
	ID              string  `json:"id"`
	Capability      string  `json:"capability"`
	Card            Ref     `json:"card"`
	CheckerContract Ref     `json:"checker_contract"`
	Concept         string  `json:"concept"`
	Stratum         Stratum `json:"stratum"`
	PositiveMinimum int     `json:"positive_minimum"`
	NegativeMinimum int     `json:"negative_minimum"`
	Checker         Pin     `json:"checker"`
}
type Anchor struct {
	Repository RepositoryPin `json:"repository"`
	Path       string        `json:"path"`
	StartLine  int           `json:"start_line"`
	EndLine    int           `json:"end_line"`
}
type Witness struct {
	ID          string   `json:"id"`
	Environment Ref      `json:"environment"`
	Requirement Ref      `json:"requirement"`
	Anchor      Anchor   `json:"anchor"`
	Polarity    Polarity `json:"polarity"`
	MatchID     string   `json:"match_id"`
	Origin      Ref      `json:"origin"`
	Checker     Pin      `json:"checker"`
	Receipt     Ref      `json:"receipt"`
}
type CoverageClaim struct {
	ID           string  `json:"id"`
	Capability   string  `json:"capability"`
	Environment  Ref     `json:"environment"`
	Stratum      Stratum `json:"stratum"`
	Requirements []Ref   `json:"requirements"`
}
type EnvironmentRecords struct {
	Environments []EnvironmentHead `json:"environments"`
	Requirements []Requirement     `json:"requirements"`
	Witnesses    []Witness         `json:"witnesses"`
	Coverage     []CoverageClaim   `json:"coverage"`
}

type Metric string

const (
	MetricNone                Metric = "none"
	MetricPositive            Metric = "positive"
	MetricNegative            Metric = "negative"
	MetricAbsentSymbols       Metric = "absent_symbols"
	MetricUnwiredEntries      Metric = "unwired_entries"
	MetricCrossFilePaths      Metric = "cross_file_paths"
	MetricRepositoryLinks     Metric = "repository_links"
	MetricLinkedRepositories  Metric = "linked_repositories"
	MetricLinkedServices      Metric = "linked_services"
	MetricFaninSymbols        Metric = "fanin_symbols"
	MetricDispatchSites       Metric = "dispatch_sites"
	MetricHazardousChanges    Metric = "hazardous_changes"
	MetricSafeChanges         Metric = "safe_changes"
	MetricModuleExpressions   Metric = "module_expressions"
	MetricMissingContext      Metric = "missing_context"
	MetricProviderTransitions Metric = "provider_transitions"
	MetricStaleInstructions   Metric = "stale_instructions"
	MetricLiveInstructions    Metric = "live_instructions"
	MetricGoverningLinks      Metric = "governing_links"
	MetricPublicFixes         Metric = "public_fixes"
	MetricCollateralMutants   Metric = "collateral_mutants"
	MetricNonlexicalIssues    Metric = "nonlexical_issues"
	MetricLexicalIssues       Metric = "lexical_issues"
	MetricConflictingPairs    Metric = "conflicting_pairs"
	MetricCleanPairs          Metric = "clean_pairs"
	MetricHistoryCommits      Metric = "history_commits"
	MetricRelevantChanges     Metric = "relevant_changes"
	MetricBoundarySites       Metric = "boundary_sites"
	MetricLanguageSites       Metric = "language_sites"
	MetricDataReads           Metric = "data_reads"
	MetricSubprocessReads     Metric = "subprocess_reads"
	MetricUncoveredChanges    Metric = "uncovered_changes"
	MetricAcquireReleasePairs Metric = "acquire_release_pairs"
	MetricMissingReleaseFixes Metric = "missing_release_fixes"
	MetricEscapes             Metric = "escapes"
	MetricBoundedSites        Metric = "bounded_sites"
	MetricUnboundedSites      Metric = "unbounded_sites"
	MetricShutdownCases       Metric = "shutdown_cases"
	MetricMultisessionTasks   Metric = "multisession_tasks"
	MetricDeclaredDeletions   Metric = "declared_deletions"
	MetricUndeclaredDeletions Metric = "undeclared_deletions"
	MetricRuleLabels          Metric = "rule_labels"
	MetricPinnedInputs        Metric = "pinned_inputs"
	MetricFloatingInputs      Metric = "floating_inputs"
	MetricInterfaceChanges    Metric = "interface_changes"
	MetricValidEnvelopes      Metric = "valid_envelopes"
	MetricInvalidEnvelopes    Metric = "invalid_envelopes"
	MetricResolvableBindings  Metric = "resolvable_bindings"
	MetricAmbiguousBindings   Metric = "ambiguous_bindings"
	MetricOutputCases         Metric = "output_cases"
	MetricLongOutputs         Metric = "long_outputs"
	MetricShortOutputs        Metric = "short_outputs"
	MetricQuestionCases       Metric = "question_cases"
	MetricRouteQuestions      Metric = "route_questions"
	MetricRankQuestions       Metric = "rank_questions"
	MetricTableQuestions      Metric = "table_questions"
)

type MetricUnit string

const (
	MetricUnitSymbols      MetricUnit = "symbols"
	MetricUnitEntryPoints  MetricUnit = "entry_points"
	MetricUnitPaths        MetricUnit = "paths"
	MetricUnitLinks        MetricUnit = "links"
	MetricUnitRepositories MetricUnit = "repositories"
	MetricUnitServices     MetricUnit = "services"
	MetricUnitSites        MetricUnit = "sites"
	MetricUnitChanges      MetricUnit = "changes"
	MetricUnitExpressions  MetricUnit = "expressions"
	MetricUnitCases        MetricUnit = "cases"
	MetricUnitTransitions  MetricUnit = "transitions"
	MetricUnitInstructions MetricUnit = "instructions"
	MetricUnitFixes        MetricUnit = "fixes"
	MetricUnitMutants      MetricUnit = "mutants"
	MetricUnitIssues       MetricUnit = "issues"
	MetricUnitPairs        MetricUnit = "pairs"
	MetricUnitCommits      MetricUnit = "commits"
	MetricUnitReads        MetricUnit = "reads"
	MetricUnitEscapes      MetricUnit = "escapes"
	MetricUnitTasks        MetricUnit = "tasks"
	MetricUnitDeletions    MetricUnit = "deletions"
	MetricUnitLabels       MetricUnit = "labels"
	MetricUnitInputs       MetricUnit = "inputs"
	MetricUnitEnvelopes    MetricUnit = "envelopes"
	MetricUnitBindings     MetricUnit = "bindings"
	MetricUnitOutputs      MetricUnit = "outputs"
	MetricUnitQuestions    MetricUnit = "questions"
)

type MinimumCitation string

const (
	MinimumFalsePlan       MinimumCitation = "BE-D2:False plan claims"
	MinimumCrossRepository MinimumCitation = "BE-D2:Cross-repository dependencies"
	MinimumFanin           MinimumCitation = "BE-D2:Fan-in and dispatch"
	MinimumOutput          MinimumCitation = "BE-D2:Output fidelity and judgments"
)

type MetricExpectation struct {
	Metric       Metric          `json:"metric"`
	Minimum      int64           `json:"minimum"`
	Unit         MetricUnit      `json:"unit"`
	LowerMinimum MinimumCitation `json:"lower_minimum,omitempty"`
}

type ConceptDesignRow string

const (
	ConceptDesignBED2 ConceptDesignRow = "BE-D2"
	ConceptDesignBED9 ConceptDesignRow = "BE-D9"
)

type ConceptCitation struct {
	Card      *Ref             `json:"card,omitempty"`
	DesignRow ConceptDesignRow `json:"design_row,omitempty"`
}
type ConceptRecord struct {
	RequiredBy   []ConceptCitation   `json:"required_by"`
	Expectations []MetricExpectation `json:"expectations"`
}
type ConceptGroup struct {
	Card            Ref      `json:"card"`
	CheckerContract Ref      `json:"checker_contract"`
	Concepts        []string `json:"concepts"`
}
type ConceptInventory struct {
	Concepts map[string]ConceptRecord `json:"concepts"`
	Groups   []ConceptGroup           `json:"groups"`
}

// schema/benchenv/validate.go; source is CUE data or its JSON subset.

type ValidationError struct {
	Path   string           `json:"path"`
	Reason ValidationReason `json:"reason"`
}
type ValidationReason string

const (
	ValidationShape     ValidationReason = "shape"
	ValidationPath      ValidationReason = "path"
	ValidationReference ValidationReason = "reference"
	ValidationIdentity  ValidationReason = "identity"
)

type PreflightInput struct {
	PairID        string `json:"pair_id"`
	Arm           string `json:"arm"`
	Environment   Ref    `json:"environment"`
	ProfileSHA256 string `json:"profile_sha256"`
	Receipt       Ref    `json:"receipt"`
}
type AgentOutcome struct {
	PairID  string `json:"pair_id"`
	Arm     string `json:"arm"`
	CellKey string `json:"cell_key"`
	RunID   string `json:"run_id"`
	Raw     Ref    `json:"raw"`
}
type FreshReadingInput struct {
	ID          string           `json:"id"`
	RunID       string           `json:"run_id"`
	MeasuredAt  string           `json:"measured_at"`
	Environment Ref              `json:"environment"`
	Matrix      Ref              `json:"matrix"`
	Fingerprint Ref              `json:"fingerprint"`
	Raw         Ref              `json:"raw"`
	Preflights  []PreflightInput `json:"preflights"`
	Outcomes    []AgentOutcome   `json:"outcomes"`
}

// Fleet declarations describe inputs and outputs without materializing them.
type FileMode string

const (
	FileRegular    FileMode = "100644"
	FileExecutable FileMode = "100755"
)

type FileClass string

const (
	FileProduct   FileClass = "product"
	FileVendored  FileClass = "vendored"
	FileGenerated FileClass = "generated"
	FileTest      FileClass = "test"
	FileBuild     FileClass = "build"
)

type FleetFile struct {
	Path         string    `json:"path"`
	ContentID    string    `json:"content_id"`
	SourceSHA256 string    `json:"source_sha256"`
	SHA256       string    `json:"sha256"`
	Mode         FileMode  `json:"mode"`
	Class        FileClass `json:"class"`
	Seeded       bool      `json:"seeded"`
}
type FleetCommit struct {
	ID      string      `json:"id"`
	Parents []string    `json:"parents"`
	Files   []FleetFile `json:"files"`
}
type FleetRepository struct {
	ID      string        `json:"id"`
	Head    string        `json:"head"`
	Commits []FleetCommit `json:"commits"`
}
type FleetSize struct {
	Lines        int64 `json:"lines"`
	Files        int64 `json:"files"`
	Repositories int64 `json:"repositories"`
	History      int64 `json:"history"`
}
type GeneratorVersion string

const (
	GeneratorVersionV1 GeneratorVersion = "env-fleet/v1"
)

type FleetManifest struct {
	Seed             uint64            `json:"seed"`
	GeneratorVersion GeneratorVersion  `json:"generator_version"`
	Template         string            `json:"template"`
	Size             FleetSize         `json:"size"`
	Repositories     []FleetRepository `json:"repositories"`
	Truth            []FleetFile       `json:"truth"`
}
type FleetLimits struct {
	MaxRepositories int64 `json:"max_repositories"`
	MaxCommits      int64 `json:"max_commits"`
	MaxFiles        int64 `json:"max_files"`
	MaxBytes        int64 `json:"max_bytes"`
}
type FleetCommitResult struct {
	ID      string   `json:"id"`
	Commit  string   `json:"commit"`
	Parents []string `json:"parents"`
}
type FleetRepositoryResult struct {
	ID      string              `json:"id"`
	Head    string              `json:"head"`
	Commits []FleetCommitResult `json:"commits"`
}
type FleetResult struct {
	Repositories []FleetRepositoryResult `json:"repositories"`
	Truth        []Ref                   `json:"truth"`
	Size         FleetSize               `json:"size"`
	Excluded     []ExcludedCount         `json:"excluded"`
}
type FleetReason string

const (
	FleetShape        FleetReason = "shape"
	FleetPath         FleetReason = "path"
	FleetDuplicate    FleetReason = "duplicate"
	FleetParent       FleetReason = "parent"
	FleetCycle        FleetReason = "cycle"
	FleetDigest       FleetReason = "digest"
	FleetSizeMismatch FleetReason = "size_mismatch"
	FleetTarget       FleetReason = "target"
	FleetIO           FleetReason = "io"
	FleetCancelled    FleetReason = "cancelled"
)

type FleetError struct {
	Path   string      `json:"path"`
	Reason FleetReason `json:"reason"`
}
type FleetLimit string

const (
	LimitRepositories FleetLimit = "repositories"
	LimitCommits      FleetLimit = "commits"
	LimitFiles        FleetLimit = "files"
	LimitBytes        FleetLimit = "bytes"
)

type FleetLimitError struct {
	Path     string     `json:"path"`
	Kind     FleetLimit `json:"kind"`
	Limit    int64      `json:"limit"`
	Observed int64      `json:"observed"`
}

type EnvironmentMode string

const (
	ModeUnselected   EnvironmentMode = "" // Output only when input mode is unknown.
	ModeFixtures     EnvironmentMode = "fixtures"
	ModePortfolio    EnvironmentMode = "portfolio"
	ModeBaseline     EnvironmentMode = "baseline"
	ModeAudit        EnvironmentMode = "audit"
	ModeSeed         EnvironmentMode = "seed"
	ModeCandidate    EnvironmentMode = "candidate"
	ModeSeal         EnvironmentMode = "seal"
	ModeReading      EnvironmentMode = "reading"
	ModeSnapshots    EnvironmentMode = "snapshots"
	ModeModelRerun   EnvironmentMode = "model-rerun"
	ModeLandedReplay EnvironmentMode = "landed-replay"
)

type FixturesInput struct {
	Records  EnvironmentRecords `json:"records"`
	Catalog  Ref                `json:"catalog"`
	Evidence []Ref              `json:"evidence"`
}
type PortfolioInput struct {
	Records  EnvironmentRecords `json:"records"`
	Catalog  Ref                `json:"catalog"`
	Subjects []Ref              `json:"subjects"`
	Evidence []Ref              `json:"evidence"`
}
type BaselineInput struct {
	Matrix     Ref              `json:"matrix"`
	Preflights []PreflightInput `json:"preflights"`
	Outcomes   []AgentOutcome   `json:"outcomes"`
}
type AuditInput struct {
	Sources []Ref `json:"sources"`
	Runs    []Ref `json:"runs"`
	Rulings []Ref `json:"rulings"`
}
type SeedInput struct {
	Manifest     FleetManifest `json:"manifest"`
	Sources      []Ref         `json:"sources"`
	Limits       FleetLimits   `json:"limits"`
	Requirements []Requirement `json:"requirements"`
	Catalog      Ref           `json:"catalog"`
}
type CandidateInput struct {
	SubjectInput Ref                `json:"subject_input"`
	Records      EnvironmentRecords `json:"records"`
	Catalog      Ref                `json:"catalog"`
	Evidence     []Ref              `json:"evidence"`
}
type SealInput struct {
	Admission   Ref   `json:"admission"`
	Environment Ref   `json:"environment"`
	PriorReads  []Ref `json:"prior_reads"`
	Topology    Ref   `json:"topology"`
	OracleKeys  []Ref `json:"oracle_keys"`
}
type ReadingInput struct {
	Reading            FreshReadingInput `json:"reading"`
	ClaimTime          string            `json:"claim_time"`
	CurrentFingerprint Ref               `json:"current_fingerprint"`
	ExistingRuns       []string          `json:"existing_runs"`
	ExistingCellKeys   []string          `json:"existing_cell_keys"`
}
type SnapshotInput struct {
	ProductCommit string   `json:"product_commit"`
	BinarySHA256  string   `json:"binary_sha256"`
	Commits       []string `json:"commits"`
	Service       string   `json:"service"`
	WallCapMS     int64    `json:"wall_cap_ms"`
}
type ModelRerunInput struct {
	ReadingID          string   `json:"reading_id"`
	Preregistration    Ref      `json:"preregistration"`
	Approval           Ref      `json:"approval"`
	Spend              Ref      `json:"spend"`
	CellNamespace      string   `json:"cell_namespace"`
	ExistingNamespaces []string `json:"existing_namespaces"`
	MaxUSDMicros       int64    `json:"max_usd_micros"`
	WallCapMS          int64    `json:"wall_cap_ms"`
}
type LandedReplayInput struct {
	Input     Ref   `json:"input"`
	WallCapMS int64 `json:"wall_cap_ms"`
}
type EnvironmentRequest struct {
	Mode         EnvironmentMode    `json:"mode"`
	Fixtures     *FixturesInput     `json:"fixtures,omitempty"`
	Portfolio    *PortfolioInput    `json:"portfolio,omitempty"`
	Baseline     *BaselineInput     `json:"baseline,omitempty"`
	Audit        *AuditInput        `json:"audit,omitempty"`
	Seed         *SeedInput         `json:"seed,omitempty"`
	Candidate    *CandidateInput    `json:"candidate,omitempty"`
	Seal         *SealInput         `json:"seal,omitempty"`
	Reading      *ReadingInput      `json:"reading,omitempty"`
	Snapshots    *SnapshotInput     `json:"snapshots,omitempty"`
	ModelRerun   *ModelRerunInput   `json:"model_rerun,omitempty"`
	LandedReplay *LandedReplayInput `json:"landed_replay,omitempty"`
}
type ModeState string

const (
	ModePass    ModeState = "pass"
	ModeRefused ModeState = "refused"
	ModeUnknown ModeState = "unknown"
)

type ModeReason string

const (
	ModeShape       ModeReason = "shape"
	ModeUnknownName ModeReason = "unknown_mode"
	ModeDuplicate   ModeReason = "duplicate_mode"
	ModeUnavailable ModeReason = "unavailable"
	ModeDependency  ModeReason = "dependency"
	ModeCancelled   ModeReason = "cancelled"
	ModeLimit       ModeReason = "limit"
	ModeIO          ModeReason = "io"
)

type ModeError struct {
	Path   string     `json:"path"`
	Reason ModeReason `json:"reason"`
	cause  error      // Private, never JSON or rendered raw.
}
type ModeDiagnostic struct {
	Path     string     `json:"path"`
	Reason   ModeReason `json:"reason"`
	Evidence []Ref      `json:"evidence"`
}
type SnapshotCase string

const (
	SnapshotCold      SnapshotCase = "cold"
	SnapshotWarm      SnapshotCase = "warm"
	SnapshotCacheFrom SnapshotCase = "cache-from"
)

type SnapshotState string

const (
	SnapshotPassed    SnapshotState = "passed"
	SnapshotFailed    SnapshotState = "failed"
	SnapshotDegraded  SnapshotState = "degraded"
	SnapshotTimeout   SnapshotState = "timeout"
	SnapshotCancelled SnapshotState = "cancelled"
)

type SnapshotSample struct {
	Commit   string        `json:"commit"`
	Case     SnapshotCase  `json:"case"`
	State    SnapshotState `json:"state"`
	WallMS   *int64        `json:"wall_ms"`
	Evidence []Ref         `json:"evidence"`
}

var enumValues = map[string][]string{
	"Tier":             {"public", "constructed", "smoke"},
	"Split":            {"dev", "held-out"},
	"Axis":             {"lines", "files", "repositories", "history"},
	"ScaleClass":       {"S", "M", "L", "XL"},
	"Prediction":       {"effect", "null"},
	"Polarity":         {"positive", "negative"},
	"ExclusionKind":    {"vendored", "generated", "test", "build"},
	"Metric":           {"none", "positive", "negative", "absent_symbols", "unwired_entries", "cross_file_paths", "repository_links", "linked_repositories", "linked_services", "fanin_symbols", "dispatch_sites", "hazardous_changes", "safe_changes", "module_expressions", "missing_context", "provider_transitions", "stale_instructions", "live_instructions", "governing_links", "public_fixes", "collateral_mutants", "nonlexical_issues", "lexical_issues", "conflicting_pairs", "clean_pairs", "history_commits", "relevant_changes", "boundary_sites", "language_sites", "data_reads", "subprocess_reads", "uncovered_changes", "acquire_release_pairs", "missing_release_fixes", "escapes", "bounded_sites", "unbounded_sites", "shutdown_cases", "multisession_tasks", "declared_deletions", "undeclared_deletions", "rule_labels", "pinned_inputs", "floating_inputs", "interface_changes", "valid_envelopes", "invalid_envelopes", "resolvable_bindings", "ambiguous_bindings", "output_cases", "long_outputs", "short_outputs", "question_cases", "route_questions", "rank_questions", "table_questions"},
	"MetricUnit":       {"symbols", "entry_points", "paths", "links", "repositories", "services", "sites", "changes", "expressions", "cases", "transitions", "instructions", "fixes", "mutants", "issues", "pairs", "commits", "reads", "escapes", "tasks", "deletions", "labels", "inputs", "envelopes", "bindings", "outputs", "questions"},
	"MinimumCitation":  {"BE-D2:False plan claims", "BE-D2:Cross-repository dependencies", "BE-D2:Fan-in and dispatch", "BE-D2:Output fidelity and judgments"},
	"ConceptDesignRow": {"BE-D2", "BE-D9"},
	"ValidationReason": {"shape", "path", "reference", "identity"},
	"FileMode":         {"100644", "100755"},
	"FileClass":        {"product", "vendored", "generated", "test", "build"},
	"GeneratorVersion": {"env-fleet/v1"},
	"FleetReason":      {"shape", "path", "duplicate", "parent", "cycle", "digest", "size_mismatch", "target", "io", "cancelled"},
	"FleetLimit":       {"repositories", "commits", "files", "bytes"},
	"EnvironmentMode":  {"", "fixtures", "portfolio", "baseline", "audit", "seed", "candidate", "seal", "reading", "snapshots", "model-rerun", "landed-replay"},
	"ModeState":        {"pass", "refused", "unknown"},
	"ModeReason":       {"shape", "unknown_mode", "duplicate_mode", "unavailable", "dependency", "cancelled", "limit", "io"},
	"SnapshotCase":     {"cold", "warm", "cache-from"},
	"SnapshotState":    {"passed", "failed", "degraded", "timeout", "cancelled"},
}

// The wire schema is embedded; source validation performs no external reads.
func pointer(base, member string) string {
	return base + "/" + strings.ReplaceAll(strings.ReplaceAll(member, "~", "~0"), "/", "~1")
}
func indexed(base string, i int) string { return pointer(base, strconv.Itoa(i)) }
func first(es []*ValidationError) error {
	if len(es) == 0 {
		return nil
	}
	sort.Slice(es, func(i, j int) bool {
		if es[i].Path != es[j].Path {
			return es[i].Path < es[j].Path
		}
		return es[i].Reason < es[j].Reason
	})
	return es[0]
}
func add(es *[]*ValidationError, p string, r ValidationReason) {
	*es = append(*es, &ValidationError{Path: p, Reason: r})
}
func (e *ValidationError) Error() string { return "environment: " + string(e.Reason) }
func (e *FleetError) Error() string      { return "environment fleet: " + string(e.Reason) }
func (e *FleetLimitError) Error() string { return "environment fleet: limit" }
func canonicalPointer(s string) bool {
	if s != "" && !strings.HasPrefix(s, "/") {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '~' {
			if i+1 == len(s) || (s[i+1] != '0' && s[i+1] != '1') {
				return false
			}
			i++
		}
	}
	return true
}

// NewModeError normalizes the pointer and reason while retaining the original
// private cause for errors.Is and errors.As.
func NewModeError(p string, r ModeReason, cause error) *ModeError {
	if !canonicalPointer(p) {
		p = ""
	}
	if !contains(enumValues["ModeReason"], string(r)) {
		r = ModeShape
	}
	return &ModeError{Path: p, Reason: r, cause: cause}
}
func (e *ModeError) Error() string {
	r := e.Reason
	if !contains(enumValues["ModeReason"], string(r)) {
		r = ModeShape
	}
	return "environment: " + string(r)
}
func (e *ModeError) Unwrap() error { return e.cause }
func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
func text(s string) bool { return strings.TrimSpace(s) != "" }

var digestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
var commitRE = regexp.MustCompile(`^[0-9a-f]{40}$`)
var fleetIDRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var utcRE = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)

func sourceValue(source []byte) (cue.Value, any, error) {
	ctx := cuecontext.New()
	v := ctx.CompileBytes(source)
	if err := v.Validate(cue.Concrete(true)); err != nil {
		// Only actual conflicting concrete fields get source-specific attribution.
		// Incomplete/syntactically invalid sources use the root pointer.
		es := []*ValidationError{}
		for _, e := range cueerrors.Errors(err) {
			p := ""
			if strings.Contains(e.Error(), "conflicting values") {
				for _, s := range e.Path() {
					p = pointer(p, s)
				}
			}
			add(&es, p, ValidationShape)
		}
		if len(es) == 0 {
			add(&es, "", ValidationShape)
		}
		return v, nil, first(es)
	}
	b, err := v.MarshalJSON()
	if err != nil {
		return v, nil, &ValidationError{Reason: ValidationShape}
	}
	var data any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if d.Decode(&data) != nil {
		return v, nil, &ValidationError{Reason: ValidationShape}
	}
	return v, data, nil
}

func sourceSchema(v cue.Value, definition string, out any) error {
	def := v.Context().CompileString(schemaSource).LookupPath(cue.MakePath(cue.Def(definition)))
	value := def.Unify(v)
	if e := value.Validate(cue.Concrete(true)); e != nil {
		return &ValidationError{Reason: ValidationShape}
	}
	if e := value.Decode(out); e != nil {
		return &ValidationError{Reason: ValidationShape}
	}
	return nil
}

// shape walks the declared Go projection to attribute required/type failures
// independently of CUE's disjunction diagnostic order. It never normalizes data.
func shape(data any, t reflect.Type, p string, es *[]*ValidationError) {
	if t.Kind() == reflect.Pointer {
		if data == nil {
			if t.Elem().Kind() != reflect.Int64 {
				add(es, p, ValidationShape)
			}
			return
		}
		shape(data, t.Elem(), p, es)
		return
	}
	bad := func() { add(es, p, ValidationShape) }
	switch t.Kind() {
	case reflect.Struct:
		m, ok := data.(map[string]any)
		if !ok {
			bad()
			return
		}
		fields := map[string]reflect.StructField{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")
			fields[tag[0]] = f
			x, exists := m[tag[0]]
			q := pointer(p, tag[0])
			if !exists {
				if len(tag) < 2 {
					add(es, q, ValidationShape)
				}
				continue
			}
			shape(x, f.Type, q, es)
		}
		for k := range m {
			if _, ok := fields[k]; !ok {
				add(es, pointer(p, k), ValidationShape)
			}
		}
		if t == reflect.TypeFor[MetricExpectation]() {
			if metric, ok := m["metric"].(string); ok {
				if _, known := metricRules[Metric(metric)]; !known {
					add(es, pointer(p, "metric"), ValidationShape)
				}
			}
		}
		if t == reflect.TypeFor[ConceptCitation]() {
			_, a := m["card"]
			_, b := m["design_row"]
			if a == b {
				bad()
			}
		}
		if t == reflect.TypeFor[Anchor]() {
			a, aok := number(m["start_line"])
			b, bok := number(m["end_line"])
			if aok && bok && a >= 1 && a <= 2147483647 && b < a {
				add(es, pointer(p, "end_line"), ValidationShape)
			}
		}
		// These completeness checks are declaration shape, not evidence checks.
		if t == reflect.TypeFor[Scale]() {
			if xs, ok := m["excluded"].([]any); ok {
				seen := map[string]bool{}
				complete := true
				for _, x := range xs {
					z, ok := x.(map[string]any)
					if !ok {
						complete = false
						continue
					}
					k, ok := z["kind"].(string)
					if !ok || !contains(enumValues["ExclusionKind"], k) {
						complete = false
					}
					seen[k] = true
				}
				if complete {
					for _, k := range enumValues["ExclusionKind"] {
						if !seen[k] {
							add(es, pointer(p, "excluded"), ValidationShape)
							break
						}
					}
				}
			}
		}
		var ids []string
		switch t {
		case reflect.TypeFor[FleetFile]():
			ids = []string{"content_id"}
		case reflect.TypeFor[FleetRepository]():
			ids = []string{"id", "head"}
		case reflect.TypeFor[FleetCommit]():
			ids = []string{"id"}
		}
		for _, k := range ids {
			if v, ok := m[k].(string); ok && !fleetIDRE.MatchString(v) {
				add(es, pointer(p, k), ValidationShape)
			}
		}
		if t == reflect.TypeFor[FleetCommit]() {
			if ps, ok := m["parents"].([]any); ok {
				for i, v := range ps {
					if s, ok := v.(string); ok && !fleetIDRE.MatchString(s) {
						add(es, indexed(pointer(p, "parents"), i), ValidationShape)
					}
				}
			}
		}

	case reflect.Map:
		m, ok := data.(map[string]any)
		if !ok {
			bad()
			return
		}
		ks := keys(m)
		for _, k := range ks {
			q := pointer(p, k)
			if !text(k) {
				add(es, q, ValidationShape)
			}
			shape(m[k], t.Elem(), q, es)
		}
	case reflect.Slice:
		a, ok := data.([]any)
		if !ok {
			bad()
			return
		}
		min := 0
		switch t.Elem().Name() {
		case "RepositoryPin", "Pin", "ScaleClaim", "MetricExpectation", "ConceptCitation", "FleetCommit", "FleetRepository", "FleetFile":
			min = 1
		}
		if t.Elem() == reflect.TypeFor[Ref]() {
			switch path.Base(p) {
			case "oracles", "receipts", "requirements", "sources", "runs", "rulings":
				min = 1
			}
		}
		if strings.HasSuffix(p, "/groups") {
			min = 0
		}
		if strings.HasSuffix(p, "/concepts") && t.Elem().Kind() == reflect.String {
			min = 1
		}
		if strings.HasSuffix(p, "/images") {
			min = 0
		}
		if strings.HasSuffix(p, "/requirements") && t.Elem() == reflect.TypeFor[Requirement]() {
			min = 0
		}
		if strings.HasSuffix(p, "/files") && t.Elem() == reflect.TypeFor[FleetFile]() {
			min = 0
		}
		if t.Elem() == reflect.TypeFor[PreflightInput]() {
			min = 1
			if strings.HasPrefix(p, "/baseline/") {
				min = 2
			}
		}
		if t.Elem() == reflect.TypeFor[AgentOutcome]() {
			min = 1
			if strings.HasPrefix(p, "/baseline/") {
				min = 0
			}
		}
		if len(a) < min {
			bad()
		}
		if p == "/snapshots/commits" && len(a) != 21 {
			bad()
		}
		for i, x := range a {
			q := indexed(p, i)
			shape(x, t.Elem(), q, es)
			if t.Elem().Kind() == reflect.String {
				if s, ok := x.(string); ok {
					if (p == "/snapshots/commits" && !commitRE.MatchString(s)) || (strings.HasSuffix(p, "/existing_cell_keys") && !digestRE.MatchString(s)) {
						add(es, q, ValidationShape)
					}
				}
			}
		}
	case reflect.String:
		s, ok := data.(string)
		if !ok {
			bad()
			return
		}
		if vals, ok := enumValues[t.Name()]; ok {
			if !contains(vals, s) {
				bad()
			}
			return
		}
		member := path.Base(p)
		if member == "subtype" || member == "url" {
			if member == "subtype" && s != "" && !text(s) {
				bad()
			}
			return
		}
		if !text(s) {
			bad()
			return
		}
		switch member {
		case "sha256", "source_sha256", "profile_sha256", "binary_sha256", "cell_key":
			if !digestRE.MatchString(s) {
				bad()
			}
		case "commit", "product_commit":
			if !commitRE.MatchString(s) {
				bad()
			}
		case "measured_at", "claim_time":
			tm, e := time.Parse(time.RFC3339, s)
			if e != nil || !utcRE.MatchString(s) || tm.Location() != time.UTC {
				bad()
			}
		}

	case reflect.Int, reflect.Int64, reflect.Uint64:
		n, ok := data.(json.Number)
		if !ok {
			bad()
			return
		}
		valid := true
		if t.Kind() == reflect.Uint64 {
			_, e := strconv.ParseUint(string(n), 10, 64)
			valid = e == nil
		} else {
			v, e := strconv.ParseInt(string(n), 10, 64)
			valid = e == nil && v >= 0
			if t.Kind() == reflect.Int {
				valid = valid && v <= 2147483647
			}
			switch path.Base(p) {
			case "positive_minimum", "negative_minimum":
				valid = valid && v >= 30
			case "start_line", "end_line", "minimum", "wall_cap_ms", "max_usd_micros", "max_repositories", "max_commits", "max_files", "max_bytes":
				valid = valid && v > 0
			}
		}
		if !valid {
			bad()
		}
	case reflect.Bool:
		if _, ok := data.(bool); !ok {
			bad()
		}
	}
}
func number(x any) (int64, bool) {
	n, ok := x.(json.Number)
	if !ok {
		return 0, false
	}
	v, e := n.Int64()
	return v, e == nil
}
func keys[K ~string, V any](m map[K]V) []K {
	ks := make([]K, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i] < ks[j] })
	return ks
}
func typedShape(v any) error {
	b, _ := json.Marshal(v)
	var d any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if dec.Decode(&d) != nil {
		return &ValidationError{Reason: ValidationShape}
	}
	es := []*ValidationError{}
	shape(d, reflect.TypeOf(v), "", &es)
	return first(es)
}
func canonicalPath(s string, fleet bool) bool {
	if s == "" || strings.HasPrefix(s, "/") || path.Clean(s) != s || s == "." {
		return false
	}
	for _, c := range s {
		if c == '\\' || c == ':' || unicode.IsControl(c) {
			return false
		}
	}
	for _, part := range strings.Split(s, "/") {
		if part == ".." || part == "." || (fleet && strings.EqualFold(part, ".git")) {
			return false
		}
	}
	return true
}
func paths(v reflect.Value, p string, es *[]*ValidationError) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		paths(v.Elem(), p, es)
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.PkgPath != "" {
				continue
			}
			k := strings.Split(f.Tag.Get("json"), ",")[0]
			q := pointer(p, k)
			if k == "path" && v.Field(i).Kind() == reflect.String {
				if !canonicalPath(v.Field(i).String(), false) {
					add(es, q, ValidationPath)
				}
			} else {
				paths(v.Field(i), q, es)
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			paths(v.Index(i), indexed(p, i), es)
		}
	case reflect.Map:
		for _, k := range keysValue(v) {
			paths(v.MapIndex(reflect.ValueOf(k)), pointer(p, k), es)
		}
	}
}
func keysValue(v reflect.Value) []string {
	ks := []string{}
	for _, k := range v.MapKeys() {
		ks = append(ks, k.String())
	}
	sort.Strings(ks)
	return ks
}
func emptyRecords() EnvironmentRecords {
	return EnvironmentRecords{Environments: []EnvironmentHead{}, Requirements: []Requirement{}, Witnesses: []Witness{}, Coverage: []CoverageClaim{}}
}
func emptyInventory() ConceptInventory {
	return ConceptInventory{Concepts: map[string]ConceptRecord{}, Groups: []ConceptGroup{}}
}
func emptyFleet() FleetManifest {
	return FleetManifest{Repositories: []FleetRepository{}, Truth: []FleetFile{}}
}

// ValidateEnvironmentRecords checks shape and in-bundle consistency. External
// evidence bytes and cross-file record ownership remain the caller's responsibility.
func ValidateEnvironmentRecords(source []byte) (EnvironmentRecords, error) {
	v, d, e := sourceValue(source)
	if e != nil {
		return emptyRecords(), e
	}
	es := []*ValidationError{}
	shape(d, reflect.TypeFor[EnvironmentRecords](), "", &es)
	if e = first(es); e != nil {
		return emptyRecords(), e
	}
	var r EnvironmentRecords
	if e = sourceSchema(v, "EnvironmentRecords", &r); e != nil {
		return emptyRecords(), e
	}
	if e = recordsValue(r); e != nil {
		return emptyRecords(), e
	}
	return r, nil
}
func duplicate(es *[]*ValidationError, seen map[string]bool, s, p string) {
	if seen[s] {
		add(es, p, ValidationIdentity)
	}
	seen[s] = true
}
func publicURL(s string, t Tier) bool {
	if t == TierConstructed {
		return s == ""
	}
	if t == TierSmoke && s == "" {
		return true
	}
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.Contains(s, "#") && !strings.ContainsAny(s, "\x00\r\n")
}
func recordsValue(r EnvironmentRecords) error {
	if e := typedShape(r); e != nil {
		return e
	}
	es := []*ValidationError{}
	paths(reflect.ValueOf(r), "", &es)
	envs := map[string]EnvironmentHead{}
	reqs := map[string]Requirement{}
	badStrata := map[string]bool{}
	for i, h := range r.Environments {
		p := indexed("/environments", i)
		if _, ok := envs[h.ID]; ok {
			add(&es, p+"/id", ValidationIdentity)
		} else {
			envs[h.ID] = h
		}
		seen := map[string]bool{}
		for j, rp := range h.Repositories {
			q := indexed(p+"/repositories", j)
			duplicate(&es, seen, rp.ID, q+"/id")
			if !publicURL(rp.URL, h.Tier) {
				add(&es, q+"/url", ValidationPath)
			}
		}
		seen = map[string]bool{}
		for j, x := range h.Scale.Excluded {
			duplicate(&es, seen, string(x.Kind), indexed(p+"/scale/excluded", j)+"/kind")
		}
		if len(h.PriorReads) > 0 && h.Split == SplitHeldOut {
			add(&es, p+"/prior_reads", ValidationReference)
		}
	}
	checkStratum := func(s Stratum, p string) {
		seen := map[string]bool{}
		for j, c := range s.Scale {
			duplicate(&es, seen, string(c.Axis), indexed(p+"/scale", j)+"/axis")
		}
	}
	for i, q := range r.Requirements {
		p := indexed("/requirements", i)
		if _, ok := reqs[q.ID]; ok {
			add(&es, p+"/id", ValidationIdentity)
		} else {
			reqs[q.ID] = q
		}
		before := len(es)
		checkStratum(q.Stratum, p+"/stratum")
		badStrata[q.ID] = len(es) > before
		if q.Card.ID != q.Capability {
			add(&es, p+"/card/id", ValidationReference)
		}
	}
	seen := map[string]bool{}
	for i, w := range r.Witnesses {
		p := indexed("/witnesses", i)
		duplicate(&es, seen, w.ID, p+"/id")
		h, ok := envs[w.Environment.ID]
		if !ok {
			add(&es, p+"/environment/id", ValidationReference)
		} else {
			found := false
			for _, rp := range h.Repositories {
				if rp.ID == w.Anchor.Repository.ID {
					found = true
					compare(&es, p+"/anchor/repository", rp, w.Anchor.Repository)
				}
			}
			if !found {
				add(&es, p+"/anchor/repository/id", ValidationReference)
			}
		}
		q, ok := reqs[w.Requirement.ID]
		if !ok {
			add(&es, p+"/requirement/id", ValidationReference)
		} else {
			compare(&es, p+"/checker", q.Checker, w.Checker)
		}
	}
	seen = map[string]bool{}
	for i, c := range r.Coverage {
		p := indexed("/coverage", i)
		duplicate(&es, seen, c.ID, p+"/id")
		before := len(es)
		checkStratum(c.Stratum, p+"/stratum")
		invalidClaimStratum := len(es) > before
		if _, ok := envs[c.Environment.ID]; !ok {
			add(&es, p+"/environment/id", ValidationReference)
		}
		rs := map[string]bool{}
		for j, ref := range c.Requirements {
			rp := indexed(p+"/requirements", j)
			duplicate(&es, rs, ref.ID, rp+"/id")
			q, ok := reqs[ref.ID]
			if !ok {
				add(&es, rp+"/id", ValidationReference)
				continue
			}
			if q.Capability != c.Capability {
				add(&es, p+"/capability", ValidationReference)
			}
			if !badStrata[q.ID] && !invalidClaimStratum {
				compare(&es, p+"/stratum", q.Stratum, c.Stratum)
			}
		}
	}
	return first(es)
}

// compare reports changed leaf members and retains source list indices.
func compare(es *[]*ValidationError, p string, want, got any) {
	var a, b any
	ba, _ := json.Marshal(want)
	bb, _ := json.Marshal(got)
	_ = json.Unmarshal(ba, &a)
	_ = json.Unmarshal(bb, &b)
	compareData(es, p, a, b)
}
func compareData(es *[]*ValidationError, p string, a, b any) {
	if reflect.DeepEqual(a, b) {
		return
	}
	switch x := a.(type) {
	case map[string]any:
		if y, ok := b.(map[string]any); ok {
			for _, k := range keys(x) {
				compareData(es, pointer(p, k), x[k], y[k])
			}
			return
		}
	case []any:
		if y, ok := b.([]any); ok && len(x) == len(y) {
			for i := range x {
				compareData(es, indexed(p, i), x[i], y[i])
			}
			return
		}
	}
	add(es, p, ValidationReference)
}

// metricRules is the canonical metric/unit/floor relation; the CUE schema projects it.
type metricRule struct {
	unit     MetricUnit
	floor    int64
	citation MinimumCitation
}

var metricRules = map[Metric]metricRule{
	"absent_symbols":        {"symbols", 1, "BE-D2:False plan claims"},
	"unwired_entries":       {"entry_points", 1, "BE-D2:False plan claims"},
	"cross_file_paths":      {"paths", 30, ""},
	"repository_links":      {"links", 30, ""},
	"linked_repositories":   {"repositories", 3, "BE-D2:Cross-repository dependencies"},
	"linked_services":       {"services", 3, "BE-D2:Cross-repository dependencies"},
	"fanin_symbols":         {"symbols", 3, "BE-D2:Fan-in and dispatch"},
	"dispatch_sites":        {"sites", 30, ""},
	"hazardous_changes":     {"changes", 30, ""},
	"safe_changes":          {"changes", 30, ""},
	"module_expressions":    {"expressions", 30, ""},
	"missing_context":       {"cases", 30, ""},
	"provider_transitions":  {"transitions", 30, ""},
	"stale_instructions":    {"instructions", 30, ""},
	"live_instructions":     {"instructions", 30, ""},
	"governing_links":       {"links", 30, ""},
	"public_fixes":          {"fixes", 30, ""},
	"collateral_mutants":    {"mutants", 30, ""},
	"nonlexical_issues":     {"issues", 30, ""},
	"lexical_issues":        {"issues", 30, ""},
	"conflicting_pairs":     {"pairs", 30, ""},
	"clean_pairs":           {"pairs", 30, ""},
	"history_commits":       {"commits", 200, ""},
	"relevant_changes":      {"changes", 30, ""},
	"boundary_sites":        {"sites", 30, ""},
	"language_sites":        {"sites", 30, ""},
	"data_reads":            {"reads", 30, ""},
	"subprocess_reads":      {"reads", 30, ""},
	"uncovered_changes":     {"changes", 30, ""},
	"acquire_release_pairs": {"pairs", 30, ""},
	"missing_release_fixes": {"fixes", 30, ""},
	"escapes":               {"escapes", 30, ""},
	"bounded_sites":         {"sites", 30, ""},
	"unbounded_sites":       {"sites", 30, ""},
	"shutdown_cases":        {"cases", 30, ""},
	"multisession_tasks":    {"tasks", 30, ""},
	"declared_deletions":    {"deletions", 30, ""},
	"undeclared_deletions":  {"deletions", 30, ""},
	"rule_labels":           {"labels", 30, ""},
	"pinned_inputs":         {"inputs", 30, ""},
	"floating_inputs":       {"inputs", 30, ""},
	"interface_changes":     {"changes", 30, ""},
	"valid_envelopes":       {"envelopes", 30, ""},
	"invalid_envelopes":     {"envelopes", 30, ""},
	"resolvable_bindings":   {"bindings", 30, ""},
	"ambiguous_bindings":    {"bindings", 30, ""},
	"output_cases":          {"outputs", 30, ""},
	"long_outputs":          {"outputs", 1, "BE-D2:Output fidelity and judgments"},
	"short_outputs":         {"outputs", 1, "BE-D2:Output fidelity and judgments"},
	"question_cases":        {"questions", 30, ""},
	"route_questions":       {"questions", 1, "BE-D2:Output fidelity and judgments"},
	"rank_questions":        {"questions", 1, "BE-D2:Output fidelity and judgments"},
	"table_questions":       {"questions", 1, "BE-D2:Output fidelity and judgments"},
}

func expectationShape(d any, defaults bool, es *[]*ValidationError) {
	m, ok := d.(map[string]any)
	if !ok {
		return
	}
	cs, ok := m["concepts"].(map[string]any)
	if !ok {
		return
	}
	for _, k := range keys(cs) {
		c, ok := cs[k].(map[string]any)
		if !ok {
			continue
		}
		xs, ok := c["expectations"].([]any)
		if !ok {
			continue
		}
		for j, x := range xs {
			e, ok := x.(map[string]any)
			if !ok {
				continue
			}
			if _, ok := e["minimum"]; !ok && defaults {
				e["minimum"] = json.Number("30")
			}
			p := indexed(pointer("/concepts", k)+"/expectations", j)
			metric, mok := e["metric"].(string)
			unit, uok := e["unit"].(string)
			minimum, nok := number(e["minimum"])
			rule, known := metricRules[Metric(metric)]
			if mok && !known {
				add(es, p+"/metric", ValidationShape)
			}
			// Individual shape errors are collected before this relation is considered.
			if !known || !uok || !contains(enumValues["MetricUnit"], unit) || !nok || minimum <= 0 {
				continue
			}
			if unit != string(rule.unit) {
				add(es, p+"/unit", ValidationShape)
				continue
			}
			if minimum < rule.floor {
				add(es, p+"/minimum", ValidationShape)
				continue
			}
			cite, exists := e["lower_minimum"]
			if minimum >= 30 {
				if exists {
					add(es, p+"/lower_minimum", ValidationShape)
				}
			} else if cite != string(rule.citation) || !exists {
				add(es, p+"/lower_minimum", ValidationShape)
			}
		}
	}
}
func inventoryShape(d any, defaults bool) error {
	// Apply the CUE minimum default to the parsed projection only. This does not
	// repair typed Go inputs, and the embedded CUE definition still validates it.
	if defaults {
		expectationShape(d, true, &[]*ValidationError{})
	}
	es := []*ValidationError{}
	shape(d, reflect.TypeFor[ConceptInventory](), "", &es)
	// MetricNone/positive/negative exist as diagnostic vocabulary, not expectations.
	if e := first(es); e != nil {
		return e
	}
	expectationShape(d, false, &es)
	return first(es)
}

// ValidateConceptInventory evaluates CUE field identity and validates the local
// inventory projection. A successful result makes no admission claim.
func ValidateConceptInventory(source []byte) (ConceptInventory, error) {
	_, d, e := sourceValue(source)
	if e != nil {
		return emptyInventory(), e
	}
	if e = inventoryShape(d, true); e != nil {
		return emptyInventory(), e
	}
	b, _ := json.Marshal(d)
	v := cuecontext.New().CompileBytes(b)
	var r ConceptInventory
	if e = sourceSchema(v, "ConceptInventory", &r); e != nil {
		return emptyInventory(), e
	}
	if e = ValidateConceptInventoryValue(r); e != nil {
		return emptyInventory(), e
	}
	return r, nil
}

// ValidateConceptInventoryValue checks exact supplied Go keys and references.
// It neither parses CUE nor supplies defaults or normalizes strings.
func ValidateConceptInventoryValue(r ConceptInventory) error {
	b, _ := json.Marshal(r)
	var d any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	_ = dec.Decode(&d)
	if e := inventoryShape(d, false); e != nil {
		return e
	}
	es := []*ValidationError{}
	paths(reflect.ValueOf(r), "", &es)
	refs := map[string]Ref{}
	pairs := map[[2]string]bool{}
	for i, g := range r.Groups {
		p := indexed("/groups", i)
		pair := [2]string{g.Card.ID, g.CheckerContract.ID}
		if pairs[pair] {
			add(&es, p+"/checker_contract/id", ValidationIdentity)
		}
		pairs[pair] = true
		if ref, ok := refs[g.Card.ID]; ok {
			if canonicalPath(ref.Path, false) {
				compare(&es, p+"/card", ref, g.Card)
			}
		} else {
			refs[g.Card.ID] = g.Card
		}
		seen := map[string]bool{}
		for j, k := range g.Concepts {
			q := indexed(p+"/concepts", j)
			duplicate(&es, seen, k, q)
			if _, ok := r.Concepts[k]; !ok {
				add(&es, q, ValidationReference)
			}
		}
	}
	for _, k := range keys(r.Concepts) {
		c := r.Concepts[k]
		p := pointer("/concepts", k)
		seen := map[string]bool{}
		for j, cite := range c.RequiredBy {
			q := indexed(p+"/required_by", j)
			if cite.Card != nil {
				duplicate(&es, seen, "card:"+cite.Card.ID, q+"/card/id")
				if ref, ok := refs[cite.Card.ID]; ok && canonicalPath(ref.Path, false) {
					compare(&es, q+"/card", ref, *cite.Card)
				}
			} else {
				duplicate(&es, seen, "design:"+string(cite.DesignRow), q+"/design_row")
			}
		}
		seen = map[string]bool{}
		for j, e := range c.Expectations {
			duplicate(&es, seen, string(e.Metric), indexed(p+"/expectations", j)+"/metric")
		}
	}
	return first(es)
}
func prefixed(e error, p string) error {
	if x, ok := e.(*ValidationError); ok {
		return &ValidationError{Path: p + x.Path, Reason: x.Reason}
	}
	return e
}

// ValidateEnvironmentConceptJoin checks declared mapping closure only. The caller
// must first resolve frozen Card bytes and run capability.CheckCardClosure.
func ValidateEnvironmentConceptJoin(r EnvironmentRecords, in ConceptInventory, cardIDs []string) error {
	if e := recordsValue(r); e != nil {
		return prefixed(e, "/records")
	}
	if e := ValidateConceptInventoryValue(in); e != nil {
		return prefixed(e, "/inventory")
	}
	es := []*ValidationError{}
	cards := map[string]bool{}
	for i, k := range cardIDs {
		p := indexed("/card_ids", i)
		if !text(k) {
			add(&es, p, ValidationShape)
		}
		duplicate(&es, cards, k, p)
	}
	if e := first(es); e != nil {
		return e
	}
	groups := map[[2]string]ConceptGroup{}
	cardRefs := map[string]Ref{}
	mapped := map[string]bool{}
	for i, g := range in.Groups {
		groups[[2]string{g.Card.ID, g.CheckerContract.ID}] = g
		cardRefs[g.Card.ID] = g.Card
		mapped[g.Card.ID] = true
		if !cards[g.Card.ID] {
			add(&es, indexed("/inventory/groups", i)+"/card/id", ValidationReference)
		}
	}
	for i, k := range cardIDs {
		if !mapped[k] {
			add(&es, indexed("/card_ids", i), ValidationReference)
		}
	}
	for _, k := range keys(in.Concepts) {
		for j, c := range in.Concepts[k].RequiredBy {
			if c.Card != nil {
				p := indexed(pointer("/inventory/concepts", k)+"/required_by", j) + "/card"
				if !cards[c.Card.ID] {
					add(&es, p+"/id", ValidationReference)
				} else if ref, ok := cardRefs[c.Card.ID]; ok {
					compare(&es, p, ref, *c.Card)
				}
			}
		}
	}
	reqs := map[string]Requirement{}
	badReq := map[string]bool{}
	for i, q := range r.Requirements {
		p := indexed("/records/requirements", i)
		before := len(es)
		reqs[q.ID] = q
		if !cards[q.Capability] {
			add(&es, p+"/capability", ValidationReference)
		} else {
			if ref, ok := cardRefs[q.Capability]; ok {
				compare(&es, p+"/card", ref, q.Card)
			}
			g, ok := groups[[2]string{q.Capability, q.CheckerContract.ID}]
			if !ok {
				add(&es, p+"/checker_contract/id", ValidationReference)
			} else {
				compare(&es, p+"/checker_contract", g.CheckerContract, q.CheckerContract)
				if !contains(g.Concepts, q.Concept) {
					add(&es, p+"/concept", ValidationReference)
				}
			}
		}
		if c, ok := in.Concepts[q.Concept]; !ok {
			add(&es, p+"/concept", ValidationReference)
		} else if q.Stratum.Subtype != "" {
			for j, e := range c.Expectations {
				if metricRules[e.Metric].citation == MinimumOutput && e.Minimum < 30 {
					add(&es, indexed(pointer("/inventory/concepts", q.Concept)+"/expectations", j)+"/minimum", ValidationReference)
				}
			}
		}
		badReq[q.ID] = len(es) > before
	}
	for i, c := range r.Coverage {
		p := indexed("/records/coverage", i)
		if !cards[c.Capability] {
			add(&es, p+"/capability", ValidationReference)
		}
		unavailable := false
		for _, ref := range c.Requirements {
			if badReq[ref.ID] {
				unavailable = true
			}
		}
		if unavailable {
			continue
		}
		seen := map[[2]string]bool{}
		for j, ref := range c.Requirements {
			q := reqs[ref.ID]
			key := [2]string{q.CheckerContract.ID, q.Concept}
			if seen[key] {
				add(&es, indexed(p+"/requirements", j)+"/id", ValidationIdentity)
			}
			seen[key] = true
		}
		for _, g := range in.Groups {
			if g.Card.ID == c.Capability {
				for _, k := range g.Concepts {
					if !seen[[2]string{g.CheckerContract.ID, k}] {
						add(&es, p+"/requirements", ValidationReference)
					}
				}
			}
		}
	}
	return first(es)
}

// ValidateFleetManifest validates and deterministically orders a declared fleet
// DAG. It never materializes files, resolves sources or measures their sizes.
func ValidateFleetManifest(source []byte) (FleetManifest, error) {
	v, d, e := sourceValue(source)
	if e != nil {
		return emptyFleet(), fleetError(e)
	}
	es := []*ValidationError{}
	shape(d, reflect.TypeFor[FleetManifest](), "", &es)
	if e = first(es); e != nil {
		return emptyFleet(), fleetError(e)
	}
	var r FleetManifest
	if e = sourceSchema(v, "FleetManifest", &r); e != nil {
		return emptyFleet(), fleetError(e)
	}
	if e = fleetValue(&r); e != nil {
		return emptyFleet(), e
	}
	return r, nil
}
func fleetError(e error) error {
	x := e.(*ValidationError)
	return &FleetError{Path: x.Path, Reason: FleetShape}
}
func fleetValue(r *FleetManifest) error {
	es := []*FleetError{}
	addF := func(p string, r FleetReason) { es = append(es, &FleetError{Path: p, Reason: r}) }
	id := func(s, p string) {
		if !fleetIDRE.MatchString(s) {
			addF(p, FleetShape)
		}
	}
	fileChecks := func(fs []FleetFile, p string) {
		seen := map[string]bool{}
		for i, f := range fs {
			q := indexed(p, i)
			id(f.ContentID, q+"/content_id")
			if !canonicalPath(f.Path, true) {
				addF(q+"/path", FleetPath)
			}
			if seen[f.Path] {
				addF(q+"/path", FleetDuplicate)
			}
			seen[f.Path] = true
			for j := 0; j < i; j++ {
				other := fs[j].Path
				if strings.HasPrefix(f.Path, other+"/") || strings.HasPrefix(other, f.Path+"/") {
					addF(q+"/path", FleetPath)
				}
			}
		}
	}
	repositories := map[string]bool{}
	for i, repo := range r.Repositories {
		p := indexed("/repositories", i)
		id(repo.ID, p+"/id")
		id(repo.Head, p+"/head")
		if repositories[repo.ID] {
			addF(p+"/id", FleetDuplicate)
		}
		repositories[repo.ID] = true
		commits := map[string]FleetCommit{}
		for j, c := range repo.Commits {
			q := indexed(p+"/commits", j)
			id(c.ID, q+"/id")
			if _, ok := commits[c.ID]; ok {
				addF(q+"/id", FleetDuplicate)
			} else {
				commits[c.ID] = c
			}
			fileChecks(c.Files, q+"/files")
		}
		for j, c := range repo.Commits {
			q := indexed(p+"/commits", j)
			seen := map[string]bool{}
			for k, parent := range c.Parents {
				pp := indexed(q+"/parents", k)
				id(parent, pp)
				if seen[parent] {
					addF(pp, FleetDuplicate)
				}
				seen[parent] = true
				if _, ok := commits[parent]; !ok {
					addF(pp, FleetParent)
				}
			}
		}
		// Reachability for each source edge identifies cycles without depending on
		// map iteration or reordering the input diagnostics.
		reaches := func(start, target string) bool {
			seen := map[string]bool{}
			stack := []string{start}
			for len(stack) > 0 {
				x := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if x == target {
					return true
				}
				if seen[x] {
					continue
				}
				seen[x] = true
				stack = append(stack, commits[x].Parents...)
			}
			return false
		}
		for j, c := range repo.Commits {
			for k, parent := range c.Parents {
				if reaches(parent, c.ID) {
					addF(indexed(indexed(p+"/commits", j)+"/parents", k), FleetCycle)
				}
			}
		}
		if _, ok := commits[repo.Head]; !ok {
			addF(p+"/head", FleetParent)
		} else {
			for _, c := range repo.Commits {
				if !reaches(repo.Head, c.ID) {
					addF(p+"/head", FleetParent)
				}
			}
		}
	}
	fileChecks(r.Truth, "/truth")
	if len(es) > 0 {
		sort.Slice(es, func(i, j int) bool {
			if es[i].Path != es[j].Path {
				return es[i].Path < es[j].Path
			}
			return es[i].Reason < es[j].Reason
		})
		return es[0]
	}
	sort.Slice(r.Repositories, func(i, j int) bool { return r.Repositories[i].ID < r.Repositories[j].ID })
	for i := range r.Repositories {
		repo := &r.Repositories[i]
		pending := map[string]FleetCommit{}
		for _, c := range repo.Commits {
			sort.Slice(c.Files, func(i, j int) bool { return c.Files[i].Path < c.Files[j].Path })
			pending[c.ID] = c
		}
		out := make([]FleetCommit, 0, len(pending))
		done := map[string]bool{}
		for len(pending) > 0 {
			for _, k := range keys(pending) {
				c := pending[k]
				ready := true
				for _, parent := range c.Parents {
					if !done[parent] {
						ready = false
					}
				}
				if ready {
					out = append(out, c)
					done[k] = true
					delete(pending, k)
					break
				}
			}
		}
		repo.Commits = out
	}
	sort.Slice(r.Truth, func(i, j int) bool { return r.Truth[i].Path < r.Truth[j].Path })
	return nil
}

var modePayloads = map[EnvironmentMode]string{
	ModeFixtures: "fixtures", ModePortfolio: "portfolio", ModeBaseline: "baseline", ModeAudit: "audit", ModeSeed: "seed", ModeCandidate: "candidate", ModeSeal: "seal", ModeReading: "reading", ModeSnapshots: "snapshots", ModeModelRerun: "model_rerun", ModeLandedReplay: "landed_replay",
}

func duplicateMode(source []byte) bool {
	f, e := parser.ParseFile("", source)
	if e != nil {
		return false
	}
	count := 0
	var inspect func([]ast.Decl)
	inspect = func(ds []ast.Decl) {
		for _, d := range ds {
			switch x := d.(type) {
			case *ast.Field:
				label, _, e := ast.LabelName(x.Label)
				if e == nil && label == "mode" {
					count++
				}
			case *ast.EmbedDecl:
				if st, ok := x.Expr.(*ast.StructLit); ok {
					inspect(st.Elts)
				}
			}
		}
	}
	inspect(f.Decls)
	return count > 1
}

// ValidateEnvironmentRequest checks the closed input union and local consistency
// without running a mode or resolving external evidence.
func ValidateEnvironmentRequest(source []byte) (EnvironmentRequest, error) {
	refuse := func(p string, r ModeReason) (EnvironmentRequest, error) {
		return EnvironmentRequest{}, NewModeError(p, r, nil)
	}
	if duplicateMode(source) {
		return refuse("/mode", ModeDuplicate)
	}
	v, d, e := sourceValue(source)
	if e != nil {
		return refuse(e.(*ValidationError).Path, ModeShape)
	}
	m, ok := d.(map[string]any)
	if !ok {
		return refuse("", ModeShape)
	}
	mode, ok := m["mode"].(string)
	if !ok {
		return refuse("/mode", ModeShape)
	}
	payload, ok := modePayloads[EnvironmentMode(mode)]
	if !ok {
		return refuse("/mode", ModeUnknownName)
	}
	if x, ok := m[payload]; !ok || x == nil {
		return refuse(pointer("", payload), ModeShape)
	}
	es := []*ValidationError{}
	for k := range m {
		if k != "mode" && k != payload {
			add(&es, pointer("", k), ValidationShape)
		}
	}
	shape(d, reflect.TypeFor[EnvironmentRequest](), "", &es)
	if e = first(es); e != nil {
		return refuse(e.(*ValidationError).Path, ModeShape)
	}
	var r EnvironmentRequest
	if e = sourceSchema(v, "EnvironmentRequest", &r); e != nil {
		return refuse(e.(*ValidationError).Path, ModeShape)
	}
	paths(reflect.ValueOf(r), "", &es)
	// Nested records and fleet declarations retain their local consistency laws.
	checkRecords := func(records EnvironmentRecords, p string) {
		if e := recordsValue(records); e != nil {
			x := e.(*ValidationError)
			add(&es, p+x.Path, ValidationShape)
		}
	}
	if r.Fixtures != nil {
		checkRecords(r.Fixtures.Records, "/fixtures/records")
	}
	if r.Portfolio != nil {
		checkRecords(r.Portfolio.Records, "/portfolio/records")
	}
	if r.Candidate != nil {
		checkRecords(r.Candidate.Records, "/candidate/records")
	}
	if r.Seed != nil {
		if e := fleetValue(&r.Seed.Manifest); e != nil {
			add(&es, "/seed/manifest"+e.(*FleetError).Path, ValidationShape)
		}
		for i, q := range r.Seed.Requirements {
			if q.Card.ID != q.Capability {
				add(&es, indexed("/seed/requirements", i)+"/card/id", ValidationShape)
			}
		}
	}
	if r.Snapshots != nil {
		seen := map[string]bool{}
		for i, c := range r.Snapshots.Commits {
			duplicate(&es, seen, c, indexed("/snapshots/commits", i))
		}
	}
	if e = first(es); e != nil {
		return refuse(e.(*ValidationError).Path, ModeShape)
	}
	return r, nil
}
