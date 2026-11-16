package records

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestLaneHeadSchemas tests parsed shapes only. Calendar validity, terminal
// refusal policy and condition target semantics belong to later Go consumers.
func TestLaneHeadSchemas(t *testing.T) {
	root := filepath.Join("..", "..", "schema", "records")
	paths, err := filepath.Glob(filepath.Join(root, "*.cue"))
	if err != nil {
		t.Fatal(err)
	}
	var schema strings.Builder
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		schema.WriteString(strings.ReplaceAll(string(raw), "package records", ""))
		schema.WriteByte('\n')
	}
	for _, kind := range []string{"decision", "invariant", "goal", "future", "question", "incident", "observation", "ask"} {
		t.Run(kind, func(t *testing.T) {
			ctx := cuecontext.New()
			base := ctx.CompileString(schema.String())
			if err := base.Err(); err != nil {
				t.Fatal(err)
			}
			definition := "#" + strings.ToUpper(kind[:1]) + kind[1:] + "Head"
			shape := base.LookupPath(cue.ParsePath(definition))
			if err := shape.Err(); err != nil {
				t.Fatalf("required head schema %s: %v", definition, err)
			}
			readFixture := func(suffix string) string {
				t.Helper()
				raw, err := os.ReadFile(filepath.Join(root, "testdata", "heads", kind+suffix+".cue"))
				if err != nil {
					t.Fatal(err)
				}
				text := string(raw)
				if suffix == "-invalid" {
					var lines []string
					for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
						if !strings.HasPrefix(line, "//") {
							t.Fatalf("active negative fixture line: %q", line)
						}
						lines = append(lines, strings.TrimPrefix(strings.TrimPrefix(line, "//"), " "))
					}
					// First line describes the failure rather than being CUE source.
					text = strings.Join(lines[1:], "\n")
				}
				return strings.ReplaceAll(text, "package records", "")
			}
			valid := ctx.CompileString(schema.String() + "\n" + readFixture("-valid")).LookupPath(cue.ParsePath(kind))
			if err := valid.Validate(cue.Concrete(true)); err != nil {
				t.Fatal(err)
			}
			raw, err := valid.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if err := shape.Subsume(ctx.CompileBytes(raw), cue.Final()); err != nil {
				t.Fatalf("positive fixture not admitted: %v", err)
			}
			var original map[string]any
			if err := json.Unmarshal(raw, &original); err != nil {
				t.Fatal(err)
			}
			// Every positive field, including optional nested fields and lists, survives
			// JSON -> actual CUE definition -> JSON without defaults or dropped labels.
			roundTrip := shape.Unify(ctx.CompileBytes(raw))
			got, err := roundTrip.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(got, &decoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original, decoded) {
				t.Fatalf("round trip changed fields: %s -> %s", raw, got)
			}
			accept := func(name string, mutate func(map[string]any)) {
				t.Helper()
				t.Run(name, func(t *testing.T) {
					var fields map[string]any
					if err := json.Unmarshal(raw, &fields); err != nil {
						t.Fatal(err)
					}
					mutate(fields)
					payload, err := json.Marshal(fields)
					if err != nil {
						t.Fatal(err)
					}
					if err := shape.Subsume(ctx.CompileBytes(payload), cue.Final()); err != nil {
						t.Fatalf("valid shape rejected: %s: %v", payload, err)
					}
				})
			}
			accept("empty-optional-lists", func(m map[string]any) {
				m["conditions"] = []any{}
				for _, field := range []string{"supersedes", "enforced_by", "outcome"} {
					if _, ok := m[field]; ok {
						m[field] = []any{}
					}
				}
				for _, field := range []string{"value", "basis", "verdict"} {
					delete(m, field)
				}
			})
			for _, enum := range []struct {
				field  string
				values []string
			}{
				{"ruling", []string{"proposed", "accepted", "rejected", "deferred", "withdrawn"}},
				{"confidence", []string{"likely", "plausible", "speculative"}},
				{"horizon", []string{"next", "later", "someday"}},
				{"verdict", []string{"fix", "wontfix", "duplicate", "ceiling"}},
				{"review", []string{"never", "2027-02-31"}},
			} {
				if _, ok := original[enum.field]; ok {
					for _, value := range enum.values {
						accept(enum.field+"-"+value, func(m map[string]any) { m[enum.field] = value })
					}
				}
			}
			// Shape accepts these combinations; later Go semantic checks must
			// enforce real dates, terminal refusals and target/want policy.
			for _, label := range []string{"Done when", "Promote when", "Drop if", "Revisit when", "If false"} {
				for _, conditionKind := range []string{"date", "record", "reading", "manual"} {
					accept(label+"-"+conditionKind, func(m map[string]any) {
						c := m["conditions"].([]any)[0].(map[string]any)
						c["label"], c["kind"], c["review_by"] = label, conditionKind, "never"
						delete(c, "target")
						delete(c, "want")
					})
				}
			}
			negative := ctx.CompileString(schema.String() + "\n" + readFixture("-invalid"))
			if negative.Validate(cue.Concrete(true)) == nil {
				t.Fatal("negative fixture accepted")
			}
			reject := func(name string, mutate func(map[string]any)) {
				t.Helper()
				t.Run(name, func(t *testing.T) {
					var fields map[string]any
					if err := json.Unmarshal(raw, &fields); err != nil {
						t.Fatal(err)
					}
					mutate(fields)
					payload, err := json.Marshal(fields)
					if err != nil {
						t.Fatal(err)
					}
					// Subsumption checks supplied fields as well as values. Unification
					// alone can supply a missing singleton kind or empty list.
					if err := shape.Subsume(ctx.CompileBytes(payload), cue.Final()); err == nil {
						t.Fatalf("invalid head accepted: %s", payload)
					}
				})
			}
			// All populated head fields are required except the contract's explicit
			// optional Future value/basis and Observation verdict.
			for _, field := range []string{"kind", "id", "title", "statement", "ruling", "supersedes", "enforced_by", "bet", "instrument", "threshold", "review", "confidence", "horizon", "outcome", "conditions"} {
				if _, ok := original[field]; ok {
					reject("missing-"+field, func(m map[string]any) { delete(m, field) })
				}
			}
			for _, field := range []string{"phase", "state", "condition_verdict", "unknown_label"} {
				reject("derived-"+field, func(m map[string]any) { m[field] = "holds" })
			}
			reject("wrong-kind", func(m map[string]any) { m["kind"] = "lane" })
			reject("empty-id", func(m map[string]any) { m["id"] = "" })
			if kind != "ask" {
				reject("wrong-id-family", func(m map[string]any) { m["id"] = "OTHER-1" })
			}
			for _, field := range []string{"ruling", "confidence", "horizon", "verdict", "review"} {
				if _, ok := original[field]; ok {
					reject("bad-"+field, func(m map[string]any) { m[field] = "invalid" })
				}
			}
			for _, field := range []string{"label", "kind", "review_by", "text", "state", "verdict", "phase"} {
				reject("condition-"+field, func(m map[string]any) {
					value := "invalid"
					if field == "text" {
						value = ""
					}
					m["conditions"].([]any)[0].(map[string]any)[field] = value
				})
			}
			for _, field := range []string{"label", "kind", "text", "review_by"} {
				reject("condition-missing-"+field, func(m map[string]any) { delete(m["conditions"].([]any)[0].(map[string]any), field) })
			}
			if kind == "future" {
				for _, pair := range [][2]string{{"value", "level"}, {"basis", "kind"}} {
					reject("bad-"+pair[0], func(m map[string]any) { m[pair[0]].(map[string]any)[pair[1]] = "invalid" })
				}
			}
		})
	}
}
