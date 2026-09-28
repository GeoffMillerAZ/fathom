package console

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitFixtureWithCommit makes a scratch git repository with one commit
// whose author (and committer) date is authorDate (any git-parseable
// date), and returns its root.
func gitFixtureWithCommit(t *testing.T, authorDate string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
			"GIT_AUTHOR_DATE="+authorDate, "GIT_COMMITTER_DATE="+authorDate,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "one")
	return dir
}

// PE-15 (docs/design/progress-and-economics.md §5, "The join"): a
// session's proven outcome is the set of rows that moved to
// claimed-proven either in a reading the session's own run produced
// (sessionFor's window rule, reused from PE-10) or in a reading whose
// commit the session authored (the commit's author time, in its working
// tree, falls inside the session's window) — and the share neither reaches
// is named in Unjoined, never dropped (I38).
func TestJoinSessionProofUnjoinedNamed(t *testing.T) {
	lines := []string{
		// R4 is already claimed-proven at the very first reading in the
		// history: it predates the window this history can see, so it can
		// never be joined to a session — named, not silently skipped.
		ledgerLine("2026-09-01T00:00:00Z", "c0", "cli", "R1:open", "R2:open", "R3:open", "R4:claimed-proven"),
		// R1 proven by a reading whose OWN time falls inside session s1's
		// window: the window join (PE-10's rule).
		ledgerLine("2026-09-01T01:00:00Z", "c1", "cli", "R1:claimed-proven", "R2:open", "R3:open", "R4:claimed-proven"),
		// R2 proven by a reading OUTSIDE any session's window, but its
		// commit c2 was authored (per the injected authorAt) inside
		// session s2's window: the commit join.
		ledgerLine("2026-09-05T00:00:00Z", "c2", "stop", "R1:claimed-proven", "R2:claimed-proven", "R3:open", "R4:claimed-proven"),
		// R3 proven by a reading no session's window covers, and whose
		// commit c3 the fake authorAt cannot place at all: unjoined.
		ledgerLine("2026-09-06T00:00:00Z", "c3", "cli", "R1:claimed-proven", "R2:claimed-proven", "R3:claimed-proven", "R4:claimed-proven"),
	}
	h := ParseProofHistory([]byte(strings.Join(lines, "\n")+"\n"), false)

	sessions := []AgentSession{
		{ID: "s1", Harness: "claude-code", Model: "sonnet", Repo: "svc", Start: "2026-09-01T00:30:00Z", End: "2026-09-01T02:00:00Z"},
		{ID: "s2", Harness: "claude-code", Model: "sonnet", Repo: "svc", Start: "2026-09-04T00:00:00Z", End: "2026-09-04T01:00:00Z"},
	}
	fakeAuthorAt := func(ctx context.Context, root, commit string) (string, bool) {
		if commit == "c2" {
			return "2026-09-04T00:30:00Z", true
		}
		return "", false
	}

	res := JoinSessionProof(context.Background(), h, sessions, HistoryFilter{}, fakeAuthorAt)

	if len(res.Outcomes) != 2 {
		t.Fatalf("want 2 session outcomes (s1, s2), got %+v", res.Outcomes)
	}
	byID := map[string]SessionOutcome{}
	for _, so := range res.Outcomes {
		byID[so.SessionID] = so
	}
	s1, s2 := byID["s1"], byID["s2"]
	if len(s1.Rows) != 1 || s1.Rows[0].ID != "R1" || s1.Rows[0].By != JoinedByWindow {
		t.Fatalf("s1 should own R1 by window: %+v", s1)
	}
	if len(s2.Rows) != 1 || s2.Rows[0].ID != "R2" || s2.Rows[0].By != JoinedByCommit {
		t.Fatalf("s2 should own R2 by commit: %+v", s2)
	}

	if len(res.Unjoined) != 2 {
		t.Fatalf("want R3 and R4 named unjoined, got %+v", res.Unjoined)
	}
	unjoinedIDs := map[string]ProvenRow{}
	for _, pr := range res.Unjoined {
		unjoinedIDs[pr.ID] = pr
	}
	if pr, ok := unjoinedIDs["R3"]; !ok || pr.Commit != "c3" {
		t.Fatalf("R3 (commit unresolved) should be unjoined: %+v", unjoinedIDs)
	}
	if pr, ok := unjoinedIDs["R4"]; !ok || pr.Commit != "c0" {
		t.Fatalf("R4 (proven before the history began) should be unjoined: %+v", unjoinedIDs)
	}

	// A nil authorAt defaults to the real (git-backed) resolver rather than
	// panicking — proven by giving it a commit and root that cannot
	// resolve, which must fall through to unjoined, not crash.
	res2 := JoinSessionProof(context.Background(), h, sessions, HistoryFilter{}, nil)
	if len(res2.Unjoined) == 0 {
		t.Fatalf("a nil authorAt must still run (via the real resolver) and name what it cannot join: %+v", res2)
	}
}

// CommitAuthorAt (PE-15's git-backed authorAt) reads a real commit's
// author time from a real working tree — read-only, no index write
// (internal/gitro) — and is honestly false for a root or commit it cannot
// resolve, never a guessed time.
func TestCommitAuthorAtReadsRealCommit(t *testing.T) {
	dir := gitFixtureWithCommit(t, "2026-09-04T00:30:00-07:00")
	at, ok := CommitAuthorAt(context.Background(), dir, "HEAD")
	if !ok {
		t.Fatal("CommitAuthorAt should resolve HEAD in a real repository")
	}
	if !strings.HasPrefix(at, "2026-09-04T00:30:00") {
		t.Fatalf("author time = %q, want the commit's own author date", at)
	}

	if _, ok := CommitAuthorAt(context.Background(), dir, "not-a-commit"); ok {
		t.Fatal("an unresolvable commit must report ok=false, never a guessed time")
	}
	if _, ok := CommitAuthorAt(context.Background(), "", "HEAD"); ok {
		t.Fatal("an empty root must report ok=false")
	}
	if _, ok := CommitAuthorAt(context.Background(), dir, ""); ok {
		t.Fatal("an empty commit must report ok=false")
	}
}
