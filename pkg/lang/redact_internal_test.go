package lang

import (
	"testing"
)

func TestWithholdURLUserinfo(t *testing.T) {
	const s = "SENTINEL9f3a"
	for _, tc := range []struct {
		name, in, want string
	}{
		{"url password", "git::https://u:" + s + "@example.com/x/y.git", "git::https://example.com/x/y.git"},
		{"url token only", "https://" + s + "@example.com/x", "https://example.com/x"},
		{"url password with an at sign", "https://u:p@" + s + "@example.com/x", "https://example.com/x"},
		{"path join collapsed the slashes", "live/https:/u:" + s + "@x/y", "live/https:/x/y"},
		{"pip vcs", "a @ git+https://u:" + s + "@x/y", "a @ git+https://x/y"},
		{"ssh git account stays", "ssh://git@github.com/o/r.git", "ssh://git@github.com/o/r.git"},
		{"ssh token goes", "ssh://" + s + "@host/o/r.git", "ssh://host/o/r.git"},
		{"ssh account with a password goes", "ssh://git:" + s + "@host/o/r.git", "ssh://host/o/r.git"},
		{"scp form stays", "git@github.com:o/r.git", "git@github.com:o/r.git"},
		{"email stays", "mailto:dev@example.com", "mailto:dev@example.com"},
		{"drive letter path stays", "C:/me@corp/m", "C:/me@corp/m"},
		// The userinfo ends where the authority does, so an @ in the query, the
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
		{"oci token goes", "oci://" + s + "@reg/x", "oci://reg/x"},
		// A userinfo written without a scheme is still a userinfo.
		{"scheme-less user and password", "FROM u:" + s + "@reg/i:1", "FROM reg/i:1"},
		{"scheme-less in a key", "image: u:" + s + "@reg/i", "image: reg/i"},
		{"scheme-less in quotes", `"u:` + s + `@reg/i"`, `"reg/i"`},
		{"slashes only", "//u:" + s + "@x/y", "//x/y"},
		{"slashes only after a space", "a //u:" + s + "@x", "a //x"},
		{"image pinned by digest stays", "alpine:3.19@sha256:abc123", "alpine:3.19@sha256:abc123"},
		{"registry image pinned by digest stays", "reg.example/i:1@sha256:abc123", "reg.example/i:1@sha256:abc123"},
		{"a bare email stays", "dev@example.com", "dev@example.com"},
		{"a sip address stays", "sip:alice@example.com", "sip:alice@example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := withholdURLUserinfo(tc.in); got != tc.want {
				t.Errorf("withholdURLUserinfo(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// Every quoted literal of every quote style reads ***, whatever its name or its
// place, and a quote that is never closed withholds the rest. Numbers,
// identifiers and unquoted text stay.
func TestMaskLiterals(t *testing.T) {
	const s = "SENTINEL9f3a"
	for _, tc := range []struct {
		name, in, want string
	}{
		{"default", `def f(password="` + s + `"):`, `def f(password="***"):`},
		{"typed default", `def f(a: int, api_key: str = "` + s + `"):`, `def f(a: int, api_key: str = "***"):`},
		{"every default", `def f(mode="fast", name: str = "x"):`, `def f(mode="***", name: str = "***"):`},
		{"single quoted", `function f(authToken: string = '` + s + `') {`, `function f(authToken: string = '***') {`},
		{"backtick", "const f = (password = `" + s + "`) =>", "const f = (password = `***`) =>"},
		{"bytes prefix", `def f(password=b"` + s + `"):`, `def f(password=b"***"):`},
		{"triple quoted", `def f(password="""` + s + `"""):`, `def f(password="""***"""):`},
		{"escaped quote", `f("a\"` + s + `")`, `f("***")`},
		{"dictionary", `def f(cfg={"password": "` + s + `"}):`, `def f(cfg={"***": "***"}):`},
		{"a name and a place no list holds", `const TOKEN = "` + s + `"; public $secret = '` + s + `';`, `const TOKEN = "***"; public $secret = '***';`},
		{"an argument", `f("` + s + `", other)`, `f("***", other)`},
		{"a named argument", `local_rule(name = "a", src = "` + s + `")`, `local_rule(name = "***", src = "***")`},
		{"a comparison", `allow { input.x == "` + s + `" }`, `allow { input.x == "***" }`},
		{"a function rule", `f(x) := "` + s + `"`, `f(x) := "***"`},
		{"a set rule", `l contains "` + s + `"`, `l contains "***"`},
		{"an enum case", `case Password = '` + s + `'`, `case Password = '***'`},
		{"never closed", `password := "` + s, `password := "***`},
		{"never closed single", `x = '` + s, `x = '***`},
		{"identifiers and numbers stay", `func F(a int, b string) (n int, err error)`, `func F(a int, b string) (n int, err error)`},
		{"empty literal", `f("")`, `f("***")`},
		{"two literals", `f("a") + g('b')`, `f("***") + g('***')`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskLiterals(tc.in, nil); got != tc.want {
				t.Errorf("maskLiterals(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
	// A Rust lifetime is no literal, and a character is.
	for in, want := range map[string]string{
		`fn f<'a>(x: &'a str) -> &'a str`: `fn f<'a>(x: &'a str) -> &'a str`,
		`fn f<'a>(c: char = 'x')`:         `fn f<'a>(c: char = '***')`,
		`impl<'static> T`:                 `impl<'static> T`,
	} {
		if got := maskLiterals(in, rustLifetime); got != want {
			t.Errorf("rust maskLiterals(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

func TestWithholdFactValue(t *testing.T) {
	const s = "SENTINEL9f3a"
	for _, tc := range []struct{ name, in, want string }{
		{"bound literal", `password = "` + s + `"`, `password = "***"`},
		{"bound literal in a dictionary", `{ clientSecret: "` + s + `" }`, `{ clientSecret: "***" }`},
		{"assignment", `adminpass := "` + s + `"`, `adminpass := "***"`},
		{"a comparison is no binding", `x == "y"`, `x == "y"`},
		{"an argument is no binding", `f("a", "b")`, `f("a", "b")`},
		{"url userinfo", "git::https://u:" + s + "@example.com/x", "git::https://example.com/x"},
		{"curl user", "curl -u a:" + s + " https://example.com", "curl -u ***" + " https://example.com"},
		{"push -u stays", "git push -u origin main", "git push -u origin main"},
		{"password flag", "tool --password " + s + " --other x", "tool --password *** --other x"},
		{"token flag equals", "tool --token=" + s, "tool --token=***"},
		{"flag from the environment stays", "tool --token $TOKEN --user $USER", "tool --token $TOKEN --user $USER"},
		{"authorization header", `curl -H "Authorization: Bearer ` + s + `" x`, `curl -H "Authorization: Bearer ***" x`},
		{"a plain value is returned as written", s, s},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := withholdFactValue(tc.in); got != tc.want {
				t.Errorf("withholdFactValue(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCredentialKey(t *testing.T) {
	for key, want := range map[string]bool{
		"password": true, "labels.password": true, "env.DB_PASSWORD": true, "clientSecret": true, "api-key": true,
		"api_key": true, "apiKey": true, "x.ApiKey": true, "Token": true, "access_token": true, "passwd": true,
		"private_key": true, "privateKey": true, "credentials": true, "spec/credential": true,
		"name": false, "labels.app": false, "containers.web.image": false, "host": false, "password.hint": false,
	} {
		if got := credentialKey(key); got != want {
			t.Errorf("credentialKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestScrubNote(t *testing.T) {
	const s = "SENTINEL9f3a"
	for _, tc := range []struct{ name, in, want string }{
		{"quoted token", `proto parse: <input>:11:14: found "` + s + `" but expected [field sequence number]`, `proto parse: <input>:11:14: found "***" but expected [field sequence number]`},
		{"single quoted", `yaml: line 4: did not find expected '` + s + `' or ']'`, `yaml: line 4: did not find expected '***' or ']'`},
		{"quoted punctuation is grammar", `yaml: line 4: did not find expected ',' or '}'`, `yaml: line 4: did not find expected ',' or '}'`},
		{"a quoted equals sign", `proto parse: <input>:11:14: found "=" but expected [field sequence number]`, `proto parse: <input>:11:14: found "=" but expected [field sequence number]`},
		{"quoted setting", `libconfig: setting "` + s + `" has no = or :`, `libconfig: setting "***" has no = or :`},
		{"a word after the last colon", "unknown directive: " + s, "unknown directive:"},
		{"a word before a semicolon", "go.mod parse: go.mod:3: unknown directive: " + s + "; rejected lines were skipped", "go.mod parse: go.mod:3: unknown directive:; rejected lines were skipped"},
		{"a sentence after the last colon stays", "yaml: line 4: did not find expected key", "yaml: line 4: did not find expected key"},
		{"EOF stays", "xml decode: EOF", "xml decode: EOF"},
		{"an apostrophe in a word is no quote", "parser couldn't read line 3: unexpected end", "parser couldn't read line 3: unexpected end"},
		{"never closed", `rego parse: password := "` + s, `rego parse: password := "***`},
		{"cue literal token", "cue parse: expected label or ':', found 'IDENT' " + s, "cue parse: expected label or ':', found '***'"},
		{"cue literal token with a string", `cue parse: expected operand, found 'STRING' "` + s + ` x"`, "cue parse: expected operand, found '***'"},
		{"cue literal token keeps the error count", "cue parse: expected label or ':', found 'ATTRIBUTE' @a(" + s + ") (and 5 more errors)", "cue parse: expected label or ':', found '***' (and 5 more errors)"},
		{"cue punctuation token stays", "cue parse: unexpected ']' (and 5 more errors)", "cue parse: unexpected ']' (and 5 more errors)"},
		{"a line number stays", "xml decode: XML syntax error on line 7: unexpected EOF", "xml decode: XML syntax error on line 7: unexpected EOF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := scrubNote(tc.in); got != tc.want {
				t.Errorf("scrubNote(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// A literal that interpolates holds quoted literals of its own, and the quote
// styles that are no quote (percent literals, sigils, heredocs) are literals
// too: the family of the literals rule, one case per way a delimiter hides text.
func TestMaskLiteralsNestedAndNonQuoteStyles(t *testing.T) {
	const s = "SENTINEL9f3a"
	for _, tc := range []struct {
		name, in, want string
	}{
		{"python 3.12 f-string with a nested quote", `def f(x=f"{d["` + s + `"]}"):`, `def f(x=f"***"):`},
		{"csharp interpolation", `void F(string x = $"{d["` + s + `"]}")`, `void F(string x = $"***")`},
		{"ruby interpolation", `def f(x = "a#{"` + s + `"}b")`, `def f(x = "***")`},
		{"elixir interpolation", `def f(x \\ "a#{"` + s + `"}")`, `def f(x \\ "***")`},
		{"kotlin template", `fun f(x: String = "a${"` + s + `"}b")`, `fun f(x: String = "***")`},
		{"scala template", `def f(x: String = s"a${"` + s + `"}")`, `def f(x: String = s"***")`},
		{"dart template", `void f({String x = '${"` + s + `"}'})`, `void f({String x = '***'})`},
		{"swift interpolation", `func f(x: String = "a\(g("` + s + `"))b")`, `func f(x: String = "***")`},
		{"js template in a template", "const f = (x = `a${`" + s + "`}b`) =>", "const f = (x = `***`) =>"},
		{"template in a template in a template", "f(`a${`b${`" + s + "`}`}`)", "f(`***`)"},
		{"a brace inside an interpolated string", `f("a${ {"k": "` + s + `"} }b", z)`, `f("***", z)`},
		{"unbalanced interpolation withholds the rest", `f("a${` + s + `", z)`, `f("***`},
		{"elixir sigil with bars", `def f(x \\ ~S|` + s + `|)`, `def f(x \\ ~S|***|)`},
		{"elixir sigil words", `def f(x \\ ~w(` + s + ` ` + s + `)a)`, `def f(x \\ ~w(***)a)`},
		{"elixir sigil modifiers stay", `~r/` + s + `/i`, `~r/***/i`},
		{"ruby %w", `def f(x = %w[` + s + `])`, `def f(x = %w[***])`},
		{"ruby %q", `def f(x = %q(` + s + `))`, `def f(x = %q(***))`},
		{"ruby %i", `def f(x = %i[` + s + `])`, `def f(x = %i[***])`},
		{"ruby %q nests", `def f(x = %q(a(` + s + `)b))`, `def f(x = %q(***))`},
		{"ruby %Q interpolates", `def f(x = %Q(a#{"` + s + `"}))`, `def f(x = %Q(***))`},
		{"ruby heredoc", `def f(x = <<~EOS ` + s + ` EOS)`, `def f(x = <<~EOS***EOS)`},
		{"php heredoc", `function f($x = <<<EOT ` + s + ` EOT)`, `function f($x = <<<EOT***EOT)`},
		{"php nowdoc", `function f($x = <<<'EOT' ` + s + ` EOT)`, `function f($x = <<<'EOT'***EOT)`},
		{"unclosed heredoc withholds the rest", `function f($x = <<<EOT ` + s, `function f($x = <<<EOT***`},
		// What is no literal stays.
		{"percent of a name stays", `n%w[1]`, `n%w[1]`},
		{"shift stays", `x << 3`, `x << 3`},
		{"a destructor is no sigil", `~Foo(int x)`, `~Foo(int x)`},
		{"a bitwise not stays", `~flags & mask`, `~flags & mask`},
		{"an elixir map stays", `def f(%{a: 1})`, `def f(%{a: 1})`},
		{"a plain string with a balanced brace stays inside", `f("{x}")`, `f("***")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := maskLiterals(tc.in, nil)
			if got != tc.want {
				t.Errorf("maskLiterals(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}
