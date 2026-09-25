package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestParseAddress(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8766", "127.0.0.2:0", "[::1]:8766"} {
		got, err := parseAddress([]string{"-addr", addr}, io.Discard)
		if err != nil || got != addr {
			t.Fatalf("parse %q = %q, %v", addr, got, err)
		}
	}
	for _, args := range [][]string{
		{"-addr", "0.0.0.0:8766"}, {"-addr", "[::]:8766"}, {"-addr", "localhost:8766"},
		{"-addr", "127.0.0.1:http"}, {"-addr", "127.0.0.1:65536"},
		{"-addr", "[::1%lo0]:8766"}, {"extra"}, {"-unknown"},
	} {
		if _, err := parseAddress(args, io.Discard); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	if got, err := parseAddress(nil, io.Discard); err != nil || got != "127.0.0.1:8766" {
		t.Fatalf("default = %q, %v", got, err)
	}
	if _, err := parseAddress([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help = %v", err)
	}
}

func TestServeCancelsAndReleasesListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	finished := make(chan error, 1)
	go func() {
		finished <- serve(ctx, listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "synthetic")
		}))
	}()
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(body) != "synthetic" {
		t.Fatalf("body = %q, %v", body, err)
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	if conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		conn.Close()
		t.Fatal("listener survived shutdown")
	}
}
