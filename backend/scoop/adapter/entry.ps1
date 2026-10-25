#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
try {
    . "$PSScriptRoot\request-reader.ps1"
    $portscoopRequest = Read-PortscoopRequest
    $engine = Join-Path $env:PORTSCOOP_ROOT 'scoop\apps\scoop\current'
    if (-not (Test-Path -LiteralPath (Join-Path $env:PORTSCOOP_ROOT 'scoop\config.json') -PathType Leaf)) { throw 'Portable config is missing' }
    . "$engine\lib\core.ps1"
    if ($scoopdir -ine (Join-Path $env:PORTSCOOP_ROOT 'scoop') -or $globaldir -ine (Join-Path $scoopdir 'global') -or $cachedir -ine (Join-Path $scoopdir 'cache')) { throw 'Backend paths differ from the portable layout' }
    # Fixed compatibility settings also override hand-edited unsafe config.
    $scoopConfig = [pscustomobject]@{
        aria2_enabled=$false; use_external_7zip=$false; use_isolated_path=$false;
        use_git_history=$false; use_sqlite_cache=$false; no_junction=$false;
        scoop_repo='https://github.com/ScoopInstaller/Scoop'; scoop_branch='portscoop';
    }
    $scoopPathEnvVar = 'PATH'
    . "$engine\lib\buckets.ps1"
    . "$engine\lib\system.ps1"
    Initialize-PortscoopEnvironment
    $operation = $portscoopRequest.arguments[0]
    if ($operation -notin @('install','uninstall','update','list','search','status','bucket','reset','cleanup','config')) { throw 'Unsupported portable command' }
    [string[]]$forward = @($portscoopRequest.arguments | Select-Object -Skip 1)
    & "$engine\libexec\scoop-$operation.ps1" @forward
    if (-not $?) { exit 1 }
} catch {
    [Console]::Error.WriteLine('portscoop backend: ' + $_.Exception.Message)
    [Console]::Error.WriteLine($_.ScriptStackTrace)
    exit 1
}
