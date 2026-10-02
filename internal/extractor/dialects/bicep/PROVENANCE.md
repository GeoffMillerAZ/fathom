# bicep — provenance and divergences

LB-Q-bicep, design `docs/design/language-breadth.md` §3, §6, §13 (wave iac). A tags-engine
dialect on tree-sitter-grammars/tree-sitter-bicep v1.1.0 (MIT, ABI 14, a scanner), pinned in
`internal/extractor/tsengine/grammars/grammars.lock.json` (blob `bicep`, 76,391 bytes
gzipped). `tags.scm` and `imports.scm` are written for this dialect: the grammar ships no
`tags.scm` and neither aider nor nvim-treesitter carries Bicep tags. Node shapes were read with
`indexbench lang-probe`.

## What it models

- **Definitions**: `param`, `var`, `resource` (an `existing` one, and a child declared inside
  its parent's body), `module`, `output`, `type`, `func` and `metadata`, each named by its
  symbolic name; every kind is `callable` so it owns the references in its body, and a
  signature runs to the `=` (`resource vnet 'Microsoft.Network/virtualNetworks@2023-09-01' =`).
  A `.bicepparam` file's `param` assignments are read by name (the grammar does not parse
  `param x = v`, recovering around an ERROR node).
- **References** (Calls, L2): a symbolic name used as a value inside a declaration (an object
  property's value, a member access's object, `parent::child`, a subscript, an argument, an
  array item, an interpolation, an operand, a loop's iterable or body, a lambda's body, a
  direct declaration value, a type annotation) and a call of the file's own function. A loop's
  or a lambda's variable and a function's parameter are captured too: the tags engine has no
  local scopes, and the oracle counts their uses as references of the declaration they sit in
  (the language server resolves each to its local symbol). Bicep's builtin functions (the CLI's
  `sys` and `az` namespaces at v0.47.16), decorators and the `az`/`sys` namespace names are not.
- **A module's params keys** are its target's parameters: each is stored qualified by the
  module's path (`./storage.bicep.location`, `calls.qualify`).
- **Imports** (L3, `relative-path`, `relative: true`): a module's path binds its file as a
  whole, so its qualified params keys resolve to that file's parameters — the module's file
  edge; `import {a, b as c} from './t.bicep'` binds names, `import * as t from` the file. A
  `br:`, `br/` or `ts:` registry module resolves to no file. A `.bicepparam`'s `using` is
  recorded and binds no name.

## What is not modelled, and why

- A module with no `params` draws no file edge (nothing in its body names a symbol of the
  target), and a `.bicepparam`'s `using` none (LB-Q-bicep-rest).
- `import * as t` then `t.x`: the namespace's member access is not stored qualified.

## Calibration oracle

`calibration/oracles/bicep/bicep_oracle.py` (bench-only): the Bicep CLI's JSON-RPC server
(`bicep/compile`, `bicep/compileParams`, `bicep/getDeploymentGraph`) and the Bicep language
server (`documentSymbol`, `references`), both v0.47.16 (MIT), the language server run by the
.NET 10 runtime (tier A). Subjects: `calibration/corpus.d/bicep.yaml`.
