package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTierRequired (round 3 A3, F3): -tier has no default: a run that names
// none is a usage error (exit 2), never an output read as tier A.
func TestTierRequired(t *testing.T) {
	if os.Getenv("ORACLE_MAIN") == "1" {
		os.Args = append([]string{"oracle"}, strings.Fields(os.Getenv("ORACLE_ARGS"))...)
		main()
		return
	}
	dir := t.TempDir()
	db := filepath.Join(dir, "compile_commands.json")
	if err := os.WriteFile(db, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestTierRequired$")
	cmd.Env = append(os.Environ(), "ORACLE_MAIN=1", "ORACLE_ARGS=-subject "+dir+" -compdb "+db)
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("no -tier: %v, want exit 2 (usage)", err)
	}
}

// TestCalledNameKernelLaunch (round 3 C): a CUDA kernel launch is a call
// (the compiler's CUDAKernelCallExpr): `k<<<g, b>>>(x)` and
// `k<T, 1><<<g, b>>>(x)` are call-shaped sites of k, as `k(x)` is.
func TestCalledNameKernelLaunch(t *testing.T) {
	for _, c := range []struct{ src, callee string }{
		{"k<<<grid, block>>>(p)", "k"},
		{"reduce6<T, 1, false><<<g, b, s>>>(d)", "reduce6"},
		{"k <<< 1, 1 >>> (p)", "k"},
	} {
		src := []byte(c.src)
		end := strings.Index(c.src, "<<<")
		for end > 0 && src[end-1] == ' ' {
			end--
		}
		if got, _, ok := calledName(src, 0, end); !ok || got != c.callee {
			t.Errorf("calledName(%q) = %q, %v; want %q, true", c.src, got, ok, c.callee)
		}
	}
	if _, _, ok := calledName([]byte("a << b"), 0, 1); ok {
		t.Error("a shift is no call")
	}
}
