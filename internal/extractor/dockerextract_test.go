package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

// dockerFixture is a multi-stage Dockerfile with:
//   - a builder stage using golang (FROM ... AS build)
//   - a final stage based on the builder stage (FROM build)
//   - an intermediate base stage with a platform flag
//   - a backslash line continuation
const dockerFixture = `# syntax=docker/dockerfile:1

FROM --platform=linux/amd64 golang:1.26-alpine AS build

RUN apk add --no-cache \
    git \
    ca-certificates

FROM build AS test

FROM alpine:3.19
`

// dockerMinimalFixture is a simple single-stage Dockerfile.
const dockerMinimalFixture = `FROM ubuntu:22.04
RUN apt-get update
`

func TestDockerfileExtractor_MultiStage(t *testing.T) {
	x := extractor.NewDockerfileExtractor()
	nodes, err := x.Extract("svc", "Dockerfile", []byte(dockerFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Expect 3 stages: build, test, alpine:3.19 (final unnamed stage).
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %v", len(nodes), dockerNodeNames(nodes))
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("build stage", func(t *testing.T) {
		n, ok := byName["build"]
		if !ok {
			t.Fatalf("expected stage 'build'; got: %v", dockerNodeNames(nodes))
		}
		if n.Kind != core.KindDockerStage {
			t.Errorf("Kind: got %q, want KindDockerStage", n.Kind)
		}
		if n.Tier != core.TierPreScan {
			t.Errorf("Tier: got %q, want TierPreScan", n.Tier)
		}
		if n.Signature != "FROM golang:1.26-alpine AS build" {
			t.Errorf("Signature: got %q, want %q", n.Signature, "FROM golang:1.26-alpine AS build")
		}
		// No Calls edge — golang:1.26-alpine is an external image.
		if len(n.Calls) != 0 {
			t.Errorf("Calls: got %v, want empty", n.Calls)
		}
		if n.Line == 0 {
			t.Errorf("Line should be non-zero")
		}
	})

	t.Run("test stage (from build)", func(t *testing.T) {
		n, ok := byName["test"]
		if !ok {
			t.Fatalf("expected stage 'test'; got: %v", dockerNodeNames(nodes))
		}
		if n.Kind != core.KindDockerStage {
			t.Errorf("Kind: got %q, want KindDockerStage", n.Kind)
		}
		if n.Tier != core.TierPreScan {
			t.Errorf("Tier: got %q, want TierPreScan", n.Tier)
		}
		if n.Signature != "FROM build AS test" {
			t.Errorf("Signature: got %q, want %q", n.Signature, "FROM build AS test")
		}
		// Calls must contain "build" (stage→base edge).
		if len(n.Calls) != 1 || n.Calls[0] != "build" {
			t.Errorf("Calls: got %v, want [build]", n.Calls)
		}
	})

	t.Run("final unnamed stage (alpine)", func(t *testing.T) {
		n, ok := byName["alpine:3.19"]
		if !ok {
			t.Fatalf("expected stage 'alpine:3.19'; got: %v", dockerNodeNames(nodes))
		}
		if n.Kind != core.KindDockerStage {
			t.Errorf("Kind: got %q, want KindDockerStage", n.Kind)
		}
		if n.Signature != "FROM alpine:3.19" {
			t.Errorf("Signature: got %q, want %q", n.Signature, "FROM alpine:3.19")
		}
		// No Calls edge — alpine:3.19 is an external image, not a prior stage.
		if len(n.Calls) != 0 {
			t.Errorf("Calls: got %v, want empty", n.Calls)
		}
	})
}

func TestDockerfileExtractor_SingleStage(t *testing.T) {
	x := extractor.NewDockerfileExtractor()
	nodes, err := x.Extract("svc", "Dockerfile", []byte(dockerMinimalFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d: %v", len(nodes), dockerNodeNames(nodes))
	}
	n := nodes[0]
	if n.Name != "ubuntu:22.04" {
		t.Errorf("Name: got %q, want ubuntu:22.04", n.Name)
	}
	if n.Kind != core.KindDockerStage {
		t.Errorf("Kind: got %q, want KindDockerStage", n.Kind)
	}
	if n.Tier != core.TierPreScan {
		t.Errorf("Tier: got %q, want TierPreScan", n.Tier)
	}
	if n.Signature != "FROM ubuntu:22.04" {
		t.Errorf("Signature: got %q, want FROM ubuntu:22.04", n.Signature)
	}
	if n.Line != 1 {
		t.Errorf("Line: got %d, want 1", n.Line)
	}
}

func TestDockerfileExtractor_Empty(t *testing.T) {
	x := extractor.NewDockerfileExtractor()
	nodes, err := x.Extract("svc", "Dockerfile", []byte("# comment only\nRUN echo hi\n"))
	if err != nil {
		t.Fatalf("Extract must not error: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for comment-only input, got %d", len(nodes))
	}
}

func TestDockerfileExtractor_Determinism(t *testing.T) {
	x := extractor.NewDockerfileExtractor()
	n1, err := x.Extract("svc", "Dockerfile", []byte(dockerFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "Dockerfile", []byte(dockerFixture))
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

func TestDockerfileExtractor_TierIsPreScan(t *testing.T) {
	x := extractor.NewDockerfileExtractor()
	nodes, _ := x.Extract("svc", "Dockerfile", []byte("FROM alpine:3.19\n"))
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Tier != core.TierPreScan {
		t.Errorf("Tier: got %q, want TierPreScan", nodes[0].Tier)
	}
}

// dockerNodeNames returns Kind:Name for each node for diagnostics.
func dockerNodeNames(nodes []core.SymbolNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = string(n.Kind) + ":" + n.Name
	}
	return out
}
