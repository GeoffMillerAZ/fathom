param([Parameter(Mandatory)][string]$Out)
$ErrorActionPreference = 'Stop'
if ($PSVersionTable.PSVersion.ToString() -ne '7.6.9') { throw 'Pinned runtime required' }
$null = New-Item -ItemType Directory -Path $Out -Force
function NativeContractTarget { 1 }
function NativeContractDriver { NativeContractTarget }
Set-Alias NativeContractAlias NativeContractTarget
$providers = @((Get-TraceSource).Name | Sort-Object)
$trace = Join-Path $Out 'command-discovery.log'
Trace-Command -Name CommandDiscovery -Expression { NativeContractDriver; NativeContractAlias; Write-Output 1 } -FilePath $trace | Out-Null
$memberTrace = Join-Path $Out 'member-resolution.log'
Trace-Command -Name MemberResolution -Option All -Expression { $instance = 'x'; $method = 'ToUpperInvariant'; [Math]::Abs(-1); $instance.$method(); $instance.PSObject.Methods[$method].Invoke() } -FilePath $memberTrace | Out-Null
$nativeMember = @([Math].GetMember('Abs', [Reflection.MemberTypes]::Method, [Reflection.BindingFlags]'Public,Static'))
Set-PSDebug -Trace 2
NativeContractDriver | Out-Null
Set-PSDebug -Off
$receipt = [ordered]@{
    pwsh = '7.6.9'; command_discovery_provider = 'CommandDiscovery' -in $providers;
    provider_count = $providers.Count;
    member_related_providers = @($providers | Where-Object { $_ -match 'Member|Adapter|Type' });
    native_lookup_events_emitted = [IO.File]::ReadAllText($trace).Contains('NativeContract');
    member_resolution_events_emitted = [IO.File]::ReadAllText($memberTrace).Length -gt 0;
    native_static_reflection_overloads = $nativeMember.Count;
    native_static_reflection_method_names = @($nativeMember.Name | Sort-Object -Unique);
    member_trace_sha256 = (Get-FileHash -LiteralPath $memberTrace -Algorithm SHA256).Hash.ToLowerInvariant();
    trace_sha256 = (Get-FileHash -LiteralPath $trace -Algorithm SHA256).Hash.ToLowerInvariant();
    limitation = 'CommandDiscovery emits lookup/selection text, without a structured caller-file/line/column reference table. Set-PSDebug emits execution/variable diagnostics, not independently resolved targets. Neither alone pairs full written references.';
}
$receipt | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $Out 'contracts.json') -Encoding utf8NoBOM
