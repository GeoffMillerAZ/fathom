package bench_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"cuelang.org/go/cue"
)

func validateReviewValue(t *testing.T, def cue.Value, data map[string]any) error {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return def.Unify(def.Context().CompileBytes(b)).Validate(cue.Concrete(true))
}

func TestBenchSubjectCorpusEntries(t *testing.T) {
	// Entries copied unchanged from calibration/corpus.d/glsl.yaml
	// (KhronosGroup/SPIRV-Cross) and jenkinsfile.yaml (ROCm/MIOpen).
	b, err := os.ReadFile("testdata/corpus-repos.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(b, &entries); err != nil {
		t.Fatal(err)
	}
	def, base := fixture(t, "subject")
	for _, entry := range entries {
		entry["bench"] = base["bench"]
		if err := validateReviewValue(t, def, entry); err != nil {
			t.Fatalf("corpus entry %s: %v", entry["name"], err)
		}
	}
}

func TestBenchPreregArmPredictions(t *testing.T) {
	def, data := fixture(t, "prereg")
	data["arms"] = []any{"A1", "A2", "A3"}
	// Preserve every stratum's prediction while leaving A3 uncovered.
	if err := validateReviewValue(t, def, data); err == nil {
		t.Fatal("arm without a prediction accepted")
	}
	data["baseline_arm"] = "A1"
	if err := validateReviewValue(t, def, data); err == nil || !strings.Contains(err.Error(), "predictions") {
		t.Fatalf("missing arm prediction must be rejected: %v", err)
	}
	h := data["hypotheses"].([]any)[0].(map[string]any)
	h["predictions"] = append(h["predictions"].([]any), map[string]any{
		"stratum": "lexical", "arm": "A3", "direction": "decrease",
	})
	if err := validateReviewValue(t, def, data); err != nil {
		t.Fatalf("all comparison arms covered: %v", err)
	}
	delete(data, "baseline_arm")
	if err := validateReviewValue(t, def, data); err == nil {
		t.Fatal("missing baseline accepted")
	}
	data["baseline_arm"] = "A9"
	if err := validateReviewValue(t, def, data); err == nil {
		t.Fatal("undeclared baseline accepted")
	}
	// Exemption follows the declaration, even when the baseline is not first.
	data["baseline_arm"] = "A3"
	for _, p := range h["predictions"].([]any) {
		p := p.(map[string]any)
		if p["arm"] == "A3" {
			p["arm"] = "A1"
		}
	}
	if err := validateReviewValue(t, def, data); err != nil {
		t.Fatalf("declared baseline needs no prediction: %v", err)
	}
}

func TestBenchSubjectPendingCutoff(t *testing.T) {
	def, data := fixture(t, "subject")
	c := data["bench"].(map[string]any)["contamination"].(map[string]any)
	delete(c, "fresh_after")
	if err := validateReviewValue(t, def, data); err != nil {
		t.Fatalf("Q2 cutoff may be absent before the model pin: %v", err)
	}
	c["fresh_after"] = "2026-02-30"
	if err := validateReviewValue(t, def, data); err == nil {
		t.Fatal("invalid pinned cutoff accepted")
	}
}

func TestBenchPreregNullOutcomeRule(t *testing.T) {
	def, data := fixture(t, "prereg")
	rule := data["three_outcome_rule"].(map[string]any)
	delete(rule, "predicted_null")
	if err := validateReviewValue(t, def, data); err == nil {
		t.Fatal("missing predicted-null outcome rule accepted")
	}
	null := map[string]any{
		"equivalent":   "90_percent_interval_inside_symmetric_margin",
		"harm":         "95_percent_upper_bound_below_zero_counts_as_loss",
		"inconclusive": "otherwise",
		"above_delta":  "unexpected_effect_not_win",
	}
	rule["predicted_null"] = null
	if err := validateReviewValue(t, def, data); err != nil {
		t.Fatalf("complete null rule: %v", err)
	}
	for _, key := range []string{"equivalent", "harm", "inconclusive", "above_delta"} {
		value := null[key]
		delete(null, key)
		if err := validateReviewValue(t, def, data); err == nil {
			t.Fatalf("missing null %s rule accepted", key)
		}
		null[key] = "win"
		if err := validateReviewValue(t, def, data); err == nil {
			t.Fatalf("incorrect null %s rule accepted", key)
		}
		null[key] = value
	}
}

func TestBenchToolchainPrereleases(t *testing.T) {
	def, data := fixture(t, "subject")
	versions := data["bench"].(map[string]any)["toolchain"].(map[string]any)["versions"].(map[string]any)
	for _, version := range []string{"3.13.0rc1", "3.14.0a2", "3.14.0b3", "v1.27.0rc1", "1.2.3-rc.1+build.2", "1.2.3+build.2"} {
		versions["go"] = version
		if err := validateReviewValue(t, def, data); err != nil {
			t.Fatalf("exact prerelease %s rejected: %v", version, err)
		}
	}
	for _, version := range []string{"latest", "3.13", "3.13.0rc", "3.13.0rc1junk", "3.13.0\n", "1.2.3-"} {
		versions["go"] = version
		if err := validateReviewValue(t, def, data); err == nil {
			t.Fatalf("non-exact version %q accepted", version)
		}
	}
}
