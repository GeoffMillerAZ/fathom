package extractor_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

const chartYAMLFixture = `name: my-service
version: 1.2.3
apiVersion: v2
description: A sample Helm chart
dependencies:
  - name: postgresql
    version: "12.1.0"
    repository: https://charts.bitnami.com/bitnami
  - name: redis
    version: "17.3.2"
    repository: https://charts.bitnami.com/bitnami
`

const valuesYAMLFixture = `replicaCount: 1
image:
  repository: nginx
  tag: latest
service:
  type: ClusterIP
  port: 80
ingress:
  enabled: false
resources: {}
`

// _helpers.tpl with two named templates, one using a sprig func (trunc/default)
// to exercise the helmFuncs map during text/template/parse.Parse.
const helpersTplFixture = `{{/*
Expand the name of the chart.
*/}}
{{- define "chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
`

// ─── HelmChartExtractor tests ─────────────────────────────────────────────────

func TestHelmChartExtractor_Basic(t *testing.T) {
	x := extractor.NewHelmChartExtractor()
	nodes, err := x.Extract("svc", "Chart.yaml", []byte(chartYAMLFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("chart node", func(t *testing.T) {
		n, ok := byName["my-service"]
		if !ok {
			t.Fatalf("expected chart node 'my-service'; got: %v", helmNodeNames(nodes))
		}
		if n.Kind != core.KindHelmChart {
			t.Errorf("Kind: got %q, want KindHelmChart", n.Kind)
		}
		if n.Signature != "1.2.3" {
			t.Errorf("Signature: got %q, want 1.2.3", n.Signature)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("postgresql dependency", func(t *testing.T) {
		n, ok := byName["postgresql"]
		if !ok {
			t.Fatalf("expected dep 'postgresql'; got: %v", helmNodeNames(nodes))
		}
		if n.Kind != core.KindHelmDependency {
			t.Errorf("Kind: got %q, want KindHelmDependency", n.Kind)
		}
		wantSig := "12.1.0 https://charts.bitnami.com/bitnami"
		if n.Signature != wantSig {
			t.Errorf("Signature: got %q, want %q", n.Signature, wantSig)
		}
	})

	t.Run("redis dependency", func(t *testing.T) {
		n, ok := byName["redis"]
		if !ok {
			t.Fatalf("expected dep 'redis'; got: %v", helmNodeNames(nodes))
		}
		if n.Kind != core.KindHelmDependency {
			t.Errorf("Kind: got %q, want KindHelmDependency", n.Kind)
		}
	})

	t.Run("symbol count", func(t *testing.T) {
		// 1 chart + 2 deps
		if len(nodes) != 3 {
			t.Errorf("node count: got %d, want 3; nodes: %v", len(nodes), helmNodeNames(nodes))
		}
	})
}

func TestHelmChartExtractor_Empty(t *testing.T) {
	x := extractor.NewHelmChartExtractor()
	nodes, err := x.Extract("svc", "Chart.yaml", []byte(""))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(nodes))
	}
}

func TestHelmChartExtractor_Determinism(t *testing.T) {
	x := extractor.NewHelmChartExtractor()
	a, _ := x.Extract("svc", "Chart.yaml", []byte(chartYAMLFixture))
	b, _ := x.Extract("svc", "Chart.yaml", []byte(chartYAMLFixture))
	if !reflect.DeepEqual(a, b) {
		t.Error("Extract is not deterministic")
	}
}

// ─── HelmValuesExtractor tests ────────────────────────────────────────────────

func TestHelmValuesExtractor_Basic(t *testing.T) {
	x := extractor.NewHelmValuesExtractor()
	nodes, err := x.Extract("svc", "values.yaml", []byte(valuesYAMLFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	wantKeys := []string{"replicaCount", "image", "service", "ingress", "resources"}
	for _, k := range wantKeys {
		n, ok := byName[k]
		if !ok {
			t.Errorf("expected value key %q; got nodes: %v", k, helmNodeNames(nodes))
			continue
		}
		if n.Kind != core.KindHelmValue {
			t.Errorf("key %q Kind: got %q, want KindHelmValue", k, n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("key %q Tier: got %q, want TierVerified", k, n.Tier)
		}
		if n.Line == 0 {
			t.Errorf("key %q Line: got 0, want >0", k)
		}
	}

	t.Run("top-level only", func(t *testing.T) {
		// "repository", "tag", "port" etc. are nested and must not appear.
		for _, nested := range []string{"repository", "tag", "port", "type", "enabled"} {
			if _, ok := byName[nested]; ok {
				t.Errorf("nested key %q should not be emitted at top level", nested)
			}
		}
	})

	t.Run("count", func(t *testing.T) {
		if len(nodes) != len(wantKeys) {
			t.Errorf("node count: got %d, want %d; nodes: %v", len(nodes), len(wantKeys), helmNodeNames(nodes))
		}
	})
}

func TestHelmValuesExtractor_Determinism(t *testing.T) {
	x := extractor.NewHelmValuesExtractor()
	a, _ := x.Extract("svc", "values.yaml", []byte(valuesYAMLFixture))
	b, _ := x.Extract("svc", "values.yaml", []byte(valuesYAMLFixture))
	if !reflect.DeepEqual(a, b) {
		t.Error("Extract is not deterministic")
	}
}

// ─── HelmTemplateExtractor tests ─────────────────────────────────────────────

func TestHelmTemplateExtractor_Basic(t *testing.T) {
	x := extractor.NewHelmTemplateExtractor()
	nodes, err := x.Extract("svc", "templates/_helpers.tpl", []byte(helpersTplFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	wantNames := []string{"chart.name", "chart.fullname"}
	for _, name := range wantNames {
		n, ok := byName[name]
		if !ok {
			t.Errorf("expected template %q; got nodes: %v", name, helmNodeNames(nodes))
			continue
		}
		if n.Kind != core.KindHelmTemplate {
			t.Errorf("template %q Kind: got %q, want KindHelmTemplate", name, n.Kind)
		}
	}

	t.Run("count", func(t *testing.T) {
		if len(nodes) != 2 {
			t.Errorf("node count: got %d, want 2; nodes: %v", len(nodes), helmNodeNames(nodes))
		}
	})
}

func TestHelmTemplateExtractor_TierVerified(t *testing.T) {
	// The fixture uses sprig funcs (trunc, trimSuffix, default, printf) that are
	// covered by helmFuncs, so the real parse path should succeed → TierVerified.
	x := extractor.NewHelmTemplateExtractor()
	nodes, err := x.Extract("svc", "templates/_helpers.tpl", []byte(helpersTplFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("template %q Tier: got %q, want TierVerified (real parse path)", n.Name, n.Tier)
		}
	}
}

func TestHelmTemplateExtractor_FallbackRegex(t *testing.T) {
	// Deliberately broken template (unclosed action) forces the parse to fail
	// and the regex fallback to run, yielding TierPreScan.
	broken := `{{- define "broken.template" -}}
{{ .Values.foo | UNKNOWN_FUNC_NOT_IN_MAP }}
{{- end }}
{{- define "broken.other" -}}
{{- end }}
`
	x := extractor.NewHelmTemplateExtractor()
	nodes, err := x.Extract("svc", "templates/broken.tpl", []byte(broken))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Should still extract names via regex.
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	for _, name := range []string{"broken.template", "broken.other"} {
		n, ok := byName[name]
		if !ok {
			t.Errorf("expected fallback template %q; got: %v", name, helmNodeNames(nodes))
			continue
		}
		if n.Tier != core.TierPreScan {
			t.Errorf("template %q Tier: got %q, want TierPreScan (regex fallback)", name, n.Tier)
		}
	}
}

func TestHelmTemplateExtractor_Determinism(t *testing.T) {
	x := extractor.NewHelmTemplateExtractor()
	a, _ := x.Extract("svc", "templates/_helpers.tpl", []byte(helpersTplFixture))
	b, _ := x.Extract("svc", "templates/_helpers.tpl", []byte(helpersTplFixture))
	if !reflect.DeepEqual(a, b) {
		t.Error("Extract is not deterministic")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func helmNodeNames(nodes []core.SymbolNode) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = string(n.Kind) + ":" + n.Name
	}
	return names
}
