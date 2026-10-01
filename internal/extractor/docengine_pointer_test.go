package extractor

// docengine_pointer_test.go — the doc engine's extension for OpenAPI
// (LB-F90, LB-Q-openapi, design §4): a `$ref` key in a selector, the
// `$parent` and `@upper`/`@file` value forms and `fmt:` templates, and a
// reference's `pointer` (a local JSON Pointer read as an entity name by its
// first segment). Each is opt-in: every other dialect's selectors and values
// parse and read as before (their goldens are the proof, run beside these).

import (
	"reflect"
	"strings"
	"testing"
)

const pointerSpec = `{
  "decode": "yaml",
  "detect": {},
  "entities": [
    {"select": "$.defs.*", "include_null": true, "kind": "type_alias", "source_kind": "PSchema", "name": "$key"},
    {"select": "$.params.*", "where": {"is_map": true}, "kind": "variable_decl", "source_kind": "PParam", "name": "$key"},
    {"select": "$.ops.*.*", "where": {"key_in": ["get", "post"]}, "kind": "route", "source_kind": "POp", "name": "fmt:{$key @upper} {$parent}", "signature": ".id|.summary"}
  ],
  "references": [
    {"from": "POp|PSchema", "select": "$..$ref", "value": "$", "edge": "ref", "declared_only": true,
     "pointer": [{"prefix": "#/defs/", "to_kind": "PSchema"}, {"prefix": "#/params/", "to_kind": "PParam"}]}
  ],
  "cross_file": [
    {"select": "$..$ref", "value": "$ @file", "resolve": "repo-path", "edge": "ref"}
  ]
}`

const pointerDoc = `defs:
  Pet:
    properties:
      owner:
        $ref: '#/defs/Owner'
      tags:
        items:
          $ref: ./common.yaml#/Tag
  Owner:
    type: object
  Alias:
    $ref: '#/defs/a~1b'
  a/b: {}
  Gone:
params:
  Limit:
    name: limit
ops:
  /pets:
    get:
      id: listPets
      parameters:
        - $ref: '#/params/Limit'
        - $ref: '#/params/Missing'
        - $ref: '#/defs/Pet/properties/owner'
        - $ref: https://example.com/spec.yaml#/defs/Pet
      responses:
        "200":
          schema:
            $ref: 'other.yaml'
    parameters: []
    x-note: {}
  /pets/{id}:
    post:
      summary: make
`

// TestDocEnginePointerAndTemplates: a `$ref` selects, a fragment-only value
// is an entity name (first segment, unescaped), a file part is the
// cross-file value, and a `fmt:` name joins the method and its path.
func TestDocEnginePointerAndTemplates(t *testing.T) {
	x := mustDoc(t, pointerSpec)
	r, err := x.ExtractDoc("s", "api/spec.yaml", []byte(pointerDoc))
	if err != nil {
		t.Fatal(err)
	}
	got := brief(r.Nodes)
	want := []string{
		"PSchema Pet@2-8 calls=Owner",
		"PSchema Owner@9-10",
		"PSchema Alias@11-12 calls=a/b",
		"PSchema a/b@13-13",
		"PSchema Gone@14-14",
		"PParam Limit@16-17",
		"POp GET /pets@20-30 sig=listPets calls=Limit,Pet",
		"POp POST /pets/{id}@34-35 sig=make",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("symbols\n got: %q\nwant: %q", got, want)
	}
	var raws []string
	for _, c := range r.CrossFile {
		raws = append(raws, c.Raw+" -> "+c.Target)
	}
	if len(r.CrossFile) != 2 {
		t.Fatalf("cross_file: %q (a URL and a local pointer name no file)", raws)
	}
	for _, c := range r.CrossFile {
		switch c.Raw {
		case "./common.yaml":
			if c.Target != "api/common.yaml" {
				t.Errorf("target %q", c.Target)
			}
		case "other.yaml":
			if c.Target != "api/other.yaml" {
				t.Errorf("target %q", c.Target)
			}
		default:
			t.Errorf("unexpected cross_file raw %q", c.Raw)
		}
	}
}

// TestDocEnginePointerSpecRefusals: the new fields are checked at compile.
func TestDocEnginePointerSpecRefusals(t *testing.T) {
	base := `{"decode":"yaml","detect":{},"entities":[{"select":"$.a.*","kind":"key","source_kind":"K","name":"$key"}],"references":[%s]}`
	for ref, want := range map[string]string{
		`{"from":"K","select":"$","value":"$","edge":"e","to_kind":"K","pointer":[{"prefix":"#/a/","to_kind":"K"}]}`: "to_kind",
		`{"from":"K","select":"$","value":"$","edge":"e","pointer":[{"prefix":"#/a/","to_kind":"nope"}]}`:            "pointer[0]",
		`{"from":"K","select":"$","value":"$","edge":"e","pointer":[{"prefix":"","to_kind":"K"}]}`:                   "pointer[0]",
		`{"from":"K|nope","select":"$","value":"$","edge":"e","to_kind":"K"}`:                                        "names no entity kind",
		`{"from":"K","select":"$","value":"$ @lower","edge":"e","to_kind":"K"}`:                                      "unknown transform",
		`{"from":"K","select":"$","value":"fmt:{$key","edge":"e","to_kind":"K"}`:                                     "unclosed",
	} {
		spec := strings.Replace(base, "%s", ref, 1)
		s, err := ParseDocSpec([]byte(spec))
		if err == nil {
			_, err = NewDocExtractor(s)
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ref %s: err = %v, want one containing %q", ref, err, want)
		}
	}
}

// TestLocalPointerTarget: the unescaping of a pointer segment.
func TestLocalPointerTarget(t *testing.T) {
	ps := []DocPointer{{"#/definitions/", "S"}, {"#/components/schemas/", "S"}}
	for in, want := range map[string]string{
		"#/definitions/Pet":                     "Pet",
		"#/components/schemas/Pet/properties/x": "Pet",
		"#/components/schemas/a~1b":             "a/b",
		"#/components/schemas/t~0x":             "t~x",
		"#/components/schemas/Pet%20Store":      "Pet Store",
		"#/components/responses/Pet":            "",
		"other.yaml#/definitions/Pet":           "",
		"":                                      "",
	} {
		if got, _ := localPointerTarget(ps, in); got != want {
			t.Errorf("localPointerTarget(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"./a/b.yaml#/Pet": "./a/b.yaml", "b.yaml": "b.yaml", "#/definitions/Pet": "",
		"https://x.test/a.yaml#/P": "", "": "",
	} {
		if got := fileOfRef(in); got != want {
			t.Errorf("fileOfRef(%q) = %q, want %q", in, got, want)
		}
	}
}
