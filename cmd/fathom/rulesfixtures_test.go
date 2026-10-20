package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixturePackDir(t *testing.T, fixtures string) string {
	t.Helper()
	dir := t.TempDir()
	pack := `pack: {name: "demo", version: 2, about: "demo pack"}
guards: [{scopes: ["**/*.templ"], note: "n", max_rules: 1}]
`
	if err := os.WriteFile(filepath.Join(dir, "pack.cue"), []byte(pack), 0o644); err != nil {
		t.Fatal(err)
	}
	if fixtures != "" {
		_ = os.MkdirAll(filepath.Join(dir, "fixtures"), 0o755)
		if err := os.WriteFile(filepath.Join(dir, "fixtures", "g.cue"), []byte(fixtures), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCmdRulesFixturesPrintsReading(t *testing.T) {
	good := `guards: [{guard: 0, match: ["a.templ"], nomatch: ["a.go"]}]`
	var out, errb bytes.Buffer
	if rc := cmdRules([]string{"fixtures", fixturePackDir(t, good)}, &out, &errb); rc != 0 || !strings.Contains(out.String(), "served: guards (1 of 1 passed)") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
	out.Reset()
	if rc := cmdRules([]string{"fixtures", fixturePackDir(t, ""), "--json"}, &out, &errb); rc != 1 {
		t.Fatalf("unfixtured: rc=%d", rc)
	}
	var got struct {
		Sections []struct {
			Section  string
			Withheld bool
		}
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got.Sections) != 1 || !got.Sections[0].Withheld {
		t.Fatalf("json %q err=%v", out.String(), err)
	}
	out.Reset()
	if rc := cmdRules([]string{"fixtures", t.TempDir()}, &out, &errb); rc != 2 {
		t.Fatalf("invalid pack: rc=%d", rc)
	}
	if rc := cmdRules([]string{"fixtures"}, &out, &errb); rc != 2 {
		t.Fatalf("no dir: rc=%d", rc)
	}
}
