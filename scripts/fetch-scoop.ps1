[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$pin = Get-Content -LiteralPath (Join-Path $projectRoot 'backend\scoop\upstream.json') -Raw | ConvertFrom-Json
if ($pin.schema_version -ne 1 -or $pin.upstream_revision -notmatch '^[a-f0-9]{40}$') { throw 'Invalid Scoop source pin' }
$workRoot = Join-Path $projectRoot '.work'
New-Item -ItemType Directory -Path $workRoot -Force | Out-Null
$archive = Join-Path $workRoot 'scoop-upstream.zip'
if (-not (Test-Path -LiteralPath $archive)) {
    Invoke-WebRequest -Uri $pin.archive_url -OutFile $archive
}
if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $pin.archive_sha256) {
    throw 'Scoop archive digest mismatch; refusing extraction'
}
$sourceRoot = Join-Path $workRoot "upstream\Scoop-$($pin.upstream_revision)"
if (-not (Test-Path -LiteralPath $sourceRoot)) {
    Expand-Archive -LiteralPath $archive -DestinationPath (Join-Path $workRoot 'upstream')
}
foreach ($sourceFile in $pin.source_files.PSObject.Properties) {
    $path = Join-Path $sourceRoot $sourceFile.Name
    if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -ne $sourceFile.Value) {
        throw "Pinned Scoop source changed: $($sourceFile.Name)"
    }
}
Write-Output $sourceRoot
