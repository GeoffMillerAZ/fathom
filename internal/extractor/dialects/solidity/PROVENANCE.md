# solidity — provenance

- Grammar: `JoranHonig/tree-sitter-solidity` v1.2.9 (MIT), commit `bf268722bc6406429776f71f012e6d1a88801931`, ABI 14, no
  external scanner, `src/parser.c` shipped at the repository root (`tree_sitter_solidity`), built into
  `internal/extractor/tsengine/grammars/solidity.wasm.gz` (353,063 bytes raw, 104,112 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`. Licence read at the pinned tag: the top-level `LICENSE` (MIT).
- `tags.scm`: rewritten from the grammar's own `queries/tags.scm` (same commit, MIT), which tags a contract, struct, enum and
  event as `class`, a library as `interface`, and reads a call as its expression text. This file names each declaration by
  its own kind (contract, interface, library, struct, enum, user-defined value type, event, error, modifier, function,
  method, constructor/receive/fallback, state variable, file-level constant), anchors a function by its body, and reads a
  call by the callee's last identifier.
- Calls (L2, by syntax): a function call, a member call, `emit E(...)`, `revert E(...)`, a modifier invocation, and
  `require(cond, E())`. Not calls: the global functions (`require`, `assert`, `keccak256`, ...), `abi.*`, the members of
  an address, array, bytes or user-defined value type (`push`, `pop`, `concat`, `call`, `staticcall`, `delegatecall`,
  `send`, `wrap`, `unwrap`), a callee that starts with a capital (a type conversion or struct constructor), and a base
  constructor named in a constructor's header. A call belongs to the innermost function, modifier or constructor.
- Not extracted, on purpose: struct members, enum values, parameters and locals, `using ... for` directives, inheritance
  (`is`) and the import graph (the target is L2).
- Known grammar gap: a file-level `using X for T global;` is a syntax error under v1.2.9 and loses the rest of the file
  (O1416).
- Oracle (calibration/oracles/solc-ast): `solc --standard-json` 0.8.37, tier A, for definitions and call pairs.
