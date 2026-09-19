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

	t.Run("json carries typed_edge_producers (SU-13, ADR-041 decision 5)", func(t *testing.T) {
		resetObsCache()
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"--url", "http://127.0.0.1:1", "--repo", repo, "--json"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errb.String())
		}
		var m capabilityModel
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatalf("bad json: %v\n%s", err, out.String())
		}
		if got := m.TypedEdgeProducers["go"]; got != "native" {
			t.Errorf(`TypedEdgeProducers["go"] = %q, want "native"`, got)
		}
		if got := m.TypedEdgeProducers["ts"]; got != "scip:scip-typescript" {
			t.Errorf(`TypedEdgeProducers["ts"] = %q, want "scip:scip-typescript"`, got)
		}
		if got := m.TypedEdgeProducers["sql"]; got != "syntax" {
			t.Errorf(`TypedEdgeProducers["sql"] = %q, want "syntax"`, got)
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
		if code := cmdCapabilities([]string{"set", "--repo", repo, "guards", "on"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		reg := capability.Default()
		got, err := reg.LoadSettings(filepath.Join(repo, ".fathom", "capabilities.cue"), capability.ScopeProject)
		if err != nil {
			t.Fatalf("what set wrote must load: %v", err)
		}
		if v, ok := got["guards"]; !ok || !v {
			t.Errorf("got %+v, want guards=true", got)
		}
	})

	t.Run("an id that is not a CUE identifier is written quoted and loads back", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "verify-gate", "off"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		p := filepath.Join(repo, ".fathom", "capabilities.cue")
		got, err := capability.Default().LoadSettings(p, capability.ScopeProject)
		if err != nil {
			t.Fatalf("what set wrote must load: %v", err)
		}
		if v, ok := got["verify-gate"]; !ok || v {
			t.Errorf("got %+v, want verify-gate=false", got)
		}
		if raw, _ := os.ReadFile(p); !strings.Contains(string(raw), "\"verify-gate\": on: false") {
			t.Errorf("a hyphenated id must be written as a quoted label:\n%s", raw)
		}
		// Put the file back to the shape the next subtest expects.
		if code := cmdCapabilities([]string{"set", "--repo", repo, "verify-gate", "on"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
	})

	t.Run("a second set preserves the first", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "obligations", "off"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		reg := capability.Default()
		got, _ := reg.LoadSettings(filepath.Join(repo, ".fathom", "capabilities.cue"), capability.ScopeProject)
		if len(got) != 3 || !got["guards"] || got["obligations"] || !got["verify-gate"] {
			t.Errorf("got %+v, want every earlier setting retained", got)
		}
	})
}

// TestCapabilitiesSetPreservesBudgetsBlock pins the BA-9 tenancy contract:
// `set` rewrites ONLY the capabilities block. A `budgets` block sharing
// the file — and every comment around it — survives byte-for-byte, the
// capability still changes, and a file `set` cannot round-trip is refused
// with the file left exactly as it was.
func TestCapabilitiesSetPreservesBudgetsBlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg := capability.Default()

	write := func(t *testing.T, body string) (repo, path string) {
		t.Helper()
		repo = t.TempDir()
		path = filepath.Join(repo, ".fathom", "capabilities.cue")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return repo, path
	}
	read := func(t *testing.T, path string) string {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	const budgetsBlock = "// sized by hand for this repo\n" +
		"budgets: {\n\tlist:      3000 // roomy\n\tcomposite: 8000\n}\n"

	t.Run("both blocks present: budgets survives verbatim, the capability changes", func(t *testing.T) {
		before := "// hand-written header\n\n" +
			"capabilities: {\n\tguards: on: true\n}\n\n" + budgetsBlock
		repo, path := write(t, before)
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "guards", "off"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		after := read(t, path)
		if !strings.Contains(after, budgetsBlock) {
			t.Errorf("the budgets block must survive byte-for-byte:\n--- before ---\n%s\n--- after ---\n%s", before, after)
		}
		if !strings.HasPrefix(after, "// hand-written header\n\n") {
			t.Errorf("the header must survive verbatim:\n%s", after)
		}
		got, err := reg.LoadSettings(path, capability.ScopeProject)
		if err != nil {
			t.Fatalf("what set wrote must load: %v", err)
		}
		if v, ok := got["guards"]; !ok || v {
			t.Errorf("settings = %v, want guards=false", got)
		}
		b, berr := capability.LoadBudgets(path, capability.ScopeProject)
		if berr != nil {
			t.Fatalf("budgets must still load: %v", berr)
		}
		if b["list"] != 3000 || b["composite"] != 8000 {
			t.Errorf("budgets = %v, want list 3000 / composite 8000 untouched", b)
		}
	})

	t.Run("budgets only: the block is appended and budgets survives verbatim", func(t *testing.T) {
		repo, path := write(t, budgetsBlock)
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "guards", "on"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		after := read(t, path)
		if !strings.HasPrefix(after, budgetsBlock) {
			t.Errorf("the budgets block must survive in place:\n%s", after)
		}
		got, err := reg.LoadSettings(path, capability.ScopeProject)
		if err != nil || !got["guards"] {
			t.Errorf("settings = %v, %v; want guards=true", got, err)
		}
		if b, _ := capability.LoadBudgets(path, capability.ScopeProject); b["list"] != 3000 {
			t.Errorf("budgets = %v, want list 3000 untouched", b)
		}
	})

	t.Run("the init scaffold keeps its teaching lines above the new block", func(t *testing.T) {
		scaffold := string(capability.ExampleTemplate(reg))
		repo, path := write(t, scaffold)
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "guards", "on"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		if after := read(t, path); !strings.HasPrefix(after, scaffold) {
			t.Errorf("the scaffold must survive byte-for-byte in front of the block:\n%s", after)
		}
	})

	t.Run("a second set still keeps both blocks", func(t *testing.T) {
		repo, path := write(t, "capabilities: {\n\t\"verify-gate\": on: true\n}\n\n"+budgetsBlock)
		for _, args := range [][]string{{"guards", "on"}, {"obligations", "off"}} {
			var out, errb bytes.Buffer
			if code := cmdCapabilities(append([]string{"set", "--repo", repo}, args...), &out, &errb); code != 0 {
				t.Fatalf("exit %d: %s", code, errb.String())
			}
		}
		after := read(t, path)
		if strings.Count(after, budgetsBlock) != 1 {
			t.Errorf("exactly one budgets block must survive:\n%s", after)
		}
		if strings.Count(after, "capabilities: {") != 1 {
			t.Errorf("exactly one capabilities block must exist:\n%s", after)
		}
		got, err := reg.LoadSettings(path, capability.ScopeProject)
		if err != nil || len(got) != 3 || !got["verify-gate"] || !got["guards"] || got["obligations"] {
			t.Errorf("settings = %v, %v; want verify-gate/guards on, obligations off", got, err)
		}
	})

	t.Run("a file that does not load is refused and left untouched", func(t *testing.T) {
		// The phase-O inversion: the loader refuses it, so set must too —
		// a rewrite would replace the author's lines with one of its own.
		before := "capabilities: {\n\tnarrate: on?: false\n}\n\n" + budgetsBlock
		repo, path := write(t, before)
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "guards", "on"}, &out, &errb); code == 0 {
			t.Fatal("a file the loader refuses must not be rewritten")
		}
		if !strings.Contains(errb.String(), "narrate") || !strings.Contains(errb.String(), "by hand") {
			t.Errorf("the refusal must name the problem and the manual edit: %s", errb.String())
		}
		if after := read(t, path); after != before {
			t.Errorf("a refused set must leave the file untouched:\n%s", after)
		}
	})

	t.Run("two capabilities blocks are refused and left untouched", func(t *testing.T) {
		before := "capabilities: obligations: on: true\n" + budgetsBlock + "capabilities: guards: on: false\n"
		repo, path := write(t, before)
		var out, errb bytes.Buffer
		if code := cmdCapabilities([]string{"set", "--repo", repo, "guards", "on"}, &out, &errb); code == 0 {
			t.Fatal("two capabilities blocks cannot be rewritten as one without guessing")
		}
		if !strings.Contains(errb.String(), "more than one") {
			t.Errorf("the refusal must say why: %s", errb.String())
		}
		if after := read(t, path); after != before {
			t.Errorf("a refused set must leave the file untouched:\n%s", after)
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
