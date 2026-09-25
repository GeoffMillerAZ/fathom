// Command wails-gallery is the explicitly experimental G6 native-host specimen.
// It does not expose a reusable shell API or claim platform support.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cli/browser"
	"github.com/geoffmilleraz/app-kit/apps/gallery"
)

var browserMu sync.Mutex

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "gallery-host:", err)
		os.Exit(1)
	}
}

func run(args []string, diagnostics io.Writer) error {
	flags := flag.NewFlagSet("gallery-host", flag.ContinueOnError)
	flags.SetOutput(diagnostics)
	browserMode := flags.Bool("browser", false, "open the same synthetic HTTP app in the OS-selected browser")
	nativeHost := flags.String("native-host", "", "opt-in macOS companion: absolute executable path (requires -native-host-sha256)")
	nativeSHA := flags.String("native-host-sha256", "", "exact SHA256 of the selected native companion")
	nativeChild := flags.Bool("native-child", false, "private companion entry; requires an inherited control socket")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if (*nativeHost == "") != (*nativeSHA == "") || (*browserMode && (*nativeHost != "" || *nativeChild)) || (*nativeChild && *nativeHost != "") {
		return errors.New("select -browser, direct native, or paired -native-host/-native-host-sha256 flags")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *nativeChild {
		return runNativeChild(ctx, diagnostics)
	}
	var companion *preparedCompanion
	if *nativeHost != "" {
		var err error
		companion, err = prepareCompanion(*nativeHost, *nativeSHA)
		if err != nil {
			return err // identity failures must not become successful browser recovery
		}
		defer companion.close()
	}
	h, err := startHost(ctx, gallery.New()) // no private corpora or real consumer data
	if err != nil {
		return fmt.Errorf("HTTP startup: %w", err)
	}
	defer h.Close()
	fmt.Fprintln(diagnostics, "Experimental synthetic Gallery at", h.baseURL, "(authorization required)")
	fmt.Fprintln(diagnostics, "Native navigation/bridge containment, signed delivery and platform conformance are not certified.")
	if *browserMode {
		return runBrowser(h)
	}
	if companion != nil {
		return runCompanion(ctx, h, companion.path, diagnostics, nil, 15*time.Second)
	}
	return runNative(ctx, h, diagnostics)
}

// The URL is an expiring capability. It is passed directly to the chosen opener,
// never emitted in diagnostics or forwarded in an opener error.
func openBrowser(h *host) error {
	browserMu.Lock()
	defer browserMu.Unlock()
	launch, err := h.auth.LaunchURL()
	if err != nil {
		return errors.New("could not issue browser launch")
	}
	// Child opener output can include arguments; keep it out of diagnostics.
	browser.Stdout, browser.Stderr = io.Discard, io.Discard
	if err := browser.OpenURL(launch); err != nil {
		return errors.New("browser launch failed; no launch credential was logged")
	}
	return nil
}

func runBrowser(h *host) error {
	if err := openBrowser(h); err != nil {
		return err
	}
	// Explicit experiment lifetime: Ctrl-C closes the server. Tab-based idle
	// detection is not implemented or advertised as working.
	return h.Wait()
}
