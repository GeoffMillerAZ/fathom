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

// A credential is withheld by the rule, not by a list of names or places: every
// shape below read through the lists that preceded it. A script's text is not
// returned, a Signature from a dialect that is no code is withheld unless its kind
// is structural, and every quoted literal in a code Signature reads ***.
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
		{"rego function rule", "p.rego", "package p\n\nf(x) := \"" + s + "\"\ng(x) = \"" + s + "\"\nl contains \"" + s + "\"\n"},
		{"php constants and fields", "a.php", "<?php\nconst TOKEN = \"" + s + "\";\nconst PASSWORD = \"" + s + "\";\nclass A { public $secret = \"" + s + "\"; }\nenum E: string { case Password = '" + s + "'; }\n"},
		{"dart constants", "a.dart", "const token = \"" + s + "\";\nfinal password = \"" + s + "\";\nclass A { static const apiKey = \"" + s + "\"; }\n"},
		{"thrift constant", "a.thrift", "const string PASSWORD = \"" + s + "\"\n"},
		{"bicep parameter default", "main.bicep", "param token string = '" + s + "'\n"},
		{"actions run step", ".github/workflows/ci.yml", "name: ci\non: push\njobs:\n  b:\n    runs-on: ubuntu-latest\n    steps:\n    - name: a\n      run: curl --user a:" + s + " x\n    - run: mysql -p" + s + "\n    - run: PASSWORD=" + s + " node x\n    - run: export TOKEN=" + s + "\n    - run: \"curl -H 'X-Api-Key: " + s + "' x\"\n"},
		{"actions input default", "action.yml", "name: a\ninputs:\n  token:\n    description: t\n    default: " + s + "\nruns:\n  using: node20\n  main: index.js\n"},
		{"lefthook command", "lefthook.yml", "pre-commit:\n  commands:\n    a:\n      run: curl --user a:" + s + " x\n    b:\n      run: mysql -p" + s + "\n"},
		{"pyproject script", "pyproject.toml", "[project]\nname = \"x\"\nversion = \"1\"\n\n[project.scripts]\nrun = \"curl --user a:" + s + "\"\n"},
		{"package.json bin", "package.json", `{"name":"x","version":"1.0.0","bin":{"run":"curl --user a:` + s + `"}}`},
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
	// What is withheld is the script, not the task: the name stays and the text of
	// the script is not returned.
	got := extract(t, r, "package.json", script("curl --user a:"+s+" https://example.com"))
	var seen bool
	for _, sym := range got.Symbols {
		if sym.Name == "run" {
			seen = true
			if sym.Signature != "" {
				t.Errorf("the script's signature is %q, want it withheld", sym.Signature)
			}
		}
	}
	if !seen {
		t.Errorf("the script is not read: %+v", got.Symbols)
	}
}

// The userinfo ends where the authority does. An @ in the query or the fragment
// belongs to the URL's tail, the host is the text before it, and a scrub that ran
// across `?` or `#` moved the host to one the file's author chose. Where a
// Signature is withheld (a Terraform module, a Helm dependency) the URL is not in
// the Result at all, and nothing is moved.
func TestUserinfoScrubDoesNotMoveTheHost(t *testing.T) {
	r := newReader(t)
	for _, tc := range []struct{ name, path, src, url string }{
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
	for _, tc := range []struct{ name, path, src string }{
		{"terraform query", "infra/main.tf", "module \"m\" {\n  source = \"https://evil.example?x=@trusted.example/mod.zip\"\n}\n"},
		{"terraform fragment", "infra/main.tf", "module \"m\" {\n  source = \"https://evil.example#@trusted.example/mod.zip\"\n}\n"},
		{"helm repository", "chart/Chart.yaml", "apiVersion: v2\nname: c\nversion: 1.0.0\ndependencies:\n- name: d\n  version: 1.0.0\n  repository: https://evil.example?x=@trusted.example\n"},
	} {
		t.Run(tc.name+" is withheld", func(t *testing.T) {
			got := extract(t, r, tc.path, tc.src)
			b, _ := json.Marshal(got)
			if strings.Contains(string(b), "https://trusted.example") || strings.Contains(string(b), "//trusted.example") {
				t.Errorf("the host was moved: %s", b)
			}
		})
	}
}
