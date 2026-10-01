package main

import (
	"reflect"
	"testing"
)

func TestOptionKeysFromEmbeddedSchema(t *testing.T) {
	bin := append([]byte("\x00\x01garbage"), schemaMarker...)
	bin = append(bin, []byte(`,
  "properties": {
    "min_version": {"type": "string"},
    "templates": {"type": "object"},
    "pre-commit": {"$ref": "#/$defs/Hook"},
    "commit-msg": {"$ref": "#/$defs/Hook"}
  }
}trailing binary bytes`)...)
	got, err := optionKeys(bin)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]bool{"min_version": true, "templates": true}; !reflect.DeepEqual(got, want) {
		t.Errorf("optionKeys = %v, want %v", got, want)
	}
	if _, err := optionKeys([]byte("no schema here")); err == nil {
		t.Error("a binary without the schema must fail")
	}
}

func TestDumpEntities(t *testing.T) {
	dump := []byte(`{
  "colors": false,
  "templates": {"x": "y"},
  "pre-commit": {
    "commands": {"lint": {"run": "x"}, "nullcmd": null},
    "scripts": {"a.sh": {"runner": "bash"}},
    "jobs": [
      {"name": "first", "run": "1"},
      {"run": "unnamed"},
      {"name": "g", "group": {"jobs": [{"name": "inner", "run": "2"}, {"script": "s.sh"}]}}
    ]
  },
  "custom": {"commands": {"smoke": {"run": "s"}}}
}`)
	ents, unnamed, err := dumpEntities(dump, map[string]bool{"colors": true, "templates": true})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range ents {
		got = append(got, e.kind+":"+e.name+"@"+pathKey(e.path))
	}
	want := []string{
		"stage:custom@s:custom",
		"job:smoke@s:custom\x00s:commands\x00s:smoke",
		"stage:pre-commit@s:pre-commit",
		"job:lint@s:pre-commit\x00s:commands\x00s:lint",
		"job:nullcmd@s:pre-commit\x00s:commands\x00s:nullcmd",
		"job:a.sh@s:pre-commit\x00s:scripts\x00s:a.sh",
		"job:first@s:pre-commit\x00s:jobs\x00i:0",
		"job:g@s:pre-commit\x00s:jobs\x00i:2",
		"job:inner@s:pre-commit\x00s:jobs\x00i:2\x00s:group\x00s:jobs\x00i:0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entities\n got: %q\nwant: %q", got, want)
	}
	if unnamed != 2 {
		t.Errorf("unnamed = %d, want 2", unnamed)
	}
}

func TestEntityLine(t *testing.T) {
	ls := lines{
		pathKey([]any{"pre-commit"}):                     {line: 2, keyLine: 1},
		pathKey([]any{"pre-commit", "jobs", float64(0)}): {line: 4, keyLine: 0},
	}
	if l, ok := (entity{path: []any{"pre-commit"}}).lineIn(ls); !ok || l != 1 {
		t.Errorf("hook line = %d %v, want its key's line 1", l, ok)
	}
	if l, ok := (entity{path: []any{"pre-commit", "jobs", 0}, itemLine: true}).lineIn(ls); !ok || l != 4 {
		t.Errorf("job line = %d %v, want the item's line 4", l, ok)
	}
	if _, ok := (entity{path: []any{"pre-push"}}).lineIn(ls); ok {
		t.Error("a path the decode lacks has no line")
	}
}
