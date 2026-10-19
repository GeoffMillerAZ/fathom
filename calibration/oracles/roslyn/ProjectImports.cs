// Bench-only raw native using facts. This is NOT the neutral import oracle:
// declaration membership must never become actual-use dependency truth.
using System.Security.Cryptography;
using System.Text;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.FindSymbols;
using Microsoft.CodeAnalysis.Text;

sealed class ProjectImports
{
    readonly string root,prefix;
    readonly Solution solution;
    readonly List<string> failures;
    readonly SortedDictionary<string,string> inputs=new(StringComparer.Ordinal);
    Task<List<(Project Project,Compilation Compilation)>> sourceProjects;
    sealed record Origin(Compilation Compilation,Project Project,string Method,object[] Candidates,bool Unknown);
    public readonly List<object> Projects=new(),Files=new(),Usings=new();
    public object[] Inputs=>inputs.Select(pair=>(object)new {path=pair.Key,sha256=pair.Value}).ToArray();

    public ProjectImports(string root,string selector,Solution solution,List<string> failures)
    {this.root=root;prefix=selector+"/";this.solution=solution;this.failures=failures;}

    public void Track(string path)
    {
        if(path==null||!File.Exists(path))return;
        path=Path.GetFullPath(path);
        string hash=Hash(File.ReadAllBytes(path));
        if(inputs.TryGetValue(path,out var old)&&old!=hash)
            throw new IOException("Native input changed during export: "+path);
        inputs[path]=hash;
    }
    public void CheckClosure(List<string> failures)
    {
        foreach(var item in inputs)
            if(!File.Exists(item.Key)||Hash(File.ReadAllBytes(item.Key))!=item.Value)
                failures.Add("Native input changed during import export: "+item.Key);
    }
    public void AddProject(Project project,Compilation compilation)
    {
        Track(project.FilePath);
        if(project.FilePath==null||!File.Exists(project.FilePath))failures.Add("Missing loaded project file: "+project.Name);
        var parse=project.ParseOptions as CSharpParseOptions;
        var references=new List<object>();
        foreach(var reference in compilation.References)
        {
            var assembly=compilation.GetAssemblyOrModuleSymbol(reference);
            string path=(reference as PortableExecutableReference)?.FilePath;Track(path);
            if(path!=null&&!File.Exists(path))failures.Add("Missing loaded PE reference: "+path);
            references.Add(new {display=reference.Display,path,
                native_assembly=assembly?.ContainingAssembly?.Identity.ToString()??(assembly as IAssemblySymbol)?.Identity.ToString(),
                reference_kind=reference is CompilationReference?"compilation":"portable-executable",
                aliases=reference.Properties.Aliases.IsDefaultOrEmpty?Array.Empty<string>():reference.Properties.Aliases.ToArray(),embed_interop_types=reference.Properties.EmbedInteropTypes});
        }
        var trees=new List<object>();
        foreach(var tree in compilation.SyntaxTrees.OrderBy(tree=>tree.FilePath,StringComparer.Ordinal))
        {
            Track(tree.FilePath);
            bool regularDocument=project.Documents.Any(document=>document.FilePath==tree.FilePath);
            if(Selected(tree.FilePath)&&!regularDocument)
                failures.Add("Selected native tree has no regular project.Document; using coverage incomplete: "+tree.FilePath);
            if(File.Exists(tree.FilePath)&&File.ReadAllText(tree.FilePath)!=tree.GetText().ToString())
                failures.Add("Physical source differs from the native compilation snapshot: "+tree.FilePath);
            trees.Add(new {path=Relative(tree.FilePath),native_text_sha256=Hash(Encoding.UTF8.GetBytes(tree.GetText().ToString())),
                physical_file=File.Exists(tree.FilePath),selected=Selected(tree.FilePath),regular_project_document=regularDocument,
                using_emission=Selected(tree.FilePath)?regularDocument?"selected regular document":"incomplete: selected non-document tree":"outside using emission selection"});
        }
        Projects.Add(new {name=project.Name,path=project.FilePath,assembly=compilation.Assembly.Identity.ToString(),
            language_version=parse?.LanguageVersion.ToString(),preprocessor_symbols=parse?.PreprocessorSymbolNames.OrderBy(x=>x,StringComparer.Ordinal).ToArray(),
            output_kind=compilation.Options.OutputKind.ToString(),platform=compilation.Options.Platform.ToString(),
            project_references=project.ProjectReferences.Select(reference=>new {
                path=solution.GetProject(reference.ProjectId)?.FilePath,aliases=reference.Aliases.IsDefaultOrEmpty?Array.Empty<string>():reference.Aliases.ToArray(),embed_interop_types=reference.EmbedInteropTypes}).ToArray(),
            native_references=references,native_trees=trees});
    }

    public async Task AddFileAsync(string path,Project project,SyntaxNode syntax,SemanticModel model)
    {
        Track(syntax.SyntaxTree.FilePath);
        var options=syntax.SyntaxTree.Options as CSharpParseOptions;
        // Disabled regions are native trivia with exact positions/hashes. Do not
        // reparse them into fake active bindings or silently union configurations.
        var inactive=syntax.DescendantTrivia(descendIntoTrivia:true)
            .Where(trivia=>trivia.IsKind(SyntaxKind.DisabledTextTrivia))
            .Select(trivia=>new {span=Span(syntax.SyntaxTree,trivia.Span),
                text_sha256=Hash(Encoding.UTF8.GetBytes(trivia.ToFullString())),
                status="inactive in native parse configuration; no binding attempted"}).ToArray();
        Files.Add(new {project=project.FilePath,path,
            source_sha256=File.Exists(syntax.SyntaxTree.FilePath)?inputs[Path.GetFullPath(syntax.SyntaxTree.FilePath)]:null,
            native_text_sha256=Hash(Encoding.UTF8.GetBytes(syntax.SyntaxTree.GetText().ToString())),
            language_version=options?.LanguageVersion.ToString(),preprocessor_symbols=options?.PreprocessorSymbolNames.OrderBy(x=>x,StringComparer.Ordinal).ToArray(),
            inactive_regions=inactive});
        foreach(var directive in syntax.DescendantNodes().OfType<UsingDirectiveSyntax>().OrderBy(node=>node.SpanStart))
        {
            var name=directive.NamespaceOrType;
            var info=name==null?default: model.GetSymbolInfo(name);
            var symbol=info.Symbol;
            if(symbol is IAliasSymbol alias)symbol=alias.Target;
            var candidates=info.CandidateSymbols.IsDefaultOrEmpty?Array.Empty<ISymbol>():info.CandidateSymbols.ToArray();
            var candidateReason=info.CandidateReason;
            if(symbol is IErrorTypeSymbol error)
            {
                if(candidates.Length==0&&!error.CandidateSymbols.IsDefaultOrEmpty)candidates=error.CandidateSymbols.ToArray();
                if(candidateReason==CandidateReason.None)candidateReason=error.CandidateReason;
            }
            var diagnostics=model.GetDiagnostics(directive.Span).Where(d=>d.Severity==DiagnosticSeverity.Error).ToArray();
            var contributions=new List<object>();var targets=new SortedSet<string>(StringComparer.Ordinal);
            bool metadata=false,source=false,unknown=false;
            string status;
            if(candidateReason==CandidateReason.Ambiguous||diagnostics.Any(d=>d.Id is "CS0104" or "CS0433"))status="ambiguous";
            else if(candidateReason==CandidateReason.Inaccessible||diagnostics.Any(d=>d.Id=="CS0122"))status="inaccessible";
            else if(symbol==null||symbol is IErrorTypeSymbol)status="unresolved";
            else if(diagnostics.Length>0)status="invalid_statement";
            else if(symbol is INamespaceSymbol ns)
            {
                foreach(var part in Leaves(ns).OrderBy(PartKey,StringComparer.Ordinal))
                {
                    var declaration=part;string mapping="native constituent syntax references";
                    var refs=part.DeclaringSyntaxReferences;
                    var origin=await OriginAsync(model.Compilation,part.ContainingAssembly);
                    var sourceCompilation=origin.Compilation;
                    if(refs.IsDefaultOrEmpty&&sourceCompilation!=null)
                    {
                        // OriginAsync requires a native reference/source identity
                        // mapping. It never selects a project by simple name.
                        declaration=WithinAssembly(sourceCompilation.Assembly,part);
                        refs=declaration?.DeclaringSyntaxReferences??System.Collections.Immutable.ImmutableArray<SyntaxReference>.Empty;
                        mapping=origin.Method+"; exact namespace within that source assembly";
                    }
                    // A source-project candidate without a native compilation
                    // reference is UNKNOWN, never reverse-mapped by spelling.
                    bool isSource=!refs.IsDefaultOrEmpty||sourceCompilation!=null||origin.Unknown||part.Locations.Any(location=>location.IsInSource);
                    bool isUnknown=isSource&&refs.IsDefaultOrEmpty;
                    source|=isSource;metadata|=!isSource;unknown|=isUnknown;
                    var locations=Locations(refs,targets);
                    contributions.Add(new {native_namespace=Describe(part),source_project=sourceCompilation==null?null:origin.Project?.FilePath,mapping,
                        native_preferred_source_project=origin.Project?.FilePath,
                        source_identity_method=origin.Method,source_identity_candidates=origin.Candidates,
                        origin_assembly=declaration?.ContainingAssembly?.Identity.ToString(),
                        status=isUnknown?"source_mapping_unknown":isSource?"source":"metadata-only",
                        declarations=locations});
                }
                status=unknown?"native_mapping_unknown":source&&metadata?"mixed_source_metadata":source?"source":"external_metadata";
            }
            else if(symbol is INamedTypeSymbol named)
            {
                var original=named.OriginalDefinition;var refs=original.DeclaringSyntaxReferences;
                string mapping="native original type syntax references";
                var origin=await OriginAsync(model.Compilation,named.ContainingAssembly);
                var sourceCompilation=origin.Compilation;
                ISymbol sourceLookup=null;
                if(refs.IsDefaultOrEmpty&&sourceCompilation!=null)
                {
                    var found=await SymbolFinder.FindSourceDefinitionAsync(original,solution);
                    sourceLookup=found;
                    if(found is INamedTypeSymbol foundType&&
                        foundType.ContainingAssembly.Identity.Equals(named.ContainingAssembly.Identity)&&
                        foundType.OriginalDefinition.GetDocumentationCommentId()==named.OriginalDefinition.GetDocumentationCommentId()&&
                        foundType.DeclaringSyntaxReferences.All(reference=>sourceCompilation.SyntaxTrees.Contains(reference.SyntaxTree)))
                    {original=foundType.OriginalDefinition;refs=original.DeclaringSyntaxReferences;mapping=origin.Method+"; SymbolFinder.FindSourceDefinitionAsync with native identity/tree checks";}
                }
                source=!refs.IsDefaultOrEmpty;
                unknown=!source&&(sourceCompilation!=null||origin.Unknown);
                metadata=!source&&!unknown;
                contributions.Add(new {native_type=Describe(named),native_source_type=Describe(original),mapping,
                    source_lookup=Describe(sourceLookup),source_project=sourceCompilation==null?null:origin.Project?.FilePath,
                    native_preferred_source_project=origin.Project?.FilePath,
                    source_identity_method=origin.Method,source_identity_candidates=origin.Candidates,
                    status=unknown?"source_mapping_unknown":source?"source":"metadata-only",declarations=Locations(refs,targets)});
                status=unknown?"native_mapping_unknown":source?"source":"external_metadata";
            }
            else status="unsupported_bound_symbol";
            var selected=targets.Where(target=>target.StartsWith(prefix,StringComparison.Ordinal)).ToArray();
            var measured=selected.Where(target=>target!=path).ToArray();
            Usings.Add(new {project=project.FilePath,assembly=model.Compilation.Assembly.Identity.ToString(),path,
                span=Span(syntax.SyntaxTree,directive.Span),name_span=name==null?null:Span(syntax.SyntaxTree,name.Span),
                written_name=name?.ToString(),alias=directive.Alias?.Name.Identifier.ValueText,
                is_static=directive.StaticKeyword.IsKind(SyntaxKind.StaticKeyword),is_global=directive.GlobalKeyword.IsKind(SyntaxKind.GlobalKeyword),
                scope=directive.GlobalKeyword.IsKind(SyntaxKind.GlobalKeyword)?"compilation":directive.Parent is CompilationUnitSyntax?"file":"namespace",
                scope_span=Span(syntax.SyntaxTree,directive.Parent.Span),
                scope_symbol=directive.Parent is BaseNamespaceDeclarationSyntax scopeNamespace?Describe(model.GetDeclaredSymbol(scopeNamespace)):null,
                status,bound_symbol=Describe(symbol),candidate_reason=candidateReason.ToString(),candidates=candidates.Select(Describe).ToArray(),
                errors=diagnostics.Select(d=>new {id=d.Id,message=d.GetMessage(),span=d.Location.IsInSource?Span(d.Location.SourceTree,d.Location.SourceSpan):null}).ToArray(),
                contributions,all_source_target_paths=targets.ToArray(),selected_target_paths=selected,measured_cross_file_target_paths=measured,
                target_scope=measured.Length>0?"has_measured_cross_file_targets":selected.Length>0?"selected_self_only":targets.Count>0?"outside_emission_selection":"no_source_target",
                source_contribution_mapping_complete=!unknown&&(status is "source" or "mixed_source_metadata" or "external_metadata"),
                dependency_semantics="None: unused using succeeds without establishing an actual-use file edge"});
        }
    }

    object[] Locations(System.Collections.Immutable.ImmutableArray<SyntaxReference> refs,SortedSet<string> targets)
    {
        if(refs.IsDefaultOrEmpty)return Array.Empty<object>();
        return refs.OrderBy(reference=>reference.SyntaxTree.FilePath,StringComparer.Ordinal).ThenBy(reference=>reference.Span.Start).Select(reference=>{
            string file=reference.SyntaxTree.FilePath;Track(file);string path=Relative(file);
            // Missing/generated paths remain raw evidence, never measured targets.
            if(File.Exists(file)&&path!=null)targets.Add(path);
            return (object)new {path,span=Span(reference.SyntaxTree,reference.Span),
                physical_file=File.Exists(file),selected=Selected(file),
                native_text_sha256=Hash(Encoding.UTF8.GetBytes(reference.SyntaxTree.GetText().ToString())),
                physical_sha256=File.Exists(file)?inputs[Path.GetFullPath(file)]:null};
        }).ToArray();
    }

    static IEnumerable<INamespaceSymbol> Leaves(INamespaceSymbol ns)
    {
        var parts=ns.ConstituentNamespaces;
        if(parts.Length==1&&SymbolEqualityComparer.Default.Equals(parts[0],ns)){yield return ns;yield break;}
        foreach(var part in parts)foreach(var leaf in Leaves(part))yield return leaf;
    }
    static string PartKey(INamespaceSymbol ns)=>ns.ContainingAssembly?.Identity+"|"+ns.ContainingModule?.Name+"|"+ns.ToDisplayString();
    async Task<Origin> OriginAsync(Compilation context,IAssemblySymbol assembly)
    {
        if(assembly==null)return new(null,null,"no containing assembly",Array.Empty<object>(),false);
        if(SymbolEqualityComparer.Default.Equals(assembly,context.Assembly))
            return new(context,solution.GetProject(context.Assembly),"native current compilation assembly",Array.Empty<object>(),false);
        if(context.GetMetadataReference(assembly) is CompilationReference reference)
            return new(reference.Compilation,solution.GetProject(reference.Compilation.Assembly),
                "Compilation.GetMetadataReference -> CompilationReference.Compilation",Array.Empty<object>(),false);
        // PE-to-selected-source association is unsupported in this producer.
        // Native preference and exact AssemblyIdentity candidates are diagnostic
        // facts only: they never admit source targets or namespace membership.
        var preferred=solution.GetProject(assembly);
        sourceProjects??=LoadSourceProjectsAsync();
        var matches=(await sourceProjects).Where(entry=>entry.Compilation.Assembly.Identity.Equals(assembly.Identity)).ToArray();
        var candidates=matches.Select(entry=>(object)new {path=entry.Project.FilePath,
            assembly=entry.Compilation.Assembly.Identity.ToString(),native_preferred=entry.Project.Id==preferred?.Id}).ToArray();
        if(matches.Length==0)
            return new(null,preferred,"metadata; no exact source AssemblyIdentity match",candidates,false);
        return new(null,preferred,
            "UNSUPPORTED: PE-to-selected-source association; native preference and exact AssemblyIdentity candidates are diagnostics only",
            candidates,true);
    }
    async Task<List<(Project Project,Compilation Compilation)>> LoadSourceProjectsAsync()
    {
        var result=new List<(Project,Compilation)>();
        foreach(var project in solution.Projects.Where(project=>project.Language==LanguageNames.CSharp).OrderBy(project=>project.FilePath,StringComparer.Ordinal))
        {
            var compilation=await project.GetCompilationAsync();
            if(compilation!=null)result.Add((project,compilation));
        }
        return result;
    }
    static INamespaceSymbol WithinAssembly(IAssemblySymbol assembly,INamespaceSymbol target)
    {
        var names=new Stack<string>();for(var ns=target;!ns.IsGlobalNamespace;ns=ns.ContainingNamespace)names.Push(ns.Name);
        var found=assembly.GlobalNamespace;
        foreach(string name in names)
        {
            var next=found.GetNamespaceMembers().Where(member=>member.Name==name).ToArray();
            if(next.Length!=1)return null;found=next[0];
        }
        return found;
    }
    static object Describe(ISymbol symbol)=>symbol==null?null:new {
        kind=symbol.Kind.ToString(),display=symbol.ToDisplayString(SymbolDisplayFormat.FullyQualifiedFormat),
        assembly=symbol.ContainingAssembly?.Identity.ToString()??(symbol as IAssemblySymbol)?.Identity.ToString(),
        module=symbol.ContainingModule?.Name,documentation_id=symbol.GetDocumentationCommentId(),
        namespace_kind=(symbol as INamespaceSymbol)?.NamespaceKind.ToString(),implicit_declaration=symbol.IsImplicitlyDeclared};
    static object Span(SyntaxTree tree,TextSpan span)
    {
        var lines=tree.GetLineSpan(span);return new {start_line=lines.StartLinePosition.Line+1,start_character=lines.StartLinePosition.Character,
            end_line=lines.EndLinePosition.Line+1,end_character=lines.EndLinePosition.Character,start_offset=span.Start,length=span.Length};
    }
    string Relative(string path)=>string.IsNullOrEmpty(path)?null:Path.GetRelativePath(root,Path.GetFullPath(path)).Replace('\\','/');
    bool Selected(string path)=>!string.IsNullOrEmpty(path)&&Relative(path).StartsWith(prefix,StringComparison.Ordinal);
    static string Hash(byte[] bytes)=>Convert.ToHexString(SHA256.HashData(bytes)).ToLowerInvariant();
}
