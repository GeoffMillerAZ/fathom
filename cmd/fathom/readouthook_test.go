package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// readoutTestRule is a user rule file the hook tests load: a lossy ls rule
// that keeps the first two entries, with its content-stream fixture.
const readoutTestRule = `schema = 1

[[rule]]
id = "ls-head"
version = 1
about = "ls: the first two entries"
match.argv0 = ["ls"]
recovery = "repeat"

[[rule.stage]]
kind = "cap"
head = 2

[[rule.test]]
input = "a.go\nb.go\nerror.log\n"
expect = "a.go\nb.go\n[readout ls-head@1 observe: 3→2 lines; dropped 1 (error 1); raw: repeat the command]\n"
`

// readoutHookEnv isolates one readout-hook run: HOME, the per-user fathom
// directory (config.toml from cfg when non-empty, the test rule file) and
// the hook log are private to the test.
func readoutHookEnv(t *testing.T, cfg string) string {
	t.Helper()
	isolateHookRun(t)
	home := t.TempDir()
	fh := filepath.Join(home, ".fathom")
	if err := os.MkdirAll(fh, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_HOME", fh)
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("FATHOM_CHANNELS", "")
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	if cfg != "" {
		if err := os.WriteFile(filepath.Join(fh, "config.toml"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(fh, "readouts.toml"), []byte(readoutTestRule), 0o644); err != nil {
		t.Fatal(err)
	}
	return fh
}

// readoutListing is an ls output long enough that keeping two entries is
// smaller than the raw output with its receipt.
func readoutListing(extra string) string {
	var b strings.Builder
	for i := range 40 {
		fmt.Fprintf(&b, "internal/some/longer/path/file_%02d.go\n", i)
	}
	b.WriteString(extra)
	return b.String()
}

func readoutPostToolEnvelope(t *testing.T, cwd, command, stdout string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"session_id": "s-1", "cwd": cwd, "hook_event_name": "PostToolUse", "tool_name": "Bash", "tool_use_id": "toolu_1",
		"tool_input":    map[string]any{"command": command},
		"tool_response": map[string]any{"stdout": stdout, "stderr": "", "interrupted": false, "isImage": false},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type readoutTestRow struct {
	Kind string            `json:"kind"`
	Meta map[string]string `json:"meta"`
}

// readoutRows reads the hook log's rows on the readout channel.
func readoutRows(t *testing.T) []readoutTestRow {
	t.Helper()
	f, err := os.Open(os.Getenv("FATHOM_HOOKS_LOG"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []readoutTestRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var row struct {
			Channel string `json:"channel"`
			readoutTestRow
		}
		if json.Unmarshal(sc.Bytes(), &row) == nil && row.Channel == readoutChannel {
			out = append(out, row.readoutTestRow)
		}
	}
	return out
}

// runReadoutHook runs the verb in process and records the steps it took.
func runReadoutHook(t *testing.T, args []string, payload string) (code int, stdout, stderr string, steps []string) {
	t.Helper()
	orig := readoutHookTrace
	t.Cleanup(func() { readoutHookTrace = orig })
	readoutHookTrace = func(step string) { steps = append(steps, step) }
	var out, errb bytes.Buffer
	code = cmdReadoutHook(args, strings.NewReader(payload), &out, &errb)
	return code, out.String(), errb.String(), steps
}

var readoutSHAPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestReadoutHookObserveEmitsNothingAndLogsOneRow(t *testing.T) {
	readoutHookEnv(t, "[readouts]\nmode = \"observe\"\n")
	cwd := t.TempDir()
	listing := readoutListing("")
	code, out, errOut, _ := runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, cwd, "ls -1", listing))
	if code != 0 || out != "" || errOut != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q; want 0 and nothing emitted", code, out, errOut)
	}
	rows := readoutRows(t)
	if len(rows) != 1 {
		t.Fatalf("%d readout rows, want exactly 1: %+v", len(rows), rows)
	}
	m := rows[0].Meta
	sum := sha256.Sum256([]byte("ls -1"))
	want := map[string]string{
		"rule": "ls-head@1", "mode": "observe", "configured": "observe", "class": "content", "event": "post-tool",
		"raw": "false", "emitted": "false", "trips": "0", "session": "s-1", "tool_use_id": "toolu_1",
		"bytes_in": fmt.Sprint(len(listing)), "cmd_sha": hex.EncodeToString(sum[:]),
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("meta[%s] = %q, want %q (row %v)", k, m[k], v, m)
		}
	}
	if !readoutSHAPattern.MatchString(m["rule_sha"]) {
		t.Errorf("rule_sha = %q, want a sha256", m["rule_sha"])
	}
	bytesIn, _ := strconv.Atoi(m["bytes_in"])
	if bytesOut, err := strconv.Atoi(m["bytes_out"]); err != nil || bytesOut >= bytesIn {
		t.Errorf("bytes_out = %q, want fewer than bytes_in %q", m["bytes_out"], m["bytes_in"])
	}
	if m["dropped"] == "" || m["ms"] == "" {
		t.Errorf("dropped %q and ms %q must be recorded", m["dropped"], m["ms"])
	}
	if _, ok := m["bypass"]; ok {
		t.Errorf("bypass recorded for a command with no FATHOM_READOUT= prefix: %v", m)
	}
}

// TestReadoutHookLeanAndRichObserveThisBuild: a mode set to lean or rich
// still emits nothing; the row names the configured mode beside the one
// the engine ran.
func TestReadoutHookLeanAndRichObserveThisBuild(t *testing.T) {
	for _, mode := range []string{"lean", "rich"} {
		t.Run(mode, func(t *testing.T) {
			readoutHookEnv(t, "[readouts]\nmode = \""+mode+"\"\n")
			code, out, _, _ := runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, t.TempDir(), "ls -1", readoutListing("")))
			if code != 0 || out != "" {
				t.Fatalf("exit %d, stdout %q; want 0 and nothing emitted", code, out)
			}
			rows := readoutRows(t)
			if len(rows) != 1 || rows[0].Meta["mode"] != "observe" || rows[0].Meta["configured"] != mode || rows[0].Meta["emitted"] != "false" {
				t.Fatalf("rows = %+v, want one observe row naming configured %s", rows, mode)
			}
		})
	}
}

func TestReadoutHookOffReadsOnlyTheEnvelope(t *testing.T) {
	for name, cfg := range map[string]string{
		"no config":     "",
		"no [readouts]": "[hooks]\ntest_run = true\n",
		"mode off":      "[readouts]\nmode = \"off\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			readoutHookEnv(t, cfg)
			code, out, errOut, steps := runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, t.TempDir(), "ls -1", readoutListing("")))
			if code != 0 || out != "" || errOut != "" {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
			}
			if !slices.Contains(steps, "envelope") || slices.Contains(steps, "rules") || slices.Contains(steps, "render") {
				t.Errorf("mode off must read the envelope and stop there: steps %v", steps)
			}
			if rows := readoutRows(t); len(rows) != 0 {
				t.Errorf("mode off wrote %d rows", len(rows))
			}
		})
	}
}

func TestReadoutHookOutsideRootsActsAsModeOff(t *testing.T) {
	inside := t.TempDir()
	cwd := filepath.Join(inside, "clone")
	cfgFor := func(root string) string {
		return fmt.Sprintf("[readouts]\nmode = \"observe\"\nroots = [%q]\n", root)
	}
	t.Run("outside", func(t *testing.T) {
		readoutHookEnv(t, cfgFor(t.TempDir()))
		code, out, _, steps := runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, cwd, "ls -1", readoutListing("")))
		if code != 0 || out != "" {
			t.Fatalf("exit %d, stdout %q", code, out)
		}
		if slices.Contains(steps, "rules") || len(readoutRows(t)) != 0 {
			t.Errorf("outside every root the hook ran: steps %v, rows %d", steps, len(readoutRows(t)))
		}
	})
	t.Run("a prefix that is not a directory boundary", func(t *testing.T) {
		readoutHookEnv(t, cfgFor(inside+"/cl"))
		runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, cwd, "ls -1", readoutListing("")))
		if n := len(readoutRows(t)); n != 0 {
			t.Errorf("root %s/cl matched %s: %d rows", inside, cwd, n)
		}
	})
	t.Run("inside", func(t *testing.T) {
		readoutHookEnv(t, cfgFor(inside))
		runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, cwd, "ls -1", readoutListing("")))
		if n := len(readoutRows(t)); n != 1 {
			t.Errorf("under a root: %d rows, want 1", n)
		}
	})
}

func TestReadoutHookErrorExitsZeroAndEmitsNothing(t *testing.T) {
	const on = "[readouts]\nmode = \"observe\"\n"
	cases := map[string]struct {
		args    []string
		payload string
		panicAt string
	}{
		"unparseable envelope": {args: []string{"--event", "post-tool"}, payload: "{not json"},
		"unknown event":        {args: []string{"--event", "nope"}},
		"unknown flag":         {args: []string{"--nope"}},
		"panic in the engine":  {args: []string{"--event", "post-tool"}, panicAt: "render"},
		"unknown harness":      {args: []string{"--event", "post-tool", "--harness", "nope"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			readoutHookEnv(t, on)
			payload := c.payload
			if payload == "" {
				payload = readoutPostToolEnvelope(t, t.TempDir(), "ls -1", readoutListing(""))
			}
			orig := readoutHookTrace
			t.Cleanup(func() { readoutHookTrace = orig })
			readoutHookTrace = func(step string) {
				if step == c.panicAt {
					panic("boom")
				}
			}
			var out bytes.Buffer
			code := cmdReadoutHook(c.args, strings.NewReader(payload), &out, &bytes.Buffer{})
			if code != 0 || out.Len() != 0 {
				t.Fatalf("exit %d, stdout %q; want 0 and nothing emitted", code, out.String())
			}
			if rows := readoutRows(t); len(rows) != 0 {
				t.Errorf("an error wrote %d rows", len(rows))
			}
		})
	}
}

func TestReadoutHookRowCarriesNoOutputOrCommandText(t *testing.T) {
	readoutHookEnv(t, "[readouts]\nmode = \"observe\"\n")
	cwd := t.TempDir()
	runs := []struct{ command, stdout string }{
		{"ls -1 secretcmdword", readoutListing("secretoutword.go\n")},
		{`echo "secretcmdword`, "secretoutword\n"},
		{"FATHOM_READOUT=raw ls secretcmdword", readoutListing("secretoutword\n")},
	}
	for _, r := range runs {
		runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, cwd, r.command, r.stdout))
	}
	rows := readoutRows(t)
	if len(rows) != len(runs) {
		t.Fatalf("%d rows, want %d", len(rows), len(runs))
	}
	raw, err := os.ReadFile(os.Getenv("FATHOM_HOOKS_LOG"))
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"secretcmdword", "secretoutword", "file_00"} {
		if bytes.Contains(raw, []byte(word)) {
			t.Errorf("the hook log carries %q:\n%s", word, raw)
		}
	}
	if m := rows[1].Meta; m["raw"] != "true" || m["reason"] != "unreadable" || m["rule"] != "" {
		t.Errorf("unreadable command row = %v, want raw with reason unreadable and no rule", m)
	}
	if m := rows[2].Meta; m["bypass"] != "raw" || m["reason"] != "prefix-raw" {
		t.Errorf("FATHOM_READOUT=raw row = %v, want bypass raw", m)
	}
}

func TestReadoutHookFailureEventLogsOneRow(t *testing.T) {
	readoutHookEnv(t, "[readouts]\nmode = \"observe\"\n")
	b, _ := json.Marshal(map[string]any{
		"session_id": "s-2", "cwd": t.TempDir(), "hook_event_name": "PostToolUseFailure", "tool_name": "Bash", "tool_use_id": "toolu_2",
		"tool_input": map[string]any{"command": "ls -1 missing"},
		"error":      "Exit code 2\n" + readoutListing("ls: missing: No such file or directory\n"),
	})
	code, out, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, string(b))
	if code != 0 || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	rows := readoutRows(t)
	if len(rows) != 1 || rows[0].Meta["event"] != "tool-failure" || rows[0].Meta["tool_use_id"] != "toolu_2" || rows[0].Meta["emitted"] != "false" {
		t.Fatalf("rows = %+v, want one tool-failure row", rows)
	}
}

// TestReadoutHookRowsWrittenOnlyWhileModeIsOn: init, reconcile and doctor
// carry the readout-hook rows from the one hook table, and write or expect
// them only while the user's [readouts] mode is set and not off.
func TestReadoutHookRowsWrittenOnlyWhileModeIsOn(t *testing.T) {
	markers := []string{"fathom readout-hook --event post-tool", "fathom readout-hook --event tool-failure"}
	for _, m := range markers {
		found := false
		for _, hs := range fathomHookSpecs {
			found = found || hs.marker == m
		}
		if !found {
			t.Fatalf("the hook table lacks %q", m)
		}
	}
	for _, c := range []struct {
		cfg  string
		want bool
	}{{"", false}, {"[readouts]\nmode = \"off\"\n", false}, {"[readouts]\nmode = \"observe\"\n", true}} {
		readoutHookEnv(t, c.cfg)
		var wired []string
		for _, hs := range wiredHookSpecs(fathomHookSpecs) {
			wired = append(wired, hs.marker)
		}
		settings, _, _, err := mergeHookSettings(nil, "fathom")
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		missing := strings.Join(analyzeWiring(root).Missing, "\n")
		for _, m := range markers {
			if got := slices.Contains(wired, m); got != c.want {
				t.Errorf("config %q: wired %q = %v, want %v", c.cfg, m, got, c.want)
			}
			if got := bytes.Contains(settings, []byte(m)); got != c.want {
				t.Errorf("config %q: init writes %q = %v, want %v", c.cfg, m, got, c.want)
			}
		}
		if got := strings.Contains(missing, "readout-hook"); got != c.want {
			t.Errorf("config %q: reconcile names the readout hook missing = %v, want %v:\n%s", c.cfg, got, c.want, missing)
		}
	}
}
