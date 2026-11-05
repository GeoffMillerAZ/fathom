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
	found := false
	for _, d := range got.Dialects {
		found = found || d.ID == "terraform" && d.Level == "L3" && d.Grade == "excellent"
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
		{[]string{"extract"}, 1},
		{[]string{"extract", "-root", t.TempDir(), "missing.go"}, 1},
		{[]string{"extract", "-nonsense"}, 1},
		{[]string{"help"}, 0},
	}
	for _, c := range cases {
		var out, errw bytes.Buffer
		if got := run(context.Background(), c.args, strings.NewReader(""), &out, &errw); got != c.want {
			t.Errorf("%v: exit %d, want %d (%s)", c.args, got, c.want, errw.String())
		}
	}
}
