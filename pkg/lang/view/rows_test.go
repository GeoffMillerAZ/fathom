package view_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

// nestedEntry is one file of the nested extract document.
type nestedEntry struct {
	Path   string       `json:"path"`
	Error  string       `json:"error"`
	Result *lang.Result `json:"result"`
}

// encodeNested prints the nested extract document the way fathom-lang extract does.
func encodeNested(t testing.TB, version int, files []nestedEntry) []byte {
	t.Helper()
	if files == nil {
		files = []nestedEntry{}
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(struct {
		SchemaVersion int           `json:"schema_version"`
		Files         []nestedEntry `json:"files"`
	}{version, files}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// rowsOf is the export of a nested document: the header, then Rows of each file.
func rowsOf(version int, files []nestedEntry) []view.Row {
	rows := []view.Row{view.Header(version)}
	for _, f := range files {
		var r lang.Result
		if f.Result != nil {
			r = *f.Result
		}
		rows = append(rows, view.Rows(f.Path, f.Error, r)...)
	}
	return rows
}

func linesOf(t testing.TB, rows []view.Row) []string {
	t.Helper()
	var b bytes.Buffer
	if err := view.WriteJSONLines(&b, rows); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if out == "" || !strings.HasSuffix(out, "\n") {
		t.Fatalf("lines %q do not end in a newline", out)
	}
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

// rebuildNested is the test-tree inverse of the export: it rebuilds the nested document from JSON Lines
// and nothing else. It decodes each row into the lang struct it mirrors.
func rebuildNested(t testing.TB, lines []string) []byte {
	t.Helper()
	var version int
	var files []nestedEntry
	var null map[string]bool
	flush := func() {
		if len(files) == 0 {
			return
		}
		e := &files[len(files)-1]
		if e.Result == nil {
			return
		}
		r := e.Result
		if null["symbols"] {
			r.Symbols = nil
		}
		if null["references"] {
			r.References = nil
		}
		if null["unresolved"] {
			r.Unresolved = nil
		}
		if null["facts"] {
			r.Facts = nil
		}
		if null["notes"] {
			r.Notes = nil
		}
	}
	for n, line := range lines {
		var head struct {
			Row string `json:"row"`
			I   *int   `json:"i"`
		}
		if err := json.Unmarshal([]byte(line), &head); err != nil {
			t.Fatalf("line %d: %v", n, err)
		}
		wantI := func(have int) int {
			if head.I == nil || *head.I != have {
				t.Fatalf("line %d: i is %v, want %d", n, head.I, have)
			}
			return have
		}
		var cur *lang.Result
		if len(files) > 0 {
			cur = files[len(files)-1].Result
		}
		switch head.Row {
		case "header":
			var h struct {
				SchemaVersion int `json:"schema_version"`
			}
			if err := json.Unmarshal([]byte(line), &h); err != nil {
				t.Fatal(err)
			}
			version = h.SchemaVersion
		case "file":
			flush()
			var f struct {
				Path       string   `json:"path"`
				Error      string   `json:"error"`
				HasResult  bool     `json:"has_result"`
				Dialect    string   `json:"dialect"`
				Language   string   `json:"language"`
				Skipped    string   `json:"skipped"`
				Level      string   `json:"level"`
				Incomplete bool     `json:"incomplete"`
				NullArrays []string `json:"null_arrays"`
			}
			if err := json.Unmarshal([]byte(line), &f); err != nil {
				t.Fatal(err)
			}
			e := nestedEntry{Path: f.Path, Error: f.Error}
			if f.HasResult {
				e.Result = &lang.Result{Dialect: f.Dialect, Language: f.Language, Skipped: f.Skipped, Level: f.Level, Incomplete: f.Incomplete,
					Symbols: []lang.Symbol{}, References: []lang.Reference{}, Unresolved: []lang.Unresolved{}, Facts: []lang.Fact{}, Notes: []string{}}
			}
			null = map[string]bool{}
			for _, a := range f.NullArrays {
				null[a] = true
			}
			files = append(files, e)
		case "symbol":
			var s lang.Symbol
			json.Unmarshal([]byte(line), &s)
			cur.Symbols = append(cur.Symbols, s)
			wantI(len(cur.Symbols) - 1)
		case "reference":
			var s lang.Reference
			json.Unmarshal([]byte(line), &s)
			cur.References = append(cur.References, s)
			wantI(len(cur.References) - 1)
		case "unresolved":
			var s lang.Unresolved
			json.Unmarshal([]byte(line), &s)
			cur.Unresolved = append(cur.Unresolved, s)
			wantI(len(cur.Unresolved) - 1)
		case "fact":
			var s lang.Fact
			json.Unmarshal([]byte(line), &s)
			cur.Facts = append(cur.Facts, s)
			wantI(len(cur.Facts) - 1)
		case "note":
			var s struct {
				Note string `json:"note"`
			}
			json.Unmarshal([]byte(line), &s)
			cur.Notes = append(cur.Notes, s.Note)
			wantI(len(cur.Notes) - 1)
		default:
			t.Fatalf("line %d: unknown row kind %q", n, head.Row)
		}
	}
	flush()
	return encodeNested(t, version, files)
}

// checkLossless exports files, rebuilds the document from the lines and requires the same bytes.
func checkLossless(t testing.TB, name string, version int, files []nestedEntry) []view.Row {
	t.Helper()
	rows := rowsOf(version, files)
	want := encodeNested(t, version, files)
	got := rebuildNested(t, linesOf(t, rows))
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: the document rebuilt from rows differs from the nested one\n--- nested\n%s\n--- rebuilt\n%s", name, want, got)
	}
	return rows
}

func sampleResult() lang.Result {
	return lang.Result{
		Dialect: "go", Language: "go", Level: "L3",
		Symbols: []lang.Symbol{
			{Key: "a.F", Name: "F", Package: "a", Kind: "func", Canonical: "function", Line: 3, EndLine: 5, Signature: "func F()", Tier: "verified"},
			{Key: "a.G", Name: "G", Package: "a", Kind: "func", Canonical: "function", Line: 7},
		},
		References: []lang.Reference{{From: "a.F", To: "G", ToKey: "a.G", Edge: "calls", Raw: "G()", Strategy: "local", Line: 4}},
		Unresolved: []lang.Unresolved{{From: "a.F", Edge: "calls", Raw: "x.Y()", Reason: "no-target", Line: 5}},
		Facts:      []lang.Fact{{Entity: "a.F", Key: "exported", Value: "true", Line: 3}},
		Notes:      []string{"a note"},
	}
}

// HLP-3 (CH-D6): the shape of every row kind, its fields and their order.
func TestRowsShape(t *testing.T) {
	r := sampleResult()
	got := linesOf(t, append([]view.Row{view.Header(1)}, view.Rows("a.go", "", r)...))
	want := []string{
		`{"row":"header","rows_version":1,"schema_version":1}`,
		`{"row":"file","path":"a.go","error":"","dialect":"go","language":"go","skipped":"","level":"L3","incomplete":false,"has_result":true,"null_arrays":[]}`,
		`{"row":"symbol","path":"a.go","i":0,"key":"a.F","name":"F","package":"a","kind":"func","canonical":"function","line":3,"end_line":5,"signature":"func F()","tier":"verified"}`,
		`{"row":"symbol","path":"a.go","i":1,"key":"a.G","name":"G","package":"a","kind":"func","canonical":"function","line":7,"end_line":0,"signature":"","tier":""}`,
		`{"row":"reference","path":"a.go","i":0,"from":"a.F","to":"G","to_key":"a.G","edge":"calls","raw":"G()","strategy":"local","line":4}`,
		`{"row":"unresolved","path":"a.go","i":0,"from":"a.F","edge":"calls","raw":"x.Y()","reason":"no-target","line":5}`,
		`{"row":"fact","path":"a.go","i":0,"entity":"a.F","key":"exported","value":"true","line":3}`,
		`{"row":"note","path":"a.go","i":0,"note":"a note"}`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rows differ\n got: %s\nwant: %s", strings.Join(got, "\n      "), strings.Join(want, "\n      "))
	}
}

func TestRowsUnreadableAndNullArrays(t *testing.T) {
	got := linesOf(t, view.Rows("gone.go", "open gone.go: no such file", lang.Result{}))
	want := `{"row":"file","path":"gone.go","error":"open gone.go: no such file","dialect":"","language":"","skipped":"","level":"","incomplete":false,"has_result":false,"null_arrays":[]}`
	if len(got) != 1 || got[0] != want {
		t.Errorf("unreadable file rows = %q, want one row %s", got, want)
	}
	got = linesOf(t, view.Rows("b.bin", "", lang.Result{Language: "binary", Skipped: "binary"}))
	if !strings.Contains(got[0], `"has_result":true`) || !strings.Contains(got[0], `"null_arrays":["symbols","references","unresolved","facts","notes"]`) || len(got) != 1 {
		t.Errorf("a zero Result has every array null: %q", got)
	}
}

// HLP-3 lossless: synthetic documents, null against empty arrays, an unreadable file and an empty document.
func TestRowsLosslessSynthetic(t *testing.T) {
	full := sampleResult()
	empty := lang.Result{Dialect: "go", Language: "go", Level: "L3", Symbols: []lang.Symbol{}, References: []lang.Reference{}, Unresolved: []lang.Unresolved{}, Facts: []lang.Fact{}, Notes: []string{}}
	nulls := lang.Result{Language: "binary", Skipped: "binary"}
	mixed := lang.Result{Dialect: "go", Symbols: []lang.Symbol{{Key: "k"}}, Facts: []lang.Fact{}, Incomplete: true, Notes: []string{"x", "y"}}
	tricky := lang.Result{Dialect: "go", Symbols: []lang.Symbol{{Key: "k\"\n<&> ", Name: "é\t\\", Signature: "<T>(a & b)"}}, Notes: []string{"line\nbreak", ""}}
	for name, files := range map[string][]nestedEntry{
		"empty document": nil,
		"full":           {{Path: "a.go", Result: &full}},
		"empty arrays":   {{Path: "e.go", Result: &empty}},
		"null arrays":    {{Path: "b.bin", Result: &nulls}},
		"mixed":          {{Path: "m.go", Result: &mixed}},
		"tricky strings": {{Path: "t go", Result: &tricky}},
		"unreadable":     {{Path: "gone.go", Error: "open gone.go: no such file or directory"}},
		"several":        {{Path: "z.go", Result: &full}, {Path: "gone.go", Error: "boom"}, {Path: "e.go", Result: &empty}, {Path: "b.bin", Result: &nulls}, {Path: "a.go", Result: &mixed}},
	} {
		checkLossless(t, name, 1, files)
	}
}

// HLP-3 lossless on the binary goldens: the extract golden's nested document, and every file under the golden
// source tree read in this process.
func TestRowsLosslessGoldens(t *testing.T) {
	golden := filepath.Join("..", "..", "..", "cmd", "fathom-lang", "testdata", "golden")
	raw, err := os.ReadFile(filepath.Join(golden, "extract.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The golden keeps a measured level as a placeholder, which an HTML-escaping encoder would print differently.
	raw = bytes.ReplaceAll(raw, []byte(`"<level>"`), []byte(`"L9"`))
	var doc struct {
		SchemaVersion int           `json:"schema_version"`
		Files         []nestedEntry `json:"files"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Files) == 0 {
		t.Fatal("the extract golden lists no files")
	}
	if !bytes.Equal(encodeNested(t, doc.SchemaVersion, doc.Files), raw) {
		t.Fatal("the test encoder does not reproduce the extract golden, so a lossless result would prove nothing")
	}
	checkLossless(t, "extract.json", doc.SchemaVersion, doc.Files)

	rd, err := lang.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()
	var files []nestedEntry
	src := filepath.Join(golden, "src")
	err = filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		res, err := rd.Extract(t.Context(), lang.File{Path: filepath.ToSlash(rel), Src: b})
		if err != nil {
			files = append(files, nestedEntry{Path: filepath.ToSlash(rel), Error: err.Error()})
			return nil
		}
		files = append(files, nestedEntry{Path: filepath.ToSlash(rel), Result: &res})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 5 {
		t.Fatalf("read only %d golden source files", len(files))
	}
	checkLossless(t, "golden src", 1, files)
}

// Equal input gives equal bytes, and Rows keeps no reference to the caller's slices.
func TestRowsDeterministicAndOwned(t *testing.T) {
	r := sampleResult()
	a := linesOf(t, view.Rows("a.go", "", r))
	b := linesOf(t, view.Rows("a.go", "", r))
	if !reflect.DeepEqual(a, b) {
		t.Error("equal input gave different rows")
	}
	rows := view.Rows("a.go", "", r)
	r.Symbols[0].Name = "changed"
	r.Notes[0] = "changed"
	if got := linesOf(t, rows); !reflect.DeepEqual(got, a) {
		t.Error("a later change to the caller's Result changed rows already built")
	}
}

// CH-D4: rows are a re-shaping of the Result's values. Every string value in every row line is a value of the
// Result, the path, the error or one of the fixed words of the shape, and a row adds no text.
func TestRowsCarryNothingTheNestedWithholds(t *testing.T) {
	r := sampleResult()
	allowed := map[string]bool{"a.go": true}
	collect := func(v any) {
		var walk func(reflect.Value)
		walk = func(v reflect.Value) {
			switch v.Kind() {
			case reflect.String:
				allowed[v.String()] = true
			case reflect.Slice, reflect.Array:
				for i := 0; i < v.Len(); i++ {
					walk(v.Index(i))
				}
			case reflect.Struct:
				for i := 0; i < v.NumField(); i++ {
					walk(v.Field(i))
				}
			}
		}
		walk(reflect.ValueOf(v))
	}
	collect(r)
	fixed := map[string]bool{"row": true, "header": true, "file": true, "symbol": true, "reference": true, "unresolved": true, "fact": true, "note": true,
		"symbols": true, "references": true, "facts": true, "notes": true}
	for _, line := range linesOf(t, view.Rows("a.go", "", r)) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatal(err)
		}
		for k, v := range m {
			if s, ok := v.(string); ok && !allowed[s] && !(k == "row" && fixed[s]) {
				t.Errorf("row %s field %s holds %q, which is not in the Result", line, k, s)
			}
		}
	}
}

// The drift guard, on the rows side: every JSON tag of Result, Symbol, Reference, Unresolved and Fact is a key of
// the row it becomes. The schema side of the guard is in cmd/fathom-lang.
func TestRowsCoverEveryJSONTag(t *testing.T) {
	r := sampleResult()
	rows := linesOf(t, view.Rows("a.go", "", r))
	keysByKind := map[string]map[string]bool{}
	for _, line := range rows {
		var m map[string]json.RawMessage
		json.Unmarshal([]byte(line), &m)
		var kind string
		json.Unmarshal(m["row"], &kind)
		keysByKind[kind] = map[string]bool{}
		for k := range m {
			keysByKind[kind][k] = true
		}
	}
	tags := func(v any) []string {
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
	check := func(kind string, v any) {
		for _, tag := range tags(v) {
			if !keysByKind[kind][tag] {
				t.Errorf("%s rows lack the JSON key %q of %T", kind, tag, v)
			}
		}
	}
	check("symbol", lang.Symbol{})
	check("reference", lang.Reference{})
	check("unresolved", lang.Unresolved{})
	check("fact", lang.Fact{})
	// Result's scalar tags are file row keys, and its arrays are the other row kinds.
	arrays := map[string]string{"symbols": "symbol", "references": "reference", "unresolved": "unresolved", "facts": "fact", "notes": "note"}
	for _, tag := range tags(lang.Result{}) {
		if kind, ok := arrays[tag]; ok {
			if keysByKind[kind] == nil {
				t.Errorf("Result.%s has no %s rows", tag, kind)
			}
			continue
		}
		if !keysByKind["file"][tag] {
			t.Errorf("file rows lack the JSON key %q of lang.Result", tag)
		}
	}
}

func TestRowsHeader(t *testing.T) {
	if view.RowsVersion != 1 {
		t.Errorf("RowsVersion = %d, want 1", view.RowsVersion)
	}
	h := linesOf(t, []view.Row{view.Header(7)})
	if h[0] != `{"row":"header","rows_version":1,"schema_version":7}` {
		t.Errorf("header = %s", h[0])
	}
	_ = fmt.Sprint
}
