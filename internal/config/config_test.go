package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Load — absent layers
// ---------------------------------------------------------------------------

func TestLoad_BothLayersAbsent_EmptyConfigNoError(t *testing.T) {
	dir := t.TempDir()
	loaded, err := Load(filepath.Join(dir, "user.toml"), filepath.Join(dir, "project.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.UserFound || loaded.ProjectFound {
		t.Fatalf("neither layer should be found: %+v", loaded)
	}
	if loaded.Config.DefaultProfile != "" {
		t.Errorf("DefaultProfile = %q, want empty", loaded.Config.DefaultProfile)
	}
	if len(loaded.Config.Profiles) != 0 {
		t.Errorf("Profiles = %v, want empty", loaded.Config.Profiles)
	}
}

func TestLoad_EmptyPaths_Skipped(t *testing.T) {
	loaded, err := Load("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.UserFound || loaded.ProjectFound {
		t.Fatalf("empty paths must never be treated as found: %+v", loaded)
	}
}

// ---------------------------------------------------------------------------
// Load — single layer, parse + validate
// ---------------------------------------------------------------------------

func writeTOML(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_UserLayer_ParsesProfile(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, `
default_profile = "work"

[profiles.work]
provider = "anthropic"
model = "claude-opus-4-1-20250805"
`)
	loaded, err := Load(userPath, filepath.Join(dir, "project.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.UserFound {
		t.Fatal("expected UserFound")
	}
	if loaded.Config.DefaultProfile != "work" {
		t.Errorf("DefaultProfile = %q, want work", loaded.Config.DefaultProfile)
	}
	p, ok := loaded.Config.Profiles["work"]
	if !ok {
		t.Fatal("profile 'work' missing")
	}
	if p.Provider != "anthropic" || p.Model != "claude-opus-4-1-20250805" {
		t.Errorf("profile = %+v", p)
	}
}

func TestLoad_InvalidTOML_NamesFile(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, `this is not valid toml =`)

	_, err := Load(userPath, "")
	if err == nil {
		t.Fatal("expected error")
	}
	var loadErr *LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("error is not *LoadError: %v", err)
	}
	if loadErr.Path != userPath {
		t.Errorf("LoadError.Path = %q, want %q", loadErr.Path, userPath)
	}
}

// TestLoad_InvalidTOML_NamesLineAndColumn pins N4: go-toml's *DecodeError
// carries the position but its Error() text drops it; readLayer must
// surface line/column explicitly.
func TestLoad_InvalidTOML_NamesLineAndColumn(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, "default_profile = \"ok\"\n\n[profiles.work\nprovider = \"anthropic\"\n")

	_, err := Load(userPath, "")
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("err = %v, want it to carry the offending line number (line 3)", err)
	}
	if !strings.Contains(err.Error(), "column") {
		t.Errorf("err = %v, want it to carry a column number", err)
	}
}

// TestLoad_ValidationError_PathAppearsOnce pins N5: formatCUEError used to
// render "path: path: message" because cue's Error() already prepends the
// path the formatter was adding again.
func TestLoad_ValidationError_PathAppearsOnce(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, `
[profiles.typo]
provider = "not-a-real-provider"
`)
	_, err := Load(userPath, "")
	if err == nil {
		t.Fatal("expected a validation error")
	}
	const key = "profiles.typo.provider"
	if got := strings.Count(err.Error(), key); got != 1 {
		t.Errorf("err = %v\npath %q appears %d times, want exactly once", err, key, got)
	}
}

func TestLoad_UnknownProvider_RejectedNamingFile(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, `
[profiles.bad]
provider = "not-a-real-provider"
`)
	_, err := Load(userPath, "")
	if err == nil {
		t.Fatal("expected a schema validation error")
	}
	var loadErr *LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("error is not *LoadError: %v", err)
	}
	if loadErr.Path != userPath {
		t.Errorf("LoadError.Path = %q, want %q", loadErr.Path, userPath)
	}
}

func TestLoad_LiteralAPIKeyField_Rejected(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, `
[profiles.leaked]
provider = "anthropic"
api_key = "sk-ant-should-not-be-representable"
`)
	_, err := Load(userPath, "")
	if err == nil {
		t.Fatal("expected a schema validation error rejecting the literal api_key field")
	}
	if strings.Contains(err.Error(), "sk-ant-should-not-be-representable") {
		t.Errorf("error must not echo the attempted secret value: %v", err)
	}
}

func TestLoad_MissingProvider_Rejected(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, `
[profiles.noprovider]
model = "gpt-4o"
`)
	if _, err := Load(userPath, ""); err == nil {
		t.Fatal("expected error: provider is required")
	}
}

// ---------------------------------------------------------------------------
// Load — merge precedence: project over user, field-by-field
// ---------------------------------------------------------------------------

func TestLoad_ProjectOverridesUser_FieldByField(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "project.toml")
	writeTOML(t, userPath, `
default_profile = "work"

[profiles.work]
provider = "anthropic"
model = "claude-haiku-4-5-20251001"
bin = "/opt/homebrew/bin/some-shim"
`)
	// Each layer validates independently against #Config (provider is
	// required per #Profile), so a project-layer override that targets an
	// existing profile name repeats provider — bin, only ever set in the
	// user layer, is what proves per-FIELD (not whole-profile) merge below.
	writeTOML(t, projectPath, `
[profiles.work]
provider = "anthropic"
model = "claude-opus-4-1-20250805"
`)
	loaded, err := Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := loaded.Config.Profiles["work"]
	if p.Model != "claude-opus-4-1-20250805" {
		t.Errorf("Model = %q, want project override", p.Model)
	}
	// bin was only ever set in the user layer — the project layer's partial
	// override must not blow it away (field-by-field merge, not
	// whole-profile replacement).
	if p.Bin != "/opt/homebrew/bin/some-shim" {
		t.Errorf("Bin = %q, want it preserved from the user layer", p.Bin)
	}
	// default_profile was only set at the user layer.
	if loaded.Config.DefaultProfile != "work" {
		t.Errorf("DefaultProfile = %q, want work (from user layer)", loaded.Config.DefaultProfile)
	}
}

func TestLoad_Provenance_NamesCorrectLayer(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "project.toml")
	writeTOML(t, userPath, `
[profiles.work]
provider = "anthropic"
model = "claude-haiku-4-5-20251001"
bin = "/opt/homebrew/bin/some-shim"
`)
	writeTOML(t, projectPath, `
[profiles.work]
provider = "anthropic"
model = "claude-opus-4-1-20250805"
`)
	loaded, err := Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	modelInfo, ok := loaded.Provenance["profiles.work.model"]
	if !ok {
		t.Fatal("no provenance recorded for profiles.work.model")
	}
	if modelInfo.Source != SourceProject || modelInfo.Path != projectPath {
		t.Errorf("model provenance = %+v, want project layer %q", modelInfo, projectPath)
	}
	// bin was only ever set in the user layer — its provenance must still
	// name the user file even though the project layer also touched this
	// profile (field-by-field provenance, not whole-profile).
	binInfo, ok := loaded.Provenance["profiles.work.bin"]
	if !ok {
		t.Fatal("no provenance recorded for profiles.work.bin")
	}
	if binInfo.Source != SourceUser || binInfo.Path != userPath {
		t.Errorf("bin provenance = %+v, want user layer %q", binInfo, userPath)
	}
}

// TestLoad_FindingsTable pins the findings-loop's config surface
// (docs/vision/30-findings-loop.md §3.3/§3.7): subject/outbox/producers
// decode, provenance is recorded, and project overrides user like every
// other field.
func TestLoad_FindingsTable(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, `
[findings]
subject = "geoffmilleraz/fathom/fathom"
outbox = ".reviewkit/outbox/projection.json"
producers = ["reviewkit.system-review", "reviewkit.security-review"]
`)
	loaded, err := Load(userPath, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	f := loaded.Config.Findings
	if f.Subject != "geoffmilleraz/fathom/fathom" || f.Outbox != ".reviewkit/outbox/projection.json" {
		t.Errorf("findings = %+v", f)
	}
	if len(f.Producers) != 2 || f.Producers[0] != "reviewkit.system-review" || f.Producers[1] != "reviewkit.security-review" {
		t.Errorf("Producers = %v", f.Producers)
	}
	if info := loaded.Provenance["findings.subject"]; info.Source != SourceUser || info.Path != userPath {
		t.Errorf("provenance = %+v", info)
	}

	projectPath := filepath.Join(dir, "project.toml")
	writeTOML(t, projectPath, `[findings]
subject = "other/repo/component"
`)
	loaded2, err := Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded2.Config.Findings.Subject != "other/repo/component" {
		t.Errorf("project did not override user: %+v", loaded2.Config.Findings)
	}
	// The user layer's outbox survives — merge is field-by-field, not a
	// wholesale table replacement.
	if loaded2.Config.Findings.Outbox != ".reviewkit/outbox/projection.json" {
		t.Errorf("Outbox = %q, want the user layer's value to survive", loaded2.Config.Findings.Outbox)
	}
}

func TestLoad_DaemonTable(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, `
[daemon]
store = "workspace/db/fleet.sqlite"
addr = "127.0.0.1:7581"
auto_refresh = false
default_profile = "work"
`)
	loaded, err := Load(userPath, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	d := loaded.Config.Daemon
	if d.Store != "workspace/db/fleet.sqlite" || d.Addr != "127.0.0.1:7581" || d.DefaultProfile != "work" {
		t.Errorf("daemon = %+v", d)
	}
	if d.AutoRefresh == nil || *d.AutoRefresh != false {
		t.Errorf("AutoRefresh = %v, want explicit false", d.AutoRefresh)
	}
}

// TestLoad_ConsoleTable pins `[console] conserve_max_files` (RT-118): it
// decodes, records its provenance, and a project layer overrides the user's.
func TestLoad_ConsoleTable(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "project.toml")
	writeTOML(t, userPath, "[console]\nconserve_max_files = 200\n")
	loaded, err := Load(userPath, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.Config.Console.ConserveMaxFiles; got != 200 {
		t.Errorf("conserve_max_files = %d, want 200", got)
	}
	if _, ok := loaded.Provenance["console.conserve_max_files"]; !ok {
		t.Error("no provenance for console.conserve_max_files")
	}
	writeTOML(t, projectPath, "[console]\nconserve_max_files = 50\n")
	loaded, err = Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.Config.Console.ConserveMaxFiles; got != 50 {
		t.Errorf("project conserve_max_files = %d, want 50", got)
	}
	writeTOML(t, projectPath, "[console]\nconserve_max_files = 0\n")
	if _, err := Load(userPath, projectPath); err == nil {
		t.Error("conserve_max_files = 0 must be rejected by the schema")
	}
}

// TestLoad_ZonesTable pins `[zones]` (WI-1): name to globs decodes, the
// project layer replaces the user's table outright, and an empty glob
// list is a schema error.
func TestLoad_ZonesTable(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "project.toml")
	writeTOML(t, userPath, "[zones]\nold = [\"x/**\"]\n")
	writeTOML(t, projectPath, "[zones]\npayments = [\"internal/pay/**\", \"cmd/payd/*\"]\n")
	loaded, err := Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	z := loaded.Config.Zones
	if len(z) != 1 || len(z["payments"]) != 2 || z["payments"][1] != "cmd/payd/*" {
		t.Fatalf("zones = %v, want the project table only", z)
	}
	if loaded.Provenance["zones"].Path != projectPath {
		t.Errorf("zones provenance = %+v", loaded.Provenance["zones"])
	}
	writeTOML(t, projectPath, "[zones]\nempty = []\n")
	if _, err := Load("", projectPath); err == nil {
		t.Error("an empty glob list must be rejected by the schema")
	}
}

// TestLoad_WorkCISummaries pins `[[work.ci_summaries]]` (WI-2w): each
// entry decodes with its path and commit, the project layer replaces the
// user's list outright, and a non-hex commit is a schema error.
func TestLoad_WorkCISummaries(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "project.toml")
	writeTOML(t, userPath, "[[work.ci_summaries]]\npath = \"old.xml\"\ncommit = \"abcdef0\"\n")
	writeTOML(t, projectPath, "[[work.ci_summaries]]\npath = \"ci/junit.xml\"\ncommit = \"0123abc\"\n\n[[work.ci_summaries]]\npath = \"ci/runs.json\"\n")
	loaded, err := Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := loaded.Config.Work.CISummaries
	if len(got) != 2 || got[0] != (CISummary{Path: "ci/junit.xml", Commit: "0123abc"}) || got[1] != (CISummary{Path: "ci/runs.json"}) {
		t.Fatalf("ci_summaries = %+v, want the project list only", got)
	}
	if loaded.Provenance["work.ci_summaries"].Path != projectPath {
		t.Errorf("provenance = %+v", loaded.Provenance["work.ci_summaries"])
	}
	writeTOML(t, projectPath, "[[work.ci_summaries]]\npath = \"ci/junit.xml\"\ncommit = \"main\"\n")
	if _, err := Load("", projectPath); err == nil {
		t.Error("a commit that is not a hex object name must be rejected by the schema")
	}
}

// TestLoad_GuardsTable pins ADR-038's escape hatch config field: `[guards]
// enforce = false` decodes to an explicit *bool, provenance is recorded,
// and project overrides user like every other field.
func TestLoad_GuardsTable(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, "[guards]\nenforce = false\n")

	loaded, err := Load(userPath, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Config.Guards.Enforce == nil || *loaded.Config.Guards.Enforce != false {
		t.Errorf("Guards.Enforce = %v, want explicit false", loaded.Config.Guards.Enforce)
	}
	if info := loaded.Provenance["guards.enforce"]; info.Source != SourceUser || info.Path != userPath {
		t.Errorf("provenance = %+v", info)
	}

	// Unset: nil, never a false default masquerading as "explicitly set".
	empty := filepath.Join(dir, "empty.toml")
	writeTOML(t, empty, "default_profile = \"x\"\n")
	loaded, err = Load(empty, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Config.Guards.Enforce != nil {
		t.Errorf("Guards.Enforce = %v, want nil when unset", *loaded.Config.Guards.Enforce)
	}

	// Project wins over user, like every other field.
	projectPath := filepath.Join(dir, "project.toml")
	writeTOML(t, projectPath, "[guards]\nenforce = true\n")
	loaded, err = Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Config.Guards.Enforce == nil || *loaded.Config.Guards.Enforce != true {
		t.Errorf("Guards.Enforce = %v, want project's explicit true to win", loaded.Config.Guards.Enforce)
	}
}

// TestConfig_DecideDefaultSitesExcludeRoute pins JF-7
// (docs/design/judgment-foundations.md §6): route leaves the default
// [decide] sites, frozen at the sites that existed before JF-7 minus route;
// its code and SR-44's deterministic precheck stay, and a repository or a
// user that names route in [decide] sites gets it back. RepoSites records
// only what the project layer named (JF-4: the repository's own list).
func TestConfig_DecideDefaultSitesExcludeRoute(t *testing.T) {
	frozen := []string{"compile_prefilter", "obligations_rerank", "certify_suggest", "assess", "preflight"}
	dir := t.TempDir()

	loaded, err := Load("", "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(loaded.Config.Decide.Sites, frozen) {
		t.Errorf("default [decide] sites = %v, want the frozen list %v", loaded.Config.Decide.Sites, frozen)
	}
	for _, s := range loaded.Config.Decide.Sites {
		if s == "route" {
			t.Error("default [decide] sites lists route, want it off by default")
		}
	}
	if loaded.Config.Decide.RepoSites != nil {
		t.Errorf("RepoSites = %v with no project layer, want nil", loaded.Config.Decide.RepoSites)
	}

	user := filepath.Join(dir, "user.toml")
	writeTOML(t, user, "[decide]\nsites = [\"route\", \"compile_prefilter\"]\n")
	loaded, err = Load(user, "")
	if err != nil {
		t.Fatalf("Load user: %v", err)
	}
	if got := loaded.Config.Decide.Sites; !reflect.DeepEqual(got, []string{"route", "compile_prefilter"}) {
		t.Errorf("user-layer sites = %v, want route back as named", got)
	}
	if loaded.Config.Decide.RepoSites != nil {
		t.Errorf("RepoSites = %v from a user layer, want nil (only the project layer is the repository's own)", loaded.Config.Decide.RepoSites)
	}

	project := filepath.Join(dir, "project.toml")
	writeTOML(t, project, "[decide]\nsites = [\"route\", \"ask\"]\n")
	loaded, err = Load(user, project)
	if err != nil {
		t.Fatalf("Load project: %v", err)
	}
	if got := loaded.Config.Decide.RepoSites; !reflect.DeepEqual(got, []string{"route", "ask"}) {
		t.Errorf("RepoSites = %v, want the project layer's list", got)
	}
	if got := loaded.Config.Decide.Sites; !reflect.DeepEqual(got, []string{"route", "ask"}) {
		t.Errorf("Sites = %v, want the project layer's list", got)
	}
}

// TestConfig_DecideBlock pins ADR-042's decision port config surface
// (docs/vision/32-decision-port.md, schema.cue #Decide): defaults when the
// `[decide]` table is absent, every key parses when present, an unknown
// provider is refused by name, and a threshold outside [0,1] is refused.
//
// certifies: invariant I42 (asserts)
func TestConfig_DecideBlock(t *testing.T) {
	t.Run("defaults when absent", func(t *testing.T) {
		dir := t.TempDir()
		loaded, err := Load(filepath.Join(dir, "user.toml"), "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		// Asserted as a literal (docs/vision/32-decision-port.md's exact
		// documented defaults), not just "equals DefaultDecide" — that
		// comparison alone would still pass if DefaultDecide itself were
		// wrong.
		want := Decide{
			Provider:         "none",
			CompileThreshold: 0.5,
			RouteConfidence:  0.9,
			RouteTimeoutMS:   3000,
			RankWithinTier:   false,
			RerankTimeoutMS:  1500,
			Sites:            []string{"compile_prefilter", "obligations_rerank", "certify_suggest", "assess", "preflight"},
		}
		if !reflect.DeepEqual(loaded.Config.Decide, want) {
			t.Errorf("Decide = %+v, want defaults %+v", loaded.Config.Decide, want)
		}
		if !reflect.DeepEqual(want, DefaultDecide) {
			t.Errorf("DefaultDecide = %+v, want it to match the documented defaults %+v", DefaultDecide, want)
		}
	})

	t.Run("mergeDecideFields sets fields and records provenance directly", func(t *testing.T) {
		var d Decide
		prov := Provenance{}
		mergeDecideFields(&d, map[string]any{
			"provider":          "llm",
			"profile":           "work",
			"compile_threshold": 0.3,
			"route_confidence":  0.8,
			"route_timeout_ms":  int64(500),
			"rank_within_tier":  false,
			"rerank_timeout_ms": int64(800),
		}, SourceProject, "project.toml", prov)
		want := Decide{Provider: "llm", Profile: "work", CompileThreshold: 0.3, RouteConfidence: 0.8, RouteTimeoutMS: 500, RankWithinTier: false, RerankTimeoutMS: 800, ProviderSource: SourceProject}
		if !reflect.DeepEqual(d, want) {
			t.Errorf("mergeDecideFields result = %+v, want %+v", d, want)
		}
		if info := prov["decide.provider"]; info.Source != SourceProject || info.Path != "project.toml" {
			t.Errorf("provenance[decide.provider] = %+v", info)
		}
		if info := prov["decide.route_timeout_ms"]; info.Source != SourceProject || info.Path != "project.toml" {
			t.Errorf("provenance[decide.route_timeout_ms] = %+v", info)
		}
		if info := prov["decide.rerank_timeout_ms"]; info.Source != SourceProject || info.Path != "project.toml" {
			t.Errorf("provenance[decide.rerank_timeout_ms] = %+v", info)
		}
	})

	t.Run("mergeDecideFields sets typesafe fields and records provenance", func(t *testing.T) {
		var d Decide
		prov := Provenance{}
		mergeDecideFields(&d, map[string]any{
			"provider":    "typesafe",
			"endpoint":    "https://example.invalid",
			"model":       "jev-1.13.0",
			"api_key_env": "TYPESAFE_API_KEY",
			"api_key_cmd": []any{"op", "read", "op://vault/jev/key"},
		}, SourceProject, "project.toml", prov)
		want := Decide{
			Provider:  "typesafe",
			Endpoint:  "https://example.invalid",
			Model:     "jev-1.13.0",
			APIKeyEnv: "TYPESAFE_API_KEY",
			APIKeyCmd: []string{"op", "read", "op://vault/jev/key"},
		}
		if d.Provider != want.Provider || d.Endpoint != want.Endpoint || d.Model != want.Model || d.APIKeyEnv != want.APIKeyEnv || strings.Join(d.APIKeyCmd, " ") != strings.Join(want.APIKeyCmd, " ") {
			t.Errorf("mergeDecideFields result = %+v, want %+v", d, want)
		}
		for _, key := range []string{"decide.endpoint", "decide.model", "decide.api_key_env", "decide.api_key_cmd"} {
			if info := prov[key]; info.Source != SourceProject || info.Path != "project.toml" {
				t.Errorf("provenance[%q] = %+v", key, info)
			}
		}
	})

	t.Run("parses every key", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		writeTOML(t, userPath, `
[decide]
provider           = "llm"
profile            = "work"
compile_threshold  = 0.7
route_confidence    = 0.95
route_timeout_ms   = 1500
rank_within_tier   = false
rerank_timeout_ms  = 900
`)
		loaded, err := Load(userPath, "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		d := loaded.Config.Decide
		if d.Provider != "llm" || d.Profile != "work" || d.CompileThreshold != 0.7 || d.RouteConfidence != 0.95 || d.RouteTimeoutMS != 1500 || d.RankWithinTier != false || d.RerankTimeoutMS != 900 {
			t.Errorf("Decide = %+v", d)
		}
		for _, key := range []string{"decide.provider", "decide.profile", "decide.compile_threshold", "decide.route_confidence", "decide.route_timeout_ms", "decide.rank_within_tier", "decide.rerank_timeout_ms"} {
			if info := loaded.Provenance[key]; info.Source != SourceUser || info.Path != userPath {
				t.Errorf("provenance[%q] = %+v, want user layer %q", key, info, userPath)
			}
		}
	})

	t.Run("parses typesafe provider and key-indirection keys", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		writeTOML(t, userPath, `
[decide]
provider    = "typesafe"
endpoint    = "https://api.typesafe.ai"
model       = "jev-latest"
api_key_env = "TYPESAFE_API_KEY"
`)
		loaded, err := Load(userPath, "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		d := loaded.Config.Decide
		if d.Provider != "typesafe" || d.Endpoint != "https://api.typesafe.ai" || d.Model != "jev-latest" || d.APIKeyEnv != "TYPESAFE_API_KEY" {
			t.Errorf("Decide = %+v", d)
		}
		for _, key := range []string{"decide.provider", "decide.endpoint", "decide.model", "decide.api_key_env"} {
			if info := loaded.Provenance[key]; info.Source != SourceUser || info.Path != userPath {
				t.Errorf("provenance[%q] = %+v, want user layer %q", key, info, userPath)
			}
		}
	})

	t.Run("refuses unknown provider by name", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		writeTOML(t, userPath, "[decide]\nprovider = \"jev\"\n")
		_, err := Load(userPath, "")
		if err == nil {
			t.Fatal("expected an error for an unknown [decide] provider")
		}
		if !strings.Contains(err.Error(), "jev") {
			t.Errorf("error = %v, want it to name the offending value %q", err, "jev")
		}
	})

	t.Run("refuses threshold out of range", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		writeTOML(t, userPath, "[decide]\ncompile_threshold = 1.5\n")
		if _, err := Load(userPath, ""); err == nil {
			t.Fatal("expected an error for compile_threshold outside [0,1]")
		}

		userPath2 := filepath.Join(dir, "user2.toml")
		writeTOML(t, userPath2, "[decide]\nroute_confidence = -0.1\n")
		if _, err := Load(userPath2, ""); err == nil {
			t.Fatal("expected an error for route_confidence outside [0,1]")
		}
	})

	t.Run("refuses route_timeout_ms out of range", func(t *testing.T) {
		dir := t.TempDir()

		tooLow := filepath.Join(dir, "low.toml")
		writeTOML(t, tooLow, "[decide]\nroute_timeout_ms = 50\n")
		if _, err := Load(tooLow, ""); err == nil {
			t.Fatal("expected an error for route_timeout_ms below 100")
		}

		tooHigh := filepath.Join(dir, "high.toml")
		writeTOML(t, tooHigh, "[decide]\nroute_timeout_ms = 30001\n")
		if _, err := Load(tooHigh, ""); err == nil {
			t.Fatal("expected an error for route_timeout_ms above 30000")
		}

		inRange := filepath.Join(dir, "ok.toml")
		writeTOML(t, inRange, "[decide]\nroute_timeout_ms = 100\n")
		loaded, err := Load(inRange, "")
		if err != nil {
			t.Fatalf("Load: %v (route_timeout_ms=100 should be in range)", err)
		}
		if loaded.Config.Decide.RouteTimeoutMS != 100 {
			t.Errorf("RouteTimeoutMS = %d, want 100", loaded.Config.Decide.RouteTimeoutMS)
		}
	})

	t.Run("mergeDecideFields sets sites and records provenance (ADR-044 decision 8)", func(t *testing.T) {
		var d Decide
		prov := Provenance{}
		mergeDecideFields(&d, map[string]any{
			"sites": []any{"route", "certify_suggest"},
		}, SourceProject, "project.toml", prov)
		want := []string{"route", "certify_suggest"}
		if strings.Join(d.Sites, ",") != strings.Join(want, ",") {
			t.Errorf("Sites = %v, want %v", d.Sites, want)
		}
		if info := prov["decide.sites"]; info.Source != SourceProject || info.Path != "project.toml" {
			t.Errorf("provenance[decide.sites] = %+v", info)
		}
	})

	t.Run("sites narrows the default allowlist when set, from TOML", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		writeTOML(t, userPath, `
[decide]
sites = ["route"]
`)
		loaded, err := Load(userPath, "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if strings.Join(loaded.Config.Decide.Sites, ",") != "route" {
			t.Errorf("Sites = %v, want [route]", loaded.Config.Decide.Sites)
		}
	})

	t.Run("an empty sites list disables every site", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		writeTOML(t, userPath, "[decide]\nsites = []\n")
		loaded, err := Load(userPath, "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(loaded.Config.Decide.Sites) != 0 {
			t.Errorf("Sites = %v, want an empty (never nil-defaulted) allowlist", loaded.Config.Decide.Sites)
		}
	})

	t.Run("refuses an unrecognised sites entry by name", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		writeTOML(t, userPath, `[decide]
sites = ["route", "made_up_site"]
`)
		_, err := Load(userPath, "")
		if err == nil {
			t.Fatal("expected an error for an unrecognised [decide] sites entry")
		}
		if !strings.Contains(err.Error(), "made_up_site") {
			t.Errorf("error = %v, want it to name the offending value %q", err, "made_up_site")
		}
	})

	t.Run("refuses rerank_timeout_ms out of range", func(t *testing.T) {
		dir := t.TempDir()

		tooLow := filepath.Join(dir, "low.toml")
		writeTOML(t, tooLow, "[decide]\nrerank_timeout_ms = 50\n")
		if _, err := Load(tooLow, ""); err == nil {
			t.Fatal("expected an error for rerank_timeout_ms below 100")
		}

		tooHigh := filepath.Join(dir, "high.toml")
		writeTOML(t, tooHigh, "[decide]\nrerank_timeout_ms = 60001\n")
		if _, err := Load(tooHigh, ""); err == nil {
			t.Fatal("expected an error for rerank_timeout_ms above 60000")
		}

		inRange := filepath.Join(dir, "ok.toml")
		writeTOML(t, inRange, "[decide]\nrerank_timeout_ms = 100\n")
		loaded, err := Load(inRange, "")
		if err != nil {
			t.Fatalf("Load: %v (rerank_timeout_ms=100 should be in range)", err)
		}
		if loaded.Config.Decide.RerankTimeoutMS != 100 {
			t.Errorf("RerankTimeoutMS = %d, want 100", loaded.Config.Decide.RerankTimeoutMS)
		}
	})
}

// TestConfig_SurfacePatternBlock pins CV-17 (ADR-044 §2.1): `[[surface.
// pattern]]` blocks parse into SurfacePatterns, project replaces the whole
// user-layer list (array-of-tables has no per-element merge key), and an
// absent table leaves SurfacePatterns empty.
func TestConfig_SurfacePatternBlock(t *testing.T) {
	t.Run("absent means empty", func(t *testing.T) {
		dir := t.TempDir()
		loaded, err := Load(filepath.Join(dir, "user.toml"), "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := SurfacePatterns(loaded.Config); len(got) != 0 {
			t.Errorf("SurfacePatterns = %v, want empty", got)
		}
	})

	t.Run("parses every field", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		writeTOML(t, userPath, `
[[surface.pattern]]
kind = "tool"
glob = "internal/mcp/*.go"
regex = 'Name:\s*"([a-zA-Z_][a-zA-Z0-9_]*)"'

[[surface.pattern]]
kind = "verb"
glob = "cmd/fathom/verbs.go"
regex = '\{Name:\s*"([a-z-]+)"'
`)
		loaded, err := Load(userPath, "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		got := SurfacePatterns(loaded.Config)
		if len(got) != 2 {
			t.Fatalf("SurfacePatterns = %+v, want 2 entries", got)
		}
		if got[0].Kind != "tool" || got[0].Glob != "internal/mcp/*.go" {
			t.Errorf("pattern[0] = %+v", got[0])
		}
		if got[1].Kind != "verb" || got[1].Glob != "cmd/fathom/verbs.go" {
			t.Errorf("pattern[1] = %+v", got[1])
		}
		if info := loaded.Provenance["surface.pattern"]; info.Source != SourceUser || info.Path != userPath {
			t.Errorf("provenance = %+v", info)
		}
	})

	t.Run("project replaces user's whole list", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		writeTOML(t, userPath, `
[[surface.pattern]]
kind = "tool"
regex = "Name"
`)
		projectPath := filepath.Join(dir, "project.toml")
		writeTOML(t, projectPath, `
[[surface.pattern]]
kind = "verb"
regex = "Use"
`)
		loaded, err := Load(userPath, projectPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		got := SurfacePatterns(loaded.Config)
		if len(got) != 1 || got[0].Kind != "verb" {
			t.Errorf("SurfacePatterns = %+v, want only the project layer's [verb]", got)
		}
	})
}

// ---------------------------------------------------------------------------
// ResolveProfile — named selection + env overlay
// ---------------------------------------------------------------------------

func TestResolveProfile_NoConfigNoEnv_DefaultsToAnthropic(t *testing.T) {
	clearProviderEnv(t)
	p, prov, err := ResolveProfile(Config{Profiles: map[string]Profile{}}, nil, "")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if p.Provider != ProviderAnthropic {
		t.Errorf("Provider = %q, want anthropic (built-in default)", p.Provider)
	}
	if prov["provider"].Source != SourceDefault {
		t.Errorf("provider source = %v, want default", prov["provider"].Source)
	}
}

func TestResolveProfile_UnknownFlagProfile_Errors(t *testing.T) {
	clearProviderEnv(t)
	_, _, err := ResolveProfile(Config{Profiles: map[string]Profile{"a": {Provider: "anthropic"}}}, nil, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "nonexistent") || !strings.Contains(err.Error(), "a") {
		t.Errorf("error = %v, want it to name the bad value and known profiles", err)
	}
}

func TestResolveProfile_DefaultProfile_SelectsNamedProfile(t *testing.T) {
	clearProviderEnv(t)
	cfg := Config{
		DefaultProfile: "work",
		Profiles: map[string]Profile{
			"work": {Provider: "bedrock", Region: "us-gov-west-1", Model: "m1"},
		},
	}
	p, _, err := ResolveProfile(cfg, nil, "")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if p.Provider != "bedrock" || p.Region != "us-gov-west-1" || p.Model != "m1" {
		t.Errorf("resolved = %+v", p)
	}
}

func TestResolveProfile_FlagProfile_OverridesDefault(t *testing.T) {
	clearProviderEnv(t)
	cfg := Config{
		DefaultProfile: "a",
		Profiles: map[string]Profile{
			"a": {Provider: "anthropic"},
			"b": {Provider: "codex-cli"},
		},
	}
	p, _, err := ResolveProfile(cfg, nil, "b")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if p.Provider != "codex-cli" {
		t.Errorf("Provider = %q, want codex-cli (the --profile flag choice)", p.Provider)
	}
}

func TestResolveProfile_EnvOverlay_WinsOverFileProfile(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("FATHOM_LLM_PROVIDER", "openai")
	t.Setenv("FATHOM_COMPILE_MODEL", "env-model")

	cfg := Config{
		DefaultProfile: "work",
		Profiles: map[string]Profile{
			"work": {Provider: "anthropic", Model: "file-model"},
		},
	}
	p, prov, err := ResolveProfile(cfg, nil, "")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if p.Provider != "openai" {
		t.Errorf("Provider = %q, want env override openai", p.Provider)
	}
	if p.Model != "env-model" {
		t.Errorf("Model = %q, want env override", p.Model)
	}
	if prov["provider"].Source != SourceEnv || prov["provider"].Path != "FATHOM_LLM_PROVIDER" {
		t.Errorf("provider provenance = %+v", prov["provider"])
	}
	if prov["model"].Source != SourceEnv || prov["model"].Path != "FATHOM_COMPILE_MODEL" {
		t.Errorf("model provenance = %+v", prov["model"])
	}
}

func TestResolveProfile_FileEndpointSurvivesWhenEnvUnset(t *testing.T) {
	clearProviderEnv(t)
	cfg := Config{
		Profiles: map[string]Profile{
			"gov": {
				Provider:   "azure-openai",
				Endpoint:   "https://example-resource.openai.azure.us",
				Deployment: "gov-deployment",
			},
		},
	}
	p, _, err := ResolveProfile(cfg, nil, "gov")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if p.Endpoint != "https://example-resource.openai.azure.us" {
		t.Errorf("Endpoint = %q, want the file-sourced Azure Government endpoint", p.Endpoint)
	}
	if p.Deployment != "gov-deployment" {
		t.Errorf("Deployment = %q", p.Deployment)
	}
}

func TestResolveProfile_DefaultAPIKeyEnvPerProvider(t *testing.T) {
	clearProviderEnv(t)
	cases := []struct {
		provider string
		want     string
	}{
		{ProviderAnthropic, "ANTHROPIC_API_KEY"},
		{ProviderOpenAI, "OPENAI_API_KEY"},
		{ProviderAzureOpenAI, "AZURE_OPENAI_API_KEY"},
		{ProviderBedrock, ""},
		{ProviderClaudeCLI, ""},
		{ProviderCodexCLI, ""},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			cfg := Config{Profiles: map[string]Profile{"p": {Provider: tc.provider}}}
			p, _, err := ResolveProfile(cfg, nil, "p")
			if err != nil {
				t.Fatalf("ResolveProfile: %v", err)
			}
			if p.APIKeyEnv != tc.want {
				t.Errorf("APIKeyEnv = %q, want %q", p.APIKeyEnv, tc.want)
			}
		})
	}
}

func TestResolveProfile_ExplicitAPIKeyEnv_NotOverriddenByDefault(t *testing.T) {
	clearProviderEnv(t)
	cfg := Config{Profiles: map[string]Profile{
		"p": {Provider: ProviderOpenAI, APIKeyEnv: "MY_CUSTOM_KEY"},
	}}
	p, _, err := ResolveProfile(cfg, nil, "p")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if p.APIKeyEnv != "MY_CUSTOM_KEY" {
		t.Errorf("APIKeyEnv = %q, want the profile's own override preserved", p.APIKeyEnv)
	}
}

// ---------------------------------------------------------------------------
// ResolveAPIKey — env + argv-exec indirection
// ---------------------------------------------------------------------------

func TestResolveAPIKey_Env(t *testing.T) {
	t.Setenv("FATHOM_TEST_KEY_VAR", "sekret-value")
	p := Profile{APIKeyEnv: "FATHOM_TEST_KEY_VAR"}
	key, err := p.ResolveAPIKey(context.Background())
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if key != "sekret-value" {
		t.Errorf("key = %q", key)
	}
}

func TestResolveAPIKey_NeitherSet_EmptyNoError(t *testing.T) {
	p := Profile{}
	key, err := p.ResolveAPIKey(context.Background())
	if err != nil || key != "" {
		t.Errorf("key=%q err=%v, want empty/nil", key, err)
	}
}

const sentinelKey = "sentinel-9f3a7c-do-not-leak"

func writeExecutableScript(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "keycmd.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveAPIKey_Cmd_TrimmedStdout(t *testing.T) {
	script := "#!/bin/sh\nprintf '" + sentinelKey + "\\n'\n"
	path := writeExecutableScript(t, script)

	p := Profile{APIKeyCmd: []string{path}}
	key, err := p.ResolveAPIKey(context.Background())
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if key != sentinelKey {
		t.Errorf("key = %q, want %q", key, sentinelKey)
	}
}

func TestResolveAPIKey_Cmd_WinsOverEnv(t *testing.T) {
	t.Setenv("FATHOM_TEST_KEY_VAR", "env-value-should-not-be-used")
	script := "#!/bin/sh\nprintf '" + sentinelKey + "'\n"
	path := writeExecutableScript(t, script)

	p := Profile{APIKeyEnv: "FATHOM_TEST_KEY_VAR", APIKeyCmd: []string{path}}
	key, err := p.ResolveAPIKey(context.Background())
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if key != sentinelKey {
		t.Errorf("key = %q, want the api_key_cmd result to win over api_key_env", key)
	}
}

// TestResolveAPIKey_Cmd_FailureNeverLeaksSecret is CF-2's secrecy contract:
// a failing api_key_cmd must not echo the sentinel value it printed (to
// stdout, before failing) anywhere in the returned error.
func TestResolveAPIKey_Cmd_FailureNeverLeaksSecret(t *testing.T) {
	script := "#!/bin/sh\nprintf '" + sentinelKey + "' >&2\nprintf '" + sentinelKey + "'\nexit 1\n"
	path := writeExecutableScript(t, script)

	p := Profile{APIKeyCmd: []string{path}}
	_, err := p.ResolveAPIKey(context.Background())
	if err == nil {
		t.Fatal("expected an error from the failing command")
	}
	if strings.Contains(err.Error(), sentinelKey) {
		t.Fatalf("error leaked the sentinel secret: %v", err)
	}
}

func TestResolveAPIKey_Cmd_NotFound(t *testing.T) {
	p := Profile{APIKeyCmd: []string{filepath.Join(t.TempDir(), "does-not-exist")}}
	_, err := p.ResolveAPIKey(context.Background())
	if err == nil {
		t.Fatal("expected an error for a nonexistent command")
	}
}

// TestDecide_ResolveAPIKey pins [decide]'s own credential indirection
// (provider "typesafe"): api_key_cmd wins over api_key_env, api_key_env
// wins over the well-known TYPESAFE_API_KEY fallback, and neither set falls
// back to reading TYPESAFE_API_KEY directly — the one place Decide.
// ResolveAPIKey diverges from Profile.ResolveAPIKey, which stops at "".
func TestDecide_ResolveAPIKey(t *testing.T) {
	t.Run("api_key_cmd", func(t *testing.T) {
		script := "#!/bin/sh\nprintf '" + sentinelKey + "'\n"
		path := writeExecutableScript(t, script)
		d := Decide{APIKeyCmd: []string{path}}
		key, err := d.ResolveAPIKey(context.Background())
		if err != nil {
			t.Fatalf("ResolveAPIKey: %v", err)
		}
		if key != sentinelKey {
			t.Errorf("key = %q, want %q", key, sentinelKey)
		}
	})

	t.Run("api_key_env", func(t *testing.T) {
		t.Setenv("FATHOM_TEST_KEY_VAR", "sekret-value")
		d := Decide{APIKeyEnv: "FATHOM_TEST_KEY_VAR"}
		key, err := d.ResolveAPIKey(context.Background())
		if err != nil {
			t.Fatalf("ResolveAPIKey: %v", err)
		}
		if key != "sekret-value" {
			t.Errorf("key = %q", key)
		}
	})

	t.Run("default env TYPESAFE_API_KEY", func(t *testing.T) {
		t.Setenv("TYPESAFE_API_KEY", "jev-default-key")
		d := Decide{}
		key, err := d.ResolveAPIKey(context.Background())
		if err != nil {
			t.Fatalf("ResolveAPIKey: %v", err)
		}
		if key != "jev-default-key" {
			t.Errorf("key = %q, want the TYPESAFE_API_KEY fallback", key)
		}
	})

	t.Run("none set", func(t *testing.T) {
		t.Setenv("TYPESAFE_API_KEY", "")
		os.Unsetenv("TYPESAFE_API_KEY")
		d := Decide{}
		key, err := d.ResolveAPIKey(context.Background())
		if err != nil || key != "" {
			t.Errorf("key=%q err=%v, want empty/nil", key, err)
		}
	})
}

// clearProviderEnv scrubs every env var applyEnvOverlay reads, so tests are
// hermetic regardless of the developer's or CI's ambient environment.
func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{
		"FATHOM_LLM_PROVIDER", "FATHOM_COMPILE_MODEL",
		"FATHOM_CLAUDE_BIN", "FATHOM_CODEX_BIN", "FATHOM_OPENAI_BASE_URL",
		"AZURE_OPENAI_ENDPOINT", "AZURE_OPENAI_DEPLOYMENT", "AZURE_OPENAI_API_VERSION",
		"AWS_REGION",
	} {
		t.Setenv(v, "")
	}
}

// RT-145: `[connector] include` validates and loads, so a corpus that
// re-includes a default-ignored cache directory does not break `fathom
// config` or any command that loads the project layer.
func TestLoad_ConnectorInclude(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project.toml")
	writeTOML(t, project, "[connector]\ninclude = [\"coverage\", \"compute/.tools\"]\n")
	loaded, err := Load("", project)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loaded.Config.Connector.Include, []string{"coverage", "compute/.tools"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Connector.Include = %v, want %v", got, want)
	}
	if loaded.Provenance["connector.include"].Source != SourceProject {
		t.Fatalf("connector.include provenance = %+v", loaded.Provenance["connector.include"])
	}
}

// RT-145 review: `[connector] include` is a property of the corpus — the
// connector reads it only from the corpus's own .fathom/config.toml. In the
// user layer it would validate, show in `fathom config --explain`, and do
// nothing; it is refused by name instead.
func TestLoad_ConnectorIncludeRefusedInUserLayer(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user.toml")
	writeTOML(t, user, "[connector]\ninclude = [\"dist\"]\n")
	_, err := Load(user, "")
	var le *LoadError
	if !errors.As(err, &le) || le.Path != user || !strings.Contains(err.Error(), ".fathom/config.toml") {
		t.Fatalf("user-layer connector.include: err = %v, want a LoadError naming %s and the corpus file it belongs in", err, user)
	}
}

// EW-34: a claude-cli profile's timeout key loads, rides the env overlay and
// an invalid duration is refused by the schema at load.
func TestLoad_ClaudeCLIProfileTimeout(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, "[profiles.sub]\nprovider = \"claude-cli\"\ntimeout = \"20m\"\n")
	loaded, err := Load(userPath, filepath.Join(dir, "project.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.Config.Profiles["sub"].Timeout; got != "20m" {
		t.Fatalf("Timeout = %q, want 20m", got)
	}
	writeTOML(t, userPath, "[profiles.sub]\nprovider = \"claude-cli\"\ntimeout = \"soon\"\n")
	if _, err := Load(userPath, filepath.Join(dir, "project.toml")); err == nil {
		t.Fatal("a non-duration timeout must be refused at load")
	}
}

// TestLoad_ReceiptsRepositoryURI pins `[receipts] repository_uri`: an
// optional absolute URI (empty when unset) a receipt's SARIF names in
// versionControlProvenance. It decodes from either layer with its
// provenance, the project layer wins, and a value without a scheme is
// refused by the schema at load: fathom never derives one from a git
// remote (ADR-011), so the repository states it whole.
func TestLoad_ReceiptsRepositoryURI(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user.toml")
	project := filepath.Join(dir, "project.toml")
	loaded, err := Load("", project)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Config.Receipts.RepositoryURI; got != "" {
		t.Fatalf("unset RepositoryURI = %q, want empty", got)
	}
	writeTOML(t, user, "[receipts]\nrepository_uri = \"https://example.test/user/repo\"\n")
	writeTOML(t, project, "[receipts]\nrepository_uri = \"https://example.test/org/repo\"\n")
	loaded, err = Load(user, project)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loaded.Config.Receipts.RepositoryURI, "https://example.test/org/repo"; got != want {
		t.Fatalf("RepositoryURI = %q, want %q (the project layer wins)", got, want)
	}
	if p := loaded.Provenance["receipts.repository_uri"]; p.Source != SourceProject || p.Path != project {
		t.Fatalf("receipts.repository_uri provenance = %+v, want the project file", p)
	}
	writeTOML(t, project, "[receipts]\nrepository_uri = \"example.test/org/repo\"\n")
	if _, err := Load("", project); err == nil || !strings.Contains(err.Error(), "repository_uri") {
		t.Fatalf("a repository_uri with no scheme: err = %v, want a load error naming repository_uri", err)
	}
}

// TestLoad_TypedSkipModules pins `[typed] skip_modules` (EW-68): repo-relative
// directory globs the typed pass does not load a nested Go module under. It
// decodes from the repository's own config with its provenance, an empty
// glob is a schema error, and like `[connector] include` it is refused in
// the user layer, where it would validate and change nothing (the typed pass
// reads it only from the corpus's own .fathom/config.toml).
func TestLoad_TypedSkipModules(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project.toml")
	writeTOML(t, project, "[typed]\nskip_modules = [\"calibration/oracles/**\", \"tools/gen\"]\n")
	loaded, err := Load("", project)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loaded.Config.Typed.SkipModules, []string{"calibration/oracles/**", "tools/gen"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Typed.SkipModules = %v, want %v", got, want)
	}
	if p := loaded.Provenance["typed.skip_modules"]; p.Source != SourceProject || p.Path != project {
		t.Fatalf("typed.skip_modules provenance = %+v, want the project file", p)
	}
	writeTOML(t, project, "[typed]\nskip_modules = [\"\"]\n")
	if _, err := Load("", project); err == nil || !strings.Contains(err.Error(), "skip_modules") {
		t.Fatalf("an empty skip_modules glob: err = %v, want a load error naming skip_modules", err)
	}
	user := filepath.Join(dir, "user.toml")
	writeTOML(t, user, "[typed]\nskip_modules = [\"calibration/**\"]\n")
	_, err = Load(user, "")
	var le *LoadError
	if !errors.As(err, &le) || le.Path != user || !strings.Contains(err.Error(), ".fathom/config.toml") {
		t.Fatalf("user-layer typed.skip_modules: err = %v, want a LoadError naming %s and the corpus file it belongs in", err, user)
	}
	// A repo-path field: an escaping glob is named by RefusedRepoPaths.
	refused := RefusedRepoPaths(dir, Config{Typed: Typed{SkipModules: []string{"ok/**", "../out/**", "/abs/**"}}}, nil)
	if len(refused) != 2 || refused[0].Field != "typed.skip_modules[1]" || refused[1].Field != "typed.skip_modules[2]" {
		t.Fatalf("RefusedRepoPaths = %+v, want typed.skip_modules[1] and [2]", refused)
	}
}
