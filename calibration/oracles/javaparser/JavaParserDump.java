// JavaParserDump — a tier-B Java L1/L2 oracle for fathom's language-breadth
// harness (docs/design/language-breadth.md §1.3, §10; ADR-048 amendment D2).
// Bench-only instrument: a JVM program run as a separate process, never a
// product dependency. It parses every .java file under <root> with
// JavaParser (syntax only, no symbol solving) and prints the neutral
// tool-oracle JSON `indexbench lang-oracle -tool-oracle` reads:
//
//   {"tool","version","tier","definitions":[{path,name,line,end,kind}],
//    "calls":[{path,from,to}]}
//
// kinds: class interface enum record annotation method (constructors too,
// as universal-ctags and the dialect both do) field (one per declarator)
// enumConstant annotationMember compactConstructor. Local classes and the
// members of anonymous classes are definitions like any other; an
// anonymous class itself has no name and is not one.
// imports (needs javaparser-symbol-solver-core on the classpath): every
// import declaration as {path,line,to_path,module}, line its first line,
// module the name as written, to_path the subject file JavaParser's own
// resolution (JavaSymbolSolver over a JavaParserTypeSolver per source root,
// each root derived from a file's package declaration) finds the imported
// type in ("" when it names a package, the JDK or a dependency: no file of
// the subject). A static import and a nested type resolve to the enclosing
// type's file; `import a.b.*` of a package resolves to no single file.
// calls: every method-call expression, paired with the innermost enclosing
// definition this dump emits (skipped when there is none); from and to are
// bare names.
//
// Usage: java -cp javaparser-core-<v>.jar JavaParserDump.java <root> > out.json
import com.github.javaparser.JavaParser;
import com.github.javaparser.ParserConfiguration;
import com.github.javaparser.ParseResult;
import com.github.javaparser.ast.CompilationUnit;
import com.github.javaparser.ast.Node;
import com.github.javaparser.ast.body.*;
import com.github.javaparser.ast.expr.MethodCallExpr;
import com.github.javaparser.ast.ImportDeclaration;
import com.github.javaparser.ast.PackageDeclaration;
import com.github.javaparser.resolution.TypeSolver;
import com.github.javaparser.resolution.declarations.ResolvedReferenceTypeDeclaration;
import com.github.javaparser.resolution.model.SymbolReference;
import com.github.javaparser.symbolsolver.resolution.typesolvers.CombinedTypeSolver;
import com.github.javaparser.symbolsolver.resolution.typesolvers.JavaParserTypeSolver;
import com.github.javaparser.symbolsolver.resolution.typesolvers.ReflectionTypeSolver;
import java.nio.file.*;
import java.util.*;
import java.util.stream.*;

public class JavaParserDump {
    static final String VERSION = "3.28.2";

    record Def(String path, String name, int line, int end, String kind, Node node) {}

    public static void main(String[] args) throws Exception {
        Path root = Paths.get(args[0]).toAbsolutePath().normalize();
        ParserConfiguration cfg = new ParserConfiguration()
            .setLanguageLevel(ParserConfiguration.LanguageLevel.BLEEDING_EDGE)
            .setAttributeComments(false);
        JavaParser parser = new JavaParser(cfg);
        List<Path> files;
        try (Stream<Path> s = Files.walk(root)) {
            files = s.filter(p -> p.toString().endsWith(".java") && Files.isRegularFile(p)).sorted().collect(Collectors.toList());
        }
        StringBuilder defsOut = new StringBuilder(), callsOut = new StringBuilder(), importsOut = new StringBuilder();
        int failed = 0, ndefs = 0, ncalls = 0, nimports = 0, nresolved = 0;
        Map<String, Path> unitFiles = new HashMap<>();
        CombinedTypeSolver solver = new CombinedTypeSolver();
        Set<Path> srcRoots = new TreeSet<>();
        for (Path f : files) {
            ParseResult<CompilationUnit> pr = parser.parse(f);
            if (!pr.isSuccessful() || pr.getResult().isEmpty()) continue;
            Optional<PackageDeclaration> pd = pr.getResult().get().getPackageDeclaration();
            Path dir = f.getParent();
            if (pd.isPresent()) {
                for (int i = pd.get().getNameAsString().split("\\.").length; i > 0 && dir != null; i--) dir = dir.getParent();
            }
            if (dir != null) srcRoots.add(dir);
        }
        for (Path r : srcRoots) solver.add(new JavaParserTypeSolver(r, cfg));
        // the JDK last: the subject's own sources win over a same-named class on this dump's classpath (guava's)
        solver.add(new ReflectionTypeSolver(false));
        for (Path f : files) {
            String rel = root.relativize(f).toString().replace('\\', '/');
            ParseResult<CompilationUnit> pr = parser.parse(f);
            if (!pr.isSuccessful() || pr.getResult().isEmpty()) {
                failed++;
                System.err.println("parse failed: " + rel + ": " + pr.getProblems().stream().limit(1).map(Object::toString).collect(Collectors.joining()));
                continue;
            }
            CompilationUnit cu = pr.getResult().get();
            List<Def> defs = new ArrayList<>();
            cu.walk(n -> {
                String kind = null, name = null;
                if (n instanceof ClassOrInterfaceDeclaration c) { kind = c.isInterface() ? "interface" : "class"; name = c.getNameAsString(); }
                else if (n instanceof EnumDeclaration e) { kind = "enum"; name = e.getNameAsString(); }
                else if (n instanceof RecordDeclaration r) { kind = "record"; name = r.getNameAsString(); }
                else if (n instanceof AnnotationDeclaration a) { kind = "annotation"; name = a.getNameAsString(); }
                else if (n instanceof MethodDeclaration m) { kind = "method"; name = m.getNameAsString(); }
                else if (n instanceof ConstructorDeclaration c) { kind = "method"; name = c.getNameAsString(); }
                else if (n instanceof CompactConstructorDeclaration c) { kind = "compactConstructor"; name = c.getNameAsString(); }
                else if (n instanceof AnnotationMemberDeclaration a) { kind = "annotationMember"; name = a.getNameAsString(); }
                else if (n instanceof EnumConstantDeclaration e) { kind = "enumConstant"; name = e.getNameAsString(); }
                else if (n instanceof FieldDeclaration fd) {
                    // One definition per declarator, ranged by the whole
                    // declaration (the dialect's definition node).
                    for (VariableDeclarator v : fd.getVariables()) {
                        defs.add(new Def(rel, v.getNameAsString(), fd.getBegin().get().line, fd.getEnd().get().line, "field", fd));
                    }
                    return;
                }
                if (kind != null && n.getBegin().isPresent()) {
                    defs.add(new Def(rel, name, n.getBegin().get().line, n.getEnd().get().line, kind, n));
                }
            });
            Map<Node, Def> byNode = new IdentityHashMap<>();
            for (Def d : defs) {
                byNode.putIfAbsent(d.node(), d); // a multi-declarator field: its first declarator
                append(defsOut, String.format("{\"path\":%s,\"name\":%s,\"line\":%d,\"end\":%d,\"kind\":%s}", q(d.path()), q(d.name()), d.line(), d.end(), q(d.kind())));
                ndefs++;
            }
            for (ImportDeclaration imp : cu.getImports()) {
                String name = imp.getNameAsString();
                String to = "";
                List<String> tries = new ArrayList<>();
                tries.add(name);
                if (imp.isStatic() && !imp.isAsterisk() && name.contains(".")) tries.add(name.substring(0, name.lastIndexOf('.')));
                else if (!imp.isStatic() && !imp.isAsterisk() && name.contains(".")) tries.add(name.substring(0, name.lastIndexOf('.')));
                for (String t : tries) {
                    SymbolReference<ResolvedReferenceTypeDeclaration> sr;
                    try { sr = solver.tryToSolveType(t); } catch (RuntimeException e) { continue; }
                    if (!sr.isSolved()) continue;
                    Optional<Node> ast = sr.getCorrespondingDeclaration().toAst();
                    if (ast.isEmpty()) break;
                    Optional<CompilationUnit> dcu = ast.get().findCompilationUnit();
                    if (dcu.isEmpty() || dcu.get().getStorage().isEmpty()) break;
                    Path tp = dcu.get().getStorage().get().getPath().toAbsolutePath().normalize();
                    if (tp.startsWith(root)) to = root.relativize(tp).toString().replace('\\', '/');
                    break;
                }
                int line = imp.getBegin().isPresent() ? imp.getBegin().get().line : 0;
                String mod = (imp.isStatic() ? "static " : "") + name + (imp.isAsterisk() ? ".*" : "");
                append(importsOut, String.format("{\"path\":%s,\"line\":%d,\"to_path\":%s,\"module\":%s}", q(rel), line, q(to), q(mod)));
                nimports++;
                if (!to.isEmpty()) nresolved++;
            }
            Set<String> seen = new TreeSet<>();
            for (MethodCallExpr call : cu.findAll(MethodCallExpr.class)) {
                Node p = call.getParentNode().orElse(null);
                while (p != null && !byNode.containsKey(p)) p = p.getParentNode().orElse(null);
                if (p == null) continue;
                Def enc = byNode.get(p);
                if (enc.kind().equals("field")) {
                    // every declarator of the field encloses its own initializer
                    for (VariableDeclarator v : ((FieldDeclaration) p).getVariables()) {
                        if (v.getInitializer().isPresent() && isAncestor(v, call)) {
                            seen.add(v.getNameAsString() + "\u0000" + call.getNameAsString());
                        }
                    }
                    continue;
                }
                seen.add(enc.name() + "\u0000" + call.getNameAsString());
            }
            for (String s : seen) {
                String[] parts = s.split("\u0000");
                append(callsOut, String.format("{\"path\":%s,\"from\":%s,\"to\":%s}", q(rel), q(parts[0]), q(parts[1])));
                ncalls++;
            }
        }
        System.err.printf("files=%d parse_failed=%d definitions=%d calls=%d imports=%d resolved_to_subject=%d%n", files.size(), failed, ndefs, ncalls, nimports, nresolved);
        System.out.print("{\"tool\":\"javaparser\",\"version\":\"" + VERSION + "\",\"tier\":\"B\",\"definitions\":[");
        System.out.print(defsOut);
        System.out.print("],\"calls\":[");
        System.out.print(callsOut);
        System.out.print("],\"imports\":[");
        System.out.print(importsOut);
        System.out.println("]}");
    }

    static boolean isAncestor(Node anc, Node n) {
        for (Node p = n; p != null; p = p.getParentNode().orElse(null)) if (p == anc) return true;
        return false;
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
