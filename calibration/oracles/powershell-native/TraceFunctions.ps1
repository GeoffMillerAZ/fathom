param(
    [Parameter(Mandatory)][string]$Subject,
    [Parameter(Mandatory)][string]$FilesJson,
    [Parameter(Mandatory)][string]$Out,
    [Parameter(Mandatory)][ValidateSet('pester','psa','fixture')][string]$Driver,
    [string]$PesterManifest,
    [switch]$SkipPTests,
    [string]$AnalyzerManifest
)
$ErrorActionPreference = 'Stop'
if ($PSVersionTable.PSVersion.ToString() -ne '7.6.9') { throw 'Pinned PowerShell 7.6.9 required' }
$global:OracleRoot = [IO.Path]::GetFullPath($Subject).TrimEnd('/')
$files = @(Get-Content -LiteralPath $FilesJson -Raw | ConvertFrom-Json | ForEach-Object { Join-Path $global:OracleRoot $_ })
if (-not $files.Count -or $files.Count -gt 2048) { throw 'whole-unit refusal: trace file count cap or empty scope' }
$global:OracleRows = [Collections.Generic.Dictionary[string,object]]::new([StringComparer]::Ordinal)
$global:OracleNativeEvents = 0L
$global:OracleRefused = $false
$global:OracleRowCap = if ($env:PWSH_TRACE_ROW_CAP) { [int]$env:PWSH_TRACE_ROW_CAP } else { 1000000 }
if ($global:OracleRowCap -lt 1 -or $global:OracleRowCap -gt 1000000) { throw 'whole-unit refusal: invalid native row cap' }
$global:OracleStackCmdlet = Get-Command Microsoft.PowerShell.Utility\Get-PSCallStack -CommandType Cmdlet
$action = {
    $global:OracleNativeEvents++
    if ($global:OracleRefused) { return }
    $frames = @(& $global:OracleStackCmdlet)
    if ($frames.Count -lt 3) { return }
    $invocation = $frames[1].InvocationInfo
    $command = $invocation.MyCommand
    if ($command -isnot [Management.Automation.FunctionInfo] -and $command -isnot [Management.Automation.FilterInfo]) { return }
    $source = $invocation.ScriptName
    if (-not $source -or -not $source.StartsWith($global:OracleRoot + '/', [StringComparison]::Ordinal)) { return }
    $relative = [IO.Path]::GetRelativePath($global:OracleRoot, $source).Replace('\','/')
    $caller = $frames[2].FunctionName
    if ($caller -eq '<ScriptBlock>') { $caller = '<module>' }
    $row = @($relative, [int]$invocation.ScriptLineNumber, [int]$invocation.OffsetInLine, $caller, $command.Name, $invocation.InvocationName)
    $key = [string]::Join("`t", $row)
    $global:OracleRows[$key] = $row
    if ($global:OracleRows.Count -gt $global:OracleRowCap) { $global:OracleRefused = $true; $global:OracleRows.Clear() }
}
# Script-qualified command breakpoints in pwsh 7.6.9 dereference a null
# ScriptName during New-Module. A global breakpoint avoids that owner bug;
# native source positions are filtered to the subject in the action and to
# exact original tracked sites in the decoder. No original site is removed.
$breakpoints = @(Set-PSBreakpoint -Command '*' -Action $action)
$status = 'completed'
$exceptionHash = $null
try {
    switch ($Driver) {
        'pester' { & (Join-Path $Subject 'test.ps1') -NoBuild -SkipPTests:$SkipPTests }
        'psa' {
            Import-Module -Name $PesterManifest -ErrorAction Stop
            Import-Module -Name $AnalyzerManifest -ErrorAction Stop
            $paths = @('Build','Engine','Rules','Documentation') | ForEach-Object { Join-Path $Subject $_ }
            Invoke-Pester -Path $paths -CI
        }
        'fixture' { & (Join-Path $Subject 'native.ps1') }
    }
} catch {
    [Console]::Error.WriteLine($_.Exception.ToString())
    [Console]::Error.WriteLine($_.ScriptStackTrace)
    $status = 'driver-exception'
    $exceptionHash = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($_.ToString()))).ToLowerInvariant()
} finally {
    $remaining = @(Get-PSBreakpoint)
    $retained = @($remaining | Where-Object { $_.Id -in $breakpoints.Id }).Count
    $breakpoints | Remove-PSBreakpoint -ErrorAction SilentlyContinue
    $receipt = [ordered]@{
        pwsh = '7.6.9'; scope = 'observed native function frames only, current process; cmdlets/members and child processes remain unattested';
        refused = $global:OracleRefused; native_events = $global:OracleNativeEvents;
        driver_status = $status; exception_sha256 = $exceptionHash;
        original_breakpoints = $breakpoints.Count; retained_breakpoints = $retained;
        calls = @($global:OracleRows.GetEnumerator() | Sort-Object Key | ForEach-Object { ,$_.Value });
    }
    $receipt | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $Out -Encoding utf8NoBOM
}
