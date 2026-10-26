Import-Module (Join-Path $PSScriptRoot 'module/Probe.psd1')
Write-Output 'native-baseline-fixture' | Out-Null
# Script-qualified command breakpoints in 7.6.9 fail on this owner operation.
$null = New-Module -Name NativeEphemeral -ScriptBlock { function Ephemeral { 1 } }
function NativeDriver { nativeAlias; PublicEntry }
function DeclaredButUnloaded { PrivateTarget; git; Install-Module; MissingCommand; Get-OracleBare; Clear-Host; [Math]::Abs(-1) }
NativeDriver
