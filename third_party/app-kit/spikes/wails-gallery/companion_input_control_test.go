//go:build darwin && server && recoveryprobe && inputprobe && !workflowprobe && !capabilityprobe

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// This independent Cocoa control identifies input-driver failures without
// changing Wails or granting the actual native host an inherited pass.
func TestNativeKeyboardAppKitControl(t *testing.T) {
	mode := os.Getenv("APPKIT_NATIVE_UI_CONTROL_MODE")
	if mode != "" && mode != "ax" {
		t.Fatal("unknown AppKit control input mode")
	}
	control, err := prepareCompanion(os.Getenv("APPKIT_NATIVE_UI_CONTROL"), os.Getenv("APPKIT_NATIVE_UI_CONTROL_SHA256"))
	if err != nil {
		t.Fatal("explicit AppKit control identity required")
	}
	defer control.close()
	driver, err := prepareCompanion(os.Getenv("APPKIT_NATIVE_UI_DRIVER"), os.Getenv("APPKIT_NATIVE_UI_DRIVER_SHA256"))
	if err != nil {
		t.Fatal("explicit native UI driver identity required")
	}
	defer driver.close()
	t.Logf("control_sha256=%s driver_sha256=%s", os.Getenv("APPKIT_NATIVE_UI_CONTROL_SHA256"), os.Getenv("APPKIT_NATIVE_UI_DRIVER_SHA256"))
	ctx, cancel := context.WithTimeout(t.Context(), 18*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, control.path)
	cmd.Env = recoveryParentEnvironment("", "")
	var output uiProbeOutput
	cmd.Stdout, cmd.Stderr = &output, io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal("AppKit control unavailable")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waited := false
	defer func() {
		cancel()
		if !waited {
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Error("AppKit control cleanup exceeded bound")
				return // Do not read a buffer whose writer may still be active.
			}
		}
		logAppKitControlFacts(t, output.Bytes())
	}()
	run := func(operation string) ([]byte, error) {
		callCtx, stop := context.WithTimeout(ctx, 3*time.Second)
		defer stop()
		call := exec.CommandContext(callCtx, driver.path, strconv.Itoa(cmd.Process.Pid), control.path, operation)
		call.Env = recoveryParentEnvironment("", "")
		var result uiProbeOutput
		call.Stdout, call.Stderr = &result, io.Discard
		err := call.Run()
		return result.Bytes(), err
	}
	windows := func(want int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) && ctx.Err() == nil {
			data, err := run("inspect")
			state, parseErr := parseNativeUIState(data)
			if err == nil && parseErr == nil && state.OK && state.PID == cmd.Process.Pid && state.Operation == "inspect" && state.Windows == want {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatalf("AppKit control did not reach %d windows", want)
	}
	windows(1)
	// This is the same tagged PID stimulus already observed in the Wails run.
	if mode == "ax" {
		data, err := run("new-window")
		state, parseErr := parseNativeUIState(data)
		if err != nil || parseErr != nil || !state.OK || state.PID != cmd.Process.Pid || state.Operation != "new-window" {
			t.Fatal("AppKit AX positive control failed")
		}
	} else {
		logNativeKeyDiagnostic(t, ctx, driver.path, control.path, cmd.Process.Pid)
	}
	windows(2)
	for _, remaining := range []int{1, 0} {
		data, err := run("close-window")
		state, parseErr := parseNativeUIState(data)
		if err != nil || parseErr != nil || !state.OK || state.PID != cmd.Process.Pid || state.Operation != "close-window" {
			t.Fatal("AppKit control close action failed")
		}
		if remaining != 0 {
			windows(remaining)
		}
	}
	select {
	case err := <-done:
		waited = true
		if err != nil || ctx.Err() != nil {
			t.Fatal("AppKit control failed normal close")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("AppKit control did not exit after last close")
	}
	t.Logf("plain_AppKit_input_and_close=passed mode=%q; Wails acceptance remains separate", mode)
}

// Fixture lines are diagnostics, not task receipts. Only fixed categories and
// scalar booleans/counts are logged; never arbitrary event text or raw stdout.
func logAppKitControlFacts(t *testing.T, output []byte) {
	t.Helper()
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte{'\n'}) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(line, &fields) != nil {
			t.Log("AppKit control diagnostic invalid")
			continue
		}
		var event string
		_ = json.Unmarshal(fields["event"], &event)
		switch event {
		case "ready", "created", "menu_action", "tagged_key", "timeout", "browser_placeholder", "menu_open", "menu_closed", "menu_equivalent", "window_equivalent":
		default:
			event = "unknown"
		}
		var count int
		_ = json.Unmarshal(fields["count"], &count)
		if count < 0 || count > 2 {
			count = -1
		}
		t.Logf("AppKit control event=%s count=%d", event, count)
		if event == "menu_equivalent" || event == "window_equivalent" {
			var handled, gallery bool
			_ = json.Unmarshal(fields["handled"], &handled)
			_ = json.Unmarshal(fields["gallery"], &gallery)
			t.Logf("AppKit equivalent handled=%v gallery=%v", handled, gallery)
		}
		if event == "ready" {
			var active, didFinish, owned bool
			_ = json.Unmarshal(fields["is_active"], &active)
			_ = json.Unmarshal(fields["finished_launching"], &didFinish)
			_ = json.Unmarshal(fields["key_window_owned"], &owned)
			t.Logf("AppKit ready active=%v did_finish_callback=%v key_window_owned=%v", active, didFinish, owned)
		}
		if event == "tagged_key" {
			var down, up, n, command, owned bool
			_ = json.Unmarshal(fields["key_down"], &down)
			_ = json.Unmarshal(fields["key_up"], &up)
			_ = json.Unmarshal(fields["n_characters"], &n)
			_ = json.Unmarshal(fields["command"], &command)
			_ = json.Unmarshal(fields["window_belongs"], &owned)
			t.Logf("AppKit key down=%v up=%v n=%v command=%v window_belongs=%v", down, up, n, command, owned)
			var menuOwned, active, enabled bool
			_ = json.Unmarshal(fields["menu_owned"], &menuOwned)
			_ = json.Unmarshal(fields["app_active"], &active)
			_ = json.Unmarshal(fields["new_enabled"], &enabled)
			t.Logf("AppKit key menu_owned=%v app_active=%v new_enabled=%v", menuOwned, active, enabled)
		}
	}
}
