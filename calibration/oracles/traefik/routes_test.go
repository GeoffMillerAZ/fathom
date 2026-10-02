package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostOf(t *testing.T) {
	for in, want := range map[string]string{
		"auth":                       "auth",
		"auth:9999":                  "auth",
		"http://auth:9999/verify":    "auth",
		"https://u:p@host.example/x": "host.example",
		"http://[::1]:80/":           "::1",
		"10.0.0.5:8000":              "10.0.0.5",
		"":                           "",
	} {
		if got := hostOf(in); got != want {
			t.Errorf("hostOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// dd builds a deployed service's declarations, each file declaring name itself.
func dd(name string, files ...string) []decl {
	var out []decl
	for _, f := range files {
		out = append(out, decl{f, name, "A"})
	}
	return out
}

func joinFiles(ds []decl) string {
	var fs []string
	for _, d := range ds {
		fs = append(fs, d.File+"#"+d.Entity)
	}
	return strings.Join(fs, ",")
}

func TestClassify(t *testing.T) {
	d := deployed{"auth": dd("auth", "docker-compose.yml"), "db": dd("db", "x/a.yml", "y/b.yml"), "web": {{"k8s/svc.yaml", "Service/shop/web", "C"}}}
	for _, c := range []struct{ raw, want, reason string }{
		{"http://auth:9999/v1", "docker-compose.yml#auth", ""},
		{"auth:80", "docker-compose.yml#auth", ""},
		{"web:80", "k8s/svc.yaml#Service/shop/web", ""},
		{"db:5432", "", "ambiguous"},
		{"10.0.0.5:80", "", "ip-literal"},
		{"http://[::1]:80", "", "ip-literal"},
		{"api.example.com", "", "external"},
		{"${UPSTREAM}:80", "", "variable"},
		{"{{ .Host }}", "", "variable"},
		{"unix:/var/run/x.sock", "", "unix-socket"},
		{"cache", "", "undeclared"},
	} {
		ds, reason := d.classify(c.raw, "proxy/envoy.yaml")
		if joinFiles(ds) != c.want || reason != c.reason {
			t.Errorf("classify(%q) = %q, %q; want %q, %q", c.raw, joinFiles(ds), reason, c.want, c.reason)
		}
	}
}

// TestClassifyNearestProject (LB-F169): a compose project is a directory, so a
// route joins the declarations of its nearest project (the route's own
// directory, then each ancestor), one pair per file of that directory that
// declares the service (an override file is the same service, compose merges
// them); a name no ancestor declares joins only when exactly one file of the
// subject does.
func TestClassifyNearestProject(t *testing.T) {
	d := deployed{
		"service": dd("service", "brotli/docker-compose.yaml", "gzip/docker-compose.yaml", "stack/docker-compose.override.yml", "stack/docker-compose.yml"),
		"solo":    dd("solo", "other/compose.yaml"),
		"twin":    dd("twin", "a/compose.yaml", "b/compose.yaml"),
	}
	for _, c := range []struct {
		raw, from, want, reason string
	}{
		{"service:80", "brotli/envoy.yaml", "brotli/docker-compose.yaml#service", ""},
		{"service:80", "gzip/conf/envoy.yaml", "gzip/docker-compose.yaml#service", ""},
		{"service:80", "stack/envoy.yaml", "stack/docker-compose.override.yml#service,stack/docker-compose.yml#service", ""},
		{"service:80", "elsewhere/envoy.yaml", "", "ambiguous"},
		{"solo:80", "elsewhere/envoy.yaml", "other/compose.yaml#solo", ""},
		{"twin", "a/sub/envoy.yaml", "a/compose.yaml#twin", ""},
		{"twin", "c/envoy.yaml", "", "ambiguous"},
		{"cache", "a/envoy.yaml", "", "undeclared"},
	} {
		got, reason := d.classify(c.raw, c.from)
		if reason != c.reason || joinFiles(got) != c.want {
			t.Errorf("classify(%q, %q) = %q, %q; want %q, %q", c.raw, c.from, joinFiles(got), reason, c.want, c.reason)
		}
	}
}

// TestScanDeployedTiers (LB-F169): a compose-named file is read by Docker
// Compose (tier A, an override file's partial service included), a Kubernetes
// Service by yaml.v3 (tier C, named `Service/<namespace>/<name>`).
func TestScanDeployedTiers(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("no docker")
	}
	dir := t.TempDir()
	for name, text := range map[string]string{
		"docker-compose.yml":          "services:\n  web:\n    image: example.com/web:1\n",
		"docker-compose.override.yml": "services:\n  web:\n    environment:\n      A: b\n  worker:\n    environment:\n      A: b\n",
		"k8s/svc.yaml":                "apiVersion: v1\nkind: Service\nmetadata:\n  name: api\n  namespace: shop\n",
	} {
		full := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d := scanDeployed(dir)
	if got := joinFiles(d["web"]); got != "docker-compose.override.yml#web,docker-compose.yml#web" {
		t.Errorf("web declared in %q", got)
	}
	if got := joinFiles(d["worker"]); got != "docker-compose.override.yml#worker" {
		t.Errorf("worker declared in %q (an override file's partial service)", got)
	}
	if got := d["web"][0].Tier; got != "A" {
		t.Errorf("a compose service's tier = %q, want A", got)
	}
	if got := joinFiles(d["api"]); got != "k8s/svc.yaml#Service/shop/api" || d["api"][0].Tier != "C" {
		t.Errorf("api declared in %q tier %q", got, d["api"][0].Tier)
	}
	if routesTier([]routeRef{{Tier: ""}}) != "A" || routesTier([]routeRef{{Tier: ""}, {Tier: "C"}}) != "C" {
		t.Error("routesTier is A unless a pair is tier C")
	}
}
