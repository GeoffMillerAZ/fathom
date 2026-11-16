package benchenv

import (
	"bytes"
	"crypto/sha256"
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestValidateEnvironmentRecords_UnknownField(t *testing.T) {
	result, err := ValidateEnvironmentRecords([]byte(`{"environments":[],"requirements":[],"witnesses":[],"coverage":[],"pass":false}`))
	var refusal *ValidationError
	if !errors.As(err, &refusal) || refusal.Path != "/pass" || refusal.Reason != ValidationShape {
		t.Fatalf("want shape /pass, got %v", err)
	}
	b, _ := json.Marshal(result)
	if string(b) != `{"environments":[],"requirements":[],"witnesses":[],"coverage":[]}` {
		t.Fatalf("failure result: %s", b)
	}
}

// The stored fixture is evaluated with the same embedded schema. Cache bytes,
// never mutable decoded maps shared between mutation tests.
var fixtureOnce sync.Once
var fixtureBytes map[string][]byte
var fixtureError error

func fixtureSource(t *testing.T, name string) []byte {
	t.Helper()
	fixtureOnce.Do(func() {
		b, e := os.ReadFile("testdata/valid.cue")
		if e != nil {
			fixtureError = e
			return
		}
		v := cuecontext.New().CompileString(schemaSource + "\n" + strings.Replace(string(b), "package benchenv", "", 1))
		if e = v.Validate(cue.Concrete(true)); e != nil {
			fixtureError = e
			return
		}
		fixtureBytes = map[string][]byte{}
		it, e := v.Fields()
		if e != nil {
			fixtureError = e
			return
		}
		for it.Next() {
			b, e := it.Value().MarshalJSON()
			if e != nil {
				fixtureError = e
				return
			}
			fixtureBytes[it.Selector().Unquoted()] = b
		}
	})
	if fixtureError != nil {
		t.Fatal(fixtureError)
	}
	b, ok := fixtureBytes[name]
	if !ok {
		t.Fatalf("missing fixture %s", name)
	}
	return bytes.Clone(b)
}
func object(t *testing.T, name string) map[string]any {
	t.Helper()
	var m map[string]any
	d := json.NewDecoder(bytes.NewReader(fixtureSource(t, name)))
	d.UseNumber()
	if e := d.Decode(&m); e != nil {
		t.Fatal(e)
	}
	return m
}
func encoded(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func edit(m map[string]any, p string, value any, remove bool) {
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	var node any = m
	for _, k := range parts[:len(parts)-1] {
		k = strings.ReplaceAll(strings.ReplaceAll(k, "~1", "/"), "~0", "~")
		switch x := node.(type) {
		case map[string]any:
			node = x[k]
		case []any:
			i, _ := strconv.Atoi(k)
			node = x[i]
		}
	}
	k := parts[len(parts)-1]
	k = strings.ReplaceAll(strings.ReplaceAll(k, "~1", "/"), "~0", "~")
	switch x := node.(type) {
	case map[string]any:
		if remove {
			delete(x, k)
		} else {
			x[k] = value
		}
	case []any:
		i, _ := strconv.Atoi(k)
		x[i] = value
	}
}
func assertValidation(t *testing.T, err error, reason ValidationReason, p string) {
	t.Helper()
	var e *ValidationError
	if !errors.As(err, &e) || e.Path != p || e.Reason != reason {
		t.Fatalf("want %s %s, got %#v", reason, p, err)
	}
	if e.Error() != "environment: "+string(reason) {
		t.Fatalf("unsafe diagnostic %q", e.Error())
	}
}

const emptyRecordsJSON = `{"environments":[],"requirements":[],"witnesses":[],"coverage":[]}`
const emptyInventoryJSON = `{"concepts":{},"groups":[]}`
const emptyFleetJSON = `{"seed":0,"generator_version":"","template":"","size":{"lines":0,"files":0,"repositories":0,"history":0},"repositories":[],"truth":[]}`

func recordMutation(t *testing.T, p string, value any, remove bool, reason ValidationReason) {
	t.Helper()
	m := object(t, "records")
	edit(m, p, value, remove)
	b := encoded(t, m)
	r, e := ValidateEnvironmentRecords(b)
	if got := string(encoded(t, r)); got != emptyRecordsJSON {
		t.Fatalf("failure result: %s", got)
	}
	assertValidation(t, e, reason, p)
	if !bytes.Equal(b, encoded(t, m)) {
		t.Fatal("input mutated")
	}
}
func TestValidateEnvironmentRecords_Valid(t *testing.T) {
	for _, name := range []string{"records", "unknownCounts", "constructed"} {
		t.Run(name, func(t *testing.T) {
			r, e := ValidateEnvironmentRecords(fixtureSource(t, name))
			if e != nil {
				t.Fatal(e)
			}
			if len(r.Witnesses) != 120 || len(r.Environments) != 2 {
				t.Fatal("fixture cardinality")
			}
			if name == "unknownCounts" && r.Environments[0].Scale.Lines != nil {
				t.Fatal("unknown became zero")
			}
			var want any
			_ = json.Unmarshal(fixtureSource(t, name), &want)
			var got any
			_ = json.Unmarshal(encoded(t, r), &got)
			if !reflect.DeepEqual(want, got) {
				t.Fatal("wire fields changed")
			}
		})
	}
	m := object(t, "records")
	edit(m, "/environments/0/scale/lines", 0, false)
	r, e := ValidateEnvironmentRecords(encoded(t, m))
	if e != nil || r.Environments[0].Scale.Lines == nil || *r.Environments[0].Scale.Lines != 0 {
		t.Fatalf("known zero: %v", e)
	}
}
func walkObjects(v any, p string, visit func(map[string]any, string)) {
	switch x := v.(type) {
	case map[string]any:
		visit(x, p)
		for _, k := range keys(x) {
			walkObjects(x[k], pointer(p, k), visit)
		}
	case []any:
		for i, v := range x {
			walkObjects(v, indexed(p, i), visit)
		}
	}
}
func testRecordsRequiredFields(t *testing.T) {
	// Each source instance is checked with the exact declaration walker. Public
	// source validation is also exercised once for every distinct descendant shape.
	m := object(t, "records")
	seen := map[string]bool{}
	walkObjects(m, "", func(obj map[string]any, p string) {
		ks := keys(obj)
		signature := strings.Join(ks, ",")
		for _, k := range ks {
			q := pointer(p, k)
			old := obj[k]
			delete(obj, k)
			es := []*ValidationError{}
			shape(m, reflect.TypeFor[EnvironmentRecords](), "", &es)
			assertValidation(t, first(es), ValidationShape, q)
			obj[k] = old
		}
		if seen[signature] {
			return
		}
		seen[signature] = true
		for _, k := range ks {
			q := pointer(p, k)
			t.Run(q, func(t *testing.T) { recordMutation(t, q, nil, true, ValidationShape) })
		}
	})
}
func TestValidateEnvironmentRecords_NoAuthoredPass(t *testing.T) {
	m := object(t, "records")
	seen := map[string]bool{}
	walkObjects(m, "", func(obj map[string]any, p string) {
		sig := strings.Join(keys(obj), ",")
		if seen[sig] {
			return
		}
		seen[sig] = true
		for _, k := range []string{"status", "pass", "admitted", "adequate", "valid", "fresh", "unknown"} {
			for _, b := range []bool{false, true} {
				q := pointer(p, k)
				t.Run(q+strconv.FormatBool(b), func(t *testing.T) { recordMutation(t, q, b, false, ValidationShape) })
			}
		}
	})
	recordMutation(t, "/x~1y~0z", false, false, ValidationShape)
}
func testRecordsNumericAndEnumShape(t *testing.T) {
	tests := map[string][]any{
		"/environments/0/tier": {"other"}, "/environments/0/split": {"other"}, "/requirements/0/stratum/scale/0/axis": {"other"}, "/requirements/0/stratum/scale/0/class": {"other"}, "/requirements/0/stratum/prediction": {"other"}, "/witnesses/0/polarity": {"other"}, "/environments/0/scale/excluded/0/kind": {"other"},
		"/requirements/0/positive_minimum": {29, -1, 30.5, json.Number("2147483648")}, "/requirements/0/negative_minimum": {29, -1, 30.5, json.Number("2147483648")},
		"/environments/0/scale/lines": {-1, 0.5, json.Number("9223372036854775808")}, "/environments/0/scale/excluded/0/files": {-1, 0.5, json.Number("9223372036854775808")},
		"/witnesses/0/anchor/start_line": {0, 0.5, json.Number("2147483648")}, "/witnesses/0/anchor/end_line": {0, 0.5, json.Number("2147483648")}, "/requirements/0/checker/version": {"", " "},
	}
	for p, vs := range tests {
		for i, v := range vs {
			t.Run(p+strconv.Itoa(i), func(t *testing.T) { recordMutation(t, p, v, false, ValidationShape) })
		}
	}
	m := object(t, "records")
	edit(m, "/witnesses/0/anchor/start_line", 4, false)
	r, e := ValidateEnvironmentRecords(encoded(t, m))
	assertValidation(t, e, ValidationShape, "/witnesses/0/anchor/end_line")
	if string(encoded(t, r)) != emptyRecordsJSON {
		t.Fatal("failure result")
	}
	for _, src := range []string{"{", `a: string`} {
		r, e := ValidateEnvironmentRecords([]byte(src))
		assertValidation(t, e, ValidationShape, "")
		if string(encoded(t, r)) != emptyRecordsJSON {
			t.Fatal("failure result")
		}
	}
}
func testRecordsScalarShape(t *testing.T) {
	m := object(t, "records")
	seen := map[string]bool{}
	walkObjects(m, "", func(obj map[string]any, p string) {
		for _, k := range keys(obj) {
			v := obj[k]
			q := pointer(p, k)
			switch v.(type) {
			case string:
				old := obj[k]
				if k != "url" && k != "subtype" {
					for _, s := range []string{"", " "} {
						obj[k] = s
						es := []*ValidationError{}
						shape(m, reflect.TypeFor[EnvironmentRecords](), "", &es)
						assertValidation(t, first(es), ValidationShape, q)
					}
				}
				obj[k] = old
				var vs []any
				if k == "commit" {
					vs = []any{strings.Repeat("a", 39), strings.Repeat("a", 41), strings.Repeat("g", 40), strings.Repeat("A", 40)}
				}
				if k == "sha256" {
					vs = []any{strings.Repeat("a", 63), strings.Repeat("a", 65), strings.Repeat("g", 64), strings.Repeat("A", 64)}
				}
				for _, s := range vs {
					obj[k] = s
					es := []*ValidationError{}
					shape(m, reflect.TypeFor[EnvironmentRecords](), "", &es)
					assertValidation(t, first(es), ValidationShape, q)
				}
				obj[k] = old
				if len(vs) > 0 && !seen[k] {
					seen[k] = true
					for _, s := range vs {
						recordMutation(t, q, s, false, ValidationShape)
					}
				}
			case []any:
				if !seen[q] {
					for _, s := range []any{nil, "scalar"} {
						recordMutation(t, q, s, false, ValidationShape)
					}
					seen[q] = true
				}
			}
		}
	})
	for _, p := range []string{"/environments/0/repositories", "/environments/0/toolchains", "/environments/0/oracles", "/environments/0/receipts", "/requirements/0/stratum/scale", "/coverage/0/requirements"} {
		recordMutation(t, p, []any{}, false, ValidationShape)
	}
}

func TestValidateEnvironmentRecords_References(t *testing.T) {
	tests := []struct {
		p string
		v any
	}{
		{"/witnesses/0/environment/id", "missing-e"}, {"/witnesses/0/requirement/id", "missing-q"}, {"/coverage/0/environment/id", "missing-e"}, {"/coverage/0/requirements/0/id", "missing-q"},
		{"/witnesses/0/anchor/repository/id", "r-missing"}, {"/witnesses/0/anchor/repository/commit", strings.Repeat("c", 40)},
		{"/witnesses/0/checker/name", "other"}, {"/witnesses/0/checker/version", "2"}, {"/witnesses/0/checker/sha256", strings.Repeat("3", 64)},
		{"/coverage/0/stratum/language", "python"}, {"/coverage/0/stratum/prediction", "null"}, {"/coverage/0/stratum/subtype", "long"},
		{"/requirements/0/card/id", "different-fixture-card"}, {"/coverage/0/stratum/scale/0/class", "M"}, {"/coverage/0/capability", "different-fixture-card"},
	}
	for _, tc := range tests {
		t.Run(tc.p, func(t *testing.T) { recordMutation(t, tc.p, tc.v, false, ValidationReference) })
	}
	duplicates := []struct {
		list  string
		idx   int
		field string
	}{
		{"/environments", 2, "id"}, {"/requirements", 1, "id"}, {"/witnesses", 120, "id"}, {"/coverage", 2, "id"},
		{"/environments/0/repositories", 1, "id"}, {"/coverage/0/requirements", 1, "id"}, {"/requirements/0/stratum/scale", 1, "axis"}, {"/environments/0/scale/excluded", 4, "kind"},
	}
	for _, tc := range duplicates {
		t.Run(tc.list, func(t *testing.T) {
			m := object(t, "records")
			a := at(m, tc.list).([]any)
			edit(m, tc.list, append(a, a[0]), false)
			r, e := ValidateEnvironmentRecords(encoded(t, m))
			assertValidation(t, e, ValidationIdentity, indexed(tc.list, tc.idx)+"/"+tc.field)
			if string(encoded(t, r)) != emptyRecordsJSON {
				t.Fatal("failure result")
			}
		})
	}
	m := object(t, "records")
	xs := at(m, "/environments/0/scale/excluded").([]any)
	edit(m, "/environments/0/scale/excluded", xs[1:], false)
	_, e := ValidateEnvironmentRecords(encoded(t, m))
	assertValidation(t, e, ValidationShape, "/environments/0/scale/excluded")
	edit(m, "/environments/0/scale/excluded", xs, false)
	edit(m, "/environments/0/receipts/0/sha256", strings.Repeat("f", 64), false)
	if _, e = ValidateEnvironmentRecords(encoded(t, m)); e != nil {
		t.Fatalf("external bytes must remain unchecked: %v", e)
	}
}
func at(m map[string]any, p string) any {
	var x any = m
	for _, k := range strings.Split(strings.TrimPrefix(p, "/"), "/") {
		k = strings.ReplaceAll(strings.ReplaceAll(k, "~1", "/"), "~0", "~")
		switch v := x.(type) {
		case map[string]any:
			x = v[k]
		case []any:
			i, _ := strconv.Atoi(k)
			x = v[i]
		}
	}
	return x
}

var badPaths = []string{"/x", "../x", "a/../x", "./x", "a/./x", "a//b", "a/", "a\\b", "a\x00b", "a\nb", "a:b"}

func TestValidateEnvironmentRecords_Paths(t *testing.T) {
	for _, p := range []string{"/requirements/0/card/path", "/witnesses/0/anchor/path", "/environments/0/licence/path"} {
		for i, s := range badPaths {
			t.Run(p+strconv.Itoa(i), func(t *testing.T) { recordMutation(t, p, s, false, ValidationPath) })
		}
	}
	for _, s := range []string{"https://user:secret@example.invalid/a", "https://example.invalid/a?secret=1", "https://example.invalid/a#fragment", "http://example.invalid/a", ""} {
		recordMutation(t, "/environments/0/repositories/0/url", s, false, ValidationPath)
	}
	m := object(t, "constructed")
	edit(m, "/environments/0/repositories/0/url", "https://example.invalid/a", false)
	_, e := ValidateEnvironmentRecords(encoded(t, m))
	assertValidation(t, e, ValidationPath, "/environments/0/repositories/0/url")
}
func TestValidateEnvironmentRecords_PriorRead(t *testing.T) {
	for _, changed := range []bool{false, true} {
		m := object(t, "records")
		edit(m, "/environments/0/prior_reads", []any{at(m, "/environments/0/receipts/0")}, false)
		if _, e := ValidateEnvironmentRecords(encoded(t, m)); e != nil {
			t.Fatal(e)
		}
		edit(m, "/environments/0/split", "held-out", false)
		if changed {
			edit(m, "/environments/0/repositories/0/commit", strings.Repeat("c", 40), false)
		}
		r, e := ValidateEnvironmentRecords(encoded(t, m))
		assertValidation(t, e, ValidationReference, "/environments/0/prior_reads")
		if string(encoded(t, r)) != emptyRecordsJSON {
			t.Fatal("failure result")
		}
	}
}
func TestValidateEnvironmentRecords_ErrorOrder(t *testing.T) {
	m := object(t, "records")
	edit(m, "/witnesses/0/anchor/path", "../x", false)
	edit(m, "/witnesses/0/checker/version", "different", false)
	for _, z := range []bool{false, true, false} {
		if z {
			m["z"] = false
		} else {
			delete(m, "z")
		}
		_, e := ValidateEnvironmentRecords(encoded(t, m))
		if z {
			assertValidation(t, e, ValidationShape, "/z")
		} else {
			assertValidation(t, e, ValidationPath, "/witnesses/0/anchor/path")
		}
	}
	m = object(t, "records")
	delete(m, "coverage")
	delete(m, "environments")
	_, e := ValidateEnvironmentRecords(encoded(t, m))
	assertValidation(t, e, ValidationShape, "/coverage")
}
func TestValidateEnvironmentRecords_T3Metadata(t *testing.T) {
	for _, p := range []string{"/requirements/0/card", "/requirements/0/checker_contract", "/requirements/0/stratum/subtype"} {
		recordMutation(t, p, nil, true, ValidationShape)
	}
	recordMutation(t, "/requirements/0/card/id", "wrong-card", false, ValidationReference)
	recordMutation(t, "/requirements/0/card/path", "../x", false, ValidationPath)
	recordMutation(t, "/requirements/0/checker_contract/sha256", "bad", false, ValidationShape)
}
func TestValidateEnvironmentRecords_T3Subtype(t *testing.T) {
	recordMutation(t, "/coverage/0/stratum/subtype", "long", false, ValidationReference)
	recordMutation(t, "/requirements/0/stratum/subtype", " ", false, ValidationShape)
	m := object(t, "records")
	edit(m, "/requirements/0/stratum/subtype", "long", false)
	for i := 0; i < 2; i++ {
		edit(m, indexed("/coverage", i)+"/stratum/subtype", "long", false)
	}
	if _, e := ValidateEnvironmentRecords(encoded(t, m)); e != nil {
		t.Fatal(e)
	}
}

func inventory(t *testing.T) ConceptInventory {
	t.Helper()
	r, e := ValidateConceptInventory(fixtureSource(t, "conceptInventory"))
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func records(t *testing.T) EnvironmentRecords {
	t.Helper()
	r, e := ValidateEnvironmentRecords(fixtureSource(t, "records"))
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func conceptMutation(t *testing.T, p string, v any, remove bool, reason ValidationReason) {
	t.Helper()
	m := object(t, "conceptInventory")
	edit(m, p, v, remove)
	b := encoded(t, m)
	r, e := ValidateConceptInventory(b)
	if string(encoded(t, r)) != emptyInventoryJSON {
		t.Fatalf("failure JSON %s", encoded(t, r))
	}
	assertValidation(t, e, reason, p)
	if !bytes.Equal(b, encoded(t, m)) {
		t.Fatal("mutated")
	}
	var typed ConceptInventory
	if json.Unmarshal(b, &typed) == nil {
		var original, projected any
		_ = json.Unmarshal(b, &original)
		_ = json.Unmarshal(encoded(t, typed), &projected)
		if reflect.DeepEqual(original, projected) {
			snapshot := encoded(t, typed)
			assertValidation(t, ValidateConceptInventoryValue(typed), reason, p)
			if !bytes.Equal(snapshot, encoded(t, typed)) {
				t.Fatal("typed input mutated")
			}
		}
	}

}
func TestValidateConceptInventory_Valid(t *testing.T) {
	r := inventory(t)
	if len(r.Concepts) != 1 {
		t.Fatal("lost K")
	}
	for _, cs := range [][]ConceptCitation{{{DesignRow: ConceptDesignBED2}}, {{DesignRow: ConceptDesignBED9}}, {{DesignRow: ConceptDesignBED2}, {DesignRow: ConceptDesignBED9}}} {
		c := r.Concepts["K"]
		c.RequiredBy = cs
		r.Concepts["K"] = c
		got, e := ValidateConceptInventory(encoded(t, r))
		if e != nil {
			t.Fatal(e)
		}
		b := encoded(t, got)
		if bytes.Contains(b, []byte(`"card":null`)) {
			t.Fatal("null optional arm")
		}
	}
	empty, e := ValidateConceptInventory([]byte(emptyInventoryJSON))
	if e != nil || empty.Concepts == nil || empty.Groups == nil || string(encoded(t, empty)) != emptyInventoryJSON {
		t.Fatalf("empty %v", e)
	}
}
func TestValidateConceptInventory_OpenKeys(t *testing.T) {
	r := records(t)
	in := inventory(t)
	c := in.Concepts["K"]
	c.RequiredBy = []ConceptCitation{{DesignRow: ConceptDesignBED9}}
	in.Concepts["L"] = c
	in.Groups[0].Concepts = append(in.Groups[0].Concepts, "L")
	q := r.Requirements[0]
	q.ID = "q-other"
	q.Concept = "L"
	r.Requirements = append(r.Requirements, q)
	for i := range r.Coverage {
		ref := r.Coverage[i].Requirements[0]
		ref.ID = q.ID
		r.Coverage[i].Requirements = append(r.Coverage[i].Requirements, ref)
	}
	if e := ValidateEnvironmentConceptJoin(r, in, []string{q.Capability}); e != nil {
		t.Fatal(e)
	}
	for _, k := range []string{"another supplied key", "a/~ λ"} {
		c := in.Concepts["K"]
		delete(in.Concepts, "K")
		in.Concepts[k] = c
		in.Groups[0].Concepts[0] = k
		r.Requirements[0].Concept = k
		if e := ValidateEnvironmentConceptJoin(r, in, []string{q.Capability}); e != nil {
			t.Fatal(e)
		}
		in.Concepts["K"] = c
		delete(in.Concepts, k)
	}
}
func TestValidateConceptInventory_Shape(t *testing.T) {
	m := object(t, "conceptInventory")
	walkObjects(m, "", func(obj map[string]any, p string) {
		if p == "/concepts" {
			return
		}
		for _, k := range keys(obj) {
			if p == "/concepts/K/required_by/0" || (strings.HasSuffix(p, "/expectations/0") && k == "minimum") {
				continue
			}
			t.Run(pointer(p, k), func(t *testing.T) { conceptMutation(t, pointer(p, k), nil, true, ValidationShape) })
		}
	})
	for _, tc := range []struct {
		p string
		v any
	}{
		{"/concepts", nil}, {"/concepts", []any{}}, {"/concepts", "scalar"}, {"/groups", nil}, {"/groups", "scalar"},
		{"/concepts/K/required_by", []any{}}, {"/groups/0/concepts", []any{}}, {"/concepts/K/required_by/0", map[string]any{}},
		{"/concepts/K/required_by/0", map[string]any{"card": at(m, "/groups/0/card"), "design_row": "BE-D2"}},
		{"/concepts/K/required_by/0/card", nil}, {"/concepts/K/required_by/0/card/sha256", "bad"},
	} {
		conceptMutation(t, tc.p, tc.v, false, ValidationShape)
	}
	for _, p := range []string{"", "/concepts/K", "/concepts/K/required_by/0", "/concepts/K/required_by/0/card", "/concepts/K/expectations/0", "/groups/0", "/groups/0/card", "/groups/0/checker_contract"} {
		for _, k := range []string{"status", "pass", "admitted", "adequate", "valid", "fresh", "unknown"} {
			for _, v := range []bool{false, true} {
				conceptMutation(t, pointer(p, k), v, false, ValidationShape)
			}
		}
	}
}
func TestValidateConceptInventory_Expectations(t *testing.T) {
	base := "/concepts/K/expectations/0"
	for _, metric := range keys(metricRules) {
		rule := metricRules[metric]
		if metric == MetricHistoryCommits {
			continue
		}
		t.Run("Defaults/"+string(metric), func(t *testing.T) {
			m := object(t, "conceptInventory")
			edit(m, base, map[string]any{"metric": metric, "unit": rule.unit}, false)
			r, e := ValidateConceptInventory(encoded(t, m))
			if e != nil {
				t.Fatal(e)
			}
			if r.Concepts["K"].Expectations[0].Minimum != 30 {
				t.Fatal("default not concrete30")
			}
			edit(m, base+"/minimum", 30, false)
			explicit, e := ValidateConceptInventory(encoded(t, m))
			if e != nil || !bytes.Equal(encoded(t, r), encoded(t, explicit)) {
				t.Fatalf("default/export mismatch %v", e)
			}
		})
	}
	for _, metric := range keys(metricRules) {
		rule := metricRules[metric]
		m := object(t, "conceptInventory")
		expect := map[string]any{"metric": metric, "unit": rule.unit, "minimum": rule.floor}
		if rule.floor < 30 {
			expect["lower_minimum"] = rule.citation
		}
		edit(m, base, expect, false)
		r, e := ValidateConceptInventory(encoded(t, m))
		if e != nil {
			t.Fatalf("%s floor: %v", metric, e)
		}
		if r.Concepts["K"].Expectations[0].Minimum != rule.floor {
			t.Fatal("clamped")
		}
		if rule.floor < 30 {
			for _, cite := range []any{nil, "", "unknown", MinimumOutput} {
				if cite == rule.citation {
					continue
				}
				edit(m, base+"/lower_minimum", cite, false)
				rr, e := ValidateConceptInventory(encoded(t, m))
				assertValidation(t, e, ValidationShape, base+"/lower_minimum")
				if string(encoded(t, rr)) != emptyInventoryJSON {
					t.Fatal("failure")
				}
			}
			edit(m, base+"/lower_minimum", nil, true)
			_, e := ValidateConceptInventory(encoded(t, m))
			assertValidation(t, e, ValidationShape, base+"/lower_minimum")
			edit(m, base+"/lower_minimum", rule.citation, false)
		}
		edit(m, base+"/minimum", rule.floor-1, false)
		_, e = ValidateConceptInventory(encoded(t, m))
		assertValidation(t, e, ValidationShape, base+"/minimum")
	}
	for _, tc := range []struct {
		p  string
		vs []any
	}{
		{"/concepts/K/expectations", []any{nil, []any{}, "scalar"}}, {base, []any{"scalar"}},
		{base + "/metric", []any{nil, "", "unknown", "none", "positive", "negative"}}, {base + "/unit", []any{nil, "", "unknown", "commits"}},
		{base + "/minimum", []any{0, -1, 30.5, json.Number("9223372036854775808"), nil, 29}},
		{base + "/lower_minimum", []any{MinimumFalsePlan}},
	} {
		for _, v := range tc.vs {
			conceptMutation(t, tc.p, v, false, ValidationShape)
		}
	}
	conceptMutation(t, base+"/metric", nil, true, ValidationShape)
	conceptMutation(t, base+"/unit", nil, true, ValidationShape)
	m := object(t, "conceptInventory")
	edit(m, base+"/metric", "history_commits", false)
	edit(m, base+"/unit", "commits", false)
	edit(m, base+"/minimum", nil, true)
	_, e := ValidateConceptInventory(encoded(t, m))
	assertValidation(t, e, ValidationShape, base+"/minimum")
}
func TestValidateConceptInventoryValue(t *testing.T) {
	r := inventory(t)
	before := encoded(t, r)
	if e := ValidateConceptInventoryValue(r); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(before, encoded(t, r)) {
		t.Fatal("mutated")
	}
	for _, xs := range [][]MetricExpectation{nil, {}, {{Metric: MetricCrossFilePaths, Unit: MetricUnitPaths, Minimum: 0}}} {
		c := r.Concepts["K"]
		c.Expectations = xs
		r.Concepts["K"] = c
		p := "/concepts/K/expectations"
		if len(xs) > 0 {
			p += "/0/minimum"
		}
		assertValidation(t, ValidateConceptInventoryValue(r), ValidationShape, p)
	}
	assertValidation(t, ValidateConceptInventoryValue(ConceptInventory{Groups: []ConceptGroup{}}), ValidationShape, "/concepts")
	if e := ValidateConceptInventoryValue(emptyInventory()); e != nil {
		t.Fatal(e)
	}
	// Every expressible source mutation is also checked through typed shape by
	// the table of metric policies; typed zero never acquires a CUE default.
	for _, metric := range keys(metricRules) {
		rule := metricRules[metric]
		in := inventory(t)
		c := in.Concepts["K"]
		e := MetricExpectation{Metric: metric, Unit: rule.unit, Minimum: rule.floor}
		if e.Minimum < 30 {
			e.LowerMinimum = rule.citation
		}
		c.Expectations = []MetricExpectation{e}
		in.Concepts["K"] = c
		if err := ValidateConceptInventoryValue(in); err != nil {
			t.Fatal(err)
		}
		e.Minimum = 0
		c.Expectations[0] = e
		in.Concepts["K"] = c
		assertValidation(t, ValidateConceptInventoryValue(in), ValidationShape, "/concepts/K/expectations/0/minimum")
	}
}

func TestValidateConceptInventory_Identity(t *testing.T) {
	in := inventory(t)
	in.Groups[0].Concepts = append(in.Groups[0].Concepts, "K")
	assertValidation(t, ValidateConceptInventoryValue(in), ValidationIdentity, "/groups/0/concepts/1")
	in = inventory(t)
	in.Groups = append(in.Groups, in.Groups[0])
	assertValidation(t, ValidateConceptInventoryValue(in), ValidationIdentity, "/groups/1/checker_contract/id")
	for _, cite := range []ConceptCitation{{Card: &in.Groups[0].Card}, {DesignRow: ConceptDesignBED2}} {
		in = inventory(t)
		c := in.Concepts["K"]
		c.RequiredBy = []ConceptCitation{cite, cite}
		in.Concepts["K"] = c
		p := "/concepts/K/required_by/1/design_row"
		if cite.Card != nil {
			p = "/concepts/K/required_by/1/card/id"
		}
		assertValidation(t, ValidateConceptInventoryValue(in), ValidationIdentity, p)
	}
	in = inventory(t)
	c := in.Concepts["K"]
	c.Expectations = append(c.Expectations, c.Expectations[0])
	in.Concepts["K"] = c
	assertValidation(t, ValidateConceptInventoryValue(in), ValidationIdentity, "/concepts/K/expectations/1/metric")
	r, e := ValidateConceptInventory(encoded(t, in))
	assertValidation(t, e, ValidationIdentity, "/concepts/K/expectations/1/metric")
	if string(encoded(t, r)) != emptyInventoryJSON {
		t.Fatal("failure")
	}
}
func TestValidateConceptInventory_References(t *testing.T) {
	conceptMutation(t, "/groups/0/concepts/0", "L", false, ValidationReference)
	for _, member := range []string{"path", "sha256"} {
		in := inventory(t)
		g := in.Groups[0]
		g.CheckerContract.ID = "other"
		if member == "path" {
			g.Card.Path = "fixtures/other.json"
		} else {
			g.Card.SHA256 = strings.Repeat("f", 64)
		}
		in.Groups = append(in.Groups, g)
		p := "/groups/1/card/" + member
		assertValidation(t, ValidateConceptInventoryValue(in), ValidationReference, p)
		in = inventory(t)
		c := in.Concepts["K"]
		ref := *c.RequiredBy[0].Card
		if member == "path" {
			ref.Path = "fixtures/other.json"
		} else {
			ref.SHA256 = strings.Repeat("f", 64)
		}
		c.RequiredBy[0].Card = &ref
		in.Concepts["K"] = c
		assertValidation(t, ValidateConceptInventoryValue(in), ValidationReference, "/concepts/K/required_by/0/card/"+member)
	}
	for _, p := range []string{"/concepts/K/required_by/0/card/path", "/groups/0/card/path", "/groups/0/checker_contract/path"} {
		for _, s := range badPaths {
			conceptMutation(t, p, s, false, ValidationPath)
		}
	}
}
func TestValidateEnvironmentConceptJoin(t *testing.T) {
	r := records(t)
	in := inventory(t)
	ids := []string{r.Requirements[0].Capability}
	if e := ValidateEnvironmentConceptJoin(r, in, ids); e != nil {
		t.Fatal(e)
	}
	if e := ValidateEnvironmentConceptJoin(emptyRecords(), emptyInventory(), []string{}); e != nil {
		t.Fatal(e)
	}
	assertValidation(t, ValidateEnvironmentConceptJoin(emptyRecords(), emptyInventory(), ids), ValidationReference, "/card_ids/0")
	noGroups := inventory(t)
	noGroups.Groups = []ConceptGroup{}
	assertValidation(t, ValidateEnvironmentConceptJoin(r, noGroups, ids), ValidationReference, "/card_ids/0")
	assertValidation(t, ValidateEnvironmentConceptJoin(r, in, append(ids, "unused")), ValidationReference, "/card_ids/1")
	in = inventory(t)
	c := in.Concepts["K"]
	c.RequiredBy = []ConceptCitation{{DesignRow: ConceptDesignBED2}}
	in.Concepts["K"] = c
	assertValidation(t, ValidateEnvironmentConceptJoin(emptyRecords(), in, []string{}), ValidationReference, "/inventory/groups/0/card/id")
	in = inventory(t)
	ref := in.Groups[0].Card
	ref.ID = "outside"
	c = in.Concepts["K"]
	c.RequiredBy = []ConceptCitation{{Card: &ref}}
	in.Concepts["L"] = c
	assertValidation(t, ValidateEnvironmentConceptJoin(r, in, ids), ValidationReference, "/inventory/concepts/L/required_by/0/card/id")
	in = inventory(t)
	for _, tc := range []struct {
		ids []string
		r   ValidationReason
		p   string
	}{{[]string{" "}, ValidationShape, "/card_ids/0"}, {[]string{ids[0], ids[0]}, ValidationIdentity, "/card_ids/1"}, {[]string{"fixture title"}, ValidationReference, "/card_ids/0"}, {[]string{"fixture.surface"}, ValidationReference, "/card_ids/0"}} {
		assertValidation(t, ValidateEnvironmentConceptJoin(r, in, tc.ids), tc.r, tc.p)
	}
}
func TestValidateEnvironmentConceptJoin_Requirements(t *testing.T) {
	for _, field := range []string{"capability", "concept", "checker_contract/id", "card/path", "card/sha256", "checker_contract/path", "checker_contract/sha256"} {
		r := records(t)
		in := inventory(t)
		r.Witnesses = []Witness{}
		r.Coverage = []CoverageClaim{}
		q := &r.Requirements[0]
		switch field {
		case "capability":
			q.Capability = "unknown"
			q.Card.ID = q.Capability
		case "concept":
			q.Concept = "L"
		case "checker_contract/id":
			q.CheckerContract.ID = "unknown"
		case "card/path":
			q.Card.Path = "fixtures/changed.json"
		case "card/sha256":
			q.Card.SHA256 = strings.Repeat("f", 64)
		case "checker_contract/path":
			q.CheckerContract.Path = "fixtures/changed.json"
		case "checker_contract/sha256":
			q.CheckerContract.SHA256 = strings.Repeat("f", 64)
		}
		assertValidation(t, ValidateEnvironmentConceptJoin(r, in, []string{"opaque-fixture-card"}), ValidationReference, "/records/requirements/0/"+field)
	}
}
func TestValidateEnvironmentConceptJoin_GroupClosure(t *testing.T) {
	r := records(t)
	in := inventory(t)
	ids := []string{r.Requirements[0].Capability}
	g := in.Groups[0]
	g.CheckerContract.ID = "other-contract"
	g.CheckerContract.Path = "fixtures/other-contract.json"
	in.Groups = append(in.Groups, g)
	assertValidation(t, ValidateEnvironmentConceptJoin(r, in, ids), ValidationReference, "/records/coverage/0/requirements")
	q := r.Requirements[0]
	q.ID = "q-other"
	q.CheckerContract = g.CheckerContract
	r.Requirements = append(r.Requirements, q)
	for i := range r.Coverage {
		ref := r.Coverage[i].Requirements[0]
		ref.ID = q.ID
		r.Coverage[i].Requirements = append(r.Coverage[i].Requirements, ref)
	}
	if e := ValidateEnvironmentConceptJoin(r, in, ids); e != nil {
		t.Fatal(e)
	}
	c := in.Concepts["K"]
	c.RequiredBy = []ConceptCitation{{DesignRow: ConceptDesignBED9}}
	in.Concepts["L"] = c
	in.Groups[0].Concepts = append(in.Groups[0].Concepts, "L")
	assertValidation(t, ValidateEnvironmentConceptJoin(r, in, ids), ValidationReference, "/records/coverage/0/requirements")
	q = r.Requirements[0]
	q.ID = "q-l"
	q.Concept = "L"
	r.Requirements = append(r.Requirements, q)
	for i := range r.Coverage {
		ref := r.Coverage[i].Requirements[0]
		ref.ID = q.ID
		r.Coverage[i].Requirements = append(r.Coverage[i].Requirements, ref)
	}
	if e := ValidateEnvironmentConceptJoin(r, in, ids); e != nil {
		t.Fatal(e)
	}
	q = r.Requirements[0]
	q.ID = "q-duplicate"
	r.Requirements = append(r.Requirements, q)
	ref := r.Coverage[0].Requirements[0]
	ref.ID = q.ID
	r.Coverage[0].Requirements = append(r.Coverage[0].Requirements, ref)
	assertValidation(t, ValidateEnvironmentConceptJoin(r, in, ids), ValidationIdentity, "/records/coverage/0/requirements/3/id")
}
func TestValidateEnvironmentConceptJoin_ExpectationScope(t *testing.T) {
	r := records(t)
	ids := []string{r.Requirements[0].Capability}
	for _, metric := range []Metric{MetricLongOutputs, MetricShortOutputs, MetricRouteQuestions, MetricRankQuestions, MetricTableQuestions} {
		in := inventory(t)
		c := in.Concepts["K"]
		c.Expectations = []MetricExpectation{{Metric: metric, Minimum: 1, Unit: metricRules[metric].unit, LowerMinimum: MinimumOutput}}
		in.Concepts["K"] = c
		if e := ValidateEnvironmentConceptJoin(r, in, ids); e != nil {
			t.Fatal(e)
		}
		r.Requirements[0].Stratum.Subtype = "selected"
		for i := range r.Coverage {
			r.Coverage[i].Stratum.Subtype = "selected"
		}
		assertValidation(t, ValidateEnvironmentConceptJoin(r, in, ids), ValidationReference, "/inventory/concepts/K/expectations/0/minimum")
		c.Expectations[0].Minimum = 30
		c.Expectations[0].LowerMinimum = ""
		in.Concepts["K"] = c
		if e := ValidateEnvironmentConceptJoin(r, in, ids); e != nil {
			t.Fatal(e)
		}
		r.Requirements[0].Stratum.Subtype = ""
		for i := range r.Coverage {
			r.Coverage[i].Stratum.Subtype = ""
		}
	}
}
func TestConceptInventory_JSONArrays(t *testing.T) {
	r, e := ValidateConceptInventory([]byte(emptyInventoryJSON))
	if e != nil || string(encoded(t, r)) != emptyInventoryJSON {
		t.Fatal(e)
	}
	for _, s := range []string{"{", `{"concepts":null,"groups":[]}`} {
		r, e := ValidateConceptInventory([]byte(s))
		if string(encoded(t, r)) != emptyInventoryJSON || e == nil {
			t.Fatal("bad failure JSON")
		}
	}
	r = inventory(t)
	if bytes.Contains(encoded(t, r), []byte("null")) {
		t.Fatal("null array")
	}
}
func TestConceptInventory_ErrorOrder(t *testing.T) {
	m := object(t, "conceptInventory")
	edit(m, "/groups/0/concepts/0", "missing", false)
	m["z"] = false
	r, e := ValidateConceptInventory(encoded(t, m))
	assertValidation(t, e, ValidationShape, "/z")
	if string(encoded(t, r)) != emptyInventoryJSON {
		t.Fatal("failure")
	}
	delete(m, "z")
	_, e = ValidateConceptInventory(encoded(t, m))
	assertValidation(t, e, ValidationReference, "/groups/0/concepts/0")
	delete(m, "concepts")
	delete(m, "groups")
	_, e = ValidateConceptInventory(encoded(t, m))
	assertValidation(t, e, ValidationShape, "/concepts")
	r2 := records(t)
	in := inventory(t)
	r2.Requirements[0].Concept = "missing"
	assertValidation(t, ValidateEnvironmentConceptJoin(r2, in, []string{r2.Requirements[0].Capability, "unused"}), ValidationReference, "/card_ids/1")
}
func TestConceptInventory_MapKeys(t *testing.T) {
	for _, k := range []string{"/", "~", "a b", "λ", "a/~ λ", "", " "} {
		in := inventory(t)
		c := in.Concepts["K"]
		delete(in.Concepts, "K")
		in.Concepts[k] = c
		in.Groups[0].Concepts[0] = k
		if k == " " || k == "" {
			assertValidation(t, ValidateConceptInventoryValue(in), ValidationShape, pointer("/concepts", k))
			continue
		}
		b := encoded(t, in)
		got, e := ValidateConceptInventory(b)
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := got.Concepts[k]; !ok {
			t.Fatal("key changed")
		}
		c.Expectations = nil
		in.Concepts[k] = c
		p := pointer("/concepts", k) + "/expectations"
		assertValidation(t, ValidateConceptInventoryValue(in), ValidationShape, p)
		r := records(t)
		assertValidation(t, ValidateEnvironmentConceptJoin(r, in, []string{r.Requirements[0].Capability}), ValidationShape, "/inventory"+p)
	}
	for _, order := range [][]string{{"a/b", "a~b"}, {"a~b", "a/b"}} {
		in := emptyInventory()
		for _, k := range order {
			in.Concepts[k] = ConceptRecord{RequiredBy: []ConceptCitation{{DesignRow: ConceptDesignBED2}}, Expectations: []MetricExpectation{}}
		}
		assertValidation(t, ValidateConceptInventoryValue(in), ValidationShape, "/concepts/a~0b/expectations")
	}
}

func conceptSource(label, reference string, minimum int) string {
	refJSON, _ := json.Marshal(reference)
	return fmt.Sprintf(`concepts: {%q: {required_by: [{design_row: "BE-D2"}], expectations: [{metric: "cross_file_paths", unit: "paths", minimum: %d}]}}
 groups: [{card: {id:"opaque-fixture-card", path:"fixtures/opaque-fixture-card.json",sha256:%q},checker_contract:{id:"native-flow-contract",path:"fixtures/native-flow-contract.json",sha256:%q},concepts:[%s]}]`, label, minimum, strings.Repeat("0", 64), strings.Repeat("0", 64), refJSON)
}
func cueConcrete(t *testing.T, s string) []byte {
	t.Helper()
	v := cuecontext.New().CompileString(s)
	if e := v.Validate(cue.Concrete(true)); e != nil {
		t.Fatal(e)
	}
	b, e := v.MarshalJSON()
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func negativeExample(t *testing.T) string {
	t.Helper()
	b, e := os.ReadFile("testdata/invalid.cue")
	if e != nil {
		t.Fatal(e)
	}
	var out strings.Builder
	active, finished := false, false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "// ") {
			t.Fatalf("active stored negative: %s", line)
		}
		if line == "// conceptInventoryConflict: #ConceptInventory & {" {
			active = true
		}
		if active && !finished {
			out.WriteString(strings.TrimPrefix(line, "// ") + "\n")
			if line == "// }" {
				finished = true
			}
		}
	}
	if !finished {
		t.Fatal("missing concept conflict")
	}
	return out.String()
}

const exactNegative = `conceptInventoryConflict: #ConceptInventory & {
    concepts: {
        "K": {
            required_by: [{design_row: "BE-D2"}]
            expectations: [{metric: "cross_file_paths", unit: "paths", minimum: 30}]
        }
        "K": {expectations: [{metric: "cross_file_paths", unit: "paths", minimum: 31}]}
    }
    groups: []
}
`

func TestConceptInventory_CommentedNegative(t *testing.T) {
	negative := negativeExample(t)
	if negative != exactNegative {
		t.Fatalf("negative bytes changed: %s", negative)
	}
	valid, e := os.ReadFile("testdata/valid.cue")
	if e != nil {
		t.Fatal(e)
	}
	invalid, e := os.ReadFile("testdata/invalid.cue")
	if e != nil {
		t.Fatal(e)
	}
	cueConcrete(t, schemaSource+"\n"+strings.Replace(string(valid), "package benchenv", "", 1)+"\n"+string(invalid))
	v := cuecontext.New().CompileString(schemaSource + "\n" + negative)
	err := v.Validate(cue.Concrete(true))
	if err == nil || !strings.Contains(err.Error(), "conceptInventoryConflict.concepts.K.expectations.0.minimum") || !strings.Contains(err.Error(), "30") || !strings.Contains(err.Error(), "31") {
		t.Fatalf("missing exact conflict: %v", err)
	}
}
func TestValidateConceptInventory_KeyConflict(t *testing.T) {
	negative := negativeExample(t)
	s := strings.TrimPrefix(negative, "conceptInventoryConflict: #ConceptInventory & ")
	for _, src := range []string{s, `concepts: {"K": {expectations: [{metric: "cross_file_paths", unit: "paths", minimum: 31}]},"K": {required_by: [{design_row: "BE-D2"}], expectations: [{metric: "cross_file_paths", unit: "paths", minimum: 30}]}}, groups: []`} {
		r, e := ValidateConceptInventory([]byte(src))
		assertValidation(t, e, ValidationShape, "/concepts/K/expectations/0/minimum")
		if string(encoded(t, r)) != emptyInventoryJSON {
			t.Fatal("failure JSON")
		}
	}
}
func TestValidateConceptInventory_CompatibleDeclarations(t *testing.T) {
	value := `{required_by: [{design_row:"BE-D2"}],expectations:[{metric:"cross_file_paths",unit:"paths",minimum:30}]}`
	single := `concepts:{K:` + value + `},groups:[]`
	want, e := ValidateConceptInventory([]byte(single))
	if e != nil {
		t.Fatal(e)
	}
	for _, src := range []string{`concepts:{K:` + value + `, K:` + value + `},groups:[]`, `concepts:{K:{required_by:[{design_row:"BE-D2"}]},K:{expectations:[{metric:"cross_file_paths",unit:"paths",minimum:30}]}},groups:[]`} {
		cueConcrete(t, schemaSource+"\n"+"fixture: #ConceptInventory & {"+src+"}")
		got, e := ValidateConceptInventory([]byte(src))
		if e != nil || !bytes.Equal(encoded(t, want), encoded(t, got)) {
			t.Fatalf("compatible declarations %v", e)
		}
	}
}
func TestConceptInventory_CUEFieldSemantics(t *testing.T) {
	nfd, nfc := "e\u0301", "é"
	if hex.EncodeToString([]byte(nfd)) != "65cc81" || hex.EncodeToString([]byte(nfc)) != "c3a9" {
		t.Fatal("incorrect input bytes")
	}
	for _, label := range []string{nfd, nfc} {
		for _, reference := range []string{nfd, nfc} {
			src := conceptSource(label, reference, 30)
			raw := cueConcrete(t, schemaSource+"\nfixture: #ConceptInventory & {"+src+"}")
			t.Logf("label %U (%x), reference %U (%x), concrete export=%s", []rune(label), []byte(label), []rune(reference), []byte(reference), raw)
			var exported struct {
				Fixture ConceptInventory `json:"fixture"`
			}
			if e := json.Unmarshal(raw, &exported); e != nil {
				t.Fatal(e)
			}
			if len(exported.Fixture.Concepts) != 1 {
				t.Fatal("field count")
			}
			if _, ok := exported.Fixture.Concepts[nfc]; !ok {
				t.Fatal("field must evaluate NFC")
			}
			if exported.Fixture.Groups[0].Concepts[0] != reference {
				t.Fatal("reference normalized")
			}
			got, e := ValidateConceptInventory([]byte(src))
			if reference == nfc {
				if e != nil {
					t.Fatal(e)
				}
				if _, ok := got.Concepts[nfc]; !ok {
					t.Fatal("decoded key bytes")
				}
			} else {
				assertValidation(t, e, ValidationReference, "/groups/0/concepts/0")
				assertValidation(t, ValidateEnvironmentConceptJoin(records(t), exported.Fixture, []string{"opaque-fixture-card"}), ValidationReference, "/inventory/groups/0/concepts/0")
			}
		}
		raw := cueConcrete(t, fmt.Sprintf(`keys:{%q:30},reference:%q`, label, label))
		var data struct {
			Keys      map[string]int
			Reference string
		}
		_ = json.Unmarshal(raw, &data)
		if len(data.Keys) != 1 || data.Keys[nfc] != 30 || data.Reference != label {
			t.Fatal("schema-free field/value distinction")
		}
	}
	for _, order := range [][]string{{nfd, nfc}, {nfc, nfd}} {
		value := `{required_by:[{design_row:"BE-D2"}],expectations:[{metric:"cross_file_paths",unit:"paths",minimum:30}]}`
		src := fmt.Sprintf(`concepts:{%q:%s,%q:%s},groups:[]`, order[0], value, order[1], value)
		got, e := ValidateConceptInventory([]byte(src))
		if e != nil || len(got.Concepts) != 1 {
			t.Fatalf("compatible NFC declarations %v", e)
		}
		src = fmt.Sprintf(`concepts:{%q:{required_by:[{design_row:"BE-D2"}],expectations:[{metric:"cross_file_paths",unit:"paths",minimum:%d}]},%q:{expectations:[{metric:"cross_file_paths",unit:"paths",minimum:%d}]}},groups:[]`, order[0], map[string]int{nfd: 30, nfc: 31}[order[0]], order[1], map[string]int{nfd: 30, nfc: 31}[order[1]])
		got, e = ValidateConceptInventory([]byte(src))
		assertValidation(t, e, ValidationShape, "/concepts/é/expectations/0/minimum")
		if string(encoded(t, got)) != emptyInventoryJSON {
			t.Fatal("conflict result")
		}
	}
	// A typed Go map has exact equality semantics without a normalization pass.
	for _, key := range []string{nfd, nfc} {
		for _, ref := range []string{nfd, nfc} {
			in := inventory(t)
			c := in.Concepts["K"]
			delete(in.Concepts, "K")
			in.Concepts[key] = c
			in.Groups[0].Concepts[0] = ref
			e := ValidateConceptInventoryValue(in)
			if key == ref {
				if e != nil {
					t.Fatal(e)
				}
			} else {
				assertValidation(t, e, ValidationReference, "/groups/0/concepts/0")
			}
		}
	}
	r := records(t)
	in := inventory(t)
	c := in.Concepts["K"]
	delete(in.Concepts, "K")
	in.Concepts[nfc] = c
	in.Groups[0].Concepts[0] = nfc
	r.Requirements[0].Concept = nfd
	assertValidation(t, ValidateEnvironmentConceptJoin(r, in, []string{"opaque-fixture-card"}), ValidationReference, "/records/requirements/0/concept")
	r.Requirements[0].Concept = nfc
	if e := ValidateEnvironmentConceptJoin(r, in, []string{"opaque-fixture-card"}); e != nil {
		t.Fatal(e)
	}
}

func TestValidateFleetManifest(t *testing.T) {
	base := object(t, "fleet")
	r, e := ValidateFleetManifest(encoded(t, base))
	if e != nil {
		t.Fatal(e)
	}
	if r.Seed != 7 || len(r.Repositories) != 2 || len(r.Repositories[0].Commits[0].Files) != 0 {
		t.Fatal("fixture")
	}
	// Digests come from literal fixture bytes, independently of a generator.
	product := sha256.Sum256([]byte("package p\n// seed=@ENV_SEED@\n"))
	expanded := sha256.Sum256([]byte("package p\n// seed=7\n"))
	key, e := os.ReadFile("../../internal/connector/testdata/fauxtown/links.answer.json")
	if e != nil {
		t.Fatal(e)
	}
	keyDigest := sha256.Sum256(key)
	f := r.Repositories[0].Commits[1].Files[0]
	if f.SourceSHA256 != hex.EncodeToString(product[:]) || f.SHA256 != hex.EncodeToString(expanded[:]) || r.Truth[0].SHA256 != hex.EncodeToString(keyDigest[:]) {
		t.Fatal("literal digest fixture")
	}
	check := func(t *testing.T, m map[string]any, reason FleetReason, p string) {
		t.Helper()
		got, e := ValidateFleetManifest(encoded(t, m))
		if string(encoded(t, got)) != emptyFleetJSON {
			t.Fatalf("failure: %s", encoded(t, got))
		}
		var fe *FleetError
		if !errors.As(e, &fe) || fe.Path != p || fe.Reason != reason {
			t.Fatalf("want %s %s got %#v", reason, p, e)
		}
		if strings.Contains(fe.Error(), fe.Path) && fe.Path != "" {
			t.Fatal("path leak")
		}
	}
	tests := []struct {
		p string
		v any
		r FleetReason
	}{
		{"/repositories/0/commits/1/files/0/path", "../p.go", FleetPath}, {"/repositories/0/commits/1/files/0/path", "/p.go", FleetPath}, {"/repositories/0/commits/1/files/0/path", "a//p.go", FleetPath}, {"/repositories/0/commits/1/files/0/path", ".git/config", FleetPath}, {"/repositories/0/commits/1/files/0/path", "a\\b", FleetPath}, {"/repositories/0/commits/1/files/0/path", "a\x00b", FleetPath},
		{"/repositories/0/commits/1/parents/0", "missing", FleetParent}, {"/repositories/0/head", "missing", FleetParent}, {"/repositories/0/head", "c0", FleetParent},
		{"/seed", -1, FleetShape}, {"/seed", json.Number("18446744073709551616"), FleetShape}, {"/size/lines", -1, FleetShape}, {"/size/files", 1.5, FleetShape}, {"/size/history", json.Number("9223372036854775808"), FleetShape},
		{"/repositories/0/id", "bad_ID", FleetShape}, {"/repositories/0/commits/1/id", "bad_ID", FleetShape}, {"/repositories/0/commits/1/files/0/content_id", "bad_ID", FleetShape},
		{"/repositories/0/commits/1/files/0/mode", "invalid", FleetShape}, {"/repositories/0/commits/1/files/0/class", "invalid", FleetShape},
	}
	for _, tc := range tests {
		t.Run(tc.p+fmt.Sprint(tc.v), func(t *testing.T) { m := object(t, "fleet"); edit(m, tc.p, tc.v, false); check(t, m, tc.r, tc.p) })
	}
	m := object(t, "fleet")
	edit(m, "/repositories/0/commits/0/parents", []any{"c1"}, false)
	check(t, m, FleetCycle, "/repositories/0/commits/0/parents/0")
	for _, tc := range []struct {
		p     string
		idx   int
		field string
	}{{"/repositories", 2, "id"}, {"/repositories/0/commits", 2, "id"}, {"/repositories/0/commits/1/files", 1, "path"}, {"/repositories/0/commits/1/parents", 1, ""}} {
		m := object(t, "fleet")
		xs := at(m, tc.p).([]any)
		edit(m, tc.p, append(xs, xs[0]), false)
		p := indexed(tc.p, tc.idx)
		if tc.field != "" {
			p += "/" + tc.field
		}
		check(t, m, FleetDuplicate, p)
	}
	m = object(t, "fleet")
	xs := at(m, "/repositories/0/commits/1/files").([]any)
	var child map[string]any
	_ = json.Unmarshal(encoded(t, xs[0]), &child)
	child["path"] = "p.go/child"
	edit(m, "/repositories/0/commits/1/files", append(xs, child), false)
	check(t, m, FleetPath, "/repositories/0/commits/1/files/1/path")
	seen := map[string]bool{}
	walkObjects(base, "", func(obj map[string]any, p string) {
		sig := strings.Join(keys(obj), ",")
		if seen[sig] {
			return
		}
		seen[sig] = true
		for _, k := range keys(obj) {
			m := object(t, "fleet")
			q := pointer(p, k)
			edit(m, q, nil, true)
			check(t, m, FleetShape, q)
		}
	})
	// Permutations change neither the topological order nor sorted file/repo order.
	m = object(t, "fleet")
	repos := m["repositories"].([]any)
	repos[0], repos[1] = repos[1], repos[0]
	for _, x := range repos {
		cs := x.(map[string]any)["commits"].([]any)
		cs[0], cs[1] = cs[1], cs[0]
	}
	permuted, e := ValidateFleetManifest(encoded(t, m))
	if e != nil || !bytes.Equal(encoded(t, r), encoded(t, permuted)) {
		t.Fatalf("permutation %v", e)
	}
}
func TestFleetManifest_GeneratorVersion(t *testing.T) {
	if reflect.TypeFor[FleetManifest]().Field(1).Type != reflect.TypeFor[GeneratorVersion]() || GeneratorVersionV1 != "env-fleet/v1" {
		t.Fatal("named generator version")
	}
	for _, v := range []any{nil, "", "future", 123} {
		m := object(t, "fleet")
		edit(m, "/generator_version", v, false)
		r, e := ValidateFleetManifest(encoded(t, m))
		var fe *FleetError
		if !errors.As(e, &fe) || fe.Reason != FleetShape || fe.Path != "/generator_version" || string(encoded(t, r)) != emptyFleetJSON {
			t.Fatalf("generator version: %#v", e)
		}
		req := object(t, "request_seed")
		edit(req, "/seed/manifest/generator_version", v, false)
		got, e := ValidateEnvironmentRequest(encoded(t, req))
		assertMode(t, got, e, ModeShape, "/seed/manifest/generator_version")
	}
	m := object(t, "fleet")
	delete(m, "generator_version")
	_, e := ValidateFleetManifest(encoded(t, m))
	if fe, ok := e.(*FleetError); !ok || fe.Path != "/generator_version" {
		t.Fatal(e)
	}
	req := object(t, "request_seed")
	edit(req, "/seed/manifest/generator_version", nil, true)
	got, e := ValidateEnvironmentRequest(encoded(t, req))
	assertMode(t, got, e, ModeShape, "/seed/manifest/generator_version")
}
func assertMode(t *testing.T, r EnvironmentRequest, e error, reason ModeReason, p string) {
	t.Helper()
	var me *ModeError
	if !errors.As(e, &me) || me.Path != p || me.Reason != reason {
		t.Fatalf("want %s %s got %#v", reason, p, e)
	}
	if !reflect.DeepEqual(r, EnvironmentRequest{}) || me.Error() != "environment: "+string(reason) {
		t.Fatal("unsafe result")
	}
}
func TestEnvironmentRequest_Negatives(t *testing.T) {
	for _, mode := range keys(modePayloads) {
		payload := modePayloads[mode]
		t.Run(string(mode), func(t *testing.T) {
			name := "request_" + payload
			src := fixtureSource(t, name)
			r, e := ValidateEnvironmentRequest(src)
			if e != nil || r.Mode != mode {
				t.Fatalf("valid request %s: %#v", mode, e)
			}
			for _, remove := range []bool{false, true} {
				m := object(t, name)
				edit(m, "/"+payload, nil, remove)
				r, e := ValidateEnvironmentRequest(encoded(t, m))
				assertMode(t, r, e, ModeShape, "/"+payload)
			}
			m := object(t, name)
			other := "audit"
			if payload == other {
				other = "fixtures"
			}
			m[other] = at(object(t, "request_"+other), "/"+other)
			r, e = ValidateEnvironmentRequest(encoded(t, m))
			assertMode(t, r, e, ModeShape, "/"+other)
			m = object(t, name)
			m["mode"] = other
			r, e = ValidateEnvironmentRequest(encoded(t, m))
			assertMode(t, r, e, ModeShape, "/"+other)
			m = object(t, name)
			m["z/~"] = false
			r, e = ValidateEnvironmentRequest(encoded(t, m))
			assertMode(t, r, e, ModeShape, "/z~1~0")
			m = object(t, name)
			m["mode"] = "unknown"
			r, e = ValidateEnvironmentRequest(encoded(t, m))
			assertMode(t, r, e, ModeUnknownName, "/mode")
			// Check each payload descendant shape once rather than repeatedly walking
			// the 120 identical witness structures already tested by record mutations.
			seen := map[string]bool{}
			walkObjects(at(object(t, name), "/"+payload), "/"+payload, func(obj map[string]any, p string) {
				sig := strings.Join(keys(obj), ",")
				if seen[sig] {
					return
				}
				seen[sig] = true
				for _, k := range keys(obj) {
					m := object(t, name)
					q := pointer(p, k)
					edit(m, q, nil, true)
					r, e := ValidateEnvironmentRequest(encoded(t, m))
					assertMode(t, r, e, ModeShape, q)
				}
				for _, k := range keys(obj) {
					if k == "path" {
						m := object(t, name)
						q := pointer(p, k)
						edit(m, q, "../secret", false)
						r, e := ValidateEnvironmentRequest(encoded(t, m))
						assertMode(t, r, e, ModeShape, q)
					}
				}
			})
		})
	}
	for _, src := range []string{`{"mode":"audit","mode":"audit","audit":{"sources":[],"runs":[],"rulings":[]}}`, `{"mode":"audit","mode":"fixtures"}`, `mode:"audit",mode:"audit"`, `mode:"audit",mode:"fixtures"`} {
		r, e := ValidateEnvironmentRequest([]byte(src))
		assertMode(t, r, e, ModeDuplicate, "/mode")
	}
	m := object(t, "request_snapshots")
	cs := at(m, "/snapshots/commits").([]any)
	cs[1] = cs[0]
	r, e := ValidateEnvironmentRequest(encoded(t, m))
	assertMode(t, r, e, ModeShape, "/snapshots/commits/1")
}
func TestNewModeError(t *testing.T) {
	cause := errors.New("secret cause\n")
	for _, p := range []string{"", "/", "/x~0y~1z", "/unknown λ"} {
		e := NewModeError(p, ModeDependency, cause)
		if e.Path != p || e.Unwrap() != cause || !errors.Is(e, cause) || e.Error() != "environment: dependency" {
			t.Fatal("constructor law")
		}
		if bytes.Contains(encoded(t, e), []byte("secret")) {
			t.Fatal("cause exported")
		}
		if NewModeError(p, ModeDependency, cause) == e {
			t.Fatal("not fresh")
		}
	}
	for _, p := range []string{"x", "/~", "/~2", "/x~0~"} {
		e := NewModeError(p, "unknown", cause)
		if e.Path != "" || e.Reason != ModeShape || e.Error() != "environment: shape" {
			t.Fatal("normalization")
		}
	}
	if (&ModeError{Reason: "unknown"}).Error() != "environment: shape" {
		t.Fatal("manual unknown")
	}
	if NewModeError("", ModeShape, nil).Unwrap() != nil {
		t.Fatal("nil cause")
	}
}
func TestEnvironmentRecords_JSONArrays(t *testing.T) {
	r, e := ValidateEnvironmentRecords([]byte(emptyRecordsJSON))
	if e != nil || string(encoded(t, r)) != emptyRecordsJSON {
		t.Fatal(e)
	}
	f := FleetResult{Repositories: []FleetRepositoryResult{{ID: "a", Head: "a", Commits: []FleetCommitResult{{ID: "root", Commit: strings.Repeat("a", 40), Parents: []string{}}}}}, Truth: []Ref{}, Excluded: []ExcludedCount{}}
	for _, v := range []any{f, ModeDiagnostic{Evidence: []Ref{}}, SnapshotSample{Evidence: []Ref{}}} {
		b := encoded(t, v)
		if bytes.Contains(b, []byte(`"evidence":null`)) || bytes.Contains(b, []byte(`"parents":null`)) || bytes.Contains(b, []byte(`"truth":null`)) {
			t.Fatalf("null collections: %s", b)
		}
	}
	for _, src := range []string{"{", `{"seed":0}`} {
		f, e := ValidateFleetManifest([]byte(src))
		if e == nil || string(encoded(t, f)) != emptyFleetJSON {
			t.Fatal("fleet failure arrays")
		}
	}
}

func TestValidateEnvironmentRecords_NegativeShape(t *testing.T) {
	t.Run("RequiredFields", testRecordsRequiredFields)
	t.Run("NumericAndEnums", testRecordsNumericAndEnumShape)
	t.Run("TextDigestCommitAndArrays", testRecordsScalarShape)
}

// Reverse all object fields in the source, without changing list order.
func reversedJSON(t *testing.T, v any) []byte {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		parts := []string{}
		ks := keys(x)
		for i := len(ks) - 1; i >= 0; i-- {
			parts = append(parts, string(encoded(t, ks[i]))+":"+string(reversedJSON(t, x[ks[i]])))
		}
		return []byte("{" + strings.Join(parts, ",") + "}")
	case []any:
		parts := []string{}
		for _, y := range x {
			parts = append(parts, string(reversedJSON(t, y)))
		}
		return []byte("[" + strings.Join(parts, ",") + "]")
	}
	return encoded(t, v)
}
func TestValidation_DeterministicFieldOrder(t *testing.T) {
	m := object(t, "records")
	edit(m, "/witnesses/0/anchor/path", "../x", false)
	edit(m, "/witnesses/0/checker/version", "different", false)
	for _, src := range [][]byte{encoded(t, m), reversedJSON(t, m)} {
		_, e := ValidateEnvironmentRecords(src)
		assertValidation(t, e, ValidationPath, "/witnesses/0/anchor/path")
	}
	m["z"] = false
	for _, src := range [][]byte{encoded(t, m), reversedJSON(t, m)} {
		_, e := ValidateEnvironmentRecords(src)
		assertValidation(t, e, ValidationShape, "/z")
	}
	for _, src := range []string{`{concepts:null,groups:null}`, `{groups:null,concepts:null}`} {
		_, e := ValidateConceptInventory([]byte(src))
		assertValidation(t, e, ValidationShape, "/concepts")
	}
	for _, order := range [][]string{{"a/b", "a~b"}, {"a~b", "a/b"}} {
		in := emptyInventory()
		for _, k := range order {
			in.Concepts[k] = ConceptRecord{RequiredBy: []ConceptCitation{{DesignRow: ConceptDesignBED2}}, Expectations: []MetricExpectation{}}
		}
		for _, src := range [][]byte{encoded(t, in), reversedJSON(t, toObject(t, in))} {
			r, e := ValidateConceptInventory(src)
			assertValidation(t, e, ValidationShape, "/concepts/a~0b/expectations")
			if string(encoded(t, r)) != emptyInventoryJSON {
				t.Fatal("failure")
			}
		}
		r := records(t)
		assertValidation(t, ValidateEnvironmentConceptJoin(r, in, []string{"opaque-fixture-card"}), ValidationShape, "/inventory/concepts/a~0b/expectations")
	}
}
func toObject(t *testing.T, v any) map[string]any {
	t.Helper()
	var m map[string]any
	if e := json.Unmarshal(encoded(t, v), &m); e != nil {
		t.Fatal(e)
	}
	return m
}

func TestConceptInventory_ShapePhaseOrder(t *testing.T) {
	m := object(t, "conceptInventory")
	edit(m, "/concepts/K/expectations/0/metric", "none", false)
	edit(m, "/concepts/K/expectations/0/unit", "unknown", false)
	for _, b := range [][]byte{encoded(t, m), reversedJSON(t, m)} {
		r, e := ValidateConceptInventory(b)
		assertValidation(t, e, ValidationShape, "/concepts/K/expectations/0/metric")
		if string(encoded(t, r)) != emptyInventoryJSON {
			t.Fatal("failure")
		}
	}
}

func TestEnvironmentRequest_FieldMutations(t *testing.T) {
	for _, mode := range keys(modePayloads) {
		payload := modePayloads[mode]
		m := object(t, "request_"+payload)
		seen := map[string]bool{}
		walkObjects(at(m, "/"+payload), "/"+payload, func(obj map[string]any, p string) {
			for _, k := range keys(obj) {
				q := pointer(p, k)
				var bad any
				key := k
				switch k {
				case "sha256", "source_sha256", "profile_sha256", "binary_sha256", "cell_key":
					bad = "not-a-digest"
				case "commit", "product_commit":
					bad = "not-a-commit"
				case "measured_at", "claim_time":
					bad = "2026-02-30T23:59:59Z"
				case "wall_cap_ms", "max_usd_micros", "max_repositories", "max_commits", "max_files", "max_bytes":
					bad = 0
				case "tier", "split", "axis", "class", "prediction", "polarity", "kind", "generator_version":
					bad = "unknown"
				default:
					continue
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				changed := object(t, "request_"+payload)
				edit(changed, q, bad, false)
				r, e := ValidateEnvironmentRequest(encoded(t, changed))
				assertMode(t, r, e, ModeShape, q)
			}
		})
	}
}
func TestConceptInventory_EscapedMetricIdentity(t *testing.T) {
	in := inventory(t)
	c := in.Concepts["K"]
	delete(in.Concepts, "K")
	c.Expectations = append(c.Expectations, c.Expectations[0])
	in.Concepts["a/~ λ"] = c
	in.Groups[0].Concepts[0] = "a/~ λ"
	p := "/concepts/a~1~0 λ/expectations/1/metric"
	assertValidation(t, ValidateConceptInventoryValue(in), ValidationIdentity, p)
	got, e := ValidateConceptInventory(encoded(t, in))
	assertValidation(t, e, ValidationIdentity, p)
	if string(encoded(t, got)) != emptyInventoryJSON {
		t.Fatal("failure")
	}
	assertValidation(t, ValidateEnvironmentConceptJoin(records(t), in, []string{"opaque-fixture-card"}), ValidationIdentity, "/inventory"+p)
}

func TestValidateEnvironmentRecords_ClaimAxisIdentity(t *testing.T) {
	m := object(t, "records")
	xs := at(m, "/coverage/0/stratum/scale").([]any)
	edit(m, "/coverage/0/stratum/scale", append(xs, xs[0]), false)
	r, e := ValidateEnvironmentRecords(encoded(t, m))
	if string(encoded(t, r)) != emptyRecordsJSON {
		t.Fatal("failure")
	}
	assertValidation(t, e, ValidationIdentity, "/coverage/0/stratum/scale/1/axis")
}
