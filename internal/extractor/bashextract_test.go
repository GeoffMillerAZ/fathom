package extractor

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// bashFixture exercises both function definition styles supported by Bash:
// the "function name() { }" keyword form and the "name() { }" POSIX form.
const bashFixture = `#!/bin/bash

function greet() {
    echo "Hello, $1"
}

deploy() {
    echo "deploying"
}

function_with_args() {
    local x=$1
    echo $x
}
`

func TestBashExtractor_BasicSymbols(t *testing.T) {
	e := NewBashExtractor()
	nodes, err := e.Extract("svc", "scripts/deploy.sh", []byte(bashFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	want := []string{"greet", "deploy", "function_with_args"}
	for _, name := range want {
		n, ok := byName[name]
		if !ok {
			t.Errorf("missing symbol %q", name)
			continue
		}
		if n.Kind != core.KindBashFunction {
			t.Errorf("symbol %q: want KindBashFunction, got %q", name, n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: want TierVerified, got %q", name, n.Tier)
		}
		if n.Service != "svc" {
			t.Errorf("symbol %q: want Service %q, got %q", name, "svc", n.Service)
		}
		if n.FilePath != "scripts/deploy.sh" {
			t.Errorf("symbol %q: want FilePath %q, got %q", name, "scripts/deploy.sh", n.FilePath)
		}
	}
}

func TestBashExtractor_KeywordAndPOSIXForms(t *testing.T) {
	// Verify both "function name() { }" (keyword) and "name() { }" (POSIX) are extracted.
	src := []byte(`function keyword_form() { echo "keyword"; }
posix_form() { echo "posix"; }`)
	e := NewBashExtractor()
	nodes, err := e.Extract("svc", "forms.sh", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	if _, ok := byName["keyword_form"]; !ok {
		t.Error("expected keyword_form from 'function name()' syntax")
	}
	if _, ok := byName["posix_form"]; !ok {
		t.Error("expected posix_form from 'name()' syntax")
	}
}

func TestBashExtractor_TierVerified(t *testing.T) {
	e := NewBashExtractor()
	nodes, err := e.Extract("svc", "deploy.sh", []byte(bashFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: expected TierVerified, got %q", n.Name, n.Tier)
		}
	}
}

func TestBashExtractor_Determinism(t *testing.T) {
	e := NewBashExtractor()
	a, err := e.Extract("svc", "deploy.sh", []byte(bashFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	b, err := e.Extract("svc", "deploy.sh", []byte(bashFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: first run %d symbols, second %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Kind != b[i].Kind || a[i].Line != b[i].Line {
			t.Errorf("index %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestBashExtractor_EmptySource(t *testing.T) {
	e := NewBashExtractor()
	nodes, err := e.Extract("svc", "empty.sh", []byte{})
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty source, got %d", len(nodes))
	}
}
