# scala named ceiling routes (X-R1)

The committed reading is unchanged: level L2, grades L1 good, L2 good, L3 unmet, L4 not-attempted.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Scalameta syntax dump | 4.17.4 | Existing independent syntax dump gives range_agreement .8042 and call pairs. Its tier-B call truth caps L2 at good; it is not SemanticDB resolution. |
| Universal Ctags | 6.2.1 | Its --list-languages has no Scala parser, so the old queue suggestion cannot be an oracle. |
| SCIP via SemanticDB | scip-java SemanticDB route; 0.13.2 current candidate | Requires the subject own sbt/Mill compile and SemanticDB output. No ranged index was produced here. The written one-JVM/1GB build budget and absent sbt/JDK setup prevented the earlier cross-built cats/http4s build; this is a local build limit, not missing upstream tooling. |
| End-line trim | main5b84aa5b4 tsEndLine / O761 | The tags engine still counts trailing whitespace to the outdent. The previously measured trim reads .9511 but is a shared ENGINE change with K29 regrades, not a Scala-only edit. |
| Reference query / qualified calls | current Scala dialect / O807 | No references.scm and no qualified call text route are implemented. Adding them requires K17/K26 cost proof and independent validation. |

Existing measured ceilings hold; build and shared-engine routes are named and remain deferred, not claimed nonexistent.

Governing row: `LB-Q-scala-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.
