#Requires -Version 5.1
param([Parameter(Mandatory=$true)][string]$Root,[Parameter(Mandatory=$true)][string]$Inputs)
$ErrorActionPreference='Stop'
if((Get-CimInstance Win32_ComputerSystem).Manufacturer -notmatch 'VMware'){throw 'Disposable VMware guest required'}
$Root=[IO.Path]::GetFullPath($Root);if(Test-Path -LiteralPath $Root){throw 'Fresh qualification root required'}
New-Item -ItemType Directory -Path $Root|Out-Null
Start-Transcript -Path (Join-Path $Root 'qualification.log')|Out-Null
$oldLocal=$env:LOCALAPPDATA;$oldPath=$env:PATH
$before=@{};foreach($scope in @('User','Machine')){foreach($name in @('PATH','SCOOP','SCOOP_GLOBAL','SCOOP_CACHE')){$before[$scope+'/'+$name]=[Environment]::GetEnvironmentVariable($name,$scope)}}
$r=@{passed=$false;error=$null}
try {
 $env:LOCALAPPDATA=Join-Path $Root 'profile';$env:PATH=Join-Path $env:SystemRoot 'System32'
 Expand-Archive -LiteralPath (Join-Path $Inputs 'update-candidate.zip') -DestinationPath (Join-Path $Root 'host')
 Expand-Archive -LiteralPath (Join-Path $Inputs 'update-old.zip') -DestinationPath (Join-Path $Root 'old-store')
 & (Join-Path $Root 'old-store\setup.ps1') -Mode Portable -Destination (Join-Path $Root 'old-store') -Name Old
 if($LASTEXITCODE -ne 0){throw 'Old Store setup failed'}
 Expand-Archive -LiteralPath (Join-Path $Inputs 'portable-hooks-dev1.zip') -DestinationPath (Join-Path $Root 'compatible-store')
 & (Join-Path $Root 'compatible-store\setup.ps1') -Mode Portable -Destination (Join-Path $Root 'compatible-store') -Name Compatible
 if($LASTEXITCODE -ne 0){throw 'Compatible Store setup failed'}
 Copy-Item -LiteralPath (Join-Path $Root 'compatible-store\scoop\buckets\main') -Destination (Join-Path $Root 'old-store\scoop\buckets\main') -Recurse
 $sentinel=Join-Path $Root 'old-store\scoop\persist\migration-sentinel';New-Item -ItemType Directory -Path $sentinel -Force|Out-Null
 [IO.File]::WriteAllText((Join-Path $sentinel 'settings'),'preserve me')
 Copy-Item -LiteralPath (Join-Path $Inputs 'update-candidate.zip') -Destination (Join-Path $Root 'candidate.zip')
 $env:PORTSCOOP_PACKAGED_UPDATE_FIXTURE=$Root
 & (Join-Path $Inputs 'packaged-update-tests.exe') '-test.run=^TestPackagedManagedToolQualification$' '-test.v' '-test.timeout=12m' *> (Join-Path $Root 'tests.log')
 if($LASTEXITCODE -ne 0){throw 'Packaged runtime qualification failed; see tests.log'}
 foreach($scope in @('User','Machine')){foreach($name in @('PATH','SCOOP','SCOOP_GLOBAL','SCOOP_CACHE')){if([Environment]::GetEnvironmentVariable($name,$scope) -cne $before[$scope+'/'+$name]){throw "Persistent environment changed: $scope/$name"}}}
 $r.passed=$true
} catch {$r.error=$_.Exception.Message;throw} finally {
 $env:LOCALAPPDATA=$oldLocal;$env:PATH=$oldPath;Remove-Item Env:PORTSCOOP_PACKAGED_UPDATE_FIXTURE -ErrorAction SilentlyContinue
 [IO.File]::WriteAllText((Join-Path $Root 'result.json'),($r|ConvertTo-Json -Depth 5));Stop-Transcript|Out-Null
}
