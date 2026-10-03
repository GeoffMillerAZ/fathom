package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// servemcp_front.go — CS-2 (docs/plans/cloud-sessions.md): `fathom
// serve-mcp --url <daemon rpc url>` is a stdio front for a running
// serve-http daemon. The front opens no store: each newline-delimited
// JSON-RPC message read from stdin is POSTed to the URL and the daemon's
// answer is written back as one line, so any number of stdio clients share
// the one daemon that holds the store (and that the hooks talk to).
//
// The case it exists for is a cloud session (docs/adoption/cloud-sessions.md):
// the VM runs one daemon on 127.0.0.1:7575 for the hook stack, and the
// harness can only be handed a stdio command before that daemon is known to
// be up — so the front waits for /healthz first and answers every request
// that carries an id, with a JSON-RPC error naming the daemon when it cannot
// reach it. A harness never waits on a call nobody will answer.

const (
	// frontCallTimeout bounds one forwarded call. The daemon carries its own
	// per-class deadlines and runs ingest-class calls detached; this only
	// stops a wedged connection from hanging the harness forever.
	frontCallTimeout = 10 * time.Minute
	// frontMaxAnswer caps one answer read from the daemon.
	frontMaxAnswer = 64 << 20
)

// runServeMCPFront forwards stdin's JSON-RPC lines to rpcURL until stdin
// ends. Exit 0 at end of input; 1 when stdin itself cannot be read.
func runServeMCPFront(ctx context.Context, rpcURL string, wait time.Duration, stdin io.Reader, stdout, stderr io.Writer) int {
	u, err := url.Parse(rpcURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		fmt.Fprintf(stderr, "error: --url %q is not an http(s) URL of a serve-http RPC path (e.g. http://127.0.0.1:7575/rpc/agent)\n", rpcURL)
		return 1
	}
	fmt.Fprintf(stderr, "serve-mcp: stdio front for %s (no store opened here) %s\n", rpcURL, BuildRevision())

	hc := &http.Client{Timeout: frontCallTimeout}
	healthz := u.Scheme + "://" + u.Host + "/healthz"
	if wait > 0 && !waitForHealthz(ctx, hc, healthz, wait) {
		fmt.Fprintf(stderr, "serve-mcp: the daemon at %s did not answer /healthz within %s; forwarding anyway (each call is answered with an error until it does)\n", u.Host, wait)
	}

	protocol := ""
	in := bufio.NewReader(stdin)
	for {
		line, readErr := in.ReadBytes('\n')
		if msg := bytes.TrimSpace(line); len(msg) > 0 {
			answer, negotiated := forwardFrontLine(ctx, hc, rpcURL, protocol, msg)
			if negotiated != "" {
				protocol = negotiated
			}
			if len(answer) > 0 {
				if _, err := stdout.Write(append(answer, '\n')); err != nil {
					fmt.Fprintf(stderr, "serve-mcp: writing to stdout: %v\n", err)
					return 1
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return 0
			}
			fmt.Fprintf(stderr, "serve-mcp: reading stdin: %v\n", readErr)
			return 1
		}
	}
}

// waitForHealthz polls the daemon's liveness probe until it answers ok or
// wait elapses.
func waitForHealthz(ctx context.Context, hc *http.Client, healthz string, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for {
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, healthz, nil)
		if err == nil {
			if resp, err := hc.Do(req); err == nil {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok" {
					cancel()
					return true
				}
			}
		}
		cancel()
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// forwardFrontLine POSTs one JSON-RPC message and returns the one line to
// write back (nil for a notification's empty answer) and, for an initialize
// answer, the protocol revision the daemon negotiated.
func forwardFrontLine(ctx context.Context, hc *http.Client, rpcURL, protocol string, msg []byte) ([]byte, string) {
	var env struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	_ = json.Unmarshal(msg, &env) // a malformed line still goes to the daemon, which answers the parse error
	hasID := len(env.ID) > 0 && string(env.ID) != "null"

	fail := func(why string) ([]byte, string) {
		if !hasID {
			return nil, ""
		}
		out, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      env.ID,
			"error":   map[string]any{"code": -32603, "message": fmt.Sprintf("fathom daemon at %s did not answer: %s", rpcURL, why)},
		})
		return out, ""
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(msg))
	if err != nil {
		return fail(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if protocol != "" {
		req.Header.Set("MCP-Protocol-Version", protocol)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fail(err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, frontMaxAnswer))
	if err != nil {
		return fail("reading the answer: " + err.Error())
	}
	if len(bytes.TrimSpace(body)) == 0 {
		if resp.StatusCode/100 == 2 {
			return nil, "" // a notification: no answer is the answer
		}
		return fail(resp.Status)
	}
	var line bytes.Buffer
	if err := json.Compact(&line, body); err != nil {
		// Not JSON (a plain-text 404 for a wrong surface path, a proxy's
		// error page): say the status and the first words.
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fail(resp.Status + ": " + snippet)
	}
	negotiated := ""
	if env.Method == "initialize" {
		var ans struct {
			Result struct {
				ProtocolVersion string `json:"protocolVersion"`
			} `json:"result"`
		}
		if json.Unmarshal(line.Bytes(), &ans) == nil {
			negotiated = ans.Result.ProtocolVersion
		}
	}
	return line.Bytes(), negotiated
}
