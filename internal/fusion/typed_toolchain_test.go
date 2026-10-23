package fusion_test

import (
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// O25: the typed pass type-checks with the binary's own go/types but runs the
// go on PATH. A binary built by a Go older than go.mod's toolchain line cannot
// read the standard library of the newer go the adopter has. CI builds with
// go.mod's toolchain (setup-go go-version-file), so a drift fails here.
func TestToolchainMatchesGoMod(t *testing.T) {
	if os.Getenv("GOTOOLCHAIN") == "local" {
		t.Skip("GOTOOLCHAIN=local: the toolchain line is not honored on purpose")
	}
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^toolchain go(\d+)\.(\d+)(?:\.(\d+))?\s*$`).FindStringSubmatch(string(mod))
	if m == nil {
		t.Skip("go.mod has no toolchain line")
	}
	run := regexp.MustCompile(`^go(\d+)\.(\d+)(?:\.(\d+))?`).FindStringSubmatch(runtime.Version())
	if run == nil {
		t.Skipf("unparseable runtime version %q", runtime.Version())
	}
	num := func(s string) int { n, _ := strconv.Atoi(s); return n }
	want := [3]int{num(m[1]), num(m[2]), num(m[3])}
	got := [3]int{num(run[1]), num(run[2]), num(run[3])}
	for i := range want {
		if got[i] > want[i] {
			return
		}
		if got[i] < want[i] {
			t.Fatalf("built with %s, older than go.mod's %s: typed Go loads would fail on a newer go's standard library", runtime.Version(), strings.TrimSpace(m[0]))
		}
	}
}
