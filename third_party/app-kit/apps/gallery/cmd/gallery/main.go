// Command gallery serves the kit's living style guide on loopback.
//
// The Gallery itself is just an http.Handler (D5); this is the thinnest
// possible shell around it. It binds 127.0.0.1 only — the style guide is a
// local development surface, not a public service.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
	"github.com/geoffmilleraz/app-kit/internal/designassets"
)

func main() {
	cfg, err := parseConfig(os.Args[1:], os.Getenv("APPKIT_DESIGN_ASSETS"), os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gallery: %v\n", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		log.Fatalf("gallery: listen: %v", err)
	}
	log.Printf("app-kit Gallery → http://%s", listener.Addr())
	options := gallery.Options{DesignAssetsRoot: cfg.root}
	if cfg.themeSource != "" {
		source, err := readThemeSource(cfg.themeSource)
		if err != nil {
			log.Fatalf("gallery: %v", err)
		}
		identity, err := executableIdentity()
		if err != nil {
			log.Fatalf("gallery: theme lab build identity: %v", err)
		}
		options.ThemeLab, err = gallery.NewThemeLab("http://"+listener.Addr().String(), source, cfg.themeSource, identity)
		if err != nil {
			log.Fatalf("gallery: theme lab: %v", err)
		}
		log.Printf("experimental theme lab → http://%s/theme-lab", listener.Addr())
	}
	if err := serveGallery(ctx, listener, gallery.NewWithOptions(options)); err != nil {
		log.Fatalf("gallery: %v", err)
	}
}

// serveGallery owns the listener and the request lifetime. It is a command-local
// HTTP seam, not a public runtime/session API or native-host support claim.
func serveGallery(ctx context.Context, listener net.Listener, handler http.Handler) error {
	return serveGalleryWithShutdownTimeout(ctx, listener, handler, 5*time.Second)
}

func serveGalleryWithShutdownTimeout(ctx context.Context, listener net.Listener, handler http.Handler, timeout time.Duration) error {
	requestContext, cancelRequests := context.WithCancel(ctx)
	defer cancelRequests()
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return requestContext },
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()
	var serveErr error
	serveReturned := false
	select {
	case <-ctx.Done():
	case serveErr = <-served:
		serveReturned = true
	}
	// Shutdown alone waits for active streams. Cancel their shared request
	// context first, including when the listener fails independently of ctx.
	cancelRequests()
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), timeout)
	defer cancelShutdown()
	shutdownErr := srv.Shutdown(shutdownContext)
	var closeErr error
	if shutdownErr != nil {
		closeErr = srv.Close()
		shutdownErr = fmt.Errorf("shutdown: %w", shutdownErr)
	}
	if !serveReturned {
		serveErr = <-served
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(serveErr, shutdownErr, closeErr)
}

type config struct{ addr, root, themeSource string }

func parseConfig(args []string, environmentRoot string, output io.Writer) (config, error) {
	cfg := config{addr: "127.0.0.1:8765", root: "data/design-assets"}
	if environmentRoot != "" {
		cfg.root = environmentRoot
	}
	fs := flag.NewFlagSet("gallery", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&cfg.addr, "addr", cfg.addr, "loopback address to serve on")
	fs.StringVar(&cfg.root, "design-assets", cfg.root, "private design archive store (APPKIT_DESIGN_ASSETS); empty disables")
	fs.StringVar(&cfg.themeSource, "theme-source", "", "explicit trusted CUE token file for the local experimental theme lab (read-only)")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		return cfg, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	host, _, err := net.SplitHostPort(cfg.addr)
	if err != nil {
		return cfg, fmt.Errorf("bad -addr %q: %w", cfg.addr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return cfg, fmt.Errorf("-addr must be a loopback IP (got %q); the style guide is local-only", host)
	}
	explicit := environmentRoot != ""
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "design-assets" {
			explicit = true
		}
	})
	if explicit && cfg.root != "" {
		if _, err := designassets.Load(cfg.root); err != nil {
			return cfg, fmt.Errorf("selected design-assets store is unavailable: %w; import it first or pass -design-assets='' to disable", err)
		}
	}
	return cfg, nil
}
