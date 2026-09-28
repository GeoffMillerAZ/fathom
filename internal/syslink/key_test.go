package syslink

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/inventory"
)

// TestParseContractKeyKindsAndNormalization is SG-1's contract: the eleven
// kinds of docs/design/system-graph.md §3 parse, each body is normalized by
// its kind's normalizer, anything else is refused BY NAME, and ADR-044's
// four inventory kinds round-trip byte-identically.
func TestParseContractKeyKindsAndNormalization(t *testing.T) {
	if got := len(Kinds()); got != 11 {
		t.Fatalf("Kinds() = %d kinds, want the eleven of §3", got)
	}

	cases := []struct {
		in, want string
	}{
		{"route:get /orders/:id/", "route:GET /orders/{}"},
		{"route:POST /orders/{orderID}?x=1", "route:POST /orders/{}"},
		{"route:GET https://api.example/items/%s", "route:GET /items/{}"},
		{"rpc:shop.v1.Orders/Create", "rpc:shop.v1.Orders/Create"},
		{"gql:Order.total", "gql:Order.total"},
		{"gql:Query.orders", "gql:Query.orders"},
		{"topic:orders.created", "topic:orders.created"},
		{"table:Orders.Total_Cents", "table:orders.total_cents"},
		{`table:PUBLIC."Orders"`, `table:public."Orders"`},
		{"env:ORDERS_DB_URL", "env:ORDERS_DB_URL"},
		{"config:flags/checkout.v2", "config:flags/checkout.v2"},
		{"pkg:NPM/lodash@4.17.21#debounce", "pkg:npm/lodash@4.17.21#debounce"},
		{"pkg:npm/@scope/pkg@1.0.0", "pkg:npm/@scope/pkg@1.0.0"},
		{"pkg:golang/github.com/x/y@v1.2.3#Do", "pkg:golang/github.com/x/y@v1.2.3#Do"},
		{"image:Registry.Example/orders:1.2", "image:registry.example/orders"},
		{"image:registry.example/orders@sha256:abc", "image:registry.example/orders"},
		{"deploy:K8S/Shop/Orders", "deploy:k8s/shop/orders"},
		{"deploy:host/Orders.Example", "deploy:host/orders.example"},
		{"cli:fathom verify", "cli:fathom verify"},
		{"cli:fathom   verify", "cli:fathom verify"},
	}
	for _, c := range cases {
		k, err := ParseContractKey(c.in)
		if err != nil {
			t.Errorf("ParseContractKey(%q): %v", c.in, err)
			continue
		}
		if got := k.String(); got != c.want {
			t.Errorf("ParseContractKey(%q) = %q, want %q", c.in, got, c.want)
		}
		// Normalization is idempotent: a normalized key parses to itself.
		k2, err := ParseContractKey(k.String())
		if err != nil || k2 != k {
			t.Errorf("re-parse of %q = %v, %v; want identical", k.String(), k2, err)
		}
	}

	refused := []struct {
		in, name string
	}{
		{"widget:foo", `"widget"`},
		{"route", "no kind separator"},
		{"route:", "route"},
		{"route:/orders", "route"},
		{"route:GET  /a", "route"},
		{"rpc:Create", "rpc"},
		{"gql:Order", "gql"},
		{"topic:orders created", "topic"},
		{"table:a.b.c.d", "table"},
		{"config:flag", "config"},
		{"pkg:bogus/x@1", "pkg"},
		{"deploy:vm/x", "deploy"},
		{"env:ÄÖ", "ASCII"},
	}
	for _, c := range refused {
		_, err := ParseContractKey(c.in)
		if err == nil {
			t.Errorf("ParseContractKey(%q) accepted, want refusal", c.in)
			continue
		}
		if !strings.Contains(err.Error(), c.name) {
			t.Errorf("ParseContractKey(%q) error %q does not name %q", c.in, err, c.name)
		}
	}

	// ADR-044's four kinds, exactly as internal/analysis/inventory spells
	// them, round-trip byte-identically (route in its normalized form:
	// inventory keeps the router's own param spelling, which §3 collapses).
	files := []inventory.File{
		{Path: "api/orders.proto", Content: []byte("syntax = \"proto3\";\npackage shop.v1;\nservice Orders { rpc Create(Req) returns (Resp); }\n")},
		{Path: "main.go", Content: []byte("package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/spf13/cobra\"\n)\n\nvar cmd = &cobra.Command{Use: \"verify\"}\n\nfunc main() {\n\t_ = os.Getenv(\"ORDERS_DB_URL\")\n\thttp.HandleFunc(\"/orders\", nil)\n}\n")},
	}
	seen := map[string]bool{}
	for _, it := range inventory.Extract(files, nil) {
		switch it.Kind {
		case "route", "rpc", "env", "cli":
		default:
			continue
		}
		seen[it.Kind] = true
		k, err := ParseContractKey(it.Key)
		if err != nil {
			t.Errorf("inventory key %q refused: %v", it.Key, err)
			continue
		}
		if k.String() != it.Key {
			t.Errorf("inventory key %q round-tripped to %q", it.Key, k.String())
		}
	}
	for _, kind := range []string{"route", "rpc", "env", "cli"} {
		if !seen[kind] {
			t.Errorf("fixture produced no inventory %s: key", kind)
		}
	}
}
