package evalrunner

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPlanBenchmarkReuseDependencyMutationMatrix(t *testing.T) {
	for _, group := range []struct {
		name string
		keys map[string]string
	}{{"solution", testManifest().Solution}, {"evaluation", testManifest().Evaluation}} {
		for key := range group.keys {
			t.Run(group.name+"/"+key, func(t *testing.T) {
				request := testReuseRequest()
				ref := group.name + "." + key
				setTargetReuseDependency(&request, ref, digest("changed-"+ref))
				plan, err := PlanBenchmarkReuse(request)
				if err != nil {
					t.Fatal(err)
				}
				actions := plan.Units[0].Actions
				switch ref {
				case "evaluation.grader", "evaluation.acceptance":
					if !containsReuseAction(actions, BenchmarkReuseRegrade) || containsReuseAction(actions, BenchmarkReuseExactDeterministic) {
						t.Fatalf("%s actions = %v", ref, actions)
					}
				case "evaluation.pricing":
					if !containsReuseAction(actions, BenchmarkReuseReprice) || containsReuseAction(actions, BenchmarkReuseExactDeterministic) {
						t.Fatalf("%s actions = %v", ref, actions)
					}
				default:
					if !containsReuseAction(actions, BenchmarkReuseRerun) || containsReuseAction(actions, BenchmarkReuseExactDeterministic) {
						t.Fatalf("%s actions = %v", ref, actions)
					}
				}
			})
		}
	}
}

func TestPlanBenchmarkReuseCombinesIndependentActions(t *testing.T) {
	request := testReuseRequest()
	setTargetReuseDependency(&request, "evaluation.grader", digest("new-grader"))
	setTargetReuseDependency(&request, "evaluation.pricing", digest("new-pricing"))
	plan, err := PlanBenchmarkReuse(request)
	if err != nil {
		t.Fatal(err)
	}
	actions := plan.Units[0].Actions
	if !containsReuseAction(actions, BenchmarkReuseRegrade) || !containsReuseAction(actions, BenchmarkReuseReprice) {
		t.Fatalf("combined changes hid an action: %v", actions)
	}
}

func TestPlanBenchmarkReuseGenerationChangeInvalidatesGrade(t *testing.T) {
	request := testReuseRequest()
	setTargetReuseDependency(&request, "solution.code", digest("new-code"))
	setTargetReuseDependency(&request, "evaluation.grader", digest("new-grader"))
	plan, err := PlanBenchmarkReuse(request)
	if err != nil {
		t.Fatal(err)
	}
	actions := plan.Units[0].Actions
	if !containsReuseAction(actions, BenchmarkReuseRerun) || containsReuseAction(actions, BenchmarkReuseRegrade) || containsReuseAction(actions, BenchmarkReuseGrade) {
		t.Fatalf("generation change permitted stale reuse: %v", actions)
	}
}

func TestPlanBenchmarkReuseConservativelyRefusesIncompleteOrUnknownBindings(t *testing.T) {
	for _, ref := range []string{"solution.prompt", "solution.model", "evaluation.dataset"} {
		t.Run("generic unrelated closure cannot cover changed "+ref, func(t *testing.T) {
			request := testReuseRequest()
			request.Artifacts = append(request.Artifacts, testReuseArtifact("generic-closure", digest("generic-closure")))
			request.SourceUnits[0].Bindings = withoutReuseBinding(request.SourceUnits[0].Bindings, ref)
			request.TargetUnits[0].Bindings = withoutReuseBinding(request.TargetUnits[0].Bindings, ref)
			setTargetReuseManifestValue(&request, ref, digest("changed-"+ref))
			plan, err := PlanBenchmarkReuse(request)
			if err != nil || !containsReuseAction(plan.Units[0].Actions, BenchmarkReuseRerun) {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
		})
	}
	for name, mutate := range map[string]func(*BenchmarkReuseRequest){
		"unknown dependency": func(request *BenchmarkReuseRequest) { request.TargetUnits[0].Bindings[0].Ref = "solution.unknown" },
		"unknown binding class": func(request *BenchmarkReuseRequest) {
			request.TargetUnits[0].Bindings[0].Class = "future_binding_class"
		},
	} {
		t.Run(name, func(t *testing.T) {
			plan, err := PlanBenchmarkReuse(func() BenchmarkReuseRequest { request := testReuseRequest(); mutate(&request); return request }())
			if err != nil || !containsReuseAction(plan.Units[0].Actions, BenchmarkReuseRerun) {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
		})
	}
}

func TestPlanBenchmarkReuseRefusesInvalidShape(t *testing.T) {
	for name, mutate := range map[string]func(*BenchmarkReuseRequest){
		"source input not trial spec": func(request *BenchmarkReuseRequest) { request.SourceUnits[0].InputDigest = digest("not-the-trial") },
		"missing dependency":          func(request *BenchmarkReuseRequest) { delete(request.TargetManifest.Solution, "code") },
		"duplicate binding": func(request *BenchmarkReuseRequest) {
			request.TargetUnits[0].Bindings = append(request.TargetUnits[0].Bindings, request.TargetUnits[0].Bindings[0])
		},
		"duplicate unit": func(request *BenchmarkReuseRequest) {
			request.TargetUnits = append(request.TargetUnits, request.TargetUnits[0])
		},
		"duplicate artifact": func(request *BenchmarkReuseRequest) {
			request.Artifacts = append(request.Artifacts, request.Artifacts[0])
		},
		"nan source": func(request *BenchmarkReuseRequest) { value := math.NaN(); request.Source.Trials[0].CostUSD = &value },
	} {
		t.Run(name, func(t *testing.T) {
			request := testReuseRequest()
			mutate(&request)
			if _, err := PlanBenchmarkReuse(request); !errors.Is(err, ErrInvalidBenchmark) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPlanBenchmarkReuseLocalDeterministicNeedsNoExternalState(t *testing.T) {
	request := testReuseRequest()
	if len(request.ExternalStates) != 0 {
		t.Fatal("fixture should demonstrate entirely local deterministic reuse")
	}
	plan, err := PlanBenchmarkReuse(request)
	if err != nil || !containsReuseAction(plan.Units[0].Actions, BenchmarkReuseExactDeterministic) || containsReuseAction(plan.Units[0].Actions, BenchmarkReuseFreshLive) {
		t.Fatalf("local exact plan=%+v err=%v", plan, err)
	}
	if plan.IndependentTrialsAdded != 0 || plan.Units[0].IndependentTrialsAdded != 0 || plan.ExecutionOccurred {
		t.Fatalf("proposal claimed execution: %+v", plan)
	}
}

func TestPlanBenchmarkReuseReceiptsAndLiveMode(t *testing.T) {
	for name, test := range map[string]struct {
		mutate func(*BenchmarkReuseRequest)
		want   BenchmarkReuseAction
	}{
		"determinism receipt absent": {func(request *BenchmarkReuseRequest) { request.SourceDeterminismReceipt = nil }, BenchmarkReuseGrade},
		"local artifact verification absent": {func(request *BenchmarkReuseRequest) {
			request.TargetUnits[0].Bindings[0].VerificationReceipt = BenchmarkReuseReceipt{}
		}, BenchmarkReuseRerun},
		"local artifact verification identity mismatched": {func(request *BenchmarkReuseRequest) {
			request.TargetUnits[0].Bindings[0].VerificationReceipt = BenchmarkReuseReceipt{ID: "wrong", Digest: digest("wrong")}
		}, BenchmarkReuseRerun},
		"output verification absent": {func(request *BenchmarkReuseRequest) {
			removeReuseArtifact(request, *request.Source.Trials[0].OutputDigest)
		}, BenchmarkReuseRerun},
	} {
		t.Run(name, func(t *testing.T) {
			request := testReuseRequest()
			test.mutate(&request)
			plan, err := PlanBenchmarkReuse(request)
			if err != nil || !containsReuseAction(plan.Units[0].Actions, test.want) {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
			if name == "determinism receipt absent" && containsReuseAction(plan.Units[0].Actions, BenchmarkReuseExactDeterministic) {
				t.Fatalf("missing determinism receipt allowed exact reuse: %v", plan.Units[0].Actions)
			}
		})
	}
	live := testReuseRequest()
	live.TargetMode = BenchmarkModeLive
	plan, err := PlanBenchmarkReuse(live)
	if err != nil || !containsReuseAction(plan.Units[0].Actions, BenchmarkReuseFreshLive) || containsReuseAction(plan.Units[0].Actions, BenchmarkReuseExactDeterministic) || containsReuseAction(plan.Units[0].Actions, BenchmarkReuseGrade) {
		t.Fatalf("live plan substituted retained evidence: %+v err=%v", plan, err)
	}
}

func TestPlanBenchmarkReuseExternalStateBindsComponentValues(t *testing.T) {
	for name, addState := range map[string]func(*BenchmarkReuseRequest){
		"absent": func(*BenchmarkReuseRequest) {},
		"generic unrelated prompt receipt": func(request *BenchmarkReuseRequest) {
			addExternalReuseState(request, "solution.model", digest("other-prompt"), request.TargetManifest.Solution["model"])
		},
		"expired": func(request *BenchmarkReuseRequest) {
			addExternalReuseState(request, "solution.model", request.Source.Manifest.Solution["model"], request.TargetManifest.Solution["model"])
			request.ExternalStates[0].ValidUntil = request.AsOf
		},
		"future": func(request *BenchmarkReuseRequest) {
			addExternalReuseState(request, "solution.model", request.Source.Manifest.Solution["model"], request.TargetManifest.Solution["model"])
			request.ExternalStates[0].ObservedAt = request.AsOf.Add(time.Hour)
			request.ExternalStates[0].ValidUntil = request.AsOf.Add(2 * time.Hour)
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := testReuseRequest()
			setExternalReuseBinding(&request, "solution.model")
			addState(&request)
			plan, err := PlanBenchmarkReuse(request)
			if err != nil || !containsReuseAction(plan.Units[0].Actions, BenchmarkReuseFreshLive) {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
		})
	}
	matching := testReuseRequest()
	setExternalReuseBinding(&matching, "solution.model")
	addExternalReuseState(&matching, "solution.model", matching.Source.Manifest.Solution["model"], matching.TargetManifest.Solution["model"])
	plan, err := PlanBenchmarkReuse(matching)
	if err != nil || containsReuseAction(plan.Units[0].Actions, BenchmarkReuseFreshLive) || containsReuseAction(plan.Units[0].Actions, BenchmarkReuseExactDeterministic) {
		t.Fatalf("external observed plan=%+v err=%v", plan, err)
	}
}

func TestPlanBenchmarkReuseRepriceNeedsCompleteRetainedUsage(t *testing.T) {
	for name, mutate := range map[string]func(*BenchmarkReuseRequest){
		"usage artifact lost": func(request *BenchmarkReuseRequest) {
			removeReuseArtifact(request, request.SourceUnits[0].UsageReceipt.Digest)
		},
		"usage absent":     func(request *BenchmarkReuseRequest) { request.SourceUnits[0].UsageReceipt = nil },
		"usage incomplete": func(request *BenchmarkReuseRequest) { request.Source.Trials[0].CostComplete = false },
	} {
		t.Run(name, func(t *testing.T) {
			request := testReuseRequest()
			setTargetReuseDependency(&request, "evaluation.pricing", digest("changed-price"))
			mutate(&request)
			plan, err := PlanBenchmarkReuse(request)
			if err != nil || !containsReuseAction(plan.Units[0].Actions, BenchmarkReuseRerun) {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
		})
	}
}

func TestPlanBenchmarkReuseFingerprintsEveryDecisionInputAndCanonicalizesBindings(t *testing.T) {
	first, err := PlanBenchmarkReuse(testReuseRequest())
	if err != nil {
		t.Fatal(err)
	}
	changed := testReuseRequest()
	changed.AsOf = changed.AsOf.Add(time.Minute)
	second, err := PlanBenchmarkReuse(changed)
	if err != nil || first.RequestFingerprint == second.RequestFingerprint || first.PlanFingerprint == second.PlanFingerprint {
		t.Fatalf("as_of did not bind plan input: first=%+v second=%+v err=%v", first, second, err)
	}
	permuted := testReuseRequest()
	reverseReuseBindings(permuted.SourceUnits[0].Bindings)
	reverseReuseBindings(permuted.TargetUnits[0].Bindings)
	permutedPlan, err := PlanBenchmarkReuse(permuted)
	if err != nil || first.TargetFingerprint != permutedPlan.TargetFingerprint || first.RequestFingerprint != permutedPlan.RequestFingerprint {
		t.Fatalf("binding order changed canonical identity, plan=%+v err=%v", permutedPlan, err)
	}
}

func TestPlanBenchmarkReuseJSONProjectsToCUE(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue not installed")
	}
	plan, err := PlanBenchmarkReuse(testReuseRequest())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip BenchmarkReusePlan
	if err := json.Unmarshal(body, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan, roundTrip) {
		t.Fatalf("JSON round trip changed plan\n%+v\n%+v", plan, roundTrip)
	}
	path := filepath.Join(t.TempDir(), "reuse.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("cue", "vet", "-d", "#BenchmarkReusePlan", "benchmark.cue", "reuse.cue", path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("proposal JSON did not project to CUE: %v\n%s", err, output)
	}
	request := testReuseRequest()
	requestBody, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var requestRoundTrip BenchmarkReuseRequest
	if err := json.Unmarshal(requestBody, &requestRoundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(request, requestRoundTrip) {
		t.Fatalf("request JSON round trip changed input\n%+v\n%+v", request, requestRoundTrip)
	}
	requestPath := filepath.Join(t.TempDir(), "reuse-request.json")
	if err := os.WriteFile(requestPath, requestBody, 0o600); err != nil {
		t.Fatal(err)
	}
	command = exec.Command("cue", "vet", "-d", "#BenchmarkReuseRequest", "benchmark.cue", "reuse.cue", requestPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("request JSON did not project to CUE: %v\n%s", err, output)
	}
}

func TestBenchmarkReuseCUERejectsContradictoryBinding(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue not installed")
	}
	requestBody, err := json.Marshal(testReuseRequest())
	if err != nil {
		t.Fatal(err)
	}
	var projection map[string]any
	if err := json.Unmarshal(requestBody, &projection); err != nil {
		t.Fatal(err)
	}
	units := projection["target_units"].([]any)
	bindings := units[0].(map[string]any)["bindings"].([]any)
	binding := bindings[0].(map[string]any)
	binding["class"] = string(BenchmarkReuseExternalObserved)
	// An external observation with a local artifact identity is contradictory.
	// It must not project even though both individual values look well formed.
	path := filepath.Join(t.TempDir(), "contradictory-reuse-request.json")
	body, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("cue", "vet", "-d", "#BenchmarkReuseRequest", "benchmark.cue", "reuse.cue", path)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("contradictory external/local binding projected successfully:\n%s", output)
	}
}

func testReuseRequest() BenchmarkReuseRequest {
	manifest := testManifest()
	trial := testTrial("task-reuse", "r-1", BenchmarkOutcomeAccepted)
	trial.CostBasis = BenchmarkCostUsage
	request := BenchmarkReuseRequest{Source: testEvidence(manifest, BenchmarkModeLive, trial), TargetManifest: testManifest(), TargetMode: BenchmarkModeReplay, AsOf: time.Date(2026, time.September, 16, 1, 0, 0, 0, time.UTC)}
	bindings := make([]BenchmarkReuseDependencyBinding, 0, len(benchmarkReuseManifestRefs(manifest)))
	for _, ref := range benchmarkReuseManifestRefs(manifest) {
		value, _ := benchmarkReuseDependency(manifest, ref)
		receipt := addReuseArtifact(&request, "component-"+ref, value)
		bindings = append(bindings, BenchmarkReuseDependencyBinding{Ref: ref, Class: BenchmarkReuseLocalRetained, ArtifactDigest: value, VerificationReceipt: receipt})
	}
	usage := retainedReuseReceipt(&request, "retained-usage")
	sourceDeterminism := retainedReuseReceipt(&request, "source-determinism")
	targetDeterminism := retainedReuseReceipt(&request, "target-determinism")
	addReuseArtifact(&request, "output", *trial.OutputDigest)
	addReuseArtifact(&request, "grade", *trial.AcceptanceDigest)
	unit := BenchmarkReuseUnit{ID: "unit-reuse", TaskID: trial.TaskID, ReplicateID: trial.ReplicateID, InputDigest: trial.TrialSpecDigest, Bindings: bindings, UsageReceipt: &usage}
	request.SourceUnits = []BenchmarkReuseUnit{unit}
	request.TargetUnits = []BenchmarkReuseUnit{cloneReuseUnit(unit)}
	request.SourceDeterminismReceipt = &sourceDeterminism
	request.TargetDeterminismReceipt = &targetDeterminism
	return request
}

func testReuseArtifact(label, value string) BenchmarkReuseArtifact {
	return BenchmarkReuseArtifact{Digest: value, VerificationReceipt: BenchmarkReuseReceipt{ID: label + "-verification", Digest: digest(label + "-verification")}}
}
func addReuseArtifact(request *BenchmarkReuseRequest, label, value string) BenchmarkReuseReceipt {
	for _, artifact := range request.Artifacts {
		if artifact.Digest == value {
			return artifact.VerificationReceipt
		}
	}
	artifact := testReuseArtifact(label, value)
	request.Artifacts = append(request.Artifacts, artifact)
	return artifact.VerificationReceipt
}

func retainedReuseReceipt(request *BenchmarkReuseRequest, label string) BenchmarkReuseReceipt {
	receipt := BenchmarkReuseReceipt{ID: label, Digest: digest(label)}
	addReuseArtifact(request, label, receipt.Digest)
	return receipt
}
func cloneReuseUnit(unit BenchmarkReuseUnit) BenchmarkReuseUnit {
	copy := unit
	copy.Bindings = append([]BenchmarkReuseDependencyBinding(nil), unit.Bindings...)
	return copy
}
func setTargetReuseManifestValue(request *BenchmarkReuseRequest, ref, value string) {
	if name, ok := strings.CutPrefix(ref, "solution."); ok {
		request.TargetManifest.Solution[name] = value
		return
	}
	name, _ := strings.CutPrefix(ref, "evaluation.")
	request.TargetManifest.Evaluation[name] = value
}
func setTargetReuseDependency(request *BenchmarkReuseRequest, ref, value string) {
	setTargetReuseManifestValue(request, ref, value)
	for index := range request.TargetUnits[0].Bindings {
		binding := &request.TargetUnits[0].Bindings[index]
		if binding.Ref == ref && binding.Class == BenchmarkReuseLocalRetained {
			binding.ArtifactDigest = value
			binding.VerificationReceipt = addReuseArtifact(request, "target-"+ref, value)
		}
	}
}
func withoutReuseBinding(bindings []BenchmarkReuseDependencyBinding, ref string) []BenchmarkReuseDependencyBinding {
	result := make([]BenchmarkReuseDependencyBinding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Ref != ref {
			result = append(result, binding)
		}
	}
	return result
}
func removeReuseArtifact(request *BenchmarkReuseRequest, value string) {
	artifacts := request.Artifacts[:0]
	for _, artifact := range request.Artifacts {
		if artifact.Digest != value {
			artifacts = append(artifacts, artifact)
		}
	}
	request.Artifacts = artifacts
}
func setExternalReuseBinding(request *BenchmarkReuseRequest, ref string) {
	for _, unit := range []*BenchmarkReuseUnit{&request.SourceUnits[0], &request.TargetUnits[0]} {
		for index := range unit.Bindings {
			binding := &unit.Bindings[index]
			if binding.Ref == ref {
				binding.Class = BenchmarkReuseExternalObserved
				binding.ArtifactDigest = ""
				binding.VerificationReceipt = BenchmarkReuseReceipt{}
			}
		}
	}
}
func addExternalReuseState(request *BenchmarkReuseRequest, ref, source, target string) {
	identity := retainedReuseReceipt(request, "identity-"+ref)
	coverage := retainedReuseReceipt(request, "coverage-"+ref)
	request.ExternalStates = append(request.ExternalStates, BenchmarkReuseExternalState{ID: "state-" + strings.ReplaceAll(ref, ".", "-"), Ref: ref, SourceDigest: source, TargetDigest: target, IdentityReceipt: identity, CoverageReceipt: coverage, ObservedAt: request.AsOf.Add(-time.Hour), ValidUntil: request.AsOf.Add(time.Hour)})
}
func reverseReuseBindings(bindings []BenchmarkReuseDependencyBinding) {
	for left, right := 0, len(bindings)-1; left < right; left, right = left+1, right-1 {
		bindings[left], bindings[right] = bindings[right], bindings[left]
	}
}
