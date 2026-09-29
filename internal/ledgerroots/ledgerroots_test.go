package ledgerroots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkdir makes dir (and its parents) and returns it.
func mkdir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// resolved is dir with its symlinks resolved (a t.TempDir on macOS lives
// under /var, a symlink to /private/var).
func resolved(t *testing.T, dir string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestRegisteredRootsCanonicalAndSymlinkResolved pins PE-23: the registered
// roots are canonical (absolute, clean, symlinks resolved, no trailing
// slash); a row root matches only by exact canonical equality, so a
// worktree or a subdirectory under a registered root is not registered; a
// root under a scratch (temp) directory is ignored even when registered and
// is named scratch; and an unread registry excludes nothing and says why.
func TestRegisteredRootsCanonicalAndSymlinkResolved(t *testing.T) {
	base := t.TempDir()
	scratch := mkdir(t, filepath.Join(base, "scratch"))
	app := mkdir(t, filepath.Join(base, "repos", "app"))
	lib := mkdir(t, filepath.Join(base, "repos", "lib"))
	links := mkdir(t, filepath.Join(base, "links"))
	appLink := filepath.Join(links, "app")
	if err := os.Symlink(app, appLink); err != nil {
		t.Fatal(err)
	}
	baseLink := filepath.Join(links, "base")
	if err := os.Symlink(base, baseLink); err != nil {
		t.Fatal(err)
	}
	clone := mkdir(t, filepath.Join(scratch, "clone"))
	other := mkdir(t, filepath.Join(base, "elsewhere", "other"))

	// app registered through its symlink, with a trailing slash; lib by its
	// own path; one registered root inside the scratch area.
	roots := RegisteredRootsWith([]string{appLink + "/", lib, clone}, []string{scratch})
	if !roots.Read {
		t.Fatal("a registry built from paths was not read")
	}

	t.Run("canonical list: symlink resolved, no trailing slash", func(t *testing.T) {
		got := roots.List()
		want := []string{resolved(t, clone), resolved(t, app), resolved(t, lib)}
		if strings.Join(got, "\n") != strings.Join(sortedCopy(want), "\n") {
			t.Fatalf("List() = %q, want %q", got, sortedCopy(want))
		}
		for _, r := range got {
			if strings.HasSuffix(r, "/") {
				t.Errorf("canonical root %q keeps a trailing slash", r)
			}
		}
		if c := Canonical(appLink + "/"); c != resolved(t, app) {
			t.Errorf("Canonical(symlink/) = %q, want %q", c, resolved(t, app))
		}
	})

	t.Run("a symlinked dir equals its target", func(t *testing.T) {
		for _, spelling := range []string{app, app + "/", appLink, appLink + "/", resolved(t, app)} {
			if !roots.Registered(spelling) || roots.Excludes(spelling) {
				t.Errorf("%q: Registered=%v Excludes=%v, want registered and kept", spelling, roots.Registered(spelling), roots.Excludes(spelling))
			}
			if c := roots.Class(spelling); c != ClassRegistered {
				t.Errorf("Class(%q) = %q, want %q", spelling, c, ClassRegistered)
			}
		}
	})

	t.Run("a t.TempDir symlink resolved", func(t *testing.T) {
		// base is a t.TempDir: its own spelling, its resolved spelling and a
		// symlink to it all name the same registered root below it.
		for _, spelling := range []string{
			filepath.Join(base, "repos", "lib"),
			filepath.Join(resolved(t, base), "repos", "lib"),
			filepath.Join(baseLink, "repos", "lib"),
		} {
			if !roots.Registered(spelling) {
				t.Errorf("%q is the registered lib root but was not registered", spelling)
			}
		}
	})

	t.Run("a worktree or subdirectory under a registered root is not registered", func(t *testing.T) {
		wt := mkdir(t, filepath.Join(app, ".claude", "worktrees", "agent-1"))
		sub := mkdir(t, filepath.Join(lib, "workspace", "tmp", "pushclone"))
		mkdir(t, filepath.Join(app, "cmd"))
		for _, r := range []string{wt, sub, filepath.Join(appLink, "cmd")} {
			if roots.Registered(r) || !roots.Excludes(r) {
				t.Errorf("%q under a registered root: Registered=%v Excludes=%v, want excluded", r, roots.Registered(r), roots.Excludes(r))
			}
			if c := roots.Class(r); c != ClassWorktree {
				t.Errorf("Class(%q) = %q, want %q", r, c, ClassWorktree)
			}
		}
		if c := roots.Class(other); c != ClassUnregistered || !roots.Excludes(other) {
			t.Errorf("an unrelated root: Class=%q Excludes=%v, want %q and excluded", c, roots.Excludes(other), ClassUnregistered)
		}
		if !roots.Excludes("") {
			t.Error("a row naming no root is not a registered root, yet it was kept")
		}
	})

	t.Run("a root under the scratch dir is ignored even when registered", func(t *testing.T) {
		if roots.Registered(clone) || !roots.Excludes(clone) || roots.Class(clone) != ClassScratch {
			t.Errorf("registered scratch root: Registered=%v Excludes=%v Class=%q, want excluded as scratch", roots.Registered(clone), roots.Excludes(clone), roots.Class(clone))
		}
		gone := filepath.Join(scratch, "deleted-clone") // never created: canonical falls back to the clean path
		if roots.Class(gone) != ClassScratch {
			t.Errorf("a deleted scratch clone: Class=%q, want %q", roots.Class(gone), ClassScratch)
		}
		// The default scratch dirs are the OS temp dirs (a t.TempDir may sit
		// under GOTMPDIR instead, so the clone is made under os.TempDir).
		osClone, err := os.MkdirTemp("", "ledgerroots-clone-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(osClone) })
		def := RegisteredRoots([]string{osClone, app})
		if def.Registered(osClone) || def.Class(osClone) != ClassScratch {
			t.Errorf("RegisteredRoots over a root under os.TempDir: Registered=%v Class=%q, want scratch", def.Registered(osClone), def.Class(osClone))
		}
		if def.Class("/tmp/fathom-scratch-clone-never-made") != ClassScratch {
			t.Errorf("a /tmp root is not named scratch by the default dirs")
		}
	})

	t.Run("an unread registry excludes nothing and says why", func(t *testing.T) {
		u := Unread("the daemon did not answer (connection refused)")
		if u.Read || u.Note == "" {
			t.Fatalf("Unread = %+v, want Read false with a note", u)
		}
		for _, r := range []string{app, clone, other, ""} {
			if u.Excludes(r) || u.Registered(r) {
				t.Errorf("unread registry: %q Excludes=%v Registered=%v, want neither", r, u.Excludes(r), u.Registered(r))
			}
		}
		f := u.NewFilter()
		for _, r := range []string{app, clone, other} {
			if !f.Keep(r) {
				t.Errorf("unread registry dropped %q", r)
			}
		}
		ign := f.Ignored()
		if ign.Rows != 0 || ign.RegistryNote != u.Note {
			t.Errorf("unread Ignored = %+v, want no rows and the note", ign)
		}
		if p := ign.Phrase("verify-ledger row"); !strings.Contains(p, "connection refused") || !strings.Contains(p, "none ignored") {
			t.Errorf("unread Phrase = %q, want it to say why nothing was ignored", p)
		}
	})

	t.Run("a filter keeps registered rows and names what it ignored", func(t *testing.T) {
		f := roots.NewFilter()
		rowRoots := []string{app, appLink + "/", lib, other, other, clone, filepath.Join(app, "sub"), ""}
		var kept int
		for _, r := range rowRoots {
			if f.Keep(r) {
				kept++
			}
		}
		ign := f.Ignored()
		if kept != 3 || ign.Rows != 5 || ign.ScratchRows != 1 || len(ign.Roots) != 4 {
			t.Fatalf("kept %d, Ignored %+v; want 3 kept, 5 rows under 4 roots, 1 scratch", kept, ign)
		}
		p := ign.Phrase("verify-ledger row")
		for _, want := range []string{"5 verify-ledger rows", "4 unregistered roots", "1 under a scratch dir", "ignored"} {
			if !strings.Contains(p, want) {
				t.Errorf("Phrase = %q, want %q in it", p, want)
			}
		}
		if (Ignored{}).Phrase("row") != "" {
			t.Error("nothing ignored from a read registry should phrase as empty")
		}
		var sum Ignored
		sum.Merge(ign)
		sum.Merge(ign)
		if sum.Rows != 10 || len(sum.Roots) != 4 || sum.ScratchRows != 2 {
			t.Errorf("Merge = %+v, want 10 rows, 4 distinct roots, 2 scratch", sum)
		}
	})

	t.Run("key names the registry a memo was computed under", func(t *testing.T) {
		same := RegisteredRootsWith([]string{lib, appLink, clone}, []string{scratch})
		if roots.Key() != same.Key() {
			t.Errorf("the same registry spelled differently keys differently: %q vs %q", roots.Key(), same.Key())
		}
		if roots.Key() == RegisteredRootsWith([]string{lib}, []string{scratch}).Key() || roots.Key() == Unread("x").Key() {
			t.Error("different registries share a key")
		}
	})
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
