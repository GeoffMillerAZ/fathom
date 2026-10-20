package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// TestReadoutHookClassMatchesTheReplayProjection (RO-28): past leading
// `&&`-joined quiet segments, the readout hook classifies a call by its last
// segment, so on a corpus of such calls its class is the replay's projected
// class; a `[readouts] quiet_prefixes` head counts, and a call outside the
// shape stays unmatched as before.
func TestReadoutHookClassMatchesTheReplayProjection(t *testing.T) {
	readoutHookEnv(t, "[readouts]\nmode = \"observe\"\nquiet_prefixes = [\"nvm\"]\n")
	cwd := t.TempDir()
	rs, err := readout.LoadRules(fathomhome.FathomHome())
	if err != nil {
		t.Fatal(err)
	}
	var goTest, grep, diff strings.Builder
	for i := range 30 {
		fmt.Fprintf(&goTest, "ok  \texample.com/shop/internal/p%02d\t0.%03ds\n", i, 100+i)
		fmt.Fprintf(&grep, "internal/shop/file_%02d.go:%d:\treturn tokenValue\n", i%3, 10+i)
	}
	goTest.WriteString("--- FAIL: TestCheckoutTotals (0.00s)\nFAIL\nFAIL\texample.com/shop/internal/checkout\t0.233s\n")
	diff.WriteString("diff --git a/a.go b/a.go\nindex 1..2 100644\n--- a/a.go\n+++ b/a.go\n@@ -1,3 +1,3 @@\n-old\n+new\n")
	listing := readoutListing("")

	corpus := []struct{ command, stdout string }{
		{"cd " + cwd + " && go test ./...", goTest.String()},
		{"cd " + cwd + " && go test ./... 2>&1 | tail -n 80", goTest.String()},
		{"export X=1 && sleep 2 && grep -rn tokenValue .", grep.String()},
		{"tccutil reset All com.example.term && grep -rn tokenValue internal", grep.String()},
		{"mkdir -p out && ls -1", listing},
		{"X=1 && set -e && git diff", diff.String()},
		{"cd " + cwd + " && git log --oneline -5", "c0ffee one\nbeef02 two\n"},
		{"nvm use 20 && ls -1", listing},
	}
	for i, c := range corpus {
		payload := strings.Replace(readoutPostToolEnvelope(t, cwd, c.command, c.stdout), "toolu_1", fmt.Sprintf("toolu_q%02d", i), 1)
		if code, out, _, _ := runReadoutHook(t, []string{"--event", "post-tool"}, payload); code != 0 || out != "" {
			t.Fatalf("%s: exit %d, stdout %q", c.command, code, out)
		}
		rows := readoutRows(t)
		hook := rows[len(rows)-1].Meta["rule"]
		parsed := shellcmd.ParseCommand(c.command)
		projected := readout.Render(rs, readout.Input{Command: replayProjected(&parsed, []string{"nvm"}), Stdout: c.stdout, Mode: readout.ModeObserve, Quiet: []string{"nvm"}}).Rule
		if hook == "" || hook != projected {
			t.Errorf("%s: hook class %q, replay projected %q; want the same rule", c.command, hook, projected)
		}
	}

	for _, command := range []string{"make all && ls -1", "cd " + cwd + "; ls -1", "cd " + cwd + " || ls -1", "nohup mkdir -p out && ls -1"} {
		if code, _, _, _ := runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, cwd, command, listing)); code != 0 {
			t.Fatalf("%s: exit %d", command, code)
		}
		rows := readoutRows(t)
		if m := rows[len(rows)-1].Meta; m["rule"] != "" || m["reason"] != readout.ReasonNoRule {
			t.Errorf("%s: rule %q reason %q, want no rule (not a quiet prefix)", command, m["rule"], m["reason"])
		}
	}
}
