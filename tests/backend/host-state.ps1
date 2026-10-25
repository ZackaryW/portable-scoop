# Read-only host sentinel. This does not invoke Scoop or change any environment.
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$files = New-Object 'System.Collections.Generic.SortedDictionary[string,string]'
function Add-FileState([string]$Path) {
    if (Test-Path -LiteralPath $Path -PathType Leaf) {
        $files[$Path] = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    } else {
        $files[$Path] = 'absent'
    }
}
function Get-EnvironmentDigest([EnvironmentVariableTarget]$Target) {
    $items = [Environment]::GetEnvironmentVariables($Target)
    $ordered = [ordered]@{}
    foreach ($name in ($items.Keys | Sort-Object)) { $ordered[$name] = $items[$name] }
    $bytes = [Text.Encoding]::UTF8.GetBytes(($ordered | ConvertTo-Json -Compress))
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

$scoopCommand = Get-Command scoop -ErrorAction SilentlyContinue
$scoopRoot = $null
if ($scoopCommand -and $scoopCommand.Source -and (Split-Path -Leaf (Split-Path -Parent $scoopCommand.Source)) -eq 'shims') {
    $scoopRoot = Split-Path -Parent (Split-Path -Parent $scoopCommand.Source)
    Get-ChildItem -LiteralPath (Join-Path $scoopRoot 'shims') -File | ForEach-Object { Add-FileState $_.FullName }
    foreach ($relative in @('config.json','apps\scoop\current\bin\scoop.ps1','apps\scoop\current\lib\core.ps1')) {
        Add-FileState (Join-Path $scoopRoot $relative)
    }
    Get-ChildItem -LiteralPath (Join-Path $scoopRoot 'apps') -Directory | ForEach-Object {
        Get-ChildItem -LiteralPath $_.FullName -Directory | Where-Object { -not ($_.Attributes -band [IO.FileAttributes]::ReparsePoint) } | ForEach-Object {
            Add-FileState (Join-Path $_.FullName 'manifest.json')
            Add-FileState (Join-Path $_.FullName 'install.json')
        }
    }
}
$configHome = if ($env:XDG_CONFIG_HOME) { $env:XDG_CONFIG_HOME } else { Join-Path $env:USERPROFILE '.config' }
Add-FileState (Join-Path $configHome 'scoop\config.json')
$shortcutRoot = Join-Path ([Environment]::GetFolderPath('StartMenu')) 'Programs\Scoop Apps'
if (Test-Path -LiteralPath $shortcutRoot) {
    Get-ChildItem -LiteralPath $shortcutRoot -File -Recurse | ForEach-Object { Add-FileState $_.FullName }
}
[ordered]@{
    schema_version = 1
    scoop_root = $scoopRoot
    user_environment_sha256 = Get-EnvironmentDigest -Target User
    machine_environment_sha256 = Get-EnvironmentDigest -Target Machine
    files = $files
}
