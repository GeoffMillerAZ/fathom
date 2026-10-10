package extractor

// telemetryidentity_test.go — TL-5 (docs/plans/telemetry-links.md, ADR-062,
// docs/design/telemetry-links.md section 4): the TelemetryIdentity contract
// the docker-compose, serverless and cloudformation dialects emit to, as
// data. A doc entity of source kind TelemetryIdentity, canonical key, named
// by the literal value and signed `<scheme>=<value>`; an intrinsic never
// yields one.

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
			// The interpolated value is emitted as written (the doc engine has
			// no value filter: PROVENANCE.md's ceiling); localize never
			// matches it (resolveTelemetryIdentity).
			[]string{"newrelic.app_name=Checkout (prod)", "service.name=${SERVICE_NAME:-templated}", "service.name=checkout-api"}},
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
