package harness

// codec_codex_live_test.go — HP-33's anchor. Every other Codex fixture in
// testdata/codecs/codex was written FROM THE DOCUMENTATION: it proves the
// codec matches what the docs say, which is a different and much weaker
// claim than "the codec matches what the binary sends".
//
// These five fixtures are the bytes codex-cli 0.153.1 actually wrote to
// fathom's hook stdin during one headless `codex exec --json` session on
// this workstation (2026-09-12), captured by temporarily wrapping the
// wired command with a tee script under gitignored workspace/. They are
// what promotes the descriptor's verified_against from "unverified" to a
// dated version string, and if a future Codex release changes the
// envelope, THIS is the test that fails.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodexHooksFixtureFromLiveSession replays each live capture through
// CodexCodec and asserts the fields the hook verbs actually consume.
func TestCodexHooksFixtureFromLiveSession(t *testing.T) {
	const sessionID = "01a0979a-f066-7a31-a024-fa83690b18ce"
	codec := CodexCodec()

	cases := []struct {
		file  string
		event Event
		check func(*testing.T, HookInput)
	}{
		{"session-start.live.in.json", EventSessionStart, func(t *testing.T, in HookInput) {
			if in.Source != "startup" {
				t.Errorf("source = %q, want startup (the primer's startup/clear gate reads it)", in.Source)
			}
			if in.Cwd == "" || in.TranscriptPath == "" {
				t.Errorf("cwd/transcript_path must survive the decode: %+v", in)
			}
		}},
		{"pre-tool.live.in.json", EventPreTool, func(t *testing.T, in HookInput) {
			if in.Tool != ToolFileEdit || in.ToolName != "apply_patch" {
				t.Errorf("tool = %q/%s, want apply_patch/file-edit", in.ToolName, in.Tool)
			}
			// The documented assumption, now measured: apply_patch reports
			// the WHOLE patch in tool_input.command and never a file_path,
			// so the first `*** Add/Update/Delete File:` header is the only
			// way to learn which file the guard is about.
			if !strings.HasPrefix(in.Command, "*** Begin Patch") {
				t.Errorf("command = %q, want the apply_patch body", in.Command)
			}
			if !strings.HasSuffix(in.FilePath, "workspace/tmp/capture/smoke.txt") {
				t.Errorf("file_path = %q, want the patch's first file resolved against cwd", in.FilePath)
			}
			if !filepath.IsAbs(in.FilePath) {
				t.Errorf("file_path = %q, want an absolute path (the verbs resolve obligations by absolute path)", in.FilePath)
			}
		}},
		{"post-tool.live.in.json", EventPostTool, func(t *testing.T, in HookInput) {
			if in.Tool != ToolFileEdit || !strings.HasSuffix(in.FilePath, "smoke.txt") {
				t.Errorf("post-tool = %+v", in)
			}
		}},
		{"stop.live.in.json", EventStop, func(t *testing.T, in HookInput) {
			if in.ReEntrant {
				t.Errorf("stop_hook_active = true on a first stop: %+v", in)
			}
		}},
		{"session-end.live.in.json", EventSessionEnd, func(t *testing.T, in HookInput) {
			// The descriptor's honesty note, confirmed by the binary: Codex
			// reports "other", and the deposit verb deposits only on
			// clear|logout|exit|prompt_input_exit — so nothing is deposited
			// on Codex today, and the stanza says `fathom thread note`.
			if in.Reason != "other" {
				t.Errorf("reason = %q, want other (the descriptor's CapDepositSessionEnd note depends on it)", in.Reason)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "codecs", "codex", tc.file))
			if err != nil {
				t.Fatalf("reading the live capture: %v", err)
			}
			// The header keys the fixture carries (_source, _codex_cli_version,
			// …) are provenance, not envelope: an unknown field must be
			// ignored by the codec, which is itself part of the contract.
			var hdr map[string]any
			if err := json.Unmarshal(raw, &hdr); err != nil {
				t.Fatalf("the fixture must be JSON: %v", err)
			}
			for _, k := range []string{"_source", "_codex_cli_version", "_captured"} {
				if _, ok := hdr[k]; !ok {
					t.Errorf("fixture %s is missing the %s provenance header — a live capture with no version and date is indistinguishable from an invented one", tc.file, k)
				}
			}
			if v, _ := hdr["_codex_cli_version"].(string); v != "0.153.1" {
				t.Errorf("fixture %s names codex-cli %q; update the descriptor's verified_against in the same change", tc.file, v)
			}
			in, err := codec.DecodeInput(tc.event, raw)
			if err != nil {
				t.Fatalf("CodexCodec could not decode a real %s envelope: %v", tc.event, err)
			}
			if in.SessionID != sessionID {
				t.Errorf("session_id = %q, want %q (all five captures are one session)", in.SessionID, sessionID)
			}
			tc.check(t, in)
		})
	}
}

// TestCodexDescriptorIsVerifiedAgainstTheLiveSession keeps the descriptor
// field and the fixtures from drifting apart: the version the fixtures
// were captured from must be the version verified_against names, and a
// verified descriptor may never fall back to the literal "unverified"
// without the fixtures going with it.
func TestCodexDescriptorIsVerifiedAgainstTheLiveSession(t *testing.T) {
	d, _ := Lookup(Codex)
	if d.Unverified() {
		t.Fatal("codex is verified by a live session (testdata/codecs/codex/*.live.in.json) — verified_against must name it")
	}
	for _, want := range []string{"0.153.1", "2026-09-12"} {
		if !strings.Contains(d.VerifiedAgainst, want) {
			t.Errorf("verified_against %q must name %q — the version and date the fixtures were captured from", d.VerifiedAgainst, want)
		}
	}
	// The measured scope fact. It is the single most surprising thing in
	// this descriptor and the one an adopter is most likely to "fix" back
	// to the documentation, so it is pinned.
	init, ok := d.InitHookFile()
	if ok {
		t.Errorf("codex must name NO writable project hook file: codex-cli 0.153.1 does not load <repo>/.codex/hooks.json, so writing %s would be wiring that cannot fire", init.Path)
	}
	var userHooks bool
	for _, f := range d.HookFiles {
		if f.Scope == "user" && f.Kind == "json-hook" {
			userHooks = true
		}
		if f.Scope == "project" && f.Kind != "" {
			t.Errorf("project hook file %s must carry no wiring kind (it is not loaded by this Codex build)", f.Path)
		}
	}
	if !userHooks {
		t.Error("codex must name its CODEX_HOME hooks.json as a readable (never written) json-hook file — it is the layer that actually fires")
	}
}
