# Analytical resolution browser fixture

Historical evidence, **not a passing browser suite**. See the
[bounded assessment](../../2026-09-22-bar-resolution-and-guide-discovery.md).
All data are synthetic; no licensed corpus or real consumer data is included.

- `fixture.go.txt`: exact executed server source, SHA-256
  `20378f68c6e3bec45b8a92a624cad2e5ea3c00fb6afe72149c8a87f5d60d6e3c`.
  The `.txt` suffix deliberately keeps this isolated fixture outside the root Go graph.
- `browser.mjs`: original failed driver, SHA-256
  `f77c4b88c88e43132d765c774b8cf992725b355299a922a0cfdac962f3ec1648`.
- `browser-correction.mjs`: single admitted corrected driver, SHA-256
  `0442959b1330909bacbdbd81c89289de1be92a9a37523e780191bb0e294e8e99`.
- `browser-owned_process.mjs`: exact lifecycle helper, SHA-256
  `8283a22b3b11530e95745d1996a35bb1c5e9422ae2c5e97f6d310ffa5075f294`.
- `report-01.json` / `report-02.json`: unchanged failed acquisition reports.

Reconstruction: extract private app-kit revision
`7f9f054750ee722c66335d7acc53f1b08ddc6261` into two isolated copies. Place
`fixture.go.txt` as `cmd/resolution-fixture/main.go` in both. Candidate differs only
in the assessed `appkit/ui/analytics/render.go`; retain/hash all input bytes before
building each with Go1.26.3, `GOWORK=off`, `GOTOOLCHAIN=local`, `GOPROXY=off`,
`GOSUMDB=off`, `GOMAXPROCS=2`, `go build -mod=readonly -trimpath`.
Pass the same baseline `apps/gallery/ui/app.css` to both binaries.

Driver arguments, in order, must be six absolute paths: baseline binary, candidate
binary, CSS, installed Playwright `index.mjs`, Chrome executable, new output directory.
The recorded launcher was `/opt/homebrew/Cellar/node/26.3.0/bin/node`; dependencies
were Playwright1.62.1 and Chrome153.0.8010.53. Nothing installs dependencies or uses
the signed-in profile. Retain exact interpreter/runtime identities for any new run.

Do not silently replace the retained drivers. RUN-01 has SVG/header harness defects.
RUN-02 completes all eight cells but fails four narrow ArrowRight immediate-scroll
checks symmetrically across baseline/candidate. Two additional failures are the
required baseline semantic reproduction. Candidate fallback, independent values,
CSV equality, exact CSS and process cleanup pass; whole-suite acceptance does not.
The budget is exhausted for this batch. A follow-up requires a new prospective
contract, causal investigation of event settling, and separate evidence files.
