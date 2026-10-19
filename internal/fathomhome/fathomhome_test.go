package fathomhome

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestFathomHomeDefaultsUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(Env, "")
	if got, want := FathomHome(), filepath.Join(home, ".fathom"); got != want {
		t.Fatalf("FathomHome() = %q, want %q", got, want)
	}
}

func TestFathomHomeEnvRelocates(t *testing.T) {
	home, scratch := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(Env, scratch)
	if got := FathomHome(); got != scratch {
		t.Fatalf("FathomHome() = %q, want %q", got, scratch)
	}
	if got, want := Path("daemon", "x.json"), filepath.Join(scratch, "daemon", "x.json"); got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}

// A relative FATHOM_HOME is anchored at the working directory once, so a
// child that changes directory reads the same place.
func TestFathomHomeRelativeIsMadeAbsolute(t *testing.T) {
	t.Setenv(Env, "rel-home")
	got := FathomHome()
	if !filepath.IsAbs(got) || filepath.Base(got) != "rel-home" {
		t.Fatalf("FathomHome() = %q, want an absolute path ending in rel-home", got)
	}
}

func TestFathomHomeUnresolvableIsEmpty(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("os.UserHomeDir does not depend on $HOME here")
	}
	t.Setenv("HOME", "")
	t.Setenv(Env, "")
	if got := FathomHome(); got != "" {
		t.Fatalf("FathomHome() with no home = %q, want empty", got)
	}
	if got := Path("a"); got != "" {
		t.Fatalf("Path with no home = %q, want empty", got)
	}
	if got := PerFile("FATHOM_TEST_UNSET_LEDGER", "a"); got != "" {
		t.Fatalf("PerFile with no home = %q, want empty", got)
	}
}

func TestPerFileVariableWinsOverFathomHome(t *testing.T) {
	scratch, other := t.TempDir(), filepath.Join(t.TempDir(), "one.jsonl")
	t.Setenv(Env, scratch)
	t.Setenv("FATHOM_TEST_LEDGER", "")
	if got, want := PerFile("FATHOM_TEST_LEDGER", "l.jsonl"), filepath.Join(scratch, "l.jsonl"); got != want {
		t.Fatalf("unset per-file var: %q, want %q", got, want)
	}
	t.Setenv("FATHOM_TEST_LEDGER", other)
	if got := PerFile("FATHOM_TEST_LEDGER", "l.jsonl"); got != other {
		t.Fatalf("set per-file var: %q, want %q", got, other)
	}
}

func TestUnderHonorsTheOverrideForAnInjectedHome(t *testing.T) {
	injected, scratch := t.TempDir(), t.TempDir()
	t.Setenv(Env, "")
	if got, want := Under(injected), filepath.Join(injected, ".fathom"); got != want {
		t.Fatalf("Under without override = %q, want %q", got, want)
	}
	if got := Under(""); got != "" {
		t.Fatalf("Under(\"\") without override = %q, want empty", got)
	}
	t.Setenv(Env, scratch)
	if got := Under(injected); got != scratch {
		t.Fatalf("Under with override = %q, want %q", got, scratch)
	}
	if got := Under(""); got != scratch {
		t.Fatalf("Under(\"\") with override = %q, want %q", got, scratch)
	}
}

func TestAtIgnoresTheOverride(t *testing.T) {
	home, scratch := t.TempDir(), t.TempDir()
	t.Setenv(Env, scratch)
	if got, want := At(home, "daemons", "k.json"), filepath.Join(home, ".fathom", "daemons", "k.json"); got != want {
		t.Fatalf("At = %q, want %q", got, want)
	}
	if got, want := At(home), filepath.Join(home, ".fathom"); got != want {
		t.Fatalf("At(home) = %q, want %q", got, want)
	}
}
