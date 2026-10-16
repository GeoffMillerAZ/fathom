# imports.ps1 - dot-sourcing, Import-Module and using module with a path (LB-Q-powershell L3).
using module ./Shared.psm1
. $PSScriptRoot\Private\Helpers.ps1
. "$PSScriptRoot/Public/Get-Thing.ps1"
. .\Local.ps1
Import-Module $PSScriptRoot\..\bin\Tool.psd1 -Force
Import-Module -Name "$PSScriptRoot/Nested/Inner.psm1"
ipmo ./Other.psm1
Import-Module PSScriptAnalyzer
Import-Module (Join-Path $PSScriptRoot 'Joined.psm1')
. (Join-Path $PSScriptRoot 'joined.ps1')
& $PSScriptRoot\Run.ps1
Get-ChildItem $PSScriptRoot\Public -Filter *.ps1 | ForEach-Object { . $_.FullName }
function Initialize-Thing {
    . $PSScriptRoot\Inner\Late.ps1
}
