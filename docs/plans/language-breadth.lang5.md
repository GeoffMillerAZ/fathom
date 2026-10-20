# Language breadth, wave lang5: foundation and deferred rows

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/language-breadth.lang5.md` proves it. Rows here are the
foundation and `-rest` rows wave lang5 files (`docs/plans/language-breadth.md` is edited only for a worker's own rows,
AP-14). Queue needs use the cross-plan form `language-breadth.lang5.md#LB-F<n>`.

## Foundation

- [ ] A `global-name` import strategy: a type name binds across the files of one schema with no import statement, so a reference resolves to the file defining the name (unique definition only; two definitions of a name bind nothing, counted in `FileResolveReport`). Unblocks GraphQL L3 (`LB-Q-graphql`, O1391) {id: LB-F580, disposition: deferred, reason: "a new import strategy is a foundation-level change (Opus row, docs/design/language-breadth.md section 6), never a worker's; no strategy in resolveByStrategy binds a bare name across files without an import spec", trigger: "an Opus foundation wave takes the row; GraphQL (3 cross-file pairs in gqlgen, 430 ambiguous names) is its first customer"}
