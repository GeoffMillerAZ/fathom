// RoslynDump — a tier-B C# L1/L2 oracle for fathom's language-breadth
// harness (docs/design/language-breadth.md §1.3, §10; ADR-048 amendment D2).
// Bench-only instrument, run as a separate process; never a product
// dependency. Syntax only: every .cs file under <root> is parsed with
// Roslyn (Microsoft.CodeAnalysis.CSharp), nothing is compiled or resolved.
//
// Preprocessor: tree-sitter parses every #if branch, Roslyn only the active
// ones, so each file is parsed twice — once with no symbols defined, once
// with every symbol any #if/#elif in the file names defined — and, when a
// directive negates a symbol (`#if !B`, `#if A && !B`), a third time with
// every symbol but the negated ones (wave lf1: Newtonsoft.Json's
// `#if HAVE_DYNAMIC` / `#if !HAVE_REFLECTION_BINDER` nesting reaches its
// fields under neither of the first two). The definitions are unioned by
// (kind, name, line). A definition only reachable under another mixed
// combination can still be missed; the count of files carrying directives is
// printed.
//
// Output: the neutral tool-oracle JSON `indexbench lang-oracle -tool-oracle`
// reads: {"tool","version","tier","definitions":[{path,name,line,end,kind}],
// "calls":[{path,from,to}]}. Names are bare (a namespace's last segment).
// line is the name's line (the identifier; a namespace's name, an operator's
// token, a conversion operator's type, an indexer's `this`, a field's first
// declarator), end the declaration's last line (O457).
// Kinds: namespace class record interface struct recordStruct enum method
// constructor destructor operator conversionOperator property indexer
// event field delegate enumMember. calls: every invocation's callee bare
// name paired with its innermost enclosing type/method/constructor/
// conversion-operator/property definition (skipped when none); local
// functions and lambdas are not definitions and do not enclose.
//
// Usage: dotnet run -c Release -- <root> > out.json
// The `calls` mode (SemanticCalls.cs) is a separate, tier-A instrument: the
// compiler's semantic model over the subject's restored projects.
using System.Text;
using System.Text.RegularExpressions;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;

static class RoslynDump
{
    const string Version = "Microsoft.CodeAnalysis.CSharp 5.9.0";
    static readonly Regex DirectiveSym = new(@"^\s*#\s*(?:if|elif)\b(.*)$", RegexOptions.Multiline);
    static readonly Regex Ident = new(@"[A-Za-z_][A-Za-z0-9_]*");
    static readonly Regex Negated = new(@"!\s*([A-Za-z_][A-Za-z0-9_]*)");

    record Def(string Kind, string Name, int Line, int End, SyntaxNode Node);

    static int Main(string[] args)
    {
        if (args.Length > 0 && args[0] == "calls") return SemanticCalls.Run(args[1..]);
        var root = Path.GetFullPath(args[0]);
        var files = Directory.EnumerateFiles(root, "*.cs", SearchOption.AllDirectories)
            .Select(f => Path.GetRelativePath(root, f).Replace('\\', '/'))
            .Where(r => !r.Split('/').Any(s => s == "obj" || s == "bin" || s == ".git"))
            .OrderBy(r => r, StringComparer.Ordinal).ToList();
        var defsOut = new List<string>();
        var callsOut = new List<string>();
        int withDirectives = 0;
        foreach (var rel in files)
        {
            var text = File.ReadAllText(Path.Combine(root, rel));
            var syms = new SortedSet<string>(StringComparer.Ordinal);
            foreach (Match m in DirectiveSym.Matches(text))
                foreach (Match id in Ident.Matches(m.Groups[1].Value))
                    if (id.Value != "true" && id.Value != "false") syms.Add(id.Value);
            var passes = new List<CSharpParseOptions> { new CSharpParseOptions(LanguageVersion.Preview) };
            if (syms.Count > 0)
            {
                withDirectives++;
                passes.Add(new CSharpParseOptions(LanguageVersion.Preview, preprocessorSymbols: syms));
                var negated = new HashSet<string>(StringComparer.Ordinal);
                foreach (Match m in DirectiveSym.Matches(text))
                    foreach (Match id in Negated.Matches(m.Groups[1].Value))
                        negated.Add(id.Groups[1].Value);
                var mixed = syms.Where(x => !negated.Contains(x)).ToList();
                if (negated.Count > 0 && mixed.Count > 0)
                    passes.Add(new CSharpParseOptions(LanguageVersion.Preview, preprocessorSymbols: mixed));
            }
            var seenDefs = new HashSet<(string, string, int)>();
            var seenCalls = new SortedSet<string>(StringComparer.Ordinal);
            foreach (var opts in passes)
            {
                var tree = CSharpSyntaxTree.ParseText(text, opts);
                var rootNode = tree.GetRoot();
                var byNode = new Dictionary<SyntaxNode, Def>();
                foreach (var n in rootNode.DescendantNodes())
                {
                    foreach (var d in DefsOf(n, tree))
                    {
                        if (!byNode.ContainsKey(d.Node)) byNode[d.Node] = d;
                        if (seenDefs.Add((d.Kind, d.Name, d.Line)))
                            defsOut.Add($"{{\"path\":{Q(rel)},\"name\":{Q(d.Name)},\"line\":{d.Line},\"end\":{d.End},\"kind\":{Q(d.Kind)}}}");
                    }
                }
                foreach (var inv in rootNode.DescendantNodes().OfType<InvocationExpressionSyntax>())
                {
                    var callee = CalleeName(inv.Expression);
                    if (callee == null) continue;
                    Def enc = null;
                    for (var p = inv.Parent; p != null && enc == null; p = p.Parent)
                        if (byNode.TryGetValue(p, out var d) && Encloses(d.Kind)) enc = d;
                    if (enc == null) continue;
                    seenCalls.Add(enc.Name + "\0" + callee);
                }
            }
            foreach (var c in seenCalls)
            {
                var parts = c.Split('\0');
                callsOut.Add($"{{\"path\":{Q(rel)},\"from\":{Q(parts[0])},\"to\":{Q(parts[1])}}}");
            }
        }
        Console.Error.WriteLine($"files={files.Count} with_directives={withDirectives} definitions={defsOut.Count} calls={callsOut.Count}");
        var sb = new StringBuilder();
        sb.Append($"{{\"tool\":\"roslyn-syntax\",\"version\":{Q(Version)},\"tier\":\"B\",\"definitions\":[");
        sb.Append(string.Join(",", defsOut.Select(x => "\n" + x)));
        sb.Append("],\"calls\":[");
        sb.Append(string.Join(",", callsOut.Select(x => "\n" + x)));
        sb.Append("]}");
        Console.Out.WriteLine(sb.ToString());
        return 0;
    }

    static bool Encloses(string kind) => kind is "class" or "record" or "interface" or "struct" or "recordStruct"
        or "method" or "constructor" or "conversionOperator" or "property";

    static IEnumerable<Def> DefsOf(SyntaxNode n, SyntaxTree tree)
    {
        // A definition is dated by its name (O457): the node's own first line
        // is its first attribute list's, and SCIP, the other dumps and the
        // dialect date a declaration by its name. `end` is the node's.
        Def Mk(string kind, string name, SyntaxNodeOrToken at) => new(kind, name,
            at.GetLocation().GetLineSpan().StartLinePosition.Line + 1,
            n.GetLocation().GetLineSpan().EndLinePosition.Line + 1, n);
        switch (n)
        {
            case BaseNamespaceDeclarationSyntax ns: yield return Mk("namespace", Last(ns.Name.ToString()), ns.Name); break;
            case RecordDeclarationSyntax r:
                yield return Mk(r.ClassOrStructKeyword.IsKind(SyntaxKind.StructKeyword) ? "recordStruct" : "record", r.Identifier.Text, r.Identifier); break;
            case ClassDeclarationSyntax c: yield return Mk("class", c.Identifier.Text, c.Identifier); break;
            case InterfaceDeclarationSyntax i: yield return Mk("interface", i.Identifier.Text, i.Identifier); break;
            case StructDeclarationSyntax s: yield return Mk("struct", s.Identifier.Text, s.Identifier); break;
            case EnumDeclarationSyntax e: yield return Mk("enum", e.Identifier.Text, e.Identifier); break;
            case EnumMemberDeclarationSyntax em: yield return Mk("enumMember", em.Identifier.Text, em.Identifier); break;
            case DelegateDeclarationSyntax dd: yield return Mk("delegate", dd.Identifier.Text, dd.Identifier); break;
            case MethodDeclarationSyntax m: yield return Mk("method", m.Identifier.Text, m.Identifier); break;
            case ConstructorDeclarationSyntax ct: yield return Mk("constructor", ct.Identifier.Text, ct.Identifier); break;
            case DestructorDeclarationSyntax dt: yield return Mk("destructor", dt.Identifier.Text, dt.Identifier); break;
            case OperatorDeclarationSyntax op: yield return Mk("operator", op.OperatorToken.Text, op.OperatorToken); break;
            case ConversionOperatorDeclarationSyntax co: yield return Mk("conversionOperator", co.Type.ToString(), co.Type); break;
            case PropertyDeclarationSyntax p: yield return Mk("property", p.Identifier.Text, p.Identifier); break;
            case IndexerDeclarationSyntax ix: yield return Mk("indexer", "this", ix.ThisKeyword); break;
            case EventDeclarationSyntax ev: yield return Mk("event", ev.Identifier.Text, ev.Identifier); break;
            case BaseFieldDeclarationSyntax f:
                // every declarator dated by the field's first (the declaration's name line)
                var first = f.Declaration.Variables.FirstOrDefault();
                foreach (var v in f.Declaration.Variables)
                    yield return Mk(f is EventFieldDeclarationSyntax ? "event" : "field", v.Identifier.Text, first.Identifier);
                break;
        }
    }

    static string Last(string dotted) => dotted[(dotted.LastIndexOf('.') + 1)..];

    static string CalleeName(ExpressionSyntax e) => e switch
    {
        IdentifierNameSyntax id => id.Identifier.Text,
        GenericNameSyntax g => g.Identifier.Text,
        MemberAccessExpressionSyntax ma => ma.Name.Identifier.Text,
        MemberBindingExpressionSyntax mb => mb.Name.Identifier.Text,
        _ => null,
    };

    static string Q(string s)
    {
        var b = new StringBuilder("\"");
        foreach (var c in s)
        {
            if (c == '"') b.Append("\\\"");
            else if (c == '\\') b.Append("\\\\");
            else if (c < 0x20) b.Append($"\\u{(int)c:x4}");
            else b.Append(c);
        }
        return b.Append('"').ToString();
    }
}
