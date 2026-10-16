//go:build js && wasm

// Package main is a thin WASM shim that registers governance primitives as
// JS-callable functions. It delegates entirely to the already-tested
// foundations — no policy logic lives here.
//
// Proof-of-work challenges are HMAC-signed by the server; the secret never
// leaves it. The browser's job is only to BURN CPU on a server-issued
// challenge (govSolvePoW) — authoritative verification (MAC authenticity,
// difficulty floor, expiry, single-use replay set) happens exclusively
// server-side via proofofwork.Issuer.Verify. govVerifyPoW here is a local
// pre-check so a client can confirm its solution meets the difficulty before
// submitting; it proves nothing to anyone else.
//
// Build:
//
//	GOOS=js GOARCH=wasm go build -o main.wasm ./examples/governance-wasm
//
// Copy wasm_exec.js from $(go env GOROOT)/lib/wasm/wasm_exec.js into the
// same directory as index.html before serving.
package main

import (
	"encoding/json"
	"syscall/js"
	"time"
	"unicode/utf8"

	proofofwork "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/proof-of-work"
)

func main() {
	js.Global().Set("govSolvePoW", js.FuncOf(govSolvePoW))
	js.Global().Set("govVerifyPoW", js.FuncOf(govVerifyPoW))
	js.Global().Set("govPrefilter", js.FuncOf(govPrefilter))

	// Block forever — the WASM module is event-driven from the JS side.
	select {}
}

// govSolvePoW parses a Challenge JSON string, calls proofofwork.Solve, and
// returns the solution string. This burns the client's CPU, not the server's.
// The challenge is opaque to the client: the MAC field is carried along
// untouched and must be returned to the server verbatim together with the
// solution.
//
// JS signature: govSolvePoW(challengeJSON: string) => string
func govSolvePoW(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return js.ValueOf("error: missing challengeJSON argument")
	}
	raw := args[0].String()

	var c proofofwork.Challenge
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return js.ValueOf("error: " + err.Error())
	}

	solution := proofofwork.Solve(c)
	return js.ValueOf(solution)
}

// govVerifyPoW is a CLIENT-SIDE PRE-CHECK ONLY: it parses a Challenge JSON
// string and reports whether the solution meets the challenge's difficulty
// and the challenge is not yet expired. It cannot — by design — perform
// authoritative verification: that requires the HMAC secret (to authenticate
// the challenge) and the replay seen-set, both of which live only on the
// server (proofofwork.Issuer.Verify).
//
// JS signature: govVerifyPoW(challengeJSON: string, solution: string) => bool
func govVerifyPoW(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return js.ValueOf(false)
	}
	raw := args[0].String()
	solution := args[1].String()

	var c proofofwork.Challenge
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return js.ValueOf(false)
	}
	if !time.Now().Before(c.ExpiresAt) {
		return js.ValueOf(false)
	}
	return js.ValueOf(proofofwork.MeetsDifficulty(c, solution))
}

// govPrefilter is a cheap edge-side prefilter: checks for empty input and
// enforces a maximum rune-length without any model or embedding call.
// An edge worker can run this before forwarding to origin.
//
// JS signature: govPrefilter(text: string, maxLen: int) => "ok"|"empty"|"too_long"
func govPrefilter(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return js.ValueOf("empty")
	}
	text := args[0].String()
	maxLen := args[1].Int()

	if text == "" {
		return js.ValueOf("empty")
	}
	if maxLen > 0 && utf8.RuneCountInString(text) > maxLen {
		return js.ValueOf("too_long")
	}
	return js.ValueOf("ok")
}
