package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func cleanScannerRunner(calls *int, change func(int, []byte) scannerRunResult) scannerRunner {
	return func(_ context.Context, _, _ string, in []byte, _ int, args ...string) scannerRunResult {
		if len(args) == 1 {
			return scannerRunResult{started: true, out: []byte("8.30.1\n")}
		}
		*calls++
		if change != nil {
			if v := change(*calls, in); v.code != "" || v.out != nil || v.errout != nil || v.exit != 0 {
				return v
			}
		}
		return scannerRunResult{started: true, out: []byte("[]"), errout: scannerTestLog(len(in), 0)}
	}
}

func TestScannerReuseABAAndInvocationBoundary(t *testing.T) {
	c := testScannerPolicy(t)
	c.ReuseEqualInputs = true
	data := [][]byte{[]byte("A"), []byte("B"), []byte("A")}
	for i := 0; i < 2; i++ {
		calls := 0
		r := &scannerReceipt{ExpectedFiles: 3}
		if !scanSelectedWithRunner(c, data, r, cleanScannerRunner(&calls, nil)) || calls != 2 || r.AttemptedFiles != 2 || r.InvokedFiles != 2 || r.CompletedFiles != 2 || r.BytesSubmitted != 2 || r.BytesObserved != 2 || r.Reuse == nil || r.Reuse.Files != 1 || r.Reuse.Bytes != 1 {
			t.Fatal(r, calls)
		}
	}
	c.ReuseEqualInputs = false
	calls := 0
	r := &scannerReceipt{ExpectedFiles: 3}
	if !scanSelectedWithRunner(c, data, r, cleanScannerRunner(&calls, nil)) || calls != 3 || r.Reuse != nil || bytes.Contains(encode(report{ExternalScanner: r}, "json"), []byte("reuse")) {
		t.Fatal(r, calls)
	}
}

func TestScannerReuseCollisionAndEmpty(t *testing.T) {
	c := testScannerPolicy(t)
	c.ReuseEqualInputs = true
	data := [][]byte{[]byte("A"), []byte("B"), []byte("A"), {}, {}}
	calls := 0
	r := &scannerReceipt{ExpectedFiles: len(data)}
	constant := func([]byte) [32]byte { return [32]byte{} }
	if !scanSelectedWithContextHash(context.Background(), c, data, r, cleanScannerRunner(&calls, nil), constant) || calls != 3 || r.Reuse.Files != 2 || r.Reuse.Bytes != 1 {
		t.Fatal(r, calls)
	}
}

func TestScannerReuseMutationAndNoReuseOfFailures(t *testing.T) {
	c := testScannerPolicy(t)
	c.ReuseEqualInputs = true
	for _, phase := range []string{"current", "prior", "finding", "error"} {
		t.Run(phase, func(t *testing.T) {
			data := [][]byte{[]byte("A"), []byte("B"), []byte("A")}
			calls := 0
			r := &scannerReceipt{ExpectedFiles: 3}
			change := func(n int, in []byte) scannerRunResult {
				if phase == "current" && n == 1 {
					in[0] = 'Z'
				}
				if phase == "prior" && n == 2 {
					data[0][0] = 'Z'
				}
				if phase == "finding" && n == 1 {
					return scannerRunResult{started: true, out: []byte(`[{"RuleID":"x"}]`), errout: scannerTestLog(1, 1), exit: 3}
				}
				if phase == "error" && n == 1 {
					return scannerRunResult{started: true, code: "execution_incomplete"}
				}
				return scannerRunResult{}
			}
			ok := scanSelectedWithRunner(c, data, r, cleanScannerRunner(&calls, change))
			if phase == "current" || phase == "prior" {
				if ok || r.Status != "incomplete" || r.Code != "coverage_mismatch" {
					t.Fatal(r)
				}
				if phase == "prior" && (r.CompletedFiles != 2 || r.Reuse.Files != 1) {
					t.Fatal("final witness not exercised", r)
				}
			} else if phase == "finding" {
				if !ok || calls != 3 || r.Findings != 1 || r.Reuse.Files != 0 {
					t.Fatal(r, calls)
				}
			} else {
				if ok || r.Code != "execution_incomplete" || r.Reuse.Files != 0 {
					t.Fatal(r)
				}
			}
		})
	}
}

func TestScannerReuseEntryAndByteBudgets(t *testing.T) {
	c := testScannerPolicy(t)
	c.ReuseEqualInputs = true
	c.TimeoutMillis = 30000
	data := make([][]byte, 0, 258)
	for i := 0; i < 257; i++ {
		data = append(data, []byte(fmt.Sprintf("%03d", i)))
	}
	data = append(data, []byte("256"))
	calls := 0
	r := &scannerReceipt{ExpectedFiles: len(data)}
	if !scanSelectedWithRunner(c, data, r, cleanScannerRunner(&calls, nil)) || calls != len(data) || r.Reuse.Files != 0 {
		t.Fatal(r, calls)
	}
	big := bytes.Repeat([]byte("x"), 8<<20)
	data = [][]byte{big, big}
	calls = 0
	r = &scannerReceipt{ExpectedFiles: 2}
	if !scanSelectedWithRunner(c, data, r, cleanScannerRunner(&calls, nil)) || calls != 1 || r.Reuse.Files != 1 {
		t.Fatal(r, calls)
	}
	tooBig := bytes.Repeat([]byte("y"), (8<<20)+1)
	data = [][]byte{tooBig, tooBig}
	calls = 0
	r = &scannerReceipt{ExpectedFiles: 2}
	if !scanSelectedWithRunner(c, data, r, cleanScannerRunner(&calls, nil)) || calls != 2 || r.Reuse.Files != 0 {
		t.Fatal(r, calls)
	}
	a := bytes.Repeat([]byte("a"), 3<<20)
	b := bytes.Repeat([]byte("b"), 3<<20)
	d := bytes.Repeat([]byte("d"), 3<<20)
	data = [][]byte{a, b, d, d}
	calls = 0
	r = &scannerReceipt{ExpectedFiles: 4}
	if !scanSelectedWithRunner(c, data, r, cleanScannerRunner(&calls, nil)) || calls != 4 || r.Reuse.Files != 0 || r.BytesObserved != 12<<20 {
		t.Fatal(r, calls)
	}
}

func TestScannerLinearWitnessWorstFileCorpus(t *testing.T) {
	data := make([][]byte, 32)
	for i := range data {
		data[i] = bytes.Repeat([]byte{'x'}, 2<<20)
		data[i][0] = byte(i)
	}
	for _, enabled := range []bool{false, true} {
		c := testScannerPolicy(t)
		c.ReuseEqualInputs = enabled
		c.TimeoutMillis = 30000
		calls, hashes := 0, 0
		r := &scannerReceipt{ExpectedFiles: 32}
		witness := func(b []byte) [32]byte { hashes++; return sha256.Sum256(b) }
		if !scanSelectedWithContextDeps(context.Background(), c, data, r, cleanScannerRunner(&calls, nil), sha256.Sum256, witness, scannerBinary) || calls != 32 || r.BytesObserved != 64<<20 || hashes != 4*len(data) {
			t.Fatal(enabled, r, calls, hashes)
		}
	}
}

func TestScannerReuseDeadlineOnHit(t *testing.T) {
	c := testScannerPolicy(t)
	c.ReuseEqualInputs = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	r := &scannerReceipt{ExpectedFiles: 2}
	hashes := 0
	hash := func(b []byte) [32]byte {
		hashes++
		if hashes == 3 {
			cancel()
		}
		return sha256.Sum256(b)
	}
	if scanSelectedWithContextHash(ctx, c, [][]byte{[]byte("A"), []byte("A")}, r, cleanScannerRunner(&calls, nil), hash) || r.Code != "deadline" || calls != 1 || r.Reuse.Files != 0 {
		t.Fatal(r, calls, hashes)
	}
}

func TestScannerReuseCleanupFailureAfterHit(t *testing.T) {
	c := testScannerPolicy(t)
	c.ReuseEqualInputs = true
	snapshot := func(context.Context, scannerPolicy) (string, string, func() error, string) {
		return "/private/tool", "/private/home", func() error { return errors.New("cleanup") }, ""
	}
	calls := 0
	r := &scannerReceipt{ExpectedFiles: 2}
	if scanSelectedWithContextDeps(context.Background(), c, [][]byte{[]byte("A"), []byte("A")}, r, cleanScannerRunner(&calls, nil), sha256.Sum256, sha256.Sum256, snapshot) || r.Code != "cleanup_failed" || r.Status != "incomplete" || r.Reuse.Files != 1 || calls != 1 {
		t.Fatal(r, calls)
	}
}

func TestScannerReuseReceiptEncodings(t *testing.T) {
	r := &scannerReceipt{ID: "gitleaks", Profile: "gitleaks-stdin-8.30.1", Status: "complete", Code: "verified", ExpectedFiles: 2, AttemptedFiles: 1, InvokedFiles: 1, CompletedFiles: 1, BytesSubmitted: 1, BytesObserved: 1, Reuse: &scannerReuseReceipt{Profile: "invocation-exact/1", Files: 1, Bytes: 1}}
	for _, format := range []string{"json", "text", "compact"} {
		b := encode(report{Status: "no-findings", Code: "ok", ExternalScanner: r}, format)
		if !bytes.Contains(b, []byte(`"reuse"`)) || !bytes.Contains(b, []byte(`"invocation-exact/1"`)) || strings.Contains(string(b), "sha256") {
			t.Fatal(format, string(b))
		}
	}

}

func TestScannerReusePolicyCUE(t *testing.T) {
	c := testScannerPolicy(t)
	c.ReuseEqualInputs = true
	b, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	schema, e := os.ReadFile("schema.cue")
	if e != nil {
		t.Fatal(e)
	}
	for _, cue := range []string{scannerTestCUE(t)} {
		vetScannerOutput(t, cue, schema, "#ScannerPolicy", b, true)
		bad := strings.Replace(string(b), `"reuseEqualInputs":true`, `"reuseEqualInputs":"yes"`, 1)
		vetScannerOutput(t, cue, schema, "#ScannerPolicy", []byte(bad), false)
	}
}

func TestScannerReuseFileLateResultWithholds(t *testing.T) {
	root := t.TempDir()
	c := testScannerPolicy(t)
	c.ReuseEqualInputs = true
	q := request{Version: "command-guard.request/1", Kind: "read-range", StartLine: 1, EndLine: 1}
	for i, v := range []string{"public A\n", "public A\n", "public B\n"} {
		name := fmt.Sprintf("%d.txt", i)
		if e := os.WriteFile(filepath.Join(root, name), []byte(v), 0600); e != nil {
			t.Fatal(e)
		}
		h := sha256.Sum256([]byte(v))
		q.Files = append(q.Files, fileSpec{Path: name, SHA256: hex.EncodeToString(h[:])})
	}
	s := scope{Version: "command-guard.scope/1", RequiredLayerIDs: []string{"L"}, RequiredRuleIDs: []string{"R"}, RequiredScannerIDs: []string{"gitleaks"}}
	p := policy{Version: "command-guard.policy/1", Layers: []layer{{ID: "L", AllowedOperations: []string{"read-range"}, Limits: limits{MaxFileBytes: 1000, MaxTotalBytes: 3000, MaxOutputBytes: 4096, MaxResults: 10}}}, Rules: []rule{{ID: "R", Pattern: "NEVER-MATCH"}}, Scanner: &c}
	schema, e := os.ReadFile("schema.cue")
	if e != nil {
		t.Fatal(e)
	}
	var clean report
	for _, format := range []string{"json", "text", "compact"} {
		calls := 0
		r := runWithScanner(root, s, p, q, format, cleanScannerRunner(&calls, nil))
		if r.Status != "no-findings" || calls != 2 || r.ExternalScanner.Reuse.Files != 1 || r.ExternalScanner.Reuse.Bytes != len("public A\n") {
			t.Fatal(format, r, calls)
		}
		clean = r
		c.ReuseEqualInputs = false
		disabledCalls := 0
		disabled := runWithScanner(root, s, p, q, format, cleanScannerRunner(&disabledCalls, nil))
		if disabled.Status != "no-findings" || disabled.ExternalScanner == nil || disabled.ExternalScanner.Reuse != nil || disabled.ExternalScanner.CompletedFiles != disabled.ExternalScanner.ExpectedFiles || disabledCalls != disabled.ExternalScanner.ExpectedFiles {
			t.Fatal(format, disabled, disabledCalls)
		}
		for _, cue := range []string{scannerTestCUE(t)} {
			vetScannerOutput(t, cue, schema, "#OutputReport", encode(disabled, format), true)
		}
		c.ReuseEqualInputs = true
		for _, cue := range []string{scannerTestCUE(t)} {
			vetScannerOutput(t, cue, schema, "#OutputReport", encode(r, format), true)
			var bad map[string]any
			if e := json.Unmarshal(encode(r, format), &bad); e != nil {
				t.Fatal(e)
			}
			reuse := bad["externalScanner"].(map[string]any)["reuse"].(map[string]any)
			reuse["files"] = float64(0)
			wire, _ := json.Marshal(bad)
			vetScannerOutput(t, cue, schema, "#OutputReport", wire, false)
			delete(bad["externalScanner"].(map[string]any), "reuse")
			wire, _ = json.Marshal(bad)
			vetScannerOutput(t, cue, schema, "#OutputReport", wire, false)
		}
	}
	if emit(failWriter{}, clean, "json", 0) != 2 {
		t.Fatal("broken writer")
	}
	p.Layers[0].Limits.MaxOutputBytes = 2
	tiny := runWithScanner(root, s, p, q, "json", cleanScannerRunner(new(int), nil))
	if tiny.Code != "output_limit" || tiny.ExternalScanner != nil {
		t.Fatal(tiny)
	}
	p.Layers[0].Limits.MaxOutputBytes = 4096
	for _, kind := range []string{"finding", "error"} {
		calls := 0
		change := func(n int, in []byte) scannerRunResult {
			if n != 2 {
				return scannerRunResult{}
			}
			if kind == "finding" {
				return scannerRunResult{started: true, out: []byte(`[{"RuleID":"x"}]`), errout: scannerTestLog(len(in), 1), exit: 3}
			}
			return scannerRunResult{started: true, code: "execution_incomplete"}
		}
		r := runWithScanner(root, s, p, q, "json", cleanScannerRunner(&calls, change))
		if calls != 2 || r.ExternalScanner.Reuse.Files != 1 || len(r.Rows) != 0 || bytes.Contains(encode(r, "json"), []byte("public A")) {
			t.Fatal(kind, r, calls)
		}
		if kind == "finding" && (r.Status != "blocked" || r.Code != "scanner_finding") {
			t.Fatal(r)
		}
		if kind == "finding" {
			c.ReuseEqualInputs = false
			disabledCalls := 0
			disabled := runWithScanner(root, s, p, q, "json", cleanScannerRunner(&disabledCalls, change))
			if disabled.Status != "blocked" || disabled.Code != "scanner_finding" || disabled.ExternalScanner == nil || disabled.ExternalScanner.Reuse != nil || disabled.ExternalScanner.CompletedFiles != disabled.ExternalScanner.ExpectedFiles || disabledCalls != disabled.ExternalScanner.ExpectedFiles || len(disabled.Rows) != 0 {
				t.Fatal(disabled, disabledCalls)
			}
			for _, format := range []string{"json", "text", "compact"} {
				for _, cue := range []string{scannerTestCUE(t)} {
					vetScannerOutput(t, cue, schema, "#OutputReport", encode(disabled, format), true)
				}
			}
			c.ReuseEqualInputs = true
			for _, cue := range []string{scannerTestCUE(t)} {
				vetScannerOutput(t, cue, schema, "#OutputReport", encode(r, "json"), true)
				var bad map[string]any
				_ = json.Unmarshal(encode(r, "json"), &bad)
				delete(bad["externalScanner"].(map[string]any), "reuse")
				wire, _ := json.Marshal(bad)
				vetScannerOutput(t, cue, schema, "#OutputReport", wire, false)
			}
		}
		if kind == "error" && (r.Status != "incomplete" || r.Code != "scanner_incomplete") {
			t.Fatal(r)
		}
	}
	var first []byte
	calls := 0
	mutate := func(n int, in []byte) scannerRunResult {
		if n == 1 {
			first = in
		}
		if n == 2 {
			first[0] = 'Z'
		}
		return scannerRunResult{}
	}
	mutated := runWithScanner(root, s, p, q, "json", cleanScannerRunner(&calls, mutate))
	if mutated.Status != "incomplete" || mutated.Code != "scanner_incomplete" || mutated.ExternalScanner.Code != "coverage_mismatch" || len(mutated.Rows) != 0 || bytes.Contains(encode(mutated, "json"), []byte("public A")) {
		t.Fatal(mutated, calls)
	}
}

func TestScannerReuseGitAndCUEParity(t *testing.T) {
	root, s, p := gitFixture(t)
	blob := []byte("public\n")
	bid := oidHash("blob", blob, "sha1")
	commit := []byte("tree " + strings.Repeat("a", 40) + "\nauthor A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\npublic\n")
	cid := oidHash("commit", commit, "sha1")
	objects := map[string][]byte{bid: blob, cid: commit}
	s.RequiredScannerIDs = []string{"gitleaks"}
	c := testScannerPolicy(t)
	c.ReuseEqualInputs = true
	p.Scanner = &c
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-object-diff", Base: cid, Target: cid, Paths: []string{"a.txt"}}
	gitRunner := fakeGit(objects, "", ":100644 100644 "+bid+" "+bid+" M\x00a.txt\x00", "")
	schema, e := os.ReadFile("schema.cue")
	if e != nil {
		t.Fatal(e)
	}
	for _, format := range []string{"json", "text", "compact"} {
		calls := 0
		r := runGitWithScanner(root, s, p, q, format, gitRunner, cleanScannerRunner(&calls, nil))
		if r.Status != "no-findings" || r.ExternalScanner == nil || r.ExternalScanner.Reuse == nil || r.ExternalScanner.Reuse.Files < 1 || r.ExternalScanner.CompletedFiles+r.ExternalScanner.Reuse.Files != r.ExternalScanner.ExpectedFiles || calls != r.ExternalScanner.InvokedFiles {
			t.Fatal(format, r, calls)
		}
		c.ReuseEqualInputs = false
		disabledCalls := 0
		disabled := runGitWithScanner(root, s, p, q, format, gitRunner, cleanScannerRunner(&disabledCalls, nil))
		if disabled.Status != "no-findings" || disabled.ExternalScanner == nil || disabled.ExternalScanner.Reuse != nil || disabled.ExternalScanner.CompletedFiles != disabled.ExternalScanner.ExpectedFiles || disabledCalls != disabled.ExternalScanner.ExpectedFiles {
			t.Fatal(format, disabled, disabledCalls)
		}
		for _, cue := range []string{scannerTestCUE(t)} {
			vetScannerOutput(t, cue, schema, "#OutputGitReport", encodeGit(disabled, format), true)
		}
		c.ReuseEqualInputs = true
		for _, cue := range []string{scannerTestCUE(t)} {
			vetScannerOutput(t, cue, schema, "#OutputGitReport", encodeGit(r, format), true)
			var bad map[string]any
			if e := json.Unmarshal(encodeGit(r, format), &bad); e != nil {
				t.Fatal(e)
			}
			es := bad["externalScanner"].(map[string]any)
			reuse := es["reuse"].(map[string]any)
			reuse["profile"] = "invalid"
			b, _ := json.Marshal(bad)
			vetScannerOutput(t, cue, schema, "#OutputGitReport", b, false)
			reuse["profile"] = "invocation-exact/1"
			reuse["files"] = float64(0)
			b, _ = json.Marshal(bad)
			vetScannerOutput(t, cue, schema, "#OutputGitReport", b, false)
			delete(es, "reuse")
			b, _ = json.Marshal(bad)
			vetScannerOutput(t, cue, schema, "#OutputGitReport", b, false)
			es["reuse"] = reuse
			reuse["files"] = float64(r.ExternalScanner.Reuse.Files)
			reuse["unknown"] = true
			b, _ = json.Marshal(bad)
			vetScannerOutput(t, cue, schema, "#OutputGitReport", b, false)
		}
	}
	for _, kind := range []string{"finding", "error"} {
		calls := 0
		change := func(n int, in []byte) scannerRunResult {
			if n != 4 {
				return scannerRunResult{}
			}
			if kind == "finding" {
				return scannerRunResult{started: true, out: []byte(`[{"RuleID":"x"}]`), errout: scannerTestLog(len(in), 1), exit: 3}
			}
			return scannerRunResult{started: true, code: "execution_incomplete"}
		}
		r := runGitWithScanner(root, s, p, q, "json", gitRunner, cleanScannerRunner(&calls, change))
		if r.ExternalScanner.Reuse.Files != 1 || r.Returned != 0 || len(r.Rows) != 0 || len(r.Scope) != 0 || r.BaseCommit != "" || r.TargetCommit != "" || bytes.Contains(encodeGit(r, "json"), []byte("public")) {
			t.Fatal(kind, r, calls)
		}
		if kind == "finding" && (r.Status != "blocked" || r.Code != "scanner_finding") {
			t.Fatal(r)
		}
		if kind == "finding" {
			c.ReuseEqualInputs = false
			disabledCalls := 0
			disabled := runGitWithScanner(root, s, p, q, "json", gitRunner, cleanScannerRunner(&disabledCalls, change))
			if disabled.Status != "blocked" || disabled.Code != "scanner_finding" || disabled.ExternalScanner == nil || disabled.ExternalScanner.Reuse != nil || disabled.ExternalScanner.CompletedFiles != disabled.ExternalScanner.ExpectedFiles || disabledCalls != disabled.ExternalScanner.ExpectedFiles || len(disabled.Rows) != 0 {
				t.Fatal(disabled, disabledCalls)
			}
			for _, format := range []string{"json", "text", "compact"} {
				for _, cue := range []string{scannerTestCUE(t)} {
					vetScannerOutput(t, cue, schema, "#OutputGitReport", encodeGit(disabled, format), true)
				}
			}
			c.ReuseEqualInputs = true
			for _, cue := range []string{scannerTestCUE(t)} {
				vetScannerOutput(t, cue, schema, "#OutputGitReport", encodeGit(r, "json"), true)
				var bad map[string]any
				_ = json.Unmarshal(encodeGit(r, "json"), &bad)
				delete(bad["externalScanner"].(map[string]any), "reuse")
				wire, _ := json.Marshal(bad)
				vetScannerOutput(t, cue, schema, "#OutputGitReport", wire, false)
			}
		}
		if kind == "error" && (r.Status != "incomplete" || r.Code != "scanner_incomplete") {
			t.Fatal(r)
		}
	}
}

func scannerTestCUE(t *testing.T) string {
	t.Helper()
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("cue unavailable; schema qualification untested")
	}
	return cue
}

func vetScannerOutput(t *testing.T, cue string, schema []byte, definition string, wire []byte, want bool) {
	t.Helper()
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "schema.cue"), schema, 0600); e != nil {
		t.Fatal(e)
	}
	fixture := []byte("package commandguard\nv: " + definition + " & " + string(wire) + "\n")
	if e := os.WriteFile(filepath.Join(dir, "value.cue"), fixture, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(cue, "vet", "-c", ".")
	cmd.Dir = dir
	out, e := cmd.CombinedOutput()
	if (e == nil) != want {
		t.Fatalf("%s want=%v: %v\n%s\n%s", cue, want, e, out, wire)
	}
}
