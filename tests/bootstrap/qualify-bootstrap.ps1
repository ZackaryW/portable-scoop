[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Root,[Parameter(Mandatory=$true)][string]$Archive)
$ErrorActionPreference='Stop'
$Root=[IO.Path]::GetFullPath($Root)
if(Test-Path -LiteralPath $Root){throw 'Qualification root must be new'}
New-Item -ItemType Directory -Path $Root | Out-Null
$report=@{passed=$false;checks=@();error=$null}
$saved=@{}
foreach($name in @('PATH','LOCALAPPDATA','GIT_CONFIG_GLOBAL','GIT_EXEC_PATH','GIT_DIR','HOME')){$saved[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
$persistent=@{}
foreach($scope in @('User','Machine')){foreach($name in @('PATH','SCOOP','SCOOP_GLOBAL','SCOOP_CACHE')){$persistent[$scope+'/'+$name]=[Environment]::GetEnvironmentVariable($name,$scope)}}
$sentinels=@{}
foreach($p in @((Join-Path $env:USERPROFILE '.gitconfig'),(Join-Path $env:USERPROFILE '.config\scoop\config.json'),(Join-Path $env:USERPROFILE 'scoop\config.json'))){$sentinels[$p]=$null;if(Test-Path -LiteralPath $p -PathType Leaf){$sentinels[$p]=(Get-FileHash -LiteralPath $p).Hash}}
function Assert([bool]$Condition,[string]$Message){if(-not $Condition){throw $Message}}
try {
    $payload=Join-Path $Root 'payload'
    Expand-Archive -LiteralPath $Archive -DestinationPath $payload
    $env:LOCALAPPDATA=Join-Path $Root 'localappdata'
    $env:HOME=Join-Path $Root 'hostile'
    New-Item -ItemType Directory -Path $env:HOME -Force | Out-Null
    $env:GIT_CONFIG_GLOBAL=Join-Path $env:HOME '.gitconfig'
    [IO.File]::WriteAllText($env:GIT_CONFIG_GLOBAL,"[url `"https://invalid.example/`"]`n insteadOf = https://github.com/`n[credential]`n helper = !exit 99`n")
    $env:GIT_EXEC_PATH=$env:HOME
    $env:GIT_DIR=Join-Path $env:HOME 'missing.git'
    $env:PATH=Join-Path $env:SystemRoot 'System32'
    $hostRoot=Join-Path $Root 'host'
    & (Join-Path $payload 'setup.ps1') -Mode Central -Destination $hostRoot -CreateDefaultStore
    Assert ($LASTEXITCODE -eq 0) 'Host default Store bootstrap failed'
    $psp=Join-Path $hostRoot 'psp.exe'
    $store=Join-Path $hostRoot 'store'
    $main=Join-Path $store 'scoop\buckets\main'
    Assert (Test-Path -LiteralPath (Join-Path $main 'bucket\7zip.json')) 'Main not available'
    $report.checks+='Host default Store acquired pinned Main without host Git or inherited Git configuration'
    & $psp env bootstrap
    Assert ($LASTEXITCODE -eq 0) 'Existing Store bootstrap failed'
    & $psp bucket add extras
    Assert ($LASTEXITCODE -eq 0) 'Private Git could not add Extras'
    Assert (Test-Path -LiteralPath (Join-Path $store 'scoop\buckets\extras\bucket\potplayer.json')) 'Extras clone missing'
    $report.checks+='Existing Store bootstrap and private Git Extras clone succeeded'
    $second=Join-Path $Root 'second'
    & $psp env init $second --name Second
    Assert ($LASTEXITCODE -eq 0) 'Direct Store bootstrap failed'
    $otherReceipt=(Get-FileHash -LiteralPath (Join-Path $second '.portscoop\state\bootstrap.json')).Hash
    [IO.File]::WriteAllText((Join-Path $main 'local-sentinel'),'keep')
    $moved=Join-Path $Root 'moved store'
    Move-Item -LiteralPath $store -Destination $moved
    & (Join-Path $moved 'psp.exe') env bootstrap
    Assert ($LASTEXITCODE -eq 0) 'Relocated bootstrap failed'
    Assert (-not (Test-Path -LiteralPath $store)) 'Old root recreated'
    Assert ([IO.File]::ReadAllText((Join-Path $moved 'scoop\buckets\main\local-sentinel')) -ceq 'keep') 'Main checkout reset'
    Assert ((Get-FileHash -LiteralPath (Join-Path $second '.portscoop\state\bootstrap.json')).Hash -ceq $otherReceipt) 'Other Store modified'
    $report.checks+='Direct initialization, repeat bootstrap, relocation, and independent second Store succeeded'
    foreach($scope in @('User','Machine')){foreach($name in @('PATH','SCOOP','SCOOP_GLOBAL','SCOOP_CACHE')){Assert ([Environment]::GetEnvironmentVariable($name,$scope) -ceq $persistent[$scope+'/'+$name]) ('Persistent environment changed: '+$name)}}
    foreach($p in $sentinels.Keys){$after=$null;if(Test-Path -LiteralPath $p -PathType Leaf){$after=(Get-FileHash -LiteralPath $p).Hash};Assert ($after -ceq $sentinels[$p]) ('Host sentinel changed: '+$p)}
    $report.checks+='Ordinary Scoop/Git sentinels and persistent environment unchanged'
    $report.passed=$true
} catch {$report.error=$_.Exception.Message;throw} finally {
    foreach($name in $saved.Keys){[Environment]::SetEnvironmentVariable($name,$saved[$name],'Process')}
    [IO.File]::WriteAllText((Join-Path $Root 'result.json'),($report | ConvertTo-Json -Depth 5))
}
