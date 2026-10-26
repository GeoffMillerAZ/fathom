package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const honestRead = `{"version":"command-guard.report/1","status":"no-findings","code":"ok","noActionAuthority":true,"filesRead":1,"bytesRead":25,"bytesScanned":25,"scope":"selected-files-only","scanner":"configured-regex-only","limitation":"not-proof-of-secret-or-pii-absence","totalMatches":2,"returned":2,"omitted":0,"truncated":false,"rows":[{"file":0,"line":1,"text":"alpha public"},{"file":0,"line":2,"text":"beta public"}]}`

func TestCandidateOracleRejectsUntrustedReports(t *testing.T) {
	f := baseFixture("read-json")
	if !checkReport([]byte(honestRead+"\n"), f) {
		t.Fatal("honest report rejected")
	}
	tests := map[string]string{
		"empty": "", "malformed": "{", "duplicate": strings.Replace(honestRead, `"status":"no-findings"`, `"status":"no-findings","status":"no-findings"`, 1),
		"extended":          strings.Replace(honestRead, `"version":`, `"extra":1,"version":`, 1),
		"wrong row":         strings.Replace(honestRead, "beta public", "other", 1),
		"wrong counter":     strings.Replace(honestRead, `"bytesScanned":25`, `"bytesScanned":0`, 1),
		"missing zero":      strings.Replace(honestRead, `"omitted":0,`, "", 1),
		"malformed unicode": strings.Replace(honestRead, "alpha public", `\ud800`, 1),
		"trailing":          honestRead + " {}",
	}
	for name, wire := range tests {
		t.Run(name, func(t *testing.T) {
			if checkReport([]byte(wire+"\n"), f) {
				t.Fatal("false pass")
			}
		})
	}
}

func TestTransportStartAndOverflowAreUnknown(t *testing.T) {
	work := t.TempDir()
	f := baseFixture("read-json")
	for _, tt := range []struct {
		name string
		obs  observation
		code string
	}{
		{"start", observation{code: "start_failed"}, "start_failed"},
		{"timeout", observation{started: true, code: "deadline"}, "deadline"},
		{"overflow", observation{started: true, code: "output_limit"}, "output_limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := runCase(context.Background(), f, "fake", "", work, func(context.Context, string, string, []string) observation { return tt.obs })
			if r.Status != "unknown" || r.Code != tt.code {
				t.Fatalf("%+v", r)
			}
		})
	}
}
func TestBoundedWriterBoundary(t *testing.T) {
	w := &boundedWriter{}
	if n, err := w.Write(bytes.Repeat([]byte{'x'}, outputCap)); err != nil || n != outputCap || w.overflow {
		t.Fatal("exact boundary rejected")
	}
	if n, err := w.Write([]byte{'x'}); err == nil || n != 0 || !w.overflow || w.Len() != outputCap {
		t.Fatal("overflow accepted")
	}
}
func TestHelperStdout(t *testing.T) { helperOutput(t, os.Stdout) }
func TestHelperStderr(t *testing.T) { helperOutput(t, os.Stderr) }
func helperOutput(t *testing.T, w io.Writer) {
	if !strings.HasPrefix(filepath.Base(os.Args[0]), "cq-child-") {
		return
	}
	if _, err := w.Write(bytes.Repeat([]byte{'x'}, outputCap+1)); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}
func TestInvokeCapsBothChannels(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, tc := range []struct{ name, test string }{{"stdout", "TestHelperStdout"}, {"stderr", "TestHelperStderr"}} {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.Open(self)
			if err != nil {
				t.Fatal(err)
			}
			defer src.Close()
			name := "cq-child-" + tc.name
			if filepath.Ext(self) == ".exe" {
				name += ".exe"
			}
			path := filepath.Join(dir, name)
			dst, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = io.Copy(dst, src); err != nil {
				dst.Close()
				t.Fatal(err)
			}
			if err = dst.Close(); err != nil {
				t.Fatal(err)
			}
			o := invoke(context.Background(), path, dir, []string{"-test.run=^" + tc.test + "$"})
			if !o.started || o.code != "output_limit" {
				t.Fatalf("%s: started=%v code=%s exit=%d stdout=%d stderr=%d", tc.name, o.started, o.code, o.exit, len(o.stdout), len(o.stderr))
			}
		})
	}
}

func TestAllowAllCannotPassHeldCase(t *testing.T) {
	f := baseFixture("late-regex")
	if checkReport([]byte(honestRead+"\n"), f) {
		t.Fatal("allow-all passed late finding")
	}
	r := runCase(context.Background(), f, "fake", "", t.TempDir(), func(context.Context, string, string, []string) observation {
		return observation{started: true, stdout: []byte(honestRead + "\n")}
	})
	if r.Status != "fail" || r.Code != "oracle_mismatch" {
		t.Fatalf("%+v", r)
	}
}

func TestRosterAndReportSemantics(t *testing.T) {
	for _, profile := range []string{"files-core/1", "files-gitleaks/1"} {
		r := newReport(profile, strings.Repeat("a", 64), strings.Repeat("b", 64))
		if !validQualification(r) || len(r.Cases) != r.Required || r.RosterSHA256 != rosterHash(profile) {
			t.Fatalf("roster %s", profile)
		}
		r.Cases[0].ID = "other"
		if validQualification(r) {
			t.Fatal("changed ID passed")
		}
	}
	r := newReport("files-core/1", strings.Repeat("a", 64), "")
	for i := range r.Cases {
		r.Cases[i].Status = "pass"
		r.Cases[i].Code = "oracle_met"
	}
	if r.finish() != 0 || !validQualification(r) {
		t.Fatal("all pass")
	}
	r.Cases[0].Status = "fail"
	r.Cases[0].Code = "oracle_mismatch"
	r.Cases[1].Status = "unknown"
	r.Cases[1].Code = "deadline"
	if r.finish() != 2 || !validQualification(r) {
		t.Fatal("unknown precedence")
	}
	r.Cases[1].Code = "/private/workspace/raw-error"
	if validQualification(r) {
		t.Fatal("unbounded unknown reason accepted")
	}
}
func TestRosterDigestBindsFixtureAndOracle(t *testing.T) {
	baseline := rosterHash("files-core/1")
	for _, mutate := range []struct {
		name string
		edit func(*fixture)
	}{
		{"body", func(f *fixture) { f.files[0].body = []byte("changed synthetic body\n") }},
		{"rows", func(f *fixture) { f.wantRows[0].Text = "changed row" }},
		{"counter", func(f *fixture) { f.wantScanned++ }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			changed := rosterHashWith("files-core/1", func(id string, _ int) fixture {
				f := baseFixture(id)
				if id == "read-json" {
					mutate.edit(&f)
				}
				return f
			})
			if changed == baseline {
				t.Fatal("descriptor mutation kept digest")
			}
		})
	}
}

type badWriter struct{}

func (badWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestOutputErrorCannotPass(t *testing.T) {
	r := newReport("files-core/1", strings.Repeat("a", 64), "")
	for i := range r.Cases {
		r.Cases[i].Status = "pass"
		r.Cases[i].Code = "oracle_met"
	}
	if emit(badWriter{}, &r, "json", "") != 2 {
		t.Fatal("write failure passed")
	}
}
func TestCleanupFailureCannotPass(t *testing.T) {
	r := newReport("files-core/1", strings.Repeat("a", 64), "")
	for i := range r.Cases {
		r.Cases[i].Status = "pass"
		r.Cases[i].Code = "oracle_met"
	}
	var out bytes.Buffer
	if emit(&out, &r, "json", "cleanup_failed") != 2 || !validQualification(r) || r.Unknown != 1 || r.Status != "unknown" {
		t.Fatal("cleanup failure passed")
	}
}
func TestSnapshotHashAndFileType(t *testing.T) {
	work := t.TempDir()
	path := filepath.Join(work, "candidate")
	if err := os.WriteFile(path, []byte("synthetic binary"), 0700); err != nil {
		t.Fatal(err)
	}
	copy, code := snapshot(context.Background(), path, sum([]byte("synthetic binary")), work, "copy")
	if code != "" || copy == "" {
		t.Fatalf("snapshot %s", code)
	}
	if _, code = snapshot(context.Background(), path, strings.Repeat("0", 64), work, "bad"); code != "hash_mismatch" {
		t.Fatalf("hash %s", code)
	}
	if _, code = snapshot(context.Background(), work, strings.Repeat("0", 64), work, "dir"); code != "binary_unavailable" {
		t.Fatalf("type %s", code)
	}
}
func TestFixedArguments(t *testing.T) {
	base := []string{"--profile", "files-core/1", "--guard", "/abs/tool", "--guard-sha256", strings.Repeat("a", 64)}
	if !strictArgs(base) {
		t.Fatal("valid argument shape rejected")
	}
	for _, extra := range [][]string{{"--skip", "late-regex"}, {"unexpected"}, {"--guard-sha256", strings.Repeat("A", 64)}, {"--guard-sha256", strings.Repeat("a", 64)}, {"--format", "json", "--format", "text"}} {
		args := append(append([]string(nil), base...), extra...)
		if strictArgs(args) {
			t.Fatalf("accepted malformed or duplicate arguments %v", extra)
		}
	}
	if validHash(strings.Repeat("A", 64)) || validHash("short") {
		t.Fatal("invalid pin accepted")
	}
}
func TestHelpIsFixedAndDoesNotInvokeCandidate(t *testing.T) {
	called := false
	transport := func(context.Context, string, string, []string) observation { called = true; return observation{} }
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"run", "--help"}, {"run", "-h"}} {
		var out bytes.Buffer
		if code := execute(args, &out, transport); code != 0 || !strings.Contains(out.String(), "files-core/1|files-gitleaks/1") || !strings.Contains(out.String(), "Exit 0:") {
			t.Fatalf("help %v: code=%d", args, code)
		}
	}
	if called {
		t.Fatal("help invoked candidate")
	}
	if execute([]string{"help"}, badWriter{}, transport) != 2 {
		t.Fatal("help write error passed")
	}
	var out bytes.Buffer
	if execute([]string{"run", "--help", "--skip", "x"}, &out, transport) != 2 || out.Len() != 0 {
		t.Fatal("extra help arguments accepted")
	}
}
func TestStrictScannerReceipt(t *testing.T) {
	f := scannerFixture("scanner-clean")
	var m map[string]any
	if err := json.Unmarshal([]byte(honestRead), &m); err != nil {
		t.Fatal(err)
	}
	m["scanner"] = "configured-regex-and-gitleaks"
	m["externalScanner"] = map[string]any{"id": "gitleaks", "profile": "gitleaks-stdin-8.30.1", "status": "complete", "code": "verified", "expectedFiles": 1, "attemptedFiles": 1, "invokedFiles": 1, "completedFiles": 1, "bytesSubmitted": 25, "bytesObserved": 25, "findings": 0}
	b, _ := json.Marshal(m)
	b = append(b, '\n')
	if !checkReport(b, f) {
		t.Fatal("honest scanner receipt rejected")
	}
	delete(m["externalScanner"].(map[string]any), "findings")
	b, _ = json.Marshal(m)
	if checkReport(append(b, '\n'), f) {
		t.Fatal("missing zero finding accepted")
	}
}
func TestABAProjectionIncludesDistinctMiddleFile(t *testing.T) {
	for _, id := range []string{"scanner-aba-disabled", "scanner-aba-enabled"} {
		t.Run(id, func(t *testing.T) {
			f := scannerFixture(id)
			m := map[string]any{
				"version": "command-guard.report/1", "status": "no-findings", "code": "ok", "noActionAuthority": true,
				"filesRead": 3, "bytesRead": 63, "bytesScanned": 63, "scope": "selected-files-only", "scanner": "configured-regex-and-gitleaks",
				"limitation": "not-proof-of-secret-or-pii-absence", "totalMatches": 5, "returned": 5, "omitted": 0, "truncated": false,
				"rows": []any{map[string]any{"file": 0, "line": 1, "text": "alpha public"}, map[string]any{"file": 0, "line": 2, "text": "beta public"}, map[string]any{"file": 1, "line": 1, "text": "gamma public"}, map[string]any{"file": 2, "line": 1, "text": "alpha public"}, map[string]any{"file": 2, "line": 2, "text": "beta public"}},
			}
			external := map[string]any{"id": "gitleaks", "profile": "gitleaks-stdin-8.30.1", "status": "complete", "code": "verified", "expectedFiles": 3, "attemptedFiles": 3, "invokedFiles": 3, "completedFiles": 3, "bytesSubmitted": 63, "bytesObserved": 63, "findings": 0}
			if id == "scanner-aba-enabled" {
				external["attemptedFiles"] = 2
				external["invokedFiles"] = 2
				external["completedFiles"] = 2
				external["bytesSubmitted"] = 38
				external["bytesObserved"] = 38
				external["reuse"] = map[string]any{"profile": "invocation-exact/1", "files": 1, "bytes": 25}
			}
			m["externalScanner"] = external
			b, _ := json.Marshal(m)
			if !checkReport(append(b, '\n'), f) {
				t.Fatal("actual native A/B/A shape rejected")
			}
			m["rows"] = append(m["rows"].([]any)[:2], m["rows"].([]any)[3:]...)
			m["totalMatches"] = 4
			m["returned"] = 4
			b, _ = json.Marshal(m)
			if checkReport(append(b, '\n'), f) {
				t.Fatal("missing middle-file row accepted")
			}
		})
	}
}

// Opt-in native witness. The candidate outputs stay in the test's private temp
// directory for this invocation; public qualification receipts contain no source.
func TestNativeQualificationOptIn(t *testing.T) {
	guard, hash := os.Getenv("COMMAND_QUALIFY_TEST_GUARD"), os.Getenv("COMMAND_QUALIFY_TEST_GUARD_SHA256")
	if guard == "" || hash == "" {
		t.Skip("operator-supplied guard pin required")
	}
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("CUE unavailable; native qualification remains unknown")
	}
	profiles := []struct{ profile, scanner, scannerHash string }{{profile: "files-core/1"}, {profile: "files-loopback-fixture/1"}}
	if scanner, sh := os.Getenv("COMMAND_QUALIFY_TEST_SCANNER"), os.Getenv("COMMAND_QUALIFY_TEST_SCANNER_SHA256"); scanner != "" && sh != "" {
		profiles = append(profiles, struct{ profile, scanner, scannerHash string }{"files-gitleaks/1", scanner, sh})
	}
	for _, p := range profiles {
		t.Run(p.profile, func(t *testing.T) {
			private := t.TempDir()
			seen := map[string]int{}
			nonempty := map[string]int{}
			compactVersions := map[string]int{}
			var captured []string
			capture := func(ctx context.Context, exe, work string, args []string) observation {
				o := invoke(ctx, exe, work, args)
				format := ""
				for i := 0; i+1 < len(args); i++ {
					if args[i] == "--format" {
						format = args[i+1]
					}
				}
				seen[format]++
				if len(o.stdout) > 0 {
					name := filepath.Join(private, "output-"+format+"-"+strings.Repeat("x", seen[format])+".json")
					if err := os.WriteFile(name, o.stdout, 0600); err != nil {
						t.Fatal(err)
					}
					captured = append(captured, name)
					nonempty[format]++
					if format == "compact" {
						var m map[string]json.RawMessage
						if !validJSON(o.stdout, &m) {
							t.Fatal("invalid compact JSON")
						}
						var v string
						if json.Unmarshal(m["version"], &v) != nil {
							t.Fatal("missing compact version")
						}
						compactVersions[v]++
					}
				}
				return o
			}
			args := []string{"run", "--profile", p.profile, "--guard", guard, "--guard-sha256", hash, "--format", "json"}
			if p.scanner != "" {
				args = append(args, "--scanner", p.scanner, "--scanner-sha256", p.scannerHash)
			}
			var out bytes.Buffer
			code := execute(args, &out, capture)
			if code != 0 {
				t.Fatalf("native qualification exit %d: %s", code, out.String())
			}
			var receipt qualification
			if !validJSON(out.Bytes(), &receipt) || !validQualification(receipt) {
				t.Fatal("invalid qualification receipt")
			}
			if receipt.SyntheticPeerOnly != (p.profile == "files-loopback-fixture/1") {
				t.Fatal("incorrect synthetic peer marker")
			}
			want := 15
			if p.profile == "files-gitleaks/1" {
				want = 24
			} else if p.profile == "files-loopback-fixture/1" {
				want = 27 // 30 required cases, with only core's three tiny-output cases empty.
			}
			if len(captured) != want || nonempty["json"] != want-3 || nonempty["text"] != 1 || nonempty["compact"] != 2 || compactVersions["command-guard.compact-report/1"] != 1 || compactVersions["command-guard.report/1"] != 1 {
				t.Fatalf("candidate format coverage: nonempty=%d byFormat=%v compactVersions=%v", len(captured), nonempty, compactVersions)
			}
			for _, path := range captured {
				if ok, diag := cueValid(t, filepath.Join("..", "command-guard", "schema.cue"), "#OutputReport", path); !ok {
					t.Fatalf("candidate CUE rejected %s: %s", filepath.Base(path), diag)
				}
			}
			receiptPath := filepath.Join(private, "receipt.json")
			if err := os.WriteFile(receiptPath, out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if ok, diag := cueValid(t, "schema.cue", "#Report", receiptPath); !ok {
				t.Fatalf("qualification JSON CUE rejected: %s", diag)
			}
			textReceipt, _ := json.MarshalIndent(receipt, "", "  ")
			textReceipt = append(textReceipt, '\n')
			textPath := filepath.Join(private, "receipt-text.json")
			if err := os.WriteFile(textPath, textReceipt, 0600); err != nil {
				t.Fatal(err)
			}
			if ok, diag := cueValid(t, "schema.cue", "#Report", textPath); !ok {
				t.Fatalf("qualification text CUE rejected: %s", diag)
			}
			var raw map[string]any
			if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
				t.Fatal(err)
			}
			bad := map[string]func(map[string]any){
				"count":        func(m map[string]any) { m["required"] = float64(17) },
				"status":       func(m map[string]any) { m["status"] = "unknown" },
				"missing-case": func(m map[string]any) { a := m["cases"].([]any); m["cases"] = a[:len(a)-1] },
			}
			if p.profile == "files-loopback-fixture/1" {
				bad["missing-synthetic-marker"] = func(m map[string]any) { delete(m, "syntheticPeerOnly") }
			}
			for name, mutate := range bad {
				var m map[string]any
				b, _ := json.Marshal(raw)
				if err := json.Unmarshal(b, &m); err != nil {
					t.Fatal(err)
				}
				mutate(m)
				b, _ = json.Marshal(m)
				path := filepath.Join(private, "bad-"+name+".json")
				if err := os.WriteFile(path, b, 0600); err != nil {
					t.Fatal(err)
				}
				if ok, _ := cueValid(t, "schema.cue", "#Report", path); ok {
					t.Fatalf("CUE accepted %s", name)
				}
			}
			wrong := receipt
			wrong.Cases = append([]caseResult(nil), receipt.Cases...)
			wrong.Cases[0], wrong.Cases[1] = wrong.Cases[1], wrong.Cases[0]
			if validQualification(wrong) {
				t.Fatal("Go accepted reordered roster")
			}
		})
	}
}

func cueValid(t *testing.T, schema, definition, data string) (bool, string) {
	t.Helper()
	cmd := exec.Command("cue", "vet", "-c", "-d", definition, schema, data)
	cmd.Env = childEnv(t.TempDir())
	b, err := cmd.CombinedOutput()
	if len(b) > 1024 {
		b = b[:1024]
	}
	return err == nil, string(b)
}
func TestCUECandidateCapture(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("CUE unavailable")
	}
	path := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(path, []byte(honestRead+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, diag := cueValid(t, filepath.Join("..", "command-guard", "schema.cue"), "#OutputReport", path); !ok {
		t.Fatalf("candidate CUE rejected: %s", diag)
	}
}

func TestEmittedReceiptCUEParity(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("CUE unavailable")
	}
	for _, tc := range []struct {
		name, format, preflight string
		change                  func(*qualification)
	}{
		{"pass-json", "json", "", nil},
		{"pass-text", "text", "", nil},
		{"fail-json", "json", "", func(r *qualification) { r.Cases[0].Status = "fail"; r.Cases[0].Code = "oracle_mismatch" }},
		{"unknown-json", "json", "", func(r *qualification) { r.Cases[0].Status = "unknown"; r.Cases[0].Code = "deadline" }},
		{"preflight-json", "json", "binary_unavailable", func(r *qualification) {
			for i := range r.Cases {
				r.Cases[i].Status = "unknown"
				r.Cases[i].Code = "unrun"
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newReport("files-core/1", strings.Repeat("a", 64), "")
			for i := range r.Cases {
				r.Cases[i].Status = "pass"
				r.Cases[i].Code = "oracle_met"
			}
			if tc.change != nil {
				tc.change(&r)
			}
			var out bytes.Buffer
			code := emit(&out, &r, tc.format, tc.preflight)
			if tc.name == "pass-json" || tc.name == "pass-text" {
				if code != 0 {
					t.Fatalf("pass exit %d", code)
				}
			} else if tc.name == "fail-json" {
				if code != 1 {
					t.Fatalf("fail exit %d", code)
				}
			} else if code != 2 {
				t.Fatalf("unknown exit %d", code)
			}
			path := filepath.Join(t.TempDir(), "receipt.json")
			if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if ok, diag := cueValid(t, "schema.cue", "#Report", path); !ok {
				t.Fatalf("emitted report CUE rejected: %s", diag)
			}
			if !validQualification(r) {
				t.Fatal("emitted report failed Go semantics")
			}
			if tc.name != "pass-json" {
				return
			}
			var original map[string]any
			if err := json.Unmarshal(out.Bytes(), &original); err != nil {
				t.Fatal(err)
			}
			for _, bad := range []struct {
				name   string
				mutate func(map[string]any)
			}{
				{"extra", func(m map[string]any) { m["surprise"] = "value" }},
				{"count", func(m map[string]any) { m["passed"] = float64(17) }},
				{"status", func(m map[string]any) { m["status"] = "fail" }},
				{"missing-case", func(m map[string]any) { a := m["cases"].([]any); m["cases"] = a[:len(a)-1] }},
			} {
				t.Run(bad.name, func(t *testing.T) {
					wire, _ := json.Marshal(original)
					var m map[string]any
					if err := json.Unmarshal(wire, &m); err != nil {
						t.Fatal(err)
					}
					bad.mutate(m)
					wire, _ = json.Marshal(m)
					path := filepath.Join(t.TempDir(), "bad.json")
					if err := os.WriteFile(path, wire, 0600); err != nil {
						t.Fatal(err)
					}
					if ok, _ := cueValid(t, "schema.cue", "#Report", path); ok {
						t.Fatal("CUE accepted invalid receipt")
					}
				})
			}
		})
	}
}
