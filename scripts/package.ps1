[CmdletBinding()]
param([string]$Version='0.1.0-dev')
$ErrorActionPreference='Stop'
if ($Version -notmatch '^[a-zA-Z0-9][a-zA-Z0-9._+-]*$') { throw 'Invalid release version' }
$projectRoot=Split-Path -Parent $PSScriptRoot
$dist=Join-Path $projectRoot 'dist'
New-Item -ItemType Directory -Path $dist -Force | Out-Null
$stage=Join-Path $dist ('portscoop-'+$Version+'-windows-amd64')
$archive=$stage+'.zip'
if ((Test-Path -LiteralPath $stage) -or (Test-Path -LiteralPath $archive)) { throw 'Release output already exists; choose a new version' }
New-Item -ItemType Directory -Path $stage | Out-Null
& (Join-Path $PSScriptRoot 'build.ps1') -Version $Version
Copy-Item -LiteralPath (Join-Path $projectRoot 'bin\portscoop.exe'),(Join-Path $projectRoot 'bin\psp.exe') -Destination $stage
$null=& (Join-Path $PSScriptRoot 'prepare-backend.ps1') -Destination (Join-Path $stage 'backend')
Copy-Item -LiteralPath (Join-Path $projectRoot 'README.md'),(Join-Path $projectRoot 'LICENSE'),(Join-Path $PSScriptRoot 'setup.ps1') -Destination $stage
Copy-Item -LiteralPath (Join-Path $projectRoot 'docs') -Destination $stage -Recurse
$artifacts=[ordered]@{}
Get-ChildItem -LiteralPath $stage -File -Recurse -Force | Sort-Object FullName | ForEach-Object {
    $relative=$_.FullName.Substring($stage.Length+1).Replace('\','/')
    $artifacts[$relative]=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
}
$metadata=@{schema_version=1;patch_revision='portable-zip-v1.1';artifacts=$artifacts} | ConvertTo-Json -Depth 5
[IO.File]::WriteAllText((Join-Path $stage 'release.json'),$metadata,(New-Object Text.UTF8Encoding($false)))
& (Join-Path $stage 'portscoop.exe') __portscoop_verify_release
if ($LASTEXITCODE -ne 0) { throw 'Release payload verification failed' }
Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $archive
Write-Output $archive
