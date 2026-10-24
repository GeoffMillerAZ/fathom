package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestCmdPlansWeightAcceptWritesAuthoredWeight(t *testing.T) {
	for _, tc := range []struct{ name, annotation, wantAnnotation, before string }{
		{"replace", `{id: W-1, symbol: Build, package: internal/example, reachable_from: TestBuild, weight: 13, reason: "keep, {weight: 99}"}`, `{id: W-1, symbol: Build, package: internal/example, reachable_from: TestBuild, weight: 3, reason: "keep, {weight: 99}"}`, "13"},
		{"append", `{id: W-1, symbol: Build, package: internal/example, reachable_from: TestBuild}`, `{id: W-1, symbol: Build, package: internal/example, reachable_from: TestBuild, weight: 3}`, "absent"},
		{"wrapped", "{id: W-1,\n    symbol: Build, package: internal/example, reachable_from: TestBuild, weight: 13}", "{id: W-1,\n    symbol: Build, package: internal/example, reachable_from: TestBuild, weight: 3}", "13"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "plan.md")
			prefix := "# Plan\r\n\r\n- [ ] Keep {id: W-0, weight: 9}\r\n- [~] Build{} "
			suffix := "  \r\n- [x] Keep {id: W-2, weight: 8}\r\n"
			raw := prefix + tc.annotation + suffix
			if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			var out, errs bytes.Buffer
			args := []string{"weight", "accept", "W-1", "--plan", file}
			if code := cmdPlans(args, &out, &errs); code != 0 {
				t.Fatalf("exit %d: %s", code, errs.String())
			}
			got, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if want := prefix + tc.wantAnnotation + suffix; string(got) != want {
				t.Fatalf("edited unrelated bytes:\ngot  %q\nwant %q", got, want)
			}
			if !strings.Contains(out.String(), "weight: "+tc.before+" -> 3") {
				t.Fatalf("missing before/after: %s", out.String())
			}
			info, err := os.Stat(file)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0600 {
				t.Fatalf("mode changed: %v", info.Mode())
			}
			out.Reset()
			errs.Reset()
			if code := cmdPlans(args, &out, &errs); code != 0 {
				t.Fatalf("second exit %d: %s", code, errs.String())
			}
			again, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, again) || !strings.Contains(out.String(), "weight: 3 -> 3") {
				t.Fatalf("not idempotent: %s", out.String())
			}
		})
	}
}

func TestCmdPlansWeightAcceptRefusesUnknownRow(t *testing.T) {
	file := filepath.Join(t.TempDir(), "plan.md")
	raw := []byte("- [ ] Keep {id: W-1, weight: 9}\n")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := cmdPlans([]string{"weight", "accept", "missing", "--plan", file}, &out, &errs); code == 0 {
		t.Fatal("accepted unknown row")
	}
	if !strings.Contains(errs.String(), "not found") {
		t.Fatalf("diagnostic: %s", errs.String())
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, got) || out.Len() != 0 {
		t.Fatal("unknown row changed plan or printed success")
	}
}

func TestCmdPlansWeightAcceptRefusesAmbiguousOrUnsafe(t *testing.T) {
	for _, raw := range []string{
		"- [ ] A {id: W-1}\n- [ ] B {id: W-1}\n",
		"- [ ] A {id: W-1, weight: 2, weight: 3}\n",
	} {
		file := filepath.Join(t.TempDir(), "plan.md")
		if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		var out, errs bytes.Buffer
		if code := cmdPlans([]string{"weight", "accept", "W-1", "--plan", file}, &out, &errs); code == 0 {
			t.Fatal("accepted ambiguous row")
		}
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != raw {
			t.Fatal("refusal changed plan")
		}
	}
}

func TestCmdPlansWeightAcceptDiscoversPlan(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "example.md")
	if err := os.WriteFile(file, []byte("- [ ] Proof {id: W-1, oracle: lexical, path: proof.md}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	var out, errs bytes.Buffer
	if code := cmdPlans([]string{"weight", "accept", "W-1"}, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "weight: 2") {
		t.Fatalf("missing accepted weight: %s", got)
	}
}

func TestCmdPlansWeightAcceptPreservesWrappedWeight(t *testing.T) {
	for _, ann := range []string{
		"{id: W-1,\n    weight: 13,\n    symbol: Build, reachable_from: TestBuild}",
		"{id: W-1, symbol: Build, reachable_from: TestBuild,\r\n    weight: 13\r\n    }",
		`{id: W-1, symbol: Build, reachable_from: TestBuild, reason: "quoted, {weight: 99}", weight: 13}`,
	} {
		file := filepath.Join(t.TempDir(), "plan.md")
		raw := "- [ ] Build{} " + ann + "  \n"
		if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		var out, errs bytes.Buffer
		if code := cmdPlans([]string{"weight", "accept", "W-1", "--plan", file}, &out, &errs); code != 0 {
			t.Fatalf("exit %d: %s", code, &errs)
		}
		got, _ := os.ReadFile(file)
		if want := strings.Replace(raw, "weight: 13", "weight: 3", 1); string(got) != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestCmdPlansWeightAcceptDryRunAndMalformed(t *testing.T) {
	for _, tc := range []struct {
		weight, before string
		dry, refuse    bool
	}{
		{"13", "13", true, false}, {"", "absent", true, false}, {"abc", "", false, true}, {"0", "", true, true}, {"-1", "", false, true},
	} {
		file := filepath.Join(t.TempDir(), "plan.md")
		raw := "- [ ] Build {id: W-1, symbol: Build, reachable_from: TestBuild"
		if tc.weight != "" {
			raw += ", weight: " + tc.weight
		}
		raw += "}\n"
		if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		args := []string{"weight", "accept", "W-1", "--plan", file}
		if tc.dry {
			args = append(args, "--dry-run")
		}
		var out, errs bytes.Buffer
		code := cmdPlans(args, &out, &errs)
		if tc.refuse {
			if code == 0 || !strings.Contains(errs.String(), "W-1") || !strings.Contains(errs.String(), "weight") {
				t.Fatalf("malformed weight: exit %d: %s", code, &errs)
			}
		} else if code != 0 || !strings.Contains(out.String(), tc.before+" -> 3") || !strings.Contains(out.String(), "estimate (no index evidence)") {
			t.Fatalf("dry run: exit %d: %s %s", code, &out, &errs)
		}
		got, _ := os.ReadFile(file)
		if string(got) != raw {
			t.Fatal("dry run or refusal wrote the plan")
		}
	}
}

func TestPlansWeightAcceptHelp(t *testing.T) {
	for _, args := range [][]string{{"plans", "weight", "--help"}, {"plans", "weight", "accept", "--help"}} {
		var out, errs bytes.Buffer
		if code := run(args, &out, &errs); code != 0 || errs.Len() != 0 || !strings.Contains(out.String(), "accept") {
			t.Fatalf("%v: exit %d, out %q, err %q", args, code, &out, &errs)
		}
		if len(args) == 4 && !strings.Contains(out.String(), "dry-run") {
			t.Fatal("accept help omits dry-run")
		}
	}
}

func TestPlansContextEstimatedWeight(t *testing.T) {
	preflightTestEnv(t)
	storePath := seedPlansFixtureStore(t)
	pf := writeTempPlan(t, "- [ ] Row {id: W-1, symbol: validateCharge, weight: 13}\n")
	for _, pre := range []bool{false, true} {
		args := []string{"plans", "context", pf, "--row", "W-1", "--service", "payment-service", "--store", storePath}
		if pre {
			args = append(args, "--preflight")
		}
		var out, errs bytes.Buffer
		if code := run(args, &out, &errs); code != 0 || !strings.Contains(out.String(), "authored weight 13; estimated weight") {
			t.Fatalf("exit %d: %s %s", code, &out, &errs)
		}
	}
}

func TestPlansContextRowCardEstimatedWeight(t *testing.T) {
	row := console.PlanRowFact{ID: "W-1", Symbol: "Build", ReachableFrom: "TestBuild"}
	view := console.RowEvidence(console.RowEvidenceInput{Plan: console.PlanFacts{Rows: []console.PlanRowFact{row}}, RowID: row.ID})
	estimate := console.EstimateRowWeight(console.RowWeightInput{ContractKind: console.ContractReach, AuthoredWeight: 13})
	page := renderRowEvidence(view, "fathom verify", "", estimate)
	for _, want := range []string{"authored weight 13", "estimated weight 3", "estimate (no index evidence)"} {
		if !strings.Contains(page, want) {
			t.Fatalf("row card omits %q: %s", want, page)
		}
	}
}

func TestCmdPlansWeightAcceptUsesContextEstimate(t *testing.T) {
	preflightTestEnv(t)
	storePath := seedPlansFixtureStore(t)
	pf := writeTempPlan(t, "- [ ] Prerequisite {id: W-0}\n- [ ] Row {id: W-1, symbol: validateCharge, needs: W-0, weight: 13}\n")
	raw, err := os.ReadFile(pf)
	if err != nil {
		t.Fatal(err)
	}
	var contextOut, errs bytes.Buffer
	if code := cmdPlans([]string{"context", pf, "--row", "W-1", "--service", "payment-service", "--store", storePath}, &contextOut, &errs); code != 0 {
		t.Fatalf("context exit %d: %s", code, &errs)
	}
	if !strings.Contains(contextOut.String(), "authored weight 13; estimated weight 5; estimate (no similar-row effort)") {
		t.Fatalf("indexed context: %s", &contextOut)
	}
	var out bytes.Buffer
	args := []string{"weight", "accept", "W-1", "--plan", pf, "--store", storePath, "--service", "payment-service"}
	if code := cmdPlans(append(append([]string(nil), args...), "--dry-run"), &out, &errs); code != 0 {
		t.Fatalf("preview exit %d: %s", code, &errs)
	}
	preview := out.String()
	got, _ := os.ReadFile(pf)
	if !bytes.Equal(got, raw) {
		t.Fatal("indexed preview wrote plan")
	}
	out.Reset()
	if code := cmdPlans(args, &out, &errs); code != 0 {
		t.Fatalf("accept exit %d: %s", code, &errs)
	}
	if out.String() != preview || !strings.Contains(preview, "13 -> 5") {
		t.Fatalf("preview differs from acceptance: %q / %q", preview, &out)
	}
	got, _ = os.ReadFile(pf)
	if string(got) != strings.Replace(string(raw), "weight: 13", "weight: 5", 1) {
		t.Fatalf("accept differs from context: %s", got)
	}
}

func TestCmdPlansWeightAcceptMissingIndexDoesNotCreateStore(t *testing.T) {
	file := filepath.Join(t.TempDir(), "plan.md")
	raw := "- [ ] Row {id: W-1, symbol: Build}\n"
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	for _, dry := range []bool{false, true} {
		var out, errs bytes.Buffer
		args := []string{"weight", "accept", "W-1", "--plan", file, "--store", missing, "--service", "example"}
		if dry {
			args = append(args, "--dry-run")
		}
		if code := cmdPlans(args, &out, &errs); code == 0 || !strings.Contains(errs.String(), "store") || out.Len() != 0 {
			t.Fatalf("unreadable explicit store: exit %d: %s %s", code, &out, &errs)
		}
		got, err := os.ReadFile(file)
		if err != nil || string(got) != raw {
			t.Fatalf("refusal changed plan: %s, %v", got, err)
		}
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing index was created: %v", err)
	}
}

func TestCmdPlansWeightAcceptRefusesCorruptStore(t *testing.T) {
	file := writeTempPlan(t, "- [ ] Row {id: W-1, symbol: Build}\n")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(t.TempDir(), "bad.db")
	if err := os.WriteFile(bad, []byte("not a sqlite store"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := cmdPlans([]string{"weight", "accept", "W-1", "--plan", file, "--store", bad, "--service", "example"}, &out, &errs); code == 0 || !strings.Contains(errs.String(), "store") || out.Len() != 0 {
		t.Fatalf("corrupt explicit store: exit %d: %s %s", code, &out, &errs)
	}
	got, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("refusal changed plan: %s, %v", got, err)
	}
}

func TestCmdPlansWeightAcceptUnknownSubcommand(t *testing.T) {
	for _, args := range [][]string{{"weight", "bogus"}, {"weight", "bogus", "W-1"}, {"weight", "bogus", "--help"}} {
		var out, errs bytes.Buffer
		if code := cmdPlans(args, &out, &errs); code == 0 || !strings.Contains(errs.String(), `unknown plans weight subcommand "bogus" (want accept)`) {
			t.Fatalf("%v: exit %d: %s %s", args, code, &out, &errs)
		}
	}
}

func TestCmdPlansWeightAcceptFixedPoint(t *testing.T) {
	preflightTestEnv(t)
	root := t.TempDir()
	t.Chdir(root)
	base := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	gitAt(t, root, base, "init", "-q")
	var proofs, sessions strings.Builder
	for i := range 10 {
		at := base.Add(time.Duration(i) * 24 * time.Hour)
		if err := os.WriteFile(filepath.Join(root, "build.go"), []byte(fmt.Sprintf("package example\nfunc Build() int { return %d }\n", i)), 0600); err != nil {
			t.Fatal(err)
		}
		gitAt(t, root, at, "add", "build.go")
		gitAt(t, root, at, "commit", "-q", "-m", "example change")
		commit := repoHeadNoFork(root)
		weight, turns := 1, 10
		if i >= 5 {
			weight, turns = 3, 100
		}
		for j, state := range []string{"open", "claimed-proven"} {
			fmt.Fprintf(&proofs, `{"schema_version":3,"at":%q,"repo_root":%q,"service":"example","plan":"history.md","commit":%q,"origin":"cli","digest":%q,"rows":[{"id":%q,"state":%q,"weight":%d}]}`+"\n",
				at.Add(time.Duration(j)*time.Minute).Format(time.RFC3339), root, commit, fmt.Sprintf("d-%d-%d", i, j), fmt.Sprintf("H-%d", i), state, weight)
		}
		fmt.Fprintf(&sessions, `{"schema_version":2,"session_id":%q,"repo":%q,"at":%q,"duration_ms":120000,"turn_count":%d,"tokens_in":%d,"tokens_out":0}`+"\n",
			fmt.Sprintf("s-%d", i), root, at.Add(time.Minute).Format(time.RFC3339), turns, turns*100)
	}
	for path, data := range map[string]string{os.Getenv("FATHOM_VERIFY_LEDGER"): proofs.String(), os.Getenv("FATHOM_SESSION_LEDGER"): sessions.String()} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	storePath := filepath.Join(t.TempDir(), "index.db")
	st, err := store.NewSQLiteStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.PutObservations(context.Background(), []core.Observation{{ServiceID: "example", SourceRef: "build.go:2", SourceKind: core.KindGoFunc, Symbol: "Build", Line: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	file := writeTempPlan(t, "- [ ] Build {id: W-1, symbol: Build, reachable_from: TestBuild, needs: \"W-0,W-2\"}\n- [ ] First {id: W-0}\n- [ ] Second {id: W-2}\n")
	in, err := rowWeightPreflightInput(context.Background(), mustWeightTask(t, file), file, "example", storePath, base.Add(30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if in.Index == nil || in.Index.Len() != 10 {
		t.Fatalf("history was not indexed: %+v", in.Index)
	}
	args := []string{"weight", "accept", "W-1", "--plan", file, "--store", storePath, "--service", "example"}
	var first []byte
	for i := range 2 {
		var out, errs bytes.Buffer
		if code := cmdPlans(args, &out, &errs); code != 0 {
			t.Fatalf("accept %d: exit %d: %s", i, code, &errs)
		}
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if !strings.Contains(out.String(), "absent -> 8") {
				t.Fatalf("estimate did not use weight-independent neighbours: %s", &out)
			}
			first = got
		} else if !bytes.Equal(first, got) {
			t.Fatalf("acceptance changed estimate twice: %s -> %s", first, got)
		}
	}
}

func mustWeightTask(t *testing.T, file string) verify.Task {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	task, err := readWeightRow(f, 1)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestPlansContextRowWeightTask(t *testing.T) {
	for _, ann := range []string{
		"{id: W-1, symbol: Build, weight: 13, needs: W-0}",
		"{id: W-1, symbol: Build,\n  weight: 13,\n  needs: W-0\n  }",
	} {
		raw := "# Plan\n- [ ] Other {id: W-0, weight: 99}\n- [ ] Row " + ann + "\n- [ ] Unrelated {id: W-2, weight: 99}\n"
		task, err := readWeightRow(strings.NewReader(raw), 3)
		if err != nil || task.ID != "W-1" || task.Weight != 13 || len(task.Needs) != 1 || task.Needs[0] != "W-0" {
			t.Fatalf("selected row: %+v, %v", task, err)
		}
	}
	task, err := readWeightRow(strings.NewReader("- [ ] Row {id: W-1}\n"), 1)
	if err != nil || task.Weight != 0 {
		t.Fatalf("absent authored weight: %+v, %v", task, err)
	}
}
