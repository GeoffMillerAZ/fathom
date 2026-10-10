package extractor

// telemetryidentity_test.go — TL-5 (docs/plans/telemetry-links.md, ADR-062,
// docs/design/telemetry-links.md section 4): the TelemetryIdentity contract
// the docker-compose, serverless and cloudformation dialects emit to, as
// data. A doc entity of source kind TelemetryIdentity, canonical key, named
// by the literal value and signed `<scheme>=<value>`; an intrinsic, an
// interpolation or a dynamic reference never yields one (K20's
// refuse_templates, TL-9).

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// telemetrySchemes is the contract's closed scheme list.
var telemetrySchemes = []string{"service.name", "newrelic.app_name", "aws.lambda.function_name", "prometheus.job"}

func TestTelemetryIdentityDialectsEmitTheContract(t *testing.T) {
	cases := []struct {
		dialect, fixture, rel string
		want                  []string // "<scheme>=<name>", sorted
	}{
		{"docker-compose", "dialects/docker-compose/testdata/telemetry.yaml", "telemetry.yaml",
			// TL-9 (K20): the list form and OTEL_RESOURCE_ATTRIBUTES'
			// service.name are read, one identity per name in the file, and an
			// interpolated value yields none at extraction (refuse_templates).
			[]string{"newrelic.app_name=Checkout (prod)", "newrelic.app_name=Ledger", "service.name=billing-svc",
				"service.name=cart-svc", "service.name=checkout-api", "service.name=ledger-svc", "service.name=order-worker"}},
		{"serverless", "dialects/serverless/testdata/telemetry/serverless.yml", "infra/telemetry/serverless.yml",
			[]string{"aws.lambda.function_name=billing-invoice-prod"}},
		{"cloudformation", "dialects/cloudformation/testdata/telemetry.yaml", "infra/telemetry.yaml",
			[]string{"aws.lambda.function_name=ledger-api", "aws.lambda.function_name=payments-handler"}},
	}
	for _, c := range cases {
		t.Run(c.dialect, func(t *testing.T) {
			x, err := NewDocDialectExtractor(c.dialect)
			if err != nil {
				t.Fatal(err)
			}
			src, err := os.ReadFile(filepath.FromSlash(c.fixture))
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", c.rel, src)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, n := range res.Nodes {
				if n.Kind != "TelemetryIdentity" {
					continue
				}
				scheme, value, ok := strings.Cut(n.Signature, "=")
				if !ok || value != n.Name || !containsStr(telemetrySchemes, scheme) {
					t.Errorf("%s: signature %q breaks the contract (name %q)", c.dialect, n.Signature, n.Name)
				}
				got = append(got, n.Signature)
			}
			sort.Strings(got)
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("identities = %q, want %q", got, c.want)
			}
		})
	}
}

// TestTelemetryIdentityIsAContractSourceKind: several dialects declare the
// one TelemetryIdentity source kind (contractSourceKinds) and the manifests
// still load; every declaration is canonical key.
func TestTelemetryIdentityIsAContractSourceKind(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	var declaring []string
	for _, d := range ds {
		for _, k := range d.Kinds {
			if k.SourceKind == "TelemetryIdentity" {
				declaring = append(declaring, d.ID)
				if k.Canonical != "key" {
					t.Errorf("%s declares TelemetryIdentity as %q, want key", d.ID, k.Canonical)
				}
			}
		}
	}
	sort.Strings(declaring)
	for _, want := range []string{"cloudformation", "docker-compose", "serverless"} {
		if !containsStr(declaring, want) {
			t.Errorf("dialects declaring TelemetryIdentity = %v, want %s among them", declaring, want)
		}
	}
}
