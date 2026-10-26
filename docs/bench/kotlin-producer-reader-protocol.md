# C-R5 bounded Kotlin producer/reader audit

Freeze before the producer run:

- Exposed1.3.0, source a68a26884952875722c1802ef42b382876d7d056, Apache-2.0.
  Build the three original main modules exposed-core, exposed-dao and exposed-jdbc.
  Preserve source/build configuration and the subject's own compiler version.
- scip-java0.13.2 release fat-jar launcher, SHA256
  5248136592d19ea2c468f64ebd18587d32de7c961536ff19346f8a025af1d13c.
- Temurin17.0.21+9 aarch64 macOS JDK, SHA256
  78b08c7f5a0598242d5719c022e19479cb4a3aa45c37c19726a757a469ed1c15.
  scip-java's pinned setup docs require JDK17 or newer. SDK, Gradle/coursier caches,
  output and temporary files remain isolated under scratch. No global installation.
- nice10, wait for one-minute load<=9 before each heavy phase, CPU count2,
  Gradle workers2 and in-process Kotlin compilation. Cap the actual index/build
  command at20minutes. Limit the indexer heap to256MB and Gradle to640MB.

Use the subject's actual Gradle build, not substitute source. Record checksum/version,
source pin, exact command, exit, failure and elapsed time. The intended command is:

```sh
scip-java index --output "$KOTLIN_AUDIT_SCRATCH/exposed.scip" -- \
  --no-daemon --max-workers=2 \
  '-Dorg.gradle.jvmargs=-Xmx640m -XX:ActiveProcessorCount=2' \
  -Pkotlin.compiler.execution.strategy=in-process \
  :exposed-core:compileKotlin :exposed-dao:compileKotlin :exposed-jdbc:compileKotlin
```

If an index is produced, census its raw occurrence fields1/8/9 and enclosing7/10/11
with the committed `calibration/oracles/scip-wire-census/census.py`. Compare those
counts with the bundled reader's legacy GetRange/GetEnclosingRange contract before
blaming the producer. Keep source SHA, index hash, document/occurrence counts and
encoding samples, with no private metadata paths in public evidence.

A producer/build failure is a concrete limit of that named route, not proof that no
other Kotlin indexer can work. A valid typed-only index would demonstrate a reader
compatibility gap requiring a separate ENGINE/K29 wave. Neither outcome promotes a
language level. No production bindings, engine, denominator, generated board, reading
or canonical corpus changes are authorized by this audit. Existing spent holdouts
stay outside it. Small synthetic codec self-tests prove the instrument only.

## Preregistered second route after buildSrc failure

The unmodified automatic route failed in57seconds at buildSrc:compileKotlin with
NoClassDefFoundError for org.jetbrains.kotlin.extensions.ExtensionPointDescriptor,
before any requested module compiled. The indexer injects its compiler plugin into
the build-logic compiler too. No SCIP index was produced.

One additional bounded route uses native Gradle init-script configuration, with no
source, version-catalog or module-compiler edits. First run the pristine build's
`gradlew help` to compile its original buildSrc normally. Then, only during indexing,
an init script marks buildSrc's compileKotlin task `onlyIf { false }`, reusing those
compiled build-logic classes while leaving all three requested module compilations
active. This is an audit of those modules, not a full-corpus grade or buildSrc proof.
Do not disable any requested module task or substitute source. Verify a clean subject
before/after and retain a hash of tracked Kotlin sources. Stop the combined warm-up
and producer attempt after20minutes. Any new failure stays a named route limit.


The committed init script is
`calibration/oracles/scip-wire-census/skip-prebuilt-buildsrc.gradle`. From the clean
pinned subject, with `SCIP_JAVA` naming the checksum-verified release launcher and
`KOTLIN_AUDIT_INIT` naming that script by absolute path, the second route is:

```sh
./gradlew help --no-daemon --max-workers=2 \
  '-Dorg.gradle.jvmargs=-Xmx640m -XX:ActiveProcessorCount=2' \
  -Pkotlin.compiler.execution.strategy=in-process
"$SCIP_JAVA" index --output "$KOTLIN_AUDIT_SCRATCH/exposed-prebuilt.scip" -- \
  --no-daemon --max-workers=2 \
  '-Dorg.gradle.jvmargs=-Xmx640m -XX:ActiveProcessorCount=2' \
  -Pkotlin.compiler.execution.strategy=in-process \
  --init-script "$KOTLIN_AUDIT_INIT" \
  :exposed-core:compileKotlin :exposed-dao:compileKotlin :exposed-jdbc:compileKotlin
```

## Measured outcome

Both indexing routes failed with missing
`org.jetbrains.kotlin.extensions.ExtensionPointDescriptor`. The first failed in
buildSrc (Gradle reported 57 seconds). The pristine warmup succeeded in 81.745 seconds;
the second index then failed at `exposed-core:compileKotlin` in 13.842 seconds.
No SCIP file was emitted. The subject's tracked Kotlin source hash and clean checkout
were unchanged. The official checksum-verified release launcher reports its version
as `0.0.0-SNAPSHOT`; nested producer artifacts identify 0.13.2. No index exists for a
wire census, so the reader hypothesis remains unproven and no language grade changes.
The [sanitized result](../assessments/data/2026-10-26-core-t1-kotlin-producer.json)
records source identity and log hashes. Raw logs contain local build paths and stay
in workspace.
