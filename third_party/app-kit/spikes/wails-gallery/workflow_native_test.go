//go:build darwin && !server && workflowprobe && appkit_contained

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Keep TestMain on the native main thread. NSApp termination may not return
// through Go defers; the receipt and HTTP teardown belong to OnShutdown.
func workflowNativeMain(ctx context.Context, h *host, meter *workflowMeter, initial *workflowReceipt) {
	if requireNativeContainment() != nil {
		return
	}
	completed := make(chan workflowReceipt, 1)
	var frameworkErrors, frameworkWarnings, bridge atomic.Int32
	app := application.New(application.Options{
		Name: "App Kit Gallery Workflow Probe", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Assets: httpOnlyAssets(), DisableDefaultSignalHandler: true,
		// Explicit teardown closes both windows before Quit; automatic last-window
		// termination would race that evidence. No window security option differs.
		Mac:          application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		ErrorHandler: func(error) { frameworkErrors.Add(1) }, WarningHandler: func(string) { frameworkWarnings.Add(1) },
		RawMessageHandler: func(application.Window, string, *application.OriginInfo) { bridge.Add(1) },
		OnShutdown: func() {
			r := *initial
			select {
			case r = <-completed:
			default:
				r.Failure = "native-shutdown-before-workflow"
			}
			r.FrameworkErrors = int(frameworkErrors.Load())
			r.FrameworkWarnings = int(frameworkWarnings.Load())
			r.BridgeMessages = int(bridge.Load())
			finishWorkflowHost(h, meter, &r)
			writeWorkflowReceipt(r)
		},
	})
	menu := app.NewMenu()
	menu.AddRole(application.AppMenu)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.WindowMenu)
	app.Menu.Set(menu)
	d := &workflowNative{ctx: ctx, app: app, origin: h.baseURL, windows: map[string]*application.WebviewWindow{}, loaded: map[string]chan struct{}{}}
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() { r := *initial; runWorkflow(ctx, h, meter, d, &r); completed <- r; app.Quit() }()
	})
	watchdog := time.AfterFunc(95*time.Second, func() {
		// Hard failure must not wait for graceful HTTP teardown. Process exit
		// closes owned descriptors; no success receipt is emitted.
		fmt.Fprintln(os.Stderr, "native workflow hard deadline")
		os.Exit(2)
	})
	defer watchdog.Stop()
	if app.Run() != nil {
		fmt.Fprintln(os.Stderr, "native workflow event loop failed")
	}
}

type workflowNative struct {
	ctx     context.Context
	app     *application.App
	origin  string
	windows map[string]*application.WebviewWindow
	loaded  map[string]chan struct{}
}

func (d *workflowNative) New(name, launch string) error {
	if len(d.windows) >= 2 || d.windows[name] != nil {
		return errors.New("native window limit")
	}
	window := application.NewWindow(galleryNativeWindowOptions("workflow-"+name, launch, d.origin))
	loaded := make(chan struct{}, 1)
	window.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(*application.WindowEvent) {
		select {
		case loaded <- struct{}{}:
		default:
		}
	})
	d.windows[name] = window
	d.loaded[name] = loaded
	d.app.Window.Add(window)
	window.Run()
	return d.waitLoaded(name)
}
func (d *workflowNative) waitLoaded(name string) error {
	select {
	case <-d.loaded[name]:
		return nil
	case <-d.ctx.Done():
		return d.ctx.Err()
	case <-time.After(8 * time.Second):
		return errors.New("native navigation timeout")
	}
}
func (d *workflowNative) Eval(ctx context.Context, name, script string) (string, error) {
	window := d.windows[name]
	if window == nil {
		return "", errors.New("native window missing")
	}
	return workflowNativeEval(ctx, window, script)
}
func (d *workflowNative) Navigate(name, path string) error {
	window := d.windows[name]
	if window == nil {
		return errors.New("native window missing")
	}
	if path != "/shell/profile" && path != "/shell/activity" && path != "/shell/settings" {
		return errors.New("native navigation path refused")
	}
	for {
		select {
		case <-d.loaded[name]:
			continue
		default:
			goto drained
		}
	}
drained:
	window.SetURL(d.origin + path)
	return d.waitLoaded(name)
}
func (d *workflowNative) Close(name string) error {
	window := d.windows[name]
	if window == nil {
		return errors.New("native window missing")
	}
	window.Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := d.app.Window.GetByName("workflow-" + name); !ok {
			delete(d.windows, name)
			delete(d.loaded, name)
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("native window close timeout")
}
func (d *workflowNative) Finish() error {
	if len(d.windows) != 0 || len(d.app.Window.GetAll()) != 0 {
		return errors.New("native windows remain")
	}
	return nil
}
