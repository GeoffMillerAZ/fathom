package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/encoding/jsonschema"
	_ "modernc.org/sqlite"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

func runCmd(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errw bytes.Buffer
	code = run(context.Background(), args, strings.NewReader(""), &out, &errw)
	return out.String(), errw.String(), code
}

var goldenSrc = filepath.Join("testdata", "golden", "src")
var goldenFiles = []string{"main.tf", "a.go", "blob.bin", "misc.yaml"}

// levelCompact matches a measured level in rows output, which is compact JSON.
var levelCompact = regexp.MustCompile(`"level":"(L[1-4]|none)"`)

// HLP-3 (CH-D6): the shape of extract -rows is pinned by a binary golden. A level is what a reading measured, so
// the golden pins the field and not the state of a reading, as the extract golden does.
// Regenerate deliberately: go test ./cmd/fathom-lang -run TestRowsJSONGolden -args -update-golden
func TestRowsJSONGolden(t *testing.T) {
	args := append([]string{"extract", "-rows", "-root", goldenSrc}, goldenFiles...)
	out, errw, code := runCmd(t, args...)
	if code != 0 {
		t.Fatalf("extract -rows exit %d: %s", code, errw)
	}
	checkGolden(t, "rows.jsonl", levelCompact.ReplaceAll([]byte(out), []byte(`"level":"<level>"`)))

	inProc, _, code := runCmd(t, append([]string{"extract", "-rows", "-in-process", "-root", goldenSrc}, goldenFiles...)...)
	if code != 0 || inProc != out {
		t.Errorf("supervised and -in-process rows differ (exit %d)\n--- supervised\n%s\n--- in process\n%s", code, out, inProc)
	}
	if again, _, _ := runCmd(t, args...); again != out {
		t.Error("equal inputs gave different bytes")
	}
}

// -rows re-shapes the nested output of the same run: the lines are the header and view.Rows of each file in order.
func TestRowsEqualViewRowsOfNested(t *testing.T) {
	nested, _, code := runCmd(t, append([]string{"extract", "-root", goldenSrc}, goldenFiles...)...)
	if code != 0 {
		t.Fatal(code)
	}
	var doc struct {
		SchemaVersion int `json:"schema_version"`
		Files         []struct {
			Path   string       `json:"path"`
			Error  string       `json:"error"`
			Result *lang.Result `json:"result"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(nested), &doc); err != nil {
		t.Fatal(err)
	}
	rows := []view.Row{view.Header(doc.SchemaVersion)}
	for _, f := range doc.Files {
		var r lang.Result
		if f.Result != nil {
			r = *f.Result
		}
		rows = append(rows, view.Rows(f.Path, f.Error, r)...)
	}
	var want bytes.Buffer
	if err := view.WriteJSONLines(&want, rows); err != nil {
		t.Fatal(err)
	}
	got, _, code := runCmd(t, append([]string{"extract", "-rows", "-root", goldenSrc}, goldenFiles...)...)
	if code != 0 || got != want.String() {
		t.Errorf("-rows output is not the header and view.Rows of the nested document (exit %d)\n--- want\n%s\n--- got\n%s", code, want.String(), got)
	}
}

// Same reads, flags and exit codes: a file that cannot be read exits 1 in both modes, and its error is in a file row.
func TestRowsUnreadableFileAndExitCodes(t *testing.T) {
	out, _, code := runCmd(t, "extract", "-rows", "-root", goldenSrc, "a.go", "missing.go")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatal(err)
	}
	if last["row"] != "file" || last["path"] != "missing.go" || last["has_result"] != false || last["error"] == "" {
		t.Errorf("last row = %v, want the file row of missing.go with an error and no result", last)
	}
	if _, _, code := runCmd(t, "extract", "-rows"); code != 2 {
		t.Errorf("-rows without PATH exit %d, want 2", code)
	}
	if _, _, code := runCmd(t, "extract", "-rows", "-deadline", "0s", "a.go"); code != 2 {
		t.Errorf("-rows with a bad -deadline exit %d, want 2", code)
	}
}

// Without -rows the output is the nested document, unchanged (CH-D2).
func TestExtractWithoutRowsIsNested(t *testing.T) {
	out, _, code := runCmd(t, append([]string{"extract", "-root", goldenSrc}, goldenFiles...)...)
	if code != 0 || !strings.HasPrefix(out, "{\n  \"schema_version\": 1,\n  \"files\": [") || strings.Contains(out, `"row"`) {
		t.Errorf("extract without -rows changed (exit %d):\n%.200s", code, out)
	}
}

var schemaNames = []string{"extract", "rows", "at"}

// HLP-3: the schemas the binary embeds are pinned by binary goldens and equal the published copies.
func TestSchemaSubcommandGoldensAndPublishedCopies(t *testing.T) {
	for _, name := range schemaNames {
		out, errw, code := runCmd(t, "schema", name)
		if code != 0 {
			t.Fatalf("schema %s exit %d: %s", name, code, errw)
		}
		checkGolden(t, "schema-"+name+".json", []byte(out))
		if !json.Valid([]byte(out)) {
			t.Errorf("schema %s is not JSON", name)
		}
		var doc map[string]any
		json.Unmarshal([]byte(out), &doc)
		if doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("schema %s declares %v, want draft 2020-12", name, doc["$schema"])
		}
		file := name + ".schema.json"
		beside, err := os.ReadFile(filepath.Join("schemas", file))
		if err != nil {
			t.Fatal(err)
		}
		published, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", "fathom-lang", file))
		if err != nil {
			t.Fatalf("the schema is not published: %v", err)
		}
		if !bytes.Equal(beside, published) {
			t.Errorf("%s: the copy under docs/schema/fathom-lang differs from the source beside the command", file)
		}
		if string(beside) != out {
			t.Errorf("%s: the embedded schema differs from the source beside the command", file)
		}
	}
	entries, err := os.ReadDir(filepath.Join("..", "..", "docs", "schema", "fathom-lang"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(schemaNames) {
		t.Errorf("docs/schema/fathom-lang holds %d files, want %d: a published schema has no source beside the command", len(entries), len(schemaNames))
	}
}

func TestSchemaSubcommandMisuseAndOutline(t *testing.T) {
	if _, _, code := runCmd(t, "schema"); code != 2 {
		t.Errorf("schema with no name exit %d, want 2", code)
	}
	if _, _, code := runCmd(t, "schema", "nope"); code != 2 {
		t.Errorf("schema nope exit %d, want 2", code)
	}
	if _, _, code := runCmd(t, "schema", "extract", "rows"); code != 2 {
		t.Errorf("schema with two names exit %d, want 2", code)
	}
	out, errw, code := runCmd(t, "schema", "outline")
	if code != 1 || out != "" || !strings.Contains(errw, "not yet available") {
		t.Errorf("schema outline = exit %d, stdout %q, stderr %q, want exit 1 and a not yet available error", code, out, errw)
	}
}

// CH-D6: the usage text names the new surface and marks it provisional.
func TestRowsUsageMarksProvisional(t *testing.T) {
	out, _, _ := runCmd(t, "help")
	for _, want := range []string{"-rows", "fathom-lang schema [extract|rows|at|outline]"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage lacks %q", want)
		}
	}
	for _, key := range []string{"extract -rows", "fathom-lang schema (provisional"} {
		i := strings.Index(out, key)
		if i < 0 {
			t.Errorf("usage has no paragraph starting %q", key)
			continue
		}
		window := out[i:min(len(out), i+700)]
		if !strings.Contains(strings.ToLower(window), "provisional") {
			t.Errorf("usage does not mark %q as provisional", key)
		}
	}
}

// validator compiles a JSON Schema (draft 2020-12) with the CUE module pinned in go.mod and checks one JSON
// document against it.
func validator(t *testing.T, name string) func(doc []byte) error {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("schemas", name+".schema.json"))
	if err != nil {
		t.Fatal(err)
	}
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

func validateRows(check func([]byte) error, jsonl string) error {
	for n, line := range strings.Split(strings.TrimSuffix(jsonl, "\n"), "\n") {
		if err := check([]byte(line)); err != nil {
			return fmt.Errorf("line %d: %w", n+1, err)
		}
	}
	return nil
}

// HLP-3: every golden output validates against its schema.
func TestSchemasAcceptGoldens(t *testing.T) {
	golden := filepath.Join("testdata", "golden")
	for _, c := range []struct{ schema, file string }{{"extract", "extract.json"}, {"at", "at.json"}} {
		b, err := os.ReadFile(filepath.Join(golden, c.file))
		if err != nil {
			t.Fatal(err)
		}
		if err := validator(t, c.schema)(b); err != nil {
			t.Errorf("%s does not validate against the %s schema: %v", c.file, c.schema, err)
		}
	}
	rows, _, code := runCmd(t, append([]string{"extract", "-rows", "-root", goldenSrc}, goldenFiles...)...)
	if code != 0 {
		t.Fatal(code)
	}
	if err := validateRows(validator(t, "rows"), rows); err != nil {
		t.Errorf("-rows output does not validate against the rows schema: %v", err)
	}
	// An unreadable file's row validates too.
	rows, _, _ = runCmd(t, "extract", "-rows", "-root", goldenSrc, "missing.go")
	if err := validateRows(validator(t, "rows"), rows); err != nil {
		t.Errorf("rows of an unreadable file do not validate: %v", err)
	}
}

// HLP-3 valid and invalid pairs (schema/AGENTS.md): testdata/schema/valid/<shape>.<case>.json is accepted and
// testdata/schema/invalid/<shape>.<case>.json is rejected by the schema its <shape> names. Each invalid fixture
// shape has a valid twin.
func TestSchemaFixturePairs(t *testing.T) {
	for _, dir := range []string{"valid", "invalid"} {
		files, err := filepath.Glob(filepath.Join("testdata", "schema", dir, "*.json"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no %s fixtures: %v", dir, err)
		}
		for _, f := range files {
			shape, _, _ := strings.Cut(filepath.Base(f), ".")
			doc, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			err = validator(t, shape)(doc)
			if dir == "valid" && err != nil {
				t.Errorf("%s: rejected: %v", f, err)
			}
			if dir == "invalid" && err == nil {
				t.Errorf("%s: accepted, want rejected", f)
			}
		}
	}
	// The faults the row says the schema rejects are all present.
	for _, want := range []string{"rows.missing-required-field.json", "rows.wrong-type.json", "rows.unknown-row-kind.json", "extract.missing-required-field.json", "extract.wrong-type.json", "at.missing-required-field.json", "at.wrong-type.json"} {
		if _, err := os.Stat(filepath.Join("testdata", "schema", "invalid", want)); err != nil {
			t.Errorf("missing invalid fixture %s", want)
		}
	}
	invalid, _ := filepath.Glob(filepath.Join("testdata", "schema", "invalid", "*.json"))
	for _, f := range invalid {
		shape, _, _ := strings.Cut(filepath.Base(f), ".")
		twins, _ := filepath.Glob(filepath.Join("testdata", "schema", "valid", shape+".*.json"))
		if len(twins) == 0 {
			t.Errorf("%s has no valid twin", f)
		}
	}
}

func jsonTags(v any) []string {
	var out []string
	rt := reflect.TypeOf(v)
	for i := 0; i < rt.NumField(); i++ {
		tag := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]
		if tag != "" && tag != "-" {
			out = append(out, tag)
		}
	}
	return out
}

func loadSchemaDoc(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("schemas", name+".schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func defOf(t *testing.T, doc map[string]any, name string) (props map[string]any, required []string) {
	t.Helper()
	defs, _ := doc["$defs"].(map[string]any)
	d, _ := defs[name].(map[string]any)
	if d == nil {
		t.Fatalf("the schema has no $defs.%s", name)
	}
	props, _ = d["properties"].(map[string]any)
	req, _ := d["required"].([]any)
	for _, r := range req {
		required = append(required, r.(string))
	}
	if d["additionalProperties"] != false {
		t.Errorf("$defs.%s does not close its properties with additionalProperties false", name)
	}
	return props, required
}

// HLP-3 drift guard, schema side: every JSON tag of Result, Symbol, Reference, Unresolved and Fact is a required
// property of the schemas, in the nested shape and in the rows. A new field fails here until a schema carries it.
func TestSchemasCoverEveryJSONTag(t *testing.T) {
	type shape struct {
		def  string
		tags []string
	}
	resultScalars := []string{"dialect", "language", "skipped", "level", "incomplete"}
	if got := jsonTags(lang.Result{}); !reflect.DeepEqual(got, []string{"dialect", "language", "skipped", "symbols", "references", "unresolved", "facts", "level", "incomplete", "notes"}) {
		t.Errorf("lang.Result's JSON tags changed to %v: extend the schemas, the rows and this guard", got)
	}
	check := func(which string, doc map[string]any, shapes []shape) {
		for _, s := range shapes {
			props, required := defOf(t, doc, s.def)
			for _, tag := range s.tags {
				if _, ok := props[tag]; !ok {
					t.Errorf("%s schema $defs.%s lacks property %q", which, s.def, tag)
				}
				if !contains(required, tag) {
					t.Errorf("%s schema $defs.%s does not require %q", which, s.def, tag)
				}
			}
			if len(props) != len(s.tags) {
				t.Errorf("%s schema $defs.%s has %d properties, want %d", which, s.def, len(props), len(s.tags))
			}
		}
	}
	check("extract", loadSchemaDoc(t, "extract"), []shape{
		{"Result", jsonTags(lang.Result{})},
		{"Symbol", jsonTags(lang.Symbol{})},
		{"Reference", jsonTags(lang.Reference{})},
		{"Unresolved", jsonTags(lang.Unresolved{})},
		{"Fact", jsonTags(lang.Fact{})},
	})
	rowKey := []string{"row", "path", "i"}
	check("rows", loadSchemaDoc(t, "rows"), []shape{
		{"SymbolRow", append(append([]string{}, rowKey...), jsonTags(lang.Symbol{})...)},
		{"ReferenceRow", append(append([]string{}, rowKey...), jsonTags(lang.Reference{})...)},
		{"UnresolvedRow", append(append([]string{}, rowKey...), jsonTags(lang.Unresolved{})...)},
		{"FactRow", append(append([]string{}, rowKey...), jsonTags(lang.Fact{})...)},
		{"NoteRow", []string{"row", "path", "i", "note"}},
		{"HeaderRow", []string{"row", "rows_version", "schema_version"}},
		{"FileRow", append(append([]string{"row", "path", "error"}, resultScalars...), "has_result", "null_arrays")},
	})
	// The at schema carries lang.Symbol for its enclosing and touched symbols.
	check("at", loadSchemaDoc(t, "at"), []shape{{"Symbol", jsonTags(lang.Symbol{})}})
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// The SQL example of the row contract. One executed example, in an in-memory SQLite database loaded from the rows
// through the driver already in go.mod, from a test outside the door's closure.
const (
	sqlSymbolsPerKind = `SELECT path, canonical, COUNT(*) FROM symbol GROUP BY path, canonical ORDER BY path, canonical`
	sqlNoInFileTarget = `SELECT r.path, r."from", r."to", r.line FROM reference r LEFT JOIN symbol s ON s.path = r.path AND s.key = r.to_key WHERE s.key IS NULL ORDER BY r.path, r.i`
)

func loadRows(t *testing.T, db *sql.DB, jsonl string) {
	t.Helper()
	for _, ddl := range []string{
		`CREATE TABLE file (path TEXT, error TEXT, dialect TEXT, language TEXT, skipped TEXT, level TEXT, incomplete INTEGER, has_result INTEGER)`,
		`CREATE TABLE symbol (path TEXT, i INTEGER, key TEXT, name TEXT, package TEXT, kind TEXT, canonical TEXT, line INTEGER, end_line INTEGER, signature TEXT, tier TEXT)`,
		`CREATE TABLE reference (path TEXT, i INTEGER, "from" TEXT, "to" TEXT, to_key TEXT, edge TEXT, raw TEXT, strategy TEXT, line INTEGER)`,
		`CREATE TABLE unresolved (path TEXT, i INTEGER, "from" TEXT, edge TEXT, raw TEXT, reason TEXT, line INTEGER)`,
		`CREATE TABLE fact (path TEXT, i INTEGER, entity TEXT, key TEXT, value TEXT, line INTEGER)`,
		`CREATE TABLE note (path TEXT, i INTEGER, note TEXT)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	cols := map[string][]string{
		"file":       {"path", "error", "dialect", "language", "skipped", "level", "incomplete", "has_result"},
		"symbol":     {"path", "i", "key", "name", "package", "kind", "canonical", "line", "end_line", "signature", "tier"},
		"reference":  {"path", "i", "from", "to", "to_key", "edge", "raw", "strategy", "line"},
		"unresolved": {"path", "i", "from", "edge", "raw", "reason", "line"},
		"fact":       {"path", "i", "entity", "key", "value", "line"},
		"note":       {"path", "i", "note"},
	}
	for _, line := range strings.Split(strings.TrimSuffix(jsonl, "\n"), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatal(err)
		}
		kind := m["row"].(string)
		cs, ok := cols[kind]
		if !ok {
			continue // the header
		}
		args := make([]any, len(cs))
		marks := make([]string, len(cs))
		quoted := make([]string, len(cs))
		for i, c := range cs {
			v := m[c]
			if b, ok := v.(bool); ok {
				if b {
					v = 1
				} else {
					v = 0
				}
			}
			args[i], marks[i], quoted[i] = v, "?", `"`+c+`"`
		}
		if _, err := db.Exec(`INSERT INTO `+kind+` (`+strings.Join(quoted, ",")+`) VALUES (`+strings.Join(marks, ",")+`)`, args...); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRowsLoadIntoSQLite(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("x.go", "package x\n\nimport \"fmt\"\n\ntype T struct{}\n\nfunc A() {\n\tB()\n\tfmt.Println()\n}\n\nfunc B() {}\n")
	write("y.go", "package x\n\nfunc C() { A() }\n")
	files := []string{"x.go", "y.go"}
	jsonl, errw, code := runCmd(t, append([]string{"extract", "-rows", "-root", root}, files...)...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errw)
	}
	nestedOut, _, _ := runCmd(t, append([]string{"extract", "-root", root}, files...)...)
	var doc struct {
		Files []struct {
			Path   string      `json:"path"`
			Result lang.Result `json:"result"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(nestedOut), &doc); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	loadRows(t, db, jsonl)

	// Symbols per canonical kind per file.
	type kc struct {
		path, canonical string
		n               int
	}
	var want, got []kc
	for _, f := range doc.Files {
		counts := map[string]int{}
		for _, s := range f.Result.Symbols {
			counts[s.Canonical]++
		}
		for c, n := range counts {
			want = append(want, kc{f.Path, c, n})
		}
	}
	sort.Slice(want, func(i, j int) bool {
		if want[i].path != want[j].path {
			return want[i].path < want[j].path
		}
		return want[i].canonical < want[j].canonical
	})
	rs, err := db.Query(sqlSymbolsPerKind)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var k kc
		if err := rs.Scan(&k.path, &k.canonical, &k.n); err != nil {
			t.Fatal(err)
		}
		got = append(got, k)
	}
	rs.Close()
	if len(want) < 3 || !reflect.DeepEqual(got, want) {
		t.Errorf("symbols per canonical kind per file = %v, want %v (and at least 3 groups)", got, want)
	}

	// References with no in-file target: no symbol of the same file has the reference's to_key.
	type nt struct {
		path, from, to string
		line           int
	}
	var wantNT, gotNT []nt
	for _, f := range doc.Files {
		keys := map[string]bool{}
		for _, s := range f.Result.Symbols {
			keys[s.Key] = true
		}
		for _, r := range f.Result.References {
			if !keys[r.ToKey] {
				wantNT = append(wantNT, nt{f.Path, r.From, r.To, r.Line})
			}
		}
	}
	rs, err = db.Query(sqlNoInFileTarget)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var k nt
		if err := rs.Scan(&k.path, &k.from, &k.to, &k.line); err != nil {
			t.Fatal(err)
		}
		gotNT = append(gotNT, k)
	}
	rs.Close()
	if len(wantNT) == 0 || !reflect.DeepEqual(gotNT, wantNT) {
		t.Errorf("references with no in-file target = %v, want %v (and at least one)", gotNT, wantNT)
	}
	// The example is not vacuous on the other side either: some reference does have an in-file target.
	var inFile int
	if err := db.QueryRow(`SELECT COUNT(*) FROM reference r JOIN symbol s ON s.path = r.path AND s.key = r.to_key`).Scan(&inFile); err != nil || inFile == 0 {
		t.Errorf("no reference has an in-file target (%d, %v): the fixture proves nothing", inFile, err)
	}
}

// The queries the test executes are the ones the use-case guide shows.
func TestSQLExampleIsInTheGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "use-cases", "lang-library.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{sqlSymbolsPerKind, sqlNoInFileTarget} {
		if !strings.Contains(string(b), q) {
			t.Errorf("docs/use-cases/lang-library.md does not show the executed query:\n%s", q)
		}
	}
}
