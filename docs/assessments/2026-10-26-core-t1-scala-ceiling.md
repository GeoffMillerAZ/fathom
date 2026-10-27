# scala named ceiling routes (X-R1)

Measured baseline: [scala reading](../bench/readings/lang/scala.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Scalameta syntax dump | 4.17.4 | Existing independent syntax dump gives range_agreement .8042 and call pairs. Its tier-B call truth caps L2 at good; it is not SemanticDB resolution. |
| Universal Ctags | 6.2.1 | Its --list-languages has no Scala parser, so the old queue suggestion cannot be an oracle. |
| SCIP via SemanticDB | scip-java SemanticDB route; 0.13.2 current candidate | Requires the subject own sbt/Mill compile and SemanticDB output. No ranged index was produced here. The written one-JVM/1GB build budget and absent sbt/JDK setup prevented the earlier cross-built cats/http4s build; this is a local build limit, not missing upstream tooling. |
| End-line trim | main 5b84aa5b4 tsEndLine / O761 | The tags engine still counts trailing whitespace to the outdent. The previously measured trim reads .9511 but is a shared ENGINE change with K29 regrades, not a Scala-only edit. |
| Reference query / qualified calls | current Scala dialect / O807 | No references.scm and no qualified call text route are implemented. Adding them requires K17/K26 cost proof and independent validation. |

Existing measured ceilings hold; build and shared-engine routes are named and remain deferred, not claimed nonexistent.

Governing row: `LB-Q-scala-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / compiler | [Scala 3 compiler SemanticDB](https://docs.scala-lang.org/scala3/reference/metaprogramming/semanticdb.html) emits SemanticDB with `-Xsemanticdb` without a plugin; [scala-cli v1.18.0](https://github.com/VirtusLab/scala-cli/releases/tag/v1.18.0) compiles with `--semanticdb`. [Coursier `cs launch`](https://get-coursier.io/docs/cli-installation) can run sbt without a system install. These routes are **LIVE UNMEASURED** under the one-JVM/1 GB budget; no successful subject build or ranged index is claimed. Scala 2 remains covered by [semanticdb-scalac 4.17.4](https://scalameta.org/docs/semanticdb/guide). |
| SCIP / LSIF | [scip-java](https://github.com/sourcegraph/scip-java) via SemanticDB and historical [lsif-java](https://github.com/sourcegraph/lsif-java) were found; 0.13.2 has no successful Scala subject index under this lane. |
| Language server | [Metals](https://scalameta.org/metals/docs/index.html), version **1.6.9**, exposes definitions/references but requires an imported build. **LIVE UNMEASURED** under one-JVM/1GB limits. |
| Registry AST | [SemanticDB specification](https://scalameta.org/docs/semanticdb/specification.html) defines occurrence ranges; [Scalameta trees](https://scalameta.org/docs/trees/guide.html) can represent syntax spans, neither was produced on frozen subjects here. |
| Runtime tracing | [JVM Flight Recorder](https://docs.oracle.com/en/java/javase/17/jfapi/flight-recorder.html) profiles executed methods, not complete source occurrences. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.
