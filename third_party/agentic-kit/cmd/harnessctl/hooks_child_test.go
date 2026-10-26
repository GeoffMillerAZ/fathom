//go:build harness_maintenance && (darwin || linux)

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type hookProcessRequest struct {
	Mode, Release, Ack, PID string
}

func TestHookDefaultFactoriesDoNotReturnTypedNil(t *testing.T) {
	deps := hookDefaultDeps()
	if child, err := deps.prepare(hookEvaluator{}, ""); err == nil || child != nil {
		t.Error("failed evaluator preparation returned a live interface")
	}
	if native, err := deps.maintenance.newNative(maintenanceNative{}); err == nil || native != nil {
		t.Error("failed native preparation returned a live interface")
	}
	if store, err := deps.maintenance.newStore("", nil); err == nil || store != nil {
		t.Error("failed state preparation returned a live interface")
	}
}

// The test executable is also the fixed, explicitly pinned subprocess fixture.
// This dispatch is compiled only into tests. Production has no test-mode switch.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "evaluate-spawn" {
		// This tiny fixture is not an application test run. Avoid the race
		// runtime's one-second successful-exit drain in its process protocol.
		syscall.Exit(hookProcessFixtureMain())
	}
	os.Exit(m.Run())
}

func hookProcessFixtureMain() int {
	var request hookProcessRequest
	if json.NewDecoder(os.Stdin).Decode(&request) != nil {
		return 3
	}
	switch request.Mode {
	case "status-0", "status-1", "status-2":
		fmt.Fprintln(os.Stdout, `{"fixture":true}`)
		n, _ := strconv.Atoi(strings.TrimPrefix(request.Mode, "status-"))
		return n
	case "stdout-cap":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 8193))
		return 0
	case "stderr-cap":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 4097))
		return 0
	case "stderr":
		fmt.Fprintln(os.Stderr, "synthetic private diagnostic")
		return 0
	case "hang":
		for {
			// A real process timeout without sleeps, network or a host mutation.
		}
	case "pipes":
		request.Mode = "drain"
		payload, _ := json.Marshal(request)
		child := exec.Command(os.Args[0], "evaluate-spawn")
		child.Stdin = bytes.NewReader(payload)
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if child.Start() != nil {
			return 3
		}
		if os.WriteFile(request.PID, []byte(strconv.Itoa(child.Process.Pid)), 0600) != nil {
			_ = child.Process.Kill()
			return 3
		}
		fmt.Fprintln(os.Stdout, `{"fixture":true}`)
		return 0
	case "drain":
		release, err := os.Open(request.Release)
		if err != nil {
			return 3
		}
		ack, err := os.OpenFile(request.Ack, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			_ = release.Close()
			return 3
		}
		_, _ = ack.Write([]byte("R"))
		_, err = io.Copy(io.Discard, release)
		_ = release.Close()
		if err == nil {
			_, _ = ack.Write([]byte("D"))
		}
		_ = ack.Close()
		return 0
	default:
		return 3
	}
}

func hookPinnedProcess(t *testing.T) *hookChild {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	child, err := hookPrepareChild(hookEvaluator{Profile: "agent-hooks/spawn-eval/1", Executable: executable, SHA256: hex.EncodeToString(sum[:]), TimeoutMillis: 1000}, nativeSafeRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := child.Close(); err != nil {
			t.Error(err)
		}
	})
	return child
}

func TestHookChildActualExitCapsAndCancellation(t *testing.T) {
	child := hookPinnedProcess(t)
	for _, tc := range []struct {
		mode       string
		status     int
		incomplete bool
	}{
		{"status-0", 0, false}, {"status-1", 1, false}, {"status-2", 2, false},
		{"stdout-cap", 0, true}, {"stderr-cap", 0, true}, {"stderr", 0, true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			request, _ := json.Marshal(hookProcessRequest{Mode: tc.mode})
			raw, invoked, completed, status, err := child.Evaluate(context.Background(), request, time.Second)
			if !invoked || !completed || status != tc.status || (err != nil) != tc.incomplete {
				t.Fatalf("invoked=%v completed=%v status=%d error=%v", invoked, completed, status, err)
			}
			if !tc.incomplete && string(raw) != "{\"fixture\":true}\n" {
				t.Fatal("unexpected complete frame")
			}
			if tc.incomplete && raw != nil {
				t.Fatal("incomplete process exposed output")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	raw, invoked, completed, _, err := child.Evaluate(ctx, []byte(`{"Mode":"hang"}`), time.Second)
	if err == nil || raw != nil || invoked || completed {
		t.Fatal("pre-canceled process started or completed")
	}
	_, invoked, completed, _, err = child.Evaluate(context.Background(), []byte(`{"Mode":"hang"}`), 75*time.Millisecond)
	if err == nil || !invoked || completed {
		t.Fatal("hung process escaped the deadline")
	}
}

func TestHookChildInheritedPipesDoNotWaitOneSecond(t *testing.T) {
	child := hookPinnedProcess(t)
	dir := t.TempDir()
	releasePath, ackPath, pidPath := filepath.Join(dir, "release"), filepath.Join(dir, "ack"), filepath.Join(dir, "owned-child-pid")
	if err := syscall.Mkfifo(releasePath, 0600); err != nil {
		t.Fatal(err)
	}
	release, err := os.OpenFile(releasePath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer release.Close()
	finished := false
	defer func() {
		if !finished {
			// Only a PID just written by this pinned fixture in our private directory.
			raw, _ := os.ReadFile(pidPath)
			if pid, err := strconv.Atoi(string(raw)); err == nil && pid > 0 {
				if process, err := os.FindProcess(pid); err == nil {
					_ = process.Kill()
				}
			}
		}
	}()
	request, _ := json.Marshal(hookProcessRequest{Mode: "pipes", Release: releasePath, Ack: ackPath, PID: pidPath})
	start := time.Now()
	_, invoked, _, _, err := child.Evaluate(context.Background(), request, 750*time.Millisecond)
	if err == nil || !invoked || time.Since(start) >= 900*time.Millisecond {
		t.Fatal("inherited pipe was accepted or consumed the old one-second drain")
	}
	waitAck := func(want string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		for {
			if raw, err := os.ReadFile(ackPath); err == nil && string(raw) == want {
				return
			}
			select {
			case <-ctx.Done():
				_, pidErr := os.Stat(pidPath)
				_, ackErr := os.Stat(ackPath)
				t.Fatalf("owned descendant handshake incomplete at %s: pid=%v ack=%v", want, pidErr, ackErr)
			default:
				runtime.Gosched()
			}
		}
	}
	waitAck("R")
	if err := release.Close(); err != nil {
		t.Fatal(err)
	}
	waitAck("RD")
	finished = true
}
