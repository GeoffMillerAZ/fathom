package navigationproof_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/navigationproof"
)

// This consumes an explicitly selected private run, not fixtures from user apps.
// The expected digest comes from the pre-run freeze, never the observed report.
func TestNavigationQualityLive(t *testing.T) {
	dir, hasDir := os.LookupEnv("APPKIT_NAVIGATION_PROOF_ROOT")
	expected, hasExpected := os.LookupEnv("APPKIT_NAVIGATION_CONTRACT_SHA")
	if !hasDir && !hasExpected {
		t.Skip("set APPKIT_NAVIGATION_PROOF_ROOT and the pre-run APPKIT_NAVIGATION_CONTRACT_SHA")
	}
	if !hasDir || !hasExpected || strings.TrimSpace(dir) == "" || strings.TrimSpace(expected) == "" {
		t.Fatal("explicit private verification needs both root and pre-run contract digest")
	}
	if !filepath.IsAbs(dir) {
		t.Fatal("APPKIT_NAVIGATION_PROOF_ROOT must be an absolute private run directory")
	}
	read := func(name string) []byte {
		t.Helper()
		file := filepath.Join(dir, name)
		info, err := os.Lstat(file)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 256<<10 {
			t.Fatalf("%s is not bounded regular metadata: %v", name, err)
		}
		f, err := os.Open(file)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		body, err := io.ReadAll(io.LimitReader(f, (256<<10)+1))
		if err != nil || len(body) > 256<<10 {
			t.Fatalf("read %s: %v", name, err)
		}
		return body
	}
	result, err := navigationproof.Check(expected, read("contract.json"), read("report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "passed" || result.Verification != "not-verified" || len(result.Findings) != 0 {
		t.Fatalf("scoped navigation tasks failed: %+v", result)
	}
	t.Logf("24-case private input/contrast task passed; contract=%s; target remains %s", expected, result.Verification)
}
