package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// fixtureEvents maps a fixture stem to the canonical event it documents.
var fixtureEvents = map[string]Event{
	"session-start":        EventSessionStart,
	"session-end":          EventSessionEnd,
	"pre-compact":          EventPreCompact,
	"pre-tool":             EventPreTool,
	"pre-tool-apply-patch": EventPreTool,
	"post-tool":            EventPostTool,
	"stop":                 EventStop,
}

// readFixture returns a fixture's header line (the source citation) and
// its JSON body. Every fixture opens with `// source: <url> … <date>` so
// the contract each byte came from is named in the file itself (HP-23).
func readFixture(t *testing.T, path string) (header string, body []byte) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	nl := strings.IndexByte(string(raw), '\n')
	if nl < 0 {
		t.Fatalf("%s: no header line", path)
	}
	header = string(raw[:nl])
	if !strings.HasPrefix(header, "// source: ") || !strings.Contains(header, "http") || !strings.Contains(header, "2026-") {
		t.Errorf("%s: header must cite a source URL and a date: %q", path, header)
	}
	body = raw[nl+1:]
	if !json.Valid(body) {
		t.Fatalf("%s: body is not valid JSON", path)
	}
	return header, body
}

// TestCodecsRoundTripDocumentedFixtures pins HP-23: every wired non-Claude
// codec decodes each documented stdin fixture into the neutral envelope,
// re-encodes the documented stdout fixtures in the harness's own shape,
// and never emits a deny/permissionDecision — for any event, any output.
func TestCodecsRoundTripDocumentedFixtures(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("testdata", "codecs", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatal("no codec fixture directories")
	}
	sort.Strings(dirs)
	seen := map[string]bool{}
	for _, dir := range dirs {
		id := filepath.Base(dir)
		codec, ok := CodecFor(id)
		if !ok {
			t.Errorf("%s: fixtures exist but no codec is wired", id)
			continue
		}
		seen[id] = true
		files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
		sort.Strings(files)
		if len(files) == 0 {
			t.Errorf("%s: no fixtures", id)
		}
		for _, f := range files {
			name := filepath.Base(f)
			if strings.HasSuffix(name, ".live.in.json") {
				// A LIVE capture, not a documented fixture: its provenance
				// is a JSON header inside the object (version + date), and
				// codec_codex_live_test.go replays and checks it. Holding it
				// to the documented-fixture header comment would force a
				// hand-edit of bytes whose whole value is being unedited.
				continue
			}
			_, body := readFixture(t, f)
			switch {
			case strings.HasSuffix(name, ".in.json"):
				stem := strings.TrimSuffix(name, ".in.json")
				ev, ok := fixtureEvents[stem]
				if !ok {
					t.Errorf("%s: unknown fixture stem %q", f, stem)
					continue
				}
				in, err := codec.DecodeInput(ev, body)
				if err != nil {
					t.Errorf("%s: decode: %v", f, err)
					continue
				}
				if in.Event != ev || in.SessionID == "" || in.Cwd == "" {
					t.Errorf("%s: decoded envelope lacks event/session/cwd: %+v", f, in)
				}
				checkDecodedFields(t, f, id, stem, in)
				// Whatever the harness sent, the codec answers in JSON or in
				// silence — never in prose (Codex: plain text is invalid on
				// Stop; Gemini: a non-JSON byte means Allow + systemMessage).
				out, err := codec.EncodeOutput(ev, HookOutput{Context: "x"})
				if err != nil {
					t.Errorf("%s: encode after decode: %v", f, err)
				} else if out != nil && !json.Valid(out) {
					t.Errorf("%s: encoded output is not JSON: %s", f, out)
				}
			case strings.HasSuffix(name, ".out.json"):
				stem := strings.TrimSuffix(name, ".out.json")
				ev := fixtureEvents[stem]
				checkEncodedShape(t, f, codec, ev, body)
			case strings.HasSuffix(name, ".deny.json"):
				// The documented deny shape is what fathom must never produce.
				stem := strings.TrimSuffix(name, ".deny.json")
				ev := fixtureEvents[stem]
				for _, o := range []HookOutput{{Context: "ctx"}, {Block: true, Reason: "r"}} {
					got, err := codec.EncodeOutput(ev, o)
					if err != nil {
						t.Fatal(err)
					}
					if string(got) == strings.TrimSpace(string(body)) {
						t.Errorf("%s: codec reproduced the deny fixture", f)
					}
				}
			default:
				t.Errorf("%s: fixture name must end in .in.json, .out.json or .deny.json", f)
			}
		}
	}
	for _, id := range []string{Codex, GeminiCLI} {
		if !seen[id] {
			t.Errorf("%s: no fixture directory", id)
		}
	}

	// §5.3 inject-only, for every wired codec, every event, both outputs.
	for _, id := range IDs() {
		codec, ok := CodecFor(id)
		if !ok {
			continue
		}
		d, _ := Lookup(id)
		for _, b := range d.Hooks {
			for _, o := range []HookOutput{{Context: "x"}, {Block: true, Reason: "r"}} {
				got, err := codec.EncodeOutput(b.Event, o)
				if err != nil {
					t.Errorf("%s/%s: %v", id, b.Event, err)
					continue
				}
				s := string(got)
				if strings.Contains(s, "permissionDecision") || strings.Contains(s, `"deny"`) || strings.Contains(s, "updatedInput") {
					t.Errorf("%s/%s emits a decision: %s", id, b.Event, s)
				}
			}
		}
	}
}

// checkDecodedFields pins the per-fixture facts the envelope must carry.
func checkDecodedFields(t *testing.T, f, id, stem string, in HookInput) {
	t.Helper()
	switch {
	case id == Codex && stem == "pre-tool":
		if in.Tool != ToolShell || in.Command != "npm test" || in.FilePath != "" {
			t.Errorf("%s: Bash decoded wrong: %+v", f, in)
		}
	case id == Codex && stem == "pre-tool-apply-patch":
		if in.Tool != ToolFileEdit || in.FilePath != "/workspace/internal/store/store.go" || !strings.HasPrefix(in.Command, "*** Begin Patch") {
			t.Errorf("%s: apply_patch decoded wrong: %+v", f, in)
		}
	case id == Codex && stem == "post-tool":
		if in.Tool != ToolFileEdit || in.FilePath != "/workspace/docs/new.md" {
			t.Errorf("%s: apply_patch Add File decoded wrong: %+v", f, in)
		}
	case id == Codex && stem == "session-end":
		if in.Reason != "other" || in.TranscriptPath != "/workspace/.codex/rollout.jsonl" {
			t.Errorf("%s: SessionEnd decoded wrong: %+v", f, in)
		}
	case id == GeminiCLI && (stem == "pre-tool" || stem == "post-tool"):
		if in.Tool != ToolFileEdit || in.FilePath != "/home/user/project/src/main.go" {
			t.Errorf("%s: file edit decoded wrong: %+v", f, in)
		}
	case stem == "session-start":
		if in.Source != "startup" {
			t.Errorf("%s: source = %q", f, in.Source)
		}
	case stem == "pre-compact":
		if in.Trigger != "auto" {
			t.Errorf("%s: trigger = %q", f, in.Trigger)
		}
	case stem == "stop":
		if id == Codex && !in.ReEntrant {
			t.Errorf("%s: stop_hook_active not decoded: %+v", f, in)
		}
	}
}

// checkEncodedShape re-encodes the fixture's own additionalContext (or
// block reason) and compares the part of the fixture fathom emits:
// hookSpecificOutput verbatim, or the flat decision/reason pair.
func checkEncodedShape(t *testing.T, f string, codec Codec, ev Event, body []byte) {
	t.Helper()
	var want map[string]any
	if err := json.Unmarshal(body, &want); err != nil {
		t.Fatal(err)
	}
	if hso, ok := want["hookSpecificOutput"].(map[string]any); ok {
		ctx, _ := hso["additionalContext"].(string)
		got, err := codec.EncodeOutput(ev, HookOutput{Context: ctx})
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		var gotM map[string]any
		if err := json.Unmarshal(got, &gotM); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		gotHSO, _ := gotM["hookSpecificOutput"].(map[string]any)
		if gotHSO["hookEventName"] != hso["hookEventName"] || gotHSO["additionalContext"] != ctx {
			t.Errorf("%s: hookSpecificOutput differs:\n got %v\nwant %v", f, gotHSO, hso)
		}
		if _, leaked := gotM["decision"]; leaked {
			t.Errorf("%s: context output carries a decision: %s", f, got)
		}
		return
	}
	// Flat decision/reason (Stop): only the --block verdict renders it.
	reason, _ := want["reason"].(string)
	got, err := codec.EncodeOutput(ev, HookOutput{Block: true, Reason: reason})
	if err != nil {
		t.Fatalf("%s: %v", f, err)
	}
	var gotM map[string]any
	if err := json.Unmarshal(got, &gotM); err != nil {
		t.Fatalf("%s: %v", f, err)
	}
	if gotM["decision"] != want["decision"] || gotM["reason"] != reason {
		t.Errorf("%s: block verdict differs:\n got %v\nwant %v", f, gotM, want)
	}
}

func TestCodexCodecCapsAdditionalContextWithNamedCut(t *testing.T) {
	long := strings.Repeat("é", CodexAdditionalContextCap) // 2 bytes each: twice the cap
	got := CapCodexContext(long)
	if len(got) > CodexAdditionalContextCap {
		t.Errorf("capped payload is %d bytes, cap %d", len(got), CodexAdditionalContextCap)
	}
	if !strings.Contains(got, "[fathom: cut ") || !strings.Contains(got, "additionalContext limit]") {
		t.Errorf("the cut is not named: …%s", got[len(got)-80:])
	}
	if !utf8.ValidString(got) {
		t.Error("cut split a rune")
	}
	if CapCodexContext("short") != "short" {
		t.Error("an under-cap payload must pass through untouched")
	}
	// The cap reaches the wire: the encoder's additionalContext is capped.
	out, err := CodexCodec().EncodeOutput(EventSessionStart, HookOutput{Context: long})
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		HSO struct {
			Ctx string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &m); err != nil || len(m.HSO.Ctx) > CodexAdditionalContextCap {
		t.Errorf("encoded context is %d bytes (err %v)", len(m.HSO.Ctx), err)
	}
	if CapCodexContext(long) != CapCodexContext(long) {
		t.Error("CapCodexContext is not deterministic")
	}
}

func TestApplyPatchFirstFile(t *testing.T) {
	cases := map[string]string{
		"*** Begin Patch\n*** Update File: a/b.go\n@@\n*** End Patch":                     "a/b.go",
		"*** Begin Patch\n*** Add File: new.md\n+x\n*** Update File: b.go\n*** End Patch": "new.md",
		"*** Begin Patch\r\n*** Delete File: gone.txt\r\n*** End Patch":                   "gone.txt",
		"echo hi":                      "",
		"":                             "",
		"*** Update File:   spaced.go": "spaced.go",
	}
	for in, want := range cases {
		if got := ApplyPatchFirstFile(in); got != want {
			t.Errorf("ApplyPatchFirstFile(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGeminiCodecNeverDeniesAndUsesSystemMessageWhereContextIsUnshown(t *testing.T) {
	c := GeminiCodec()
	got, err := c.EncodeOutput(EventStop, HookOutput{Block: true, Reason: "T-1 fails"})
	if err != nil || string(got) != `{"systemMessage":"T-1 fails"}` {
		t.Errorf("stop --block on Gemini = %s, %v; want an advisory systemMessage", got, err)
	}
	got, _ = c.EncodeOutput(EventPreTool, HookOutput{Context: "guard"})
	if string(got) != `{"systemMessage":"guard"}` {
		t.Errorf("BeforeTool context = %s; additionalContext there is UNSHOWN", got)
	}
	got, _ = c.EncodeOutput(EventPostTool, HookOutput{Context: "obl"})
	if string(got) != `{"hookSpecificOutput":{"additionalContext":"obl","hookEventName":"AfterTool"}}` {
		t.Errorf("AfterTool context = %s", got)
	}
	if got, err := c.EncodeOutput(EventSessionEnd, HookOutput{}); err != nil || got != nil {
		t.Errorf("silence = %s, %v", got, err)
	}
	if _, err := c.EncodeOutput(Event("nope"), HookOutput{Context: "x"}); err == nil {
		t.Error("unknown event must error")
	}
	// Malformed stdin is an error the verbs turn into silence.
	if _, err := c.DecodeInput(EventPreTool, []byte("{")); err == nil {
		t.Error("malformed stdin must error")
	}
}

func TestCodexCodecDecodesNonObjectToolInput(t *testing.T) {
	// MCP / local function tools send their arguments as tool_input; a
	// non-object (or an object without command) must not fail the envelope.
	for _, raw := range []string{
		`{"session_id":"s","cwd":"/w","tool_name":"mcp__fs__read","tool_input":["a"]}`,
		`{"session_id":"s","cwd":"/w","tool_name":"update_plan","tool_input":{"plan":[]}}`,
		`{"session_id":"s","cwd":"/w","tool_name":"Bash","tool_input":"raw"}`,
	} {
		in, err := CodexCodec().DecodeInput(EventPreTool, []byte(raw))
		if err != nil {
			t.Errorf("%s: %v", raw, err)
		}
		if in.SessionID != "s" || in.FilePath != "" {
			t.Errorf("%s: %+v", raw, in)
		}
	}
	// A relative patch path resolves against the session cwd; an absolute
	// one is kept.
	in, _ := CodexCodec().DecodeInput(EventPostTool, []byte(`{"session_id":"s","cwd":"/w","tool_name":"apply_patch","tool_input":{"command":"*** Update File: /abs/x.go\n"}}`))
	if in.FilePath != "/abs/x.go" {
		t.Errorf("absolute patch path: %+v", in)
	}
}

// TestWiredCodecsStartTheDemotionClock pins HP-16's wiring: both wave-2
// codecs carry a landing date, nothing is due at landing, and both are
// due 61 days later unless a real session verifies them first.
func TestWiredCodecsStartTheDemotionClock(t *testing.T) {
	landed := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	// Gemini CLI is still unverified: its clock runs, and 61 days after
	// landing it demotes unless a real session verifies it first.
	for _, id := range []string{GeminiCLI} {
		d, _ := Lookup(id)
		if d.Codec == "" || d.CodecLanded != "2026-09-02" || !d.Unverified() {
			t.Errorf("%s: codec=%q landed=%q verified=%q", id, d.Codec, d.CodecLanded, d.VerifiedAgainst)
		}
		if due, _ := DemotionDue(d, landed); due {
			t.Errorf("%s: due at landing", id)
		}
		if due, _ := DemotionDue(d, landed.Add(61*24*time.Hour)); !due {
			t.Errorf("%s: not due after 61 days", id)
		}
		if _, ok := d.InitHookFile(); !ok {
			t.Errorf("%s: no project hook file for init to write", id)
		}
	}
	// Codex's clock STOPPED: HP-34's live session verified it, which is the
	// only thing that stops a demotion clock. Pinned because a regression
	// to "unverified" would silently re-arm a 60-day countdown that has
	// already been satisfied, and `init --harness=codex` would start
	// refusing a harness that demonstrably works.
	cx, _ := Lookup(Codex)
	if cx.Unverified() {
		t.Fatal("codex was verified by a live session; verified_against must say so")
	}
	if due, reason := DemotionDue(cx, landed.Add(400*24*time.Hour)); due {
		t.Errorf("a verified descriptor never demotes: %s", reason)
	}
	if EffectiveTier(cx, landed.Add(400*24*time.Hour)) != "A" {
		t.Error("codex must stay tier A once verified")
	}
	for _, d := range DataOnlyDescriptors() {
		if d.Codec != "" || !d.HasHooks() {
			t.Errorf("%s is not data-only", d.ID)
		}
		if _, ok := d.InitHookFile(); ok {
			t.Errorf("%s: a data row must not name a hook file init writes", d.ID)
		}
	}
	if ids := func() []string {
		var out []string
		for _, d := range DataOnlyDescriptors() {
			out = append(out, d.ID)
		}
		return out
	}(); strings.Join(ids, ",") != "cursor,copilot" {
		t.Errorf("DataOnlyDescriptors = %v", ids)
	}
}

func TestLadderRowsAndTable(t *testing.T) {
	rows := LadderRows()
	want := []string{"windsurf", "opencode", "amp", "zed", "junie", "warp", "continue", "aider"}
	if len(rows) != len(want) {
		t.Fatalf("%d rows, want %d", len(rows), len(want))
	}
	for i, r := range rows {
		if r.ID != want[i] {
			t.Errorf("row %d = %s, want %s", i, r.ID, want[i])
		}
		if r.Tier == "A" || r.Tier == "" {
			t.Errorf("%s: ladder rows are tier B/C/D, got %q", r.ID, r.Tier)
		}
		if !strings.HasPrefix(r.Injection, "no injection here") {
			t.Errorf("%s: the honest line must open with 'no injection here': %q", r.ID, r.Injection)
		}
		if !strings.HasPrefix(r.Source, "http") || r.MCPAddLine == "" || r.Instruction == "" {
			t.Errorf("%s: missing source/mcp/instruction: %+v", r.ID, r)
		}
		if _, ok := Lookup(r.ID); ok {
			t.Errorf("%s: a ladder row must not also be a descriptor", r.ID)
		}
	}
	table := RenderLadderTable("http://127.0.0.1:7575")
	if strings.Contains(table, "{{url}}") || !strings.Contains(table, "http://127.0.0.1:7575/rpc/agent") {
		t.Errorf("url not filled:\n%s", table)
	}
	if lines := strings.Split(strings.TrimSpace(table), "\n"); len(lines) != 2+len(want) {
		t.Errorf("%d lines", len(lines))
	}
	if RenderLadderTable("") != RenderLadderTable("") {
		t.Error("not deterministic")
	}
}
