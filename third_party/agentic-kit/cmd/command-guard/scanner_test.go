package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testScannerPolicy(t *testing.T) scannerPolicy {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(b)
	return scannerPolicy{Kind: "gitleaks", Profile: "gitleaks-stdin-8.30.1", Executable: exe, SHA256: hex.EncodeToString(h[:]), TimeoutMillis: 3000, MaxReportBytes: 1 << 20}
}
func scannerTestLog(n, count int) []byte {
	tail := "no leaks found"
	level := "INF"
	if count > 0 {
		tail = fmt.Sprintf("leaks found: %d", count)
		level = "WRN"
	}
	return []byte(fmt.Sprintf("12:01AM INF scanned ~%d bytes (%d bytes) in 1ms\n12:01AM %s %s\n", n, n, level, tail))
}
func wrapScannerTest(f func(context.Context, string, string, []byte, int, ...string) ([]byte, []byte, int, bool)) scannerRunner {
	return func(ctx context.Context, exe, dir string, in []byte, cap int, args ...string) scannerRunResult {
		o, e, x, ok := f(ctx, exe, dir, in, cap, args...)
		if !ok {
			return scannerRunResult{code: "execution_incomplete"}
		}
		return scannerRunResult{out: o, errout: e, exit: x, started: true}
	}
}
func TestScannerSelectedAndFailures(t *testing.T) {
	c := testScannerPolicy(t)
	data := [][]byte{[]byte("first\n"), []byte("late secret\n")}
	for _, tc := range []struct {
		name   string
		mutate func([]byte, []byte, int, int) ([]byte, []byte, int, bool)
		want   string
		done   int
	}{
		{"clean", nil, "complete", 2},
		{"finding late", func(o, e []byte, x, i int) ([]byte, []byte, int, bool) {
			if i == 2 {
				return []byte(`[{"RuleID":"generic-api-key","Secret":"[REDACTED]","Match":"[REDACTED]","File":"stdin","StartLine":1,"Tags":[]}]`), scannerTestLog(12, 1), 3, true
			}
			return o, e, x, true
		}, "finding", 2},
		{"byte mismatch", func(o, e []byte, x, i int) ([]byte, []byte, int, bool) {
			if i == 2 {
				return o, scannerTestLog(0, 0), x, true
			}
			return o, e, x, true
		}, "incomplete", 1},
		{"extra log", func(o, e []byte, x, i int) ([]byte, []byte, int, bool) {
			if i == 2 {
				return o, append(e, []byte("12:01AM INF extra\n")...), x, true
			}
			return o, e, x, true
		}, "incomplete", 1},
		{"malformed report", func(o, e []byte, x, i int) ([]byte, []byte, int, bool) {
			if i == 2 {
				return []byte(`[{"RuleID":"a","RuleID":"b"}]`), e, x, true
			}
			return o, e, x, true
		}, "incomplete", 1},
		{"version", func(o, e []byte, x, i int) ([]byte, []byte, int, bool) {
			if i == -1 {
				return []byte("8.30.0\n"), e, x, true
			}
			return o, e, x, true
		}, "incomplete", 0},
		{"timeout", func(o, e []byte, x, i int) ([]byte, []byte, int, bool) {
			if i == 2 {
				return nil, nil, -1, false
			}
			return o, e, x, true
		}, "incomplete", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			invoke := func(ctx context.Context, exe, dir string, in []byte, cap int, args ...string) ([]byte, []byte, int, bool) {
				if filepath.Dir(exe) != filepath.Dir(dir) || exe == c.Executable {
					t.Errorf("execution path not private copy")
				}
				if len(args) == 1 {
					if args[0] != "version" {
						t.Error(args)
					}
					o, e, x, ok := []byte("8.30.1\n"), []byte(nil), 0, true
					if tc.mutate != nil {
						return tc.mutate(o, e, x, -1)
					}
					return o, e, x, ok
				}
				if len(args) != 15 || args[0] != "stdin" || args[1] != "--report-path" || args[2] != "-" || args[len(args)-1] != "--ignore-gitleaks-allow" {
					t.Errorf("recipe %v", args)
				}
				calls++
				if !bytes.Equal(in, data[calls-1]) {
					t.Error("snapshot mismatch")
				}
				o, e, x, ok := []byte("[]"), scannerTestLog(len(in), 0), 0, true
				if tc.mutate != nil {
					return tc.mutate(o, e, x, calls)
				}
				return o, e, x, ok
			}
			r := &scannerReceipt{ExpectedFiles: 2}
			ok := scanSelectedWithRunner(c, data, r, wrapScannerTest(invoke))
			if ok != (tc.want == "complete" || tc.want == "finding") || r.Status != tc.want || r.CompletedFiles != tc.done {
				t.Fatalf("ok=%v receipt=%+v", ok, r)
			}
			if tc.want == "finding" && r.Findings != 1 {
				t.Fatal(r)
			}
		})
	}
}
func TestScannerControlMismatch(t *testing.T) {
	c := testScannerPolicy(t)
	s := scope{}
	p := policy{Scanner: &c}
	if validScannerControls(s, p) {
		t.Fatal("policy without scope")
	}
	s.RequiredScannerIDs = []string{"gitleaks"}
	if !validScannerControls(s, p) {
		t.Fatal("valid")
	}
	p.Scanner = nil
	if validScannerControls(s, p) {
		t.Fatal("scope without policy")
	}
}
func TestScannerReportRejectsUntrustedJSON(t *testing.T) {
	for _, b := range []string{`{}`, `null`, `[null]`, `[{"Secret":null}]`, `[{"FutureField":1}]`, `[{"Tags":[1]}]`, `[{"RuleID":"x","RuleID":"y"}]`, `[] {}`, `["x"]`} {
		if _, ok := scannerFindings([]byte(b)); ok {
			t.Errorf("accepted %s", b)
		}
	}
}
func TestScannerLogStrict(t *testing.T) {
	for _, b := range []string{string(scannerTestLog(0, 0)), "12:01AM INF scanned ~5 bytes (5 bytes) in 1ms\n12:01AM WRN leaks found: 1\n"} {
		_, ok := scannerLog([]byte(b), 5, 0, 0)
		if ok {
			t.Errorf("accepted %q", b)
		}
	}
	if _, ok := scannerLog(scannerTestLog(0, 0), 0, 0, 0); !ok {
		t.Fatal("empty input")
	}
	if _, ok := scannerLog(scannerTestLog(0, 0), 40, 0, 0); ok {
		t.Fatal("nonempty PDF input falsely complete")
	}
	if n, ok := scannerLog([]byte("12:09AM INF scanned ~4608 bytes (4.61 KB) in 76.3ms\n12:09AM INF no leaks found\n"), 4608, 0, 0); !ok || n != 4608 {
		t.Fatal("raw byte count must survive human formatting")
	}
}
func TestScannerStartFailureReceipt(t *testing.T) {
	c := testScannerPolicy(t)
	r := &scannerReceipt{ExpectedFiles: 1}
	invoke := func(ctx context.Context, exe, dir string, in []byte, cap int, args ...string) scannerRunResult {
		if len(args) == 1 {
			return scannerRunResult{out: []byte("8.30.1\n"), started: true}
		}
		return scannerRunResult{code: "execution_incomplete"}
	}
	if scanSelectedWithRunner(c, [][]byte{[]byte("sample")}, r, invoke) {
		t.Fatal("false clear")
	}
	if r.AttemptedFiles != 1 || r.InvokedFiles != 0 || r.CompletedFiles != 0 || r.BytesSubmitted != 6 || r.BytesObserved != 0 || r.Code != "execution_incomplete" {
		t.Fatal(r)
	}
}
func TestScannerUnstartedSuccessRejected(t *testing.T) {
	for _, phase := range []string{"version", "input"} {
		t.Run(phase, func(t *testing.T) {
			c := testScannerPolicy(t)
			r := &scannerReceipt{ExpectedFiles: 1}
			invoke := func(_ context.Context, _, _ string, in []byte, _ int, args ...string) scannerRunResult {
				if len(args) == 1 {
					return scannerRunResult{out: []byte("8.30.1\n"), started: phase != "version"}
				}
				return scannerRunResult{out: []byte("[]"), errout: scannerTestLog(len(in), 0), started: phase != "input"}
			}
			if scanSelectedWithRunner(c, [][]byte{[]byte("sample")}, r, invoke) || r.Status != "incomplete" || r.Code != "execution_incomplete" || r.CompletedFiles != 0 {
				t.Fatal(r)
			}
		})
	}
}
func TestScannerUnstartedDeadlineAndTransportCode(t *testing.T) {
	for _, phase := range []string{"version", "input"} {
		t.Run(phase, func(t *testing.T) {
			c := testScannerPolicy(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := &scannerReceipt{ExpectedFiles: 1}
			invoke := func(_ context.Context, _, _ string, _ []byte, _ int, args ...string) scannerRunResult {
				if len(args) == 1 && phase == "input" {
					return scannerRunResult{started: true, out: []byte("8.30.1\n")}
				}
				cancel()
				return scannerRunResult{code: "execution_incomplete"}
			}
			if scanSelectedWithContext(ctx, c, [][]byte{[]byte("A")}, r, invoke) || r.Code != "deadline" || r.CompletedFiles != 0 {
				t.Fatal(r)
			}
		})
	}
	c := testScannerPolicy(t)
	r := &scannerReceipt{ExpectedFiles: 1}
	invoke := func(_ context.Context, _, _ string, _ []byte, _ int, args ...string) scannerRunResult {
		if len(args) == 1 {
			return scannerRunResult{started: true, out: []byte("8.30.1\n")}
		}
		return scannerRunResult{code: "output_limit"}
	}
	if scanSelectedWithRunner(c, [][]byte{[]byte("A")}, r, invoke) || r.Code != "output_limit" {
		t.Fatal(r)
	}
}
func TestScannerChild(t *testing.T) {
	if os.Getenv("NO_COLOR") != "1" || os.Getenv("GITLEAKS_CONFIG") != "" || os.Getenv("GIT_CONFIG") != "" || os.Getenv("HTTP_PROXY") != "" {
		return
	}
	if strings.HasPrefix(os.Getenv("HOME"), os.TempDir()) && os.Getenv("TMPDIR") == os.Getenv("HOME") {
		fmt.Print("child-ok\n")
	}
}
func TestScannerCommandEnvironmentAndCaps(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	t.Setenv("GITLEAKS_CONFIG", "hostile")
	t.Setenv("GIT_CONFIG", "hostile")
	t.Setenv("HTTP_PROXY", "hostile")
	res := scannerCommand(context.Background(), exe, dir, nil, 100, "-test.run=TestScannerChild")
	if res.code != "" || res.exit != 0 || !strings.HasPrefix(string(res.out), "child-ok\n") || len(res.errout) != 0 {
		t.Fatalf("result %+v", res)
	}
	res = scannerCommand(context.Background(), exe, dir, nil, 2, "-test.run=TestScannerChild")
	if res.code != "output_limit" || !res.started {
		t.Fatal("stdout cap")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	res = scannerCommand(ctx, exe, dir, nil, 100, "-test.run=TestScannerChild")
	if res.code == "" {
		t.Fatal("timeout")
	}
}

func TestScannerRunWithholdsLateFinding(t *testing.T) {
	c := testScannerPolicy(t)
	root := t.TempDir()
	files := []string{"public one\n", "second private token\n"}
	q := request{Version: "command-guard.request/1", Kind: "read-range", StartLine: 1, EndLine: 1}
	for i, s := range files {
		name := fmt.Sprintf("%d.txt", i)
		if e := os.WriteFile(filepath.Join(root, name), []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
		h := sha256.Sum256([]byte(s))
		q.Files = append(q.Files, fileSpec{Path: name, SHA256: hex.EncodeToString(h[:])})
	}
	scope := scope{Version: "command-guard.scope/1", RequiredLayerIDs: []string{"L"}, RequiredRuleIDs: []string{"R"}, RequiredScannerIDs: []string{"gitleaks"}}
	p := policy{Version: "command-guard.policy/1", Layers: []layer{{ID: "L", AllowedOperations: []string{"read-range"}, Limits: limits{MaxFileBytes: 1000, MaxTotalBytes: 2000, MaxOutputBytes: 1000, MaxResults: 10}}}, Rules: []rule{{ID: "R", Pattern: "NEVER-MATCH"}}, Scanner: &c}
	calls := 0
	invoke := func(ctx context.Context, exe, dir string, in []byte, cap int, args ...string) ([]byte, []byte, int, bool) {
		if len(args) == 1 {
			return []byte("8.30.1\n"), nil, 0, true
		}
		calls++
		if calls == 2 {
			return []byte(`[{"RuleID":"generic-api-key","Secret":"[REDACTED]"}]`), scannerTestLog(len(in), 1), 3, true
		}
		return []byte("[]"), scannerTestLog(len(in), 0), 0, true
	}
	r := runWithScanner(root, scope, p, q, "json", wrapScannerTest(invoke))
	if r.Status != "blocked" || r.Code != "scanner_finding" || len(r.Rows) != 0 || r.ExternalScanner.Findings != 1 || calls != 2 {
		t.Fatal(r)
	}
	if strings.Contains(string(encode(r, "json")), "private token") {
		t.Fatal("content leaked")
	}
	p.Scanner.SHA256 = strings.Repeat("0", 64)
	r = runWithScanner(root, scope, p, q, "json", wrapScannerTest(invoke))
	if r.Status != "incomplete" || r.Code != "scanner_incomplete" || len(r.Rows) != 0 || r.ExternalScanner.Code != "hash_mismatch" {
		t.Fatal(r)
	}
}

func TestScannerOverflowChild(t *testing.T) {
	if os.Getenv("NO_COLOR") != "1" || os.Getenv("GITLEAKS_CONFIG") != "" || os.Getenv("HOME") != os.Getenv("TMPDIR") || !strings.Contains(strings.Join(os.Args, " "), "-test.run=TestScannerOverflowChild") {
		return
	}
	_, _ = os.Stderr.Write(bytes.Repeat([]byte("x"), 65<<10))
}
func TestScannerStderrCap(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("GITLEAKS_CONFIG", "hostile")
	res := scannerCommand(context.Background(), exe, t.TempDir(), nil, 100, "-test.run=TestScannerOverflowChild")
	if res.code != "output_limit" || !res.started {
		t.Fatal("stderr cap")
	}
}
