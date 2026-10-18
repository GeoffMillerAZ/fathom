package readout

import (
	"strings"
	"testing"
)

// TestBaseStagesAreDisplayEquivalent pins RO-13: the base stages strip ANSI
// escapes and carriage-return progress overwrites (display-equivalent) and
// collapse identical consecutive lines to `line ×N`; they run inside every
// matched rule (Render calls them), are no rule of their own, and are never
// claimed as a saving in the receipt.
func TestBaseStagesAreDisplayEquivalent(t *testing.T) {
	raw := "\x1b[32mok\x1b[0m  \texample.com/shop/cart\t0.412s\n" +
		"downloading  10%\rdownloading  55%\rdownloading 100%\n" +
		"waiting for lock\nwaiting for lock\nwaiting for lock\n" +
		"\x1b[1;31mFAIL\x1b[0m\texample.com/shop/checkout\t0.233s\n" +
		"retry ×2\n" +
		"done\n"
	in, _ := splitLines(raw)
	got := baseStages(inputLines(in))
	want := []string{
		"ok  \texample.com/shop/cart\t0.412s",
		"downloading 100%",
		"waiting for lock ×3",
		"FAIL\texample.com/shop/checkout\t0.233s",
		"retry ×2 ×1",
		"done",
	}
	if strings.Join(texts(got), "|") != strings.Join(want, "|") {
		t.Fatalf("baseStages = %q, want %q", texts(got), want)
	}

	// Display-equivalent: expanding the collapse gives back what a terminal
	// shows for every raw line, in order.
	shown := make([]string, len(in))
	for i, l := range in {
		shown[i] = renderTerminal(l)
	}
	if strings.Join(expandLines(got), "\n") != strings.Join(shown, "\n") {
		t.Fatalf("expand(baseStages) = %q, want the terminal text %q", expandLines(got), shown)
	}
	if err := baseRoundTrip(inputLines(in), got); err != nil {
		t.Fatal(err)
	}
	// Every raw line is carried exactly (no line drops, no fate change).
	seen := make([]bool, len(in))
	for _, l := range got {
		if l.fate != fateExact || l.origin != fromInput {
			t.Fatalf("line %q: fate %v origin %v, want exact input", l.text, l.fate, l.origin)
		}
		for _, i := range l.src {
			seen[i] = true
		}
	}
	for i, ok := range seen {
		if !ok {
			t.Fatalf("raw line %d is carried by no base line", i+1)
		}
	}
	// Idempotent: base over its own output, and a rule's own collapse or ansi
	// stage after it, change nothing (no ` ×1` escape on a collapsed line).
	again := baseStages(got)
	if strings.Join(texts(again), "|") != strings.Join(want, "|") {
		t.Fatalf("baseStages twice = %q", texts(again))
	}
	for _, s := range []Stage{{Kind: StageCollapse}, {Kind: StageANSI}} {
		next := s.apply(got, &runCtx{})
		if strings.Join(texts(next), "|") != strings.Join(want, "|") {
			t.Fatalf("%s after base = %q", s.Kind, texts(next))
		}
		if err := s.roundTrip(got, next); err != nil {
			t.Fatalf("%s after base: %v", s.Kind, err)
		}
	}

	// Inside Render: a matched rule runs the base stages first, and the
	// receipt's line count starts from the base lines, so base is never
	// claimed as a saving.
	rs := loadTestRules(t)
	stdout := strings.Repeat("\x1b[2mwaiting for jobserver\x1b[0m\n", 50) + lines("cc -c f", 40) + "ok\n"
	r := Render(rs, Input{Command: lexed("make"), Stdout: stdout})
	if r.Raw || r.Rule != "make-quiet@3" {
		t.Fatalf("raw=%v reason=%q rule=%q, want make-quiet's readout", r.Raw, r.Reason, r.Rule)
	}
	wantView := "waiting for jobserver ×50\nok\n[readout make-quiet@3 observe: 42→2 lines; dropped 40 (plain 40); raw: repeat the command]\n"
	if r.Stdout != wantView {
		t.Fatalf("view = %q, want %q", r.Stdout, wantView)
	}
	if r.LinesIn != 91 || r.BytesIn != len(stdout) {
		t.Fatalf("LinesIn=%d BytesIn=%d, want the raw 91 lines and %d bytes", r.LinesIn, r.BytesIn, len(stdout))
	}
	if r.BaseBytes != len("waiting for jobserver ×50\n"+lines("cc -c f", 40)+"ok\n") {
		t.Fatalf("BaseBytes = %d, want the bytes after the base stages", r.BaseBytes)
	}
	// What only the base stages would save is not served: the rule's own
	// stages drop nothing here, so the raw output stays (never-worse).
	only := strings.Repeat("\x1b[2mwaiting for jobserver\x1b[0m\n", 200) + "ok\n"
	if r := Render(rs, Input{Command: lexed("make"), Stdout: only}); !r.Raw || r.Reason != ReasonNeverWorse {
		t.Fatalf("base-only saving: raw=%v reason=%q, want never-worse raw", r.Raw, r.Reason)
	}
	// A protected line hidden behind escapes is still classed by what the
	// terminal shows, and the base stages never drop it.
	diag := "\x1b[31m--- FAIL: TestA (0.00s)\x1b[0m\n" + lines("cc -c f", 40)
	if r := Render(rs, Input{Command: lexed("make"), Stdout: diag}); !r.Raw || r.Reason != ReasonProtected || len(r.Trips) != 1 {
		t.Fatalf("escaped FAIL line: raw=%v reason=%q trips=%v, want a protected trip", r.Raw, r.Reason, r.Trips)
	}
	// No rule file is the base: the embedded built-ins carry no `base` id.
	all, err := LoadRules("")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range all.Rules {
		if r.ID == "base" {
			t.Fatalf("a rule named base is loaded from %s; the base stages are never a rule", r.Source)
		}
	}
}
