// Bench-only native declaration evidence from the native project loader.
// No extractor output/SCIP/store is read to construct these answers. Rich raw
// spans remain available when the product's declared signature rule is lossy.
using System.Security.Cryptography;
using System.Text;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Text;

static class DeclarationEvidence
{
    // Grounded in docs/design/language-breadth.md section3.3 and
    // internal/extractor/tsextract.go:1231-1250, exact f844af51. This exports
    // evidence, not a new score. No-body FIRST-LINE behavior is preserved.
    public const string SignatureContract = "definition Span.Start to body Span.Start; without a body only its first line; Unicode whitespace collapsed/trimmed; cap200Unicode scalar values (199 + ellipsis), per existing product contract";

    internal sealed record Result(object File, List<object> Declarations);
    sealed record Declaration(string Kind, SyntaxNode Definition, SyntaxNode Statement, SyntaxNodeOrToken Name);

    public static Result Collect(string root, string relative, Project project, SyntaxNode syntax, SemanticModel model)
    {
        var path=Path.GetFullPath(syntax.SyntaxTree.FilePath);
        var raw=System.IO.File.ReadAllBytes(path);
        var before=Convert.ToHexString(SHA256.HashData(raw)).ToLowerInvariant();
        var text=syntax.SyntaxTree.GetText();
        using(var reader=new StreamReader(new MemoryStream(raw),text.Encoding??Encoding.UTF8,true))
            if(!String.Equals(reader.ReadToEnd(),text.ToString(),StringComparison.Ordinal))
                throw new IOException("Loaded semantic source differs from disk: "+relative);
        var rows=new List<object>();
        foreach(var declaration in Declarations(syntax))
        {
            var node=declaration.Definition;
            var symbol=model.GetDeclaredSymbol(node);
            // Events/fields are emitted at their individual variable declarator;
            // no first-variable line is borrowed for later names.
            var nameText=declaration.Name.ToString();
            var nameToken=declaration.Name.IsToken?declaration.Name.AsToken():default;
            var bodyStart=BodyStart(node);
            var headerEnd=bodyStart??node.Span.End;
            var contractEnd=headerEnd;
            if(bodyStart==null)
            {
                var firstLine=text.Lines.GetLineFromPosition(node.SpanStart);
                contractEnd=Math.Min(contractEnd,firstLine.End);
            }
            var header=text.ToString(TextSpan.FromBounds(node.SpanStart,headerEnd));
            var contractSource=text.ToString(TextSpan.FromBounds(node.SpanStart,contractEnd));
            var semanticContainer=symbol?.ContainingSymbol;
            var declarations=symbol?.DeclaringSyntaxReferences.OrderBy(reference=>reference.SyntaxTree.FilePath,StringComparer.Ordinal).ThenBy(reference=>reference.Span.Start).Select(reference=>{
                var target=reference.GetSyntax();
                return new {path=Path.GetRelativePath(root,target.SyntaxTree.FilePath).Replace('\\','/'),
                    span=Span(target.SyntaxTree,target.Span)};
            }).ToArray();
            rows.Add(new {path=relative,project=project.Name,project_file=project.FilePath,
                assembly=model.Compilation.Assembly.Identity.ToString(),
                raw_kind=declaration.Kind,syntax_kind=node.Kind().ToString(),
                name_text=nameText,name_value=nameToken.RawKind==0?nameText:nameToken.ValueText,
                name_span=Span(node.SyntaxTree,declaration.Name.Span),
                definition_span=Span(node.SyntaxTree,node.Span),full_span=Span(node.SyntaxTree,node.FullSpan),
                statement_span=Span(node.SyntaxTree,declaration.Statement.Span),
                containing_namespace=symbol?.ContainingNamespace?.ToDisplayString(),
                containing_symbol=semanticContainer?.ToDisplayString(SymbolDisplayFormat.FullyQualifiedFormat),
                semantic_kind=symbol?.Kind.ToString(),semantic_type_kind=(symbol as INamedTypeSymbol)?.TypeKind.ToString(),
                method_kind=(symbol as IMethodSymbol)?.MethodKind.ToString(),metadata_name=symbol?.MetadataName,
                display_name=symbol?.ToDisplayString(SymbolDisplayFormat.CSharpErrorMessageFormat),
                documentation_id=symbol?.GetDocumentationCommentId(),
                binding_status=symbol==null?"unresolved-declared-symbol":"resolved",declaring_syntax_references=declarations,
                raw_header=header,header_span=Span(node.SyntaxTree,TextSpan.FromBounds(node.SpanStart,headerEnd)),
                signature_contract_applies=SignatureApplies(declaration.Kind),
                signature_normalized_candidate=NormalizeSignature(contractSource),
                signature_source_span=Span(node.SyntaxTree,TextSpan.FromBounds(node.SpanStart,contractEnd)),
                signature_has_conditional_directives=contractSource.Split('\n').Any(line=>line.TrimStart().StartsWith("#",StringComparison.Ordinal)),
                signature_limit="Raw active-tree source slice; conditional directive/preprocessor policy differences remain unjudged until compared explicitly; this output computes no accuracy"});
        }
        var after=Convert.ToHexString(SHA256.HashData(System.IO.File.ReadAllBytes(path))).ToLowerInvariant();
        if(before!=after)throw new IOException("Source changed while exporting declarations: "+relative);
        var options=syntax.SyntaxTree.Options as CSharpParseOptions;
        var diagnostics=syntax.SyntaxTree.GetDiagnostics().Select(d=>new {id=d.Id,severity=d.Severity.ToString(),message=d.ToString()}).ToArray();
        return new Result(new {path=relative,project=project.Name,source_sha256=before,
            source_text_checksum=Convert.ToHexString(text.GetChecksum().ToArray()).ToLowerInvariant(),
            source_text_checksum_algorithm=text.ChecksumAlgorithm.ToString(),
            language_version=options?.LanguageVersion.ToString(),
            preprocessor_symbols=options?.PreprocessorSymbolNames.OrderBy(value=>value,StringComparer.Ordinal).ToArray(),
            diagnostics,declarations=rows.Count},rows);
    }

    static object Span(SyntaxTree tree,TextSpan span)
    {
        var lines=tree.GetLineSpan(span).Span;
        return new {start_line=lines.Start.Line+1,start_column_utf16=lines.Start.Character,
            end_line=lines.End.Line+1,end_column_utf16=lines.End.Character,
            start_offset_utf16=span.Start,length_utf16=span.Length};
    }

    static IEnumerable<Declaration> Declarations(SyntaxNode root)
    {
        foreach(var node in root.DescendantNodes())
        {
            Declaration declaration=node switch {
                BaseNamespaceDeclarationSyntax n=>new("namespace",n,n,n.Name.GetLastToken()),
                RecordDeclarationSyntax n=>new(n.ClassOrStructKeyword.IsKind(SyntaxKind.StructKeyword)?"recordStruct":"record",n,n,n.Identifier),
                ClassDeclarationSyntax n=>new("class",n,n,n.Identifier),
                StructDeclarationSyntax n=>new("struct",n,n,n.Identifier),
                InterfaceDeclarationSyntax n=>new("interface",n,n,n.Identifier),
                EnumDeclarationSyntax n=>new("enum",n,n,n.Identifier),
                EnumMemberDeclarationSyntax n=>new("enumMember",n,n,n.Identifier),
                DelegateDeclarationSyntax n=>new("delegate",n,n,n.Identifier),
                MethodDeclarationSyntax n=>new("method",n,n,n.Identifier),
                ConstructorDeclarationSyntax n=>new("constructor",n,n,n.Identifier),
                DestructorDeclarationSyntax n=>new("destructor",n,n,n.Identifier),
                OperatorDeclarationSyntax n=>new("operator",n,n,n.OperatorToken),
                ConversionOperatorDeclarationSyntax n=>new("conversionOperator",n,n,n.Type),
                PropertyDeclarationSyntax n=>new("property",n,n,n.Identifier),
                IndexerDeclarationSyntax n=>new("indexer",n,n,n.ThisKeyword),
                EventDeclarationSyntax n=>new("event",n,n,n.Identifier),
                _=>null};
            if(declaration!=null)yield return declaration;
            if(node is BaseFieldDeclarationSyntax field)
                foreach(var variable in field.Declaration.Variables)
                    yield return new Declaration(field is EventFieldDeclarationSyntax?"event":"field",variable,field,variable.Identifier);
        }
    }

    static int? BodyStart(SyntaxNode node)=>node switch {
        BaseMethodDeclarationSyntax method=>method.Body?.SpanStart??method.ExpressionBody?.SpanStart,
        TypeDeclarationSyntax type when !type.OpenBraceToken.IsMissing&&type.OpenBraceToken.RawKind!=0=>type.OpenBraceToken.SpanStart,
        _=>null};

    static bool SignatureApplies(string kind)=>kind is "method" or "constructor" or "conversionOperator" or "class" or "record" or "recordStruct" or "struct" or "interface";

    internal static string NormalizeSignature(string source)
    {
        var builder=new StringBuilder();var pendingSpace=false;
        foreach(var rune in source.EnumerateRunes())
        {
            if(Rune.IsWhiteSpace(rune)){pendingSpace=builder.Length>0;continue;}
            if(pendingSpace)builder.Append(' ');
            builder.Append(rune.ToString());pendingSpace=false;
        }
        var normalized=builder.ToString();var runes=normalized.EnumerateRunes().ToArray();
        if(runes.Length<=200)return normalized;
        return string.Concat(runes.Take(199).Select(rune=>rune.ToString())).TrimEnd(' ')+"…";
    }
}
