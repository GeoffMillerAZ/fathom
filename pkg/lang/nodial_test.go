//go:build unix

package lang_test

import (
	"context"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// LL-21 (design section 8, row 2): the library opens no socket and writes no
// file unless it was given a cache directory. net stays in the closure through
// wazero, so this test watches behavior instead of imports. The work runs in a
// child process with HOME, TMPDIR, XDG_CACHE_HOME and the working directory
// pointed at empty scratch directories, and a sampler that looks for a socket
// in the descriptor table the whole time it runs.

const noDialModeEnv = "LANG_NODIAL_MODE"

// socketFDs returns the descriptors that are sockets right now.
func socketFDs() []int {
	var out []int
	for fd := 0; fd < 512; fd++ {
		var st syscall.Stat_t
		if syscall.Fstat(fd, &st) == nil && st.Mode&syscall.S_IFMT == syscall.S_IFSOCK {
			out = append(out, fd)
		}
	}
	return out
}

// watchSockets samples the descriptor table until stop is called and returns
// every socket descriptor it ever saw.
func watchSockets() (stop func() []int) {
	var done atomic.Bool
	var wg sync.WaitGroup
	seen := map[int]bool{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !done.Load() {
			for _, fd := range socketFDs() {
				seen[fd] = true
			}
			time.Sleep(20 * time.Microsecond)
		}
	}()
	return func() []int {
		done.Store(true)
		wg.Wait()
		var out []int
		for fd := range seen {
			out = append(out, fd)
		}
		sort.Ints(out)
		return out
	}
}

var noDialFiles = map[string]string{
	"infra/main.tf":  tfSrc,
	"a/a.go":         "package a\n\nfunc F() { G() }\n\nfunc G() {}\n",
	"app.py":         "def f():\n    g()\n\ndef g():\n    pass\n",
	"rules.yml":      "groups:\n- name: g\n  rules:\n  - alert: High\n    expr: up == 0\n",
	"Dockerfile":     "FROM alpine\nRUN echo\n",
	"README.md":      "# Title\n\nSee [a](a/a.go).\n",
	"Makefile":       "all: b\nb:\n\techo hi\n",
	"web/index.html": "<html><body><a href=\"x.js\">x</a></body></html>\n",
}

// TestNoDialHelper is the child process of TestLangNeverDialsOrWrites.
func TestNoDialHelper(t *testing.T) {
	mode := os.Getenv(noDialModeEnv)
	if mode == "" {
		t.Skip("runs in the child process of TestLangNeverDialsOrWrites")
	}
	// A dial through the default resolvers or transport is refused loudly.
	var dials atomic.Int64
	refuse := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dials.Add(1)
		return nil, &net.OpError{Op: "dial", Net: network, Err: os.ErrPermission}
	}
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: refuse}
	http.DefaultTransport = &http.Transport{DialContext: refuse}

	if mode == "probe" {
		// The detector must see a socket when there is one, or a clean run
		// proves nothing.
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skipf("no loopback to probe with: %v", err)
		}
		defer l.Close()
		stop := watchSockets()
		time.Sleep(5 * time.Millisecond)
		if got := stop(); len(got) == 0 {
			t.Fatal("the sampler saw no socket while a listener was open")
		}
		t.Log("DETECTED")
		return
	}

	before := socketFDs()
	stop := watchSockets()
	if mode == "cache" {
		if err := lang.EnableCompilationCache(os.Getenv("LANG_NODIAL_CACHE")); err != nil {
			t.Fatal(err)
		}
	}
	r, err := lang.New()
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(noDialFiles))
	for p := range noDialFiles {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if _, err := r.Extract(context.Background(), lang.File{Path: p, Src: []byte(noDialFiles[p]), Siblings: paths}); err != nil {
			t.Fatalf("Extract %s: %v", p, err)
		}
	}
	r.Dialects()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	seen := stop()
	// Descriptors that existed before the work (the runtime's own) are not it.
	var fresh []int
	for _, fd := range seen {
		known := false
		for _, b := range before {
			known = known || b == fd
		}
		if !known {
			fresh = append(fresh, fd)
		}
	}
	if len(fresh) > 0 {
		t.Fatalf("SOCKETS %v opened while reading files", fresh)
	}
	if n := dials.Load(); n != 0 {
		t.Fatalf("DIALS %d through the default resolver or transport", n)
	}
	t.Log("CLEAN")
}

// runNoDialChild runs TestNoDialHelper in mode with every writable root
// pointed into scratch, and returns the child's output and the scratch dirs.
func runNoDialChild(t *testing.T, mode string, extra ...string) (out string, dirs map[string]string) {
	t.Helper()
	root := t.TempDir()
	dirs = map[string]string{}
	for _, d := range []string{"home", "tmp", "xdg", "cwd", "cache"} {
		dirs[d] = filepath.Join(root, d)
		if err := os.Mkdir(dirs[d], 0o700); err != nil {
			t.Fatal(err)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestNoDialHelper$", "-test.v")
	cmd.Dir = dirs["cwd"]
	// The parent's environment first, then the overrides: the last value of a
	// name wins, so HOME, TMPDIR and the cache variables point into scratch.
	cmd.Env = append(append(os.Environ(),
		"HOME="+dirs["home"], "TMPDIR="+dirs["tmp"], "XDG_CACHE_HOME="+dirs["xdg"],
		"GOCACHE="+dirs["xdg"], "GOTMPDIR="+dirs["tmp"],
		noDialModeEnv+"="+mode, "LANG_NODIAL_CACHE="+dirs["cache"],
	), extra...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child (%s): %v\n%s", mode, err, b)
	}
	return string(b), dirs
}

// written lists the files and directories under dir.
func written(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != dir {
			out = append(out, strings.TrimPrefix(p, dir+string(filepath.Separator)))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLangNeverDialsOrWrites(t *testing.T) {
	// The detector works: it sees a socket that is really there.
	if out, _ := runNoDialChild(t, "probe"); !strings.Contains(out, "DETECTED") {
		t.Fatalf("the socket sampler is blind; a clean run below would prove nothing:\n%s", out)
	}

	// No cache directory: no socket, and nothing written anywhere it could.
	out, dirs := runNoDialChild(t, "read")
	if !strings.Contains(out, "CLEAN") {
		t.Fatalf("child did not report a clean run:\n%s", out)
	}
	for name, dir := range dirs {
		if w := written(t, dir); len(w) != 0 {
			t.Errorf("reading files wrote under %s: %v", name, w)
		}
	}

	// With a cache directory the library writes there and nowhere else.
	out, dirs = runNoDialChild(t, "cache")
	if !strings.Contains(out, "CLEAN") {
		t.Fatalf("child with a cache did not report a clean run:\n%s", out)
	}
	if w := written(t, dirs["cache"]); len(w) == 0 {
		t.Error("the cache directory stayed empty, so this check cannot see a write")
	}
	for _, name := range []string{"home", "tmp", "xdg", "cwd"} {
		if w := written(t, dirs[name]); len(w) != 0 {
			t.Errorf("enabling the cache wrote under %s: %v", name, w)
		}
	}
}
