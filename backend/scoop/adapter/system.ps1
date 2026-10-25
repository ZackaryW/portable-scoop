# portscoop patch v1: persistent means this portable root, never HKCU/HKLM.
function Read-PortscoopEnvironment {
    $file = Join-Path $env:PORTSCOOP_ROOT '.portscoop\state\environment.json'
    $values = @{}
    if (Test-Path -LiteralPath $file) {
        $state = Get-Content -LiteralPath $file -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($state.schema_version -ne 1) { throw 'Unsupported portable environment state' }
        foreach ($property in $state.values.PSObject.Properties) { $values[$property.Name] = [string]$property.Value }
    }
    return ,$values
}

function Initialize-PortscoopEnvironment {
    $values = Read-PortscoopEnvironment
    foreach ($name in $values.Keys) {
        if ($name -ieq 'PATH') {
            $env:PATH = $values[$name] + ';' + $env:PATH
        } else {
            [Environment]::SetEnvironmentVariable($name, $values[$name], 'Process')
        }
    }
}

function Publish-EnvVar { }
function Get-EnvVar {
    param([string]$Name, [switch]$Global)
    if ($Global) { return '' }
    return [string](Read-PortscoopEnvironment)[$Name]
}
function Set-EnvVar {
    param([string]$Name, [AllowNull()][string]$Value, [switch]$Global)
    if ($Global) { throw 'Global environment writes are disabled in Portscoop' }
    $values = Read-PortscoopEnvironment
    $previous = $values[$Name]
    if ([string]::IsNullOrEmpty($Value)) { $values.Remove($Name) } else { $values[$Name] = $Value }
    $file = Join-Path $env:PORTSCOOP_ROOT '.portscoop\state\environment.json'
    $temporary = $file + '.' + [guid]::NewGuid().ToString('N') + '.tmp'
    try {
        $json = @{ schema_version = 1; values = $values } | ConvertTo-Json -Depth 5
        [IO.File]::WriteAllText($temporary, $json, (New-Object Text.UTF8Encoding($false)))
        if ([IO.File]::Exists($file)) { [IO.File]::Replace($temporary, $file, [NullString]::Value) } else { [IO.File]::Move($temporary, $file) }
    } finally { if ([IO.File]::Exists($temporary)) { [IO.File]::Delete($temporary) } }
    if ($Name -ieq 'PATH') {
        $old = @($previous -split ';' | Where-Object { $_ })
        $remaining = @($env:PATH -split ';' | Where-Object { $_ -and $_ -notin $old })
        $env:PATH = (@($Value -split ';' | Where-Object { $_ }) + $remaining) -join ';'
    } else {
        [Environment]::SetEnvironmentVariable($Name, $Value, 'Process')
    }
}
function Split-PathLikeEnvVar {
    param([string[]]$Pattern, [string]$Path)
    $parts = @($Path -split ';' | Where-Object { $_ })
    $matching = @()
    foreach ($value in ($Pattern -split ';' | Where-Object { $_ })) {
        $matching += @($parts | Where-Object { $_ -like $value })
        $parts = @($parts | Where-Object { $_ -notlike $value })
    }
    return ($matching -join ';'), ($parts -join ';')
}
function Add-Path {
    param([string[]]$Path, [string]$TargetEnvVar = 'PATH', [switch]$Global, [switch]$Force, [switch]$Quiet)
    $null, $remaining = Split-PathLikeEnvVar $Path (Get-EnvVar $TargetEnvVar -Global:$Global)
    Set-EnvVar $TargetEnvVar ((@($Path) + @($remaining | Where-Object { $_ })) -join ';') -Global:$Global
}
function Remove-Path {
    param([string[]]$Path, [string]$TargetEnvVar = 'PATH', [switch]$Global, [switch]$Quiet, [switch]$PassThru)
    $matching, $remaining = Split-PathLikeEnvVar $Path (Get-EnvVar $TargetEnvVar -Global:$Global)
    if ($matching) { Set-EnvVar $TargetEnvVar $remaining -Global:$Global }
    if ($PassThru) { return $matching }
}
function env($name, $global, $val) {
    if ($PSBoundParameters.ContainsKey('val')) { Set-EnvVar $name $val -Global:$global } else { Get-EnvVar $name -Global:$global }
}
function strip_path($orig_path, $dir) { Split-PathLikeEnvVar @($dir) $orig_path }
function add_first_in_path($dir, $global) { Add-Path @($dir) -Global:$global -Force }
function remove_from_path($dir, $global) { Remove-Path @($dir) -Global:$global }

# Only typed operations from the frozen request reach this executor. No manifest
# script text is evaluated. Check ancestors as well as the destination itself.
function Assert-PortscoopActionPath {
    param([string]$Root, [string]$Path)
    $full = [IO.Path]::GetFullPath($Path)
    $rootPath = [IO.Path]::GetFullPath($Root).TrimEnd('\')
    if (-not $full.StartsWith($rootPath + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Portable action path escapes Store' }
    $cursor = $full
    while ($cursor.Length -ge $rootPath.Length) {
        try { $attributes = [IO.File]::GetAttributes($cursor) }
        catch [IO.FileNotFoundException] { $attributes = 0 }
        catch [IO.DirectoryNotFoundException] { $attributes = 0 }
        if (($attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Portable action path contains a reparse point' }
        if ($cursor.Equals($rootPath, [StringComparison]::OrdinalIgnoreCase)) { break }
        $cursor = [IO.Path]::GetDirectoryName($cursor)
    }
}

function Invoke-PortscoopAction {
    param([psobject]$Action, [string]$App, [string]$Version, [string]$Directory, [string]$PersistDirectory)
    if ($Action.kind -cnotin @('ensure-persisted-file', 'rename-package-file') -or $Action.phase -cne 'pre_install') { throw 'Unsupported portable action' }
    $components = @($App, $Version, [string]$Action.path)
    if ($Action.kind -ceq 'rename-package-file') { $components += [string]$Action.source }
    foreach ($component in $components) {
        if ($component -notmatch '\A[A-Za-z0-9][A-Za-z0-9._+-]*\z' -or $component.EndsWith('.') -or
            ($component.Split('.')[0] -match '\A(CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])\z')) { throw 'Invalid portable action path component' }
    }
    if ($App -ieq 'scoop' -or $Version -ieq 'current') { throw 'Reserved portable action layout' }
    $root = [IO.Path]::GetFullPath($env:PORTSCOOP_ROOT)
    $expectedDir = Join-Path $root "scoop\apps\$App\$Version"
    $expectedPersist = Join-Path $root "scoop\persist\$App"
    if (-not ([IO.Path]::GetFullPath($Directory)).Equals($expectedDir, [StringComparison]::OrdinalIgnoreCase) -or
        -not ([IO.Path]::GetFullPath($PersistDirectory)).Equals($expectedPersist, [StringComparison]::OrdinalIgnoreCase)) { throw 'Portable action layout differs from Store' }
    $target = Join-Path $expectedDir $Action.path
    $saved = Join-Path $expectedPersist $Action.path
    Assert-PortscoopActionPath -Root $root -Path $target
    if ($Action.kind -ceq 'rename-package-file') {
        $source = Join-Path $expectedDir $Action.source
        Assert-PortscoopActionPath -Root $root -Path $source
        if (-not [IO.File]::Exists($source)) { throw 'Portable rename source is not a regular file' }
        if ([IO.File]::Exists($target) -or [IO.Directory]::Exists($target)) { throw 'Portable rename destination already exists' }
        # Two-argument File.Move never overwrites. No executable is launched.
        [IO.File]::Move($source, $target)
        return
    }
    Assert-PortscoopActionPath -Root $root -Path $saved
    if ([IO.Directory]::Exists($target) -or [IO.Directory]::Exists($saved)) { throw 'Portable configuration path is a directory' }
    if ([IO.File]::Exists($saved)) { return }
    # Never truncate an existing file (including a preexisting hard link).
    if (-not [IO.File]::Exists($target)) {
        $stream = [IO.File]::Open($target, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
        $stream.Dispose()
    }
}
