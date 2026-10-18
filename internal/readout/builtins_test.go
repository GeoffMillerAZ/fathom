package readout

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// builtinSet loads the embedded built-ins alone and fails on any refusal:
// every built-in's fixtures run here, in the gate.
func builtinSet(t *testing.T) *RuleSet {
	t.Helper()
	rs, err := LoadRules("")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Refused) > 0 {
		t.Fatalf("built-ins refused: %+v", rs.Refused)
	}
	return rs
}

// builtin is the served built-in with id, which must ship in observe.
func builtin(t *testing.T, rs *RuleSet, id string) *Rule {
	t.Helper()
	for _, r := range rs.Rules {
		if r.ID == id {
			if r.Mode != ModeObserve {
				t.Fatalf("%s ships in %q, want observe", id, r.Mode)
			}
			if !strings.HasPrefix(r.Source, "builtin:") {
				t.Fatalf("%s comes from %s, want an embedded built-in", id, r.Source)
			}
			return r
		}
	}
	t.Fatalf("built-in %s is not served", id)
	return nil
}

func lastLine(s string) string {
	ls, _ := splitLines(s)
	if len(ls) == 0 {
		return ""
	}
	return ls[len(ls)-1]
}

// stayRaw asserts each command's output is not shown as a readout.
func stayRaw(t *testing.T, rs *RuleSet, stdout string, cmds ...string) {
	t.Helper()
	for _, c := range cmds {
		got := Render(rs, Input{Command: lexed(c), Stdout: stdout})
		if !got.Raw || got.Stdout != stdout {
			t.Errorf("%s: raw=%v rule=%q reason=%q, want the raw output", c, got.Raw, got.Rule, got.Reason)
		}
	}
}

// ─── ls-long ────────────────────────────────────────────────────────────

func lsListing(n int) (string, []string) {
	var b strings.Builder
	var names []string
	b.WriteString("total 1184\n")
	b.WriteString("drwxr-xr-x   64 dev  staff    2048 Oct 12 10:00 .\n")
	b.WriteString("drwxr-xr-x   12 dev  staff     384 Oct 12 09:58 ..\n")
	names = append(names, "./", "../")
	for i := 0; i < n; i++ {
		switch i % 5 {
		case 0:
			name := fmt.Sprintf("pkg%02d", i)
			fmt.Fprintf(&b, "drwxr-xr-x    7 dev  staff     224 Oct  3 18:20 %s\n", name)
			names = append(names, name+"/")
		case 1:
			name := fmt.Sprintf("current%02d", i)
			fmt.Fprintf(&b, "lrwxr-xr-x    1 dev  staff      11 Sep 30  2025 %s -> releases/v%d\n", name, i)
			names = append(names, name+"@ -> releases/v"+fmt.Sprint(i))
		case 2:
			name := fmt.Sprintf("errors_%02d.go", i)
			fmt.Fprintf(&b, "-rw-r--r--@   1 dev  staff   12040 Oct 12 10:00 %s\n", name)
			names = append(names, name)
		default:
			name := fmt.Sprintf("file %02d.txt", i)
			fmt.Fprintf(&b, "-rw-r--r--    1 dev  staff  %6d Oct 11 07:%02d %s\n", 100*i, i%60, name)
			names = append(names, name)
		}
	}
	return b.String(), names
}

func TestBuiltinLsLongKeepsEveryEntry(t *testing.T) {
	rs := builtinSet(t)
	r := builtin(t, rs, "ls-long")
	// Computed, never declared: the fields stage drops columns and the total
	// line's drop makes the rule lines-lossy, every entry still kept.
	if r.loss != LossLines || r.Recovery != RecoveryRepeat {
		t.Fatalf("ls-long: lossiness %v recovery %q, want lines and repeat", r.loss, r.Recovery)
	}
	listing, names := lsListing(60)
	for _, cmd := range []string{"ls -l", "ls -la", "ls -al internal", "ls -lah", "ls -l -a", "ls -ltr", "ls -la | head -n 100"} {
		got := Render(rs, Input{Command: lexed(cmd), Stdout: listing})
		if got.Raw || got.Rule != "ls-long@1" || got.Mode != ModeObserve || got.Stream != StreamContent {
			t.Fatalf("%s: raw=%v reason=%q rule=%q mode=%q stream=%q, want ls-long's observe readout", cmd, got.Raw, got.Reason, got.Rule, got.Mode, got.Stream)
		}
		body, _ := splitLines(got.Stdout)
		body = body[:len(body)-1]
		if !reflect.DeepEqual(body, names) {
			t.Fatalf("%s: entries = %q, want every entry, one per line with its type marker, in order: %q", cmd, body, names)
		}
		want := fmt.Sprintf("[readout ls-long@1 observe: %d→%d lines; dropped 1 (plain 1); columns dropped on %d lines; %d entries, 0 dropped; raw: repeat the command]", len(names)+1, len(names), len(names), len(names))
		if got.Receipt != want || lastLine(got.Stdout) != want {
			t.Fatalf("%s: receipt %q, want %q", cmd, got.Receipt, want)
		}
	}
	// Lean serves the same view: repeat is always available for a listing.
	if got := Render(rs, Input{Command: lexed("ls -la"), Stdout: listing, Mode: ModeLean}); got.Raw || !strings.HasSuffix(got.Receipt, "; raw: repeat the command]") {
		t.Fatalf("lean: raw=%v reason=%q receipt=%q", got.Raw, got.Reason, got.Receipt)
	}
	// Counts over a cut listing say what they cover.
	if got := Render(rs, Input{Command: lexed("ls -la"), Stdout: listing, Cap: len(listing)}); !strings.Contains(got.Receipt, fmt.Sprintf("%d entries in the shown %s chars, 0 dropped", len(names), thousands(len(listing)))) {
		t.Fatalf("cut listing: receipt %q does not label its count", got.Receipt)
	}
	// Not a long listing, a machine-bound shape, or below the floor: raw.
	stayRaw(t, rs, listing, "ls", "ls -a", "ls -1", "ls -la > listing.txt", "ls -la | wc -l", "ls -la | sort", "ls -lo", "ls -l --time-style=long-iso", "ls -la; ls -la")
	small, _ := lsListing(3)
	stayRaw(t, rs, small, "ls -la")
}

// ─── grep-group ─────────────────────────────────────────────────────────

// grepOutput is `grep -rn` output: per file, n matches, files in order.
func grepOutput(files []string, n []int) string {
	var b strings.Builder
	for i, f := range files {
		for k := 1; k <= n[i]; k++ {
			switch k % 3 {
			case 0:
				fmt.Fprintf(&b, "%s:%d:\tif err != nil {\n", f, 10*k)
			case 1:
				fmt.Fprintf(&b, "%s:%d:\treturn fmt.Errorf(\"load %%s: %%w\", path, err)\n", f, 10*k+1)
			default:
				fmt.Fprintf(&b, "%s:%d:// errors are wrapped once, at the boundary\n", f, 10*k+2)
			}
		}
	}
	return b.String()
}

// ungroupView undoes grep-group's grouping on a view's body.
func ungroupView(t *testing.T, view string) string {
	t.Helper()
	s := Stage{Kind: StageGroup, Key: grepKey}
	if err := compileStage(&s); err != nil {
		t.Fatal(err)
	}
	ls, _ := splitLines(view)
	return joinLines(s.ungroup(ls[:len(ls)-1]), true)
}

const grepKey = `^[^:\s]*[/.][^:\s]*:`

func TestBuiltinGrepGroupIsLosslessUnderTheCap(t *testing.T) {
	rs := builtinSet(t)
	for _, id := range []string{"grep-group", "git-grep-group"} {
		if r := builtin(t, rs, id); r.loss != LossLines || r.Recovery != RecoveryRepeat {
			t.Fatalf("%s: lossiness %v recovery %q, want lines (the cap) and repeat", id, r.loss, r.Recovery)
		}
	}
	files := []string{"internal/config/load.go", "internal/store/sqlite.go", "cmd/shop/main.go"}

	// Under the cap: grouped by file, the path factored out, nothing lost.
	under := grepOutput(files, []int{90, 70, 40})
	for _, cmd := range []string{"grep -rn err .", "rg -n err", "grep -rn err . | head -n 300", "git grep -n err"} {
		got := Render(rs, Input{Command: lexed(cmd), Stdout: under})
		if got.Raw || got.Lossiness != LossLines || got.Stream != StreamContent {
			t.Fatalf("%s: raw=%v reason=%q, want a grouped readout", cmd, got.Raw, got.Reason)
		}
		if got := ungroupView(t, got.Stdout); got != under {
			t.Fatalf("%s: the grouped view does not ungroup to the raw output", cmd)
		}
		if !strings.HasPrefix(got.Stdout, "internal/config/load.go:\n  11:\treturn fmt.Errorf(") {
			t.Fatalf("%s: view starts %q, want the path factored to a header", cmd, got.Stdout[:80])
		}
		want := "nothing dropped; 200 matches in 3 files, 0 dropped]"
		if !strings.HasSuffix(got.Receipt, want) || strings.Contains(got.Receipt, "raw:") || len(got.Dropped) != 0 {
			t.Fatalf("%s: receipt %q, want a lossless receipt ending %q", cmd, got.Receipt, want)
		}
	}
	if got := Render(rs, Input{Command: lexed("git grep -n err"), Stdout: under}); got.Rule != "git-grep-group@1" {
		t.Fatalf("git grep: rule %q", got.Rule)
	}

	// Above 200 matches: a per-file cap, every file named with its count.
	over := grepOutput(files, []int{150, 80, 20})
	got := Render(rs, Input{Command: lexed("grep -rn err ."), Stdout: over})
	if got.Raw {
		t.Fatalf("over the cap: raw (%s)", got.Reason)
	}
	for _, want := range []string{
		"internal/config/load.go:\n  11:\treturn fmt.Errorf(",
		"… +130 more lines of internal/config/load.go (150 in all)\n",
		"… +60 more lines of internal/store/sqlite.go (80 in all)\n",
		"cmd/shop/main.go:\n",
	} {
		if !strings.Contains(got.Stdout, want) {
			t.Fatalf("over the cap: view lacks %q", want)
		}
	}
	if strings.Contains(got.Stdout, "more lines of cmd/shop/main.go") {
		t.Fatal("a file under the per-file cap was capped")
	}
	wantReceipt := "[readout grep-group@1 observe: 250→65 lines; dropped 190 (error 63, frame 127); 250 matches in 3 files, 190 dropped; raw: repeat the command]"
	if got.Receipt != wantReceipt {
		t.Fatalf("over the cap: receipt %q, want %q", got.Receipt, wantReceipt)
	}

	// The harness cut the output and saved none: counts are labelled as over
	// the shown text, never presented as totals.
	got = Render(rs, Input{Command: lexed("grep -rn err ."), Stdout: over, Cap: len(over)})
	label := "in the shown " + thousands(len(over)) + " chars"
	if !strings.Contains(got.Stdout, "… +130 more lines of internal/config/load.go (150 "+label+")\n") ||
		!strings.Contains(got.Receipt, "250 matches in 3 files "+label+", 190 dropped") {
		t.Fatalf("cut output: counts not labelled: %q", got.Receipt)
	}
	// The harness persisted the output: per-file and total counts come from
	// the persisted text the caller read.
	persisted := grepOutput(files, []int{400, 80, 20}) + grepOutput([]string{"internal/api/routes.go"}, []int{9})
	got = Render(rs, Input{Command: lexed("grep -rn err ."), Stdout: over, Persisted: "/state/tool-results/b1.txt", PersistedText: persisted})
	if !strings.Contains(got.Stdout, "… +130 more lines of internal/config/load.go (400 in the persisted output)\n") ||
		!strings.Contains(got.Receipt, "509 matches in 4 files in the persisted output; 190 of the shown dropped") ||
		strings.Contains(got.Receipt, "in the shown") {
		t.Fatalf("persisted output: counts not from the persisted text: %q", got.Receipt)
	}
	// Persisted but not read: labelled, never a total.
	got = Render(rs, Input{Command: lexed("grep -rn err ."), Stdout: over, Persisted: "/state/tool-results/b1.txt"})
	if !strings.Contains(got.Receipt, "250 matches in 3 files in the shown 30,000 chars") {
		t.Fatalf("persisted, unread: receipt %q", got.Receipt)
	}

	// Machine-bound shapes stay raw.
	stayRaw(t, rs, under,
		"grep -c err -r .", "grep -rl err .", "grep -rL err .", "grep -ro err .", "grep -rlZ err .", "rg -l err", "rg -c err",
		"git grep -l err", "git grep -c err", "grep -rn err . | wc -l", "grep -rn err . | sort", "grep -rn err . > hits.txt")
}

// ─── git-log, git-show, git-diff ────────────────────────────────────────

func gitLog(n int, signed bool) (string, []string) {
	var b strings.Builder
	var shas []string
	for i := 0; i < n; i++ {
		sha := fmt.Sprintf("%040x", 0xa1b2c3d4e5+i*7919)
		shas = append(shas, sha)
		fmt.Fprintf(&b, "commit %s\n", sha)
		if signed {
			b.WriteString("gpg: Signature made Mon Oct 12 10:00:00 2026 MST\n")
			b.WriteString("gpg:                using EDDSA key 0123456789ABCDEF0123456789ABCDEF01234567\n")
			b.WriteString("gpg: Good signature from \"Dev Example <dev@example.com>\" [ultimate]\n")
		}
		fmt.Fprintf(&b, "Author: Dev Example <dev@example.com>\nDate:   Mon Oct 12 10:%02d:00 2026 -0700\n\n", i%60)
		fmt.Fprintf(&b, "    fix(cart): round totals once, at checkout (%d)\n\n", i)
		b.WriteString("    The error path returned before the discount applied.\n\n")
		b.WriteString("    Verified: go test ./internal/cart/...\n")
		b.WriteString("    Signed-off-by: Dev Example <dev@example.com>\n")
		b.WriteString("    Reviewed-by: Other Example <other@example.com>\n")
		if i < n-1 {
			b.WriteString("\n")
		}
	}
	return b.String(), shas
}

// gitDiff is `git diff` output: one small file, then one with a hunk of
// body lines, then a deletion.
func gitDiff(body int) string {
	var b strings.Builder
	b.WriteString("diff --git a/internal/cart/errors.go b/internal/cart/errors.go\n")
	b.WriteString("index 3f2a1b0..9c8d7e6 100644\n--- a/internal/cart/errors.go\n+++ b/internal/cart/errors.go\n")
	b.WriteString("@@ -12,6 +12,7 @@ var (\n \tErrEmpty = errors.New(\"cart: empty\")\n+\tErrClosed = errors.New(\"cart: closed\")\n \tErrLimit = errors.New(\"cart: over limit\")\n )\n")
	b.WriteString("diff --git a/internal/cart/total.go b/internal/cart/total.go\n")
	b.WriteString("index 1111111..2222222 100644\n--- a/internal/cart/total.go\n+++ b/internal/cart/total.go\n")
	fmt.Fprintf(&b, "@@ -40,3 +40,%d @@ func Total(items []Item) (int, error) {\n", body+3)
	b.WriteString(" \tsum := 0\n")
	for i := 0; i < body; i++ {
		if i%4 == 0 {
			fmt.Fprintf(&b, "+\tif err := check(items[%d]); err != nil {\n", i)
		} else {
			fmt.Fprintf(&b, "+\tsum += items[%d].Price * items[%d].Qty\n", i, i)
		}
	}
	b.WriteString(" \treturn sum, nil\n }\n")
	b.WriteString("@@ -90,4 +")
	fmt.Fprintf(&b, "%d,3 @@ func round(v int) int {\n", 90+body)
	b.WriteString(" \tif v < 0 {\n-\t\treturn 0\n \t}\n \treturn v\n")
	b.WriteString("diff --git a/internal/cart/legacy.go b/internal/cart/legacy.go\n")
	b.WriteString("deleted file mode 100644\nindex 4444444..0000000\n--- a/internal/cart/legacy.go\n+++ /dev/null\n")
	b.WriteString("@@ -1,3 +0,0 @@\n-package cart\n-\n-// legacy totals\n")
	return b.String()
}

func TestBuiltinGitRulesKeepEveryCommitAndHunkHeader(t *testing.T) {
	rs := builtinSet(t)
	for _, id := range []string{"git-log", "git-show", "git-diff"} {
		if r := builtin(t, rs, id); r.loss != LossLines || r.Recovery != RecoveryRepeat {
			t.Fatalf("%s: lossiness %v recovery %q, want lines and repeat", id, r.loss, r.Recovery)
		}
	}
	stamp := map[string][]string{"head_stamp": {"HEAD a1b2c3d"}}

	// git log: trailer and signature lines dropped, every commit kept and
	// counted, the receipt stamped with HEAD.
	log, shas := gitLog(30, true)
	for _, cmd := range []string{"git log", "git log -n 30", "git log --show-signature | head -n 400", "git -C sub log"} {
		got := Render(rs, Input{Command: lexed(cmd), Stdout: log, Joins: stamp})
		if got.Raw || got.Rule != "git-log@1" || got.Stream != StreamContent {
			t.Fatalf("%s: raw=%v reason=%q rule=%q", cmd, got.Raw, got.Reason, got.Rule)
		}
		for _, sha := range shas {
			if !strings.Contains(got.Stdout, "commit "+sha+"\n") {
				t.Fatalf("%s: commit %s missing from the view", cmd, sha)
			}
		}
		for _, gone := range []string{"Signed-off-by:", "Reviewed-by:", "gpg:"} {
			if strings.Contains(got.Stdout, gone) {
				t.Fatalf("%s: %q kept", cmd, gone)
			}
		}
		if !strings.Contains(got.Stdout, "    Verified: go test ./internal/cart/...\n") {
			t.Fatalf("%s: a trailer outside the closed list was dropped", cmd)
		}
		want := "[readout git-log@1 observe: 449→299 lines; dropped 150 (plain 150); 30 commits, 0 dropped; raw: repeat the command at HEAD a1b2c3d]"
		if got.Receipt != want {
			t.Fatalf("%s: receipt %q, want %q", cmd, got.Receipt, want)
		}
	}
	// No HEAD stamp: observe says so; lean has no reproducible recovery.
	if got := Render(rs, Input{Command: lexed("git log"), Stdout: log}); !strings.Contains(got.Receipt, "join head_stamp: not computed; raw: repeat the command]") {
		t.Fatalf("unstamped: receipt %q", got.Receipt)
	}
	if got := Render(rs, Input{Command: lexed("git log"), Stdout: log, Mode: ModeLean}); !got.Raw || got.Reason != ReasonNoRecovery {
		t.Fatalf("unstamped lean: raw=%v reason=%q, want no-recovery raw", got.Raw, got.Reason)
	}
	if got := Render(rs, Input{Command: lexed("git log"), Stdout: log, Mode: ModeLean, Joins: stamp}); got.Raw {
		t.Fatalf("stamped lean: raw (%s)", got.Reason)
	}
	// Cut output: the commit count is labelled, never a total; persisted
	// output counts every commit.
	if got := Render(rs, Input{Command: lexed("git log"), Stdout: log, Joins: stamp, Cap: len(log)}); !strings.Contains(got.Receipt, "30 commits in the shown "+thousands(len(log))+" chars, 0 dropped") {
		t.Fatalf("cut log: receipt %q", got.Receipt)
	}
	all, _ := gitLog(120, false)
	if got := Render(rs, Input{Command: lexed("git log"), Stdout: log, Joins: stamp, Persisted: "/state/tool-results/l.txt", PersistedText: all}); !strings.Contains(got.Receipt, "120 commits in the persisted output; 0 of the shown dropped") {
		t.Fatalf("persisted log: receipt %q", got.Receipt)
	}

	// git diff: grouped by file (the index, --- and +++ lines factored into
	// the diff --git header), every hunk header kept, a body capped only
	// above the floor, with a span recipe.
	small := gitDiff(20)
	got := Render(rs, Input{Command: lexed("git diff"), Stdout: small, Joins: stamp})
	if got.Raw || got.Rule != "git-diff@1" {
		t.Fatalf("git diff: raw=%v reason=%q rule=%q", got.Raw, got.Reason, got.Rule)
	}
	if strings.Contains(got.Stdout, "… ") || !strings.Contains(got.Stdout, "+\tsum += items[19].Price * items[19].Qty\n") {
		t.Fatal("git diff: a body under the floor was capped")
	}
	if strings.Contains(got.Stdout, "\nindex ") || strings.Contains(got.Stdout, "\n--- a/") || strings.Contains(got.Stdout, "\n+++ ") {
		t.Fatal("git diff: per-file header lines kept")
	}
	if !strings.Contains(got.Stdout, "diff --git a/internal/cart/legacy.go b/internal/cart/legacy.go\ndeleted file mode 100644\n@@ -1,3 +0,0 @@\n") {
		t.Fatalf("git diff: a deletion's header lost its mode line:\n%s", got.Stdout)
	}
	big := gitDiff(100)
	got = Render(rs, Input{Command: lexed("git diff HEAD~1"), Stdout: big, Joins: stamp})
	if got.Raw {
		t.Fatalf("big diff: raw (%s)", got.Reason)
	}
	hunks := 0
	for _, l := range strings.Split(big, "\n") {
		if strings.HasPrefix(l, "@@ ") {
			hunks++
			if !strings.Contains(got.Stdout, l+"\n") {
				t.Fatalf("big diff: hunk header %q dropped", l)
			}
		}
	}
	wantNote := "… 73 lines dropped, new side 60-132: sed -n 60,132p internal/cart/total.go (work tree)\n"
	if !strings.Contains(got.Stdout, wantNote) {
		t.Fatalf("big diff: no span recipe %q in\n%s", wantNote, got.Stdout)
	}
	wantReceipt := fmt.Sprintf("[readout git-diff@1 observe: %d→%d lines; dropped 82 (error 2, plain 80); 3 files, 0 dropped; %d hunks, 0 dropped; raw: repeat the command at HEAD a1b2c3d]", 131, 50, hunks)
	if got.Receipt != wantReceipt {
		t.Fatalf("big diff: receipt %q, want %q", got.Receipt, wantReceipt)
	}

	// git show: the same, the recipe naming the shown commit.
	show := "commit 9f8e7d6c5b4a39281706f5e4d3c2b1a098765432\nAuthor: Dev Example <dev@example.com>\nDate:   Mon Oct 12 10:00:00 2026 -0700\n\n    fix(cart): totals\n\n" + big
	got = Render(rs, Input{Command: lexed("git show"), Stdout: show, Joins: stamp})
	if got.Raw || got.Rule != "git-show@1" {
		t.Fatalf("git show: raw=%v reason=%q rule=%q", got.Raw, got.Reason, got.Rule)
	}
	if !strings.Contains(got.Stdout, "… 73 lines dropped: git show 9f8e7d6c5b4a39281706f5e4d3c2b1a098765432:internal/cart/total.go | sed -n 60,132p\n") ||
		!strings.HasSuffix(got.Receipt, fmt.Sprintf("3 files, 0 dropped; %d hunks, 0 dropped; raw: repeat the command at HEAD a1b2c3d]", hunks)) {
		t.Fatalf("git show: view\n%s", got.Stdout)
	}

	// Machine-bound and stat shapes stay raw.
	stayRaw(t, rs, log, "git log -z", "git log > log.txt", "git log | grep fix", "git log --stat | head -n 50", "git log -p")
	stayRaw(t, rs, big, "git diff --name-only", "git diff --stat | head -n 20", "git diff --porcelain", "git diff -z", "git diff > d.patch", "git diff | wc -l", "git show --stat", "git show --name-only")
}

// ─── go-test ────────────────────────────────────────────────────────────

func goTestRun(pass int, failing bool) string {
	var b strings.Builder
	for i := 0; i < pass; i++ {
		switch i % 4 {
		case 0:
			fmt.Fprintf(&b, "ok  \texample.com/shop/internal/p%02d\t0.%03ds\n", i, 100+i)
		case 1:
			fmt.Fprintf(&b, "ok  \texample.com/shop/internal/p%02d\t(cached)\n", i)
		case 2:
			fmt.Fprintf(&b, "?   \texample.com/shop/cmd/c%02d\t[no test files]\n", i)
		default:
			fmt.Fprintf(&b, "ok  \texample.com/shop/internal/p%02d\t1.%03ds\tcoverage: 71.2%% of statements\n", i, i)
		}
		if failing && i == pass/2 {
			b.WriteString(goTestFailure)
		}
	}
	return b.String()
}

const goTestFailure = "--- FAIL: TestCheckoutTotals (0.00s)\n" +
	"    checkout_test.go:41: total = 1180, want 1200\n" +
	"    --- FAIL: TestCheckoutTotals/discount (0.00s)\n" +
	"        checkout_test.go:58: discount applied twice\n" +
	"FAIL\n" +
	"FAIL\texample.com/shop/internal/checkout\t0.233s\n"

func TestBuiltinGoTestExtractsFailuresAndKeepsThemVerbatim(t *testing.T) {
	rs := builtinSet(t)
	if r := builtin(t, rs, "go-test"); r.loss != LossWhole || r.Recovery != RecoverySaved || !r.Match.DeclaredTest {
		t.Fatalf("go-test: lossiness %v recovery %q declared %v, want whole, saved and the declared test commands", r.loss, r.Recovery, r.Match.DeclaredTest)
	}
	run := goTestRun(40, true) + "FAIL\n"
	for _, tc := range []struct {
		cmd      string
		declared bool
	}{{"go test ./...", false}, {"go test -race ./internal/...", false}, {"go test ./... 2>&1 | tail -n 80", false}, {"task test", true}, {"make check", true}} {
		got := Render(rs, Input{Command: lexed(tc.cmd), Stdout: run, DeclaredTest: tc.declared})
		if got.Raw || got.Rule != "go-test@1" || got.Mode != ModeObserve || got.Stream != StreamDiagnostic {
			t.Fatalf("%s: raw=%v reason=%q rule=%q", tc.cmd, got.Raw, got.Reason, got.Rule)
		}
		// Every failing line verbatim, in order, as one block.
		if !strings.Contains(got.Stdout, goTestFailure) {
			t.Fatalf("%s: the failure block is not kept verbatim:\n%s", tc.cmd, got.Stdout)
		}
		wantRec := []Record{
			{Kind: "test_failure", Line: 22, Fields: []Field{{"id", "TestCheckoutTotals"}}},
			{Kind: "test_failure", Line: 24, Fields: []Field{{"id", "TestCheckoutTotals/discount"}}},
		}
		if !reflect.DeepEqual(got.Records, wantRec) {
			t.Fatalf("%s: records %+v, want %+v", tc.cmd, got.Records, wantRec)
		}
		want := "ok: 30 packages\nno test files: 10 packages\n" + goTestFailure + "FAIL\n" +
			"[readout go-test@1 observe: 47→9 lines; 40 lines summarized; failing tests: 2, 0 dropped; raw: not saved]\n"
		if got.Stdout != want {
			t.Fatalf("%s: view %q, want %q", tc.cmd, got.Stdout, want)
		}
	}
	// Observe computes the collapse without saving anything; lean serves it
	// only once the hook's saved file exists.
	if got := Render(rs, Input{Command: lexed("go test ./..."), Stdout: run, Mode: ModeLean}); !got.Raw || got.Reason != ReasonNoRecovery {
		t.Fatalf("lean, nothing saved: raw=%v reason=%q, want no-recovery raw", got.Raw, got.Reason)
	}
	if got := Render(rs, Input{Command: lexed("go test ./..."), Stdout: run, Mode: ModeLean, Saved: "/state/readouts/saved/1f2e.txt"}); got.Raw || !strings.HasSuffix(got.Receipt, "; raw saved: /state/readouts/saved/1f2e.txt]") {
		t.Fatalf("lean, saved: raw=%v reason=%q receipt=%q", got.Raw, got.Reason, got.Receipt)
	}
	// A passing package whose path carries an error word is protected in
	// this diagnostic stream: the collapse would drop it, so raw is served.
	tripped := run + "ok  \texample.com/shop/internal/errors\t0.120s\n"
	if got := Render(rs, Input{Command: lexed("go test ./..."), Stdout: tripped}); !got.Raw || got.Reason != ReasonProtected {
		t.Fatalf("errors package: raw=%v reason=%q, want a protected trip", got.Raw, got.Reason)
	}
	// Not a declared test command, or a machine-bound or verbose run: raw.
	stayRaw(t, rs, run, "task test", "go test -json ./...", "go test -v ./...", "go test ./... > out.txt", "go test ./... | grep FAIL", "go vet ./...")
}
