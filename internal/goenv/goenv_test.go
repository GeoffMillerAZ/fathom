package goenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
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
// on its own gets GOPROXY=off, GOSUMDB=off, GOTELEMETRY=off and
// GOTOOLCHAIN=local (last), whatever the inherited environment says; an
// inherited -mod=mod leaves GOFLAGS (it lets go list rewrite the adopter's
// go.mod and go.sum), and every other flag, -mod=vendor and -mod=readonly
// included, is kept as it was.
func TestOfflineClosesTheModuleNetwork(t *testing.T) {
	cases := []struct {
		flags     string // "" = no GOFLAGS entry
		wantFlags string // "-" = no GOFLAGS entry added
	}{
		{"", "-"},
		{"GOFLAGS=-mod=mod", ""},
		{"GOFLAGS=-mod=mod -p=8 -tags=integration", "-p=8 -tags=integration"},
		{"GOFLAGS=--mod=mod -trimpath", "-trimpath"},
		{"GOFLAGS=-mod=vendor -p=1", "-"},
		{"GOFLAGS=-mod=readonly", "-"},
		{"GOFLAGS=-modcacherw -modfile=alt.mod", "-"},
	}
	for _, c := range cases {
		base := []string{"PATH=/usr/bin", "GOPROXY=https://proxy.example.test", "GOSUMDB=sum.example.test", "GOTELEMETRY=on", "GOTOOLCHAIN=auto", "HOME=/h"}
		if c.flags != "" {
			base = append(base, c.flags)
		}
		env := Offline(append([]string(nil), base...), false)
		for key, want := range map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GOTELEMETRY": "off", "GOTOOLCHAIN": "local", "HOME": "/h", "PATH": "/usr/bin"} {
			if got, _ := last(env, key); got != want {
				t.Errorf("%q: %s = %q, want %q", c.flags, key, got, want)
			}
		}
		if env[len(env)-1] != "GOTOOLCHAIN=local" {
			t.Errorf("%q: env ends %q, want GOTOOLCHAIN=local last", c.flags, env[len(env)-1])
		}
		got, _ := last(env, "GOFLAGS")
		inherited, _ := last(base, "GOFLAGS")
		if c.wantFlags == "-" {
			if got != inherited {
				t.Errorf("%q: GOFLAGS = %q, want the inherited %q untouched", c.flags, got, inherited)
			}
		} else if got != c.wantFlags {
			t.Errorf("%q: GOFLAGS = %q, want %q (-mod=mod stripped, the rest kept)", c.flags, got, c.wantFlags)
		}
		if !strings.HasPrefix(strings.Join(env, "\n"), strings.Join(base, "\n")) {
			t.Errorf("%q: the inherited entries were not kept in order", c.flags)
		}
	}
}

// TestOfflineNetworkOptOut: with the user's [ingest] go_network = true the
// inherited GOPROXY and GOSUMDB pass through (GOSUMDB stays on whenever a
// download can happen); the toolchain pin, the telemetry line and the
// -mod=mod strip stay.
func TestOfflineNetworkOptOut(t *testing.T) {
	base := []string{"GOPROXY=https://proxy.example.test", "GOSUMDB=sum.example.test", "GOFLAGS=-mod=mod -p=2"}
	env := Offline(base, true)
	for key, want := range map[string]string{"GOPROXY": "https://proxy.example.test", "GOSUMDB": "sum.example.test", "GOTELEMETRY": "off", "GOTOOLCHAIN": "local", "GOFLAGS": "-p=2"} {
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
