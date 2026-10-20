# prisma-dmmf: tier-A Prisma schema oracle (definitions, references, cross_references)

Prisma's own schema engine (`@prisma/prisma-schema-wasm`, the module `getDMMF` in `@prisma/internals` calls) attests
the names of a schema: models, views, composite types, enums and their values, each block's fields, each field's type
and kind, and the `@relation` fields: and references: lists. It runs offline (no query-engine binary). Bench-only; never
a product dependency.

    npm install --prefix <toolcache>/prisma --ignore-scripts @prisma/prisma-schema-wasm@6.19.0-26.2ba551f319ab1df4bc874a89965d8b3641056773
    node calibration/oracles/prisma-dmmf/prisma_oracle.mjs -tools <toolcache>/prisma -subject <checkout> \
        -out workspace/tmp/oracle/prisma-<subject>.json [-summary-out <file>]

The `.prisma` files of one directory are one schema (the prismaSchemaFolder layout), handed to the engine together.
DMMF has no source position, so a block's and a member's lines come from a bracket-depth line scan in the harness,
cross-checked both ways against the engine (`name_disagreements`). The 7.x and 8.x engines reject `url` in a
datasource, which most schemas in the wild still carry, so the pin is 6.19. An `Unsupported("...")` column is left
out of DMMF and is a definition with no reference. A schema the engine rejects falls back to the scan alone and the
reading is tier C (`engine_rejected`).

What it writes: `definitions` (`model`, `view`, `type`, `enum`, `datasource`, `generator`, `field` as Model.field,
`enum_value` as Enum.VALUE), `references` ((Model.field, type) for an object or enum field, and (Model.field, field
name) for each name in its @relation lists) and `cross_references` (a type reference one other file of the same schema
defines).
