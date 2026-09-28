# calls.ps1 - call shapes: a member call in argument position, a command run with
# & or dot-sourcing, and the tokens the grammar misreads as commands.

function Use-Arguments {
    $type = Format-Type $Value.GetType()
    $list.ToArray() | Out-Null
    & Get-ChildItem | Sort-Object
    . Import-Helper -Path $p
    & $dynamic arg
    & $context.Runner -Scope A
    $color = if ($ok) {
        "Red"
    }
    else {
        "Blue"
    }
    try {
        Start-Work
    }
    catch {
        Write-Error $_
    }
    foreach ($i in 1..$items.Count) { Write-Host $i }
}
