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

function Assert-PortscoopRelativePath {
    param([string]$Path)
    if (-not $Path -or [IO.Path]::IsPathRooted($Path) -or $Path -match '[<>:"|?*`$\x00-\x1f]') { throw 'Invalid action relative path' }
    foreach ($part in ($Path -split '[\\/]')) {
        if (-not $part -or $part -in @('.','..') -or $part.EndsWith('.') -or $part.EndsWith(' ') -or $part.Split('.')[0] -match '\A(CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])\z') { throw 'Invalid action path component' }
    }
}
function Invoke-PortscoopAction {
    param([psobject]$Action, [string]$App, [string]$Version, [string]$Directory, [string]$PersistDirectory)
    if ($Action.kind -cnotin @('ensure-persisted-file','rename-package-file') -or $Action.phase -cnotin @('pre_install','post_install')) { throw 'Unsupported portable action' }
    foreach ($component in @($App,$Version)) {
        if ($component -notmatch '\A[A-Za-z0-9][A-Za-z0-9._+-]*\z') { throw 'Invalid portable action identity' }
        Assert-PortscoopRelativePath $component
    }
    Assert-PortscoopRelativePath $Action.path
    if ($App -ieq 'scoop' -or $Version -ieq 'current') { throw 'Reserved portable action layout' }
    $root=[IO.Path]::GetFullPath($env:PORTSCOOP_ROOT)
    $physical=Join-Path $root "scoop\apps\$App\$Version"
    $expectedPersist=Join-Path $root "scoop\persist\$App"
    $expectedDir=$physical
    Assert-PortscoopActionPath $root $physical
    Assert-PortscoopActionPath $root $expectedPersist
    if ($Action.phase -ceq 'post_install') {
        $expectedDir=Join-Path $root "scoop\apps\$App\current"
        $link=Get-Item -LiteralPath $expectedDir -Force
        $targets=@($link.Target)
        if ($link.LinkType -ne 'Junction' -or $targets.Count -ne 1 -or -not ([IO.Path]::GetFullPath($targets[0])).Equals($physical,[StringComparison]::OrdinalIgnoreCase)) { throw 'Current alias differs from approved version' }
    }
    if (-not ([IO.Path]::GetFullPath($Directory)).Equals($expectedDir,[StringComparison]::OrdinalIgnoreCase) -or -not ([IO.Path]::GetFullPath($PersistDirectory)).Equals($expectedPersist,[StringComparison]::OrdinalIgnoreCase)) { throw 'Portable action layout differs from Store' }
    $target=Join-Path $physical $Action.path
    if ($Action.kind -ceq 'rename-package-file') {
        Assert-PortscoopRelativePath $Action.source
        $source=Join-Path $physical $Action.source
        Assert-PortscoopActionPath $root $source
        Assert-PortscoopActionPath $root $target
        if (-not [IO.File]::Exists($source) -or [IO.File]::Exists($target) -or [IO.Directory]::Exists($target)) { throw 'Rename source missing or destination already exists' }
        [IO.File]::Move($source,$target)
        return
    }
    Assert-PortscoopRelativePath $Action.persist_path
    $saved=Join-Path $expectedPersist $Action.persist_path
    Assert-PortscoopActionPath $root $saved
    Assert-PortscoopActionPath $root $target
    if ([IO.Directory]::Exists($target) -or [IO.Directory]::Exists($saved)) { throw 'Portable configuration path is a directory' }
    if ($Action.phase -ceq 'post_install') {
        # Persisted files are hard links after upstream persist_data. Verify the
        # exact saved path among the link targets, then preserve it untouched.
        $file=Get-Item -LiteralPath $target -Force
        if (-not [IO.File]::Exists($saved) -or $file.LinkType -ne 'HardLink' -or $saved -notin @($file.Target)) { throw 'Unverified persisted file mapping after installation' }
        return
    }
    if ([IO.File]::Exists($saved)) { return }
    if (-not [IO.File]::Exists($target)) {
        $parent=[IO.Path]::GetDirectoryName($target)
        [IO.Directory]::CreateDirectory($parent) | Out-Null
        $stream=[IO.File]::Open($target,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
        $stream.Dispose()
    }
}
