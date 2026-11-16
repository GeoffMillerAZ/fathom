package wiring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
)

const conformanceDir = "../../schema/wiring-proof/"

var conformanceFixtures = map[string]string{
	"wired_preflight": "#Preflight", "baseline_preflight": "#Preflight", "hook_only_preflight": "#Preflight",
	"unknown_preflight": "#Preflight", "failed_preflight": "#Preflight", "wsl_preflight": "#Preflight",
	"zero_opportunities": "#Outcome", "zero_calls": "#Outcome", "zero_actions": "#Outcome", "acted_at_k": "#Outcome",
	"two_deliveries_no_capture": "#Outcome", "two_deliveries_complete_no_actions": "#Outcome",
	"classifier_counts_only": "#Outcome", "classifier_failed_validation": "#Outcome", "unknown_denominator": "#Outcome",
	"one_third": "#Outcome", "two_thirds": "#Outcome",
	"preflight_only": "#Conformance", "with_zero_calls": "#Conformance", "with_zero_actions": "#Conformance",
}

func conformanceLoad(t *testing.T) cue.Value {
	t.Helper()
	schema, err := os.ReadFile(conformanceDir + "schema.cue")
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := os.ReadFile(conformanceDir + "testdata/valid.cue")
	if err != nil {
		t.Fatal(err)
	}
	v := cuecontext.New().CompileString(string(schema) + "\n" + strings.Replace(string(fixtures), "package wiringproof", "", 1))
	if err := v.Err(); err != nil {
		t.Fatal(errors.Details(err, nil))
	}
	return v
}

func conformancePositive(t *testing.T, v cue.Value, name string) map[string]any {
	t.Helper()
	def, ok := conformanceFixtures[name]
	if !ok {
		t.Fatalf("unknown fixture %q", name)
	}
	value := v.LookupPath(cue.ParsePath(name))
	if !value.Exists() {
		t.Fatalf("missing fixture %s", name)
	}
	var original map[string]any
	sourceJSON, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(sourceJSON, &original); err != nil {
		t.Fatal(err)
	}
	value = v.LookupPath(cue.ParsePath(def)).Unify(value)
	if err := value.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("%s: %s", name, errors.Details(err, nil))
	}
	raw, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, original) {
		t.Fatalf("%s: schema introduced output fields or changed fixture", name)
	}
	return data
}

// Decode a fresh concrete JSON object for every mutation; never unify a changed
// value into the constrained positive (which would manufacture a conflict).
func conformanceCopy(t *testing.T, data map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var copy map[string]any
	if err := json.Unmarshal(b, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func conformanceAt(t *testing.T, data any, path string) any {
	t.Helper()
	if path == "" {
		return data
	}
	for _, part := range strings.Split(path, "/") {
		switch x := data.(type) {
		case map[string]any:
			var ok bool
			data, ok = x[part]
			if !ok {
				t.Fatalf("missing path %s", path)
			}
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(x) {
				t.Fatalf("bad path %s", path)
			}
			data = x[i]
		default:
			t.Fatalf("non-container in %s", path)
		}
	}
	return data
}

func conformanceMutate(t *testing.T, data map[string]any, path string, value any, remove bool) {
	t.Helper()
	parts := strings.Split(path, "/")
	parent := conformanceAt(t, data, strings.Join(parts[:len(parts)-1], "/"))
	key := parts[len(parts)-1]
	switch x := parent.(type) {
	case map[string]any:
		old, exists := x[key]
		if remove {
			if !exists {
				t.Fatalf("remove absent %s", path)
			}
			delete(x, key)
		} else {
			if exists && reflect.DeepEqual(old, value) {
				t.Fatalf("no-op mutation %s", path)
			}
			x[key] = value
		}
	case []any:
		i, err := strconv.Atoi(key)
		if err != nil || i < 0 || i >= len(x) || remove {
			t.Fatalf("invalid list mutation %s", path)
		}
		if reflect.DeepEqual(x[i], value) {
			t.Fatalf("no-op mutation %s", path)
		}
		x[i] = value
	default:
		t.Fatalf("invalid mutation parent %s", path)
	}
}

func conformanceCheck(t *testing.T, v cue.Value, def string, data map[string]any) cue.Value {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	candidate := v.Context().CompileBytes(b)
	if err := candidate.Err(); err != nil {
		t.Fatal(err)
	}
	return v.LookupPath(cue.ParsePath(def)).Unify(candidate)
}

func conformanceReject(t *testing.T, v cue.Value, def string, data map[string]any, diagnostic string) {
	t.Helper()
	err := conformanceCheck(t, v, def, data).Validate(cue.Concrete(true))
	if err == nil {
		t.Fatal("CUE accepted independent mutation")
	}
	detail := errors.Details(err, nil)
	located := false
	for _, issue := range errors.Errors(err) {
		if strings.Contains(issue.Error(), "required field missing: "+diagnostic) {
			located = true
		}
		for _, part := range issue.Path() {
			if part == diagnostic {
				located = true
			}
		}
	}
	if !located {
		t.Fatalf("expected diagnostic at %s: %s", diagnostic, detail)
	}
}

func conformanceEqual(t *testing.T, data any, path string, want any) {
	t.Helper()
	got := conformanceAt(t, data, path)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: got %#v, want %#v", path, got, want)
	}
}

func conformanceAbsent(t *testing.T, data any, path, key string) {
	t.Helper()
	if _, ok := conformanceAt(t, data, path).(map[string]any)[key]; ok {
		t.Fatalf("unexpected %s/%s", path, key)
	}
}

func conformanceRoundTrip(t *testing.T, v cue.Value, name string) map[string]any {
	t.Helper()
	data := conformancePositive(t, v, name)
	data = conformanceCopy(t, data)
	if err := conformanceCheck(t, v, conformanceFixtures[name], data).Validate(cue.Concrete(true)); err != nil {
		t.Fatal(errors.Details(err, nil))
	}
	return data
}

func TestConformanceSchemaValid(t *testing.T) {
	v := conformanceLoad(t)
	names := make([]string, 0, len(conformanceFixtures))
	for name := range conformanceFixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			data := conformanceRoundTrip(t, v, name)
			var walk func(any)
			walk = func(value any) {
				switch x := value.(type) {
				case nil:
					t.Fatal("null in concrete fixture")
				case map[string]any:
					for _, child := range x {
						walk(child)
					}
				case []any:
					for _, child := range x {
						walk(child)
					}
				}
			}
			walk(data)
		})
	}
	wired := conformancePositive(t, v, "wired_preflight")
	for _, r := range []string{"w0", "w1", "w2", "w3", "w6"} {
		conformanceEqual(t, wired, "rungs/"+r+"/verdict", "PASS")
	}
	base := conformancePositive(t, v, "baseline_preflight")
	conformanceEqual(t, base, "profile/product_present", false)
	for _, r := range []string{"w0", "w1", "w2", "w3"} {
		conformanceEqual(t, base, "rungs/"+r+"/verdict", "NOT-APPLICABLE")
	}
	conformanceEqual(t, base, "rungs/w6/verdict", "PASS")
	hook := conformancePositive(t, v, "hook_only_preflight")
	conformanceEqual(t, hook, "profile/product_present", true)
	conformanceEqual(t, hook, "rungs/w1/verdict", "NOT-APPLICABLE")
	for _, r := range []string{"w0", "w2", "w3", "w6"} {
		conformanceEqual(t, hook, "rungs/"+r+"/verdict", "PASS")
	}
	conformanceEqual(t, conformancePositive(t, v, "unknown_preflight"), "rungs/w6/verdict", "UNKNOWN")
	conformanceEqual(t, conformancePositive(t, v, "failed_preflight"), "rungs/w6/verdict", "FAIL")
}

const conformanceNegativeNames = `
unknown_field unknown_verdict unknown_harness unknown_environment unknown_source unknown_channel unknown_metric
missing_pin short_product_commit branch_subject_commit bad_digest profile_name_as_digest
w4_requirement w5_requirement missing_requirement duplicate_requirement
applicable_na inapplicable_pass na_without_reason absence_without_inventory missing_off_channel enabled_absence baseline_product_present_mismatch baseline_enabled_hook surface_off_w0_na surface_off_w2_na surface_off_w3_na missing_baseline_checks wrong_baseline_capture
hooks_channel_contradiction hooks_channel_enabled_without_hook
ancestor_served wrong_installed missing_served recent_wrong_index wrong_primer unnegotiated_tool no_answered_probe missing_hook unknown_hook wrong_beat_kind unjoined_beat
missing_registration missing_registration_evidence wrong_registration_kind wrong_registration_service wrong_registration_tools missing_registration_rung_citation missing_startup_service wrong_startup_service
missing_sample over_budget wrong_footprint_metric
post_agent_decision decision_at_start reverse_window late_freeze changed_environment changed_profile changed_workload changed_budget
late_evidence wrong_session wrong_evidence_product wrong_evidence_subject wrong_evidence_profile wrong_evidence_harness empty_range dangling_reference duplicate_evidence unsorted_evidence
agent_in_preflight probe_in_outcome null_list outcome_binding delivery_parameters_binding environment_pin_binding
zero_measured_denominator negative_count numerator_exceeds_denominator redundant_rate_value wrong_denominator missing_denominator_provenance zero_k late_k
missing_expected_service missing_registration_evidence_service wrong_registration_evidence_service missing_negotiation_service wrong_negotiation_service missing_probe_service wrong_probe_service
missing_measured_numerator unavailable_numerator unvalidated_numerator unavailable_measured unvalidated_measured fabricated_acted_zero unknown_as_zero positive_zero_denominator_cause wrong_w5_numerator unsupported_w4_numerator
missing_capture_summary wrong_capture_kind incomplete_capture_measured dangling_captured_action empty_capture_positive_acted capture_in_preflight capture_under_profile capture_evidence_in_preflight
`

func TestConformanceSchemaNegatives(t *testing.T) {
	v := conformanceLoad(t)
	raw, err := os.ReadFile(conformanceDir + "testdata/invalid.cue")
	if err != nil {
		t.Fatal(err)
	}
	var uncommented strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			t.Fatalf("uncommented invalid.cue line %q", line)
		}
		uncommented.WriteString(strings.TrimPrefix(line, "//"))
		uncommented.WriteByte('\n')
	}
	_, inventoryText, found := strings.Cut(uncommented.String(), "cases: [")
	if !found {
		t.Fatal("invalid.cue has no cases list")
	}
	cases := v.Context().CompileString("cases: [" + inventoryText)
	var inventory struct {
		Cases []struct {
			Name       string `json:"name"`
			Fixture    string `json:"fixture"`
			Definition string `json:"definition"`
			Path       string `json:"path"`
			Value      any    `json:"value"`
			Remove     bool   `json:"remove"`
			Diagnostic string `json:"diagnostic"`
		} `json:"cases"`
	}
	if err := cases.Decode(&inventory); err != nil {
		t.Fatal(err)
	}
	required := map[string]bool{}
	for _, name := range strings.Fields(conformanceNegativeNames) {
		required[name] = false
	}
	if len(required) != 110 {
		t.Fatalf("required inventory: %d", len(required))
	}
	if len(inventory.Cases) == 0 {
		t.Fatal("empty inventory")
	}
	seen := map[string]bool{}
	for index, c := range inventory.Cases {
		if c.Name == "" || seen[c.Name] {
			t.Fatalf("empty/duplicate case %q", c.Name)
		}
		seen[c.Name] = true
		hasValue := cases.LookupPath(cue.MakePath(cue.Str("cases"), cue.Index(index), cue.Str("value"))).Exists()
		hasRemove := cases.LookupPath(cue.MakePath(cue.Str("cases"), cue.Index(index), cue.Str("remove"))).Exists()
		if hasValue == hasRemove || (hasRemove && !c.Remove) {
			t.Fatalf("%s: require exactly one replacement value or remove:true", c.Name)
		}
		if _, ok := required[c.Name]; ok {
			required[c.Name] = true
		}
		t.Run(c.Name, func(t *testing.T) {
			if c.Path == "" || c.Diagnostic == "" || c.Definition != conformanceFixtures[c.Fixture] {
				t.Fatal("missing/incorrect mutation metadata")
			}
			// Validate the source immediately before the independent mutation.
			data := conformanceCopy(t, conformancePositive(t, v, c.Fixture))
			conformanceMutate(t, data, c.Path, c.Value, c.Remove)
			conformanceReject(t, v, c.Definition, data, c.Diagnostic)
		})
	}
	for name, present := range required {
		if !present {
			t.Errorf("missing required negative %s", name)
		}
	}
	t.Logf("%d independent negative cases", len(inventory.Cases))
}

func TestConformanceSchemaPhaseSeparation(t *testing.T) {
	v := conformanceLoad(t)
	for _, name := range []string{"wired_preflight", "baseline_preflight"} {
		for _, parent := range []string{"", "profile/"} {
			for _, key := range []string{"w4", "w5", "k", "bars", "successful_calls", "acted", "capture"} {
				t.Run(name+"/"+parent+key, func(t *testing.T) {
					data := conformancePositive(t, v, name)
					conformanceMutate(t, data, parent+key, true, false)
					conformanceReject(t, v, "#Preflight", data, key)
				})
			}
		}
		for _, key := range []string{"w4", "w5"} {
			data := conformancePositive(t, v, name)
			conformanceMutate(t, data, "rungs/"+key, true, false)
			conformanceReject(t, v, "#Preflight", data, key)
		}
	}
	for _, c := range []struct {
		name, def string
		paths     []string
	}{
		{"wired_preflight", "#Preflight", []string{"source_text", "profile/source_text", "evidence/0/source_text"}},
		{"acted_at_k", "#Outcome", []string{"source_text", "capabilities/0/source_text", "evidence/0/source_text"}},
	} {
		for _, path := range c.paths {
			t.Run(c.name+"/"+path, func(t *testing.T) {
				data := conformancePositive(t, v, c.name)
				conformanceMutate(t, data, path, "synthetic raw bytes", false)
				conformanceReject(t, v, c.def, data, "source_text")
			})
		}
	}
	preflight := conformancePositive(t, v, "wired_preflight")
	want, _ := json.Marshal(preflight)
	for _, name := range []string{"zero_calls", "zero_actions", "two_deliveries_complete_no_actions"} {
		envelope := map[string]any{"preflight": conformanceCopy(t, preflight), "outcome": conformancePositive(t, v, name)}
		value := conformanceCheck(t, v, "#Conformance", envelope)
		if err := value.Validate(cue.Concrete(true)); err != nil {
			t.Fatal(errors.Details(err, nil))
		}
		got, err := value.LookupPath(cue.ParsePath("preflight")).MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var canonical map[string]any
		if err := json.Unmarshal(got, &canonical); err != nil {
			t.Fatal(err)
		}
		got, _ = json.Marshal(canonical)
		if !bytes.Equal(got, want) {
			t.Fatal("outcome changed admission preflight")
		}
	}
	for _, c := range []struct{ name, phase, def string }{{"wired_preflight", "agent", "#Preflight"}, {"zero_calls", "preflight", "#Outcome"}} {
		data := conformancePositive(t, v, c.name)
		conformanceMutate(t, data, "evidence/0/phase", c.phase, false)
		conformanceReject(t, v, c.def, data, "phase")
	}
}

func TestConformanceSchemaOutcomeBoundaries(t *testing.T) {
	v := conformanceLoad(t)
	const root = "capabilities/0/"
	for _, c := range []struct {
		name, rate string
		n, d       float64
		display    string
	}{
		{"zero_calls", "w4", 0, 2, "0"}, {"zero_actions", "w5", 0, 2, "0"}, {"acted_at_k", "w4", 1, 2, "0.5"},
		{"acted_at_k", "w5", 2, 2, "1"}, {"one_third", "w4", 1, 3, "0.3333333333333333"}, {"two_thirds", "w4", 2, 3, "0.6666666666666666"},
	} {
		t.Run(c.name+"/"+c.rate, func(t *testing.T) {
			data := conformanceRoundTrip(t, v, c.name)
			conformanceEqual(t, data, root+c.rate+"/state", "measured")
			conformanceEqual(t, data, root+c.rate+"/numerator", c.n)
			conformanceEqual(t, data, root+c.rate+"/denominator", c.d)
			conformanceAbsent(t, data, root+c.rate, "value")
			if got := strconv.FormatFloat(conformanceAt(t, data, root+c.rate+"/numerator").(float64)/conformanceAt(t, data, root+c.rate+"/denominator").(float64), 'g', -1, 64); got != c.display {
				t.Fatalf("display %s", got)
			}
			conformanceMutate(t, data, root+c.rate+"/value", c.n/c.d, false)
			conformanceReject(t, v, "#Outcome", data, "value")
		})
	}
	zero := conformanceRoundTrip(t, v, "zero_opportunities")
	for _, r := range []string{"w4", "w5"} {
		conformanceEqual(t, zero, root+r+"/state", "not_measured")
		conformanceEqual(t, zero, root+r+"/cause", "zero_denominator")
		conformanceEqual(t, zero, root+r+"/denominator/count", float64(0))
		conformanceAbsent(t, zero, root+r, "numerator")
	}
	complete := conformanceRoundTrip(t, v, "two_deliveries_complete_no_actions")
	conformanceEqual(t, complete, root+"denominator/delivered_targets", map[string]any{"state": "known", "count": float64(2)})
	conformanceEqual(t, complete, root+"denominator/delivery/state", "validated")
	conformanceEqual(t, complete, root+"action_capture/state", "validated")
	conformanceEqual(t, complete, root+"acted", map[string]any{"state": "known", "count": float64(0)})
	conformanceEqual(t, complete, root+"w5", map[string]any{"state": "measured", "numerator": float64(0), "denominator": float64(2)})
	captures, actions := 0, 0
	for _, item := range complete["evidence"].([]any) {
		e := item.(map[string]any)
		if e["source_kind"] == "action" {
			actions++
		}
		if e["source_kind"] == "action_capture" {
			captures++
			conformanceEqual(t, e, "capture/complete", true)
			conformanceEqual(t, e, "capture/action_evidence_ids", []any{})
		}
	}
	if captures != 1 || actions != 0 {
		t.Fatalf("capture/action evidence %d/%d", captures, actions)
	}
	missing := conformanceRoundTrip(t, v, "two_deliveries_no_capture")
	conformanceEqual(t, missing, root+"denominator/delivered_targets/count", float64(2))
	conformanceEqual(t, missing, root+"denominator/delivery/state", "validated")
	for _, e := range missing["evidence"].([]any) {
		if e.(map[string]any)["source_kind"] == "action_capture" {
			t.Fatal("invented capture evidence")
		}
	}
	conformanceEqual(t, missing, root+"w5/state", "not_measured")
	conformanceEqual(t, missing, root+"w5/cause", "unavailable")
	conformanceEqual(t, missing, root+"w5/denominator/count", float64(2))
	conformanceEqual(t, missing, root+"acted/state", "unavailable")
	conformanceEqual(t, missing, root+"action_capture/state", "unavailable")
	conformanceEqual(t, missing, root+"action_capture/evidence_ids", []any{})
	for _, key := range []string{"set_digest", "classifier_digest", "validation_reading_digest"} {
		conformanceAbsent(t, missing, root+"action_capture", key)
	}
	conformanceAbsent(t, missing, root+"w5", "numerator")
	for _, name := range []string{"classifier_counts_only", "classifier_failed_validation"} {
		data := conformanceRoundTrip(t, v, name)
		conformanceEqual(t, data, root+"eligible/count", float64(2))
		conformanceEqual(t, data, root+"successful_calls/count", float64(1))
		conformanceEqual(t, data, root+"denominator/eligibility/state", "unvalidated")
		conformanceEqual(t, data, root+"w4/state", "not_measured")
		conformanceEqual(t, data, root+"w4/cause", "unvalidated")
		conformanceEqual(t, data, root+"w5/state", "measured")
		conformanceAbsent(t, data, root+"w4", "numerator")
		conformanceAbsent(t, data, root+"w4", "value")
		if name == "classifier_counts_only" {
			conformanceAbsent(t, data, root+"denominator/eligibility", "validation_reading_digest")
		} else {
			for _, key := range []string{"classifier_digest", "validation_reading_digest"} {
				conformanceEqual(t, data, root+"denominator/eligibility/"+key, strings.Repeat("a", 64))
			}
		}
	}
	unknown := conformanceRoundTrip(t, v, "unknown_denominator")
	conformanceEqual(t, unknown, root+"eligible/state", "unavailable")
	conformanceEqual(t, unknown, root+"w4/cause", "unavailable")
	conformanceEqual(t, unknown, root+"denominator/eligible_opportunities/state", "unavailable")
	conformanceEqual(t, unknown, root+"w4/denominator/state", "unavailable")
	conformanceAbsent(t, unknown, root+"denominator/eligible_opportunities", "count")
	conformanceAbsent(t, unknown, root+"w4/denominator", "count")
	conformanceAbsent(t, unknown, root+"eligible", "count")
	conformanceAbsent(t, unknown, root+"w4", "numerator")
	for _, k := range []int{1, 3, 0, -1} {
		data := conformancePositive(t, v, "zero_calls")
		data["k"] = k
		err := conformanceCheck(t, v, "#Outcome", data).Validate(cue.Concrete(true))
		if (k > 0) != (err == nil) {
			t.Fatal(fmt.Sprintf("k=%d: %v", k, err))
		}
	}
}
