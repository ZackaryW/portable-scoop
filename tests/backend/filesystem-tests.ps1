#Requires -Version 5.1
param([Parameter(Mandatory=$true)][string]$Root)
$ErrorActionPreference='Stop'
$project=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
. (Join-Path $project 'backend\scoop\adapter\portscoop-filesystem.ps1')
function Assert([bool]$Condition,[string]$Message){if(-not $Condition){throw $Message}}
function Reject([scriptblock]$Body){$message=$null;try{& $Body}catch{$message=$_.Exception.Message};if(-not $message){throw 'Expected rejection'};return $message}
function Put([string]$Path,[string]$Text){[IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Path))|Out-Null;[IO.File]::WriteAllText($Path,$Text)}
$source=Join-Path $Root 'source'
$destination=Join-Path $Root 'destination'
$outside=Join-Path $Root 'outside'
Put "$source\nested\first.txt" 'new nested'
Put "$source\a.txt" 'new a'
Put "$source\z.txt" 'new z'
Put "$destination\a.txt" 'old a'
Put "$destination\unrelated.txt" 'keep'
Put "$outside\sentinel" 'outside sentinel'
$plan=Get-PortableTreePlan $source @('z.txt','nested','a.txt') -Recurse
Assert (($plan.Entries.Relative -join ',') -ceq 'a.txt,nested,nested\first.txt,z.txt') 'Enumeration is not ordinal and complete'
$null=Reject {Get-PortableTreePlan $source @('..\outside') -Recurse}
$null=Reject {Get-PortableTreePlan $source @('nested') -Recurse -MaxDepth 1}
$null=Reject {Get-PortableTreePlan $source @('nested') -Recurse -MaxBytes 2}
$null=Reject {Get-PortableTreePlan $source @('nested') -Recurse -MaxFileBytes 2}
$null=Reject {Get-PortableTreePlan $source @('nested','a.txt','z.txt') -Recurse -MaxEntries 2}
$null=Reject {Get-PortableTreePlan $source @('*') -Recurse -AllowPrefix}
$null=Reject {Get-PortableTreePlan $source @('nested*\first') -Recurse -AllowPrefix}
$null=Reject {Get-PortableTreePlan $source @('a.txt','A.txt') -Recurse}
$null=Reject {Get-PortableTreePlan $source @('nested')}

# Validate all destination types before replacing the first file.
[IO.Directory]::CreateDirectory("$destination\z.txt")|Out-Null
$null=Reject {Copy-PortableTree $source $destination}
Assert ([IO.File]::ReadAllText("$destination\a.txt") -ceq 'old a') 'Prevalidation changed an earlier file'
[IO.Directory]::Delete("$destination\z.txt")
# Hard-link replacement must not truncate another name for the old file.
New-Item -ItemType HardLink -Path "$outside\old-a.txt" -Target "$destination\a.txt" | Out-Null
Copy-PortableTree $source $destination
Assert ([IO.File]::ReadAllText("$destination\a.txt") -ceq 'new a') 'Copy did not replace collision'
Assert ([IO.File]::ReadAllText("$outside\old-a.txt") -ceq 'old a') 'Copy truncated an external hard link'
Assert ([IO.File]::ReadAllText("$destination\unrelated.txt") -ceq 'keep') 'Copy removed unrelated entries'
Assert ([IO.File]::ReadAllText("$destination\nested\first.txt") -ceq 'new nested') 'Nested copy missing'
$null=Reject {Copy-PortableTree $source "$source\child"}

# No descendant link may be followed, even after an earlier valid selection.
New-Item -ItemType Junction -Path "$source\nested\redirect" -Target $outside | Out-Null
$null=Reject {Copy-PortableTree $source $destination}
$null=Reject {Remove-PortableTree $source @('a.txt','nested') -Recurse -Force}
Assert ([IO.File]::Exists("$source\a.txt")) 'Cleanup mutated before validating all selections'
Assert ([IO.File]::ReadAllText("$outside\sentinel") -ceq 'outside sentinel') 'Reparse target changed'
[IO.Directory]::Delete("$source\nested\redirect")
New-Item -ItemType Junction -Path "$destination\redirect" -Target $outside | Out-Null
Put "$source\redirect\file" 'inert'
$null=Reject {Copy-PortableTree $source $destination}
[IO.Directory]::Delete("$destination\redirect")

# A locked later destination produces an honest partial-copy result; source stays.
Put "$source\a.txt" 'next a'
$locked=[IO.File]::Open("$destination\z.txt",[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::None)
try {$failure=Reject {Copy-PortableTree $source $destination}} finally {$locked.Dispose()}
Assert ($failure -match 'Checked copy stopped after [1-9][0-9]* files') 'Partial-copy count missing'
Assert ([IO.File]::ReadAllText("$destination\a.txt") -ceq 'next a') 'Earlier file was not committed'
Assert ([IO.File]::Exists("$source\z.txt")) 'Failed copy cleaned source'

# Prefix matching treats dollars as literal filenames and never root-wide globs.
$cleanup=Join-Path $Root 'cleanup'
Put (Join-Path $cleanup '$PLUGINSDIR\plugin.dll') 'plugin'
Put (Join-Path $cleanup '$0\file') 'temporary'
Put "$cleanup\Uninstall.exe" 'inert'
Put "$cleanup\keep" 'keep'
Remove-PortableTree $cleanup @('$*','Uninst*') -AllowPrefix -Recurse -Force -MissingOK
Assert (-not [IO.Directory]::Exists((Join-Path $cleanup '$PLUGINSDIR'))) 'Dollar-prefix directory survived'
Assert (-not [IO.File]::Exists("$cleanup\Uninstall.exe")) 'Prefix file survived'
Assert ([IO.File]::ReadAllText("$cleanup\keep") -ceq 'keep') 'Unselected file removed'
Remove-PortableTree $cleanup @('missing') -Recurse -MissingOK
$null=Reject {Remove-PortableTree $cleanup @('missing') -Recurse}
Put "$cleanup\a" 'a';Put "$cleanup\z" 'z'
$locked=[IO.File]::Open("$cleanup\z",[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::None)
try {$failure=Reject {Remove-PortableTree $cleanup @('a','z') -MissingOK -Force}} finally {$locked.Dispose()}
Assert ($failure -match 'Checked cleanup stopped after 1 entries') 'Access error was hidden or partial cleanup misreported'
Assert ([IO.File]::Exists("$cleanup\z")) 'Locked file removed'
[IO.File]::SetAttributes("$cleanup\z",[IO.FileAttributes]::ReadOnly)
Remove-PortableTree $cleanup @('z') -Force
Assert (-not [IO.File]::Exists("$cleanup\z")) 'Force did not remove read-only selection'
Assert ([IO.File]::ReadAllText("$outside\sentinel") -ceq 'outside sentinel') 'Outside sentinel changed'
$empty=Join-Path $Root 'empty'
[IO.Directory]::CreateDirectory($empty)|Out-Null
Copy-PortableTree $empty (Join-Path $Root 'empty-copy')
Assert ([IO.Directory]::Exists((Join-Path $Root 'empty-copy'))) 'Empty tree copy failed'
Write-Output 'Checked filesystem fixtures passed'
