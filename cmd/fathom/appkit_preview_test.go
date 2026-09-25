package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	appkitPreviewDirEnv            = "APPKIT_PREVIEW_DIR"
	appkitObservationPreviewDirEnv = "APPKIT_OBSERVATION_PREVIEW_DIR"
	appkitPreviewCSP               = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'self' 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'">`
)

var appkitPreviewRoutes = []string{
	"/observatory",
	"/observatory/impact",
	"/observatory/service",
	"/observatory/story",
}

var appkitPreviewThemes = []string{"light", "dark"}

var appkitObservationPreviewScenarios = []string{
	"all-unknown",
	"partial",
	"complete-zero",
	"complete-positive",
	"call-log-unavailable",
	"registry-partial",
	"registry-unavailable",
	"registry-call-log-unavailable",
	"measured-zero-catches",
	"measured-positive-catches",
}

var appkitObservationPreviewServices = []string{"alpha", "beta", "gamma"}

// TestExportAppkitPreview exports synthetic, inert Observatory pages only
// when an explicit workspace/tmp destination is provided. It does not build a
// live model, start a listener, or call any daemon/provider surface.
func TestExportAppkitPreview(t *testing.T) {
	outDir := os.Getenv(appkitPreviewDirEnv)
	if outDir == "" {
		t.Skip(appkitPreviewDirEnv + " is not set")
	}
	requireEmptyAppkitPreviewDir(t, outDir)

	model := syntheticModel()
	for _, theme := range appkitPreviewThemes {
		for _, route := range appkitPreviewRoutes {
			body, err := renderAppkitPreviewPage(http.MethodGet, route, theme, model)
			if err != nil {
				t.Fatalf("render %s %s: %v", theme, route, err)
			}
			assertInertAppkitPreview(t, theme, route, body)
			writeAppkitPreviewFile(t, filepath.Join(outDir, theme, previewRouteDir(route), "index.html"), []byte(body))
		}
		writeAppkitPreviewFile(t, filepath.Join(outDir, theme, "assets", "app.css"), obsAppCSS)
	}

	t.Logf("synthetic app-kit previews exported to %s", outDir)
}

// TestExportAppkitObservationPreview exports the fixed coverage matrix used to
// review unknown, partial, measured-zero, measured-positive and unavailable
// observation states. It reads no registry, observation file or live service.
func TestExportAppkitObservationPreview(t *testing.T) {
	outDir := os.Getenv(appkitObservationPreviewDirEnv)
	if outDir == "" {
		t.Skip(appkitObservationPreviewDirEnv + " is not set")
	}
	requireEmptyAppkitPreviewDir(t, outDir)

	for _, scenario := range appkitObservationPreviewScenarios {
		model := syntheticObservationPreviewModel(scenario)
		for _, theme := range appkitPreviewThemes {
			for _, route := range []string{"/", "/observatory", "/observatory/impact", "/observatory/story"} {
				body, err := renderAppkitObservationPreviewPage(http.MethodGet, route, theme, "", model)
				if err != nil {
					t.Fatalf("render %s %s %s: %v", scenario, theme, route, err)
				}
				assertInertAppkitPreview(t, theme, route, body)
				writeAppkitPreviewFile(t, filepath.Join(outDir, scenario, theme, previewRouteDir(route), "index.html"), []byte(body))
			}
			services := appkitObservationPreviewServices
			if len(model.Services) == 0 {
				services = nil
			}
			for _, service := range services {
				body, err := renderAppkitObservationPreviewPage(http.MethodGet, "/observatory/service", theme, service, model)
				if err != nil {
					t.Fatalf("render %s %s service/%s: %v", scenario, theme, service, err)
				}
				assertInertAppkitPreview(t, theme, "service/"+service, body)
				writeAppkitPreviewFile(t, filepath.Join(outDir, scenario, theme, "service", service, "index.html"), []byte(body))
			}
			writeAppkitPreviewFile(t, filepath.Join(outDir, scenario, theme, "assets", "app.css"), obsAppCSS)
		}
	}

	t.Logf("synthetic observation previews exported to %s", outDir)
}

func TestAppkitPreviewRoutePolicy(t *testing.T) {
	model := syntheticModel()
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, route := range appkitPreviewRoutes {
			if _, err := renderAppkitPreviewPage(method, route, "light", model); err != nil {
				t.Errorf("%s %s: %v", method, route, err)
			}
		}
	}
	for _, tc := range []struct {
		method string
		path   string
		theme  string
	}{
		{http.MethodPost, "/observatory", "light"},
		{http.MethodGet, "/api/observatory", "light"},
		{http.MethodGet, "/observatory/wiring/", "light"},
		{http.MethodGet, "/observatory", "system"},
	} {
		if _, err := renderAppkitPreviewPage(tc.method, tc.path, tc.theme, model); err == nil {
			t.Errorf("%s %s theme=%s unexpectedly allowed", tc.method, tc.path, tc.theme)
		}
	}
}

func TestAppkitObservationPreviewPolicy(t *testing.T) {
	model := syntheticObservationPreviewModel("partial")
	for _, route := range []string{"/", "/observatory", "/observatory/impact", "/observatory/story"} {
		body, err := renderAppkitObservationPreviewPage(http.MethodHead, route, "dark", "", model)
		if err != nil || body != "" {
			t.Errorf("HEAD %s = %q, %v; want empty body", route, body, err)
		}
	}
	for _, service := range appkitObservationPreviewServices {
		if _, err := renderAppkitObservationPreviewPage(http.MethodGet, "/observatory/service", "light", service, model); err != nil {
			t.Errorf("fixed service %s: %v", service, err)
		}
		body, err := renderAppkitObservationPreviewPage(http.MethodHead, "/observatory/service", "dark", service, model)
		if err != nil || body != "" {
			t.Errorf("HEAD service/%s = %q, %v; want empty body", service, body, err)
		}
	}
	for _, tc := range []struct {
		method  string
		path    string
		theme   string
		service string
	}{
		{http.MethodPost, "/observatory/service", "light", "alpha"},
		{http.MethodGet, "/observatory/service/alpha", "light", "alpha"},
		{http.MethodGet, "/observatory/service", "system", "alpha"},
		{http.MethodGet, "/observatory/service", "light", "delta"},
		{http.MethodGet, "/observatory", "light", "alpha"},
		{http.MethodGet, "/?query=anything", "light", ""},
	} {
		if _, err := renderAppkitObservationPreviewPage(tc.method, tc.path, tc.theme, tc.service, model); err == nil {
			t.Errorf("%s %s theme=%s service=%s unexpectedly allowed", tc.method, tc.path, tc.theme, tc.service)
		}
	}
	for _, scenario := range []string{"registry-unavailable", "registry-call-log-unavailable"} {
		got := syntheticObservationPreviewModel(scenario)
		if len(got.Services) != 0 || got.DegradedNote != "Synthetic service registry unavailable; observation coverage is unknown, never zero." {
			t.Fatalf("%s fixture must be empty and explicitly degraded: %+v", scenario, got)
		} else if _, err := renderAppkitObservationPreviewPage(http.MethodGet, "/observatory/service", "light", "alpha", got); err == nil {
			t.Fatalf("%s fixture must not invent a service page", scenario)
		}
	}
	for scenario, want := range map[string][]string{
		"all-unknown":               {"unknown", "unknown", "unknown"},
		"partial":                   {"0", "4", "unknown"},
		"complete-zero":             {"0", "0", "0"},
		"complete-positive":         {"0", "4", "2"},
		"call-log-unavailable":      {"0", "4", "2"},
		"registry-partial":          {"0", "4", "2"},
		"measured-zero-catches":     {"0", "4", "2"},
		"measured-positive-catches": {"0", "4", "2"},
	} {
		got := syntheticObservationPreviewModel(scenario)
		for i, row := range got.Services {
			value := "unknown"
			if row.OpenObservations != nil {
				value = fmt.Sprint(*row.OpenObservations)
			}
			if value != want[i] {
				t.Errorf("%s service %s observations = %s, want %s", scenario, row.Service, value, want[i])
			}
			wantRemedy := "cd /tmp/" + row.Service + " && fathom ingest --service " + row.Service
			if row.FreshnessRemedy != wantRemedy {
				t.Errorf("%s service %s remedy = %q, want %q", scenario, row.Service, row.FreshnessRemedy, wantRemedy)
			}
		}
	}
	if got := syntheticObservationPreviewModel("call-log-unavailable"); got.CallLogNote != "Synthetic call log unavailable; traffic counts are unknown." {
		t.Errorf("call-log-unavailable note = %q", got.CallLogNote)
	}
	if got := syntheticObservationPreviewModel("registry-partial"); got.DegradedNote != "Synthetic registry incomplete; listed services only." {
		t.Errorf("registry-partial note = %q", got.DegradedNote)
	}
	if got := syntheticObservationPreviewModel("registry-call-log-unavailable"); got.DegradedNote != "Synthetic service registry unavailable; observation coverage is unknown, never zero." || got.CallLogNote != "Synthetic call log unavailable; traffic counts are unknown." {
		t.Errorf("combined unavailable notes = degraded %q, call log %q", got.DegradedNote, got.CallLogNote)
	}
	for scenario, want := range map[string]int{"measured-zero-catches": 0, "measured-positive-catches": 3} {
		got := syntheticObservationPreviewModel(scenario)
		if got.Impact.VerifyCatchesTotal == nil || *got.Impact.VerifyCatchesTotal != want || got.Impact.VerifyCatchesNote != "Synthetic verify ledger: 2 reporting sessions." {
			t.Errorf("%s catches = %+v, note %q; want %d and exact synthetic note", scenario, got.Impact.VerifyCatchesTotal, got.Impact.VerifyCatchesNote, want)
		}
	}
}

func TestAppkitPreviewEvidenceDestination(t *testing.T) {
	model := syntheticModel()
	impact, err := renderAppkitPreviewPage(http.MethodGet, "/observatory/impact", "light", model)
	if err != nil || !strings.Contains(impact, `href="/sessions"`) {
		t.Fatalf("missing sample must link to the fixed evidence route: %v", err)
	}
	story, err := renderAppkitPreviewPage(http.MethodGet, "/observatory/story", "light", model)
	if err != nil || !strings.Contains(story, "Synthetic session summary") || !strings.Contains(story, `href="/impact"`) {
		t.Fatalf("evidence destination must render fixture totals and a return path: %v", err)
	}
}

func TestInertAppkitPreviewHTML(t *testing.T) {
	current := `<!doctype html><html lang="en"><head></head><body><script src="/mutate.js"></script><form method="post" action="/mutate"><button>mutate</button></form></body></html>`
	got := inertAppkitPreviewHTML(current, "dark")
	for _, want := range []string{`<html lang="en" data-theme="dark">`, appkitPreviewCSP, `<template data-disabled-script`} {
		if !strings.Contains(got, want) {
			t.Errorf("current shell preview missing %q: %s", want, got)
		}
	}
	for _, forbidden := range []string{"<script", "<form"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("current shell preview retained %q: %s", forbidden, got)
		}
	}

	baseline := inertAppkitPreviewHTML(`<html><head></head><body></body></html>`, "light")
	if !strings.Contains(baseline, `<html data-theme="light">`) {
		t.Errorf("baseline shell did not receive forced light theme: %s", baseline)
	}
}

func TestAppkitPreviewOutputScopeRefusesRepoRoot(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	if err := validateAppkitPreviewDir(repoRoot); err == nil || !strings.Contains(err.Error(), "child of workspace/tmp") {
		t.Fatalf("repo root must be refused by output scope, got %v", err)
	}
}

func renderAppkitPreviewPage(method, route, theme string, model observatoryModel) (string, error) {
	service := ""
	if route == "/observatory/service" {
		service = "alpha"
	}
	return renderAppkitObservationPreviewPage(method, route, theme, service, model)
}

func renderAppkitObservationPreviewPage(method, route, theme, service string, model observatoryModel) (string, error) {
	if method != http.MethodGet && method != http.MethodHead {
		return "", fmt.Errorf("preview method %s is not allowed", method)
	}
	if theme != "light" && theme != "dark" {
		return "", fmt.Errorf("preview theme %s is not allowed", theme)
	}
	if route == "/observatory/service" {
		if !fixedAppkitObservationPreviewService(service) {
			return "", fmt.Errorf("preview service %s is not allowed", service)
		}
	} else if service != "" {
		return "", fmt.Errorf("preview service selector is only allowed for the service route")
	}

	var title, active, body string
	switch route {
	case "/":
		body = renderObsLanding(model)
	case "/observatory":
		title, active, body = "fathom observatory", "fleet", renderObsFleet(model)
	case "/observatory/impact":
		title, active, body = "fathom · impact", "impact", renderObsImpact(model)
	case "/observatory/service":
		var ok bool
		body, ok = renderObsService(model, service)
		if !ok {
			return "", fmt.Errorf("synthetic service %s is absent", service)
		}
		title, active = "fathom · "+service, "fleet"
	case "/observatory/story":
		// A fixed, known synthetic summary, not a read of the session ledger.
		ws := weekStory{
			From: "2026-08-02", To: "2026-08-08", Sessions: 2,
			CatchesRan: 2, EditSessions: 1, Note: "Synthetic session summary. No production ledger is read.",
		}
		title, active, body = "fathom · the week", "story", renderWeekStory(ws, model)
	default:
		return "", fmt.Errorf("preview route %s is not allowed", route)
	}
	if method == http.MethodHead {
		return "", nil
	}
	if route == "/" {
		return inertAppkitPreviewHTML(body, theme), nil
	}
	return inertAppkitPreviewHTML(obsPage(title, active, body, model), theme), nil
}

func fixedAppkitObservationPreviewService(service string) bool {
	switch service {
	case "alpha", "beta", "gamma":
		return true
	default:
		return false
	}
}

func syntheticObservationPreviewModel(scenario string) observatoryModel {
	m := syntheticModel()
	base := m.Services[0]
	m.Services = make([]obsServiceRow, 0, len(appkitObservationPreviewServices))
	for i, service := range appkitObservationPreviewServices {
		row := base
		row.Service = service
		row.Path = "/tmp/" + service
		row.Symbols += i
		row.OpenObservations = nil
		row.FreshnessRemedy = "cd /tmp/" + service + " && fathom ingest --service " + service
		m.Services = append(m.Services, row)
	}
	set := func(index, count int) { m.Services[index].OpenObservations = intPointer(count) }
	switch scenario {
	case "all-unknown":
	case "partial":
		set(0, 0)
		set(1, 4)
	case "complete-zero":
		set(0, 0)
		set(1, 0)
		set(2, 0)
	case "complete-positive":
		set(0, 0)
		set(1, 4)
		set(2, 2)
	case "call-log-unavailable":
		set(0, 0)
		set(1, 4)
		set(2, 2)
		m.CallLogNote = "Synthetic call log unavailable; traffic counts are unknown."
	case "registry-partial":
		set(0, 0)
		set(1, 4)
		set(2, 2)
		m.DegradedNote = "Synthetic registry incomplete; listed services only."
	case "registry-unavailable":
		m.Services = nil
		m.DegradedNote = "Synthetic service registry unavailable; observation coverage is unknown, never zero."
	case "registry-call-log-unavailable":
		m.Services = nil
		m.DegradedNote = "Synthetic service registry unavailable; observation coverage is unknown, never zero."
		m.CallLogNote = "Synthetic call log unavailable; traffic counts are unknown."
	case "measured-zero-catches":
		set(0, 0)
		set(1, 4)
		set(2, 2)
		m.Impact.VerifyCatchesTotal = intPointer(0)
		m.Impact.VerifyCatchesNote = "Synthetic verify ledger: 2 reporting sessions."
	case "measured-positive-catches":
		set(0, 0)
		set(1, 4)
		set(2, 2)
		m.Impact.VerifyCatchesTotal = intPointer(3)
		m.Impact.VerifyCatchesNote = "Synthetic verify ledger: 2 reporting sessions."
	default:
		panic("unreachable observation preview scenario " + scenario)
	}
	return m
}

func intPointer(n int) *int { return &n }

func inertAppkitPreviewHTML(body, theme string) string {
	if strings.Contains(body, `<html lang="en">`) {
		body = strings.Replace(body, `<html lang="en">`, `<html lang="en" data-theme="`+theme+`">`, 1)
	} else {
		body = strings.Replace(body, "<html>", `<html data-theme="`+theme+`">`, 1)
	}
	body = strings.Replace(body, "<head>", "<head>"+appkitPreviewCSP, 1)
	body = strings.ReplaceAll(body, "<script", `<template data-disabled-script`)
	body = strings.ReplaceAll(body, "</script>", "</template>")
	body = strings.ReplaceAll(body, "<form ", "<div ")
	body = strings.ReplaceAll(body, "</form>", "</div>")
	return body
}

func assertInertAppkitPreview(t *testing.T, theme, route, body string) {
	t.Helper()
	for _, forbidden := range []string{"<script", "<form"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("rendered %s %s contains %s", theme, route, forbidden)
		}
	}
	for _, required := range []string{appkitPreviewCSP, `data-theme="` + theme + `"`} {
		if !strings.Contains(body, required) {
			t.Fatalf("rendered %s %s is missing %s", theme, route, required)
		}
	}
}

func previewRouteDir(route string) string {
	switch route {
	case "/":
		return "home"
	case "/observatory":
		return "overview"
	case "/observatory/impact":
		return "impact"
	case "/observatory/service":
		return "service"
	case "/observatory/story":
		return "story"
	default:
		panic("unreachable preview route " + route)
	}
}

func requireEmptyAppkitPreviewDir(t *testing.T, path string) {
	t.Helper()
	if err := validateAppkitPreviewDir(path); err != nil {
		t.Fatal(err)
	}
}

func validateAppkitPreviewDir(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve preview directory: %w", err)
	}
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return fmt.Errorf("resolve preview directory symlinks: %w", err)
	}
	workspaceTmp, err := filepath.Abs(filepath.Join("..", "..", "workspace", "tmp"))
	if err != nil {
		return fmt.Errorf("resolve workspace/tmp: %w", err)
	}
	// workspace/ is gitignored scratch space (not part of the repo tree), so
	// a fresh clone or CI checkout has no workspace/tmp yet. Create it before
	// resolving symlinks rather than failing the safety check on its absence.
	if err := os.MkdirAll(workspaceTmp, 0o755); err != nil {
		return fmt.Errorf("create workspace/tmp: %w", err)
	}
	workspaceTmp, err = filepath.EvalSymlinks(workspaceTmp)
	if err != nil {
		return fmt.Errorf("resolve workspace/tmp symlinks: %w", err)
	}
	rel, err := filepath.Rel(workspaceTmp, absPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s must name a child of workspace/tmp", appkitPreviewDirEnv)
	}
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("read preview directory: %w", err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("preview directory must be empty; refusing to overwrite %s", absPath)
	}
	return nil
}

func writeAppkitPreviewFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create preview directory: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write preview file: %v", err)
	}
}
