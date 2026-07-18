# governance-wasm

> One policy, runs in-pod **and** at the edge/browser.

This example compiles the pure-Go governance foundations — proof-of-work,
prefilter — to a single WebAssembly binary with no external dependencies.
The identical policy code that protects your backend also runs in the browser
and at CDN edge workers (Cloudflare Workers, Fastly Compute, Deno Deploy),
giving you a single source of truth for abuse-protection rules.

## Key rationale

| Layer | What runs | Why it matters |
|-------|-----------|----------------|
| Browser | `govSolvePoW` | PoW work is paid by the **attacker's** CPU, not your servers |
| Edge worker | `govPrefilter` | Reject empty/oversized payloads before they reach origin |
| In-pod / server | All foundations | Same compiled code, same behaviour — no drift |

## Build

```sh
# 1. Compile the WASM binary
GOOS=js GOARCH=wasm go build -o examples/governance-wasm/main.wasm \
    ./examples/governance-wasm

# Or via Task (outputs to workspace/wasm/main.wasm):
task wasm-build

# 2. Copy wasm_exec.js from the Go toolchain
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" examples/governance-wasm/

# 3. Serve (any static server works)
python3 -m http.server 8080 --directory examples/governance-wasm
# → open http://localhost:8080/
```

## JS API (registered on globalThis)

| Function | Signature | Description |
|----------|-----------|-------------|
| `govSolvePoW` | `(challengeJSON: string) => string` | Brute-force solve a PoW challenge; burns client CPU |
| `govVerifyPoW` | `(challengeJSON: string, solution: string) => bool` | **Local pre-check only**: solution meets difficulty + not expired |
| `govPrefilter` | `(text: string, maxLen: int) => "ok"\|"empty"\|"too_long"` | Cheap length/empty check for edge workers |

Challenge JSON shape (matches `proofofwork.Challenge`; the `MAC` is the
server's HMAC-SHA256 signature over the other fields — treat the whole
challenge as opaque and return it verbatim with your solution):

```json
{
  "Nonce":     "abc:1234567890",
  "Bits":      12,
  "ExpiresAt": "2099-01-01T00:00:00Z",
  "MAC":       "<64 hex chars issued by the server>"
}
```

> **Security note** — challenges are HMAC-signed server-side and only the
> server can verify them: `proofofwork.Issuer.Verify` authenticates the MAC
> (constant-time), enforces a minimum-difficulty floor and expiry, and
> rejects replays via a bounded seen-set. The browser's only job is to burn
> CPU on a server-issued challenge; `govVerifyPoW` is a convenience pre-check
> that proves nothing to anyone else.

## Host build is unaffected

`main.go` carries `//go:build js && wasm`, so it is excluded from the normal
host build:

```sh
go build ./...            # succeeds — build tag excludes the shim
go vet ./...              # clean
task validate             # full gate still passes
```

## Extending

- **Rate limiter**: `ratelimiter.New` / `ratelimiter.KeyedLimiter` are already
  WASM-safe (no file I/O, injectable clock). Add `govRateLimit` following the
  same pattern.
- **Persistent SeenSet** (server-side, not in the browser): implement
  `proofofwork.SeenSet` against a shared KV store and pass it to
  `proofofwork.NewIssuerWithSeenSet` so replay protection spans replicas.
  Honour the fail-closed contract: never evict unexpired entries at the cap.
- **Bundling**: `wasm_exec.js` can be inlined or loaded from a CDN; the
  `.wasm` binary itself is the only Go artifact.
