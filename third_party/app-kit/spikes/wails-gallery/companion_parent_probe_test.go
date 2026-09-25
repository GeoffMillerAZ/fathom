//go:build darwin && server && recoveryprobe && !workflowprobe && !capabilityprobe

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/spikes/wails-gallery/internal/hostwire"
)

// TestActualNativeParentLoss is an explicit, actual-macOS probe. It proves only
// parent-loss containment for the selected Wails companion and loopback host.
func TestActualNativeParentLoss(t *testing.T) {
	native, digest := os.Getenv("APPKIT_RECOVERY_NATIVE"), os.Getenv("APPKIT_RECOVERY_SHA256")
	if native == "" || digest == "" {
		t.Fatal("actual parent-loss recovery requires explicit native binary and SHA256")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	parentContext, cancelParent := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancelParent()
	cmd := exec.CommandContext(parentContext, executable, "-recovery-parent-fixture")
	cmd.Env = recoveryParentEnvironment(native, digest)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	parentWaited := false
	childPID := 0
	childGone := false
	staged := ""
	stageValidated := false
	defer func() {
		if childPID == 0 && stageValidated && !parentWaited {
			childPID, _ = exactRecoveryChild(cmd.Process.Pid, staged)
		}
		if !parentWaited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		if childPID != 0 && !childGone && exactRecoveryCommand(childPID, staged) {
			if child, findErr := os.FindProcess(childPID); findErr == nil {
				_ = child.Kill()
			}
			_ = waitPIDAbsent(childPID, 3*time.Second)
		}
		if stageValidated {
			_ = os.Remove(staged)
			_ = os.Remove(filepath.Dir(staged))
		}
	}()

	reader := bufio.NewReaderSize(stdout, 4096)
	type metadataLine struct {
		line []byte
		err  error
	}
	metadataLines := make(chan metadataLine, 1)
	go func() {
		line, readErr := reader.ReadSlice('\n')
		metadataLines <- metadataLine{line: line, err: readErr}
	}()
	var line []byte
	select {
	case result := <-metadataLines:
		line, err = result.line, result.err
	case <-time.After(8 * time.Second):
		t.Fatal("parent fixture metadata read exceeded its deadline")
	}
	if err != nil || len(line) > 4096 {
		t.Fatal("parent fixture metadata unavailable or oversized")
	}
	metadata, err := parseRecoveryParentMetadata(bytes.TrimSuffix(line, []byte{'\n'}))
	if err != nil {
		t.Fatal(err)
	}
	staged = metadata.Native
	if err := validateRecoveryStage(t, staged, digest); err != nil {
		t.Fatal(err)
	}
	stageValidated = true

	log := &nativeRecoveryLog{ready: make(chan struct{})}
	copyDone := make(chan struct{})
	go func() {
		for {
			line, readErr := reader.ReadSlice('\n')
			if len(line) != 0 {
				_, _ = log.Write(line)
			}
			if readErr != nil {
				break
			}
		}
		close(copyDone)
	}()
	select {
	case <-log.ready:
	case <-time.After(20 * time.Second):
		t.Fatal("actual native companion did not acknowledge navigation")
	}
	childPID, err = exactRecoveryChild(cmd.Process.Pid, staged)
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Timeout: 2 * time.Second}).Get(metadata.Origin + "/healthz")
	if err != nil {
		t.Fatal("parent HTTP health check failed before process-loss injection")
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("parent HTTP health status %d", response.StatusCode)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatal("could not kill the exact owned parent fixture")
	}
	waitErr := cmd.Wait()
	parentWaited = true
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if waitErr == nil || !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatal("owned parent fixture did not terminate by SIGKILL")
	}
	if err := waitPIDAbsent(childPID, 6*time.Second); err != nil {
		t.Fatal("actual native companion remained after parent loss")
	}
	childGone = true
	if err := waitListenerRefusal(metadata.Origin, 3*time.Second); err != nil {
		t.Fatal("parent HTTP listener remained after parent loss")
	}
	select {
	case <-copyDone:
	case <-time.After(time.Second):
		t.Fatal("parent diagnostic pipe did not close")
	}
	if text := log.String(); strings.Contains(text, "token=") || strings.Contains(text, "/auth/launch") {
		t.Fatal("parent fixture diagnostics exposed launch authority")
	}
	t.Log("actual_parent_sigkill=passed exact_native_child_exit=passed parent_listener_closed=passed")
}

type recoveryParentMetadata struct {
	Origin string `json:"origin"`
	Native string `json:"native"`
}

func parseRecoveryParentMetadata(payload []byte) (recoveryParentMetadata, error) {
	var metadata recoveryParentMetadata
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil || decoder.Decode(new(any)) != io.EOF {
		return metadata, errors.New("parent fixture metadata invalid")
	}
	canonical, err := json.Marshal(metadata)
	if err != nil || !bytes.Equal(canonical, payload) || hostwire.ValidateOrigin(metadata.Origin) != nil || metadata.Native == "" {
		return metadata, errors.New("parent fixture metadata invalid")
	}
	return metadata, nil
}

func validateRecoveryStage(t *testing.T, staged, digest string) error {
	t.Helper()
	dir := filepath.Dir(staged)
	if filepath.Base(staged) != "gallery-native" || filepath.Dir(dir) != filepath.Clean(os.TempDir()) ||
		!strings.HasPrefix(filepath.Base(dir), "appkit-native-companion-") || !filepath.IsAbs(staged) || filepath.Clean(staged) != staged {
		return errors.New("parent fixture reported an unexpected staging path")
	}
	info, err := os.Lstat(staged)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0500 {
		return errors.New("parent fixture staging identity invalid")
	}
	if fileSHA256(t, staged) != digest {
		return errors.New("parent fixture staged hash mismatch")
	}
	return nil
}

func exactRecoveryChild(parentPID int, staged string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/pgrep", "-P", strconv.Itoa(parentPID)).Output()
	if err != nil {
		return 0, errors.New("could not enumerate owned parent children")
	}
	found := 0
	for _, text := range strings.Fields(string(output)) {
		pid, err := strconv.Atoi(text)
		if err != nil || !exactRecoveryCommand(pid, staged) {
			continue
		}
		if found != 0 {
			return 0, errors.New("ambiguous native companion child")
		}
		found = pid
	}
	if found == 0 {
		return 0, errors.New("exact native companion child absent")
	}
	return found, nil
}

func exactRecoveryCommand(pid int, staged string) bool {
	if pid <= 1 || staged == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/bin/ps", "-ww", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	return err == nil && strings.TrimSpace(string(output)) == staged+" -native-child"
}

func waitPIDAbsent(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		output, err := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "pid=").Output()
		contextErr := ctx.Err()
		cancel()
		if contextErr != nil {
			return errors.New("process absence observation timed out")
		}
		if err == nil {
			if strings.TrimSpace(string(output)) == strconv.Itoa(pid) {
				time.Sleep(25 * time.Millisecond)
				continue
			}
			return errors.New("process absence observation was ambiguous")
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(string(output)) == "" {
			return nil
		}
		return errors.New("process absence observation failed")
	}
	return errors.New("process identifier remained present")
}

func waitListenerRefusal(origin string, timeout time.Duration) error {
	address := strings.TrimPrefix(origin, "http://")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if errors.Is(err, syscall.ECONNREFUSED) {
			return nil
		}
		if err != nil {
			return errors.New("listener refusal observation failed")
		}
		_ = conn.Close()
		time.Sleep(25 * time.Millisecond)
	}
	return errors.New("listener still accepted connections")
}

func TestParentProbeObservationHelpers(t *testing.T) {
	if err := waitPIDAbsent(os.Getpid(), 40*time.Millisecond); err == nil {
		t.Fatal("reported the live test process absent")
	}
	exited := exec.Command("/usr/bin/true")
	if err := exited.Start(); err != nil {
		t.Fatal(err)
	}
	pid := exited.Process.Pid
	if err := exited.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := waitPIDAbsent(pid, time.Second); err != nil {
		t.Fatal("did not accept explicit ps exit 1 for a reaped process")
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://" + listener.Addr().String()
	if err := waitListenerRefusal(origin, 40*time.Millisecond); err == nil {
		_ = listener.Close()
		t.Fatal("reported a listening socket as refusing connections")
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitListenerRefusal(origin, time.Second); err != nil {
		t.Fatal("did not accept explicit connection refusal")
	}
}

func recoveryParentEnvironment(native, digest string) []string {
	environment := make([]string, 0, 11)
	for _, name := range []string{"PATH", "HOME", "USER", "LOGNAME", "TMPDIR", "LANG", "LC_ALL", "LC_CTYPE", "__CF_USER_TEXT_ENCODING"} {
		if value, ok := os.LookupEnv(name); ok {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment, "APPKIT_RECOVERY_NATIVE="+native, "APPKIT_RECOVERY_SHA256="+digest)
}
