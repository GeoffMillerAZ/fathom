package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decidekey_test.go — DP-28..DP-30 (docs/plans/decision-port.md section H):
// [decide] names a remote provider's key by file or variable, never by
// value, and every check of that source reports where the key is, never
// what it is.

// keySentinel stands in for a key in these tests: a value the errors and
// details under test must never contain.
const keySentinel = "sentinel-7c41e0-key-material-never-printed"

func TestLoad_DecideRemoteProviderKeys(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "project.toml")
	writeTOML(t, userPath, `
[decide]
provider     = "typesafe"
transport    = "openrouter"
api_key_file = "~/keys/openrouter"
`)
	writeTOML(t, projectPath, `
[decide]
provider    = "openai"
model       = "gpt-6-luna"
base_url    = "https://api.openai.com/v1"
transport   = "openai"
api_key_env = "OPENAI_API_KEY"
`)
	loaded, err := Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	d := loaded.Config.Decide
	if d.Provider != "openai" || d.Model != "gpt-6-luna" || d.BaseURL != "https://api.openai.com/v1" || d.Transport != "openai" || d.APIKeyEnv != "" {
		t.Errorf("Decide = %+v", d)
	}
	// SEC-1: the key source is the user's. The project layer's api_key_env
	// is dropped (and named in IgnoredKeySources); the user's key file, an
	// OpenRouter key, stays bound to the transport it was named for, so it
	// cannot travel to the project's OpenAI port.
	if d.APIKeyEnv != "" {
		t.Errorf("APIKeyEnv = %q, want the project layer's key source dropped", d.APIKeyEnv)
	}
	if d.APIKeyFile != "~/keys/openrouter" {
		t.Errorf("APIKeyFile = %q, want the user layer's", d.APIKeyFile)
	}
	if d.KeyTransport != DecideTransportOpenRouter {
		t.Errorf("KeyTransport = %q, want openrouter (the transport in effect where the user named the key)", d.KeyTransport)
	}
	if d.BaseURLSource != SourceProject {
		t.Errorf("BaseURLSource = %q, want project (the layer that set base_url)", d.BaseURLSource)
	}
	for key, want := range map[string]Source{
		"decide.base_url": SourceProject, "decide.transport": SourceProject, "decide.api_key_file": SourceUser,
	} {
		if info := loaded.Provenance[key]; info.Source != want {
			t.Errorf("provenance[%q] = %+v, want %s", key, info, want)
		}
	}
	if info, ok := loaded.Provenance["decide.api_key_env"]; ok {
		t.Errorf("provenance[decide.api_key_env] = %+v, want none (a repository's key source is dropped)", info)
	}
}

func TestLoad_DecideKeyTransportIsTheNamingLayers(t *testing.T) {
	for _, tc := range []struct{ name, user, project, want string }{
		{"no layer names a key", "provider = \"openai\"", "", ""},
		{"the user layer names it beside its transport", "provider = \"typesafe\"\ntransport = \"openrouter\"\napi_key_file = \"~/keys/or\"", "compile_threshold = 0.7", DecideTransportOpenRouter},
		{"a project layer moves the port without naming a key", "provider = \"typesafe\"\napi_key_file = \"~/keys/ts\"", "transport = \"openrouter\"", DecideTransportTypeSafe},
		{"the key is named where no remote provider is set", "api_key_file = \"~/keys/x\"", "provider = \"openai\"", KeyTransportNone},
		{"the project layer names provider and key together (the key is dropped)", "api_key_cmd = [\"op\", \"read\"]", "provider = \"openai\"\napi_key_env = \"OPENAI_API_KEY\"", KeyTransportNone}, // the repository's key source is dropped; the user's helper stays bound to its own (no-provider) layer
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			userPath, projectPath := filepath.Join(dir, "user.toml"), ""
			writeTOML(t, userPath, "[decide]\n"+tc.user+"\n")
			if tc.project != "" {
				projectPath = filepath.Join(dir, "project.toml")
				writeTOML(t, projectPath, "[decide]\n"+tc.project+"\n")
			}
			loaded, err := Load(userPath, projectPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := loaded.Config.Decide.KeyTransport; got != tc.want {
				t.Errorf("KeyTransport = %q, want %q", got, tc.want)
			}
			if strings.HasPrefix(tc.name, "the project layer names provider and key together") && loaded.Config.Decide.APIKeyEnv != "" {
				t.Errorf("APIKeyEnv = %q, want the repository's key source dropped", loaded.Config.Decide.APIKeyEnv)
			}
		})
	}
}

func TestLoad_DecideEndpointRecordsItsLayer(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, "[decide]\nprovider = \"typesafe\"\nendpoint = \"https://api.typesafe.ai\"\n")
	loaded, err := Load(userPath, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.Config.Decide.BaseURLSource; got != SourceUser {
		t.Errorf("BaseURLSource = %q, want user", got)
	}
	if got := loaded.Config.Decide.EffectiveBaseURL(); got != "https://api.typesafe.ai" {
		t.Errorf("EffectiveBaseURL = %q, want the endpoint (base_url's older name)", got)
	}
	both := Decide{BaseURL: "https://a.example", Endpoint: "https://b.example"}
	if got := both.EffectiveBaseURL(); got != "https://a.example" {
		t.Errorf("EffectiveBaseURL with both = %q, want base_url", got)
	}
}

func TestLoad_DecideInlineAPIKeyRejectedPointingAtKeyFile(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	writeTOML(t, userPath, "[decide]\nprovider = \"openai\"\napi_key = \""+keySentinel+"\"\n")
	_, err := Load(userPath, "")
	if err == nil {
		t.Fatal("expected [decide] api_key to be refused")
	}
	if !strings.Contains(err.Error(), "api_key_file") {
		t.Errorf("error = %v, want it to point at api_key_file", err)
	}
	if strings.Contains(err.Error(), keySentinel) {
		t.Errorf("error echoes the attempted key: %v", err)
	}
}

func TestLoad_DecideKeyFieldsNeverEchoAValue(t *testing.T) {
	for _, tc := range []struct{ name, toml, field string }{
		{"a key pasted into api_key_file", "api_key_file = \"" + keySentinel + "\"", "api_key_file"},
		{"a key pasted into api_key_env", "api_key_env = \"" + keySentinel + "\"", "api_key_env"},
		{"a relative api_key_file", "api_key_file = \"secrets/token\"", "api_key_file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "user.toml")
			writeTOML(t, userPath, "[decide]\nprovider = \"openai\"\n"+tc.toml+"\n")
			_, err := Load(userPath, "")
			if err == nil {
				t.Fatalf("expected %s to be refused", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error = %v, want it to name %s", err, tc.field)
			}
			if strings.Contains(err.Error(), keySentinel) || strings.Contains(err.Error(), "secrets/token") {
				t.Errorf("error echoes the value: %v", err)
			}
		})
	}
}

func TestLoad_DecideTransportMustFitProvider(t *testing.T) {
	for _, tc := range []struct {
		name, toml string
		ok         bool
	}{
		{"typesafe over openrouter", "provider = \"typesafe\"\ntransport = \"openrouter\"", true},
		{"typesafe over its own API", "provider = \"typesafe\"\ntransport = \"typesafe\"", true},
		{"openai over its own API", "provider = \"openai\"\ntransport = \"openai\"", true},
		{"openai over openrouter", "provider = \"openai\"\ntransport = \"openrouter\"", false},
		{"typesafe over openai", "provider = \"typesafe\"\ntransport = \"openai\"", false},
		{"an unknown transport", "transport = \"carrier-pigeon\"", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "user.toml")
			writeTOML(t, userPath, "[decide]\n"+tc.toml+"\n")
			_, err := Load(userPath, "")
			if tc.ok && err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !tc.ok {
				if err == nil {
					t.Fatal("expected the transport to be refused")
				}
				if !strings.Contains(err.Error(), "transport") {
					t.Errorf("error = %v, want it to name transport", err)
				}
			}
		})
	}
}

func TestDecide_KeySourcePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name string
		d    Decide
		want string
	}{
		{"api_key_cmd wins", Decide{Provider: "openai", APIKeyCmd: []string{"helper"}, APIKeyFile: "~/k", APIKeyEnv: "K"}, `api_key_cmd "helper"`},
		{"api_key_file over api_key_env", Decide{Provider: "openai", APIKeyFile: "~/keys/openai", APIKeyEnv: "K"}, `key file "~/keys/openai"`},
		{"api_key_env", Decide{Provider: "typesafe", APIKeyEnv: "MY_JEV_KEY"}, `environment variable "MY_JEV_KEY"`},
		{"typesafe default variable", Decide{Provider: "typesafe"}, `environment variable "TYPESAFE_API_KEY"`},
		{"openrouter default variable", Decide{Provider: "typesafe", Transport: "openrouter"}, `environment variable "OPENROUTER_API_KEY"`},
		{"openai default variable", Decide{Provider: "openai"}, `environment variable "OPENAI_API_KEY"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.d.KeySource().String(); got != tc.want {
				t.Errorf("KeySource = %s, want %s", got, tc.want)
			}
		})
	}
	if got := (Decide{Provider: "typesafe", Transport: "openrouter"}).EffectiveTransport(); got != "openrouter" {
		t.Errorf("EffectiveTransport = %q, want openrouter", got)
	}
	if got := (Decide{Provider: "openai"}).EffectiveTransport(); got != "openai" {
		t.Errorf("EffectiveTransport(openai) = %q, want openai", got)
	}
	if got := (Decide{Provider: "llm"}).EffectiveTransport(); got != "" {
		t.Errorf("EffectiveTransport(llm) = %q, want none", got)
	}
}

// TestKeySource_ProbeNeverReadsOutTheKey: a probe reports whether the
// source resolves (a file present and private, a variable set) and names
// the source and the fix; no detail or error carries the key.
func TestKeySource_ProbeNeverReadsOutTheKey(t *testing.T) {
	dir := t.TempDir()
	private := filepath.Join(dir, "private")
	if err := os.WriteFile(private, []byte(keySentinel+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	open := filepath.Join(dir, "open")
	if err := os.WriteFile(open, []byte(keySentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(open, 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_TEST_DECIDE_KEY", keySentinel)
	t.Setenv("FATHOM_TEST_DECIDE_BLANK", "  ")
	script := writeExecutableScript(t, "#!/bin/sh\nprintf '"+keySentinel+"'\n")

	for _, tc := range []struct {
		name    string
		src     KeySource
		wantErr string
		detail  string
	}{
		{"a private file", KeySource{File: private}, "", "present, mode 0600"},
		{"a file others can read", KeySource{File: open}, "chmod 600", ""},
		{"a missing file", KeySource{File: filepath.Join(dir, "missing")}, "not found", ""},
		{"an empty file", KeySource{File: empty}, "empty", ""},
		{"a set variable", KeySource{Env: "FATHOM_TEST_DECIDE_KEY"}, "", "set"},
		{"a blank variable", KeySource{Env: "FATHOM_TEST_DECIDE_BLANK"}, "empty", ""},
		{"an unset variable", KeySource{Env: "FATHOM_TEST_DECIDE_UNSET"}, "not set", ""},
		{"a credential helper", KeySource{Cmd: []string{script}}, "", "returned a key"},
		{"no source", KeySource{}, "no API key source", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			detail, err := tc.src.Probe(context.Background())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Probe err = %v, want it to contain %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Probe: %v", err)
			}
			if tc.detail != "" && detail != tc.detail {
				t.Errorf("detail = %q, want %q", detail, tc.detail)
			}
			if strings.Contains(detail, keySentinel) || (err != nil && strings.Contains(err.Error(), keySentinel)) {
				t.Errorf("probe carries the key: detail=%q err=%v", detail, err)
			}
		})
	}
	if t.Failed() {
		return
	}
	// The file's mode is part of the fix the error names.
	_, err := KeySource{File: open}.Probe(context.Background())
	if !strings.Contains(err.Error(), open) {
		t.Errorf("err = %v, want it to name the path", err)
	}
}

func TestKeySource_ProbeExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "secrets", "p"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "secrets", "p", "token"), []byte(keySentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	detail, err := KeySource{File: "~/secrets/p/token"}.Probe(context.Background())
	if err != nil || detail != "present, mode 0600" {
		t.Fatalf("Probe = %q, %v", detail, err)
	}
}
