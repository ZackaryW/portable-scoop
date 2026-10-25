[CmdletBinding()]
param([string]$Version='0.1.0-dev')
$ErrorActionPreference='Stop'
if ($Version -notmatch '^[a-zA-Z0-9][a-zA-Z0-9._+-]*$') { throw 'Invalid release version' }
$projectRoot=Split-Path -Parent $PSScriptRoot
$dist=Join-Path $projectRoot 'dist'
New-Item -ItemType Directory -Path $dist -Force | Out-Null
$stage=Join-Path $dist ('portscoop-'+$Version+'-windows-amd64')
$archive=$stage+'.zip'
$assets=Join-Path $dist ('release-assets\'+$Version)
if ((Test-Path -LiteralPath $stage) -or (Test-Path -LiteralPath $archive) -or (Test-Path -LiteralPath $assets)) { throw 'Release output already exists; choose a new version' }
New-Item -ItemType Directory -Path $stage | Out-Null
& (Join-Path $PSScriptRoot 'build.ps1') -Version $Version
Copy-Item -LiteralPath (Join-Path $projectRoot 'bin\portscoop.exe'),(Join-Path $projectRoot 'bin\psp.exe'),(Join-Path $projectRoot 'bin\pspe.exe') -Destination $stage
$null=& (Join-Path $PSScriptRoot 'prepare-backend.ps1') -Destination (Join-Path $stage 'backend')
Copy-Item -LiteralPath (Join-Path $projectRoot 'README.md'),(Join-Path $projectRoot 'LICENSE'),(Join-Path $PSScriptRoot 'setup.ps1') -Destination $stage
Copy-Item -LiteralPath (Join-Path $projectRoot 'get.ps1') -Destination $stage
New-Item -ItemType Directory -Path (Join-Path $stage 'scripts') | Out-Null
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'install.ps1') -Destination (Join-Path $stage 'scripts')
Copy-Item -LiteralPath (Join-Path $projectRoot 'docs') -Destination $stage -Recurse
$updateDescriptor=@{
    schema_version=1;update_protocol=1;version=$Version;platform='windows-amd64'
    target_kinds=@('host','store');launchers=@('portscoop.exe','psp.exe','pspe.exe')
    backends=@(@{schema_version=1;protocol_major=1;upstream_revision='e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f';patch_revision='portable-operations-v1.0'})
} | ConvertTo-Json -Depth 6
[IO.File]::WriteAllText((Join-Path $stage 'runtime-update.json'),$updateDescriptor,(New-Object Text.UTF8Encoding($false)))
$artifacts=[ordered]@{}
Get-ChildItem -LiteralPath $stage -File -Recurse -Force | Sort-Object FullName | ForEach-Object {
    $relative=$_.FullName.Substring($stage.Length+1).Replace('\','/')
    $artifacts[$relative]=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
}
$metadata=@{schema_version=1;patch_revision='portable-operations-v1.0';artifacts=$artifacts} | ConvertTo-Json -Depth 5
[IO.File]::WriteAllText((Join-Path $stage 'release.json'),$metadata,(New-Object Text.UTF8Encoding($false)))
& (Join-Path $stage 'portscoop.exe') __portscoop_verify_release
if ($LASTEXITCODE -ne 0) { throw 'Release payload verification failed' }
Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $archive
New-Item -ItemType Directory -Path $assets -Force | Out-Null
$download=Join-Path $assets 'portscoop-windows-amd64.zip'
Copy-Item -LiteralPath $archive -Destination $download
$hash=(Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash.ToLowerInvariant()
[IO.File]::WriteAllText(($download+'.sha256'),($hash+'  portscoop-windows-amd64.zip'+[Environment]::NewLine),(New-Object Text.UTF8Encoding($false)))
Write-Output $archive
