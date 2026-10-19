// Bench-only declaration export from the subject's actual MSBuild selection.
// No extractor output, SCIP or store is read to construct native answers.
using System.Security.Cryptography;
using System.Text.Json;
using System.Xml.Linq;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.MSBuild;

static class ProjectDeclarations
{
    public static int Run(string[] args) => RunAsync(args,false).GetAwaiter().GetResult();
    public static int RunImports(string[] args) => RunAsync(args,true).GetAwaiter().GetResult();

    static async Task<int> RunAsync(string[] args,bool importsMode)
    {
        if(args.Length is not (3 or 4))
            throw new ArgumentException((importsMode?"project-imports":"project-declarations")+" <repository-root> <actual.sln|slnx|csproj> <repository-relative-source-directory> [owned-intermediate-root]");
        var root=Path.GetFullPath(args[0]);
        var input=Path.GetFullPath(args[1]);
        var selector=args[2];
        if(Path.IsPathRooted(selector)||selector.Contains('\\')||selector.Any(char.IsControl)||
            selector.Split('/').Any(segment=>segment is "" or "." or ".."))
            throw new ArgumentException("Source directory must be an explicit canonical repository-relative path");
        if(!Directory.Exists(Path.Combine(root,selector)))throw new DirectoryNotFoundException(selector);
        var prefix=selector+"/";
        var failures=new List<string>();var errors=new List<object>();var projects=new List<object>();
        var declarations=new List<object>();var files=new List<object>();var outside=new List<object>();
        var inputHash=Hash(input);
        var properties=new Dictionary<string,string>();
        string isolatedProperties=null;
        if(args.Length==4)
        {
            var owned=Path.GetFullPath(args[3]);Directory.CreateDirectory(owned);
            isolatedProperties=Path.Combine(owned,"oracle-output-paths.props");
            // Imported after Unity-style pre-SDK property groups. Keep each
            // project's design-time artifacts distinct without editing it.
            var obj=Path.Combine(owned,"obj").Replace('\\','/')+"/$(MSBuildProjectName)/";
            var bin=Path.Combine(owned,"bin").Replace('\\','/')+"/$(MSBuildProjectName)/";
            new XDocument(new XElement("Project",new XElement("PropertyGroup",
                new XElement("BaseIntermediateOutputPath",obj),new XElement("IntermediateOutputPath",obj),
                new XElement("MSBuildProjectExtensionsPath",obj),new XElement("OutputPath",bin),
                new XElement("UseSharedCompilation","false"),new XElement("NuGetAudit","false"))))
                .Save(isolatedProperties);
            properties["CustomBeforeMicrosoftCommonProps"]=isolatedProperties;
        }
        using var workspace=MSBuildWorkspace.Create(properties);
        workspace.RegisterWorkspaceFailedHandler(e=>{
            if(e.Diagnostic.Kind==WorkspaceDiagnosticKind.Failure)failures.Add(e.Diagnostic.Message);
            Console.Error.WriteLine("workspace: "+e.Diagnostic);
        });
        Solution solution;
        if(Path.GetExtension(input).Equals(".csproj",StringComparison.OrdinalIgnoreCase))
            solution=(await workspace.OpenProjectAsync(input)).Solution;
        else solution=await workspace.OpenSolutionAsync(input);
        // Both modes use this exact workspace, project selection and isolation.
        // The raw using exporter has no dependency on Fathom, SCIP or its scorer.
        var imports=importsMode?new ProjectImports(root,selector,solution,failures):null;
        if(importsMode){imports.Track(input);imports.Track(isolatedProperties);}
        foreach(var project in solution.Projects.OrderBy(project=>project.Name,StringComparer.Ordinal).ThenBy(project=>project.FilePath,StringComparer.Ordinal))
        {
            if(project.Language!=LanguageNames.CSharp)continue;
            var compilation=await project.GetCompilationAsync();
            if(compilation==null){failures.Add("null compilation: "+project.Name);continue;}
            // Emitted error order is ordinal rendered-message order per project.
            // OrderBy retains every diagnostic, including identical messages.
            var projectErrors=compilation.GetDiagnostics().Where(d=>d.Severity==DiagnosticSeverity.Error)
                .OrderBy(d=>d.ToString(),StringComparer.Ordinal).ToArray();
            foreach(var error in projectErrors)errors.Add(new {project=project.Name,message=error.ToString()});
            projects.Add(new {name=project.Name,path=project.FilePath,
                source_sha256=project.FilePath==null?null:Hash(project.FilePath),
                assembly=compilation.Assembly.Identity.ToString(),trees=compilation.SyntaxTrees.Count(),errors=projectErrors.Length});
            if(importsMode)imports.AddProject(project,compilation);
            Console.Error.WriteLine($"project {project.Name}: {compilation.SyntaxTrees.Count()} trees, {projectErrors.Length} errors");
            foreach(var document in project.Documents.OrderBy(document=>document.FilePath,StringComparer.Ordinal))
            {
                if(document.FilePath==null){failures.Add("document has no source path: "+project.Name+"/"+document.Name);continue;}
                var relative=Path.GetRelativePath(root,document.FilePath).Replace('\\','/');
                if(!relative.StartsWith(prefix,StringComparison.Ordinal))
                {
                    outside.Add(new {project=project.Name,path=relative,reason="outside explicit source directory; not declaration-emission selected"});
                    continue;
                }
                var model=await document.GetSemanticModelAsync();var syntax=await document.GetSyntaxRootAsync();
                if(model==null||syntax==null){failures.Add("null native model/root: "+relative);continue;}
                if(importsMode)await imports.AddFileAsync(relative,project,syntax,model);
                else
                {
                    var evidence=DeclarationEvidence.Collect(root,relative,project,syntax,model);
                    declarations.AddRange(evidence.Declarations);files.Add(evidence.File);
                }
            }
        }
        if(inputHash!=Hash(input))failures.Add("Project/solution input changed while exporting declarations");
        if(importsMode)
        {
            imports.CheckClosure(failures);
            Console.WriteLine(JsonSerializer.Serialize(new {
                schema="fathom.roslyn.project-imports/v1",tool="roslyn-project-imports",
                version=typeof(Microsoft.CodeAnalysis.CSharp.CSharpCompilation).Assembly.GetName().Version.ToString(),
                repository_root=root,input,input_sha256=inputHash,source_directory=selector,
                isolated_output_properties=isolatedProperties,
                isolated_output_properties_sha256=isolatedProperties==null?null:Hash(isolatedProperties),
                positions="1-based lines, 0-based UTF16 columns, half-open spans",
                semantics="Raw native using-statement binding and namespace declaration contributions only; never actual-use dependencies or a neutral grading document",
                emission_scope="Active using directives in selected regular project.Documents. Every compilation tree is inventoried; a selected generated/non-document tree refuses completeness rather than silently disappearing.",
                read_boundary="Source directory selects emitted using rows only. MSBuild loads projects, source generators and references outside it; no allowed-read, no-network or full MSBuild-import closure guarantee. Use an independently controlled offline environment and source receipt.",
                closure_scope="Input, loaded project files, physical compilation trees, PE references and owned output props; transitive MSBuild imports are not exhaustively enumerated",
                projects=imports.Projects,files=imports.Files,usings=imports.Usings,
                native_input_hashes=imports.Inputs,workspace_failures=failures,compile_errors=errors,
                outside_emission_selection=outside
            },new JsonSerializerOptions{WriteIndented=true}));
            return failures.Count==0&&errors.Count==0?0:1;
        }
        var payload=new {schema="fathom.roslyn.project-declarations/v1",tool="roslyn-project-declarations",
            version=typeof(Microsoft.CodeAnalysis.CSharp.CSharpCompilation).Assembly.GetName().Version.ToString(),
            repository_root=root,input,input_sha256=inputHash,source_directory=selector,
            isolated_output_properties=isolatedProperties,
            isolated_output_properties_sha256=isolatedProperties==null?null:Hash(isolatedProperties),
            scope="Actual MSBuild project/solution and active configuration only; source-directory-selected native declaration occurrences, not a syntax-union or runtime proof",
            read_boundary="Source directory controls emitted declarations only. MSBuildWorkspace loads other project/dependency inputs; this is not an allowed-read sandbox or offline guarantee.",
            positions="1-based lines, 0-based UTF16 columns, half-open ranges; definition/full/statement/name spans are separate",
            signature_contract=DeclarationEvidence.SignatureContract,projects,workspace_failures=failures,compile_errors=errors,
            outside_emission_selection=outside,files,declarations};
        Console.WriteLine(JsonSerializer.Serialize(payload,new JsonSerializerOptions{WriteIndented=true}));
        return failures.Count==0&&errors.Count==0?0:1;
    }

    static string Hash(string path)=>Convert.ToHexString(SHA256.HashData(File.ReadAllBytes(path))).ToLowerInvariant();
}
