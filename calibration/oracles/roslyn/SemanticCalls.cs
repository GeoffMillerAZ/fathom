// SemanticCalls — RoslynDump's `calls` mode: the compiler's own semantic model
// as a tier-A C# call-site oracle (docs/design/language-breadth.md §1.3; the
// C# counterpart of calibration/oracles/clang-calls, LB-F116). Bench-only,
// never a product dependency.
//
// scip-dotnet 0.2.14 writes no occurrence for the name of a generic method
// called with explicit type arguments (`Policy.Handle<T>()`, O271), nor for a
// call through a delegate-typed local, parameter or field (it references the
// variable), so its references alone cannot judge most of the C# calls fathom
// records. This mode loads the subject's projects with MSBuildWorkspace (the
// same project evaluation `dotnet build` does, after a `dotnet restore`),
// compiles every project under every target framework it builds for, and
// asks each compilation's SemanticModel what every call-shaped site written
// in a file of the subject binds to. One record per site: the callee's name
// token (1-based line, 1-based UTF-8 byte column, the name as written) and a
// class:
//
//   resolved   an invocation bound to a method (ordinary, static, extension,
//              generic with or without type arguments, local function,
//              explicit `.Invoke` of a delegate)
//   object     an invocation of a delegate-typed local, parameter, field,
//              property or event (`h(x)`), named by what is written
//   construct  `new X(..)`, `new X<T>(..)`, `new N.X(..)` bound to a
//              constructor, named by the type's own name token
//   unresolved an invocation the compiler binds to nothing in any target
//              (late-bound `dynamic`, an error): no oracle can judge it
//
// A site bound in any target framework is that class (resolved, object and
// construct before unresolved): a call written under `#if NET8_0` is a call
// of the build that defines it. Not written as a call, so not sites: an
// attribute (`[X(..)]`, its constructor runs only when reflection reads it), a
// constructor initializer (`: base(..)`, `: this(..)`), a target-typed
// `new(..)`, array and anonymous-object creation, `nameof(..)`, operators,
// indexers and property accesses. LINQ query syntax binds its clauses to
// methods (`where` to Where) with no invocation written: those are counted
// (`query_clauses`), never sites.
//
// active_lines: per file, the lines some compiled configuration keeps (not
// inside a `#if` branch every target framework discards), as [first, last]
// ranges, the shape clang-calls' preprocessor oracle writes.
//
// file_edges: the (referencing file, declaring file) pairs of every name the
// compiler binds, in any target framework, to a type, method, property,
// field, event or enum member declared in another file of the subject: the
// cross-file edges an index of every configuration would write (scip-dotnet
// 0.2.14 writes no occurrence for a generic type used as a type argument,
// `Func<CircuitBreakerPredicateArguments<T>, ..>`, nor for a generic method
// called with type arguments, O271, nor for code its one framework does not
// compile). A symbol declared in more than one file (a partial type) names
// no one file and draws no edge (counted, `file_edges_multi_declared`); a
// constructed generic is its definition, an extension method called on its
// receiver its declaration, an implicitly declared constructor its type.
//
// Usage: RoslynDump calls <subject-root> <solution-or-project> [Name=Value ...]
// (global MSBuild properties, e.g. LibraryFrameworks=net8.0); run after
// `dotnet restore` of the same solution with the same properties.
using System.Collections.Concurrent;
using System.Text;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.MSBuild;
using Microsoft.CodeAnalysis.Text;

static class SemanticCalls
{
    const string Tool = "roslyn-semantic";

    record Site(string Path, int Line, int Col, string Name, string Class);

    static int Rank(string cls) => cls switch { "resolved" => 0, "object" => 0, "construct" => 0, _ => 1 };

    public static int Run(string[] args)
    {
        if (args.Length < 2)
        {
            Console.Error.WriteLine("usage: RoslynDump calls <subject-root> <solution-or-project> [Name=Value ...]");
            return 2;
        }
        var root = Path.GetFullPath(args[0]);
        var target = Path.GetFullPath(args[1]);
        var props = new Dictionary<string, string>();
        foreach (var kv in args.Skip(2))
        {
            var i = kv.IndexOf('=');
            if (i > 0) props[kv[..i]] = kv[(i + 1)..];
        }
        return RunAsync(root, target, props).GetAwaiter().GetResult();
    }

    static async Task<int> RunAsync(string root, string target, Dictionary<string, string> props)
    {
        using var ws = MSBuildWorkspace.Create(props);
        var failures = 0;
        ws.RegisterWorkspaceFailedHandler(e =>
        {
            if (e.Diagnostic.Kind == WorkspaceDiagnosticKind.Failure) failures++;
            Console.Error.WriteLine($"workspace: {e.Diagnostic.Kind}: {e.Diagnostic.Message}");
        });
        var projects = new List<Project>();
        if (target.EndsWith(".sln", StringComparison.OrdinalIgnoreCase) || target.EndsWith(".slnx", StringComparison.OrdinalIgnoreCase))
            projects.AddRange((await ws.OpenSolutionAsync(target)).Projects);
        else
            projects.AddRange((await ws.OpenProjectAsync(target)).Solution.Projects);
        projects = projects.Where(p => p.Language == LanguageNames.CSharp).OrderBy(p => p.Name, StringComparer.Ordinal).ToList();

        // (path, line, col, name) -> best class over every compiled configuration.
        var sites = new Dictionary<(string, int, int, string), string>();
        // path -> line (1-based) -> kept by some configuration.
        var kept = new Dictionary<string, HashSet<int>>(StringComparer.Ordinal);
        var lineCounts = new Dictionary<string, int>(StringComparer.Ordinal);
        long queryClauses = 0;
        var fileEdges = new HashSet<(string, string)>();
        long multiDeclared = 0;
        var compiled = 0;
        foreach (var project in projects)
        {
            var comp = await project.GetCompilationAsync();
            if (comp == null) continue;
            compiled++;
            var errors = comp.GetDiagnostics().Count(d => d.Severity == DiagnosticSeverity.Error);
            Console.Error.WriteLine($"project {project.Name}: {comp.SyntaxTrees.Count()} trees, {errors} errors");
            foreach (var tree in comp.SyntaxTrees)
            {
                var rel = Relative(root, tree.FilePath);
                if (rel == null) continue;
                var text = tree.GetText();
                lineCounts[rel] = text.Lines.Count;
                if (!kept.TryGetValue(rel, out var keptLines)) kept[rel] = keptLines = new HashSet<int>();
                var disabled = new bool[text.Lines.Count + 1];
                foreach (var t in tree.GetRoot().DescendantTrivia(descendIntoTrivia: true))
                {
                    if (!t.IsKind(SyntaxKind.DisabledTextTrivia)) continue;
                    var span = text.Lines.GetLinePositionSpan(t.Span);
                    // A disabled run ends with its last line's newline: the next
                    // line (the #else or #endif) is not inside it.
                    var last = span.End.Character == 0 && span.End.Line > span.Start.Line ? span.End.Line - 1 : span.End.Line;
                    for (var l = span.Start.Line; l <= last; l++) disabled[l + 1] = true;
                }
                for (var l = 1; l <= text.Lines.Count; l++)
                    if (!disabled[l]) keptLines.Add(l);
                var model = comp.GetSemanticModel(tree);
                foreach (var node in tree.GetRoot().DescendantNodes())
                {
                    if (node is SimpleNameSyntax sn)
                    {
                        var to = DeclaringFile(root, model, sn, ref multiDeclared);
                        if (to != null && to != rel) fileEdges.Add((rel, to));
                    }
                    switch (node)
                    {
                        case InvocationExpressionSyntax inv:
                            {
                                var nameTok = CalleeToken(inv.Expression);
                                if (nameTok is not { } tok) break;
                                if (tok.ValueText == "nameof" && inv.Expression is IdentifierNameSyntax && model.GetSymbolInfo(inv).Symbol == null) break;
                                var cls = InvocationClass(model, inv);
                                Add(sites, text, rel, tok, cls);
                                break;
                            }
                        case ObjectCreationExpressionSyntax oc:
                            {
                                var tok = TypeNameToken(oc.Type);
                                if (tok is not { } t) break;
                                var sym = model.GetSymbolInfo(oc).Symbol as IMethodSymbol;
                                if (sym == null)
                                {
                                    // `new T()` of a type parameter binds no constructor: no call.
                                    if (model.GetTypeInfo(oc).Type is ITypeParameterSymbol) break;
                                    Add(sites, text, rel, t, "unresolved");
                                    break;
                                }
                                Add(sites, text, rel, t, "construct");
                                break;
                            }
                        case QueryClauseSyntax or SelectOrGroupClauseSyntax:
                            {
                                var qi = node is QueryClauseSyntax qc ? model.GetQueryClauseInfo(qc) : default;
                                if (qi.OperationInfo.Symbol != null || qi.CastInfo.Symbol != null || model.GetSymbolInfo(node).Symbol != null) queryClauses++;
                                break;
                            }
                    }
                }
            }
        }

        var outSites = sites.Select(kv => new Site(kv.Key.Item1, kv.Key.Item2, kv.Key.Item3, kv.Key.Item4, kv.Value))
            .OrderBy(s => s.Path, StringComparer.Ordinal).ThenBy(s => s.Line).ThenBy(s => s.Col).ThenBy(s => s.Name, StringComparer.Ordinal).ToList();
        var byClass = outSites.GroupBy(s => s.Class).OrderBy(g => g.Key, StringComparer.Ordinal).Select(g => $"{g.Key}={g.Count()}");
        Console.Error.WriteLine($"projects={projects.Count} compiled={compiled} workspace_failures={failures} files={kept.Count} sites={outSites.Count} ({string.Join(" ", byClass)}) query_clauses={queryClauses} file_edges={fileEdges.Count} multi_declared_refs={multiDeclared}");

        var sb = new StringBuilder();
        sb.Append($"{{\"tool\":\"{Tool}\",\"version\":{Q(Version())},\"tier\":\"A\",\"query_clauses\":{queryClauses},\"call_sites\":[");
        sb.Append(string.Join(",", outSites.Select(s => $"\n{{\"path\":{Q(s.Path)},\"line\":{s.Line},\"col\":{s.Col},\"name\":{Q(s.Name)},\"class\":{Q(s.Class)}}}")));
        sb.Append("],\"active_lines\":{");
        var first = true;
        foreach (var path in kept.Keys.OrderBy(p => p, StringComparer.Ordinal))
        {
            var ranges = Ranges(kept[path], lineCounts[path]);
            if (!first) sb.Append(',');
            first = false;
            sb.Append($"\n{Q(path)}:[{string.Join(",", ranges.Select(r => $"[{r.Item1},{r.Item2}]"))}]");
        }
        sb.Append("},\"file_edges\":[");
        sb.Append(string.Join(",", fileEdges.OrderBy(e => e.Item1, StringComparer.Ordinal).ThenBy(e => e.Item2, StringComparer.Ordinal)
            .Select(e => $"\n[{Q(e.Item1)},{Q(e.Item2)}]")));
        sb.Append($"],\"file_edges_multi_declared\":{multiDeclared}}}");
        Console.Out.WriteLine(sb.ToString());
        return 0;
    }

    // DeclaringFile is the one subject file declaring what name binds to (a
    // type, method, property, field, event or enum member; never a local, a
    // parameter, a namespace or a label), or null; a symbol declared in more
    // than one file counts in multi and is null.
    static string DeclaringFile(string root, SemanticModel model, SimpleNameSyntax name, ref long multi)
    {
        var info = model.GetSymbolInfo(name);
        var sym = info.Symbol ?? (info.CandidateSymbols.Length == 1 ? info.CandidateSymbols[0] : null);
        if (sym == null) return null;
        if (sym is IMethodSymbol m)
        {
            sym = m.ReducedFrom ?? m;
            if (m.MethodKind == MethodKind.DelegateInvoke) return null;
        }
        sym = sym.OriginalDefinition;
        if (sym.IsImplicitlyDeclared && sym.ContainingType != null) sym = sym.ContainingType.OriginalDefinition;
        switch (sym.Kind)
        {
            case SymbolKind.NamedType:
            case SymbolKind.Method:
            case SymbolKind.Property:
            case SymbolKind.Field:
            case SymbolKind.Event:
                break;
            default:
                return null;
        }
        if (sym is IMethodSymbol lm && lm.MethodKind == MethodKind.LocalFunction) return null;
        var files = sym.Locations.Where(l => l.IsInSource).Select(l => Relative(root, l.SourceTree?.FilePath)).Where(f => f != null).Distinct().ToList();
        if (files.Count > 1)
        {
            multi++;
            return null;
        }
        return files.Count == 1 ? files[0] : null;
    }

    static string Version() => "Microsoft.CodeAnalysis.CSharp " + typeof(CSharpCompilation).Assembly.GetName().Version!.ToString(3) + " (MSBuildWorkspace)";

    // The call's own name token: an identifier, a generic method's identifier
    // (never its type arguments), a member access's or a conditional access's
    // name. Anything else (a call of a call's result, `(f)(x)`) writes no name.
    static SyntaxToken? CalleeToken(ExpressionSyntax e) => e switch
    {
        IdentifierNameSyntax id => id.Identifier,
        GenericNameSyntax g => g.Identifier,
        MemberAccessExpressionSyntax ma => ma.Name.Identifier,
        MemberBindingExpressionSyntax mb => mb.Name.Identifier,
        AliasQualifiedNameSyntax aq => aq.Name.Identifier,
        _ => null,
    };

    // A constructed type's own name token: `X`, `X<T>`, `N.X`, `global::N.X`.
    static SyntaxToken? TypeNameToken(TypeSyntax t) => t switch
    {
        IdentifierNameSyntax id => id.Identifier,
        GenericNameSyntax g => g.Identifier,
        QualifiedNameSyntax q => TypeNameToken(q.Right),
        AliasQualifiedNameSyntax aq => TypeNameToken(aq.Name),
        _ => null,
    };

    static string InvocationClass(SemanticModel model, InvocationExpressionSyntax inv)
    {
        var info = model.GetSymbolInfo(inv);
        if (info.Symbol is IMethodSymbol m)
        {
            if (m.MethodKind == MethodKind.DelegateInvoke)
            {
                // `x.Invoke(..)` names the method; `h(..)` names the delegate.
                var named = inv.Expression is MemberAccessExpressionSyntax or MemberBindingExpressionSyntax
                    ? (inv.Expression is MemberAccessExpressionSyntax ma ? ma.Name.Identifier.ValueText : ((MemberBindingExpressionSyntax)inv.Expression).Name.Identifier.ValueText)
                    : null;
                return named == "Invoke" ? "resolved" : "object";
            }
            return "resolved";
        }
        return "unresolved";
    }

    static void Add(Dictionary<(string, int, int, string), string> sites, SourceText text, string rel, SyntaxToken tok, string cls)
    {
        var pos = text.Lines.GetLinePosition(tok.SpanStart);
        var line = text.Lines[pos.Line];
        var prefix = text.ToString(TextSpan.FromBounds(line.Start, tok.SpanStart));
        var col = Encoding.UTF8.GetByteCount(prefix) + 1;
        var name = tok.ValueText;
        var key = (rel, pos.Line + 1, col, name);
        if (!sites.TryGetValue(key, out var prev) || Rank(cls) < Rank(prev)) sites[key] = cls;
    }

    static List<(int, int)> Ranges(HashSet<int> lines, int count)
    {
        var out_ = new List<(int, int)>();
        var l = 1;
        while (l <= count)
        {
            if (!lines.Contains(l)) { l++; continue; }
            var start = l;
            while (l + 1 <= count && lines.Contains(l + 1)) l++;
            out_.Add((start, l));
            l++;
        }
        return out_;
    }

    static string Relative(string root, string path)
    {
        if (string.IsNullOrEmpty(path)) return null;
        var full = Path.GetFullPath(path);
        if (!full.StartsWith(root + Path.DirectorySeparatorChar, StringComparison.Ordinal)) return null;
        var rel = Path.GetRelativePath(root, full).Replace('\\', '/');
        if (rel.Split('/').Any(s => s == "obj" || s == "bin" || s == ".git")) return null;
        return File.Exists(full) ? rel : null;
    }

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
