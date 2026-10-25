[CmdletBinding()]
param([string]$Version = 'dev')

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $projectRoot 'bin'
New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
$previousLocation = Get-Location
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousCGO = $env:CGO_ENABLED
try {
    Set-Location -LiteralPath $projectRoot
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    & go build -trimpath -ldflags "-X main.version=$Version" -o (Join-Path $outputDirectory 'portscoop.exe') ./cmd/portscoop
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
    Copy-Item -LiteralPath (Join-Path $outputDirectory 'portscoop.exe') -Destination (Join-Path $outputDirectory 'psp.exe') -Force
} finally {
    Set-Location -LiteralPath $previousLocation.Path
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:CGO_ENABLED = $previousCGO
}
