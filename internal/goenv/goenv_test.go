package goenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/goenv/goenvtest"
)

// last is the value exec hands the child for key: later entries win.
func last(env []string, key string) (string, bool) {
	v, ok := "", false
	for _, kv := range env {
		if k, val, found := strings.Cut(kv, "="); found && k == key {
			v, ok = val, true
		}
	}
	return v, ok
}

// TestOfflineClosesTheModuleNetwork (ADR-083): every go command fathom runs
// on its own gets GOPROXY=off, GOSUMDB=off, GONOPROXY="," (a GOPRIVATE
// pattern would otherwise send its modules straight to their VCS),
// GOTELEMETRY=off and GOTOOLCHAIN=local (last), whatever the inherited
// environment says. GOFLAGS is always set and never empty, so the GOENV
// file cannot fill it back in: an inherited -mod=mod leaves it (it lets go
// list rewrite the adopter's go.mod and go.sum) in every form cmd/go reads
// (--mod=mod, quoted), and every other flag, -mod=vendor and -mod=readonly
// included, is kept as it was. Before the security review this test pinned
// no GOFLAGS entry for an environment without -mod=mod; ADR-083 now pins
// one.
func TestOfflineClosesTheModuleNetwork(t *testing.T) {
	cases := []struct {
		flags     string // "" = no GOFLAGS entry
		wantFlags string
	}{
		{"", " "},
		{"GOFLAGS=-mod=mod", " "},
		{"GOFLAGS=-mod=mod -p=8 -tags=integration", "-p=8 -tags=integration"},
		{"GOFLAGS=--mod=mod -trimpath", "-trimpath"},
		{"GOFLAGS='-mod=mod' \"--mod=mod\" -trimpath", "-trimpath"},
		{"GOFLAGS=-mod=mod\t-trimpath\n", "-trimpath"},
		{"GOFLAGS='-tags=a b' -mod=mod", "'-tags=a b'"},
		{"GOFLAGS=\"-ldflags=-X 'm.v=1'\" -mod=mod", "\"-ldflags=-X 'm.v=1'\""},
		{"GOFLAGS=-mod=vendor -p=1", "-mod=vendor -p=1"},
		{"GOFLAGS=-mod=readonly", "-mod=readonly"},
		{"GOFLAGS=-modcacherw -modfile=alt.mod", "-modcacherw -modfile=alt.mod"},
		// Unterminated: the go command refuses to parse it and runs
		// nothing, so it is handed over as it was.
		{"GOFLAGS='-mod=mod", "'-mod=mod"},
	}
	for _, c := range cases {
		base := []string{"PATH=/usr/bin", "GOENV=off", "GOPROXY=https://proxy.example.test", "GOSUMDB=sum.example.test", "GOPRIVATE=corp.example.test", "GOTELEMETRY=on", "GOTOOLCHAIN=auto", "HOME=/h"}
		if c.flags != "" {
			base = append(base, c.flags)
		}
		env := Offline(append([]string(nil), base...), false)
		for key, want := range map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GONOPROXY": ",", "GOTELEMETRY": "off", "GOTOOLCHAIN": "local", "HOME": "/h", "PATH": "/usr/bin"} {
			if got, _ := last(env, key); got != want {
				t.Errorf("%q: %s = %q, want %q", c.flags, key, got, want)
			}
		}
		if env[len(env)-1] != "GOTOOLCHAIN=local" {
			t.Errorf("%q: env ends %q, want GOTOOLCHAIN=local last", c.flags, env[len(env)-1])
		}
		if got, ok := last(env, "GOFLAGS"); !ok || got != c.wantFlags {
			t.Errorf("%q: GOFLAGS = %q (set %v), want %q (-mod=mod stripped, the rest kept, never empty)", c.flags, got, ok, c.wantFlags)
		}
		if !strings.HasPrefix(strings.Join(env, "\n"), strings.Join(base, "\n")) {
			t.Errorf("%q: the inherited entries were not kept in order", c.flags)
		}
	}
}

// TestOfflineReadsTheGOENVFile (ADR-083): with no GOFLAGS in the
// environment the go command reads it from the GOENV file (`go env -w
// GOFLAGS=-mod=mod`), so Offline asks `go env GOFLAGS` and strips that
// value too; a process GOFLAGS wins over the file, as in cmd/go.
func TestOfflineReadsTheGOENVFile(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	file := filepath.Join(t.TempDir(), "env")
	for _, c := range []struct{ file, process, want string }{
		{"GOFLAGS=-mod=mod -tags=fromfile\n", "", "-tags=fromfile"},
		{"GOFLAGS='-mod=mod'\n", "", " "},
		{"GOFLAGS=-tags=fromfile\n", "-trimpath", "-trimpath"},
		{"", "", " "},
	} {
		if err := os.WriteFile(file, []byte(c.file), 0o600); err != nil {
			t.Fatal(err)
		}
		base := append(os.Environ(), "GOENV="+file, "GOFLAGS="+c.process)
		if got, _ := last(Offline(base, false), "GOFLAGS"); got != c.want {
			t.Errorf("GOENV file %q, process GOFLAGS %q: GOFLAGS = %q, want %q", c.file, c.process, got, c.want)
		}
	}
}

// TestOfflineNeverRewritesGoSum (ADR-083): over a warm module cache and a
// go.sum one line short, `go list` under Offline's environment leaves go.sum
// as it was, with -mod=mod in the process GOFLAGS or in the GOENV file. The
// control run (the network closed, the GOENV file's -mod=mod left in place)
// rewrites it, so the check can fail.
func TestOfflineNeverRewritesGoSum(t *testing.T) {
	w := goenvtest.WarmModule(t)
	file := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(file, []byte("GOFLAGS=-mod=mod\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := func(flags string) []string {
		return append(os.Environ(), "GOWORK=off", "GOENV="+file, "GOMODCACHE="+w.ModCache, "GOFLAGS="+flags)
	}
	list := func(env []string) {
		t.Helper()
		cmd := exec.Command("go", "list", "-e", "-deps", "./...")
		cmd.Dir, cmd.Env = w.Dir, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go list: %v\n%s", err, out)
		}
	}
	list(append(base(""), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local"))
	if rewritten, _ := w.GoSumRewritten(t); !rewritten {
		t.Fatal("control: the GOENV file's -mod=mod did not rewrite go.sum, so this fixture cannot detect a rewrite")
	}
	for _, flags := range []string{"", "-mod=mod"} {
		w.Reset(t)
		list(Offline(base(flags), false))
		if rewritten, got := w.GoSumRewritten(t); rewritten {
			t.Errorf("process GOFLAGS %q, GOENV file -mod=mod: go.sum was rewritten:\n%s", flags, got)
		}
	}
}

// TestOfflineNetworkOptOut: with the user's [ingest] go_network = true the
// inherited GOPROXY, GOSUMDB and GONOPROXY pass through (GOSUMDB stays on
// whenever a download can happen); the toolchain pin, the telemetry line
// and the -mod=mod strip stay.
func TestOfflineNetworkOptOut(t *testing.T) {
	base := []string{"GOENV=off", "GOPROXY=https://proxy.example.test", "GOSUMDB=sum.example.test", "GONOPROXY=corp.example.test", "GOFLAGS=-mod=mod -p=2"}
	env := Offline(base, true)
	for key, want := range map[string]string{"GOPROXY": "https://proxy.example.test", "GOSUMDB": "sum.example.test", "GONOPROXY": "corp.example.test", "GOTELEMETRY": "off", "GOTOOLCHAIN": "local", "GOFLAGS": "-p=2"} {
		if got, _ := last(env, key); got != want {
			t.Errorf("go_network = true: %s = %q, want %q", key, got, want)
		}
	}
}

// TestApplyReadsOnlyTheUserLayer: Apply asks the user config
// (~/.fathom/config.toml) for go_network. Absent, unreadable or invalid, the
// go command runs offline; set true, the proxy is the inherited one.
func TestApplyReadsOnlyTheUserLayer(t *testing.T) {
	home := t.TempDir()
	t.Setenv(fathomhome.Env, home)
	base := []string{"GOPROXY=https://proxy.example.test"}
	if v, _ := last(Apply(base), "GOPROXY"); v != "off" {
		t.Fatalf("no user config: GOPROXY = %q, want off", v)
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("[ingest]\ngo_network = true\n")
	if v, _ := last(Apply(base), "GOPROXY"); v != "https://proxy.example.test" {
		t.Fatalf("user go_network = true: GOPROXY = %q, want the inherited proxy", v)
	}
	write("[ingest]\ngo_network = \"yes\"\n")
	if v, _ := last(Apply(base), "GOPROXY"); v != "off" {
		t.Fatalf("an invalid user config: GOPROXY = %q, want off (never open on a broken file)", v)
	}
}

// TestMissingModuleNamesTheRefusal: the go command's own words for a module
// it may not fetch name path@version on a whole-load refusal; a package
// error names only the package, and is reported as such.
func TestMissingModuleNamesTheRefusal(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"go: example.com/m@v1.0.0: module lookup disabled by GOPROXY=off", "example.com/m@v1.0.0"},
		{"err: exit status 1: stderr: go: downloading x\ngo: example.com/m@v1.2.3: module lookup disabled by GOPROXY=off\n", "example.com/m@v1.2.3"},
		{"/repo/main.go:3:8: module lookup disabled by GOPROXY=off", ""},
	} {
		got, ok := MissingModule(c.in)
		if !ok || got != c.want {
			t.Errorf("MissingModule(%q) = %q, %v; want %q, true", c.in, got, ok, c.want)
		}
	}
	if _, ok := MissingModule("go: some other failure"); ok {
		t.Error("MissingModule matched text without the refusal")
	}
}
