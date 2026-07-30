package main

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
)

func TestFathomChannelsDisabled(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		t.Setenv("FATHOM_CHANNELS", "")
		if fathomChannelsDisabled() {
			t.Fatal("want false when FATHOM_CHANNELS is unset")
		}
	})
	t.Run("off", func(t *testing.T) {
		t.Setenv("FATHOM_CHANNELS", "off")
		if !fathomChannelsDisabled() {
			t.Fatal("want true when FATHOM_CHANNELS=off")
		}
	})
	t.Run("other value does not disable", func(t *testing.T) {
		t.Setenv("FATHOM_CHANNELS", "on")
		if fathomChannelsDisabled() {
			t.Fatal("want false for any value other than exactly \"off\"")
		}
	})
}

// TestEditHook_ChannelsOff_TotalSilence proves the kill switch's load-bearing
// claim for the post edit-hook: FATHOM_CHANNELS=off means zero daemon calls
// and zero stdout, even for a file that would otherwise be answered.
func TestEditHook_ChannelsOff_TotalSilence(t *testing.T) {
	t.Setenv("FATHOM_CHANNELS", "off")
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")

	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the daemon must never be queried when channels are off", 500)
	})
	defer srv.Close()

	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, file, dir)
	hookStdin(t, hookInput)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("expected silence, got %q", out.String())
	}
	if n := *calls; n != 0 {
		t.Errorf("daemon queried %d time(s), want 0", n)
	}
}

// TestSessionHook_ChannelsOff_TotalSilence mirrors the edit-hook proof for
// the SessionStart hook.
func TestSessionHook_ChannelsOff_TotalSilence(t *testing.T) {
	t.Setenv("FATHOM_CHANNELS", "off")
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the daemon must never be queried when channels are off", 500)
	})
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := run([]string{"session-hook", "--url=" + srv.URL, "--dir=" + t.TempDir()}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("expected silence, got %q", out.String())
	}
	if n := *calls; n != 0 {
		t.Errorf("daemon queried %d time(s), want 0", n)
	}
}

// TestStopHook_ChannelsOff_TotalSilence mirrors the same proof for the Stop
// hook — a plans dir with a newly-claimed, unverifiable task would normally
// produce advisory output; FATHOM_CHANNELS=off must suppress it entirely.
func TestStopHook_ChannelsOff_TotalSilence(t *testing.T) {
	t.Setenv("FATHOM_CHANNELS", "off")
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the daemon must never be queried when channels are off", 500)
	})
	defer srv.Close()

	hookStdin(t, `{"cwd":"/nonexistent"}`)
	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"stop-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("expected silence, got %q", out.String())
	}
	if n := *calls; n != 0 {
		t.Errorf("daemon queried %d time(s), want 0", n)
	}
}
