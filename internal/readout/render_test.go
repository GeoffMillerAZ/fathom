package readout

import (
	"reflect"
	"strings"
	"testing"
)

// TestRenderServesRawWhenAProtectedLineWouldDrop is RO-5's contract: in a
// diagnostic stream a dropped FAIL line trips and serves the raw output with
// a receipt naming the trip; in a content stream a dropped line carrying
// `error` and a path:line: prefix is ordinary lossiness, counted by class;
// `Exit code N` is protected in every stream; the bytes are deterministic.
func TestRenderServesRawWhenAProtectedLineWouldDrop(t *testing.T) {
	rs := loadTestRules(t)

	diag := "cc -c a.c\n--- FAIL: TestA (0.01s)\n" + lines("cc -c gen", 30)
	got := Render(rs, Input{Command: lexed("cd sub && make"), Stdout: diag})
	if !got.Raw || got.Reason != ReasonProtected || got.Stream != StreamDiagnostic {
		t.Fatalf("diagnostic drop of FAIL: raw=%v reason=%q stream=%q, want a protected trip", got.Raw, got.Reason, got.Stream)
	}
	if len(got.Trips) != 1 || got.Trips[0] != (Trip{Line: 2, Class: "error"}) {
		t.Fatalf("trips = %+v, want line 2 (error)", got.Trips)
	}
	wantView := diag + "[readout make-quiet@3 observe: raw served; protected line 2 (error) would drop]\n"
	if got.Stdout != wantView {
		t.Fatalf("trip view = %q, want the raw output and the trip receipt %q", got.Stdout, wantView)
	}

	content := lines("x_test.go:4:\terr := errors.New(\"boom\") ", 20) + "a.go:1:\treturn err\n"
	got = Render(rs, Input{Command: lexed("grep -rn err . | head -n 100"), Stdout: content})
	if got.Raw || got.Stream != StreamContent {
		t.Fatalf("content drop: raw=%v reason=%q stream=%q, want a served readout", got.Raw, got.Reason, got.Stream)
	}
	wantView = "a.go:1:\treturn err\n[readout grep-skip-tests@1 observe: 21→1 lines; dropped 20 (error 20); raw: repeat the command]\n"
	if got.Stdout != wantView {
		t.Fatalf("content view = %q, want %q", got.Stdout, wantView)
	}
	if want := []ClassCount{{"error", 20}}; !reflect.DeepEqual(got.Dropped, want) {
		t.Fatalf("dropped = %+v, want %+v", got.Dropped, want)
	}

	// The same grep with stderr merged into stdout is diagnostic: its error
	// lines are protected again.
	got = Render(rs, Input{Command: lexed("grep -rn err . 2>&1 | head -n 100"), Stdout: content})
	if !got.Raw || got.Reason != ReasonProtected || got.Stream != StreamDiagnostic {
		t.Fatalf("merged stderr: raw=%v reason=%q stream=%q, want a protected trip", got.Raw, got.Reason, got.Stream)
	}

	got = Render(rs, Input{Command: lexed("ls"), Stdout: "a.go\nb.go\n" + lines("c", 20) + "Exit code 2\n"})
	if !got.Raw || got.Reason != ReasonProtected || got.Trips[0].Class != "exit" {
		t.Fatalf("content drop of Exit code: raw=%v reason=%q trips=%+v, want an exit trip", got.Raw, got.Reason, got.Trips)
	}

	a := Render(rs, Input{Command: lexed("grep -rn err ."), Stdout: content})
	b := Render(rs, Input{Command: lexed("grep -rn err ."), Stdout: content})
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two renders of the same input differ")
	}
}

func TestRenderKeepsRawWhatItMustNotRead(t *testing.T) {
	rs := loadTestRules(t)
	out := lines("cc -c f", 40) + "ok\n"
	cases := []struct{ cmd, reason, detail string }{
		{"make 'x", ReasonUnreadable, "unterminated '"},
		{"FATHOM_READOUT=raw make", ReasonPrefixRaw, ""},
		{"rtk make", ReasonRtk, ""},
		{"make > build.log", ReasonMachineBound, "redirect"},
		{"make 2>/dev/null", ReasonMachineBound, "redirect"},
		{"make &", ReasonMachineBound, "background job"},
		{"sudo make", ReasonMachineBound, "sudo"},
		{"bash -c 'make'", ReasonMachineBound, "shell -c"},
		{"(make)", ReasonMachineBound, "subshell"},
		{"make <<EOF\nx\nEOF", ReasonMachineBound, "heredoc"},
		{"make | grep ok", ReasonMachineBound, "pipe into grep"},
		{"make | tail -f", ReasonMachineBound, "pipe into tail"},
		{"grep -rl err .", ReasonMachineBound, "flag -l"},
		{"git status --porcelain", ReasonMachineBound, "flag --porcelain"},
		{"go test -json ./...", ReasonMachineBound, "flag -json"},
		{"make build && make test", ReasonNoRule, "list"},
		{"cmake", ReasonNoRule, "simple"},
	}
	for _, tc := range cases {
		got := Render(rs, Input{Command: lexed(tc.cmd), Stdout: out})
		if !got.Raw || got.Reason != tc.reason || got.Detail != tc.detail || got.Stdout != out {
			t.Errorf("%q: raw=%v reason=%q detail=%q, want raw %q %q and the stdout untouched", tc.cmd, got.Raw, got.Reason, got.Detail, tc.reason, tc.detail)
		}
	}
	if got := Render(rs, Input{Command: lexed("make"), Stdout: out, Interrupted: true}); got.Reason != ReasonInterrupted {
		t.Errorf("interrupted run: reason %q, want %q", got.Reason, ReasonInterrupted)
	}
	if got := Render(rs, Input{Stdout: out}); got.Reason != ReasonUnreadable {
		t.Errorf("no command: reason %q, want %q", got.Reason, ReasonUnreadable)
	}
}

func TestRenderNeverServesALeanReadoutLargerThanTheRaw(t *testing.T) {
	rs := loadTestRules(t)
	got := Render(rs, Input{Command: lexed("make"), Stdout: "cc -c a.c\nok\n"})
	if !got.Raw || got.Reason != ReasonNeverWorse {
		t.Fatalf("small output: raw=%v reason=%q, want never-worse", got.Raw, got.Reason)
	}
	big := lines("cc -c f", 40) + "ok\n"
	got = Render(rs, Input{Command: lexed("FATHOM_READOUT=lean make"), Stdout: big})
	if got.Raw || got.Mode != ModeLean {
		t.Fatalf("lean prefix: raw=%v reason=%q mode=%q, want a lean readout", got.Raw, got.Reason, got.Mode)
	}
	want := "ok\n[readout make-quiet@3 lean: 41→1 lines; dropped 40 (plain 40); raw: repeat the command]\n"
	if got.Stdout != want || got.Receipt+"\n" != strings.TrimPrefix(want, "ok\n") {
		t.Fatalf("lean view = %q, want %q", got.Stdout, want)
	}
	if got.BytesIn != len(big) || got.BytesOut != len(want) || got.LinesIn != 41 || got.LinesOut != 1 {
		t.Fatalf("counts = %d→%d bytes, %d→%d lines", got.BytesIn, got.BytesOut, got.LinesIn, got.LinesOut)
	}
}

func TestRenderEmitsRecordsAndALosslessReceipt(t *testing.T) {
	rs := loadTestRules(t)
	out := strings.Repeat("# example.com/pkg\n", 30) + "x.go:3:1: unreachable code\n"
	got := Render(rs, Input{Command: lexed("go vet ./..."), Stdout: out})
	if got.Raw || got.Lossiness != LossNone {
		t.Fatalf("raw=%v reason=%q loss=%v, want a lossless readout", got.Raw, got.Reason, got.Lossiness)
	}
	want := "# example.com/pkg ×30\nx.go:3:1: unreachable code\n[readout go-vet-records@1 observe: 31→2 lines; nothing dropped]\n"
	if got.Stdout != want {
		t.Fatalf("view = %q, want %q", got.Stdout, want)
	}
	wantRec := []Record{{Kind: "vet_finding", Line: 31, Fields: []Field{{"file", "x.go"}, {"line", "3"}}}}
	if !reflect.DeepEqual(got.Records, wantRec) {
		t.Fatalf("records = %+v, want %+v", got.Records, wantRec)
	}
}

func TestRenderLabelsCountsOverTheHarnessCap(t *testing.T) {
	rs := loadTestRules(t)
	out := lines("cc -c f", 40) + "ok\n"
	got := Render(rs, Input{Command: lexed("make"), Stdout: out, Cap: len(out)})
	if !strings.Contains(got.Receipt, "counts in the shown "+thousands(len(out))+" chars") {
		t.Fatalf("receipt %q does not label its counts as over the shown text", got.Receipt)
	}
	if thousands(30000) != "30,000" || thousands(999) != "999" || thousands(1234567) != "1,234,567" {
		t.Fatal("thousands separators")
	}
}
