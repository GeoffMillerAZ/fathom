package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestStopDeliveryPersistentBlockAndAdvisory(t *testing.T) {
	for _, block := range []bool{false, true} {
		t.Run(map[bool]string{false: "advisory", true: "block"}[block], func(t *testing.T) {
			root, svc := stopDeliveryRepo(t, "symbol: Ghost, package: pkg/tracked")
			for i := 0; i < 3; i++ {
				got := stopDeliveryMode(t, root, svc, block)
				if block && (!strings.Contains(got, `"decision":"block"`) || !strings.Contains(got, "Ghost not found in the index")) {
					t.Fatalf("Stop %d lost persistent blocker: %s", i, got)
				}
				if !block && ((i == 0 && got == "") || (i > 0 && got != "")) {
					t.Fatalf("Stop %d advisory once-per-change: %s", i, got)
				}
			}
		})
	}
}

func TestStopDeliveryQuietStateDoesNotWrite(t *testing.T) {
	root, svc := stopDeliveryRepo(t, "symbol: Ghost, package: pkg/tracked")
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	previous := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = previous })
	planPath := "docs/plans/p.md"
	open := "- [ ] One {id: T-1, symbol: Ghost, package: pkg/tracked}\n"
	writeEditedFile(t, root, planPath, open)
	if got := stopDelivery(t, root, svc); got != "" {
		t.Fatalf("empty Stop: %s", got)
	}
	path := cbDedupePath("session-a")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("empty first Stop created session state: %v", err)
	}
	// An unrelated channel's entry is not a prior stop-verify report.
	cbSeen("session-a", "cwd-changed", "repo", "primer", now, false)
	assertUnwritten := func() {
		t.Helper()
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		marker := now.Add(-time.Hour)
		if err := os.Chtimes(path, marker, marker); err != nil {
			t.Fatal(err)
		}
		if got := stopDeliveryMode(t, root, svc, false); got != "" {
			t.Fatalf("quiet Stop: %s", got)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) || !info.ModTime().Equal(marker) {
			t.Fatal("quiet Stop rewrote state")
		}
	}
	assertUnwritten()
	writeEditedFile(t, root, planPath, strings.Replace(open, "[ ]", "[x]", 1))
	stopDeliveryMode(t, root, svc, false)
	assertUnwritten() // unchanged advisory suppression
	writeEditedFile(t, root, planPath, open)
	stopDeliveryMode(t, root, svc, false) // clear the old set once
	assertUnwritten()
}

func TestUnindexedClaimsNarrowFreshness(t *testing.T) {
	for _, tc := range []struct {
		name, criterion, detail string
		unknown                 bool
	}{
		{"symbol", "symbol_present", `"Ghost" not found in the index`, true},
		{"package", "package_present", `"pkg/tracked" not found in the index`, true},
		{"test", "test_present", `"TestGhost" not found in the index`, true},
		{"reach", "test_reaches", "no Test* statically reaches it", false},
		{"tier", "tier", "doc-claimed (< required verified)", false},
		{"ambiguous", "symbol_present", "ambiguous: 2 definitions; add package:", false},
		{"wiring", "edge_wired", "main does not reach Ghost", false},
		{"self-skip", "skip_test", "skip_test/skip was added in the same edit", false},
		{"lexical", "lexical", "pattern not found in file", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := stopDeliveryRepo(t, "symbol: Ghost, package: pkg/tracked")
			writeEditedFile(t, root, "pkg/tracked/code.go", "package changed\n")
			plan := verify.ParseMarkdownPlan("svc", []byte("- [x] One {id: T-1, symbol: Ghost, package: pkg/tracked}\n"))
			rep := verify.Report{Summary: verify.Summary{Fail: 1}, Tasks: []verify.TaskVerdict{{TaskID: "T-1", Title: "One", Claimed: true, Status: verify.Fail, Criteria: []verify.Criterion{{Name: tc.criterion, Status: verify.Fail, Detail: tc.detail}}}}}
			lines := stopUnindexedClaims(root, "docs/plans/p.md", []string{"T-1"}, plan, &rep)
			want := tc.criterion + ": " + tc.detail + "; index may lag: 1 uncommitted files under pkg/tracked"
			state := "claimed-unproven"
			if tc.unknown {
				state = "unknown"
				if rep.Tasks[0].Status != verify.Unknown || rep.Summary.Fail != 0 || rep.Summary.Unknown != 1 || len(lines) != 1 || !strings.Contains(lines[0], want) {
					t.Fatalf("absent criterion: %+v, %v", rep, lines)
				}
			} else if rep.Tasks[0].Status != verify.Fail || rep.Summary.Fail != 1 || len(lines) != 0 || !strings.Contains(strings.Join(claimRefusals(root, "p.md", []string{"T-1"}, rep), "\n"), want) {
				t.Fatalf("proven failure lost: %+v, %v", rep, lines)
			}
			rows := ledgerRowVerdicts(rep)
			if len(rows) != 1 || rows[0].State != state || rows[0].Criterion != want {
				t.Fatalf("criterion/freshness lost from ledger: %+v", rows)
			}
		})
	}
}

func TestUnindexedClaimsMixedFailureStaysFail(t *testing.T) {
	root, _ := stopDeliveryRepo(t, "symbol: Ghost, package: pkg/tracked")
	writeEditedFile(t, root, "pkg/tracked/code.go", "package changed\n")
	plan := verify.ParseMarkdownPlan("svc", []byte("- [x] One {id: T-1, symbol: Ghost, package: pkg/tracked}\n"))
	rep := verify.Report{Summary: verify.Summary{Fail: 1}, Tasks: []verify.TaskVerdict{{TaskID: "T-1", Status: verify.Fail, Criteria: []verify.Criterion{
		{Name: "symbol_present", Status: verify.Fail, Detail: "Ghost not found in the index"},
		{Name: "skip_test", Status: verify.Fail, Detail: "skip_test: true with no skip: reason"},
	}}}}
	if lines := stopUnindexedClaims(root, "p.md", []string{"T-1"}, plan, &rep); len(lines) != 0 || rep.Tasks[0].Status != verify.Fail || rep.Summary.Fail != 1 || rep.Tasks[0].Criteria[1].Status != verify.Fail {
		t.Fatalf("mixed failure downgraded: %+v, %v", rep, lines)
	}
}

func TestUnindexedClaimsInvalidAndGitError(t *testing.T) {
	for _, path := range []string{"../outside", "/outside", "pkg/new"} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir() // not a git repo: an execution failure is operational
			plan := verify.Plan{Tasks: []verify.Task{{ID: "T-1", Expect: verify.Expect{Package: path}}}}
			rep := verify.Report{Summary: verify.Summary{Fail: 1}, Tasks: []verify.TaskVerdict{{TaskID: "T-1", Status: verify.Fail, Criteria: []verify.Criterion{{Name: "symbol_present", Status: verify.Fail, Detail: "Ghost not found in the index"}}}}}
			lines := stopUnindexedClaims(root, "p.md", []string{"T-1"}, plan, &rep)
			if path == "pkg/new" {
				if rep.Tasks[0].Status != verify.Unknown || len(lines) != 1 || !strings.Contains(lines[0], "Ghost not found in the index") || !strings.Contains(lines[0], "working tree could not be checked") {
					t.Fatalf("git error must be unknown with original criterion: %+v, %v", rep, lines)
				}
			} else if rep.Tasks[0].Status != verify.Fail || len(lines) != 0 || rep.Tasks[0].Criteria[0].Detail != "Ghost not found in the index" {
				t.Fatalf("invalid path must stay FAIL: %+v, %v", rep, lines)
			}
		})
	}
}

func TestClaimMomentNarrowFreshness(t *testing.T) {
	for _, missing := range []bool{false, true} {
		name, detail := "test_reaches", "no Test* statically reaches it"
		if missing {
			name, detail = "symbol_present", "Ghost not found in the index"
		}
		t.Run(name, func(t *testing.T) {
			root, svc := stopDeliveryRepo(t, "symbol: Ghost, package: pkg/tracked")
			writeEditedFile(t, root, "pkg/tracked/code.go", "package changed\n")
			hc := newStopHookClient()
			hc.Transport = stopDeliveryTransport{service: svc, criteria: []verify.Criterion{{Name: name, Status: verify.Fail, Detail: detail}}}
			got := claimMomentVerifyWith(hc, "http://127.0.0.1:1", root, filepath.Join(root, "docs/plans/p.md"), "session-a")
			want := name + ": " + detail + "; index may lag: 1 uncommitted files under pkg/tracked"
			if !strings.Contains(got, want) || (missing && (!strings.Contains(got, "unknown (") || strings.Contains(got, "⛔"))) || (!missing && !strings.Contains(got, "⛔")) {
				t.Fatalf("claim moment verdict: %q", got)
			}
			rows := readVerifyLedger()
			if len(rows) != 1 || len(rows[0].Rows) != 1 || rows[0].Rows[0].Criterion != want || (missing && (rows[0].Failing != 0 || rows[0].Rows[0].State != "unknown")) || (!missing && rows[0].Failing != 1) {
				t.Fatalf("claim moment ledger: %+v", rows)
			}
		})
	}
}

func TestUnindexedClaimsDeadlinePreservesCriterion(t *testing.T) {
	root, _ := stopDeliveryRepo(t, "symbol: Ghost, package: pkg/tracked")
	restore := activateHookGuard(&hookguard.Guard{Deadline: hookguard.HookDeadline(time.Unix(1, 0), 5)})
	defer restore()
	plan := verify.ParseMarkdownPlan("svc", []byte("- [x] One {id: T-1, symbol: Ghost, package: pkg/tracked}\n"))
	rep := verify.Report{Summary: verify.Summary{Fail: 1}, Tasks: []verify.TaskVerdict{{TaskID: "T-1", Claimed: true, Status: verify.Fail, Criteria: []verify.Criterion{{Name: "symbol_present", Status: verify.Fail, Detail: "Ghost not found in the index"}}}}}
	lines := stopUnindexedClaims(root, "p.md", []string{"T-1"}, plan, &rep)
	want := "symbol_present: Ghost not found in the index; working tree could not be checked under pkg/tracked"
	if rep.Tasks[0].Status != verify.Unknown || len(lines) != 1 || !strings.Contains(lines[0], want) || ledgerRowVerdicts(rep)[0].Criterion != want {
		t.Fatalf("deadline must be advisory with original criterion: %+v, %v", rep, lines)
	}
}
