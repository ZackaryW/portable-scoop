[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$null = & (Join-Path $PSScriptRoot 'fetch-scoop.ps1')
$stagingRoot = Join-Path $projectRoot '.work\vmware-testing'
$bundleRoot = Join-Path $stagingRoot ('bundle-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path (Join-Path $bundleRoot 'backend\scoop'),(Join-Path $bundleRoot 'tests\backend') -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $projectRoot 'backend\scoop\upstream.json') -Destination (Join-Path $bundleRoot 'backend\scoop\upstream.json')
Copy-Item -LiteralPath (Join-Path $projectRoot 'tests\backend\probe-upstream.ps1') -Destination (Join-Path $bundleRoot 'tests\backend\probe-upstream.ps1')
Copy-Item -LiteralPath (Join-Path $projectRoot 'tests\backend\run-guest-probe.ps1') -Destination (Join-Path $bundleRoot 'run-guest-probe.ps1')
Copy-Item -LiteralPath (Join-Path $projectRoot '.work\scoop-upstream.zip') -Destination (Join-Path $bundleRoot 'scoop-upstream.zip')
$previousLocation = Get-Location
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousCGO = $env:CGO_ENABLED
try {
    Set-Location -LiteralPath $projectRoot
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    & go build -trimpath -o (Join-Path $bundleRoot 'probe-server.exe') ./tests/backend/server
    if ($LASTEXITCODE -ne 0) { throw 'Could not build guest fixture server' }
} finally {
    Set-Location -LiteralPath $previousLocation.Path
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:CGO_ENABLED = $previousCGO
}
$archivePath = Join-Path $stagingRoot 'portscoop-probe.zip'
Compress-Archive -Path (Join-Path $bundleRoot '*') -DestinationPath $archivePath -Force
Get-ChildItem -LiteralPath $bundleRoot -File -Recurse | ForEach-Object {
    [pscustomobject]@{
        path = $_.FullName.Substring($bundleRoot.Length + 1)
        sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    }
} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $stagingRoot 'bundle-manifest.json') -Encoding UTF8
Write-Output $archivePath
