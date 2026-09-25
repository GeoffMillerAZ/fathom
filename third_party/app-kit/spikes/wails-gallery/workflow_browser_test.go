//go:build workflowprobe

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

const (
	workflowBrowserCallTimeout = 8 * time.Second
	workflowBrowserInitTimeout = 15 * time.Second
	workflowBrowserExitGrace   = time.Second
	workflowBrowserKillWait    = 2 * time.Second
)

type workflowBrowser struct {
	ctx       context.Context
	cmd       *exec.Cmd
	in        io.WriteCloser
	outPipe   io.ReadCloser
	out       *bufio.Scanner
	id        int
	mu        sync.Mutex
	poisoned  bool
	waitDone  chan struct{}
	waitErr   error
	closeOnce sync.Once
}

func newWorkflowBrowser(ctx context.Context, origin string) (*workflowBrowser, error) {
	return newWorkflowBrowserCommand(ctx, exec.CommandContext(ctx, "node", "workflow_browser.mjs"), origin)
}

func newWorkflowBrowserCommand(ctx context.Context, cmd *exec.Cmd, origin string) (*workflowBrowser, error) {
	return startWorkflowBrowser(ctx, cmd, origin, false, "")
}

// Headed/capture settings are explicit test inputs. Ordinary workflow runs keep
// their original headless behavior and exact observation/console oracle.
func startWorkflowBrowser(ctx context.Context, cmd *exec.Cmd, origin string, headed bool, captureDir string) (*workflowBrowser, error) {
	d := &workflowBrowser{ctx: ctx, cmd: cmd, waitDone: make(chan struct{})}
	var err error
	d.in, err = d.cmd.StdinPipe()
	if err != nil {
		return nil, errors.New("browser pipe")
	}
	var childOutput *os.File
	d.outPipe, childOutput, err = os.Pipe()
	if err != nil {
		d.in.Close()
		return nil, errors.New("browser pipe")
	}
	d.cmd.Stdout = childOutput
	d.cmd.Stderr = io.Discard // Third-party launch errors may include capability URLs.
	d.out = bufio.NewScanner(d.outPipe)
	d.out.Buffer(make([]byte, 4096), 65536)
	if err = d.cmd.Start(); err != nil {
		_ = childOutput.Close()
		_ = d.outPipe.Close()
		_ = d.in.Close()
		return nil, errors.New("browser start")
	}
	// Only the child keeps the write end. Unlike Cmd.StdoutPipe, this owned pipe
	// is not closed by Wait before Scanner drains the child's final frame.
	_ = childOutput.Close()
	go func() {
		d.waitErr = d.cmd.Wait()
		close(d.waitDone)
	}()
	initCtx, cancel := d.commandContext(nil, workflowBrowserInitTimeout)
	_, err = d.call(initCtx, map[string]any{"op": "init", "origin": origin, "headed": headed, "captureDir": captureDir})
	cancel()
	if err != nil {
		d.shutdown()
		return nil, err
	}
	return d, nil
}

type workflowBrowserResult struct {
	value string
	err   error
}

func (d *workflowBrowser) commandContext(caller context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(d.ctx, limit)
	if caller == nil {
		return ctx, cancel
	}
	stop := context.AfterFunc(caller, cancel)
	if caller.Err() != nil {
		cancel()
	}
	return ctx, func() {
		stop()
		cancel()
	}
}

func (d *workflowBrowser) call(ctx context.Context, args map[string]any) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.poisoned {
		return "", errors.New("browser unavailable")
	}
	if err := ctx.Err(); err != nil {
		return "", errors.New("browser timeout")
	}
	d.id++
	id := d.id
	args["id"] = id
	done := make(chan workflowBrowserResult, 1)
	go func() { done <- d.exchange(id, args) }()
	select {
	case result := <-done:
		if result.err != nil && !errors.Is(result.err, errBrowserRefused) {
			d.poisoned = true
			d.shutdown()
		}
		return result.value, result.err
	case <-ctx.Done():
		d.poisoned = true
		d.shutdown()
		return "", errors.New("browser timeout")
	}
}

var errBrowserRefused = errors.New("browser command refused")

func (d *workflowBrowser) exchange(id int, args map[string]any) workflowBrowserResult {
	if json.NewEncoder(d.in).Encode(args) != nil {
		return workflowBrowserResult{err: errors.New("browser input")}
	}
	if !d.out.Scan() {
		return workflowBrowserResult{err: errors.New("browser output")}
	}
	var reply struct {
		ID    int    `json:"id"`
		OK    bool   `json:"ok"`
		Value string `json:"value"`
		Error string `json:"error"`
	}
	if json.Unmarshal(d.out.Bytes(), &reply) != nil || reply.ID != id {
		return workflowBrowserResult{err: errors.New("browser protocol")}
	}
	if !reply.OK {
		return workflowBrowserResult{err: errBrowserRefused}
	}
	return workflowBrowserResult{value: reply.Value}
}

func (d *workflowBrowser) shutdown() {
	d.closeOnce.Do(func() {
		_ = d.in.Close() // EOF gives Node a chance to close its browser and context.
		select {
		case <-d.waitDone:
			_ = d.outPipe.Close()
			return
		case <-time.After(workflowBrowserExitGrace):
		}
		_ = d.outPipe.Close() // Interrupt any blocked Scanner before the kill fallback.
		if d.cmd.Process != nil {
			_ = d.cmd.Process.Kill()
		}
		select {
		case <-d.waitDone:
		case <-time.After(workflowBrowserKillWait):
		}
	})
}

func (d *workflowBrowser) New(name, launch string) error {
	ctx, cancel := d.commandContext(nil, workflowBrowserCallTimeout)
	defer cancel()
	_, err := d.call(ctx, map[string]any{"op": "new", "name": name, "launch": launch})
	return err
}
func (d *workflowBrowser) Eval(ctx context.Context, name, script string) (string, error) {
	callCtx, cancel := d.commandContext(ctx, workflowBrowserCallTimeout)
	defer cancel()
	return d.call(callCtx, map[string]any{"op": "eval", "name": name, "script": script})
}
func (d *workflowBrowser) Navigate(name, path string) error {
	ctx, cancel := d.commandContext(nil, workflowBrowserCallTimeout)
	defer cancel()
	_, err := d.call(ctx, map[string]any{"op": "navigate", "name": name, "path": path})
	return err
}
func (d *workflowBrowser) Close(name string) error {
	ctx, cancel := d.commandContext(nil, workflowBrowserCallTimeout)
	defer cancel()
	_, err := d.call(ctx, map[string]any{"op": "close", "name": name})
	return err
}

func (d *workflowBrowser) Capture(name string) (string, error) {
	ctx, cancel := d.commandContext(nil, workflowBrowserCallTimeout)
	defer cancel()
	return d.call(ctx, map[string]any{"op": "capture", "name": name})
}
func (d *workflowBrowser) Finish() error {
	ctx, cancel := d.commandContext(nil, workflowBrowserCallTimeout)
	_, err := d.call(ctx, map[string]any{"op": "finish"})
	cancel()
	d.shutdown()
	select {
	case <-d.waitDone:
		if err == nil && d.waitErr != nil {
			err = errors.New("browser shutdown")
		}
	default:
		if err == nil {
			err = errors.New("browser shutdown")
		}
	}
	return err
}

func TestWorkflowBrowserCommandTimeoutPoisonsAndStopsChild(t *testing.T) {
	for _, mode := range []string{"eof", "hang"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := fakeWorkflowBrowserCommand(ctx, mode)
			d, err := newWorkflowBrowserCommand(ctx, cmd, "http://127.0.0.1:18780")
			if err != nil {
				t.Fatal("fake browser init failed")
			}
			callCtx, callCancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer callCancel()
			if _, err = d.Eval(callCtx, "page", "synthetic"); err == nil || err.Error() != "browser timeout" {
				t.Fatalf("Eval timeout error = %v", err)
			}
			started := time.Now()
			if err = d.New("later", "synthetic"); err == nil || err.Error() != "browser unavailable" {
				t.Fatalf("poisoned New error = %v", err)
			}
			if time.Since(started) > 100*time.Millisecond {
				t.Fatal("poisoned call waited on the abandoned scanner")
			}
			select {
			case <-d.waitDone:
			case <-time.After(workflowBrowserKillWait + workflowBrowserExitGrace + time.Second):
				t.Fatal("fake browser child survived timeout shutdown")
			}
			if mode == "eof" && d.waitErr != nil {
				t.Fatal("EOF-aware child was killed instead of closing gracefully")
			}
			if mode == "hang" && d.waitErr == nil {
				t.Fatal("unresponsive child exited without the kill fallback")
			}
		})
	}
}

func TestWorkflowBrowserDrainsImmediateFinalReply(t *testing.T) {
	for i := 0; i < 12; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		d, err := newWorkflowBrowserCommand(ctx, fakeWorkflowBrowserCommand(ctx, "finish"), "http://127.0.0.1:18780")
		if err != nil {
			cancel()
			t.Fatal("fake browser init failed")
		}
		if err = d.Finish(); err != nil {
			cancel()
			t.Fatalf("iteration %d lost final reply: %v", i, err)
		}
		cancel()
	}
}

func fakeWorkflowBrowserCommand(ctx context.Context, mode string) *exec.Cmd {
	const source = `
const readline = require('node:readline');
const mode = process.argv[1];
const input = readline.createInterface({input: process.stdin});
input.on('line', line => {
  const command = JSON.parse(line);
  if (command.op === 'init') {
    process.stdout.write(JSON.stringify({id: command.id, ok: true, value: 'synthetic'}) + '\n');
  } else if (mode === 'finish') {
    process.stdout.write(JSON.stringify({id: command.id, ok: true, value: 'finished'}) + '\n', () => process.exit(0));
  } else if (mode === 'hang') {
    setInterval(() => {}, 1000);
  }
});`
	return exec.CommandContext(ctx, "node", "-e", source, mode)
}
