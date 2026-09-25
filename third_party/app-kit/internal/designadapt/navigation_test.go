package designadapt

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Invented markup/copy: only the reviewed element shape and wrapping token match.
const syntheticNavigation = `<div><div class="fixture-narrow"><select aria-label="Choose fixture page" class="fixture-select"><option>One</option> <option>Two</option> <option selected>Three</option> <option>Four</option></select><svg aria-hidden="true"><path d="M1 1h1" /></svg></div><div><div><nav aria-label="Fixture pages" class="fixture-nav"><a href="#" class="fixture-link whitespace-nowrap">One</a> <a href="#" class="fixture-link whitespace-nowrap">Two</a> <a href="#" aria-current="page" class="fixture-current whitespace-nowrap">Three</a> <a href="#" class="fixture-link whitespace-nowrap">Four</a></nav></div></div></div>`

func TestNavigationSourceStructureAndReplayRefusals(t *testing.T) {
	source := syntheticSource(syntheticNavigation)
	if _, err := AdaptNavigation(source, []byte(syntheticNavigation)); err == nil {
		t.Fatal("synthetic accepted by frozen entrypoint")
	}
	source.ArchiveID, source.Path, source.SHA256 = NavigationArchive, NavigationMember, NavigationSourceSHA
	if _, err := AdaptNavigation(source, []byte(syntheticNavigation)); err == nil {
		t.Fatal("spoofed source accepted")
	}
	for name, body := range map[string]string{
		"missing-select-label":    strings.Replace(syntheticNavigation, ` aria-label="Choose fixture page"`, "", 1),
		"mismatched-label":        strings.Replace(syntheticNavigation, `>One</a>`, `>Wrong</a>`, 1),
		"missing-selected":        strings.Replace(syntheticNavigation, ` selected`, "", 1),
		"multiple-selected":       strings.Replace(syntheticNavigation, `<option>One`, `<option selected>One`, 1),
		"missing-current":         strings.Replace(syntheticNavigation, ` aria-current="page"`, "", 1),
		"multiple-current":        strings.Replace(syntheticNavigation, `<a href="#"`, `<a aria-current="page" href="#"`, 1),
		"unnamed-mode":            strings.Replace(syntheticNavigation, `<select `, `<select name="secret" `, 1),
		"runtime":                 strings.Replace(syntheticNavigation, `<select `, `<select onchange="bad()" `, 1),
		"unexpected-option-value": strings.Replace(syntheticNavigation, `<option>One`, `<option value="one">One`, 1),
		"missing-wrapping-map":    strings.ReplaceAll(syntheticNavigation, " whitespace-nowrap", ""),
		"unknown-default-class":   strings.Replace(syntheticNavigation, `class="fixture-link`, `class="different-link`, 1),
		"new-nested-copy":         strings.Replace(syntheticNavigation, `>One</a>`, `><b>One</b></a>`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := adaptNavigation(syntheticSource(body), []byte(body))
			if err == nil || got.Templ != "" || got.Model != "" {
				t.Fatal("unreviewed shape emitted output")
			}
		})
	}
	a, err := adaptNavigation(syntheticSource(syntheticNavigation), []byte(syntheticNavigation))
	if err != nil {
		t.Fatal(err)
	}
	if VerifyNavigationReplay(a, []byte(syntheticNavigation)) == nil {
		t.Fatal("synthetic replay bypass")
	}
	a.Templ = strings.Repeat("x", 1<<20+1)
	if VerifyNavigationReplay(a, []byte(syntheticNavigation)) == nil {
		t.Fatal("oversized replay accepted")
	}
}

func TestNavigationSyntheticCompilerRuntime(t *testing.T) {
	a, err := adaptNavigation(syntheticSource(syntheticNavigation), []byte(syntheticNavigation))
	if err != nil {
		t.Fatal(err)
	}
	b, err := adaptNavigation(syntheticSource(syntheticNavigation), []byte(syntheticNavigation))
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("nondeterministic output")
	}
	if a.Verification != "not-verified" || a.Adapter != "p06-page-navigation-v1" {
		t.Fatal("generation boundary")
	}
	var defaults struct {
		CurrentKey string
		Items      []struct{ Key, Label, URL string }
	}
	if json.Unmarshal(a.Defaults, &defaults) != nil || defaults.CurrentKey != "page-3" || len(defaults.Items) != 4 || defaults.Items[0].Label != "One" {
		t.Fatal("defaults mismatch")
	}
	tests, err := os.ReadFile("navigation_runtime_test.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, body := range map[string]string{"navigation.templ": a.Templ, "navigation.go": a.Model, "navigation_test.go": string(tests) + "\nconst sourceExpected = " + strconv.Quote(syntheticNavigation) + "\nconst sourceDefaults = " + strconv.Quote(string(a.Defaults)) + "\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("compiler/runtime: %v\n%s", err, out)
		}
	}
	for _, phase := range []string{"raw", "formatted"} {
		if phase == "formatted" {
			run("tool", "templ", "fmt", filepath.Join(dir, "navigation.templ"))
		}
		run("tool", "templ", "generate", "-f", filepath.Join(dir, "navigation.templ"), "-include-version=false")
		run("test", "-count=1", filepath.Join(dir, "navigation.go"), filepath.Join(dir, "navigation_templ.go"), filepath.Join(dir, "navigation_test.go"))
	}
}
