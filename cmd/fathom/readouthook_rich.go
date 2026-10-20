package main

// Rich mode's injected lines (docs/plans/readouts.md phase 2, ADR-071):
// built now and reachable only under the user's [readouts] mode = rich
// (default off); turning rich on waits for RO-M1's PASS. A line is added
// beside the raw output, never in place of it, so the output and any saved
// path stand whole. A miss is silent and named in the row (rich_missed),
// never a wrong line.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
	"github.com/geoffmilleraz/fathom/internal/testrun"
)

// readoutRichLineBudget caps each rich line: one line, well inside I35's
// 320-token whole-payload ceiling, as the test-run line's cap is.
const readoutRichLineBudget = testRunLineBudget

// readoutRich is one run's rich lines and what its row records.
type readoutRich struct {
	lines  []string
	kinds  []string
	missed []string
}

// readoutRichCodec is the hook's codec that also keeps the context the
// test-run tier encodes, so a rich line joins the tier's line in one hook
// output instead of a second JSON document on stdout.
type readoutRichCodec struct {
	harness.Codec
	contexts []string
}

func (c *readoutRichCodec) EncodeOutput(ev harness.Event, out harness.HookOutput) ([]byte, error) {
	b, err := c.Codec.EncodeOutput(ev, out)
	if err == nil && len(b) > 0 && out.Context != "" {
		c.contexts = append(c.contexts, out.Context)
	}
	return b, err
}

// readoutRichOutput holds the test-run tier's output while rich mode may
// add a line: flush writes the tier's bytes unchanged when no rich line was
// added, else one output carrying the tier's context and the rich lines.
type readoutRichOutput struct {
	codec   harness.Codec
	ev      harness.Event
	tier    bytes.Buffer
	capture *readoutRichCodec
	done    bool
}

func newReadoutRichOutput(codec harness.Codec, ev harness.Event) *readoutRichOutput {
	return &readoutRichOutput{codec: codec, ev: ev, capture: &readoutRichCodec{Codec: codec}}
}

// flush writes once; it reports whether the rich lines were emitted.
func (o *readoutRichOutput) flush(stdout io.Writer, lines []string) bool {
	if o == nil || o.done {
		return false
	}
	o.done = true
	if len(lines) > 0 {
		ctx := strings.Join(append(append([]string(nil), o.capture.contexts...), lines...), "\n")
		if out, err := o.codec.EncodeOutput(o.ev, harness.HookOutput{Context: ctx}); err == nil && len(out) > 0 {
			fmt.Fprintln(stdout, string(out))
			return true
		}
	}
	_, _ = stdout.Write(o.tier.Bytes())
	return false
}

// readoutRichLines computes rich mode's lines for one call inside ctx.
func readoutRichLines(ctx context.Context, in harness.HookInput, dir string, c *shellcmd.Command) readoutRich {
	line, missed := readoutCompileLine(ctx, in, dir, c)
	r := readoutRich{missed: missed}
	if line != "" {
		r.lines = append(r.lines, line)
		r.kinds = append(r.kinds, readout.RecordCompileError)
	}
	if ctx.Err() != nil {
		return r
	}
	if line := readoutGuardLine(dir, c); line != "" {
		r.lines = append(r.lines, line)
		r.kinds = append(r.kinds, readoutRichGuardKind)
	}
	return r
}

// goCompileTool names the `go build` or `go vet` a command runs, "" when it
// runs neither.
func goCompileTool(c *shellcmd.Command) string {
	if c == nil || c.Unreadable {
		return ""
	}
	for _, u := range c.Units() {
		if u.Kind != shellcmd.KindSimple || u.Name() != "go" || len(u.Words) < 2 {
			continue
		}
		if sub := u.Words[1].Text; sub == "build" || sub == "vet" {
			return "go " + sub
		}
	}
	return ""
}

// readoutCompileLine is RO-24's line for a go build or go vet run: its
// errors as compile_error records grouped by root, the root a symbol the
// working tree changed placed at its definition, and the group touching a
// file this session edited first. missed names a join that did not answer.
func readoutCompileLine(ctx context.Context, in harness.HookInput, dir string, c *shellcmd.Command) (string, []string) {
	tool := goCompileTool(c)
	if tool == "" {
		return "", nil
	}
	text := in.Shell.Stdout
	if in.Shell.Stderr != "" {
		text += "\n" + in.Shell.Stderr
	}
	root := gitToplevelByStat(dir)
	norm := func(p string) string {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(dir, p)
		}
		if root != "" {
			if rel, ok := relativeToRoot(root, abs); ok {
				return filepath.ToSlash(rel)
			}
		}
		return strings.TrimPrefix(filepath.ToSlash(p), "./")
	}
	recs := readout.CompileErrors(text, norm)
	if len(recs) == 0 {
		return "", nil
	}
	var defs []readout.CompileDef
	var missed []string
	if root != "" {
		if diff, ok := workingTreeDiffErr(ctx, root); ok {
			for _, s := range testrun.ChangedGoSymbols(root, diff) {
				defs = append(defs, readout.CompileDef{Name: s.Name, File: s.File, Line: s.Line})
			}
		} else {
			missed = append(missed, "changed_symbols")
		}
	}
	now := readoutHookClock()
	edited := func(file string) bool {
		if root == "" {
			return false
		}
		abs := filepath.Join(root, filepath.FromSlash(file))
		for _, cand := range localCandidateRoots(dir) {
			if rel, ok := relativeToRoot(cand, abs); ok && fileRecentlyEdited(in.SessionID, rel, now) {
				return true
			}
		}
		return fileInEditSet(abs, in.Cwd, in.SessionID)
	}
	return readout.CompileErrorLine(tool, recs, defs, edited, readoutRichLineBudget), missed
}
