# C-R5 bounded Kotlin producer/reader audit

Freeze before the producer run:

- Exposed 1.3.0, source a68a26884952875722c1802ef42b382876d7d056, Apache-2.0.
  Build the three original main modules exposed-core, exposed-dao and exposed-jdbc.
  Preserve source/build configuration and the subject's own compiler version.
- scip-java 0.13.2 release fat-jar launcher, SHA256
  5248136592d19ea2c468f64ebd18587d32de7c961536ff19346f8a025af1d13c.
- Temurin 17.0.21+9 aarch64 macOS JDK, SHA256
  78b08c7f5a0598242d5719c022e19479cb4a3aa45c37c19726a757a469ed1c15.
  scip-java's pinned setup docs require JDK 17 or newer. SDK, Gradle/coursier caches,
  output and temporary files remain isolated under scratch. No global installation.
- nice 10, wait for one-minute load<=9 before each heavy phase, CPU count 2,
  Gradle workers 2 and in-process Kotlin compilation. Cap the actual index/build
  command at 20 minutes. Limit the indexer heap to 256 MB and Gradle to 640 MB.

Use the subject's actual Gradle build, not substitute source. Record checksum/version,
source pin, exact command, exit, failure and elapsed time. The original command body was (the historical workspace driver also applied the caps recorded below):

```sh
scip-java index --output "$KOTLIN_AUDIT_SCRATCH/exposed.scip" -- \
  --no-daemon --max-workers=2 \
  '-Dorg.gradle.jvmargs=-Xmx640m -XX:ActiveProcessorCount=2' \
  -Pkotlin.compiler.execution.strategy=in-process \
  :exposed-core:compileKotlin :exposed-dao:compileKotlin :exposed-jdbc:compileKotlin
```

If an index is produced, census its raw occurrence fields 1/8/9 and enclosing 7/10/11
with the committed `calibration/oracles/scip-wire-census/census.py`. Compare those
counts with the bundled reader's legacy GetRange/GetEnclosingRange contract before
blaming the producer. Keep source SHA, index hash, document/occurrence counts and
encoding samples, with no private metadata paths in public evidence.

Tagged producer source is reviewed independently of whether a real index exists.
A producer/build failure is a concrete limit of that named route, not proof that no
other Kotlin indexer can work. Tagged producer source already establishes a typed-only reader compatibility gap
(see the assessment); a real index census will confirm its wire counts. Repairing
the bundled proto and reader requires a separate ENGINE/K29 wave. Neither outcome promotes a
language level. No production bindings, engine, denominator, generated board, reading
or canonical corpus changes are authorized by this audit. Existing spent holdouts
stay outside it. Small synthetic codec self-tests prove the instrument only.

## Preregistered second route after buildSrc failure

The unmodified automatic route failed in 57 seconds at buildSrc:compileKotlin with
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
and producer attempt after 20 minutes. Any new failure stays a named route limit.


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
wire census. Separately, tagged producer source proves typed-only writes and the
bundled reader lacks those fields; this is source-read evidence, not a new grade.
The [sanitized result](../assessments/data/2026-10-26-core-t1-kotlin-producer.json)
records source identity and log hashes. Raw logs contain local build paths and stay
in workspace.

## Committed bounded runner and evidence

Run from the repository root with caller-supplied absolute paths. The producer
checksum is fixed in the runner. Use a fresh scratch directory for each route:

```sh
python3 calibration/oracles/scip-wire-census/run-producer.py --self-test
python3 calibration/oracles/scip-wire-census/run-producer.py \
  --producer "$SCIP_JAVA" --jdk "$KOTLIN_AUDIT_JDK" \
  --subject "$KOTLIN_AUDIT_SUBJECT" --scratch "$KOTLIN_AUDIT_SCRATCH" --route direct
# Repeat with a different fresh scratch directory for --route prebuilt.
```

The runner verifies the pinned subject and producer, waits for load <= 9, applies
nice 10, a combined 1,200-second active-command timeout, two processors/workers,
`JAVA_OPTS=-Xmx256m -XX:ActiveProcessorCount=2` for the launcher's JVM and the
explicit 640 MB Gradle JVM argument. The pinned launcher header expands JAVA_OPTS.
A timeout terminates the process group, then kills it after five seconds if needed.
Gradle and Coursier caches, temporary files, logs and exact local argv stay in
scratch. The JSON receipt records exit codes, applied caps and source checks.

This runner was added after the original measurements; its self-test checks exit
and timeout cleanup, not a replay of the subject. The original workspace drivers
used the same caps and commands. The committed sanitized result records their
exit codes and failing task/top frames. It never copies subject source or file
lists. The source digest is SHA-256 over byte-sorted tracked `.kt`/`.kts` names,
each followed by NUL and the raw 32-byte SHA-256 of its file bytes. Both original
routes used this recipe (794 files). Source alignment in each document's declared
SCIP position encoding remains a separate check on the first real index.
