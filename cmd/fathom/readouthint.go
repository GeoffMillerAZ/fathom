package main

// readouthint.go: H8's Read outline hint (docs/design/readouts.md section
// 20, docs/plans/readouts.md): `fathom readout-hook --event pre-read` on
// PreToolUse(Read), its own hook entry beside the discovery observer's,
// which stays counting-only. For a Read with no offset or limit of a source
// file over 200 lines that this session has not edited, one line: the
// file's top-level symbols with line ranges from the outline join over the
// file on disk (fathom's own extractors in process, never the index, so
// never stale), then the Read offset and limit recipe. The read runs
// unchanged: the hint is additionalContext only, never a permission
// decision or a changed input, and it claims no saving. Off by default
// behind [hooks] read_outline_hint; any error exits 0 and emits nothing.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// readOutlineHintChannel is the hook log channel of the hint's rows (no
// table: the rows ride the existing hook log).
const readOutlineHintChannel = "read-outline-hint"

// readOutlineHintMinLines is the floor: a file of at most this many lines
// on disk is read whole without a hint.
const readOutlineHintMinLines = 200

// readOutlineHintMaxSymbols caps the symbols the line names; the rest are
// counted (`and N more`). Twelve `name A-B` pairs keep the whole line near
// 45 words, inside two of I35's 28-word rule slots and far under its
// 320-token ceiling.
const readOutlineHintMaxSymbols = 12

// readOutlineHintFileCap bounds the file read; a larger file gets no hint.
const readOutlineHintFileCap = 4 << 20

// readOutlineHintEditBudget bounds the edit check (the edit set, then a
// read-only git status); a test widens it.
var readOutlineHintEditBudget = 300 * time.Millisecond

// readOutlineHintJoin is the outline join; a test injects a slow one.
var readOutlineHintJoin = readout.Outline

// readOutlineHintDeadline is the join's deadline; past it the hint is
// silent and the row says missed.
var readOutlineHintDeadline = readout.OutlineDeadline

// readOutlineHintClock is the hint's clock; a test may pin it.
var readOutlineHintClock = time.Now

// outlineJoinResult is the join's two results, carried across the goroutine.
type outlineJoinResult struct {
	lines  []string
	missed bool
}

// outlineSymbol is one top-level symbol of the join's lines.
type outlineSymbol struct {
	name string
	a, b int
}

// emitReadOutlineHint prints the hint as PreToolUse context when there is
// one. It always returns 0.
func emitReadOutlineHint(codec harness.Codec, raw []byte, stdout io.Writer) int {
	line := readOutlineHint(codec, raw)
	if line == "" {
		return 0
	}
	out, err := codec.EncodeOutput(harness.EventPreTool, harness.HookOutput{Context: line})
	if err != nil || len(out) == 0 {
		return 0
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

// readOutlineHint returns the hint line for one PreToolUse(Read) envelope,
// or "" for silence. Cheapest gates first: the envelope, a range, the
// language, the switch, the file on disk and its line count, the session's
// edits; then the join inside its deadline (the Read waits at most that long). Every call that reaches the
// join logs one row on readOutlineHintChannel: the file's sha, its lines,
// its symbols, the ms, the language and, on a deadline miss, missed.
func readOutlineHint(codec harness.Codec, raw []byte) string {
	start := readOutlineHintClock()
	if codec.ID() != harness.ClaudeCode {
		return "" // only Claude Code's codec reads a Read's range
	}
	in, err := codec.DecodeInput(harness.EventPreTool, raw)
	if err != nil || in.Tool != harness.ToolDiscoveryRead || in.FilePath == "" || in.ReadRange {
		return ""
	}
	lang := connector.LanguageForPath(in.FilePath)
	if lang == core.LangUnclassified {
		return ""
	}
	// The Read acts in its file's repository (hookCallRoot), which inside a
	// subagent's scratch clone is not the session's project (O1230).
	cfg, ok := hooksConfigAt(hookCallRoot("", in.FilePath, in.Cwd, eventProjectDir(codec, in)), hookOptInReadOutlineHint)
	if !ok || !cfg.Hooks.ReadOutlineHint {
		return ""
	}
	abs := resolveBashDir(in.Cwd, in.FilePath)
	text, ok := readOutlineHintFile(abs)
	if !ok {
		return ""
	}
	lines := strings.Count(text, "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		lines++
	}
	if lines <= readOutlineHintMinLines {
		return ""
	}
	ectx, ecancel := context.WithTimeout(context.Background(), readOutlineHintEditBudget)
	edited, known := readoutFileEdited(ectx, in, in.Cwd, abs)
	ecancel()
	if edited || !known {
		return ""
	}

	j0 := readOutlineHintClock()
	jctx, jcancel := context.WithTimeout(context.Background(), readOutlineHintDeadline)
	// The join takes no context inside its extraction, so it runs abandonable:
	// at the deadline the hook returns whatever the extraction is doing and
	// the one-shot process exit is the cancellation.
	join := readOutlineHintJoin // read here: the abandoned goroutine must not touch the var
	res, jerr := tenancy.RunUntilDone(jctx, func() outlineJoinResult {
		lines, miss := join(jctx, abs, text)
		return outlineJoinResult{lines, miss}
	})
	jcancel()
	out, missed := res.lines, res.missed
	if jerr != nil {
		out, missed = []string{readout.OutlineDeadlineLine}, true
	}
	joinMS := readOutlineHintClock().Sub(j0)
	deadline := missed && len(out) == 1 && out[0] == readout.OutlineDeadlineLine
	var syms []outlineSymbol
	total := 0
	if !missed {
		syms, total = outlineSymbols(out)
	}
	line := ""
	if len(syms) > 0 {
		line = readOutlineHintLine(filepath.Base(abs), lines, syms, total)
	}
	meta := map[string]string{
		"event":    "pre-read",
		"lang":     lang,
		"lines":    strconv.Itoa(lines),
		"symbols":  strconv.Itoa(total),
		"file_sha": sha256Hex(text),
		"join_ms":  strconv.FormatInt(joinMS.Milliseconds(), 10),
		"ms":       strconv.FormatInt(readOutlineHintClock().Sub(start).Milliseconds(), 10),
		"emitted":  boolStr(line != ""),
	}
	if deadline {
		meta["missed"] = "true"
	} else if missed {
		meta["reason"] = "no outline"
	}
	if in.ToolUseID != "" {
		meta["tool_use_id"] = in.ToolUseID
	}
	kind := "served"
	if line == "" {
		kind = "silent"
	}
	emitBeat(readOutlineHintChannel, kind, firingMeta(meta, in.SessionID, in.FilePath))
	return line
}

// readOutlineHintFile reads a regular file of at most
// readOutlineHintFileCap bytes; ok is false otherwise.
func readOutlineHintFile(path string) (string, bool) {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > readOutlineHintFileCap {
		return "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, readOutlineHintFileCap+1))
	if err != nil || len(b) > readOutlineHintFileCap {
		return "", false
	}
	return string(b), true
}

// outlineSymbols reads the outline join's lines (readout.Outline: a header,
// then `  A-B kind name` per top-level symbol in line order, then
// `  +N more symbols` past its own cap): the symbols it lists and how many
// the file has.
func outlineSymbols(lines []string) ([]outlineSymbol, int) {
	if len(lines) < 2 {
		return nil, 0 // no top-level symbols found
	}
	var syms []outlineSymbol
	total := 0
	for _, l := range lines[1:] {
		t := strings.TrimSpace(l)
		if rest, ok := strings.CutPrefix(t, "+"); ok {
			if n, err := strconv.Atoi(strings.TrimSuffix(rest, " more symbols")); err == nil {
				total += n
			}
			continue
		}
		f := strings.Fields(t)
		if len(f) < 3 {
			continue
		}
		a, b, ok := outlineSpan(f[0])
		if !ok {
			continue
		}
		syms = append(syms, outlineSymbol{name: strings.Join(f[2:], " "), a: a, b: b})
		total++
	}
	return syms, total
}

// outlineSpan parses `A-B` or `A`.
func outlineSpan(s string) (int, int, bool) {
	as, bs, found := strings.Cut(s, "-")
	a, err := strconv.Atoi(as)
	if err != nil || a <= 0 {
		return 0, 0, false
	}
	if !found {
		return a, a, true
	}
	b, err := strconv.Atoi(bs)
	if err != nil || b < a {
		return 0, 0, false
	}
	return a, b, true
}

// readOutlineHintLine is the one line: the file, its line count, at most
// readOutlineHintMaxSymbols symbols with their ranges, the count past the
// cap, and the Read offset and limit recipe worked for the first symbol.
func readOutlineHintLine(base string, lines int, syms []outlineSymbol, total int) string {
	shown := syms
	if len(shown) > readOutlineHintMaxSymbols {
		shown = shown[:readOutlineHintMaxSymbols]
	}
	parts := make([]string, 0, len(shown))
	for _, s := range shown {
		if s.a == s.b {
			parts = append(parts, fmt.Sprintf("%s %d", s.name, s.a))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %d-%d", s.name, s.a, s.b))
	}
	more := ""
	if total > len(shown) {
		more = fmt.Sprintf(" and %d more", total-len(shown))
	}
	first := shown[0]
	return fmt.Sprintf("outline of %s (%d lines, from the file on disk): %s%s. To read one symbol, pass Read offset=<start> limit=<end-start+1> (%s: offset=%d limit=%d).",
		base, lines, strings.Join(parts, ", "), more, first.name, first.a, first.b-first.a+1)
}
