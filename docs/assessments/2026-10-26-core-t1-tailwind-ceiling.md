# tailwind named ceiling routes (X-R1)

Measured baseline: [tailwind reading](../bench/readings/lang/tailwind.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Tailwind design-system compiler API | tailwindcss 4.3.2 | The committed adapter uses __unstable__loadDesignSystem. Theme/utility/variant registries expose no rule ends and the public package exports no parser; range_agreement remains null. |
| Tailwind internal parser source | npm tailwindcss 4.3.2, exact bundle | Inspected dist/lib.js: internal Pe assigns rule src/dst end at the opening brace; its closing-brace branch only parses children and pops the stack. It never extends the parent span to the closing brace. Pe is unexported; patching/extracting it still requires adding full-span behavior. |
| PostCSS independent parser | 8.5.28 | It exposes source.end but is tier C under the current oracle rules. No compiler-tier range truth has been measured from it. |
| Lightning CSS browser AST | 1.33.0 | It lacks full rule ends and does not attest Tailwind compiler registrations. |
| Template candidate scanner | @tailwindcss/oxide 4.3.2 | Template utility candidates are outside the CSS-at-rule reading scope (O1480); they do not attest source rule-end spans. |

Retain the measured API/tier ceiling. The exact internal parser route was checked, not inferred from a different revision. Retry when a pinned first-party end-location API is demonstrated, or an owner changes oracle tiers.

Governing row: `LB-Q-tailwind-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

Exact package inspected: [npm tailwindcss 4.3.2 tarball](https://registry.npmjs.org/tailwindcss/-/tailwindcss-4.3.2.tgz), SHA256 `3673da9004404d12d4672bb5d002945319c2b3dea8c13ea022fdd33a3e260e62`. The export map and dist/lib.d.mts confirm compile returns CSS/source maps, compileAst takes caller-supplied AST, and the design system exposes no raw parser. Reconstructing closing spans with an adapter scanner would be adapter truth, not a new compiler span.
