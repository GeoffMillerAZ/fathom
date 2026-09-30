package langbench

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const linguistTestRev = "5fbdfcb8133be2bed88bf3ce62b2335f50474525"

func allowNames(names ...string) *[]string {
	s := append([]string{}, names...)
	return &s
}

// linguistQueue is the fixture queue: languages take extensions, filenames
// and interpreters; tooling takes filenames only.
func linguistQueue() []QueueEntry {
	lang := func(id, name string) QueueEntry { return QueueEntry{ID: id, Name: name, Kind: "language"} }
	tool := func(id, name string) QueueEntry { return QueueEntry{ID: id, Name: name, Kind: "tooling"} }
	return []QueueEntry{
		lang("objective-c", "Objective-C (.m .h by sniff)"),
		lang("matlab", "MATLAB (.m by sniff)"),
		lang("perl", "Perl (.pl .pm .t by sniff)"),
		lang("prolog", "Prolog (.pl .pro)"),
		lang("zed", "Zed (.zz)"),
		tool("bazel", "Bazel / Starlark"),
		tool("tiltfile", "Tiltfile (Starlark)"),
		tool("jenkinsfile", "Jenkins pipelines"),
		tool("dotenv", "Dotenv files"),
		tool("json-schema", "JSON Schema documents"),
		tool("gemfile-podfile", "Ruby DSL manifests"),
	}
}

func linguistMapping() map[string]LinguistItem {
	return map[string]LinguistItem{
		"objective-c":     {Linguist: []string{"Objective-C"}},
		"matlab":          {Linguist: []string{"MATLAB"}},
		"perl":            {Linguist: []string{"Perl"}},
		"prolog":          {Linguist: []string{"Prolog"}},
		"zed":             {Linguist: []string{"Zed"}},
		"bazel":           {Linguist: []string{"Starlark"}, AllowFilenames: allowNames("BUILD", "BUILD.bazel")},
		"tiltfile":        {Linguist: []string{"Starlark"}, AllowFilenames: allowNames("Tiltfile")},
		"jenkinsfile":     {Linguist: []string{"Groovy"}, AllowFilenames: allowNames("Jenkinsfile")},
		"dotenv":          {Linguist: []string{"Dotenv"}},
		"json-schema":     {Linguist: []string{"JSON"}, AllowFilenames: allowNames()},
		"gemfile-podfile": {Linguist: []string{"Ruby"}, AllowFilenames: allowNames("Gemfile", "Podfile")},
	}
}

func linguistFixture(t *testing.T) (languages, heuristics []byte) {
	t.Helper()
	l, err := os.ReadFile("testdata/linguist/languages.yml")
	if err != nil {
		t.Fatal(err)
	}
	h, err := os.ReadFile("testdata/linguist/heuristics.yml")
	if err != nil {
		t.Fatal(err)
	}
	return l, h
}

func importFixture(t *testing.T) LinguistData {
	t.Helper()
	l, h := linguistFixture(t)
	d, err := ImportLinguist(l, h, linguistTestRev, linguistQueue(), linguistMapping())
	if err != nil {
		t.Fatalf("ImportLinguist: %v", err)
	}
	return d
}

const (
	patObjC    = `^\s*(@(interface|class|protocol|property|end|synchronised|selector|implementation)\b|#import\s+.+\.h[">])`
	patMercury = `:- module`
	patPercent = `^\s*%`
	patFunc    = `^[ \t]*function[ \t]+(?:\[[^\]\r\n]*\][ \t]*=[ \t]*|[A-Za-z]\w*[ \t]*=[ \t]*)?[A-Za-z]\w*[ \t]*\(`
	patClass   = `^[ \t]*classdef[ \t]+(?:\([^\r\n)]*\)[ \t]*)?[A-Za-z]\w*`
	patProlog  = `^[^#]*:-`
	patPerl1   = `\buse\s+(?:strict\b|v?5\b)`
	patPerl2   = `^\s*use\s+(?:constant|overload)\b`
	patPerl3   = `^\s*package\s+[^\W\d]\w*(?:::\w+)*\s*(?:[;{]|\sv?\d)`
	patRaku    = `^\s*(?:use\s+v6\b|\bmodule\b|\b(?:my\s+)?class\b)`
	patNoV6    = `^\s*use\s+v6\b`
)

var patCpp = []string{
	`^\s*#\s*include <(cstdint|string|vector|map|list|array|bitset|queue|stack|forward_list|unordered_map|unordered_set|(i|o|io)stream)>`,
	`^\s*template\s*<`,
	`std::\w+`,
}

// TestImportLinguist is LB-F48/LB-F49's import half on the fixture yml: the
// id-to-name mapping, the tooling filenames-only rule, the shared-language
// allowlist, heuristic compile and drop counts, and byte-stable output.
func TestImportLinguist(t *testing.T) {
	d := importFixture(t)

	if d.Source != "github-linguist/linguist" || d.Revision != linguistTestRev || d.Licence != "MIT" {
		t.Errorf("provenance = %q %q %q", d.Source, d.Revision, d.Licence)
	}

	// The mapping: a language takes the extensions, filenames and
	// interpreters of every linguist language it names, sorted.
	perl := d.Items["perl"]
	if !reflect.DeepEqual(perl.Linguist, []string{"Perl"}) ||
		!reflect.DeepEqual(perl.Extensions, []string{".pl", ".pm", ".t"}) ||
		!reflect.DeepEqual(perl.Filenames, []string{"Makefile.PL", "cpanfile"}) ||
		!reflect.DeepEqual(perl.Interpreters, []string{"cperl", "perl"}) {
		t.Errorf("perl = %+v", perl)
	}
	if got := d.Items["objective-c"].Extensions; !reflect.DeepEqual(got, []string{".h", ".m"}) {
		t.Errorf("objective-c extensions = %v, want sorted [.h .m]", got)
	}

	// Tooling takes filenames only: never a language's extensions or
	// interpreters (Groovy has both), and never a heuristic.
	for _, id := range []string{"bazel", "tiltfile", "jenkinsfile", "dotenv", "json-schema", "gemfile-podfile"} {
		it := d.Items[id]
		if len(it.Extensions) != 0 || len(it.Interpreters) != 0 || len(it.Heuristics) != 0 {
			t.Errorf("tooling %s carries extensions %v, interpreters %v, heuristics %d", id, it.Extensions, it.Interpreters, len(it.Heuristics))
		}
	}
	if got := d.Items["dotenv"].Filenames; !reflect.DeepEqual(got, []string{".env", ".env.example"}) {
		t.Errorf("dotenv filenames = %v (an unshared tooling language keeps every linguist filename)", got)
	}

	// A language two items share keeps, per item, only its allowlist.
	for id, want := range map[string][]string{
		"bazel":           {"BUILD", "BUILD.bazel"},
		"tiltfile":        {"Tiltfile"},
		"jenkinsfile":     {"Jenkinsfile"},
		"json-schema":     {},
		"gemfile-podfile": {"Gemfile", "Podfile"},
	} {
		if got := d.Items[id].Filenames; !reflect.DeepEqual(got, want) {
			t.Errorf("%s filenames = %v, want %v", id, got, want)
		}
	}
	if d.Items["bazel"].AllowFilenames == nil || d.Items["json-schema"].AllowFilenames == nil {
		t.Error("the hand-kept allowlist, empty included, is not carried into the data")
	}

	// Heuristics: only for the extensions a language claims, named patterns
	// expanded, a rule's language mapped to a queue id when it is mapped.
	oc := d.Items["objective-c"].Heuristics
	if len(oc) != 2 || !reflect.DeepEqual(oc[0].Extensions, []string{".h"}) || !reflect.DeepEqual(oc[1].Extensions, []string{".m"}) {
		t.Fatalf("objective-c heuristics = %+v, want the .h and .m blocks", oc)
	}
	wantH := []LinguistRule{
		{Language: []string{"objective-c"}, LinguistCond: LinguistCond{Pattern: []string{patObjC}}},
		{Language: []string{"C++"}, LinguistCond: LinguistCond{Pattern: patCpp}},
		{Language: []string{"C"}},
	}
	if !reflect.DeepEqual(oc[0].Rules, wantH) {
		t.Errorf(".h rules = %+v\nwant %+v", oc[0].Rules, wantH)
	}
	wantM := []LinguistRule{
		{Language: []string{"objective-c"}, LinguistCond: LinguistCond{Pattern: []string{patObjC}}},
		{Language: []string{"Mercury"}, LinguistCond: LinguistCond{Pattern: []string{patMercury}}},
		{Language: []string{"matlab"}, LinguistCond: LinguistCond{Pattern: []string{patPercent}}},
		{Language: []string{"matlab"}, LinguistCond: LinguistCond{Pattern: []string{patFunc, patClass}}},
	}
	if !reflect.DeepEqual(oc[1].Rules, wantM) {
		t.Errorf(".m rules = %+v\nwant %+v", oc[1].Rules, wantM)
	}
	wantPl := []LinguistRule{
		{Language: []string{"prolog"}, LinguistCond: LinguistCond{Pattern: []string{patProlog}}},
		{Language: []string{"perl"}, And: []LinguistCond{{NegativePattern: []string{patNoV6}}, {Pattern: []string{patPerl1, patPerl2, patPerl3}}}},
		{Language: []string{"Raku"}, LinguistCond: LinguistCond{Pattern: []string{patRaku}}},
	}
	for _, id := range []string{"perl", "prolog"} {
		if h := d.Items[id].Heuristics; len(h) != 1 || !reflect.DeepEqual(h[0].Extensions, []string{".pl"}) || !reflect.DeepEqual(h[0].Rules, wantPl) {
			t.Errorf("%s heuristics = %+v\nwant one .pl block %+v", id, h, wantPl)
		}
	}
	if h := d.Items["matlab"].Heuristics; len(h) != 1 || !reflect.DeepEqual(h[0].Extensions, []string{".m"}) {
		t.Errorf("matlab heuristics = %+v, want the .m block (its .matlab has none)", h)
	}

	// A pattern RE2 cannot compile is dropped and counted; a rule left with
	// no alternative for a condition it stated is dropped too (it must not
	// turn into a rule that always matches), counted separately.
	zed := d.Items["zed"].Heuristics
	if len(zed) != 1 {
		t.Fatalf("zed heuristics = %+v", zed)
	}
	wantZ := []LinguistRule{
		{Language: []string{"zed"}, LinguistCond: LinguistCond{Pattern: []string{`^zed\b`}}},
		{Language: []string{"zed"}, LinguistCond: LinguistCond{Pattern: []string{`^ok\b`}}},
	}
	if !reflect.DeepEqual(zed[0].Rules, wantZ) {
		t.Errorf(".zz rules = %+v\nwant %+v", zed[0].Rules, wantZ)
	}
	// Counted once per imported block: .h 4 patterns, .m 5, .pl 6, .zz 6;
	// three fail (a lookahead, a negative lookahead, a lookbehind), and two
	// rules lose a whole condition.
	if want := (LinguistStats{Patterns: 21, Dropped: 3, DroppedRules: 2}); d.Stats != want {
		t.Errorf("stats = %+v, want %+v", d.Stats, want)
	}

	// Every stored pattern compiles exactly as ClassifyDialect compiles a sniff.
	assertLinguistPatternsCompile(t, d)

	// Byte-stable: the same input is the same bytes, the bytes reload to the
	// same bytes, and the file is 2-space indented with patterns unescaped.
	b1, err := MarshalLinguist(d)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := MarshalLinguist(importFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Error("two imports of the same input differ")
	}
	back, err := ParseLinguist(b1)
	if err != nil {
		t.Fatal(err)
	}
	if b3, _ := MarshalLinguist(back); !bytes.Equal(b1, b3) {
		t.Error("the data does not survive a reload byte for byte")
	}
	if !bytes.HasPrefix(b1, []byte("{\n  \"source\": ")) || !bytes.HasSuffix(b1, []byte("}\n")) {
		t.Errorf("not 2-space indented with a trailing newline:\n%.80s", b1)
	}
	if bytes.Contains(b1, []byte("\\u003"+"c")) || !bytes.Contains(b1, []byte(`include <(cstdint`)) {
		t.Error("a pattern's < was HTML-escaped")
	}
}

func assertLinguistPatternsCompile(t *testing.T, d LinguistData) {
	t.Helper()
	check := func(where string, ps []string) {
		for _, p := range ps {
			if _, err := regexp.Compile("(?m)" + p); err != nil {
				t.Errorf("%s: %q does not compile: %v", where, p, err)
			}
		}
	}
	for id, it := range d.Items {
		for _, h := range it.Heuristics {
			for i, r := range h.Rules {
				where := id + " " + strings.Join(h.Extensions, ",") + " rule " + string(rune('0'+i))
				check(where, r.Pattern)
				check(where, r.NegativePattern)
				for _, c := range r.And {
					check(where, c.Pattern)
					check(where, c.NegativePattern)
				}
			}
		}
	}
}

func TestImportLinguistRefusals(t *testing.T) {
	l, h := linguistFixture(t)
	q := linguistQueue()
	with := func(mut func(m map[string]LinguistItem)) map[string]LinguistItem {
		m := linguistMapping()
		mut(m)
		return m
	}
	cases := map[string]struct {
		rev     string
		mapping map[string]LinguistItem
		want    string
	}{
		"short revision":     {"5fbdfcb", linguistMapping(), "40"},
		"uppercase revision": {strings.ToUpper(linguistTestRev), linguistMapping(), "40"},
		"id not in queue": {linguistTestRev, with(func(m map[string]LinguistItem) {
			m["nowhere"] = LinguistItem{Linguist: []string{"Perl"}}
		}), `"nowhere"`},
		"linguist name unknown": {linguistTestRev, with(func(m map[string]LinguistItem) {
			m["perl"] = LinguistItem{Linguist: []string{"Pearl"}}
		}), `"Pearl"`},
		"no linguist name": {linguistTestRev, with(func(m map[string]LinguistItem) {
			m["perl"] = LinguistItem{}
		}), "perl"},
		"shared language without allowlist": {linguistTestRev, with(func(m map[string]LinguistItem) {
			m["tiltfile"] = LinguistItem{Linguist: []string{"Starlark"}}
		}), "Starlark"},
		"allowlist names a filename linguist lacks": {linguistTestRev, with(func(m map[string]LinguistItem) {
			m["tiltfile"] = LinguistItem{Linguist: []string{"Starlark"}, AllowFilenames: allowNames("Tiltfile", "Tiltfile.star")}
		}), "Tiltfile.star"},
	}
	for name, c := range cases {
		_, err := ImportLinguist(l, h, c.rev, q, c.mapping)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one naming %q", name, err, c.want)
		}
	}
	if _, err := ImportLinguist([]byte("not: [a, map"), h, linguistTestRev, q, linguistMapping()); err == nil {
		t.Error("a malformed languages.yml was accepted")
	}
	// An unknown heuristics key fails loudly rather than being ignored: a
	// rule field this importer does not understand would otherwise be lost.
	bad := bytes.Replace(h, []byte("  - language: Mercury\n"), []byte("  - language: Mercury\n    lookahead: yes\n"), 1)
	if _, err := ImportLinguist(l, bad, linguistTestRev, q, linguistMapping()); err == nil {
		t.Error("an unknown heuristics rule field was accepted")
	}
}

func draftFor(t *testing.T, d LinguistData, id string, claims ExtensionClaims) (DraftMatch, []string) {
	t.Helper()
	m, warns, err := DraftMatchFromLinguist(d, id, claims)
	if err != nil {
		t.Fatalf("DraftMatchFromLinguist(%s): %v", id, err)
	}
	return m, warns
}

func hasWarning(warns []string, parts ...string) bool {
	for _, w := range warns {
		ok := true
		for _, p := range parts {
			ok = ok && strings.Contains(w, p)
		}
		if ok {
			return true
		}
	}
	return false
}

// mustMatchSchema validates the draft as JSON against dialect.cue's #Match.
func mustMatchSchema(t *testing.T, id string, m DraftMatch) {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	cctx := cuecontext.New()
	v := cctx.CompileString(dialectSchema(t, false) + "\nm: #Match & " + string(b))
	if err := v.Err(); err != nil {
		t.Fatalf("%s: draft does not compile against #Match: %v\n%s", id, err, b)
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Errorf("%s: draft violates #Match: %v\n%s", id, err, b)
	}
}

// TestDraftMatchFromLinguist is LB-F49: a draft manifest match block from
// the imported data (extensions, basenames, shebangs, and a sniff for an
// extension another item or a registered dialect claims), validating against
// dialect.cue #Match, warning about every claimed extension.
func TestDraftMatchFromLinguist(t *testing.T) {
	d := importFixture(t)
	registered := map[string]ManifestMatch{
		"c":       {Extensions: []string{".c", ".h"}},
		"matlabx": {Sniff: &ManifestSniff{Extensions: []string{".zz"}, Pattern: "x", Priority: 50}},
	}
	claims := LinguistExtensionClaims(linguistQueue(), d, registered)

	// Objective-C: both its extensions are shared, so both move into the
	// sniff (the shape matlab's manifest has), one pattern from the two
	// blocks, first rule in linguist's order.
	oc, warns := draftFor(t, d, "objective-c", claims)
	if len(oc.Extensions) != 0 || oc.Sniff == nil || !reflect.DeepEqual(oc.Sniff.Extensions, []string{".h", ".m"}) ||
		oc.Sniff.Pattern != patObjC || oc.Sniff.Priority != 0 {
		t.Errorf("objective-c draft = %+v sniff %+v", oc, oc.Sniff)
	}
	if !hasWarning(warns, ".m", "matlab") || !hasWarning(warns, ".h", "c (registered dialect)") {
		t.Errorf("objective-c warnings = %q, want the .m (matlab) and .h (c) claims named", warns)
	}
	mustMatchSchema(t, "objective-c", oc)

	// MATLAB: .matlab is its own; .m is sniffed from both of its rules, at
	// linguist's position of the first (index 2, priority 20).
	ml, _ := draftFor(t, d, "matlab", claims)
	wantPat := "(?:" + patPercent + ")|(?:" + patFunc + ")|(?:" + patClass + ")"
	if !reflect.DeepEqual(ml.Extensions, []string{".matlab"}) || ml.Sniff == nil ||
		!reflect.DeepEqual(ml.Sniff.Extensions, []string{".m"}) || ml.Sniff.Pattern != wantPat || ml.Sniff.Priority != 20 {
		t.Errorf("matlab draft = %+v sniff %+v", ml, ml.Sniff)
	}
	mustMatchSchema(t, "matlab", ml)

	// Perl: filenames are basenames, interpreters shebangs; .pl is sniffed
	// (the and-rule's negative half cannot be RE2, and is warned about).
	pl, warns := draftFor(t, d, "perl", claims)
	if !reflect.DeepEqual(pl.Extensions, []string{".pm", ".t"}) ||
		!reflect.DeepEqual(pl.Basenames, []string{"Makefile.PL", "cpanfile"}) ||
		!reflect.DeepEqual(pl.Shebangs, []string{"cperl", "perl"}) || len(pl.Globs) != 0 {
		t.Errorf("perl draft = %+v", pl)
	}
	if pl.Sniff == nil || !reflect.DeepEqual(pl.Sniff.Extensions, []string{".pl"}) || pl.Sniff.Priority != 10 ||
		pl.Sniff.Pattern != "(?:"+patPerl1+")|(?:"+patPerl2+")|(?:"+patPerl3+")" {
		t.Errorf("perl sniff = %+v", pl.Sniff)
	}
	if !hasWarning(warns, "negative_pattern", ".pl") || !hasWarning(warns, ".pl", "prolog") {
		t.Errorf("perl warnings = %q", warns)
	}
	mustMatchSchema(t, "perl", pl)
	// The sniff does what linguist's rule says on the head of a file, as
	// ClassifyDialect compiles and runs it.
	re := regexp.MustCompile("(?m)" + pl.Sniff.Pattern)
	if !re.MatchString("#!/usr/bin/perl\nuse strict;\nmy $x = 1;\n") || re.MatchString("parent(X, Y) :- father(X, Y).\n") {
		t.Error("the perl sniff misjudges a perl and a prolog file")
	}

	// Prolog is first in the block, so it is tried first.
	pr, _ := draftFor(t, d, "prolog", claims)
	if pr.Sniff == nil || pr.Sniff.Priority != 0 || pr.Sniff.Pattern != patProlog ||
		!reflect.DeepEqual(pr.Sniff.Extensions, []string{".pl"}) || !reflect.DeepEqual(pr.Extensions, []string{".pro"}) {
		t.Errorf("prolog draft = %+v sniff %+v", pr, pr.Sniff)
	}

	// A tooling item is filenames only and has no sniff.
	bz, _ := draftFor(t, d, "bazel", claims)
	if len(bz.Extensions) != 0 || bz.Sniff != nil || !reflect.DeepEqual(bz.Basenames, []string{"BUILD", "BUILD.bazel"}) {
		t.Errorf("bazel draft = %+v", bz)
	}
	mustMatchSchema(t, "bazel", bz)

	// Only a registered dialect's sniff shares .zz: the claim is warned about
	// and the extension sniffed from the two rules that survived the import
	// (their position in the stored block, not in linguist's file).
	ze, warns := draftFor(t, d, "zed", claims)
	if ze.Sniff == nil || !reflect.DeepEqual(ze.Sniff.Extensions, []string{".zz"}) || ze.Sniff.Pattern != "(?:"+`^zed\b`+")|(?:"+`^ok\b`+")" || ze.Sniff.Priority != 0 {
		t.Errorf("zed draft = %+v sniff %+v", ze, ze.Sniff)
	}
	if !hasWarning(warns, ".zz", "matlabx (registered dialect)") {
		t.Errorf("zed warnings = %q", warns)
	}

	if _, _, err := DraftMatchFromLinguist(d, "nowhere", claims); err == nil {
		t.Error("a draft for an id with no linguist data succeeded")
	}
}

// TestDraftMatchEdges: an unshared extension is a plain claim with no
// sniff; a shared one no linguist rule can sniff stays plain and is warned;
// an extension #Match refuses is left out; a multi-dot suffix (which
// filepath.Ext never sees) becomes a glob.
func TestDraftMatchEdges(t *testing.T) {
	d := LinguistData{Items: map[string]LinguistItem{
		"thing": {
			Linguist:   []string{"Thing"},
			Extensions: []string{".Thing", ".thg", ".bad ext", ".cmake.in", ".def"},
			Filenames:  []string{"Thingfile"},
			Heuristics: []LinguistHeuristic{{Extensions: []string{".def"}, Rules: []LinguistRule{
				{Language: []string{"other"}, LinguistCond: LinguistCond{Pattern: []string{`^other`}}},
				{Language: []string{"thing"}},
			}}},
		},
	}}
	claims := ExtensionClaims{
		".def": {{ID: "other", Source: "registered dialect"}},
		".thg": {{ID: "thing", Source: "linguist data"}}, // its own claim is not a warning
	}
	m, warns := draftFor(t, d, "thing", claims)
	if !reflect.DeepEqual(m.Extensions, []string{".def", ".thg", ".thing"}) || m.Sniff != nil {
		t.Errorf("draft = %+v sniff %+v", m, m.Sniff)
	}
	if !reflect.DeepEqual(m.Globs, []string{"**/*.cmake.in"}) || !reflect.DeepEqual(m.Basenames, []string{"Thingfile"}) {
		t.Errorf("globs/basenames = %v / %v", m.Globs, m.Basenames)
	}
	for _, want := range [][]string{{".def", "other (registered dialect)"}, {".def", "no pattern"}, {".bad ext"}, {".cmake.in", "glob"}} {
		if !hasWarning(warns, want...) {
			t.Errorf("no warning containing %q in %q", want, warns)
		}
	}
	if hasWarning(warns, ".thg") {
		t.Errorf("an item's own claim was warned about: %q", warns)
	}
	mustMatchSchema(t, "thing", m)
}

func TestLinguistExtensionClaims(t *testing.T) {
	d := importFixture(t)
	claims := LinguistExtensionClaims(linguistQueue(), d, map[string]ManifestMatch{
		"c": {Extensions: []string{".C", ".h"}},
	})
	sources := func(ext, id string) []string {
		var s []string
		for _, c := range claims[ext] {
			if c.ID == id {
				s = append(s, c.Source)
			}
		}
		return s
	}
	if got := sources(".m", "matlab"); !reflect.DeepEqual(got, []string{"linguist data", "queue name"}) {
		t.Errorf(".m by matlab = %v", got)
	}
	if got := sources(".h", "c"); !reflect.DeepEqual(got, []string{"registered dialect"}) {
		t.Errorf(".h by c = %v", got)
	}
	if got := sources(".c", "c"); len(got) != 1 {
		t.Errorf("a dialect's extension is matched case-insensitively: .c by c = %v", got)
	}
	if len(claims[".pro"]) != 2 { // prolog: linguist data and queue name
		t.Errorf(".pro claims = %v", claims[".pro"])
	}
}

// TestLinguistDataMatchesQueue holds the committed data to the queue and to
// its own contract. It does not require every queue id to be present: the
// hand-kept mapping grows as queue items are added.
func TestLinguistDataMatchesQueue(t *testing.T) {
	q, err := LoadQueue("../../docs/plans/language-breadth.queue.json")
	if err != nil {
		t.Fatal(err)
	}
	kind := map[string]string{}
	for _, e := range q {
		kind[e.ID] = e.Kind
	}
	raw, err := os.ReadFile(LinguistDataPath("../.."))
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseLinguist(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Source != "github-linguist/linguist" || d.Licence != "MIT" {
		t.Errorf("provenance = %q %q", d.Source, d.Licence)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(d.Revision) {
		t.Errorf("revision %q is not a 40-hex commit", d.Revision)
	}
	if len(d.Items) == 0 {
		t.Fatal("no items")
	}
	for id, it := range d.Items {
		k, ok := kind[id]
		if !ok {
			t.Errorf("item %q is not a queue id", id)
			continue
		}
		if len(it.Linguist) == 0 {
			t.Errorf("%s names no linguist language", id)
		}
		if k == "tooling" && (len(it.Extensions) != 0 || len(it.Interpreters) != 0 || len(it.Heuristics) != 0) {
			t.Errorf("tooling item %s carries extensions %v, interpreters %v or heuristics", id, it.Extensions, it.Interpreters)
		}
		if it.AllowFilenames != nil {
			for _, f := range it.Filenames {
				if !slices.Contains(*it.AllowFilenames, f) {
					t.Errorf("%s keeps filename %q outside its allowlist", id, f)
				}
			}
		}
		for _, h := range it.Heuristics {
			for _, x := range h.Extensions {
				if !slicesContainsFold(it.Extensions, x) {
					t.Errorf("%s has a heuristic for %s, an extension it does not claim", id, x)
				}
			}
		}
	}
	assertLinguistPatternsCompile(t, d)
	if d.Stats.Patterns == 0 || d.Stats.Dropped > d.Stats.Patterns {
		t.Errorf("stats %+v are not a count of imported patterns", d.Stats)
	}
	// Canonical form: sorted, 2-space indented, byte-stable.
	if again, err := MarshalLinguist(d); err != nil || !bytes.Equal(again, raw) {
		t.Errorf("the committed file is not in MarshalLinguist's canonical form (err %v)", err)
	}
}

func slicesContainsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(x string) bool { return strings.EqualFold(x, s) })
}
