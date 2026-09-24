package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/indexbench"
)

// paymentCertifyTest is staged into the fauxtown payment-service fixture
// (internal/indexbench, the same fixture VB-20's other three receipt-kind
// golden tests use): TestChargeReachesValidate calls Charge, which calls
// validateCharge (payment.go) — a real, indexed, two-hop reach chain — and
// TestChargeRejectsEmptyAccount is a genuinely FAILING test (asserts a nil
// error from a call that returns one), the fixture's --run-tests fail case.
const paymentCertifyTest = `package payment

import (
	"context"
	"testing"
)

func TestChargeReachesValidate(t *testing.T) {
	_, err := Charge(context.Background(), ChargeRequest{AccountID: "a1", Amount: 100})
	if err != nil {
		t.Fatalf("Charge: %v", err)
	}
}

func TestChargeRejectsEmptyAccount(t *testing.T) {
	_, err := Charge(context.Background(), ChargeRequest{Amount: 100})
	if err != nil {
		t.Fatal("expected no error (deliberately wrong assertion — this test fails on purpose)")
	}
}
`

// stageCertifyFixture stages the payment-service fauxtown module plus
// paymentCertifyTest and commits it, returning (root, commit).
func stageCertifyFixture(t *testing.T) (string, string) {
	t.Helper()
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "payment_test.go"), []byte(paymentCertifyTest), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return root, initReceiptTestRepo(t, root)
}

// TestReceiptVerify_CertificationKind is TC-13's contract test
// (docs/plans/test-certifications.md, reachable_from verifyCertificationReceipt):
// a certification receipt's reach re-derives independently with go/types
// from a bare checkout; without --run-tests the run verdict is always
// "unverified (no run)" (ADR-043 decision 5 — a reached test is not a
// passing test); with --run-tests a genuinely passing test reports
// pass: true and a genuinely failing one reports pass: false and exit 1;
// a tampered Reached entry names the difference rather than passing
// silently.
func TestReceiptVerify_CertificationKind(t *testing.T) {
	root, commit := stageCertifyFixture(t)

	t.Run("unverified without run-tests", func(t *testing.T) {
		receiptPath := writeReceiptFile(t, core.Receipt{
			Kind:        core.ReceiptKindCertification,
			Test:        "TestChargeReachesValidate",
			Target:      "invariant:I-fixture",
			Strength:    "reaches",
			IndexCommit: commit,
			Reached:     []string{"validateCharge"},
		})
		var stdout, stderr bytes.Buffer
		code := cmdReceipt([]string{"verify", "--repo", root, receiptPath}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "pass: unverified (no run)") {
			t.Errorf("stdout = %q, want an unverified run line", stdout.String())
		}
	})

	t.Run("run-tests passing", func(t *testing.T) {
		receiptPath := writeReceiptFile(t, core.Receipt{
			Kind:        core.ReceiptKindCertification,
			Test:        "TestChargeReachesValidate",
			IndexCommit: commit,
			Reached:     []string{"Charge", "validateCharge"},
		})
		var stdout, stderr bytes.Buffer
		code := cmdReceipt([]string{"verify", "--repo", root, "--run-tests", receiptPath}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "pass: true") {
			t.Errorf("stdout = %q, want pass: true", stdout.String())
		}
	})

	t.Run("run-tests failing", func(t *testing.T) {
		receiptPath := writeReceiptFile(t, core.Receipt{
			Kind:        core.ReceiptKindCertification,
			Test:        "TestChargeRejectsEmptyAccount",
			IndexCommit: commit,
			Reached:     []string{"Charge"},
		})
		var stdout, stderr bytes.Buffer
		code := cmdReceipt([]string{"verify", "--repo", root, "--run-tests", receiptPath}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "pass: false") || !strings.Contains(stdout.String(), "FAIL") {
			t.Errorf("stdout = %q, want pass: false and a FAIL line", stdout.String())
		}
	})

	t.Run("tampered reached names the difference", func(t *testing.T) {
		receiptPath := writeReceiptFile(t, core.Receipt{
			Kind:        core.ReceiptKindCertification,
			Test:        "TestChargeReachesValidate",
			IndexCommit: commit,
			Reached:     []string{"validateCharge", "NotAReachedSymbol"},
		})
		var stdout, stderr bytes.Buffer
		code := cmdReceipt([]string{"verify", "--repo", root, receiptPath}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "NotAReachedSymbol") {
			t.Errorf("stdout = %q, want the tampered symbol named", stdout.String())
		}
	})
}
