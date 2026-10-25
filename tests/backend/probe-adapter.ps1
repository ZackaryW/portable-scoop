#Requires -Version 5.1
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$machine = Get-CimInstance Win32_ComputerSystem
if ($machine.Manufacturer -notmatch 'VMware' -and $machine.Model -notmatch 'VMware') { throw 'Run this fixture only in the selected disposable VMware guest' }
$scratch = $PSScriptRoot
$result = Join-Path $scratch 'results'
New-Item -ItemType Directory -Path $result -Force | Out-Null
$observations = [ordered]@{ schema_version=1; status='running'; error=$null }
$server = $null
try {
    . (Join-Path $scratch 'host-state.ps1') | ConvertTo-Json -Depth 6 | Set-Content (Join-Path $result 'before.json') -Encoding UTF8
    $ordinary = Join-Path $scratch 'ordinary-scoop-sentinel'
    $other = Join-Path $scratch 'other-portable-sentinel'
    New-Item -ItemType Directory -Path $ordinary,$other -Force | Out-Null
    'ordinary must survive' | Set-Content (Join-Path $ordinary 'sentinel.txt')
    'other must survive' | Set-Content (Join-Path $other 'sentinel.txt')
    $root = Join-Path $scratch 'portable (adapter)'
    $scoop = Join-Path $root 'scoop'
    $engine = Join-Path $scoop 'apps\scoop\current'
    $control = Join-Path $root '.portscoop'
    foreach ($path in @($engine,"$scoop\tmp","$scoop\cache","$scoop\shims","$scoop\persist","$scoop\global","$scoop\buckets\probe\bucket","$control\backend","$control\state")) { New-Item -ItemType Directory -Path $path -Force | Out-Null }
    Get-ChildItem (Join-Path $scratch 'backend\engine') -Force | ForEach-Object { Copy-Item -LiteralPath $_.FullName -Destination $engine -Recurse -Force }
    Copy-Item (Join-Path $scratch 'backend\adapter\*') "$control\backend" -Force
    '{}' | Set-Content "$scoop\config.json" -Encoding UTF8
    $id = [guid]::NewGuid().ToString()
    @{schema_version=1;id=$id;name='adapter fixture';protocol_major=1;ready=$true} | ConvertTo-Json | Set-Content "$control\environment.json" -Encoding UTF8
    $payload = Join-Path $scratch 'payload'
    $contents = Join-Path $scratch 'contents'
    New-Item -ItemType Directory -Path $payload,"$contents\tools","$contents\data" -Force | Out-Null
    '@echo off', 'echo fixture' | Set-Content "$contents\tools\probe.cmd" -Encoding ASCII
    'keep this data' | Set-Content "$contents\data\seed.txt"
    Compress-Archive -Path "$contents\*" -DestinationPath "$payload\probe.zip" -Force
    $ready = Join-Path $scratch 'ready.json'
    $server = Start-Process -FilePath (Join-Path $scratch 'probe-server.exe') -ArgumentList @('-root',('"{0}"' -f $payload),'-ready-file',('"{0}"' -f $ready)) -PassThru -WindowStyle Hidden
    $deadline = [DateTime]::UtcNow.AddSeconds(10)
    while (-not (Test-Path $ready)) { if ($server.HasExited -or [DateTime]::UtcNow -gt $deadline) { throw 'Fixture server failed' }; Start-Sleep -Milliseconds 100 }
    $url = (Get-Content $ready -Raw | ConvertFrom-Json).url
    $shortcutName = 'Portscoop adapter ' + $id
    $moduleName = 'PortscoopAdapter' + $id.Replace('-','')
    $variable = 'PSP_FIXTURE_' + $id.Replace('-','')
    $settings = @{}; $settings[$variable] = '$dir'
    $manifest = @{version='1.0';url="$url/probe.zip";hash=(Get-FileHash "$payload\probe.zip").Hash.ToLowerInvariant();bin='tools\probe.cmd';env_add_path=@('tools');env_set=$settings;persist=@('data');shortcuts=@(,@('tools\probe.cmd',$shortcutName));psmodule=@{name=$moduleName}}
    $powershell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $env:PORTSCOOP_ROOT=$root; $env:SCOOP=$scoop; $env:SCOOP_GLOBAL="$scoop\global"; $env:SCOOP_CACHE="$scoop\cache"; $env:TEMP="$scoop\tmp"; $env:TMP=$env:TEMP
    function Invoke-Adapter([string[]]$Arguments, [string]$Name) {
        $env:PORTSCOOP_REQUEST_FILE = "$scoop\tmp\request.json"
        @{schema_version=1;expected_id=$id;arguments=@($Arguments);manifests=@{'probe-tool'=@{bucket='probe';manifest=$manifest}}} | ConvertTo-Json -Depth 30 | Set-Content $env:PORTSCOOP_REQUEST_FILE -Encoding UTF8
        & $powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$control\backend\entry.ps1" *> "$result\$Name.log"
        if ($LASTEXITCODE -ne 0) { throw "Adapter $Name failed with status $LASTEXITCODE" }
    }
    Invoke-Adapter @('install','probe/probe-tool') 'install'
    $observations.installed = Test-Path "$scoop\apps\probe-tool\1.0\tools\probe.cmd"
    $observations.persisted = Test-Path "$scoop\persist\probe-tool\data\seed.txt"
    $state = Get-Content "$control\state\environment.json" -Raw | ConvertFrom-Json
    $observations.portable_environment_saved = $state.values.$variable -eq "$scoop\apps\probe-tool\current"
    $observations.shortcut_suppressed = -not (Test-Path (Join-Path ([Environment]::GetFolderPath('StartMenu')) "Programs\Scoop Apps\$shortcutName.lnk"))
    $observations.module_suppressed = -not (Test-Path (Join-Path ([Environment]::GetFolderPath('MyDocuments')) "WindowsPowerShell\Modules\$moduleName"))
    Invoke-Adapter @('list') 'list'
    $manifest.version='2.0'
    Invoke-Adapter @('update','probe-tool') 'update'
    $observations.updated = Test-Path "$scoop\apps\probe-tool\2.0\tools\probe.cmd"
    Invoke-Adapter @('uninstall','probe-tool') 'uninstall'
    $observations.removed = -not (Test-Path "$scoop\apps\probe-tool")
    $state = Get-Content "$control\state\environment.json" -Raw | ConvertFrom-Json
    $observations.environment_removed = -not $state.values.$variable
    $observations.persistence_survived = Test-Path "$scoop\persist\probe-tool\data\seed.txt"
    $observations.other_root_unchanged = (Get-Content "$other\sentinel.txt") -eq 'other must survive'
    $observations.ordinary_scoop_unchanged = (Get-Content "$ordinary\sentinel.txt") -eq 'ordinary must survive'
    . (Join-Path $scratch 'host-state.ps1') | ConvertTo-Json -Depth 6 | Set-Content "$result\after.json" -Encoding UTF8
    $before = Get-Content "$result\before.json" -Raw
    $after = Get-Content "$result\after.json" -Raw
    $observations.host_state_unchanged = $before -ceq $after
    foreach ($key in @($observations.Keys)) { if ($observations[$key] -is [bool] -and -not $observations[$key]) { throw "Observation failed: $key" } }
    $observations.status='passed'
} catch {
    $observations.status='failed'; $observations.error=$_.Exception.Message
    $_ | Out-String | Set-Content "$result\failure.txt"
} finally {
    if ($server -and -not $server.HasExited) { Stop-Process $server.Id -ErrorAction SilentlyContinue }
    $observations | ConvertTo-Json -Depth 6 | Set-Content "$result\observations.json" -Encoding UTF8
    Compress-Archive -Path "$result\*" -DestinationPath "$scratch\results.zip" -Force
}
if ($observations.status -ne 'passed') { exit 1 }
