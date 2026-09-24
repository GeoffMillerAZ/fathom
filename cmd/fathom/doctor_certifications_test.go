package main

// doctor_certifications_test.go — TC-14's own test:
// TestDoctorCertifications_Row.
import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorCertifications_Row(t *testing.T) {
	t.Run("no store at all is a named skip, never a clean pass", func(t *testing.T) {
		c := checkCertifications(filepath.Join(t.TempDir(), "nope.sqlite"), "svc")
		if c.Status != doctorSkipped {
			t.Fatalf("Status = %q, want skipped", c.Status)
		}
		if !strings.Contains(c.Reason, "no manifests found") {
			t.Errorf("Reason = %q, want it to name 'no manifests found'", c.Reason)
		}
		if c.Remedy == "" {
			t.Error("expected a non-empty remedy")
		}
	})

	t.Run("a store with nothing declared is also a named skip", func(t *testing.T) {
		storePath := filepath.Join(t.TempDir(), "s.sqlite")
		var out, errBuf bytes.Buffer
		if code := run([]string{"ingest", "--service", "empty-svc", "--path", t.TempDir(), "--store", storePath}, &out, &errBuf); code != 0 {
			t.Fatalf("ingest exit = %d, stderr: %s", code, errBuf.String())
		}
		c := checkCertifications(storePath, "empty-svc")
		if c.Status != doctorSkipped {
			t.Fatalf("Status = %q, want skipped, reason=%q", c.Status, c.Reason)
		}
		if !strings.Contains(c.Reason, "no manifests found") {
			t.Errorf("Reason = %q, want it to name 'no manifests found'", c.Reason)
		}
	})

	t.Run("declared, live, gaps and the join rate, once compiled", func(t *testing.T) {
		_, storePath, service := setUpCertifyGapsFixture(t)
		c := checkCertifications(storePath, service)
		if c.Status != doctorPass {
			t.Fatalf("Status = %q, want pass, reason=%q", c.Status, c.Reason)
		}
		for _, want := range []string{"1 declared", "1 live", "0 gap(s)", "joined 1 of 1 cases"} {
			if !strings.Contains(c.Reason, want) {
				t.Errorf("Reason = %q, missing %q", c.Reason, want)
			}
		}
		if c.Remedy != "" {
			t.Errorf("Remedy = %q, want empty on a pass", c.Remedy)
		}
	})

	t.Run("suites declared but nothing compiled warns, never fails", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "tests", "cart.cue"), []byte(certifyGapsFixtureManifest), 0o644); err != nil {
			t.Fatal(err)
		}
		storePath := filepath.Join(t.TempDir(), "s.sqlite")
		service := "cart-svc"
		var out, errBuf bytes.Buffer
		if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &out, &errBuf); code != 0 {
			t.Fatalf("ingest exit = %d, stderr: %s", code, errBuf.String())
		}
		// Declares the suite (tests/cart.cue) without any `// certifies:`
		// marker or attested run — `fathom tests ingest --manifest` is
		// what loads tests/*.cue into test_suites; plain ingest alone
		// never does (TC-2's own scoping).
		var tiOut, tiErr bytes.Buffer
		if code := cmdTestsIngest([]string{"--service", service, "--store", storePath, "--manifest", root}, &tiOut, &tiErr); code != 0 {
			t.Fatalf("tests ingest exit = %d, stderr: %s", code, tiErr.String())
		}
		c := checkCertifications(storePath, service)
		if c.Status != doctorWarn {
			t.Fatalf("Status = %q, want warn (declared suite, nothing certified), reason=%q", c.Status, c.Reason)
		}
		if c.Status == doctorFail {
			t.Fatal("this row must never FAIL (never-FAIL doctrine)")
		}
	})
}
