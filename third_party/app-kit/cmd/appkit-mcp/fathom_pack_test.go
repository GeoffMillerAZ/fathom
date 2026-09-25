package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

type kitGuard struct {
	Scopes         []string `json:"scopes"`
	CommandScopes  []string `json:"command_scopes"`
	Note           string   `json:"note"`
	MaxRules       int      `json:"max_rules"`
	IncludeModules bool     `json:"include_modules"`
}

func loadKitGuards(t *testing.T, rel string) []kitGuard {
	t.Helper()
	text := readFile(t, filepath.Join(kitRootRel, rel))
	v := cuecontext.New().CompileString(text)
	var out struct {
		Guards []kitGuard `json:"guards"`
	}
	if err := v.Decode(&out); err != nil {
		t.Fatalf("decode %s: %v", rel, err)
	}
	return out.Guards
}

func staleKitGuidance(text string) string {
	normal := strings.ToLower(strings.Join(strings.Fields(text), " "))
	for _, phrase := range []string{
		"dark: color variants are dropped", "dark: color variants dropped",
		"palette families become roles", "ui markup is never hand-written",
		"never a runtime js dep", "tailwindplus and matdash ship no charting",
		"conversion machinery is planned, not an available command",
	} {
		if strings.Contains(normal, phrase) {
			return phrase
		}
	}
	return ""
}

func TestFathomPackCurrentGuidance(t *testing.T) {
	for _, rel := range []string{
		"fathom-pack/guard-pack.cue", "fathom-pack/layers/ui-appkit/prompt.md",
		"AGENTS.md", "CLAUDE.md",
	} {
		t.Run(rel, func(t *testing.T) {
			text := readFile(t, filepath.Join(kitRootRel, rel))
			if phrase := staleKitGuidance(text); phrase != "" {
				t.Fatalf("retired guidance %q in %s", phrase, rel)
			}
			for _, required := range []string{"ADR-0022", "ADR-0023"} {
				if !strings.Contains(text, required) {
					t.Errorf("%s must route %s", rel, required)
				}
			}
		})
	}
}

func TestFathomPackRetiredGuidanceCanaries(t *testing.T) {
	for _, bad := range []string{
		"dark: color variants are DROPPED", "dark: color variants dropped",
		"palette families become ROLES", "UI markup is never hand-written",
		"never a runtime JS dep", "TailwindPlus and MatDash ship NO charting",
		"Conversion machinery is planned, not an available command",
	} {
		if staleKitGuidance(bad) == "" {
			t.Errorf("missed retired instruction: %s", bad)
		}
	}
	if got := staleKitGuidance("Preserve originals; choose a consumer profile under ADR-0022/ADR-0023."); got != "" {
		t.Fatalf("false refusal: %s", got)
	}
}

func TestFathomPackLocalGuardParityAndBudgets(t *testing.T) {
	shipped := loadKitGuards(t, "fathom-pack/guard-pack.cue")
	local := loadKitGuards(t, ".fathom/guard.cue")
	if len(shipped) != 10 || len(local) != 6 {
		t.Fatalf("want four consumer and six kit guards, got shipped=%d local=%d", len(shipped), len(local))
	}
	if !reflect.DeepEqual(shipped[4:], local) {
		t.Fatal("local guards drifted from shipped kit-self group; review and synchronize both")
	}
	for i, g := range shipped {
		if len(g.Scopes)+len(g.CommandScopes) == 0 || strings.TrimSpace(g.Note) == "" {
			t.Errorf("guard %d needs scope and note", i)
		}
		if g.MaxRules < 1 || g.MaxRules > 4 || len(strings.Fields(g.Note)) > 100 {
			t.Errorf("guard %d exceeds attention budget", i)
		}
		if g.IncludeModules {
			t.Errorf("guard %d must not imply an unproven module binding", i)
		}
	}
}

func TestFathomSemanticPackDoesNotInventEvaluation(t *testing.T) {
	text := readFile(t, filepath.Join(kitRootRel, "fathom-pack/layers/ui-appkit/layer.cue"))
	var def struct {
		Version string `json:"version"`
		Evaled  bool   `json:"evaled"`
		Corpus  struct {
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		} `json:"corpus"`
	}
	if err := cuecontext.New().CompileString(text).Decode(&def); err != nil {
		t.Fatal(err)
	}
	if def.Evaled {
		t.Fatal("current structural tests do not establish real-model precision/recall evaluation")
	}
	if def.Version != "v2" || len(def.Corpus.Exclude) == 0 {
		t.Fatal("corrected pack needs v2 identity and historical/private corpus exclusions")
	}
	for _, p := range def.Corpus.Include {
		if p == "docs/**" || p == "**" {
			t.Fatal("broad historical corpus can reintroduce superseded policy")
		}
	}
}

func missingGuidance(text string, required []string) string {
	for _, phrase := range required {
		if !strings.Contains(text, phrase) {
			return phrase
		}
	}
	return ""
}

func TestFathomPackPositiveFidelityAndScope(t *testing.T) {
	guards := loadKitGuards(t, "fathom-pack/guard-pack.cue")
	cases := []struct {
		name, text string
		required   []string
	}{
		{"consumer", guards[0].Note, []string{"Preserve original hierarchy and interaction states", "review semantic roles contextually", "originals remain valid alternatives"}},
		{"kit", guards[6].Note, []string{"preserve private originals", "Refuse unsupported conversion explicitly", "Do not weaken the oracle", "browser/behavior certification"}},
		{"prompt", readFile(t, filepath.Join(kitRootRel, "fathom-pack/layers/ui-appkit/prompt.md")), []string{"preserve originals", "hierarchy, typography and interaction states", "contextual design review", "profiles are valid", "original access", "not target certification"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if missing := missingGuidance(tc.text, tc.required); missing != "" {
				t.Fatalf("lost positive fidelity instruction %q", missing)
			}
			for _, phrase := range tc.required {
				if missingGuidance(strings.ReplaceAll(tc.text, phrase, ""), tc.required) == "" {
					t.Fatalf("deletion canary accepted loss of %q", phrase)
				}
			}
		})
	}
	for _, scope := range []string{"internal/designassets/**", "internal/designconvert/**", "internal/designattempts/**", "apps/gallery/pricing*", "apps/gallery/conversions*"} {
		if !strings.Contains("\n"+strings.Join(guards[6].Scopes, "\n")+"\n", "\n"+scope+"\n") {
			t.Errorf("fidelity guard omits %s", scope)
		}
	}
}

// This protects the observed literal-root exclusion regression, not every
// possible gitignore expression. Re-ingest and query a source sentinel too.
func TestFathomIgnoreKeepsSourceRoots(t *testing.T) {
	text := readFile(t, filepath.Join(kitRootRel, ".fathomignore"))
	for _, line := range strings.Split(text, "\n") {
		pattern := strings.TrimSpace(line)
		if pattern == "" || strings.HasPrefix(pattern, "#") || strings.HasPrefix(pattern, "!") {
			continue
		}
		pattern = strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "**"), "/")
		for _, root := range []string{"appkit", "apps", "cmd", "internal", "cue", "docs"} {
			if info, err := os.Stat(filepath.Join(kitRootRel, root)); err != nil || !info.IsDir() {
				t.Fatalf("source sentinel %s unavailable: %v", root, err)
			}
			if pattern == root {
				t.Errorf("%q excludes source root %s; target the actual executable path", line, root)
			}
		}
	}
}

func TestFathomIntentStableIDsAndSurfaces(t *testing.T) {
	for _, tc := range []struct{ file, pattern, marker string }{
		{"scope.md", `(?m)^\*\*(G\d+)\.`, "**Done when:**"},
		{"futures.md", `(?m)^## (F\d+)\.`, "**Promote when:**"},
	} {
		text := readFile(t, filepath.Join(kitRootRel, "docs/intent", tc.file))
		ids := regexp.MustCompile(tc.pattern).FindAllStringSubmatch(text, -1)
		if len(ids) == 0 || len(ids) != strings.Count(text, tc.marker) {
			t.Errorf("%s needs one parsed stable ID per %s", tc.file, tc.marker)
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id[1]] {
				t.Errorf("%s repeats %s", tc.file, id[1])
			}
			seen[id[1]] = true
		}
	}
	router := readFile(t, filepath.Join(kitRootRel, "docs/intent/README.md"))
	parts := strings.SplitN(router, "## Sensitive surfaces\n", 2)
	if len(parts) != 2 {
		t.Fatal("missing sensitive-surface table")
	}
	section := strings.SplitN(parts[1], "\n## ", 2)[0]
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| Surface |") {
			continue
		}
		if cells := strings.Split(line, "|"); len(cells) < 5 || !strings.Contains(cells[1], "`") {
			t.Errorf("sensitive surface needs explicit backticked file globs: %s", line)
		}
	}
}
