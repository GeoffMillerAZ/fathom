# Native Unity evidence migration

Native admission is explicit: pass `tests ingest --manifest ROOT --unity-nunit XML --unity-receipt ACTIVE_RECEIPT`. The two native flags must appear together and cannot be combined with `--results`. The producer must emit the standard test-kit record and bind it in the source-bound active receipt. Historical XML without that record is not execution evidence. See the [producer contract](../guides/unity-nunit-evidence.md).

Generic bare JSON and DSSE envelopes declaring `env.runnerClass` as Unity (case-insensitive), or artifacts named `unity-nunit-xml` or `unity-source-inputs`, require that adapter. They cannot fall back to generic file/name joins. Other generic envelopes retain upstream joining and unmatched-case reporting. Declarations are not authenticated producer identity.

Native source inventories, command, Editor, XML, envelope, declared build/reference byte-hash closure and optional SCIP source receipt must agree with actual current inputs. Only exact unique typed method identities join. Missing identities, filtered runs and ambiguous definitions remain unjoined. A clipped display signature requires the bounded current full-header read; unreadable or unsupported policy never proves a full cohort.

Store Format v0 gains additive native provenance and protected symbol-key columns. Native cases are isolated from old readers that would otherwise choose a passing parameterized leaf. Conflicting native run-ID replay refuses; read-only legacy evidence remains readable without requiring the new columns.

Runtime reads revalidate the active receipt and current closure, aggregate the complete method cohort and return named UNKNOWN on freshness refusal. They remain informational and do not promote native observations to live clean-commit certificates. This contract does not execute Unity, establish a language-support grade or qualify an adopting repository.

The native validator does not parse or compare NUnit-loaded test-assembly MVIDs against a compiler/assembly oracle. Declared reference hashes and source-bound SCIP definitions do not establish that loaded-assembly identity. Preserve separately observed producer/oracle facts without claiming this validator proves their MVID relationship.
