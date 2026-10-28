# Native AST inventory; never evaluate path expressions from subject source.
function Get-OraclePathValue($Node, [string]$Directory) {
    if ($Node -is [Management.Automation.Language.StringConstantExpressionAst]) { return $Node.Value }
    if ($Node -is [Management.Automation.Language.VariableExpressionAst]) { return $oraclePathBindings[$Node.VariablePath.UserPath] }
    if ($Node -is [Management.Automation.Language.ExpandableStringExpressionAst]) {
        foreach ($nested in $Node.NestedExpressions) {
            if ($nested -isnot [Management.Automation.Language.VariableExpressionAst] -or -not $oraclePathBindings.ContainsKey($nested.VariablePath.UserPath)) { return $null }
        }
        return [regex]::Replace($Node.Value, '\$(?:\{(?<name>\w+)\}|(?<name>\w+))', [Text.RegularExpressions.MatchEvaluator]{ param($m) $oraclePathBindings[$m.Groups['name'].Value] })
    }
    if ($Node -is [Management.Automation.Language.ParenExpressionAst]) { return Get-OraclePathValue $Node.Pipeline $Directory }
    if ($Node -is [Management.Automation.Language.PipelineAst]) {
        if ($Node.PipelineElements.Count -eq 1) { return Get-OraclePathValue $Node.PipelineElements[0] $Directory }
    }
    if ($Node -is [Management.Automation.Language.CommandExpressionAst]) { return Get-OraclePathValue $Node.Expression $Directory }
    if ($Node -is [Management.Automation.Language.CommandAst]) {
        $values = @($Node.CommandElements | Select-Object -Skip 1 | Where-Object { $_ -isnot [Management.Automation.Language.CommandParameterAst] })
        if ($Node.GetCommandName() -eq 'Join-Path' -and $values.Count -eq 2) {
            $left = Get-OraclePathValue $values[0] $Directory
            $right = Get-OraclePathValue $values[1] $Directory
            if ($left -and $right) { return [IO.Path]::Combine($left, $right) }
        }
        if ($Node.GetCommandName() -eq 'Split-Path' -and $values.Count -eq 1 -and @($Node.CommandElements | Where-Object { $_ -is [Management.Automation.Language.CommandParameterAst] -and $_.ParameterName -eq 'Parent' }).Count -eq 1) {
            $value = Get-OraclePathValue $values[0] $Directory
            if ($value) { return [IO.Path]::GetDirectoryName($value) }
        }
    }
    return $null
}
function Get-OracleDeclarationId([string]$Value) {
    [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($Value))).ToLowerInvariant()
}
function Get-OracleHarnessInventory([string]$Repository, [string[]]$Tracked) {
    $helperFileCap = if ($env:PWSH_ORACLE_HELPER_FILE_CAP) { [int]$env:PWSH_ORACLE_HELPER_FILE_CAP } else { 2048 }
    $helperByteCap = if ($env:PWSH_ORACLE_HELPER_BYTE_CAP) { [long]$env:PWSH_ORACLE_HELPER_BYTE_CAP } else { 67108864 }
    if ($helperFileCap -lt 1 -or $helperFileCap -gt 2048 -or $helperByteCap -lt 1 -or $helperByteCap -gt 67108864) { throw 'whole-unit refusal: invalid helper caps' }
    if ($Tracked.Count -gt $helperFileCap) { throw 'whole-unit refusal: helper inventory file cap' }
    $total = 0L
    foreach ($relative in $Tracked) {
        $inputPath = [IO.Path]::GetFullPath((Join-Path $Repository $relative))
        if (-not $inputPath.StartsWith($Repository + '/', [StringComparison]::Ordinal)) { throw 'whole-unit refusal: inventory path outside repository' }
        if ([IO.File]::Exists($inputPath)) {
            $item = Get-Item -LiteralPath $inputPath
            if ($item.LinkType) { throw 'whole-unit refusal: helper inventory symlink' }
            $total += $item.Length
        }
    }
    if ($total -gt $helperByteCap) { throw 'whole-unit refusal: helper inventory byte cap' }
    $declarations = @{}
    foreach ($relative in $Tracked) {
        if ($relative -notmatch '(?i)(^|/)(tst|tests)/|(^|/)test\.ps1$|\.(tests|ts)\.ps1$') { continue }
        $path = Join-Path $Repository $relative
        if (-not [IO.File]::Exists($path)) { continue }
        $directory = [IO.Path]::GetDirectoryName($path)
        $ast = [Management.Automation.Language.Parser]::ParseFile($path, [ref]$null, [ref]$null)
        $oraclePathBindings = @{'PSScriptRoot' = $directory}
        $assignments = @($ast.FindAll({ param($n) $n -is [Management.Automation.Language.AssignmentStatementAst] -and $n.Left -is [Management.Automation.Language.VariableExpressionAst] }, $true))
        foreach ($assignment in $assignments | Sort-Object { $_.Extent.StartOffset }) {
            $variable = $assignment.Left.VariablePath.UserPath
            if (@($assignments | Where-Object { $_.Left.VariablePath.UserPath -eq $variable }).Count -ne 1) { continue }
            $value = Get-OraclePathValue $assignment.Right $directory
            if ($value) { $oraclePathBindings[$variable] = $value }
        }
        foreach ($command in @($ast.FindAll({ param($n) $n -is [Management.Automation.Language.CommandAst] }, $true))) {
            $kind = if ($command.InvocationOperator -eq 'Dot') { 'dot-source' } elseif ($command.GetCommandName() -eq 'Import-Module') { 'module' } elseif ($command.GetCommandName() -eq 'New-Module') { 'module-block' } else { continue }
            $candidates = if ($kind -eq 'dot-source') { @($command.CommandElements[0]) } else { @($command.CommandElements | Select-Object -Skip 1) }
            $block = $null
            if ($kind -eq 'module-block') {
                # Only driver-level declarations, never modules created inside tests/functions.
                $nested = $false
                for ($p = $command.Parent; $null -ne $p; $p = $p.Parent) {
                    if ($p -is [Management.Automation.Language.ScriptBlockExpressionAst] -or $p -is [Management.Automation.Language.FunctionDefinitionAst]) { $nested = $true; break }
                }
                if ($nested) { continue }
                $blocks = @($candidates | Where-Object { $_ -is [Management.Automation.Language.ScriptBlockExpressionAst] })
                $names = @()
                for ($i = 0; $i -lt $candidates.Count - 1; $i++) {
                    if ($candidates[$i] -is [Management.Automation.Language.CommandParameterAst] -and $candidates[$i].ParameterName -eq 'Name') { $names += Get-OraclePathValue $candidates[$i+1] $directory }
                }
                if ($blocks.Count -ne 1 -or $names.Count -ne 1) { throw 'whole-unit refusal: ambiguous declared driver module' }
                $block = $blocks[0].ScriptBlock
                $candidates = @($names[0])
            }
            foreach ($candidate in $candidates) {
                if ($candidate -is [Management.Automation.Language.CommandParameterAst]) { continue }
                $value = if ($kind -eq 'module-block') { $candidate } else { Get-OraclePathValue $candidate $directory }
                # Named external modules and flag values are not local path declarations.
                if ($kind -ne 'module-block' -and (-not $value -or $value -notmatch '(?i)\.(ps1|psm1|psd1)$')) { continue }
                $target = if ($kind -eq 'module-block') { $path + '#' + $value } else {
                    $normalized = $value.Replace('\','/')
                    if (-not [IO.Path]::IsPathRooted($normalized)) { $normalized = Join-Path $directory $normalized }
                    [IO.Path]::GetFullPath($normalized)
                }
                if (-not $target.StartsWith($Repository + '/', [StringComparison]::Ordinal)) { continue }
                $owner = $null
                for ($parent = $command.Parent; $null -ne $parent; $parent = $parent.Parent) {
                    if ($parent -is [Management.Automation.Language.CommandAst] -and $parent.GetCommandName() -eq 'New-Module') {
                        $owner = Get-OracleDeclarationId $parent.Extent.Text; break
                    }
                }
                $identity = $kind + ':' + $target.Substring($Repository.Length+1)
                if (-not $declarations.ContainsKey($identity)) {
                    $file = if ($kind -eq 'module-block') { $path } else { $target }
                    $hash = if ([IO.File]::Exists($file)) { (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() } else { $null }
                    $declarations[$identity] = [ordered]@{ id = Get-OracleDeclarationId $identity; kind = $kind; target = $target; name = $(if ($kind -eq 'module-block') { $value } else { [IO.Path]::GetFileNameWithoutExtension($target) }); code_sha256 = $hash; declaring_files = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); declaration_count = 0; owner_block = $owner; block_id = $(if ($kind -eq 'module-block') { Get-OracleDeclarationId $command.Extent.Text } else { $null }); cwd_relative = $(if ($kind -eq 'module-block') { $false } else { -not [IO.Path]::IsPathRooted($value.Replace('\','/')) }); status = 'pending'; reason = ''; block = $block; site_count = 0 }
                }
                $declarations[$identity].declaration_count++
                $null = $declarations[$identity].declaring_files.Add($path)
            }
        }
    }
    @($declarations.Values | Sort-Object { $_.kind + ":" + $_.target })
}
function Get-OracleDefinitionOnly([string]$Path) {
    $errors = $null
    $ast = [Management.Automation.Language.Parser]::ParseFile($Path, [ref]$null, [ref]$errors)
    if ($errors.Count) { return $false }
    foreach ($part in @($ast.BeginBlock,$ast.ProcessBlock,$ast.EndBlock,$ast.CleanBlock)) {
        if (-not $part) { continue }
        foreach ($statement in $part.Statements) {
            if ($statement -is [Management.Automation.Language.IfStatementAst]) {
                # Recognise the original preamble shape without executing it.
                # The loader compiles top-level function bodies only; neither
                # this conditional nor any transitive dot-source can run.
                $commands = @($statement.FindAll({ param($n) $n -is [Management.Automation.Language.CommandAst] }, $true))
                $unsafe = @($commands | Where-Object { $_.GetCommandName() -ne 'Get-Variable' -and $_.InvocationOperator -ne 'Dot' })
                $bodies = @($statement.Clauses | ForEach-Object { $_.Item2 })
                if ($statement.ElseClause) { $bodies += $statement.ElseClause }
                $badBody = $false
                foreach ($body in $bodies) {
                    foreach ($entry in $body.Statements) {
                        if ($entry -isnot [Management.Automation.Language.PipelineAst] -or $entry.PipelineElements.Count -ne 1 -or $entry.PipelineElements[0] -isnot [Management.Automation.Language.CommandAst] -or $entry.PipelineElements[0].InvocationOperator -ne 'Dot') { $badBody = $true }
                    }
                }
                if ($commands.Count -and -not $unsafe.Count -and -not $badBody -and -not @($statement.FindAll({ param($n) $n -is [Management.Automation.Language.InvokeMemberExpressionAst] }, $true)).Count) { continue }
            }
            if ($statement -isnot [Management.Automation.Language.FunctionDefinitionAst] -and $statement -isnot [Management.Automation.Language.TypeDefinitionAst]) { return $false }
        }
    }
    return $true
}

function Get-OracleDeclaringClosure([string]$QueryPath, $Inventory, [string[]]$RunnerFiles, [string]$RootManifest) {
    $paths = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    $ids = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    $null = $paths.Add($QueryPath)
    foreach ($file in $RunnerFiles) { $null = $paths.Add($file) }
    # Follow source declarations, including a refused test body's declarations;
    # this is a provenance closure, not a claim that its commands executed.
    do {
        $changed = $false
        foreach ($d in $Inventory) {
            $targetFile = ($d.target -split '#')[0]
            if ($paths.Contains($targetFile) -or $d.declaring_files.Overlaps($paths) -or $d.target -eq $RootManifest) {
                if ($ids.Add($d.id)) { $changed = $true }
                $null = $paths.Add($targetFile)
            }
        }
    } while ($changed)
    return ,$ids
}
