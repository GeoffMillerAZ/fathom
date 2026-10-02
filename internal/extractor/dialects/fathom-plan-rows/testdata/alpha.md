# Alpha plan

Baseline: 0123456789abcdef

## Rows

- [x] The first row names a function {id: AL-1, symbol: Alpha, package: internal/alpha, reachable_from: TestAlpha}
- [ ] The second row needs the first and a row of another plan {id: AL-2, oracle: lexical, path: docs/alpha.md, pattern: "alpha", needs: "AL-1,beta.md#BE-1"}
- [-] A skipped row {id: AL-3, disposition: deferred, reason: "later", trigger: "never", needs: "AL-2,AL-2,AL-9"}
- [ ] A row with no id has no durable name
- [ ] A row naming its own plan {id: AL-4, symbol: Four, needs: "alpha.md#AL-1, beta.md#BE-2"}
