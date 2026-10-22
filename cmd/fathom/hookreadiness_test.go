package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func readinessEnv(t *testing.T) (string, time.Time) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv("FATHOM_THREADS", filepath.Join(t.TempDir(), "threads.jsonl"))
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("FATHOM_CHANNELS", "")
	return t.TempDir(), time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
}

func TestReadinessRegistryCoversEveryInjectingCapability(t *testing.T) {
	for _, d := range harness.All() {
		for cap, vehicle := range d.Vehicles {
			if vehicle == harness.VehicleHookInject && hookReadinessRegistry()[cap].check == nil {
				t.Errorf("%s: injecting capability %s lacks readiness", d.ID, cap)
			}
		}
		for _, b := range d.Hooks {
			if b.Vehicle == harness.VehicleHookInject && hookReadinessRegistry()[b.Capability].check == nil {
				t.Errorf("%s: injecting binding %s lacks readiness", d.ID, b.Capability)
			}
		}
	}
}

func TestReadinessMapsRecentRowsAndReadErrors(t *testing.T) {
	root, now := readinessEnv(t)
	ctx := readinessInput{root: root, harnessID: harness.ClaudeCode, now: now}
	for _, tc := range []struct {
		name, kind string
		age        time.Duration
		ready      bool
	}{
		{"no map", "handoff-note", 0, false},
		{"old map", "map", 8 * 24 * time.Hour, false},
		{"recent header", "map", time.Hour, true},
		{"recent entry", "map-entry", 2 * time.Hour, true},
		{"future map", "map", -time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(threadItem{ID: "map:example", Kind: tc.kind, FiledAt: now.Add(-tc.age).Format(time.RFC3339)})
			if err := os.WriteFile(threadsPath(), append(raw, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			got := hookReadinessRegistry()[harness.CapSubagentPrime].check(ctx)
			if got.ready != tc.ready || (!got.ready && got.reason != "no-maps") {
				t.Fatalf("readiness = %+v", got)
			}
		})
	}
	t.Setenv("FATHOM_THREADS", t.TempDir())
	if got := hookReadinessRegistry()[harness.CapSubagentPrime].check(ctx); !got.ready {
		t.Fatalf("read error must fail open: %+v", got)
	}
}

func TestReadinessObligationsAndPartialCoverage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rules []semantic.Obligation
		ready bool
	}{
		{"none", nil, false},
		{"heading only", []semantic.Obligation{{Tier: "raw", Rule: "Working conventions"}}, false},
		{"raw instruction", []semantic.Obligation{{Tier: "raw", Rule: "Never widen a query."}}, true},
		{"compiled", []semantic.Obligation{{Tier: "doc-claimed", Rule: "Working conventions"}}, true},
		{"partial", []semantic.Obligation{{Tier: "raw", Rule: "Working conventions"}, {Tier: "doc-claimed", Rule: "Keep output sorted."}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := hookReadinessRegistry()[harness.CapObligationsPostEdit].check(readinessInput{rules: tc.rules, instructionsKnown: true, uncompiled: 3})
			if got.ready != tc.ready || got.uncompiled != 3 {
				t.Fatalf("got %+v", got)
			}
			if !got.ready && got.reason != "no-instructions" {
				t.Fatalf("got %+v", got)
			}
		})
	}
	if got := hookReadinessRegistry()[harness.CapObligationsPostEdit].check(readinessInput{}); !got.ready {
		t.Fatal("unknown coverage must fail open")
	}
}

func TestReadinessOptInBinding(t *testing.T) {
	root, now := readinessEnv(t)
	for _, cap := range []harness.Capability{harness.CapReadout, harness.CapClaimCheck, harness.CapReadOutlineHint} {
		ctx := readinessInput{root: root, now: now, harnessID: harness.ClaudeCode}
		if got := hookReadinessRegistry()[cap].check(ctx); !got.ready {
			t.Fatalf("Claude binding %s: %+v", cap, got)
		}
		ctx.harnessID = harness.GeminiCLI
		if got := hookReadinessRegistry()[cap].check(ctx); got.ready || got.reason != "hook-not-registered" {
			t.Fatalf("unsupported binding %s: %+v", cap, got)
		}
	}
}

func TestReadinessNotReadyMapIsSilentAndLogged(t *testing.T) {
	root, now := readinessEnv(t)
	if got := subagentMapLine(root, "parent", "child", now); got != "" {
		t.Fatalf("map line = %q", got)
	}
	rows := readBeats(t, beatLogPath(), "subagent-hook")
	if len(rows) != 1 || rows[0].Kind != "skipped" || rows[0].Meta["reason"] != "not-ready:no-maps" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestReadinessUnsupportedReadoutEmitsNothing(t *testing.T) {
	root, _ := readinessEnv(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml"), []byte("[readouts]\nmode = 'rich'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := cmdReadoutHook([]string{"--harness", harness.Codex}, strings.NewReader(`{"session_id":"s","cwd":`+mustJSON(root)+`,"tool_name":"Bash","tool_input":{"command":"ls"},"tool_response":{"stdout":"a\nb\n"}}`), &out, &bytes.Buffer{})
	if code != 0 || out.Len() != 0 {
		t.Fatalf("code=%d output=%q", code, out.String())
	}
	rows := readBeats(t, beatLogPath(), readoutChannel)
	if len(rows) != 1 || rows[0].Meta["reason"] != "not-ready:hook-not-registered" {
		t.Fatalf("rows = %+v", rows)
	}
}

func readinessHTTP(t *testing.T, payload map[string]any) {
	t.Helper()
	prior := http.DefaultTransport
	http.DefaultTransport = testRunRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/identity" {
			return &http.Response{StatusCode: 404, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		result := payload
		if req.Params.Name == "list_services" {
			result = map[string]any{"services": []any{}}
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(mustJSON(rpcTextResult(result))))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = prior })
}

func TestReadinessCoverageUnknownPreservesExistingBehavior(t *testing.T) {
	for _, coverage := range []any{nil, map[string]any{"status": "unknown"}, map[string]any{"stale_withheld": 2}, map[string]any{"status": "skipped"}} {
		t.Run(mustJSON(coverage), func(t *testing.T) {
			readinessEnv(t)
			readinessHTTP(t, map[string]any{"rules": []any{}, "layer_coverage": coverage})
			var got readinessResult
			_, _, _, ok, err := fetchObligations("http://daemon.invalid", "example", "file.go", 0, time.Second, "", func(r readinessResult) { got = r })
			if !ok || err != nil || !got.ready {
				t.Fatalf("unknown coverage = %+v, %v", got, err)
			}
		})
	}
}

func TestReadinessPostEditNoInstructionsSkipsExistingHookRow(t *testing.T) {
	root, now := readinessEnv(t)
	readinessHTTP(t, map[string]any{
		"rules":          []any{map[string]any{"rule": "Working conventions", "tier": "raw"}},
		"layer_coverage": map[string]any{"sections_total": 3, "sections_compiled": 0},
	})
	clock := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = clock })
	text, meta, ok := renderPostEdit("http://daemon.invalid", root, filepath.Join(root, "file.go"), "session", "")
	if ok || text != "" {
		t.Fatalf("text=%q meta=%+v ok=%v", text, meta, ok)
	}
	rows := readBeats(t, beatLogPath(), "edit-hook")
	if len(rows) != 1 || rows[0].Kind != "skipped" || rows[0].Meta["reason"] != "not-ready:no-instructions" {
		t.Fatalf("rows = %+v", rows)
	}
	s, err := readHookHealth(root, harness.ClaudeCode, now)
	if err != nil || s.Channels[harness.CapObligationsPostEdit].Uncompiled != 3 {
		t.Fatalf("state=%+v err=%v", s, err)
	}
}

func TestReadinessInstalledHookRows(t *testing.T) {
	root, _ := readinessEnv(t)
	cap := harness.CapReadout
	if got := readinessInstalled(root, harness.ClaudeCode, cap); got.ready {
		t.Fatal("absent hook is ready")
	}
	d, _ := harness.Lookup(harness.ClaudeCode)
	hooks := map[string][]any{}
	for _, b := range d.Hooks {
		if b.Capability == cap {
			hooks[b.Harness] = append(hooks[b.Harness], map[string]any{"matcher": b.Matcher, "hooks": []any{map[string]any{"type": "command", "command": b.Marker}}})
		}
	}
	path := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(mustJSON(map[string]any{"hooks": hooks})), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readinessInstalled(root, harness.ClaudeCode, cap); !got.ready {
		t.Fatalf("installed hook = %+v", got)
	}
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readinessInstalled(root, harness.ClaudeCode, cap); !got.ready {
		t.Fatal("unreadable hook config did not fail open")
	}
}

func TestReadinessOptInSeamsUseTheirParentHookRows(t *testing.T) {
	root, now := readinessEnv(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml"), []byte("[hooks]\ngate_preflight = true\nprompt_definitions = true\ntests_to_run = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	line := inactiveFeaturesLine(root, harness.Codex, now)
	for _, name := range []string{"gate preflight (hook not registered)", "prompt definitions (hook not registered)", "tests to run (hook not registered)"} {
		if !strings.Contains(line, name) {
			t.Fatalf("missing %q: %q", name, line)
		}
	}
}

func TestReadinessInstalledWrapperAndPluginUncertainty(t *testing.T) {
	root, _ := readinessEnv(t)
	d, _ := harness.Lookup(harness.ClaudeCode)
	hooks := map[string][]any{}
	var script strings.Builder
	for _, b := range d.Hooks {
		if b.Capability != harness.CapReadout {
			continue
		}
		hooks[b.Harness] = append(hooks[b.Harness], map[string]any{"matcher": b.Matcher, "hooks": []any{map[string]any{"type": "command", "command": "bash ./hook-wrapper.sh"}}})
		script.WriteString("fathom " + b.Verb + "\n")
	}
	if err := os.WriteFile(filepath.Join(root, "hook-wrapper.sh"), []byte(script.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(mustJSON(map[string]any{"hooks": hooks})), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readinessInstalled(root, harness.ClaudeCode, harness.CapReadout); !got.ready {
		t.Fatalf("registered command wrapper: %+v", got)
	}
	if err := os.WriteFile(path, []byte(`{"enabledPlugins":{"example@local":true}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readinessInstalled(root, harness.ClaudeCode, harness.CapReadout); !got.ready {
		t.Fatalf("unknown plugin registration must fail open: %+v", got)
	}
}
