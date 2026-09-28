# JavaParserDump — tier-B Java L1/L2 oracle (syntax only)

ADR-048 amendment D2: universal-ctags 6.2.1 undercounts local (in-method)
Java classes and their methods, holding sym_precision under .90. scip-java
was not re-attempted this round: LB-Q-java's wave-1b attempt is on record
(retrofit's Gradle build pins a JDK 8 toolchain with no arm64 build here;
guava's Maven build under JDK 21 gave scip-java v0.13.1 "No SCIP shards
found"), and no Maven or Gradle is installed. JavaParser is the D2 fallback.
Bench-only: a JVM program run as a separate process.

    curl -O https://repo1.maven.org/maven2/com/github/javaparser/javaparser-core/3.28.2/javaparser-core-3.28.2.jar
    # sha256 b5499a3b1c40b16c0671fabe478c9aafeab38160c6fde74a6c13f42d86716ecd
    java -cp javaparser-core-3.28.2.jar JavaParserDump.java <clone>/retrofit        > out/java-retrofit.json  # 306 files, 4,279 defs, 9,412 call pairs
    java -cp javaparser-core-3.28.2.jar JavaParserDump.java <clone>/guava/guava/src > out/java-guava.json     # 611 files, 17,494 defs, 18,646 call pairs

JDK: Eclipse Temurin 21.0.12.1+1 aarch64 tarball (api.adoptium.net), no
install. 0 parse failures on both subjects. Kinds are mapped by
dialects/java/dialect.json `oracle_kinds.tool` (annotationMember and
compactConstructor are emitted but outside the dialect's kinds). Retrofit's
RetrofitTest.java: 9 classes / 142 methods, the count LB-Q-java verified by
hand against the source (ctags: 1 / 99).
