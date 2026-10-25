[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$ReleaseArchive,
    [Parameter(Mandatory=$true)][string]$Destination
)
$ErrorActionPreference='Stop'
$project=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$ReleaseArchive=[IO.Path]::GetFullPath($ReleaseArchive)
$Destination=[IO.Path]::GetFullPath($Destination)
if(-not(Test-Path -LiteralPath $ReleaseArchive -PathType Leaf)){throw 'Local release archive is required'}
if(Test-Path -LiteralPath $Destination){throw 'Choose a new qualification directory; existing artifacts are preserved'}
if(Test-Path -LiteralPath ($Destination+'.zip')){throw 'Qualification ZIP already exists'}
New-Item -ItemType Directory -Path (Join-Path $Destination 'scripts') -Force|Out-Null
$previous=Get-Location
try{
    Set-Location -LiteralPath $project
    foreach($build in @(
        @('build','-o',(Join-Path $Destination 'pspe-fixture.exe'),'./tests/application/fixture'),
        @('build','-ldflags','-H=windowsgui','-o',(Join-Path $Destination 'pspe-gui-fixture.exe'),'./tests/application/fixture'),
        @('build','-o',(Join-Path $Destination 'probe-server.exe'),'./tests/backend/server'),
        @('test','-c','-o',(Join-Path $Destination 'application.test.exe'),'./internal/application'),
        @('test','-c','-o',(Join-Path $Destination 'runner.test.exe'),'./internal/runner')
    )){& go @build;if($LASTEXITCODE-ne0){throw 'Fixture compilation failed'}}
    Copy-Item -LiteralPath $ReleaseArchive -Destination (Join-Path $Destination 'portscoop-windows-amd64.zip')
    $hash=(Get-FileHash -LiteralPath $ReleaseArchive -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $Destination 'portscoop-windows-amd64.zip.sha256'),($hash+'  portscoop-windows-amd64.zip'+[Environment]::NewLine),[Text.UTF8Encoding]::new($false))
    Copy-Item -LiteralPath get.ps1,tests/backend/host-state.ps1 -Destination $Destination
    Copy-Item -LiteralPath scripts/install.ps1 -Destination (Join-Path $Destination 'scripts/install.ps1')
    Copy-Item -LiteralPath tests/installer/qualify-release.ps1 -Destination (Join-Path $Destination 'qualify-installer.ps1')
    Copy-Item -LiteralPath tests/installer/qualify-host-bootstrap.ps1 -Destination (Join-Path $Destination 'qualify-host-bootstrap.ps1')
    Compress-Archive -Path (Join-Path $Destination '*') -DestinationPath ($Destination+'.zip')
    Write-Output ($Destination+'.zip')
}finally{Set-Location -LiteralPath $previous.Path}
