//go:build darwin

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/geoffmilleraz/app-kit/spikes/wails-gallery/internal/hostwire"
)

type companionProcess struct {
	wire   *hostwire.Conn
	cmd    *exec.Cmd
	exited chan struct{}
	err    error // published by exited
}

func companionSocket() (net.Conn, *os.File, error) {
	syscall.ForkLock.RLock()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		syscall.ForkLock.RUnlock()
		return nil, nil, errors.New("companion control socket unavailable")
	}
	syscall.CloseOnExec(fds[0])
	syscall.CloseOnExec(fds[1])
	syscall.ForkLock.RUnlock()
	parent, child := os.NewFile(uintptr(fds[0]), "parent-control"), os.NewFile(uintptr(fds[1]), "child-control")
	conn, err := net.FileConn(parent) // duplicate into a pollable, deadline-capable connection
	_ = parent.Close()
	if err != nil {
		_ = child.Close()
		return nil, nil, errors.New("companion control socket unavailable")
	}
	return conn, child, nil
}

func startCompanion(path string) (*companionProcess, error) {
	conn, child, err := companionSocket()
	if err != nil {
		return nil, err
	}
	defer child.Close()
	cmd := exec.Command(path, "-native-child")
	cmd.Env = []string{}
	cmd.ExtraFiles = []*os.File{child} // descriptor 3, never a token in argv/environment
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	// Do not inherit Wails updater/helper/MCP/debug controls or unrelated secrets.
	for _, name := range []string{"PATH", "HOME", "USER", "LOGNAME", "TMPDIR", "LANG", "LC_ALL", "LC_CTYPE", "__CF_USER_TEXT_ENCODING"} {
		if value, ok := os.LookupEnv(name); ok {
			cmd.Env = append(cmd.Env, name+"="+value)
		}
	}
	if err := cmd.Start(); err != nil {
		_ = conn.Close()
		return nil, errors.New("native companion could not start")
	}
	p := &companionProcess{wire: hostwire.New(conn), cmd: cmd, exited: make(chan struct{})}
	go func() { p.err = cmd.Wait(); close(p.exited) }()
	return p, nil
}

func (p *companionProcess) stop() error {
	_ = p.wire.Close()
	select {
	case <-p.exited:
		return nil
	default:
	}
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-p.exited:
		return nil
	case <-time.After(2 * time.Second):
	}
	_ = p.cmd.Process.Kill()
	select {
	case <-p.exited:
		return nil
	case <-time.After(2 * time.Second):
		return errors.New("native companion did not reap within shutdown bound")
	}
}

// runCompanion owns one native attempt, never a restart loop. The HTTP host stays
// in this process; neither native Exit nor Cocoa termination can take it down.
func runCompanion(ctx context.Context, h *host, path string, diagnostics io.Writer, opener func(*host) error, readyTimeout time.Duration) (result error) {
	if opener == nil {
		opener = func(h *host) error { return openCompanionBrowser(ctx, h) }
	}
	continued := false
	recoverBrowser := func() error {
		if ctx.Err() != nil {
			return h.Wait()
		}
		select {
		case <-h.done:
			return h.Wait()
		default:
		}
		fmt.Fprintln(diagnostics, "Native companion unavailable; continuing the existing HTTP app in the browser.")
		if !continued {
			if err := opener(h); err != nil {
				failure := errors.New("native recovery browser launch failed; after stopping, retry with -browser; no launch credential was logged")
				fmt.Fprintln(diagnostics, "Browser opening failed. Existing HTTP sessions remain usable; Ctrl-C stops this server and returns the failure.")
				// A broken opener is not a reason to destroy an already-open
				// browser's work. Retain foreground lifetime, but fail on exit.
				return errors.Join(failure, h.Wait())
			}
			continued = true
		}
		fmt.Fprintln(diagnostics, "Browser continuation is active; Ctrl-C stops the HTTP app. Existing browser sessions retain their server state; native drafts are not transferred.")
		return h.Wait()
	}
	if ctx.Err() != nil {
		return h.Wait()
	}
	p, err := startCompanion(path)
	if err != nil {
		return recoverBrowser()
	}
	defer func() { result = errors.Join(result, p.stop()) }()
	fallback := func() error {
		if err := p.stop(); err != nil {
			return err
		}
		return recoverBrowser()
	}
	launch, err := h.auth.LaunchURL()
	if err != nil {
		return errors.New("could not issue companion launch")
	}
	if err := p.wire.Write(hostwire.Message{Version: 1, Kind: "init", Origin: h.baseURL, Launch: launch}); err != nil {
		return fallback()
	}
	type received struct {
		message hostwire.Message
		err     error
	}
	messages := make(chan received, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			m, err := p.wire.Read(0)
			select {
			case messages <- received{m, err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	deadline := time.NewTimer(readyTimeout)
	defer deadline.Stop()
	readyLimit := deadline.C
	ready := false
	processExit := p.exited
	var exitLimit <-chan time.Time
	var exitTimer *time.Timer
	defer func() {
		if exitTimer != nil {
			exitTimer.Stop()
		}
	}()
	window, count := time.Now(), 0
	for {
		select {
		case <-ctx.Done():
			return h.Wait()
		case <-h.done:
			return h.Wait()
		case <-readyLimit:
			return fallback()
		case <-processExit:
			// A final 'closed' frame can already be in the socket when Wait wins
			// the select. Drain that bounded tail before classifying the exit.
			processExit = nil
			exitTimer = time.NewTimer(3 * time.Second)
			exitLimit = exitTimer.C
		case <-exitLimit:
			return fallback()
		case event := <-messages:
			if event.err != nil {
				return fallback()
			}
			if time.Since(window) >= time.Second {
				window, count = time.Now(), 0
			}
			count++
			if count > 32 {
				return fallback()
			}
			m := event.message
			if m.Kind == "ready" && !ready {
				ready = true
				deadline.Stop()
				readyLimit = nil
				fmt.Fprintln(diagnostics, "Native companion navigation acknowledged (not a task or platform certification).")
				continue
			}
			// A user can quit before the first document finishes loading. A
			// PostShutdown + zero exit remains an intentional close, not a crash.
			if !ready && m.Kind != "closed" {
				return fallback()
			}
			switch m.Kind {
			case "launch-request":
				url, err := h.auth.LaunchURL()
				if err != nil {
					return fallback()
				}
				if err := p.wire.Write(hostwire.Message{Version: 1, Kind: "launch", Launch: url}); err != nil {
					return fallback()
				}
			case "browser":
				kind := "browser-ok"
				if !continued {
					if err := opener(h); err != nil {
						kind = "browser-failed"
						fmt.Fprintln(diagnostics, "Browser handoff failed; native view remains available. No credential was logged.")
					} else {
						continued = true
						fmt.Fprintln(diagnostics, "Browser continuation selected; Ctrl-C stops the HTTP app even after native windows close.")
					}
				}
				if err := p.wire.Write(hostwire.Message{Version: 1, Kind: kind}); err != nil {
					return fallback()
				}
			case "closed":
				select {
				case <-p.exited:
					if p.err != nil {
						return fallback()
					}
				case <-time.After(2 * time.Second):
					return fallback()
				case <-ctx.Done():
					return h.Wait()
				}
				if continued {
					return h.Wait()
				}
				return h.Close()
			default:
				return fallback()
			}
		}
	}
}

func inheritedCompanionWire() (*hostwire.Conn, error) {
	f := os.NewFile(3, "companion-control")
	if f == nil {
		return nil, errors.New("native child needs its private inherited control socket")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return nil, errors.New("native child needs its private inherited control socket")
	}
	conn, err := net.FileConn(f)
	if err != nil {
		return nil, errors.New("native child control socket unavailable")
	}
	return hostwire.New(conn), nil
}

// Bound the OS opener too: a wedged LaunchServices invocation must not prevent
// parent cancellation. Its errors/stdout may contain the one-time browser URL.
func openCompanionBrowser(ctx context.Context, h *host) error {
	launch, err := h.auth.LaunchURL()
	if err != nil {
		return errors.New("browser launch unavailable")
	}
	openCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(openCtx, "/usr/bin/open", launch)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Run(); err != nil {
		return errors.New("browser launch unavailable")
	}
	return nil
}
