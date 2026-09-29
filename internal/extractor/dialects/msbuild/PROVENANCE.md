# msbuild — provenance and divergences

LB-Q-msbuild, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect (no
`tags.scm`, no upstream grammar, no incumbent: a `.csproj` had no extractor): this file
records what it models, what it deliberately does not and why, and where the calibration
oracle comes from. The decode is `encoding/xml` (the doc engine's `xml` decoder: an element
with neither attributes nor children is a scalar, a repeated sibling becomes a list, a single
one reads as a one-item list, an attribute is the key `@Name`, and the element's own name is
dropped from the decoded root). The manifest claims by extension: `.csproj`, `.fsproj`,
`.vbproj`, `.props` and `.targets`. Every one of them is an MSBuild `<Project>` document.

## Two engine additions this dialect needed

Both are in `internal/extractor/docengine.go`, tested in `docengine_file_test.go`, recorded in
design §4, and additive: no other dialect's output changes.

- **`$file`**, a value expression: the base name of the file being read, extension included.
  An MSBuild project has no name of its own (its identity is its path; `AssemblyName` and
  `PackageId` are optional and mostly absent from an SDK-style project), so without a
  file-name value there is no entity to name. The extension stays in the name because
  `Directory.Build.props` and `Directory.Build.targets` share a directory and a stem, and the
  store's symbol key carries no kind. (Docker Compose's and GitLab CI's deferred rows name the
  same missing value.)
- **`repo-path` reads `\` as a separator.** A ProjectReference is written `..\Lib\Lib.csproj`
  in nearly every Windows-authored project (all of Polly's 31), and no repository file is named
  with a backslash. The reference as written stays in `Raw`.

Also `fusion.isDocCandidate` now names the five extensions: the cross-file pass gates on a
decode's extensions, and these are XML by another name.

## What it models

- **The project** (`MSBuildProject`, canonical `package`): the file's root, at its `<Project>`
  start tag, named by the file (`$file`), signed by its target framework when it writes one
  (`TargetFramework`, else `TargetFrameworks`, else the old-style `TargetFrameworkVersion`,
  else the `Sdk` attribute). Every MSBuild file has one, a props or targets file included: a
  `Directory.Build.targets` written empty to stop the parent's import is still a file of the
  build, and it is one entity whether the decode reads its empty root as a map or a scalar.
- **Targets** (`MSBuildTarget`, canonical `target`): each `<Target Name>` directly under the
  root, at its start tag. Not scored (below).
- **Package references** (`Calls` on the project): the `Include` of every `PackageReference`
  in an `ItemGroup` that is a child of the root. An `Update` or `Remove` modifies a reference
  declared elsewhere and is not one; a name-less `Include=""` is not one.
- **Project references** (`fusion.ResolveDocXrefs`, file-tier edges entity to entity):
  `ProjectReference Include` in an `ItemGroup` that is a child of the root, resolved by
  `repo-path` from the referring file's directory to the referenced file's project entity. A
  path that names no file of the repository (a package outside it), one that climbs out of it,
  one written with a property the file cannot expand (`$(RepoRoot)`) and one that names its own
  file resolve to nothing.

## What it deliberately does not model

- **Property expansion.** `$(RepoRoot)src\Http\...`, `$(MSBuildThisFileDirectory)..\lib.csproj`
  and every other `$(...)` are text as written; the engine has no expression language. On
  the subjects two of aspnetcore's seven in-scope ProjectReferences are written from
  `$(RepoRoot)` and miss (`xref_recall` .9474); Polly writes none.
- **`<Import>` edges.** A props or targets file is reached through an `<Import>` (or
  implicitly, as `Directory.Build.props` is), and both of the subjects write every explicit one
  with `$(MSBuildThisFileDirectory)`. The oracle reads the imports from `-preprocess` but the
  reading does not score them, since the dialect draws none.
- **Conditions and evaluation.** A declaration under a false `Condition` is still a reference
  here; the tool does not list it. The oracle unions the Debug and Release evaluations of
  every target framework a project lists to bring most of them back, and what a macOS run
  cannot evaluate (a framework the project adds only on Windows, `net481`: `System.Memory` under a
  `.NETFramework` condition in three of Polly's test projects) is the whole of the dialect-only
  references. The harness
  compares the references of the files the tool's truth holds one for, so the third file
  (`Polly.Testing.Tests.csproj`, whose one PackageReference is that Windows-only one, so the
  tool lists none for it) is left out of
  `ref_precision`, which reads two dialect-only pairs of 64; the oracle's own cross-check counts
  all three.
- **Implicit imports.** The tool attributes the items of a `Directory.Build.props` to the
  project that imports it and lists them for that project, with the props file as the item's
  defining file. The dialect reads each file alone, so the props file's references are the
  props entity's, which is also where the oracle puts them (`DefiningProjectFullPath`).
- **Assembly references that a build converts.** aspnetcore's `<Reference Include=assembly>`
  items become ProjectReferences in `eng/targets/ResolveReferences.targets` (12 oracle edges,
  all attributed to that file, outside the measured `src/Http`). Not modelled.
- **Central package management.** `PackageVersion` and `GlobalPackageReference` in a
  `Directory.Packages.props` are policy, not a project's own references.
- **Items, properties and control flow.** `Choose`/`When`, an `ItemGroup` inside a target,
  `Compile`/`None`/`Content` items, `UsingTask`, properties, and a target's `DependsOnTargets`,
  `BeforeTargets` and `AfterTargets` are not entities or references (`DependsOnTargets` is a
  `;`-separated list the engine cannot split).
- **Solutions.** `.sln` is a line-based text format and not one of the engine's decoders;
  `.slnx` is XML whose root is a `<Solution>` that a decoded root (which carries no element
  name) cannot tell from a `<Project>`. Neither is claimed, so `dotnet sln list` is not an
  oracle here.
- **Other MSBuild dialects of the format.** `.vcxproj`, `.proj`, `.wixproj`, `.sqlproj`,
  `.shproj` and `.projitems` are not claimed (the queue entry lists five extensions and a
  `.user` sidecar is not a project).
- **Ranges are not scored for tool formats** (§1.2): a project's end line is its `</Project>`
  line, a target's its `</Target>` line.

## The calibration oracle

One oracle file per subject, written by `calibration/oracles/msbuild` (its own Go module,
never a product dependency; `calibration/oracles/README.md` has the invocation), **tier A**
throughout: MSBuild itself, through `dotnet msbuild` (MSBuild 18.9.11 in .NET SDK 10.0.401),
evaluates every project file, and each item it lists names the file that defined it
(`DefiningProjectFullPath`), so what a file declares is the tool's word.

- definitions: one `package` per evaluated project and per file of the subject that an
  evaluation imported (read from `-preprocess`, which brackets every imported file with
  `<Import …>`/`</Import>` markers), at its `<Project>` start tag.
- references: `PackageReference` items, from the file that defined them.
- cross_references, entity granularity: `ProjectReference` items, from the file that defined
  them to the project MSBuild resolves each to (`FullPath`: properties expanded, separators
  normalised) when that is a file of the subject.

One evaluation sees one configuration and one framework, so each project is evaluated for
Debug and Release, for its default framework and for each one its `TargetFrameworks` names,
and the items are unioned. The independent `encoding/xml` decode inside the oracle counts what
a file writes that no evaluation listed (`declared-not-in-tool`: 3 over Polly, 0 over
aspnetcore) and what the tool lists that no file writes (`tool-not-declared`: 0 and 0), and
checks every `<Target Name>` the files declare against the targets in the tool's preprocessed
projects (4 of 4 in Polly, none in the measured part of aspnetcore); none of it is scored.

aspnetcore needs a stub to evaluate: its root `Directory.Build.props` and `.targets` import
`artifacts/bin/GenerateFiles/Directory.Build.{props,targets}`, which the repository's own first
build step (`eng/tools/GenerateFiles`, a restore and a build the subject cannot do offline in
20 minutes) writes. The oracle run creates both as empty `<Project />` files inside the
scratch clone (outside git, deleted with the clone), and the clone carries the MSBuild files
only (`calibration/corpus.d/msbuild.yaml`, `sparse:`), so a project that imports a file the
clone lacks fails to evaluate: `src/Testing/src/build/` is in the set for that reason, and the
oracle reads 40 of 40 projects evaluated.

## Subjects

`App-vNext/Polly` 8.8.0 (BSD-3-Clause; 34 files: 23 projects, 11 props and targets; 58
package references, 31 ProjectReference edges, every one written with backslashes) measured
whole, and `dotnet/aspnetcore` v10.0.12 (MIT; `src/Http`: 40 projects, 41 files in scope; 6
package references, 7 in-scope edges) measured with `-only 'src/Http/**'`, its 12 further
edges and 2 further references attributed to files under `eng/`.
