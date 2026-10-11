#!/usr/bin/env pwsh
<#
  PowerShellImportDump - tier-A PowerShell L3 oracle (LB-Q-powershell, wave lf6): the import
  statements PowerShell's own parser attests and the files PowerShell's own rules resolve them to,
  as the neutral `imports` `indexbench lang-oracle -tool-oracle` reads (LB-F40, LB-F57).
  Bench-only; nothing in the subject is executed (ParseFile only parses; Test-ModuleManifest and
  Import-PowerShellDataFile read a manifest's data in restricted language mode).

  Statements, each from the AST:
  - dot-sourcing: a CommandAst whose InvocationOperator is Dot (`. path`);
  - Import-Module (or its alias ipmo): the path StaticParameterBinder binds to -Name (positional
    or named);
  - `using module path`: a UsingStatementAst of kind Module;
  - a module manifest (.psd1): its RootModule (or ModuleToProcess) and each NestedModules entry,
    the string literals of the manifest's top-level hashtable.

  Path bases, from PowerShell's semantics (about_Scripts, about_Automatic_Variables,
  about_Using, about_Module_Manifests):
  - `$PSScriptRoot` is the directory of the script, module or manifest being run, so a path that
    opens with it (an expandable string whose only expansion is $PSScriptRoot, at its start)
    resolves against the file's own directory;
  - `using module` resolves a relative path against the directory of the script holding it;
  - a manifest's RootModule and NestedModules resolve against the module base, the manifest's own
    directory (Test-ModuleManifest attests it: its PSModuleInfo names the files);
  - a dot-sourced or Import-Module path with no $PSScriptRoot resolves against the current
    location at run time, which no static reading knows: such a statement is counted
    (cwd_relative) and written to no truth. A bare module name with no path or extension is
    PSModulePath's (by_name), and any other expansion, a parenthesized expression (Join-Path) or
    a variable is dynamic; each is counted, never resolved.

  A path that names a file of the subject is written (subject-relative, `/`); a target outside
  it, or naming no file, is counted. With no extension, PowerShell's own order is tried (a
  directory's <leaf>.psd1 then <leaf>.psm1; a file name plus .psd1, .psm1, .ps1, .dll).

  usage: pwsh -NoLogo -NoProfile -File PowerShellImportDump.ps1 <subject-dir> [subject-relative files...]
         default files: every *.ps1 *.psm1 *.psd1 under the subject (dot-prefixed directories included)
#>
param(
  [Parameter(Mandatory, Position = 0)][string]$Subject,
  [Parameter(ValueFromRemainingArguments)][string[]]$Files
)
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path -LiteralPath $Subject).Path
if (-not $Files -or $Files.Count -eq 0) {
  $Files = Get-ChildItem -LiteralPath $root -Recurse -File -Force |
    Where-Object { $_.Extension -in '.ps1', '.psm1', '.psd1' -and $_.FullName -notmatch '[\\/]\.git[\\/]' } |
    ForEach-Object { [System.IO.Path]::GetRelativePath($root, $_.FullName).Replace('\', '/') } |
    Sort-Object -Property { $_ } -CaseSensitive
}
$L = [System.Management.Automation.Language.Ast]
$counts = [ordered]@{
  files = 0; files_with_parse_errors = 0
  dot_source = 0; import_module = 0; using_module = 0; manifest_module = 0
  in_subject = 0; outside_or_missing = 0
  cwd_relative = 0; by_name = 0; dynamic = 0
  manifest_validated = 0; manifest_data_only = 0
}
$imports = [System.Collections.Generic.List[object]]::new()

# Resolve a path PowerShell names to a regular file of the subject, its own order for a path with
# no extension; returns $null outside the subject or when nothing exists.
function Resolve-SubjectFile([string]$full) {
  $full = [System.IO.Path]::GetFullPath($full.Replace('\', '/'))
  $cands = @($full)
  if (Test-Path -LiteralPath $full -PathType Container) {
    $leaf = Split-Path -Leaf $full
    $cands = @((Join-Path $full "$leaf.psd1"), (Join-Path $full "$leaf.psm1"))
  } elseif ([System.IO.Path]::GetExtension($full) -eq '') {
    $cands = @($full, "$full.psd1", "$full.psm1", "$full.ps1", "$full.dll")
  }
  foreach ($c in $cands) {
    if (Test-Path -LiteralPath $c -PathType Leaf) {
      # The name as stored on disk (a case-insensitive file system answers any case).
      $dir = Split-Path -Parent $c
      $name = (Get-ChildItem -LiteralPath $dir -File -Force | Where-Object { $_.Name -ieq (Split-Path -Leaf $c) } | Select-Object -First 1).Name
      $real = Join-Path $dir $name
      $rel = [System.IO.Path]::GetRelativePath($root, $real)
      if ($rel.StartsWith('..') -or [System.IO.Path]::IsPathRooted($rel)) { return $null }
      return $rel.Replace('\', '/')
    }
  }
  return $null
}

# The path an argument AST names: @{ kind = 'scriptroot' | 'cwd' | 'name' | 'dynamic'; path }.
function Get-PathOf($ast) {
  if ($ast -is [System.Management.Automation.Language.StringConstantExpressionAst]) {
    $v = $ast.Value
    if ($v -match '[\\/]' -or $v -match '\.(ps1|psm1|psd1|dll)$') { return @{ kind = 'cwd'; path = $v } }
    return @{ kind = 'name'; path = $v }
  }
  if ($ast -is [System.Management.Automation.Language.ExpandableStringExpressionAst]) {
    $nested = @($ast.NestedExpressions)
    if ($nested.Count -eq 1 -and $nested[0] -is [System.Management.Automation.Language.VariableExpressionAst] -and
        $nested[0].VariablePath.UserPath -ieq 'PSScriptRoot') {
      $text = $ast.Value
      foreach ($form in '$PSScriptRoot', '${PSScriptRoot}') {
        if ($text.Length -gt $form.Length -and $text.Substring(0, $form.Length) -ieq $form -and $text[$form.Length] -in '/', '\') {
          return @{ kind = 'scriptroot'; path = $text.Substring($form.Length + 1) }
        }
      }
    }
  }
  return @{ kind = 'dynamic'; path = '' }
}

function Add-Import([string]$rel, [int]$line, [string]$base, [string]$path, [string]$module) {
  $to = Resolve-SubjectFile (Join-Path $base $path)
  if ($null -eq $to) { $script:counts.outside_or_missing++; return }
  $script:counts.in_subject++
  $script:imports.Add([ordered]@{ path = $rel; line = $line; to_path = $to; module = $module })
}

function Add-Statement([string]$rel, [string]$dir, $ast, [int]$line, [string]$module, [bool]$scriptRelative) {
  $p = Get-PathOf $ast
  switch ($p.kind) {
    'scriptroot' { Add-Import $rel $line $dir $p.path $module }
    'cwd' { if ($scriptRelative) { Add-Import $rel $line $dir $p.path $module } else { $script:counts.cwd_relative++ } }
    'name' { if ($scriptRelative) { Add-Import $rel $line $dir $p.path $module } else { $script:counts.by_name++ } }
    default { $script:counts.dynamic++ }
  }
}

foreach ($rel in $Files) {
  $full = Join-Path $root $rel
  $dir = Split-Path -Parent $full
  $tokens = $null; $errors = $null
  $ast = [System.Management.Automation.Language.Parser]::ParseFile($full, [ref]$tokens, [ref]$errors)
  $counts.files++
  if ($errors -and $errors.Count -gt 0) { $counts.files_with_parse_errors++ }
  foreach ($n in $ast.FindAll({ param($x) $x -is [System.Management.Automation.Language.CommandAst] -or $x -is [System.Management.Automation.Language.UsingStatementAst] }, $true)) {
    $line = $n.Extent.StartLineNumber
    if ($n -is [System.Management.Automation.Language.UsingStatementAst]) {
      if ($n.UsingStatementKind -ne 'Module') { continue }
      $counts.using_module++
      if ($null -eq $n.Name) { $counts.dynamic++; continue } # using module @{ModuleName=...}
      Add-Statement $rel $dir $n.Name $line $n.Name.Extent.Text $true
      continue
    }
    if ($n.InvocationOperator -eq 'Dot') {
      $counts.dot_source++
      Add-Statement $rel $dir $n.CommandElements[0] $line $n.CommandElements[0].Extent.Text $false
      continue
    }
    if ($n.GetCommandName() -in 'Import-Module', 'ipmo') {
      $counts.import_module++
      $bound = [System.Management.Automation.Language.StaticParameterBinder]::BindCommand($n, $true)
      $name = $bound.BoundParameters['Name']
      if ($null -eq $name) { $counts.dynamic++; continue } # -ModuleInfo, -Assembly, -FullyQualifiedName
      Add-Statement $rel $dir $name.Value $line $name.Value.Extent.Text $false
    }
  }
  if ($rel -match '\.psd1$') {
    # The manifest's own top-level hashtable: RootModule / ModuleToProcess and NestedModules.
    $ht = $ast.Find({ param($x) $x -is [System.Management.Automation.Language.HashtableAst] }, $false)
    if ($null -eq $ht) { continue }
    $entries = @()
    foreach ($kv in $ht.KeyValuePairs) {
      $key = "$($kv.Item1.Extent.Text)".Trim("'", '"')
      if ($key -notin 'RootModule', 'ModuleToProcess', 'NestedModules') { continue }
      foreach ($s in $kv.Item2.FindAll({ param($x) $x -is [System.Management.Automation.Language.StringConstantExpressionAst] }, $true)) {
        $entries += , @($s.Value, $s.Extent.StartLineNumber)
      }
    }
    if ($entries.Count -eq 0) { continue }
    # Test-ModuleManifest attests the module base and the files; a manifest it refuses (a
    # RequiredModules entry this machine lacks) is read by Import-PowerShellDataFile alone.
    $validated = $false
    try { $null = Test-ModuleManifest -Path $full -ErrorAction Stop -WarningAction SilentlyContinue; $validated = $true } catch { }
    foreach ($e in $entries) {
      $counts.manifest_module++
      if ($validated) { $counts.manifest_validated++ } else { $counts.manifest_data_only++ }
      Add-Import $rel $e[1] $dir $e[0] $e[0]
    }
  }
}
# Ordinal order (path, line, file), as the harness's other dumps sort.
$imports.Sort([System.Comparison[object]] {
    param($a, $b)
    $c = [string]::CompareOrdinal($a.path, $b.path); if ($c -ne 0) { return $c }
    $c = $a.line.CompareTo($b.line); if ($c -ne 0) { return $c }
    return [string]::CompareOrdinal($a.to_path, $b.to_path)
  })
$summary = ($counts.GetEnumerator() | ForEach-Object { "$($_.Key) $($_.Value)" }) -join ' '
[Console]::Error.WriteLine("powershell-imports: $summary")
$c = $counts
$disclose = @(
  ("powershell-ast imports (tier A, PowerShell $($PSVersionTable.PSVersion)): $($c.dot_source) dot-sourcings, $($c.import_module) Import-Module, " +
   "$($c.using_module) using module and $($c.manifest_module) manifest RootModule/NestedModules entries in $($c.files) files " +
   "($($c.files_with_parse_errors) with parse errors); $($c.in_subject) resolve to a subject file and $($c.outside_or_missing) name a file outside it or none; " +
   "$($c.cwd_relative) paths resolve against the current location, which no static reading knows, $($c.by_name) are module names PSModulePath resolves " +
   "and $($c.dynamic) are dynamic (a variable, Join-Path, a hashtable), each counted and written to no truth; " +
   "$($c.manifest_validated) manifest entries Test-ModuleManifest validated, $($c.manifest_data_only) read by Import-PowerShellDataFile's rules alone")
)
[ordered]@{
  tool     = 'powershell-ast-imports'
  version  = "PowerShell $($PSVersionTable.PSVersion) System.Management.Automation.Language.Parser, StaticParameterBinder, Test-ModuleManifest"
  tier     = 'A'
  disclose = $disclose
  imports  = @($imports)
} | ConvertTo-Json -Depth 6
