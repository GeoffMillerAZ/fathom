package projectdir_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

const packHeader = `pack: {name: "k", version: 1, about: "a kit"}` + "\n"

// #Pack is the one format: a header plus the sections a kit may carry, and
// closed against everything only a repository governs.
func TestValidatePackAcceptsKitSectionsAndRefusesRepositoryKeys(t *testing.T) {
	ok := map[string]string{
		"header alone": packHeader,
		"all sections": packHeader + `
guards: [{scopes: ["**/*.templ"], note: "n"}]
generated: [{output: "{dir}/{name}_templ.go", source: ["{dir}/{name}.templ"], regenerate: "templ generate"}]
formats: [{dialect: "templ", globs: ["**/*.templ"]}]
concerns: c: {want: "w", why: "y", check: {kind: "max_lines", file: "AGENTS.md", lines: 5}}
`,
	}
	for name, src := range ok {
		if err := projectdir.ValidatePack([]byte(src), name+".cue"); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad := map[string]string{
		"no header":      `guards: [{scopes: ["a"]}]`,
		"layer_rules":    packHeader + `layer_rules: []`,
		"capabilities":   packHeader + `capabilities: {}`,
		"bindings":       packHeader + `bindings: []`,
		"exclude_packs":  packHeader + `exclude_packs: ["x"]`,
		"orientation":    packHeader + `orientation: [{anchor: "a", why: "b"}]`,
		"empty sources":  packHeader + `generated: [{output: "o", source: [], regenerate: "r"}]`,
		"version zero":   `pack: {name: "k", version: 0, about: "a"}`,
		"unknown check":  packHeader + `concerns: c: check: kind: "nope"`,
		"empty globs":    packHeader + `formats: [{dialect: "d", globs: []}]`,
		"header unknown": `pack: {name: "k", version: 1, about: "a", extra: 1}`,
	}
	for name, src := range bad {
		if err := projectdir.ValidatePack([]byte(src), name+".cue"); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.Contains(err.Error(), "tech pack") && name != "version zero" {
			t.Logf("%s: %v", name, err)
		}
	}
}
