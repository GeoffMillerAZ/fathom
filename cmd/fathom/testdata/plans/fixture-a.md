# Fixture Plan A (cmd/fathom testdata — plans status/next golden fixture)

Rows are deliberately numbered out of document order (`id: A-9`, `A-2`,
`A-7`, `A-1`, …) so a test asserting `fathom plans next`'s ordering catches a
regression to TaskID-lexical order instead of true document order.

- [x] Charge handler, claimed and proven {id: A-9, symbol: Charge, skip_test: true, skip: "test fixture"}
- [x] Claimed but the symbol does not exist {id: A-2, symbol: NoSuchSymbolXYZ, skip_test: true, skip: "test fixture"}
- [ ] Not started yet {id: A-7, symbol: Charge, skip_test: true, skip: "test fixture"}
- [~] Started but not done {id: A-1, symbol: Charge, skip_test: true, skip: "test fixture"}
- [-] Deferred pending an external trigger {id: A-5, disposition: deferred, reason: "waiting on the payments SDK bump", trigger: "SDK v3 ships"}
- [-] Refused — decided out of scope {id: A-6, disposition: refused, reason: "out of scope for this service"}
