package harness

// codex_cost_test.go — PE-14's anchor (docs/design/progress-and-economics.md
// §5): Codex session files read by the same incremental fold the Stop hook
// gives Claude Code transcripts, on the harness port.
//
// The three fixtures under testdata/rollouts/codex are SANITIZED copies of
// real rollout files from ~/.codex/sessions (read-only, 2026-09-28): every
// message, tool argument, tool output, reasoning blob, working directory,
// git remote, rate-limit and account field was stripped; what stays is the
// envelope (timestamp, ordinal, type, payload.type), the model on each
// turn_context, the tool-call names, and the token_count usage numbers
// verbatim. They cover three vintages of one schema:
//
//	early.sanitized.jsonl        codex-cli 0.45.0   (2025-10): an info:null
//	                             token_count before the first response
//	modern.sanitized.jsonl       codex-cli 0.154.x  (2026-09): both tool-call
//	                             kinds, token_usage_record lines beside every
//	                             token_count
//	nonmonotone.sanitized.jsonl  codex-cli 0.98.0   (2026-02): an excerpt in
//	                             which the cumulative totals go backwards
//
// Expected numbers below come from an independent oracle over the same
// bytes (jq: the token_count event with the greatest input+output, cached
// input subtracted from input, function_call + custom_tool_call counted),
// not from the code under test.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rolloutFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", "rollouts", "codex", name)
}

// TestCodexSessionCost pins the mapping from a real Codex rollout to the
// cost fields outcome economics prices.
func TestCodexSessionCost(t *testing.T) {
	cases := []struct {
		file string
		want SessionCost
	}{
		// Codex's input_tokens INCLUDES cached_input_tokens (cached is
		// always <= input in all 327 real rollouts surveyed); Claude Code's
		// input_tokens excludes cache reads. TokensIn is the uncached part,
		// so the two harnesses' rows price the same way.
		{"modern.sanitized.jsonl", SessionCost{
			TokensIn: 67196, TokensOut: 3049, TokensCacheRead: 635136, TokensCacheCreate: 0,
			ToolCalls: 14, Model: "gpt-5.6-sol", Available: true,
		}},
		{"early.sanitized.jsonl", SessionCost{
			TokensIn: 896, TokensOut: 120, TokensCacheRead: 2048, TokensCacheCreate: 0,
			ToolCalls: 0, Model: "gpt-5-codex", Available: true,
		}},
		{"nonmonotone.sanitized.jsonl", SessionCost{
			TokensIn: 90199, TokensOut: 20752, TokensCacheRead: 851072, TokensCacheCreate: 0,
			ToolCalls: 10, Model: "gpt-5.3-codex", Available: true,
			Note: codexNoteNonMonotone,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			got, _ := codexSessionCost(rolloutFixture(t, tc.file), nil, codexReadMaxBytes)
			if got != tc.want {
				t.Fatalf("codexSessionCost(%s)\n got %+v\nwant %+v", tc.file, got, tc.want)
			}
		})
	}
}

// The exported port entry point is the same fold under the production read
// cap.
func TestCodexSessionCostExportedEntry(t *testing.T) {
	p := rolloutFixture(t, "modern.sanitized.jsonl")
	want, _ := codexSessionCost(p, nil, codexReadMaxBytes)
	got, fold := CodexSessionCost(p, nil)
	if got != want {
		t.Fatalf("CodexSessionCost = %+v, want %+v", got, want)
	}
	if fold.Offset == 0 {
		t.Fatal("the returned fold carries no offset to resume from")
	}
}

// TestCodexSessionCostIncrementalEqualsWhole: appending the rollout one
// chunk at a time (cut mid-line, as a live write is) and folding from the
// returned state after each append ends at the whole-file cost, and every
// intermediate cost equals the whole-file cost of the bytes written so far.
func TestCodexSessionCostIncrementalEqualsWhole(t *testing.T) {
	raw, err := os.ReadFile(rolloutFixture(t, "modern.sanitized.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(t.TempDir(), "rollout.jsonl")
	var fold *CodexCostFold
	for cut := 0; ; {
		next := cut + 997 // a stride that lands mid-line
		if next > len(raw) {
			next = len(raw)
		}
		f, err := os.OpenFile(live, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(raw[cut:next]); err != nil {
			t.Fatal(err)
		}
		f.Close()
		cut = next

		got, st := codexSessionCost(live, fold, codexReadMaxBytes)
		fold = &st

		whole := filepath.Join(t.TempDir(), "whole.jsonl")
		if err := os.WriteFile(whole, raw[:cut], 0o600); err != nil {
			t.Fatal(err)
		}
		want, _ := codexSessionCost(whole, nil, codexReadMaxBytes)
		if got != want {
			t.Fatalf("after %d of %d bytes: incremental\n got %+v\nwant %+v", cut, len(raw), got, want)
		}
		if cut == len(raw) {
			break
		}
	}
	if !fold.SawUsage || fold.ToolCalls != 14 {
		t.Fatalf("final fold = %+v, want usage seen and 14 tool calls", *fold)
	}
}

// TestCodexSessionCostByteCapResumes: a read that stops at the byte cap is
// Truncated, and repeated calls from the returned state converge on the
// whole-file cost — a 1 GB rollout costs many bounded reads, never one
// unbounded one.
func TestCodexSessionCostByteCapResumes(t *testing.T) {
	p := rolloutFixture(t, "modern.sanitized.jsonl")
	want, _ := codexSessionCost(p, nil, codexReadMaxBytes)

	var fold *CodexCostFold
	var got SessionCost
	sawTruncated := false
	for i := 0; i < 500; i++ {
		var st CodexCostFold
		got, st = codexSessionCost(p, fold, 4096)
		fold = &st
		if got.Truncated {
			sawTruncated = true
			continue
		}
		break
	}
	if !sawTruncated {
		t.Fatal("a 4 KiB cap over a 19 KB rollout never reported Truncated")
	}
	if got.Truncated || got != want {
		t.Fatalf("capped fold did not converge\n got %+v\nwant %+v", got, want)
	}
}

// TestCodexSessionCostOverlongLineDoesNotWedge: one line longer than the
// read cap (real rollouts reach 11 MB) is skipped and counted, never a
// permanent stall at the same offset, and the usage after it is still read.
func TestCodexSessionCostOverlongLineDoesNotWedge(t *testing.T) {
	usage := func(in, cached, out int) string {
		return `{"timestamp":"2026-09-28T00:00:00Z","ordinal":1,"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":` +
			itoa(in) + `,"cached_input_tokens":` + itoa(cached) + `,"cache_write_input_tokens":0,"output_tokens":` + itoa(out) +
			`,"reasoning_output_tokens":0,"total_tokens":` + itoa(in+out) + `}}}}` + "\n"
	}
	giant := `{"timestamp":"2026-09-28T00:00:00Z","ordinal":2,"type":"response_item","payload":{"type":"custom_tool_call_output","output":"` +
		strings.Repeat("x", 20000) + `"}}` + "\n"
	body := usage(100, 40, 10) + giant + usage(300, 100, 30)
	p := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var fold *CodexCostFold
	var got SessionCost
	for i := 0; i < 50; i++ {
		var st CodexCostFold
		got, st = codexSessionCost(p, fold, 4096)
		fold = &st
		if !got.Truncated {
			break
		}
	}
	if got.Truncated {
		t.Fatalf("still truncated after 50 capped reads: the over-long line wedged the fold: %+v", got)
	}
	want := SessionCost{TokensIn: 200, TokensOut: 30, TokensCacheRead: 100, Available: true, SkippedLines: 1}
	want.Note = codexNoteSkipped
	if got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// TestCodexSessionCostNoUsageIsNotZero: a rollout that never carried usage
// is Available=false with every count 0 and a reason — never a fabricated
// zero indistinguishable from a measured zero-cost session (law 12) — and
// its tool calls are still counted.
func TestCodexSessionCostNoUsageIsNotZero(t *testing.T) {
	body := `{"timestamp":"2026-09-28T00:00:00Z","ordinal":0,"type":"session_meta","payload":{"id":"s"}}` + "\n" +
		`{"timestamp":"2026-09-28T00:00:01Z","ordinal":1,"type":"event_msg","payload":{"type":"token_count","info":null}}` + "\n" +
		`{"timestamp":"2026-09-28T00:00:02Z","ordinal":2,"type":"response_item","payload":{"type":"function_call","name":"exec"}}` + "\n"
	p := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := codexSessionCost(p, nil, codexReadMaxBytes)
	want := SessionCost{ToolCalls: 1, Note: codexNoteNoUsage}
	if got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

// TestCodexSessionCostOutputsAndProviderSearchesAreNotToolCalls: only the
// client-side invocations (function_call, custom_tool_call) count; their
// *_output lines, provider-side searches and messages that merely mention
// the words do not.
func TestCodexSessionCostOutputsAndProviderSearchesAreNotToolCalls(t *testing.T) {
	lines := []string{
		`{"timestamp":"t","ordinal":1,"type":"response_item","payload":{"type":"function_call","name":"exec"}}`,
		`{"timestamp":"t","ordinal":2,"type":"response_item","payload":{"type":"function_call_output","output":"function_call"}}`,
		`{"timestamp":"t","ordinal":3,"type":"response_item","payload":{"type":"custom_tool_call","name":"js"}}`,
		`{"timestamp":"t","ordinal":4,"type":"response_item","payload":{"type":"custom_tool_call_output","output":"\"custom_tool_call\""}}`,
		`{"timestamp":"t","ordinal":5,"type":"response_item","payload":{"type":"web_search_call"}}`,
		`{"timestamp":"t","ordinal":6,"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"\"type\":\"function_call\""}]}}`,
	}
	p := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := codexSessionCost(p, nil, codexReadMaxBytes)
	if got.ToolCalls != 2 {
		t.Fatalf("ToolCalls = %d, want 2 (one function_call, one custom_tool_call)", got.ToolCalls)
	}
}

// TestCodexSessionCostUnreadable: a missing path, a directory and a symlink
// (O_NOFOLLOW, the transcript hardening the Claude fold keeps) all report
// the fixed unavailable note and change nothing.
func TestCodexSessionCostUnreadable(t *testing.T) {
	dir := t.TempDir()
	real := rolloutFixture(t, "early.sanitized.jsonl")
	link := filepath.Join(dir, "link.jsonl")
	abs, err := filepath.Abs(real)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(abs, link); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{"missing": filepath.Join(dir, "nope.jsonl"), "directory": dir, "symlink": link, "empty path": ""} {
		got, fold := codexSessionCost(p, nil, codexReadMaxBytes)
		want := SessionCost{Note: codexNoteUnreadable}
		if got != want || fold.Offset != 0 {
			t.Errorf("%s: got %+v (offset %d), want %+v", name, got, fold.Offset, want)
		}
	}
}

// TestCodexSessionCostRestartsOnReplacedFile: a stored fold whose offset
// lies past the file's end (a truncated or replaced rollout) is discarded
// and the file is folded from 0, like the Claude stamp's identity check.
func TestCodexSessionCostRestartsOnReplacedFile(t *testing.T) {
	raw, err := os.ReadFile(rolloutFixture(t, "modern.sanitized.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, fold := codexSessionCost(p, nil, codexReadMaxBytes)

	small, err := os.ReadFile(rolloutFixture(t, "early.sanitized.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(small)) >= fold.Offset {
		t.Fatalf("test premise: replacement (%d B) must be shorter than the folded offset (%d)", len(small), fold.Offset)
	}
	q := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(q, small, 0o600); err != nil {
		t.Fatal(err)
	}
	want, _ := codexSessionCost(q, nil, codexReadMaxBytes)
	got, _ := codexSessionCost(q, &fold, codexReadMaxBytes)
	if got != want {
		t.Fatalf("stale fold leaked into a replaced file\n got %+v\nwant %+v", got, want)
	}
}
