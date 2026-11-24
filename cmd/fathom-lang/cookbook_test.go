package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/encoding/jsonschema"
	_ "modernc.org/sqlite"
)

// HLP-4: the agent cookbook (docs/use-cases/library-agent-cookbook.md) and the tool definitions
// (examples/lang/agenttools/tools.json) are executed, not trusted. TestAgentCookbook builds the fathom-lang
// binary, makes a small tree (Go, Python and TypeScript files, a boundary rule file, a git repository with an
// uncommitted change), extracts the exact command text of every recipe and every tool definition from the
// committed files, runs it, and validates the output against the binary's own `schema` output wherever a schema
// exists. A recipe or tool that is missing from either side, stops running or whose output stops validating fails.
// SQL recipes run through the SQLite driver already in go.mod. Nothing here needs a tool the test does not run:
// sh, git, xargs and awk are the tools the recipes name, and the test runs each of them.

// schemaChecker compiles a JSON Schema (draft 2020-12) with the CUE module pinned in go.mod and returns a
// function that checks one JSON document against it.
func schemaChecker(t *testing.T, name string, src []byte) func(doc []byte) error {
	t.Helper()
	ctx := cuecontext.New()
	sv := ctx.CompileBytes(src)
	if err := sv.Err(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	f, err := jsonschema.Extract(sv, &jsonschema.Config{})
	if err != nil {
		t.Fatalf("%s: the pinned CUE cannot read the schema: %v", name, err)
	}
	schema := ctx.BuildFile(f)
	if err := schema.Err(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return func(doc []byte) error {
		dv := ctx.CompileBytes(doc)
		if err := dv.Err(); err != nil {
			return err
		}
		return schema.Unify(dv).Validate(cue.Concrete(true))
	}
}

var cookbookDoc = filepath.Join("..", "..", "docs", "use-cases", "library-agent-cookbook.md")
var cookbookTools = filepath.Join("..", "..", "examples", "lang", "agenttools", "tools.json")

// cookbookFence matches the opening line of a recipe block: ```sh recipe=NAME or ```sql recipe=NAME.
var cookbookFence = regexp.MustCompile("^```(sh|sql) recipe=([a-z0-9-]+)$")

// parseCookbook returns the body of every recipe block, by name, from the committed guide.
func parseCookbook(t *testing.T) map[string]string {
	t.Helper()
	b, err := os.ReadFile(cookbookDoc)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	var name string
	var body []string
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case name == "":
			if m := cookbookFence.FindStringSubmatch(line); m != nil {
				name, body = m[2], nil
			}
		case line == "```":
			if _, dup := out[name]; dup {
				t.Fatalf("recipe %q appears twice in the cookbook", name)
			}
			out[name] = strings.Join(body, "\n") + "\n"
			name = ""
		default:
			body = append(body, line)
		}
	}
	if name != "" {
		t.Fatalf("recipe %q has no closing fence", name)
	}
	return out
}

// sampleFence matches the opening line of a displayed sample: ```text sample=NAME (the block is the exact output)
// or ```json sample-elided=NAME (the block is the output with "..." standing for what it leaves out).
var sampleFence = regexp.MustCompile("^```(text|json) (sample|sample-elided)=([a-z0-9-]+)$")

type sampleBlock struct {
	elided bool
	body   string
}

// parseSamples returns every displayed sample block of the guide, by name.
func parseSamples(t *testing.T) map[string]sampleBlock {
	t.Helper()
	b, err := os.ReadFile(cookbookDoc)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]sampleBlock{}
	var name string
	var cur sampleBlock
	var body []string
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case name == "":
			if m := sampleFence.FindStringSubmatch(line); m != nil {
				name, cur, body = m[3], sampleBlock{elided: m[2] == "sample-elided"}, nil
			}
		case line == "```":
			cur.body = strings.Join(body, "\n") + "\n"
			out[name] = cur
			name = ""
		default:
			body = append(body, line)
		}
	}
	return out
}

var anySpace = regexp.MustCompile(`\s+`)

// checkSample fails when a displayed sample is not what the recipe printed. An exact sample must equal the output
// byte for byte. An elided sample must be the output with parts left out: ignoring white space, each piece between
// the "..." marks occurs in the output, in order.
func checkSample(t *testing.T, name string, sb sampleBlock, actual string) {
	t.Helper()
	if !sb.elided {
		if sb.body != actual {
			t.Errorf("sample %q differs from the recipe output:\n--- sample ---\n%s--- output ---\n%s", name, sb.body, actual)
		}
		return
	}
	hay := anySpace.ReplaceAllString(actual, "")
	for _, piece := range strings.Split(sb.body, "...") {
		piece = anySpace.ReplaceAllString(piece, "")
		i := strings.Index(hay, piece)
		if i < 0 {
			t.Errorf("sample %q: piece %q is not in the recipe output (in order):\n%s", name, piece, actual)
			return
		}
		hay = hay[i+len(piece):]
	}
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	Command     []string       `json:"command"`
	Output      struct {
		Format string `json:"format"`
		Schema string `json:"schema"`
	} `json:"output"`
	ExitCodes    map[string]string `json:"exit_codes"`
	ExampleInput map[string]any    `json:"example_input"`
}

type toolsFile struct {
	SchemaVersion int       `json:"schema_version"`
	Provisional   bool      `json:"provisional"`
	Description   string    `json:"description"`
	Tools         []toolDef `json:"tools"`
}

func parseTools(t *testing.T) toolsFile {
	t.Helper()
	b, err := os.ReadFile(cookbookTools)
	if err != nil {
		t.Fatal(err)
	}
	var tf toolsFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tf); err != nil {
		t.Fatalf("tools.json: %v", err)
	}
	return tf
}

var placeholder = regexp.MustCompile(`^\{([a-z_]+)\}$`)

// expandCommand fills a command template. An element that is exactly {name} becomes the parameter: one argument for
// a string, one argument per item for an array. No other substitution exists, so a value is never parsed by a shell.
func expandCommand(t *testing.T, tpl []string, in map[string]any) []string {
	t.Helper()
	var argv []string
	for _, el := range tpl {
		m := placeholder.FindStringSubmatch(el)
		if m == nil {
			argv = append(argv, el)
			continue
		}
		switch v := in[m[1]].(type) {
		case string:
			argv = append(argv, v)
		case []any:
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					t.Fatalf("parameter %q has a non-string item %v", m[1], item)
				}
				argv = append(argv, s)
			}
		default:
			t.Fatalf("placeholder {%s} has no string or array value in the input (%v)", m[1], in)
		}
	}
	return argv
}

// cookbookTree writes the small tree every recipe and tool runs against, makes it a git repository, commits it and
// then changes it without committing, so recipe 3 has a diff to read.
func cookbookTree(t *testing.T) string {
	t.Helper()
	root, write, _ := cookbookBase(t)
	// The change: one line edited in A, a line added in B, one edited in Greeter.hello, and a comment line removed
	// from util.py (a hunk with no new lines).
	write("pkg/a.go", "package pkg\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc A() {\n\tB()\n\tfmt.Println(\"a\")\n}\n\nfunc B() {\n\tfmt.Println(\"b\")\n}\n")
	write("app.py", "import os\nfrom util import helper\n\nclass Greeter:\n    def hello(self, name):\n        return helper(name.title())\n\ndef main():\n    print(os.getcwd())\n")
	write("util.py", "def helper(x):\n    return x\n")
	return root
}

// cookbookBase writes the tree and commits it, with no change after the commit. It returns the root, a function that
// writes a file under it and a function that runs git in it.
func cookbookBase(t *testing.T) (string, func(name, body string), func(args ...string)) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/fixture\n")
	write("pkg/a.go", "package pkg\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc A() {\n\tB()\n\tfmt.Println()\n}\n\nfunc B() {\n}\n")
	write("app.py", "import os\nfrom util import helper\n\nclass Greeter:\n    def hello(self, name):\n        return helper(name)\n\ndef main():\n    print(os.getcwd())\n")
	write("util.py", "def helper(x):\n    # identity\n    return x\n")
	write("web/index.ts", "import { add } from \"./lib\";\n\nexport function run(): number {\n  return add(1, 2);\n}\n")
	write("web/lib.ts", "export function add(a: number, b: number): number {\n  return a + b;\n}\n")
	write("rules.json", `{"schema_version":1,"rules":[`+
		`{"id":"app-not-util","from":["app.py"],"deny":["util.py"],"reason":"app reaches util through its package"},`+
		`{"id":"web-isolated","from":["web/**/*.ts"],"deny":["**/*.py"],"reason":"web stays out of the Python tree"}]}`+"\n")

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=cookbook", "-c", "user.email=cookbook@invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	return root, write, git
}

func buildLangBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(bin, "fathom-lang")
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", exe, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func runIn(dir string, name string, args ...string) (stdout, stderr string, code int) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	code = 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			return o.String(), err.Error(), -1
		}
		code = ee.ExitCode()
	}
	return o.String(), e.String(), code
}

type atDocT struct {
	SchemaVersion int `json:"schema_version"`
	Queries       []struct {
		Path      string `json:"path"`
		Start     int    `json:"start"`
		End       int    `json:"end"`
		Status    string `json:"status"`
		Enclosing *struct {
			Name string `json:"name"`
		} `json:"enclosing"`
		Touched []struct {
			Name string `json:"name"`
		} `json:"touched"`
		Complete bool `json:"complete"`
	} `json:"queries"`
}

func decodeAt(t *testing.T, out string) atDocT {
	t.Helper()
	var d atDocT
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("at output is not JSON: %v\n%s", err, out)
	}
	return d
}

type depsDocT struct {
	SchemaVersion int `json:"schema_version"`
	Files         []struct {
		Path  string `json:"path"`
		Edges []struct {
			Raw        string `json:"raw"`
			To         string `json:"to"`
			Resolution string `json:"resolution"`
		} `json:"edges"`
		Unknowns []struct {
			Code string `json:"code"`
			Raw  string `json:"raw"`
		} `json:"unknowns"`
	} `json:"files"`
}

type boundariesDocT struct {
	SchemaVersion int `json:"schema_version"`
	Rules         []struct {
		ID         string `json:"id"`
		Verdict    string `json:"verdict"`
		Violations []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"violations"`
	} `json:"rules"`
}

// checkOutline asserts the text form's frame: a header line and a footer line around the nodes.
func checkOutline(t *testing.T, out string) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "# ") || !strings.Contains(lines[0], " dialect=") ||
		!strings.HasPrefix(lines[len(lines)-1], "# symbols=") {
		t.Errorf("outline text has no header and footer frame:\n%s", out)
	}
}

func checkDeps(t *testing.T, out string) {
	t.Helper()
	var d depsDocT
	if err := json.Unmarshal([]byte(out), &d); err != nil || d.SchemaVersion != 1 || len(d.Files) != 3 {
		t.Fatalf("deps output: %v, %d files\n%s", err, len(d.Files), out)
	}
	var resolved, unknownOS bool
	for _, f := range d.Files {
		for _, e := range f.Edges {
			if f.Path == "app.py" && e.To == "util.py" && e.Resolution == "file" {
				resolved = true
			}
		}
		for _, u := range f.Unknowns {
			if f.Path == "app.py" && u.Raw == "os" {
				unknownOS = true
			}
		}
	}
	if !resolved || !unknownOS {
		t.Errorf("deps: app.py -> util.py resolved=%v, unknown for os=%v\n%s", resolved, unknownOS, out)
	}
}

func checkBoundaries(t *testing.T, out string) {
	t.Helper()
	var d boundariesDocT
	if err := json.Unmarshal([]byte(out), &d); err != nil || d.SchemaVersion != 1 {
		t.Fatalf("boundaries output: %v\n%s", err, out)
	}
	got := map[string]string{}
	for _, r := range d.Rules {
		got[r.ID] = r.Verdict
	}
	want := map[string]string{"app-not-util": "violated", "web-isolated": "clean"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("boundaries verdicts = %v, want %v", got, want)
	}
}

// checkDiffSymbols asserts what recipe 3 must report on the fixture change: A and B in pkg/a.go and Greeter.hello in
// app.py, each found, and a stderr note for the hunk that removed a line from util.py.
func checkDiffSymbols(t *testing.T, out, errOut string) {
	t.Helper()
	d := decodeAt(t, out)
	type q struct {
		path, status, enclosing string
	}
	var got []q
	for _, x := range d.Queries {
		e := ""
		if x.Enclosing != nil {
			e = x.Enclosing.Name
		}
		got = append(got, q{x.Path, x.Status, e})
	}
	want := []q{{"app.py", "found", "Greeter.hello"}, {"pkg/a.go", "found", "A"}, {"pkg/a.go", "found", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diff symbols = %v, want %v", got, want)
	}
	if !strings.Contains(errOut, "util.py") || !strings.Contains(errOut, "not queried") {
		t.Errorf("a deletion-only hunk must be reported on stderr, got %q", errOut)
	}
}

func jsonLinesCheck(t *testing.T, check func([]byte) error, out string) {
	t.Helper()
	if err := validateRows(check, out); err != nil {
		t.Errorf("rows do not validate: %v", err)
	}
}

func TestAgentCookbook(t *testing.T) {
	for _, tool := range []string{"sh", "git", "xargs", "awk", "sed"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the cookbook recipes use %s and the test must run them: %v", tool, err)
		}
	}
	bin := buildLangBinary(t)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := cookbookTree(t)

	// The binary's own schemas.
	schemas := map[string]func([]byte) error{}
	for _, name := range []string{"extract", "rows", "at", "outline"} {
		out, errw, code := runIn(root, "fathom-lang", "schema", name)
		if code != 0 {
			t.Fatalf("fathom-lang schema %s: exit %d: %s", name, code, errw)
		}
		schemas[name] = schemaChecker(t, name, []byte(out))
	}
	validate := func(t *testing.T, schema, format, out string) {
		t.Helper()
		switch {
		case schema == "":
		case format == "jsonl":
			jsonLinesCheck(t, schemas[schema], out)
		default:
			if err := schemas[schema]([]byte(out)); err != nil {
				t.Errorf("output does not validate against %q: %v\n%s", schema, err, out)
			}
		}
	}

	// Recipes: the exact text of each block, run with sh in the tree.
	recipes := parseCookbook(t)
	type recipeCase struct {
		exit   int
		schema string // the binary's schema the output must validate against, "" when none exists
		format string // json, jsonl or text
		check  func(t *testing.T, out, errOut string)
	}
	cases := map[string]recipeCase{
		"outline":      {0, "", "text", func(t *testing.T, out, _ string) { checkOutline(t, out) }},
		"outline-json": {0, "outline", "json", nil},
		"at-line": {0, "at", "json", func(t *testing.T, out, _ string) {
			d := decodeAt(t, out)
			if len(d.Queries) != 2 || d.Queries[0].Status != "found" || d.Queries[0].Enclosing == nil || d.Queries[0].Enclosing.Name != "Greeter.hello" ||
				d.Queries[1].Status != "found" || d.Queries[1].Enclosing == nil || d.Queries[1].Enclosing.Name != "A" {
				t.Errorf("at-line answers: %+v", d.Queries)
			}
		}},
		"at-range": {0, "at", "json", func(t *testing.T, out, _ string) {
			d := decodeAt(t, out)
			if len(d.Queries) != 1 || d.Queries[0].Status != "none" || d.Queries[0].Enclosing != nil || len(d.Queries[0].Touched) != 2 || !d.Queries[0].Complete {
				t.Errorf("at-range answer (no one symbol holds a range that spans A and B, and both are touched): %+v", d.Queries)
			}
		}},
		"diff-symbols": {0, "at", "json", func(t *testing.T, out, e string) { checkDiffSymbols(t, out, e) }},
		"deps":         {0, "", "json", func(t *testing.T, out, _ string) { checkDeps(t, out) }},
		"boundaries":   {1, "", "json", func(t *testing.T, out, _ string) { checkBoundaries(t, out) }},
		"rows":         {0, "rows", "jsonl", nil},
	}
	sqlNames := []string{"sql-setup", "sql-symbols-per-kind", "sql-longest-symbols", "sql-refs-without-target"}

	t.Run("cookbook-names-match-the-test", func(t *testing.T) {
		var have, want []string
		for n := range recipes {
			have = append(have, n)
		}
		for n := range cases {
			want = append(want, n)
		}
		want = append(want, sqlNames...)
		sort.Strings(have)
		sort.Strings(want)
		if !reflect.DeepEqual(have, want) {
			t.Fatalf("recipes in the cookbook %v, recipes the test runs %v", have, want)
		}
	})

	outputs := map[string]string{}
	for name, rc := range cases {
		t.Run("recipe/"+name, func(t *testing.T) {
			body, ok := recipes[name]
			if !ok {
				t.Fatalf("the cookbook has no recipe %q", name)
			}
			out, errOut, code := runIn(root, "sh", "-c", body)
			if code != rc.exit {
				t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", code, rc.exit, out, errOut)
			}
			validate(t, rc.schema, rc.format, out)
			if rc.check != nil {
				rc.check(t, out, errOut)
			}
			outputs[name] = out
		})
	}

	// Displayed samples are checked against what the recipe printed.
	t.Run("samples", func(t *testing.T) {
		samples := parseSamples(t)
		want := map[string]string{"outline": "outline", "at-line": "at-line", "deps": "deps"}
		for name, recipe := range want {
			sb, ok := samples[name]
			if !ok {
				t.Errorf("the cookbook has no sample %q", name)
				continue
			}
			checkSample(t, name, sb, outputs[recipe])
		}
		if len(samples) != len(want) {
			t.Errorf("samples in the cookbook %d, samples the test checks %d", len(samples), len(want))
		}
	})

	// Recipe 6: the rows loaded into SQLite through the driver already in go.mod, then the three queries.
	t.Run("recipe/sql", func(t *testing.T) {
		rows := outputs["rows"]
		if rows == "" {
			t.Fatal("the rows recipe produced no output")
		}
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		db.SetMaxOpenConns(1)
		if _, err := db.Exec(`CREATE TABLE rows (doc TEXT)`); err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSuffix(rows, "\n"), "\n") {
			if _, err := db.Exec(`INSERT INTO rows (doc) VALUES (?)`, line); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(recipes["sql-setup"]); err != nil {
			t.Fatalf("sql-setup: %v", err)
		}
		query := func(name string) [][]any {
			rs, err := db.Query(recipes[name])
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			defer rs.Close()
			cols, _ := rs.Columns()
			var all [][]any
			for rs.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rs.Scan(ptrs...); err != nil {
					t.Fatal(err)
				}
				for i, v := range vals {
					if b, ok := v.([]byte); ok {
						vals[i] = string(b)
					}
				}
				all = append(all, vals)
			}
			return all
		}

		// The independent reference: the nested document of the same files.
		list, _, _ := runIn(root, "git", "ls-files", "-z", "--", "*.go", "*.py", "*.ts")
		files := strings.Split(strings.TrimSuffix(list, "\x00"), "\x00")
		nested, errw, code := runIn(root, "fathom-lang", append([]string{"extract", "-walk"}, files...)...)
		if code != 0 {
			t.Fatalf("extract: exit %d: %s", code, errw)
		}
		var doc struct {
			Files []struct {
				Path   string `json:"path"`
				Result struct {
					Dialect string `json:"dialect"`
					Symbols []struct {
						Name      string `json:"name"`
						Canonical string `json:"canonical"`
						Line      int    `json:"line"`
						EndLine   int    `json:"end_line"`
					} `json:"symbols"`
					References []struct {
						ToKey string `json:"to_key"`
					} `json:"references"`
				} `json:"result"`
			} `json:"files"`
		}
		if err := json.Unmarshal([]byte(nested), &doc); err != nil {
			t.Fatal(err)
		}

		// Q1: symbols per canonical kind per file.
		var want1 [][]any
		type kc struct{ path, canonical string }
		count := map[kc]int64{}
		for _, f := range doc.Files {
			for _, s := range f.Result.Symbols {
				count[kc{f.Path, s.Canonical}]++
			}
		}
		for k, n := range count {
			want1 = append(want1, []any{k.path, k.canonical, n})
		}
		sort.Slice(want1, func(i, j int) bool {
			if want1[i][0] != want1[j][0] {
				return want1[i][0].(string) < want1[j][0].(string)
			}
			return want1[i][1].(string) < want1[j][1].(string)
		})
		if got := query("sql-symbols-per-kind"); len(want1) < 6 || !reflect.DeepEqual(got, want1) {
			t.Errorf("symbols per kind per file:\n got %v\nwant %v (and at least 6 groups)", got, want1)
		}

		// Q2: the five longest symbols whose span is known.
		type span struct {
			path, name string
			lines, ln  int64
		}
		var spans []span
		for _, f := range doc.Files {
			for _, s := range f.Result.Symbols {
				if s.Line > 0 && s.EndLine >= s.Line {
					spans = append(spans, span{f.Path, s.Name, int64(s.EndLine - s.Line + 1), int64(s.Line)})
				}
			}
		}
		sort.Slice(spans, func(i, j int) bool {
			a, b := spans[i], spans[j]
			if a.lines != b.lines {
				return a.lines > b.lines
			}
			if a.path != b.path {
				return a.path < b.path
			}
			if a.name != b.name {
				return a.name < b.name
			}
			return a.ln < b.ln
		})
		var want2 [][]any
		for _, s := range spans[:5] {
			want2 = append(want2, []any{s.path, s.name, s.lines})
		}
		if got := query("sql-longest-symbols"); !reflect.DeepEqual(got, want2) {
			t.Errorf("longest symbols:\n got %v\nwant %v", got, want2)
		}

		// Q3: references whose target is not a symbol of the same file, per file.
		var want3 [][]any
		for _, f := range doc.Files {
			var n int64
			for _, r := range f.Result.References {
				if r.ToKey == "" {
					n++
				}
			}
			if n > 0 {
				want3 = append(want3, []any{f.Path, f.Result.Dialect, n})
			}
		}
		sort.Slice(want3, func(i, j int) bool { return want3[i][0].(string) < want3[j][0].(string) })
		if got := query("sql-refs-without-target"); len(want3) == 0 || !reflect.DeepEqual(got, want3) {
			t.Errorf("references without an in-file target:\n got %v\nwant %v (and at least one file)", got, want3)
		}
	})

	// Tools: the argv template of each definition, filled from its own example input, run in the tree.
	tools := parseTools(t)
	t.Run("tools/file", func(t *testing.T) {
		if tools.SchemaVersion != 1 || !tools.Provisional {
			t.Errorf("tools.json must carry schema_version 1 and provisional true: %+v", tools)
		}
	})
	type toolCase struct {
		exit  int
		check func(t *testing.T, out, errOut string)
	}
	toolCases := map[string]toolCase{
		"outline_file": {0, func(t *testing.T, out, _ string) { checkOutline(t, out) }},
		"enclosing_symbol": {0, func(t *testing.T, out, _ string) {
			d := decodeAt(t, out)
			if len(d.Queries) != 1 || d.Queries[0].Status != "found" || d.Queries[0].Enclosing == nil || d.Queries[0].Enclosing.Name != "Greeter.hello" {
				t.Errorf("enclosing_symbol: %+v", d.Queries)
			}
		}},
		"symbols_touched_by_diff": {0, func(t *testing.T, out, e string) { checkDiffSymbols(t, out, e) }},
		"file_dependencies":       {0, func(t *testing.T, out, _ string) { checkDeps(t, out) }},
		"boundary_check":          {1, func(t *testing.T, out, _ string) { checkBoundaries(t, out) }},
		"tree_rows":               {0, nil},
	}
	var seen []string
	for _, tool := range tools.Tools {
		seen = append(seen, tool.Name)
		tc, ok := toolCases[tool.Name]
		t.Run("tool/"+tool.Name, func(t *testing.T) {
			if !ok {
				t.Fatalf("no test case for tool %q", tool.Name)
			}
			if tool.Description == "" || len(tool.Command) == 0 || tool.Command[0] == "" {
				t.Fatalf("a tool needs a description and a command: %+v", tool)
			}
			// Input schema: compiles, accepts its own example, rejects an empty input when something is required.
			check := schemaChecker(t, tool.Name+" input", mustJSON(t, tool.InputSchema))
			if err := check(mustJSON(t, tool.ExampleInput)); err != nil {
				t.Fatalf("example_input does not validate against input_schema: %v", err)
			}
			if req, _ := tool.InputSchema["required"].([]any); len(req) > 0 {
				if err := check([]byte(`{}`)); err == nil {
					t.Errorf("input_schema accepts an empty input but requires %v", req)
				}
			}
			if _, ok := tool.ExitCodes[fmt.Sprint(tc.exit)]; !ok {
				t.Errorf("exit_codes does not declare exit %d: %v", tc.exit, tool.ExitCodes)
			}
			argv := expandCommand(t, tool.Command, tool.ExampleInput)
			out, errOut, code := runIn(root, argv[0], argv[1:]...)
			if code != tc.exit {
				t.Fatalf("%v: exit %d, want %d\nstdout: %s\nstderr: %s", argv, code, tc.exit, out, errOut)
			}
			if _, ok := tool.ExitCodes[fmt.Sprint(code)]; !ok {
				t.Errorf("exit %d is not declared in exit_codes %v", code, tool.ExitCodes)
			}
			validate(t, tool.Output.Schema, tool.Output.Format, out)
			if tc.check != nil {
				tc.check(t, out, errOut)
			}
		})
	}
	sort.Strings(seen)
	var wantTools []string
	for n := range toolCases {
		wantTools = append(wantTools, n)
	}
	sort.Strings(wantTools)
	if !reflect.DeepEqual(seen, wantTools) {
		t.Errorf("tools in tools.json %v, tools the test runs %v", seen, wantTools)
	}

	// The diff script is one text in two places.
	t.Run("diff-script-is-one-text", func(t *testing.T) {
		for _, tool := range tools.Tools {
			if tool.Name != "symbols_touched_by_diff" {
				continue
			}
			if len(tool.Command) < 3 || tool.Command[0] != "sh" || tool.Command[1] != "-c" {
				t.Fatalf("symbols_touched_by_diff must be sh -c SCRIPT: %v", tool.Command)
			}
			if tool.Command[2]+"\n" != recipes["diff-symbols"] {
				t.Errorf("the script in tools.json differs from the cookbook recipe:\n%s\n--- vs ---\n%s", tool.Command[2], recipes["diff-symbols"])
			}
		}
	})

	// CH-D3: a diff whose every change goes unqueried must not look like "no symbol touched".
	t.Run("diff-unqueried-changes", func(t *testing.T) {
		var script string
		for _, tool := range tools.Tools {
			if tool.Name == "symbols_touched_by_diff" {
				script = tool.Command[2]
				for _, code := range []string{"0", "2", "3"} {
					if _, ok := tool.ExitCodes[code]; !ok {
						t.Errorf("exit_codes does not declare %s: %v", code, tool.ExitCodes)
					}
				}
			}
		}
		const aBody = "package pkg\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc A() {\n\tfmt.Println()\n}\n\nfunc B() {\n}\n"
		cases := []struct {
			name       string
			change     func(write func(string, string), git func(...string))
			exit       int
			stdoutHas  string
			stderrHas  []string
			stdoutNone bool
			base       string // default HEAD
			where      string // "" the cookbook repo, "notrepo", "unborn"
		}{
			{"no-changes", func(func(string, string), func(...string)) {}, 0, "", []string{"no changes"}, true, "", ""},
			{"deletion-only", func(w func(string, string), _ func(...string)) { w("pkg/a.go", aBody) }, 3, "",
				[]string{"deletion-only hunk in pkg/a.go", "none could be queried"}, true, "", ""},
			{"deleted-file", func(_ func(string, string), g func(...string)) { g("rm", "-q", "util.py") }, 3, "",
				[]string{"deleted file util.py: not queried", "none could be queried"}, true, "", ""},
			{"untracked-file", func(w func(string, string), _ func(...string)) { w("new.py", "def f():\n    pass\n") }, 3, "",
				[]string{"untracked file new.py: not queried", "none could be queried"}, true, "", ""},
			// A pure rename must not print nothing: the old path is a deleted file and the new path is queried whole.
			{"pure-rename", func(_ func(string, string), g func(...string)) { g("mv", "web/lib.ts", "web/lib2.ts") }, 0, `"path": "web/lib2.ts"`,
				[]string{"deleted file web/lib.ts: not queried"}, false, "", ""},
			{"edit-and-delete", func(w func(string, string), g func(...string)) {
				w("web/lib.ts", "export function add(a: number, b: number): number {\n  return b + a;\n}\n")
				g("rm", "-q", "util.py")
			}, 0, `"path": "web/lib.ts"`, []string{"deleted file util.py: not queried"}, false, "", ""},
			{"non-ascii-and-dash-paths", func(w func(string, string), g func(...string)) {
				w("\u00e9.py", "def e():\n    return 1\n")
				w("-x.py", "def x():\n    return 1\n")
				g("add", "--", "\u00e9.py", "-x.py")
				g("commit", "-q", "-m", "more")
				w("\u00e9.py", "def e():\n    return 2\n")
				w("-x.py", "def x():\n    return 2\n")
			}, 0, "\"path\": \"\u00e9.py\"", nil, false, "", ""},
			// CH-D3: git's own failure must not read as "no changes".
			// Run from a subdirectory, root changes are still found and named.
			{name: "subdir-untracked-root-file", change: func(w func(string, string), _ func(...string)) { w("rootnew.py", "def f():\n    pass\n") },
				exit: 3, stderrHas: []string{"untracked file rootnew.py: not queried", "none could be queried"}, stdoutNone: true, where: "subdir"},
			{name: "subdir-root-edit", change: func(w func(string, string), _ func(...string)) {
				w("web/lib.ts", "export function add(a: number, b: number): number {\n  return b + a;\n}\n")
			}, exit: 0, stdoutHas: `"path": "web/lib.ts"`, where: "subdir"},
			// A change with no line hunk (mode only, an empty file) is named, never "no changes".
			{name: "mode-only", change: func(_ func(string, string), g func(...string)) {
				g("config", "core.fileMode", "false")
				g("update-index", "--chmod=+x", "util.py")
			}, exit: 3, stderrHas: []string{"no changed lines", "none could be queried"}, stdoutNone: true},
			{name: "empty-file-deleted", change: func(w func(string, string), g func(...string)) {
				w("empty.py", "")
				g("add", "empty.py")
				g("commit", "-q", "-m", "empty")
				g("rm", "-q", "empty.py")
			}, exit: 3, stderrHas: []string{"no changed lines", "none could be queried"}, stdoutNone: true},
			// A textconv driver must not shift the queried lines.
			{name: "textconv-driver", change: func(w func(string, string), g func(...string)) {
				w(".gitattributes", "*.py diff=up\n")
				g("config", "diff.up.textconv", "awk 'BEGIN { for (i = 0; i < 7; i++) print \"x\" } { print }'")
				g("add", ".gitattributes")
				g("commit", "-q", "-m", "attrs")
				w("util.py", "def helper(x):\n    # identity\n    return x + 0\n")
			}, exit: 0, stdoutHas: `"status": "found"`},
			// A no-hunk block before an edited file is still named.
			{name: "mode-only-then-edit", change: func(w func(string, string), g func(...string)) {
				g("config", "core.fileMode", "false")
				g("update-index", "--chmod=+x", "util.py")
				w("web/lib.ts", "export function add(a: number, b: number): number {\n  return b + a;\n}\n")
			}, exit: 0, stdoutHas: `"path": "web/lib.ts"`, stderrHas: []string{"no changed lines: diff --git a/util.py b/util.py"}},
			// A user's diff.submodule setting must not hide a submodule change.
			{name: "submodule-log-config", change: func(w func(string, string), g func(...string)) {
				g("init", "-q", "subsrc")
				w("subsrc/f.txt", "a\n")
				g("-C", "subsrc", "add", "f.txt")
				g("-C", "subsrc", "commit", "-q", "-m", "s")
				w(".git/info/exclude", "subsrc\n")
				g("-c", "protocol.file.allow=always", "submodule", "add", "-q", "./subsrc", "mod")
				g("commit", "-q", "-m", "sub")
				w("mod/f.txt", "b\n")
				g("config", "diff.submodule", "log")
			}, exit: 1, stdoutHas: `"path": "mod"`},
			// A failed sed must not read as "no changes", with or without untracked files.
			{name: "sed-fails-with-untracked", change: func(w func(string, string), _ func(...string)) {
				w("new.py", "def f():\n    pass\n")
			}, exit: 2, stderrHas: []string{"sed failed"}, stdoutNone: true, where: "sedfail"},
			{name: "sed-fails", change: func(w func(string, string), _ func(...string)) {
				w("web/lib.ts", "export function add(a: number, b: number): number {\n  return b + a;\n}\n")
			}, exit: 2, stderrHas: []string{"sed failed"}, stdoutNone: true, where: "sedfail"},
			// A failed awk must not read as "no changes".
			{name: "awk-fails", change: func(w func(string, string), _ func(...string)) {
				w("web/lib.ts", "export function add(a: number, b: number): number {\n  return b + a;\n}\n")
			}, exit: 2, stderrHas: []string{"awk failed"}, stdoutNone: true, where: "awkfail"},
			{name: "bad-revision", change: func(func(string, string), func(...string)) {}, exit: 2, stderrHas: []string{"not a commit"}, stdoutNone: true, base: "no-such-rev"},
			{name: "not-a-repo", change: func(func(string, string), func(...string)) {}, exit: 2, stderrHas: []string{"not a commit"}, stdoutNone: true, where: "notrepo"},
			{name: "unborn-head", change: func(func(string, string), func(...string)) {}, exit: 2, stderrHas: []string{"not a commit"}, stdoutNone: true, where: "unborn"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				dir, write, git := cookbookBase(t)
				switch c.where {
				case "notrepo", "unborn":
					dir = t.TempDir()
					t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
					if c.where == "unborn" {
						if _, e, code := runIn(dir, "git", "init", "-q"); code != 0 {
							t.Fatalf("git init: %s", e)
						}
					}
				default:
					c.change(write, git)
				}
				runDir := dir
				switch c.where {
				case "subdir":
					runDir = filepath.Join(dir, "pkg")
				case "awkfail", "sedfail":
					shim := t.TempDir()
					if err := os.WriteFile(filepath.Join(shim, strings.TrimSuffix(c.where, "fail")), []byte("#!/bin/sh\nexit 2\n"), 0o755); err != nil {
						t.Fatal(err)
					}
					t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
				}
				base := c.base
				if base == "" {
					base = "HEAD"
				}
				out, errOut, code := runIn(runDir, "sh", "-c", script, "fathom-lang-diff", base)
				if code != c.exit {
					t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", code, c.exit, out, errOut)
				}
				if c.stdoutNone && out != "" {
					t.Errorf("stdout should be empty, got %q", out)
				}
				if c.exit == 2 && strings.Contains(errOut, "no changes") {
					t.Errorf("a git failure printed %q: %q", "no changes", errOut)
				}
				if c.stdoutHas != "" && !strings.Contains(out, c.stdoutHas) {
					t.Errorf("stdout lacks %q: %s", c.stdoutHas, out)
				}
				for _, h := range c.stderrHas {
					if !strings.Contains(errOut, h) {
						t.Errorf("stderr lacks %q: %q", h, errOut)
					}
				}
				if c.name == "non-ascii-and-dash-paths" {
					d := decodeAt(t, out)
					if len(d.Queries) != 2 {
						t.Fatalf("want 2 queries, got %+v", d.Queries)
					}
					for _, q := range d.Queries {
						if q.Status != "found" {
							t.Errorf("%s: status %s, want found", q.Path, q.Status)
						}
					}
				}
			})
		}
	})

	// A flag-like value is data to the tools that take paths: the template puts "--" before it. Each placeholder
	// must follow a literal "--", or be the value of --rules, or be the argument of the script (which rejects a
	// leading dash itself).
	t.Run("tools-put-dashdash-before-values", func(t *testing.T) {
		for _, tool := range tools.Tools {
			script := len(tool.Command) > 1 && tool.Command[0] == "sh" && tool.Command[1] == "-c"
			dashdash := false
			for i, el := range tool.Command {
				if el == "--" {
					dashdash = true
				}
				if placeholder.MatchString(el) && !dashdash && !script && !(i > 0 && tool.Command[i-1] == "--rules") {
					t.Errorf("tool %s: placeholder %s at %d is not after a literal -- element: %v", tool.Name, el, i, tool.Command)
				}
			}
		}
	})
	// The flag-parsing claim itself, on the binary.
	t.Run("tools-stop-flag-parsing", func(t *testing.T) {
		out, _, code := runIn(root, "fathom-lang", "outline", "--", "-not-a-flag.py")
		if code == 2 || !strings.Contains(out, "-not-a-flag.py") {
			t.Errorf("fathom-lang outline -- -not-a-flag.py: exit %d, %q (want the path read as a path, not a flag)", code, out)
		}
	})

	t.Run("guide-is-indexed-and-names-the-tools", func(t *testing.T) {
		idx, err := os.ReadFile(filepath.Join("..", "..", "docs", "use-cases", "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(idx), "](library-agent-cookbook.md)") {
			t.Errorf("docs/use-cases/README.md does not link library-agent-cookbook.md")
		}
		guide, _ := os.ReadFile(cookbookDoc)
		if !strings.Contains(string(guide), "examples/lang/agenttools/tools.json") {
			t.Errorf("the cookbook does not point at examples/lang/agenttools/tools.json")
		}
	})
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
