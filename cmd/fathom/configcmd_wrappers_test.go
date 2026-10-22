package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
)

func TestConfigExplainAttributesAppendedWrappers(t *testing.T) {
	_, repo := withIsolatedConfigHome(t)
	writeUserConfig(t, "[shell]\nwrappers = [{prefix = 'uv run'}]\n")
	writeProjectConfig(t, repo, "[shell]\nwrappers = [{prefix = 'bundle exec'}]\n")
	for _, args := range [][]string{{"--explain"}, {"--explain", "--json"}} {
		var out, errs bytes.Buffer
		if code := cmdConfig(args, &out, &errs); code != 0 {
			t.Fatalf("exit=%d error=%s", code, errs.String())
		}
		if len(args) == 1 {
			for _, want := range []string{"uv run", "bundle exec", "user:" + config.DefaultUserPath(), "project:" + filepath.Join(repo, ".fathom", "config.toml")} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in %s", want, out.String())
				}
			}
		} else {
			var result struct {
				Shell struct {
					Wrappers []map[string]any `json:"wrappers"`
				} `json:"shell"`
				Explain map[string]string `json:"explain"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Shell.Wrappers) != 2 || result.Explain["shell.wrappers[0]"] != "user:"+config.DefaultUserPath() || result.Explain["shell.wrappers[1]"] != "project:"+filepath.Join(repo, ".fathom", "config.toml") {
				t.Errorf("incorrect appended sources: %s", out.String())
			}
		}
	}
}
