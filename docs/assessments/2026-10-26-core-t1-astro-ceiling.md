# Astro shared-query ceiling, C-R4 and X-R1

Measured: the [canonical reading](../bench/readings/lang/astro.json) and
[tuning reading](../bench/readings/lang/tuned/astro.json) hold the computed grades.
Ruling C-R4 ([issue #20](https://github.com/GeoffMillerAZ/fathom/issues/20#issuecomment-6407071302))
defers the extractor repair to a serialized ENGINE wave. X-R1 names the checked routes.

| Candidate oracle or route | Version/pin | Checked result and concrete limit |
|---|---|---|
| Astro compiler parse plus TypeScript createSourceFile | @astrojs/compiler 4.0.0, TypeScript 5.9.3 | Existing tier-A adapter and readings already expose the missed defaulted destructured props (O989). A new oracle cannot fix missing extractor definitions. |
| Astro-local tags query | current main 5b84aa5b4 | Frontmatter and JavaScript scripts inject TypeScript; `astro/injections.scm` and `tsextract_host.go` use the peer's default variant. There is no host-local query override. Editing Astro tags alone does not reach these declarations. |
| Shared TypeScript tags query | tree-sitter-typescript v0.23.2, f975a621; current tags.scm | The missing defaulted `object_assignment_pattern` is in the shared query's domain. Changing it affects TypeScript and every hosting framework, requiring a K29 regrade and cost gates. No query change is made in this branch. |
| New host-query override mechanism | current engine 5b84aa5b4 | Not implemented. Adding it is a shared engine feature with its own proof and cost requirements, rather than a dialect-only repair. |
| Fathom's pinned Astro grammar | virchau13/tree-sitter-astro 213f6e6973d9b456c6e50e86f19f66877e7ef0ee | The pinned grammar ships highlights and injections queries only, with no upstream tags query. It is the product's own parser and cannot grade itself. It supplies no independent fallback truth. |

The shared-query route touches these 15 committed readings, including the historical baseline
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
- `docs/bench/readings/routes/node.json`
- `docs/bench/readings/routes/react-router.json`
- `docs/bench/readings/routes/react-router-consts.json`

The three route readings consume TypeScript/TSX observations through
`cmd/indexbench/routeoracle.go`'s `HandlersIndexed` set. Adding definitions can change
handler binding even when route discovery is unchanged; re-read them to confirm.

Retry when a scoped ENGINE wave can perform those measurements under unchanged rules,
K17/K26 cost gates and independent validation. This is an owner-deferred repair, not an
upstream refusal. There is no new corpus, holdout, extractor epoch or generated-board edit.
