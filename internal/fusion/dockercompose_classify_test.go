package fusion

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// TestDockerComposeClassification (LB-Q-docker-compose, design §5): the
// manifest's basenames and globs claim Compose's conventional file names —
// the four the CLI discovers, and the per-environment / override variants
// beside them at any depth — without taking a neighbouring YAML file from the
// dialects that own it.
func TestDockerComposeClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
		by         string
	}{
		{"compose.yaml", "docker-compose", connector.ClassifiedByBasename},
		{"a/b/docker-compose.yml", "docker-compose", connector.ClassifiedByBasename},
		{"docker-compose.override.yaml", "docker-compose", connector.ClassifiedByGlob},
		{"deploy/docker-compose.prod.yml", "docker-compose", connector.ClassifiedByGlob},
		{"services/api/compose.dev.yaml", "docker-compose", connector.ClassifiedByGlob},
		{"docker-compose-ci.yml", "docker-compose", connector.ClassifiedByGlob},
		{"e2e/fixtures/compose-with-extends.yaml", "docker-compose", connector.ClassifiedByGlob},
		{"k8s/deploy.yaml", "kubernetes", connector.ClassifiedByExtension},
		{"docker-compose.yml.bak", "", connector.ClassifiedNone},
		{"composer.json", "", connector.ClassifiedNone},
		{"docs/composer-notes.yaml", "kubernetes", connector.ClassifiedByExtension},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, nil)
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
}
