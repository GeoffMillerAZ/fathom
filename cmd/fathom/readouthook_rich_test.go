package main

import (
	"bytes"
	"encoding/json"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// compileFailureEnvelope is a PostToolUseFailure(Bash) envelope for
// command whose combined output is out, in session s-1.
func compileFailureEnvelope(t *testing.T, cwd, command, out string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"session_id": "s-1", "cwd": cwd, "hook_event_name": "PostToolUseFailure", "tool_name": "Bash", "tool_use_id": "toolu_9",
		"tool_input": map[string]any{"command": command},
		"error":      "Exit code 1\n" + out,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// compileErrorsOutput is a go build failure with three roots: a changed
// signature (money.Round) called from two files, an undefined name (Bar)
// in two others, and an unused import.
const compileErrorsOutput = `# example.com/shop/other
other/x.go:5:2: undefined: Bar
other/x.go:6:2: undefined: Bar
other/y.go:7:9: undefined: Bar
other/y.go:9:9: undefined: Bar
# example.com/shop/cart
cart/cart.go:10:20: not enough arguments in call to money.Round
	have (int)
	want (int, int)
cart/cart.go:14:20: not enough arguments in call to money.Round
	have (int)
	want (int, int)
checkout/checkout.go:8:11: not enough arguments in call to money.Round
	have (int)
	want (int, int)
checkout/checkout.go:3:2: "fmt" imported and not used
`

// TestCompileErrorsGroupByRootAndEditSet: under [readouts] mode = rich, a
// failing go build's errors are extracted as compile_error records and
// grouped by the identifier or file they share; the group whose root sits in
// a file this session edited (money.Round's changed signature) leads even
// though another group holds more errors. The line is injected beside the
// output, never in place of it; observe mode adds nothing.
func TestCompileErrorsGroupByRootAndEditSet(t *testing.T) {
	repo := t.TempDir()
	readoutGitRepo(t, repo, map[string]string{
		"go.mod":         "module example.com/shop\n\ngo 1.22\n",
		"money/money.go": "package money\n\nfunc Round(x int) int { return x }\n",
	})
	if err := os.WriteFile(filepath.Join(repo, "money", "money.go"), []byte("package money\n\nfunc Round(x, places int) int { return x + places }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origSet := editSetDir
	editSetDir = filepath.Join(t.TempDir(), "edit-set")
	t.Cleanup(func() { editSetDir = origSet })
	markFileEdited("s-1", "money/money.go", time.Now())

	payload := compileFailureEnvelope(t, t.TempDir(), "cd "+repo+" && go build ./...", compileErrorsOutput)

	t.Run("rich", func(t *testing.T) {
		readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
		code, out, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, payload)
		if code != 0 || strings.Count(strings.TrimSpace(out), "\n") != 0 || out == "" {
			t.Fatalf("exit %d, stdout %q; want 0 and one JSON line", code, out)
		}
		var doc struct {
			Spec map[string]any `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
		}
		for k := range doc.Spec {
			if k != "hookEventName" && k != "additionalContext" {
				t.Errorf("hook output carries %q: the line is injected, the raw output never replaced", k)
			}
		}
		line, _ := doc.Spec["additionalContext"].(string)
		if strings.Contains(line, "\n") {
			t.Errorf("want one line, got %q", line)
		}
		want := []string{
			"go build: 8 errors, 3 roots:",
			"the signature of money.Round, money/money.go:3, edited this session (3 errors)",
			"undefined: Bar, other/x.go:5 (4 errors)",
			`checkout/checkout.go:3: "fmt" imported and not used`,
		}
		at := -1
		for _, w := range want {
			i := strings.Index(line, w)
			if i < 0 || i < at {
				t.Fatalf("line %q: want %q after position %d", line, w, at)
			}
			at = i
		}
		rows := readoutRows(t)
		if len(rows) != 1 || rows[0].Meta["rich"] != "compile_error" || rows[0].Meta["emitted"] != "true" {
			t.Fatalf("rows = %+v, want one row naming the compile_error line as emitted", rows)
		}
	})

	t.Run("observe", func(t *testing.T) {
		readoutHookEnv(t, "[readouts]\nmode = \"observe\"\n")
		code, out, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, payload)
		if code != 0 || out != "" {
			t.Fatalf("exit %d, stdout %q; observe adds no line", code, out)
		}
	})

	t.Run("not a build", func(t *testing.T) {
		readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
		code, out, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, compileFailureEnvelope(t, t.TempDir(), "cd "+repo+" && grep -rn Bar .", compileErrorsOutput))
		if code != 0 || out != "" {
			t.Fatalf("exit %d, stdout %q; a grep that prints file:line text is no compile", code, out)
		}
	})
}

// TestReadoutRichPastTheCeilingAddsNothing pins the hook's ceiling on an
// injected clock (readoutHookEnv freezes it for the other readout tests): a
// run that has spent its ceiling by the time rich mode builds its lines adds
// nothing beside the output and its row names the ceiling it missed; one
// that has spent just under it still adds the line.
func TestReadoutRichPastTheCeilingAddsNothing(t *testing.T) {
	repo := t.TempDir()
	readoutGitRepo(t, repo, map[string]string{
		"go.mod":         "module example.com/shop\n\ngo 1.22\n",
		"money/money.go": "package money\n\nfunc Round(x int) int { return x }\n",
	})
	if err := os.WriteFile(filepath.Join(repo, "money", "money.go"), []byte("package money\n\nfunc Round(x, places int) int { return x + places }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origSet := editSetDir
	editSetDir = filepath.Join(t.TempDir(), "edit-set")
	t.Cleanup(func() { editSetDir = origSet })
	markFileEdited("s-1", "money/money.go", time.Now())
	payload := compileFailureEnvelope(t, t.TempDir(), "cd "+repo+" && go build ./...", compileErrorsOutput)

	// run spends `spent` on the hook's clock as rich mode starts its lines.
	run := func(t *testing.T, spent time.Duration) (string, map[string]string) {
		t.Helper()
		readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
		t.Setenv("FATHOM_HTTP", "http://127.0.0.1:1")
		now := time.Now()
		readoutHookClock = func() time.Time { return now } // readoutHookEnv restores it
		origTrace := readoutHookTrace
		t.Cleanup(func() { readoutHookTrace = origTrace })
		readoutHookTrace = func(step string) {
			if step == "rich" {
				now = now.Add(spent)
			}
		}
		var out, errb bytes.Buffer
		if code := cmdReadoutHook([]string{"--event", "tool-failure"}, strings.NewReader(payload), &out, &errb); code != 0 {
			t.Fatalf("exit %d", code)
		}
		rows := readoutRows(t)
		if len(rows) != 1 {
			t.Fatalf("rows = %+v, want one", rows)
		}
		return out.String(), rows[0].Meta
	}

	t.Run("just inside the ceiling: the line", func(t *testing.T) {
		out, m := run(t, readoutHookCeiling-time.Millisecond)
		if !strings.Contains(out, "go build: 8 errors") || m["emitted"] != "true" {
			t.Fatalf("stdout %q, meta %v; want the compile line emitted", out, m)
		}
		if strings.Contains(m["rich_missed"], "ceiling") || m["ceiling"] != "" {
			t.Errorf("meta %v names a ceiling the run kept inside", m)
		}
	})

	t.Run("past the ceiling: nothing added", func(t *testing.T) {
		out, m := run(t, readoutHookCeiling+time.Millisecond)
		if out != "" || m["emitted"] != "false" {
			t.Fatalf("stdout %q, meta %v; a run past its ceiling adds nothing", out, m)
		}
		if !strings.Contains(m["rich_missed"], "ceiling") || m["ceiling"] != "missed" {
			t.Errorf("meta %v; want rich_missed and ceiling naming the missed ceiling", m)
		}
	})
}

// TestReadoutRichOutputJoinsTheTestRunLine: the test-run tier's line and a
// rich line leave in one hook output; with no rich line the tier's bytes
// pass unchanged.
func TestReadoutRichOutputJoinsTheTestRunLine(t *testing.T) {
	codec, _ := harness.CodecFor(harness.ClaudeCode)
	o := newReadoutRichOutput(codec, harness.EventToolFailure)
	tier, err := o.capture.EncodeOutput(harness.EventToolFailure, harness.HookOutput{Context: "tier line"})
	if err != nil {
		t.Fatal(err)
	}
	o.tier.Write(append(tier, '\n'))
	var joined strings.Builder
	if !o.flush(&joined, []string{"rich line"}) || strings.Count(strings.TrimSpace(joined.String()), "\n") != 0 || !strings.Contains(joined.String(), `tier line\nrich line`) {
		t.Fatalf("joined output %q", joined.String())
	}
	o = newReadoutRichOutput(codec, harness.EventToolFailure)
	o.tier.Write(append(tier, '\n'))
	var alone strings.Builder
	if o.flush(&alone, nil) || alone.String() != string(tier)+"\n" || o.flush(&alone, []string{"late"}) {
		t.Fatalf("tier output %q, want the tier's bytes once", alone.String())
	}
}
