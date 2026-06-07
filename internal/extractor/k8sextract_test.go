package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

const k8sFixture = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-server
  namespace: production
spec:
  replicas: 3
---
apiVersion: v1
kind: Service
metadata:
  name: api-svc
spec:
  port: 8080
---
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate-db
`

func TestK8sExtractor_Basic(t *testing.T) {
	x := extractor.NewK8sExtractor()
	nodes, err := x.Extract("svc", "deploy.yaml", []byte(k8sFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("Deployment with namespace", func(t *testing.T) {
		n, ok := byName["Deployment/production/api-server"]
		if !ok {
			t.Fatal("expected symbol Deployment/production/api-server")
		}
		if n.Kind != core.KindK8sWorkload {
			t.Errorf("Kind: got %q, want KindK8sWorkload", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("Service without namespace", func(t *testing.T) {
		n, ok := byName["Service/api-svc"]
		if !ok {
			t.Fatal("expected symbol Service/api-svc")
		}
		if n.Kind != core.KindK8sWorkload {
			t.Errorf("Kind: got %q, want KindK8sWorkload", n.Kind)
		}
	})

	t.Run("Job", func(t *testing.T) {
		_, ok := byName["Job/migrate-db"]
		if !ok {
			t.Fatal("expected symbol Job/migrate-db")
		}
	})

	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d: %v", len(nodes), nodes)
	}
}

func TestK8sExtractor_SingleDoc(t *testing.T) {
	const single = `apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
`
	x := extractor.NewK8sExtractor()
	nodes, err := x.Extract("svc", "config.yaml", []byte(single))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "ConfigMap/app-config" {
		t.Errorf("Name: got %q, want ConfigMap/app-config", nodes[0].Name)
	}
}

func TestK8sExtractor_Determinism(t *testing.T) {
	x := extractor.NewK8sExtractor()
	n1, err := x.Extract("svc", "deploy.yaml", []byte(k8sFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "deploy.yaml", []byte(k8sFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name {
			t.Errorf("non-deterministic at [%d]", i)
		}
	}
}
