# JavaParserDump — tier-B Java L1/L2 oracle, and the import-to-file truth (LB-F71)

ADR-048 amendment D2: universal-ctags 6.2.1 undercounts local (in-method)
Java classes and their methods, holding sym_precision under .90, so JavaParser
is the L1 oracle (tier B). scip-java (calibration/oracles/scip-java, tier A) now
runs on both subjects and gives calls, def_mapping, typed_recall, imports and
file edges; one calls oracle and one imports oracle per subject, so beside it
this dump is given with `calls` emptied and `imports` removed (L1 definitions
only), exactly as the php-parser dump is. Run alone, without -scip, its calls
are the tier-B L2 truth and its imports the file-edge truth (LB-F57).
Bench-only: a JVM program run as a separate process.

`imports`: every import declaration as `{path, line, to_path, module}`, `to_path`
the subject file JavaParser's own resolution gives the imported type: a
`JavaSymbolSolver`-family `CombinedTypeSolver` of one `JavaParserTypeSolver`
per source root (each derived from a file's package declaration, so a
multi-module checkout needs no configuration) and then `ReflectionTypeSolver`
last (the dump's own classpath carries guava, which must not shadow the
subject's copy). A static import and a nested type resolve to the enclosing
type's file; a package wildcard, a JDK type and a dependency's type resolve to
no file (`to_path` ""). Needs javaparser-symbol-solver-core on the classpath:

    coursier fetch com.github.javaparser:javaparser-symbol-solver-core:3.28.2   # + javassist, guava, ...
    java -cp "$(coursier fetch ... | tr '\n' ':')" JavaParserDump.java <root> > out.json   # root = the SCIP source root

    curl -O https://repo1.maven.org/maven2/com/github/javaparser/javaparser-core/3.28.2/javaparser-core-3.28.2.jar
    # sha256 b5499a3b1c40b16c0671fabe478c9aafeab38160c6fde74a6c13f42d86716ecd
    java -cp <the classpath above> JavaParserDump.java <clone>/retrofit        > out/java-retrofit.json  # 306 files, 4,279 defs, 9,412 call pairs, 2,952 imports (627 to a file of the subject)
    java -cp <the classpath above> JavaParserDump.java <clone>/guava > out/java-guava.json     # 611 files (paths src/com/...), 17,494 defs, 18,646 call pairs, 6,522 imports (2,526 to a file)

JDK: Eclipse Temurin 21.0.12.1+1 aarch64 tarball (api.adoptium.net), no
install. 0 parse failures on both subjects. Kinds are mapped by
dialects/java/dialect.json `oracle_kinds.tool` (annotationMember and
compactConstructor are emitted but outside the dialect's kinds). Retrofit's
RetrofitTest.java: 9 classes / 142 methods, the count LB-Q-java verified by
hand against the source (ctags: 1 / 99).
