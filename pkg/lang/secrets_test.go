package lang_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// Design section 1: secret-bearing values are not in a Result. Each shape below
// came back from the door with the credential in it: a URL's userinfo in a name,
// a signature, a reference target or its raw spelling, a credential flag in a
// script, a default value on a credential-named parameter.
func TestSecretsAreWithheld(t *testing.T) {
	const s = "SENTINEL9f3a"
	r := newReader(t)
	for _, tc := range []struct {
		name, path, src string
		siblings        []string
	}{
		{"terraform module source", "infra/main.tf", "module \"m\" {\n  source = \"git::https://u:" + s + "@example.com/x/y.git\"\n}\n", nil},
		{"helm dependency repository", "chart/Chart.yaml", "apiVersion: v2\nname: c\nversion: 1.0.0\ndependencies:\n- name: d\n  version: 1.0.0\n  repository: https://u:" + s + "@charts.example.com\n", nil},
		{"package.json script", "package.json", `{"name":"x","version":"1.0.0","scripts":{"fetch":"curl -u a:` + s + ` https://example.com"}}`, nil},
		{"python default", "a.py", "def f(password=\"" + s + "\"):\n    pass\n", nil},
		{"typescript default", "a.ts", "export function f(password: string = \"" + s + "\"): void {}\n", nil},
		{"notebook default", "n/a.ipynb", `{"cells":[{"cell_type":"code","source":["def f(password=\"` + s + `\"):\n","    pass\n"]}],"nbformat":4,"nbformat_minor":5,"metadata":{"kernelspec":{"language":"python","name":"python3"}}}`, nil},
		{"rego rule value", "p.rego", "package p\n\npw := \"" + s + "\"\n", nil},
		{"kustomize resource url", "k/kustomization.yaml", "resources:\n- https://u:" + s + "@example.com/x/base\n", nil},
		{"terragrunt include path", "live/terragrunt.hcl", "include \"root\" {\n  path = \"https://u:" + s + "@example.com/x\"\n}\n", nil},
		{"pyproject dependency", "pyproject.toml", "[project]\nname = \"x\"\nversion = \"1\"\ndependencies = [\"a @ git+https://u:" + s + "@x/y\"]\n", nil},
		{"kong service url", "kong.yml", "_format_version: \"3.0\"\nservices:\n- name: s\n  url: https://u:" + s + "@example.com/api\n", nil},
		{"nginx upstream", "etc/nginx.conf", "http {\n  upstream u {\n    server s1;\n  }\n  server {\n    location / {\n      proxy_pass https://u:" + s + "@example.com/;\n    }\n  }\n}\n", nil},
		{"compose image", "docker-compose.yml", "services:\n  a:\n    image: https://u:" + s + "@registry.example.com/x:1\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := extract(t, r, tc.path, tc.src, tc.siblings...)
			if got.Skipped != "" {
				t.Fatalf("skipped %q: the shape is not read", got.Skipped)
			}
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), s) {
				t.Errorf("the credential is in the Result: %s", b)
			}
			if len(got.Symbols) == 0 && len(got.References) == 0 && len(got.Facts) == 0 {
				t.Errorf("nothing was read, so the test says nothing: %+v", got)
			}
		})
	}
}
