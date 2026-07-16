# H1 red-first proof artifacts

Script-captured `go test` output proving each regression test failed
before its fix and passed after (agentic-kit handoff spec H1 requires
proof beyond commit-body assertions; squashed history means the
red-first states live here instead of in separate test-only commits).

| Artifact | Pins |
|---|---|
| `callgraph-dedup-{red,green}.txt` | duplicate observations poisoning byName/byPkgName + duplicated edges |
| `callgraph-methodedge-{red,green}.txt` | `<var>.Method` callees never resolving to `Recv.Method` nodes |
| `package-grammar-red.txt`, `package-grammar-red2-enginekey.txt`, `package-grammar-green.txt` | `package:` annotation dropped; `observationToNode` malformed Key |
| `connector-skipdir-{red,green}.txt` | `.claude/` worktrees entering the index |
| `fusion-orphan-purge-{red,green}.txt` | orphaned sources (and fingerprints) never leaving the store |
| `baseline-foundations.txt` | the production symptom: `test_reaches` failing verified claims |
| `alias-canonicalization-{red,green}.txt` | selector calls through an explicit import alias recorded under the file-local alias, unresolvable (recall gap: evidence-spine `PreconditionSatisfied`) |
| `samepkg-bare-bind-{red,green}.txt` | bare same-package callees (declared in a sibling file) dropped as globally ambiguous (recall gap: llm-surface `openai.New`) |
| `ctor-scoped-method-{red,green}.txt` | constructor-initialized receiver method calls unresolvable when the method name is globally ambiguous; plus the encoded form's global fall-through defect (recall gap: evidence-spine `Ladder.Run`) |
