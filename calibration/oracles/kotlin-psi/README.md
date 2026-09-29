# KotlinPsiDump — tier-B Kotlin L1/L2 oracle (syntax only)

ADR-048 amendment D2: universal-ctags 6.2.1 is deficient for Kotlin twice over. It writes no
end line for any definition (so range agreement cannot be measured and L1 is capped at good),
and it reports every local `val`/`var`, every primary-constructor parameter and every lambda
(`<lambda>`, and the name it is assigned to) as a definition and every unnamed companion
object as `Companion`: on square/okhttp and JetBrains/Exposed (1,363 files) it lists 60,761
definitions against the dialect's 16,335 with 97% precision, so recall reads .26 for a reason
no query can fix. A scip-java index cannot stand in for it either: scip-java 0.13.1's Kotlin plugin
refuses Exposed's Kotlin 2.3.20, and under 2.2.0 (with four `Uuid.generateV4/V7` calls shimmed) it
indexed only what Gradle compiled (Exposed's three main modules, 166 of 794 files) and wrote all
58,698 occurrences without a range or a symbol kind; okhttp's Kotlin Multiplatform build needs an
Android SDK. What it does write speaks the JVM's vocabulary (a Kotlin property is a field plus
`getX()`/`setX()`, a constructor is `<init>`), so even with ranges it would be the L3/L4 oracle,
not the L1 one.
KotlinPsiDump parses every file with the Kotlin compiler's own parser (PSI, from
`kotlin-compiler-embeddable`, no resolution): the reference parser of the language, run as a
separate process, reporting the definitions the dialect declares and a range for each.

Bench-only: a JVM program, never a product dependency. The tier is B on the D2 precedent
(JavaParser for Java, Roslyn for C#): an independent syntactic parser, so L1 can be excellent
and L2 (calls) at most good.

    # kotlin-compiler-embeddable 2.3.20 sha256 976f989d0b5f5d80e8e8a8ad4b73da0bfc27fdd965b9fa38362b2be79ecc1337
    # plus kotlin-stdlib, kotlin-script-runtime, kotlin-daemon-embeddable 2.3.20 (Maven Central),
    # kotlin-reflect 1.6.10, kotlinx-coroutines-core-jvm 1.8.0, trove4j 1.0.20200330 and
    # annotations 13.0, all in one directory <ktc>
    java -cp '<ktc>/*' KotlinPsiDump.java <clone>/okhttp > out/kotlin-okhttp.json
    java -cp '<ktc>/*' KotlinPsiDump.java <clone>/exposed > out/kotlin-exposed.json

JDK: Eclipse Temurin 21.0.12.1+1 aarch64, no install. The dump prints `files`,
`with_syntax_errors`, `definitions` and `calls` on stderr; PSI recovers from syntax errors,
so a file with errors still contributes what it parsed.

What it emits (the manifest's `oracle_kinds.tool` maps the kinds): `class` (class, enum class,
annotation class, data/value/sealed class), `interface` (interface, fun interface), `object` (a
named object declaration or a named companion; an unnamed companion and an object expression
have no name), `function` (every named function: top-level, member, extension, local),
`constructor` (a secondary constructor, named `constructor`), `property` (top-level, member and
primary-constructor `val`/`var`, never a local variable, a destructuring entry or a plain
parameter), `typealias`, `enumEntry`. `line` is the name's row, `end` the declaration's last row.
Calls are every call expression whose callee is a plain name, paired with the innermost
enclosing function or constructor (a property initialiser, accessor, `init` block or supertype
initialiser belongs to no callable definition, which is also how the dialect attaches calls).
