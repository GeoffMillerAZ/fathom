package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/capability"
)

// TestCmdCapabilities pins CR-5: the verb renders every capability with its
// state, and every off entry carries either a command or an explanation.
// It runs against an UNREACHABLE daemon on purpose — a degraded probe must
// still produce an honest, useful table rather than an error.
func TestCmdCapabilities(t *testing.T) {
	dir := hermeticObsEnv(t)
	_ = dir
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // no user-scope config

	t.Run("a virgin repo gets a full table with reasons", func(t *testing.T) {
		resetObsCache()
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"--url", "http://127.0.0.1:1", "--repo", repo}, &out, &errb); code != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errb.String())
		}
		got := out.String()
		for _, id := range capability.Default().IDs() {
			if !strings.Contains(got, id) {
				t.Errorf("table is missing %q:\n%s", id, got)
			}
		}
		if strings.Contains(got, "✓") {
			t.Errorf("nothing can be operating in a repository fathom has never touched:\n%s", got)
		}
	})

	t.Run("json carries reason and remedy for every off entry", func(t *testing.T) {
		resetObsCache()
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"--url", "http://127.0.0.1:1", "--repo", repo, "--json"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errb.String())
		}
		var m capabilityModel
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatalf("bad json: %v\n%s", err, out.String())
		}
		if m.Total == 0 || len(m.Capabilities) != m.Total {
			t.Fatalf("model = %+v", m)
		}
		for _, c := range m.Capabilities {
			if c.Operating {
				continue
			}
			if c.Reason == "" {
				t.Errorf("%q is off with no reason", c.ID)
			}
			if c.Cmd == "" && c.Note == "" {
				t.Errorf("%q is off with neither a command nor an explanation", c.ID)
			}
			if c.Summary == "" || c.Cost == "" {
				t.Errorf("%q must always carry its summary and cost: %+v", c.ID, c)
			}
		}
	})

	t.Run("explain names the cost, the scope rule and the evidence it produces", func(t *testing.T) {
		resetObsCache()
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"explain", "--url", "http://127.0.0.1:1", "--repo", repo, "obligations"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errb.String())
		}
		got := out.String()
		for _, want := range []string{"Costs when on:", "Settable by:", "Observed:", "Evidence it produces:"} {
			if !strings.Contains(got, want) {
				t.Errorf("explain is missing %q:\n%s", want, got)
			}
		}
	})

	t.Run("explain refuses an unknown id with a suggestion", func(t *testing.T) {
		resetObsCache()
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"explain", "--repo", repo, "oblgations"}, &out, &errb); code == 0 {
			t.Fatal("an unknown id must be refused")
		}
		if !strings.Contains(errb.String(), "obligations") {
			t.Errorf("the refusal must suggest the near miss: %s", errb.String())
		}
	})
}

// TestCmdCapabilitiesSet pins the write path: it refuses a scope violation
// BEFORE writing (a file that fails to load next time is worse than a
// refused command), and what it writes loads back cleanly.
func TestCmdCapabilitiesSet(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	t.Run("user scope may not govern", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "--scope", "user", "verify-gate", "off"}, &out, &errb); code == 0 {
			t.Fatal("setting a project-only capability at user scope must be refused (law 13)")
		}
		if !strings.Contains(errb.String(), "verify-gate") {
			t.Errorf("the refusal must name the key: %s", errb.String())
		}
		home, _ := os.UserHomeDir()
		if _, err := os.Stat(filepath.Join(home, ".fathom", "capabilities.cue")); err == nil {
			t.Error("a refused set must not have written a file")
		}
	})

	t.Run("project scope writes a file that loads back", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "narrate", "on"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		reg := capability.Default()
		got, err := reg.LoadSettings(filepath.Join(repo, ".fathom", "capabilities.cue"), capability.ScopeProject)
		if err != nil {
			t.Fatalf("what set wrote must load: %v", err)
		}
		if v, ok := got["narrate"]; !ok || !v {
			t.Errorf("got %+v, want narrate=true", got)
		}
	})

	t.Run("a second set preserves the first", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "modules", "off"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		reg := capability.Default()
		got, _ := reg.LoadSettings(filepath.Join(repo, ".fathom", "capabilities.cue"), capability.ScopeProject)
		if len(got) != 2 || !got["narrate"] || got["modules"] {
			t.Errorf("got %+v, want both settings retained", got)
		}
	})
}

// TestDoctorCapabilitiesOnVirginRepo pins CR-6 and the O23 class: in a
// repository with no fathom configuration at all, the capability row must
// produce ZERO known-false failures. Absence is not breakage.
func TestDoctorCapabilitiesOnVirginRepo(t *testing.T) {
	hermeticObsEnv(t)
	resetObsCache()
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	got := checkCapabilities("http://127.0.0.1:1", repo, "")
	if got.Status == doctorFail {
		t.Fatalf("a repository fathom has never touched must not FAIL the capability row: %+v", got)
	}
	if got.Reason == "" {
		t.Error("the row must always say what it found")
	}
}

// TestDoctorCapabilitiesFailsOnlyOnBrokenDelivery pins the distinction that
// keeps doctor honest: inputs-absent is a pass, inputs-present-but-
// undelivered is a fail.
func TestDoctorCapabilitiesFailsOnlyOnBrokenDelivery(t *testing.T) {
	reg := capability.Default()

	t.Run("absent inputs are not a failure", func(t *testing.T) {
		for _, r := range reg.DetectAll(capability.Probe{}, capability.Resolution{}) {
			if r.Capability.ID == "modules" || r.Capability.ID == "layers" {
				if r.State.Broken {
					t.Errorf("%q reports BROKEN when its inputs are merely absent", r.Capability.ID)
				}
			}
		}
	})

	t.Run("compiled rules with no hook is broken", func(t *testing.T) {
		p := capability.Probe{ObligationRules: 3, ObligationsCompiled: true, HooksReferenced: false}
		c, _ := reg.Get("obligations")
		st := c.Detect(p)
		if st.Operating || !st.Broken {
			t.Errorf("got %+v, want a BROKEN delivery — the rules exist and nothing serves them", st)
		}
	})

	t.Run("a stale index is broken", func(t *testing.T) {
		c, _ := reg.Get("index")
		st := c.Detect(capability.Probe{IndexedSymbols: 10, IndexStale: true})
		if !st.Broken {
			t.Error("an index behind HEAD is a defect, not an absence")
		}
		// ...but "nobody could tell" is not.
		unknown := c.Detect(capability.Probe{IndexedSymbols: 10, IndexNote: "unknown: statusz unreachable"})
		if unknown.Broken {
			t.Error("unknown freshness is an absence of information, not a defect")
		}
	})
}

// TestCapabilityConfigErrorIsLoud pins the refusal contract end to end: a
// broken config file makes doctor FAIL rather than being ignored.
func TestCapabilityConfigErrorIsLoud(t *testing.T) {
	hermeticObsEnv(t)
	resetObsCache()
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(repo, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The phase-O inversion: reads as "switch narrate off", does nothing.
	body := []byte("capabilities: {\n\tnarrate: on?: false\n}\n")
	if err := os.WriteFile(filepath.Join(repo, ".fathom", "capabilities.cue"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	got := checkCapabilities("http://127.0.0.1:1", repo, "")
	if got.Status != doctorFail {
		t.Fatalf("a config file that does not load must FAIL, not be ignored: %+v", got)
	}
	if !strings.Contains(got.Reason, "narrate") {
		t.Errorf("the failure must name the offending capability: %s", got.Reason)
	}
}

// TestInitScaffoldsCapabilities pins the O22 contract for the new file:
// `fathom init` writes it commented, and the ACTUAL bytes it writes load
// as an empty configuration rather than as a parse error.
func TestInitScaffoldsCapabilities(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := writeFathomConfigScaffolds(root, &out); err != nil {
		t.Fatalf("scaffolding failed: %v", err)
	}
	p := filepath.Join(root, ".fathom", "capabilities.cue")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("init must write %s: %v", p, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if t2 := strings.TrimSpace(line); t2 != "" && !strings.HasPrefix(t2, "//") {
			t.Fatalf("the scaffold must be entirely commented so it changes nothing; found: %q", line)
		}
	}
	got, lerr := capability.Default().LoadSettings(p, capability.ScopeProject)
	if lerr != nil {
		t.Fatalf("what init writes must load: %v", lerr)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want no settings from a commented scaffold", got)
	}

	// Write-if-absent: a second init must not clobber a real config.
	if err := os.WriteFile(p, []byte("capabilities: {\n\tnarrate: on: true\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := writeFathomConfigScaffolds(root, &out); err != nil {
		t.Fatal(err)
	}
	after, _ := capability.Default().LoadSettings(p, capability.ScopeProject)
	if !after["narrate"] {
		t.Error("a second init overwrote a real configuration")
	}
}
