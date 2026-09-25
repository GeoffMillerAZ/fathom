package form_test

import (
	"crypto/sha256"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
)

func TestExportSettingsSectionsFixture(t *testing.T) {
	outDir := os.Getenv("APPKIT_PROBE_SETTINGS_SECTIONS_OUT")
	if outDir == "" {
		t.Skip("set APPKIT_PROBE_SETTINGS_SECTIONS_OUT to an absolute existing private directory")
	}
	if !filepath.IsAbs(outDir) {
		t.Fatal("APPKIT_PROBE_SETTINGS_SECTIONS_OUT must be absolute")
	}
	info, err := os.Lstat(outDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("APPKIT_PROBE_SETTINGS_SECTIONS_OUT must be an existing real directory: %v", err)
	}

	input := form.RenderInput(form.Input{ID: "fixture-name", Name: "display_name", Value: "Alex Morgan"})
	name := form.RenderField(form.Field{Label: "Display name", For: "fixture-name", Control: template.HTML(input)})
	region := form.RenderSelect(form.Select{ID: "fixture-region", Name: "region", Value: "phoenix", Options: []form.Option{
		{Value: "phoenix", Label: "Phoenix"}, {Value: "london", Label: "London"},
	}})
	regionField := form.RenderField(form.Field{Label: "Region", For: "fixture-region", Control: template.HTML(region)})
	sections, err := form.RenderSettingsSections([]form.SettingsSection{
		{HeadingID: "fixture-name-title", Title: strings.Repeat("Workspace", 18), Description: strings.Repeat("preference", 40) + " remains app-owned and should wrap without hiding the control.", Content: template.HTML(name)},
		{HeadingID: "fixture-region-title", Title: "Region and language", Description: "Choose the display region for this layout-only fixture.", Content: template.HTML(regionField)},
		{HeadingID: "fixture-identity-title", Title: strings.Repeat("Identity", 18), Description: "Read-only identity information remains owned by the application.", Content: template.HTML(`<dl class="text-sm"><dt class="text-ink-muted">Email</dt><dd class="mt-2 break-all font-medium">person@example.test</dd></dl>`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	document := `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><link rel="stylesheet" href="/static/app.css"></head><body class="bg-surface-sunken text-ink font-sans"><main class="mx-auto max-w-4xl p-4"><h1 class="text-3xl font-semibold">Settings section fixture</h1><h2 class="mt-6 text-2xl font-semibold">Account preferences</h2><form method="post" action="/not-implemented">` + sections + `</form></main></body></html>`

	path := filepath.Join(outDir, "settings-sections.html")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(document); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("layout-only fixture path=%s sha256=%x", path, sha256.Sum256([]byte(document)))
}
