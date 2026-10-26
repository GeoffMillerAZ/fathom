# Astro ceiling, owner Q4 and X-R1

Astro meets L3. Its committed main reading remains L1 good: symbol recall .9362,
precision .9787, ranges/kinds/signatures 1. The tuned reading is also L1 good,
recall .9448 and precision .9942. Both read L2/L3 excellent. No reading was changed.
Owner Q4 chooses the written ceiling this wave, with a separate ENGINE wave for the repair.

| Candidate oracle or route | Version/pin | Checked result and concrete limit |
|---|---|---|
| Astro compiler parse plus TypeScript createSourceFile | @astrojs/compiler 4.0.0, TypeScript 5.9.3 | Existing tier-A adapter and readings already expose the missed defaulted destructured props (O989). A new oracle cannot fix missing extractor definitions. |
| Astro-local tags query | current main 5b84aa5b4 | Frontmatter and JavaScript scripts inject TypeScript; `astro/injections.scm` and `tsextract_host.go` use the peer's default variant. There is no host-local query override. Editing Astro tags alone does not reach these declarations. |
| Shared TypeScript tags query | tree-sitter-typescript v0.23.2, f975a621; current tags.scm | The missing defaulted `object_assignment_pattern` is in the shared query's domain. Changing it affects TypeScript and every hosting framework, requiring a K29 regrade and cost gates. No query change is made in this branch. |
| New host-query override mechanism | current engine 5b84aa5b4 | Not implemented. Adding it is a shared engine feature with its own proof and cost requirements, rather than a dialect-only repair. |
| Third-party Astro grammar query | virchau13/tree-sitter-astro 213f6e6973d9b456c6e50e86f19f66877e7ef0ee | It describes the host markup, not TypeScript frontmatter definitions. It also supplies tier-C truth only and cannot replace the existing first-party oracle. |

The shared-query route touches these 12 committed readings, including the historical baseline
that would need an explicitly labelled regrade rather than a silent rewrite:

- `docs/bench/readings/lang/typescript.json`
- `docs/bench/readings/lang/tuned/typescript.json`
- `docs/bench/readings/lang/tuned/lf3/typescript.json`
- `docs/bench/readings/lang/baseline/typescript.json`
- `docs/bench/readings/lang/holdout/typescript.json`
- `docs/bench/readings/lang/holdout/typescript-headlessui.json`
- `docs/bench/readings/lang/holdout/typescript-react-bootstrap.json`
- `docs/bench/readings/lang/react.json`
- `docs/bench/readings/lang/astro.json`
- `docs/bench/readings/lang/tuned/astro.json`
- `docs/bench/readings/lang/vue.json`
- `docs/bench/readings/lang/svelte.json`

Retry when a scoped ENGINE wave can perform those measurements under unchanged rules,
K17/K26 cost gates and independent validation. This is an owner-deferred repair, not an
upstream refusal. There is no new corpus, holdout, extractor epoch or generated-board edit.
