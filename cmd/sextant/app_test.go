package main

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fauxPaymentPath returns the absolute path to the testdata/fauxtown/payment-service
// directory, resolved relative to this source file so the test is working-directory
// independent.
func fauxPaymentPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "../../internal/connector/testdata/fauxtown/payment-service"
	}
	return filepath.Join(filepath.Dir(file),
		"../../internal/connector/testdata/fauxtown/payment-service")
}

// ─── version ────────────────────────────────────────────────────────────────

func TestVersion(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"version"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("version exited %d; stderr=%q", code, errBuf.String())
	}
	if got := strings.TrimSpace(out.String()); got == "" {
		t.Fatal("version printed nothing to stdout")
	}
}

// ─── help ────────────────────────────────────────────────────────────────────

func TestHelp(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("help exited %d; stderr=%q", code, errBuf.String())
	}
	combined := out.String() + errBuf.String()
	if !strings.Contains(combined, "ingest") {
		t.Errorf("help output missing 'ingest'; got:\n%s", combined)
	}
}

// ─── no subcommand ────────────────────────────────────────────────────────────

func TestNoSubcommand(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when no subcommand given")
	}
}

func TestUnknownSubcommand(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"bogus"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit for unknown subcommand")
	}
	if !strings.Contains(errBuf.String(), "bogus") {
		t.Errorf("stderr should mention unknown subcommand; got %q", errBuf.String())
	}
}

// ─── query: missing --concern ─────────────────────────────────────────────────

func TestQueryMissingConcern(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"query", "--service", "svc"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --concern is missing")
	}
	if errBuf.String() == "" {
		t.Fatal("expected a non-empty stderr message")
	}
}

// ─── query: missing --service ─────────────────────────────────────────────────

func TestQueryMissingService(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"query", "--concern", "payments"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --service is missing")
	}
}

// ─── ingest: missing --service ────────────────────────────────────────────────

func TestIngestMissingService(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"ingest", "--path", "/tmp"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --service is missing")
	}
}

// ─── inspect: missing --service ───────────────────────────────────────────────

func TestInspectMissingService(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"inspect"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --service is missing")
	}
}

// ─── ingest then query (end-to-end Faux fixture) ─────────────────────────────
//
// Uses the in-repo payment-service fixture via the Faux connector so no network
// or external dependencies are required.  The store lives in a temp dir, keeping
// the test hermetic.

func TestIngestThenQuery(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.duckdb")
	paymentPath := fauxPaymentPath()

	// Step 1 — ingest using ModeFaux (--path is ignored by the faux connector but
	// we set it to the payment-service subdir so LocalConnector callers can also
	// work; the faux connector resolves the service by --service name which must
	// match the fauxtown subdirectory).
	var ingestOut, ingestErr bytes.Buffer
	code := run([]string{
		"ingest",
		"--service", "payment-service",
		"--path", paymentPath,
		"--store", store,
		"--mode", "faux",
	}, &ingestOut, &ingestErr)
	if code != 0 {
		t.Fatalf("ingest exited %d\nstdout=%s\nstderr=%s", code, ingestOut.String(), ingestErr.String())
	}
	// Ingest output should mention at least one symbol or file count.
	ingestOutput := ingestOut.String()
	if ingestOutput == "" {
		t.Fatal("ingest produced no output")
	}

	// Step 2 — query for a symbol we know the payment-service fixture contains.
	// "Charge" is a top-level exported function in payment.go.
	var queryOut, queryErr bytes.Buffer
	code = run([]string{
		"query",
		"--service", "payment-service",
		"--concern", "payments",
		"--keywords", "Charge",
		"--store", store,
	}, &queryOut, &queryErr)
	if code != 0 {
		t.Fatalf("query exited %d\nstdout=%s\nstderr=%s", code, queryOut.String(), queryErr.String())
	}
	queryOutput := queryOut.String()
	if !strings.Contains(queryOutput, "Charge") {
		t.Errorf("query output should contain 'Charge'; got:\n%s", queryOutput)
	}
}

// ─── ingest then inspect (end-to-end Faux fixture) ───────────────────────────

func TestIngestThenInspect(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.duckdb")
	paymentPath := fauxPaymentPath()

	var ingestOut, ingestErr bytes.Buffer
	code := run([]string{
		"ingest",
		"--service", "payment-service",
		"--path", paymentPath,
		"--store", store,
		"--mode", "faux",
	}, &ingestOut, &ingestErr)
	if code != 0 {
		t.Fatalf("ingest exited %d; stderr=%s", code, ingestErr.String())
	}

	var inspectOut, inspectErr bytes.Buffer
	code = run([]string{
		"inspect",
		"--service", "payment-service",
		"--store", store,
	}, &inspectOut, &inspectErr)
	if code != 0 {
		t.Fatalf("inspect exited %d; stderr=%s", code, inspectErr.String())
	}
	if !strings.Contains(inspectOut.String(), "payment-service") {
		t.Errorf("inspect output should mention service; got:\n%s", inspectOut.String())
	}
}

// ─── table tests ─────────────────────────────────────────────────────────────

func TestRunTable(t *testing.T) {
	rows := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string // substring expected in stdout (empty = don't check)
		wantErr  string // substring expected in stderr (empty = don't check)
	}{
		{
			name:     "version outputs sextant",
			args:     []string{"version"},
			wantCode: 0,
			wantOut:  "sextant",
		},
		{
			name:     "query no concern nonzero",
			args:     []string{"query", "--service", "s"},
			wantCode: 1,
		},
		{
			name:     "query no service nonzero",
			args:     []string{"query", "--concern", "c"},
			wantCode: 1,
		},
		{
			name:     "ingest no service nonzero",
			args:     []string{"ingest"},
			wantCode: 1,
		},
		{
			name:     "unknown subcommand nonzero",
			args:     []string{"notacommand"},
			wantCode: 1,
		},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			got := run(row.args, &out, &errBuf)
			if got != row.wantCode {
				t.Errorf("exit code = %d, want %d (stdout=%q stderr=%q)",
					got, row.wantCode, out.String(), errBuf.String())
			}
			if row.wantOut != "" && !strings.Contains(out.String(), row.wantOut) {
				t.Errorf("stdout %q does not contain %q", out.String(), row.wantOut)
			}
			if row.wantErr != "" && !strings.Contains(errBuf.String(), row.wantErr) {
				t.Errorf("stderr %q does not contain %q", errBuf.String(), row.wantErr)
			}
		})
	}
}
