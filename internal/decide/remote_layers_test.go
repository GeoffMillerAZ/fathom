package decide_test

// remote_layers_test.go — DP-32 across config layers: a key goes only to
// the provider it was named for. The key source (api_key_cmd, api_key_file,
// api_key_env) is one setting taken whole from the layer that names it, and
// a layer that moves the port to another provider or transport without
// naming its own key turns the port off instead of sending the other
// provider's key there.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

// envKey stands in for a key held in a variable: a value distinct from
// testKey (the key file's), so a captured Authorization header says which
// source reached the wire.
const envKey = "env-key-93be17-not-a-real-key"

// loadLayers writes user and project [decide] blocks (either may be "")
// and loads them as fathom does: user first, project over it.
func loadLayers(t *testing.T, user, project string) config.Decide {
	t.Helper()
	dir := t.TempDir()
	userPath, projectPath := "", ""
	if user != "" {
		userPath = filepath.Join(dir, "user.toml")
		if err := os.WriteFile(userPath, []byte("[decide]\n"+user+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if project != "" {
		projectPath = filepath.Join(dir, "project.toml")
		if err := os.WriteFile(projectPath, []byte("[decide]\n"+project+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := config.Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loaded.Config.Decide
}

func TestOpen_KeyStaysWithTheProviderItWasNamedFor(t *testing.T) {
	predicateReply := `{"model":"gpt-6-luna","answers":[{"type":"predicate","name":"q1","probability":0.8}]}`
	noulReply := `{"model":"typesafe/jev-1.13-20260917","answers":{"q1":{"type":"noul","noul":0.7}}}`

	t.Run("a project layer's own key source is dropped, so the user layer's key stays with its transport", func(t *testing.T) {
		unsetProviderKeys(t)
		t.Setenv("OPENAI_API_KEY", envKey)
		c := &capture{reply: predicateReply}
		srv := c.server(t)
		keyFile := writeKeyFile(t, 0o600)
		d := loadLayers(t,
			`provider = "typesafe"
transport = "openrouter"
api_key_file = "`+keyFile+`"`,
			`provider = "openai"
transport = "openai"
base_url = "`+srv.URL+`"
api_key_env = "OPENAI_API_KEY"`)
		ports, err := decide.Open(d, nil, "", nil)
		if err == nil {
			_, _ = ports.Decider.Decide(context.Background(), predicateRequest("q1"))
			t.Fatalf("Open succeeded; a repository's api_key_env or the user's OpenRouter key reached the wire (auth=%q)", c.auth)
		}
		if !strings.Contains(err.Error(), "named for") {
			t.Errorf("Open err = %v, want it to say which transport the key was named for", err)
		}
		if c.auth != "" {
			t.Errorf("a request was sent with auth %q", c.auth)
		}
	})

	for _, tc := range []struct{ name, user, project string }{
		{
			"a project layer switching provider without naming a key",
			`provider = "typesafe"
transport = "openrouter"
api_key_file = "%KEYFILE%"`,
			`provider = "openai"
transport = "openai"
base_url = "%URL%"`,
		},
		{
			"a project layer switching transport without naming a key",
			`provider = "typesafe"
api_key_file = "%KEYFILE%"`,
			`transport = "openrouter"
base_url = "%URL%"`,
		},
		{
			"a key named where no remote provider was set",
			`api_key_file = "%KEYFILE%"`,
			`provider = "openai"
base_url = "%URL%"`,
		},
	} {
		t.Run(tc.name+" turns the port off", func(t *testing.T) {
			unsetProviderKeys(t)
			c := &capture{reply: predicateReply}
			srv := c.server(t)
			keyFile := writeKeyFile(t, 0o600)
			fill := strings.NewReplacer("%KEYFILE%", keyFile, "%URL%", srv.URL)
			d := loadLayers(t, fill.Replace(tc.user), fill.Replace(tc.project))
			ports, err := decide.Open(d, nil, "", nil)
			if err == nil {
				_, _ = ports.Decider.Decide(context.Background(), predicateRequest("q1"))
				t.Fatalf("Open succeeded; the key file named for another provider reached the wire: %v", c.auth == "Bearer "+testKey)
			}
			if !strings.Contains(err.Error(), "named for") {
				t.Errorf("Open err = %v, want it to say which transport the key was named for", err)
			}
			if strings.Contains(err.Error(), testKey) {
				t.Errorf("Open err carries the key: %v", err)
			}
			if err := decide.CheckRemote(d); err == nil {
				t.Errorf("CheckRemote = nil, want the same refusal (fathom doctor reads it)")
			}
		})
	}

	for _, tc := range []struct{ name, user, project string }{
		{
			"a project layer that keeps the user layer's transport",
			`provider = "typesafe"
transport = "openrouter"
base_url = "%URL%"
api_key_file = "%KEYFILE%"`,
			`compile_threshold = 0.7`,
		},
		{
			"a user layer naming provider, transport and key together",
			`provider = "typesafe"
transport = "openrouter"
base_url = "%URL%"
api_key_file = "%KEYFILE%"`,
			``,
		},
		{
			"a project layer restating the user layer's provider",
			`provider = "typesafe"
transport = "openrouter"
api_key_file = "%KEYFILE%"`,
			`provider = "typesafe"
base_url = "%URL%"`,
		},
	} {
		t.Run(tc.name+" still opens", func(t *testing.T) {
			unsetProviderKeys(t)
			c := &capture{reply: noulReply}
			srv := c.server(t)
			keyFile := writeKeyFile(t, 0o600)
			fill := strings.NewReplacer("%KEYFILE%", keyFile, "%URL%", srv.URL)
			d := loadLayers(t, fill.Replace(tc.user), fill.Replace(tc.project))
			ports, err := decide.Open(d, nil, "", nil)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if _, err := ports.Decider.Decide(context.Background(), predicateRequest("q1")); err != nil {
				t.Fatalf("Decide: %v", err)
			}
			if c.auth != "Bearer "+testKey {
				t.Errorf("the request did not carry the key file's key")
			}
		})
	}
}
