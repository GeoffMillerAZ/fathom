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

// A credential is withheld by where it sits, not by what it looks like: every
// shape below read through the pattern list that preceded this one. A script's
// text is not returned (its program is), and a string literal bound to a name or
// a key in a signature is masked whatever the name is.
func TestSecretsAreWithheldByPlace(t *testing.T) {
	const s = "SENTINEL9f3a"
	r := newReader(t)
	script := func(cmd string) string {
		b, _ := json.Marshal(map[string]any{"name": "x", "version": "1.0.0", "scripts": map[string]string{"run": cmd}})
		return string(b)
	}
	for _, tc := range []struct{ name, path, src string }{
		{"curl --user", "package.json", script("curl --user a:" + s + " https://example.com")},
		{"curl glued -u", "package.json", script("curl -ua:" + s + " https://example.com")},
		{"mysql glued -p", "package.json", script("mysql -p" + s)},
		{"environment assignment", "package.json", script("PASSWORD=" + s + " node x")},
		{"api key header", "package.json", script("curl -H 'X-Api-Key: " + s + "' https://example.com")},
		{"python dbpassword", "a.py", "def f(dbpassword=\"" + s + "\"):\n    pass\n"},
		{"python adminpass", "a.py", "def f(adminpass='" + s + "'):\n    pass\n"},
		{"python accesstoken", "a.py", "def f(accesstoken=\"" + s + "\"):\n    pass\n"},
		{"python clientsecret", "a.py", "def f(clientsecret=\"" + s + "\"):\n    pass\n"},
		{"python password2", "a.py", "def f(password2=\"" + s + "\"):\n    pass\n"},
		{"python bytes", "a.py", "def f(password=b\"" + s + "\"):\n    pass\n"},
		{"python f-string", "a.py", "def f(token=f\"" + s + "\"):\n    pass\n"},
		{"python triple quoted", "a.py", "def f(password=\"\"\"" + s + "\"\"\"):\n    pass\n"},
		{"python dictionary", "a.py", "def f(cfg={\"password\": \"" + s + "\"}):\n    pass\n"},
		{"python connection string", "a.py", "def f(dsn=\"host=x password=" + s + "\"):\n    pass\n"},
		{"typescript backtick", "a.ts", "export function f(password = `" + s + "`): void {}\n"},
		{"rego adminpass", "p.rego", "package p\n\nadminpass := \"" + s + "\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := extract(t, r, tc.path, tc.src)
			if got.Skipped != "" || len(got.Symbols) == 0 {
				t.Fatalf("skipped %q, symbols %d: the shape is not read", got.Skipped, len(got.Symbols))
			}
			if b, _ := json.Marshal(got); strings.Contains(string(b), s) {
				t.Errorf("the credential is in the Result: %s", b)
			}
		})
	}
	// What is withheld is the script's arguments, not the task: the name stays and
	// the program is returned.
	got := extract(t, r, "package.json", script("curl --user a:"+s+" https://example.com"))
	var seen bool
	for _, sym := range got.Symbols {
		if sym.Name == "run" {
			seen = true
			if sym.Signature != "curl" {
				t.Errorf("the script's signature is %q, want its program", sym.Signature)
			}
		}
	}
	if !seen {
		t.Errorf("the script is not read: %+v", got.Symbols)
	}
}

// The userinfo ends where the authority does. An @ in the query or the fragment
// belongs to the URL's tail, the host is the text before it, and a scrub that ran
// across `?` or `#` moved the host to one the file's author chose.
func TestUserinfoScrubDoesNotMoveTheHost(t *testing.T) {
	r := newReader(t)
	for _, tc := range []struct{ name, path, src, url string }{
		{"terraform query", "infra/main.tf", "module \"m\" {\n  source = \"https://evil.example?x=@trusted.example/mod.zip\"\n}\n", "https://evil.example?x=@trusted.example/mod.zip"},
		{"terraform fragment", "infra/main.tf", "module \"m\" {\n  source = \"https://evil.example#@trusted.example/mod.zip\"\n}\n", "https://evil.example#@trusted.example/mod.zip"},
		{"helm repository", "chart/Chart.yaml", "apiVersion: v2\nname: c\nversion: 1.0.0\ndependencies:\n- name: d\n  version: 1.0.0\n  repository: https://evil.example?x=@trusted.example\n", "https://evil.example?x=@trusted.example"},
		{"kustomize resource", "k/kustomization.yaml", "resources:\n- https://evil.example?x=@trusted.example/base\n", "https://evil.example?x=@trusted.example/base"},
		{"actions docker image", ".github/workflows/ci.yml", "name: ci\non: push\njobs:\n  b:\n    runs-on: ubuntu-latest\n    steps:\n    - uses: docker://alpine@sha256:abc123\n", "docker://alpine@sha256:abc123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := extract(t, r, tc.path, tc.src)
			b, _ := json.Marshal(got)
			if !strings.Contains(string(b), tc.url) {
				t.Errorf("%s is not in the Result as written (symbols %d, references %d, incomplete=%v): %s", tc.url, len(got.Symbols), len(got.References), got.Incomplete, b)
			}
		})
	}
}
