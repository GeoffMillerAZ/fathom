//go:build darwin && !server

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/app-kit/spikes/wails-gallery/internal/hostwire"
)

func runNativeChild(ctx context.Context, diagnostics io.Writer) error {
	if err := requireNativeContainment(); err != nil {
		return err
	}
	wire, err := inheritedCompanionWire()
	if err != nil {
		return err
	}
	defer wire.Close()
	init, err := wire.Read(5 * time.Second)
	if err != nil || init.Kind != "init" {
		return errors.New("native child bootstrap unavailable")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
		case <-finished:
			return
		}
		// EOF must also stop a child stuck before ApplicationStarted, where
		// Wails cannot yet promise a safe Quit. Only this companion exits.
		deadline := time.NewTimer(4 * time.Second)
		defer deadline.Stop()
		select {
		case <-finished:
			return
		case <-deadline.C:
			os.Exit(1)
		}
	}()
	responses := make(chan hostwire.Message, 1)
	var replies companionReplies
	var failed, ready atomic.Bool
	failSession := func() { failed.Store(true); cancel() }
	go func() {
		defer failSession() // parent EOF closes the native lifecycle, never an orphan loop
		for {
			m, err := wire.Read(0)
			if err != nil {
				return
			}
			if !replies.accept(m.Kind) {
				return
			}
			select {
			case responses <- m:
			case <-ctx.Done():
				return
			default:
				return
			}
		}
	}()
	var requestMu sync.Mutex
	request := func(kind string) (hostwire.Message, error) {
		requestMu.Lock()
		defer requestMu.Unlock()
		fail := errors.New("native companion control request failed")
		if ctx.Err() != nil {
			return hostwire.Message{}, fail
		}
		if !replies.begin(kind) {
			failSession()
			return hostwire.Message{}, fail
		}
		if err := wire.Write(hostwire.Message{Version: 1, Kind: kind}); err != nil {
			failSession()
			return hostwire.Message{}, fail
		}
		select {
		case m := <-responses:
			return m, nil
		case <-ctx.Done():
			return hostwire.Message{}, fail
		case <-time.After(6 * time.Second):
			failSession()
			return hostwire.Message{}, fail
		}
	}
	first := true // launch is serialized by runNativeSession's window mutex
	session := nativeSession{
		origin: init.Origin, done: ctx.Done(), shutdown: func() error { return nil },
		launch: func() (string, error) {
			if first {
				first = false
				return init.Launch, nil
			}
			if !ready.Load() {
				return "", errors.New("native navigation is still starting")
			}
			m, err := request("launch-request")
			if err != nil || m.Kind != "launch" || hostwire.ValidateLaunch(init.Origin, m.Launch) != nil {
				failSession()
				return "", errors.New("native launch unavailable")
			}
			return m.Launch, nil
		},
		browser: func() error {
			if !ready.Load() {
				return errors.New("native navigation is still starting")
			}
			m, err := request("browser")
			if err != nil {
				return err
			}
			if m.Kind == "browser-ok" {
				return nil
			}
			if m.Kind != "browser-failed" {
				failSession()
			}
			return errors.New("browser handoff unavailable; native view retained")
		},
		ready: func() {
			if wire.Write(hostwire.Message{Version: 1, Kind: "ready"}) != nil {
				failSession()
				return
			}
			ready.Store(true)
		},
		closed: func() {
			if !failed.Load() {
				_ = wire.Write(hostwire.Message{Version: 1, Kind: "closed"})
			}
		},
	}
	return runNativeSession(ctx, session, diagnostics)
}
