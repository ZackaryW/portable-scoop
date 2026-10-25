#Requires -Version 5.1
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$machine = Get-CimInstance Win32_ComputerSystem
if ($machine.Manufacturer -notmatch 'VMware' -and $machine.Model -notmatch 'VMware') {
    throw 'This entry point is restricted to the explicitly selected VMware test guest.'
}
$root = $PSScriptRoot
$results = Join-Path $root 'results'
New-Item -ItemType Directory -Path $results -Force | Out-Null
$previousTemp = $env:TEMP
$previousTmp = $env:TMP
$env:TEMP = Join-Path $root 'temp'
$env:TMP = $env:TEMP
New-Item -ItemType Directory -Path $env:TEMP -Force | Out-Null
$summary = [ordered]@{
    schema_version = 1
    status = 'running'
    guest_model = $machine.Model
    guest_windows = [Environment]::OSVersion.Version.ToString()
    powershell_version = $PSVersionTable.PSVersion.ToString()
    upstream_revision = $null
    error = $null
}
$exitCode = 1
try {
    $pin = Get-Content -LiteralPath (Join-Path $root 'backend\scoop\upstream.json') -Raw | ConvertFrom-Json
    $summary.upstream_revision = $pin.upstream_revision
    $archive = Join-Path $root 'scoop-upstream.zip'
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $pin.archive_sha256) {
        throw 'Guest source archive digest mismatch'
    }
    $extractionRoot = Join-Path $root 'upstream'
    if (Test-Path -LiteralPath $extractionRoot) { throw 'Use a fresh guest staging folder or restore the baseline before rerunning' }
    Expand-Archive -LiteralPath $archive -DestinationPath $extractionRoot
    $sourceRoot = Join-Path $extractionRoot "Scoop-$($pin.upstream_revision)"
    $null = & (Join-Path $root 'tests\backend\probe-upstream.ps1') -UpstreamSource $sourceRoot -ServerExecutable (Join-Path $root 'probe-server.exe') -ConfirmDisposableMachine
    $summary.status = 'passed'
    $exitCode = 0
} catch {
    $summary.status = 'failed'
    $summary.error = $_.Exception.Message
    $_ | Out-String | Set-Content -LiteralPath (Join-Path $results 'failure.txt') -Encoding UTF8
} finally {
    Get-ChildItem -LiteralPath $env:TEMP -Directory -Filter 'portscoop-upstream-probe-*' -ErrorAction SilentlyContinue | ForEach-Object {
        $runResult = Join-Path $results $_.Name
        New-Item -ItemType Directory -Path $runResult -Force | Out-Null
        Get-ChildItem -LiteralPath $_.FullName -File | Where-Object { $_.Extension -in @('.log', '.json') } | ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination $runResult -Force
        }
    }
    $summary | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $results 'summary.json') -Encoding UTF8
    Compress-Archive -Path (Join-Path $results '*') -DestinationPath (Join-Path $root 'results.zip') -Force
    $env:TEMP = $previousTemp
    $env:TMP = $previousTmp
}
exit $exitCode
