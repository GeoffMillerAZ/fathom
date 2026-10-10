// JenkinsfileAstDump — a tier-C oracle for the jenkinsfile dialect
// (LB-Q-jenkinsfile; docs/design/language-breadth.md §1.3, §10). Bench-only
// instrument: a JVM program run as a separate process, never a product
// dependency. It parses every file the dialect's manifest claims under <root>
// (the basename Jenkinsfile, the extension .jenkinsfile, vars/<name>.groovy)
// with the Groovy compiler itself (org.apache.groovy:groovy, Apache-2.0),
// stopping at the CONVERSION phase (the AST as written: nothing is resolved,
// so a Jenkins step or a library class need not exist), then walks the AST
// and prints the neutral tool-oracle JSON `indexbench lang-oracle
// -tool-oracle` reads:
//
//   {"tool","version","tier":"C","definitions":[{path,name,line,end,kind}],
//    "references":[{path,from,to}],"cross_references":[{path,from,to_path,to}]}
//
// Tier C: the parser is Groovy's own, but the walk (this file), not the
// compiler, says what a stage or a section is, so the reading is capped at
// good. What the walk names (kinds are the dialect's canonical kinds):
//
//   pipeline (job)            a call pipeline { } (one closure argument)
//   stage (stage)             stage('name') { } (a string and a closure), named
//                             by the string as written, one pair of quotes off
//   stages, post, parameters, options (section), steps (step),
//   environment (variable_decl), agent (resource)
//                             a call of that name with one closure argument
//                             (agent also with one name or string: agent any);
//                             named by the keyword, qualified by the stage that
//                             immediately encloses it (Build.steps)
//   function (function)       a script-level method (def name(...) { })
//   library (import)          @Library('a@ref') / @Library(['a', 'b']) on a
//                             declaration or an import, library 'a',
//                             library identifier: 'a'
//
// references: every implicit-this method call (a bare name: sh '...',
// echo(...), helper()) paired with the innermost definition enclosing it,
// skipped when none does; a section keyword (pipeline, agent, stages, stage,
// steps, post, environment, parameters, options) is never a reference.
// cross_references: each reference whose callee names a vars/<name>.groovy of
// the subject that declares a script-level `call`, to that file's `call`.
// A file the compiler refuses is counted on stderr and contributes nothing.
//
// Usage: java -cp <groovy-4.0.x.jar> JenkinsfileAstDump.java <root> > out.json
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.util.*;
import java.util.stream.*;
import org.codehaus.groovy.ast.*;
import org.codehaus.groovy.ast.expr.*;
import org.codehaus.groovy.ast.stmt.*;
import org.codehaus.groovy.control.*;

public class JenkinsfileAstDump {
    static final Set<String> SECTIONS = Set.of("stages", "steps", "post", "environment", "parameters", "options");
    static final Set<String> KEYWORDS = Set.of("pipeline", "agent", "stages", "stage", "steps", "post", "environment", "parameters", "options");
    static final Map<String, String> KIND = Map.of(
        "pipeline", "job", "agent", "resource", "stages", "section", "stage", "stage", "steps", "step",
        "post", "section", "environment", "variable_decl", "parameters", "section", "options", "section");

    record Def(String path, String name, int line, int end, String kind, boolean container) {}
    record Ref(String path, String from, String to) {}

    static final List<Def> defs = new ArrayList<>();
    static final List<Ref> refs = new ArrayList<>();

    public static void main(String[] args) throws Exception {
        Path root = Paths.get(args[0]).toAbsolutePath().normalize();
        List<Path> files;
        try (Stream<Path> s = Files.walk(root)) {
            files = s.filter(Files::isRegularFile).filter(p -> !root.relativize(p).toString().startsWith(".git/"))
                .filter(p -> claimed(root.relativize(p).toString().replace('\\', '/'))).sorted().collect(Collectors.toList());
        }
        int refused = 0;
        for (Path f : files) {
            String rel = root.relativize(f).toString().replace('\\', '/');
            String src = Files.readString(f, StandardCharsets.UTF_8);
            ModuleNode module;
            try {
                CompilationUnit cu = new CompilationUnit(new CompilerConfiguration());
                SourceUnit su = cu.addSource(rel, src);
                cu.compile(Phases.CONVERSION);
                module = su.getAST();
            } catch (Exception e) {
                refused++;
                System.err.println("refused: " + rel + ": " + e.getMessage().lines().findFirst().orElse(""));
                continue;
            }
            new Walker(rel, src).walk(module);
        }
        // vars/<name>.groovy -> its call
        Map<String, List<String>> vars = new TreeMap<>();
        for (Def d : defs) {
            if (d.kind.equals("function") && d.name.equals("call") && isVars(d.path)) {
                String name = Paths.get(d.path).getFileName().toString().replaceFirst("\\.groovy$", "");
                List<String> l = vars.computeIfAbsent(name, k -> new ArrayList<>());
                if (!l.contains(d.path)) l.add(d.path);
            }
        }
        StringBuilder out = new StringBuilder();
        out.append("{\"tool\": \"groovy AST (CONVERSION) + jenkinsfile walk\", \"version\": \"")
            .append(GroovySystemVersion.get()).append("\", \"tier\": \"C\",\n\"definitions\": [");
        String sep = "\n";
        for (Def d : defs) {
            out.append(sep).append("{\"path\": ").append(q(d.path)).append(", \"name\": ").append(q(d.name))
                .append(", \"line\": ").append(d.line).append(", \"end\": ").append(d.end)
                .append(", \"kind\": ").append(q(d.kind)).append("}");
            sep = ",\n";
        }
        out.append("],\n\"references\": [");
        sep = "\n";
        Set<String> seen = new HashSet<>();
        for (Ref r : refs) {
            if (!seen.add(r.path + "\0" + r.from + "\0" + r.to)) continue;
            out.append(sep).append("{\"path\": ").append(q(r.path)).append(", \"from\": ").append(q(r.from))
                .append(", \"to\": ").append(q(r.to)).append("}");
            sep = ",\n";
        }
        out.append("],\n\"cross_references\": [");
        sep = "\n";
        Set<String> xseen = new HashSet<>();
        int xrefs = 0;
        for (Ref r : refs) {
            List<String> targets = vars.get(r.to);
            if (targets == null || targets.size() != 1 || targets.get(0).equals(r.path) && r.from.equals("call")) continue;
            if (!xseen.add(r.path + "\0" + r.from + "\0" + targets.get(0))) continue;
            out.append(sep).append("{\"path\": ").append(q(r.path)).append(", \"from\": ").append(q(r.from))
                .append(", \"to_path\": ").append(q(targets.get(0))).append(", \"to\": \"call\"}");
            sep = ",\n";
            xrefs++;
        }
        out.append("]}\n");
        System.out.print(out);
        System.err.printf("files %d, refused %d, definitions %d, references %d, cross_references %d%n",
            files.size(), refused, defs.size(), seen.size(), xrefs);
    }

    static boolean claimed(String rel) {
        String base = rel.substring(rel.lastIndexOf('/') + 1);
        return base.equals("Jenkinsfile") || base.toLowerCase().endsWith(".jenkinsfile") || isVars(rel);
    }

    static boolean isVars(String rel) {
        String[] parts = rel.split("/");
        return rel.endsWith(".groovy") && parts.length >= 2 && parts[parts.length - 2].equals("vars");
    }

    static String q(String s) {
        StringBuilder b = new StringBuilder("\"");
        for (char c : s.toCharArray()) {
            switch (c) {
                case '"' -> b.append("\\\"");
                case '\\' -> b.append("\\\\");
                case '\n' -> b.append("\\n");
                case '\t' -> b.append("\\t");
                case '\r' -> b.append("\\r");
                default -> { if (c < 0x20) b.append(String.format("\\u%04x", (int) c)); else b.append(c); }
            }
        }
        return b.append('"').toString();
    }

    /** GroovySystemVersion reads the compiler's own version. */
    static class GroovySystemVersion {
        static String get() { return groovy.lang.GroovySystem.getVersion(); }
    }

    static class Walker extends CodeVisitorSupport {
        final String path;
        final List<String> lines;
        final Deque<Def> stack = new ArrayDeque<>();

        Walker(String path, String src) {
            this.path = path;
            this.lines = src.lines().collect(Collectors.toList());
        }

        void walk(ModuleNode m) {
            for (ImportNode i : m.getImports()) libraries(i.getAnnotations(), i.getLineNumber(), i.getLastLineNumber());
            for (ImportNode i : m.getStarImports()) libraries(i.getAnnotations(), i.getLineNumber(), i.getLastLineNumber());
            if (m.getPackage() != null) libraries(m.getPackage().getAnnotations(), m.getPackage().getLineNumber(), m.getPackage().getLastLineNumber());
            // the script body, then the script's own methods
            m.getStatementBlock().visit(this);
            for (MethodNode mn : m.getMethods()) {
                if (mn.getCode() == null) continue;
                Def d = new Def(path, mn.getName(), mn.getLineNumber(), mn.getLastLineNumber(), "function", false);
                defs.add(d);
                stack.push(d);
                mn.getCode().visit(this);
                stack.pop();
            }
        }

        void libraries(List<AnnotationNode> anns, int line, int end) {
            for (AnnotationNode a : anns) {
                String n = a.getClassNode().getName();
                if (!n.equals("Library") && !n.endsWith(".Library")) continue;
                Expression v = a.getMember("value");
                List<Expression> items = v instanceof ListExpression l ? l.getExpressions() : v == null ? List.of() : List.of(v);
                for (Expression e : items) {
                    if (e instanceof ConstantExpression c && c.getValue() instanceof String s) {
                        defs.add(new Def(path, s, a.getLineNumber() > 0 ? a.getLineNumber() : line, a.getLineNumber() > 0 ? a.getLastLineNumber() : end, "import", false));
                    }
                }
            }
        }

        @Override
        public void visitDeclarationExpression(DeclarationExpression e) {
            libraries(e.getAnnotations(), e.getLineNumber(), e.getLastLineNumber());
            super.visitDeclarationExpression(e);
        }

        /** the source text of an expression, one pair of matching quotes off. */
        String text(Expression e) {
            int l1 = e.getLineNumber(), c1 = e.getColumnNumber(), l2 = e.getLastLineNumber(), c2 = e.getLastColumnNumber();
            if (l1 < 1 || l2 < l1) return null;
            StringBuilder b = new StringBuilder();
            for (int l = l1; l <= l2; l++) {
                String s = lines.get(l - 1);
                int from = l == l1 ? c1 - 1 : 0, to = l == l2 ? Math.min(c2 - 1, s.length()) : s.length();
                b.append(s, Math.max(0, from), Math.max(Math.max(0, from), to));
                if (l < l2) b.append('\n');
            }
            String t = b.toString();
            if (t.length() >= 2) {
                char a = t.charAt(0), z = t.charAt(t.length() - 1);
                if ((a == '"' || a == '\'') && a == z) t = t.substring(1, t.length() - 1);
            }
            return t;
        }

        String qualify(String keyword) {
            Def parent = stack.peek();
            return parent != null && parent.container ? parent.name + "." + keyword : keyword;
        }

        @Override
        public void visitMethodCallExpression(MethodCallExpression call) {
            String name = call.isImplicitThis() ? call.getMethodAsString() : null;
            if (name == null) {
                super.visitMethodCallExpression(call);
                return;
            }
            List<Expression> args = call.getArguments() instanceof ArgumentListExpression al ? al.getExpressions() : List.of();
            boolean namedArgs = call.getArguments() instanceof TupleExpression t && !(t instanceof ArgumentListExpression);
            Def def = null;
            int line = call.getLineNumber(), end = call.getLastLineNumber();
            boolean oneClosure = args.size() == 1 && args.get(0) instanceof ClosureExpression;
            if (name.equals("pipeline") && oneClosure) {
                def = new Def(path, "pipeline", line, end, "job", false);
            } else if (name.equals("stage") && args.size() == 2 && args.get(1) instanceof ClosureExpression
                && (args.get(0) instanceof ConstantExpression c && c.getValue() instanceof String || args.get(0) instanceof GStringExpression)) {
                String n = text(args.get(0));
                if (n != null) def = new Def(path, n, line, end, "stage", true);
            } else if (SECTIONS.contains(name) && oneClosure) {
                def = new Def(path, qualify(name), line, end, KIND.get(name), false);
            } else if (name.equals("agent") && args.size() == 1 && (args.get(0) instanceof ClosureExpression
                || args.get(0) instanceof VariableExpression || args.get(0) instanceof ConstantExpression c2 && c2.getValue() instanceof String)) {
                def = new Def(path, qualify(name), line, end, "resource", false);
            }
            if (name.equals("library")) {
                String lib = null;
                if (!namedArgs && !args.isEmpty() && args.get(0) instanceof ConstantExpression c && c.getValue() instanceof String s) lib = s;
                if (namedArgs) {
                    for (Expression e : ((TupleExpression) call.getArguments()).getExpressions()) {
                        if (e instanceof MapExpression me) {
                            for (MapEntryExpression en : me.getMapEntryExpressions()) {
                                if (en.getKeyExpression() instanceof ConstantExpression k && "identifier".equals(k.getValue())
                                    && en.getValueExpression() instanceof ConstantExpression v && v.getValue() instanceof String s) lib = s;
                            }
                        }
                    }
                }
                if (lib != null) defs.add(new Def(path, lib, line, end, "import", false));
            }
            if (!KEYWORDS.contains(name) && !stack.isEmpty()) {
                refs.add(new Ref(path, stack.peek().name, name));
            }
            if (def != null) {
                defs.add(def);
                stack.push(def);
                super.visitMethodCallExpression(call);
                stack.pop();
            } else {
                super.visitMethodCallExpression(call);
            }
        }
    }
}
