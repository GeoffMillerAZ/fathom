package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

// composeFixture is a docker-compose file with 3 services, images, and depends_on
// in both list and map forms.
const composeFixture = `version: "3.9"
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_DB: appdb

  cache:
    image: redis:7-alpine
    depends_on:
      - db

  app:
    image: registry.example.com/app:latest
    depends_on:
      db:
        condition: service_healthy
      cache:
        condition: service_started
`

// composeBuildFixture covers the build: context scalar form (no image).
const composeBuildFixture = `services:
  builder:
    build: ./myapp
  helper:
    build:
      context: ./tools
      dockerfile: Dockerfile.dev
`

// composeEmptyFixture has no services key.
const composeEmptyFixture = `version: "3.9"
networks:
  default: {}
`

func TestComposeExtractor_Basic(t *testing.T) {
	x := extractor.NewComposeExtractor()
	nodes, err := x.Extract("svc", "docker-compose.yml", []byte(composeFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %v", len(nodes), composeNodeNames(nodes))
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("db service", func(t *testing.T) {
		n, ok := byName["db"]
		if !ok {
			t.Fatalf("expected service 'db'; got: %v", composeNodeNames(nodes))
		}
		if n.Kind != core.KindComposeService {
			t.Errorf("Kind: got %q, want KindComposeService", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
		if n.Signature != "postgres:16" {
			t.Errorf("Signature: got %q, want postgres:16", n.Signature)
		}
		if len(n.Calls) != 0 {
			t.Errorf("Calls: got %v, want empty", n.Calls)
		}
		if n.Line == 0 {
			t.Errorf("Line should be non-zero")
		}
	})

	t.Run("cache service (list depends_on)", func(t *testing.T) {
		n, ok := byName["cache"]
		if !ok {
			t.Fatalf("expected service 'cache'; got: %v", composeNodeNames(nodes))
		}
		if n.Kind != core.KindComposeService {
			t.Errorf("Kind: got %q, want KindComposeService", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
		if n.Signature != "redis:7-alpine" {
			t.Errorf("Signature: got %q, want redis:7-alpine", n.Signature)
		}
		if len(n.Calls) != 1 || n.Calls[0] != "db" {
			t.Errorf("Calls: got %v, want [db]", n.Calls)
		}
	})

	t.Run("app service (map depends_on)", func(t *testing.T) {
		n, ok := byName["app"]
		if !ok {
			t.Fatalf("expected service 'app'; got: %v", composeNodeNames(nodes))
		}
		if n.Signature != "registry.example.com/app:latest" {
			t.Errorf("Signature: got %q, want registry.example.com/app:latest", n.Signature)
		}
		// depends_on map form: both db and cache should be in Calls (sorted).
		wantCalls := []string{"cache", "db"}
		if len(n.Calls) != len(wantCalls) {
			t.Fatalf("Calls: got %v, want %v", n.Calls, wantCalls)
		}
		for i, want := range wantCalls {
			if n.Calls[i] != want {
				t.Errorf("Calls[%d]: got %q, want %q", i, n.Calls[i], want)
			}
		}
	})
}

func TestComposeExtractor_BuildContext(t *testing.T) {
	x := extractor.NewComposeExtractor()
	nodes, err := x.Extract("svc", "compose.yml", []byte(composeBuildFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %v", len(nodes), composeNodeNames(nodes))
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("scalar build string", func(t *testing.T) {
		n, ok := byName["builder"]
		if !ok {
			t.Fatalf("expected service 'builder'; got: %v", composeNodeNames(nodes))
		}
		if n.Signature != "build: ./myapp" {
			t.Errorf("Signature: got %q, want 'build: ./myapp'", n.Signature)
		}
	})

	t.Run("build mapping with context", func(t *testing.T) {
		n, ok := byName["helper"]
		if !ok {
			t.Fatalf("expected service 'helper'; got: %v", composeNodeNames(nodes))
		}
		if n.Signature != "build: ./tools" {
			t.Errorf("Signature: got %q, want 'build: ./tools'", n.Signature)
		}
	})
}

func TestComposeExtractor_NoServices(t *testing.T) {
	x := extractor.NewComposeExtractor()
	nodes, err := x.Extract("svc", "compose.yml", []byte(composeEmptyFixture))
	if err != nil {
		t.Fatalf("Extract must not error: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for compose with no services, got %d", len(nodes))
	}
}

func TestComposeExtractor_Malformed(t *testing.T) {
	x := extractor.NewComposeExtractor()
	nodes, err := x.Extract("svc", "compose.yml", []byte("not: valid: yaml: :::"))
	if err != nil {
		t.Fatalf("Extract must not return an error for malformed YAML, got: %v", err)
	}
	// Best-effort: may return 0 nodes.
	_ = nodes
}

func TestComposeExtractor_TierIsVerified(t *testing.T) {
	x := extractor.NewComposeExtractor()
	nodes, _ := x.Extract("svc", "docker-compose.yml", []byte(composeFixture))
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("service %q: Tier = %q, want TierVerified", n.Name, n.Tier)
		}
	}
}

func TestComposeExtractor_Determinism(t *testing.T) {
	x := extractor.NewComposeExtractor()
	n1, err := x.Extract("svc", "docker-compose.yml", []byte(composeFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "docker-compose.yml", []byte(composeFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Line != n2[i].Line {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
		}
	}
}

// composeNodeNames returns Kind:Name for each node for diagnostics.
func composeNodeNames(nodes []core.SymbolNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = string(n.Kind) + ":" + n.Name
	}
	return out
}
