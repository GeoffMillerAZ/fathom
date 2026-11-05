package lang

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langroute"
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
		{"go parser identifier", "extractor error: a.go:3:11: expected ';', found " + s, "extractor error: a.go:3:11: expected ';', found ***"},
		{"go parser identifier among more errors", "extractor error: a.go:3:11: expected ';', found " + s + " (and 2 more errors)", "extractor error: a.go:3:11: expected ';', found *** (and 2 more errors)"},
		{"go parser newline stays", "extractor error: a.go:3:11: expected ';', found newline", "extractor error: a.go:3:11: expected ';', found newline"},
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

// Every kind on structuralKinds has its Signature cut at its first quote as a
// code Signature is, whatever the kind: a class header holds calls with
// literals, a keyword argument and a template (MF10). The cut kinds are cut
// before the quote.
func TestStructuralSignaturesMaskTheirLiterals(t *testing.T) {
	const s = "SENTINEL9f3a"
	var res Result
	for kind := range structuralKinds {
		res.Symbols = append(res.Symbols, Symbol{Name: "N" + kind, Package: "p", Kind: kind, Signature: "header(base, mixin(\"" + s + "\"), tok='" + s + "', t=`" + s + "`)"})
	}
	res.withhold("tooling")
	if len(res.Symbols) != len(structuralKinds) {
		t.Fatalf("%d symbols, want %d", len(res.Symbols), len(structuralKinds))
	}
	for _, sym := range res.Symbols {
		if strings.Contains(sym.Signature, s) {
			t.Errorf("%s returns a literal: %q", sym.Kind, sym.Signature)
		}
	}
}

// The cut rule (ADR-092): a Signature is kept up to its first quote character and
// reads *** from there on, with no pairing and no closer search. A quote that
// opens no literal cuts too. In Rust a lifetime or a label stays.
func TestCutAtQuote(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
		rust           bool
	}{
		{"no quote", `func F(a int, b string) (n int, err error)`, `func F(a int, b string) (n int, err error)`, false},
		{"double", `def f(password="x"):`, `def f(password=***`, false},
		{"single", `function f(a = 'x') {`, `function f(a = ***`, false},
		{"backtick", "const f = (a = `x`) =>", "const f = (a = ***", false},
		{"prefix stays", `def f(a=rb"x"):`, `def f(a=rb***`, false},
		{"sigil prefix stays", `string p = $@"x"`, `string p = $@***`, false},
		{"hash prefix stays", `let p = #"x"#`, `let p = #***`, false},
		{"the first quote wins", `f("a") + g('b')`, `f(***`, false},
		{"a comment apostrophe cuts", `void g(int a /* it's */, const char* s = "x")`, `void g(int a /* it***`, false},
		{"a digit separator cuts", `void f(int a = 1'000, std::string s = "x")`, `void f(int a = 1***`, false},
		{"a regex quote cuts", `function f(a = /'/, s = 'x')`, `function f(a = /***`, false},
		{"a lifetime stays", `fn f<'a>(x: &'a str, s: &str) -> &'a str`, `fn f<'a>(x: &'a str, s: &str) -> &'a str`, true},
		{"a static lifetime stays", `impl<'static> T`, `impl<'static> T`, true},
		{"a label stays", `'outer: loop`, `'outer: loop`, true},
		{"a lifetime then a string", `fn f<'a>(x: &'a str, s: &str = "x")`, `fn f<'a>(x: &'a str, s: &str = ***`, true},
		{"a char is a quote", `fn f<'a>(c: char = 'x')`, `fn f<'a>(c: char = ***`, true},
		{"a char of a lifetime shape is a quote", `const C: char = 'a';`, `const C: char = ***`, true},
		{"an escaped char cuts", `const C: char = '\n';`, `const C: char = ***`, true},
		{"a lifetime at the end reads as a quote", `fn f<'a`, `fn f<***`, true},
		{"a rust lifetime is a quote elsewhere", `fn f<'a>(x)`, `fn f<***`, false},
		{"a rust raw string cuts at its quote", `#[doc = r#"x"#]`, `#[doc = r#***`, true},
		{"empty", ``, ``, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cutAtQuote(tc.in, tc.rust); got != tc.want {
				t.Errorf("cutAtQuote(%q, %v)\n got %q\nwant %q", tc.in, tc.rust, got, tc.want)
			}
		})
	}
}

// withholdedCode is the code dialects that withhold their Signature, each with
// the reason it is not on quoteLed: a literal form that does not open with a quote
// character, or no verification yet. A code dialect is on quoteLed or here, so a
// dialect added to the built-ins fails closed until somebody classifies it.
var withholdedCode = map[string]string{
	"ada":         "not verified against the Ada reference yet",
	"astro":       "a template language over JavaScript: attribute values may be unquoted, not verified",
	"bash":        "heredocs, $'...' and unquoted words hold values with no quote",
	"commonlisp":  "not verified: #\\a characters and |symbols| hold text with no quote",
	"css":         "url(x) and unquoted values hold text with no quote",
	"cue":         "not verified against the CUE specification yet",
	"elixir":      "sigils (~S|x|) and ?\" character literals hold text with no quote",
	"emacs-lisp":  "?a characters hold text with no quote",
	"glsl":        "not verified against the GLSL specification yet",
	"hlsl":        "not verified against the HLSL reference yet",
	"html":        "unquoted attribute values and element text hold values with no quote",
	"jinja2":      "template text outside a tag holds values with no quote",
	"lua":         "long brackets ([[x]]) hold a string with no quote",
	"luau":        "long brackets ([[x]]) hold a string with no quote",
	"matlab":      "the transpose quote and command syntax make a quote ambiguous, not verified",
	"objective-c": "not verified against the Objective-C reference yet",
	"perl":        "q//, qq//, heredocs and regexes hold text with no quote",
	"php":         "heredocs (<<<EOT) hold a string with no quote",
	"powershell":  "here-strings and unquoted arguments hold text with no quote",
	"r":           "raw strings (r\"(x)\") and backtick names, not verified",
	"rego":        "not verified against the Rego reference yet",
	"ruby":        "% literals, ?c characters and heredocs hold a string with no quote",
	"scss":        "unquoted values hold text with no quote",
	"solidity":    "hex\"x\" and unicode\"x\" literals, not verified against the Solidity reference yet",
	"sql":         "dollar quoting ($$x$$) holds a string with no quote",
	"svelte":      "a template language over JavaScript: attribute values may be unquoted, not verified",
	"tcl":         "braces ({x}) hold a string with no quote",
	"templ":       "a template language: element text outside a tag holds values with no quote",
	"vhdl":        "not verified against the VHDL reference yet",
	"vimscript":   "not verified against the Vim script reference yet",
	"vue":         "a template language over JavaScript: attribute values may be unquoted, not verified",
	"wgsl":        "not verified against the WGSL specification yet",
}

// Every code dialect the built-ins list is on quoteLed or named in withholdedCode,
// exactly one, with a one-line reason, and the two lists name no dialect that is
// not a built-in code dialect (ADR-092): a dialect added to the built-ins fails
// this test until it is classified.
func TestEveryCodeDialectIsClassified(t *testing.T) {
	ds, err := extractor.LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	code := map[string]bool{"go": true, langroute.Templ: true}
	for _, d := range ds {
		if d.Kind == "" || d.Kind == "language" {
			code[d.ID] = true
		}
	}
	for id := range code {
		_, led := quoteLed[id]
		_, withheld := withholdedCode[id]
		switch {
		case led && withheld:
			t.Errorf("%s is on quoteLed and on withholdedCode", id)
		case !led && !withheld:
			t.Errorf("%s is a code dialect that is neither on quoteLed nor named withheld", id)
		}
	}
	for name, list := range map[string]map[string]string{"quoteLed": quoteLed, "withholdedCode": withholdedCode} {
		for id, why := range list {
			switch {
			case !code[id]:
				t.Errorf("%s names %s, which is no built-in code dialect", name, id)
			case strings.TrimSpace(why) == "" || strings.Contains(why, "\n") || len(why) > 160:
				t.Errorf("%s[%s]: the reason must be one line of at most 160 characters: %q", name, id, why)
			}
		}
	}
}

// quoteLedLiterals is every literal form of every dialect on quoteLed, as the
// language reference spells it, each holding the sentinel. They are placed in a
// Signature by hand, so this covers the dialects the Reader does not read.
var quoteLedLiterals = map[string][]string{
	"c":          {`"S"`, `L"S"`, `u8"S"`, `u"S"`, `U"S"`, `'S'`, `L'S'`, `u'S'`},
	"cpp":        {`"S"`, `R"(S)"`, `R"x(S)x"`, `u8R"(S)"`, `LR"(S)"`, `'S'`, `u8'S'`},
	"cuda":       {`"S"`, `R"(S)"`, `R"x(S)x"`, `'S'`},
	"csharp":     {`"S"`, `@"S"`, `$"S"`, `$@"S"`, `@$"S"`, `"""S"""`, `""""S""""`, `$$"""S"""`, `'S'`, `u8"S"u8`},
	"dart":       {`'S'`, `"S"`, `r'S'`, `r"S"`, `'''S'''`, `"""S"""`, `'a$S'`},
	"go":         {`"S"`, "`S`", `'S'`},
	"java":       {`"S"`, `'S'`, "\"\"\"\nS\n\"\"\""},
	"javascript": {`"S"`, `'S'`, "`S`", "`a${S}`", "tag`S`"},
	"kotlin":     {`"S"`, `"""S"""`, `'S'`, `"a$S"`},
	"python":     {`"S"`, `'S'`, `b"S"`, `r"S"`, `u"S"`, `f"S"`, `rb"S"`, `Rb"S"`, `br'S'`, `t"S"`, `"""S"""`, `'''S'''`},
	"rust":       {`"S"`, `r"S"`, `r#"S"#`, `r##"S"##`, `b"S"`, `br#"S"#`, `c"S"`, `cr#"S"#`, `'S'`, `b'S'`},
	"scala":      {`"S"`, `"""S"""`, `s"S"`, `f"S"`, `raw"S"`, `'S'`, `'S`},
	"swift":      {`"S"`, `"""S"""`, `#"S"#`, `##"S"##`, `#"""S"""#`},
	"typescript": {`"S"`, `'S'`, "`S`", "`a${S}`", "tag`S`"},
}

// Every literal form of every dialect on quoteLed, wherever it sits in a
// Signature, is cut at its quote: nothing of it comes back, only its prefix
// characters.
func TestQuoteLedDialectsCutTheirFormsDirectly(t *testing.T) {
	for id := range quoteLed {
		forms, ok := quoteLedLiterals[id]
		if !ok {
			t.Errorf("%s is on quoteLed and quoteLedLiterals holds no literal form of it", id)
			continue
		}
		for i, form := range forms {
			lit := strings.ReplaceAll(form, "S", "QZ"+string(rune('0'+i%10)))
			for _, sig := range []string{"f(a = " + lit + ", b)", "@A(" + lit + ") f()", "f(a /* it's */, b = " + lit + ")", lit} {
				res := Result{Dialect: id, Symbols: []Symbol{{Name: "f", Package: "p", Kind: "Function", Signature: sig}}}
				res.withhold("language")
				if got := res.Symbols[0].Signature; strings.Contains(got, "QZ") || !strings.HasSuffix(got, redacted) {
					t.Errorf("%s %q: read %q", id, sig, got)
				}
			}
		}
	}
}

// The userinfo of a URL written before the first quote of a Signature is dropped
// as it is anywhere, and the cut then reads *** from the quote on.
func TestQuoteLedSignatureDropsURLUserinfo(t *testing.T) {
	res := Result{Dialect: "python", Symbols: []Symbol{{Name: "f", Package: "p", Kind: "Function", Signature: `def f(u=https://user:SENTINEL9f3a@h.example/x, s="y")`}}}
	res.withhold("language")
	if got, want := res.Symbols[0].Signature, `def f(u=https://h.example/x, s=***`; got != want {
		t.Errorf("Signature %q, want %q", got, want)
	}
}
