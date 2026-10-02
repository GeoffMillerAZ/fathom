package extractor

// docengine_pattern_test.go — the doc engine's interpolation references
// (LB-F145, wave iac): a reference's or a cross-file reference's optional
// `pattern`, an RE2 with exactly one capture group applied to every scalar
// string the selector reaches (a map or sequence selected is walked for its
// strings, never into another entity's node), every non-overlapping match one
// value. It is opt-in: every other dialect's references read as before (their
// goldens run beside these, unchanged).

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const patternSpec = `{
  "decode": "json",
  "detect": {},
  "entities": [
    {"select": "$", "kind": "package", "source_kind": "PTemplate", "name": "$file"},
    {"select": "$.parameters.*", "where": {"is_map": true}, "kind": "variable_decl", "source_kind": "PParam", "name": "$key"},
    {"select": "$.variables.*", "kind": "variable", "source_kind": "PVar", "name": "$key"},
    {"select": "$.resources[*]", "where": {"has_key": ["type"]}, "kind": "resource", "source_kind": "PResource", "name": ".name"},
    {"select": "$.resources[*].resources[*]", "where": {"has_key": ["type"]}, "kind": "resource", "source_kind": "PResource", "name": ".name"}
  ],
  "references": [
    {"from": "PResource|PVar", "select": "$", "value": "$", "pattern": "\\bparameters\\('([^']+)'\\)", "to_kind": "PParam", "edge": "parameter", "declared_only": true},
    {"from": "PResource|PVar", "select": "$", "value": "$", "pattern": "\\bvariables\\('([^']+)'\\)", "to_kind": "PVar", "edge": "variable", "declared_only": true},
    {"from": "PResource", "select": "$.dependsOn[*]", "value": "fmt:[{$}]", "pattern": "^\\[resourceId\\('[^']*',\\s*((?:parameters|variables)\\('[^']*'\\))\\)\\]$", "to_kind": "PResource", "edge": "depends_on", "declared_only": true}
  ],
  "cross_file": [
    {"from": "PResource", "select": "$.properties.templateLink.uri", "value": "$", "pattern": "^\\[uri\\(deployment\\(\\)\\.properties\\.templateLink\\.uri,\\s*'([^']+)'\\)\\]$", "resolve": "repo-path", "edge": "linked_template"},
    {"select": "$.metadata.notes", "value": "$", "pattern": "\\$\\{file\\(([^)]+)\\)\\}", "resolve": "repo-path", "edge": "include"}
  ]
}`

const patternDoc = `{
  "metadata": {"notes": "see ${file(./a.json)} and ${file(../b/c.json)}, not ${file()}"},
  "parameters": {
    "name": {"type": "string"},
    "location": {"type": "string"}
  },
  "variables": {
    "vnet": "[concat(parameters('name'), '-vnet')]",
    "ghost": "[parameters('missing')]"
  },
  "resources": [
    {
      "type": "Microsoft.Network/virtualNetworks",
      "name": "[variables('vnet')]",
      "location": "[parameters('location')]",
      "dependsOn": ["[resourceId('Microsoft.Network/virtualNetworks', variables('vnet'))]"],
      "properties": {"tags": ["[parameters('name')]", "[concat(parameters('name'), parameters('location'), parameters('name'))]"], "[variables('ghost')]": "a key is a string too"},
      "resources": [
        {"type": "subnets", "name": "default", "properties": {"x": "[variables('ghost')]"}}
      ]
    },
    {
      "type": "Microsoft.Resources/deployments",
      "name": "linked",
      "dependsOn": ["[resourceId('Microsoft.Network/virtualNetworks', variables('vnet'))]", "[resourceId('X/y', 'lit')]", "nope"],
      "properties": {"templateLink": {"uri": "[uri(deployment().properties.templateLink.uri, 'nested/vnet.json')]"}}
    }
  ]
}`

// TestDocEnginePattern pins `pattern` on references: each match's group is
// a value (several in one string, a string selected or reached under a
// selected map), declared_only filters it, a value template reads the
// capture (`fmt:[{$}]`), a string inside a nested entity belongs to that
// entity, not to its parent, and a value naming the referring entity itself
// (a resource whose dependsOn names its own resourceId) is no reference.
func TestDocEnginePattern(t *testing.T) {
	x := mustDoc(t, patternSpec)
	res, err := x.ExtractDoc("svc", "dir/main.json", []byte(patternDoc))
	if err != nil {
		t.Fatal(err)
	}
	got := brief(res.Nodes)
	want := []string{
		"PTemplate main.json@1-29",
		"PParam name@4-4",
		"PParam location@5-5",
		"PVar vnet@8-8 calls=name",
		"PVar ghost@9-9",
		"PResource [variables('vnet')]@12-21 calls=ghost,location,name,vnet",
		"PResource default@19-19 calls=ghost",
		"PResource linked@22-27 calls=[variables('vnet')],vnet",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nodes:\n got %s\nwant %s", strings.Join(got, "\n     "), strings.Join(want, "\n     "))
	}
	var xs []string
	for _, c := range res.CrossFile {
		xs = append(xs, c.From+" "+c.Edge+" "+c.Raw+" -> "+c.Target)
	}
	wantX := []string{
		" include ../b/c.json -> b/c.json", // (line, raw) order
		" include ./a.json -> dir/a.json",
		"linked linked_template nested/vnet.json -> dir/nested/vnet.json",
	}
	if !reflect.DeepEqual(xs, wantX) {
		t.Errorf("cross-file:\n got %q\nwant %q", xs, wantX)
	}
}

// TestDocEnginePatternRefusals: a pattern that does not compile, or whose
// capture-group count is not exactly one, refuses the spec at load, on a
// reference and on a cross-file reference alike.
func TestDocEnginePatternRefusals(t *testing.T) {
	const ent = `"entities":[{"select":"$","kind":"package","source_kind":"K","name":"$file"}]`
	cases := map[string]string{
		`{"decode":"json","detect":{},` + ent + `,"references":[{"from":"K","select":"$","value":"$","to_kind":"K","edge":"e","pattern":"("}]}`:      "references[0]: pattern",
		`{"decode":"json","detect":{},` + ent + `,"references":[{"from":"K","select":"$","value":"$","to_kind":"K","edge":"e","pattern":"abc"}]}`:    "exactly one capture group",
		`{"decode":"json","detect":{},` + ent + `,"references":[{"from":"K","select":"$","value":"$","to_kind":"K","edge":"e","pattern":"(a)(b)"}]}`: "exactly one capture group",
		`{"decode":"json","detect":{},` + ent + `,"cross_file":[{"select":"$","value":"$","resolve":"repo-path","edge":"e","pattern":"[x"}]}`:        "cross_file[0]: pattern",
		`{"decode":"json","detect":{},` + ent + `,"cross_file":[{"select":"$","value":"$","resolve":"repo-path","edge":"e","pattern":"(?:a)b"}]}`:    "exactly one capture group",
		`{"decode":"json","detect":{},` + ent + `,"cross_file":[{"select":"$","value":"$","resolve":"label-selector","edge":"e","pattern":"(a)"}]}`:  "label-selector",
	}
	for spec, want := range cases {
		s, err := ParseDocSpec([]byte(spec))
		if err == nil {
			_, err = NewDocExtractor(s)
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("spec %s: err = %v, want one containing %q", spec, err, want)
		}
	}
	// One named group is one group.
	s, err := ParseDocSpec([]byte(`{"decode":"json","detect":{},` + ent + `,"references":[{"from":"K","select":"$","value":"$","to_kind":"K","edge":"e","pattern":"(?P<n>a)(?:b)"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewDocExtractor(s); err != nil {
		t.Errorf("one named group plus a non-capturing one: %v", err)
	}
}

// TestDocEnginePatternUnset: without a pattern a reference reads its value
// as a whole scalar, as before — a selected map gives no value.
func TestDocEnginePatternUnset(t *testing.T) {
	x := mustDoc(t, `{"decode":"json","detect":{},
	  "entities":[{"select":"$.a.*","kind":"variable","source_kind":"A","name":"$key"}],
	  "references":[{"from":"A","select":"$","value":"$","to_kind":"A","edge":"e"}]}`)
	res, err := x.ExtractDoc("svc", "f.json", []byte(`{"a":{"p":"q","q":{"r":"p"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := brief(res.Nodes)
	want := []string{"A p@1-1 calls=q", "A q@1-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestDocEngineCrossVia pins a cross-file reference's `via`: a value that
// names a declared entity (an ARM deployment's templateLink.uri written as
// `[variables('netUri')]`) is read through that entity's own strings with
// via's pattern; a name no entity of via's kind declares gives nothing.
func TestDocEngineCrossVia(t *testing.T) {
	x := mustDoc(t, `{
	  "decode": "json",
	  "detect": {},
	  "entities": [
	    {"select": "$.variables.*", "kind": "variable", "source_kind": "VVar", "name": "$key"},
	    {"select": "$.resources[*]", "where": {"has_key": ["type"]}, "kind": "resource", "source_kind": "VRes", "name": ".name"}
	  ],
	  "cross_file": [
	    {"from": "VRes", "select": "$.uri", "value": "$", "pattern": "^\\[variables\\('([^']+)'\\)\\]$",
	     "via": {"kind": "VVar", "pattern": "\\buri\\([^,]*,\\s*'([^']+\\.json)'\\)"}, "resolve": "repo-path", "edge": "linked_template"}
	  ]
	}`)
	res, err := x.ExtractDoc("svc", "dir/main.json", []byte(`{
  "variables": {
    "netUri": "[concat(uri(parameters('base'), 'nested/net.json'), parameters('sas'))]",
    "plain": "nothing here"
  },
  "resources": [
    {"type": "Microsoft.Resources/deployments", "name": "net", "uri": "[variables('netUri')]"},
    {"type": "Microsoft.Resources/deployments", "name": "none", "uri": "[variables('plain')]"},
    {"type": "Microsoft.Resources/deployments", "name": "ghost", "uri": "[variables('missing')]"}
  ]
}`))
	if err != nil {
		t.Fatal(err)
	}
	var xs []string
	for _, c := range res.CrossFile {
		xs = append(xs, fmt.Sprintf("%s %s %s -> %s @%d", c.From, c.Edge, c.Raw, c.Target, c.Line))
	}
	want := []string{"net linked_template nested/net.json -> dir/nested/net.json @7"}
	if !reflect.DeepEqual(xs, want) {
		t.Errorf("cross-file via:\n got %q\nwant %q", xs, want)
	}
	for spec, wantErr := range map[string]string{
		`{"decode":"json","detect":{},"entities":[{"select":"$","kind":"package","source_kind":"K","name":"$file"}],"cross_file":[{"select":"$","value":"$","pattern":"(a)","via":{"kind":"Nope","pattern":"(b)"},"resolve":"repo-path","edge":"e"}]}`: "via",
		`{"decode":"json","detect":{},"entities":[{"select":"$","kind":"package","source_kind":"K","name":"$file"}],"cross_file":[{"select":"$","value":"$","pattern":"(a)","via":{"kind":"K","pattern":"b"},"resolve":"repo-path","edge":"e"}]}`:      "exactly one capture group",
		`{"decode":"json","detect":{},"entities":[{"select":"$","kind":"package","source_kind":"K","name":"$file"}],"cross_file":[{"select":"$","value":"$","via":{"kind":"K","pattern":"(b)"},"resolve":"repo-path","edge":"e"}]}`:                    "via needs a pattern",
	} {
		s, err := ParseDocSpec([]byte(spec))
		if err == nil {
			_, err = NewDocExtractor(s)
		}
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("spec %s: err = %v, want one containing %q", spec, err, wantErr)
		}
	}
}
