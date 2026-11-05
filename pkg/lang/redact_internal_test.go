package lang

import (
	"testing"
)

func TestWithholdSecrets(t *testing.T) {
	const s = "SENTINEL9f3a"
	for _, tc := range []struct {
		name, in, want string
	}{
		{"url password", "git::https://u:" + s + "@example.com/x/y.git", "git::https://example.com/x/y.git"},
		{"url token only", "https://" + s + "@example.com/x", "https://example.com/x"},
		{"url password with an at sign", "https://u:p@" + s + "@example.com/x", "https://example.com/x"},
		{"path join collapsed the slashes", "live/https:/u:" + s + "@x/y", "live/https:/x/y"},
		{"pip vcs", "a @ git+https://u:" + s + "@x/y", "a @ git+https://x/y"},
		{"ssh account stays", "ssh://git@github.com/o/r.git", "ssh://git@github.com/o/r.git"},
		{"scp form stays", "git@github.com:o/r.git", "git@github.com:o/r.git"},
		{"email stays", "mailto:dev@example.com", "mailto:dev@example.com"},
		// M7: the userinfo ends where the authority does, so an @ in the query, the
		// fragment or a backslash path is not a userinfo and the host is not moved.
		{"at sign in the query", "https://evil.example?x=@trusted.example/mod.zip", "https://evil.example?x=@trusted.example/mod.zip"},
		{"at sign in the fragment", "https://evil.example#@trusted.example/mod.zip", "https://evil.example#@trusted.example/mod.zip"},
		{"at sign after a backslash", `https://evil.example\@trusted.example/mod.zip`, `https://evil.example\@trusted.example/mod.zip`},
		{"at sign in the path", "https://evil.example/a@trusted.example/mod.zip", "https://evil.example/a@trusted.example/mod.zip"},
		{"query then a real userinfo later", "https://h/p?x=1 https://u:" + s + "@e.example/y", "https://h/p?x=1 https://e.example/y"},
		{"docker image stays", "docker://alpine@sha256:abc123", "docker://alpine@sha256:abc123"},
		{"docker image with a tag and a digest stays", "docker://alpine:3.19@sha256:abc123", "docker://alpine:3.19@sha256:abc123"},
		{"docker registry password goes", "docker://u:" + s + "@registry.example.com/i", "docker://registry.example.com/i"},
		{"oci image stays", "oci://alpine@sha256:abc123", "oci://alpine@sha256:abc123"},
		{"python default", `def f(password="` + s + `"):`, `def f(password="***"):`},
		{"typed python default", `def f(a: int, api_key: str = "` + s + `"):`, `def f(a: int, api_key: str = "***"):`},
		{"typescript default", `function f(authToken: string = '` + s + `') {`, `function f(authToken: string = '***') {`},
		{"rego rule value", `pw := "` + s + `"`, `pw := "***"`},
		{"object field", `{ clientSecret: "` + s + `" }`, `{ clientSecret: "***" }`},
		// M9: a literal bound to a name or a key is masked whatever the name says, so
		// a credential in a name no list holds (dbpassword, adminpass, a connection
		// string) is withheld, and a literal that is no default stays as written.
		{"every default is masked", `def f(mode="fast", name: str = "x"):`, `def f(mode="***", name: str = "***"):`},
		{"name no list holds", `def f(dbpassword="` + s + `", adminpass='` + s + `', accesstoken="` + s + `", clientsecret="` + s + `", password2="` + s + `"):`, `def f(dbpassword="***", adminpass='***', accesstoken="***", clientsecret="***", password2="***"):`},
		{"bytes default", `def f(password=b"` + s + `"):`, `def f(password=b"***"):`},
		{"f-string default", `def f(token=f"` + s + `"):`, `def f(token=f"***"):`},
		{"triple quoted default", `def f(password="""` + s + `"""):`, `def f(password="""***"""):`},
		{"dictionary default", `def f(cfg={"password": "` + s + `"}):`, `def f(cfg={"password": "***"}):`},
		{"connection string", `def f(dsn="host=x password=` + s + `"):`, `def f(dsn="***"):`},
		{"backtick default", "const f = (password = `" + s + "`) =>", "const f = (password = `***`) =>"},
		{"rego assignment", `adminpass := "` + s + `"`, `adminpass := "***"`},
		{"comparison is no binding", `allow { input.x == "y" }`, `allow { input.x == "y" }`},
		{"label is no binding", `resource "aws_s3_bucket" "b"`, `resource "aws_s3_bucket" "b"`},
		{"argument is no binding", `f("a", "b")`, `f("a", "b")`},
		{"curl user", "curl -u a:" + s + " https://example.com", "curl -u ***" + " https://example.com"},
		{"push -u stays", "git push -u origin main", "git push -u origin main"},
		{"password flag", "tool --password " + s + " --other x", "tool --password *** --other x"},
		{"token flag equals", "tool --token=" + s, "tool --token=***"},
		{"flag from the environment stays", "tool --token $TOKEN --user $USER", "tool --token $TOKEN --user $USER"},
		{"authorization header", `curl -H "Authorization: Bearer ` + s + `" x`, `curl -H "Authorization: Bearer ***" x`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := withholdSecrets(tc.in, true); got != tc.want {
				t.Errorf("withholdSecrets(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// Only a literal in parentheses (a parameter's default) is masked in a
// declaration that is not a rule: a block's own attributes are what it is read
// for, and a Terraform module's source must stay readable by a policy.
func TestWithholdSecretsInADeclaration(t *testing.T) {
	const s = "SENTINEL9f3a"
	for _, tc := range []struct{ name, in, want string }{
		{"module source stays", `module "m" { source = "git::https://example.com/x/y.git" }`, `module "m" { source = "git::https://example.com/x/y.git" }`},
		{"module source userinfo goes", `module "m" { source = "git::https://u:` + s + `@example.com/x/y.git" }`, `module "m" { source = "git::https://example.com/x/y.git" }`},
		{"module source with an at sign in the query stays", `module "m" { source = "https://evil.example?x=@trusted.example/m.zip" }`, `module "m" { source = "https://evil.example?x=@trusted.example/m.zip" }`},
		{"default in parentheses", `def f(dbpassword="` + s + `"):`, `def f(dbpassword="***"):`},
		{"second default after a literal", `def f(a="x(", b="` + s + `"):`, `def f(a="***", b="***"):`},
		{"arrow default", "f = (password = `" + s + "`) =>", "f = (password = `***`) =>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := withholdSecrets(tc.in, false); got != tc.want {
				t.Errorf("withholdSecrets(%q, false)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}
