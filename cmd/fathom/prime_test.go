package main

// prime_test.go — WF-8 (docs/plans/work-frontier.md): `fathom prime` prints
// the full session primer as plain text — the same content the SessionStart
// hook (cmdSessionHook) composes into its JSON envelope's additionalContext
// — so a hook-less harness (Codex) pulls in one command what Claude Code
// gets pushed at session start.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakePrimeDaemon serves healthz, list_services (one indexed service named
// after the repo dir), and query_symbols at /rpc/agent (the live-example
// line's endpoint) — everything buildSessionContext needs to render every
// section of the primer.
func fakePrimeDaemon(t *testing.T, service string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case "/rpc":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"services\":[{\"service\":\"` + service + `\",\"symbol_count\":42,\"languages\":[\"go\"]}]}"}]}}`))
		case "/rpc/agent":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"nodes\":[{\"name\":\"BuildModel\"}]}"}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestPrimeMatchesSessionHookPayload asserts `fathom prime`'s plain-text
// stdout carries every content section the SessionStart hook's
// additionalContext carries for the same fake inputs: service identity +
// freshness (buildSessionContext), the live example query, the continuity
// thread brief, and the custodian attention line.
func TestPrimeMatchesSessionHookPayload(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate from any user-scope concerns.cue
	threadEnv(t)                  // FATHOM_THREADS + FATHOM_SESSION_STAMPS on temp paths
	repoDir := t.TempDir()
	svc := strings.ToLower(filepath.Base(repoDir))

	srv := fakePrimeDaemon(t, svc)
	defer srv.Close()

	// A thread item local to this repo, so the brief carries substantive
	// content (not just its "nothing deposited" fallback).
	// Filed NOW, not at seedItem's fixed fixture date: both cmdPrime and
	// cmdSessionHook read the real clock, so a fixed date makes this test
	// pass until the item crosses the brief's 14-day horizon and then fail
	// on a day nobody changed anything.
	seedItem(t, "th-prime", "handoff-note", "prime parity fixture note", func(it *threadItem) {
		it.Pointers = map[string]string{"repo": svc}
		it.FiledAt = time.Now().UTC().Format(time.RFC3339)
	})

	// A violated custodian concern, so the attention line renders too.
	writeConcernFixture(t, repoDir)

	var hookOut, hookErr bytes.Buffer
	if code := cmdSessionHook([]string{"--url", srv.URL, "--dir", repoDir}, strings.NewReader("{}"), &hookOut, &hookErr); code != 0 {
		t.Fatalf("session-hook exit %d: %s", code, hookErr.String())
	}
	var env struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(hookOut.Bytes(), &env); err != nil {
		t.Fatalf("decoding hook JSON: %v\n%s", err, hookOut.String())
	}
	hookCtx := env.HookSpecificOutput.AdditionalContext
	if hookCtx == "" {
		t.Fatal("hook payload additionalContext is empty — fixture is broken")
	}

	var primeOut, primeErr bytes.Buffer
	if code := cmdPrime([]string{"--url", srv.URL, "--dir", repoDir}, &primeOut, &primeErr); code != 0 {
		t.Fatalf("prime exit %d: %s", code, primeErr.String())
	}
	primeCtx := strings.TrimRight(primeOut.String(), "\n")

	if primeCtx != hookCtx {
		t.Fatalf("prime output does not match the hook payload:\nprime:\n%s\n\nhook:\n%s", primeCtx, hookCtx)
	}

	for _, want := range []string{
		`service "` + svc + `"`, // identity
		"42 symbols",            // freshness
		"first query to try",    // live example
		"BuildModel",
		"prime parity fixture note", // thread brief
		"⚠️ attention: config 1",    // the composed attention line (EP-19)
	} {
		if !strings.Contains(primeCtx, want) {
			t.Errorf("prime output missing %q:\n%s", want, primeCtx)
		}
	}
}

// TestPrimeExitsZeroOnDaemonDown pins WF-8's degraded-branch contract:
// prime must never fail just because the daemon is unreachable — it prints
// the same degraded line buildSessionContext would hand the hook.
func TestPrimeExitsZeroOnDaemonDown(t *testing.T) {
	threadEnv(t)
	var out, errB bytes.Buffer
	code := cmdPrime([]string{"--url", "http://127.0.0.1:1", "--dir", t.TempDir()}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d, want 0 even when the daemon is unreachable: %s", code, errB.String())
	}
	if !strings.Contains(out.String(), "not responding") {
		t.Errorf("expected the degraded daemon-down line, got: %s", out.String())
	}
}
