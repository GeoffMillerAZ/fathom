# scip-java — tier-A SCIP index for Java (LB-Q-java)

`scip-java` 0.12.3 (Apache-2.0, sourcegraph/scip-java), a javac plugin that writes SemanticDB per compilation unit and merges it
into one SCIP index. Bench-only: launched through coursier into a scratch cache, nothing global and nothing in the product.

    export JAVA_HOME=<Temurin 21.0.12.1+1 aarch64> PATH=$JAVA_HOME/bin:$PATH COURSIER_CACHE=<scratch>
    coursier launch -q com.sourcegraph:scip-java_2.13:0.12.3 -- index --output <out.scip> [-- build args]

`run.sh <jdk-home> <module-dir> <out.scip> [build args]` is that command (see its header). The tool runs the subject's own build
with its compiler swapped, so the build must work on JDK 21 and OFFLINE after one dependency fetch; both measured subjects do.

## Subjects (docs/bench/readings/lang/java.json)

- **google/guava v33.7.1** (c5b5a383), Maven, run in the `guava/` module directory (so document paths are `src/com/google/...`, the
  measured subject root): `ln -s ../mvnw mvnw; ln -s ../.mvn .mvn`, then
  `run.sh $JDK <clone>/guava <out> clean verify -DskipTests -Dmaven.javadoc.skip=true -Dmaven.source.skip=true -Dmaven.repo.local=<scratch>/m2`
  (about 50 s; 611 documents, 29 MB). Offline proof: the same command with `-o`, 0 downloads.
- **square/retrofit 3.0.0** (3ab0f1a2), Gradle 8.14 wrapper with `GRADLE_USER_HOME` in scratch. Three edits to the checkout, all
  build configuration, none to a source file, undone with `git checkout -- .`: the root `build.gradle` toolchain
  `JavaLanguageVersion.of(8)` becomes `21` (Temurin 8 has no arm64 macOS build) and its `net.ltgt.errorprone` plugin, dependencies and
  `options.errorprone` block are deleted (errorprone replaces the compiler's processor path, so javac fails `plug-in not found:
  semanticdb`); `retrofit/build.gradle`'s multi-release toolchains (`JavaLanguageVersion.of(version)`, Azul vendor) become `of(21)`.
  `run.sh $JDK <clone> <out> compileJava compileTestJava --continue` (the default `clean build` also runs scip-java's Kotlin plugin
  on the Kotlin test modules, which fails under it: `unknown symbol kind FirFileSymbol`, then `InvocationTargetException`). 314
  documents (the 306 Java files plus generated protobuf sources), about 20 s after the dependency fetch; `--offline` re-run: 0 downloads.

## What the index carries

- Every occurrence has a range; 335 of guava's 40,321 definition occurrences and 880 of retrofit's 10,434 carry no `enclosing_range`
  (the LB-F54 fallback, D5, supplies the mapped definition's lines for those).
- A constructor is `Foo#<init>().` (an overload `<init>(+1).`), named as its class `Foo`, which is also how `new Foo(...)` is written as a
  call (`internal/langbench/scip_java.go`, `javaName`). A nested type's constructor is the nested type's name. `<clinit>` is left as is.
- Parameters (`(x)`), type parameters (`[T]`) and `local N` symbols are not definitions fathom maps.
