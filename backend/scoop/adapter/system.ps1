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
