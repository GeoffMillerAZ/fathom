// Command receipts serves the first-party synthetic analytical reference locally.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/apps/receipts"
)

func main() {
	addr, err := parseAddress(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "receipts: %v\n", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, addr, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "receipts: %v\n", err)
		os.Exit(1)
	}
}

func parseAddress(args []string, output io.Writer) (string, error) {
	addr := "127.0.0.1:8766"
	flags := flag.NewFlagSet("receipts", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&addr, "addr", addr, "literal loopback IP and port")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if flags.NArg() != 0 {
		return "", errors.New("unexpected positional arguments")
	}
	ap, err := netip.ParseAddrPort(addr)
	if err != nil || !ap.Addr().IsLoopback() || ap.Addr().Zone() != "" {
		return "", errors.New("-addr must be a literal loopback IP and numeric port")
	}
	return ap.String(), nil
}

func run(ctx context.Context, addr string, output io.Writer) error {
	handler, err := receipts.New()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "app-kit Receipts: http://%s (synthetic data only)\n", listener.Addr())
	return serve(ctx, listener, handler)
}

// serve owns the listener. Cancellation drains this bounded read-only surface.
func serve(ctx context.Context, listener net.Listener, handler http.Handler) error {
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
		IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	var closeErr error
	if shutdownErr != nil {
		closeErr = server.Close()
	}
	serveErr := <-done
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(serveErr, shutdownErr, closeErr)
}
