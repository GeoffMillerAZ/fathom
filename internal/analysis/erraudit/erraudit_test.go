package erraudit_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/erraudit"
)

const ignoredErrSrc = `package foo

import "os"

func bad() {
	_ = os.Remove("/tmp/x") // ignored error
}
`

const logOnlySrc = `package foo

import "log"

func bad(err error) {
	if err != nil {
		log.Println(err)
	}
}
`

const logOnlyWithReturnSrc = `package foo

import "log"

func ok(err error) error {
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
`

const nakedReturnSrc = `package foo

import "errors"

func bad() error {
	err := errors.New("oops")
	return err
}
`

const cleanErrSrc = `package foo

import "fmt"

func good() error {
	return fmt.Errorf("wrapped: %w", fmt.Errorf("inner"))
}
`

func TestScan_IgnoredError(t *testing.T) {
	findings := erraudit.Scan("ign.go", ignoredErrSrc)
	if !anyMsg(findings, "ignored error") {
		t.Fatalf("expected ignored-error finding, got %v", findings)
	}
}

func TestScan_LogOnly(t *testing.T) {
	findings := erraudit.Scan("log.go", logOnlySrc)
	if !anyMsg(findings, "log-only") {
		t.Fatalf("expected log-only finding, got %v", findings)
	}
}

func TestScan_LogOnlyWithReturn_NoFinding(t *testing.T) {
	findings := erraudit.Scan("ok.go", logOnlyWithReturnSrc)
	for _, f := range findings {
		if strings.Contains(f.Message, "log-only") {
			t.Fatalf("unexpected log-only finding for handler that returns: %v", findings)
		}
	}
}

func TestScan_NakedReturn(t *testing.T) {
	findings := erraudit.Scan("naked.go", nakedReturnSrc)
	if !anyMsg(findings, "naked error return") {
		t.Fatalf("expected naked-error-return finding, got %v", findings)
	}
}

func TestScan_Clean(t *testing.T) {
	findings := erraudit.Scan("clean.go", cleanErrSrc)
	// fmt.Errorf chain — no ignored errors, no log-only, no naked return.
	for _, f := range findings {
		if strings.Contains(f.Message, "ignored") ||
			strings.Contains(f.Message, "log-only") ||
			strings.Contains(f.Message, "naked") {
			t.Fatalf("unexpected finding in clean code: %v", findings)
		}
	}
}

func TestScan_ParseError(t *testing.T) {
	findings := erraudit.Scan("bad.go", "not go { }")
	if len(findings) == 0 {
		t.Fatal("expected parse error finding")
	}
}

func TestScan_Deterministic(t *testing.T) {
	src := ignoredErrSrc + "\n" + logOnlySrc + "\n" + nakedReturnSrc
	a := erraudit.Scan("multi.go", src)
	b := erraudit.Scan("multi.go", src)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at [%d]: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func anyMsg(findings []erraudit.Finding, substr string) bool {
	for _, f := range findings {
		if strings.Contains(f.Message, substr) {
			return true
		}
	}
	return false
}
