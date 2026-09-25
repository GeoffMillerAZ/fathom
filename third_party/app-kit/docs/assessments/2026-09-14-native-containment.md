# Actual macOS HTTP-only containment evidence

Date: 2026-09-14. Scope: the unpublished Wails Gallery experiment, not a released
adapter or completed G6. Decision: [ADR-0035](../adr/0035-reproducible-macos-http-only-experiment.md).
The owner-approved web/macOS target scope stands; Windows remains deferred.

## Outcome at a glance

| Actual WKWebView cases | Unchanged beta.21 | Patched HTTP-only profile |
|---|---|---|
| Bootstrap, same-origin redirect, reload | 3/3 pass | 3/3 pass |
| Foreign port, hostname and redirect | 3/3 fail | 3/3 pass |
| `about:` and `wails:` navigation | 2/2 fail | 2/2 pass |
| Same-origin and foreign subframes | 2/2 fail | 2/2 pass |
| Native script-message/runtime exposure | fails; benign canary reaches Go once | passes; zero host canaries |
| Data URL and two popup refusals | 3/3 pass already | 3/3 pass preserved |
| Total | **6/14 pass; verifier exit 1** | **14/14 pass; verifier exit 0** |

Both runs completed the native shutdown callback, removed each managed window before
opening the next, and recorded zero framework errors/warnings. Baseline hostname
navigation actually reached the destination; a DNS/connectivity failure did not create
the patched result. Popup/data refusals are preserved behavior, not patch-attributed wins.
No screenshot, browser emulation or static call graph was substituted for these runs.

## Reproduction and fixed inputs

Commands and boundaries: [native probe runbook](../../spikes/wails-gallery/NATIVE-PROBE.md).
Native fixture/identical oracle: [main.go](../../spikes/wails-gallery/cmd/native-probe/main.go).
Saved first-party receipts: [baseline](fixtures/native-containment/baseline.json) and
[contained](fixtures/native-containment/contained.json). These contain synthetic case
observations and a compatibility user-agent string, no credentials or purchased source.

- macOS 26.6.2, build 25G83, arm64; Go 1.26.3.
- Wails `v3.0.0-beta.21`, module sum
  `h1:2jc9E7eT4g3vNQIaEoRP19Uvm4LCuL9OzYPD+Uf+yYk=`; module-file sum
  `h1:/6QR46/nhGCSADHbS++XtDb9dkTnenTHlGskTPRo9S0=`.
- WKWebView reported `AppleWebKit/605.1.15` and `wails.io`; its compatibility UA also
  says Intel/Mac OS X 10_15_7. That is not the machine's actual architecture/OS version.
- Local IPv4 loopback only, two synthetic listeners, sequential 640×400 native windows,
  JavaScript-driven attempts and HTTP/native callbacks. No keyboard/IME/pointer proof.
- Two-second case observation, one-second close wait, 50-second process watchdog.
  These are test deadlines, not application latency measurements or performance claims.

SHA-256 identities:

| Artifact | SHA-256 |
|---|---|
| Probe source | `323e0eebeb2ad244bb5b26d662a3adae1f6d9300c87a927aeec1c8e2769582de` |
| Probe tests | `c939148416804dddd5d9885a6e2899a1bd039b25eb78a286b649960b55721f8f` |
| Native patch | `48adbb4904968b6ece05484262a4b8bf936a8c44c7afe53145b71fce19e98f90` |
| Baseline binary | `568c1c90b79d01500f5d20b678a4ca8561a834b4d45df29b72f32fd6c369699b` |
| Contained binary | `3fd5ec6b6f6246039a45db0ea422669c97558e8d77f8ce7d0407e62da838b4f4` |
| Baseline receipt | `4c1a7c8e48de18ef17dc31a45ccea10b48ae3e8c555ed0e2b819d9a47b6a8f3a` |
| Contained receipt | `b15bfafe86bc508cd6651384ee167646b912b3f1e61cef9a7da1fc51d4249e7b` |

Final binaries/initial receipts remain in the temporary directory
`/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/appkit-native-boundary.fPhY2hzGJi`.
The final prepared module is in `appkit-wails-beta21-3672660153` under the same parent.
Temporary paths are not durable delivery; replay uses tracked source/patch/checksums.
The stored JSON copies match original hashes. No probe/Gallery process remained at
the final process check; no global installation, deployment, signing or commit occurred.

## Defects found and controls added

The original delegate installed `external` and `wails` handlers and injected runtime
JS after navigation, without a native origin veto. The patch sets an exact origin
before first load, retains the existing delegate/lifecycle, permits only matching
top-level navigation, and omits those handlers/runtime. The Gallery host additionally
denies framework asset routing through middleware; its real HTTP tree stays separate.
Unprepared ordinary native launch now fails before creating the native application,
with explicit browser recovery. The unsafe baseline requires a dedicated probe flag.

The first prototype terminated with empty stdout because `NSApp terminate` bypassed
code after `app.Run`. No pass was accepted. [INC-21](../intent/incidents.md) now records
shutdown-callback receipts plus a separate verifier. Independent review added exact
ordered evidence, host-side bridge delivery, bounded window removal, strict receipt
decoding and warning failures. A port-normalization defect (`:080` versus `:80`) and
empty-query input were also repaired and tested before final patch replay.

The first native pair passed/faulted the same 14 cases, but final evidence above uses
the hardened oracle and corrected patch. Earlier prototype receipts are superseded,
not additional independent passing cases.

## Supporting checks and remaining boundaries

- Nested-module native/server race suites pass, including real HTTP launch/refusal,
  Gallery form/SSE tests and the independent server dependency-closure canary.
- Baseline and contained `nativeprobe` race suites and vet pass; planted missing,
  duplicated, wrong-order, altered, warning, bridge and non-closed receipts fail.
- Patched Wails `pkg/application` tests pass (6.631s). Canonical-origin tests cover
  default/explicit ports, IPv6 and malformed origins; no Windows runtime claim follows.
- Offline staging succeeds; exact selected pin/replacement, checksum and patch drift
  are refused. Exclusive temporary output avoids overwriting existing source/cache.
- Native Gallery and cgo-free server builds pass. The nested module needed the root's
  already-selected `uniseg v0.4.7` indirect dependency/checksums; no Wails pin or root
  dependency was changed. A compile remains separate from running the Gallery task.
- An actual compiled unprepared Gallery launch exits 1 with explicit `-browser`
  recovery, without creating a native application. The protected HTTP startup remains
  short-lived; no launch capability is printed.
- Full MCP document/protocol tests pass (2.863s); scoped structural checks cover 53
  documents, 248 relative links, ADR index membership, current budgets and identical
  intent stanzas. This is not a full intent-doctor, global host audit or root `task check`.

This strict profile blocks Gallery's iframe-based private comparisons; private inputs
are disabled in the candidate. Their independent browser viewer remains the supported
inspection path. A future selected native comparison task must revisit the frame policy
with explicit constrained tests, not silently weaken this refusal oracle.

G6 still needs the same Gallery save/stream/recovery task in independent browser and
native hosts, simultaneous-view convergence, actual keyboard/IME/zoom/pointer and
capability grant/denial, startup/failure recovery and applicable distribution evidence.
The native Computer Use connection was not repaired by this programmatic probe.
No general WebKit sandbox, network firewall, hostile-local-process isolation, ephemeral
cookie container, remote hosting, signed delivery or production platform support is proven.
