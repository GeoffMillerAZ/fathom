//go:build harness_maintenance

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var errHookChild = errors.New("hook evaluator unavailable")

type hookChild struct {
	selection  hookEvaluator
	root       string
	chain      []os.FileInfo
	directory  string
	dirInfo    os.FileInfo
	executable string
	sourceInfo os.FileInfo
	closed     bool
}

func hookPrepareChild(selection hookEvaluator, root string) (*hookChild, error) {
	if selection.Profile != "agent-hooks/spawn-eval/1" || !maintenanceAbs(selection.Executable) || !maintenanceHash(selection.SHA256) || !maintenanceAbs(root) || selection.TimeoutMillis < 1 || selection.TimeoutMillis > 1000 {
		return nil, errHookChild
	}
	chain, ok := maintenanceNativeProtectedChain(root)
	if !ok || !maintenanceNativeNoSymlink(selection.Executable) {
		return nil, errHookChild
	}
	f, e := maintenanceNativeOpenSource(selection.Executable)
	if e != nil {
		return nil, errHookChild
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || !maintenanceSingleLink(info) || info.Size() <= 0 || info.Size() > 64<<20 || info.Mode().Perm()&0111 == 0 {
		return nil, errHookChild
	}
	raw, e := io.ReadAll(io.LimitReader(f, (64<<20)+1))
	if e != nil || int64(len(raw)) != info.Size() {
		return nil, errHookChild
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != selection.SHA256 {
		return nil, errHookChild
	}
	pathInfo, e := os.Lstat(selection.Executable)
	if e != nil || !os.SameFile(info, pathInfo) || !maintenanceNativeNoSymlink(selection.Executable) || !maintenanceNativeSameChain(root, chain) {
		return nil, errHookChild
	}
	dir, e := os.MkdirTemp(root, "harnessctl-hook-")
	if e != nil {
		return nil, errHookChild
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	di, e := os.Lstat(dir)
	if e != nil || !di.IsDir() || di.Mode().Perm() != 0700 || !maintenanceNativeRootOwned(di) {
		cleanup()
		return nil, errHookChild
	}
	executable := filepath.Join(dir, "helper")
	out, e := os.OpenFile(executable, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if e != nil {
		cleanup()
		return nil, errHookChild
	}
	n, e := out.Write(raw)
	se := out.Sync()
	ce := out.Close()
	if e != nil || se != nil || ce != nil || n != len(raw) {
		cleanup()
		return nil, errHookChild
	}
	cf, e := maintenanceNativeOpenSource(executable)
	if e != nil {
		cleanup()
		return nil, errHookChild
	}
	copied, e := io.ReadAll(io.LimitReader(cf, 64<<20+1))
	cErr := cf.Close()
	if e != nil || cErr != nil || len(copied) != len(raw) || sha256.Sum256(copied) != sum || !maintenanceNativeSameChain(root, chain) {
		cleanup()
		return nil, errHookChild
	}
	return &hookChild{selection: selection, root: root, chain: chain, directory: dir, dirInfo: di, executable: executable, sourceInfo: info}, nil
}
func (c *hookChild) intact() bool {
	if c == nil || c.closed || !maintenanceNativeSameChain(c.root, c.chain) || !maintenanceNativeNoSymlink(c.selection.Executable) {
		return false
	}
	info, e := os.Lstat(c.selection.Executable)
	if e != nil || !os.SameFile(info, c.sourceInfo) {
		return false
	}
	f, e := maintenanceNativeOpenSource(c.selection.Executable)
	if e != nil {
		return false
	}
	raw, e := io.ReadAll(io.LimitReader(f, (64<<20)+1))
	_ = f.Close()
	sum := sha256.Sum256(raw)
	if e != nil || hex.EncodeToString(sum[:]) != c.selection.SHA256 {
		return false
	}
	di, e := os.Lstat(c.directory)
	if e != nil || !os.SameFile(di, c.dirInfo) || di.Mode().Perm() != 0700 {
		return false
	}
	cf, e := maintenanceNativeOpenSource(c.executable)
	if e != nil {
		return false
	}
	copy, e := io.ReadAll(io.LimitReader(cf, (64<<20)+1))
	_ = cf.Close()
	return e == nil && sha256.Sum256(copy) == sum
}
func (c *hookChild) Close() error {
	if c == nil || c.closed {
		return errHookChild
	}
	c.closed = true
	if !maintenanceNativeSameChain(c.root, c.chain) {
		return errHookChild
	}
	di, e := os.Lstat(c.directory)
	if e != nil || !os.SameFile(di, c.dirInfo) {
		return errHookChild
	}
	if os.RemoveAll(c.directory) != nil {
		return errHookChild
	}
	if _, e = os.Lstat(c.directory); !os.IsNotExist(e) {
		return errHookChild
	}
	return nil
}

type hookCap struct {
	limit    int
	bytes    []byte
	overflow bool
}

func (w *hookCap) Write(p []byte) (int, error) {
	n := len(p)
	if len(w.bytes)+len(p) > w.limit {
		w.overflow = true
		remaining := w.limit - len(w.bytes)
		if remaining > 0 {
			w.bytes = append(w.bytes, p[:remaining]...)
		}
		return n, nil
	}
	w.bytes = append(w.bytes, p...)
	return n, nil
}
func (c *hookChild) Evaluate(ctx context.Context, request []byte, timeout time.Duration) ([]byte, bool, bool, int, error) {
	const drainReserve = time.Millisecond
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	if !c.intact() || len(request) == 0 || len(request) > 512<<10 || timeout <= drainReserve {
		return nil, false, false, -1, errHookChild
	}
	bound, cancel := context.WithTimeout(ctx, timeout-drainReserve)
	defer cancel()
	cmd := exec.CommandContext(bound, c.executable, "evaluate-spawn")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + c.directory, "TMPDIR=" + c.directory, "LANG=C", "LC_ALL=C"}
	cmd.Dir = c.directory
	cmd.Stdin = bytes.NewReader(request)
	// A descendant holding stdout cannot extend this call by the default one-second
	// drain window after the evaluator exits or the context expires.
	cmd.WaitDelay = drainReserve
	stdout := &hookCap{limit: 8192}
	stderr := &hookCap{limit: 4096}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if e := cmd.Start(); e != nil {
		return nil, false, false, -1, errHookChild
	}
	e := cmd.Wait()
	completed := bound.Err() == nil
	status := 0
	if e != nil {
		if ex, ok := e.(*exec.ExitError); ok {
			status = ex.ExitCode()
		} else {
			status = -1
		}
	}
	if !completed || status < 0 || status > 2 || stdout.overflow || stderr.overflow || len(stderr.bytes) != 0 || len(stdout.bytes) == 0 {
		return nil, true, completed, status, errHookChild
	}
	return stdout.bytes, true, completed, status, nil
}
