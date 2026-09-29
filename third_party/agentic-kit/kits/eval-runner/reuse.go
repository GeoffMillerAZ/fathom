package evalrunner

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	recordfingerprint "github.com/geoffmilleraz/agentic-kit/foundations/evidence/record-fingerprint"
)

const (
	maxBenchmarkReuseUnits  = maxBenchmarkTrials
	maxBenchmarkReuseRefs   = maxBenchmarkComponents * 2
	maxBenchmarkReuseStates = maxBenchmarkComponents * 2
)

// BenchmarkReuseAction is one non-executing action proposed for a unit. More
// than one action may be appropriate: a changed grader and price table must
// remain visible as separate actions.
type BenchmarkReuseAction string

const (
	// BenchmarkReuseExactDeterministic reuses a retained deterministic output.
	BenchmarkReuseExactDeterministic BenchmarkReuseAction = "exact_deterministic_reuse"
	// BenchmarkReuseGrade reuses a retained grade with unchanged grading inputs.
	BenchmarkReuseGrade BenchmarkReuseAction = "reuse_grade"
	// BenchmarkReuseRegrade grades a retained output under changed grading inputs.
	BenchmarkReuseRegrade BenchmarkReuseAction = "regrade_retained_output"
	// BenchmarkReuseReprice recalculates price from complete retained usage.
	BenchmarkReuseReprice BenchmarkReuseAction = "recalculate_price_from_complete_usage"
	// BenchmarkReuseRerun reruns a unit whose relevant dependency cannot be reused.
	BenchmarkReuseRerun BenchmarkReuseAction = "rerun_affected_tests"
	// BenchmarkReuseFreshLive collects a new live measurement. It never turns a
	// historical trial into another independent sample.
	BenchmarkReuseFreshLive BenchmarkReuseAction = "fresh_live_measurement"
)

// BenchmarkReuseReason is an inspectable rule result, not an execution fact.
type BenchmarkReuseReason struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// BenchmarkReuseReceipt identifies a retained, versioned caller attestation.
// The planner checks that a reference was supplied; it does not read bytes or
// authenticate the attestation.
type BenchmarkReuseReceipt struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// BenchmarkReuseDependencyClass states how the material component is retained.
// Unknown classifications never qualify for reuse.
type BenchmarkReuseDependencyClass string

const (
	// BenchmarkReuseLocalRetained identifies locally retained component bytes.
	BenchmarkReuseLocalRetained BenchmarkReuseDependencyClass = "local_retained"
	// BenchmarkReuseExternalObserved requires an observed remote identity window.
	BenchmarkReuseExternalObserved BenchmarkReuseDependencyClass = "external_observed"
)

// BenchmarkReuseDependencyBinding binds one manifest component to its
// retention class. Local components carry a retained byte identity and its
// caller-attested verification receipt; external components require a distinct
// observed-identity record for the exact source and target values.
type BenchmarkReuseDependencyBinding struct {
	Ref                 string                        `json:"ref"`
	Class               BenchmarkReuseDependencyClass `json:"class"`
	ArtifactDigest      string                        `json:"artifact_digest"`
	VerificationReceipt BenchmarkReuseReceipt         `json:"verification_receipt"`
}

// BenchmarkReuseUnit binds a named workload/test unit to its input and the
// manifest dependencies material to that unit. A source InputDigest must equal
// the bound BenchmarkTrial.TrialSpecDigest; TaskID and ReplicateID identify it.
//
// This first slice is intentionally full-manifest conservative: it does not
// certify a dependency-selective rerun. BenchmarkReuseRerun is a unit-level
// recovery proposal, not a claim that only an inferred dependency subset ran.
type BenchmarkReuseUnit struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	ReplicateID string `json:"replicate_id"`
	InputDigest string `json:"input_digest"`
	// Bindings must cover the complete manifest. This first slice deliberately
	// does not accept a generic closure receipt as permission to omit a ref.
	Bindings     []BenchmarkReuseDependencyBinding `json:"bindings"`
	UsageReceipt *BenchmarkReuseReceipt            `json:"usage_receipt"`
}

// BenchmarkReuseArtifact is a caller-attested retained artifact. Its receipt
// is a reference to the consumer's verification record, not verification by
// this package.
type BenchmarkReuseArtifact struct {
	Digest              string                `json:"digest"`
	VerificationReceipt BenchmarkReuseReceipt `json:"verification_receipt"`
}

// BenchmarkReuseExternalState records the caller-observed identity and
// validity and coverage of a remote/alias dependency. An absent, future, or
// expired state forces a fresh live assessment for affected units.
type BenchmarkReuseExternalState struct {
	ID              string                `json:"id"`
	Ref             string                `json:"ref"`
	SourceDigest    string                `json:"source_digest"`
	TargetDigest    string                `json:"target_digest"`
	IdentityReceipt BenchmarkReuseReceipt `json:"identity_receipt"`
	CoverageReceipt BenchmarkReuseReceipt `json:"coverage_receipt"`
	ObservedAt      time.Time             `json:"observed_at"`
	ValidUntil      time.Time             `json:"valid_until"`
}

// BenchmarkReuseRequest describes an assessment only. AsOf is injected by
// the caller; PlanBenchmarkReuse never reads a clock, network, or store.
type BenchmarkReuseRequest struct {
	Source                   BenchmarkEvidence             `json:"source"`
	SourceUnits              []BenchmarkReuseUnit          `json:"source_units"`
	TargetManifest           BenchmarkManifest             `json:"target_manifest"`
	TargetUnits              []BenchmarkReuseUnit          `json:"target_units"`
	SourceDeterminismReceipt *BenchmarkReuseReceipt        `json:"source_determinism_receipt"`
	TargetDeterminismReceipt *BenchmarkReuseReceipt        `json:"target_determinism_receipt"`
	TargetMode               BenchmarkMode                 `json:"target_mode"`
	Artifacts                []BenchmarkReuseArtifact      `json:"artifacts"`
	ExternalStates           []BenchmarkReuseExternalState `json:"external_states"`
	AsOf                     time.Time                     `json:"as_of"`
}

// BenchmarkReuseUnitPlan is the proposed action set for one target unit.
type BenchmarkReuseUnitPlan struct {
	ID                     string                 `json:"id"`
	Actions                []BenchmarkReuseAction `json:"actions"`
	Reasons                []BenchmarkReuseReason `json:"reasons"`
	IndependentTrialsAdded int                    `json:"independent_trials_added"`
}

// BenchmarkReusePlan is an inspectable proposal. ExecutionOccurred is always
// false and IndependentTrialsAdded is always zero: callers must retain a new
// execution receipt before claiming a new observation.
type BenchmarkReusePlan struct {
	EvidenceTrust          string                   `json:"evidence_trust"`
	RequestFingerprint     string                   `json:"request_fingerprint"`
	SourceFingerprint      string                   `json:"source_fingerprint"`
	TargetFingerprint      string                   `json:"target_fingerprint"`
	PlanFingerprint        string                   `json:"plan_fingerprint"`
	AsOf                   time.Time                `json:"as_of"`
	ExecutionOccurred      bool                     `json:"execution_occurred"`
	IndependentTrialsAdded int                      `json:"independent_trials_added"`
	Units                  []BenchmarkReuseUnitPlan `json:"units"`
}

// PlanBenchmarkReuse compares caller-declared retained evidence with a target
// configuration and returns a deterministic, bounded proposal. It neither
// executes an action nor treats hashes as proof of bytes, provenance, or
// external freshness.
func PlanBenchmarkReuse(request BenchmarkReuseRequest) (BenchmarkReusePlan, error) {
	if err := validateBenchmarkReuseRequest(request); err != nil {
		return BenchmarkReusePlan{}, err
	}
	sourceFingerprint, err := request.Source.Fingerprint()
	if err != nil {
		return BenchmarkReusePlan{}, benchmarkInvalid("source fingerprint: %v", err)
	}
	targetFingerprint, err := benchmarkReuseTargetFingerprint(request)
	if err != nil {
		return BenchmarkReusePlan{}, err
	}
	requestFingerprint, err := benchmarkReuseRequestFingerprint(request, sourceFingerprint)
	if err != nil {
		return BenchmarkReusePlan{}, err
	}
	sourceUnits := make(map[string]BenchmarkReuseUnit, len(request.SourceUnits))
	for _, unit := range request.SourceUnits {
		sourceUnits[unit.ID] = unit
	}
	trials := make(map[benchmarkTrialPair]BenchmarkTrial, len(request.Source.Trials))
	for _, trial := range request.Source.Trials {
		trials[benchmarkTrialKey(trial)] = trial
	}
	artifacts := make(map[string]BenchmarkReuseArtifact, len(request.Artifacts))
	for _, artifact := range request.Artifacts {
		artifacts[artifact.Digest] = artifact
	}
	plan := BenchmarkReusePlan{
		EvidenceTrust: BenchmarkEvidenceTrustCallerDeclared, RequestFingerprint: requestFingerprint, SourceFingerprint: sourceFingerprint,
		TargetFingerprint: targetFingerprint, AsOf: request.AsOf, ExecutionOccurred: false,
		IndependentTrialsAdded: 0, Units: make([]BenchmarkReuseUnitPlan, 0, len(request.TargetUnits)),
	}
	for _, target := range canonicalBenchmarkReuseUnits(request.TargetUnits) {
		unitPlan := benchmarkReuseUnitPlan(request, target, sourceUnits[target.ID], trials, artifacts)
		plan.Units = append(plan.Units, unitPlan)
	}
	planFingerprint, err := benchmarkReusePlanFingerprint(plan)
	if err != nil {
		return BenchmarkReusePlan{}, err
	}
	plan.PlanFingerprint = planFingerprint
	return plan, nil
}

func benchmarkReuseUnitPlan(request BenchmarkReuseRequest, target, source BenchmarkReuseUnit, trials map[benchmarkTrialPair]BenchmarkTrial, artifacts map[string]BenchmarkReuseArtifact) BenchmarkReuseUnitPlan {
	plan := BenchmarkReuseUnitPlan{ID: target.ID, Actions: make([]BenchmarkReuseAction, 0, 4), Reasons: make([]BenchmarkReuseReason, 0, 8)}
	add := func(action BenchmarkReuseAction) {
		if !containsReuseAction(plan.Actions, action) {
			plan.Actions = append(plan.Actions, action)
		}
	}
	reason := func(code, detail string) {
		plan.Reasons = append(plan.Reasons, BenchmarkReuseReason{Code: code, Detail: detail})
	}
	if request.TargetMode == BenchmarkModeLive {
		add(BenchmarkReuseFreshLive)
		reason("live_requires_new_observation", "historical evidence is not a new independent live sample or refreshed validity")
	}
	trial, sourceFound := trials[benchmarkTrialPair{Task: source.TaskID, Replicate: source.ReplicateID}]
	if source.ID == "" || !sourceFound {
		add(BenchmarkReuseRerun)
		reason("source_unit_missing", "no retained source unit and terminal trial match the target unit")
		return benchmarkReuseFinalizeUnit(plan)
	}
	if target.InputDigest != source.InputDigest || target.TaskID != source.TaskID || target.ReplicateID != source.ReplicateID {
		add(BenchmarkReuseRerun)
		reason("input_changed", "input or task/replicate identity differs from retained evidence")
		return benchmarkReuseFinalizeUnit(plan)
	}
	if !benchmarkReuseUnitCoverageKnown(request.Source.Manifest, source, artifacts) || !benchmarkReuseUnitCoverageKnown(request.TargetManifest, target, artifacts) || !benchmarkReuseSameBindings(source.Bindings, target.Bindings) {
		add(BenchmarkReuseRerun)
		reason("dependency_coverage_unknown", "retained source unit does not name every target dependency")
		return benchmarkReuseFinalizeUnit(plan)
	}
	refs := benchmarkReuseBindingRefs(target.Bindings)
	_, unknown, generationChanged, graderChanged, priceChanged := benchmarkReuseChanges(request.Source.Manifest, request.TargetManifest, refs)
	externalInvalid := benchmarkReuseExternalInvalid(request, target.Bindings, artifacts)
	if unknown {
		add(BenchmarkReuseRerun)
		reason("dependency_unknown", "a named dependency is absent or unrecognized in source or target manifest")
		if externalInvalid {
			add(BenchmarkReuseFreshLive)
			reason("external_state_not_fresh", "a relevant alias or remote state is unobserved, future-dated, or expired")
		}
		return benchmarkReuseFinalizeUnit(plan)
	}
	if generationChanged {
		add(BenchmarkReuseRerun)
		reason("generation_dependency_changed", "a relevant solution or non-reusable evaluation dependency changed")
	}
	if externalInvalid {
		add(BenchmarkReuseFreshLive)
		reason("external_state_not_fresh", "a relevant alias or remote state is unobserved, future-dated, or expired")
	}
	outputAvailable := trial.OutputDigest != nil && benchmarkReuseArtifactAvailable(artifacts, *trial.OutputDigest)
	gradeAvailable := outputAvailable && trial.AcceptanceDigest != nil && benchmarkReuseArtifactAvailable(artifacts, *trial.AcceptanceDigest)
	// Regrade and reprice are bounded calculations over verified historical source
	// output or usage. They neither execute the target nor establish its current
	// validity, so they may accompany a required rerun or fresh-live measurement.
	if graderChanged {
		if outputAvailable {
			add(BenchmarkReuseRegrade)
			reason("grader_changed", "verified historical source output can be graded under target grading inputs; it does not reuse target execution or validity")
		} else {
			add(BenchmarkReuseRerun)
			reason("output_unavailable", "changed grading inputs require retained verified output bytes")
		}
	} else if !generationChanged && request.TargetMode != BenchmarkModeLive && gradeAvailable {
		add(BenchmarkReuseGrade)
		reason("grade_reusable", "retained grade and its grading dependencies match")
	} else if !generationChanged && request.TargetMode != BenchmarkModeLive {
		add(BenchmarkReuseRerun)
		reason("grade_unavailable", "retained grade bytes are absent or not integrity-attested")
	}
	if priceChanged {
		if trial.CostBasis == BenchmarkCostUsage && trial.CostComplete && benchmarkReuseReceiptRetained(artifacts, source.UsageReceipt) {
			add(BenchmarkReuseReprice)
			reason("pricing_changed", "complete verified historical source usage can be recalculated under target pricing; it does not refresh target execution or validity")
		} else {
			add(BenchmarkReuseRerun)
			reason("usage_unavailable", "changed pricing requires complete retained usage before recalculation")
		}
	}
	if request.TargetMode != BenchmarkModeLive && !generationChanged && !graderChanged && !priceChanged && !externalInvalid && benchmarkReuseAllLocal(target.Bindings) && benchmarkReuseReceiptRetained(artifacts, request.SourceDeterminismReceipt) && benchmarkReuseReceiptRetained(artifacts, request.TargetDeterminismReceipt) && outputAvailable {
		add(BenchmarkReuseExactDeterministic)
		reason("deterministic_exact", "input, relevant dependencies, and verified retained output match")
	}
	return benchmarkReuseFinalizeUnit(plan)
}

func benchmarkReuseFinalizeUnit(plan BenchmarkReuseUnitPlan) BenchmarkReuseUnitPlan {
	order := map[BenchmarkReuseAction]int{BenchmarkReuseExactDeterministic: 0, BenchmarkReuseGrade: 1, BenchmarkReuseRegrade: 2, BenchmarkReuseReprice: 3, BenchmarkReuseRerun: 4, BenchmarkReuseFreshLive: 5}
	sort.Slice(plan.Actions, func(i, j int) bool { return order[plan.Actions[i]] < order[plan.Actions[j]] })
	return plan
}

func benchmarkReuseChanges(source, target BenchmarkManifest, refs []string) (changed, unknown, generationChanged, graderChanged, priceChanged bool) {
	for _, ref := range refs {
		left, leftOK := benchmarkReuseDependency(source, ref)
		right, rightOK := benchmarkReuseDependency(target, ref)
		if !leftOK || !rightOK {
			unknown = true
			continue
		}
		if left == right {
			continue
		}
		changed = true
		if ref == "evaluation.grader" || ref == "evaluation.acceptance" {
			graderChanged = true
		} else if ref == "evaluation.pricing" {
			priceChanged = true
		} else {
			generationChanged = true
		}
	}
	return
}

func benchmarkReuseDependency(manifest BenchmarkManifest, ref string) (string, bool) {
	if name, ok := strings.CutPrefix(ref, "solution."); ok {
		value, found := manifest.Solution[name]
		return value, found
	}
	if name, ok := strings.CutPrefix(ref, "evaluation."); ok {
		value, found := manifest.Evaluation[name]
		return value, found
	}
	return "", false
}

func benchmarkReuseExternalInvalid(request BenchmarkReuseRequest, bindings []BenchmarkReuseDependencyBinding, artifacts map[string]BenchmarkReuseArtifact) bool {
	for _, binding := range bindings {
		if binding.Class == BenchmarkReuseLocalRetained {
			continue
		}
		if binding.Class != BenchmarkReuseExternalObserved {
			return true
		}
		source, sourceOK := benchmarkReuseDependency(request.Source.Manifest, binding.Ref)
		target, targetOK := benchmarkReuseDependency(request.TargetManifest, binding.Ref)
		if !sourceOK || !targetOK {
			return true
		}
		found := false
		for _, state := range request.ExternalStates {
			if state.Ref != binding.Ref {
				continue
			}
			found = true
			if state.SourceDigest != source || state.TargetDigest != target || !benchmarkReuseReceiptRetained(artifacts, &state.IdentityReceipt) || !benchmarkReuseReceiptRetained(artifacts, &state.CoverageReceipt) || request.AsOf.Before(state.ObservedAt) || !request.AsOf.Before(state.ValidUntil) {
				return true
			}
		}
		if !found {
			return true
		}
	}
	return false
}

func benchmarkReuseContainsAll(have, need []string) bool {
	for _, wanted := range need {
		found := false
		for _, got := range have {
			if got == wanted {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func benchmarkReuseArtifactAvailable(artifacts map[string]BenchmarkReuseArtifact, digest string) bool {
	artifact, ok := artifacts[digest]
	return ok && benchmarkReuseReceiptPresent(artifact.VerificationReceipt)
}
func benchmarkReuseReceiptPresent(receipt BenchmarkReuseReceipt) bool {
	return validBenchmarkID(receipt.ID) && benchmarkDigestRE.MatchString(receipt.Digest)
}
func benchmarkReuseReceiptRetained(artifacts map[string]BenchmarkReuseArtifact, receipt *BenchmarkReuseReceipt) bool {
	if receipt == nil || !benchmarkReuseReceiptPresent(*receipt) {
		return false
	}
	if artifacts == nil {
		return true
	}
	return benchmarkReuseArtifactAvailable(artifacts, receipt.Digest)
}
func benchmarkReuseUnitCoverageKnown(manifest BenchmarkManifest, unit BenchmarkReuseUnit, artifacts map[string]BenchmarkReuseArtifact) bool {
	if !benchmarkReuseSameRefs(benchmarkReuseBindingRefs(unit.Bindings), benchmarkReuseManifestRefs(manifest)) {
		return false
	}
	for _, binding := range unit.Bindings {
		component, ok := benchmarkReuseDependency(manifest, binding.Ref)
		if !ok {
			return false
		}
		if binding.Class == BenchmarkReuseLocalRetained {
			artifact, found := artifacts[binding.ArtifactDigest]
			if binding.ArtifactDigest != component || !benchmarkReuseReceiptPresent(binding.VerificationReceipt) || !found || artifact.VerificationReceipt != binding.VerificationReceipt {
				return false
			}
		}
		if binding.Class != BenchmarkReuseLocalRetained && binding.Class != BenchmarkReuseExternalObserved {
			return false
		}
	}
	return true
}
func benchmarkReuseSameRefs(left, right []string) bool {
	return len(left) == len(right) && benchmarkReuseContainsAll(left, right)
}
func benchmarkReuseManifestRefs(manifest BenchmarkManifest) []string {
	refs := make([]string, 0, len(manifest.Solution)+len(manifest.Evaluation))
	for name := range manifest.Solution {
		refs = append(refs, "solution."+name)
	}
	for name := range manifest.Evaluation {
		refs = append(refs, "evaluation."+name)
	}
	sort.Strings(refs)
	return refs
}
func benchmarkReuseBindingRefs(bindings []BenchmarkReuseDependencyBinding) []string {
	refs := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		refs = append(refs, binding.Ref)
	}
	sort.Strings(refs)
	return refs
}
func benchmarkReuseSameBindings(left, right []BenchmarkReuseDependencyBinding) bool {
	if len(left) != len(right) {
		return false
	}
	rightClass := make(map[string]BenchmarkReuseDependencyClass, len(right))
	for _, binding := range right {
		rightClass[binding.Ref] = binding.Class
	}
	for _, binding := range left {
		if rightClass[binding.Ref] != binding.Class {
			return false
		}
	}
	return true
}
func benchmarkReuseAllLocal(bindings []BenchmarkReuseDependencyBinding) bool {
	for _, binding := range bindings {
		if binding.Class != BenchmarkReuseLocalRetained {
			return false
		}
	}
	return true
}
func containsReuseAction(actions []BenchmarkReuseAction, action BenchmarkReuseAction) bool {
	for _, got := range actions {
		if got == action {
			return true
		}
	}
	return false
}

func validateBenchmarkReuseRequest(request BenchmarkReuseRequest) error {
	if err := request.Source.Validate(); err != nil {
		return benchmarkInvalid("reuse source: %v", err)
	}
	if err := request.TargetManifest.Validate(); err != nil {
		return benchmarkInvalid("reuse target manifest: %v", err)
	}
	if request.TargetMode != BenchmarkModeLive && request.TargetMode != BenchmarkModeReplay && request.TargetMode != BenchmarkModeSynthetic {
		return benchmarkInvalid("reuse target mode %q is invalid", request.TargetMode)
	}
	if !validBenchmarkTime(request.AsOf) {
		return benchmarkInvalid("reuse as_of must round-trip through RFC3339 JSON")
	}
	if len(request.SourceUnits) > maxBenchmarkReuseUnits || len(request.TargetUnits) == 0 || len(request.TargetUnits) > maxBenchmarkReuseUnits {
		return benchmarkInvalid("reuse units exceed bounds or target units are empty")
	}
	if err := validateBenchmarkReuseUnits("source", request.SourceUnits); err != nil {
		return err
	}
	if err := validateBenchmarkReuseUnits("target", request.TargetUnits); err != nil {
		return err
	}
	if len(request.SourceUnits) != len(request.Source.Trials) {
		return benchmarkInvalid("source units must cover every retained trial exactly once")
	}
	seenTrials := make(map[benchmarkTrialPair]struct{}, len(request.SourceUnits))
	trials := make(map[benchmarkTrialPair]BenchmarkTrial, len(request.Source.Trials))
	for _, trial := range request.Source.Trials {
		trials[benchmarkTrialKey(trial)] = trial
	}
	for _, unit := range request.SourceUnits {
		key := benchmarkTrialPair{Task: unit.TaskID, Replicate: unit.ReplicateID}
		if _, exists := seenTrials[key]; exists {
			return benchmarkInvalid("duplicate source unit trial binding")
		}
		seenTrials[key] = struct{}{}
		if trial, ok := trials[key]; !ok || unit.InputDigest != trial.TrialSpecDigest {
			return benchmarkInvalid("source unit input digest must equal retained trial spec digest")
		}
	}
	for _, trial := range request.Source.Trials {
		if _, ok := seenTrials[benchmarkTrialKey(trial)]; !ok {
			return benchmarkInvalid("source unit missing retained trial binding")
		}
	}
	if len(request.Artifacts) > maxBenchmarkReuseUnits*4 {
		return benchmarkInvalid("reuse artifacts exceeds bound")
	}
	seenArtifacts := make(map[string]struct{}, len(request.Artifacts))
	for _, artifact := range request.Artifacts {
		if !benchmarkDigestRE.MatchString(artifact.Digest) {
			return benchmarkInvalid("reuse artifact digest is invalid")
		}
		if _, exists := seenArtifacts[artifact.Digest]; exists {
			return benchmarkInvalid("duplicate reuse artifact digest")
		}
		if !benchmarkReuseReceiptPresent(artifact.VerificationReceipt) {
			return benchmarkInvalid("reuse artifact verification receipt is invalid")
		}
		seenArtifacts[artifact.Digest] = struct{}{}
	}
	if len(request.ExternalStates) > maxBenchmarkReuseStates {
		return benchmarkInvalid("reuse external states exceeds bound")
	}
	seenStates := make(map[string]struct{}, len(request.ExternalStates))
	for _, state := range request.ExternalStates {
		if !validBenchmarkID(state.ID) || !validBenchmarkReuseRef(state.Ref) || !benchmarkDigestRE.MatchString(state.SourceDigest) || !benchmarkDigestRE.MatchString(state.TargetDigest) || !benchmarkReuseReceiptPresent(state.IdentityReceipt) || !benchmarkReuseReceiptPresent(state.CoverageReceipt) {
			return benchmarkInvalid("reuse external state is invalid")
		}
		if !validBenchmarkTime(state.ObservedAt) || !validBenchmarkTime(state.ValidUntil) || !state.ValidUntil.After(state.ObservedAt) {
			return benchmarkInvalid("reuse external state timestamps are invalid")
		}
		if _, exists := seenStates[state.ID]; exists {
			return benchmarkInvalid("duplicate reuse external state id")
		}
		seenStates[state.ID] = struct{}{}
	}
	return nil
}

func validateBenchmarkReuseUnits(kind string, units []BenchmarkReuseUnit) error {
	seen := make(map[string]struct{}, len(units))
	for _, unit := range units {
		if !validBenchmarkID(unit.ID) || !validBenchmarkID(unit.TaskID) || !validBenchmarkID(unit.ReplicateID) || !benchmarkDigestRE.MatchString(unit.InputDigest) || len(unit.Bindings) == 0 || len(unit.Bindings) > maxBenchmarkReuseRefs {
			return benchmarkInvalid("%s reuse unit is invalid", kind)
		}
		if _, exists := seen[unit.ID]; exists {
			return benchmarkInvalid("duplicate %s reuse unit id", kind)
		}
		seen[unit.ID] = struct{}{}
		if unit.UsageReceipt != nil && !benchmarkReuseReceiptPresent(*unit.UsageReceipt) {
			return benchmarkInvalid("%s reuse usage receipt is invalid", kind)
		}
		if err := validateBenchmarkReuseBindings(unit.Bindings); err != nil {
			return err
		}
	}
	return nil
}

func validateBenchmarkReuseRefs(refs []string) error {
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		name, solution := strings.CutPrefix(ref, "solution.")
		if !solution {
			name, solution = strings.CutPrefix(ref, "evaluation.")
		}
		if !solution || !benchmarkComponentNameRE.MatchString(name) {
			return benchmarkInvalid("reuse dependency ref %q is invalid", ref)
		}
		if _, exists := seen[ref]; exists {
			return benchmarkInvalid("duplicate reuse dependency ref %q", ref)
		}
		seen[ref] = struct{}{}
	}
	return nil
}
func validBenchmarkReuseRef(ref string) bool { return validateBenchmarkReuseRefs([]string{ref}) == nil }
func validateBenchmarkReuseBindings(bindings []BenchmarkReuseDependencyBinding) error {
	seen := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		if !validBenchmarkReuseRef(binding.Ref) {
			return benchmarkInvalid("reuse binding ref %q is invalid", binding.Ref)
		}
		if _, exists := seen[binding.Ref]; exists {
			return benchmarkInvalid("duplicate reuse binding ref %q", binding.Ref)
		}
		seen[binding.Ref] = struct{}{}
		switch binding.Class {
		case BenchmarkReuseLocalRetained:
			if !benchmarkDigestRE.MatchString(binding.ArtifactDigest) {
				return benchmarkInvalid("local retained binding %q lacks artifact verification identity", binding.Ref)
			}
		case BenchmarkReuseExternalObserved:
			if binding.ArtifactDigest != "" || binding.VerificationReceipt != (BenchmarkReuseReceipt{}) {
				return benchmarkInvalid("external observed binding %q must not impersonate retained local bytes", binding.Ref)
			}
		default:
			// Keep the shape inspectable and let planning conservatively refuse
			// coverage rather than treating a new class as locally reusable.
		}
	}
	return nil
}

func canonicalBenchmarkReuseUnits(units []BenchmarkReuseUnit) []BenchmarkReuseUnit {
	canonical := make([]BenchmarkReuseUnit, len(units))
	for i, unit := range units {
		canonical[i] = unit
		canonical[i].Bindings = append([]BenchmarkReuseDependencyBinding(nil), unit.Bindings...)
		sort.Slice(canonical[i].Bindings, func(a, b int) bool { return canonical[i].Bindings[a].Ref < canonical[i].Bindings[b].Ref })
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].ID < canonical[j].ID })
	return canonical
}

func canonicalBenchmarkReuseStates(states []BenchmarkReuseExternalState) []BenchmarkReuseExternalState {
	canonical := make([]BenchmarkReuseExternalState, len(states))
	for i, state := range states {
		canonical[i] = state
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].ID < canonical[j].ID })
	return canonical
}

func benchmarkReuseTargetFingerprint(request BenchmarkReuseRequest) (string, error) {
	manifest, err := request.TargetManifest.Fingerprint()
	if err != nil {
		return "", err
	}
	canonical := struct {
		Domain             string                        `json:"domain"`
		Manifest           string                        `json:"manifest_fingerprint"`
		Units              []BenchmarkReuseUnit          `json:"units"`
		Mode               BenchmarkMode                 `json:"mode"`
		DeterminismReceipt *BenchmarkReuseReceipt        `json:"determinism_receipt"`
		States             []BenchmarkReuseExternalState `json:"external_states"`
	}{"agentic-kit/eval-runner/benchmark-reuse-target/v2", manifest, canonicalBenchmarkReuseUnits(request.TargetUnits), request.TargetMode, request.TargetDeterminismReceipt, canonicalBenchmarkReuseStates(request.ExternalStates)}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", benchmarkInvalid("canonical reuse target: %v", err)
	}
	return recordfingerprint.HashContent(body), nil
}

func benchmarkReusePlanFingerprint(plan BenchmarkReusePlan) (string, error) {
	canonical := struct {
		Domain  string                   `json:"domain"`
		Request string                   `json:"request_fingerprint"`
		Source  string                   `json:"source_fingerprint"`
		Target  string                   `json:"target_fingerprint"`
		AsOf    string                   `json:"as_of"`
		Units   []BenchmarkReuseUnitPlan `json:"units"`
	}{"agentic-kit/eval-runner/benchmark-reuse-plan/v2", plan.RequestFingerprint, plan.SourceFingerprint, plan.TargetFingerprint, plan.AsOf.UTC().Format(time.RFC3339Nano), plan.Units}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: canonical reuse plan: %v", ErrInvalidBenchmark, err)
	}
	return recordfingerprint.HashContent(body), nil
}

func benchmarkReuseRequestFingerprint(request BenchmarkReuseRequest, sourceFingerprint string) (string, error) {
	targetManifestFingerprint, err := request.TargetManifest.Fingerprint()
	if err != nil {
		return "", err
	}
	canonical := struct {
		Domain                    string                        `json:"domain"`
		SourceFingerprint         string                        `json:"source_fingerprint"`
		SourceUnits               []BenchmarkReuseUnit          `json:"source_units"`
		TargetManifestFingerprint string                        `json:"target_manifest_fingerprint"`
		TargetUnits               []BenchmarkReuseUnit          `json:"target_units"`
		SourceDeterminismReceipt  *BenchmarkReuseReceipt        `json:"source_determinism_receipt"`
		TargetDeterminismReceipt  *BenchmarkReuseReceipt        `json:"target_determinism_receipt"`
		TargetMode                BenchmarkMode                 `json:"target_mode"`
		Artifacts                 []BenchmarkReuseArtifact      `json:"artifacts"`
		ExternalStates            []BenchmarkReuseExternalState `json:"external_states"`
		AsOf                      string                        `json:"as_of"`
	}{
		Domain: "agentic-kit/eval-runner/benchmark-reuse-request/v2", SourceFingerprint: sourceFingerprint,
		SourceUnits: canonicalBenchmarkReuseUnits(request.SourceUnits), TargetManifestFingerprint: targetManifestFingerprint,
		TargetUnits: canonicalBenchmarkReuseUnits(request.TargetUnits), SourceDeterminismReceipt: request.SourceDeterminismReceipt,
		TargetDeterminismReceipt: request.TargetDeterminismReceipt, TargetMode: request.TargetMode,
		Artifacts: canonicalBenchmarkReuseArtifacts(request.Artifacts), ExternalStates: canonicalBenchmarkReuseStates(request.ExternalStates),
		AsOf: request.AsOf.UTC().Format(time.RFC3339Nano),
	}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", benchmarkInvalid("canonical reuse request: %v", err)
	}
	return recordfingerprint.HashContent(body), nil
}

func canonicalBenchmarkReuseArtifacts(artifacts []BenchmarkReuseArtifact) []BenchmarkReuseArtifact {
	canonical := append([]BenchmarkReuseArtifact(nil), artifacts...)
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].Digest < canonical[j].Digest })
	return canonical
}
