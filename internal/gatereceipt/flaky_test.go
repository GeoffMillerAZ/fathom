package gatereceipt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFlakyTestsSameTreeFailThenPass(t *testing.T) {
	r, err := FlakyTests(flakyFixtureDirs("testdata/flaky"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := []FlakyTest{
		{Package: "fixture/alpha", Test: "TestSlow", FailedRuns: 2, Lost: 30 * time.Second, Trees: []string{strings.Repeat("a", 40)}},
		{Package: "fixture/beta", Test: "TestTieA", FailedRuns: 1, Lost: 20 * time.Second, Trees: []string{strings.Repeat("a", 40)}},
		{Package: "fixture/beta", Test: "TestTieZ", FailedRuns: 1, Lost: 20 * time.Second, Trees: []string{strings.Repeat("a", 40)}},
		{Package: "fixture/alpha", Test: "TestAggregate", FailedRuns: 1, Lost: 10 * time.Second, Trees: []string{strings.Repeat("a", 40)}},
		{Package: "fixture/alpha", Test: "TestChanged", FailedRuns: 1, Lost: 10 * time.Second, Trees: []string{strings.Repeat("a", 40)}},
	}
	if !reflect.DeepEqual(r.Tests, want) || r.UnknownIdentity != 2 {
		t.Fatalf("report: %+v, want %+v and 2 unknown", r, want)
	}
}

func TestFlakyTestsRealGateLogWithoutTaskEchoes(t *testing.T) {
	dir := "testdata/flaky-real"
	for _, name := range []string{"gate.fail.log", "gate.pass.log"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "task: [") {
			t.Fatalf("%s contains Task's stderr echoes", name)
		}
	}
	r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
	if err != nil || r.UnknownIdentity != 0 || len(r.Tests) != 1 {
		t.Fatalf("real gate logs: %+v, %v; want one flake and no unknown identity", r, err)
	}
	if got := r.Tests[0]; got.Package != "fixture/gate" || got.Test != "TestRetry" || got.FailedRuns != 1 || len(got.Trees) != 1 {
		t.Fatalf("real gate flake: %+v", got)
	}
}

func TestFlakyTestsDeterministicDeduplicatedAndSince(t *testing.T) {
	first, err := FlakyTests(flakyFixtureDirs("testdata/flaky"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := FlakyTests(append(flakyFixtureDirs("testdata/flaky"), flakyFixtureDirs("testdata/flaky")...), time.Time{})
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("duplicate input changed report: %+v %v", second, err)
	}
	since := time.Date(2026, 10, 22, 2, 0, 0, 0, time.UTC)
	r, err := FlakyTests(flakyFixtureDirs("testdata/flaky"), since)
	if err != nil || len(r.Tests) != 2 || r.Tests[0].Test != "TestAggregate" || r.Tests[1].Test != "TestChanged" || r.UnknownIdentity != 2 {
		t.Fatalf("since: %+v %v", r, err)
	}
}

func TestFlakyTestsUnboundOrUnreadableReceiptNeverFlaky(t *testing.T) {
	entries, err := os.ReadDir("testdata/flaky/receipts")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"malformed", "missing-log", "altered-log", "dirty", "wrong-tree", "wrong-time"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "receipts")
			logDir := filepath.Join(root, "workspace/logs")
			fail, err := os.ReadFile("testdata/flaky/workspace/logs/gate.01-fail.log")
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(logDir, "gate.fail"), string(fail))
			for _, e := range entries {
				b, err := os.ReadFile(filepath.Join("testdata/flaky/receipts", e.Name()))
				if err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(dir, e.Name()), string(b))
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".json") {
					continue
				}
				p := filepath.Join(dir, e.Name())
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				lp := strings.TrimSuffix(p, ".json") + ".log"
				switch kind {
				case "malformed":
					put(t, p, "{")
				case "missing-log":
					if err := os.Remove(lp); err != nil {
						t.Fatal(err)
					}
				case "altered-log":
					put(t, lp, "--- PASS: TestSlow (0.01s)\n")
				default:
					var rec Receipt
					if err := json.Unmarshal(b, &rec); err != nil {
						t.Fatal(err)
					}
					switch kind {
					case "dirty":
						rec.Dirty = true
					case "wrong-tree":
						rec.Tree = strings.Repeat("b", 40)
					case "wrong-time":
						rec.End = "2026-10-22T06:00:00Z"
					}
					data, err := json.Marshal(rec)
					if err != nil {
						t.Fatal(err)
					}
					name := digest(data)
					log, err := os.ReadFile(lp)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(p); err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(lp); err != nil {
						t.Fatal(err)
					}
					put(t, filepath.Join(dir, name+".json"), string(data))
					put(t, filepath.Join(dir, name+".log"), string(log))
				}
			}
			r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyReceipts}, {Path: logDir, Role: FlakyGateLogs}}, time.Time{})
			wantUnknown := 1
			if kind == "missing-log" || kind == "altered-log" {
				wantUnknown = 0 // No gate header survives in these inputs.
			}
			if err != nil || len(r.Tests) != 0 || r.UnknownIdentity != wantUnknown {
				t.Fatalf("%s: %+v %v", kind, r, err)
			}
		})
	}
}

func TestFlakyTestsMissingDirectory(t *testing.T) {
	r, err := FlakyTests([]FlakyDir{{Path: filepath.Join(t.TempDir(), "absent"), Role: FlakyReceipts}}, time.Time{})
	if err != nil || len(r.Tests) != 0 {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestFlakyTestsImportedCopiesAndJSONPass(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "workspace/logs")
	b, err := os.ReadFile("testdata/flaky/workspace/logs/gate.01-fail.log")
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "gate.copy"), string(b))
	receiptDir := t.TempDir()
	entries, err := os.ReadDir("testdata/flaky/receipts")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) < 64 {
			continue
		}
		b, err := os.ReadFile(filepath.Join("testdata/flaky/receipts", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		put(t, filepath.Join(receiptDir, e.Name()), string(b))
	}
	first, err := FlakyTests(flakyFixtureDirs("testdata/flaky"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := FlakyTests(append([]FlakyDir{{Path: receiptDir, Role: FlakyReceipts}, {Path: dir, Role: FlakyGateLogs}}, flakyFixtureDirs("testdata/flaky")...), time.Time{})
	if err != nil || !reflect.DeepEqual(first, r) {
		t.Fatalf("copies: %+v %v", r, err)
	}
	pass := strings.ReplaceAll(string(b), "01:00:", "01:03:")
	pass = strings.NewReplacer("--- FAIL:", "--- PASS:", "FAIL\t", "ok\t", "validate FAIL", "validate PASS").Replace(pass)
	pass = strings.ReplaceAll(pass, "--- PASS: TestSlow (0.01s)", `{"Action":"pass","Package":"fixture/alpha","Test":"TestSlow"}`+"\n"+`{"Action":"pass","Package":"fixture/alpha"}`)
	put(t, filepath.Join(dir, "gate.pass"), pass)
	r, err = FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
	if err != nil || len(r.Tests) != 3 || r.Tests[0].FailedRuns != 1 {
		t.Fatalf("JSON pass: %+v %v", r, err)
	}
}

func TestFlakyTestsMalformedStartUnknown(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "workspace/logs")
	put(t, filepath.Join(dir, "gate.log"), "fathom gate: validate START \n--- FAIL: TestBroken (0.01s)\nFAIL\tfixture/alpha\t0.01s\nfathom gate: validate FAIL 2026-10-22T01:00:00Z\n")
	r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
	if err != nil || len(r.Tests) != 0 || r.UnknownIdentity != 1 {
		t.Fatalf("malformed: %+v %v", r, err)
	}
}

func flakyFixtureDirs(root string) []FlakyDir {
	return []FlakyDir{
		{Path: filepath.Join(root, "workspace/logs"), Role: FlakyGateLogs},
		{Path: filepath.Join(root, "receipts"), Role: FlakyReceipts},
		{Path: filepath.Join(root, "workspace/tmp/test-run"), Role: FlakySavedRuns},
	}
}

func flakyLog(t *testing.T, start, tree, command, goos, arch, verdict string) string {
	t.Helper()
	stamp, err := time.Parse(time.RFC3339, start)
	if err != nil {
		t.Fatal(err)
	}
	status := "PASS"
	if strings.Contains(verdict, "--- FAIL:") {
		status = "FAIL"
	}
	identity := " tree=" + tree + " clean=1 commit=" + strings.Repeat("c", 40)
	return "fathom gate: test START " + start + identity + "\n" +
		"go version fixture fixture/fixture\n" + envHeader + "\n" + goos + "\n" + arch + "\noff\n0\n\n" +
		command + "\n" + verdict + "\nfathom gate: test " + status + " " + stamp.Add(time.Second).Format(time.RFC3339) + identity + "\n"
}

func TestFlakyTestsPackageOKRequiresMatchingScopeAndPlatform(t *testing.T) {
	tree := strings.Repeat("a", 40)
	command := "task: [test] go test -race -run TestRetry -skip TestOther ./alpha ./beta"
	for _, verbose := range []bool{false, true} {
		for _, change := range []string{"same", "run", "skip", "packages", "tree", "os", "arch", "missing-command", "missing-environment", "echoed-filter-with-continuation", "echoed-filter-with-blank-continuation"} {
			t.Run(fmt.Sprintf("verbose=%t/%s", verbose, change), func(t *testing.T) {
				failCommand, passCommand, passTree, goos, arch := command, command, tree, "fixture-os", "fixture-arch"
				switch change {
				case "run":
					passCommand = strings.Replace(command, "-run TestRetry", "-run TestOther", 1)
				case "skip":
					passCommand = strings.Replace(command, "-skip TestOther", "-skip TestRetry", 1)
				case "packages":
					passCommand = strings.Replace(command, "./alpha ./beta", "./alpha", 1)
				case "tree":
					passTree = strings.Repeat("b", 40)
				case "os":
					goos = "other-os"
				case "arch":
					arch = "other-arch"
				case "missing-command":
					failCommand, passCommand = "", ""
				case "missing-environment":
					goos, arch = "", ""
				case "echoed-filter-with-continuation", "echoed-filter-with-blank-continuation":
					failCommand = "task: [test] go test -race -run TestRetry ./alpha\nother output"
					if change == "echoed-filter-with-blank-continuation" {
						failCommand = strings.Replace(failCommand, "\nother output", "\n\nother output", 1)
					}
					passCommand = strings.Replace(failCommand, "TestRetry", "TestOther", 1)
				}
				dir := filepath.Join(t.TempDir(), "workspace/logs")
				put(t, filepath.Join(dir, "gate.fail"), flakyLog(t, "2026-10-22T01:00:00Z", tree, failCommand, "fixture-os", "fixture-arch", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s"))
				verdict := "ok\tfixture/alpha\t0.01s"
				if verbose {
					verdict = "--- PASS: TestRetry (0.01s)\n" + verdict
				}
				put(t, filepath.Join(dir, "gate.pass"), flakyLog(t, "2026-10-22T01:01:00Z", passTree, passCommand, goos, arch, verdict))
				r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
				want := 0
				if change != "tree" && change != "os" && change != "arch" && change != "missing-environment" {
					want = 1
				}
				if err != nil || len(r.Tests) != want {
					t.Fatalf("got %+v, %v; want %d flaky tests", r, err, want)
				}
			})
		}
	}
}

func TestFlakyTestsOnlyEvidenceNamesAndDatedSince(t *testing.T) {
	root := t.TempDir()
	dirs := flakyFixtureDirs(root)
	tree := strings.Repeat("a", 40)
	fail := flakyLog(t, "2026-10-22T01:00:00Z", tree, "task: [test] go test ./alpha", "fixture-os", "fixture-arch", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s")
	pass := flakyLog(t, "2026-10-22T01:01:00Z", tree, "task: [test] go test ./alpha", "fixture-os", "fixture-arch", "ok\tfixture/alpha\t0.01s")
	put(t, filepath.Join(dirs[0].Path, "gate.fail"), fail)
	// Saved hook logs are accepted only under the saved-run directory.
	put(t, filepath.Join(dirs[2].Path, strings.Repeat("a", 16)+".log"), pass)
	for _, dir := range dirs {
		for _, name := range []string{"unrelated.log", "unrelated.json", "gate-lookalike.log", strings.Repeat("g", 64) + ".json"} {
			put(t, filepath.Join(dir.Path, name), "unrelated output")
		}
	}
	put(t, filepath.Join(dirs[0].Path, strings.Repeat("a", 64)+".json"), "{}")
	put(t, filepath.Join(dirs[1].Path, "gate.stray"), "unrelated output")
	put(t, filepath.Join(dirs[1].Path, strings.Repeat("b", 64)+".log"), "orphan output")
	put(t, filepath.Join(dirs[2].Path, strings.Repeat("c", 64)+".json"), "{}")
	put(t, filepath.Join(dirs[2].Path, "gate.stray"), "unrelated output")
	// A selected but undated run is visible only without --since.
	put(t, filepath.Join(dirs[0].Path, "gate.undated"), "fathom gate: test START\n")
	r, err := FlakyTests(dirs, time.Time{})
	if err != nil || len(r.Tests) != 1 || r.UnknownIdentity != 1 {
		t.Fatalf("selected: %+v %v", r, err)
	}
	r, err = FlakyTests(dirs, time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC))
	if err != nil || len(r.Tests) != 1 || r.UnknownIdentity != 0 {
		t.Fatalf("since: %+v %v", r, err)
	}
}

func TestFlakyTestsCachedPackagesAreNoEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		freshFailure bool
	}{
		{"text", "ok\tfixture/alpha\t(cached)", false},
		{"verbose", "--- PASS: TestRetry (0.01s)\nok\tfixture/alpha\t(cached)", false},
		{"json", `{"Action":"pass","Package":"fixture/alpha","Test":"TestRetry"}` + "\n" +
			`{"Action":"output","Package":"fixture/alpha","Output":"ok\tfixture/alpha\t(cached)\n"}` + "\n" +
			`{"Action":"pass","Package":"fixture/alpha"}`, false},
		{"json-zero-elapsed-spacing", `{"Action":"pass","Package":"fixture/alpha","Test":"TestRetry"}` + "\n" +
			`{"Action":"output","Package":"fixture/alpha","Output":"ok  \tfixture/alpha\t(cached)\n"}` + "\n" +
			`{"Action":"pass","Package":"fixture/alpha","Elapsed":0}`, false},
		{"replayed-failure", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s\n--- PASS: TestRetry (0.01s)\nok\tfixture/alpha\t(cached)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cases, passed := flakyOutcomes(tc.output + "\n--- PASS: TestOther (0.01s)\nok\tfixture/beta\t0.01s")
			if _, found := passed["fixture/alpha"]; found != tc.freshFailure || passed["fixture/alpha"] {
				t.Errorf("cached package retained: %+v", passed)
			}
			wantCases := map[flakyCase]bool{{"fixture/beta", "TestOther"}: false}
			if tc.freshFailure {
				wantCases[flakyCase{"fixture/alpha", "TestRetry"}] = true
			}
			if !reflect.DeepEqual(cases, wantCases) || !passed["fixture/beta"] {
				t.Errorf("cases=%+v passed=%+v; want cases=%+v and only fresh package results", cases, passed, wantCases)
			}
			dir := filepath.Join(t.TempDir(), "workspace/logs")
			tree, command := strings.Repeat("a", 40), "task: [test] go test ./alpha"
			put(t, filepath.Join(dir, "gate.fail"), flakyLog(t, "2026-10-22T01:00:00Z", tree, command, "fixture-os", "fixture-arch", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s"))
			put(t, filepath.Join(dir, "gate.pass"), flakyLog(t, "2026-10-22T01:01:00Z", tree, command, "fixture-os", "fixture-arch", tc.output))
			r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
			if err != nil || len(r.Tests) != 0 || r.UnknownIdentity != 0 {
				t.Fatalf("cached replay became a new run: %+v, %v", r, err)
			}
		})
	}
}

func TestFlakyTestsCachedResultsPreserveFreshSteps(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		for _, cachedFirst := range []bool{false, true} {
			for _, freshFailed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/cachedFirst=%t/freshFailed=%t", format, cachedFirst, freshFailed), func(t *testing.T) {
					verdict, summary, action := "PASS", "ok", "pass"
					if freshFailed {
						verdict, summary, action = "FAIL", "FAIL", "fail"
					}
					cached := "--- PASS: TestCachedOnly (0.01s)\nok\tfixture/alpha\t(cached)\n"
					fresh := "--- " + verdict + ": TestRetry (0.01s)\n" + summary + "\tfixture/alpha\t0.01s\n"
					other := "--- PASS: TestOther (0.01s)\nok\tfixture/beta\t0.01s\n"
					if format == "json" {
						cached = `{"Action":"pass","Package":"fixture/alpha","Test":"TestCachedOnly"}` + "\n" +
							`{"Action":"output","Package":"fixture/alpha","Output":"ok  \tfixture/alpha\t(cached)\n"}` + "\n" +
							`{"Action":"pass","Package":"fixture/alpha","Elapsed":0}` + "\n"
						// Another package may finish while alpha's events are buffered.
						other = `{"Action":"pass","Package":"fixture/beta","Test":"TestOther"}` + "\n" +
							`{"Action":"pass","Package":"fixture/beta"}` + "\n"
						fresh = fmt.Sprintf("{\"Action\":%q,\"Package\":\"fixture/alpha\",\"Test\":\"TestRetry\"}\n", action) + other +
							fmt.Sprintf("{\"Action\":\"output\",\"Package\":\"fixture/alpha\",\"Output\":%q}\n", summary+"\tfixture/alpha\t0.01s\n") +
							fmt.Sprintf("{\"Action\":%q,\"Package\":\"fixture/alpha\"}\n", action)
						other = ""
					}
					output := fresh + cached + other
					if cachedFirst {
						output = cached + fresh + other
					}
					cases, passed := flakyOutcomes(output)
					wantCases := map[flakyCase]bool{{"fixture/alpha", "TestRetry"}: freshFailed, {"fixture/beta", "TestOther"}: false}
					if !reflect.DeepEqual(cases, wantCases) || len(passed) != 2 || passed["fixture/alpha"] != !freshFailed || !passed["fixture/beta"] {
						t.Fatalf("cases=%+v passed=%+v; want cases=%+v and only fresh package results", cases, passed, wantCases)
					}
					dir := t.TempDir()
					tree := strings.Repeat("a", 40)
					put(t, filepath.Join(dir, "gate.fail"), flakyLog(t, "2026-10-22T01:00:00Z", tree, "", "fixture-os", "fixture-arch", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s"))
					put(t, filepath.Join(dir, "gate.mixed"), flakyLog(t, "2026-10-22T01:01:00Z", tree, "", "fixture-os", "fixture-arch", output))
					r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
					want := 1
					if freshFailed {
						want = 0
					}
					if err != nil || len(r.Tests) != want || r.UnknownIdentity != 0 {
						t.Fatalf("mixed run: %+v, %v; want %d flaky tests", r, err, want)
					}
					put(t, filepath.Join(dir, "gate.recovered"), flakyLog(t, "2026-10-22T01:02:00Z", tree, "", "fixture-os", "fixture-arch", "ok\tfixture/alpha\t0.01s"))
					r, err = FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
					wantFailed := 1
					if freshFailed {
						wantFailed = 2
					}
					if err != nil || len(r.Tests) != 1 || r.Tests[0].FailedRuns != wantFailed || r.UnknownIdentity != 0 {
						t.Fatalf("recovered: %+v, %v; want %d failing runs", r, err, wantFailed)
					}
				})
			}
		}
	}
}

func TestFlakyTestsJSONNeedsPackageResult(t *testing.T) {
	output := `{"Action":"pass","Package":"fixture/alpha","Test":"TestRetry"}` + "\n" +
		`{"Action":"output","Package":"fixture/alpha","Output":"ok\tfixture/alpha\t0.01s\n"}` + "\n" +
		`{"Action":"pass","Package":"fixture/beta","Test":"TestOther"}` + "\n" +
		`{"Action":"pass","Package":"fixture/beta"}`
	cases, passed := flakyOutcomes(output)
	if !reflect.DeepEqual(cases, map[flakyCase]bool{{"fixture/beta", "TestOther"}: false}) || !reflect.DeepEqual(passed, map[string]bool{"fixture/beta": true}) {
		t.Fatalf("unfinished JSON package became evidence: cases=%+v passed=%+v", cases, passed)
	}
}

func TestFlakyTestsOnlyGateHeadersCountAsUnknown(t *testing.T) {
	dirs := flakyFixtureDirs(t.TempDir())
	put(t, filepath.Join(dirs[2].Path, strings.Repeat("a", 16)+".log"), "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s\n")
	put(t, filepath.Join(dirs[2].Path, strings.Repeat("b", 16)+".log"), "ordinary Bash output\n")
	put(t, filepath.Join(dirs[0].Path, "gate.not-a-gate"), "ordinary Bash output\n")
	put(t, filepath.Join(dirs[0].Path, "gate.unfinished"), "fathom gate: test START\n")
	r, err := FlakyTests(dirs, time.Time{})
	if err != nil || len(r.Tests) != 0 || r.UnknownIdentity != 1 {
		t.Fatalf("non-gate noise: %+v, %v; want only the unfinished gate to have unknown identity", r, err)
	}
}

func TestFlakyTestsRequiresMatchingToolchainAndEnvironment(t *testing.T) {
	for _, format := range []string{"text", "verbose", "json"} {
		for _, change := range []string{"same", "version", "cgo", "tags", "count", "work"} {
			t.Run(format+"/"+change, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "workspace/logs")
				tree, command := strings.Repeat("a", 40), "task: [test] go test ./alpha"
				put(t, filepath.Join(dir, "gate.fail"), flakyLog(t, "2026-10-22T01:00:00Z", tree, command, "fixture-os", "fixture-arch", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s"))
				verdict := "ok\tfixture/alpha\t0.01s"
				switch format {
				case "verbose":
					verdict = "--- PASS: TestRetry (0.01s)\n" + verdict
				case "json":
					verdict = `{"Action":"pass","Package":"fixture/alpha","Test":"TestRetry"}` + "\n" + `{"Action":"pass","Package":"fixture/alpha"}`
				}
				lines := strings.Split(flakyLog(t, "2026-10-22T01:01:00Z", tree, command, "fixture-os", "fixture-arch", verdict), "\n")
				switch change {
				case "version":
					lines[1] = "go version other fixture/fixture"
				case "cgo":
					lines[6] = "1"
				case "tags":
					lines[7] = "-tags=fixture"
				case "count":
					lines[7] = "-count=1"
				case "work":
					lines[5] = "fixture.work"
				}
				put(t, filepath.Join(dir, "gate.pass"), strings.Join(lines, "\n"))
				r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
				want := 0
				if change == "same" {
					want = 1
				}
				if err != nil || len(r.Tests) != want || r.UnknownIdentity != 0 {
					t.Fatalf("report=%+v, %v; want %d flaky tests", r, err, want)
				}
			})
		}
	}
}

func TestFlakyTestsIdentityIgnoresEchoesAndRequiresGateName(t *testing.T) {
	command := "task: [test] go test ./alpha\ntask: [vet] go vet ./alpha"
	for _, tc := range []struct {
		name, command string
		otherGate     bool
		want          int
	}{
		{"same", command, false, 1},
		{"reordered", "task: [vet] go vet ./alpha\ntask: [test] go test ./alpha", false, 1},
		{"duplicate", command + "\ntask: [test] go test ./alpha", false, 1},
		{"interleaved", "task: [vet] go vet ./alpha\nvet output\ntask: [test] go test ./alpha\ntest output", false, 1},
		{"after-result", "task: [test] go test ./alpha\nok\tfixture/beta\t0.01s\ntask: [vet] go vet ./alpha", false, 1},
		{"different-step", strings.Replace(command, "go vet ./alpha", "go vet ./beta", 1), false, 1},
		{"no-echoes", "", false, 1},
		{"different-gate", command, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "workspace/logs")
			tree := strings.Repeat("a", 40)
			put(t, filepath.Join(dir, "gate.fail"), flakyLog(t, "2026-10-22T01:00:00Z", tree, command, "fixture-os", "fixture-arch", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s"))
			pass := flakyLog(t, "2026-10-22T01:01:00Z", tree, tc.command, "fixture-os", "fixture-arch", "ok\tfixture/alpha\t0.01s")
			if tc.otherGate {
				pass = strings.ReplaceAll(pass, "fathom gate: test ", "fathom gate: validate ")
			}
			put(t, filepath.Join(dir, "gate.pass"), pass)
			r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
			if err != nil || len(r.Tests) != tc.want {
				t.Fatalf("report=%+v, %v; want %d flaky tests", r, err, tc.want)
			}
		})
	}
}

func TestFlakyTestsPackageFailureCannotBeOverridden(t *testing.T) {
	for _, verdict := range []string{
		"FAIL\tfixture/alpha\t0.01s\nok\tfixture/alpha\t0.01s",
		"ok\tfixture/alpha\t0.01s\nFAIL\tfixture/alpha\t0.01s",
		`{"Action":"fail","Package":"fixture/alpha"}` + "\n" + `{"Action":"pass","Package":"fixture/alpha"}`,
		`{"Action":"pass","Package":"fixture/alpha"}` + "\n" + `{"Action":"fail","Package":"fixture/alpha"}`,
	} {
		t.Run(verdict, func(t *testing.T) {
			_, passed := flakyOutcomes(verdict)
			if passed["fixture/alpha"] {
				t.Errorf("package failure overridden: %+v", passed)
			}
			dir := filepath.Join(t.TempDir(), "workspace/logs")
			tree, command := strings.Repeat("a", 40), "task: [test] go test ./alpha"
			put(t, filepath.Join(dir, "gate.fail"), flakyLog(t, "2026-10-22T01:00:00Z", tree, command, "fixture-os", "fixture-arch", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s"))
			put(t, filepath.Join(dir, "gate.mixed"), flakyLog(t, "2026-10-22T01:01:00Z", tree, command, "fixture-os", "fixture-arch", verdict))
			r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
			if err != nil || len(r.Tests) != 0 {
				t.Fatalf("mixed package results discharged failure: %+v, %v", r, err)
			}
		})
	}
}

func TestFlakyTestsDirectoryRolesIgnorePathNames(t *testing.T) {
	root := t.TempDir()
	original := flakyFixtureDirs(filepath.Join(root, "original"))
	for i, dir := range flakyFixtureDirs("testdata/flaky") {
		entries, err := os.ReadDir(dir.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			b, err := os.ReadFile(filepath.Join(dir.Path, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(original[i].Path, entry.Name()), string(b))
		}
	}
	tree, command := strings.Repeat("a", 40), "task: [test] go test ./alpha"
	put(t, filepath.Join(original[0].Path, "gate.retry"), flakyLog(t, "2026-10-22T01:00:00Z", tree, command, "fixture-os", "fixture-arch", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s"))
	put(t, filepath.Join(original[2].Path, strings.Repeat("a", 16)+".log"), flakyLog(t, "2026-10-22T01:01:00Z", tree, command, "fixture-os", "fixture-arch", "ok\tfixture/alpha\t0.01s"))
	dirs := []FlakyDir{
		{Path: filepath.Join(root, "gates"), Role: FlakyGateLogs},
		{Path: filepath.Join(root, "workspace/logs"), Role: FlakyReceipts},
		{Path: filepath.Join(root, "saved"), Role: FlakySavedRuns},
	}
	for i, dir := range original {
		entries, err := os.ReadDir(dir.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			b, err := os.ReadFile(filepath.Join(dir.Path, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(dirs[i].Path, entry.Name()), string(b))
		}
	}
	want, err := FlakyTests(original, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := FlakyTests(dirs, time.Time{})
	if err != nil || !reflect.DeepEqual(r, want) {
		t.Fatalf("renaming directories changed evidence: %+v, %v; want %+v", r, err, want)
	}
}

func TestFlakyTestsPackageOKCountsOnlyFromPassedGate(t *testing.T) {
	tree := strings.Repeat("a", 40)
	command := "task: [test] go test ./alpha ./beta"
	for _, laterPass := range []bool{false, true} {
		t.Run(fmt.Sprintf("laterPass=%t", laterPass), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "workspace/logs")
			put(t, filepath.Join(dir, "gate.fail"), flakyLog(t, "2026-10-22T01:00:00Z", tree, command, "fixture-os", "fixture-arch", "--- FAIL: TestRetry (0.01s)\nFAIL\tfixture/alpha\t0.01s"))
			verdict := "ok\tfixture/alpha\t0.01s"
			if !laterPass {
				verdict += "\n--- FAIL: TestOther (0.01s)\nFAIL\tfixture/beta\t0.01s"
			}
			put(t, filepath.Join(dir, "gate.later"), flakyLog(t, "2026-10-22T01:01:00Z", tree, command, "fixture-os", "fixture-arch", verdict))
			r, err := FlakyTests([]FlakyDir{{Path: dir, Role: FlakyGateLogs}}, time.Time{})
			want := 0
			if laterPass {
				want = 1
			}
			if err != nil || len(r.Tests) != want {
				t.Fatalf("got %+v, %v; want %d flaky tests", r, err, want)
			}
		})
	}
}
