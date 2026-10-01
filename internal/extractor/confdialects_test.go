package extractor

// confdialects_test.go — goldens and lexer contracts for the native proxy
// dialects (LB-Q-nginx, LB-Q-haproxy, LB-Q-caddyfile; design §13.2 steps
// 10/12). Each testdata fixture's golden was written by hand from reading
// the fixture: its entities (name, kind, lines, signature, calls), its root
// entities, and the cross-file references the extractor records (includes
// and imports as repo-path candidates, route targets per the route
// contract), whether or not anything resolves them.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

type confGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
}

type confGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type confGolden struct {
	Path      string             `json:"path"`
	Roots     []string           `json:"roots"`
	Symbols   []confGoldenSymbol `json:"symbols"`
	CrossFile []confGoldenCross  `json:"cross_file"`
}

// TestProxyDialectGoldens: every fixture of the three dialects against its
// hand-written golden, extracted as the golden's repository path.
func TestProxyDialectGoldens(t *testing.T) {
	for _, id := range []string{"nginx", "haproxy", "caddyfile"} {
		x, ok := ProxyDialectExtractor(id)
		if !ok {
			t.Fatalf("ProxyDialectExtractor(%s): not registered", id)
		}
		doc := x.(DocResulter)
		dir := filepath.Join("dialects", id, "testdata")
		ents, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range ents {
			if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
				continue
			}
			n++
			fx := filepath.Join(dir, e.Name())
			t.Run(id+"/"+e.Name(), func(t *testing.T) {
				src, err := os.ReadFile(fx)
				if err != nil {
					t.Fatal(err)
				}
				if lines := strings.Count(string(src), "\n"); lines > 200 {
					t.Fatalf("fixture has %d lines, the recipe's cap is 200", lines)
				}
				raw, err := os.ReadFile(fx + ".golden.json")
				if err != nil {
					t.Fatalf("read golden: %v", err)
				}
				var want confGolden
				if err := json.Unmarshal(raw, &want); err != nil {
					t.Fatalf("golden: %v", err)
				}
				res, err := doc.ExtractDoc("svc", want.Path, src)
				if err != nil {
					t.Fatalf("ExtractDoc: %v", err)
				}
				got := confGolden{Path: want.Path, Roots: append([]string{}, res.Roots...), Symbols: []confGoldenSymbol{}, CrossFile: []confGoldenCross{}}
				for _, nd := range res.Nodes {
					if nd.Service != "svc" || nd.FilePath != want.Path || nd.Package != pkgFromPath(want.Path) || nd.Tier != core.TierPreScan {
						t.Fatalf("plumbing changed: %+v", nd)
					}
					got.Symbols = append(got.Symbols, confGoldenSymbol{Name: nd.Name, Kind: string(nd.Kind), Line: nd.Line, EndLine: nd.EndLine, Signature: nd.Signature, Calls: nd.Calls})
				}
				for _, c := range res.CrossFile {
					got.CrossFile = append(got.CrossFile, confGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line})
				}
				if want.Roots == nil {
					want.Roots = []string{}
				}
				if !reflect.DeepEqual(got, want) {
					gb, _ := json.MarshalIndent(got, "", "  ")
					t.Fatalf("mismatch; got:\n%s", gb)
				}
				// Extract is ExtractDoc's entities.
				nodes, err := x.Extract("svc", want.Path, src)
				if err != nil || len(nodes) != len(res.Nodes) {
					t.Fatalf("Extract: %d nodes, %v; ExtractDoc %d", len(nodes), err, len(res.Nodes))
				}
			})
		}
		if n < 3 {
			t.Errorf("%s: %d fixtures, want at least 3", id, n)
		}
	}
}

// TestProxyDialectsAreNative: NativeDialectExtractor answers for the three
// dialects (the harness measures them through it), each a DocResulter.
func TestProxyDialectsAreNative(t *testing.T) {
	for _, id := range []string{"nginx", "haproxy", "caddyfile"} {
		e, ok := NativeDialectExtractor(id)
		if !ok {
			t.Fatalf("NativeDialectExtractor(%s): not registered", id)
		}
		if _, ok := e.(DocResulter); !ok {
			t.Fatalf("%s: not a DocResulter", id)
		}
	}
	if _, ok := ProxyDialectExtractor("go-mod"); ok {
		t.Fatal("ProxyDialectExtractor answers for go-mod")
	}
}

func confTexts(toks []confTok) []string {
	var out []string
	for _, t := range toks {
		switch t.kind {
		case confOpen, confClose, confSemi:
			out = append(out, t.text)
		case confEOL:
			out = append(out, "⏎")
		default:
			if t.quoted {
				out = append(out, "«"+t.text+"»")
			} else {
				out = append(out, t.text)
			}
		}
	}
	return out
}

// TestConfLexNginx pins crossplane's token rules: braces and semicolons are
// tokens of their own outside quotes; a quote opens a string only at the
// start of a word; # starts a comment only at the start of a word; ${name}
// keeps its braces; a *_by_lua_block body is one opaque word whose strings
// and comments may hold braces; escapes stay as written outside quotes.
func TestConfLexNginx(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"quoted braces", `return 200 "{ok}";`, []string{"return", "200", "«{ok}»", ";"}},
		{"quote inside a word", `add_header X a"b;`, []string{"add_header", "X", `a"b`, ";"}},
		{"escaped quote in a string", `return 200 "a\"b";`, []string{"return", "200", `«a"b»`, ";"}},
		{"regex location", `location ~* \.(png|jpg)$ { }`, []string{"location", "~*", `\.(png|jpg)$`, "{", "}"}},
		{"quoted regex with braces", `location ~ "^/a{2}$" {}`, []string{"location", "~", "«^/a{2}$»", "{", "}"}},
		{"hash inside a word", `location ~* (?:#.*#|~)$ {}`, []string{"location", "~*", "(?:#.*#|~)$", "{", "}"}},
		{"comment", "listen 80; # a comment {\nroot /x;", []string{"listen", "80", ";", "root", "/x", ";"}},
		{"parameter expansion", `set $a ${b}c;`, []string{"set", "$a", "${b}c", ";"}},
		{"escape outside quotes", `rewrite ^/a\ b /c;`, []string{"rewrite", `^/a\ b`, "/c", ";"}},
		{"if block", `if ($x = "y") { return 403; }`, []string{"if", "($x", "=", "«y»", ")", "{", "return", "403", ";", "}"}},
		{"lua block", "content_by_lua_block {\n  local t = { a = \"}\" } -- }\n  ngx.say('{')\n}\nlisten 1;", []string{"content_by_lua_block", "«\n  local t = { a = \"}\" } -- }\n  ngx.say('{')\n»", ";", "listen", "1", ";"}},
		{"lua block with args", "set_by_lua_block $v { return [[}]] }", []string{"set_by_lua_block", "$v", "« return [[}]] »", ";"}},
		{"not a lua directive in argument position", "set $x content_by_lua_block { }", []string{"set", "$x", "content_by_lua_block", "{", "}"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			toks, _ := confLex([]byte(tc.src), nginxSyntax)
			if got := confTexts(toks); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestConfLexHAProxy pins HAProxy's line rules: a newline ends a statement,
// # starts a comment anywhere outside quotes unless escaped, quotes open and
// close inside a word, single quotes are literal, a backslash escapes the
// next character, and a string never spans lines.
func TestConfLexHAProxy(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"lines", "backend be\n  server s1 a:80 check\n", []string{"backend", "be", "⏎", "server", "s1", "a:80", "check", "⏎"}},
		{"comment mid-line", "acl x path /a#b\n", []string{"acl", "x", "path", "/a", "⏎"}},
		{"escaped hash", `acl x path_reg ^/a\#b$`, []string{"acl", "x", "path_reg", "^/a#b$", "⏎"}},
		{"quotes inside a word", `http-request set-header X "a b"c`, []string{"http-request", "set-header", "X", "«a bc»", "⏎"}},
		{"single quotes literal", `log-format '%[a]\n'`, []string{"log-format", `«%[a]\n»`, "⏎"}},
		{"unterminated string ends at the line", "acl a hdr(x) \"open\nbackend b\n", []string{"acl", "a", "hdr(x)", "«open»", "⏎", "backend", "b", "⏎"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			toks, _ := confLex([]byte(tc.src), haproxySyntax)
			if got := confTexts(toks); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestConfLexCaddyfile pins Caddy's rules: braces are ordinary characters
// inside a word ({host}, {$ENV}), a backtick string is raw, a heredoc is
// one word and the closing marker's line may continue the statement, a
// backslash before the newline continues the line, # starts a comment only
// at the start of a word.
func TestConfLexCaddyfile(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"placeholders", "redir https://{host}{uri}\nreverse_proxy {$UP}", []string{"redir", "https://{host}{uri}", "⏎", "reverse_proxy", "{$UP}", "⏎"}},
		{"block braces", "a.com {\n\trespond hi\n}\n", []string{"a.com", "{", "⏎", "respond", "hi", "⏎", "}", "⏎"}},
		{"quoted brace", `respond "{"`, []string{"respond", "«{»", "⏎"}},
		{"backticks", "header `X-A` `a \"b\" \\n`", []string{"header", "«X-A»", "«a \"b\" \\n»", "⏎"}},
		{"heredoc", "respond <<EOF\n\t<p>{x}</p>\n\tEOF 200\nfile_server", []string{"respond", "«<p>{x}</p>»", "200", "⏎", "file_server", "⏎"}},
		{"continuation", "reverse_proxy a:1 \\\n\tb:2\n", []string{"reverse_proxy", "a:1", "b:2", "⏎"}},
		{"comment", "respond a#b # c\n", []string{"respond", "a#b", "⏎"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			toks, _ := confLex([]byte(tc.src), caddyfileSyntax)
			if got := confTexts(toks); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestParseNginxTreeRepairs: a stray closing brace is skipped, a statement
// a brace cuts short is kept, an unclosed block ends at the end of the
// file; each repair is counted.
func TestParseNginxTreeRepairs(t *testing.T) {
	tree, fixes := parseNginxTree([]byte("}\nserver {\n  location / {\n    proxy_pass http://a\n  }\n"))
	if fixes != 3 {
		t.Errorf("fixes = %d, want 3 (stray }, cut statement, unclosed server)", fixes)
	}
	if len(tree) != 1 || tree[0].name != "server" || tree[0].end != 5 {
		t.Fatalf("tree: %+v", tree)
	}
	loc := tree[0].children[0]
	if loc.name != "location" || loc.end != 5 || len(loc.children) != 1 || loc.children[0].name != "proxy_pass" {
		t.Fatalf("location: %+v", loc)
	}
}

// TestNginxPassHost: the host a *_pass argument names, and whether a port
// follows it (an upstream is matched only without one).
func TestNginxPassHost(t *testing.T) {
	for arg, want := range map[string]struct {
		host string
		port bool
	}{
		"http://backend":           {"backend", false},
		"http://backend/x/":        {"backend", false},
		"https://backend:8443/x":   {"backend", true},
		"backend":                  {"backend", false},
		"php:9000":                 {"php", true},
		"grpc://[::1]:50051":       {"[::1]", true},
		"http://unix:/tmp/a.sock:": {"", false},
		"http://$up":               {"$up", false},
	} {
		h, p := nginxPassHost(arg)
		if h != want.host || p != want.port {
			t.Errorf("nginxPassHost(%q) = %q, %v; want %q, %v", arg, h, p, want.host, want.port)
		}
	}
}

// TestProxyDialectDeterministic: two extractions of one fixture are
// identical, and cross-file references come sorted.
func TestProxyDialectDeterministic(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("dialects", "nginx", "testdata", "fragment.conf"))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := NewNginxExtractor().ExtractDoc("svc", "a/b/app.conf", src)
	b, _ := NewNginxExtractor().ExtractDoc("svc", "a/b/app.conf", src)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two extractions differ")
	}
	if !sort.SliceIsSorted(a.CrossFile, func(i, j int) bool { return a.CrossFile[i].Line < a.CrossFile[j].Line }) {
		t.Fatal("cross-file references not sorted by line")
	}
}
