// ScalametaDump — a tier-B Scala L1/L2 oracle for fathom's language-breadth
// harness (docs/design/language-breadth.md §1.3, §10; ADR-048 amendment D2).
// Bench-only instrument: a JVM program run as a separate process, never a
// product dependency. It parses every .scala and .sc file under <root> with
// scalameta (BSD-3-Clause; the parser scalafmt and Metals use, syntax only, no
// resolution, no tree-sitter code) and prints the neutral tool-oracle JSON
// `indexbench lang-oracle -tool-oracle` reads:
//
//   {"tool","version","tier","definitions":[{path,name,line,end,kind}],
//    "calls":[{path,from,to}]}
//
// Dialect: a file under a `scala-3` source directory is parsed as Scala 3
// first; any other file as Scala 2.13 with -Xsource:3 first; each falls back
// to the other, then to Scala 2.12. A .sc script allows top-level terms. A
// file no dialect parses contributes nothing and is named on stderr.
//
// kinds: class (Defn.Class: a class, a case class), trait, object (a
// Defn.Object or a package object), enum, enumCase (a Defn.EnumCase, and each
// name of a Defn.RepeatedEnumCase), def (Defn.Def, Decl.Def, Defn.Macro, every
// def anywhere: a member, an extension method, a top-level def, a local def),
// constructor (Ctor.Secondary, named `this`), given (a named Defn.Given,
// Defn.GivenAlias or Decl.Given; an anonymous given names nothing), type
// (Defn.Type, Decl.Type), and val / var: a Defn.Val, Decl.Val, Defn.Var or
// Decl.Var whose parent is a template, an enum body, a package or the file
// (never a local val in a block or a body), one per Pat.Var (`val a, b = 1`
// is two; a pattern, `val (a, b) = ...`, names none), and a class parameter
// that is a field: written `val` or `var`, or any parameter of a case class or
// of an enum case with parameters. line is the row of the name, end the last
// row of the whole definition (a class parameter's own). Names are bare (the
// harness's BareName: the last segment after `.`, `:`, `#`, `/`, `\`, space
// or `)`).
//
// calls (with -calls): every Term.Apply whose function is a name (`f(x)`), a
// selection (`a.b.f(x)`, `super.f(x)`) or either with type arguments
// (`f[T](x)`), the callee its last name, paired with the innermost enclosing
// Defn.Def, Defn.Macro or Ctor.Secondary (a call in a val initialiser, a
// class body or a given alias belongs to no callable and is no pair). A call
// whose function is itself a call (`f(a)(b)`) has no name; the inner one is
// counted. An infix (`xs map f`), `new X(..)`, a pattern and a parameterless
// selection are no Term.Apply.
//
// Usage: java -Xmx512m -cp <scalameta jars> ScalametaDump.java [-calls] <root> > out.json
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.util.*;
import java.util.stream.*;
import scala.meta.Dialect;
import scala.meta.Tree;
import scala.meta.inputs.Input;
import scala.meta.parsers.Parse$;
import scala.meta.parsers.Parsed;

public class ScalametaDump {
    static final String VERSION = "4.17.4";

    record Def(String name, int line, int end, String kind, Tree node) {}

    public static void main(String[] args) throws Exception {
        boolean withCalls = false;
        String rootArg = null;
        for (String a : args) {
            if (a.equals("-calls")) withCalls = true;
            else rootArg = a;
        }
        Path root = Paths.get(rootArg).toAbsolutePath().normalize();
        Dialect s3 = scala.meta.dialects.package$.MODULE$.Scala3();
        Dialect s213 = scala.meta.dialects.package$.MODULE$.Scala213Source3();
        Dialect s212 = scala.meta.dialects.package$.MODULE$.Scala212();

        List<Path> files;
        try (Stream<Path> s = Files.walk(root)) {
            files = s.filter(p -> {
                String n = p.toString();
                return (n.endsWith(".scala") || n.endsWith(".sc")) && Files.isRegularFile(p) && !n.contains("/.git/");
            }).sorted().collect(Collectors.toList());
        }
        StringBuilder defsOut = new StringBuilder(), callsOut = new StringBuilder();
        int ndefs = 0, ncalls = 0;
        Map<String, Integer> byDialect = new TreeMap<>();
        List<String> failed = new ArrayList<>();
        for (Path f : files) {
            String rel = root.relativize(f).toString().replace('\\', '/');
            String text = new String(Files.readAllBytes(f), StandardCharsets.UTF_8);
            boolean script = rel.endsWith(".sc");
            List<Dialect> order = rel.contains("/scala-3") ? List.of(s3, s213, s212) : List.of(s213, s3, s212);
            List<String> names = rel.contains("/scala-3") ? List.of("Scala3", "Scala213Source3", "Scala212") : List.of("Scala213Source3", "Scala3", "Scala212");
            Tree tree = null;
            String why = "";
            for (int i = 0; i < order.size() && tree == null; i++) {
                Dialect d = order.get(i);
                if (script) d = d.withAllowToplevelTerms(true).withAllowToplevelStatements(true);
                Parsed<?> parsed = Parse$.MODULE$.parseSource().apply(Input.VirtualFile$.MODULE$.apply(rel, text), d);
                if (parsed instanceof Parsed.Success<?> ok) {
                    tree = (Tree) ok.tree();
                    byDialect.merge(names.get(i), 1, Integer::sum);
                } else if (i == 0) {
                    why = parsed.toString().split("\n")[0];
                }
            }
            if (tree == null) {
                failed.add(rel + " (" + names.get(0) + ": " + why + ")");
                continue;
            }
            List<Def> defs = new ArrayList<>();
            Map<Tree, Def> callableOf = new IdentityHashMap<>();
            List<Tree> applies = new ArrayList<>();
            collect(tree, defs, callableOf, applies);
            for (Def d : defs) {
                append(defsOut, String.format("{\"path\":%s,\"name\":%s,\"line\":%d,\"end\":%d,\"kind\":%s}", q(rel), q(d.name()), d.line(), d.end(), q(d.kind())));
                ndefs++;
            }
            if (!withCalls) continue;
            Set<String> seen = new TreeSet<>();
            for (Tree ap : applies) {
                String to = calleeOf((Tree) call(ap, "fun"));
                if (to != null && to.length() >= 2 && to.startsWith("`") && to.endsWith("`")) to = to.substring(1, to.length() - 1);
                if (to == null || to.isEmpty()) continue;
                Def enc = null;
                for (Tree p = parentOf(ap); p != null; p = parentOf(p)) {
                    Def d = callableOf.get(p);
                    if (d != null) { enc = d; break; }
                }
                if (enc == null) continue;
                seen.add(enc.name() + "\u0000" + bare(to));
            }
            for (String s : seen) {
                String[] parts = s.split("\u0000", -1);
                append(callsOut, String.format("{\"path\":%s,\"from\":%s,\"to\":%s}", q(rel), q(parts[0]), q(parts[1])));
                ncalls++;
            }
        }
        System.err.printf("files=%d parsed=%s parse_failures=%d definitions=%d calls=%d%n", files.size(), byDialect, failed.size(), ndefs, ncalls);
        for (String f : failed) System.err.println("  parse failure: " + f);
        System.out.print("{\"tool\":\"scalameta\",\"version\":\"" + VERSION + "\",\"tier\":\"B\",\"definitions\":[");
        System.out.print(defsOut);
        System.out.print("],\"calls\":[");
        System.out.print(callsOut);
        System.out.println("]}");
    }

    static void collect(Tree t, List<Def> defs, Map<Tree, Def> callableOf, List<Tree> applies) {
        String k = t.productPrefix();
        switch (k) {
            case "Defn.Class" -> add(defs, t, nameOf(t), "class");
            case "Defn.Trait" -> add(defs, t, nameOf(t), "trait");
            case "Defn.Object", "Pkg.Object" -> add(defs, t, nameOf(t), "object");
            case "Defn.Enum" -> add(defs, t, nameOf(t), "enum");
            case "Defn.EnumCase" -> add(defs, t, nameOf(t), "enumCase");
            case "Defn.RepeatedEnumCase" -> {
                for (Object n : list(call(t, "cases"))) add(defs, t, (Tree) n, "enumCase");
            }
            case "Defn.Def", "Decl.Def", "Defn.Macro" -> {
                Def d = add(defs, t, nameOf(t), "def");
                if (d != null && !k.equals("Decl.Def")) callableOf.put(t, d);
            }
            case "Ctor.Secondary" -> {
                Def d = add(defs, t, nameOf(t), "constructor", "this");
                if (d != null) callableOf.put(t, d);
            }
            case "Defn.Given", "Defn.GivenAlias", "Decl.Given" -> add(defs, t, nameOf(t), "given");
            case "Defn.Type", "Decl.Type" -> add(defs, t, nameOf(t), "type");
            case "Defn.Val", "Decl.Val", "Defn.Var", "Decl.Var" -> {
                if (memberScope(parentOf(t))) {
                    String kind = k.endsWith(".Val") ? "val" : "var";
                    for (Object p : list(call(t, "pats"))) {
                        Tree pt = (Tree) p;
                        if (pt.productPrefix().equals("Pat.Var")) add(defs, t, nameOf(pt), kind);
                    }
                }
            }
            case "Term.Param" -> {
                String kind = fieldKind(t);
                if (kind != null) add(defs, t, nameOf(t), kind);
            }
            case "Term.Apply" -> applies.add(t);
            default -> {}
        }
        for (Object c : list(t.children())) collect((Tree) c, defs, callableOf, applies);
    }

    // A val or var counts where it is a member: a template's or an enum's body, a package's, the file's.
    static boolean memberScope(Tree p) {
        if (p == null) return false;
        String k = p.productPrefix();
        if (k.equals("Template.Body")) return true;
        return k.equals("Template") || k.equals("Source") || k.equals("Pkg") || k.equals("Pkg.Body") || k.equals("MultiSource");
    }

    // A class parameter's field kind: "val"/"var" when written so, "val" for any parameter of a case class
    // or of an enum case with parameters, null for any other parameter.
    static String fieldKind(Tree param) {
        Tree clause = parentOf(param);
        if (clause == null) return null;
        Tree ctor = clause.productPrefix().equals("Term.ParamClause") ? parentOf(clause) : clause;
        if (ctor == null || !ctor.productPrefix().equals("Ctor.Primary")) return null;
        Tree owner = parentOf(ctor);
        for (Object m : list(call(param, "mods"))) {
            String mk = ((Tree) m).productPrefix();
            if (mk.equals("Mod.ValParam")) return "val";
            if (mk.equals("Mod.VarParam")) return "var";
        }
        if (owner == null) return null;
        String ok = owner.productPrefix();
        if (ok.equals("Defn.EnumCase")) return "val";
        if (ok.equals("Defn.Class")) {
            for (Object m : list(call(owner, "mods"))) if (((Tree) m).productPrefix().equals("Mod.Case")) return "val";
        }
        return null;
    }

    static String calleeOf(Tree fun) {
        if (fun == null) return null;
        switch (fun.productPrefix()) {
            case "Term.Name": return fun.pos().text();
            case "Term.Select": return ((Tree) call(fun, "name")).pos().text();
            case "Term.ApplyType": {
                Tree inner = (Tree) call(fun, "fun");
                String k = inner.productPrefix();
                if (k.equals("Term.Name") || k.equals("Term.Select")) return calleeOf(inner);
                return null;
            }
            default: return null;
        }
    }

    static Tree nameOf(Tree t) {
        Object n = call(t, "name");
        return n instanceof Tree nt ? nt : null;
    }

    // A name as the source writes it (scalameta's value unescapes a backticked `\n` to a newline),
    // without the backticks.
    static Def add(List<Def> defs, Tree node, Tree nameNode, String kind) {
        if (nameNode == null) return null;
        String text = nameNode.pos().text();
        if (text.length() >= 2 && text.startsWith("`") && text.endsWith("`")) text = text.substring(1, text.length() - 1);
        return add(defs, node, nameNode, kind, text);
    }

    // An operator name the bare rule empties (`::`, `+:`, `/`) is kept as "", the form the harness's
    // BareName gives the dialect's same name.
    static Def add(List<Def> defs, Tree node, Tree nameNode, String kind, String name) {
        if (nameNode == null || name == null || name.isEmpty()) return null;
        String b = bare(name);
        int line = nameNode.pos().startLine() + 1;
        int end = node.pos().endLine() + 1;
        Def d = new Def(b, line, end, kind, node);
        defs.add(d);
        return d;
    }

    static Tree parentOf(Tree t) {
        scala.Option<Tree> p = t.parent();
        return p.isDefined() ? p.get() : null;
    }

    static Iterable<Object> list(Object seq) {
        List<Object> out = new ArrayList<>();
        if (seq instanceof scala.collection.Iterable<?> it) {
            scala.collection.Iterator<?> i = it.iterator();
            while (i.hasNext()) out.add(i.next());
        }
        return out;
    }

    static final Map<String, Method> methods = new HashMap<>();

    static Object call(Object o, String name) {
        if (o == null) return null;
        String key = o.getClass().getName() + "#" + name;
        try {
            Method m = methods.get(key);
            if (m == null) {
                m = o.getClass().getMethod(name);
                m.setAccessible(true);
                methods.put(key, m);
            }
            return m.invoke(o);
        } catch (NoSuchMethodException e) {
            return null;
        } catch (ReflectiveOperationException e) {
            throw new RuntimeException(key, e);
        }
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
