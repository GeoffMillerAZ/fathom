package decide_test

// keyscope_test.go — SEC-1 (ADR-042 amendment 2026-10-09, DP-40): a
// repository's own .fathom/config.toml cannot make fathom run a command to
// fetch a credential. Through config.Load and decide.Open, the repo's
// api_key_cmd never runs.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

func TestOpen_RepoAPIKeyCmdNeverRuns(t *testing.T) {
	unsetProviderKeys(t)
	for _, provider := range []string{"openai", "typesafe"} {
		t.Run(provider, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "pwned")
			dir := t.TempDir()
			proj := filepath.Join(dir, "repo", ".fathom", "config.toml")
			if err := os.MkdirAll(filepath.Dir(proj), 0o755); err != nil {
				t.Fatal(err)
			}
			body := "[decide]\nprovider = \"" + provider + "\"\napi_key_cmd = [\"/bin/sh\", \"-c\", \"touch " + marker + "\"]\n"
			if err := os.WriteFile(proj, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			loaded, err := config.Load("", proj)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decide.Open(loaded.Config.Decide, nil, "", nil); err == nil {
				t.Fatal("Open succeeded without any key; want the no-key refusal")
			}
			if err := decide.CheckRemote(loaded.Config.Decide); err != nil {
				t.Logf("CheckRemote: %v", err)
			}
			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("the repository's api_key_cmd ran: %s exists", marker)
			}
		})
	}
}
