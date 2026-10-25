#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$UpstreamSource,
    [Parameter(Mandatory = $true)][string]$ServerExecutable,
    [switch]$ConfirmDisposableMachine
)

$ErrorActionPreference = 'Stop'
if (-not $ConfirmDisposableMachine) {
    throw 'This probe intentionally observes upstream host writes. Run only in a disposable Windows user/VM and explicitly pass -ConfirmDisposableMachine.'
}

$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$pin = Get-Content -LiteralPath (Join-Path $projectRoot 'backend\scoop\upstream.json') -Raw | ConvertFrom-Json
foreach ($sourceFile in $pin.source_files.PSObject.Properties) {
    $sourcePath = Join-Path $UpstreamSource $sourceFile.Name
    if ((Get-FileHash -LiteralPath $sourcePath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $sourceFile.Value) {
        throw "Pinned source verification failed: $($sourceFile.Name)"
    }
}
if (-not (Test-Path -LiteralPath $ServerExecutable -PathType Leaf)) { throw 'Build the fixture server first' }

$runID = [guid]::NewGuid().ToString('N')
$scratch = Join-Path $env:TEMP "portscoop-upstream-probe-$runID"
$payload = Join-Path $scratch 'payload'
$archiveFiles = Join-Path $scratch 'archive-files'
$portableRoot = Join-Path $scratch 'portable (probe)'
$scoopRoot = Join-Path $portableRoot 'scoop'
$engine = Join-Path $scoopRoot 'apps\scoop\current'
$bucket = Join-Path $scoopRoot 'buckets\probe\bucket'
$variableName = "PORTSCOOP_PROBE_$runID"
$shortcutName = "Portscoop Probe $runID"
$moduleName = "PortscoopProbe$runID"
$readyFile = Join-Path $scratch 'server-ready.json'
$powershellExe = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'

New-Item -ItemType Directory -Path $payload,$engine,$bucket,(Join-Path $archiveFiles 'tools'),(Join-Path $archiveFiles 'data'),(Join-Path $scoopRoot 'shims') -Force | Out-Null
Get-ChildItem -LiteralPath $UpstreamSource -Force | ForEach-Object {
    Copy-Item -LiteralPath $_.FullName -Destination $engine -Recurse -Force
}
@{
    last_update = [DateTime]::Now.ToString('o')
    aria2_enabled = $false
    no_junction = $true
    use_git_history = $false
    portscoop_probe_marker = $runID
} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $scoopRoot 'config.json') -Encoding UTF8
'@echo off', 'echo portable fixture' | Set-Content -LiteralPath (Join-Path $archiveFiles 'tools\probe.cmd') -Encoding ASCII
'function Get-PortscoopProbe { ''probe'' }' | Set-Content -LiteralPath (Join-Path $archiveFiles "$moduleName.psm1") -Encoding ASCII
'persistent fixture data' | Set-Content -LiteralPath (Join-Path $archiveFiles 'data\seed.txt') -Encoding ASCII
$archive = Join-Path $payload 'probe.zip'
Compress-Archive -Path (Join-Path $archiveFiles '*') -DestinationPath $archive

function Invoke-ProbeScoop([string[]]$Arguments, [string]$LogName) {
    $logPath = Join-Path $scratch $LogName
    & $powershellExe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $engine 'bin\scoop.ps1') @Arguments *> $logPath
    if ($LASTEXITCODE -ne 0) { throw "Scoop probe failed ($LASTEXITCODE); see $logPath" }
    return $logPath
}

$originalProcessScoop = $env:SCOOP
$originalProcessGlobal = $env:SCOOP_GLOBAL
$originalProcessCache = $env:SCOOP_CACHE
$userPathBefore = [Environment]::GetEnvironmentVariable('PATH', 'User')
$machinePathBefore = [Environment]::GetEnvironmentVariable('PATH', 'Machine')
$modulePathBefore = [Environment]::GetEnvironmentVariable('PSModulePath', 'User')
$server = $null
try {
    $env:SCOOP = $scoopRoot
    $env:SCOOP_GLOBAL = Join-Path $scoopRoot 'global'
    $env:SCOOP_CACHE = Join-Path $scoopRoot 'cache'
    $serverArguments = @('-root', ('"{0}"' -f $payload), '-ready-file', ('"{0}"' -f $readyFile))
    $server = Start-Process -FilePath $ServerExecutable -ArgumentList $serverArguments -PassThru -WindowStyle Hidden
    $readyDeadline = [DateTime]::UtcNow.AddSeconds(10)
    while (-not (Test-Path -LiteralPath $readyFile)) {
        if ($server.HasExited -or [DateTime]::UtcNow -gt $readyDeadline) { throw 'Fixture HTTP server did not become ready' }
        Start-Sleep -Milliseconds 100
    }
    $serverURL = (Get-Content -LiteralPath $readyFile -Raw | ConvertFrom-Json).url
    $envSettings = @{}
    $envSettings[$variableName] = '$dir'
    $manifest = @{
        version = '1.0'
        description = 'Disposable Portscoop upstream compatibility probe'
        url = "$serverURL/probe.zip"
        hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
        bin = 'tools\probe.cmd'
        env_add_path = @('tools')
        env_set = $envSettings
        shortcuts = @(,@('tools\probe.cmd', $shortcutName))
        psmodule = @{ name = $moduleName }
        persist = @('data')
    }
    $manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $bucket 'probe-tool.json') -Encoding UTF8

    $configLog = Invoke-ProbeScoop -Arguments @('config', 'portscoop_probe_marker') -LogName 'config.log'
    $configSelected = (Get-Content -LiteralPath $configLog -Raw).Contains($runID)
    if (-not $configSelected) { throw 'Root-local configuration was not selected' }

    $null = Invoke-ProbeScoop -Arguments @('install', 'probe/probe-tool', '--no-update-scoop') -LogName 'install.log'
    $installedPayload = Test-Path -LiteralPath (Join-Path $scoopRoot 'apps\probe-tool\1.0\tools\probe.cmd')
    $persistedPayload = Test-Path -LiteralPath (Join-Path $scoopRoot 'persist\probe-tool\data\seed.txt')
    if (-not ($installedPayload -and $persistedPayload)) { throw 'Plain archive install/persist probe did not succeed' }
    $shortcutPath = Join-Path ([Environment]::GetFolderPath('StartMenu')) "Programs\Scoop Apps\$shortcutName.lnk"
    $observations = [ordered]@{
        schema_version = 1
        upstream_revision = $pin.upstream_revision
        powershell_version = (& $powershellExe -NoProfile -Command '$PSVersionTable.PSVersion.ToString()' | Out-String).Trim()
        root_local_config = $configSelected
        plain_archive_installed = $installedPayload
        persisted_data_created = $persistedPayload
        host_user_path_changed = ([Environment]::GetEnvironmentVariable('PATH', 'User') -ne $userPathBefore)
        host_machine_path_changed = ([Environment]::GetEnvironmentVariable('PATH', 'Machine') -ne $machinePathBefore)
        host_manifest_environment_written = (-not [string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($variableName, 'User')))
        host_module_path_changed = ([Environment]::GetEnvironmentVariable('PSModulePath', 'User') -ne $modulePathBefore)
        host_shortcut_created = (Test-Path -LiteralPath $shortcutPath)
    }
    $null = Invoke-ProbeScoop -Arguments @('list') -LogName 'list.log'
    $null = Invoke-ProbeScoop -Arguments @('uninstall', 'probe-tool') -LogName 'uninstall.log'
    $observations['package_removed'] = -not (Test-Path -LiteralPath (Join-Path $scoopRoot 'apps\probe-tool\1.0\tools\probe.cmd'))
    $observations['persisted_data_survived_uninstall'] = Test-Path -LiteralPath (Join-Path $scoopRoot 'persist\probe-tool\data\seed.txt')
    $report = Join-Path $scratch 'observations.json'
    $observations | ConvertTo-Json | Set-Content -LiteralPath $report -Encoding UTF8
    foreach ($expectedObservation in @('host_user_path_changed', 'host_manifest_environment_written', 'host_module_path_changed', 'host_shortcut_created', 'package_removed', 'persisted_data_survived_uninstall')) {
        if (-not $observations[$expectedObservation]) { throw "Expected upstream behavior was not observed: $expectedObservation; see $report" }
    }
    if ($observations['host_machine_path_changed']) { throw "Unexpected machine PATH write; see $report" }
    Write-Output $report
} finally {
    if ($server -and -not $server.HasExited) { Stop-Process -Id $server.Id -ErrorAction SilentlyContinue }
    $env:SCOOP = $originalProcessScoop
    $env:SCOOP_GLOBAL = $originalProcessGlobal
    $env:SCOOP_CACHE = $originalProcessCache
}

# The user/VM is disposable. Preserve probe files and host-effect evidence for
# inspection; do not try to restore global state on a non-disposable machine.
