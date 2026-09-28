package tsengine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCompilationCacheUserScopeOnly is LB-F26 (design §8.3): wazero's
// compilation cache lives only under the user's ~/.fathom/cache/wazero/
// <wazero version>/, is off unless the managed daemon turns it on, and
// accepts no other location — no repository or relative path reaches it.
func TestCompilationCacheUserScopeOnly(t *testing.T) {
	home := t.TempDir()
	dir, err := compilationCacheDir(home)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".fathom", "cache", "wazero", WazeroVersion())
	if dir != want {
		t.Fatalf("cache dir %q, want %q", dir, want)
	}
	if WazeroVersion() == "" || strings.ContainsAny(WazeroVersion(), `/\`) {
		t.Fatalf("wazero version %q is not a safe directory name", WazeroVersion())
	}
	for _, bad := range []string{"", "relative/home", "."} {
		if _, err := compilationCacheDir(bad); err == nil {
			t.Errorf("home %q must be refused", bad)
		}
	}

	// Off by default: a process that never enables it compiles in memory.
	if got := CompilationCacheDir(); got != "" {
		t.Fatalf("the cache is on by default (%q); only the managed daemon enables it", got)
	}

	// On, a compile writes machine code under the user-scope dir, and a
	// second runtime reads it back instead of recompiling.
	ctx := context.Background()
	r1, cache1, err := newRuntime(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := openGrammarBlob("python")
	if err != nil {
		t.Fatal(err)
	}
	cm, err := r1.CompileModule(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	_ = cm.Close(ctx)
	_ = r1.Close(ctx)
	_ = cache1.Close(ctx)
	n := dirBytes(t, dir)
	if n == 0 {
		t.Fatalf("no compiled code cached under %s", dir)
	}
	t.Logf("python: %d module bytes compiled to %d cached bytes", len(raw), n)

	// Off, the runtime has no cache at all.
	r2, cache2, err := newRuntime(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if cache2 != nil {
		t.Fatal("newRuntime with no dir built a cache")
	}
	cm2, err := r2.CompileModule(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	_ = cm2.Close(ctx)
	_ = r2.Close(ctx)
}

func TestWazeroVersionMatchesGoMod(t *testing.T) {
	b, err := os.ReadFile("../../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "github.com/tetratelabs/wazero" {
			if f[1] != WazeroVersion() {
				t.Fatalf("go.mod has wazero %s, wazeroVersion is %s", f[1], WazeroVersion())
			}
			return
		}
	}
	t.Fatal("go.mod does not require github.com/tetratelabs/wazero")
}

func TestEnableCompilationCacheBeforeRuntimeOnly(t *testing.T) {
	// The process runtime exists once any parser was made; enabling after
	// that must say so rather than silently doing nothing.
	p, err := NewParser(context.Background(), "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	p.Close()
	if err := EnableCompilationCache(t.TempDir()); err == nil {
		t.Fatal("enabling the cache after the runtime exists must be an error")
	}
}

func TestPruneCompilationCache(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	root := filepath.Join(home, ".fathom", "cache", "wazero")
	cur := filepath.Join(root, WazeroVersion())
	touch := func(p string, at time.Time) {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, at, at)
	}
	touch(filepath.Join(root, "v0.0.1", "a"), now)              // another wazero version: stale
	touch(filepath.Join(cur, "fresh"), now.Add(-time.Hour))     // current, recent: kept
	touch(filepath.Join(cur, "old"), now.Add(-40*24*time.Hour)) // current, untouched 40 days: aged out
	got := PrunableCompilationCache(home, now)
	var names []string
	for _, p := range got {
		names = append(names, strings.TrimPrefix(p, root+string(filepath.Separator)))
	}
	want := []string{"v0.0.1", filepath.Join(WazeroVersion(), "old")}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("prunable %v, want %v", names, want)
	}
	if PrunableCompilationCache(t.TempDir(), now) != nil {
		t.Fatal("a home with no cache has nothing to prune")
	}
}

func dirBytes(t *testing.T, dir string) int64 {
	t.Helper()
	var n int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			n += info.Size()
		}
		return nil
	})
	return n
}
