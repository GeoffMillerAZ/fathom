package readout

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// readOutlinePath is the path the fixture source is read at: the outline's
// language comes from it, never from the fixture's own file name.
const readOutlinePath = "internal/cart/cart.go"

// readOutlineFixture is the fixture source (testdata/read-outline/cart.go.txt,
// 270 lines of Go) and its expected outline join lines.
func readOutlineFixture(t *testing.T) (src string, outline []string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "read-outline", "cart.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := os.ReadFile(filepath.Join("testdata", "read-outline", "cart.go.outline"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b), strings.Split(strings.TrimSuffix(string(o), "\n"), "\n")
}

// TestBuiltinReadOutlineFixtures (RO-8): a whole-file read of a source file
// over 200 lines, `cat` or `sed -n 1,$p`, becomes the first 40 lines, the
// outline join's top-level symbols with their line ranges (fathom's own
// extractor run in process over the text as printed) and the `sed -n A,Bp`
// span recipe; an explicit range, several files, a head or tail, a flag that
// changes what prints, a pipe and a short file stay raw.
func TestBuiltinReadOutlineFixtures(t *testing.T) {
	rs := builtinSet(t)
	r := builtin(t, rs, "read-outline")
	if r.loss != LossWhole || r.Recovery != RecoverySpan || !r.Match.WholeRead || r.When.LinesGT != 200 {
		t.Fatalf("read-outline: lossiness %v recovery %q whole_read %v lines_gt %d, want whole, span, true, 200", r.loss, r.Recovery, r.Match.WholeRead, r.When.LinesGT)
	}
	src, want := readOutlineFixture(t)
	got, missed := Outline(context.Background(), readOutlinePath, src)
	if missed || !reflect.DeepEqual(got, want) {
		t.Fatalf("outline join (missed %v):\n%s\nwant:\n%s", missed, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	again, _ := Outline(context.Background(), readOutlinePath, src)
	if !reflect.DeepEqual(again, got) {
		t.Fatal("the outline join is not deterministic")
	}
	srcLines, _ := splitLines(src)
	head := strings.Join(srcLines[:40], "\n") + "\n"
	view := head + strings.Join(want, "\n") + "\n" +
		"[readout read-outline@1 observe: 270→" + strconv.Itoa(40+len(want)) + " lines; dropped 230 (error 23, plain 207); raw: sed -n 41,270p " + readOutlinePath + "]\n"
	for _, cmd := range []string{
		"cat " + readOutlinePath,
		"sed -n '1,$p' " + readOutlinePath,
		"sed -n -e '1,$p' " + readOutlinePath,
	} {
		ro := Render(rs, Input{Command: lexed(cmd), Stdout: src, Joins: map[string][]string{"outline": got}})
		if ro.Raw || ro.Rule != "read-outline@1" || ro.Mode != ModeObserve || ro.Stream != StreamContent || len(ro.JoinsMissed) != 0 {
			t.Fatalf("%s: raw=%v reason=%q rule=%q mode=%q stream=%q missed=%v, want read-outline's observe readout", cmd, ro.Raw, ro.Reason, ro.Rule, ro.Mode, ro.Stream, ro.JoinsMissed)
		}
		if ro.Stdout != view {
			t.Errorf("%s: %s", cmd, firstDifference(ro.Stdout, view))
		}
		if path, ok := WholeFileRead(lexed(cmd)); !ok || path != readOutlinePath {
			t.Errorf("%s: WholeFileRead = %q, %v", cmd, path, ok)
		}
	}
	stayRaw(t, rs, src,
		"sed -n 1,270p "+readOutlinePath,
		"sed -n 41,120p "+readOutlinePath,
		"sed '1,$p' "+readOutlinePath,
		"cat "+readOutlinePath+" internal/cart/item.go",
		"cat -n "+readOutlinePath,
		"head -n 300 "+readOutlinePath,
		"tail -n 300 "+readOutlinePath,
		"cat "+readOutlinePath+" | head -n 300",
		"cat "+readOutlinePath+" > /dev/null",
		"cat $F",
	)
	for _, cmd := range []string{"sed -n 1,270p " + readOutlinePath, "cat a.go b.go", "head " + readOutlinePath} {
		if _, ok := WholeFileRead(lexed(cmd)); ok {
			t.Errorf("%s: WholeFileRead accepted a read that is not one whole file", cmd)
		}
	}
	short := strings.Join(srcLines[:200], "\n") + "\n"
	if ro := Render(rs, Input{Command: lexed("cat " + readOutlinePath), Stdout: short, Joins: map[string][]string{"outline": got}}); !ro.Raw || ro.Reason != ReasonBelowFloor {
		t.Errorf("a 200-line file: raw=%v reason=%q, want raw below the floor", ro.Raw, ro.Reason)
	}
}

// TestReadOutlineEditedFileStaysRaw: a whole-file read of a file the session
// edited (the caller's Input.Edited) is verification and stays raw, the rule
// still named for the row.
func TestReadOutlineEditedFileStaysRaw(t *testing.T) {
	rs := builtinSet(t)
	src, outline := readOutlineFixture(t)
	ro := Render(rs, Input{Command: lexed("cat " + readOutlinePath), Stdout: src, Edited: true, Joins: map[string][]string{"outline": outline}})
	if !ro.Raw || ro.Reason != ReasonEdited || ro.Rule != "read-outline@1" || ro.Stdout != src {
		t.Fatalf("edited read: raw=%v reason=%q rule=%q, want the raw output under read-outline", ro.Raw, ro.Reason, ro.Rule)
	}
}

// TestReadOutlineJoinDeadline: an outline the caller could not compute in
// its 150 ms says so in the view, and a file no extractor reads leaves the
// join missing, named in the receipt and the readout.
func TestReadOutlineJoinDeadline(t *testing.T) {
	rs := builtinSet(t)
	src, _ := readOutlineFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), OutlineDeadline)
	cancel()
	lines, missed := Outline(ctx, readOutlinePath, src)
	if !missed || !reflect.DeepEqual(lines, []string{OutlineDeadlineLine}) || OutlineDeadlineLine != "outline: not computed (deadline)" || OutlineDeadline != 150*time.Millisecond {
		t.Fatalf("a done context: %v missed %v, want the deadline line", lines, missed)
	}
	ro := Render(rs, Input{Command: lexed("cat " + readOutlinePath), Stdout: src, Joins: map[string][]string{"outline": lines}})
	body, _ := splitLines(ro.Stdout)
	if ro.Raw || len(body) != 42 || body[40] != OutlineDeadlineLine {
		t.Fatalf("deadline view: raw=%v reason=%q, line 41 %q, want the head and the deadline line", ro.Raw, ro.Reason, body[min(40, len(body)-1)])
	}
	if lines, missed := Outline(context.Background(), "notes/plan.unknownext", src); lines != nil || !missed {
		t.Errorf("a file no extractor reads: %v missed %v, want no lines, missed", lines, missed)
	}
	ro = Render(rs, Input{Command: lexed("cat " + readOutlinePath), Stdout: src})
	if ro.Raw || !reflect.DeepEqual(ro.JoinsMissed, []string{"outline"}) || !strings.Contains(ro.Receipt, "join outline: not computed") {
		t.Errorf("no join: raw=%v missed=%v receipt %q, want the outline named as missed", ro.Raw, ro.JoinsMissed, ro.Receipt)
	}
}
