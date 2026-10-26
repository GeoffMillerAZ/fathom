param(
    [Parameter(Mandatory)][string]$Subject,
    [Parameter(Mandatory)][string]$FilesJson,
    [Parameter(Mandatory)][string]$Manifest,
    [Parameter(Mandatory)][string]$Out
)
$ErrorActionPreference = 'Stop'
if ($PSVersionTable.PSVersion.ToString() -ne '7.6.9') { throw 'Pinned PowerShell 7.6.9 required' }
$root = [IO.Path]::GetFullPath($Subject).TrimEnd('/')
$manifestPath = [IO.Path]::GetFullPath($Manifest)
$moduleRoot = [IO.Path]::GetDirectoryName($manifestPath)
$env:PSModulePath = Join-Path $PSHOME 'Modules'
$files = @(Get-Content -LiteralPath $FilesJson -Raw | ConvertFrom-Json)
$fileCap = if ($env:PWSH_ORACLE_FILE_CAP) { [int]$env:PWSH_ORACLE_FILE_CAP } else { 2048 }
$byteCap = if ($env:PWSH_ORACLE_BYTE_CAP) { [long]$env:PWSH_ORACLE_BYTE_CAP } else { 67108864 }
$siteCap = if ($env:PWSH_ORACLE_SITE_CAP) { [int]$env:PWSH_ORACLE_SITE_CAP } else { 1000000 }
if ($fileCap -lt 1 -or $fileCap -gt 2048 -or $byteCap -lt 1 -or $byteCap -gt 67108864 -or $siteCap -lt 1 -or $siteCap -gt 1000000) { throw 'whole-unit refusal: invalid caps' }
if (-not $files.Count -or $files.Count -gt $fileCap) { throw 'whole-unit refusal: file count cap or empty population' }
$bytes = 0L
foreach ($relative in $files) {
    $path = [IO.Path]::GetFullPath((Join-Path $root $relative))
    if (-not $path.StartsWith($root + '/', [StringComparison]::Ordinal)) { throw 'whole-unit refusal: path outside subject' }
    $file = Get-Item -LiteralPath $path
    if ($file.LinkType) { throw 'whole-unit refusal: symlink input' }
    $bytes += $file.Length
}
if ($bytes -gt $byteCap) { throw 'whole-unit refusal: source byte cap' }
function Get-OracleBare([string]$Name) {
    $index = $Name.LastIndexOfAny([char[]]'.:#/\ )')
    if ($index -ge 0) { return $Name.Substring($index + 1) }
    return $Name
}
function Get-OracleCaller($Node) {
    for ($parent = $Node.Parent; $null -ne $parent; $parent = $parent.Parent) {
        if ($parent -is [Management.Automation.Language.FunctionMemberAst]) { return Get-OracleBare $parent.Name }
        if ($parent -is [Management.Automation.Language.FunctionDefinitionAst] -and
            $parent.Parent -isnot [Management.Automation.Language.FunctionMemberAst]) { return Get-OracleBare $parent.Name }
    }
    return '<module>'
}
function Get-OracleModules {
    @(Get-Module | Sort-Object Name | ForEach-Object {
        $hash = if ($_.Path -and [IO.File]::Exists($_.Path)) { (Get-FileHash -LiteralPath $_.Path -Algorithm SHA256).Hash.ToLowerInvariant() } else { $null }
        [ordered]@{ name = $_.Name; version = $_.Version.ToString(); code_sha256 = $hash }
    })
}
$PSModuleAutoLoadingPreference = 'None'
$before = @(Get-OracleModules)
$module = Import-Module -Name $manifestPath -PassThru -ErrorAction Stop
if ($module.Count -gt 1) { throw 'whole-unit refusal: ambiguous root module' }
$lookup = { param($Name) $ExecutionContext.InvokeCommand.GetCommand($Name, [Management.Automation.CommandTypes]::All) }
# Native loader metadata determines which original files really belong to the
# declared module. Never resolve ordinary test files in a private module scope.
$moduleSources = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
$loadedFunctions = @($module.Invoke({
    @(Get-ChildItem Function: | Where-Object { $_.ModuleName -eq $ExecutionContext.SessionState.Module.Name } |
        ForEach-Object { $_.ScriptBlock.File })
}))
foreach ($file in $loadedFunctions) { if ($file -and $file.StartsWith($root + '/', [StringComparison]::Ordinal)) { $null = $moduleSources.Add($file) } }
$moduleContextSites = 0
$globalContextSites = 0
$lookupCache = @{}
$definitionCache = @{}
$fileHashCache = @{}
$targetFiles = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
$definitionScopes = @{}
$sites = [Collections.Generic.List[object]]::new()
$pairs = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
$calls = [Collections.Generic.List[object]]::new()
$parseErrors = 0
$unknown = 0
$commandSites = 0
$memberSites = 0
foreach ($relative in $files) {
    $tokens = $null; $errors = $null
    $ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $root $relative), [ref]$tokens, [ref]$errors)
    if ($errors.Count) { $parseErrors++ }
    $nodes = $ast.FindAll({ param($Node)
        $Node -is [Management.Automation.Language.CommandAst] -or $Node -is [Management.Automation.Language.InvokeMemberExpressionAst]
    }, $true)
    foreach ($node in $nodes) {
        $isCommand = $node -is [Management.Automation.Language.CommandAst]
        $written = if ($isCommand) { $node.GetCommandName() } elseif ($node.Member -is [Management.Automation.Language.StringConstantExpressionAst]) { $node.Member.Value } else { $null }
        if (-not $written) { continue }
        $caller = Get-OracleCaller $node
        if ($isCommand) { $commandSites++ } else { $memberSites++ }
        $answer = $null
        if ($isCommand) {
            $scope = if ($moduleSources.Contains([IO.Path]::GetFullPath((Join-Path $root $relative)))) { 'module' } else { 'global' }
            if ($scope -eq 'module') { $moduleContextSites++ } else { $globalContextSites++ }
            $cacheKey = $scope + "`t" + $written
            if (-not $lookupCache.ContainsKey($cacheKey)) {
                $found = if ($scope -eq 'module') { @($module.Invoke($lookup, @($written))) } else { @(& $lookup $written) }
                $command = if ($found.Count -eq 1) { $found[0] } else { $null }
                if ($command -is [Management.Automation.AliasInfo]) { $command = $command.ResolvedCommand }
                $allowed = $false
                if ($command -is [Management.Automation.CmdletInfo]) {
                    $allowed = $command.ModuleName -eq 'Microsoft.PowerShell.Core' -or
                        ($command.Module -and $command.Module.ModuleBase.StartsWith($PSHOME + '/', [StringComparison]::Ordinal)) -or
                        ($command.Module -and $command.Module.ModuleBase.StartsWith($moduleRoot, [StringComparison]::Ordinal))
                } elseif ($command -is [Management.Automation.FunctionInfo] -or $command -is [Management.Automation.FilterInfo]) {
                    $source = $command.ScriptBlock.File
                    $allowed = $source -and ($source.StartsWith($moduleRoot + '/', [StringComparison]::Ordinal) -or
                        $source.StartsWith($PSHOME + '/', [StringComparison]::Ordinal) -or
                        $source.StartsWith($root + '/', [StringComparison]::Ordinal))
                }
                $lookupCache[$cacheKey] = if ($allowed) { Get-OracleBare $command.Name } else { $null }
                $definitionCache[$cacheKey] = $null
                if ($allowed) {
                    $definitionPath = if ($command -is [Management.Automation.FunctionInfo]) { $command.ScriptBlock.File } elseif ($command.Module -and $command.Module.Path) { $command.Module.Path } else { $command.ImplementingType.Assembly.Location }
                    if ($definitionPath -and [IO.File]::Exists($definitionPath)) {
                        if (-not $fileHashCache.ContainsKey($definitionPath)) { $fileHashCache[$definitionPath] = (Get-FileHash -LiteralPath $definitionPath -Algorithm SHA256).Hash.ToLowerInvariant() }
                        $definitionScope = if ($definitionPath.StartsWith($root + '/', [StringComparison]::Ordinal)) { 'original-subject' } elseif ($definitionPath.StartsWith($PSHOME + '/', [StringComparison]::Ordinal)) { 'pinned-runtime' } else { 'declared-subject-module' }
                        $definitionCache[$cacheKey] = @($definitionScope, $fileHashCache[$definitionPath])
                    }
                }
            }
            $answer = $lookupCache[$cacheKey]
        }
        if (-not $answer) { $unknown++ }
        $definition = if ($isCommand -and $answer) { $definitionCache[$cacheKey] } else { $null }
        if ($definition) {
            $null = $targetFiles.Add($definition[0] + ':' + $definition[1])
            $definitionScopes[$definition[0]]++
        }
        $sites.Add([ordered]@{ path = $relative; line = $node.Extent.StartLineNumber; column = $node.Extent.StartColumnNumber;
            from = $caller; written = $written; kind = $(if ($isCommand) { 'command' } else { 'member' }); resolved = $answer; resolved_definition = $definition })
        if ($sites.Count -gt $siteCap) { throw 'whole-unit refusal: site count cap' }
        if ($answer) {
            $key = "$relative`t$caller`t$answer"
            if ($pairs.Add($key)) { $calls.Add([ordered]@{ path = $relative; from = $caller; to = $answer }) }
        }
    }
}
$receipt = [ordered]@{
    tool = 'PowerShell native InvokeCommand.GetCommand in declared module session'; version = '7.6.9'; tier = 'A';
    calls = $calls.ToArray(); sites = $sites.ToArray();
    receipt = [ordered]@{ files = $files.Count; bytes = $bytes; literal_sites = $sites.Count; command_sites = $commandSites;
        member_sites = $memberSites; native_module_source_files = $moduleSources.Count; module_context_sites = $moduleContextSites; global_context_sites = $globalContextSites; unresolved_sites = $unknown; parse_error_files = $parseErrors;
        resolved_definition_file_contents = $targetFiles.Count; resolved_definition_scope_sites = $definitionScopes;
        resolved_definition_sources_sha256 = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes([string]::Join("`n", @($targetFiles | Sort-Object -CaseSensitive))))).ToLowerInvariant();
        subject_module = $module.Name; subject_module_version = $module.Version.ToString();
        manifest_sha256 = (Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash.ToLowerInvariant();
        load_before = $before; load_after = @(Get-OracleModules);
        unresolvable_policy = 'all literal members, undeclared external commands/state and missing functions remain misses; no dot-sourcing of fixtures; native loader-owned source files use module context, ordinary test files use global context';
        module_loading_policy = 'autoload disabled; explicit matching subject manifest plus recorded pinned-runtime baseline only; profile disabled' }
}
$receipt | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $Out -Encoding utf8NoBOM
