package config_test

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"cuelang.org/go/cue"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/config"
)

const testSchema = `
name: string | *"default-name"
dir?: string
`

// testHost is an fstest.MapFS-backed config.Host: every read goes through
// an in-memory map, never the real filesystem, and Getenv/Getwd are
// caller-controlled fields instead of the real process environment — the
// hexagonal seam this package's Options.Host exists for.
type testHost struct {
	fstest.MapFS
	env map[string]string
	wd  string
}

func (h testHost) Getenv(key string) string { return h.env[key] }
func (h testHost) Getwd() (string, error)   { return h.wd, nil }

func newHost(files fstest.MapFS, env map[string]string, wd string) testHost {
	if files == nil {
		files = fstest.MapFS{}
	}
	if env == nil {
		env = map[string]string{}
	}
	return testHost{MapFS: files, env: env, wd: wd}
}

func file(data string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(data)} }

// TestConfigLayerPrecedence is table-driven over which layer sets `name`
// in each row: schema default, user, project, and an explicit override.
// Rows never have two layers concretely set the SAME field to different
// values — that is TestConfigConflictRefused's job, and under CUE
// unification it is a refusal, not a "later layer wins" resolution (see
// the package doc). Rows 3 and 4 still exercise MULTIPLE loaded layers at
// once — user sets an unrelated field (`dir`) alongside project/override
// setting `name` — to prove several layers coexist without name itself
// ever being set twice.
func TestConfigLayerPrecedence(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		env   map[string]string
		wd    string
		overr map[string]any
		want  string
	}{
		{
			name:  "schema default only",
			files: fstest.MapFS{},
			env:   map[string]string{"HOME": "/home/dev"},
			wd:    "/home/dev/project",
			want:  "default-name",
		},
		{
			name: "user layer sets name",
			files: fstest.MapFS{
				"home/dev/.config/kittool/config.cue": file(`name: "from-user"`),
			},
			env:  map[string]string{"HOME": "/home/dev"},
			wd:   "/home/dev/project",
			want: "from-user",
		},
		{
			name: "project layer sets name alongside an unrelated user field",
			files: fstest.MapFS{
				"home/dev/.config/kittool/config.cue": file(`dir: "/from/user"`),
				"home/dev/project/.git/HEAD":          file("ref: refs/heads/main\n"),
				"home/dev/project/kittool.cue":        file(`name: "from-project"`),
			},
			env:  map[string]string{"HOME": "/home/dev"},
			wd:   "/home/dev/project",
			want: "from-project",
		},
		{
			name: "explicit override sets name over the schema default, alongside an unrelated user field",
			files: fstest.MapFS{
				"home/dev/.config/kittool/config.cue": file(`dir: "/from/user"`),
			},
			env:   map[string]string{"HOME": "/home/dev"},
			wd:    "/home/dev/project",
			overr: map[string]any{"name": "from-override"},
			want:  "from-override",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			host := newHost(c.files, c.env, c.wd)
			res, err := config.Load("kittool", []byte(testSchema), config.Options{
				Host:      host,
				Overrides: c.overr,
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got, err := res.Value.LookupPath(cue.ParsePath("name")).String()
			if err != nil {
				t.Fatalf("lookup name: %v", err)
			}
			if got != c.want {
				t.Errorf("name = %q, want %q", got, c.want)
			}
		})
	}
}

// TestConfigProjectWalkBoundedAtGitRoot proves the project-layer walk
// stops at the git worktree root: a kittool.cue that sits ABOVE the repo
// (e.g. a monorepo checkout's parent directory) is never read, even
// though it is technically reachable by continuing to walk upward.
func TestConfigProjectWalkBoundedAtGitRoot(t *testing.T) {
	files := fstest.MapFS{
		"home/dev/kittool.cue":              file(`name: "above-the-repo"`), // must NEVER be read
		"home/dev/code/repo/.git/HEAD":      file("ref: refs/heads/main\n"),
		"home/dev/code/repo/sub/marker.txt": file("x"),
	}
	host := newHost(files, map[string]string{"HOME": "/home/dev"}, "/home/dev/code/repo/sub")

	res, err := config.Load("kittool", []byte(testSchema), config.Options{Host: host})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := res.Value.LookupPath(cue.ParsePath("name")).String()
	if err != nil {
		t.Fatalf("lookup name: %v", err)
	}
	if got != "default-name" {
		t.Errorf("name = %q, want the schema default %q — the walk read a config above the git root", got, "default-name")
	}
	for _, l := range res.Layers {
		if l.Name == "project" {
			t.Fatalf("project layer loaded from %q — should have stopped at the git root with no project config found", l.Path)
		}
	}
}

// TestConfigProjectWalkFindsConfigAtGitRoot proves the git-root directory
// ITSELF is still checked for the project config before the walk stops —
// the bound is inclusive, not exclusive.
func TestConfigProjectWalkFindsConfigAtGitRoot(t *testing.T) {
	files := fstest.MapFS{
		"home/dev/code/repo/.git/HEAD":   file("ref: refs/heads/main\n"),
		"home/dev/code/repo/kittool.cue": file(`name: "at-git-root"`),
	}
	host := newHost(files, map[string]string{"HOME": "/home/dev"}, "/home/dev/code/repo/sub/deeper")

	res, err := config.Load("kittool", []byte(testSchema), config.Options{Host: host})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := res.Value.LookupPath(cue.ParsePath("name")).String()
	if err != nil {
		t.Fatalf("lookup name: %v", err)
	}
	if got != "at-git-root" {
		t.Errorf("name = %q, want %q", got, "at-git-root")
	}
}

// TestConfigExplainNamesLayer proves Explain's field trace names the
// layer that actually last changed a field, distinguishing "changed by
// the project layer" (name) from "unchanged since the user layer" (dir —
// project never touches it, so user is still the last layer that set it).
func TestConfigExplainNamesLayer(t *testing.T) {
	files := fstest.MapFS{
		"home/dev/.config/kittool/config.cue": file(`dir: "/from/user"`),
		"home/dev/project/.git/HEAD":          file("ref: refs/heads/main\n"),
		"home/dev/project/kittool.cue":        file(`name: "from-project"`),
	}
	host := newHost(files, map[string]string{"HOME": "/home/dev"}, "/home/dev/project")

	res, err := config.Load("kittool", []byte(testSchema), config.Options{Host: host})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var buf bytes.Buffer
	if err := res.Explain(&buf, "name"); err != nil {
		t.Fatalf("Explain(name): %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "name: last set by project") {
		t.Errorf("Explain(name) does not name the project layer:\n%s", out)
	}

	buf.Reset()
	if err := res.Explain(&buf, "dir"); err != nil {
		t.Fatalf("Explain(dir): %v", err)
	}
	out = buf.String()
	if !strings.Contains(out, "dir: last set by user") {
		t.Errorf("Explain(dir) does not name the user layer (project never touched dir):\n%s", out)
	}

	// Every loaded layer's path+digest line is present too.
	for _, want := range []string{"schema", "user:", "project:"} {
		if !strings.Contains(out, want) {
			t.Errorf("Explain output missing layer line %q:\n%s", want, out)
		}
	}
}

// TestConfigConflictRefused proves two layers setting the same field to
// different concrete values is a refusal, not a silent "last one wins" —
// the opposite of TestConfigLayerPrecedence's rows, which are careful to
// never let two layers set the same field.
func TestConfigConflictRefused(t *testing.T) {
	files := fstest.MapFS{
		"home/dev/.config/kittool/config.cue": file(`name: "from-user"`),
		"home/dev/project/.git/HEAD":          file("ref: refs/heads/main\n"),
		"home/dev/project/kittool.cue":        file(`name: "from-project"`), // conflicts with user's "from-user"
	}
	host := newHost(files, map[string]string{"HOME": "/home/dev"}, "/home/dev/project")

	_, err := config.Load("kittool", []byte(testSchema), config.Options{Host: host})
	if err == nil {
		t.Fatal("Load with user and project both setting name to different concrete values: want error, got nil")
	}
}

// TestConfigOverrideConflictsWithConcreteFileValueRefused proves overrides
// are not exempt from the same rule: an override cannot silently clobber
// a concrete value a file layer already pinned, either.
func TestConfigOverrideConflictsWithConcreteFileValueRefused(t *testing.T) {
	files := fstest.MapFS{
		"home/dev/.config/kittool/config.cue": file(`name: "from-user"`),
	}
	host := newHost(files, map[string]string{"HOME": "/home/dev"}, "/home/dev/project")

	_, err := config.Load("kittool", []byte(testSchema), config.Options{
		Host:      host,
		Overrides: map[string]any{"name": "from-override"},
	})
	if err == nil {
		t.Fatal("Load with an override conflicting with a concrete user value: want error, got nil")
	}
}
