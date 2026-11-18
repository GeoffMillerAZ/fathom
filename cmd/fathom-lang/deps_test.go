package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDepsJSONGolden(t *testing.T) {
	root := filepath.Join("testdata", "deps")
	for _, c := range []struct {
		golden string
		args   []string
	}{
		{"deps.json", []string{"-dialect", "go", "lib/lib.go", "main.go"}},
		{"deps-declared.json", []string{"-dialect", "go", "-export", "main.go"}},
	} {
		var out, errw bytes.Buffer
		args := append([]string{"deps", "-root", root}, c.args...)
		if code := run(context.Background(), args, strings.NewReader(""), &out, &errw); code != 0 || errw.Len() != 0 {
			t.Fatalf("exit %d: %s", code, &errw)
		}
		checkGolden(t, c.golden, out.Bytes())
		if c.golden == "deps-declared.json" {
			// Independent mirror of the consumer's data-only decoder.
			type unit struct{ Ecosystem, Name string }
			type pair struct{ From, To unit }
			var edges []pair
			if err := json.Unmarshal(out.Bytes(), &edges); err != nil {
				t.Fatal(err)
			}
			want := pair{From: unit{"repo", "main.go"}, To: unit{"repo", "lib"}}
			if len(edges) != 2 || edges[1] != want {
				t.Fatalf("declared pair: %+v", edges)
			}
		}
	}
}

func TestDepsUsageRefusal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside-sentinel.go")
	if err := os.WriteFile(outside, []byte("package main"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(root, "linked-dir")); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args   []string
		code   int
		stdout string
	}{
		{[]string{"-h"}, 0, "fathom-lang deps"},
		{nil, 2, ""},
		{[]string{"-invalid=" + outside}, 2, ""},
		{[]string{"-dialect", outside, "local.go"}, 2, ""},
		{[]string{outside}, 2, ""},
		{[]string{"../outside-sentinel.go"}, 2, ""},
		{[]string{"linked.go"}, 1, `"files"`},
		{[]string{"linked-dir/outside-sentinel.go"}, 1, `"files"`},
		{[]string{"-export", "linked.go"}, 1, ""},
		{[]string{"absent.go"}, 1, `"files"`},
	} {
		var out, errw bytes.Buffer
		args := append([]string{"deps", "-root", root}, c.args...)
		if code := run(context.Background(), args, strings.NewReader(""), &out, &errw); code != c.code ||
			(c.stdout == "" && out.Len() != 0) || !strings.Contains(out.String(), c.stdout) ||
			strings.Contains(out.String()+errw.String(), outside) {
			t.Fatalf("deps exit %d, want %d; stdout=%q stderr=%q", code, c.code, &out, &errw)
		}
	}
}

func TestDepsNarrowOutput(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/demo\n")
	write(t, root, "main.go", "package app\nimport _ \"example.org/lib\"\n")
	write(t, root, "other.txt", "private-source-sentinel")
	t.Setenv("COLUMNS", "36")
	t.Setenv("NO_COLOR", "1")
	runDeps := func(args ...string) []byte {
		t.Helper()
		var out, errw bytes.Buffer
		all := append([]string{"deps", "-root", root}, args...)
		if code := run(context.Background(), all, strings.NewReader(""), &out, &errw); code != 0 || errw.Len() != 0 {
			t.Fatalf("exit %d: %s", code, &errw)
		}
		if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "private-source-sentinel") ||
			!json.Valid(out.Bytes()) {
			t.Fatalf("output is not a clean JSON stream: %q", &out)
		}
		return out.Bytes()
	}
	full := runDeps("other.txt", "main.go")
	if !bytes.Contains(full, []byte(`"unsupported-construct"`)) ||
		!bytes.Contains(full, []byte(`"external-unpinned"`)) ||
		bytes.Contains(full, []byte(`"complete": true`)) {
		t.Fatalf("uncertainty missing from %s", full)
	}

	type unit struct{ Ecosystem, Name string }
	var pairs []struct{ From, To unit }
	if err := json.Unmarshal(runDeps("-export", "main.go"), &pairs); err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 2 || pairs[0].From != (unit{"repo", "main.go"}) || pairs[0].To != (unit{"go", "example.org/lib"}) {
		t.Fatalf("external unit mapping: %+v", pairs)
	}
	if got := string(runDeps("-export", "other.txt")); got != "[]\n" {
		t.Fatalf("empty export: %q", got)
	}
}
