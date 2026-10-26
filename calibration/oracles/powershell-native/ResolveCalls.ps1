param(
    [Parameter(Mandatory)][string]$Subject,
    [Parameter(Mandatory)][string]$FilesJson,
    [Parameter(Mandatory)][string]$Manifest,
    [Parameter(Mandatory)][string]$Out,
    [ValidateSet('none','pester','psa')][string]$Harness = 'none',
    [string]$TestDependencyManifest,
    [string]$Repository,
    [string]$TrackedJson
)
$ErrorActionPreference = 'Stop'
$PSModuleAutoLoadingPreference = 'None'
if ($PSVersionTable.PSVersion.ToString() -ne '7.6.9') { throw 'Pinned PowerShell 7.6.9 required' }
$runtimeModules = @('Microsoft.PowerShell.Archive','Microsoft.PowerShell.Host','Microsoft.PowerShell.Management','Microsoft.PowerShell.Security','Microsoft.PowerShell.ThreadJob','Microsoft.PowerShell.Utility')
foreach ($name in $runtimeModules) { Import-Module -Name ([IO.Path]::Combine($PSHOME, 'Modules', $name, "$name.psd1")) -ErrorAction Stop }
if ([string]::Join(',', @(Get-Module | Sort-Object Name | ForEach-Object Name)) -ne [string]::Join(',', $runtimeModules)) { throw 'whole-unit refusal: unexpected runtime baseline' }
# Capture source-less SDK functions before defining collector helpers or loading
# any subject code. Their actual script-block identity must remain unchanged.
$oracleInitialRuntimeFunctions = @{}
foreach ($entry in @(Get-ChildItem Function:)) {
    if ($entry.ScriptBlock -and -not $entry.ScriptBlock.File) {
        $oracleInitialRuntimeFunctions[$entry.Name] = @{
            id = $entry.ScriptBlock.Id
            sha256 = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($entry.Definition))).ToLowerInvariant()
        }
    }
}
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
$dynamicModules = @()
$dynamicModuleHashes = @{}
function Get-OracleModules {
    @((@(Get-Module) + $dynamicModules) | Sort-Object Name | ForEach-Object {
        $hash = if ($_.Path -and [IO.File]::Exists($_.Path)) { (Get-FileHash -LiteralPath $_.Path -Algorithm SHA256).Hash.ToLowerInvariant() } else { $dynamicModuleHashes[$_.GetHashCode()] }
        [pscustomobject][ordered]@{ name = $_.Name; version = $_.Version.ToString(); code_sha256 = $hash }
    } | Sort-Object name,version,code_sha256 -Unique)
}
$PSModuleAutoLoadingPreference = 'None'
$before = @(Get-OracleModules)
$module = Import-Module -Name $manifestPath -PassThru -ErrorAction Stop
if ($module.Count -gt 1) { throw 'whole-unit refusal: ambiguous root module' }
$harnessInputs = [Collections.Generic.List[object]]::new()
$harnessSourceFiles = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
# All repository-local helper declarations come from the owner's parser.
# Keep per-declaration load results; no hand-maintained helper allowlist.
. (Join-Path $PSScriptRoot 'DiscoverHarness.ps1')
$harnessInventory = @()
$excludedNames = @{}
$harnessNames = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
if ($Harness -ne 'none') {
    if (-not $Repository -or -not $TrackedJson) { throw 'whole-unit refusal: missing original repository inventory' }
    $repositoryRoot = [IO.Path]::GetFullPath($Repository).TrimEnd('/')
    $trackedFiles = @(Get-Content -LiteralPath $TrackedJson -Raw | ConvertFrom-Json)
    $harnessInventory = @(Get-OracleHarnessInventory $repositoryRoot $trackedFiles)
    if ($Harness -eq 'psa') {
        if (-not $TestDependencyManifest) { throw 'whole-unit refusal: missing declared Pester dependency' }
        $dependency = Import-Module -Name $TestDependencyManifest -PassThru -ErrorAction Stop
        if ($dependency.Name -ne 'Pester' -or $dependency.Version.ToString() -ne '5.7.1') { throw 'whole-unit refusal: wrong declared test dependency' }
        $harnessInputs.Add([ordered]@{ name = 'Pester'; code_sha256 = (Get-FileHash -LiteralPath $dependency.Path -Algorithm SHA256).Hash.ToLowerInvariant() })
        foreach ($function in @($dependency.ExportedFunctions.Values)) {
            $null = $harnessNames.Add($function.Name)
            if ($function.ScriptBlock.File) { $null = $harnessSourceFiles.Add($function.ScriptBlock.File) }
        }
    }
    foreach ($declaration in $harnessInventory) {
        if ($declaration.owner_block) { continue }
        if ($declaration.cwd_relative) {
            $declaration.status = 'declared_not_loaded'; $declaration.reason = 'relative literal path requires undeclared working directory'; continue
        }
        if (-not [IO.File]::Exists(($declaration.target -split '#')[0])) {
            $declaration.status = 'declared_not_loaded'; $declaration.reason = 'declared target absent from original pinned repository'; continue
        }
        if ($declaration.kind -eq 'dot-source') {
            if (-not (Get-OracleDefinitionOnly $declaration.target)) {
                $declaration.status = 'declared_not_loaded'; $declaration.reason = 'dot-sourced test/fixture has top-level executable statements; no test body executed for binder'; continue
            }
            . $declaration.target
            $null = $harnessSourceFiles.Add($declaration.target)
        } elseif ($declaration.kind -eq 'module') {
            try {
                $provider = Import-Module -Name $declaration.target -DisableNameChecking -PassThru -ErrorAction Stop
                foreach ($function in @($provider.ExportedFunctions.Values)) {
                    $null = $harnessNames.Add($function.Name)
                    if ($function.ScriptBlock.File) { $null = $harnessSourceFiles.Add($function.ScriptBlock.File) }
                }
            } catch {
                $declaration.status = 'declared_not_loaded'
                $declaration.reason = 'native module import refused: ' + $_.Exception.GetType().FullName
                continue
            }
        } else {
            # Original top-level New-Module block; no surrounding test body.
            $provider = New-Module -Name $declaration.name -ScriptBlock $declaration.block.GetScriptBlock()
            $dynamicModules += $provider
            $dynamicModuleHashes[$provider.GetHashCode()] = Get-OracleDeclarationId $declaration.block.Extent.Text
            $provider | Import-Module -DisableNameChecking
            foreach ($function in @($provider.ExportedFunctions.Values)) {
                $null = $harnessNames.Add($function.Name)
                if ($function.ScriptBlock.File) { $null = $harnessSourceFiles.Add($function.ScriptBlock.File) }
            }
        }
        $declaration.status = 'loaded'; $declaration.reason = 'original parser-enumerated declaration loaded in session-wide helper union'
        $harnessInputs.Add([ordered]@{ name = $declaration.name; code_sha256 = $declaration.code_sha256; declaration_id = $declaration.id })
    }
    foreach ($declaration in $harnessInventory | Where-Object owner_block) {
        $owner = @($harnessInventory | Where-Object { $_.block_id -eq $declaration.owner_block })
        if ($owner.Count -ge 1 -and @($owner | Where-Object { $_.status -ne 'loaded' }).Count -eq 0) {
            $declaration.status = 'loaded'; $declaration.reason = 'loaded by original declared module block; not dot-sourced globally'
            $harnessInputs.Add([ordered]@{ name = $declaration.name; code_sha256 = $declaration.code_sha256; declaration_id = $declaration.id })
        } else { $declaration.status = 'declared_not_loaded'; $declaration.reason = 'enclosing module block is not an admitted driver-level declaration' }
    }
    foreach ($declaration in $harnessInventory) {
        if ($declaration.status -eq 'pending') { throw 'whole-unit refusal: unclassified helper declaration' }
        $target = ($declaration.target -split '#')[0]
        if ([IO.File]::Exists($target)) {
            $helperAst = [Management.Automation.Language.Parser]::ParseFile($target, [ref]$null, [ref]$null)
            foreach ($definition in @($helperAst.FindAll({ param($n) $n -is [Management.Automation.Language.FunctionDefinitionAst] }, $true))) {
                $name = Get-OracleBare $definition.Name
                if ($declaration.status -eq 'declared_not_loaded') {
                    if (-not $excludedNames.ContainsKey($name)) { $excludedNames[$name] = @() }
                    $excludedNames[$name] += $declaration
                } elseif ($declaration.kind -eq 'dot-source' -and -not $declaration.owner_block) { $null = $harnessNames.Add($name) }
            }
        }
    }
}
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
$definitionPathCache = @{}
$privateHelperCache = @{}
$sessionUnionNonlocalPrivateSites = 0
$fileHashCache = @{}
$targetFiles = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
$targetIntrinsics = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
$definitionScopes = @{}
$sites = [Collections.Generic.List[object]]::new()
$pairs = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
$calls = [Collections.Generic.List[object]]::new()
$parseErrors = 0
$unknown = 0
$commandSites = 0
$memberSites = 0
$unresolvedClasses = @{ member = 0; harness = 0; declared_not_loaded = 0; same_file_or_BeforeAll = 0; private_module_scope = 0; external_application = 0; found_not_admitted = 0; missing = 0 }
$resolvedHarness = 0
$lookupClasses = @{}
foreach ($relative in $files) {
    $tokens = $null; $errors = $null
    $ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $root $relative), [ref]$tokens, [ref]$errors)
    $unloadedForFile = @{}
    $unloadedModulesForFile = @{}
    $queryPath = [IO.Path]::GetFullPath((Join-Path $root $relative))
    foreach ($name in $excludedNames.Keys) {
        $matches = @($excludedNames[$name] | Where-Object { $_.target -eq $queryPath -or $_.declaring_files.Contains($queryPath) })
        if ($matches.Count) { $unloadedForFile[$name] = $matches }
    }
    foreach ($d in $harnessInventory) {
        if ($d.status -eq 'declared_not_loaded' -and $d.kind -eq 'module' -and $d.declaring_files.Contains($queryPath)) { $unloadedModulesForFile[$d.name] = $d }
    }
    $localFunctions = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($definition in @($ast.FindAll({ param($n) $n -is [Management.Automation.Language.FunctionDefinitionAst] }, $true))) { $null = $localFunctions.Add((Get-OracleBare $definition.Name)) }
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
                $lookupClasses[$cacheKey] = if ($command -is [Management.Automation.ApplicationInfo]) { 'external_application' } elseif ($found.Count) { 'found_not_admitted' } else { 'missing' }
                $allowed = $false
                $isRuntimeIntrinsic = $false
                if ($command -is [Management.Automation.CmdletInfo]) {
                    $allowed = $command.ModuleName -eq 'Microsoft.PowerShell.Core' -or
                        ($command.Module -and ($command.Module.ModuleBase -eq $PSHOME -or $command.Module.ModuleBase.StartsWith($PSHOME + '/', [StringComparison]::Ordinal))) -or
                        ($command.Module -and $command.Module.ModuleBase.StartsWith($moduleRoot, [StringComparison]::Ordinal))
                } elseif ($command -is [Management.Automation.FunctionInfo] -or $command -is [Management.Automation.FilterInfo]) {
                    $source = $command.ScriptBlock.File
                    $isRuntimeIntrinsic = $oracleInitialRuntimeFunctions.ContainsKey($command.Name) -and
                        $command.ScriptBlock.Id -eq $oracleInitialRuntimeFunctions[$command.Name].id
                    $allowed = $isRuntimeIntrinsic -or ($source -and ($source.StartsWith($moduleRoot + '/', [StringComparison]::Ordinal) -or
                        $source.StartsWith($PSHOME + '/', [StringComparison]::Ordinal) -or
                        $source.StartsWith($root + '/', [StringComparison]::Ordinal) -or $harnessSourceFiles.Contains($source)))
                }
                $lookupCache[$cacheKey] = if ($allowed) { Get-OracleBare $command.Name } else { $null }
                $definitionCache[$cacheKey] = $null
                $definitionPathCache[$cacheKey] = $null
                $privateHelperCache[$cacheKey] = $allowed -and ($command -is [Management.Automation.FunctionInfo]) -and -not $module.ExportedFunctions.ContainsKey($command.Name)
                if ($isRuntimeIntrinsic) {
                    $definitionCache[$cacheKey] = @('pinned-runtime', $oracleInitialRuntimeFunctions[$command.Name].sha256, 'runtime-intrinsic')
                } elseif ($allowed) {
                    $definitionPath = if ($command -is [Management.Automation.FunctionInfo]) { $command.ScriptBlock.File } elseif ($command.Module -and $command.Module.Path) { $command.Module.Path } else { $command.ImplementingType.Assembly.Location }
                    if ($definitionPath -and [IO.File]::Exists($definitionPath)) {
                        $definitionPathCache[$cacheKey] = $definitionPath
                        if (-not $fileHashCache.ContainsKey($definitionPath)) { $fileHashCache[$definitionPath] = (Get-FileHash -LiteralPath $definitionPath -Algorithm SHA256).Hash.ToLowerInvariant() }
                        $definitionScope = if (($definitionPath.StartsWith($root + '/', [StringComparison]::Ordinal) -or ($repositoryRoot -and $definitionPath.StartsWith($repositoryRoot + '/', [StringComparison]::Ordinal)))) { 'original-subject' } elseif ($definitionPath.StartsWith($PSHOME + '/', [StringComparison]::Ordinal)) { 'pinned-runtime' } else { 'declared-subject-module' }
                        $definitionCache[$cacheKey] = @($definitionScope, $fileHashCache[$definitionPath])
                    }
                }
            }
            $answer = $lookupCache[$cacheKey]
        }
        $nonlocalPrivateHelper = $false
        if ($isCommand -and $answer -and $scope -eq 'global' -and $privateHelperCache[$cacheKey] -and $definitionPathCache[$cacheKey] -and $moduleSources.Contains($definitionPathCache[$cacheKey])) {
            $declaredHere = @($harnessInventory | Where-Object { $_.status -eq 'loaded' -and ($_.target -eq $definitionPathCache[$cacheKey] -or $_.kind -eq 'module-block') -and $_.declaring_files.Contains((Join-Path $root $relative)) })
            if (-not $declaredHere.Count) { $nonlocalPrivateHelper = $true; $sessionUnionNonlocalPrivateSites++ }
        }
        $unresolvedClass = $null
        if (-not $answer) {
            $unknown++
            $unresolvedClass = if (-not $isCommand) { 'member' } elseif ($lookupClasses[$cacheKey] -eq 'found_not_admitted') { 'found_not_admitted' } elseif ($unloadedForFile.ContainsKey((Get-OracleBare $written)) -or ($written.Contains('\') -and $unloadedModulesForFile.ContainsKey($written.Split('\')[0]))) { 'declared_not_loaded' } elseif ($harnessNames.Contains($written)) { 'harness' } elseif ($lookupClasses[$cacheKey] -eq 'external_application') { 'external_application' } else { 'missing' }
            if ($isCommand -and $unresolvedClass -eq 'missing') {
                if ($localFunctions.Contains((Get-OracleBare $written))) { $unresolvedClass = 'same_file_or_BeforeAll' }
                for ($parent = $node.Parent; $null -ne $parent; $parent = $parent.Parent) {
                    if ($parent -is [Management.Automation.Language.CommandAst] -and ($parent.GetCommandName() -in @('InModuleScope','InPesterModuleScope') -or ($parent.InvocationOperator -eq 'Ampersand' -and $parent.CommandElements[0] -is [Management.Automation.Language.VariableExpressionAst] -and $parent.CommandElements[0].VariablePath.UserPath -match '(?i)module$'))) { $unresolvedClass = 'private_module_scope'; break }
                }
            }
            $unresolvedClasses[$unresolvedClass]++
            if ($unresolvedClass -eq 'declared_not_loaded') {
                if ($unloadedForFile.ContainsKey((Get-OracleBare $written))) { foreach ($d in $unloadedForFile[(Get-OracleBare $written)]) { $d.site_count++ } }
                elseif ($written.Contains('\') -and $unloadedModulesForFile.ContainsKey($written.Split('\')[0])) { $unloadedModulesForFile[$written.Split('\')[0]].site_count++ }
            }
        } elseif ($harnessNames.Contains($written)) { $resolvedHarness++ }
        $definition = if ($isCommand -and $answer) { $definitionCache[$cacheKey] } else { $null }
        if ($definition) {
            if ($definition.Count -eq 3 -and $definition[2] -eq 'runtime-intrinsic') { $null = $targetIntrinsics.Add($definition[1]) }
            else { $null = $targetFiles.Add($definition[0] + ':' + $definition[1]) }
            $definitionScopes[$definition[0]]++
        }
        $sites.Add([ordered]@{ path = $relative; line = $node.Extent.StartLineNumber; column = $node.Extent.StartColumnNumber;
            from = $caller; written = $written; kind = $(if ($isCommand) { 'command' } else { 'member' }); resolved = $answer; session_union_nonlocal_private_helper = $nonlocalPrivateHelper; resolved_definition = $definition; unresolved_class = $unresolvedClass })
        if ($sites.Count -gt $siteCap) { throw 'whole-unit refusal: site count cap' }
        if ($answer) {
            $key = "$relative`t$caller`t$answer"
            if ($pairs.Add($key)) { $calls.Add([ordered]@{ path = $relative; from = $caller; to = $answer }) }
        }
    }
}
$declarationCount = 0
foreach ($d in $harnessInventory) { $declarationCount += $d.declaration_count }
$receipt = [ordered]@{
    tool = 'PowerShell native InvokeCommand.GetCommand in declared module session'; version = '7.6.9'; tier = 'A';
    calls = $calls.ToArray(); sites = $sites.ToArray();
    harness_inventory = @($harnessInventory | ForEach-Object { [ordered]@{ id = $_.id; target = $_.target; kind = $_.kind; status = $_.status; reason = $_.reason; code_sha256 = $_.code_sha256; declaration_count = $_.declaration_count; declaring_files = @($_.declaring_files); site_count = $_.site_count } });
    receipt = [ordered]@{ files = $files.Count; bytes = $bytes; literal_sites = $sites.Count; command_sites = $commandSites;
        member_sites = $memberSites; native_module_source_files = $moduleSources.Count; module_context_sites = $moduleContextSites; global_context_sites = $globalContextSites; unresolved_sites = $unknown; parse_error_files = $parseErrors;
        resolved_definition_file_contents = $targetFiles.Count; resolved_definition_scope_sites = $definitionScopes;
        resolved_intrinsic_definition_contents = $targetIntrinsics.Count;
        initial_runtime_intrinsic_functions = $oracleInitialRuntimeFunctions.Count;
        initial_runtime_intrinsic_functions_sha256 = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes([string]::Join("`n", @($oracleInitialRuntimeFunctions.Keys | Sort-Object | ForEach-Object { $_ + ':' + $oracleInitialRuntimeFunctions[$_].sha256 }))))).ToLowerInvariant();
        resolved_definition_sources_sha256 = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes([string]::Join("`n", @($targetFiles | Sort-Object -CaseSensitive))))).ToLowerInvariant();
        subject_module = $module.Name; subject_module_version = $module.Version.ToString();
        manifest_sha256 = (Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash.ToLowerInvariant();
        load_identity_policy = 'unique module name/version/code hash, not instance counts; every declared block retains a separate declaration id';
        load_before = $before; load_after = @(Get-OracleModules); explicit_runtime_modules = $runtimeModules;
        session_union_nonlocal_private_helper_sites = $sessionUnionNonlocalPrivateSites;
        declared_harness_inputs = $harnessInputs.ToArray();
        declared_inventory = [ordered]@{ targets = $harnessInventory.Count; declarations = $declarationCount; loaded = @($harnessInventory | Where-Object status -eq 'loaded').Count; declared_not_loaded = @($harnessInventory | Where-Object status -eq 'declared_not_loaded' | ForEach-Object { [ordered]@{ declaration_id = $_.id; kind = $_.kind; reason = $_.reason; site_count = $_.site_count; code_sha256 = $_.code_sha256 } }); policy = 'native parser enumeration of all tracked test files and root loader; loaded plus named declaration-id refusals partition inventory; helper union is session-wide, including private product utility definitions' };  resolved_harness_sites = $resolvedHarness; unresolved_by_class = $unresolvedClasses;
        unresolvable_policy = 'all literal members, undeclared external commands/state and missing functions remain misses; no dot-sourcing of fixtures; native loader-owned source files use module context, ordinary test files use global context with session-wide declared-helper union, including per-file private utility preambles';
        module_loading_policy = 'autoload disabled before preamble; explicit six-module pinned-runtime baseline asserted; matching subject manifest and original declared test harness only; profile disabled' }
}
$receipt | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $Out -Encoding utf8NoBOM
