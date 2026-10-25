#Requires -Version 5.1
param([Parameter(Mandatory=$true)][string]$Root)
$ErrorActionPreference='Stop'
$project=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
. (Join-Path $project 'backend\scoop\adapter\system.ps1')
. (Join-Path $project 'backend\scoop\adapter\patch-functions.ps1')
function Assert([bool]$Condition,[string]$Message){if(-not $Condition){throw $Message}}
function Reject([scriptblock]$Body){$failed=$false;try{& $Body}catch{$failed=$true};Assert $failed 'Expected tree action rejection'}
function Put([string]$Path,[string]$Content){[IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Path))|Out-Null;[IO.File]::WriteAllText($Path,$Content)}
function arch_specific($name,$manifest,$arch){return $manifest.$name}
$env:PORTSCOOP_ROOT=$Root
$app='treefixture'
$physical=Join-Path $Root 'scoop\apps\treefixture\1.0'
$persist_dir=Join-Path $Root 'scoop\persist\treefixture'
$outside=Join-Path $Root 'outside'
Put "$physical\input\a.txt" 'fresh'
Put "$outside\sentinel" 'preserve'
$copy=[pscustomobject]@{phase='pre_install';kind='copy-package-tree';source='input';path='output';recursive=$true;force=$true}
Invoke-PortscoopAction $copy $app '1.0' $physical $persist_dir @()
Assert ([IO.File]::ReadAllText("$physical\output\a.txt") -ceq 'fresh') 'Pre-install tree copy failed'
Put (Join-Path $physical '$PLUGINSDIR\plugin') 'temporary'
$cleanup=[pscustomobject]@{phase='pre_install';kind='cleanup-package-tree';paths=@('$*','Uninst*');recursive=$true;force=$true;missing_ok=$true}
Invoke-PortscoopAction $cleanup $app '1.0' $physical $persist_dir @()
Assert (-not [IO.Directory]::Exists((Join-Path $physical '$PLUGINSDIR'))) 'Pre-install dollar cleanup failed'
$cleanup.paths=@('saved*')
Reject {Invoke-PortscoopAction $cleanup $app '1.0' $physical $persist_dir @('saved-profile')}
$cleanup.paths=@('nested/profile/file')
Reject {Invoke-PortscoopAction $cleanup $app '1.0' $physical $persist_dir @('nested/profile')}
$cleanup.paths=@('nested\profile\file')
Reject {Invoke-PortscoopAction $cleanup $app '1.0' $physical $persist_dir @('nested/profile')}
$cleanup.paths=@('output');$cleanup.phase='other'
Reject {Invoke-PortscoopAction $cleanup $app '1.0' $physical $persist_dir @()}

# Model upstream persistence: resources moved aside, saved assets junctioned in.
Put "$physical\assets.original\a.txt" 'updated asset'
Put "$physical\assets.original\nested\b.txt" 'new nested asset'
Put "$persist_dir\saved-assets\a.txt" 'old asset'
Put "$persist_dir\saved-assets\custom.txt" 'user resource'
Put "$persist_dir\settings.ini" 'custom settings'
New-Item -ItemType Junction -Path "$physical\assets" -Target "$persist_dir\saved-assets" | Out-Null
New-Item -ItemType HardLink -Path "$physical\settings.ini" -Target "$persist_dir\settings.ini" | Out-Null
$current=Join-Path (Split-Path -Parent $physical) 'current'
New-Item -ItemType Junction -Path $current -Target $physical | Out-Null
$dir=$current
$mapping=@(,@('assets','saved-assets'));$mapping+='settings.ini'
$copy=[pscustomobject]@{phase='post_install';kind='copy-package-tree';source='assets.original';path='assets';persist_path='saved-assets';guard='source-exists';recursive=$true;force=$true}
$cleanup=[pscustomobject]@{phase='post_install';kind='cleanup-package-tree';paths=@('assets.original');source='assets.original';guard='source-exists';recursive=$true;force=$true;missing_ok=$true}
$defaults=[pscustomobject]@{phase='post_install';kind='write-default-file';path='settings.ini';persist_path='settings.ini';guard='persist-missing';content="default`r`nnext`r`n";encoding='ascii'}
$manifest=[pscustomobject]@{version='1.0';post_install='frozen declaration never evaluated';persist=$mapping}
$portscoopRequest=[pscustomobject]@{manifests=[pscustomobject]@{treefixture=[pscustomobject]@{manifest=$manifest;actions=@($copy,$cleanup,$defaults);decisions=@([pscustomobject]@{field='post_install';disposition='rewrite'})}}}
Invoke-HookScript post_install $manifest 64bit
Assert ([IO.File]::ReadAllText("$persist_dir\saved-assets\a.txt") -ceq 'updated asset') 'Resource collision was not replaced'
Assert ([IO.File]::ReadAllText("$persist_dir\saved-assets\custom.txt") -ceq 'user resource') 'Unrelated resource removed'
Assert ([IO.File]::ReadAllText("$persist_dir\saved-assets\nested\b.txt") -ceq 'new nested asset') 'Nested persisted resource missing'
Assert ([IO.File]::ReadAllText("$persist_dir\settings.ini") -ceq 'custom settings') 'Guarded INI overwritten'
Assert (-not [IO.Directory]::Exists("$physical\assets.original")) 'Resource source not cleaned after copy'
Invoke-HookScript post_install $manifest 64bit

# The copy failure must stop the following cleanup even with missing_ok set.
Put "$physical\assets.original\a.txt" 'next asset'
Put "$physical\assets.original\z.txt" 'next z'
Put "$persist_dir\saved-assets\z.txt" 'old z'
$locked=[IO.File]::Open("$persist_dir\saved-assets\z.txt",[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::None)
try {Reject {Invoke-HookScript post_install $manifest 64bit}} finally {$locked.Dispose()}
Assert ([IO.File]::Exists("$physical\assets.original\z.txt")) 'Failed copy ran cleanup'
Assert ([IO.File]::ReadAllText("$persist_dir\saved-assets\a.txt") -ceq 'next asset') 'Earlier successful file replacement lost'

# Validate mappings, flags, phases, and the current/persistence links each time.
$copy.persist_path='outside'
Reject {Invoke-HookScript post_install $manifest 64bit}
$copy.persist_path='saved-assets';$copy.guard='unknown'
Reject {Invoke-HookScript post_install $manifest 64bit}
$copy.guard='source-exists';$copy.recursive='true'
Reject {Invoke-HookScript post_install $manifest 64bit}
$copy.recursive=$true
[IO.Directory]::Delete("$physical\assets")
New-Item -ItemType Junction -Path "$physical\assets" -Target $outside | Out-Null
Reject {Invoke-HookScript post_install $manifest 64bit}
[IO.Directory]::Delete("$physical\assets")
[IO.Directory]::Delete($current)
New-Item -ItemType Junction -Path $current -Target $outside | Out-Null
Reject {Invoke-HookScript post_install $manifest 64bit}
[IO.Directory]::Delete($current)
Assert ([IO.File]::ReadAllText("$outside\sentinel") -ceq 'preserve') 'Outside sentinel changed'
Write-Output 'Tree action fixtures passed'
