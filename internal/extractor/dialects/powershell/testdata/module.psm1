# module.psm1 - a module file: script-scope state, functions calling each other, exports.
using namespace System.Collections.Generic

$script:Cache = @{}

function Get-CachedValue {
    [CmdletBinding()]
    param([string]$Key)
    if (-not $script:Cache.ContainsKey($Key)) {
        $script:Cache[$Key] = Resolve-Value -Key $Key
    }
    return $script:Cache[$Key]
}

function Resolve-Value {
    param([string]$Key)
    $list = [List[string]]::new()
    $list.Add($Key)
    & (Join-Path $PSScriptRoot 'helper.ps1') $Key
    . "$PSScriptRoot\common.ps1"
}

Export-ModuleMember -Function Get-CachedValue
