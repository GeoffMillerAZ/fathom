package main

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestScanKeys(t *testing.T) {
	src := "{\n  \"name\": \"a\",\n  \"scripts\": {\n    \"build\": \"echo { not a brace\",\n    \"te\\u0073t\": \"y\"\n  },\n  \"bin\": {\"cli\": \"b.js\"},\n  \"exports\": {\n    \".\": {\n      \"import\": \"./i.js\"\n    },\n    \"./x\": null\n  }\n}\n"
	idx, err := scanKeys([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if idx.rootLine != 1 {
		t.Errorf("rootLine = %d, want 1", idx.rootLine)
	}
	for _, c := range []struct {
		section, key string
		line         int
	}{
		{"", "name", 2}, {"", "scripts", 3}, {"", "bin", 7}, {"", "exports", 8},
		{"scripts", "build", 4}, {"scripts", "test", 5},
		{"bin", "cli", 7},
		{"exports", ".", 9}, {"exports", "./x", 12},
	} {
		got, ok := idx.line(c.section, c.key)
		if !ok || got != c.line {
			t.Errorf("line(%q, %q) = %d, %v; want %d", c.section, c.key, got, ok, c.line)
		}
	}
	// A key two levels below a section is not an entry of it.
	if _, ok := idx.line("exports", "import"); ok {
		t.Error("exports > . > import must not be an exports entry")
	}
}

func TestScanKeysRootAfterBlankLines(t *testing.T) {
	idx, err := scanKeys([]byte("\xef\xbb\xbf\n\n{\"name\":\"a\"}"))
	if err != nil {
		t.Fatal(err)
	}
	if idx.rootLine != 3 {
		t.Errorf("rootLine = %d, want 3 (a BOM and two blank lines before the brace)", idx.rootLine)
	}
}

func TestScanKeysRefusesAnArray(t *testing.T) {
	if _, err := scanKeys([]byte(`[{"name":"a"}]`)); err == nil {
		t.Error("a manifest whose root is an array has no package; want an error")
	}
}

func TestScanKeysLastDuplicateWins(t *testing.T) {
	idx, err := scanKeys([]byte("{\n\"scripts\": {\n\"a\": \"1\",\n\"a\": \"2\"\n}\n}"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := idx.line("scripts", "a"); got != 4 {
		t.Errorf("duplicate key line = %d, want 4 (the decoder keeps the last)", got)
	}
}

func TestRewriteWorkspaceSpec(t *testing.T) {
	for _, c := range []struct {
		spec, version, want, shape string
	}{
		{"workspace:^", "7.29.9", "7.29.9", "^"},
		{"workspace:~", "7.29.9", "7.29.9", "~"},
		{"workspace:*", "8.0.0-beta.3", "8.0.0-beta.3", "*"},
		{"workspace:^", "", "*", "^"},
		{"workspace:^1.2.3", "9.9.9", "^1.2.3", "range"},
		{"workspace:>=2", "9.9.9", ">=2", "range"},
		{"workspace:../other", "1.0.0", "workspace:../other", "path"},
		{"workspace:@scope/real@^1", "1.0.0", "workspace:@scope/real@^1", "alias"},
		{"^1.0.0", "1.0.0", "^1.0.0", ""},
		{"npm:real@^2", "1.0.0", "npm:real@^2", ""},
	} {
		got, shape := rewriteWorkspaceSpec(c.spec, c.version)
		if got != c.want || shape != c.shape {
			t.Errorf("rewriteWorkspaceSpec(%q, %q) = %q, %q; want %q, %q", c.spec, c.version, got, shape, c.want, c.shape)
		}
	}
}

func decodeNode(t *testing.T, s string) queryNode {
	t.Helper()
	var n queryNode
	if err := json.Unmarshal([]byte(s), &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestToolSets(t *testing.T) {
	n := decodeNode(t, `{
	  "name": "@acme/widgets",
	  "scripts": {"build": "tsc", "weird": 3, "obj": {"a": 1}},
	  "bin": {"widgets": "bin/w.js"},
	  "exports": {".": "./i.js", "./x": {"import": "./x.mjs"}, "./hidden": null},
	  "dependencies": {"b": "1"},
	  "devDependencies": {"c": "2"},
	  "peerDependencies": {"d": "3"},
	  "optionalDependencies": {"e": "4", "b": "1"}
	}`)
	s := toolSetsOf(n)
	if s.name != "@acme/widgets" {
		t.Errorf("name = %q", s.name)
	}
	if got, want := keys(s.tasks), []string{"build", "weird"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %v, want %v (a scalar script only: the dialect selects scalars)", got, want)
	}
	if got, want := keys(s.bin), []string{"widgets"}; !reflect.DeepEqual(got, want) {
		t.Errorf("bin = %v, want %v", got, want)
	}
	if got, want := keys(s.exports), []string{".", "./x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("exports = %v, want %v (a null target is a blocked path, not an export)", got, want)
	}
	if s.nullExports != 1 {
		t.Errorf("nullExports = %d, want 1", s.nullExports)
	}
	if got, want := keys(s.refs), []string{"b", "c", "d", "e"}; !reflect.DeepEqual(got, want) {
		t.Errorf("refs = %v, want %v (the union of the four dependency maps, a name once)", got, want)
	}
}

func TestDefinitionsLines(t *testing.T) {
	src := "{\n  \"name\": \"a\",\n  \"scripts\": {\n    \"build\": \"x\"\n  },\n  \"bin\": \"./cli.js\",\n  \"exports\": {\n    \".\": \"./i.js\"\n  }\n}\n"
	idx, err := scanKeys([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	// npm normalizes a string bin to an object named after the package.
	n := decodeNode(t, `{"name":"a","scripts":{"build":"x"},"bin":{"a":"cli.js"},"exports":{".":"./i.js"}}`)
	got, miss := definitionsOf("packages/a/package.json", toolSetsOf(n), idx)
	want := []def{
		{"packages/a/package.json", "a", 1, "package"},
		{"packages/a/package.json", "build", 4, "task"},
		{"packages/a/package.json", "a", 6, "route"}, // a string bin: the line of the "bin" key
		{"packages/a/package.json", ".", 8, "route"},
	}
	if miss != 0 {
		t.Errorf("unlocated = %d, want 0", miss)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("definitions\n got: %v\nwant: %v", got, want)
	}
}

func TestDeclaredFromRawDecode(t *testing.T) {
	d, err := declaredOf([]byte(`{"name":"a","scripts":{"x":"1","y":{"z":1}},"bin":"cli.js","exports":{".":"a","./n":null},"dependencies":{"p":"1"},"peerDependencies":{"q":"2"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !d.binString {
		t.Error("a string bin must be flagged: its name is the tool's")
	}
	if got, want := keys(d.tasks), []string{"x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %v, want %v", got, want)
	}
	if got, want := keys(d.exports), []string{"."}; !reflect.DeepEqual(got, want) {
		t.Errorf("exports = %v, want %v", got, want)
	}
	if got, want := keys(d.refs), []string{"p", "q"}; !reflect.DeepEqual(got, want) {
		t.Errorf("refs = %v, want %v", got, want)
	}
	if _, err := declaredOf([]byte(`{"name":`)); err == nil {
		t.Error("a truncated manifest must not decode")
	}
}

func TestSiblingEdges(t *testing.T) {
	members := map[string]member{
		"root": {name: "root", dir: "."},
		"a":    {name: "a", dir: "packages/a"},
		"b":    {name: "b", dir: "packages/b"},
		"c":    {name: "c", dir: "packages/c"},
	}
	delete(members, "root")
	var ls lsRoot
	err := json.Unmarshal([]byte(`{
	  "name": "root",
	  "dependencies": {
	    "a": {"version": "1.0.0", "resolved": "file:../packages/a", "dependencies": {
	      "b": {"version": "1.0.0"},
	      "c": {"version": "1.0.0", "invalid": "\"^2.0.0\" from packages/a"},
	      "ext": {"required": "^1", "missing": true}
	    }},
	    "b": {"version": "1.0.0", "resolved": "file:../packages/b", "dependencies": {
	      "a": {"required": "link:../a", "missing": true}
	    }},
	    "c": {"version": "1.0.0", "resolved": "file:../packages/c"}
	  }
	}`), &ls)
	if err != nil {
		t.Fatal(err)
	}
	edges, c := siblingEdges(ls, members)
	want := []edge{{from: "a", to: "b"}}
	if !reflect.DeepEqual(edges, want) {
		t.Errorf("edges = %v, want %v", edges, want)
	}
	if c.valid != 1 || c.invalid != 1 || c.missing != 1 || c.external != 1 {
		t.Errorf("counts = %+v, want valid 1, invalid 1, missing 1, external 1", c)
	}
}

func TestPnpmPatterns(t *testing.T) {
	src := "packages:\n  - 'apps/*'\n  - \"packages/*\"\n  - crates/*/js # a comment\n  - '!**/test/**'\n\nupdateNotifier: false\npublicHoistPattern:\n  - '*eslint*'\n"
	got := pnpmPatterns([]byte(src))
	want := []string{"apps/*", "packages/*", "crates/*/js", "!**/test/**"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pnpmPatterns = %v, want %v", got, want)
	}
}

func TestExpandWorkspaces(t *testing.T) {
	dirs := []string{"packages/a", "packages/b", "packages/b/nested", "apps/web", "test/esm", "docs"}
	has := map[string]bool{"packages/a": true, "packages/b": true, "packages/b/nested": true, "apps/web": true, "test/esm": true}
	got, unexpanded := expandWorkspaces(dirs, has, []string{"packages/*", "apps/web", "test/*", "!test/esm", "**/x"})
	want := []string{"apps/web", "packages/a", "packages/b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expandWorkspaces = %v, want %v", got, want)
	}
	if unexpanded != 1 {
		t.Errorf("unexpanded = %d, want 1 (the ** pattern is left to the tool)", unexpanded)
	}
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestOutputsAreStrictForTheHarness(t *testing.T) {
	// lang-oracle decodes a tool-oracle file with DisallowUnknownFields; the
	// two files must carry only the shape's own fields, and the references file
	// no cross_references (nor the other way round).
	b, err := json.Marshal(mainOut{Tool: "t", Version: "1", Tier: "A", Definitions: []def{}, References: []ref{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "cross_references") {
		t.Errorf("the definitions file must omit cross_references: %s", b)
	}
	b, err = json.Marshal(xrefOut{Tool: "t", Version: "1", Tier: "C", CrossReferences: []xref{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "definitions") || strings.Contains(string(b), `"references"`) {
		t.Errorf("the cross-references file must carry no definitions or references: %s", b)
	}
}

func TestYarnEdges(t *testing.T) {
	// `yarn workspaces list -v --json`: one object per line, workspace
	// dependencies by location; a descriptor that names a workspace whose range
	// it does not accept is listed as mismatched, never an edge.
	src := `{"location":".","name":"root","workspaceDependencies":[],"mismatchedWorkspaceDependencies":["a@2.0.0"]}
{"location":"packages/a","name":"a","workspaceDependencies":["packages/b"],"mismatchedWorkspaceDependencies":[]}
{"location":"packages/b","name":"b","workspaceDependencies":["packages/a","packages/zz"],"mismatchedWorkspaceDependencies":["c@^9"]}
{"location":"packages/c","name":"c","workspaceDependencies":[],"mismatchedWorkspaceDependencies":[]}
`
	ws, err := parseYarnWorkspaces([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	members := map[string]member{
		"a": {name: "a", dir: "packages/a"},
		"b": {name: "b", dir: "packages/b"},
		"c": {name: "c", dir: "packages/c"},
	}
	edges, c := yarnEdges(ws, members)
	want := []edge{{from: "a", to: "b"}, {from: "b", to: "a"}}
	if !reflect.DeepEqual(edges, want) {
		t.Errorf("edges = %v, want %v", edges, want)
	}
	if c.mismatched != 2 || c.unknownTarget != 1 || c.listed != 3 {
		t.Errorf("counts = %+v, want mismatched 2, unknownTarget 1, listed 3 (the root is no member)", c)
	}
}

func TestYarnPath(t *testing.T) {
	src := "compressionLevel: mixed\n\nplugins:\n  - path: .yarn/plugins/x.cjs\n\nyarnPath: .yarn/releases/yarn-4.10.3.cjs\n"
	if got := yarnPath([]byte(src)); got != ".yarn/releases/yarn-4.10.3.cjs" {
		t.Errorf("yarnPath = %q", got)
	}
	if got := yarnPath([]byte("nodeLinker: node-modules\n")); got != "" {
		t.Errorf("yarnPath without the key = %q, want empty", got)
	}
}

func TestStripDependencies(t *testing.T) {
	src := []byte(`{"name":"a","version":"1.0.0","dependencies":{"b":"workspace:*"},"devDependencies":{"c":"^1"},"peerDependencies":{"d":"^1"},"optionalDependencies":{"e":"1"},"scripts":{"t":"x"}}`)
	out, err := stripDependencies(src)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range depSections {
		if _, ok := m[k]; ok {
			t.Errorf("%s survived", k)
		}
	}
	if string(m["name"]) != `"a"` || string(m["version"]) != `"1.0.0"` || m["scripts"] == nil {
		t.Errorf("name, version and the rest must be kept: %s", out)
	}
}

func TestPnpmSelection(t *testing.T) {
	src := `[{"name":"a","version":"1.0.0","path":"/s/w/packages/a","private":false},{"name":"root","path":"/s/w","private":true},{"name":"b","path":"/s/w/packages/b"}]`
	got, err := pnpmSelection([]byte(src), "/s/w")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".", "packages/a", "packages/b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pnpmSelection = %v, want %v", got, want)
	}
}
