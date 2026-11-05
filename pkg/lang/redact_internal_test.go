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
		{"python default", `def f(password="` + s + `"):`, `def f(password="***"):`},
		{"typed python default", `def f(a: int, api_key: str = "` + s + `"):`, `def f(a: int, api_key: str = "***"):`},
		{"typescript default", `function f(authToken: string = '` + s + `') {`, `function f(authToken: string = '***') {`},
		{"rego rule value", `pw := "` + s + `"`, `pw := "***"`},
		{"object field", `{ clientSecret: "` + s + `" }`, `{ clientSecret: "***" }`},
		{"plain default stays", `def f(mode="fast", name: str = "x"):`, `def f(mode="fast", name: str = "x"):`},
		{"key word alone stays", `def f(key="k", monkey="m"):`, `def f(key="k", monkey="m"):`},
		{"curl user", "curl -u a:" + s + " https://example.com", "curl -u ***" + " https://example.com"},
		{"push -u stays", "git push -u origin main", "git push -u origin main"},
		{"password flag", "tool --password " + s + " --other x", "tool --password *** --other x"},
		{"token flag equals", "tool --token=" + s, "tool --token=***"},
		{"flag from the environment stays", "tool --token $TOKEN --user $USER", "tool --token $TOKEN --user $USER"},
		{"authorization header", `curl -H "Authorization: Bearer ` + s + `" x`, `curl -H "Authorization: Bearer ***" x`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := withholdSecrets(tc.in); got != tc.want {
				t.Errorf("withholdSecrets(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
	if !secretName("clientSecret") || !secretName("DB_PASSWORD") || !secretName("x-api-key") || secretName("monkey") || secretName("keyboard") {
		t.Error("secretName splits words wrong")
	}
}
