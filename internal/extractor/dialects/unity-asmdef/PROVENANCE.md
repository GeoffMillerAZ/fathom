# Unity assembly files

The selectors and generated-repository fixtures are original work. The review map
is handoff unit main-0001, adapted to current HEAD. No Unity implementation or
third-party query is copied or executed.

The behavior follows the documented Unity 2021+ rules:

- [Assembly definitions](https://docs.unity3d.com/2021.3/Documentation/Manual/ScriptCompilationAssemblyDefinitionFiles.html)
- [Assembly Definition File Format](https://docs.unity3d.com/2021.3/Documentation/Manual/AssemblyDefinitionFileFormat.html)
- [Special folders and compilation order](https://docs.unity3d.com/2021.3/Documentation/Manual/ScriptCompileOrderFolders.html)

The doc engine reads declarations. `fusion.ResolveUnityAssemblies` joins those
declarations with discovered C# paths, asmrefs and sibling asmdef metadata. Ingest
stores these as observations and file-resolved edges, served by the existing
symbol and edge queries. Unity observations use `pre-scan`, the existing tier
for derived file resolution, with `evidence: derived` on computed facts. They
never become verified through this strategy. A one-file refresh keeps the same
tier; a full ingest recomputes assembly relationships after metadata changes.

Only declared assembly references are resolved. Missing, unreadable, malformed or
ambiguous GUID metadata never falls back to an assembly name. A folder with an
unresolvable asmref or conflicting definitions is an ownership barrier. Package
sources with no definition have unknown ownership. Without an asmdef, asmref or
`ProjectSettings/ProjectVersion.txt`, C# paths produce no Unity facts.

Scope describes editor-only versus player-and-editor eligibility. Platform lists,
define constraints and version defines are retained as declarations, not evaluated
against a selected build target or installed package version. Implicit engine,
plugin and auto-reference expansions and Unity-version differences are outside
this groundwork. The independent CompilationPipeline oracle runs later on the
consumer's licensed machine. There is no support-level reading or compiler-grade
claim here.
