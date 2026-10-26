//go:build harness_maintenance

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const maintenanceNativeLimit = 64 << 10
const maintenanceNativeExecutableLimit = 64 << 20

var errMaintenanceNative = errors.New("native operation incomplete")

type maintenanceNativeCounts struct {
	StampAttempts, StampInvoked, StampCompleted          int
	ApprovalAttempts, ApprovalInvoked, ApprovalCompleted int
}

// maintenanceNativeRuntime is process-local and not safe for concurrent use.
// A receiver holds its project lock across its bounded native operations.
type maintenanceNativeRuntime struct {
	selection  maintenanceNative
	rootChain  []os.FileInfo
	directory  string
	dirInfo    os.FileInfo
	executable string
	counts     maintenanceNativeCounts
	closed     bool
}

func prepareMaintenanceNative(selection maintenanceNative) (*maintenanceNativeRuntime, error) {
	return prepareMaintenanceNativeWithHost(selection, runtime.GOOS)
}

// The host override is used by test fixtures only. Production calls the wrapper above.
func prepareMaintenanceNativeWithHost(selection maintenanceNative, host string) (*maintenanceNativeRuntime, error) {
	preparation, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if host != "darwin" || selection.Profile != "darwin-protected-key/1" || !maintenanceAbs(selection.Executable) ||
		!maintenanceHash(selection.SHA256) || !maintenanceAbs(selection.PreparationRoot) || selection.TimeoutMillis < 1 || selection.TimeoutMillis > 120000 {
		return nil, errMaintenanceNative
	}
	rootChain, ok := maintenanceNativeProtectedChain(selection.PreparationRoot)
	if !ok {
		return nil, errMaintenanceNative
	}
	if !maintenanceNativeNoSymlink(selection.Executable) {
		return nil, errMaintenanceNative
	}
	f, e := maintenanceNativeOpenSource(selection.Executable)
	if e != nil {
		return nil, errMaintenanceNative
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maintenanceNativeExecutableLimit || info.Mode().Perm()&0111 == 0 {
		return nil, errMaintenanceNative
	}
	raw, e := maintenanceNativeReadBounded(f)
	if e != nil || preparation.Err() != nil || len(raw) == 0 || len(raw) > maintenanceNativeExecutableLimit || int64(len(raw)) != info.Size() {
		return nil, errMaintenanceNative
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != selection.SHA256 {
		return nil, errMaintenanceNative
	}
	// A path replacement during acquisition cannot silently change the pinned source.
	pathInfo, e := os.Lstat(selection.Executable)
	if e != nil || !os.SameFile(info, pathInfo) || !maintenanceNativeNoSymlink(selection.Executable) {
		return nil, errMaintenanceNative
	}
	directory, e := os.MkdirTemp(selection.PreparationRoot, "harnessctl-native-")
	if e != nil {
		return nil, errMaintenanceNative
	}
	dirInfo, e := os.Lstat(directory)
	if e != nil || !dirInfo.IsDir() || dirInfo.Mode().Perm() != 0700 || !maintenanceNativeRootOwned(dirInfo) {
		return nil, errMaintenanceNative
	}
	cleanup := func() {
		if maintenanceNativeSameChain(selection.PreparationRoot, rootChain) {
			if currentDir, err := os.Lstat(directory); err == nil && os.SameFile(dirInfo, currentDir) {
				_ = os.RemoveAll(directory)
			}
		}
	}
	if !maintenanceNativeSameChain(selection.PreparationRoot, rootChain) {
		cleanup()
		return nil, errMaintenanceNative
	}
	if e = os.Chmod(directory, 0700); e != nil {
		cleanup()
		return nil, errMaintenanceNative
	}
	copyPath := filepath.Join(directory, "helper")
	out, e := os.OpenFile(copyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if e != nil {
		cleanup()
		return nil, errMaintenanceNative
	}
	_, writeErr := out.Write(raw)
	syncErr := out.Sync()
	closeErr := out.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		cleanup()
		return nil, errMaintenanceNative
	}
	copy, e := maintenanceNativeOpenSource(copyPath)
	if e != nil {
		cleanup()
		return nil, errMaintenanceNative
	}
	verified, e := maintenanceNativeReadBounded(copy)
	closeReadErr := copy.Close()
	if e != nil || closeReadErr != nil || preparation.Err() != nil || len(verified) != len(raw) || sha256.Sum256(verified) != digest {
		cleanup()
		return nil, errMaintenanceNative
	}
	if !maintenanceNativeSameChain(selection.PreparationRoot, rootChain) {
		cleanup()
		return nil, errMaintenanceNative
	}
	return &maintenanceNativeRuntime{selection: selection, rootChain: rootChain, directory: directory, dirInfo: dirInfo, executable: copyPath}, nil
}

func maintenanceNativeReadBounded(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maintenanceNativeExecutableLimit+1))
}

func maintenanceNativeNoSymlink(path string) bool {
	if !maintenanceAbs(path) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, string(os.PathSeparator)), string(os.PathSeparator))
	current := string(os.PathSeparator)
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if e != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

func maintenanceNativeProtectedRoot(path string) (os.FileInfo, bool) {
	chain, ok := maintenanceNativeProtectedChain(path)
	if !ok {
		return nil, false
	}
	return chain[len(chain)-1], true
}

func maintenanceNativeSameChain(path string, expected []os.FileInfo) bool {
	actual, ok := maintenanceNativeProtectedChain(path)
	if !ok || len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if !os.SameFile(actual[i], expected[i]) {
			return false
		}
	}
	return true
}

func maintenanceNativeProtectedChain(path string) ([]os.FileInfo, bool) {
	if !maintenanceAbs(path) {
		return nil, false
	}
	current := string(os.PathSeparator)
	ancestor, e := os.Lstat(current)
	if e != nil || !ancestor.IsDir() || ancestor.Mode().Perm()&0022 != 0 || !maintenanceNativeAncestorOwned(ancestor) {
		return nil, false
	}
	chain := []os.FileInfo{ancestor}
	for _, part := range strings.Split(strings.TrimPrefix(path, current), current) {
		current = filepath.Join(current, part)
		ancestor, e = os.Lstat(current)
		if e != nil || !ancestor.IsDir() || ancestor.Mode().Perm()&0022 != 0 || !maintenanceNativeAncestorOwned(ancestor) {
			return nil, false
		}
		chain = append(chain, ancestor)
	}
	if ancestor.Mode().Perm()&0077 != 0 || !maintenanceNativeRootOwned(ancestor) {
		return nil, false
	}
	return chain, true
}

type maintenanceCappedOutput struct {
	bytes    []byte
	overflow bool
}

func (w *maintenanceCappedOutput) Write(p []byte) (int, error) {
	original := len(p)
	remaining := maintenanceNativeLimit - len(w.bytes)
	if len(p) > remaining {
		w.overflow = true
		p = p[:remaining]
	}
	w.bytes = append(w.bytes, p...)
	return original, nil
}

func (n *maintenanceNativeRuntime) Counts() maintenanceNativeCounts { return n.counts }

func (n *maintenanceNativeRuntime) intact() bool {
	if !maintenanceNativeSameChain(n.selection.PreparationRoot, n.rootChain) {
		return false
	}
	directory, e := os.Lstat(n.directory)
	if e != nil || !directory.IsDir() || directory.Mode().Perm() != 0700 || !os.SameFile(n.dirInfo, directory) {
		return false
	}
	f, e := maintenanceNativeOpenSource(n.executable)
	if e != nil {
		return false
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maintenanceNativeExecutableLimit || info.Mode().Perm()&0111 == 0 {
		return false
	}
	raw, e := maintenanceNativeReadBounded(f)
	if e != nil || len(raw) > maintenanceNativeExecutableLimit || int64(len(raw)) != info.Size() {
		return false
	}
	pathInfo, e := os.Lstat(n.executable)
	if e != nil || !os.SameFile(info, pathInfo) {
		return false
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]) == n.selection.SHA256
}

func (n *maintenanceNativeRuntime) run(ctx context.Context, timeout time.Duration, argv []string, plan []byte) ([]byte, bool, error) {
	if n == nil || n.closed || ctx == nil || !n.intact() {
		return nil, false, errMaintenanceNative
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(bounded, n.executable, argv...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + n.directory, "TMPDIR=" + n.directory, "LANG=C", "LC_ALL=C"}
	cmd.Dir = n.directory
	cmd.WaitDelay = time.Second
	var stdout, stderr maintenanceCappedOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	var input io.WriteCloser
	if plan != nil {
		var e error
		input, e = cmd.StdinPipe()
		if e != nil {
			return nil, false, errMaintenanceNative
		}
	}
	if e := cmd.Start(); e != nil {
		if input != nil {
			_ = input.Close()
		}
		return nil, false, errMaintenanceNative
	}
	invoked := true
	var written <-chan error
	if input != nil {
		finished := make(chan error, 1)
		written = finished
		go func() {
			var prefix [4]byte
			binary.BigEndian.PutUint32(prefix[:], uint32(len(plan)))
			_, e := io.Copy(input, bytes.NewReader(prefix[:]))
			if e == nil {
				_, e = io.Copy(input, bytes.NewReader(plan))
			}
			finished <- e // Keep the pipe open as the parent-liveness channel.
		}()
	}
	e := cmd.Wait()
	if input != nil {
		select {
		case writeErr := <-written:
			if writeErr != nil {
				e = errMaintenanceNative
			}
		case <-bounded.Done():
			e = errMaintenanceNative
		}
		_ = input.Close()
	}
	if e != nil || bounded.Err() != nil || stdout.overflow || stderr.overflow || len(stdout.bytes) == 0 || len(stdout.bytes) > maintenanceNativeLimit {
		return nil, invoked, errMaintenanceNative
	}
	return stdout.bytes, invoked, nil
}

func (n *maintenanceNativeRuntime) Stamp(ctx context.Context) (maintenanceHostStamp, error) {
	var zero maintenanceHostStamp
	if n == nil || n.closed || n.counts.StampAttempts >= 4 {
		return zero, errMaintenanceNative
	}
	n.counts.StampAttempts++
	timeout := time.Duration(n.selection.TimeoutMillis) * time.Millisecond
	if timeout > 5*time.Second {
		timeout = 5 * time.Second
	}
	raw, invoked, e := n.run(ctx, timeout, []string{"stamp"}, nil)
	if invoked {
		n.counts.StampInvoked++
	}
	if e != nil {
		return zero, errMaintenanceNative
	}
	stamp, e := decodeMaintenanceHostStamp(raw)
	if e != nil {
		return zero, errMaintenanceNative
	}
	n.counts.StampCompleted++
	return stamp, nil
}

func (n *maintenanceNativeRuntime) Approve(ctx context.Context, plan []byte, approver maintenanceApprover) ([]byte, error) {
	if n == nil || n.closed || n.counts.ApprovalAttempts >= 1 {
		return nil, errMaintenanceNative
	}
	n.counts.ApprovalAttempts++
	if len(plan) == 0 || len(plan) > maintenanceNativeLimit || !maintenanceApproverValid(approver) || approver.Revoked ||
		approver.OSUserID != strconv.Itoa(os.Getuid()) {
		return nil, errMaintenanceNative
	}
	argv := []string{"approve", "--key-tag", approver.KeyTag, "--os-user-id", approver.OSUserID, "--public-key", approver.PublicKey}
	raw, invoked, e := n.run(ctx, time.Duration(n.selection.TimeoutMillis)*time.Millisecond, argv, plan)
	if invoked {
		n.counts.ApprovalInvoked++
	}
	if e != nil {
		return nil, errMaintenanceNative
	}
	if _, e = decodeMaintenanceNativeApproval(raw); e != nil {
		return nil, errMaintenanceNative
	}
	n.counts.ApprovalCompleted++
	return raw, nil
}

func (n *maintenanceNativeRuntime) Close() error {
	if n == nil || n.closed {
		return errMaintenanceNative
	}
	n.closed = true
	if !maintenanceNativeSameChain(n.selection.PreparationRoot, n.rootChain) {
		return errMaintenanceNative
	}
	info, e := os.Lstat(n.directory)
	if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 || !os.SameFile(n.dirInfo, info) {
		return errMaintenanceNative
	}
	if e := os.RemoveAll(n.directory); e != nil {
		return errMaintenanceNative
	}
	if _, e := os.Lstat(n.directory); !os.IsNotExist(e) {
		return errMaintenanceNative
	}
	return nil
}
