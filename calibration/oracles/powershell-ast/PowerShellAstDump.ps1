#!/usr/bin/env pwsh
<#
  PowerShellAstDump - tier-B PowerShell L1/L2 oracle (syntax only).

  ADR-048 amendment D2/D5: universal-ctags 6.2.1's PowerShell parser reports
  no end line for any tag, no class method, no constructor and no property,
  and names a `function global:x` by its bare suffix only; PowerShell has no
  SCIP producer and its grammar ships no queries/tags.scm. The language's own
  reference parser, System.Management.Automation.Language.Parser, is the
  stronger independent oracle: this script walks its AST and writes the
  neutral oracle file `indexbench lang-oracle -tool-oracle` reads.
  Bench-only: a PowerShell process run beside the harness, never a product
  dependency. Nothing is executed; ParseFile only parses.

  Definitions (kind): FunctionDefinitionAst (function; filters and workflows
  are the same node; the body wrapper the parser synthesizes for a class
  method is not one), TypeDefinitionAst (class, enum), FunctionMemberAst
  (method, constructor), PropertyMemberAst (property in a class, enumMember
  in an enum). Calls: (enclosing FunctionDefinitionAst or FunctionMemberAst,
  callee) for every CommandAst whose command name is a literal
  (GetCommandName() - `& $dynamic` has none) and every InvokeMemberExpressionAst
  whose member is a literal name (instance and static). Names are bare (the
  text after the last . : # / \ space or ) - design 1.4's BareName) so the
  harness's last-segment rule sees the same string on both sides. Top-level
  calls (no enclosing callable) are not pairs, unless -ModuleCaller names the
  module symbol they are paired with (K18). A file the parser reports
  errors on is still walked (the AST is error-recovering) and counted on
  stderr.

  usage: pwsh -File PowerShellAstDump.ps1 <subject-dir> [-ModuleCaller '<module>'] [subject-relative files...]
         default files: every *.ps1 *.psm1 *.psd1 under the subject
#>
param(
  [Parameter(Mandatory, Position = 0)][string]$Subject,
  # -ModuleCaller <name> (wave lf6, K18): pair a call no function or method encloses with this
  # caller (the dialect's module symbol, `<module>`) instead of leaving it out.
  [string]$ModuleCaller = '',
  [Parameter(ValueFromRemainingArguments)][string[]]$Files
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Management.Automation | Out-Null
$root = (Resolve-Path -LiteralPath $Subject).Path
if (-not $Files -or $Files.Count -eq 0) {
  # -Force: Get-ChildItem hides dot-prefixed paths (.github/) on Unix; .git stays out.
  $Files = Get-ChildItem -LiteralPath $root -Recurse -File -Force |
    Where-Object { $_.Extension -in '.ps1', '.psm1', '.psd1' -and $_.FullName -notmatch '[\\/]\.git[\\/]' } |
    ForEach-Object { [System.IO.Path]::GetRelativePath($root, $_.FullName).Replace('\', '/') } |
    Sort-Object -Property { $_ } -CaseSensitive
}

function Get-Bare([string]$name) {
  $i = $name.LastIndexOfAny([char[]]'.:#/\ )')
  if ($i -ge 0) { return $name.Substring($i + 1) }
  return $name
}

# The parser wraps every class method's body in a FunctionDefinitionAst whose
# parent is the FunctionMemberAst: that node is the method itself, not a
# function statement, so it is neither a definition nor an enclosing callable.
function Test-SyntheticFunction($n) {
  return $n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
    $n.Parent -is [System.Management.Automation.Language.FunctionMemberAst]
}

function Get-EnclosingCallable($ast) {
  for ($p = $ast.Parent; $null -ne $p; $p = $p.Parent) {
    if ($p -is [System.Management.Automation.Language.FunctionDefinitionAst] -and -not (Test-SyntheticFunction $p)) { return $p.Name }
    if ($p -is [System.Management.Automation.Language.FunctionMemberAst]) { return $p.Name }
  }
  if ($ModuleCaller) { return $ModuleCaller }
  return $null
}

$defs = [System.Collections.Generic.List[object]]::new()
$calls = [System.Collections.Generic.HashSet[string]]::new()
$callList = [System.Collections.Generic.List[object]]::new()
$bad = 0
foreach ($rel in $Files) {
  $path = Join-Path $root $rel
  $tokens = $null; $errors = $null
  $ast = [System.Management.Automation.Language.Parser]::ParseFile($path, [ref]$tokens, [ref]$errors)
  if ($errors -and $errors.Count -gt 0) { $bad++ }
  $nodes = $ast.FindAll({ param($n) $true }, $true)
  foreach ($n in $nodes) {
    switch ($n.GetType().Name) {
      'FunctionDefinitionAst' {
        if (Test-SyntheticFunction $n) { break }
        $defs.Add([ordered]@{ path = $rel; name = Get-Bare $n.Name; line = $n.Extent.StartLineNumber; end = $n.Extent.EndLineNumber; kind = 'function' })
      }
      'TypeDefinitionAst' {
        $k = if ($n.IsEnum) { 'enum' } elseif ($n.IsInterface) { 'interface' } else { 'class' }
        $defs.Add([ordered]@{ path = $rel; name = Get-Bare $n.Name; line = $n.Extent.StartLineNumber; end = $n.Extent.EndLineNumber; kind = $k })
      }
      'FunctionMemberAst' {
        $k = if ($n.IsConstructor) { 'constructor' } else { 'method' }
        $defs.Add([ordered]@{ path = $rel; name = Get-Bare $n.Name; line = $n.Extent.StartLineNumber; end = $n.Extent.EndLineNumber; kind = $k })
      }
      'PropertyMemberAst' {
        $k = if ($n.Parent.IsEnum) { 'enumMember' } else { 'property' }
        $defs.Add([ordered]@{ path = $rel; name = Get-Bare $n.Name; line = $n.Extent.StartLineNumber; end = $n.Extent.EndLineNumber; kind = $k })
      }
      'CommandAst' {
        $callee = $n.GetCommandName()
        if ($callee) {
          $from = Get-EnclosingCallable $n
          if ($from) {
            $key = "$rel`t$(Get-Bare $from)`t$(Get-Bare $callee)"
            if ($calls.Add($key)) { $callList.Add([ordered]@{ path = $rel; from = Get-Bare $from; to = Get-Bare $callee }) }
          }
        }
      }
      'InvokeMemberExpressionAst' {
        if ($n.Member -is [System.Management.Automation.Language.StringConstantExpressionAst]) {
          $from = Get-EnclosingCallable $n
          if ($from) {
            $key = "$rel`t$(Get-Bare $from)`t$(Get-Bare $n.Member.Value)"
            if ($calls.Add($key)) { $callList.Add([ordered]@{ path = $rel; from = Get-Bare $from; to = Get-Bare $n.Member.Value }) }
          }
        }
      }
    }
  }
}
[Console]::Error.WriteLine("powershell-ast: $($Files.Count) files, $($defs.Count) definitions, $($callList.Count) call pairs, $bad files with parse errors")
[ordered]@{
  tool        = 'powershell-ast'
  version     = "PowerShell $($PSVersionTable.PSVersion) System.Management.Automation.Language.Parser"
  tier        = 'B'
  definitions = $defs.ToArray()
  calls       = $callList.ToArray()
} | ConvertTo-Json -Depth 6
