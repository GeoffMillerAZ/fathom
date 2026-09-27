package client

import (
	"os"
	"testing"
)

// RT-86: X-Fathom-Caller carries the caller's working directory (percent-
// encoded: a path's spaces, ';' and '=' cannot break the header), so the
// daemon can default references to the caller's own service.
func TestCallerHeaderCarriesCwd(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	c, ok := ParseCallerHeader(CallerHeader("cli", ""))
	if !ok || c.Cwd != wd {
		t.Fatalf("parsed cwd = %q (ok=%v), want %q", c.Cwd, ok, wd)
	}
	odd := "/tmp/a b;test=0=x/%/ü"
	c, _ = ParseCallerHeader("pid=1;cwd=" + encodeCallerValue(odd) + ";test=1")
	if c.Cwd != odd || !c.Test {
		t.Fatalf("round trip = %+v, want cwd %q and test", c, odd)
	}
}
