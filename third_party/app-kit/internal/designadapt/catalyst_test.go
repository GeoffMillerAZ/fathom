package designadapt

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func syntheticCatalystForm() string {
	return `<!doctype html><form novalidate class="fixture-form"><fieldset disabled class="fixture-fieldset"><section class="fixture-section"><div><h2>Fixture name</h2><p>Fixture description</p></div><div><input name="name" class="fixture-name" value="initial"></div></section><section class="fixture-section"><div><h2>Fixture bio</h2></div><div><textarea name="bio" class="fixture-bio"></textarea><p id="bio-count" class="fixture-count">0 / 240</p></div></section><section class="fixture-section"><div><h2>Fixture email</h2></div><div><input type="email" name="email" class="fixture-email" value="initial"><div><span role="checkbox" class="fixture-controller">controller</span><label class="fixture-checkbox-label">Fixture checkbox text</label><span hidden><input type="hidden" name="email_is_public" value="on"></span></div></div></section><section class="fixture-section"><div><h2>Fixture address</h2></div><div class="fixture-grid"><input name="address" class="fixture-address" value="initial"><input name="city" class="fixture-city" value="initial"><input name="postal_code" class="fixture-postal" value="initial"><span data-slot="control" class="fixture-country-layout col-span-2"><button aria-label="Country" class="fixture-country" type="button"><span>Country</span></button></span><span hidden><input type="hidden" name="country[name]" value="old"></span><span data-slot="control" class="fixture-region-layout"><button aria-label="Region" class="fixture-region" type="button"><span>Region</span></button></span><span hidden><input type="hidden" name="region" value="old"></span></div></section><section class="fixture-section"><div><h2>Fixture currency</h2></div><div><span data-slot="control" class="fixture-currency-wrapper"><select aria-label="Currency" name="currency" class="fixture-currency"><option>Old</option></select></span><span hidden><input type="hidden" name="currency" value="old"></span><p role="status" class="fixture-status">Fixture status</p><button type="reset" class="fixture-reset">Fixture reset</button><button type="submit" class="fixture-submit">Fixture submit</button></div></section></fieldset></form>`
}

func TestCatalystSyntheticStructureAndMarkerRefusals(t *testing.T) {
	body := []byte(syntheticCatalystForm())
	first, err := adaptCatalystForm(body)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adaptCatalystForm(body)
	if err != nil || first != second {
		t.Fatal("synthetic transformation is not deterministic")
	}
	if strings.Count(first, `class={ "fixture-section" }`) != 5 || !strings.Contains(first, `class={ "fixture-currency-wrapper col-span-2" }`) {
		t.Fatal("complete section order or country layout was not retained")
	}
	for _, want := range []string{
		"experimental, private, and native-not-faithful", "templ settingsForm(data SettingsView)",
		`data.Value("name")`, `settingsBioText(data)`, `data.Value("email")`, `data.Value("address")`, `data.Value("city")`, `data.Value("postal_code")`,
		`data.FieldID("form")`, `data.FieldID("email_is_public")`, `data.Invalid("bio")`, `data.Invalid("currency")`,
		`@settingsOptions(data, "country")`, `@settingsOptions(data, "region")`, `@settingsOptions(data, "currency")`,
		`@settingsError(data, "bio")`, `@settingsError(data, "email")`, `@settingsError(data, "country")`, `@settingsError(data, "region")`, `@settingsError(data, "currency")`,
		"@settingsBioCount(data)", "@settingsStatus(data)", `checked?={ data.Values.PublicEmail }`, `formaction={ data.ResetAction }`,
		`form={ data.FieldID("reset-form") }`, "@settingsResetForm(data)", "@settingsEnhancementConfig(data)",
		`if data.Focus == "country" { autofocus }`, `if data.Focus == "region" { autofocus }`, `if data.Focus == "email_is_public" { autofocus }`, `if data.Focus == "reset" { autofocus }`, `if data.Focus == "status" { autofocus }`,
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("generated skeleton omitted %q", want)
		}
	}
	for _, forbidden := range []string{"role={ \"checkbox\" }", `name={ "country[name]" }`, `name={ "email_is_public" } value={ "on" }`, "__APPKIT_P10_", "templ.Raw"} {
		if strings.Contains(first, forbidden) {
			t.Fatalf("native authority conversion retained %q", forbidden)
		}
	}
	for name, changed := range map[string]string{
		"missing section":   strings.Replace(string(body), `<section class="fixture-section"><div><h2>Fixture currency</h2>`, `<div><h2>Fixture currency</h2>`, 1),
		"duplicate country": strings.Replace(string(body), `aria-label="Country"`, `aria-label="Country" data-extra="x" aria-label="Country"`, 1),
		"missing status":    strings.Replace(string(body), `role="status"`, `role="note"`, 1),
		"marker collision":  string(body) + "__APPKIT_P10_FORM_ACTION__",
		"hard bio wrap":     strings.Replace(string(body), `textarea name="bio"`, `textarea wrap="hard" name="bio"`, 1),
		"unknown bio wrap":  strings.Replace(string(body), `textarea name="bio"`, `textarea wrap="custom" name="bio"`, 1),
		"empty bio wrap":    strings.Replace(string(body), `textarea name="bio"`, `textarea wrap name="bio"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := adaptCatalystForm([]byte(changed)); err == nil || got != "" {
				t.Fatal("unreviewed source emitted a skeleton")
			}
		})
	}
}

func TestCatalystTextareaLeadingNewlineSentinel(t *testing.T) {
	raw := "\nfirst\r\nlast\n"
	doc, err := html.Parse(strings.NewReader("<textarea>\n" + raw + "</textarea>"))
	if err != nil {
		t.Fatal(err)
	}
	textareas := catalystElements(doc, "textarea")
	if len(textareas) != 1 || textareas[0].FirstChild == nil {
		t.Fatal("textarea parse contract changed")
	}
	if got, want := textareas[0].FirstChild.Data, "\nfirst\nlast\n"; got != want {
		t.Fatalf("sacrificial LF did not retain textarea-leading newline: %q, want %q", got, want)
	}
}

func TestAdaptCatalystFormPinsPrivateBytes(t *testing.T) {
	if got, err := AdaptCatalystForm([]byte("not a private capture")); err == nil || got != "" {
		t.Fatal("unidentified private bytes were accepted")
	}
	stale := strings.Repeat("x", CatalystFormSourceBytes)
	if got, err := AdaptCatalystForm([]byte(stale)); err == nil || got != "" {
		t.Fatal("stale same-sized private bytes were accepted")
	}
}

func TestCatalystGeneratedTemplCompilesWithExplicitHelperSeam(t *testing.T) {
	generated, err := adaptCatalystForm([]byte(syntheticCatalystForm()))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":         "module example.test/converted\n\ngo 1.26.2\n\nrequire github.com/a-h/templ v0.3.1020\n",
		"settings.templ": generated,
		"helpers.templ":  catalystTemplHelpers,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"settings.templ", "helpers.templ"} {
		cmd := exec.CommandContext(ctx, "go", "tool", "templ", "generate", "-f", filepath.Join(dir, name), "-include-version=false")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generated skeleton did not parse with helper seam: %v\n%s\n%s", err, output, generated)
		}
	}
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("standalone helper seam could not resolve pinned dependencies: %v\n%s", err, output)
	}
	cmd = exec.CommandContext(ctx, "go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated skeleton did not compile with helper seam: %v\n%s", err, output)
	}
}

func TestAdaptCatalystFormOptInPrivatePacket(t *testing.T) {
	root := os.Getenv("APPKIT_CATALYST_SOURCE_PACKET")
	if root == "" {
		t.Skip("private source packet not requested")
	}
	body, err := os.ReadFile(filepath.Join(root, "reference-default.html"))
	if err != nil {
		t.Fatal("requested private source packet is missing or unreadable")
	}
	if _, err := AdaptCatalystForm(body); err != nil {
		t.Fatalf("requested private source packet is stale or no longer structurally supported: %v", err)
	}
}

const catalystTemplHelpers = `package converted

type SettingsValues struct { PublicEmail bool }
type SettingsView struct { Action, ResetAction, CSRFToken, Focus string; Values SettingsValues }
func (SettingsView) Value(string) string { return "" }
func settingsBioText(SettingsView) string { return "" }
func (SettingsView) FieldID(string) string { return "fixture" }
func (SettingsView) Invalid(string) string { return "false" }
func (SettingsView) HasError(string) bool { return false }
templ settingsOptions(SettingsView, string) {}
templ settingsError(SettingsView, string) {}
templ settingsBioCount(SettingsView) {}
templ settingsStatus(SettingsView) {}
templ settingsResetForm(SettingsView) {}
templ settingsEnhancementConfig(SettingsView) {}
`
