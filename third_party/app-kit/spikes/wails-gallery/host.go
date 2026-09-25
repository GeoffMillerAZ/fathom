package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

// host belongs only to this experiment. It owns the listener and cancels active
// HTTP requests before waiting for shutdown, including long-lived SSE requests.
type host struct {
	baseURL string
	auth    *bootstrap
	cancel  context.CancelFunc
	done    chan struct{}
	err     error // published by closing done
}

func startHost(ctx context.Context, handler http.Handler) (*host, error) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	auth, err := newBootstrap(ln.Addr().String(), handler)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	requests, cancel := context.WithCancel(ctx)
	h := &host{baseURL: "http://" + ln.Addr().String(), auth: auth, cancel: cancel, done: make(chan struct{})}
	srv := &http.Server{
		Handler: auth, ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32 << 10,
		BaseContext: func(net.Listener) context.Context { return requests },
		// Do not let a handler panic/error print a secret-bearing request URL.
		// The specimen's terminal error is reported separately without URLs.
		ErrorLog: log.New(io.Discard, "", 0),
	}
	go func() {
		defer close(h.done)
		served := make(chan error, 1)
		go func() { served <- srv.Serve(ln) }()
		var serveErr error
		returned := false
		select {
		case <-requests.Done():
		case serveErr = <-served:
			returned = true
		}
		cancel()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		shutdownErr := srv.Shutdown(shutdown)
		var closeErr error
		if shutdownErr != nil {
			closeErr = srv.Close()
		}
		if !returned {
			serveErr = <-served
		}
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		h.err = errors.Join(serveErr, shutdownErr, closeErr)
	}()
	return h, nil
}

func (h *host) Close() error {
	h.cancel()
	<-h.done
	return h.err
}

func (h *host) Wait() error {
	<-h.done
	return h.err
}
