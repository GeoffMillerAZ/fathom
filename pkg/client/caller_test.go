package client

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestCallerHeaderTestFlagFromEnv (RT-164, §10.4): test=1 comes from
// FATHOM_TEST=1 as well as testing.Testing(), so a child process a test
// execs (where testing.Testing() is false) is still marked; the header
// names pid, ppid, origin and session and cannot be forged into extra
// fields by a hostile origin or session value.
func TestCallerHeaderTestFlagFromEnv(t *testing.T) {
	if callerTestFlag(false, "") {
		t.Fatal("not a test binary and no FATHOM_TEST: test flag set")
	}
	if !callerTestFlag(false, "1") {
		t.Fatal("FATHOM_TEST=1 in a child process: test flag not set")
	}
	if !callerTestFlag(true, "") {
		t.Fatal("testing.Testing(): test flag not set")
	}

	t.Setenv(testisolate.EnvTest, "")
	h := CallerHeader("edit-hook", "sess;test=0")
	want := "pid=" + strconv.Itoa(os.Getpid()) + ";ppid=" + strconv.Itoa(os.Getppid()) + ";origin=edit-hook;session=sess_test_0;test=1;cwd=" + callerCwd()
	if h != want {
		t.Fatalf("CallerHeader = %q, want %q", h, want)
	}
	c, ok := ParseCallerHeader(h)
	if !ok || !c.Test || c.Origin != "edit-hook" || c.Session != "sess_test_0" || c.PID != os.Getpid() {
		t.Fatalf("round trip: %+v ok=%v", c, ok)
	}
	if strings.Count(h, "test=") != 1 {
		t.Fatalf("a session value forged a second test field: %q", h)
	}
	if _, ok := ParseCallerHeader("garbage"); ok {
		t.Fatal("ParseCallerHeader accepted a header with no fields")
	}
}
