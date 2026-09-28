# broken.ps1 - a syntax error the parser must survive; definitions around it still come out.
function Before-Break {
    Write-Host 'ok'
}

function Broken-One {
    param([string]$a, @#$)
    Write-Host (
}

function After-Break {
    Get-Item .
}
