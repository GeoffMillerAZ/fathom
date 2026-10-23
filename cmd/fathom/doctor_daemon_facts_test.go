package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/capability"
)

func doctorFactsEnv(t *testing.T) {
	t.Helper()
	hermeticObsEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv("FATHOM_CALL_LOG", "")
	t.Setenv("FATHOM_STATE_DIR", "")
	t.Cleanup(resetObsCache)
}

func doctorFactsModel(t *testing.T, model observatoryModel) {
	t.Helper()
	previous := obsModelBuilder
	obsModelBuilder = func(string, time.Time) observatoryModel { return model }
	t.Cleanup(func() { obsModelBuilder = previous })
}

// Exercise HTTP request/response handling without a listening socket: the
// helper sandbox disallows even loopback port 0.
type doctorFactsTransport struct {
	ctx     context.Context
	handler http.Handler
}

func (tr doctorFactsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := tr.ctx.Err(); err != nil {
		return nil, err
	}
	w := httptest.NewRecorder()
	tr.handler.ServeHTTP(w, req.Clone(tr.ctx))
	if err := tr.ctx.Err(); err != nil {
		return nil, err
	}
	return w.Result(), nil
}

func doctorFactsClient(ctx context.Context, handler http.HandlerFunc) *http.Client {
	return &http.Client{Transport: doctorFactsTransport{ctx: ctx, handler: handler}}
}

func TestDoctorFactsProbeReadsLocalWiringWithoutModel(t *testing.T) {
	doctorFactsEnv(t)
	doctorFactsModel(t, observatoryModel{DegradedNote: "fixture model build abandoned"})
	root := t.TempDir()
	doctorWriteFixture(t, root, ".claude/settings.json", `{"hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"fathom edit-hook"}]}]}}`)
	doctorWriteFixture(t, root, "AGENTS.md", fathomStanza("fixture", false))
	probe := buildCapabilityProbeContext(t.Context(), "", root, "fixture", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if !probe.HooksReferenced || !strings.Contains(probe.HooksNote, "hook reference") || !probe.StanzaPresent {
		t.Fatalf("local wiring lost without model: %+v", probe)
	}
}

func TestDoctorFactsCapabilitiesKeepLocalVerdicts(t *testing.T) {
	for _, tc := range []struct {
		name, guard  string
		hooks, model bool
		want         doctorStatus
		text         string
	}{
		{"wired missing model", `guards: [{scopes: ["src/**"]}]`, true, false, doctorUnknown, "fixture model build abandoned"},
		{"invalid guard missing model", `guards: [`, true, false, doctorFail, "guard.cue"},
		{"unwired missing model", `guards: [{scopes: ["src/**"]}]`, false, false, doctorFail, "no edit hook is wired"},
		{"unwired present model", `guards: [{scopes: ["src/**"]}]`, false, true, doctorFail, "no edit hook is wired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doctorFactsEnv(t)
			root := t.TempDir()
			writeGuardCue(t, root, tc.guard)
			doctorWriteFixture(t, root, ".fathom/rules/local.md", "Local rule.\n")
			if tc.hooks {
				doctorWriteFixture(t, root, ".claude/settings.json", `{"hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"fathom edit-hook"}]}]}}`)
			}
			model := observatoryModel{DegradedNote: "fixture model build abandoned"}
			if tc.model {
				// Deliberately disagree with disk: wiring comes from local files.
				model = observatoryModel{Services: []obsServiceRow{{fleetServiceRow: fleetServiceRow{Service: "fixture", Path: root, Symbols: 1, Freshness: "fresh", HooksNote: "1 hook reference"}, CompileNote: "obligations compiled"}}}
			}
			doctorFactsModel(t, model)
			row := checkCapabilities("", root, "fixture", t.Context())
			if row.Status != tc.want || !strings.Contains(row.Reason, tc.text) {
				t.Fatalf("capabilities = %+v, want %s containing %q", row, tc.want, tc.text)
			}
			if tc.want == doctorUnknown && (strings.Contains(row.Reason, "no edit hook is wired") || !strings.HasPrefix(row.Reason, "2 verdict(s) need daemon facts")) {
				t.Fatalf("model absence confused with local failure: %+v", row)
			}
			probe := buildCapabilityProbeContext(t.Context(), "", root, "fixture", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
			if !tc.hooks {
				guards, _ := capability.Default().Get("guards")
				if state := guards.Detect(probe); !state.Broken {
					t.Fatalf("locally unwired guards must stay broken: %+v", state)
				}
			}
		})
	}
}

// Build against a socket-free fake daemon, then freeze that result for the
// capability probe. This exercises the production registry/statusz fold.
func doctorFactsHTTPModel(t *testing.T, handler http.HandlerFunc) observatoryModel {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = doctorFactsTransport{ctx: t.Context(), handler: handler}
	defer func() { http.DefaultTransport = previous }()
	return buildObservatoryModel("http://daemon.invalid", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
}

func TestDoctorFactsUnregisteredRepository(t *testing.T) {
	doctorFactsEnv(t)
	model := doctorFactsHTTPModel(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"services":[]}}`))
	})
	if model.DegradedNote != "" {
		t.Fatalf("healthy registry degraded: %s", model.DegradedNote)
	}
	doctorFactsModel(t, model)
	root := t.TempDir()
	probe := buildCapabilityProbeContext(t.Context(), "", root, "", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	index, _ := capability.Default().Get("index")
	state, remedy := index.Detect(probe), index.Remedy(probe)
	if state.Unknown != "" || state.Broken || state.Operating || remedy.Cmd != "fathom init" {
		t.Fatalf("unregistered index lost its off/init verdict: %+v %+v", state, remedy)
	}
	if row := checkCapabilities("", root, "", t.Context()); row.Status != doctorPass || strings.Contains(row.Reason, "model not built") {
		t.Fatalf("unregistered repository misdiagnosed: %+v", row)
	}

	doctorWriteFixture(t, root, ".fathom/rules/local.cue", `rules: [{id: "local", text: "Local rule."}]`)
	doctorWriteFixture(t, root, ".claude/settings.json", `{"hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"fathom edit-hook"}]}]}}`)
	probe = buildCapabilityProbeContext(t.Context(), "", root, "", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if probe.ObligationRules != 1 || !probe.HooksReferenced {
		t.Fatalf("rules-plus-hooks fixture missing: %+v", probe)
	}
	obligations, _ := capability.Default().Get("obligations")
	state, remedy = obligations.Detect(probe), obligations.Remedy(probe)
	if state.Unknown != "" || state.Broken || state.Operating || state.Reason != "rules are authored but this repository is not indexed" || remedy.Cmd != "fathom init" {
		t.Errorf("unregistered rules lost their off/init verdict: %+v %+v", state, remedy)
	}
	if row := checkCapabilities("", root, "", t.Context()); row.Status != doctorPass || strings.Contains(row.Remedy, "compile") {
		t.Fatalf("unregistered rules must not fail or suggest compile: %+v", row)
	}
}

func TestDoctorFactsObligationsCompileReadFailure(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP error", true: "cancelled read"}[cancelled], func(t *testing.T) {
			doctorFactsEnv(t)
			setObligationsProvider(t, true)
			root := t.TempDir()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			probes, compileReads := 0, 0
			hc := doctorFactsClient(ctx, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/rpc" {
					http.NotFound(w, r)
					return
				}
				var req struct{ Params struct{ Name string } }
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				var inner string
				switch req.Params.Name {
				case "list_services":
					inner = fmt.Sprintf(`{"services":[{"service":"fixture","path":%q}]}`, root)
				case "status":
					inner = `{"sources":["main.go"]}`
				case "obligations":
					probes++
					inner = `{"rules":[]}`
				case "staleness_report":
					compileReads++
					if cancelled {
						cancel()
						<-r.Context().Done()
						return
					}
					http.Error(w, `{"error":{"code":-32603,"message":"fixture compile facts unavailable"}}`, http.StatusServiceUnavailable)
					return
				default:
					t.Fatalf("unexpected tool %q", req.Params.Name)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"content": []map[string]string{{"type": "text", "text": inner}}}})
			})
			row := checkObligationsServed(hc, "http://daemon.invalid", root, "fixture", true, ctx)
			if probes != 3 || compileReads != 1 {
				t.Fatalf("compile read must follow zero-rule probes: probes=%d compileReads=%d", probes, compileReads)
			}
			want := "fixture compile facts unavailable"
			if cancelled {
				want = "context canceled"
			}
			if row.Name != "obligations_served" || row.Status != doctorUnknown || !strings.Contains(row.Reason, "staleness_report") || !strings.Contains(row.Reason, want) || row.probeErr == nil || strings.Contains(row.Remedy, "--dry-run=false") || strings.Contains(row.Remedy, "compile") || !strings.Contains(row.Remedy, "reachability") {
				t.Fatalf("unknown compile state must suggest reachability without spending: %+v", row)
			}
			if cancelled && !errors.Is(row.probeErr, context.Canceled) {
				t.Fatalf("compile probe error lost cancellation: %v", row.probeErr)
			}
		})
	}
}

func TestDoctorFactsStatuszFailureIsUnknown(t *testing.T) {
	for _, body := range []string{"unavailable", "{"} {
		t.Run(body, func(t *testing.T) {
			doctorFactsEnv(t)
			root := t.TempDir()
			doctorWriteFixture(t, root, ".fathom/rules/local.md", "Local rule.\n")
			doctorWriteFixture(t, root, ".claude/settings.json", `{"hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"fathom edit-hook"}]}]}}`)
			model := doctorFactsHTTPModel(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/statusz" {
					if body == "unavailable" {
						http.Error(w, body, http.StatusServiceUnavailable)
					} else {
						_, _ = w.Write([]byte(body))
					}
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"services": []map[string]any{{"service": "fixture", "path": root, "symbol_count": 1}}}})
			})
			if len(model.Services) != 1 {
				t.Fatalf("registry row missing: %+v", model)
			}
			doctorFactsModel(t, model)
			row := checkCapabilities("", root, "fixture", t.Context())
			if row.Status != doctorUnknown || !strings.HasPrefix(row.Reason, "1 verdict(s) need daemon facts") || !strings.Contains(row.Reason, "statusz") || strings.Contains(row.Reason, "never compiled") || strings.Contains(row.Remedy, "compile") {
				t.Fatalf("unread statusz became a compile verdict: %+v", row)
			}
			probe := buildCapabilityProbeContext(t.Context(), "", root, "fixture", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
			obligations, _ := capability.Default().Get("obligations")
			state, remedy := obligations.Detect(probe), obligations.Remedy(probe)
			if state.Unknown == "" || state.Broken || remedy.Cmd != "" || !strings.Contains(remedy.Note, "statusz") {
				t.Fatalf("unknown compile state suggests spending: %+v %+v", state, remedy)
			}
		})
	}
}

func TestDoctorFactsUnwiredRulesWithoutModel(t *testing.T) {
	doctorFactsEnv(t)
	root := t.TempDir()
	doctorWriteFixture(t, root, ".fathom/rules/local.md", "Local rule.\n")
	doctorFactsModel(t, observatoryModel{DegradedNote: "fixture daemon unreachable"})
	row := checkCapabilities("", root, "fixture", t.Context())
	if row.Status != doctorFail || !strings.Contains(row.Reason, "no edit hook is wired") || row.Remedy != "fathom init --hooks" {
		t.Fatalf("local missing hooks hidden by missing model: %+v", row)
	}
}

func TestDoctorFactsSurfaceTrafficSumsCandidateLogs(t *testing.T) {
	for _, localAgent := range []bool{false, true} {
		t.Run(fmt.Sprint(localAgent), func(t *testing.T) {
			doctorFactsEnv(t)
			root, daemonRoot := t.TempDir(), t.TempDir()
			writeFathomGoMod(t, root)
			t.Setenv("FATHOM_CALL_LOG", "workspace/logs/mcp-calls.jsonl")
			stamp := time.Now().UTC().Format(time.RFC3339)
			entry := func(surface string) string {
				return fmt.Sprintf("{\"ts\":%q,\"tool\":\"find_symbol\",\"surface\":%q}\n", stamp, surface)
			}
			surface := "cli"
			want := 1
			if localAgent {
				surface, want = "agent", 2
			}
			doctorWriteFixture(t, root, "workspace/logs/mcp-calls.jsonl", entry(surface))
			doctorWriteFixture(t, daemonRoot, "workspace/logs/mcp-calls.jsonl", entry("agent"))
			hc := doctorFactsClient(t.Context(), func(w http.ResponseWriter, r *http.Request) {
				var req struct{ Params struct{ Name string } }
				_ = json.NewDecoder(r.Body).Decode(&req)
				if req.Params.Name == "list_services" {
					_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"services": []map[string]string{{"service": "fathom", "path": daemonRoot}}}})
					return
				}
				_, _ = w.Write([]byte(`{"result":{"content":[{"type":"text","text":"{\"surface_traffic\":{\"agent\":{\"calls\":0}}}"}]}}`))
			})
			row := checkSurfaceTraffic(hc, "http://daemon.invalid", true, root)
			if row.Status != doctorPass || !strings.Contains(row.Reason, fmt.Sprintf("%d agent call(s)", want)) {
				t.Fatalf("traffic did not sum %s and daemon log: %+v", filepath.Base(root), row)
			}
		})
	}
}

func TestDoctorFactsGuardWalkReusedByPrefix(t *testing.T) {
	for _, prefix := range []string{"", "assets/"} {
		t.Run(prefix, func(t *testing.T) {
			doctorFactsEnv(t)
			root := t.TempDir()
			for i := 0; i < 500; i++ {
				doctorWriteFixture(t, root, fmt.Sprintf("assets/file%02d.css", i), "fixture")
				doctorWriteFixture(t, root, fmt.Sprintf("assets/file%02d.txt", i), "fixture")
			}
			hc := &http.Client{Transport: doctorDeliveryTransport{}}
			run := func(n int) float64 {
				var cue strings.Builder
				cue.WriteString("guards: [\n")
				for i := 0; i < n; i++ {
					ext := []string{"css", "txt"}[i%2]
					fmt.Fprintf(&cue, "{scopes: [%q]},\n", prefix+"*."+ext)
				}
				cue.WriteString("]\n")
				writeGuardCue(t, root, cue.String())
				return testing.AllocsPerRun(1, func() {
					row := checkGuardDeadScopes(hc, "http://daemon.invalid", root, "fixture", true, t.Context())
					if row.Status != doctorPass || !strings.Contains(row.Reason, fmt.Sprintf("%d guard(s) cover real files", n)) {
						t.Fatalf("guard disk discovery: %+v", row)
					}
				})
			}
			one, many := run(1), run(20)
			// Twenty guards may add matcher/parse work, but must not allocate
			// twenty copies of the same thousand-file disk walk.
			if many > one*4 {
				t.Fatalf("repeated prefix repeated disk work: one=%.0f twenty=%.0f", one, many)
			}
		})
	}
}

func TestDoctorFactsElidedScopesCheckDisk(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "disk match", true: "missing remains unknown"}[missing], func(t *testing.T) {
			doctorFactsEnv(t)
			root := t.TempDir()
			doctorWriteFixture(t, root, "assets/ui.css", "fixture")
			cue := `guards: [{scopes: ["assets/**"]}]`
			if missing {
				cue = `guards: [{scopes: ["assets/**"]}, {scopes: ["missing/**"]}]`
			}
			writeGuardCue(t, root, cue)
			hc := doctorFactsClient(t.Context(), func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"content": []map[string]string{{"type": "text", "text": `{"sources":["other.go"],"elided":{"count":1,"of":2}}`}}}})
			})
			row := checkGuardDeadScopes(hc, "http://daemon.invalid", root, "fixture", true, t.Context())
			if missing {
				if row.Status != doctorUnknown || !strings.Contains(row.Reason, "checked 1 of 2") {
					t.Fatalf("elided missing scope verdict: %+v", row)
				}
			} else if row.Status != doctorPass || !strings.Contains(row.Reason, "1 file(s) on disk") {
				t.Fatalf("disk scope was not checked: %+v", row)
			}
		})
	}
}

func TestDoctorFactsObligationsServiceReadFailure(t *testing.T) {
	for _, hang := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP error", true: "cancelled read"}[hang], func(t *testing.T) {
			doctorFactsEnv(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			hc := doctorFactsClient(ctx, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/rpc" {
					http.NotFound(w, r)
					return
				}
				var req struct{ Params struct{ Name string } }
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				if req.Params.Name != "list_services" {
					t.Errorf("unexpected tool %q", req.Params.Name)
				}
				if hang {
					cancel() // End the caller's budget after the request arrives.
					<-r.Context().Done()
					return
				}
				http.Error(w, "fixture registry unavailable", http.StatusInternalServerError)
			})
			row := checkObligationsServed(hc, "http://daemon.invalid", t.TempDir(), "fixture", true, ctx)
			want := "HTTP 500"
			if hang {
				want = "context canceled"
			}
			if row.Status == doctorFail || !strings.Contains(row.Reason, want) || strings.Contains(row.Reason, "no candidate service ID") {
				t.Fatalf("failed registry read became a wiring verdict: %+v", row)
			}
		})
	}
}

func TestDoctorFactsLocalCallLogSkipsServices(t *testing.T) {
	doctorFactsEnv(t)
	root := t.TempDir()
	writeFathomGoMod(t, root)
	doctorWriteFixture(t, root, ".fathom/config.toml", "[daemon]\nstate_dir = 'workspace'\n")
	doctorWriteFixture(t, root, "workspace/logs/mcp-calls.jsonl", "{\"ts\":\"2026-10-01T00:00:00Z\",\"tool\":\"obligations\",\"origin\":\"edit-hook\"}\n")
	var calls atomic.Int32
	hc := doctorFactsClient(t.Context(), func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"services": []any{}}})
	})
	row := checkHooksFiring(hc, "http://daemon.invalid", true, root, "claude")
	if row.Status != doctorPass || calls.Load() != 0 {
		t.Fatalf("readable local log queried daemon: %+v, calls=%d", row, calls.Load())
	}
}
