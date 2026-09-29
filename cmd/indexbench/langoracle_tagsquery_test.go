package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// The queries below are written for these tests against the TypeScript
// fixture (internal/scip/testdata/tsfixture); none is copied from a
// third-party query file, which would vendor it (ADR-048's D5 extension,
// condition c).
const (
	tqTagsQuery = `; tags.scm convention: @definition.<kind> on the definition node, @name on its name
(class_declaration name: (_) @name) @definition.class
(method_definition name: (_) @name) @definition.method
(function_declaration name: (_) @name) @definition.function
(variable_declarator name: (identifier) @name) @definition.variable
(call_expression function: (identifier) @name) @reference.call
`
	tqLocalsQuery = `; nvim-treesitter locals.scm convention: @local.definition.<kind> on the name node
(class_declaration name: (_) @local.definition.class)
(method_definition name: (_) @local.definition.method)
(function_declaration name: (_) @local.definition.function)
(required_parameter pattern: (identifier) @local.definition.parameter)
(statement_block) @local.scope
(identifier) @local.reference
`
	tqLegacyLocalsQuery = `; legacy locals.scm: @definition.<kind> on the name node, no @name
(class_declaration name: (_) @definition.class)
(method_definition name: (_) @definition.method)
(function_declaration name: (_) @definition.function)
(statement_block) @scope
`
	tqOutlineQuery = `; Zed outline.scm convention: @item on the item, @name on its name; the kind is the item's node type
(class_declaration "class" @context name: (_) @name) @item
(method_definition name: (_) @name) @item
(function_declaration "function" @context name: (_) @name) @item
`
	tqRev     = "0123456789abcdef0123456789abcdef01234567"
	tqSource  = "Example-Org/tree-sitter-fixture queries/tags.scm"
	tqLicence = "MIT"
)

// tqSubject is TestCmdLangOracle's TypeScript subject: three files, four
// definitions (Greeter, greet, run, add), and a file no dialect owns.
func tqSubject(t *testing.T) string {
	t.Helper()
	subject := t.TempDir()
	for _, f := range []string{"greeter.ts", "main.ts", "math.ts"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "internal", "scip", "testdata", "tsfixture", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subject, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(subject, "notes.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return subject
}

// tqDialect copies the typescript dialect directory to a temp dir, gives
// its manifest an oracle_kinds.tool table for the three conventions' kinds,
// and, when header is set, starts its tags.scm with that line. It returns
// the manifest's path.
func tqDialect(t *testing.T, header string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join("..", "..", "internal", "extractor", "dialects", "typescript")
	for _, f := range []string{"dialect.json", "tags.scm", "imports.scm"} {
		b, err := os.ReadFile(filepath.Join(src, f))
		if err != nil {
			t.Fatal(err)
		}
		switch f {
		case "dialect.json":
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			m["oracle_kinds"].(map[string]any)["tool"] = map[string]string{
				"class": "class", "method": "method", "function": "function", "interface": "interface", "enum": "enum",
				"class_declaration": "class", "method_definition": "method", "function_declaration": "function",
				"interface_declaration": "interface", "enum_declaration": "enum",
			}
			if b, err = json.MarshalIndent(m, "", "  "); err != nil {
				t.Fatal(err)
			}
		case "tags.scm":
			if header != "" {
				b = append([]byte(header+"\n"), b...)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "dialect.json")
}

// tqRun runs lang-oracle with a tags-query oracle; flags override the
// defaults (a flag given an empty value is dropped).
func tqRun(t *testing.T, subject, manifest, query, out, oracleOut string, flags map[string]string) (int, string) {
	t.Helper()
	set := map[string]string{
		"-dialect": "typescript", "-manifest": manifest, "-subject": subject, "-subject-name": "fixture/ts",
		"-out": out, "-date": "2026-09-30", "-commit": "abc1234",
		"-queue":      filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json"),
		"-tags-query": query, "-tags-query-rev": tqRev, "-tags-query-source": tqSource,
		"-tags-query-licence": tqLicence, "-tags-query-out": oracleOut,
	}
	for k, v := range flags {
		set[k] = v
	}
	var args []string
	for _, k := range sortedKeys(set) {
		if set[k] != "" {
			args = append(args, k, set[k])
		}
	}
	var stdout, stderr bytes.Buffer
	code := cmdLangOracle(args, &stdout, &stderr)
	return code, stderr.String()
}

func tqWriteQuery(t *testing.T, query string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "query.scm")
	if err := os.WriteFile(p, []byte(query), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func tqReading(t *testing.T, out string) langbench.Reading {
	t.Helper()
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var r langbench.Reading
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("written reading does not validate: %v", err)
	}
	return r
}

// TestCmdLangOracleTagsQuery (LB-F46, ADR-048's D5 extension): a
// third-party definitions query, run by fathom's own engine on the
// dialect's pinned grammar, is a tier-C L1 oracle in each of the three
// capture conventions; a perfect match reads L1 good, never excellent.
func TestCmdLangOracleTagsQuery(t *testing.T) {
	type def struct {
		Path string `json:"path"`
		Name string `json:"name"`
		Line int    `json:"line"`
		End  int    `json:"end"`
		Kind string `json:"kind"`
	}
	for _, tc := range []struct {
		name, query, format string
		ends                bool // the convention gives an end line
		defs                []def
		excluded            map[string]int
	}{
		{name: "tags", query: tqTagsQuery, format: "tags", ends: true,
			defs: []def{{"greeter.ts", "Greeter", 3, 8, "class"}, {"greeter.ts", "greet", 4, 7, "method"},
				{"main.ts", "run", 3, 6, "function"}, {"main.ts", "g", 4, 4, "variable"}, {"math.ts", "add", 1, 3, "function"}},
			excluded: map[string]int{"tool:variable": 1}},
		{name: "locals", query: tqLocalsQuery, format: "locals",
			defs: []def{{"greeter.ts", "Greeter", 3, 0, "class"}, {"greeter.ts", "greet", 4, 0, "method"}, {"greeter.ts", "name", 4, 0, "parameter"},
				{"main.ts", "run", 3, 0, "function"}, {"math.ts", "a", 1, 0, "parameter"}, {"math.ts", "add", 1, 0, "function"}, {"math.ts", "b", 1, 0, "parameter"}},
			excluded: map[string]int{"tool:parameter": 3}},
		{name: "locals-legacy", query: tqLegacyLocalsQuery, format: "locals",
			defs: []def{{"greeter.ts", "Greeter", 3, 0, "class"}, {"greeter.ts", "greet", 4, 0, "method"},
				{"main.ts", "run", 3, 0, "function"}, {"math.ts", "add", 1, 0, "function"}}},
		{name: "outline", query: tqOutlineQuery, format: "outline", ends: true,
			defs: []def{{"greeter.ts", "Greeter", 3, 8, "class_declaration"}, {"greeter.ts", "greet", 4, 7, "method_definition"},
				{"main.ts", "run", 3, 6, "function_declaration"}, {"math.ts", "add", 1, 3, "function_declaration"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			subject, manifest := tqSubject(t), tqDialect(t, "")
			out := filepath.Join(t.TempDir(), "typescript.json")
			oracleOut := filepath.Join(t.TempDir(), "oracle", "typescript-ts-tagsq.json")
			if code, stderr := tqRun(t, subject, manifest, tqWriteQuery(t, tc.query), out, oracleOut, nil); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}

			// The neutral oracle file: tier C, definitions only, provenance.
			raw, err := os.ReadFile(oracleOut)
			if err != nil {
				t.Fatal(err)
			}
			var tf struct {
				Tool        string `json:"tool"`
				Tier        string `json:"tier"`
				Source      string `json:"source"`
				Revision    string `json:"revision"`
				Licence     string `json:"licence"`
				Definitions []def  `json:"definitions"`
				Calls       []any  `json:"calls"`
			}
			if err := json.Unmarshal(raw, &tf); err != nil {
				t.Fatal(err)
			}
			if tf.Tier != "C" || tf.Tool != "tags-query/"+tc.format || tf.Source != tqSource || tf.Revision != tqRev || tf.Licence != tqLicence || len(tf.Calls) != 0 {
				t.Errorf("oracle file header = %+v", tf)
			}
			got, _ := json.Marshal(tf.Definitions)
			want, _ := json.Marshal(tc.defs)
			if !bytes.Equal(got, want) {
				t.Errorf("definitions:\n got %s\nwant %s", got, want)
			}

			// The reading: L1 from the query at tier C, graded good at most.
			r := tqReading(t, out)
			for _, m := range []string{langbench.MSymRecall, langbench.MSymPrecision, langbench.MKindAgreement} {
				if v := r.Metrics[m]; v == nil || *v != 1 {
					t.Errorf("%s = %v, want 1", m, v)
				}
				if r.MetricTiers[m] != "C" {
					t.Errorf("%s tier %q, want C", m, r.MetricTiers[m])
				}
			}
			if tc.ends {
				if v := r.Metrics[langbench.MRangeAgreement]; v == nil || *v != 1 {
					t.Errorf("range_agreement = %v, want 1", v)
				}
			} else if r.Metrics[langbench.MRangeAgreement] != nil || len(r.NotMeasured) != 1 || r.NotMeasured[0] != langbench.MRangeAgreement {
				t.Errorf("a convention without end lines measured range_agreement: %v %v", r.Metrics[langbench.MRangeAgreement], r.NotMeasured)
			}
			if r.GradeAt("L1") != langbench.GradeGood {
				t.Errorf("grade %+v, want L1 good (tier C never proves excellent)", r.Grade)
			}
			// The level: a three-file fixture's throughput under -race misses
			// the tenancy floor (§1.2), which is the machine's, not the
			// oracle's; with the floor held the same metrics and tiers read L1.
			held := r
			held.Tenancy.ThroughputMBs, held.Tenancy.MaxFileMS = langbench.MinThroughputMBs, 0
			if level, grade := langbench.LevelMet(held); level != "L1" || grade.L1 != langbench.GradeGood {
				t.Errorf("with the tenancy floor held: level_met %q grade %+v, want L1 good", level, grade)
			}
			for k, n := range tc.excluded {
				if r.ExcludedByKind[k] != n {
					t.Errorf("excluded_by_kind[%s] = %d, want %d (%v)", k, r.ExcludedByKind[k], n, r.ExcludedByKind)
				}
			}
			var run *langbench.OracleRun
			for i, o := range r.Oracles {
				if o.Tool == "tags-query/"+tc.format {
					run = &r.Oracles[i]
				}
				if o.Tool == "universal-ctags" {
					t.Errorf("ctags ran beside the query's definitions: %+v", r.Oracles)
				}
			}
			if run == nil || run.Tier != "C" || run.Source != tqSource || run.Revision != tqRev || run.Licence != tqLicence {
				t.Fatalf("oracle run = %+v (oracles %+v)", run, r.Oracles)
			}
		})
	}
}

// TestDetectTagsQueryFormat: the capture convention is read from the
// query's own capture names; -tags-query-format overrides it.
func TestDetectTagsQueryFormat(t *testing.T) {
	for query, want := range map[string]string{
		tqTagsQuery:         "tags",
		tqLocalsQuery:       "locals",
		tqLegacyLocalsQuery: "locals",
		tqOutlineQuery:      "outline",
		"; @item in a comment\n(class_declaration name: (_) @name) @definition.class\n":              "tags",
		"(identifier) @x (#eq? @x \"@item\")\n(function_declaration name: (_) @definition.function)": "locals",
	} {
		if got, err := detectTagsQueryFormat(query); err != nil || got != want {
			t.Errorf("detect(%q) = %q, %v; want %q", query, got, err, want)
		}
	}
	if _, err := detectTagsQueryFormat("(identifier) @x\n"); err == nil || !strings.Contains(err.Error(), "-tags-query-format") {
		t.Errorf("a query with no definition capture: %v", err)
	}
}

// TestCmdLangOracleTagsQueryRefusals (LB-F52, ADR-048's D5 extension): the
// query is refused when the dialect was drafted from it (a), when its
// revision is not a commit or its file is vendored into the work tree (c),
// without a licence or under AGPL (d), and on a predicate the engine does
// not support; the reading's oracle run records source, revision and
// licence, and Validate refuses a run that names a source without them.
func TestCmdLangOracleTagsQueryRefusals(t *testing.T) {
	subject := tqSubject(t)
	query := tqWriteQuery(t, tqTagsQuery)
	for _, tc := range []struct {
		name     string
		header   string // the temp dialect's tags.scm header line
		query    string
		flags    map[string]string
		want     string
		needsGit bool
	}{
		{name: "drafted-from", header: "; source: example-org/Tree-Sitter-Fixture queries/tags.scm @ " + tqRev + " (MIT)",
			want: "drafted from example-org/Tree-Sitter-Fixture queries/tags.scm"},
		{name: "revision-tag", flags: map[string]string{"-tags-query-rev": "v0.23.2"}, want: "40 lowercase hex"},
		{name: "revision-uppercase", flags: map[string]string{"-tags-query-rev": strings.ToUpper(tqRev)}, want: "40 lowercase hex"},
		{name: "vendored", query: filepath.Join("..", "..", "internal", "extractor", "dialects", "typescript", "tags.scm"),
			want: "vendored", needsGit: true},
		{name: "licence-missing", flags: map[string]string{"-tags-query-licence": " "}, want: "-tags-query-licence"},
		{name: "licence-agpl", flags: map[string]string{"-tags-query-licence": "AGPL-3.0-only"}, want: "AGPL"},
		{name: "source-shape", flags: map[string]string{"-tags-query-source": "queries/tags.scm"}, want: "<owner/repo> <path>"},
		{name: "predicate", query: tqWriteQuery(t, "((identifier) @name @definition.function (#lua-match? @name \"^a\"))\n"), want: "#lua-match?"},
		{name: "flags-without-query", query: "-", want: "need -tags-query"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needsGit && gitOut(t.Context(), ".", "rev-parse", "--show-toplevel") == "" {
				t.Skip("not inside a git work tree: nothing can be vendored")
			}
			q := query
			switch tc.query {
			case "":
			case "-":
				q = ""
			default:
				q = tc.query
			}
			out := filepath.Join(t.TempDir(), "typescript.json")
			oracleOut := filepath.Join(t.TempDir(), "tagsq.json")
			code, stderr := tqRun(t, subject, tqDialect(t, tc.header), q, out, oracleOut, tc.flags)
			if code == 0 || !strings.Contains(stderr, tc.want) {
				t.Fatalf("exit %d, stderr %q; want a refusal naming %q", code, stderr, tc.want)
			}
			for _, p := range []string{out, oracleOut} {
				if _, err := os.Stat(p); err == nil {
					t.Errorf("a refused query wrote %s", p)
				}
			}
		})
	}

	// A header naming another source, or the same repository's other file,
	// is not the query the dialect was drafted from.
	t.Run("other-source-accepted", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "typescript.json")
		manifest := tqDialect(t, "; source: example-org/tree-sitter-fixture queries/highlights.scm @ "+tqRev+" (MIT)")
		if code, stderr := tqRun(t, subject, manifest, query, out, filepath.Join(t.TempDir(), "tagsq.json"), nil); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		r := tqReading(t, out)
		var run langbench.OracleRun
		for _, o := range r.Oracles {
			if o.Source != "" {
				run = o
			}
		}
		if run.Source != tqSource || run.Revision != tqRev || run.Licence != tqLicence || run.Tier != "C" {
			t.Fatalf("the reading's oracle run lost its provenance: %+v", r.Oracles)
		}

		// Validate refuses a third-party run that lacks a pinned revision or
		// a licence, or claims a tier above C.
		for name, spoil := range map[string]func(*langbench.OracleRun){
			"revision": func(o *langbench.OracleRun) { o.Revision = "main" },
			"licence":  func(o *langbench.OracleRun) { o.Licence = "" },
			"tier":     func(o *langbench.OracleRun) { o.Tier = "B" },
		} {
			bad := r
			bad.Oracles = append([]langbench.OracleRun(nil), r.Oracles...)
			for i := range bad.Oracles {
				if bad.Oracles[i].Source != "" {
					spoil(&bad.Oracles[i])
				}
			}
			if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), tqSource) {
				t.Errorf("%s: Validate accepted %+v (%v)", name, bad.Oracles, err)
			}
		}
	})
}
