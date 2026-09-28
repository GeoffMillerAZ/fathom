# functions.ps1 - function shapes the dialect must name, sign and call-scan.

function Get-Greeting {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Name,
        [int]$Count = 1
    )
    Write-Output "Hello $Name"
    Get-Other -Name $Name
    [Console]::WriteLine('done')
    $Name.ToUpper()
}

function Get-Other([string]$Name) { Write-Host $Name }

filter Select-Big { if ($_ -gt 10) { $_ } }

workflow Invoke-Flow { Get-Greeting }

function Empty {}

function Outer {
    function Inner { Write-Verbose 'inner' }
    Inner
}

function Global:Scoped-Name
{
    param($x)
    $x | ForEach-Object { Get-Other -Name $_ }
}
