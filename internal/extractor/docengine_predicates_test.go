package extractor

// docengine_predicates_test.go — K20 (wave c2, item engine): the document-path
// engine's value predicates, universal (K15) and opt-in (design
// docs/design/language-breadth.md §4): a key-absent selector
// (`where.not_has_key`, O631), an anchored RE2 value predicate
// (`where.matches`, compiled once at load, an invalid one refused there), a
// k=v split of a string value (the `kv:<KEY>` transform), template refusal for
// identity captures (`refuse_templates`) and one entity per name per file
// (`unique`). A spec that sets none of them extracts as before: every other
// dialect's goldens run beside these, unchanged.

import (
	"sort"
	"strconv"
	"strings"
	"testing"
)

// predicateNames is every entity of source kind sk, "name|signature@line",
// in document order.
func predicateNames(t *testing.T, spec, rel, src, sk string) []string {
	t.Helper()
	x := mustDoc(t, spec)
	res, err := x.ExtractDoc("svc", rel, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range res.Nodes {
		if string(n.Kind) == sk {
			s := n.Name
			if n.Signature != "" {
				s += "|" + n.Signature
			}
			out = append(out, s+"@"+strconv.Itoa(n.Line))
		}
	}
	return out
}

func eqList(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

// TestDocWhereNotHasKey: a key-absent selector keeps a node only when none of
// the listed keys is one of its entries; a node that is not a map has none. A
// step with both an id and a run is the id's entity alone (O631).
func TestDocWhereNotHasKey(t *testing.T) {
	spec := `{"decode": "yaml", "detect": {}, "entities": [
	  {"select": "$.steps[*]", "where": {"is_map": true, "has_key": ["id"]}, "kind": "step", "source_kind": "IdStep", "name": ".id"},
	  {"select": "$.steps[*]", "where": {"is_map": true, "has_key": ["run"], "not_has_key": ["id"]}, "kind": "step", "source_kind": "RunStep", "name": "fmt:steps#{$index}"},
	  {"select": "$.tags[*]", "where": {"not_has_key": ["x"]}, "kind": "key", "source_kind": "Tag", "name": "$"}
	]}`
	src := "steps:\n  - run: make\n  - id: build\n    run: make build\n  - id: only\n  - run: make test\n    name: t\n  - id: ~\n    run: x\ntags: [a, b]\n"
	eqList(t, "run steps", predicateNames(t, spec, "w.yml", src, "RunStep"), []string{"steps#0@2", "steps#3@6"})
	eqList(t, "id steps", predicateNames(t, spec, "w.yml", src, "IdStep"), []string{"build@3", "only@5"})
	eqList(t, "a scalar has no keys", predicateNames(t, spec, "w.yml", src, "Tag"), []string{"a@10", "b@10"})
}

// TestDocWhereMatches: a value predicate is an RE2 the engine anchors (^...$),
// so a partial match does not hold; `$` tests the node itself, a selector a
// scalar under it; several predicates (and equals beside them) must all hold.
func TestDocWhereMatches(t *testing.T) {
	spec := `{"decode": "yaml", "detect": {}, "entities": [
	  {"select": "$.items[*]", "where": {"is_scalar": true, "matches": {"$": "v[0-9]+"}}, "kind": "key", "source_kind": "Ver", "name": "$"},
	  {"select": "$.steps[*]", "where": {"is_map": true, "matches": {".uses": "[a-z]+/[a-z]+@[0-9a-f]{4}", ".name": "pin.*"}, "equals": {".kind": "remote"}}, "kind": "resource", "source_kind": "Pin", "name": ".uses"}
	]}`
	src := "items: [v1, xv2, v3x, v42, 7]\nsteps:\n  - {uses: a/b@00ff, name: pinned, kind: remote}\n  - {uses: a/b@00ff1, name: pinned, kind: remote}\n  - {uses: a/b@00ff, name: other, kind: remote}\n  - {uses: a/b@00ff, name: pinned, kind: local}\n  - {uses: [a/b@00ff], name: pinned, kind: remote}\n"
	eqList(t, "anchored", predicateNames(t, spec, "f.yml", src, "Ver"), []string{"v1@1", "v42@1"})
	eqList(t, "every predicate holds", predicateNames(t, spec, "f.yml", src, "Pin"), []string{"a/b@00ff@3"})
}

// TestDocWhereMatchesRefusedAtLoad: an RE2 that does not compile is refused
// when the spec loads (an entity's, a cross-file reference's and a label set's
// where alike), naming the field, never at extraction.
func TestDocWhereMatchesRefusedAtLoad(t *testing.T) {
	for name, spec := range map[string]string{
		"entity": `{"decode": "yaml", "detect": {}, "entities": [
		  {"select": "$.a", "where": {"matches": {"$": "v[0-9"}}, "kind": "key", "source_kind": "K", "name": "$"}]}`,
		"cross_file": `{"decode": "yaml", "detect": {}, "entities": [{"select": "$", "kind": "package", "source_kind": "F", "name": "$file"}],
		  "cross_file": [{"where": {"matches": {".x": "(a"}}, "select": "$.a", "value": "$", "resolve": "repo-path", "edge": "e"}]}`,
		"labels": `{"decode": "yaml", "detect": {}, "entities": [{"select": "$", "kind": "package", "source_kind": "F", "name": "$file"}],
		  "labels": [{"from": "F", "where": {"matches": {".k": "*"}}, "select": "$.labels"}]}`,
		"bad selector": `{"decode": "yaml", "detect": {}, "entities": [
		  {"select": "$.a", "where": {"matches": {"x": "a"}}, "kind": "key", "source_kind": "K", "name": "$"}]}`,
	} {
		s, err := ParseDocSpec([]byte(spec))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := NewDocExtractor(s); err == nil || !strings.Contains(err.Error(), "where.matches") {
			t.Errorf("%s: NewDocExtractor = %v, want a where.matches error", name, err)
		}
	}
}

// TestDocValueKV: `kv:<KEY>` reads a string as comma-separated key=value
// pairs and gives KEY's value: an OTEL_RESOURCE_ATTRIBUTES service.name, a
// Compose list element `NAME=value`. No pair named KEY gives nothing (the
// next alternative is tried); a comma followed by no `=` belongs to the value;
// transforms chain left to right, and cut applies last.
func TestDocValueKV(t *testing.T) {
	spec := `{"decode": "yaml", "detect": {}, "entities": [
	  {"select": "$.attrs[*]", "kind": "key", "source_kind": "Svc", "name": "$ kv:service.name | $ kv:ATTRS kv:service.name", "signature": "fmt:service.name={$ kv:service.name | $ kv:ATTRS kv:service.name}"},
	  {"select": "$.env[*]", "where": {"matches": {"$": "APP=.*"}}, "kind": "key", "source_kind": "App", "name": "$ kv:APP cut:;"}
	]}`
	src := "attrs:\n  - service.name=checkout,deployment.environment=prod\n  - deployment.environment=prod, service.name = cart \n  - ATTRS=service.name=ledger,team=x\n  - team=x\n  - service.name=\nenv:\n  - APP=a,b;rollup\n  - APP=\n  - XAPP=q\n  - APP\n"
	eqList(t, "service.name", predicateNames(t, spec, "c.yml", src, "Svc"),
		[]string{"checkout|service.name=checkout@2", "cart|service.name=cart@3", "ledger|service.name=ledger@4"})
	eqList(t, "list element", predicateNames(t, spec, "c.yml", src, "App"), []string{"a,b@8"})
	if _, err := parseDocValue("$ kv:"); err == nil {
		t.Error(`"$ kv:" (no key) parsed`)
	}
	if _, err := parseDocValue("$ @lower"); err == nil {
		t.Error("an unknown transform parsed")
	}
}

// TestDocRefuseTemplates: with refuse_templates, a value decided at deploy
// time yields no entity: one carrying `${`, `{{` or `$(`, or read from a
// scalar with a YAML local tag (`!Ref`, `!Sub`); a literal is kept, and a
// literal cut out of a templated string is kept too. Without the flag the
// value is emitted as written (opt-in).
func TestDocRefuseTemplates(t *testing.T) {
	const ents = `{"select": "$.env.*", "where": {"is_scalar": true}, "kind": "key", "source_kind": "Id", "name": "$ cut:;"%s}`
	src := "env:\n  a: checkout\n  b: ${NAME:-x}\n  c: '{{ .Values.name }}'\n  d: $(NAME)\n  e: !Ref FnName\n  f: !Sub '${AWS::StackName}-fn'\n  g: ledger;${ROLLUP}\n  h: !!str cart\n"
	on := `{"decode": "yaml", "detect": {}, "entities": [` + strings.Replace(ents, "%s", `, "refuse_templates": true`, 1) + `]}`
	off := `{"decode": "yaml", "detect": {}, "entities": [` + strings.Replace(ents, "%s", "", 1) + `]}`
	eqList(t, "refused", predicateNames(t, on, "c.yml", src, "Id"), []string{"checkout@2", "ledger@8", "cart@9"})
	all := predicateNames(t, off, "c.yml", src, "Id")
	if len(all) != 8 {
		t.Errorf("without refuse_templates every value is emitted as written: %q", all)
	}
}

// TestDocEntityUnique: a unique entity is kept once per name and source kind
// in a file, at its first occurrence; another kind of the same name, and a
// second file, are untouched.
func TestDocEntityUnique(t *testing.T) {
	spec := `{"decode": "yaml", "detect": {}, "entities": [
	  {"select": "$.steps[*]", "where": {"has_key": ["uses"]}, "kind": "resource", "source_kind": "Act", "name": ".uses"},
	  {"select": "$.steps[*]", "where": {"has_key": ["uses"]}, "kind": "resource", "source_kind": "Pin", "name": "fmt:pin:{.uses}", "unique": true}
	]}`
	src := "steps:\n  - uses: a/b@v1\n  - uses: c/d@v2\n  - uses: a/b@v1\n"
	eqList(t, "pins", predicateNames(t, spec, "w.yml", src, "Pin"), []string{"pin:a/b@v1@2", "pin:c/d@v2@3"})
	eqList(t, "actions", predicateNames(t, spec, "w.yml", src, "Act"), []string{"a/b@v1@2", "c/d@v2@3", "a/b@v1@4"})
}

// TestDocPredicatesAreOptIn: a spec using none of the K20 fields extracts the
// same nodes as before on a document that would exercise them (the generic
// half of "every other dialect's goldens are byte-identical").
func TestDocPredicatesAreOptIn(t *testing.T) {
	spec := `{"decode": "yaml", "detect": {}, "entities": [
	  {"select": "$.steps[*]", "where": {"is_map": true, "has_key": ["run"]}, "kind": "step", "source_kind": "S", "name": "fmt:s#{$index}", "signature": ".run"},
	  {"select": "$.env.*", "kind": "key", "source_kind": "E", "name": "$"}
	]}`
	src := "steps:\n  - id: a\n    run: make\n  - run: make\nenv:\n  A: ${X}\n  B: !Ref Y\n  C: k=v,x=y\n"
	got := predicateNames(t, spec, "w.yml", src, "S")
	got = append(got, predicateNames(t, spec, "w.yml", src, "E")...)
	sort.Strings(got)
	eqList(t, "opt-in", got, []string{"${X}@6", "Y@7", "k=v,x=y@8", "s#0|make@2", "s#1|make@4"})
}
