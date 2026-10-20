package readout

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestQuietPrefixKeepsThePrefixOutput (RO-28): a command whose leading
// `&&`-joined segments are all quiet is classified by its last segment, and
// the quiet segments' own output lines are kept verbatim ahead of what the
// rule renders. `;`, `||`, a non-quiet leading segment, a redirect or a
// wrapper on a quiet segment does not classify.
func TestQuietPrefixKeepsThePrefixOutput(t *testing.T) {
	rs := builtinSet(t)
	run := goTestRun(40, true) + "FAIL\n"
	ls, _ := lsListing(40)
	grep := grepOutput([]string{"internal/config/load.go", "cmd/shop/main.go"}, []int{60, 40})

	for _, tc := range []struct {
		cmd, stdout, rule string
		quiet             []string
	}{
		{"cd a && go test ./...", run, "go-test@1", nil},
		{"cd a && go test ./... 2>&1 | tail -n 80", run, "go-test@1", nil},
		{"cd a && ls -la", ls, "ls-long@1", nil},
		{"export X=1 && sleep 2 && grep -rn foo .", grep, "grep-group@1", nil},
		{"X=1 && set -euo pipefail && source env.sh && . ./more.sh && mkdir -p out && go test ./...", run, "go-test@1", nil},
		{"tccutil reset SystemPolicyRemovableVolumes com.example.term && go test ./...", run, "go-test@1", nil},
		{"nvm use 20 && go test ./...", run, "go-test@1", []string{"nvm"}},
		{"direnv exec . true && go test ./...", run, "go-test@1", []string{"direnv exec"}},
	} {
		got := Render(rs, Input{Command: lexed(tc.cmd), Stdout: tc.stdout, Quiet: tc.quiet})
		if got.Rule != tc.rule || got.Raw {
			t.Errorf("%s: rule %q raw=%v reason=%q, want %s served", tc.cmd, got.Rule, got.Raw, got.Reason, tc.rule)
		}
	}

	// The projection is the last segment's: the same view, records and
	// receipt as the bare command, rich mode's lines included.
	const saved = "/state/readouts/saved/1f2e.txt"
	bare := Render(rs, Input{Command: lexed("go test ./..."), Stdout: run, Mode: ModeRich, Saved: saved})
	pre := Render(rs, Input{Command: lexed("cd a && go test ./..."), Stdout: run, Mode: ModeRich, Saved: saved})
	if pre.Raw || pre.Stdout != bare.Stdout || !reflect.DeepEqual(pre.Records, bare.Records) || len(pre.Records) != 2 {
		t.Fatalf("cd a && go test, rich: raw=%v reason=%q records %+v; want the bare command's view and its 2 records", pre.Raw, pre.Reason, pre.Records)
	}

	// Not classified: stays as today (no built-in accepts a list).
	for _, cmd := range []string{
		"make && go test ./...", "cd a; go test ./...", "cd a || go test ./...", "cd a\ngo test ./...",
		"cd a > /dev/null && go test ./...", "nohup mkdir -p out && go test ./...", "! cd a && go test ./...",
		"mkdir out && go test ./...", "sleep soon && go test ./...", "tccutil list && go test ./...",
		"nvm use 20 && go test ./...", "cd a && go test ./... | wc -l", "cd a && go test ./... &",
	} {
		got := Render(rs, Input{Command: lexed(cmd), Stdout: run})
		if got.Rule != "" || !got.Raw || got.Stdout != run {
			t.Errorf("%s: rule %q reason %q, want no rule and the raw output", cmd, got.Rule, got.Reason)
		}
	}

	// The quiet segments' own lines (set -x traces when stderr is merged,
	// mkdir -v's created directories, cd's echo, tccutil's reset line) are
	// kept verbatim ahead of the rule's view; the records count input lines.
	for _, tc := range []struct{ cmd, head string }{
		{"set -x && mkdir -pv out/a && go test ./...", "+ mkdir -pv out/a\nmkdir: created directory 'out'\nmkdir: created directory 'out/a'\n+ go test ./...\n"},
		{"mkdir -p -v out/a && go test ./...", "out\nout/a\n"},
		{"cd - && go test ./...", "/work/prev\n"},
		{"cd shop && go test ./...", "/work/src/shop\n"},
		{"tccutil reset All com.example.term && go test ./...", "Successfully reset All approval status for com.example.term\n"},
	} {
		got := Render(rs, Input{Command: lexed(tc.cmd), Stdout: tc.head + run, Mode: ModeRich, Saved: saved})
		if got.Raw || got.Rule != "go-test@1" {
			t.Errorf("%s: raw=%v reason=%q rule=%q, want go-test served", tc.cmd, got.Raw, got.Reason, got.Rule)
			continue
		}
		if got.Stdout != tc.head+bare.Stdout {
			t.Errorf("%s: view\n%q\nwant the prefix lines verbatim, then the bare view\n%q", tc.cmd, got.Stdout, tc.head+bare.Stdout)
		}
		n := strings.Count(tc.head, "\n")
		if len(got.Records) != 2 || got.Records[0].Line != bare.Records[0].Line+n || got.Records[1].Line != bare.Records[1].Line+n {
			t.Errorf("%s: records %+v, want the bare ones moved down %d lines", tc.cmd, got.Records, n)
		}
		if got.BytesIn != len(tc.head)+len(run) || got.LinesOut != bare.LinesOut+n {
			t.Errorf("%s: bytes in %d lines out %d, want %d and %d", tc.cmd, got.BytesIn, got.LinesOut, len(tc.head)+len(run), bare.LinesOut+n)
		}
	}
	// A line no quiet segment prints is the producer's: it reaches the rule.
	odd := "+ not a trace without set -x\n/work/src/shop\n"
	oddBare := Render(rs, Input{Command: lexed("go test ./..."), Stdout: odd + run, Mode: ModeRich, Saved: saved})
	if got := Render(rs, Input{Command: lexed("cd a && go test ./..."), Stdout: odd + run, Mode: ModeRich, Saved: saved}); got.Raw || got.Stdout != oddBare.Stdout {
		t.Errorf("unrecognized leading lines were not read as the producer's: raw=%v view %q, want %q", got.Raw, got.Stdout, oddBare.Stdout)
	}

	// A protected trip in the producer's part names its whole-output line.
	tripped := "out\n" + run + "ok  \texample.com/shop/internal/errors\t0.120s\n"
	got := Render(rs, Input{Command: lexed("mkdir -pv out && go test ./..."), Stdout: tripped})
	want := strings.Count(tripped, "\n")
	if !got.Raw || got.Reason != ReasonProtected || len(got.Trips) != 1 || got.Trips[0].Line != want {
		t.Errorf("trip: raw=%v reason=%q trips %+v, want one at line %d", got.Raw, got.Reason, got.Trips, want)
	}
}

// TestQuietHeadsNameTheQuietList: the list the replay prints is the one the
// classifier reads, each head shown by a sample segment it accepts.
func TestQuietHeadsNameTheQuietList(t *testing.T) {
	samples := map[string]string{
		"cd": "cd a", "export": "export X=1", "mkdir -p": "mkdir -p a", "sleep N": "sleep 2",
		"(assignment)": "X=1", "set": "set -e", "source": "source env.sh", ".": ". env.sh",
		"tccutil reset": "tccutil reset All",
	}
	heads := QuietHeads()
	if len(heads) != len(samples) {
		t.Fatalf("QuietHeads() = %v, want the %d heads sampled here", heads, len(samples))
	}
	for _, h := range heads {
		s, ok := samples[h]
		if !ok {
			t.Errorf("head %q has no sample", h)
			continue
		}
		c := lexed(s + " && ls")
		if !IsQuiet(&c.Pipelines[0].Units[0], nil) {
			t.Errorf("head %q: %q is not quiet", h, s)
		}
		if p, rest := stripQuietPrefix(c, nil); len(p) != 1 || len(rest) != 1 {
			t.Errorf("head %q: strip = %d quiet, %d rest; want 1 and 1", h, len(p), len(rest))
		}
	}
	if p, rest := stripQuietPrefix(lexed("cd a && cd b"), nil); len(p) != 1 || len(rest) != 1 {
		t.Errorf("the last segment is never stripped: %d, %d", len(p), len(rest))
	}
	if p, _ := stripQuietPrefix(lexed(fmt.Sprintf("cd a && %s", "echo $(")), nil); p != nil {
		t.Errorf("an unreadable command strips nothing")
	}
}
