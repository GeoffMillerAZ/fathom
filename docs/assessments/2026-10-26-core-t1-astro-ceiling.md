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

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / parser | [Astro compiler registry](https://registry.npmjs.org/@astrojs/compiler/latest) reports **4.0.0**, already the measured compiler pin; it does not repair the shared TypeScript query. |
| SCIP / LSIF | The shared catalog searches and [GitHub `scip-astro` query](https://github.com/search?q=scip-astro&type=repositories) produced no subject-validated Astro indexer. This is a search limit. |
| Language server | [Astro LSP registry](https://registry.npmjs.org/@astrojs/language-server/latest) reports **2.17.3**; [changelog](https://github.com/withastro/astro/blob/main/packages/language-tools/language-server/CHANGELOG.md) records a 2.17.3 patch after the 2.17.0 `astro2tsx` update. **LIVE UNMEASURED**: public reference requests must prove complete original-source component positions; the patch is not that proof. |
| Registry AST | [Compiler package](https://registry.npmjs.org/@astrojs/compiler/latest) parses and maps Astro syntax but the frozen defaulted-props miss is extractor-side. |
| Runtime tracing | [Astro dev toolbar](https://docs.astro.build/en/guides/dev-toolbar/) observes rendered pages, not all component definitions or references. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.
