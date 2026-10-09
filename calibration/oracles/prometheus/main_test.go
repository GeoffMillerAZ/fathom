package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestKeysOf: the oracle's own reading of a route target: an address's host
// by the DNS rules, a label's value in its namespace, the reasons it names
// nothing.
func TestKeysOf(t *testing.T) {
	for _, c := range []struct {
		t            target
		keys         []string
		miss, reason string
	}{
		{target{raw: "checkout:8080"}, []string{"host/checkout", "k8s/default/checkout"}, "undeclared", ""},
		{target{raw: "http://cart.shop:80/metrics"}, []string{"k8s/shop/cart", "host/cart.shop"}, "external", ""},
		{target{raw: "cart.shop.svc.cluster.local:80"}, []string{"k8s/shop/cart"}, "undeclared", ""},
		{target{raw: "metrics.example.com:443"}, []string{"host/metrics.example.com"}, "external", ""},
		{target{raw: "localhost:9090"}, nil, "", "ip-literal"},
		{target{raw: "10.0.0.7:9100"}, nil, "", "ip-literal"},
		{target{raw: "${HOST}:9100"}, nil, "", "variable"},
		{target{raw: "cart", value: "cart", ns: "shop", label: true}, []string{"k8s/shop/cart"}, "undeclared", ""},
		{target{raw: "cart", value: "cart", label: true}, []string{"host/cart", "k8s/default/cart"}, "undeclared", ""},
	} {
		keys, miss, reason := keysOf(c.t)
		if !reflect.DeepEqual(keys, c.keys) || miss != c.miss || reason != c.reason {
			t.Errorf("keysOf(%+v) = %v %q %q, want %v %q %q", c.t, keys, miss, reason, c.keys, c.miss, c.reason)
		}
	}
}

// TestGlobRel: a rule_files pattern is relative to the referring file's
// directory, a glob allowed; an absolute path is outside the subject.
func TestGlobRel(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"mon/rules/a.yml", "mon/rules/b.yml", "mon/c.yml"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("groups: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := globRel(root, "mon", "rules/*.yml"); !reflect.DeepEqual(got, []string{"mon/rules/a.yml", "mon/rules/b.yml"}) {
		t.Errorf("glob = %v", got)
	}
	if got := globRel(root, "mon", "c.yml"); !reflect.DeepEqual(got, []string{"mon/c.yml"}) {
		t.Errorf("file = %v", got)
	}
	if got := globRel(root, "mon", "/etc/prometheus/c.yml"); got != nil {
		t.Errorf("absolute = %v", got)
	}
}
