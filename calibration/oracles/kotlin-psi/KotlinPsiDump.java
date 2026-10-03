// KotlinPsiDump — a tier-B Kotlin L1/L2 oracle for fathom's language-breadth
// harness (docs/design/language-breadth.md §1.3, §10; ADR-048 amendment D2).
// Bench-only instrument: a JVM program run as a separate process, never a
// product dependency. It parses every .kt and .kts file under <root> with the
// Kotlin compiler's own PSI parser (kotlin-compiler-embeddable: the reference
// parser of the language, syntax only, no resolution) and prints the neutral
// tool-oracle JSON `indexbench lang-oracle -tool-oracle` reads:
//
//   {"tool","version","tier","definitions":[{path,name,line,end,kind}],
//    "calls":[{path,from,to}]}
//
// kinds: class (class, enum class, annotation class, data/value/sealed class),
// interface (interface, fun interface), object (a named object declaration or
// a named companion; an unnamed companion and an anonymous object expression
// have no name and are not definitions), function (every named function:
// top-level, member, extension, local), constructor (a secondary
// constructor), property (top-level, member and primary-constructor val/var;
// never a local variable, a destructuring entry or a function parameter),
// typealias, enumEntry. line is the row of the name (the `constructor`
// keyword for a constructor), end the last row of the whole declaration.
// A backticked name with spaces or dots (`decode simple certificate`, a test
// name) is emitted on its last segment, the form the harness compares a
// dialect's names in (design §1.4: names are compared on the last segment).
// calls: every call expression whose callee is a plain name (`f(x)`,
// `a.b.f(x)`, `f<T>(x)`, a constructor call `Foo(x)`, a trailing-lambda call
// `f { }`), paired with the innermost enclosing function or constructor this
// dump emits (skipped when there is none: a property initialiser or accessor,
// an init block, a supertype initialiser or a script's top-level statement
// belongs to no callable definition, the model fathom's SymbolNode.Calls
// follows); from and to are bare names. A call whose callee is not a name
// (`f()()`, `(g)(x)`) has no callee name and is not a pair.
//
// imports (O474, LB-F57's imports oracle for Kotlin L3): every
// KtImportDirective of a file as {path, line, to_path, module}: line is the
// directive's 1-based row, module the imported name as written (`a.b.C`,
// `a.b.C as D` is `a.b.C`, a star import `a.b.*`) and to_path the subject
// file the directive names. Syntax only, so the join is by declaration, never
// by resolution: a named import `a.b.C` (or `a.b.C.member`, an enum entry, a
// nested class) takes the file(s) of package `a.b` that declare a top-level
// `C` (class, interface, object, function, property or typealias), the
// longest package prefix first; a star import `a.b.*` takes every file whose
// package is `a.b` (or, when `a.b` is no package of the subject, the file(s)
// declaring a top-level `b` in `a`, `import a.B.*` over a class's members). A
// name declared in more than one file (a multiplatform expect and its
// actuals) is one entry per file; a directive that names no file of the
// subject (a JDK or dependency type) is one entry with to_path "". Each file's
// package (KtFile.getPackageFqName) feeds the join and is not written out: the
// harness's oracle reader refuses a key it does not know.
//
// Usage: java -cp <ktc>/'*' KotlinPsiDump.java <root> > out.json
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.util.*;
import java.util.stream.*;
import org.jetbrains.kotlin.cli.common.messages.MessageCollector;
import org.jetbrains.kotlin.cli.jvm.compiler.EnvironmentConfigFiles;
import org.jetbrains.kotlin.cli.jvm.compiler.KotlinCoreEnvironment;
import org.jetbrains.kotlin.com.intellij.openapi.Disposable;
import org.jetbrains.kotlin.com.intellij.openapi.util.Disposer;
import org.jetbrains.kotlin.com.intellij.psi.PsiElement;
import org.jetbrains.kotlin.com.intellij.psi.PsiErrorElement;
import org.jetbrains.kotlin.com.intellij.psi.PsiFileFactory;
import org.jetbrains.kotlin.com.intellij.psi.util.PsiTreeUtil;
import org.jetbrains.kotlin.config.CommonConfigurationKeys;
import org.jetbrains.kotlin.config.CompilerConfiguration;
import org.jetbrains.kotlin.idea.KotlinLanguage;
import org.jetbrains.kotlin.psi.*;

public class KotlinPsiDump {
    static final String VERSION = "2.3.20";

    record Def(String name, int line, int end, String kind, PsiElement node) {}

    static int[] lineStarts;

    static int lineOf(int offset) {
        int i = Arrays.binarySearch(lineStarts, offset);
        return (i >= 0 ? i : -i - 2) + 1;
    }

    public static void main(String[] args) throws Exception {
        Path root = Paths.get(args[0]).toAbsolutePath().normalize();
        CompilerConfiguration cfg = new CompilerConfiguration();
        cfg.put(CommonConfigurationKeys.MESSAGE_COLLECTOR_KEY, MessageCollector.Companion.getNONE());
        cfg.put(CommonConfigurationKeys.MODULE_NAME, "dump");
        Disposable disposable = Disposer.newDisposable();
        KotlinCoreEnvironment env = KotlinCoreEnvironment.createForProduction(disposable, cfg, EnvironmentConfigFiles.JVM_CONFIG_FILES);
        PsiFileFactory factory = PsiFileFactory.getInstance(env.getProject());

        List<Path> files;
        try (Stream<Path> s = Files.walk(root)) {
            files = s.filter(p -> {
                String n = p.toString();
                return (n.endsWith(".kt") || n.endsWith(".kts")) && Files.isRegularFile(p) && !n.contains("/.git/");
            }).sorted().collect(Collectors.toList());
        }
        StringBuilder defsOut = new StringBuilder(), callsOut = new StringBuilder(), importsOut = new StringBuilder();
        int withErrors = 0, ndefs = 0, ncalls = 0, nimports = 0, nresolved = 0;
        // package -> files declaring in it, and "package\0name" -> files declaring that top-level name.
        Map<String, List<String>> filesOfPackage = new TreeMap<>();
        Map<String, List<String>> filesOfDecl = new TreeMap<>();
        record Imp(String path, int line, String module, String fq, boolean star) {}
        List<Imp> imps = new ArrayList<>();
        for (Path f : files) {
            String rel = root.relativize(f).toString().replace('\\', '/');
            String text = new String(Files.readAllBytes(f), StandardCharsets.UTF_8).replace("\r\n", "\n");
            List<Integer> starts = new ArrayList<>();
            starts.add(0);
            for (int i = 0; i < text.length(); i++) if (text.charAt(i) == '\n') starts.add(i + 1);
            lineStarts = starts.stream().mapToInt(Integer::intValue).toArray();
            PsiElement file = factory.createFileFromText(f.getFileName().toString(), KotlinLanguage.INSTANCE, text);
            if (!PsiTreeUtil.findChildrenOfType(file, PsiErrorElement.class).isEmpty()) withErrors++;
            if (file instanceof KtFile kf) {
                String pkg = kf.getPackageFqName().asString();
                filesOfPackage.computeIfAbsent(pkg, k -> new ArrayList<>()).add(rel);
                for (KtDeclaration td : kf.getDeclarations()) {
                    String tn = td instanceof KtNamedDeclaration nd ? nd.getName() : null;
                    if (tn == null || tn.isEmpty()) continue;
                    List<String> l = filesOfDecl.computeIfAbsent(pkg + "\u0000" + tn, k -> new ArrayList<>());
                    if (!l.contains(rel)) l.add(rel);
                }
                for (KtImportDirective imp : kf.getImportDirectives()) {
                    org.jetbrains.kotlin.name.FqName fq = imp.getImportedFqName();
                    if (fq == null) continue;
                    boolean star = imp.isAllUnder();
                    imps.add(new Imp(rel, lineOf(imp.getTextRange().getStartOffset()), fq.asString() + (star ? ".*" : ""), fq.asString(), star));
                }
            }

            List<Def> defs = new ArrayList<>();
            Map<PsiElement, Def> byNode = new IdentityHashMap<>();
            collect(file, rel, defs, byNode);
            for (Def d : defs) {
                append(defsOut, String.format("{\"path\":%s,\"name\":%s,\"line\":%d,\"end\":%d,\"kind\":%s}", q(rel), q(d.name()), d.line(), d.end(), q(d.kind())));
                ndefs++;
            }
            Set<String> seen = new TreeSet<>();
            for (KtCallExpression call : PsiTreeUtil.findChildrenOfType(file, KtCallExpression.class)) {
                KtExpression callee = call.getCalleeExpression();
                if (!(callee instanceof KtSimpleNameExpression)) continue;
                String to = bare(((KtSimpleNameExpression) callee).getReferencedName());
                PsiElement p = call.getParent();
                Def enc = null;
                while (p != null) {
                    Def d = byNode.get(p);
                    if (d != null && (d.kind().equals("function") || d.kind().equals("constructor"))) { enc = d; break; }
                    p = p.getParent();
                }
                if (enc == null) continue;
                seen.add(enc.name() + "\u0000" + to);
            }
            for (String s : seen) {
                String[] parts = s.split("\u0000");
                append(callsOut, String.format("{\"path\":%s,\"from\":%s,\"to\":%s}", q(rel), q(parts[0]), q(parts[1])));
                ncalls++;
            }
        }
        for (Imp im : imps) {
            for (String to : joinImport(im.fq(), im.star(), filesOfPackage, filesOfDecl)) {
                if (to.equals(im.path())) continue;
                append(importsOut, String.format("{\"path\":%s,\"line\":%d,\"to_path\":%s,\"module\":%s}", q(im.path()), im.line(), q(to), q(im.module())));
                nimports++;
                if (!to.isEmpty()) nresolved++;
            }
        }
        System.err.printf("files=%d with_syntax_errors=%d definitions=%d calls=%d imports=%d resolved_to_subject=%d%n", files.size(), withErrors, ndefs, ncalls, nimports, nresolved);
        System.out.print("{\"tool\":\"kotlin-psi\",\"version\":\"" + VERSION + "\",\"tier\":\"B\",\"definitions\":[");
        System.out.print(defsOut);
        System.out.print("],\"calls\":[");
        System.out.print(callsOut);
        System.out.print("],\"imports\":[");
        System.out.print(importsOut);
        System.out.println("]}");
        Disposer.dispose(disposable);
    }

    // The subject files a directive names, or [""] when it names none (a JDK or dependency type).
    static List<String> joinImport(String fq, boolean star, Map<String, List<String>> filesOfPackage, Map<String, List<String>> filesOfDecl) {
        if (star && filesOfPackage.containsKey(fq)) return new ArrayList<>(new TreeSet<>(filesOfPackage.get(fq)));
        String[] seg = fq.split("\\.");
        // longest package prefix first: a.b.C.member is class C of package a.b before it is anything of a.b.C
        for (int i = seg.length - 1; i >= 1; i--) {
            String pkg = String.join(".", Arrays.copyOfRange(seg, 0, i));
            List<String> fs = filesOfDecl.get(pkg + "\u0000" + seg[i]);
            if (fs != null) return new ArrayList<>(new TreeSet<>(fs));
        }
        return List.of("");
    }

    static void collect(PsiElement e, String rel, List<Def> defs, Map<PsiElement, Def> byNode) {
        Def d = null;
        if (e instanceof KtEnumEntry en) {
            d = def(en, en.getNameIdentifier(), "enumEntry");
        } else if (e instanceof KtClass c) {
            d = def(c, c.getNameIdentifier(), c.isInterface() ? "interface" : "class");
        } else if (e instanceof KtObjectDeclaration o) {
            d = def(o, o.getNameIdentifier(), "object");
        } else if (e instanceof KtNamedFunction fn) {
            d = def(fn, fn.getNameIdentifier(), "function");
        } else if (e instanceof KtSecondaryConstructor sc) {
            d = def(sc, sc.getConstructorKeyword(), "constructor", "constructor");
        } else if (e instanceof KtTypeAlias ta) {
            d = def(ta, ta.getNameIdentifier(), "typealias");
        } else if (e instanceof KtProperty p) {
            if (!p.isLocal()) d = def(p, p.getNameIdentifier(), "property");
        } else if (e instanceof KtParameter par) {
            if (par.hasValOrVar() && par.getParent() != null && par.getParent().getParent() instanceof KtPrimaryConstructor) {
                d = def(par, par.getNameIdentifier(), "property");
            }
        }
        if (d != null) {
            defs.add(d);
            byNode.put(e, d);
        }
        for (PsiElement c = e.getFirstChild(); c != null; c = c.getNextSibling()) collect(c, rel, defs, byNode);
    }

    static Def def(PsiElement node, PsiElement nameNode, String kind) {
        return def(node, nameNode, kind, nameNode == null ? null : bare(nameNode.getText().replace("`", "")));
    }

    static Def def(PsiElement node, PsiElement nameNode, String kind, String name) {
        if (nameNode == null || name == null || name.isEmpty()) return null;
        int line = lineOf(nameNode.getTextRange().getStartOffset());
        int end = lineOf(Math.max(node.getTextRange().getStartOffset(), node.getTextRange().getEndOffset() - 1));
        return new Def(name, line, end, kind, node);
    }

    // The last segment of a name, cut at any of ".:#/\\ )" (langbench.BareName).
    static String bare(String n) {
        int i = -1;
        for (int k = 0; k < n.length(); k++) if (".:#/\\ )".indexOf(n.charAt(k)) >= 0) i = k;
        return n.substring(i + 1);
    }

    static void append(StringBuilder sb, String item) {
        if (sb.length() > 0) sb.append(',');
        sb.append('\n').append(item);
    }

    static String q(String s) {
        StringBuilder b = new StringBuilder("\"");
        for (char c : s.toCharArray()) {
            switch (c) {
                case '"' -> b.append("\\\"");
                case '\\' -> b.append("\\\\");
                default -> { if (c < 0x20) b.append(String.format("\\u%04x", (int) c)); else b.append(c); }
            }
        }
        return b.append('"').toString();
    }
}
