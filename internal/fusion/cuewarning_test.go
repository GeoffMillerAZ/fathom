package fusion

import (
	"strings"
	"testing"
)

func TestCUEFailedPackagesWarningNamesCause(t *testing.T) {
	rep := TypedEdgeReport{PackagesTotal: 2, PackagesFailed: 1, FailedPackages: []FailedTypedPackage{{PkgID: "consumer", Err: "cannot find example.org/remote"}}}
	got := cueFailedPackagesWarning("svc", rep)
	for _, want := range []string{"1/2", "consumer", "example.org/remote"} {
		if !strings.Contains(got, want) {
			t.Fatalf("warning %q lacks %q", got, want)
		}
	}
}
