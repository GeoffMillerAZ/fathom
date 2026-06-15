package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

const tomlFixture = `
title = "My App"
version = "1.0.0"
debug = false

[database]
host = "localhost"
port = 5432

[server]
port = 8080
timeout = 30
`

func TestTOMLExtractor_Basic(t *testing.T) {
	x := extractor.NewTOMLExtractor()
	nodes, err := x.Extract("svc", "config.toml", []byte(tomlFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("scalar key title", func(t *testing.T) {
		n, ok := byName["title"]
		if !ok {
			t.Fatal("expected symbol title")
		}
		if n.Kind != core.KindTOMLKey {
			t.Errorf("Kind: got %q, want KindTOMLKey", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("scalar key version", func(t *testing.T) {
		n, ok := byName["version"]
		if !ok {
			t.Fatal("expected symbol version")
		}
		if n.Kind != core.KindTOMLKey {
			t.Errorf("Kind: got %q, want KindTOMLKey", n.Kind)
		}
	})

	t.Run("scalar key debug", func(t *testing.T) {
		n, ok := byName["debug"]
		if !ok {
			t.Fatal("expected symbol debug")
		}
		if n.Kind != core.KindTOMLKey {
			t.Errorf("Kind: got %q, want KindTOMLKey", n.Kind)
		}
	})

	t.Run("table database", func(t *testing.T) {
		n, ok := byName["database"]
		if !ok {
			t.Fatal("expected symbol database")
		}
		if n.Kind != core.KindTOMLTable {
			t.Errorf("Kind: got %q, want KindTOMLTable", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("table server", func(t *testing.T) {
		n, ok := byName["server"]
		if !ok {
			t.Fatal("expected symbol server")
		}
		if n.Kind != core.KindTOMLTable {
			t.Errorf("Kind: got %q, want KindTOMLTable", n.Kind)
		}
	})
}

func TestTOMLExtractor_Determinism(t *testing.T) {
	x := extractor.NewTOMLExtractor()
	n1, err := x.Extract("svc", "config.toml", []byte(tomlFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "config.toml", []byte(tomlFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Kind != n2[i].Kind {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
		}
	}
}

func TestTOMLExtractor_EmptyInput(t *testing.T) {
	x := extractor.NewTOMLExtractor()
	nodes, err := x.Extract("svc", "empty.toml", []byte(""))
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(nodes))
	}
}

func TestTOMLExtractor_OnlyTopLevelKeys(t *testing.T) {
	// Nested table keys should NOT appear as top-level nodes.
	x := extractor.NewTOMLExtractor()
	nodes, err := x.Extract("svc", "config.toml", []byte(tomlFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// Nested keys from [database] and [server] tables should not appear at top level.
	for _, unexpected := range []string{"host", "port", "timeout"} {
		if _, found := byName[unexpected]; found {
			t.Errorf("unexpected top-level node for nested key %q", unexpected)
		}
	}
}
