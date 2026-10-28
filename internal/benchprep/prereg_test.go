package benchprep

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const preregFixture = "testdata/prereg/localization-v1.json"

func TestLoadPrereg_ReadsAValidPreregAndHashesItsBytes(t *testing.T) {
	b, err := os.ReadFile(preregFixture)
	if err != nil {
		t.Fatal(err)
	}
	p, sha, err := LoadPrereg(preregFixture)
	if err != nil {
		t.Fatalf("LoadPrereg: %v", err)
	}
	sum := sha256.Sum256(b)
	if want := hex.EncodeToString(sum[:]); sha != want {
		t.Errorf("sha = %s, want the sha256 of the file's bytes %s", sha, want)
	}
	if p.ID != "localization-v1" || p.BaselineArm != "A1" || len(p.Strata) != 2 || len(p.Arms) != 2 {
		t.Errorf("decoded header wrong: %+v", p)
	}
	if len(p.Hypotheses) != 1 || p.Hypotheses[0].ID != "H4" || p.Hypotheses[0].PrimaryMetric != "hit@3" || p.Hypotheses[0].N != 120 {
		t.Fatalf("decoded hypotheses wrong: %+v", p.Hypotheses)
	}
	if got := p.Hypotheses[0].Predictions[1]; got.Direction != "null" || got.EquivalenceMargin != 0.05 {
		t.Errorf("predicted null decoded wrong: %+v", got)
	}
	if p.MultiplicityCorrection.Method != "holm" || p.ThreeOutcomeRule.PredictedNull.Equivalent == "" || p.TurnCap != 30 || p.Seed != 42 {
		t.Errorf("decoded rules wrong: %+v", p)
	}
	if p.DeviationLog == nil || len(p.DeviationLog) != 0 || len(p.Exclusions) != 1 {
		t.Errorf("exclusions and an empty deviation log must both decode: %+v %+v", p.Exclusions, p.DeviationLog)
	}
}

// The committed loader fixture is a copy of schema/bench/testdata/valid.cue's
// prereg value; this keeps the two from drifting apart.
func TestLoadPrereg_FixtureMirrorsTheSchemaPositive(t *testing.T) {
	want := preregSchemaPositive(t)
	var got map[string]any
	b, err := os.ReadFile(preregFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preregRoundTrip(t, want), preregRoundTrip(t, got)) {
		t.Fatalf("%s no longer mirrors valid.cue's prereg", preregFixture)
	}
}

func TestLoadPrereg_RefusesSchemaNegatives(t *testing.T) {
	cases := preregSchemaNegatives(t)
	if len(cases) == 0 {
		t.Fatal("invalid.cue has no prereg negatives")
	}
	cases = append(cases, preregNegative{Name: "unknown top-level field", Path: "verdict", Value: "confirmed"})
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			data := preregFixtureData(t)
			preregPatch(t, data, tc)
			id := "localization-v1"
			if s, ok := data["id"].(string); ok && s != "" && !strings.ContainsAny(s, `/\`) {
				id = s
			}
			path := preregWrite(t, id, data)
			_, _, err := LoadPrereg(path)
			if err == nil {
				t.Fatal("LoadPrereg accepted a schema negative")
			}
			if strings.Contains(err.Error(), "named for") {
				t.Fatalf("the schema, not the file name, must refuse this case: %v", err)
			}
		})
	}
}

func TestLoadPrereg_RefusesAFileNotNamedForItsID(t *testing.T) {
	path := preregWrite(t, "localization-v2", preregFixtureData(t))
	_, _, err := LoadPrereg(path)
	if err == nil {
		t.Fatal("a prereg whose file name is not <id>.json must be refused (D1)")
	}
	if !strings.Contains(err.Error(), "localization-v1.json") {
		t.Errorf("refusal should name the expected file name: %v", err)
	}
}

func TestLoadPrereg_RefusesNonJSONAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	cueSyntax := filepath.Join(dir, "localization-v1.json")
	if err := os.WriteFile(cueSyntax, []byte("id: \"localization-v1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadPrereg(cueSyntax); err == nil || !strings.Contains(err.Error(), "not JSON") {
		t.Errorf("CUE syntax in a .json prereg must be refused as not JSON, got %v", err)
	}
	if _, _, err := LoadPrereg(filepath.Join(dir, "absent.json")); err == nil {
		t.Error("a missing prereg must be an error")
	}
}

type preregNegative struct {
	Name    string `json:"name"`
	Fixture string `json:"fixture"`
	Path    string `json:"path"`
	Value   any    `json:"value"`
	Remove  bool   `json:"remove"`
}

// preregSchemaNegatives reads every "fixture": "prereg" case from the schema's
// commented negative list, the way schema/bench's TestBenchSchemaNegatives does.
func preregSchemaNegatives(t *testing.T) []preregNegative {
	t.Helper()
	b, err := os.ReadFile("../../schema/bench/testdata/invalid.cue")
	if err != nil {
		t.Fatal(err)
	}
	_, commented, ok := strings.Cut(string(b), "// cases: [")
	if !ok {
		t.Fatal("invalid.cue has no cases list")
	}
	var source strings.Builder
	source.WriteString("cases: [\n")
	for _, line := range strings.Split(commented, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "// ") {
			t.Fatal("invalid.cue contains an uncommented negative")
		}
		source.WriteString(strings.TrimPrefix(line, "// ") + "\n")
	}
	var all []preregNegative
	if err := cuecontext.New().CompileString(source.String()).LookupPath(cue.ParsePath("cases")).Decode(&all); err != nil {
		t.Fatal(err)
	}
	var out []preregNegative
	for _, c := range all {
		if c.Fixture == "prereg" {
			out = append(out, c)
		}
	}
	return out
}

func preregSchemaPositive(t *testing.T) map[string]any {
	t.Helper()
	schema, err := os.ReadFile("../../schema/bench/schema.cue")
	if err != nil {
		t.Fatal(err)
	}
	positive, err := os.ReadFile("../../schema/bench/testdata/valid.cue")
	if err != nil {
		t.Fatal(err)
	}
	v := cuecontext.New().CompileString(string(schema) + "\n" + strings.Replace(string(positive), "package bench", "", 1))
	var data map[string]any
	if err := v.LookupPath(cue.ParsePath("prereg")).Decode(&data); err != nil {
		t.Fatal(err)
	}
	return data
}

func preregRoundTrip(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func preregFixtureData(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(preregFixture)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func preregPatch(t *testing.T, data map[string]any, tc preregNegative) {
	t.Helper()
	var node any = data
	parts := strings.Split(tc.Path, ".")
	for _, key := range parts[:len(parts)-1] {
		if list, ok := node.([]any); ok {
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(list) {
				t.Fatalf("invalid fixture index %q in %s", key, tc.Path)
			}
			node = list[i]
		} else {
			node = node.(map[string]any)[key]
		}
	}
	obj := node.(map[string]any)
	if key := parts[len(parts)-1]; tc.Remove {
		delete(obj, key)
	} else {
		obj[key] = tc.Value
	}
}

func preregWrite(t *testing.T, id string, data map[string]any) string {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), id+".json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
