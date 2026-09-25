//go:build !server && (darwin || windows)

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func runNative(ctx context.Context, h *host, diagnostics io.Writer) error {
	if err := requireNativeContainment(); err != nil {
		return err
	}
	return runNativeSession(ctx, nativeSession{
		origin: h.baseURL, launch: h.auth.LaunchURL,
		browser: func() error { return openBrowser(h) }, done: h.done, shutdown: h.Close,
	}, diagnostics)
}

// Private lifecycle callbacks keep both entries on exactly the same native window
// policy. In companion mode HTTP and domain state are owned by the parent.
type nativeSession struct {
	origin   string
	launch   func() (string, error)
	browser  func() error
	done     <-chan struct{}
	shutdown func() error
	ready    func()
	closed   func()
}

func runNativeSession(ctx context.Context, session nativeSession, diagnostics io.Writer) error {
	if ctx.Err() != nil {
		return session.shutdown()
	}
	if err := requireNativeContainment(); err != nil {
		return err
	}
	var stopping atomic.Bool
	app := application.New(application.Options{
		Name:        "App Kit Gallery Experiment",
		Description: "Synthetic HTTP host proof; not a production desktop adapter",
		// Wails can log navigation URLs. Suppress framework logging entirely
		// while this specimen uses one-shot bearer URLs for bootstrap.
		Logger:                      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Assets:                      httpOnlyAssets(),
		Mac:                         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		DisableDefaultSignalHandler: true,
		OnShutdown: func() {
			stopping.Store(true)
			if err := session.shutdown(); err != nil {
				fmt.Fprintln(diagnostics, "HTTP shutdown did not complete cleanly")
			}
		},
		PostShutdown: session.closed,
		// No Services, event-based privileged actions or custom IPC transport.
	})
	windowID := 0
	var readyOnce sync.Once
	var windowMu sync.Mutex
	navigationReady := session.ready == nil
	var updateControls func()
	// The manager invokes this before Run can show each native window. Attaching
	// after NewWithOptions returns can miss a fast close of a later window.
	// Attachment only: newWindow already holds windowMu during this callback.
	app.Window.OnCreate(func(window application.Window) {
		// Wails drops late WindowWillClose events after manager removal. Its
		// Common.WindowClosing listeners are captured together, but dispatched
		// concurrently. Observe the manager's committed removal with a bounded
		// wait; do not create a second, eventually divergent window registry.
		window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
			tick := time.NewTicker(10 * time.Millisecond)
			defer tick.Stop()
			limit := time.NewTimer(2 * time.Second)
			defer limit.Stop()
			for {
				if stopping.Load() {
					return
				}
				if _, exists := app.Window.GetByID(window.ID()); !exists {
					windowMu.Lock()
					if !stopping.Load() && len(app.Window.GetAll()) != 0 {
						updateControls()
					}
					windowMu.Unlock()
					return
				}
				select {
				case <-tick.C:
				case <-ctx.Done():
					return
				case <-session.done:
					return
				case <-limit.C:
					fmt.Fprintln(diagnostics, "Native window close did not settle; no capacity was released early.")
					return
				}
			}
		})
	})
	newWindow := func() error {
		windowMu.Lock()
		defer windowMu.Unlock()
		if stopping.Load() {
			return errors.New("native host is closing")
		}
		if len(app.Window.GetAll()) >= 2 {
			return errors.New("this specimen permits at most two native windows")
		}
		launch, err := session.launch()
		if err != nil {
			return errors.New("could not issue native launch")
		}
		windowID++
		options := galleryNativeWindowOptions("gallery-"+strconv.Itoa(windowID), launch, session.origin)
		window := app.Window.NewWithOptions(options)
		if windowID == 1 && session.ready != nil {
			window.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(*application.WindowEvent) {
				readyOnce.Do(func() {
					session.ready()
					windowMu.Lock()
					defer windowMu.Unlock()
					navigationReady = true
					updateControls()
				})
			})
		}
		updateControls()
		return nil
	}
	menu := app.NewMenu()
	if runtime.GOOS == "darwin" {
		menu.AddRole(application.AppMenu)
	}
	menu.AddRole(application.EditMenu)
	views := menu.AddSubmenu("Gallery")
	newView := views.Add("New comparison window").SetAccelerator("CmdOrCtrl+n").OnClick(func(*application.Context) {
		if err := newWindow(); err != nil {
			fmt.Fprintln(diagnostics, err)
		}
	})
	browserView := views.Add("Open in browser").OnClick(func(*application.Context) {
		if err := session.browser(); err != nil {
			fmt.Fprintln(diagnostics, err)
		}
	})
	// All state changes and both native menu setters share windowMu once running.
	updateControls = func() {
		available := navigationReady && !stopping.Load()
		newView.SetEnabled(available && len(app.Window.GetAll()) < 2)
		browserView.SetEnabled(available)
	}
	updateControls()
	menu.AddRole(application.WindowMenu)
	app.Menu.Set(menu)
	if err := newWindow(); err != nil {
		return err
	}
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		// Quit before Run is not a portable Wails lifecycle guarantee. Observe
		// already-cancelled contexts here, after the native event loop started.
		go func() {
			select {
			case <-ctx.Done():
			case <-session.done:
			}
			if !stopping.Swap(true) {
				app.Quit()
			}
		}()
	})
	if err := app.Run(); err != nil {
		// Run failures may include navigation URLs. Report the category only.
		return errors.New("native host failed; rerun with -browser for the HTTP fallback")
	}
	return session.shutdown()
}

// Keep the opt-in workflow probe on exactly the host's window policy.
func galleryNativeWindowOptions(name, launch, origin string) application.WebviewWindowOptions {
	preferences := application.MacWebviewPreferences{}
	preferences.TabFocusesLinks.Set(true)
	preferences.JavaScriptCanOpenWindowsAutomatically.Set(false)
	options := application.WebviewWindowOptions{
		Name: name, Title: "App Kit Gallery (experimental)", URL: launch,
		Width: 1200, Height: 850, MinWidth: 390, MinHeight: 400,
		UseApplicationMenu: true, AllowSimpleEventEmit: false,
		// Beta.21 Darwin ignores this map. The contained media patch enforces
		// camera/microphone denial only; the other entries are intentions, not proof.
		Permissions: map[application.PermissionType]application.Permission{
			application.PermissionMicrophone:    application.PermissionDeny,
			application.PermissionCamera:        application.PermissionDeny,
			application.PermissionGeolocation:   application.PermissionDeny,
			application.PermissionNotifications: application.PermissionDeny,
			application.PermissionClipboardRead: application.PermissionDeny,
		},
		Mac: application.MacWindow{WebviewPreferences: preferences},
	}
	configureNativeWindow(&options, origin)
	return options
}

func httpOnlyAssets() application.AssetOptions {
	return application.AssetOptions{
		Handler: http.NotFoundHandler(), DisableLogging: true,
		// This wraps framework routing; Handler alone is a later fallback.
		// Domain traffic uses h's separate real HTTP listener, not Wails assets.
		Middleware: func(http.Handler) http.Handler { return http.NotFoundHandler() },
	}
}
