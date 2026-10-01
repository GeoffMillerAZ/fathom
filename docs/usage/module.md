### `module` — expert-module lifecycle (**experimental**, law 9: read-only before writable)

```sh
fathom module init    --name N --description T --include GLOB[,…] [--exclude …] [--dir D] [--root D] [--tier-profile P] [--license L] [--force] [--json]
fathom module sources --manifest module.cue [--root D] [--json]
fathom module map     --manifest module.cue [--root D] [--store P --service S] [--json]
fathom module compile --manifest module.cue --store P [--service S] [--dry-run=true|false] [--profile P | --mock --mock-entries-file F] [--force] [--unsafe-skip-eval-gate] [--json]
fathom module eval    --manifest module.cue [--fixtures D] [--profile P | --mock --mock-entries-file F] [--record P] [--json]
fathom module doctor  --manifest module.cue [--root D] [--store P --service S] [--pack-dir D] [--require-level L] [--json]
fathom module pack    --manifest module.cue --store P --out D [--force] [--json]
fathom module runs    --manifest module.cue --store P [--service S] [--limit 20] [--json]
```

Specs: `docs/spec/expert-module-v0.md`, `docs/spec/store-format-v0.md`;
design `docs/design/expert-modules.md`. A packed module is mounted read-only
by `serve-http --module`; no external consumer exists yet.
