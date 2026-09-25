//go:build darwin && server && recoveryprobe && inputprobe && !workflowprobe && !capabilityprobe

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
)

// External AX actions and PID-targeted keyboard events exercise the unchanged
// native executable. Neither DOM eval nor in-process test callbacks drive it.
// These are automated OS inputs, not a claim of human keyboard/mouse/IME testing.
func TestActualNativeWindowLifecycle(t *testing.T) {
	keyOperation := "key-new-window"
	switch os.Getenv("APPKIT_NATIVE_UI_KEY_MODE") {
	case "":
	case "chord":
		keyOperation = "key-new-window-chord"
	default:
		t.Fatal("unknown native keyboard input mode")
	}
	native, err := prepareCompanion(os.Getenv("APPKIT_RECOVERY_NATIVE"), os.Getenv("APPKIT_RECOVERY_SHA256"))
	if err != nil {
		t.Fatal("explicit native identity required")
	}
	defer native.close()
	driver, err := prepareCompanion(os.Getenv("APPKIT_NATIVE_UI_DRIVER"), os.Getenv("APPKIT_NATIVE_UI_DRIVER_SHA256"))
	if err != nil {
		t.Fatal("explicit native UI driver identity required")
	}
	defer driver.close()
	t.Logf("native_sha256=%s driver_sha256=%s input=external_AX_and_PID_keyboard", os.Getenv("APPKIT_RECOVERY_SHA256"), os.Getenv("APPKIT_NATIVE_UI_DRIVER_SHA256"))
	t.Logf("keyboard_operation=%s", keyOperation)

	for _, scenario := range []struct {
		name          string
		continuation  bool
		keyboard      bool
		keyboardFirst bool
	}{
		{"last_window_closes_parent", false, true, false},
		{"ax_close_and_reopen_closes_parent", false, false, false},
		{"browser_menu_retains_parent", true, false, false},
		{"keyboard_initial_and_reopen", false, true, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			continuation := scenario.continuation
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()
			var profileLoads atomic.Int32
			app := gallery.New()
			h, err := startHost(ctx, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/shell/profile" {
					profileLoads.Add(1)
				}
				app.ServeHTTP(w, r)
			}))
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			log := &nativeRecoveryLog{ready: make(chan struct{})}
			var opens atomic.Int32
			opened := make(chan error, 2)
			jar, _ := cookiejar.New(nil)
			transport := &http.Transport{Proxy: nil}
			defer transport.CloseIdleConnections()
			client := &http.Client{Jar: jar, Transport: transport, Timeout: 3 * time.Second}
			result := make(chan error, 1)
			go func() {
				result <- runCompanion(ctx, h, native.path, log, func(h *host) error {
					opens.Add(1)
					launch, err := h.auth.LaunchURL()
					if err == nil {
						err = uiProbeHTTP(client, launch)
					}
					opened <- err
					return err
				}, 15*time.Second)
			}()
			waited := false
			defer func() {
				cancel()
				if !waited {
					select {
					case err := <-result:
						if err != nil {
							t.Error("native UI parent shutdown failed")
						}
					case <-time.After(7 * time.Second):
						t.Error("native UI parent shutdown exceeded bound")
					}
				}
			}()
			select {
			case <-log.ready:
			case <-time.After(18 * time.Second):
				t.Fatal("native UI navigation did not become ready")
			}
			child := exactNativeChild(t, native.path)
			ui := func(caller context.Context, operation string) nativeUIState {
				t.Helper()
				callCtx, stop := context.WithTimeout(caller, 6*time.Second)
				defer stop()
				cmd := exec.CommandContext(callCtx, driver.path, strconv.Itoa(child.Pid), native.path, operation)
				cmd.Env = recoveryParentEnvironment("", "")
				var output uiProbeOutput
				cmd.Stdout = &output
				cmd.Stderr = io.Discard
				if err := cmd.Run(); err != nil {
					// A diagnostic may send another tagged key only after this
					// assertion has failed. It can never rescue the gate below.
					if operation == "inspect" && callCtx.Err() != nil && os.Getenv("APPKIT_NATIVE_UI_DIAGNOSTIC") == "1" {
						logNativeKeyDiagnostic(t, ctx, driver.path, native.path, child.Pid)
					}
					var refusal struct {
						Error string `json:"error"`
					}
					_ = json.Unmarshal(output.Bytes(), &refusal)
					category := "unknown"
					switch refusal.Error {
					case "invalid-arguments", "native-identity-mismatch", "accessibility-unavailable", "native-ui-unavailable", "activation-unavailable":
						category = refusal.Error
					}
					if callCtx.Err() != nil {
						category = "observation-deadline"
					}
					t.Fatalf("native UI driver failed: operation=%s category=%s", operation, category)
				}
				state, err := parseNativeUIState(output.Bytes())
				if err != nil || !state.OK || state.Operation != operation || state.PID != child.Pid {
					t.Fatalf("native UI driver returned invalid evidence: operation=%s", operation)
				}
				return state
			}
			windows := func(want int) nativeUIState {
				t.Helper()
				waitCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				defer stop()
				lastCount := -1
				for {
					state := ui(waitCtx, "inspect")
					if state.Windows != lastCount {
						t.Logf("observed_windows=%d wanted=%d new_enabled=%v", state.Windows, want, state.NewWindowEnabled)
						lastCount = state.Windows
					}
					if state.Windows == want {
						return state
					}
					if waitCtx.Err() != nil {
						if os.Getenv("APPKIT_NATIVE_UI_DIAGNOSTIC") == "1" {
							logNativeKeyDiagnostic(t, ctx, driver.path, native.path, child.Pid)
						}
						t.Fatalf("native window count=%d want=%d", state.Windows, want)
					}
					time.Sleep(40 * time.Millisecond)
				}
			}
			profiles := func(want int32) {
				t.Helper()
				deadline := time.Now().Add(5 * time.Second)
				for profileLoads.Load() < want {
					if time.Now().After(deadline) || ctx.Err() != nil {
						t.Fatalf("native profile loads=%d want at least %d", profileLoads.Load(), want)
					}
					time.Sleep(25 * time.Millisecond)
				}
			}
			initial := windows(1)
			if !initial.NewWindowEnabled || !initial.BrowserEnabled {
				t.Fatal("ready native menu actions remain unavailable")
			}
			if continuation {
				ui(ctx, "open-browser")
				select {
				case err := <-opened:
					if err != nil {
						t.Fatal("native browser menu did not exchange guarded HTTP launch")
					}
				case <-time.After(5 * time.Second):
					t.Fatal("native browser menu was not dispatched")
				}
				windows(1)
			} else {
				if scenario.keyboardFirst {
					ui(ctx, keyOperation)
				} else {
					ui(ctx, "new-window")
				}
				second := windows(2)
				profiles(2)
				if second.NewWindowEnabled {
					t.Fatal("native new-window menu remains enabled at the two-window limit")
				}
				if scenario.keyboard {
					ui(ctx, keyOperation)
					time.Sleep(250 * time.Millisecond)
					windows(2)
				}
				ui(ctx, "close-window")
				if !windows(1).NewWindowEnabled {
					t.Fatal("native new-window menu did not recover after a close")
				}
				if scenario.keyboard {
					ui(ctx, keyOperation)
				} else {
					ui(ctx, "new-window")
				}
				windows(2)
				profiles(3)
				ui(ctx, "close-window")
				windows(1)
				if profileLoads.Load() < 3 {
					t.Fatal("native windows did not reach the guarded Gallery profile")
				}
			}
			ui(ctx, "close-window")
			if err := waitPIDAbsent(child.Pid, 6*time.Second); err != nil {
				t.Fatal("last native window close did not exit the child")
			}
			if ctx.Err() != nil {
				t.Fatal("test deadline expired before native close proof")
			}
			if continuation {
				select {
				case <-result:
					waited = true
					t.Fatal("closing native window ended browser continuation")
				default:
				}
				if err := uiProbeHTTP(client, h.baseURL+"/shell/profile"); err != nil {
					t.Fatal("browser session unavailable after last native window close")
				}
				h.Close()
			}
			select {
			case err := <-result:
				waited = true
				if err != nil {
					t.Fatal("normal native close returned failure")
				}
			case <-time.After(7 * time.Second):
				t.Fatal("normal native close did not finish parent lifetime")
			}
			wantOpens := int32(0)
			if continuation {
				wantOpens = 1
			}
			if opens.Load() != wantOpens {
				t.Fatalf("browser opener count=%d want=%d", opens.Load(), wantOpens)
			}
			if err := waitListenerRefusal(h.baseURL, time.Second); err != nil {
				t.Fatal("finished parent listener did not refuse connections")
			}
			t.Logf("external_native_actions=passed close_child_exit=passed parent_lifetime=passed browser_opens=%d profile_loads=%d", opens.Load(), profileLoads.Load())
		})
	}
}

// Diagnostic booleans are not task receipts and never promote a failed test.
func logNativeKeyDiagnostic(t *testing.T, ctx context.Context, driver, native string, pid int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, driver, strconv.Itoa(pid), native, "diagnose-key-new-window")
	cmd.Env = recoveryParentEnvironment("", "")
	var output uiProbeOutput
	cmd.Stdout, cmd.Stderr = &output, io.Discard
	if err := cmd.Run(); err != nil {
		var refusal struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(output.Bytes(), &refusal)
		switch refusal.Error {
		case "invalid-arguments", "native-identity-mismatch", "accessibility-unavailable", "native-ui-unavailable", "activation-unavailable":
		default:
			refusal.Error = "unknown"
		}
		t.Logf("native_key_diagnostic=unavailable category=%s", refusal.Error)
		return
	}
	var facts struct {
		OK           bool   `json:"ok"`
		PID          int    `json:"pid"`
		Operation    string `json:"operation"`
		Category     string `json:"category"`
		FocusKnown   bool   `json:"focused_window_available"`
		CharKnown    bool   `json:"shortcut_character_available"`
		ModKnown     bool   `json:"shortcut_modifiers_available"`
		KeyKnown     bool   `json:"shortcut_virtual_key_available"`
		Focused      bool   `json:"focused_window_owned"`
		Character    bool   `json:"shortcut_character_matches"`
		Modifiers    bool   `json:"shortcut_modifiers_match"`
		VirtualKey   bool   `json:"shortcut_virtual_key_matches"`
		Listen       bool   `json:"listen_available"`
		Down         bool   `json:"key_down_seen"`
		Up           bool   `json:"key_up_seen"`
		EventMatches bool   `json:"event_shape_matches"`
	}
	if json.Unmarshal(output.Bytes(), &facts) != nil || !facts.OK || facts.PID != pid || facts.Operation != "diagnose-key-new-window" {
		t.Log("native_key_diagnostic=invalid")
		return
	}
	switch facts.Category {
	case "listen-permission-unavailable", "event-tap-unavailable", "event-source-unavailable", "events-observed", "events-not-observed":
	default:
		facts.Category = "unknown"
	}
	t.Logf("native_key_diagnostic category=%s focus_known=%v character_known=%v modifiers_known=%v virtual_key_known=%v", facts.Category, facts.FocusKnown, facts.CharKnown, facts.ModKnown, facts.KeyKnown)
	t.Logf("native_key_diagnostic focused=%v character=%v modifiers=%v virtual_key=%v listen=%v down=%v up=%v event_matches=%v", facts.Focused, facts.Character, facts.Modifiers, facts.VirtualKey, facts.Listen, facts.Down, facts.Up, facts.EventMatches)
}

type nativeUIState struct {
	OK               bool   `json:"ok"`
	Operation        string `json:"operation"`
	PID              int    `json:"pid"`
	Windows          int    `json:"windows"`
	NewWindowEnabled bool   `json:"new_window_enabled"`
	BrowserEnabled   bool   `json:"browser_enabled"`
}

func parseNativeUIState(payload []byte) (nativeUIState, error) {
	var state nativeUIState
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, errors.New("native UI receipt invalid")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return state, errors.New("native UI receipt trailing data")
	}
	// A second token walk rejects duplicate/missing keys independently of struct
	// decoding, whose last-key-wins behavior is not an evidence contract.
	decoder = json.NewDecoder(bytes.NewReader(payload))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return state, errors.New("native UI receipt not an object")
	}
	keys := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || keys[key] {
			return state, errors.New("native UI receipt duplicate key")
		}
		keys[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return state, errors.New("native UI receipt invalid value")
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return state, errors.New("native UI receipt null value")
		}
	}
	if len(keys) != 6 || state.PID <= 1 || state.Windows < 0 || state.Windows > 2 {
		return state, errors.New("native UI receipt missing field or invalid range")
	}
	return state, nil
}

type uiProbeOutput struct{ buffer bytes.Buffer }

func (b *uiProbeOutput) Bytes() []byte { return b.buffer.Bytes() }

func (b *uiProbeOutput) Write(p []byte) (int, error) {
	if b.buffer.Len()+len(p) > 4096 {
		return 0, errors.New("native UI probe output exceeded bound")
	}
	return b.buffer.Write(p)
}

func TestNativeUIReceiptRefusals(t *testing.T) {
	valid := `{"ok":true,"operation":"inspect","pid":1234,"windows":1,"new_window_enabled":true,"browser_enabled":true}`
	if _, err := parseNativeUIState([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{
		valid + `{}`, valid[:len(valid)-1] + `,"pid":1234}`,
		`{"ok":true,"operation":"inspect","pid":1234,"windows":1,"new_window_enabled":true}`,
		`{"ok":true,"operation":"inspect","pid":1234,"windows":1,"new_window_enabled":null,"browser_enabled":true}`,
		`{"ok":true,"operation":"inspect","pid":1234,"windows":3,"new_window_enabled":true,"browser_enabled":true}`,
		`{"ok":true,"operation":"inspect","pid":1234,"windows":1,"new_window_enabled":true,"browser_enabled":true,"extra":true}`,
	} {
		if _, err := parseNativeUIState([]byte(payload)); err == nil {
			t.Fatal("invalid native UI receipt accepted")
		}
	}
	var output uiProbeOutput
	if _, err := io.Copy(&output, bytes.NewBuffer(make([]byte, 4097))); err == nil || len(output.Bytes()) > 4096 {
		t.Fatal("native UI output limit bypassed")
	}
}

func uiProbeHTTP(client *http.Client, target string) error {
	response, err := client.Get(target)
	if err != nil {
		return errors.New("guarded HTTP request unavailable")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (256<<10)+1))
	if err != nil || len(body) > 256<<10 || response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("Alex Morgan")) {
		return errors.New("guarded HTTP profile unavailable")
	}
	return nil
}
