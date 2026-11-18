package boundaries_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/boundaries"
)

func TestParseBoundaryRules(t *testing.T) {
	for _, value := range []string{
		`{"schema_version":1,"rules":[{"id":"layers","from":["src/**/*.go"],"deny":["internal/store/**","go:net/http/**","oci:registry.test:5000/team/*","."],"except":[{"from":["src/cue?.go"],"to":["internal/store/test*"]}],"reason":"Keep the door standalone."}]}`,
		`{"schema_version":1,"rules":[{"id":"allowed","from":["*.go"],"allow_only":["internal/[ab]","npm:@scope/*"],"reason":"Only reviewed dependencies."}]}`,
	} {
		if _, err := boundaries.ParseRules(strings.NewReader(value)); err != nil {
			t.Errorf("rejected %s: %v", value, err)
		}
	}
	const rule = `{"id":"layers","from":["src/*.go"],"deny":["go:net/http"],"reason":"Keep the door standalone."}`
	for name, value := range map[string]string{
		"no rules":                `[]`,
		"invalid id":              strings.Replace(rule, "layers", "/private/id", 1),
		"empty source":            strings.Replace(rule, `["src/*.go"]`, `[]`, 1),
		"absolute source":         strings.Replace(rule, "src/*.go", "/private/*.go", 1),
		"external source":         strings.Replace(rule, "src/*.go", "go:net/http", 1),
		"parent source":           strings.Replace(rule, "src/*.go", "src/../*.go", 1),
		"partial globstar":        strings.Replace(rule, "src/*.go", "src/a**/x.go", 1),
		"bad character class":     strings.Replace(rule, "src/*.go", "src/[a.go", 1),
		"invalid character range": strings.Replace(rule, "src/*.go", "src/[z-a].go", 1),
		"bad ecosystem":           strings.Replace(rule, "go:net/http", "repo:net/http", 1),
		"absolute external":       strings.Replace(rule, "go:net/http", "go:/net/http", 1),
		"external parent":         strings.Replace(rule, "go:net/http", "go:net/../http", 1),
		"multiple modes":          strings.Replace(rule, `"deny":`, `"allow_only":["fmt"],"deny":`, 1),
		"no mode":                 strings.Replace(rule, `"deny":["go:net/http"],`, "", 1),
		"empty deny":              strings.Replace(rule, `["go:net/http"]`, `[]`, 1),
		"null allow":              strings.Replace(rule, `"deny":`, `"allow_only":null,"deny":`, 1),
		"null exception":          strings.Replace(rule, `"deny":`, `"except":null,"deny":`, 1),
		"partial exception":       strings.Replace(rule, `"deny":`, `"except":[{"to":["go:net/http"]}],"deny":`, 1),
		"blank reason":            strings.Replace(rule, "Keep the door standalone.", " ", 1),
		"multiline reason":        strings.Replace(rule, "Keep the door standalone.", `one\ntwo`, 1),
		"unknown field":           strings.Replace(rule, `"deny":`, `"unknown":true,"deny":`, 1),
		"case alias":              strings.Replace(rule, `"id":`, `"ID":`, 1),
		"unicode case alias":      strings.Replace(rule, `"reason":`, `"reaſon":`, 1),
		"duplicate field":         strings.Replace(rule, `"id":`, `"id":"other","id":`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := boundaries.ParseRules(strings.NewReader(`{"schema_version":1,"rules":[` + value + `]}`)); err == nil {
				t.Fatalf("accepted %s", value)
			}
		})
	}
	for _, doc := range []string{
		`{"schema_version":2,"rules":[` + rule + `]}`,
		`{"rules":[` + rule + `]}`,
		`{"schema_version":1,"rules":[]}`,
		`{"schema_version":1,"rules":[` + rule + "," + rule + `]}`,
		`{"schema_version":1,"rules":[` + rule + `]} {}`,
	} {
		if _, err := boundaries.ParseRules(strings.NewReader(doc)); err == nil {
			t.Errorf("accepted %s", doc)
		}
	}
}

func TestEvaluateBoundaries(t *testing.T) {
	rules, err := boundaries.ParseRules(strings.NewReader(`{"schema_version":1,"rules":[
		{"id":"unverified","from":["unknown.go"],"deny":["go:net/http/**"],"reason":"No network."},
		{"id":"trusted","from":["trusted.go"],"allow_only":["lib/**"],"reason":"Local only."},
		{"id":"unmatched","from":["absent.go"],"deny":["lib"],"reason":"Require evidence."},
		{"id":"not-admitted","from":["unmeasured.go"],"deny":["lib"],"reason":"Require admission."},
		{"id":"incomplete","from":["incomplete.go"],"deny":["lib"],"reason":"Require completeness."},
		{"id":"clean","from":["clean.go"],"allow_only":["go:fmt"],"reason":"Only formatting."},
		{"id":"allow-only","from":["src/**/*.go"],"allow_only":["lib/**"],"except":[{"from":["src/top.go"],"to":["go:net/http"]}],"reason":"Local only."},
		{"id":"deny","from":["src/**/*.go"],"deny":["go:net/http/**","internal/store/**"],"except":[{"from":["src/top.go"],"to":["go:net/http"]}],"reason":"Keep the layer separate."}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	edge := func(from, to string, line int, resolution deps.Resolution, kind deps.Kind) deps.Edge {
		return deps.Edge{From: from, To: to, Line: line, Resolution: resolution, Kind: kind, Provenance: deps.ProvenanceBuiltin}
	}
	unknown := deps.Unknown{From: "src/deep/a.go", Line: 2, Code: deps.UnknownExternalUnpinned, DecidableBy: []deps.DecidableBy{}}
	files := []deps.FileDeps{
		{Path: "outside.go"},
		{Path: "clean.go", Measured: true, Complete: true},
		{Path: "src/top.go", Measured: true, Complete: true, Edges: []deps.Edge{
			edge("src/top.go", "go:net/http", 2, deps.ResolvedExternal, deps.KindImport),
			edge("src/top.go", "internal/store", 3, deps.ResolvedDirectory, deps.KindImport),
		}},
		{Path: "src/deep/a.go", Measured: true, Unknowns: []deps.Unknown{unknown}, Edges: []deps.Edge{
			edge("src/deep/a.go", "internal/store/sub", 5, deps.ResolvedDirectory, deps.KindImport),
			edge("src/deep/a.go", "go:net/http", 2, deps.ResolvedExternal, deps.KindImport),
			edge("src/deep/a.go", "internal/store", 8, deps.ResolvedDirectory, deps.KindInclude),
		}},
		{Path: "unknown.go", Measured: true, Complete: true, Unknowns: []deps.Unknown{{From: "unknown.go", Code: deps.UnknownDynamicImport}}},
		{Path: "unmeasured.go", Complete: true},
		{Path: "incomplete.go", Measured: true},
		{Path: "trusted.go", Trusted: true, Complete: true, Edges: []deps.Edge{edge("trusted.go", "lib", 1, deps.ResolvedDirectory, deps.KindImport)}},
	}
	got := boundaries.EvaluateBoundaries(rules, files)
	wantIDs := []string{"allow-only", "clean", "deny", "incomplete", "not-admitted", "trusted", "unmatched", "unverified"}
	want := []boundaries.Verdict{
		boundaries.VerdictViolated, boundaries.VerdictClean, boundaries.VerdictViolated,
		boundaries.VerdictUnverified, boundaries.VerdictUnverified, boundaries.VerdictClean,
		boundaries.VerdictUnverified, boundaries.VerdictUnverified,
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, result := range got {
		if result.ID != wantIDs[i] || result.Verdict != want[i] {
			t.Errorf("result %d: %+v, want %s %s", i, result, wantIDs[i], want[i])
		}
	}
	for _, i := range []int{0, 2} {
		res := got[i]
		if res.MatchedFiles != 2 || len(res.Violations) != 3 || len(res.Unverified) != 1 ||
			res.Violations[0].To != "go:net/http" || res.Violations[1].To != "internal/store/sub" ||
			res.Violations[2].From != "src/top.go" || res.Unverified[0].Unknowns[0].Code != deps.UnknownExternalUnpinned {
			t.Errorf("lost or unordered mixed evidence: %+v", res)
		}
	}
	if got[6].MatchedFiles != 0 || len(got[6].Unverified) != 0 {
		t.Errorf("unmatched scope: %+v", got[6])
	}
	original, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(files)
	for _, file := range files {
		slices.Reverse(file.Edges)
	}
	reversed, err := json.Marshal(boundaries.EvaluateBoundaries(rules, files))
	if err != nil || string(original) != string(reversed) || strings.Contains(string(original), "null") {
		t.Fatalf("unstable or null evidence: %v\n%s\n%s", err, original, reversed)
	}
}

func TestEvaluateBoundariesRejectsMalformedEvidence(t *testing.T) {
	rules, err := boundaries.ParseRules(strings.NewReader(`{"schema_version":1,"rules":[
		{"id":"allow-local","from":["**"],"allow_only":["lib/**"],"reason":"Use the reviewed library."},
		{"id":"bad-crossing","from":["app/bad.go"],"allow_only":["lib/**"],"reason":"Use the reviewed library."}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	const unsafe = "/private-sentinel"
	if rules.Governs(unsafe+"/source.go") || rules.Governs("app/../malformed.go") {
		t.Fatal("an invalid path was bound to a rule")
	}
	edge := func(from, to, raw string, resolution deps.Resolution, line int) deps.Edge {
		return deps.Edge{From: from, To: to, Raw: raw, Kind: deps.KindImport, Resolution: resolution,
			Line: line, Provenance: deps.ProvenanceBuiltin}
	}
	files := []deps.FileDeps{
		{Path: unsafe + "/source.go", Dialect: unsafe, Measured: true, Complete: true,
			Edges: []deps.Edge{edge(unsafe+"/source.go", "internal/store", "", deps.ResolvedDirectory, 1)}},
		{Path: "app/../malformed.go", Measured: true, Complete: true},
		{Path: "app/bad.go", Dialect: "go", Measured: true, Complete: true, Edges: []deps.Edge{
			edge("app/bad.go", unsafe+"/target", "", deps.ResolvedDirectory, 1),
			edge(unsafe+"/source.go", "internal/store", "", deps.ResolvedDirectory, 2),
			edge("app/bad.go", "go:"+unsafe, "", deps.ResolvedExternal, 3),
			edge("app/bad.go", "lib/../store", "", deps.ResolvedDirectory, 4),
		}},
		{Path: "app/known.go", Dialect: "go", Measured: true, Edges: []deps.Edge{
			edge("app/known.go", "internal/store", unsafe+"/operand", deps.ResolvedDirectory, 2),
			edge("app/known.go", "internal/mcp", "internal/mcp", deps.ResolvedDirectory, 4),
		}, Unknowns: []deps.Unknown{
			{From: unsafe + "/source.go", Code: deps.UnknownDynamicImport, Raw: unsafe + "/operand", Line: 3,
				DecidableBy: []deps.DecidableBy{deps.DecideObservation, deps.DecidableBy(unsafe)}},
			{From: "app/known.go", Code: deps.UnknownCode(unsafe), Raw: unsafe},
		}},
		{Path: "app/control.go", Dialect: "go", Measured: true, Complete: true, Edges: []deps.Edge{
			edge("app/control.go", "lib", "lib", deps.ResolvedDirectory, 2),
		}},
	}

	got := boundaries.EvaluateBoundaries(rules, files)
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), unsafe) || strings.Contains(string(data), "../") {
		t.Fatalf("output disclosed an unsafe value: %s", data)
	}
	if got[0].Verdict != boundaries.VerdictViolated || got[1].Verdict != boundaries.VerdictUnverified ||
		got[0].MatchedFiles != 3 || got[1].MatchedFiles != 1 || len(got[0].Violations) != 2 ||
		len(got[1].Violations) != 0 {
		t.Fatalf("valid findings or uncertainty were lost: %s", data)
	}
	if a, b := got[0].Violations[0], got[0].Violations[1]; a.To != "internal/store" || a.Raw != "***" ||
		b.To != "internal/mcp" || b.Raw != "internal/mcp" {
		t.Fatalf("valid identities or safe raw evidence changed: %s", data)
	}
	if evidence := got[0].Unverified; len(evidence) != 3 || evidence[0].Path != "***" ||
		evidence[1].Path != "app/bad.go" || evidence[2].Path != "app/known.go" ||
		len(evidence[2].Unknowns) != 1 || evidence[2].Unknowns[0].From != "app/known.go" ||
		evidence[2].Unknowns[0].Raw != "***" ||
		!slices.Equal(evidence[2].Unknowns[0].DecidableBy, []deps.DecidableBy{deps.DecideObservation}) {
		t.Fatalf("unverified evidence was lost or not redacted: %s", data)
	}

	slices.Reverse(files)
	for _, file := range files {
		slices.Reverse(file.Edges)
		slices.Reverse(file.Unknowns)
	}
	reversed, err := json.Marshal(boundaries.EvaluateBoundaries(rules, files))
	if err != nil || string(data) != string(reversed) {
		t.Fatalf("malformed evidence changed with input order: %v\n%s\n%s", err, data, reversed)
	}
}
