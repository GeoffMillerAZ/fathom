package extractor_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
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
			t.Fatalf("expected symbol Deployment/production/api-server; got nodes: %v", k8sNodeNames(nodes))
		}
		if n.Kind != core.KindK8sWorkload {
			t.Errorf("Kind: got %q, want KindK8sWorkload", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
		if n.Signature != "apps/v1" {
			t.Errorf("Signature: got %q, want apps/v1", n.Signature)
		}
	})

	t.Run("Service without namespace", func(t *testing.T) {
		n, ok := byName["Service/api-svc"]
		if !ok {
			t.Fatalf("expected symbol Service/api-svc; got nodes: %v", k8sNodeNames(nodes))
		}
		if n.Kind != core.KindK8sWorkload {
			t.Errorf("Kind: got %q, want KindK8sWorkload", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("Job", func(t *testing.T) {
		_, ok := byName["Job/migrate-db"]
		if !ok {
			t.Fatalf("expected symbol Job/migrate-db; got nodes: %v", k8sNodeNames(nodes))
		}
	})

	// LB-F451: a Deployment's replicas is a fact of its workload.
	if n, ok := byName["Deployment/production/api-server#replicas"]; !ok || n.Kind != core.KindK8sFact || n.Signature != "3" || n.Line != 7 {
		t.Errorf("replicas fact: got %+v (present %v)", n, ok)
	}

	// 3 workloads + the Deployment's replicas fact.
	if len(nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d: %v", len(nodes), k8sNodeNames(nodes))
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
		t.Fatalf("expected 1 node, got %d: %v", len(nodes), k8sNodeNames(nodes))
	}
	if nodes[0].Name != "ConfigMap/app-config" {
		t.Errorf("Name: got %q, want ConfigMap/app-config", nodes[0].Name)
	}
	if nodes[0].Tier != core.TierVerified {
		t.Errorf("Tier: got %q, want TierVerified", nodes[0].Tier)
	}
}

// TestK8sExtractor_Containers verifies that container image references are
// emitted as KindK8sContainer nodes for both spec.containers and
// spec.template.spec.containers paths.
func TestK8sExtractor_Containers(t *testing.T) {
	const containerFixture = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: staging
spec:
  template:
    spec:
      containers:
        - name: web
          image: registry.example.com/web:v1.2.3
        - name: sidecar
          image: envoyproxy/envoy:v1.28.0
      initContainers:
        - name: migrator
          image: registry.example.com/migrator:latest
`
	x := extractor.NewK8sExtractor()
	nodes, err := x.Extract("svc", "web.yaml", []byte(containerFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("workload node", func(t *testing.T) {
		n, ok := byName["Deployment/staging/web-app"]
		if !ok {
			t.Fatalf("expected Deployment/staging/web-app; got: %v", k8sNodeNames(nodes))
		}
		if n.Kind != core.KindK8sWorkload {
			t.Errorf("Kind: got %q, want KindK8sWorkload", n.Kind)
		}
	})

	t.Run("container image web", func(t *testing.T) {
		n, ok := byName["registry.example.com/web:v1.2.3"]
		if !ok {
			t.Fatalf("expected container registry.example.com/web:v1.2.3; got: %v", k8sNodeNames(nodes))
		}
		if n.Kind != core.KindK8sContainer {
			t.Errorf("Kind: got %q, want KindK8sContainer", n.Kind)
		}
		if n.Signature != "web" {
			t.Errorf("Signature (container name): got %q, want web", n.Signature)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("container image sidecar", func(t *testing.T) {
		n, ok := byName["envoyproxy/envoy:v1.28.0"]
		if !ok {
			t.Fatalf("expected container envoyproxy/envoy:v1.28.0; got: %v", k8sNodeNames(nodes))
		}
		if n.Kind != core.KindK8sContainer {
			t.Errorf("Kind: got %q, want KindK8sContainer", n.Kind)
		}
		if n.Signature != "sidecar" {
			t.Errorf("Signature (container name): got %q, want sidecar", n.Signature)
		}
	})

	t.Run("initContainer image migrator", func(t *testing.T) {
		n, ok := byName["registry.example.com/migrator:latest"]
		if !ok {
			t.Fatalf("expected initContainer registry.example.com/migrator:latest; got: %v", k8sNodeNames(nodes))
		}
		if n.Kind != core.KindK8sContainer {
			t.Errorf("Kind: got %q, want KindK8sContainer", n.Kind)
		}
		if n.Signature != "migrator" {
			t.Errorf("Signature (container name): got %q, want migrator", n.Signature)
		}
	})

	// LB-F451: the container list and the init-container list are facts of
	// their own, never merged.
	for name, want := range map[string]string{"Deployment/staging/web-app#containers": "web,sidecar", "Deployment/staging/web-app#initContainers": "migrator"} {
		if n, ok := byName[name]; !ok || n.Kind != core.KindK8sFact || n.Signature != want {
			t.Errorf("%s: got %+v (present %v), want %q", name, n, ok, want)
		}
	}

	// 1 workload + 3 containers + the two list facts = 6 nodes total.
	if len(nodes) != 6 {
		t.Errorf("expected 6 nodes, got %d: %v", len(nodes), k8sNodeNames(nodes))
	}
}

// TestK8sExtractor_HelmMustache verifies that a document containing Helm
// template mustache expressions is silently skipped (no error, no crash).
func TestK8sExtractor_HelmMustache(t *testing.T) {
	// Mix a valid doc with a Helm-template doc that is not valid YAML.
	const helmFixture = `apiVersion: v1
kind: ConfigMap
metadata:
  name: valid-config
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-web
spec:
  replicas: {{ .Values.replicaCount }}
  template:
    spec:
      containers:
        - name: web
          image: {{ .Values.image.repository }}:{{ .Values.image.tag }}
`
	x := extractor.NewK8sExtractor()
	nodes, err := x.Extract("svc", "chart.yaml", []byte(helmFixture))
	if err != nil {
		t.Fatalf("Extract must not return an error for Helm templates, got: %v", err)
	}

	// The valid ConfigMap doc should be extracted; the Helm doc should be skipped.
	if len(nodes) == 0 {
		t.Fatal("expected at least one node from the valid ConfigMap doc")
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	if _, ok := byName["ConfigMap/valid-config"]; !ok {
		t.Errorf("expected ConfigMap/valid-config; got: %v", k8sNodeNames(nodes))
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
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, n1[i].Name, n2[i].Name)
		}
	}
}

// k8sNodeNames returns the Kind:Name of each node for k8s test diagnostics.
func k8sNodeNames(nodes []core.SymbolNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = string(n.Kind) + ":" + n.Name
	}
	return out
}

// TestK8sExtractor_TelemetryIdentity (LB-F314, TL-5): a container's literal
// OTEL_SERVICE_NAME, the service.name entry of a literal
// OTEL_RESOURCE_ATTRIBUTES and each name of a literal NEW_RELIC_APP_NAME are
// TelemetryIdentity nodes named by the value and signed `<scheme>=<value>`,
// at the env entry's line; a $(VAR), ${VAR} or {{ }} value and a valueFrom
// yield none.
func TestK8sExtractor_TelemetryIdentity(t *testing.T) {
	src := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: checkout
  namespace: shop
spec:
  template:
    spec:
      initContainers:
        - name: migrate
          image: acme/migrate:1
          env:
            - name: OTEL_SERVICE_NAME
              value: checkout-migrate
      containers:
        - name: app
          image: acme/checkout:1
          env:
            - name: OTEL_SERVICE_NAME
              value: checkout
            - name: OTEL_RESOURCE_ATTRIBUTES
              value: deployment.environment=prod, service.name=checkout-api ,service.version=1
            - name: NEW_RELIC_APP_NAME
              value: Checkout;Checkout (rollup)
        - name: sidecar
          image: acme/sidecar:1
          env:
            - name: OTEL_SERVICE_NAME
              value: $(POD_NAME)
            - name: NEW_RELIC_APP_NAME
              valueFrom:
                configMapKeyRef: {name: nr, key: app}
            - name: OTEL_RESOURCE_ATTRIBUTES
              value: service.name=${SERVICE}
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nightly
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: job
              image: acme/nightly:1
              env:
                - {name: OTEL_SERVICE_NAME, value: "{{ .Values.name }}"}
                - {name: OTEL_SERVICE_NAME, value: nightly-report}
`
	nodes, err := extractor.NewK8sExtractor().Extract("svc", "deploy/checkout.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		if n.Kind != extractor.KindTelemetryIdentity {
			continue
		}
		if n.FilePath != "deploy/checkout.yaml" || n.Tier != core.TierVerified || n.ToolKind != extractor.ToolKindK8sManifest {
			t.Errorf("%+v: plumbing", n)
		}
		got = append(got, fmt.Sprintf("%d %s %s", n.Line, n.Name, n.Signature))
	}
	want := []string{
		"13 checkout-migrate service.name=checkout-migrate",
		"19 checkout service.name=checkout",
		"21 checkout-api service.name=checkout-api",
		"23 Checkout newrelic.app_name=Checkout",
		"23 Checkout (rollup) newrelic.app_name=Checkout (rollup)",
		"50 nightly-report service.name=nightly-report",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("identities\n got: %q\nwant: %q", got, want)
	}
}
