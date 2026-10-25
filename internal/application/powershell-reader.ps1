$ErrorActionPreference = 'Stop'
$control = $env:PORTSCOOP_PS_CONTROL
Remove-Item Env:\PORTSCOOP_PS_CONTROL -ErrorAction SilentlyContinue
$deadline = [DateTime]::UtcNow.AddSeconds(15)
function Invoke-ControlIO([scriptblock]$Action) {
    while ($true) {
        try { return (& $Action) } catch {
            $code = $_.Exception.GetBaseException().HResult -band 65535
            if ($code -notin 32, 33) { throw }
            if ([DateTime]::UtcNow -ge $deadline) { throw 'PowerShell startup control file remained locked' }
            Start-Sleep -Milliseconds 10
        }
    }
}
function Read-Control([int]$Sequence) {
    $path = Join-Path $control ('server-{0:d4}.json' -f $Sequence)
    while (-not [IO.File]::Exists($path)) {
        if ([DateTime]::UtcNow -ge $deadline -or -not [IO.Directory]::Exists($control) -or [IO.File]::Exists((Join-Path $control 'server.closed'))) { throw 'PowerShell startup control disconnected or timed out' }
        Start-Sleep -Milliseconds 10
    }
    if ((Get-Item -LiteralPath $path).Length -gt 1048576) { throw 'PowerShell startup message exceeds size bound' }
    $value = (Invoke-ControlIO { [IO.File]::ReadAllText($path, [Text.Encoding]::UTF8) }) | ConvertFrom-Json
    Invoke-ControlIO { [IO.File]::Delete($path) }
    return $value
}
function Write-Control([int]$Sequence, $Value) {
    $path = Join-Path $control ('client-{0:d4}.json' -f $Sequence)
    $temporary = $path + '.tmp'
    Invoke-ControlIO { [IO.File]::WriteAllText($temporary, ($Value | ConvertTo-Json -Compress -Depth 8), [Text.UTF8Encoding]::new($false)) }
    Invoke-ControlIO { [IO.File]::Move($temporary, $path) }
}

try {
    $request = Read-Control 0
    if ($request.schema_version -ne 1 -or $request.arguments -isnot [Array]) { throw 'Invalid PowerShell invocation payload' }
    $marker = [IO.File]::ReadAllText((Join-Path $env:PORTSCOOP_ROOT '.portscoop\environment.json')) | ConvertFrom-Json
    if ($marker.id -cne $request.expected_id -or $marker.ready -ne $true) { throw 'Store identity changed before PowerShell startup' }
    # Resolve and load the script while launch preparation still holds the lock.
    $targetCommand = Get-Command -Name $request.target -CommandType ExternalScript -ErrorAction Stop
    $null = $targetCommand.ScriptBlock
    $named = @{}
    $positional = [Collections.Generic.List[object]]::new()
    $arguments = @($request.arguments)
    for ($i = 0; $i -lt $arguments.Count; $i++) {
        $arg = $arguments[$i]
        if ($arg -isnot [string] -or $arg.IndexOf([char]0) -ge 0) { throw 'Invalid PowerShell argument data' }
        $match = [regex]::Match($arg, '^-(?<name>[A-Za-z_][A-Za-z0-9_-]*)(?::(?<value>.*))?$')
        $parameter = $null
        if ($match.Success) {
            $name = $match.Groups['name'].Value
            $candidates = @($targetCommand.Parameters.Values | Where-Object { $_.Name -ieq $name -or $_.Aliases -icontains $name })
            if ($candidates.Count -eq 0) { $candidates = @($targetCommand.Parameters.Values | Where-Object { $_.Name.StartsWith($name, [StringComparison]::OrdinalIgnoreCase) }) }
            if ($candidates.Count -gt 1) { throw 'Ambiguous PowerShell named parameter' }
            if ($candidates.Count -eq 1) { $parameter = $candidates[0] }
        }
        if ($null -eq $parameter) { $positional.Add($arg); continue }
        if ($named.ContainsKey($parameter.Name)) { throw 'Duplicate PowerShell named parameter' }
        $inline = $match.Groups['value'].Success
        if ($parameter.ParameterType -eq [Management.Automation.SwitchParameter]) {
            $value = $true
            if ($inline) {
                switch ($match.Groups['value'].Value.ToLowerInvariant()) {
                    { $_ -in 'true', '$true', '1' } { $value = $true; break }
                    { $_ -in 'false', '$false', '0' } { $value = $false; break }
                    default { throw 'Unqualified PowerShell switch value; use true or false' }
                }
            }
        } elseif ($inline) { $value = $match.Groups['value'].Value
        } else {
            if ($i + 1 -ge $arguments.Count) { throw 'PowerShell named parameter requires a value' }
            $i++; $value = $arguments[$i]
            if ($value -isnot [string] -or $value.IndexOf([char]0) -ge 0) { throw 'Invalid PowerShell parameter value' }
        }
        $named[$parameter.Name] = $value
    }
    $positionals = $positional.ToArray()
    Write-Control 0 @{ state = 'ready'; error = '' }
    $start = Read-Control 1
    if ($start.state -cne 'start') { throw 'Invalid PowerShell startup handoff' }
    Write-Control 1 @{ state = 'consumed'; error = '' }
} catch {
    try { Write-Control 0 @{ state = 'failed'; error = $_.Exception.Message } } catch { }
    exit 2
}

# Ordinary parameter/array splatting keeps values as data and retains the
# console host. No dynamic script source or AST compilation is used.
$global:LASTEXITCODE = 0
& $targetCommand @named @positionals
$succeeded = $?
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
if (-not $succeeded) { exit 1 }
exit 0
