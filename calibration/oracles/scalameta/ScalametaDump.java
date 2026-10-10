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
// imports (with -imports; LB-F439, K19, the Scala L3 truth): every Import
// anywhere in a file, one {path, line, to_path, module} per importee, joined to
// the subject's files by declaration, never by resolution, as kotlin-psi's
// dump does: a named importee `p.C` (or `p.C.member`, a renamed `{C => D}`,
// Scala 3's `C as D`) takes the file(s) of package `p` declaring a top-level
// `C`, the longest package prefix first; a wildcard (`p._`, `p.*`, `p.given`,
// `p.{given T}`) takes every file of package `p`, or, when `p` is no package
// of the subject, the file(s) declaring its last segment in its parent (an
// object's members); an unimport (`{C => _}`) is no entry. A Scala import
// path is relative to the packages around it: the innermost enclosing package
// that makes the path name something of the subject wins, then the path as
// written. A top-level declaration is a package's class, trait, object, enum,
// type, def, val, var or given, and every member of a package object (whose
// file is also a file of the package `p.o`). A file's package is the full name
// of the innermost package clause holding a declaration (`package cats` then
// `package data` is cats.data). A directive naming no file of the subject is
// one entry with to_path "".
//
// Same-package references (with -imports; O519's shape, LB-F433's harness
// pairing): Scala lets a file reach its packages' members with no import, so
// the dump adds {path, line, to_path, module, same_package: true}: a name the
// file writes with no qualifier (a term or a type, a call's callee, an extends
// clause's type) joins the other files declaring it at top level in the
// innermost package around the reference that declares it (chained clauses
// make each enclosing package visible; `package a.b` makes a.b alone); a name
// written as a selection or an infix operator joins only a top-level Scala 3
// extension method. Scala's precedence where the syntax shows it: a name the
// file declares at its own top level, imports by name (or renames to), names
// in a wildcard import of a subject package, or declares anywhere inside a
// definition (a member, a local, a parameter, a pattern variable) joins
// nothing. Names in an import or a package clause, a named argument's name
// and a definition's own name are no references.
//
// Usage: java -Xmx512m -cp <scalameta jars> ScalametaDump.java [-calls] [-imports] <root> > out.json
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
        boolean withCalls = false, withImports = false;
        String rootArg = null;
        for (String a : args) {
            if (a.equals("-calls")) withCalls = true;
            else if (a.equals("-imports")) withImports = true;
            else rootArg = a;
        }
        Imports imp = new Imports();
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
            if (withImports) imp.file(rel, tree);
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
        StringBuilder importsOut = new StringBuilder();
        if (withImports) imp.write(importsOut);
        System.err.printf("files=%d parsed=%s parse_failures=%d definitions=%d calls=%d%s%n", files.size(), byDialect, failed.size(), ndefs, ncalls,
            withImports ? String.format(" imports=%d resolved_to_subject=%d same_package_pairs=%d", imp.nimports, imp.nresolved, imp.npkg) : "");
        for (String f : failed) System.err.println("  parse failure: " + f);
        System.out.print("{\"tool\":\"scalameta\",\"version\":\"" + VERSION + "\",\"tier\":\"B\",\"definitions\":[");
        System.out.print(defsOut);
        System.out.print("],\"calls\":[");
        System.out.print(callsOut);
        if (withImports) {
            System.out.print("],\"imports\":[");
            System.out.print(importsOut);
        }
        System.out.println("]}");
    }

    // Imports is the -imports pass: each file's package declarations, import directives and simple-name
    // references, joined once every file is read.
    static final class Imports {
        final Map<String, Set<String>> filesOfPackage = new TreeMap<>(); // package -> files declaring in it
        final Map<String, Set<String>> filesOfDecl = new TreeMap<>();    // "pkg\0name" -> files declaring it at top level
        final Map<String, Set<String>> filesOfExt = new TreeMap<>();     // "pkg\0name" -> files with a top-level extension method of that name
        final Map<String, Set<String>> namesOfPackage = new HashMap<>();  // package -> its top-level names
        record Imp(String path, int line, String module, String ref, String name, List<String> visible, boolean star) {}
        record Ref(int line, List<String> visible, boolean sel) {}
        record FileRefs(String path, Map<String, List<Ref>> refs, Set<String> shadow, Set<String> ownTop, List<Imp> wild) {}
        final List<Imp> imps = new ArrayList<>();
        final List<FileRefs> files = new ArrayList<>();
        int nimports, nresolved, npkg;

        void file(String rel, Tree tree) {
            Map<Tree, String> pkgName = new IdentityHashMap<>();
            FileRefs fr = new FileRefs(rel, new TreeMap<>(), new HashSet<>(), new HashSet<>(), new ArrayList<>());
            files.add(fr);
            declare(tree, "", rel, pkgName, fr);
            refs(tree, rel, pkgName, fr);
        }

        void addTo(Map<String, Set<String>> m, String k, String v) { m.computeIfAbsent(k, x -> new TreeSet<>()).add(v); }

        void top(String pkg, String name, String rel, FileRefs fr, boolean ext) {
            if (name == null || name.isEmpty()) return;
            addTo(filesOfDecl, pkg + "\u0000" + name, rel);
            if (ext) addTo(filesOfExt, pkg + "\u0000" + name, rel);
            namesOfPackage.computeIfAbsent(pkg, x -> new HashSet<>()).add(name);
            fr.ownTop().add(pkg + "\u0000" + name);
        }

        // declare walks the package structure: a Pkg's full name, its top-level declarations, a package object's members.
        void declare(Tree t, String pkg, String rel, Map<Tree, String> pkgName, FileRefs fr) {
            for (Object c : list(t.children())) {
                Tree ct = (Tree) c;
                String k = ct.productPrefix();
                switch (k) {
                    case "Pkg" -> {
                        String ref = text(call(ct, "ref"));
                        String full = pkg.isEmpty() ? ref : pkg + "." + ref;
                        pkgName.put(ct, full);
                        declare(body(ct), full, rel, pkgName, fr);
                    }
                    case "Pkg.Body" -> declare(ct, pkg, rel, pkgName, fr);
                    case "Pkg.Object" -> {
                        String o = nameText(ct);
                        top(pkg, o, rel, fr, false);
                        addTo(filesOfPackage, pkg, rel);
                        String inner = pkg.isEmpty() ? o : pkg + "." + o;
                        addTo(filesOfPackage, inner, rel);
                        for (Tree m : members(ct)) declareOne(m, inner, rel, fr);
                    }
                    case "Import" -> {}
                    default -> {
                        if (declareOne(ct, pkg, rel, fr)) addTo(filesOfPackage, pkg, rel);
                    }
                }
            }
        }

        // declareOne records a top-level definition of pkg; false when t is none.
        boolean declareOne(Tree t, String pkg, String rel, FileRefs fr) {
            String k = t.productPrefix();
            switch (k) {
                case "Defn.Class", "Defn.Trait", "Defn.Object", "Defn.Enum", "Defn.Type", "Decl.Type", "Defn.Def", "Decl.Def",
                     "Defn.Macro", "Defn.Given", "Defn.GivenAlias", "Decl.Given" -> { top(pkg, nameText(t), rel, fr, false); return true; }
                case "Defn.Val", "Decl.Val", "Defn.Var", "Decl.Var" -> {
                    for (Object p : list(call(t, "pats"))) {
                        Tree pt = (Tree) p;
                        if (pt.productPrefix().equals("Pat.Var")) top(pkg, nameText(pt), rel, fr, false);
                    }
                    return true;
                }
                case "Defn.ExtensionGroup" -> {
                    for (Tree d : descendants(t, "Defn.Def")) top(pkg, nameText(d), rel, fr, true);
                    return true;
                }
                default -> { return false; }
            }
        }

        // refs reads the file's imports, its shadowing declarations and its simple-name references.
        void refs(Tree t, String rel, Map<Tree, String> pkgName, FileRefs fr) {
            String k = t.productPrefix();
            if (k.equals("Import")) {
                List<String> vis = visible(t, pkgName);
                int line = t.pos().startLine() + 1;
                for (Object io : list(call(t, "importers"))) {
                    Tree importer = (Tree) io;
                    String path = text(call(importer, "ref"));
                    for (Object eo : list(call(importer, "importees"))) {
                        Tree ee = (Tree) eo;
                        String ek = ee.productPrefix();
                        switch (ek) {
                            case "Importee.Name", "Importee.Rename" -> {
                                String n = text(call(ee, "name"));
                                imps.add(new Imp(rel, line, path + "." + n, path, n, vis, false));
                                Object rn = call(ee, "rename");
                                fr.shadow().add(rn != null ? text(rn) : n);
                            }
                            case "Importee.Wildcard", "Importee.GivenAll", "Importee.Given" -> {
                                Imp im = new Imp(rel, line, path + (ek.equals("Importee.Wildcard") ? "._" : ".given"), path, null, vis, true);
                                imps.add(im);
                                fr.wild().add(im);
                            }
                            default -> {}
                        }
                    }
                }
                return;
            }
            if (k.equals("Pkg")) {
                refs(body(t), rel, pkgName, fr);
                return;
            }
            Tree own = nameField(t);
            if (own != null && !k.equals("Term.Select") && !k.equals("Type.Select") && !k.equals("Pkg.Object") && !isPackageLevel(t)) {
                String n = nameText(t);
                if (n != null) fr.shadow().add(n);
            }
            if ((k.equals("Term.Name") || k.equals("Type.Name")) && isReference(t)) {
                String n = t.pos().text();
                if (n.length() >= 2 && n.startsWith("`") && n.endsWith("`")) n = n.substring(1, n.length() - 1);
                fr.refs().computeIfAbsent(n, x -> new ArrayList<>()).add(new Ref(t.pos().startLine() + 1, visible(t, pkgName), isSelector(t)));
            }
            for (Object c : list(t.children())) refs((Tree) c, rel, pkgName, fr);
        }

        // isPackageLevel: t is a definition directly in a package (or the file), not inside another definition.
        static boolean isPackageLevel(Tree t) {
            Tree p = parentOf(t);
            if (p == null) return true;
            String k = p.productPrefix();
            if (k.equals("Pkg.Body") || k.equals("Source") || k.equals("Pkg")) return true;
            return false;
        }

        // A Term.Name or Type.Name is a reference unless it is a definition's own name, a parameter's, a pattern
        // variable's, a named argument's, or written in a package clause or an import.
        static boolean isReference(Tree t) {
            Tree p = parentOf(t);
            if (p == null) return false;
            String pk = p.productPrefix();
            if (nameField(p) == t && !pk.equals("Term.Select") && !pk.equals("Type.Select") && !pk.equals("Type.Project")) return false;
            if (pk.equals("Term.Assign") && call(p, "lhs") == t) {
                Tree g = parentOf(p);
                if (g != null && g.productPrefix().equals("Term.ArgClause")) return false;
            }
            for (Tree a = p; a != null; a = parentOf(a)) {
                String ak = a.productPrefix();
                if (ak.equals("Import") || ak.equals("Export")) return false;
                if (ak.equals("Pkg") && isWithin(t, (Tree) call(a, "ref"))) return false;
            }
            if (pk.equals("Type.Select") && call(p, "name") == t) return false; // a nested type: its qualifier is the reference
            if (pk.equals("Type.Project")) return false;
            return true;
        }

        // isSelector: t is written after a receiver (a.t, an infix operator): only an extension can be the package's.
        static boolean isSelector(Tree t) {
            Tree p = parentOf(t);
            if (p == null) return false;
            String pk = p.productPrefix();
            if (pk.equals("Term.Select") && call(p, "name") == t) return true;
            return pk.equals("Term.ApplyInfix") && call(p, "op") == t;
        }

        static boolean isWithin(Tree t, Tree of) {
            for (Tree a = t; a != null; a = parentOf(a)) if (a == of) return true;
            return false;
        }

        // visible is the full names of the packages around t, innermost first (chained clauses each one).
        static List<String> visible(Tree t, Map<Tree, String> pkgName) {
            List<String> out = new ArrayList<>();
            for (Tree a = parentOf(t); a != null; a = parentOf(a)) {
                String n = pkgName.get(a);
                if (n != null) out.add(n);
            }
            return out;
        }

        // absolute is an import path as Scala reads it: its first segment names a member of the innermost package
        // around the directive that has one (a package or a top-level declaration), else the root's.
        String absolute(String ref, List<String> visible) {
            String first = ref.contains(".") ? ref.substring(0, ref.indexOf('.')) : ref;
            for (String v : visible) {
                String head = v + "." + first;
                if (packages().contains(head) || filesOfDecl.containsKey(v + "\u0000" + first)) return v + "." + ref;
            }
            return ref;
        }

        Set<String> allPackages;

        // packages is every package of the subject and every prefix of one.
        Set<String> packages() {
            if (allPackages == null) {
                allPackages = new HashSet<>();
                for (String p : filesOfPackage.keySet()) {
                    for (int i = p.indexOf('.'); i >= 0; i = p.indexOf('.', i + 1)) allPackages.add(p.substring(0, i));
                    allPackages.add(p);
                }
            }
            return allPackages;
        }

        // The subject files a directive names, or [""] when it names none.
        List<String> join(Imp im) {
            String abs = absolute(im.ref(), im.visible());
            List<String> fs = joinOne(im.star() ? abs : abs + "." + im.name(), im.star());
            return fs.isEmpty() ? List.of("") : fs;
        }

        List<String> joinOne(String fq, boolean star) {
            if (star && filesOfPackage.containsKey(fq)) return new ArrayList<>(filesOfPackage.get(fq));
            String[] seg = fq.split("\\.");
            for (int i = seg.length - 1; i >= 1; i--) {
                String pkg = String.join(".", Arrays.copyOfRange(seg, 0, i));
                Set<String> fs = filesOfDecl.get(pkg + "\u0000" + seg[i]);
                if (fs != null) return new ArrayList<>(fs);
            }
            if (seg.length == 1) {
                Set<String> fs = filesOfDecl.get("\u0000" + seg[0]);
                if (fs != null) return new ArrayList<>(fs);
            }
            return List.of();
        }

        void write(StringBuilder out) {
            for (Imp im : imps) {
                for (String to : join(im)) {
                    if (to.equals(im.path())) continue;
                    append(out, String.format("{\"path\":%s,\"line\":%d,\"to_path\":%s,\"module\":%s}", q(im.path()), im.line(), q(to), q(im.module())));
                    nimports++;
                    if (!to.isEmpty()) nresolved++;
                }
            }
            for (FileRefs fr : files) {
                // a wildcard import of a subject package brings its names in above another file's package members
                Set<String> shadow = new HashSet<>(fr.shadow());
                for (Imp w : fr.wild()) {
                    String c = absolute(w.ref(), w.visible());
                    if (filesOfPackage.containsKey(c)) shadow.addAll(namesOfPackage.getOrDefault(c, Set.of()));
                }
                Map<String, Integer> byTo = new TreeMap<>();
                Map<String, String> modOf = new HashMap<>();
                for (Map.Entry<String, List<Ref>> e : fr.refs().entrySet()) {
                    String n = e.getKey();
                    if (shadow.contains(n)) continue;
                    for (Ref r : e.getValue()) {
                        for (String v : r.visible()) {
                            Set<String> fs = (r.sel() ? filesOfExt : filesOfDecl).get(v + "\u0000" + n);
                            if (fs == null) continue;
                            if (!fs.contains(fr.path()) && !fr.ownTop().contains(v + "\u0000" + n)) {
                                for (String to : fs) {
                                    if (!byTo.containsKey(to) || r.line() < byTo.get(to)) { byTo.put(to, r.line()); modOf.put(to, v + "." + n); }
                                }
                            }
                            break; // the innermost package declaring the name binds it
                        }
                    }
                }
                for (Map.Entry<String, Integer> e : byTo.entrySet()) {
                    append(out, String.format("{\"path\":%s,\"line\":%d,\"to_path\":%s,\"module\":%s,\"same_package\":true}", q(fr.path()), e.getValue(), q(e.getKey()), q(modOf.get(e.getKey()))));
                    npkg++;
                }
            }
        }
    }

    // body is a Pkg's statement holder (Pkg.Body in scalameta 4.9 and later).
    static Tree body(Tree pkg) {
        Object b = call(pkg, "body");
        return b instanceof Tree bt ? bt : pkg;
    }

    // members is a package object's template statements.
    static List<Tree> members(Tree obj) {
        List<Tree> out = new ArrayList<>();
        Object tmpl = call(obj, "templ");
        if (!(tmpl instanceof Tree tt)) return out;
        Object b = call(tt, "body");
        Object stats = b instanceof Tree bt ? call(bt, "stats") : call(tt, "stats");
        for (Object s : list(stats)) out.add((Tree) s);
        return out;
    }

    static List<Tree> descendants(Tree t, String prefix) {
        List<Tree> out = new ArrayList<>();
        for (Object c : list(t.children())) {
            Tree ct = (Tree) c;
            if (ct.productPrefix().equals(prefix)) out.add(ct);
            out.addAll(descendants(ct, prefix));
        }
        return out;
    }

    // nameField is t's `name` field when it is a Tree (a definition's, a parameter's, a selection's).
    static Tree nameField(Tree t) {
        Object n = call(t, "name");
        return n instanceof Tree nt ? nt : null;
    }

    static String nameText(Tree t) {
        Tree n = nameField(t);
        if (n == null) return null;
        String s = n.pos().text();
        if (s.length() >= 2 && s.startsWith("`") && s.endsWith("`")) s = s.substring(1, s.length() - 1);
        return s;
    }

    // text is a tree's source with whitespace and backticks dropped (a path `cats.data`).
    static String text(Object t) {
        if (!(t instanceof Tree tt)) return "";
        return tt.pos().text().replaceAll("\\s+", "").replace("`", "");
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
