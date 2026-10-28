package labs_test

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
)

func fixture(t *testing.T, name string) (cue.Value, map[string]any) {
	t.Helper()
	schema, err := os.ReadFile("schema.cue")
	if err != nil {
		t.Fatal(err)
	}
	positive, err := os.ReadFile("testdata/valid.cue")
	if err != nil {
		t.Fatal(err)
	}
	ctx := cuecontext.New()
	v := ctx.CompileString(string(schema) + "\n" + strings.Replace(string(positive), "package labs", "", 1))
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("positive fixtures: %v", err)
	}
	var data map[string]any
	value := v.LookupPath(cue.ParsePath(name))
	if !value.Exists() {
		t.Fatalf("missing positive fixture %s", name)
	}
	if err := value.Decode(&data); err != nil {
		t.Fatal(err)
	}
	definition := map[string]string{"lab": "#Lab", "scenario": "#Case", "result": "#Result", "labNA": "#Lab", "scenarioRedacted": "#Case", "resultDecision": "#Result", "resultDecisionOther": "#Result", "resultPlugin": "#Result", "resultReplay": "#Result"}[name]
	return v.LookupPath(cue.ParsePath(definition)), data
}

func TestLabSchemaValid(t *testing.T) {
	for _, name := range []string{"lab", "scenario", "result", "labNA", "scenarioRedacted", "resultDecision", "resultDecisionOther", "resultPlugin", "resultReplay"} {
		t.Run(name, func(t *testing.T) { fixture(t, name) })
	}
}

// The kit has a nested CUE module, unavailable to imports from the parent
// module without a dependency mapping. Keep the local mirror field-for-field.
func TestLabScenarioMirror(t *testing.T) {
	local, err := os.ReadFile("schema.cue")
	if err != nil {
		t.Fatal(err)
	}
	upstream, err := os.ReadFile("../../third_party/agentic-kit/foundations/quality/scenario-bank/schema.cue")
	if err != nil {
		t.Fatal(err)
	}
	ctx := cuecontext.New()
	for _, name := range []string{"#Provenance", "#PredicateKind", "#Criterion", "#Rubric", "#Scenario"} {
		t.Run(name, func(t *testing.T) {
			a := ctx.CompileBytes(local).LookupPath(cue.ParsePath(name))
			b := ctx.CompileBytes(upstream).LookupPath(cue.ParsePath(name))
			if err := a.Err(); err != nil {
				t.Fatal(err)
			}
			if err := b.Err(); err != nil {
				t.Fatal(err)
			}
			ab, err := format.Node(a.Syntax(cue.Raw(), cue.All()))
			if err != nil {
				t.Fatal(err)
			}
			bb, err := format.Node(b.Syntax(cue.Raw(), cue.All()))
			if err != nil {
				t.Fatal(err)
			}
			if string(ab) != string(bb) {
				t.Fatalf("%s differs from kit: %s != %s", name, ab, bb)
			}
		})
	}
}

func TestLabSchemaNegatives(t *testing.T) {
	casesFile, err := os.ReadFile("testdata/invalid.cue")
	if err != nil {
		t.Fatal(err)
	}
	_, commented, ok := strings.Cut(string(casesFile), "// cases: [")
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
	var cases []struct {
		Name    string `json:"name"`
		Fixture string `json:"fixture"`
		Path    string `json:"path"`
		Value   any    `json:"value"`
		Remove  bool   `json:"remove"`
	}
	v := cuecontext.New().CompileString(source.String())
	if err := v.LookupPath(cue.ParsePath("cases")).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("invalid.cue has no negative examples")
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			def, data := fixture(t, tc.Fixture)
			var node any = data
			parts := strings.Split(tc.Path, ".")
			for _, key := range parts[:len(parts)-1] {
				if list, ok := node.([]any); ok {
					index, err := strconv.Atoi(key)
					if err != nil || index < 0 || index >= len(list) {
						t.Fatalf("invalid fixture index: %s", key)
					}
					node = list[index]
				} else {
					node = node.(map[string]any)[key]
				}
			}
			obj := node.(map[string]any)
			key := parts[len(parts)-1]
			if tc.Remove {
				delete(obj, key)
			} else {
				obj[key] = tc.Value
			}
			b, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			err = def.Unify(def.Context().CompileBytes(b)).Validate(cue.Concrete(true))
			if err == nil {
				t.Fatal("invalid value accepted")
			}
			// The diagnostic must identify the field under test, rather than
			// rejecting an unrelated incomplete field in the positive fixture.
			if key != "provenance" && !strings.Contains(errors.Details(err, nil), key) {
				t.Fatalf("diagnostic does not name %s: %v", key, err)
			}
		})
	}
}
