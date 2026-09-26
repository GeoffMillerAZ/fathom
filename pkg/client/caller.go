package client

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// CallerHeaderName carries the caller's process identity to the daemon
// (ADR-047, docs/design/responsible-tenant.md §10.4): which process, its
// parent, its channel, its session, and whether it is a test.
const CallerHeaderName = "X-Fathom-Caller"

// Caller is a parsed X-Fathom-Caller value.
type Caller struct {
	PID, PPID       int
	Origin, Session string
	Test            bool
}

// CallerHeader renders this process's X-Fathom-Caller value:
// pid=…;ppid=…;origin=…;session=…;test=0|1. test=1 is derived from
// FATHOM_TEST=1 as well as testing.Testing(), because testing.Testing()
// is false in a process a test execs (RT-164) and the env var is
// inherited through os.Environ(). origin and session are sanitised so a
// value can never add or override a field.
func CallerHeader(origin, session string) string {
	test := "0"
	if callerTestFlag(testing.Testing(), os.Getenv(testisolate.EnvTest)) {
		test = "1"
	}
	return "pid=" + strconv.Itoa(os.Getpid()) +
		";ppid=" + strconv.Itoa(os.Getppid()) +
		";origin=" + callerField(origin) +
		";session=" + callerField(session) +
		";test=" + test
}

func callerTestFlag(inTestBinary bool, envTest string) bool {
	return inTestBinary || envTest == "1"
}

// callerField keeps [A-Za-z0-9._:/-] and maps everything else to '_', so
// ';' and '=' cannot reach the wire; it caps the value at 128 bytes.
func callerField(v string) string {
	if len(v) > 128 {
		v = v[:128]
	}
	b := []byte(v)
	for i, c := range b {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '.', c == '_', c == ':', c == '/', c == '-':
		default:
			b[i] = '_'
		}
	}
	return string(b)
}

// ParseCallerHeader parses an X-Fathom-Caller value; ok is false when no
// known field is present. Unknown fields are ignored (forward compatible).
func ParseCallerHeader(v string) (Caller, bool) {
	var c Caller
	ok := false
	for _, part := range strings.Split(v, ";") {
		k, val, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(k) {
		case "pid":
			c.PID, _ = strconv.Atoi(val)
			ok = true
		case "ppid":
			c.PPID, _ = strconv.Atoi(val)
			ok = true
		case "origin":
			c.Origin = val
			ok = true
		case "session":
			c.Session = val
			ok = true
		case "test":
			c.Test = val == "1"
			ok = true
		}
	}
	return c, ok
}
