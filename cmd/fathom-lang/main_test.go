package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langlevelsgen"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractPrintsJSON(t *testing.T) {
	root := t.TempDir()
	write(t, root, "infra/main.tf", "resource \"aws_s3_bucket\" \"b\" {\n}\n")
	write(t, root, "a.go", "package a\n\nfunc F() {}\n")
	var out, errw bytes.Buffer
	code := run(context.Background(), []string{"extract", "-root", root, "infra/main.tf", "a.go"}, strings.NewReader(""), &out, &errw)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errw.String())
	}
	var got struct {
		Files []struct {
			Path   string `json:"path"`
			Result struct {
				Dialect string `json:"dialect"`
				Level   string `json:"level"`
				Symbols []struct {
					Kind      string `json:"kind"`
					Canonical string `json:"canonical"`
				} `json:"symbols"`
			} `json:"result"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if len(got.Files) != 2 || got.Files[0].Path != "infra/main.tf" || got.Files[0].Result.Dialect != "terraform" ||
		got.Files[0].Result.Symbols[0].Canonical != "resource" || got.Files[1].Result.Dialect != "go" {
		t.Fatalf("%+v", got)
	}
	var again bytes.Buffer
	run(context.Background(), []string{"extract", "-root", root, "infra/main.tf", "a.go"}, strings.NewReader(""), &again, &errw)
	if !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Error("equal inputs gave different JSON")
	}
}

func TestExtractSiblings(t *testing.T) {
	root := t.TempDir()
	role := "- name: t\n  debug:\n    msg: hi\n"
	write(t, root, "r/tasks/main.yml", role)
	dialect := func(args ...string) (string, []string) {
		var out, errw bytes.Buffer
		if code := run(context.Background(), append([]string{"extract", "-root", root}, args...), strings.NewReader("r/tasks/main.yml\nr/meta/main.yml\n"), &out, &errw); code != 0 {
			t.Fatalf("exit %d: %s", code, errw.String())
		}
		var got struct {
			Files []struct {
				Result struct {
					Dialect string   `json:"dialect"`
					Notes   []string `json:"notes"`
				} `json:"result"`
			} `json:"files"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got.Files[0].Result.Dialect, got.Files[0].Result.Notes
	}
	if d, n := dialect("r/tasks/main.yml"); d == "" || len(n) != 1 {
		t.Errorf("alone: dialect %q notes %v", d, n)
	}
	if d, n := dialect("-siblings", "-", "r/tasks/main.yml"); d != "ansible" || len(n) != 0 {
		t.Errorf("-siblings: dialect %q notes %v", d, n)
	}
	if d, n := dialect("-walk", "r/tasks/main.yml"); d != "ansible" || len(n) != 0 {
		t.Errorf("-walk: dialect %q notes %v", d, n)
	}
}

func TestDialectsPrintsJSON(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"dialects"}, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("exit %d: %s", code, errw.String())
	}
	var got struct {
		Dialects []struct {
			ID, Kind, Engine, Level, Grade string
			Matches                        []string
		} `json:"dialects"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	raw, err := langlevelsgen.Generate(filepath.Join("..", "..", "docs", "bench", "readings", "lang"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []langlevelsgen.Row
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range got.Dialects {
		for _, r := range rows {
			// The level is whatever the readings say today, not a literal a regrade moves.
			found = found || d.ID == "terraform" && r.ID == "terraform" && d.Level == r.Level && d.Grade == r.Grade
		}
	}
	if !found || len(got.Dialects) < 100 {
		t.Fatalf("%d dialects, terraform found %v", len(got.Dialects), found)
	}
}

func TestUsageAndErrors(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{nil, 2},
		{[]string{"bogus"}, 2},
		{[]string{"extract"}, 2},
		{[]string{"extract", "-nonsense"}, 2},
		{[]string{"dialects", "-nonsense"}, 2},
		{[]string{"dialects", "extra"}, 2},
		{[]string{"extract", "-root", t.TempDir(), "missing.go"}, 1}, // a file that cannot be read is a failure, not misuse
		{[]string{"help"}, 0},
	}
	for _, c := range cases {
		var out, errw bytes.Buffer
		if got := run(context.Background(), c.args, strings.NewReader(""), &out, &errw); got != c.want {
			t.Errorf("%v: exit %d, want %d (%s)", c.args, got, c.want, errw.String())
		}
	}
}

// samplePack is the pack the library's conformance test passes: a doc dialect
// and a tags dialect over the thrift grammar.
const samplePack = "../../pkg/lang/langtest/testdata/samplepack"

func TestPackFlagReadsASamplePackEndToEnd(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ui/clock.widget.json", `{"name":"Clock","kind":"timer","uses":["Battery"],"imports":["parts/battery.widget.json"]}`)
	write(t, root, "idl/shop.thr", "struct Order {\n  1: i32 id\n}\n\nservice Shop {\n  void ping()\n}\n")
	write(t, root, "idl/shop.thrift", "struct Order {\n  1: i32 id\n}\n")
	var out, errw bytes.Buffer
	args := []string{"extract", "-root", root, "-pack", samplePack, "ui/clock.widget.json", "idl/shop.thr", "idl/shop.thrift"}
	if code := run(context.Background(), args, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("exit %d: %s", code, errw.String())
	}
	var got struct {
		Files []struct {
			Path   string `json:"path"`
			Result struct {
				Dialect string `json:"dialect"`
				Level   string `json:"level"`
				Skipped string `json:"skipped"`
				Symbols []struct {
					Name      string `json:"name"`
					Kind      string `json:"kind"`
					Canonical string `json:"canonical"`
				} `json:"symbols"`
				References []struct {
					To   string `json:"to"`
					Edge string `json:"edge"`
				} `json:"references"`
			} `json:"result"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if len(got.Files) != 3 {
		t.Fatalf("%d files: %s", len(got.Files), out.String())
	}
	w, s, b := got.Files[0].Result, got.Files[1].Result, got.Files[2].Result
	if w.Dialect != "widget-spec" || w.Level != "unmeasured" || len(w.Symbols) != 1 || w.Symbols[0].Name != "Clock" || w.Symbols[0].Canonical != "resource" {
		t.Errorf("widget: %+v", w)
	}
	edges := map[string]string{}
	for _, r := range w.References {
		edges[r.Edge] = r.To
	}
	if edges["imports"] != "ui/parts/battery.widget.json" || edges["uses"] != "Battery" {
		t.Errorf("widget references: %+v", w.References)
	}
	if s.Dialect != "thrift-lite" || len(s.Symbols) != 3 || s.Symbols[0].Kind != "ThriftLiteStruct" {
		t.Errorf("thrift-lite: %+v", s)
	}
	if b.Dialect != "thrift" {
		t.Errorf("a built-in file changed hands: %q", b.Dialect)
	}

	// The same files without the pack: the pack's files read as they did.
	var plain, perr bytes.Buffer
	if code := run(context.Background(), []string{"extract", "-root", root, "ui/clock.widget.json", "idl/shop.thr"}, strings.NewReader(""), &plain, &perr); code != 0 {
		t.Fatalf("exit %d: %s", code, perr.String())
	}
	if strings.Contains(plain.String(), `"widget-spec"`) || strings.Contains(plain.String(), `"thrift-lite"`) {
		t.Error("the pack's dialects read files without -pack")
	}

	out.Reset()
	if code := run(context.Background(), []string{"dialects", "-pack", samplePack}, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("dialects exit %d: %s", code, errw.String())
	}
	for _, id := range []string{`"widget-spec"`, `"thrift-lite"`} {
		if !strings.Contains(out.String(), id) {
			t.Errorf("dialects -pack lacks %s", id)
		}
	}
}

func TestPackFlagRefusesABadPackWhole(t *testing.T) {
	// Copy the sample pack and break one query: the CLI exits 1 with the reason
	// and prints no result for any file.
	dir := t.TempDir()
	for _, f := range []string{
		"dialects/widget-spec/dialect.json", "dialects/widget-spec/doc.json",
		"dialects/thrift-lite/dialect.json",
	} {
		b, err := os.ReadFile(filepath.Join(samplePack, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		write(t, dir, f, string(b))
	}
	write(t, dir, "dialects/thrift-lite/tags.scm", "(no_such_node) @name @definition.struct\n")
	write(t, dir, "x.widget.json", `{"name":"X"}`)
	var out, errw bytes.Buffer
	code := run(context.Background(), []string{"extract", "-root", dir, "-pack", dir, "x.widget.json"}, strings.NewReader(""), &out, &errw)
	if code != 1 || out.Len() != 0 {
		t.Fatalf("exit %d, stdout %q", code, out.String())
	}
	if !strings.Contains(errw.String(), "pack refused (query)") || !strings.Contains(errw.String(), "thrift-lite") {
		t.Errorf("stderr: %s", errw.String())
	}
	errw.Reset()
	if code := run(context.Background(), []string{"dialects", "-pack", filepath.Join(dir, "nowhere")}, strings.NewReader(""), &out, &errw); code != 1 || !strings.Contains(errw.String(), "pack refused (empty)") {
		t.Errorf("missing pack dir: exit %d, stderr %s", code, errw.String())
	}
}

// N7: an absolute PATH is read as given, a file that cannot be read is a result
// of its own and the batch goes on, and both commands print schema_version.
func TestExtractAbsolutePathsAndPerFileErrors(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	write(t, root, "infra/main.tf", "resource \"aws_s3_bucket\" \"b\" {\n}\n")
	write(t, other, "elsewhere/main.tf", "resource \"aws_s3_bucket\" \"c\" {\n}\n")
	write(t, root, "ok.go", "package a\n\nfunc F() {}\n")
	abs := filepath.Join(other, "elsewhere", "main.tf")
	var out, errw bytes.Buffer
	code := run(context.Background(), []string{"extract", "-root", root, filepath.Join(root, "infra", "main.tf"), abs, "missing.go", filepath.Join(other, "nope.tf"), "ok.go"}, strings.NewReader(""), &out, &errw)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (two files could not be read): %s", code, errw.String())
	}
	var got struct {
		SchemaVersion int `json:"schema_version"`
		Files         []struct {
			Path   string `json:"path"`
			Error  string `json:"error"`
			Result *struct {
				Dialect string `json:"dialect"`
			} `json:"result"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("the run printed no JSON: %v\n%s\nstderr %s", err, out.String(), errw.String())
	}
	if got.SchemaVersion != 1 || len(got.Files) != 5 {
		t.Fatalf("schema %d, %d files: %s", got.SchemaVersion, len(got.Files), out.String())
	}
	f := got.Files
	if f[0].Path != "infra/main.tf" || f[0].Error != "" || f[0].Result == nil || f[0].Result.Dialect != "terraform" {
		t.Errorf("an absolute path under -root: %+v", f[0])
	}
	if f[1].Path != filepath.ToSlash(abs) || f[1].Error != "" || f[1].Result == nil || f[1].Result.Dialect != "terraform" {
		t.Errorf("an absolute path outside -root: %+v", f[1])
	}
	for _, i := range []int{2, 3} {
		if f[i].Error == "" || f[i].Result != nil {
			t.Errorf("file %d should carry an error and no result: %+v", i, f[i])
		}
	}
	if f[4].Error != "" || f[4].Result == nil || f[4].Result.Dialect != "go" {
		t.Errorf("the batch stopped at a bad path: %+v", f[4])
	}
	if !strings.Contains(errw.String(), "could not be read") {
		t.Errorf("stderr says nothing about the unreadable files: %q", errw.String())
	}

	out.Reset()
	if code := run(context.Background(), []string{"dialects"}, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("dialects exit %d", code)
	}
	var d struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(out.Bytes(), &d); err != nil || d.SchemaVersion != 1 {
		t.Errorf("dialects schema_version %d, %v", d.SchemaVersion, err)
	}
}

// N12: a user who installs fathom gets fathom-lang too. The install steps in
// the README and the early-testers guide extract both binaries from the
// release archive, and `task deploy` links ~/.local/bin/fathom-lang at the
// installed binary the way the shim does for fathom.
func TestInstallPathsCarryFathomLang(t *testing.T) {
	for _, doc := range []string{"README.md", "docs/early-testers.md"} {
		b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(doc)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `-C ~/.local/bin fathom fathom-lang`) {
			t.Errorf("%s: the archive step does not extract fathom-lang", doc)
		}
		if !strings.Contains(string(b), "cmd/fathom-lang") {
			t.Errorf("%s: no install step names cmd/fathom-lang", doc)
		}
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	deploy := string(b)
	i := strings.Index(deploy, "\n  deploy:\n")
	j := strings.Index(deploy[i+1:], "\n  build-dev:")
	if i < 0 || j < 0 {
		t.Fatal("Taskfile.yml: cannot find the deploy task")
	}
	deploy = deploy[i : i+1+j]
	for _, want := range []string{
		`link="$HOME/.local/bin/fathom-lang"`,
		`ln -sfn "$HOME/.local/libexec/fathom/fathom-lang" "$link"`,
		`mv -f "$HOME/.local/libexec/fathom/fathom.new" "$HOME/.local/libexec/fathom/fathom"`, // fathom's own install is as it was
	} {
		if !strings.Contains(deploy, want) {
			t.Errorf("task deploy lacks %q", want)
		}
	}
}
