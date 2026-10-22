# Unity assembly files

The selectors and generated-repository fixtures are original work, adapted from
handoff unit main-0001. No Unity implementation or third-party query is copied or
executed. This is groundwork for GDEV-10.

The rules use documented behavior of Unity 2021 and later; native CompilationPipeline calibration on Unity 6000.3 pending.

- [Assembly definitions](https://docs.unity3d.com/2021.3/Documentation/Manual/ScriptCompilationAssemblyDefinitionFiles.html)
- [Assembly Definition File Format](https://docs.unity3d.com/2021.3/Documentation/Manual/AssemblyDefinitionFileFormat.html)
- [Special folders and compilation order](https://docs.unity3d.com/2021.3/Documentation/Manual/ScriptCompileOrderFolders.html)
- [UnityCsReference EditorBuildRules.cs](https://github.com/Unity-Technologies/UnityCsReference/blob/master/Editor/Mono/Scripting/ScriptCompilation/EditorBuildRules.cs): lowercase path matching, the four root-level firstpass folders (including iPhone Standard Assets), and Editor folder matching. Custom assemblies take precedence over predefined folder rules.
- [Special and hidden folders](https://docs.unity3d.com/2021.3/Documentation/Manual/SpecialFolders.html): excluded dot-prefixed, tilde-suffixed, CVS and temporary paths never become sources or assembly barriers.

The dialect and `fusion.ResolveUnityAssemblies` share declaration construction.
The pass joins discovered C# paths, asmrefs and sibling asmdef metadata using
existing observations and file-resolved edges. Facts remain derived at the
`pre-scan` tier, never verified. A single-file metadata refresh retains the same
node identity and metadata and records `Unity metadata refresh needs a full ingest`
on its claim. Only a full ingest recomputes cross-file relationships.

Roots are found within the existing bounded walk from ProjectVersion markers or
Assets paths, after an asmdef, asmref or ProjectVersion input identifies Unity.
Independent roots resolve separately. Overlapping roots are reported and the
nearest root governs. Predefined nodes attach to that project's
`ProjectSettings/ProjectVersion.txt` when present; otherwise ownership claims
retain their assembly names without minting a node or edge for that assembly.
Ignored paths do not contribute facts or barriers.

Only declared assembly references are resolved. Missing, unreadable, malformed or
ambiguous GUID metadata never falls back to an assembly name. Duplicate assembly
names and folders containing multiple asmdefs/asmrefs are reported and ownership
remains unknown. Package sources with no definition have unknown ownership.
Without an asmdef, asmref or ProjectVersion input, C# paths produce no Unity facts.

Scope describes declared editor/player eligibility. Non-Editor include lists and
an Editor exclusion are player-only. Platform lists, define constraints and
version defines are retained, not evaluated against a build target or installed
package version. Implicit engine, plugin and auto-reference expansions and
Unity-version differences remain outside this groundwork. The independent
CompilationPipeline oracle runs later on the consumer's licensed machine.
There is no support-level reading or compiler-grade claim here.
