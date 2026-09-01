# Repo layout — the target tree

**Status:** plan, 2026-06-10. The repo is docs-only today; this document is
the layout the implementation phase builds toward. It is the concrete
expression of D13 (one repo, one module —
[../adr/0013-monorepo-versioning.md](../adr/0013-monorepo-versioning.md)) and
D15 (agent-legible by construction —
[../adr/0015-ai-first-surface.md](../adr/0015-ai-first-surface.md)). When the
tree and this document disagree, one of them is wrong and the fix is a commit
that changes both.

## Principles

1. **One module, root tags.** `go.mod` at the root, module path
   `github.com/geoffmilleraz/app-kit`, semver tags from the root (D13).
2. **One obvious place for everything.** An agent that needs the token schema,
   the SSE bridge, or the upgrade notes should be able to *guess* the path
   correctly. Flat beats clever; named beats numbered; predictable depth
   (rarely more than three levels) beats taxonomy purity.
3. **The library is the product; everything else orbits it.** `appkit/` is
   what apps import. The CLI, the Assistant, the Gallery, and the pipeline are
   consumers of the library that happen to live in the same module.
4. **Visibility over privacy.** We use `internal/` only for genuinely
   load-bearing privacy. The Gallery sits at top level (`apps/`) even though
   no app imports it — discoverability wins (D15 tiebreaker); we do not need
   the compiler to police a one-person module. The one carve-out is
   license-driven, not privacy-driven: the licensed-data pipeline lives under
   `internal/twplus/` precisely so downstream importers can never reach
   licensed-data tooling.

## The tree

```
app-kit/
├── go.mod                      # the one module; tool directives pin templ + cue CLIs
├── go.sum
├── Taskfile.yml                # root task runner; includes tools/taskfiles/*
├── CLAUDE.md                   # repo-level agent briefing (the kit's own, not the shipped pack)
├── .mcp.json                   # the Assistant, connected to its own repo (dogfood)
├── .gitattributes              # *_templ.go and *_gen.go marked linguist-generated (D4, D7)
├── CHANGELOG.md                # one entry per tag (D13)
├── VISION.md                   # the north star
├── README.md                   # orientation + two-command build
│
├── appkit/                     # THE KIT — the importable library (the platform contract)
│   ├── appkit.go               # appkit.Run(myapp.New) — the front door; an app is func New() http.Handler
│   ├── app/                    # boot internals behind appkit.Run: manifest load, lock, listener, shell select
│   ├── shell/                  # capability interface + degradation matrix (D5, D6)
│   │   ├── shell.go            # Capabilities interface; Degraded() reporting
│   │   ├── wails3/             # Wails v3 implementation: window/tray/dialogs/updater (!server build tag)
│   │   └── headless/           # browser-tab + cloud (`-tags server`; cloud = headless + non-loopback bind config)
│   ├── ui/
│   │   ├── catalog/            # the component registry, generated from each component's .cue
│   │   ├── button/             # a per-component package: button.templ, button.cue, catalog_gen.go
│   │   ├── <component>/        # ...one package per component; templUI enters via its CLI, adapted to kit tokens
│   │   ├── layouts/            # app shell, nav, page chrome as templ layouts
│   │   └── tokens/             # generated Go token constants + @theme CSS emission (from cue/tokens)
│   ├── bus/                    # event spine interface (D8)
│   │   ├── bus.go              # Bus/KV interfaces — the API apps program against
│   │   ├── natsbus/            # embedded NATS JetStream impl (DontListen, InProcessServer)
│   │   └── chanbus/            # channel-backed opt-out for trivial tools
│   ├── store/                  # SQLite open/migrate/backup helpers (D9, ncruces driver)
│   ├── config/                 # CUE-validated config load (embedded cuecontext, D7)
│   ├── jobs/                   # long-running jobs: progress → bus → SSE pattern
│   ├── auth/                   # loopback token→cookie exchange, sessions (D14)
│   └── web/
│       ├── server/             # the loopback listener: 127.0.0.1:0, lifecycle, heartbeat (D5)
│       ├── sse/                # the canonical loop: bus/KV watch → templ render → Datastar patch
│       └── middleware/         # auth gate, CSP defaults, request logging
│
├── cue/                        # every schema, authored once (D7)
│   ├── app/                    # app.cue manifest schema: identity, capabilities, nav, theme
│   ├── tokens/                 # design tokens — emits DTCG 2025.10 JSON + Tailwind @theme CSS
│   ├── components/             # component metadata schema (feeds Gallery + Assistant)
│   ├── patterns/               # pattern metadata schema (scenario → pattern selection)
│   ├── tailwindplus/           # licensed-data pipeline schemas — #Manifest and friends (D10)
│   └── bus/                    # KV bucket schemas + NATS subject hierarchies (D8)
│
├── cmd/
│   ├── appkit/                 # the CLI
│   │   ├── main.go
│   │   ├── new.go              # `appkit new` — scaffold (tree below); front end over internal/scaffold
│   │   ├── dev.go              # `appkit dev` — orchestrates the dev loop
│   │   ├── build.go            # `appkit build` — codegen, vet, binaries, packaging
│   │   └── data.go             # `appkit data` — D10 pipeline entry (build/parquet/embed/index/verify)
│   └── appkit-mcp/             # the Assistant (D11) — one binary, stdio default, -http flag
│
├── apps/
│   └── gallery/                # the Gallery (D12) — a kit app, dogfood #1
│       ├── main.go             # boots with appkit.Run like any downstream app
│       ├── app.cue             # its own manifest — the first app.cue in existence
│       ├── ui/                 # the Gallery's own templ components and layouts
│       ├── sections/           # home, components, tokens, patterns, services, playbook, twplus
│       └── visreg/             # Playwright snapshot harness — the one sanctioned Node path (ADR-0002 §5)
│
├── data/
│   └── tailwindplus/
│       ├── README.md           # provenance + the dump-copy instructions (their durable home)
│       ├── tailwindplus-components.json  # the licensed 336MB raw dump — git-ignored (not LFS), never redistributed
│       ├── parquet/            # canonical committed shards, by section/group (D10)
│       ├── embeddings/         # committed vector cache: <model>-<dims>/cache.sqlite
│       ├── index/              # generated SQLite index — gitignored, rebuilt by `appkit data index`
│       └── manifest.json       # CUE-validated against #Manifest: content digests + pinned model
│
├── docs/
│   ├── DECISIONS.md            # the register — the compressed index agents load first
│   ├── adr/                    # numbered ADRs + template.md (README.md is the canonical list)
│   ├── design/                 # this corpus: detailed designs
│   └── upgrades/               # vX.Y.md per minor — consumed by the Assistant's upgrade_guide (D13)
│
├── tools/
│   ├── tailwind/
│   │   └── manifest.cue        # Tailwind v4.3 standalone: version + SHA-256 per platform (D2)
│   └── taskfiles/              # shared Taskfile includes: gen, css, check, release
│
└── internal/
    ├── codegen/                # the D7 codegen driver: cuelang.org/go + gengotypes, run by `task gen`
    ├── scaffold/               # the one generator behind `appkit new` and the Assistant's scaffold_app
    └── twplus/                 # the D10 pipeline — internal/ is license-driven here (see Principles)
        ├── schema/             # embedded CUE + generated Go types
        ├── parse/              # streaming dump decode → raw records
        ├── validate/           # per-component cuecontext validation
        ├── normalize/          # variant explosion, content hashing, dedup
        ├── parquetio/          # shard writer/reader (parquet-go)
        ├── enrich/             # embed-text builder: taxonomy + class extraction
        ├── embed/              # pinned-model embeddings (Ollama qwen3-embedding-0.6B, 768d)
        ├── index/              # the shared query/index package (FTS5 + sqlite-vec)
        └── manifest/           # manifest assembly + verification
```

## The library surface, sketched

The front door is deliberately tiny — an app is `func New() http.Handler`,
and boot is one line:

```go
package main

import (
    "github.com/geoffmilleraz/app-kit/appkit"

    "myapp"
)

func main() {
    appkit.Run(myapp.New) // manifest validated (D7), loopback listener, shell per build tag
}
```

Everything behind that line — manifest load, instance lock, listener, shell
selection — lives in `appkit/app`.

`appkit/bus` is the spine's whole API — small enough to hold in one read,
implemented twice (D8). The canonical definition lives in
[../adr/0008-embedded-nats-spine.md](../adr/0008-embedded-nats-spine.md);
this sketch matches it:

```go
package bus

type Bus interface {
    Publish(ctx context.Context, subject string, data []byte) error
    Subscribe(ctx context.Context, subject string, h func(Msg)) (CancelFunc, error)
    KV(ctx context.Context, bucket string) (KV, error)
}

type KV interface {
    Get(ctx context.Context, key string) (Entry, error)
    Put(ctx context.Context, key string, value []byte) (rev uint64, err error)
    Watch(ctx context.Context, keyPattern string) (Watcher, error) // drives the SSE loop
}
```

`appkit/web/sse` packages the canonical loop (KV watch → render → patch) so an
app page is a render function, not plumbing. `appkit/shell` is the capability
interface from D5/D6; app code imports `shell`, never `shell/wails3` — the
implementation (`shell/wails3` or `shell/headless`) is selected by the
`server` / `!server` build tags.

## Build tooling

**Taskfile, the wails3 way.** Wails v3's build system is Taskfile-based and
transparent — `wails3`-generated projects are driven by plain Taskfiles
(https://github.com/wailsapp/wails/blob/master/docs/src/content/docs/whats-new.md,
verified 2026-06-10). We adopt that convention everywhere rather than invent a
second one: the kit's root `Taskfile.yml` and every scaffolded app's
`Taskfile.yml` include the shared task definitions from `tools/taskfiles/`, so
`task dev`, `task gen`, `task check`, `task build`, `task package` mean the
same thing in every kit repo. Desktop packaging tasks delegate to `wails3`
(signing via `wails3 setup signing`, notarization, DMG/NSIS/MSIX — D6).

**Go tool directives pin the Go-installable tools.** Per D2/D4, the templ CLI
(`github.com/a-h/templ/cmd/templ`, v0.3.1020+) and the cue CLI
(`cuelang.org/go/cmd/cue`, pinned to the exact library version, v0.16.1 today)
are `tool` directives in `go.mod` — CLI and runtime module can never skew
(the documented templ failure mode,
https://github.com/a-h/templ/discussions/592). Invocation is always
`go tool templ ...` / `go tool cue ...`, including inside Taskfiles and CI.

**The Tailwind binary is pinned by manifest, not by hope.**
`tools/tailwind/manifest.cue` records the standalone binary's version (v4.3.0,
2026-05-08, https://github.com/tailwindlabs/tailwindcss/releases) and SHA-256
per platform. `task css` fetches to a cache, verifies the checksum, and runs
`tailwindcss -i assets/input.css -o assets/app.css --minify` with
`@source "./**/*.templ"` config living in the CSS file itself (D2: no Node,
no config JS). A checksum mismatch is a hard build failure.

**Dev loop** (`appkit dev`, or `task dev` directly): `go tool templ generate
--watch --proxy` for template hot-reload, the Tailwind binary in `--watch`,
and wgo restarting the server on Go changes (excluding `*_templ.go`) — the D4
loop, packaged so no app ever reassembles it.

**Codegen** (`task gen`): runs `internal/codegen`, the small Go program from
D7 that drives `cue exp gengotypes` and the token emitters (DTCG JSON +
`@theme` CSS) via cuelang.org/go directly — not `cue cmd`, whose v2 redesign
is unsettled (https://github.com/cue-lang/cue/issues/1325, open as of
2026-06). All generated output is committed.

## CI outline

One workflow, ordered cheap-to-expensive; a tag release runs all of it plus
release gates. This is also where ADR-0015's "docs are the API" gets its
teeth.

1. **Generation cleanliness:** `go tool templ generate -check` (the v0.3.1020
   flag); `task gen` then `git diff --exit-code` — committed generated code
   must match fresh generation exactly (D4, D7).
2. **Schema validity:** `go tool cue vet ./cue/...` against examples and
   fixtures; the Gallery's `app.cue` and scaffold-fixture `app.cue` must
   validate.
3. **Tests:** `go vet ./...`, `go test ./...` — including pipeline stages run
   against small committed fixtures (never the 336MB raw JSON in CI) and
   scaffold tests asserting `appkit new` output compiles, vets, and contains
   a valid `.mcp.json` + CLAUDE.md pack (D15).
4. **Gallery snapshots:** build the Gallery, snapshot every component/pattern
   page, diff against the committed baseline (Playwright per D12 — CI-only
   harness; D2's no-Node rule governs app toolchains and shipped artifacts,
   not the CI container that takes screenshots).
5. **Doc checks:** internal link check across `docs/` (hard failure),
   external links against a pinned allowlist (soft, reviewed); on minor tags,
   `docs/upgrades/vX.Y.md` must exist and its structured header must
   validate (D13, D15).
6. **Release gates (tags only):** binaries build for all targets; `wails3`
   packaging dry-run; CHANGELOG entry present; then the tag is blessed.

## What `appkit new` produces

The scaffold is the kit's mirror in miniature — same names, same shapes, so
an agent that has internalized this document is already oriented in every
downstream app (D15). Everything reusable is *imported* (D13); the scaffold
contains only what is genuinely this-app's. The Assistant's `scaffold_app` is
a thin MCP wrapper over the same `internal/scaffold` package that powers
`appkit new` — one generator, two front ends:

```
myapp/
├── go.mod                  # require github.com/geoffmilleraz/app-kit vX.Y.Z (GOPRIVATE noted in CLAUDE.md)
├── app.cue                 # the app manifest: identity, capabilities, nav, theme (D7)
├── CLAUDE.md               # generated agent briefing: kit version, conventions, failure signatures
├── .mcp.json               # {"mcpServers":{"appkit":{"type":"stdio","command":"appkit-mcp"}}}
├── .gitattributes          # *_templ.go linguist-generated
├── Taskfile.yml            # includes the kit's shared tasks; wails3 packaging wired
├── main.go                 # the one-line front door shown above — at the app root, not cmd/myapp/
├── ui/
│   ├── shell.templ         # the app shell layout
│   ├── pages/              # this app's templ pages (kit components imported, not copied)
│   └── components/         # app-specific components — rare by design; promote to kit when reused
├── domain/                 # the app's actual logic — the only directory the kit has no opinion on
├── store/
│   └── migrations/         # SQLite migrations, run by appkit/store at startup (D9)
├── assets/
│   └── input.css           # @import "tailwindcss"; @source "./**/*.templ"; theme via generated @theme
└── build/                  # icons, signing config, wails3 packaging inputs (D6)
```

No `islands/` directory is scaffolded by default — it is created when the
first L2 island is admitted.

Day one in a scaffolded app: `task dev` runs the full loop against a themed,
authenticated, SSE-live walking skeleton; `task package` produces a signed
desktop build — the day-one acceptance test (VISION.md §North-star #2) is a
property of this tree, not an aspiration.

## Growth rules

- A component used by two apps moves *up* into its own per-component package
  `appkit/ui/<component>/` and ships in the next minor with an upgrade note —
  never copied sideways.
- New schemas go in `cue/`, get a gengotypes target in `internal/codegen`,
  and their generated Go is committed — no schema exists only in prose
  (D15 commitment #1).
- New top-level directories require an update to this document in the same
  commit, and a real argument for why no existing home fits.
- `internal/` stays small. If something in `internal/` wants to be imported
  by an app, that is a signal it belongs in `appkit/`, decided deliberately.

## Open seams (deliberate, not forgotten)

- `internal/twplus/` is plain packages today; if it ever wants cgo or a
  heavyweight dependency, D13's tripwire carves it into a nested module — the
  layout already isolates it for exactly that surgery.
- `appkit/shell/wails3` tracks a pinned alpha (≥ v3.0.0-alpha.99, 2026-06-10,
  D6); a Tauri-sidecar fallback would land as `appkit/shell/tauri` beside it
  without touching app code — that is the Loopback Invariant doing its job.
- Mobile (`appkit/shell/` gaining an owned-webview entry) waits on the D6
  triggers; nothing in this tree blocks it.
