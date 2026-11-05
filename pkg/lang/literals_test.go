package lang_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

const sentinel = "SENTINEL9f3a"

// urlExtractor puts a URL with a password in every string field of a Result.
type urlExtractor struct{}

func (urlExtractor) Extract(_ context.Context, _ lang.File) (lang.Extracted, error) {
	u := "https://u:" + sentinel + "@h.example/x"
	return lang.Extracted{
		Symbols:    []lang.Symbol{{Name: "N" + u, Package: "p", Kind: "RowDef", Signature: `f("` + u + `") ` + u, Line: 1, Tier: "verified"}},
		References: []lang.Reference{{From: u, To: u, Raw: u, Edge: "uses", Line: 1}},
		Unresolved: []lang.Unresolved{{From: u, Edge: "uses", Raw: u, Reason: "dynamic", Line: 1}},
		Facts:      []lang.Fact{{Entity: u, Key: "k", Value: u, Line: 1}},
	}, nil
}

// The userinfo of a URL is withheld wherever a string from the file lands: a
// symbol's name, key and signature, every reference field, every unresolved
// field and a fact's entity and value. Each field is checked on the path a Go
// extractor takes, for a code dialect and for one that is no code.
func TestURLUserinfoIsWithheldInEveryField(t *testing.T) {
	for _, kind := range []string{"language", "tooling"} {
		t.Run(kind, func(t *testing.T) {
			m := lang.Match{ID: "url-" + kind, Kind: kind, Extensions: []string{".urlx"}, Kinds: map[string]string{"RowDef": "function"}}
			r := newReader(t, lang.WithExtractor(m, urlExtractor{}))
			got := extract(t, r, "a/b.urlx", "x")
			if len(got.Symbols) != 1 || len(got.References) != 1 || len(got.Unresolved) != 1 || len(got.Facts) != 1 {
				t.Fatalf("the extractor's result is not read: %+v", got)
			}
			b, _ := json.Marshal(got)
			if strings.Contains(string(b), sentinel) {
				t.Errorf("the password is in the Result: %s", b)
			}
			// Each field on its own, so a field that is not scrubbed is named.
			s, ref, un, f := got.Symbols[0], got.References[0], got.Unresolved[0], got.Facts[0]
			for name, v := range map[string]string{
				"Symbol.Name": s.Name, "Symbol.Key": s.Key, "Symbol.Signature": s.Signature,
				"Reference.From": ref.From, "Reference.To": ref.To, "Reference.Raw": ref.Raw,
				"Unresolved.From": un.From, "Unresolved.Raw": un.Raw,
				"Fact.Entity": f.Entity, "Fact.Value": f.Value,
			} {
				if strings.Contains(v, sentinel) {
					t.Errorf("%s keeps the password: %q", name, v)
				}
			}
			if kind == "tooling" && s.Signature != "" {
				t.Errorf("a Signature of a kind nobody listed is returned from a dialect that is no code: %q", s.Signature)
			}
			if kind == "language" && s.Signature != `f("***") https://h.example/x` {
				t.Errorf("a code Signature reads %q", s.Signature)
			}
		})
	}
}

// The same rule holds for a pack dialect, which a file reaches through the pack
// layer and not the built-in router.
func TestPackReadWithholdsURLUserinfo(t *testing.T) {
	r := newReader(t, lang.WithDialects(os.DirFS(samplePack)))
	u := "https://u:" + sentinel + "@h.example/x"
	got := extract(t, r, "ui/clock.widget.json", `{"name":"Clock","kind":"timer","uses":["`+u+`"]}`)
	if got.Dialect != "widget-spec" || len(got.References) == 0 {
		t.Fatalf("the pack dialect read nothing: %+v", got)
	}
	if b, _ := json.Marshal(got); strings.Contains(string(b), sentinel) {
		t.Errorf("the password is in the Result: %s", b)
	}
}

// A Signature from a dialect that is no code is withheld unless its kind is on
// the list, and the list is held here: each entry is a kind a fixture produced
// from a dialect that is no code, each reason is one line, and a Signature of a
// kind that is not listed is empty in every fixture.
func TestStructuralKindsHoldNoValue(t *testing.T) {
	r := newReader(t)
	kinds := dialectKinds(r)
	seen := map[string]bool{}
	for _, f := range plainReadings(t, r) {
		if kinds[f.res.Dialect] == "language" {
			continue
		}
		for _, s := range f.res.Symbols {
			seen[s.Kind] = true
			if _, listed := lang.StructuralKinds[s.Kind]; !listed && s.Signature != "" {
				t.Errorf("%s %s: a Signature of the unlisted kind %s is returned: %q", f.res.Dialect, f.path, s.Kind, s.Signature)
			}
		}
	}
	for kind, why := range lang.StructuralKinds {
		switch {
		case strings.TrimSpace(why) == "" || strings.ContainsAny(why, "\n") || len(why) > 140:
			t.Errorf("%s: the reason must be one line of at most 140 characters: %q", kind, why)
		case !seen[kind]:
			t.Errorf("%s is on the list and no fixture of a dialect that is no code produces it", kind)
		}
	}
}

var (
	plainOnce sync.Once
	plainRead []reading
)

// plainReadings is every fixture and own sample read as written, read once.
func plainReadings(t *testing.T, r *lang.Reader) []reading {
	t.Helper()
	plainOnce.Do(func() { plainRead = allReadings(t, r, nil) })
	return plainRead
}

type reading struct {
	path string
	res  lang.Result
}

// allReadings reads every dialect fixture and own sample, each source passed
// through transform when it is not nil.
func allReadings(t *testing.T, r *lang.Reader, transform func(dialectKind, path string, src []byte) []byte) []reading {
	t.Helper()
	kinds := dialectKinds(r)
	var out []reading
	one := func(dir, path string, src []byte) {
		if transform != nil {
			src = transform(kinds[dir], path, src)
		}
		res, err := r.Extract(context.Background(), lang.File{Path: path, Src: src})
		if err != nil {
			t.Fatal(err)
		}
		if res.Skipped == "" {
			out = append(out, reading{path, res})
		}
	}
	for _, f := range dialectFixtures(t) {
		one(f.dir, f.rel, f.src)
	}
	ids := make([]string, 0, len(ownSamples))
	for id := range ownSamples {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		one(id, ownSamples[id].path, []byte(ownSamples[id].src))
	}
	return out
}

func dialectKinds(r *lang.Reader) map[string]string {
	kinds := map[string]string{}
	for _, d := range r.Dialects() {
		kinds[d.ID] = d.Kind
	}
	return kinds
}

var (
	quotedRe = regexp.MustCompile("\"(?:[^\"\\\\\\n]|\\\\.)*\"|'(?:[^'\\\\\\n]|\\\\.)*'|`[^`\\n]*`")
	// keyValueRe is a `key: value` or `key = value` line of YAML, INI, TOML or HCL,
	// with an optional list dash; the value is the third group.
	keyValueRe = regexp.MustCompile(`(?m)^(\s*(?:-\s+)?[A-Za-z0-9_.\-/$@ ]+?\s*[:=]\s*)([^\s#"'\[{|>&*!][^#\n]*?)([ \t]*(?:#.*)?)$`)
	xmlTextRe  = regexp.MustCompile(`>([^<>\s][^<>]*)<`)
)

// quotedToSentinel replaces what is between the quotes of every quoted literal.
func quotedToSentinel(src []byte) []byte {
	return quotedRe.ReplaceAllFunc(src, func(m []byte) []byte {
		return append(append([]byte{m[0]}, sentinel...), m[len(m)-1])
	})
}

// valuesToSentinel replaces the value of every `key: value` line and the text of
// every XML element, after the quoted literals.
func valuesToSentinel(src []byte) []byte {
	src = quotedToSentinel(src)
	src = keyValueRe.ReplaceAll(src, []byte("${1}"+sentinel+"${3}"))
	return xmlTextRe.ReplaceAll(src, []byte(">"+sentinel+"<"))
}

// The family test of the literals rule. Every quoted literal of every fixture,
// and in a dialect that is no code every `key: value` value too, is replaced by
// a sentinel, and the sentinel must come back in no Signature, no Note and no
// value under a credential key. Reference targets are the one place a value is
// meant to be returned (Reference.To and Raw, Unresolved.Raw), so they are not
// checked. Fact values under any other key are returned as written and are only
// counted.
func TestSentinelPositionsAreNotReturned(t *testing.T) {
	r := newReader(t)
	for name, transform := range map[string]func(string, string, []byte) []byte{
		"quoted literals": func(_, _ string, src []byte) []byte { return quotedToSentinel(src) },
		"values": func(kind, _ string, src []byte) []byte {
			if kind == "language" {
				return quotedToSentinel(src)
			}
			return valuesToSentinel(src)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			leaks := map[string]string{}
			factValues := 0
			for _, f := range allReadings(t, r, transform) {
				for _, s := range f.res.Symbols {
					if strings.Contains(s.Signature, sentinel) {
						leaks[f.res.Dialect+" "+s.Kind+" Signature"] = fmt.Sprintf("%s: %q", f.path, s.Signature)
					}
				}
				for _, n := range f.res.Notes {
					if strings.Contains(n, sentinel) {
						leaks[f.res.Dialect+" Note"] = fmt.Sprintf("%s: %q", f.path, n)
					}
				}
				for _, fact := range f.res.Facts {
					if !strings.Contains(fact.Value, sentinel) {
						continue
					}
					if lang.CredentialKey(fact.Key) {
						leaks[f.res.Dialect+" Fact value of "+fact.Key] = fmt.Sprintf("%s: %q", f.path, fact.Value)
					} else {
						factValues++
					}
				}
			}
			keys := make([]string, 0, len(leaks))
			for k := range leaks {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				t.Errorf("%s returns the sentinel, first in %s", k, leaks[k])
			}
			t.Logf("%d fact values under other keys carry the sentinel, returned as written", factValues)
		})
	}
}

// Notes carry a reason and a line, never the file's text.
func TestNotesCarryNoSourceText(t *testing.T) {
	r := newReader(t)
	for _, tc := range []struct{ name, path, src string }{
		{"go.mod directive", "go.mod", "module example.com/m\n\n" + sentinel + " x\n"},
		{"proto token", "api/x.proto", "syntax = \"proto3\";\nmessage M { string " + sentinel + " = ; }\n"},
		{"libconfig setting", "etc/proxysql.cnf", "admin_variables = { " + sentinel + " }\n"},
		{"json character", "data.json", "{\"a\": " + sentinel + "}"},
		{"toml", "data.toml", "a = \"" + sentinel + "\nb = 1\n"},
		{"yaml", "k/pod.yaml", "a: [" + sentinel + "\nb: c\n"},
		{"hcl", "infra/main.tf", "resource \"a\" \"" + sentinel + "\" {\n  x = \n}\n"},
		{"rego", "p.rego", "package p\n\npassword := \"" + sentinel + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := extract(t, r, tc.path, tc.src)
			if len(got.Notes) == 0 {
				t.Skipf("the shape reads without a note (incomplete=%v)", got.Incomplete)
			}
			for _, n := range got.Notes {
				if strings.Contains(n, sentinel) {
					t.Errorf("a note echoes the file: %q", n)
				}
			}
		})
	}
}

// factExtractor reads one fact per `key value` line.
type factExtractor struct{}

func (factExtractor) Extract(_ context.Context, f lang.File) (lang.Extracted, error) {
	var out lang.Extracted
	for i, line := range strings.Split(string(f.Src), "\n") {
		if w := strings.Fields(line); len(w) == 2 {
			out.Facts = append(out.Facts, lang.Fact{Entity: "E", Key: w[0], Value: w[1], Line: i + 1})
		}
	}
	return out, nil
}

// A fact value under a key whose last segment looks like a credential reads ***,
// whatever the value is. Any other fact value is returned as written.
func TestFactValueUnderACredentialKeyIsMasked(t *testing.T) {
	m := lang.Match{ID: "fact-rows", Kind: "tooling", Extensions: []string{".facts"}, Kinds: map[string]string{"RowDef": "function"}}
	r := newReader(t, lang.WithExtractor(m, factExtractor{}))
	got := extract(t, r, "a/b.facts", "labels.password "+sentinel+"\nenv.DB_PASSWORD "+sentinel+"\nclientSecret "+sentinel+"\nlabels.app web\nimage reg/a:1\n")
	want := map[string]string{"labels.password": "***", "env.DB_PASSWORD": "***", "clientSecret": "***", "labels.app": "web", "image": "reg/a:1"}
	if len(got.Facts) != len(want) {
		t.Fatalf("facts: %+v", got.Facts)
	}
	for _, f := range got.Facts {
		if f.Value != want[f.Key] {
			t.Errorf("fact %s reads %q, want %q", f.Key, f.Value, want[f.Key])
		}
	}
}
