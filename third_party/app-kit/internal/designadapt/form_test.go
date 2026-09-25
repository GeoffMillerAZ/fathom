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

// syntheticForm contains invented copy and deliberately minimal classes. It
// preserves only the reviewed P05 element/attribute shape and the eight palette
// tokens the adapter must map; it is not purchased source markup.
const syntheticForm = `<div data-synthetic="email-field"><label for="fixture-email" class="fixture-label">Fixture mailbox</label><div class="fixture-grid"><input id="fixture-email" type="email" name="fixture_email" value="retained@example.test" placeholder="person@example.test" aria-invalid="true" aria-describedby="fixture-email-error" class="fixture-input text-red-900 outline-red-300 placeholder:text-red-300 focus:outline-red-600 dark:text-red-400 dark:outline-red-500/50 dark:placeholder:text-red-400/70 dark:focus:outline-red-400"><svg class="fixture-icon" viewBox="0 0 20 20" aria-hidden="true"><path d="M10 2 2 18h16Z" /></svg></div><p id="fixture-email-error" class="fixture-error">Synthetic rejection.</p></div>`

func TestFormSourceAndStructureRefusals(t *testing.T) {
	source := syntheticSource(syntheticForm)
	if _, err := AdaptForm(source, []byte(syntheticForm)); err == nil {
		t.Fatal("arbitrary source accepted as frozen P05")
	}
	source.ArchiveID, source.Path, source.SHA256 = FormArchive, FormMember, FormSourceSHA
	if _, err := AdaptForm(source, []byte(syntheticForm)); err == nil {
		t.Fatal("spoofed frozen source identity accepted")
	}

	cases := map[string]string{
		"missing-label":         strings.Replace(syntheticForm, `<label for="fixture-email" class="fixture-label">Fixture mailbox</label>`, "", 1),
		"label-target-mismatch": strings.Replace(syntheticForm, `for="fixture-email"`, `for="other-email"`, 1),
		"wrong-input-type":      strings.Replace(syntheticForm, `type="email"`, `type="text"`, 1),
		"description-mismatch":  strings.Replace(syntheticForm, `aria-describedby="fixture-email-error"`, `aria-describedby="other-error"`, 1),
		"error-id-mismatch":     strings.Replace(syntheticForm, `id="fixture-email-error"`, `id="other-error"`, 1),
		"missing-icon":          strings.Replace(syntheticForm, `<svg class="fixture-icon" viewBox="0 0 20 20" aria-hidden="true"><path d="M10 2 2 18h16Z" /></svg>`, "", 1),
		"extra-field":           strings.Replace(syntheticForm, `</div>`, `<input id="other" name="other"></div>`, 1),
		"unsafe-behavior":       strings.Replace(syntheticForm, `<input `, `<input onclick="bad()" `, 1),
		"missing-palette":       strings.Replace(syntheticForm, ` text-red-900`, "", 1),
		"duplicate-palette":     strings.Replace(syntheticForm, ` text-red-900`, ` text-red-900 text-red-900`, 1),
		"unknown-red-token":     strings.Replace(syntheticForm, `fixture-input `, `fixture-input ring-red-400 `, 1),
		"extra-copy":            strings.Replace(syntheticForm, `</div>`, `<span>Unexpected</span></div>`, 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := adaptForm(syntheticSource(body), []byte(body))
			if err == nil || got.Templ != "" || got.Model != "" || len(got.Defaults) != 0 {
				t.Fatal("unsupported form shape emitted partial output")
			}
		})
	}
}

func TestFormSyntheticReplayCompilerAndRuntime(t *testing.T) {
	body := []byte(syntheticForm)
	first, err := adaptForm(syntheticSource(syntheticForm), body)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adaptForm(syntheticSource(syntheticForm), body)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("non-deterministic form adaptation")
	}
	if first.SchemaVersion != 1 || first.Adapter != "p05-email-field-v1" || first.Verification != "not-verified" || len(first.RuleSHA256) != 64 || len(first.SyntaxSHA256) != 64 || len(first.TemplSHA256) != 64 || len(first.ModelSHA256) != 64 || len(first.DefaultsSHA256) != 64 {
		t.Fatal("missing form provenance boundary")
	}
	var defaults struct {
		ID, Name, Label, Value, Placeholder, Error string
		Required, Autofocus                        bool
	}
	if err := json.Unmarshal(first.Defaults, &defaults); err != nil {
		t.Fatal(err)
	}
	if defaults.ID != "fixture-email" || defaults.Name != "fixture_email" || defaults.Label != "Fixture mailbox" || defaults.Value != "retained@example.test" || defaults.Placeholder != "person@example.test" || defaults.Error != "Synthetic rejection." || defaults.Required || defaults.Autofocus {
		t.Fatalf("incorrect defaults: %+v", defaults)
	}
	if err := VerifyFormReplay(first, body); err == nil {
		t.Fatal("synthetic seam crossed the frozen public replay boundary")
	}
	oversized := first
	oversized.Templ = strings.Repeat("x", 1<<20+1)
	if err := VerifyFormReplay(oversized, body); err == nil {
		t.Fatal("oversized replay artifact accepted")
	}

	runtimeTests, err := os.ReadFile("form_runtime_test.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"form.templ":   first.Templ,
		"form.go":      first.Model,
		"form_test.go": string(runtimeTests) + "\nconst sourceExpected = " + strconv.Quote(syntheticForm) + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("synthetic form compiler test: %v\n%s", err, out)
		}
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		if phase == "formatted" {
			run("tool", "templ", "fmt", filepath.Join(dir, "form.templ"))
		}
		run("tool", "templ", "generate", "-f", filepath.Join(dir, "form.templ"), "-include-version=false")
		run("test", "-count=1", filepath.Join(dir, "form.go"), filepath.Join(dir, "form_templ.go"), filepath.Join(dir, "form_test.go"))
	}
}
